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

	// Ensure the llms.Client is initialized and default providers are registered
	// This might be done elsewhere at app startup, but good to ensure here too.
	// llms.Initialize(logger) // If llms package also has an Initialize function
	// llms.RegisterDefaultProviders() // This should load API keys and register llms.Providers

	// Initialize and get the default summary service
	GetDefaultService()

	logger.Info().Msg("Summary service initialized")
}

// GetDefaultService returns the default summary service.
// It ensures that underlying LLM providers are wrapped as summary providers.
func GetDefaultService() *Service {
	defaultServiceOnce.Do(func() {
		llmClient := llms.GetDefaultClient() // Get the initialized LLM client
		
		// Ensure LLM providers are registered (might be redundant if Initialize does it, but safe)
		// This step is crucial as it loads API keys and makes llms.Providers available.
		// If llms.Initialize and llms.RegisterDefaultProviders are not called before this,
		// llmClient.ListProviders() might be empty.
		// Consider if llms.Initialize should be a prerequisite call before summary.Initialize.
		// For now, let's assume llms.RegisterDefaultProviders has been called.

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