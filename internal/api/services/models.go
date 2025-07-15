package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/generated"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/interfaces"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/summary"
)

// Compile-time check that ModelService implements api.Service
var _ api.Service = (*ModelService)(nil)

// ModelService implements the WebRPC ModelService interface
type ModelService struct {
	logger       zerolog.Logger
	handler      http.Handler
	sttProvider  interfaces.STTModelProvider
	llmProvider  interfaces.LLMRegistryProvider
}

// NewModelService creates a new model service instance
func NewModelService(
	logger zerolog.Logger,
	sttProvider interfaces.STTModelProvider,
	llmProvider interfaces.LLMRegistryProvider,
) *ModelService {
	svc := &ModelService{
		logger:      logger,
		sttProvider: sttProvider,
		llmProvider: llmProvider,
	}
	
	// Create the WebRPC handler
	svc.handler = generated.NewModelServiceServer(svc)
	
	return svc
}

// Name implements api.Service
func (s *ModelService) Name() string {
	return "ModelService"
}

// Handler implements api.Service
func (s *ModelService) Handler() http.Handler {
	return s.handler
}

// Description implements api.Service
func (s *ModelService) Description() string {
	return "AI model service for STT and LLM providers"
}

// GetAvailableSTTModelsForUI returns ALL STT models for the UI
func (s *ModelService) GetAvailableSTTModelsForUI(ctx context.Context) (*generated.STTModelsResponse, error) {
	// Get models from provider
	modelsInterface := s.sttProvider.GetAllSTTModels()
	
	// Type assert to actual voice.STTModelInfo slice
	models, ok := modelsInterface.([]voice.STTModelInfo)
	if !ok {
		s.logger.Error().Msg("Failed to type assert STT models")
		return &generated.STTModelsResponse{
			Models:    []*generated.STTModelUIInfo{},
			Names:     []string{},
			Available: false,
			Suggested: "",
		}, nil
	}
	
	response := &generated.STTModelsResponse{
		Models:    []*generated.STTModelUIInfo{},
		Names:     []string{},
		Available: false,
		Suggested: "",
	}
	
	// Count available models
	availableCount := 0
	
	for _, model := range models {
		modelInfo := generated.STTModelUIInfo{
			Name:               model.Name,
			DisplayName:        model.DisplayName,
			Description:        model.Description,
			ProviderName:       model.ProviderName,
			IsDepreciated:      model.IsDepreciated,
			IsRecommended:      model.IsRecommended,
			TakesInitialPrompt: model.TakesInitialPrompt,
			IsAvailable:        model.IsAvailable,
		}
		
		response.Models = append(response.Models, &modelInfo)
		response.Names = append(response.Names, model.Name)
		
		if model.IsAvailable {
			availableCount++
		}
	}
	
	// Set available flag based on at least one model being available
	response.Available = availableCount > 0
	
	// Always set the first model as the suggested one (if any models exist)
	if len(response.Names) > 0 {
		response.Suggested = response.Names[0]
	}
	
	return response, nil
}

// RefreshSTTModelsAfterSettingsUpdate explicitly refreshes the STT models
func (s *ModelService) RefreshSTTModelsAfterSettingsUpdate(ctx context.Context) (*generated.STTModelsResponse, error) {
	// Force reload of API keys
	settings, err := config.LoadSettings()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to load settings for API key refresh")
	} else {
		// Explicitly load API keys
		settings.LoadKeys()
		s.logger.Info().Msg("API keys reloaded for STT model refresh")
	}
	
	// Clear any provider caches
	s.sttProvider.UpdateSTTFactory()
	
	// Now get the updated models with fresh API keys
	return s.GetAvailableSTTModelsForUI(ctx)
}

// GetAvailableSummaryProviders returns a list of available LLM providers for summarization
func (s *ModelService) GetAvailableSummaryProviders(ctx context.Context) (*generated.SummaryProvidersResponse, error) {
	s.logger.Debug().Msg("Fetching available summary providers")
	
	// Get LLM registry
	registryInterface := s.llmProvider.GetLLMRegistry()
	if registryInterface == nil {
		s.logger.Warn().Msg("LLM registry not initialized")
		return &generated.SummaryProvidersResponse{
			Providers: []*generated.ProviderInfo{},
			Names:     []string{},
			Available: false,
			Suggested: "",
			Status:    "registry_not_initialized",
			Message:   strPtr("LLM registry not initialized yet"),
		}, nil
	}
	
	// Type assert to actual registry
	llmRegistry, ok := registryInterface.(*llms.Registry)
	if !ok {
		s.logger.Error().Msg("Failed to type assert LLM registry")
		return &generated.SummaryProvidersResponse{
			Providers: []*generated.ProviderInfo{},
			Names:     []string{},
			Available: false,
			Suggested: "",
			Status:    "error",
			Message:   strPtr("Failed to access LLM registry"),
		}, nil
	}
	
	stateSnapshot := llmRegistry.GetCurrentStateSnapshot()
	
	// If registry is not ready, return appropriate status
	if stateSnapshot.GlobalState != llms.GSReady {
		s.logger.Info().
			Str("global_state", stateSnapshot.GlobalState.String()).
			Msg("LLM registry not ready yet")
		
		return &generated.SummaryProvidersResponse{
			Providers: []*generated.ProviderInfo{},
			Names:     []string{},
			Available: false,
			Suggested: "",
			Status:    stateSnapshot.GlobalState.String(),
			Message:   strPtr("LLM providers are still initializing"),
		}, nil
	}
	
	// Get the summary service
	summaryServiceInterface := s.llmProvider.GetSummaryService()
	if summaryServiceInterface == nil {
		s.logger.Error().Msg("Summary service not initialized")
		return &generated.SummaryProvidersResponse{
			Providers: []*generated.ProviderInfo{},
			Names:     []string{},
			Available: false,
			Suggested: "",
			Status:    "summary_service_not_initialized",
			Message:   strPtr("Summary service not initialized yet"),
		}, nil
	}
	
	// Type assert to actual summary service
	summaryService, ok := summaryServiceInterface.(*summary.Service)
	if !ok {
		s.logger.Error().Msg("Failed to type assert summary service")
		return &generated.SummaryProvidersResponse{
			Providers: []*generated.ProviderInfo{},
			Names:     []string{},
			Available: false,
			Suggested: "",
			Status:    "error",
			Message:   strPtr("Failed to access summary service"),
		}, nil
	}
	
	// Get the list of providers
	providers := summaryService.ListProviders()
	
	// Create the response structure
	response := &generated.SummaryProvidersResponse{
		Providers: []*generated.ProviderInfo{},
		Names:     []string{},
		Available: len(providers) > 0,
		Suggested: "",
		Status:    "ready",
	}
	
	// Add provider details
	for _, provider := range providers {
		providerName := provider.GetName()
		response.Names = append(response.Names, providerName)
		
		providerInfo := generated.ProviderInfo{
			Name:        providerName,
			DisplayName: displayNameForProvider(providerName),
			Description: descriptionForProvider(providerName),
		}
		
		// Add status information from provider states if available
		if providerState, exists := stateSnapshot.ProviderStatesSnapshot[providerName]; exists {
			providerInfo.Status = strPtr(providerState.Status)
			if providerState.Status == "error" && providerState.Error != "" {
				providerInfo.Error = strPtr(providerState.Error)
			}
		}
		
		response.Providers = append(response.Providers, &providerInfo)
	}
	
	// Set suggested provider - prioritize openrouter-free only
	for _, name := range response.Names {
		if name == "openrouter-free" {
			response.Suggested = "openrouter-free"
			break
		}
	}
	
	// If no openrouter-free and there's at least one available, use the first one
	if response.Suggested == "" && len(response.Names) > 0 {
		response.Suggested = response.Names[0]
	}
	
	return response, nil
}

// GetAvailableSummaryModels returns a list of available models for a specified provider
func (s *ModelService) GetAvailableSummaryModels(ctx context.Context, providerName string) (*generated.SummaryModelsResponse, error) {
	s.logger.Debug().Str("provider", providerName).Msg("Fetching available summary models")
	
	// First check LLM registry state
	registryInterface := s.llmProvider.GetLLMRegistry()
	if registryInterface != nil {
		llmRegistry, ok := registryInterface.(*llms.Registry)
		if ok {
			stateSnapshot := llmRegistry.GetCurrentStateSnapshot()
			
			// If registry is not ready, return appropriate status
			if stateSnapshot.GlobalState != llms.GSReady {
				return &generated.SummaryModelsResponse{
					Models:    []*generated.ModelInfo{},
					Names:     []string{},
					Available: false,
					Suggested: "",
					Status:    stateSnapshot.GlobalState.String(),
					Message:   strPtr("LLM providers are still initializing"),
				}, nil
			}
			
			// If this specific provider is in error state, return that info
			if providerState, exists := stateSnapshot.ProviderStatesSnapshot[providerName]; exists {
				if providerState.Status == "error" {
					errMsg := "Provider initialization failed"
					if providerState.Error != "" {
						errMsg = providerState.Error
					}
					
					return &generated.SummaryModelsResponse{
						Models:    []*generated.ModelInfo{},
						Names:     []string{},
						Available: false,
						Suggested: "",
						Status:    "error",
						Message:   strPtr(errMsg),
					}, nil
				}
			}
		}
	}
	
	// Get the summary service
	summaryServiceInterface := s.llmProvider.GetSummaryService()
	if summaryServiceInterface == nil {
		s.logger.Error().Msg("Summary service not initialized")
		return &generated.SummaryModelsResponse{
			Models:    []*generated.ModelInfo{},
			Names:     []string{},
			Available: false,
			Suggested: "",
			Status:    "error",
			Message:   strPtr("Summary service not initialized"),
		}, nil
	}
	
	// Type assert to actual summary service
	summaryService, ok := summaryServiceInterface.(*summary.Service)
	if !ok {
		s.logger.Error().Msg("Failed to type assert summary service")
		return &generated.SummaryModelsResponse{
			Models:    []*generated.ModelInfo{},
			Names:     []string{},
			Available: false,
			Suggested: "",
			Status:    "error",
			Message:   strPtr("Failed to access summary service"),
		}, nil
	}
	
	// Get models for the specified provider
	models, err := summaryService.GetModelsForProvider(providerName)
	if err != nil {
		s.logger.Error().Err(err).Str("provider", providerName).Msg("Failed to get models for provider")
		return &generated.SummaryModelsResponse{
			Models:    []*generated.ModelInfo{},
			Names:     []string{},
			Available: false,
			Suggested: "",
			Status:    "error",
			Message:   strPtr(fmt.Sprintf("Failed to get models: %v", err)),
		}, nil
	}
	
	// Create the response structure
	response := &generated.SummaryModelsResponse{
		Models:    []*generated.ModelInfo{},
		Names:     []string{},
		Available: len(models) > 0,
		Suggested: "",
		Status:    "ready",
	}
	
	// Add model details
	for _, model := range models {
		response.Names = append(response.Names, model.ID)
		
		modelInfo := generated.ModelInfo{
			Id:           model.ID,
			Name:         model.Name,
			Description:  model.Description,
			ProviderName: model.ProviderName,
		}
		response.Models = append(response.Models, &modelInfo)
		
		// Look for GPT-4o or Claude models to set as suggested
		if response.Suggested == "" {
			if strings.Contains(strings.ToLower(model.ID), "gpt-4o") ||
				strings.Contains(strings.ToLower(model.ID), "claude-3") {
				response.Suggested = model.ID
			}
		}
	}
	
	// If no suggested model yet and there's at least one available, use the first one
	if response.Suggested == "" && len(response.Names) > 0 {
		response.Suggested = response.Names[0]
	}
	
	return response, nil
}

// Helper function to provide friendly display names for providers
func displayNameForProvider(providerName string) string {
	switch providerName {
	case "openai":
		return "OpenAI"
	case "openrouter":
		return "OpenRouter"
	case "google":
		return "Google AI"
	default:
		// Capitalize first letter and return
		if len(providerName) > 0 {
			return strings.ToUpper(providerName[:1]) + providerName[1:]
		}
		return providerName
	}
}

// Helper function to provide descriptions for providers
func descriptionForProvider(providerName string) string {
	switch providerName {
	case "openai":
		return "OpenAI's GPT models including GPT-4o"
	case "openrouter":
		return "Access to multiple LLM providers through OpenRouter"
	case "google":
		return "Google's Gemini models"
	default:
		return "LLM provider for summarization"
	}
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}