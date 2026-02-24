package services

import (
	"context"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/generated"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/interfaces"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"
)

// Compile-time check that ExpectationService implements api.Service
var _ api.Service = (*ExpectationService)(nil)

// ExpectationService implements the WebRPC ExpectationService interface
type ExpectationService struct {
	logger   zerolog.Logger
	handler  http.Handler
	progress interfaces.ProgressReporter // optional, for GUI progress bars
}

// NewExpectationService creates a new expectation service instance
func NewExpectationService(logger zerolog.Logger, progress interfaces.ProgressReporter) *ExpectationService {
	svc := &ExpectationService{
		logger:   logger,
		progress: progress,
	}
	svc.handler = generated.NewExpectationServiceServer(svc)
	return svc
}

// Name implements api.Service
func (s *ExpectationService) Name() string {
	return "ExpectationService"
}

// Handler implements api.Service
func (s *ExpectationService) Handler() http.Handler {
	return s.handler
}

// Description implements api.Service
func (s *ExpectationService) Description() string {
	return "Media expectation checker service"
}

// RunCheck performs the expectation check and returns a validation report.
func (s *ExpectationService) RunCheck(ctx context.Context, request *generated.CheckRequest) (*generated.ValidationReport, error) {
	s.logger.Info().Str("path", request.Path).Msg("Running expectation check")

	// Convert WebRPC types to core types
	var profile *core.ExpectationProfile
	if request.Profile != nil {
		profile = &core.ExpectationProfile{
			Name:                     request.Profile.Name,
			ExpectedAudioLangs:       request.Profile.ExpectedAudioLangs,
			ExpectedSubtitleLangs:    request.Profile.ExpectedSubtitleLangs,
			RequireVideoTrack:        request.Profile.RequireVideoTrack,
			RequireLanguageTags:      request.Profile.RequireLanguageTags,
			DurationTolerancePct:     request.Profile.DurationTolerancePercent,
			CheckExternalAudio:       request.Profile.CheckExternalAudioFiles,
			VideoExtensions:          request.Profile.VideoExtensions,
		}
	}

	var autoConfig *core.AutoCheckConfig
	if request.AutoConfig != nil {
		autoConfig = &core.AutoCheckConfig{
			Enabled:      request.AutoConfig.Enabled,
			QuorumPct:    request.AutoConfig.QuorumPct,
			SoftFloorPct: request.AutoConfig.SoftFloorPct,
			MinGroupSize: int(request.AutoConfig.MinGroupSize),
		}
	}

	// Build callbacks with logger and optional progress
	cb := core.CheckCallbacks{Logger: s.logger}
	if s.progress != nil {
		pr := s.progress
		cb.OnProgress = func(barID string, increment, total int, label string) {
			pr.IncrementProgress(barID, increment, total, 20,
				"Checking media", label, "")
		}
		defer pr.RemoveProgressBar(progress.BarCheckProbe)
		defer pr.RemoveProgressBar(progress.BarCheckDecode)
	}

	// Empty depth: RunCheck will load from settings
	report, err := core.RunCheck(ctx, request.Path, profile, autoConfig, media.IntegrityDepth(""), cb)
	if err != nil {
		s.logger.Error().Err(err).Msg("Expectation check failed")
		return nil, err
	}

	return convertReport(report), nil
}

// ListProfiles returns all saved expectation profiles.
func (s *ExpectationService) ListProfiles(ctx context.Context) ([]*generated.ExpectationProfile, error) {
	profiles, err := core.LoadProfiles(s.logger)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to load profiles")
		return nil, err
	}

	result := make([]*generated.ExpectationProfile, len(profiles))
	for i, p := range profiles {
		result[i] = convertProfileToGen(p)
	}
	return result, nil
}

// SaveProfile creates or updates an expectation profile.
func (s *ExpectationService) SaveProfile(ctx context.Context, profile *generated.ExpectationProfile) error {
	p := convertProfileFromGen(profile)
	if err := core.SaveProfile(p, s.logger); err != nil {
		s.logger.Error().Err(err).Str("name", profile.Name).Msg("Failed to save profile")
		return err
	}
	s.logger.Info().Str("name", profile.Name).Msg("Profile saved")
	return nil
}

// DeleteProfile removes an expectation profile by name.
func (s *ExpectationService) DeleteProfile(ctx context.Context, name string) error {
	if err := core.DeleteProfile(name, s.logger); err != nil {
		s.logger.Error().Err(err).Str("name", name).Msg("Failed to delete profile")
		return err
	}
	s.logger.Info().Str("name", name).Msg("Profile deleted")
	return nil
}

// convertReport converts a core.ValidationReport to the generated WebRPC type.
func convertReport(report *core.ValidationReport) *generated.ValidationReport {
	gr := &generated.ValidationReport{
		RootPath:     report.RootPath,
		TotalFiles:   int32(report.TotalFiles),
		ErrorCount:   int32(report.ErrorCount),
		WarningCount: int32(report.WarningCount),
		InfoCount:    int32(report.InfoCount),
		DurationMs:   report.Duration.Milliseconds(),
	}

	// Convert issues
	for _, iss := range report.Issues {
		gr.Issues = append(gr.Issues, &generated.ValidationIssue{
			Severity:  iss.Severity.String(),
			Source:    string(iss.Source),
			FilePath:  iss.FilePath,
			Category:  iss.Category,
			IssueCode: iss.Code,
			Message:   iss.Message,
		})
	}

	// Generate interpreted summaries
	coreSummaries := core.GenerateInterpretedSummaries(report)
	for _, s := range coreSummaries {
		gr.InterpretedSummaries = append(gr.InterpretedSummaries, &generated.InterpretedSummary{
			Source:  string(s.Source),
			Message: s.Message,
		})
	}

	// Build file summaries
	fileIssues := make(map[string]struct {
		errors   int32
		warnings int32
		infos    int32
	})
	for _, iss := range report.Issues {
		fi := fileIssues[iss.FilePath]
		switch iss.Severity {
		case core.SeverityError:
			fi.errors++
		case core.SeverityWarning:
			fi.warnings++
		case core.SeverityInfo:
			fi.infos++
		}
		fileIssues[iss.FilePath] = fi
	}

	// Create a sorted list of all file paths from FileResults
	filePaths := make([]string, 0, len(report.FileResults))
	for fp := range report.FileResults {
		filePaths = append(filePaths, fp)
	}
	sort.Strings(filePaths)

	for _, fp := range filePaths {
		fi := fileIssues[fp]
		base := filepath.Base(fp)
		gr.FileSummaries = append(gr.FileSummaries, &generated.FileSummary{
			FilePath:     fp,
			FileName:     base,
			ErrorCount:   fi.errors,
			WarningCount: fi.warnings,
			InfoCount:    fi.infos,
			Passed:       fi.errors == 0,
		})
	}

	// Convert consensus summaries
	if len(report.Consensus) > 0 {
		dirs := make([]string, 0, len(report.Consensus))
		for d := range report.Consensus {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		for _, dir := range dirs {
			dc := report.Consensus[dir]
			gr.ConsensusSummaries = append(gr.ConsensusSummaries, &generated.DirectoryConsensusSummary{
				Directory:                dir,
				FileCount:                int32(dc.FileCount),
				BonusExcluded:            int32(dc.BonusExcluded),
				ConsensusAudioLangs:      dc.QuorumAudioLangs,
				ConsensusSubLangs:        dc.QuorumSubLangs,
				ConsensusAudioTrackCount: int32(dc.ConsensusAudioCount),
				MedianDurationSec:        dc.MedianDuration,
			})
		}
	}

	return gr
}

func convertProfileToGen(p core.ExpectationProfile) *generated.ExpectationProfile {
	return &generated.ExpectationProfile{
		Name:                     p.Name,
		ExpectedAudioLangs:       p.ExpectedAudioLangs,
		ExpectedSubtitleLangs:    p.ExpectedSubtitleLangs,
		RequireVideoTrack:        p.RequireVideoTrack,
		RequireLanguageTags:      p.RequireLanguageTags,
		DurationTolerancePercent: p.DurationTolerancePct,
		CheckExternalAudioFiles:  p.CheckExternalAudio,
		VideoExtensions:          p.VideoExtensions,
	}
}

func convertProfileFromGen(p *generated.ExpectationProfile) core.ExpectationProfile {
	return core.ExpectationProfile{
		Name:                     p.Name,
		ExpectedAudioLangs:       p.ExpectedAudioLangs,
		ExpectedSubtitleLangs:    p.ExpectedSubtitleLangs,
		RequireVideoTrack:        p.RequireVideoTrack,
		RequireLanguageTags:      p.RequireLanguageTags,
		DurationTolerancePct:     p.DurationTolerancePercent,
		CheckExternalAudio:       p.CheckExternalAudioFiles,
		VideoExtensions:          p.VideoExtensions,
	}
}
