package services

import (
	"context"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/generated"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/interfaces"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
)

// Compile-time check that SettingsService implements api.Service
var _ api.Service = (*SettingsService)(nil)

// SettingsService implements the WebRPC SettingsService interface
type SettingsService struct {
	logger   zerolog.Logger
	provider interfaces.SettingsProvider
	handler  http.Handler
}

// NewSettingsService creates a new settings service instance
func NewSettingsService(logger zerolog.Logger, provider interfaces.SettingsProvider) *SettingsService {
	svc := &SettingsService{
		logger:   logger,
		provider: provider,
	}
	
	// Create the WebRPC handler
	svc.handler = generated.NewSettingsServiceServer(svc)
	
	return svc
}

// Name implements api.Service
func (s *SettingsService) Name() string {
	return "SettingsService"
}

// Handler implements api.Service
func (s *SettingsService) Handler() http.Handler {
	return s.handler
}

// Description implements api.Service
func (s *SettingsService) Description() string {
	return "Settings and configuration management"
}

// InitSettings initializes the configuration file if it doesn't exist
func (s *SettingsService) InitSettings(ctx context.Context) error {
	return config.InitConfig("")
}

// LoadSettings loads the application settings from disk
func (s *SettingsService) LoadSettings(ctx context.Context) (*generated.Settings, error) {
	settings, err := config.LoadSettings()
	if err != nil {
		return nil, err
	}
	
	// Convert config.Settings to generated.Settings
	genSettings := &generated.Settings{
		ApiKeys: &generated.APIKeys{
			Replicate:  &settings.APIKeys.Replicate,
			ElevenLabs: &settings.APIKeys.ElevenLabs,
			OpenAI:     &settings.APIKeys.OpenAI,
			OpenRouter: &settings.APIKeys.OpenRouter,
			Google:     &settings.APIKeys.Google,
		},
		TargetLanguage:                   settings.TargetLanguage,
		NativeLanguages:                  settings.NativeLanguages,
		LiteMode:                         settings.LiteMode,
		ShowLogViewerByDefault:           settings.ShowLogViewerByDefault,
		MaxLogEntries:                    int32(settings.MaxLogEntries),
		MaxAPIRetries:                    int32(settings.MaxAPIRetries),
		MaxWorkers:                       int32(settings.MaxWorkers),
		TimeoutSep:                       int32(settings.TimeoutSep),
		TimeoutSTT:                       int32(settings.TimeoutSTT),
		TimeoutDL:                        int32(settings.TimeoutDL),
		LogViewerVirtualizationThreshold: int32(settings.LogViewerVirtualizationThreshold),
		EventThrottling: &generated.EventThrottling{
			Enabled:     settings.EventThrottling.Enabled,
			MinInterval: int32(settings.EventThrottling.MinInterval),
			MaxInterval: int32(settings.EventThrottling.MaxInterval),
		},
		IntermediaryFileMode:  string(settings.IntermediaryFileMode),
		DeleteResumptionFiles: settings.DeleteResumptionFiles,
		UseWasm:               settings.UseWasm,
		WasmSizeThreshold:     int32(settings.WasmSizeThreshold),
		ForceWasmMode:         settings.ForceWasmMode,
		FfmpegPath:            settings.FFmpegPath,
		MediainfoPath:         settings.MediaInfoPath,
		CustomEndpoints: &generated.CustomEndpoints{
			Stt: &generated.CustomEndpointConfig{
				Enabled:  settings.CustomEndpoints.STT.Enabled,
				Endpoint: settings.CustomEndpoints.STT.Endpoint,
				Model:    settings.CustomEndpoints.STT.Model,
			},
			Llm: &generated.CustomEndpointConfig{
				Enabled:  settings.CustomEndpoints.LLM.Enabled,
				Endpoint: settings.CustomEndpoints.LLM.Endpoint,
				Model:    settings.CustomEndpoints.LLM.Model,
			},
		},
		LastSeenVersion:           settings.LastSeenVersion,
		ChangelogDisplayFrequency: settings.ChangelogDisplayFrequency,
	}

	return genSettings, nil
}

// SaveSettings saves the application settings to disk and applies relevant changes
func (s *SettingsService) SaveSettings(ctx context.Context, genSettings *generated.Settings) error {
	s.logger.Debug().Msg("Saving settings via WebRPC")
	
	// Convert generated.Settings to config.Settings
	settings := config.Settings{
		TargetLanguage:                   genSettings.TargetLanguage,
		NativeLanguages:                  genSettings.NativeLanguages,
		LiteMode:                         genSettings.LiteMode,
		ShowLogViewerByDefault:           genSettings.ShowLogViewerByDefault,
		MaxLogEntries:                    int(genSettings.MaxLogEntries),
		MaxAPIRetries:                    int(genSettings.MaxAPIRetries),
		MaxWorkers:                       int(genSettings.MaxWorkers),
		TimeoutSep:                       int(genSettings.TimeoutSep),
		TimeoutSTT:                       int(genSettings.TimeoutSTT),
		TimeoutDL:                        int(genSettings.TimeoutDL),
		LogViewerVirtualizationThreshold: int(genSettings.LogViewerVirtualizationThreshold),
		IntermediaryFileMode:             config.IntermediaryFileMode(genSettings.IntermediaryFileMode),
		DeleteResumptionFiles:            genSettings.DeleteResumptionFiles,
		UseWasm:                          genSettings.UseWasm,
		WasmSizeThreshold:                int(genSettings.WasmSizeThreshold),
		ForceWasmMode:                    genSettings.ForceWasmMode,
		FFmpegPath:                       genSettings.FfmpegPath,
		MediaInfoPath:                    genSettings.MediainfoPath,
		LastSeenVersion:                  genSettings.LastSeenVersion,
		ChangelogDisplayFrequency:        genSettings.ChangelogDisplayFrequency,
	}

	// Handle API keys if provided
	if genSettings.ApiKeys != nil {
		if genSettings.ApiKeys.Replicate != nil {
			settings.APIKeys.Replicate = *genSettings.ApiKeys.Replicate
		}
		if genSettings.ApiKeys.ElevenLabs != nil {
			settings.APIKeys.ElevenLabs = *genSettings.ApiKeys.ElevenLabs
		}
		if genSettings.ApiKeys.OpenAI != nil {
			settings.APIKeys.OpenAI = *genSettings.ApiKeys.OpenAI
		}
		if genSettings.ApiKeys.OpenRouter != nil {
			settings.APIKeys.OpenRouter = *genSettings.ApiKeys.OpenRouter
		}
		if genSettings.ApiKeys.Google != nil {
			settings.APIKeys.Google = *genSettings.ApiKeys.Google
		}
	}

	// Handle event throttling
	if genSettings.EventThrottling != nil {
		settings.EventThrottling.Enabled = genSettings.EventThrottling.Enabled
		settings.EventThrottling.MinInterval = int(genSettings.EventThrottling.MinInterval)
		settings.EventThrottling.MaxInterval = int(genSettings.EventThrottling.MaxInterval)
	}

	// Handle custom endpoints
	if genSettings.CustomEndpoints != nil {
		if genSettings.CustomEndpoints.Stt != nil {
			settings.CustomEndpoints.STT.Enabled = genSettings.CustomEndpoints.Stt.Enabled
			settings.CustomEndpoints.STT.Endpoint = genSettings.CustomEndpoints.Stt.Endpoint
			settings.CustomEndpoints.STT.Model = genSettings.CustomEndpoints.Stt.Model
		}
		if genSettings.CustomEndpoints.Llm != nil {
			settings.CustomEndpoints.LLM.Enabled = genSettings.CustomEndpoints.Llm.Enabled
			settings.CustomEndpoints.LLM.Endpoint = genSettings.CustomEndpoints.Llm.Endpoint
			settings.CustomEndpoints.LLM.Model = genSettings.CustomEndpoints.Llm.Model
		}
	}
	
	// Save the settings to disk
	err := config.SaveSettings(settings)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to save settings")
		return err
	}
	
	// Apply side effects through the provider
	if s.provider != nil {
		// Update throttler settings
		s.provider.UpdateThrottlerSettings(settings)
		
		// Trigger registry update with new settings
		s.logger.Info().Msg("Triggering LLM registry update with new settings")
		s.provider.TriggerLLMRegistryUpdate(settings)
	}
	
	return nil
}

// LoadStatistics loads usage statistics from disk
func (s *SettingsService) LoadStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats, err := config.LoadStatistics()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to load statistics")
		return nil, err
	}
	
	return stats.GetAll(), nil
}

// UpdateStatistics performs a partial update of the usage statistics
func (s *SettingsService) UpdateStatistics(ctx context.Context, updates map[string]interface{}) error {
	stats, err := config.LoadStatistics()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to load statistics for update")
		return err
	}
	
	// Apply the updates
	stats.Update(updates)
	
	// Save back to disk
	if err := stats.Save(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save statistics")
		return err
	}
	
	s.logger.Debug().Interface("updates", updates).Msg("Statistics updated")
	return nil
}

// IncrementStatistic increments a specific statistic counter and saves the new value
func (s *SettingsService) IncrementStatistic(ctx context.Context, key string) (*generated.IncrementResult, error) {
	stats, err := config.LoadStatistics()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to load statistics for increment")
		return nil, err
	}
	
	// Increment the counter
	newValue := stats.IncrementCounter(key)
	
	// Save back to disk
	if err := stats.Save(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save statistics after increment")
		return nil, err
	}
	
	s.logger.Debug().Str("key", key).Int("newValue", newValue).Msg("Statistic incremented")
	
	return &generated.IncrementResult{
		NewValue: int32(newValue),
	}, nil
}