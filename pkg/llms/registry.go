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

	settings, err := config.LoadSettings()
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to load settings for LLM providers")
	} else {
		LoadAPIKeysFromSettings(settings)
	}

	// Force re-initialization of the client and re-registration of providers
	// This ensures that if API keys change, the providers reflect this.
	defaultClientOnce = sync.Once{} // Reset the once guard
	GetDefaultClient()              // This will create a new client instance
	RegisterDefaultProviders()      // Register providers with the new client

	Logger.Info().Msg("LLM client system initialized and providers registered")
}

// GetDefaultClient returns the default LLM client instance
// If called after Initialize has reset defaultClientOnce, it will create a new client.
func GetDefaultClient() *Client {
	defaultClientOnce.Do(func() {
		Logger.Debug().Msg("Creating new LLM client instance.")
		defaultClient = NewClient()
	})
	return defaultClient
}

// LoadAPIKeysFromSettings loads API keys from app settings into the LLM API key store
func LoadAPIKeysFromSettings(settings config.Settings) {
	APIKeys.Store("openai", settings.APIKeys.OpenAI)
	APIKeys.Store("openrouter", settings.APIKeys.OpenRouter) // Master OpenRouter key
	APIKeys.Store("google-gemini", settings.APIKeys.Google)   // Key for Gemini

	if Logger.Debug().Enabled() {
		providersToCheck := []string{"openai", "openrouter", "google-gemini"}
		for _, provider := range providersToCheck {
			Logger.Debug().
				Str("provider", provider).
				Bool("has_key", APIKeys.Has(provider)).
				Msg("LLM provider API key status")
		}
	}
}

// RegisterDefaultProviders initializes and registers all default providers
// based on configuration settings. It now prioritizes "openrouter-free" as the default.
// This function assumes GetDefaultClient() has provided a fresh client if re-initialization is needed.
func RegisterDefaultProviders() {
	client := GetDefaultClient()
	if client == nil {
		Logger.Error().Msg("LLM client is nil in RegisterDefaultProviders. Initialization failed.")
		return
	}
	// The client is now fresh if Initialize was called, so no need to ClearProviders.
	// If client.providers is not empty here, it means it's being called out of sequence
	// or GetDefaultClient() didn't re-initialize.
	if len(client.providers) > 0 {
		Logger.Warn().Msg("RegisterDefaultProviders called on a client that already has providers. This might lead to duplicates if not handled by client.RegisterProvider.")
		// For safety, let's clear it if we intend RegisterDefaultProviders to be the sole populator.
		// This requires adding ClearProviders to client.go or re-architecting initialization.
		// For now, we'll rely on Initialize creating a fresh client.
	}

	providersRegisteredCount := 0

	// --- OpenAI Provider ---
	if APIKeys.Has("openai") {
		apiKey := APIKeys.Get("openai")
		openAIProviderInstance := NewOpenAIProvider(apiKey)
		if openAIProviderInstance != nil {
			client.RegisterProvider(openAIProviderInstance)
			providersRegisteredCount++
			Logger.Debug().
				Str("provider", openAIProviderInstance.GetName()).
				Int("models", len(openAIProviderInstance.GetAvailableModels())).
				Msg("Registered OpenAI provider")
		} else {
			Logger.Warn().Str("provider", "openai").Msg("Failed to initialize OpenAI provider.")
		}
	}

	// --- Google Gemini Provider ---
	if APIKeys.Has("google-gemini") {
		apiKey := APIKeys.Get("google-gemini")
		googleProviderInstance := NewGeminiProvider(apiKey) // Corrected: NewGeminiProvider
		if googleProviderInstance != nil {
			client.RegisterProvider(googleProviderInstance)
			providersRegisteredCount++
			Logger.Debug().
				Str("provider", googleProviderInstance.GetName()).
				Int("models", len(googleProviderInstance.GetAvailableModels())).
				Msg("Registered Google Gemini provider")
		} else {
			Logger.Warn().Str("provider", "google-gemini").Msg("Failed to initialize Google Gemini provider.")
		}
	}

	// --- OpenRouter Providers (Master and Wrappers) ---
	openRouterMasterKeyName := "openrouter"
	if APIKeys.Has(openRouterMasterKeyName) {
		apiKey := APIKeys.Get(openRouterMasterKeyName)
		masterOpenRouterProvider := NewOpenRouterProvider(apiKey)

		if masterOpenRouterProvider != nil {
			providersRegisteredCount++ // Count OpenRouter as one underlying type

			freeProvider := NewOpenRouterFreeProvider(masterOpenRouterProvider)
			if freeProvider != nil {
				client.RegisterProvider(freeProvider)
				Logger.Debug().
					Str("provider", freeProvider.GetName()).
					Int("models", len(freeProvider.GetAvailableModels())).
					Msg("Registered OpenRouter-Free provider variant")
			} else {
				Logger.Warn().Str("provider_variant", "openrouter-free").Msg("Failed to initialize OpenRouter-Free provider variant.")
			}

			paidProvider := NewOpenRouterPaidProvider(masterOpenRouterProvider)
			if paidProvider != nil {
				client.RegisterProvider(paidProvider)
				Logger.Debug().
					Str("provider", paidProvider.GetName()).
					Int("models", len(paidProvider.GetAvailableModels())).
					Msg("Registered OpenRouter (Paid/Standard) provider variant")
			} else {
				Logger.Warn().Str("provider_variant", paidProvider.GetName()).Msg("Failed to initialize OpenRouter (Paid/Standard) provider variant.")
			}
		} else {
			Logger.Warn().Str("provider", openRouterMasterKeyName+" (master)").Msg("Failed to initialize master OpenRouter provider.")
		}
	}

	// Set default provider, prioritizing openrouter-free
	if _, ok := client.GetProvider("openrouter-free"); ok {
		client.SetDefaultProvider("openrouter-free")
		Logger.Info().Msg("Set 'openrouter-free' as default LLM provider.")
	} else if _, ok := client.GetProvider("openai"); ok {
		client.SetDefaultProvider("openai")
		Logger.Info().Msg("Set 'openai' as default LLM provider.")
	} else if _, ok := client.GetProvider("openrouter"); ok {
		client.SetDefaultProvider("openrouter")
		Logger.Info().Msg("Set 'openrouter' (Paid/Standard) as default LLM provider.")
	} else if _, ok := client.GetProvider("google-gemini"); ok {
		client.SetDefaultProvider("google-gemini")
		Logger.Info().Msg("Set 'google-gemini' as default LLM provider.")
	} else if len(client.ListProviders()) > 0 {
		firstProvider := client.ListProviders()[0].GetName()
		client.SetDefaultProvider(firstProvider)
		Logger.Info().Str("provider", firstProvider).Msg("Set first available provider as default LLM provider.")
	} else {
		Logger.Warn().Msg("No LLM providers available or registered. Default provider not set.")
	}

	Logger.Info().
		Int("count", providersRegisteredCount).
		Int("variants_registered", len(client.ListProviders())).
		Msg("LLM provider registration complete.")
}