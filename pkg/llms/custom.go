package llms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CustomLLMProvider implements Provider using a user-configured
// OpenAI-compatible chat completions endpoint (e.g., Ollama, llama.cpp, LocalAI)
type CustomLLMProvider struct {
	endpoint string
	model    string
}

// NewCustomLLMProvider creates a new CustomLLMProvider with the given endpoint and model
func NewCustomLLMProvider(endpoint, model string) *CustomLLMProvider {
	if endpoint == "" {
		return nil
	}
	return &CustomLLMProvider{
		endpoint: endpoint,
		model:    model,
	}
}

// GetName returns the provider name
func (p *CustomLLMProvider) GetName() string {
	return "custom"
}

// GetDescription returns the provider's description
func (p *CustomLLMProvider) GetDescription() string {
	return "User-configured local LLM endpoint (OpenAI-compatible API)"
}

// RequiresAPIKey indicates if the provider needs an API key
func (p *CustomLLMProvider) RequiresAPIKey() bool {
	return false
}

// GetAvailableModels returns a list containing just the configured model
func (p *CustomLLMProvider) GetAvailableModels(ctx context.Context) []ModelInfo {
	modelName := p.model
	if modelName == "" {
		modelName = "default"
	}
	return []ModelInfo{
		{
			ID:           modelName,
			Name:         "Custom: " + modelName,
			Description:  "Model served by custom endpoint at " + p.endpoint,
			MaxTokens:    0, // Unknown
			Capabilities: []string{"chat", "text-generation"},
			ProviderName: p.GetName(),
			ReleaseDate:  time.Now(),
		},
	}
}

// openAIChatRequest represents the OpenAI chat completions request format
type openAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openAIChatMessage `json:"messages"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	TopP        float64             `json:"top_p,omitempty"`
	N           int                 `json:"n,omitempty"`
	Stop        []string            `json:"stop,omitempty"`
	Stream      bool                `json:"stream"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIChatResponse represents the OpenAI chat completions response format
type openAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Complete generates a completion from the prompt using the custom endpoint
func (p *CustomLLMProvider) Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error) {
	if p.endpoint == "" {
		return CompletionResponse{}, fmt.Errorf("custom LLM endpoint is not configured")
	}

	// Build messages array
	var messages []openAIChatMessage
	if request.SystemPrompt != "" {
		messages = append(messages, openAIChatMessage{
			Role:    "system",
			Content: request.SystemPrompt,
		})
	}
	if request.Prompt == "" {
		return CompletionResponse{}, fmt.Errorf("%w: prompt cannot be empty", ErrInvalidRequest)
	}
	messages = append(messages, openAIChatMessage{
		Role:    "user",
		Content: request.Prompt,
	})

	// Determine model to use
	modelID := request.Model
	if modelID == "" {
		modelID = p.model
	}
	if modelID == "" {
		modelID = "default"
	}

	// Build the request body
	chatReq := openAIChatRequest{
		Model:    modelID,
		Messages: messages,
		Stream:   false, // We don't support streaming for custom endpoints for simplicity
	}

	// Only set parameters if they are meaningfully provided
	if request.MaxTokens > 0 {
		chatReq.MaxTokens = request.MaxTokens
	}
	if request.Temperature >= 0 && request.Temperature <= 2 {
		chatReq.Temperature = request.Temperature
	}
	if request.TopP > 0 && request.TopP <= 1 {
		chatReq.TopP = request.TopP
	}
	if request.N > 0 {
		chatReq.N = int(request.N)
	}
	if len(request.StopSequences) > 0 {
		chatReq.Stop = request.StopSequences
	}

	// Marshal request to JSON
	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	client := &http.Client{
		Timeout: 5 * time.Minute, // Long timeout for LLM completions
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to send request to custom endpoint: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-2xx status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return CompletionResponse{}, fmt.Errorf("custom LLM API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var chatResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract result
	if len(chatResp.Choices) == 0 {
		return CompletionResponse{}, fmt.Errorf("no choices returned from custom LLM endpoint")
	}

	choice := chatResp.Choices[0]
	return CompletionResponse{
		Text:         choice.Message.Content,
		FinishReason: choice.FinishReason,
		Usage: TokenUsage{
			PromptTokens:     chatResp.Usage.PromptTokens,
			CompletionTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:      chatResp.Usage.TotalTokens,
		},
		Model:    chatResp.Model,
		Provider: p.GetName(),
	}, nil
}
