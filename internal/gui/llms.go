package gui

import (
	"fmt"
	"strings"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/summary"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

// GetAvailableSummaryProviders returns a list of available LLM providers for summarization
func (a *App) GetAvailableSummaryProviders() (map[string]interface{}, error) {
	a.getLogger().Debug().Msg("Fetching available summary providers")

	// First check LLM registry state
	if a.llmRegistry == nil {
		a.getLogger().Warn().Msg("LLM registry not initialized")
		return map[string]interface{}{
			"providers": []map[string]string{},
			"names":     []string{},
			"available": false,
			"suggested": "",
			"status":    "registry_not_initialized",
			"message":   "LLM registry not initialized yet",
		}, nil
	}

	stateSnapshot := a.llmRegistry.GetCurrentStateSnapshot()

	// If registry is not ready, return appropriate status
	if stateSnapshot.GlobalState != llms.GSReady {
		a.getLogger().Info().
			Str("global_state", stateSnapshot.GlobalState.String()).
			Msg("LLM registry not ready yet")

		return map[string]interface{}{
			"providers": []map[string]string{},
			"names":     []string{},
			"available": false,
			"suggested": "",
			"status":    stateSnapshot.GlobalState.String(),
			"message":   "LLM providers are still initializing",
		}, nil
	}

	// Get the summary service
	summaryService := summary.GetDefaultService()
	if summaryService == nil {
		err := fmt.Errorf("summary service not initialized")
		a.getLogger().Error().Err(err).Msg("Failed to get summary providers")
		return map[string]interface{}{
			"providers": []map[string]string{},
			"names":     []string{},
			"available": false,
			"suggested": "",
			"status":    "summary_service_not_initialized",
			"message":   "Summary service not initialized yet",
		}, nil
	}

	// Get the list of providers
	providers := summaryService.ListProviders()

	// Create the response structure
	response := map[string]interface{}{
		"providers": []map[string]string{},
		"names":     []string{},
		"available": len(providers) > 0,
		"suggested": "",
		"status":    "ready",
	}

	// Add provider details
	providersList := make([]map[string]string, 0, len(providers))
	namesList := make([]string, 0, len(providers))

	for _, provider := range providers {
		providerName := provider.GetName()
		namesList = append(namesList, providerName)

		providerInfo := map[string]string{
			"name":        providerName,
			"displayName": displayNameForProvider(providerName),
			"description": descriptionForProvider(providerName),
		}

		// Add status information from provider states if available
		if providerState, exists := stateSnapshot.ProviderStatesSnapshot[providerName]; exists {
			providerInfo["status"] = providerState.Status
			if providerState.Status == "error" && providerState.Error != "" {
				providerInfo["error"] = providerState.Error
			}
		}

		providersList = append(providersList, providerInfo)
	}

	response["providers"] = providersList
	response["names"] = namesList

	// Set suggested provider - prioritize openrouter-free only
	// First check for openrouter-free
	for _, name := range namesList {
		if name == "openrouter-free" {
			response["suggested"] = "openrouter-free"
			break
		}
	}

	// If no openrouter-free and there's at least one available, use the first one
	if response["suggested"] == "" && len(namesList) > 0 {
		response["suggested"] = namesList[0]
	}

	return response, nil
}

// GetAvailableSummaryModels returns a list of available models for a specified provider
func (a *App) GetAvailableSummaryModels(providerName string) (map[string]interface{}, error) {
	a.getLogger().Debug().Str("provider", providerName).Msg("Fetching available summary models")

	// First check LLM registry state
	if a.llmRegistry != nil {
		stateSnapshot := a.llmRegistry.GetCurrentStateSnapshot()

		// If registry is not ready, return appropriate status
		if stateSnapshot.GlobalState != llms.GSReady {
			return map[string]interface{}{
				"models":    []map[string]interface{}{},
				"names":     []string{},
				"available": false,
				"suggested": "",
				"status":    stateSnapshot.GlobalState.String(),
				"message":   "LLM providers are still initializing",
			}, nil
		}

		// If this specific provider is in error state, return that info
		if providerState, exists := stateSnapshot.ProviderStatesSnapshot[providerName]; exists {
			if providerState.Status == "error" {
				errMsg := "Provider initialization failed"
				if providerState.Error != "" {
					errMsg = providerState.Error
				}

				return map[string]interface{}{
					"models":    []map[string]interface{}{},
					"names":     []string{},
					"available": false,
					"suggested": "",
					"status":    "error",
					"message":   errMsg,
				}, nil
			}
		}
	}

	// Get the summary service
	summaryService := summary.GetDefaultService()
	if summaryService == nil {
		err := fmt.Errorf("summary service not initialized")
		a.getLogger().Error().Err(err).Msg("Failed to get summary models")
		return nil, err
	}

	// Get models for the specified provider
	models, err := summaryService.GetModelsForProvider(providerName)
	if err != nil {
		a.getLogger().Error().Err(err).Str("provider", providerName).Msg("Failed to get models for provider")
		return nil, err
	}

	// Create the response structure
	response := map[string]interface{}{
		"models":    []map[string]interface{}{},
		"names":     []string{},
		"available": len(models) > 0,
		"suggested": "",
		"status":    "ready",
	}

	// Add model details
	modelsList := make([]map[string]interface{}, 0, len(models))
	namesList := make([]string, 0, len(models))

	for _, model := range models {
		namesList = append(namesList, model.ID)

		modelInfo := map[string]interface{}{
			"id":           model.ID,
			"name":         model.Name,
			"description":  model.Description,
			"providerName": model.ProviderName,
		}
		modelsList = append(modelsList, modelInfo)

		// Look for GPT-4o or Claude models to set as suggested
		if response["suggested"] == "" {
			if strings.Contains(strings.ToLower(model.ID), "gpt-4o") ||
				strings.Contains(strings.ToLower(model.ID), "claude-3") {
				response["suggested"] = model.ID
			}
		}
	}

	response["models"] = modelsList
	response["names"] = namesList

	// If no suggested model yet and there's at least one available, use the first one
	if response["suggested"] == "" && len(namesList) > 0 {
		response["suggested"] = namesList[0]
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