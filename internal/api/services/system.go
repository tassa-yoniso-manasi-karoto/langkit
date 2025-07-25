package services

import (
	"context"
	"net/http"
	goruntime "runtime"

	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/generated"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/ui"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/version"
)

// Compile-time check that SystemService implements api.Service
var _ api.Service = (*SystemService)(nil)

// SystemService implements the WebRPC SystemService interface
type SystemService struct {
	logger  zerolog.Logger
	handler http.Handler
}

// NewSystemService creates a new system service instance
func NewSystemService(logger zerolog.Logger) *SystemService {
	svc := &SystemService{
		logger: logger,
	}
	
	// Create the WebRPC handler
	svc.handler = generated.NewSystemServiceServer(svc)
	
	return svc
}

// Name implements api.Service
func (s *SystemService) Name() string {
	return "SystemService"
}

// Handler implements api.Service
func (s *SystemService) Handler() http.Handler {
	return s.handler
}

// Description implements api.Service
func (s *SystemService) Description() string {
	return "System information and version service"
}

// GetSystemInfo returns the user's operating system and architecture
func (s *SystemService) GetSystemInfo(ctx context.Context) (*generated.SystemInfo, error) {
	return &generated.SystemInfo{
		Os:   goruntime.GOOS,
		Arch: goruntime.GOARCH,
	}, nil
}

// GetVersion returns version information
func (s *SystemService) GetVersion(ctx context.Context) (*generated.VersionInfo, error) {
	return &generated.VersionInfo{
		Version: version.Version,
	}, nil
}

// CheckForUpdate checks if a newer version is available
func (s *SystemService) CheckForUpdate(ctx context.Context) (bool, error) {
	info := version.GetInfo(true)  // Wait for update check
	return info.NewerVersionAvailable, nil
}

// OpenURL opens a URL in the user's default browser
func (s *SystemService) OpenURL(ctx context.Context, url string) error {
	return ui.GetURLOpener().OpenURL(url)
}