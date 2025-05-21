package summary

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

var (
	ErrProviderNotFound = errors.New("summary provider not found")
	ErrModelNotFound    = errors.New("model not found for provider")
	ErrGenerationFailed = errors.New("summary generation failed")
	ErrInvalidOptions   = errors.New("invalid summary options")
)

// Service handles generating summaries using LLM providers
type Service struct {
	llmClient *llms.Client
	providers map[string]Provider
	mu        sync.RWMutex  // For thread-safe access to providers map
}

// NewService (definition remains the same)
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
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.providers[provider.GetName()] = provider
}

// ClearProviders removes all registered providers
func (s *Service) ClearProviders() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.providers = make(map[string]Provider)
}

// GetProvider returns a provider by name
func (s *Service) GetProvider(llmProviderName string) (Provider, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	provider, ok := s.providers[llmProviderName]
	return provider, ok
}

// ListProviders returns all registered providers
func (s *Service) ListProviders() []Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	providersList := make([]Provider, 0, len(s.providers))
	for _, provider := range s.providers {
		providersList = append(providersList, provider)
	}
	return providersList
}

// GetAvailableModels returns all available models across providers
func (s *Service) GetAvailableModels() []llms.ModelInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var allModels []llms.ModelInfo
	for _, provider := range s.providers {
		models := provider.GetSupportedModels()
		if models != nil {
			allModels = append(allModels, models...)
		}
	}
	return allModels
}

// GetModelsForProvider (definition remains the same)
func (s *Service) GetModelsForProvider(llmProviderName string) ([]llms.ModelInfo, error) {
	provider, ok := s.GetProvider(llmProviderName)
	if !ok {
		return nil, fmt.Errorf("summary functionality for LLM provider '%s' not found: %w", llmProviderName, ErrProviderNotFound)
	}
	return provider.GetSupportedModels(), nil
}

// GenerateSummary generates a summary using the specified provider and model.
// subtitleText is the already prepared text from subtitles.
// inputLanguageName is the English name of the subtitle's language.
// options includes other parameters like OutputLanguage, Model, Provider, etc.
func (s *Service) GenerateSummary(ctx context.Context, subtitleText string, inputLanguageName string, options Options) (string, error) {
	if subtitleText == "" {
		return "", fmt.Errorf("empty text provided to summarize: %w", ErrInvalidOptions)
	}
	if options.Provider == "" {
		return "", fmt.Errorf("LLM provider name is required in summary options: %w", ErrInvalidOptions)
	}
	if options.Model == "" {
		return "", fmt.Errorf("LLM model name is required in summary options: %w", ErrInvalidOptions)
	}

	summaryProvider, ok := s.GetProvider(options.Provider)
	if !ok {
		_, llmProviderExists := s.llmClient.GetProvider(options.Provider)
		if !llmProviderExists {
			return "", fmt.Errorf("LLM provider '%s' not found for summary generation: %w", options.Provider, ErrProviderNotFound)
		}
		logger.Debug().Str("llm_provider", options.Provider).Msg("No specific summary provider registered, using DefaultSummaryProvider wrapper.")
		summaryProvider = &DefaultSummaryProvider{ 
			BaseProvider: NewBaseProvider(s.llmClient, options.Provider),
		}
	}
	
	summary, err := summaryProvider.Generate(ctx, subtitleText, inputLanguageName, options)
	if err != nil {
		return "", fmt.Errorf("summary generation failed via provider '%s': %w", options.Provider, err)
	}

	return summary, nil
}

// DefaultSummaryProvider is a basic provider that uses an llms.Provider directly
type DefaultSummaryProvider struct {
	BaseProvider 
}

// Generate creates a summary from text using the embedded llms.Provider
func (p *DefaultSummaryProvider) Generate(ctx context.Context, subtitleText string, inputLanguageName string, options Options) (string, error) {
	finalPrompt := GeneratePrompt(subtitleText, inputLanguageName, options)

	llmRequest := llms.CompletionRequest{
		Prompt:           finalPrompt,
		Model:            options.Model,
		MaxTokens:        options.MaxLength * 2, // Heuristic for generation length
		Temperature:      options.Temperature,
	}
	if llmRequest.Temperature < 0 { // Indicates use LLM default
		llmRequest.Temperature = 0.7 // A common default if our internal default is negative
	}

	llmProviderInstance, ok := p.llmClient.GetProvider(p.GetName())
	if !ok {
		return "", fmt.Errorf("underlying LLM provider '%s' not found for DefaultSummaryProvider", p.GetName())
	}

	response, err := llmProviderInstance.Complete(ctx, llmRequest)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response.Text), nil
}