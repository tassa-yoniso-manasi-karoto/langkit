package summary

import (
	"sync"

	"github.com/rs/zerolog"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms" // Assuming this path
)

var (
	defaultService     *Service
	defaultServiceOnce sync.Once
	
	// Make logger accessible to other files in this package
	// It will be initialized by the Initialize function.
	logger zerolog.Logger 
)

// Initialize sets up the summary service with a logger and registers default providers.
func Initialize(log zerolog.Logger) {
	logger = log.With().Str("component", "summary").Logger() // Initialize package-level logger

	// Initialize and get the default summary service
	GetDefaultService()

	logger.Info().Msg("Summary service initialized")
}

// GetDefaultService returns the default summary service.
// It ensures that underlying LLM providers are wrapped as summary providers.
func GetDefaultService() *Service {
	defaultServiceOnce.Do(func() {
		llmClient := llms.GetDefaultClient() // Get the initialized LLM client

		defaultService = NewService(llmClient)

		// Automatically wrap all registered llms.Providers as summary.DefaultSummaryProvider
		registeredLLMProviders := llmClient.ListProviders()
		if len(registeredLLMProviders) == 0 {
			logger.Warn().Msg("No LLM providers found registered in llms.Client. Summary service might not have any providers.")
		}

		for _, llmProvider := range registeredLLMProviders {
			summaryProvider := &DefaultSummaryProvider{
				BaseProvider: NewBaseProvider(llmClient, llmProvider.GetName()),
			}
			defaultService.RegisterProvider(summaryProvider)
			logger.Debug().Str("provider", llmProvider.GetName()).Msg("Auto-registered LLM provider for summary service")
		}
	})
	return defaultService
}

// Shutdown performs any necessary cleanup for the summary service.
func Shutdown() {
	logger.Debug().Msg("Summary service shutting down")
	// No specific resources to clean up in summary service itself for now.
}