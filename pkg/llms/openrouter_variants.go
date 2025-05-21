package llms

import (
	"context"
	"strings"
)

// OpenRouterFreeProvider wraps the master OpenRouterProvider to filter for free models.
type OpenRouterFreeProvider struct {
	masterProvider *OpenRouterProvider // Points to the single underlying OpenRouter provider
}

// NewOpenRouterFreeProvider creates a new OpenRouterFreeProvider instance.
func NewOpenRouterFreeProvider(master *OpenRouterProvider) Provider {
	if master == nil {
		Logger.Error().Msg("Master OpenRouter provider is nil, cannot create OpenRouterFreeProvider.")
		return nil
	}
	return &OpenRouterFreeProvider{masterProvider: master}
}

// GetName returns the provider's name.
func (p *OpenRouterFreeProvider) GetName() string {
	return "openrouter-free"
}

// GetDescription returns the provider's description.
func (p *OpenRouterFreeProvider) GetDescription() string {
	return "OpenRouter: Free models (sorted by weekly popularity)"
}

// RequiresAPIKey indicates if the provider needs an API key (delegates to master).
func (p *OpenRouterFreeProvider) RequiresAPIKey() bool {
	return p.masterProvider.RequiresAPIKey()
}

// GetAvailableModels returns free models from the master OpenRouter provider.
func (p *OpenRouterFreeProvider) GetAvailableModels(ctx context.Context) []ModelInfo {
	allModels := p.masterProvider.GetAvailableModels(ctx) // Get all models (cached by master)
	var freeModels []ModelInfo

	for _, model := range allModels {
		isFreeByID := strings.HasSuffix(strings.ToLower(model.ID), ":free")
		isFreeByName := strings.Contains(strings.ToLower(model.Name), "(free)")

		if isFreeByID || isFreeByName {
			freeModel := model // Copy struct
			freeModel.ProviderName = p.GetName() // Set this wrapper's name
			freeModels = append(freeModels, freeModel)
		}
	}
	Logger.Debug().Int("count", len(freeModels)).Str("provider", p.GetName()).Msg("Filtered free models for OpenRouter")
	return freeModels
}

// Complete delegates the completion request to the master OpenRouter provider.
func (p *OpenRouterFreeProvider) Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error) {
	response, err := p.masterProvider.Complete(ctx, request)
	if err == nil {
		response.Provider = p.GetName() // Stamp with this wrapper's name
	}
	return response, err
}

// --- OpenRouterPaidProvider ---

// OpenRouterPaidProvider wraps the master OpenRouterProvider to filter for non-free (paid) models.
type OpenRouterPaidProvider struct {
	masterProvider *OpenRouterProvider
}

// NewOpenRouterPaidProvider creates a new OpenRouterPaidProvider instance.
func NewOpenRouterPaidProvider(master *OpenRouterProvider) Provider {
	if master == nil {
		Logger.Error().Msg("Master OpenRouter provider is nil, cannot create OpenRouterPaidProvider.")
		return nil
	}
	return &OpenRouterPaidProvider{masterProvider: master}
}

// GetName returns the provider's name.
func (p *OpenRouterPaidProvider) GetName() string {
	// We can name this "openrouter" to make it the "standard" OpenRouter offering,
	// or "openrouter-paid" to be explicit. Let's use "openrouter" for now,
	// assuming it represents the general (mostly paid) OpenRouter experience.
	return "openrouter"
}

// GetDescription returns the provider's description.
func (p *OpenRouterPaidProvider) GetDescription() string {
	return "OpenRouter: Standard (mostly paid) models (sorted by weekly popularity)"
}

// RequiresAPIKey indicates if the provider needs an API key (delegates to master).
func (p *OpenRouterPaidProvider) RequiresAPIKey() bool {
	return p.masterProvider.RequiresAPIKey()
}

// GetAvailableModels returns non-free models from the master OpenRouter provider.
func (p *OpenRouterPaidProvider) GetAvailableModels(ctx context.Context) []ModelInfo {
	allModels := p.masterProvider.GetAvailableModels(ctx) // Get all models (cached by master)
	var paidModels []ModelInfo

	for _, model := range allModels {
		isFreeByID := strings.HasSuffix(strings.ToLower(model.ID), ":free")
		isFreeByName := strings.Contains(strings.ToLower(model.Name), "(free)")

		if !(isFreeByID || isFreeByName) { // If NOT free, it's considered paid
			paidModel := model // Copy struct
			paidModel.ProviderName = p.GetName() // Set this wrapper's name
			paidModels = append(paidModels, paidModel)
		}
	}
	Logger.Debug().Int("count", len(paidModels)).Str("provider", p.GetName()).Msg("Filtered non-free models for OpenRouter")
	return paidModels
}

// Complete delegates the completion request to the master OpenRouter provider.
func (p *OpenRouterPaidProvider) Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error) {
	response, err := p.masterProvider.Complete(ctx, request)
	if err == nil {
		response.Provider = p.GetName() // Stamp with this wrapper's name
	}
	return response, err
}