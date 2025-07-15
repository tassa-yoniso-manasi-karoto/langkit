package gui

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/tassa-yoniso-manasi-karoto/dockerutil"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/downloader"
)

// CheckDockerAvailability checks if Docker is available on the system
func (a *App) CheckDockerAvailability() (map[string]interface{}, error) {
	a.getLogger().Debug().Msg("Checking Docker availability")

	// Try to run docker version command
	cmd := executils.NewCommand("docker", "version", "--format", "json")
	output, err := cmd.Output()

	result := map[string]interface{}{
		"available": false,
		"version":   "",
		"engine":    "",
		"error":     "",
	}

	if err != nil {
		// Check if it's a command not found error
		if strings.Contains(err.Error(), "executable file not found") {
			result["error"] = "Docker is not installed"
		} else {
			result["error"] = "Cannot connect to Docker daemon"
		}
		a.getLogger().Debug().Err(err).Msg("Docker check failed")
		return result, nil
	}

	// Parse docker version output
	var versionInfo map[string]interface{}
	if err := json.Unmarshal(output, &versionInfo); err == nil {
		result["available"] = true
		if client, ok := versionInfo["Client"].(map[string]interface{}); ok {
			if version, ok := client["Version"].(string); ok {
				result["version"] = version
			}
		}

		// Get the actual Docker backend name using dockerutil
		engine := dockerutil.DockerBackendName()
		result["engine"] = engine
		a.getLogger().Debug().Str("engine", engine).Msg("Docker engine detected")
	}

	a.getLogger().Debug().Interface("result", result).Msg("Docker check completed")
	return result, nil
}

// CheckInternetConnectivity checks if the system has internet connectivity
func (a *App) CheckInternetConnectivity() (map[string]interface{}, error) {
	a.getLogger().Debug().Msg("Checking internet connectivity")

	result := map[string]interface{}{
		"online":  false,
		"latency": 0,
		"error":   "",
	}

	// Try to connect to multiple reliable hosts
	hosts := []string{
		"1.1.1.1:443",        // Cloudflare DNS
		"8.8.8.8:443",        // Google DNS
		"208.67.222.222:443", // OpenDNS
	}

	for _, host := range hosts {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", host, 3*time.Second)
		if err == nil {
			conn.Close()
			result["online"] = true
			result["latency"] = int(time.Since(start).Milliseconds())
			break
		}
	}

	if !result["online"].(bool) {
		result["error"] = "No internet connection detected"
		a.getLogger().Debug().Msg("Internet connectivity check failed")
	} else {
		a.getLogger().Debug().
			Bool("online", true).
			Int("latency", result["latency"].(int)).
			Msg("Internet connectivity check passed")
	}

	return result, nil
}

// CheckFFmpegAvailability checks if FFmpeg is available on the system
func (a *App) CheckFFmpegAvailability() (map[string]interface{}, error) {
	a.getLogger().Debug().Msg("Checking FFmpeg availability")

	result := map[string]interface{}{
		"available": false,
		"version":   "",
		"path":      "",
		"error":     "",
	}

	// Try to find FFmpeg
	ffmpegPath, err := executils.FindBinary("ffmpeg")
	if err != nil {
		result["error"] = "FFmpeg is not installed or not in PATH"
		a.getLogger().Debug().Err(err).Msg("FFmpeg not found")
		return result, nil
	}

	result["path"] = ffmpegPath

	// Try to get version
	cmd := executils.NewCommand(ffmpegPath, "-version")
	output, err := cmd.Output()
	if err != nil {
		result["error"] = "FFmpeg found but could not determine version"
		a.getLogger().Debug().Err(err).Msg("Failed to get FFmpeg version")
		return result, nil
	}

	// Parse version from output
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	if len(lines) > 0 {
		// First line typically contains version info
		versionLine := lines[0]
		if strings.Contains(versionLine, "ffmpeg version") {
			parts := strings.Fields(versionLine)
			if len(parts) >= 3 {
				result["version"] = parts[2]
			}
		}
	}

	result["available"] = true
	a.getLogger().Debug().
		Str("path", ffmpegPath).
		Str("version", result["version"].(string)).
		Msg("FFmpeg check completed")

	return result, nil
}

// CheckMediaInfoAvailability checks if MediaInfo is available on the system
func (a *App) CheckMediaInfoAvailability() (map[string]interface{}, error) {
	a.getLogger().Debug().Msg("Checking MediaInfo availability")

	result := map[string]interface{}{
		"available": false,
		"version":   "",
		"path":      "",
		"error":     "",
	}

	// Try to find MediaInfo
	mediainfoPath, err := executils.FindBinary("mediainfo")
	if err != nil {
		result["error"] = "MediaInfo is not installed or not in PATH"
		a.getLogger().Debug().Err(err).Msg("MediaInfo not found")
		return result, nil
	}

	result["path"] = mediainfoPath

	// Try to get version
	cmd := executils.NewCommand(mediainfoPath, "--Version")
	output, err := cmd.Output()
	if err != nil {
		result["error"] = "MediaInfo found but could not determine version"
		a.getLogger().Debug().Err(err).Msg("Failed to get MediaInfo version")
		return result, nil
	}

	// Parse version from output
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "MediaInfo") && strings.Contains(line, "v") {
			// Extract version number
			if idx := strings.Index(line, "v"); idx != -1 {
				version := strings.TrimSpace(line[idx+1:])
				// Clean up version string
				if spaceIdx := strings.Index(version, " "); spaceIdx != -1 {
					version = version[:spaceIdx]
				}
				result["version"] = version
				break
			}
		}
	}

	result["available"] = true
	a.getLogger().Debug().
		Str("path", mediainfoPath).
		Str("version", result["version"].(string)).
		Msg("MediaInfo check completed")

	return result, nil
}

// DownloadFFmpeg automatically downloads and extracts FFmpeg using a primary/fallback strategy.
func (a *App) DownloadFFmpeg() (string, error) {
	a.getLogger().Info().Msg("Starting FFmpeg download...")

	// --- Primary Method: BtbN/FFmpeg-Builds ---
	var url string
	var err error

	a.getLogger().Info().Msg("Attempting to download from primary source: BtbN/FFmpeg-Builds")
	var keywords []string
	switch goruntime.GOOS {
	case "windows":
		switch goruntime.GOARCH {
		case "amd64":
			keywords = []string{"win64", "master-latest", "gpl.zip"}
		case "arm64":
			keywords = []string{"winarm64", "master-latest", "gpl.zip"}
		default:
			err = fmt.Errorf("unsupported architecture for Windows: %s", goruntime.GOARCH)
		}
		if err == nil {
			url, err = downloader.GetDownloadURLForAsset("BtbN/FFmpeg-Builds", keywords)
		}
	case "darwin":
		url = "https://evermeet.cx/ffmpeg/get/zip" // This source is reliable for macOS
	default:
		err = fmt.Errorf("automatic download not supported for this OS")
	}

	if err != nil {
		a.getLogger().Warn().Err(err).Msg("Primary download source failed, attempting fallback")
		// --- Fallback Method: langkit-assets ---
		var fallbackAssetPrefix string
		switch goruntime.GOOS {
		case "windows":
			switch goruntime.GOARCH {
			case "amd64":
				fallbackAssetPrefix = "ffmpeg-windows-amd64"
			case "arm64":
				fallbackAssetPrefix = "ffmpeg-windows-arm64"
			}
		case "darwin":
			fallbackAssetPrefix = "ffmpeg-macos-universal"
		}

		if fallbackAssetPrefix != "" {
			url, err = downloader.GetDownloadURLForAsset("tassa-yoniso-manasi-karoto/langkit-assets", []string{fallbackAssetPrefix, ".zip"})
			if err != nil {
				a.getLogger().Error().Err(err).Msg("Fallback download source also failed")
				return "", err
			}
		} else {
			return "", err // Return the original error if no fallback is available
		}
	}

	return a.downloadAndExtract("ffmpeg", url, []string{"ffmpeg", "ffmpeg.exe"})
}

// DownloadMediaInfo automatically downloads and extracts MediaInfo CLI from the self-hosted repo.
func (a *App) DownloadMediaInfo() (string, error) {
	a.getLogger().Info().Msg("Starting MediaInfo CLI download...")

	var assetPrefix string
	var filesToExtract []string

	switch goruntime.GOOS {
	case "windows":
		switch goruntime.GOARCH {
		case "amd64":
			assetPrefix = "mediainfo-windows-amd64"
		case "arm64":
			assetPrefix = "mediainfo-windows-arm64"
		default:
			return "", fmt.Errorf("unsupported architecture for Windows: %s", goruntime.GOARCH)
		}
		filesToExtract = []string{"MediaInfo.exe"}
	case "darwin":
		assetPrefix = "mediainfo-macos-universal"
		filesToExtract = []string{"mediainfo"}
	default:
		return "", fmt.Errorf("automatic download not supported for this OS")
	}

	url, err := downloader.GetDownloadURLForAsset("tassa-yoniso-manasi-karoto/langkit-assets", []string{assetPrefix, ".zip"})
	if err != nil {
		a.getLogger().Error().Err(err).Msg("Failed to get MediaInfo download URL from langkit-assets")
		return "", err
	}

	return a.downloadAndExtract("mediainfo", url, filesToExtract)
}

// downloadAndExtract is a helper function to handle the download and extraction process.
func (a *App) downloadAndExtract(dependencyName, url string, filesToExtract []string) (string, error) {
	a.getLogger().Debug().Str("url", url).Msgf("Got %s download URL", dependencyName)

	// Download the zip file with progress
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	tmpFile, err := os.CreateTemp("", dependencyName+"-*.zip")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())

	progressReader := &downloader.ProgressReader{
		Reader: resp.Body,
		Total:  resp.ContentLength,
		Handler: func(p float64, read, total int64, speed float64) {
			if a.wsServer != nil {
				a.wsServer.Emit("download."+dependencyName+".progress", map[string]interface{}{
					"progress":    p,
					"read":        read,
					"total":       total,
					"speed":       speed,
					"description": fmt.Sprintf("%s / %s (%s/s)", humanize.Bytes(uint64(read)), humanize.Bytes(uint64(total)), humanize.Bytes(uint64(speed))),
				})
			}
		},
	}

	if _, err = io.Copy(tmpFile, progressReader); err != nil {
		return "", err
	}
	tmpFile.Close()

	a.getLogger().Info().Msgf("%s download complete, starting extraction", dependencyName)

	toolsDir, err := config.GetToolsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return "", err
	}

	if err := downloader.ExtractZip(tmpFile.Name(), toolsDir, filesToExtract); err != nil {
		return "", err
	}

	var executablePath string
	for _, file := range filesToExtract {
		if strings.HasSuffix(file, ".exe") || (goruntime.GOOS != "windows" && !strings.HasSuffix(file, ".dll")) {
			executablePath = filepath.Join(toolsDir, file)
			break
		}
	}

	if executablePath == "" {
		return "", fmt.Errorf("could not find executable in extracted files for %s", dependencyName)
	}

	a.getLogger().Info().Str("path", executablePath).Msgf("%s extracted successfully", dependencyName)

	settings, err := config.LoadSettings()
	if err != nil {
		return "", err
	}
	if dependencyName == "ffmpeg" {
		settings.FFmpegPath = executablePath
	} else if dependencyName == "mediainfo" {
		settings.MediaInfoPath = executablePath
	}
	if err := config.SaveSettings(settings); err != nil {
		return "", err
	}
	a.getLogger().Info().Msgf("Saved new %s path to settings", dependencyName)
	
	return executablePath, nil
}