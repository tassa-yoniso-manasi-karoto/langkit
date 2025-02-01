package gui

import (
	"os"
	"strings"
	"path/filepath"
	
	"fmt"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"

	//iso "github.com/barbashov/iso639-3"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)

// VideoInfo represents information about a video file
type VideoInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (a *App) OpenDirectoryDialog() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Media Directory",
	})
}

func (a *App) OpenVideoDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Video File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Video Files",
				Pattern:	 "*.mp4;*.mkv;*.avi;*.mov;*.wmv;*.flv;*.webm;*.m4v",
			},
		},
	})
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

type LanguageCheckResponse struct {
	StandardTag string `json:"standardTag"`
	IsValid      bool   `json:"isValid"`
	Error        string `json:"error,omitempty"`
}


func (a *App) ValidateLanguageTag(tagsString string, maxOne bool) LanguageCheckResponse {
	resp := LanguageCheckResponse{
		IsValid: false,
	}
	if tagsString == "" {
		resp.Error = "provided tagsString is empty"
		return resp
	}

	// Split the string on commas and trim spaces
	tags := strings.Split(tagsString, ",")
	for i := range tags {
		tags[i] = strings.TrimSpace(tags[i])
	}
	
	if maxOne && len(tags) > 1 {
		resp.Error = "more than one tag was provided"
		return resp
	}

	// Filter out empty strings
	var nonEmptyTags []string
	for _, tag := range tags {
		if tag != "" {
			nonEmptyTags = append(nonEmptyTags, strings.TrimSpace(tag))
		}
	}

	langs, err := core.ParseLanguageTags(nonEmptyTags)
	if err != nil {
		resp.Error = err.Error()
		return resp
	}
	std := langs[0].Part3
	if langs[0].Subtag != "" {
		std += "-" + langs[0].Subtag
	}

	return LanguageCheckResponse{
		IsValid: true,
		StandardTag: std,
	}
}


// GetRomanizationStyles returns available romanization styles for a given language
func (a *App) GetRomanizationStyles(langCode string) []string {
	// This is where you'll implement the actual logic
	// For now, returning dummy data based on language
	switch langCode {
	case "jpn":
		return []string{"Hepburn", "Kunrei-shiki", "Nihon-shiki"}
	case "kor":
		return []string{"Revised Romanization", "McCune-Reischauer"}
	case "chi":
		return []string{"Pinyin", "Wade-Giles"}
	default:
		return []string{}
	}
}



func placeholder323453367() {
	fmt.Print("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}



