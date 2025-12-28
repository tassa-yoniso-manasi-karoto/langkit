package voice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/dustin/go-humanize"
	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/retrypolicy"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"
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

// DemucsModelFile represents a model file with its CDN URL and local filename.
type DemucsModelFile struct {
	URL       string // Full CDN URL
	LocalName string // Local filename (8-char checksum only, per demucs-next)
}

// DemucsModelURLs maps demucs model names to their download info.
// demucs-next uses only the 8-char checksum portion for filenames.
// Note: htdemucs and htdemucs_ft share the first layer file.
var DemucsModelURLs = map[string][]DemucsModelFile{
	"htdemucs": {
		{"https://dl.fbaipublicfiles.com/demucs/hybrid_transformer/955717e8-8726e21a.th", "8726e21a.th"},
	},
	"htdemucs_ft": {
		{"https://dl.fbaipublicfiles.com/demucs/hybrid_transformer/955717e8-8726e21a.th", "8726e21a.th"},
		{"https://dl.fbaipublicfiles.com/demucs/hybrid_transformer/f7e0c4bc-ba3fe64a.th", "ba3fe64a.th"},
		{"https://dl.fbaipublicfiles.com/demucs/hybrid_transformer/d12395a8-e57c48e6.th", "e57c48e6.th"},
		{"https://dl.fbaipublicfiles.com/demucs/hybrid_transformer/92cfc3b6-ef3bcb9c.th", "ef3bcb9c.th"},
	},
}

// DemucsModelFiles maps demucs model names to their expected weight files.
// Uses the 8-char checksum filenames that demucs-next expects.
var DemucsModelFiles = map[string][]string{
	"htdemucs": {
		"8726e21a.th",
	},
	"htdemucs_ft": {
		"8726e21a.th",
		"ba3fe64a.th",
		"e57c48e6.th",
		"ef3bcb9c.th",
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

// PreDownloadDemucsModel downloads model weights before starting demucs container.
// This provides reliable progress tracking with IncrementDownloadProgress.
// If all files already exist, this is a no-op.
// Returns nil on success (including when files already exist).
func PreDownloadDemucsModel(ctx context.Context, model string, modelsDir string, handler ProgressHandler) error {
	files, ok := DemucsModelURLs[model]
	if !ok {
		Logger.Debug().Str("model", model).Msg("No pre-download URLs for model, will use demucs internal download")
		return nil
	}

	// Check which files need downloading
	var filesToDownload []DemucsModelFile
	for _, f := range files {
		localPath := filepath.Join(modelsDir, f.LocalName)
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			filesToDownload = append(filesToDownload, f)
		}
	}

	if len(filesToDownload) == 0 {
		Logger.Debug().Str("model", model).Msg("All model files already exist, skipping pre-download")
		return nil
	}

	if handler != nil {
		handler.ZeroLog().Info().
			Str("model", model).
			Int("files", len(filesToDownload)).
			Msg("Pre-downloading demucs model weights...")
	}

	taskID := progress.BarDemucsModelDL

	totalFiles := len(filesToDownload)
	for i, f := range filesToDownload {
		localPath := filepath.Join(modelsDir, f.LocalName)

		Logger.Debug().
			Str("url", f.URL).
			Str("dest", localPath).
			Int("file", i+1).
			Int("total", totalFiles).
			Msg("Downloading model file")

		// Build description with layer count for multi-file models
		description := "Downloading model weights..."
		if totalFiles > 1 {
			description = fmt.Sprintf("Downloading model weights... (%d/%d)", i+1, totalFiles)
		}

		if err := downloadFileWithProgress(ctx, f.URL, localPath, taskID, description, handler); err != nil {
			// Clean up partial download
			os.Remove(localPath)
			if handler != nil {
				handler.RemoveProgressBar(taskID)
			}
			return fmt.Errorf("failed to download %s: %w", f.LocalName, err)
		}
	}

	if handler != nil {
		handler.RemoveProgressBar(taskID)
		handler.ZeroLog().Info().Str("model", model).Msg("Model weights pre-download complete")
	}

	return nil
}

// downloadFileWithProgress downloads a file with progress reporting via IncrementDownloadProgress.
func downloadFileWithProgress(ctx context.Context, url, destPath, taskID, description string, handler ProgressHandler) error {
	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	totalSize := resp.ContentLength
	if totalSize <= 0 {
		// If Content-Length is not available, we can't show progress
		Logger.Warn().Str("url", url).Msg("Content-Length not available, progress won't be accurate")
		totalSize = 100 * 1024 * 1024 // Assume ~100MB as fallback
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Download with progress tracking
	buf := make([]byte, 32*1024) // 32KB buffer
	var downloaded int64
	var lastReportedPercent int

	for {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			downloaded += int64(n)

			// Report progress
			if handler != nil && totalSize > 0 {
				currentPercent := int(downloaded * 100 / totalSize)
				if currentPercent > lastReportedPercent {
					increment := currentPercent - lastReportedPercent
					humanizedSize := humanize.Bytes(uint64(downloaded)) + " / " + humanize.Bytes(uint64(totalSize))
					handler.IncrementDownloadProgress(taskID, increment, 100, 25, "Demucs Setup", description, "", humanizedSize)
					lastReportedPercent = currentPercent
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("failed to read response: %w", readErr)
		}
	}

	return nil
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
	// Check for context cancellation (including wrapped errors)
	if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled") {
		return true
	}
	// Check for CUDA OOM
	if err == ErrCUDAOutOfMemory {
		return true
	}
	return false
}
