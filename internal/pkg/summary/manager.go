package summary

import (
	"sync"
	
	"github.com/rs/zerolog"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

var (
	defaultService     *Service
	defaultServiceOnce sync.Once
	
	// Logger instance for the summary package
	logger zerolog.Logger
)

// Initialize sets up the summary service with a logger
func Initialize(l zerolog.Logger) {
	logger = l.With().Str("component", "summary").Logger()
	
	// Ensure the service is initialized
	GetDefaultService()
	
	logger.Info().Msg("Summary service initialized")
}

// GetDefaultService returns the default summary service
func GetDefaultService() *Service {
	defaultServiceOnce.Do(func() {
		// Get LLM client
		llmClient := llms.GetDefaultClient()
		
		// Ensure providers are registered
		llms.RegisterDefaultProviders()
		
		// Create summary service
		defaultService = NewService(llmClient)
		
		// Register providers
		registerDefaultProviders(defaultService)
	})
	
	return defaultService
}

// Initialize and register default providers
func registerDefaultProviders(service *Service) {
	// Register the default provider which wraps the LLM providers
	llmClient := llms.GetDefaultClient()
	
	for _, llmProvider := range llmClient.ListProviders() {
		service.RegisterProvider(&DefaultSummaryProvider{
			BaseProvider: NewBaseProvider(llmClient, llmProvider.GetName()),
		})
		
		if logger.Debug().Enabled() {
			logger.Debug().
				Str("provider", llmProvider.GetName()).
				Int("models", len(llmProvider.GetAvailableModels())).
				Msg("Registered summary provider")
		}
	}
}

// Shutdown performs any necessary cleanup
func Shutdown() {
	if logger.Debug().Enabled() {
		logger.Debug().Msg("Summary service shutting down")
	}
}