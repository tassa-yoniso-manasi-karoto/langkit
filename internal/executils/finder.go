package executils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
)

// FindBinary searches for a binary with a 4-tier priority:
// 1. Saved path setting
// 2. Local `tools` folder (from XDG data directory)
// 3. Local `bin` folder (relative to the executable)
// 4. System `PATH`
func FindBinary(name string) (string, error) {
	// Add .exe extension on Windows
	if goruntime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		name += ".exe"
	}

	// 1. Check saved path in settings
	settings, err := config.LoadSettings()
	if err == nil {
		var savedPath string
		if strings.HasPrefix(name, "ffmpeg") {
			savedPath = settings.FFmpegPath
		} else if strings.HasPrefix(name, "mediainfo") {
			savedPath = settings.MediaInfoPath
		}
		if savedPath != "" {
			if _, err := os.Stat(savedPath); err == nil {
				return savedPath, nil
			}
		}
	}

	// 2. Check local tools folder
	toolsDir, err := config.GetToolsDir()
	if err == nil {
		localPath := filepath.Join(toolsDir, name)
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
	}

	// 3. Check local bin folder
	ex, err := os.Executable()
	if err == nil {
		localPath := filepath.Join(filepath.Dir(ex), "bin", name)
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
	}

	// 4. Fall back to checking PATH
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("%s not found in standard locations", name)
}
