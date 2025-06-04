package gui

import (
	"os"
	"path/filepath"
	"strings"
	"fmt"
	"time"
	
	"github.com/gookit/color"
	"github.com/k0kubun/pp"

	//iso "github.com/barbashov/iso639-3"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/tassa-yoniso-manasi-karoto/dockerutil"
	
	"github.com/tassa-yoniso-manasi-karoto/translitkit"
	"github.com/tassa-yoniso-manasi-karoto/translitkit/common"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/version"
)

// GetCurrentTimestamp returns the current timestamp in milliseconds since Unix epoch,
// using the same format as log timestamps in pkg batch/throttler.go
func (a *App) GetCurrentTimestamp() int64 {
    return time.Now().UnixNano() / int64(time.Millisecond)
}

func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

func (a *App) GetVersion() version.Info {
	return version.GetInfo()
}


func (a *App) NeedsTokenization(language string) bool {
	b, _ := translitkit.NeedsTokenization(language)
	return b
}


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
				Pattern:     "*.mp4;*.mkv;*.avi;*.mov;*.wmv;*.flv;*.webm;*.m4v",
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


// type AudioTrack struct {
//     Index    int     `json:"index"`
//     Language string `json:"language"`
// }

type MediaLanguageInfo struct {
    HasLanguageTags bool         `json:"hasLanguageTags"`
//     AudioTracks     []AudioTrack `json:"audioTracks"`
}

func (a *App) CheckMediaLanguageTags(path string) (MediaLanguageInfo, error) {
    info := MediaLanguageInfo{
        HasLanguageTags: false,
        // AudioTracks:     []AudioTrack{},
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

    // Now check the media info
    mediaInfo := core.Mediainfo(path)
    
    // Check if any audio tracks have language tags
    for _, track := range mediaInfo.AudioTracks {
        // info.AudioTracks = append(info.AudioTracks, AudioTrack{
        //     Index:    i,
        //     Language: track.Language.Part3,
        // })
        if track.Language != nil {
            info.HasLanguageTags = true
        }
    }

    return info, nil
}


type LanguageCheckResponse struct {
	StandardTag string `json:"standardTag"`
	IsValid     bool   `json:"isValid"`
	Error       string `json:"error,omitempty"`
}

func (a *App) ValidateLanguageTag(tagsString string, maxOne bool) LanguageCheckResponse {
	resp := LanguageCheckResponse{
		IsValid: false,
	}
	if tagsString == "" {
		resp.Error = "provided tagsString is empty"
		return resp
	}

	tags := core.TagsStr2TagsArr(tagsString)

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

	if len(nonEmptyTags) == 0 {
		resp.Error = "no valid tags provided"
		return resp
	}

	// For single tag validation, we can use the internal helper
	if len(nonEmptyTags) == 1 {
		std, isValid, err := validateLanguageTag(nonEmptyTags[0])
		if err != nil {
			resp.Error = err.Error()
			return resp
		}
		return LanguageCheckResponse{
			IsValid:     isValid,
			StandardTag: std,
		}
	}

	// For multiple tags, use the original logic
	langs, err := core.ParseLanguageTags(nonEmptyTags)
	if err != nil {
		resp.Error = err.Error()
		return resp
	}
	
	if len(langs) == 0 {
		resp.Error = "no valid language tags found"
		return resp
	}
	
	std := langs[0].Part3
	if langs[0].Subtag != "" {
		std += "-" + langs[0].Subtag
	}

	return LanguageCheckResponse{
		IsValid:     true,
		StandardTag: std,
	}
}

type RomanizationScheme struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Provider    string `json:"provider"`
}

type RomanizationStylesResponse struct {
	Schemes           []RomanizationScheme `json:"schemes"`
	DockerUnreachable bool                 `json:"dockerUnreachable"`
	DockerEngine      string               `json:"dockerEngine"`
	NeedsDocker       bool                 `json:"needsDocker"`
	NeedsScraper      bool                 `json:"needsScraper"`
}

func (a *App) GetRomanizationStyles(languageCode string) (RomanizationStylesResponse, error) {
	resp := RomanizationStylesResponse{DockerEngine: dockerutil.DockerBackendName()}

	// Get available schemes for the language
	schemes, err := common.GetSchemes(languageCode)
	if err != nil {
		if err == common.ErrNoSchemesRegistered {
			handler.ZeroLog().Warn().Msgf("%v \"%s\"", err, languageCode)
		} else {
			handler.ZeroLog().Error().
				Err(err).
				Str("lang", languageCode).
				Msg("Failed to get romanization schemes")
		}
		return resp, err
	}
	for _, scheme := range schemes {
		if scheme.NeedsDocker {
			resp.NeedsDocker = true
			break
		}
	}
	for _, scheme := range schemes {
		if scheme.NeedsScraper {
			resp.NeedsScraper = true
			break
		}
	}

	if resp.NeedsDocker {
		if err := dockerutil.EngineIsReachable(); err != nil {
			handler.ZeroLog().Warn().
				Err(err).
				Str("lang", languageCode).
				Msg("Docker is required but not available")

			resp.DockerUnreachable = true
		}
	}

	// Convert schemes to resp format
	resp.Schemes = make([]RomanizationScheme, len(schemes))
	for i, scheme := range schemes {
		resp.Schemes[i] = RomanizationScheme{
			Name:        scheme.Name,
			Description: scheme.Description,
			Provider:    scheme.Provider,
		}
	}
	return resp, nil
}

func placeholder323453367() {
	fmt.Print("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
