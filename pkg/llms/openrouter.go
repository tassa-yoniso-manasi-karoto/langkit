package llms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/openai/openai-go/packages/ssestream"
	"github.com/revrost/go-openrouter"
)

// OpenRouterProvider implements the Provider interface for OpenRouter.
type OpenRouterProvider struct {
	client        *openrouter.Client
	apiKey        string
	defaultModel  string
	models        []ModelInfo
	modelsMu      sync.RWMutex
	httpClient    *http.Client
	isInitialized bool
}

// NewOpenRouterProvider creates a new OpenRouter provider.
func NewOpenRouterProvider(apiKey string) *OpenRouterProvider {
	if apiKey == "" {
		if Logger.Debug().Enabled() {
			Logger.Debug().Msg("Empty API key provided to OpenRouterProvider, cannot initialize.")
		}
		return nil
	}

	officialClient := openrouter.NewClient(apiKey)
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}
	defaultModel := "openrouter/auto"

	return &OpenRouterProvider{
		client:        officialClient,
		apiKey:        apiKey,
		defaultModel:  defaultModel,
		httpClient:    httpClient,
		isInitialized: true,
	}
}

// GetName returns the provider's name
func (p *OpenRouterProvider) GetName() string {
	return "openrouter"
}

// GetDescription returns the provider's description
func (p *OpenRouterProvider) GetDescription() string {
	return "OpenRouter: Access to multiple LLMs. Uses revrost/go-openrouter and direct HTTP for streaming."
}

// RequiresAPIKey indicates if the provider needs an API key
func (p *OpenRouterProvider) RequiresAPIKey() bool {
	return true
}

// Structs for parsing OpenRouter /models response
type OpenRouterModelArchitecture struct {
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	Tokenizer        string   `json:"tokenizer"`
	InstructType     *string  `json:"instruct_type,omitempty"`
}

type OpenRouterTopProvider struct {
	IsModerated         bool     `json:"is_moderated"`
	ContextLength       *float64 `json:"context_length,omitempty"`
	MaxCompletionTokens *float64 `json:"max_completion_tokens,omitempty"`
}

type OpenRouterPricing struct {
	Prompt            string `json:"prompt"`
	Completion        string `json:"completion"`
	Image             string `json:"image,omitempty"`
	Request           string `json:"request,omitempty"`
	InputCacheRead    string `json:"input_cache_read,omitempty"`
	InputCacheWrite   string `json:"input_cache_write,omitempty"`
	WebSearch         string `json:"web_search,omitempty"`
	InternalReasoning string `json:"internal_reasoning,omitempty"`
}

type OpenRouterModelData struct {
	ID                  string                       `json:"id"`
	Name                string                       `json:"name"`
	Created             float64                      `json:"created"`
	Description         string                       `json:"description"`
	Architecture        OpenRouterModelArchitecture  `json:"architecture"`
	TopProvider         OpenRouterTopProvider        `json:"top_provider"`
	Pricing             OpenRouterPricing            `json:"pricing"`
	ContextLength       *float64                     `json:"context_length,omitempty"`
	HuggingFaceID       *string                      `json:"hugging_face_id,omitempty"`
	PerRequestLimits    map[string]interface{}       `json:"per_request_limits,omitempty"`
	SupportedParameters []string                     `json:"supported_parameters,omitempty"`
}

type OpenRouterModelsResponse struct {
	Data []OpenRouterModelData `json:"data"`
}

// GetAvailableModels fetches and returns a list of available models from OpenRouter.
func (p *OpenRouterProvider) GetAvailableModels() []ModelInfo {
	if !p.isInitialized {
		Logger.Warn().Msg("OpenRouterProvider not initialized in GetAvailableModels")
		return nil
	}

	p.modelsMu.RLock()
	if len(p.models) > 0 {
		p.modelsMu.RUnlock()
		return p.models
	}
	p.modelsMu.RUnlock()

	p.modelsMu.Lock()
	defer p.modelsMu.Unlock()
	if len(p.models) > 0 {
		return p.models
	}

	Logger.Debug().Msg("Fetching available models from OpenRouter API (https://openrouter.ai/api/v1/models)...")

	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to create request for OpenRouter models")
		return nil
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to fetch OpenRouter models")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		Logger.Error().Int("status_code", resp.StatusCode).Str("body", string(bodyBytes)).Msg("Failed to fetch OpenRouter models: non-200 status")
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to read response body from OpenRouter models")
		return nil
	}

	var openRouterResp OpenRouterModelsResponse
	if err := json.Unmarshal(body, &openRouterResp); err != nil {
		Logger.Error().Err(err).Msg("Failed to unmarshal OpenRouter models response")
		return nil
	}

	var modelInfos []ModelInfo
	for _, orModel := range openRouterResp.Data {
		capabilities := []string{}
		canGenerateText := false
		for _, outMod := range orModel.Architecture.OutputModalities {
			if strings.ToLower(outMod) == "text" {
				canGenerateText = true
				break
			}
		}

		if !canGenerateText {
			continue
		}
		capabilities = append(capabilities, "text-generation", "chat")

		for _, inMod := range orModel.Architecture.InputModalities {
			if strings.ToLower(inMod) == "image" {
				capabilities = append(capabilities, "vision")
				break
			}
		}

		var contextLength int
		if orModel.ContextLength != nil {
			contextLength = int(*orModel.ContextLength)
		} else if orModel.TopProvider.ContextLength != nil {
			contextLength = int(*orModel.TopProvider.ContextLength)
		} else {
			contextLength = 4096
		}

		releaseDate := time.Unix(int64(orModel.Created), 0)

		modelInfos = append(modelInfos, ModelInfo{
			ID:            orModel.ID,
			Name:          orModel.Name,
			Description:   orModel.Description,
			MaxTokens:     contextLength,
			Capabilities:  uniqueStrings(capabilities),
			ProviderName:  p.GetName(),
			ReleaseDate:   releaseDate,
		})
	}

	sort.SliceStable(modelInfos, func(i, j int) bool {
		if !modelInfos[i].ReleaseDate.Equal(modelInfos[j].ReleaseDate) {
			return modelInfos[i].ReleaseDate.After(modelInfos[j].ReleaseDate)
		}
		return modelInfos[i].ID < modelInfos[j].ID
	})

	p.models = modelInfos
	Logger.Debug().Int("count", len(p.models)).Msg("Successfully fetched, filtered, and cached OpenRouter models.")
	return p.models
}

// StreamDelta defines the structure of the 'delta' field in an OpenRouter stream chunk.
// This is based on OpenAI's typical stream delta.
type StreamDelta struct {
	Content string `json:"content,omitempty"`
	Role    string `json:"role,omitempty"`
	// Add other fields like ToolCalls if OpenRouter streams them in delta
}

// OpenRouterStreamChoice is used for unmarshalling choices from OpenRouter SSE stream
type OpenRouterStreamChoice struct {
	Index        int                       `json:"index"`
	Delta        StreamDelta               `json:"delta"` // Corrected: Use StreamDelta
	FinishReason openrouter.FinishReason   `json:"finish_reason,omitempty"`
	LogProbs     *openrouter.LogProbs      `json:"logprobs,omitempty"`
}

// OpenRouterStreamChunk is used for unmarshalling data events from OpenRouter SSE stream
type OpenRouterStreamChunk struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Choices []OpenRouterStreamChoice `json:"choices"` // Corrected: Use OpenRouterStreamChoice
	Usage   *openrouter.Usage        `json:"usage,omitempty"`
}

// Complete generates a completion using the OpenRouter provider.
func (p *OpenRouterProvider) Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error) {
	if !p.isInitialized {
		return CompletionResponse{}, errors.New("openrouter provider not initialized")
	}

	modelID := request.Model
	if modelID == "" {
		modelID = p.defaultModel
		Logger.Debug().Str("model", modelID).Msg("No model specified in request, using OpenRouterProvider default model.")
	}

	var messages []openrouter.ChatCompletionMessage
	if request.SystemPrompt != "" {
		messages = append(messages, openrouter.ChatCompletionMessage{
			Role:    openrouter.ChatMessageRoleSystem,
			Content: openrouter.Content{Text: request.SystemPrompt},
		})
	}
	if request.Prompt == "" {
		return CompletionResponse{}, fmt.Errorf("%w: prompt cannot be empty for OpenRouter", ErrInvalidRequest)
	}
	messages = append(messages, openrouter.ChatCompletionMessage{
		Role:    openrouter.ChatMessageRoleUser,
		Content: openrouter.Content{Text: request.Prompt},
	})

	chatReq := openrouter.ChatCompletionRequest{
		Model:    modelID,
		Messages: messages,
	}

	if request.MaxTokens > 0 {
		chatReq.MaxCompletionTokens = request.MaxTokens
	}
	if request.Temperature >= 0 {
		chatReq.Temperature = float32(request.Temperature)
	}
	if request.TopP > 0 {
		chatReq.TopP = float32(request.TopP)
	}
	if request.N > 0 {
		chatReq.N = int(request.N)
	}
	if request.FrequencyPenalty != 0.0 {
		chatReq.FrequencyPenalty = float32(request.FrequencyPenalty)
	}
	if request.PresencePenalty != 0.0 {
		chatReq.PresencePenalty = float32(request.PresencePenalty)
	}
	if request.Seed != 0 {
		seedInt := int(request.Seed)
		chatReq.Seed = &seedInt
	}
	if len(request.StopSequences) > 0 {
		chatReq.Stop = request.StopSequences
	}
	if request.User != "" {
		chatReq.User = request.User
	}

	if request.Stream {
		chatReq.Stream = true
		if request.IncludeUsage {
			chatReq.StreamOptions = &openrouter.StreamOptions{IncludeUsage: true}
		}

		Logger.Debug().Str("model", modelID).Msg("Requesting streaming completion from OpenRouter (manual HTTP + ssestream).")

		jsonData, err := json.Marshal(chatReq)
		if err != nil {
			return CompletionResponse{}, fmt.Errorf("failed to marshal OpenRouter stream request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonData))
		if err != nil {
			return CompletionResponse{}, fmt.Errorf("failed to create OpenRouter stream HTTP request: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
		// Optional: Set HTTP-Referer and X-Title if required by OpenRouter or for your tracking
		// httpReq.Header.Set("HTTP-Referer", "YOUR_SITE_URL_OR_APP_NAME")
		// httpReq.Header.Set("X-Title", "YOUR_APP_NAME")

		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			return CompletionResponse{}, fmt.Errorf("OpenRouter stream HTTP request failed: %w", err)
		}
		// Do not defer resp.Body.Close() here, ssestream.NewDecoder takes ownership

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close() // Close body as ssestream won't be used
			Logger.Error().Int("status_code", resp.StatusCode).Str("body", string(bodyBytes)).Msg("OpenRouter stream request failed: non-200 status")
			var apiErr openrouter.APIError
			if json.Unmarshal(bodyBytes, &apiErr) == nil {
				return CompletionResponse{}, &apiErr
			}
			return CompletionResponse{}, fmt.Errorf("OpenRouter stream API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
		}

		sseDecoder := ssestream.NewDecoder(resp) // Corrected: Pass the *http.Response
		stream := ssestream.NewStream[OpenRouterStreamChunk](sseDecoder, nil)
		defer stream.Close() // This will close the underlying resp.Body via the decoder

		var fullText strings.Builder
		var finalResponse CompletionResponse
		var lastChunk OpenRouterStreamChunk // To get usage and final model/finish reason

		for stream.Next() {
			chunk := stream.Current()
			lastChunk = chunk
			if len(chunk.Choices) > 0 {
				if chunk.Choices[0].Delta.Content != "" {
					fullText.WriteString(chunk.Choices[0].Delta.Content)
				}
			}
		}

		if err := stream.Err(); err != nil {
			Logger.Error().Err(err).Msg("Error receiving stream chunk from OpenRouter")
			// Check if it's an APIError from the stream
			var apiErr *openrouter.APIError
			if errors.As(err, &apiErr) {
				return CompletionResponse{}, fmt.Errorf("OpenRouter stream API error: %w", apiErr)
			}
			return CompletionResponse{}, fmt.Errorf("OpenRouter stream processing error: %w", err)
		}
		Logger.Debug().Msg("OpenRouter stream finished.")

		finalResponse.Text = fullText.String()
		if len(lastChunk.Choices) > 0 {
			finalResponse.FinishReason = string(lastChunk.Choices[0].FinishReason)
		}
		finalResponse.Model = lastChunk.Model
		finalResponse.Provider = p.GetName()

		if lastChunk.Usage != nil && lastChunk.Usage.TotalTokens > 0 {
			finalResponse.Usage = TokenUsage{
				PromptTokens:     lastChunk.Usage.PromptTokens,
				CompletionTokens: lastChunk.Usage.CompletionTokens,
				TotalTokens:      lastChunk.Usage.TotalTokens,
			}
		} else {
			Logger.Debug().Msg("Usage data not present or zero in the final OpenRouter stream chunk.")
		}
		return finalResponse, nil

	} else { // Non-Streaming Logic
		chatReq.Stream = false
		Logger.Debug().Str("model", modelID).Msg("Requesting non-streaming completion from OpenRouter via revrost/go-openrouter.")

		openRouterResp, err := p.client.CreateChatCompletion(ctx, chatReq)
		if err != nil {
			var apiErr *openrouter.APIError
			if errors.As(err, &apiErr) {
				Logger.Error().Str("code", fmt.Sprintf("%v", apiErr.Code)).Str("message", apiErr.Message).Msg("OpenRouter API error (non-streaming)")
			}
			return CompletionResponse{}, fmt.Errorf("OpenRouter chat completion failed: %w", err)
		}

		if len(openRouterResp.Choices) == 0 {
			return CompletionResponse{}, errors.New("no choices returned from OpenRouter completion")
		}

		choice := openRouterResp.Choices[0]
		var usage TokenUsage
		usage.PromptTokens = openRouterResp.Usage.PromptTokens
		usage.CompletionTokens = openRouterResp.Usage.CompletionTokens
		usage.TotalTokens = openRouterResp.Usage.TotalTokens

		return CompletionResponse{
			Text:         choice.Message.Content.Text,
			FinishReason: string(choice.FinishReason),
			Usage:        usage,
			Model:        openRouterResp.Model,
			Provider:     p.GetName(),
		}, nil
	}
}