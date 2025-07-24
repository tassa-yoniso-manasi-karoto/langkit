package services

import (
	"context"
	"net/http"
	goruntime "runtime"

	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/generated"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/interfaces"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/ui"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/ui/dialogs"
)

// Compile-time check that MediaService implements api.Service
var _ api.Service = (*MediaService)(nil)

// MediaService implements the WebRPC MediaService interface
type MediaService struct {
	logger   zerolog.Logger
	handler  http.Handler
	provider interfaces.MediaProvider
}

// NewMediaService creates a new media service
func NewMediaService(logger zerolog.Logger, provider interfaces.MediaProvider) *MediaService {
	svc := &MediaService{
		logger:   logger,
		provider: provider,
	}
	
	// Create the WebRPC handler
	svc.handler = generated.NewMediaServiceServer(svc)
	
	return svc
}

// Name implements api.Service
func (s *MediaService) Name() string {
	return "MediaService"
}

// Handler implements api.Service
func (s *MediaService) Handler() http.Handler {
	return s.handler
}

// Description implements api.Service
func (s *MediaService) Description() string {
	return "Media file operations and dialog service"
}

// OpenVideoDialog opens a native file dialog to select a video file
func (s *MediaService) OpenVideoDialog(ctx context.Context) (string, error) {
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

// OpenDirectoryDialog opens a native dialog to select a directory
func (s *MediaService) OpenDirectoryDialog(ctx context.Context) (string, error) {
	return ui.GetFileDialog().OpenDirectory(dialogs.OpenDirectoryOptions{
		Title: "Select Media Directory",
	})
}

// OpenExecutableDialog opens a native file dialog to select an executable file
func (s *MediaService) OpenExecutableDialog(ctx context.Context, title string) (string, error) {
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

// GetVideosInDirectory scans a directory and returns information about video files
func (s *MediaService) GetVideosInDirectory(ctx context.Context, dirPath string) ([]*generated.VideoInfo, error) {
	// Call the provider method which returns []interface{}
	videos, err := s.provider.GetVideosInDirectory(dirPath)
	if err != nil {
		return nil, err
	}
	
	// Convert []interface{} to []*generated.VideoInfo
	result := make([]*generated.VideoInfo, 0, len(videos))
	for _, v := range videos {
		// Type assertion to convert from interface{} to VideoInfo
		if videoInfo, ok := v.(generated.VideoInfo); ok {
			result = append(result, &videoInfo)
		} else if videoInfoPtr, ok := v.(*generated.VideoInfo); ok {
			result = append(result, videoInfoPtr)
		} else if videoMap, ok := v.(map[string]string); ok {
			// Convert map to VideoInfo
			result = append(result, &generated.VideoInfo{
				Name: videoMap["name"],
				Path: videoMap["path"],
			})
		} else {
			s.logger.Warn().Interface("video", v).Msg("Invalid video info type in GetVideosInDirectory")
		}
	}
	
	return result, nil
}

// CheckMediaLanguageTags checks if a media file has embedded language tags
func (s *MediaService) CheckMediaLanguageTags(ctx context.Context, path string) (*generated.MediaLanguageInfo, error) {
	// Call the provider method which returns interface{}
	info, err := s.provider.CheckMediaLanguageTags(path)
	if err != nil {
		return nil, err
	}
	
	// Type assertion to convert from interface{} to MediaLanguageInfo
	if mediaInfo, ok := info.(generated.MediaLanguageInfo); ok {
		return &mediaInfo, nil
	} else if mediaInfoPtr, ok := info.(*generated.MediaLanguageInfo); ok {
		return mediaInfoPtr, nil
	} else if infoMap, ok := info.(map[string]bool); ok {
		// Convert map to MediaLanguageInfo
		return &generated.MediaLanguageInfo{
			HasLanguageTags: infoMap["hasLanguageTags"],
		}, nil
	}
	
	// If type assertion fails, log and return default
	s.logger.Warn().Interface("info", info).Msg("Invalid media language info type")
	return &generated.MediaLanguageInfo{HasLanguageTags: false}, nil
}