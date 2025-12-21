package services

import (
	"context"
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
	"github.com/rs/zerolog"
	"github.com/tassa-yoniso-manasi-karoto/dockerutil"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/interfaces"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/generated"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/downloader"
)

// Compile-time check that DependencyService implements api.Service
var _ api.Service = (*DependencyService)(nil)

// DependencyService implements the WebRPC DependencyService interface
type DependencyService struct {
	logger              zerolog.Logger
	handler             http.Handler
	websocketService interfaces.WebsocketService
}

// NewDependencyService creates a new dependency service
func NewDependencyService(logger zerolog.Logger, websocketService interfaces.WebsocketService) *DependencyService {
	svc := &DependencyService{
		logger:              logger,
		websocketService: websocketService,
	}
	
	// Create the WebRPC handler
	svc.handler = generated.NewDependencyServiceServer(svc)
	
	return svc
}

// Name implements api.Service
func (s *DependencyService) Name() string {
	return "DependencyService"
}

// Handler implements api.Service
func (s *DependencyService) Handler() http.Handler {
	return s.handler
}

// Description implements api.Service
func (s *DependencyService) Description() string {
	return "Dependency management and system checks"
}

// CheckDockerAvailability checks if Docker is available on the system
func (s *DependencyService) CheckDockerAvailability(ctx context.Context) (*generated.DockerStatus, error) {
	// Try to run docker version command
	cmd := executils.NewCommand("docker", "version", "--format", "json")
	output, err := cmd.Output()

	result := &generated.DockerStatus{
		Available: false,
		Version:   "",
		Engine:    "",
		Error:     nil,
	}

	if err != nil {
		// Check if it's a command not found error
		errMsg := ""
		if strings.Contains(err.Error(), "executable file not found") {
			errMsg = "Docker is not installed"
		} else {
			errMsg = "Cannot connect to Docker daemon"
		}
		result.Error = &errMsg
		s.logger.Debug().Err(err).Msg("Docker check failed")
		return result, nil
	}

	// Parse docker version output
	var versionInfo map[string]interface{}
	if err := json.Unmarshal(output, &versionInfo); err == nil {
		result.Available = true
		if client, ok := versionInfo["Client"].(map[string]interface{}); ok {
			if version, ok := client["Version"].(string); ok {
				result.Version = version
			}
		}

		// Get the actual Docker backend name using dockerutil
		engine := dockerutil.DockerBackendName()
		result.Engine = engine
		s.logger.Debug().Str("engine", engine).Msg("Docker engine detected")
	}

	s.logger.Debug().Interface("result", result).Msg("Docker check completed")
	return result, nil
}

// CheckInternetConnectivity checks if the system has internet connectivity
func (s *DependencyService) CheckInternetConnectivity(ctx context.Context) (*generated.InternetStatus, error) {
	result := &generated.InternetStatus{
		Online:  false,
		Latency: 0,
		Error:   nil,
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
			result.Online = true
			result.Latency = int64(time.Since(start).Milliseconds())
			break
		}
	}

	if !result.Online {
		errMsg := "No internet connection detected"
		result.Error = &errMsg
		s.logger.Debug().Msg("Internet connectivity check failed")
	} else {
		s.logger.Debug().
			Bool("online", true).
			Int64("latency", result.Latency).
			Msg("Internet connectivity check passed")
	}

	return result, nil
}

// CheckFFmpegAvailability checks if FFmpeg is available on the system
func (s *DependencyService) CheckFFmpegAvailability(ctx context.Context) (*generated.FFmpegStatus, error) {
	result := &generated.FFmpegStatus{
		Available: false,
		Version:   "",
		Path:      "",
		Error:     nil,
	}

	// Try to find FFmpeg
	ffmpegPath, err := config.FindBinary("ffmpeg")
	if err != nil {
		errMsg := "FFmpeg is not installed"
		result.Error = &errMsg
		s.logger.Debug().Err(err).Msg("FFmpeg not found")
		return result, nil
	}

	result.Path = ffmpegPath

	// Try to get version
	cmd := executils.NewCommand(ffmpegPath, "-version")
	output, err := cmd.Output()
	if err != nil {
		errMsg := "FFmpeg found but could not determine version"
		result.Error = &errMsg
		s.logger.Debug().Err(err).Msg("Failed to get FFmpeg version")
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
				result.Version = parts[2]
			}
		}
	}

	result.Available = true
	s.logger.Debug().
		Str("path", ffmpegPath).
		Str("version", result.Version).
		Msg("FFmpeg check completed")

	return result, nil
}

// CheckMediaInfoAvailability checks if MediaInfo is available on the system
func (s *DependencyService) CheckMediaInfoAvailability(ctx context.Context) (*generated.MediaInfoStatus, error) {
	result := &generated.MediaInfoStatus{
		Available: false,
		Version:   "",
		Path:      "",
		Error:     nil,
	}

	// Try to find MediaInfo
	mediainfoPath, err := config.FindBinary("mediainfo")
	if err != nil {
		errMsg := "MediaInfo CLI is not installed"
		result.Error = &errMsg
		s.logger.Debug().Err(err).Msg("MediaInfo not found")
		return result, nil
	}

	result.Path = mediainfoPath

	// Try to get version
	cmd := executils.NewCommand(mediainfoPath, "--Version")
	output, err := cmd.Output()
	if err != nil {
		errMsg := "MediaInfo found but could not determine version"
		result.Error = &errMsg
		s.logger.Debug().Err(err).Msg("Failed to get MediaInfo version")
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
				result.Version = version
				break
			}
		}
	}

	result.Available = true
	s.logger.Debug().
		Str("path", mediainfoPath).
		Str("version", result.Version).
		Msg("MediaInfo check completed")

	return result, nil
}

// DownloadFFmpeg automatically downloads and extracts FFmpeg
func (s *DependencyService) DownloadFFmpeg(ctx context.Context) (*generated.DownloadResult, error) {
	s.logger.Info().Msg("Starting FFmpeg download...")

	// --- Primary Method: BtbN/FFmpeg-Builds ---
	var url string
	var err error

	s.logger.Info().Msg("Attempting to download from primary source: BtbN/FFmpeg-Builds")
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
		s.logger.Warn().Err(err).Msg("Primary download source failed, attempting fallback")
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
				s.logger.Error().Err(err).Msg("Fallback download source also failed")
				errMsg := err.Error()
				return &generated.DownloadResult{Path: "", Error: &errMsg}, nil
			}
		} else {
			errMsg := err.Error()
			return &generated.DownloadResult{Path: "", Error: &errMsg}, nil
		}
	}

	path, err := s.downloadAndExtract("ffmpeg", url, []string{"ffmpeg", "ffmpeg.exe"})
	if err != nil {
		errMsg := err.Error()
		return &generated.DownloadResult{Path: "", Error: &errMsg}, nil
	}

	return &generated.DownloadResult{Path: path, Error: nil}, nil
}

// DownloadMediaInfo automatically downloads and extracts MediaInfo CLI
func (s *DependencyService) DownloadMediaInfo(ctx context.Context) (*generated.DownloadResult, error) {
	s.logger.Info().Msg("Starting MediaInfo CLI download...")

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
			errMsg := fmt.Sprintf("unsupported architecture for Windows: %s", goruntime.GOARCH)
			return &generated.DownloadResult{Path: "", Error: &errMsg}, nil
		}
		filesToExtract = []string{"MediaInfo.exe"}
	case "darwin":
		assetPrefix = "mediainfo-macos-universal"
		filesToExtract = []string{"mediainfo"}
	default:
		errMsg := "automatic download not supported for this OS"
		return &generated.DownloadResult{Path: "", Error: &errMsg}, nil
	}

	url, err := downloader.GetDownloadURLForAsset("tassa-yoniso-manasi-karoto/langkit-assets", []string{assetPrefix, ".zip"})
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to get MediaInfo download URL from langkit-assets")
		errMsg := err.Error()
		return &generated.DownloadResult{Path: "", Error: &errMsg}, nil
	}

	path, err := s.downloadAndExtract("mediainfo", url, filesToExtract)
	if err != nil {
		errMsg := err.Error()
		return &generated.DownloadResult{Path: "", Error: &errMsg}, nil
	}

	return &generated.DownloadResult{Path: path, Error: nil}, nil
}

// downloadAndExtract is a helper function to handle the download and extraction process
func (s *DependencyService) downloadAndExtract(dependencyName, url string, filesToExtract []string) (string, error) {
	s.logger.Debug().Str("url", url).Msgf("Got %s download URL", dependencyName)

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
			if s.websocketService != nil {
				s.websocketService.Emit("download."+dependencyName+".progress", map[string]interface{}{
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

	s.logger.Info().Msgf("%s download complete, starting extraction", dependencyName)

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

	s.logger.Info().Str("path", executablePath).Msgf("%s extracted successfully", dependencyName)

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
	s.logger.Info().Msgf("Saved new %s path to settings", dependencyName)

	return executablePath, nil
}