package gui

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/ui"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/ui/dialogs"
)

// VideoInfo represents information about a video file
type VideoInfo struct {
	Name string `json:\"name\"`
	Path string `json:\"path\"`
}

// GetVideosInDirectory scans a directory and returns information about video files
func (a *App) GetVideosInDirectory(dirPath string) ([]VideoInfo, error) {
	var videos []VideoInfo

	// Common video file extensions
	videoExts := map[string]bool{
		".mp4":  true,
		".mkv":  true,
		".avi":  true,
		".mov":  true,
		".wmv":  true,
		".flv":  true,
		".webm": true,
		".m4v":  true,
	}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file has video extension
		ext := strings.ToLower(filepath.Ext(path))
		if videoExts[ext] {
			videos = append(videos, VideoInfo{
				Name: info.Name(),
				Path: path,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return videos, nil
}

func (a *App) OpenVideoDialog() (string, error) {
	return ui.GetFileDialog().OpenFile(dialogs.OpenFileOptions{
		Title: "Select Video File",
		Filters: []dialogs.FileFilter{
			{
				DisplayName: "Video Files",
				Pattern:     "*.mp4;*.mkv;*.avi;*.mov;*.wmv;*.flv;*.webm;*.m4v",
			},
		},
	})
}

func (a *App) OpenDirectoryDialog() (string, error) {
	return ui.GetFileDialog().OpenDirectory(dialogs.OpenDirectoryOptions{
		Title: "Select Media Directory",
	})
}

func (a *App) OpenExecutableDialog(title string) (string, error) {
	var filters []dialogs.FileFilter
	if goruntime.GOOS == "windows" {
		filters = []dialogs.FileFilter{
			{
				DisplayName: "Executables",
				Pattern:     "*.exe",
			},
			{
				DisplayName: "All Files",
				Pattern:     "*.*",
			},
		}
	} else {
		filters = []dialogs.FileFilter{
			{
				DisplayName: "All Files",
				Pattern:     "*.*",
			},
		}
	}
	return ui.GetFileDialog().OpenFile(dialogs.OpenFileOptions{
		Title:   title,
		Filters: filters,
	})
}

type MediaLanguageInfo struct {
	HasLanguageTags bool `json:"hasLanguageTags"`
}

func (a *App) CheckMediaLanguageTags(path string) (MediaLanguageInfo, error) {
	info := MediaLanguageInfo{
		HasLanguageTags: false,
	}

	// Check if path is a directory
	fileInfo, err := os.Stat(path)
	if err != nil {
		return info, err
	}

	if fileInfo.IsDir() {
		// Get the first video file in the directory
		videos, err := a.GetVideosInDirectory(path)
		if err != nil {
			return info, err
		}
		if len(videos) == 0 {
			return info, fmt.Errorf("no video files found in directory")
		}
		// Use the first video file for checking
		path = videos[0].Path
	}

	mediaInfo, err := core.Mediainfo(path)
	if err != nil {
		return info, err
	}

	// Check if any audio tracks have language tags
	for _, track := range mediaInfo.AudioTracks {
		if track.Language != nil {
			info.HasLanguageTags = true
			break
		}
	}

	return info, nil
}