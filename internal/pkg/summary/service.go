package summary

import (
	"context"
	"errors"
	"fmt"
	"strings"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

var (
	ErrProviderNotFound = errors.New("summary provider not found")
	ErrModelNotFound    = errors.New("model not found for provider")
	ErrGenerationFailed = errors.New("summary generation failed")
	ErrInvalidOptions   = errors.New("invalid summary options")
)

// Service handles generating summaries
type Service struct {
	llmClient *llms.Client
	providers map[string]Provider
}

// NewService creates a new summary service
func NewService(llmClient *llms.Client) *Service {
	return &Service{
		llmClient: llmClient,
		providers: make(map[string]Provider),
	}
}

// RegisterProvider adds a provider to the service
func (s *Service) RegisterProvider(provider Provider) {
	if provider == nil {
		return
	}
	s.providers[provider.GetName()] = provider
}

// GetProvider returns a provider by name
func (s *Service) GetProvider(name string) (Provider, bool) {
	provider, ok := s.providers[name]
	return provider, ok
}

// ListProviders returns all registered providers
func (s *Service) ListProviders() []Provider {
	providers := make([]Provider, 0, len(s.providers))
	for _, provider := range s.providers {
		providers = append(providers, provider)
	}
	return providers
}

// GetAvailableModels returns all available models across providers
func (s *Service) GetAvailableModels() []llms.ModelInfo {
	var models []llms.ModelInfo
	
	for _, provider := range s.providers {
		models = append(models, provider.GetSupportedModels()...)
	}
	
	return models
}

// GetModelsForProvider returns models for a specific provider
func (s *Service) GetModelsForProvider(providerName string) ([]llms.ModelInfo, error) {
	provider, ok := s.GetProvider(providerName)
	if !ok {
		return nil, ErrProviderNotFound
	}
	
	return provider.GetSupportedModels(), nil
}

// GenerateSummary generates a summary using the specified provider and model
func (s *Service) GenerateSummary(ctx context.Context, text string, options Options) (string, error) {
	if text == "" {
		return "", fmt.Errorf("empty text to summarize: %w", ErrInvalidOptions)
	}
	
	if options.Provider == "" {
		return "", fmt.Errorf("provider is required: %w", ErrProviderNotFound)
	}
	
	provider, ok := s.GetProvider(options.Provider)
	if !ok {
		// Try direct access through LLM client
		directProvider, directOk := s.llmClient.GetProvider(options.Provider)
		if !directOk {
			return "", fmt.Errorf("provider '%s' not found: %w", options.Provider, ErrProviderNotFound)
		}
		
		// Create a default summary provider wrapping the LLM provider
		provider = &DefaultSummaryProvider{
			BaseProvider: NewBaseProvider(s.llmClient, options.Provider),
		}
	}
	
	// Check if model is supported
	modelSupported := false
	for _, model := range provider.GetSupportedModels() {
		if model.ID == options.Model {
			modelSupported = true
			break
		}
	}
	
	if !modelSupported && options.Model != "" {
		return "", fmt.Errorf("model '%s' not supported by provider '%s': %w", 
			options.Model, options.Provider, ErrModelNotFound)
	}
	
	// Generate the summary
	summary, err := provider.Generate(ctx, text, options)
	if err != nil {
		return "", fmt.Errorf("summary generation failed: %w", err)
	}
	
	return summary, nil
}

// DefaultSummaryProvider is a basic provider that uses LLM directly
type DefaultSummaryProvider struct {
	BaseProvider
}

// Generate creates a summary from text
func (p *DefaultSummaryProvider) Generate(ctx context.Context, text string, options Options) (string, error) {
	prompt := GeneratePrompt(text, options)
	
	// Set system prompt for context
	systemPrompt := "You are an expert summarizer. Your task is to create accurate, " +
		"well-structured summaries of content. Focus on capturing the key points, " +
		"main ideas, and essential information while maintaining the original meaning and tone."
	
	// Create completion request
	request := llms.CompletionRequest{
		Prompt:       prompt,
		MaxTokens:    calculateMaxTokens(options.MaxLength),
		Temperature:  options.Temperature,
		Model:        options.Model,
		SystemPrompt: systemPrompt,
	}
	
	// Get completion from LLM
	response, err := p.llmClient.Complete(ctx, p.llmProvider, request)
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(response.Text), nil
}

// calculateMaxTokens estimates token count based on word length
func calculateMaxTokens(maxWords int) int {
	// Average words per token is around 0.75, so multiply by 4/3
	// Add 20% buffer
	return int(float64(maxWords) * 1.33 * 1.2)
}