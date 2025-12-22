package fsutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/disk"
)

const GB = 1024 * 1024 * 1024

// GetAvailableDiskSpace returns available disk space in bytes for the given path.
// Works cross-platform (Linux, macOS, Windows) via gopsutil.
func GetAvailableDiskSpace(path string) (uint64, error) {
	usage, err := disk.Usage(path)
	if err != nil {
		return 0, fmt.Errorf("failed to get disk space for %s: %w", path, err)
	}
	return usage.Free, nil
}

// SameFilesystem checks if two paths are on the same filesystem.
// Uses gopsutil Partitions() for cross-platform support (Linux, macOS, Windows).
func SameFilesystem(path1, path2 string) (bool, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return false, fmt.Errorf("failed to get partitions: %w", err)
	}

	mount1 := findMountpoint(path1, partitions)
	mount2 := findMountpoint(path2, partitions)

	return mount1 == mount2 && mount1 != "", nil
}

// findMountpoint finds the mountpoint that contains the given path.
// Returns the longest matching mountpoint (most specific).
func findMountpoint(path string, partitions []disk.PartitionStat) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return ""
	}

	var bestMatch string
	for _, p := range partitions {
		if strings.HasPrefix(absPath, p.Mountpoint) && len(p.Mountpoint) > len(bestMatch) {
			bestMatch = p.Mountpoint
		}
	}
	return bestMatch
}

// GetDockerDataRoot returns the Docker data root directory where images are stored.
// Falls back to XDG_DATA_HOME/docker or common locations if docker info fails.
func GetDockerDataRoot() (string, error) {
	// Try to get from docker info command
	cmd := exec.Command("docker", "info", "--format", "{{.DockerRootDir}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err == nil {
		dataRoot := strings.TrimSpace(out.String())
		if dataRoot != "" {
			return dataRoot, nil
		}
	}

	// Fallback to common locations if docker info fails
	// XDG_DATA_HOME/docker is used by rootless Docker, /var/lib/docker is the standard system location
	commonPaths := []string{
		filepath.Join(xdg.DataHome, "docker"),
		"/var/lib/docker",
	}
	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("could not determine Docker data root")
}

// CheckDiskSpace validates that there is sufficient disk space at the given path.
// Returns an error if insufficient space, nil if OK.
func CheckDiskSpace(path string, requiredGB int, logger *zerolog.Logger) error {
	available, err := GetAvailableDiskSpace(path)
	if err != nil {
		return err
	}

	availableGB := float64(available) / float64(GB)
	requiredBytes := uint64(requiredGB) * GB

	if available < requiredBytes {
		return fmt.Errorf("insufficient disk space at %s: %.2f GB available, %d GB required",
			path, availableGB, requiredGB)
	}

	logger.Debug().
		Str("path", path).
		Float64("available_gb", availableGB).
		Int("required_gb", requiredGB).
		Msg("Disk space check passed")

	return nil
}

// CheckDiskSpaceBoth checks disk space for both the media path and Docker data root (if needed).
// Returns an error if either location has insufficient space.
func CheckDiskSpaceBoth(mediaPath string, requiredGB int, checkDocker bool, logger *zerolog.Logger) error {
	// Check the media/processing path
	if err := CheckDiskSpace(mediaPath, requiredGB, logger); err != nil {
		return err
	}

	// Also check Docker data root if requested
	if checkDocker {
		dockerRoot, err := GetDockerDataRoot()
		if err != nil {
			logger.Warn().Err(err).Msg("Could not determine Docker data root for disk space check")
			return nil // Don't fail if we can't determine Docker root
		}

		// Check if Docker root is on the same filesystem as the media path
		sameFS, err := SameFilesystem(mediaPath, dockerRoot)
		if err != nil {
			logger.Warn().Err(err).Msg("Could not determine if Docker root is on same filesystem")
			// Fall through to check Docker root anyway
		} else if sameFS {
			logger.Debug().Msg("Docker data root is on same filesystem as media path, skipping duplicate check")
			return nil
		}

		// Docker root is on a different filesystem, check its disk space
		if err := CheckDiskSpace(dockerRoot, requiredGB, logger); err != nil {
			return fmt.Errorf("Docker data root: %w", err)
		}
	}

	return nil
}

// LogDiskSpaceWarnings logs warnings/errors based on available disk space during processing.
// Logs at Error level if less than 1 GB available, Warn level if less than 5 GB.
// Does NOT return errors - this is non-blocking logging only.
func LogDiskSpaceWarnings(path string, logger *zerolog.Logger) {
	available, err := GetAvailableDiskSpace(path)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to check disk space during processing")
		return
	}

	availableGB := float64(available) / float64(GB)

	if available < GB {
		logger.Error().
			Float64("available_gb", availableGB).
			Msg("Critically low disk space - less than 1 GB available")
	} else if available < 5*GB {
		logger.Warn().
			Float64("available_gb", availableGB).
			Msg("Low disk space - less than 5 GB available")
	}
}
