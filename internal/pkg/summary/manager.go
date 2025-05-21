package summary

import (
	"sync"

	"github.com/rs/zerolog"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

var (
	defaultService     *Service
	defaultServiceOnce sync.Once
	
	// Make logger accessible to other files in this package
	// It will be initialized by the Initialize function.
	logger zerolog.Logger 
)

// Initialize sets up the summary service with a logger and the LLM registry.
func Initialize(log zerolog.Logger, llmRegistry *llms.Registry) {
	logger = log.With().Str("component", "summary").Logger() // Initialize package-level logger

	// Initialize the service
	defaultServiceOnce.Do(func() {
		// Create the service without providers initially
		defaultService = NewService(llms.GetDefaultClient()) // Use default client for backward compatibility
		
		// Start monitoring LLM registry state changes
		go listenToLLMRegistryChanges(llmRegistry)
	})

	logger.Info().Msg("Summary service initialized")
}

// listenToLLMRegistryChanges monitors the registry state and updates providers when ready
func listenToLLMRegistryChanges(registry *llms.Registry) {
	if registry == nil {
		logger.Warn().Msg("LLM registry is nil, summary service will have limited functionality")
		return
	}
	
	// Subscribe to registry state changes
	stateChan := registry.SubscribeToStateChanges()
	logger.Info().Msg("Subscribed to LLM registry state changes")
	
	// Process state changes in a loop
	for stateChange := range stateChan {
		logger.Debug().
			Str("global_state", stateChange.GlobalState.String()).
			Msg("Received LLM registry state change")
			
		// Handle state change
		handleStateChange(registry, stateChange)
		
		// If registry is shutting down, break the loop
		if stateChange.GlobalState == llms.GSError && stateChange.Message == "Registry shutting down" {
			logger.Info().Msg("LLM registry shutting down, stopping provider monitoring")
			break
		}
	}
	
	logger.Info().Msg("LLM registry state change monitor stopped")
}

// handleStateChange processes registry state changes and updates summary providers
func handleStateChange(registry *llms.Registry, state llms.StateChange) {
	if state.GlobalState != llms.GSReady {
		return // Only process ready state
	}
	
	logger.Info().Msg("LLM registry is ready, updating summary providers")
	
	// Get the LLM client from the registry
	llmClient, err := registry.GetClient()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get LLM client from registry")
		defaultService.ClearProviders() // Ensure no stale providers
		return
	}
	
	// Update the summary service's internal llmClient
	defaultService.mu.Lock()
	defaultService.llmClient = llmClient
	defaultService.mu.Unlock()
	
	// Clear existing providers
	defaultService.ClearProviders()
	
	// Register wrapped providers
	registeredLLMProviders := llmClient.ListProviders()
	if len(registeredLLMProviders) == 0 {
		logger.Warn().Msg("No LLM providers found in registry client")
		return
	}
	
	for _, llmProvider := range registeredLLMProviders {
		summaryProvider := &DefaultSummaryProvider{
			BaseProvider: NewBaseProvider(llmClient, llmProvider.GetName()),
		}
		defaultService.RegisterProvider(summaryProvider)
		logger.Debug().
			Str("provider", llmProvider.GetName()).
			Msg("Registered LLM provider for summary service")
	}
	
	logger.Info().
		Int("provider_count", len(registeredLLMProviders)).
		Msg("Summary service providers updated from registry")
}

// GetDefaultService returns the default summary service.
func GetDefaultService() *Service {
	defaultServiceOnce.Do(func() {
		// Create a basic service with default client
		// This will be updated when the registry is ready
		defaultService = NewService(llms.GetDefaultClient())
	})
	return defaultService
}

// Shutdown performs any necessary cleanup for the summary service.
func Shutdown() {
	logger.Debug().Msg("Summary service shutting down")
	
	// Clean up providers if needed
	if defaultService != nil {
		defaultService.ClearProviders()
	}
	
	// Reset the singleton instance for a clean initialization if needed later
	defaultServiceOnce = sync.Once{}
	defaultService = nil
	
	logger.Info().Msg("Summary service shutdown completed")
}