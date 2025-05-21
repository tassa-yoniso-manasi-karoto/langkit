package llms

import (
	"sync"

	"github.com/rs/zerolog"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
)

var (
	defaultClient     *Client
	defaultClientOnce sync.Once
	Logger            zerolog.Logger // Package-level logger for use by providers
)

// LoadAPIKeysFromSettings loads API keys from the config
func LoadAPIKeysFromSettings(settings config.Settings) {
	// Create new APIKeys store
	APIKeys = NewAPIKeyStore()

	// Add keys from settings
	if settings.APIKeys.OpenAI != "" {
		APIKeys.Store("openai", settings.APIKeys.OpenAI)
	}
	if settings.APIKeys.OpenRouter != "" {
		APIKeys.Store("openrouter", settings.APIKeys.OpenRouter)
	}
	if settings.APIKeys.Google != "" {
		APIKeys.Store("google", settings.APIKeys.Google)
	}

	// Log which keys are available (without showing the actual keys)
	keys := make([]string, 0)
	for provider := range APIKeys.List() {
		keys = append(keys, provider)
	}
	Logger.Debug().Strs("available_providers", keys).Msg("Loaded API keys")
}

// GetDefaultClient returns the client managed by DefaultRegistry if available,
// otherwise returns an empty client to prevent nil panics
func GetDefaultClient() *Client {
	if DefaultRegistry == nil {
		Logger.Error().Msg("LLM DefaultRegistry (async manager) not initialized. Call core.InitLLM first.")
		// To prevent nil panics downstream, return a new, empty client.
		// The summary service will then find no providers until the registry is ready.
		return NewClient()
	}
	
	client, err := DefaultRegistry.GetClient() // GetClient() from registry_async.go
	if err != nil {
		// This means the registry is not in a 'Ready' state.
		Logger.Warn().Err(err).Msg("LLM Registry is not ready or encountered an error. Returning an empty LLM client to summary service.")
		return NewClient() // Return an empty client
	}
	
	return client
}

// SetDefaultClient is preserved for compatibility
// but it no longer affects the global client directly
func SetDefaultClient(client *Client) {
	Logger.Warn().Msg("SetDefaultClient called, but the default client is now managed by DefaultRegistry")
	// No-op as the client is now managed by DefaultRegistry
}

// DefaultRegistry holds the global registry instance
var DefaultRegistry *Registry