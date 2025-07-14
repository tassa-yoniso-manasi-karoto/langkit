package services

import (
	"context"
	"net/http"

	"github.com/rs/zerolog"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/generated"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)

// Compile-time check that DryRunService implements api.Service
var _ api.Service = (*DryRunService)(nil)

// DryRunService implements the WebRPC DryRunService interface
type DryRunService struct {
	logger   zerolog.Logger
	handler  http.Handler
	provider DryRunProvider
}

// NewDryRunService creates a new dry run service
func NewDryRunService(logger zerolog.Logger, provider DryRunProvider) *DryRunService {
	svc := &DryRunService{
		logger:   logger,
		provider: provider,
	}
	
	// Create the WebRPC handler
	svc.handler = generated.NewDryRunServiceServer(svc)
	
	return svc
}

// Name implements api.Service
func (s *DryRunService) Name() string {
	return "DryRunService"
}

// Handler implements api.Service
func (s *DryRunService) Handler() http.Handler {
	return s.handler
}

// Description implements api.Service
func (s *DryRunService) Description() string {
	return "Testing and debugging service for dry run mode"
}

// SetConfig implements generated.DryRunService
func (s *DryRunService) SetConfig(ctx context.Context, config *generated.DryRunConfig) error {
	s.logger.Debug().
		Bool("enabled", config.Enabled).
		Int32("delayMs", config.DelayMs).
		Msg("Setting dry run config")
	
	// Convert to core.DryRunConfig
	dryRunConfig := &core.DryRunConfig{
		Enabled:        config.Enabled,
		DelayMs:        int(config.DelayMs),
		ProcessedCount: int(config.ProcessedCount),
		NextErrorIndex: int(config.NextErrorIndex),
	}
	
	if config.NextErrorType != nil {
		dryRunConfig.NextErrorType = *config.NextErrorType
	}
	
	// Convert error points map
	if config.ErrorPoints != nil {
		dryRunConfig.ErrorPoints = make(map[int]string)
		for k, v := range config.ErrorPoints {
			dryRunConfig.ErrorPoints[int(k)] = v
		}
	}
	
	s.provider.SetDryRunConfig(dryRunConfig)
	return nil
}

// InjectError implements generated.DryRunService
func (s *DryRunService) InjectError(ctx context.Context, errorType string) error {
	s.logger.Debug().
		Str("errorType", errorType).
		Msg("Injecting dry run error")
	
	return s.provider.InjectDryRunError(errorType)
}

// GetStatus implements generated.DryRunService
func (s *DryRunService) GetStatus(ctx context.Context) (*generated.DryRunStatus, error) {
	s.logger.Debug().Msg("Getting dry run status")
	
	status := s.provider.GetDryRunStatus()
	
	// Convert to generated.DryRunStatus
	result := &generated.DryRunStatus{
		Enabled:        getBoolFromMap(status, "enabled", false),
		DelayMs:        int32(getIntFromMap(status, "delayMs", 1000)),
		ProcessedCount: int32(getIntFromMap(status, "processedCount", 0)),
		NextErrorIndex: int32(getIntFromMap(status, "nextErrorIndex", -1)),
	}
	
	if nextErrorType, ok := status["nextErrorType"].(string); ok && nextErrorType != "" {
		result.NextErrorType = &nextErrorType
	}
	
	// Convert error points
	if errorPoints, ok := status["errorPoints"].(map[int]string); ok {
		result.ErrorPoints = make(map[int32]string)
		for k, v := range errorPoints {
			result.ErrorPoints[int32(k)] = v
		}
	}
	
	return result, nil
}

// Helper functions for map conversion
func getBoolFromMap(m map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := m[key].(bool); ok {
		return val
	}
	return defaultValue
}

func getIntFromMap(m map[string]interface{}, key string, defaultValue int) int {
	if val, ok := m[key].(int); ok {
		return val
	}
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}