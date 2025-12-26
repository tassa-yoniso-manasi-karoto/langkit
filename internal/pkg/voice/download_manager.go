package voice

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/retrypolicy"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
)

// DownloadExpectation tracks expected files for a model download operation.
// Used for cleanup when download fails (network error or corrupt file).
type DownloadExpectation struct {
	ModelDir     string   // Directory where model files are stored
	ModelFiles   []string // Expected file names for this model
	ProgressBars []string // Progress bar IDs to remove on cleanup
	Handler      ProgressHandler
}

// Cleanup removes incomplete/corrupt model files and progress bars.
// Called on retry after a download failure.
func (d *DownloadExpectation) Cleanup() {
	if d == nil {
		return
	}

	// Remove progress bars
	if d.Handler != nil {
		for _, barID := range d.ProgressBars {
			d.Handler.RemoveProgressBar(barID)
		}
	}

	// Delete model files that may be incomplete/corrupt
	for _, filename := range d.ModelFiles {
		filePath := filepath.Join(d.ModelDir, filename)
		if err := os.Remove(filePath); err == nil {
			Logger.Info().Str("file", filePath).Msg("Deleted incomplete model file for retry")
		}
	}
}

// Model file mappings - files expected for each model type

// DemucsModelFiles maps demucs model names to their expected weight files.
// These are downloaded by the demucs library from fbaipublicfiles.com.
var DemucsModelFiles = map[string][]string{
	"htdemucs": {
		"955717e8-8726e21a.th",
		"htdemucs.yaml",
	},
	"htdemucs_ft": {
		"f7e0c4bc-ba3fe64a.th",
		"d12395a8-e57c48e6.th",
		"92cfc3b6-ef3bcb9c.th",
		"04573f0d-f3cf25b2.th",
		"htdemucs_ft.yaml",
	},
}

// AudioSepModelFiles maps audio-separator model names to their expected files.
var AudioSepModelFiles = map[string][]string{
	"vocals_mel_band_roformer.ckpt": {"vocals_mel_band_roformer.ckpt"},
	// More models can be added here as needed
}

// Shared model directory functions

// GetDemucsModelsDir returns the XDG-compliant shared directory for demucs models.
// Path: ~/.config/demucs-models/
func GetDemucsModelsDir() (string, error) {
	modelsDir := filepath.Join(xdg.ConfigHome, "demucs-models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return "", err
	}
	return modelsDir, nil
}

// GetAudioSepModelsDir returns the XDG-compliant shared directory for audio-separator models.
// Path: ~/.config/audio-separator-models/
func GetAudioSepModelsDir() (string, error) {
	modelsDir := filepath.Join(xdg.ConfigHome, "audio-separator-models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		return "", err
	}
	return modelsDir, nil
}

// MigrateDemucsModels migrates model files from old per-container directories
// to the new shared directory. This is a one-time migration.
// Returns true if migration occurred (caller should recreate containers).
//
// Note: Old model files are typically root-owned (created by Docker containers),
// so we use Docker to copy them to the user-owned new location.
func MigrateDemucsModels() (migrated bool, err error) {
	newDir, err := GetDemucsModelsDir()
	if err != nil {
		return false, err
	}

	// Old directories from per-container setup
	oldDirs := []string{
		filepath.Join(xdg.ConfigHome, "langkit-demucs-gpu", "models"),
		filepath.Join(xdg.ConfigHome, "langkit-demucs", "models"),
	}

	// Collect directories that have files needing migration
	var dirsToMigrate []string
	for _, oldDir := range oldDirs {
		if _, err := os.Stat(oldDir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(oldDir)
		if err != nil {
			Logger.Warn().Err(err).Str("dir", oldDir).Msg("Failed to read old model directory")
			continue
		}

		// Check if any files need migration
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			newPath := filepath.Join(newDir, entry.Name())
			if _, err := os.Stat(newPath); os.IsNotExist(err) {
				// File needs migration
				dirsToMigrate = append(dirsToMigrate, oldDir)
				break
			}
		}
	}

	if len(dirsToMigrate) == 0 {
		return false, nil
	}

	// Use Docker to copy root-owned files to user-owned destination
	Logger.Info().
		Strs("dirs", dirsToMigrate).
		Str("dest", newDir).
		Msg("Migrating model files from old directories (using Docker for root-owned files)")

	if err := migrateWithDocker(dirsToMigrate, newDir); err != nil {
		return false, fmt.Errorf("docker migration failed: %w", err)
	}

	return true, nil
}

// migrateWithDocker uses Docker CLI to copy root-owned model files
// to the new user-owned shared directory, then deletes the old directories.
// Uses CLI instead of Go SDK for better Windows compatibility.
func migrateWithDocker(oldDirs []string, newDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get current user's UID/GID for chown
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	// Build docker run command with volume mounts
	// docker run --rm -v old1:/old0 -v old2:/old1 -v new:/new busybox sh -c '...'
	// Note: old dirs are NOT read-only since we need to delete them after copy
	args := []string{"run", "--rm"}

	// Add source directory mounts (read-write for deletion)
	var copyCommands []string
	var deleteCommands []string
	for i, oldDir := range oldDirs {
		srcMount := fmt.Sprintf("/old%d", i)
		args = append(args, "-v", oldDir+":"+srcMount)
		// Copy all files, preserving names (cp -n = no clobber)
		copyCommands = append(copyCommands, fmt.Sprintf("cp -n %s/* /new/ 2>/dev/null || true", srcMount))
		// Delete old directory contents after copy (including hidden files)
		// find -mindepth 1 ensures we delete contents but not the mount point itself
		deleteCommands = append(deleteCommands, fmt.Sprintf("find %s -mindepth 1 -delete", srcMount))
	}

	// Add destination directory mount
	args = append(args, "-v", newDir+":/new")

	// Build shell command: copy files, chown to current user, then delete old files
	shellCmd := fmt.Sprintf(
		"(%s) && chown -R %s:%s /new/* && (%s)",
		joinCommands(copyCommands),
		currentUser.Uid,
		currentUser.Gid,
		joinCommands(deleteCommands),
	)

	args = append(args, "busybox", "sh", "-c", shellCmd)

	Logger.Debug().Strs("args", args).Msg("Running docker migration command")

	cmd := executils.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("migration container failed: %w\nOutput: %s", err, string(output))
	}

	// Try to remove the now-empty old directories (as user, should work if empty)
	for _, oldDir := range oldDirs {
		if err := os.Remove(oldDir); err == nil {
			Logger.Debug().Str("dir", oldDir).Msg("Removed empty old model directory")
		}
	}

	Logger.Info().Msg("Model files migrated successfully")
	return nil
}

// joinCommands joins shell commands with &&
func joinCommands(cmds []string) string {
	if len(cmds) == 0 {
		return "true"
	}
	result := cmds[0]
	for _, cmd := range cmds[1:] {
		result += " && " + cmd
	}
	return result
}

// buildRetryPolicyWithCleanup creates a retry policy that cleans up on retry.
// This is used for operations that may download model files.
func buildRetryPolicyWithCleanup[R any](maxTry int, expectation *DownloadExpectation) failsafe.Policy[R] {
	builder := retrypolicy.Builder[R]().
		// Handle download failures (these should be retried)
		HandleIf(func(_ R, err error) bool {
			if err == nil {
				return false
			}
			// Retry on download failures
			if isModelDownloadError(err) {
				return true
			}
			// Don't retry on CUDA OOM or cancellation
			if isAbortError(err) {
				return false
			}
			// Retry other errors
			return true
		}).
		// Abort immediately on these errors (no retry)
		AbortIf(func(_ R, err error) bool {
			return isAbortError(err)
		}).
		WithMaxAttempts(maxTry).
		ReturnLastFailure().
		WithBackoffFactor(500*time.Millisecond, 5*time.Second, 2.0)

	// Add cleanup on retry for download-related errors
	if expectation != nil {
		builder = builder.OnRetry(func(evt failsafe.ExecutionEvent[R]) {
			err := evt.LastError()
			Logger.Warn().
				Int("attempt", evt.Attempts()).
				Err(err).
				Msg("Retry policy: attempt failed, retrying...")

			// Clean up incomplete files if this was a download failure
			if isModelDownloadError(err) {
				Logger.Info().Msg("Cleaning up incomplete model files before retry")
				expectation.Cleanup()
			}
		})
	} else {
		builder = builder.OnRetry(func(evt failsafe.ExecutionEvent[R]) {
			Logger.Warn().
				Int("attempt", evt.Attempts()).
				Err(evt.LastError()).
				Msg("Retry policy: attempt failed, retrying...")
		})
	}

	return builder.Build()
}

// isModelDownloadError checks if an error is related to model download failure
// This includes network errors and unreadable model files (likely incomplete downloads)
func isModelDownloadError(err error) bool {
	return err == ErrModelDownloadFailed
}

// isAbortError checks if an error should abort retries immediately
func isAbortError(err error) bool {
	if err == nil {
		return false
	}
	// Check for context cancellation
	if err.Error() == "context canceled" {
		return true
	}
	// Check for CUDA OOM
	if err == ErrCUDAOutOfMemory {
		return true
	}
	return false
}
