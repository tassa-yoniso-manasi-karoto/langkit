package services

import (
	"context"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/generated"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/changelog"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/version"
)

// Compile-time check that ChangelogService implements api.Service
var _ api.Service = (*ChangelogService)(nil)

// ChangelogService implements the WebRPC ChangelogService interface
type ChangelogService struct {
	logger  zerolog.Logger
	handler http.Handler
}

// NewChangelogService creates a new changelog service instance
func NewChangelogService(logger zerolog.Logger) *ChangelogService {
	svc := &ChangelogService{
		logger: logger,
	}

	// Create the WebRPC handler
	svc.handler = generated.NewChangelogServiceServer(svc)

	return svc
}

// Name implements api.Service
func (s *ChangelogService) Name() string {
	return "ChangelogService"
}

// Handler implements api.Service
func (s *ChangelogService) Handler() http.Handler {
	return s.handler
}

// Description implements api.Service
func (s *ChangelogService) Description() string {
	return "Changelog and version tracking service"
}

// GetChangelog returns changelog entries, optionally filtered by sinceVersion
func (s *ChangelogService) GetChangelog(ctx context.Context, sinceVersion *string) (*generated.ChangelogResponse, error) {
	entries, err := changelog.Parse()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse changelog")
		return nil, err
	}

	// Filter entries if sinceVersion is provided
	if sinceVersion != nil && *sinceVersion != "" {
		entries = changelog.GetEntriesSinceVersion(entries, *sinceVersion)
	}

	// Convert to generated types
	genEntries := make([]*generated.ChangelogEntry, len(entries))
	for i, entry := range entries {
		sections := make([]*generated.ChangelogSection, len(entry.Sections))
		for j, section := range entry.Sections {
			sections[j] = &generated.ChangelogSection{
				Title: section.Title,
				Items: section.Items,
			}
		}
		genEntries[i] = &generated.ChangelogEntry{
			Version:  entry.Version,
			Date:     entry.Date,
			Sections: sections,
		}
	}

	return &generated.ChangelogResponse{
		Entries:        genEntries,
		CurrentVersion: version.Version,
	}, nil
}

// CheckUpgrade checks if user has upgraded and should see changelog
func (s *ChangelogService) CheckUpgrade(ctx context.Context) (*generated.UpgradeInfo, error) {
	settings, err := config.LoadSettings()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to load settings for upgrade check")
		return nil, err
	}

	currentVersion := version.Version
	lastSeenVersion := settings.LastSeenVersion
	frequency := settings.ChangelogDisplayFrequency

	if frequency == "" {
		frequency = "medium_major"
	}

	upgradeType := changelog.DetermineUpgradeType(lastSeenVersion, currentVersion)
	shouldShow := changelog.ShouldShowChangelog(upgradeType, frequency)

	s.logger.Debug().
		Str("lastSeenVersion", lastSeenVersion).
		Str("currentVersion", currentVersion).
		Str("upgradeType", string(upgradeType)).
		Str("frequency", frequency).
		Bool("shouldShow", shouldShow).
		Msg("Upgrade check completed")

	return &generated.UpgradeInfo{
		HasUpgrade:        upgradeType != changelog.UpgradeNone,
		PreviousVersion:   lastSeenVersion,
		CurrentVersion:    currentVersion,
		UpgradeType:       string(upgradeType),
		ShouldShowChangelog: shouldShow,
	}, nil
}

// MarkVersionSeen updates lastSeenVersion to current version
func (s *ChangelogService) MarkVersionSeen(ctx context.Context) error {
	// Never save "dev" as last seen version
	if version.Version == "dev" {
		s.logger.Debug().Msg("Skipping marking 'dev' version as seen")
		return nil
	}

	settings, err := config.LoadSettings()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to load settings for marking version seen")
		return err
	}

	settings.LastSeenVersion = version.Version

	if err := config.SaveSettings(settings); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save settings after marking version seen")
		return err
	}

	s.logger.Info().Str("version", version.Version).Msg("Marked version as seen")
	return nil
}
