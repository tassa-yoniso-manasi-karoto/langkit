package llms

import (
	"context"
)

// Provider defines the interface for LLM providers
type Provider interface {
	// GetName returns the provider's name
	GetName() string
	
	// GetDescription returns the provider's description
	GetDescription() string
	
	// RequiresAPIKey indicates if the provider needs an API key
	RequiresAPIKey() bool
	
	// GetAvailableModels returns the list of available models
	// Uses context for potential cancellation during model fetching
	GetAvailableModels(ctx context.Context) []ModelInfo
	
	// Complete generates a completion from the prompt
	Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error)
}

// ModelProvider is an interface for specific model interactions
type ModelProvider interface {
	// GetProvider returns the provider name
	GetProvider() string
	
	// GetModel returns the model name
	GetModel() string
	
	// GetCapabilities returns what the model can do
	GetCapabilities() []string
	
	// Complete generates a completion with the specific model
	Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error)
}