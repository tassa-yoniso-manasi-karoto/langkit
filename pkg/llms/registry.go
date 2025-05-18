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

// Initialize sets up the LLM system with a logger
func Initialize(l zerolog.Logger) {
	Logger = l.With().Str("component", "llms").Logger()
	
	// Register API keys from settings
	settings, err := config.LoadSettings()
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to load settings for LLM providers")
	} else {
		// Load API keys
		LoadAPIKeysFromSettings(settings)
	}
	
	// Ensure LLM client is initialized
	GetDefaultClient()
	
	Logger.Info().Msg("LLM client system initialized")
}

// GetDefaultClient returns the default LLM client instance
func GetDefaultClient() *Client {
	defaultClientOnce.Do(func() {
		defaultClient = NewClient()
	})
	return defaultClient
}

// LoadAPIKeysFromSettings loads API keys from app settings into the LLM API key store
func LoadAPIKeysFromSettings(settings config.Settings) {
	// Store API keys for LLM providers
	APIKeys.Store("openai", settings.APIKeys.OpenAI)
	APIKeys.Store("openrouter", settings.APIKeys.OpenRouter)
	APIKeys.Store("google", settings.APIKeys.Google)
	
	if Logger.Debug().Enabled() {
		// Log which providers have valid API keys (without revealing the keys)
		providers := []string{"openai", "openrouter", "google"}
		for _, provider := range providers {
			Logger.Debug().
				Str("provider", provider).
				Bool("has_key", APIKeys.Has(provider)).
				Msg("LLM provider API key status")
		}
	}
}

// RegisterDefaultProviders initializes and registers all default providers
// based on configuration settings
func RegisterDefaultProviders() {
	client := GetDefaultClient()
	providersRegistered := 0
	
	// Register OpenAI provider if API key is available
	if APIKeys.Has("openai") {
		apiKey := APIKeys.Get("openai")
		provider := NewOpenAIProvider(apiKey)
		client.RegisterProvider(provider)
		providersRegistered++
		
		// Set as default if first provider
		if providersRegistered == 1 {
			client.SetDefaultProvider("openai")
		}
		
		if Logger.Debug().Enabled() {
			Logger.Debug().
				Str("provider", "openai").
				Int("models", len(provider.GetAvailableModels())).
				Msg("Registered OpenAI provider")
		}
	}
	
	// Register OpenRouter provider if API key is available
	if APIKeys.Has("openrouter") {
		apiKey := APIKeys.Get("openrouter")
		provider := NewOpenRouterProvider(apiKey)
		client.RegisterProvider(provider)
		providersRegistered++
		
		if Logger.Debug().Enabled() {
			Logger.Debug().
				Str("provider", "openrouter").
				Int("models", len(provider.GetAvailableModels())).
				Msg("Registered OpenRouter provider")
		}
	}
	
	// Log total providers registered
	if Logger.Info().Enabled() {
		Logger.Info().
			Int("count", providersRegistered).
			Msg("LLM providers registered")
	}
}