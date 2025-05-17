package llms

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/param"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/ssestream"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	client *openai.Client // Official OpenAI Go client
	apiKey string
	models []ModelInfo // Cached list of available models
}

// NewOpenAIProvider creates a new OpenAI provider with the given API key
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	if apiKey == "" {
		if Logger.Debug().Enabled() {
			Logger.Debug().Msg("Empty API key provided to OpenAI provider, cannot initialize.")
		}
		return nil
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	provider := &OpenAIProvider{
		client: client,
		apiKey: apiKey,
	}
	return provider
}

// GetName returns the provider's name
func (p *OpenAIProvider) GetName() string {
	return "openai"
}

// GetDescription returns the provider's description
func (p *OpenAIProvider) GetDescription() string {
	return "OpenAI API for models like GPT-4o, GPT-4, and GPT-3.5"
}

// RequiresAPIKey indicates if the provider needs an API key
func (p *OpenAIProvider) RequiresAPIKey() bool {
	return true
}

// GetAvailableModels returns the list of available models
// It fetches from the API and caches the result.
func (p *OpenAIProvider) GetAvailableModels() []ModelInfo {
	if p.client == nil {
		Logger.Warn().Msg("OpenAI client not initialized in GetAvailableModels")
		return nil
	}

	if len(p.models) > 0 {
		return p.models // Return cached models
	}

	Logger.Debug().Msg("Fetching available models from OpenAI API...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := p.client.Models.List(ctx)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to list OpenAI models")
		return nil
	}

	var modelInfos []ModelInfo
	for _, model := range resp.Data {
		var maxTokens int
		capabilities := []string{"text-generation"} // Base capability

		// Provide more specific info for common chat models
		// Note: MaxTokens here refers to the context window, not necessarily max output tokens.
		// The actual max output tokens can be less and is often controlled by a separate parameter.
		switch shared.ChatModel(model.ID) { // Use shared.ChatModel for comparison with constants
		case shared.ChatModelGPT4o, openai.ChatModelGPT4o: // openai.ChatModelGPT4o is from responses, shared.ChatModel is from shared
			maxTokens = 128000
			capabilities = append(capabilities, "chat", "vision", "summarization", "reasoning")
		case shared.ChatModelGPT4oMini, openai.ChatModelGPT4oMini:
			maxTokens = 128000 // Typically shares context window with larger variant
			capabilities = append(capabilities, "chat", "vision", "summarization", "reasoning")
		case shared.ChatModelGPT4Turbo, openai.ChatModelGPT4Turbo:
			maxTokens = 128000
			capabilities = append(capabilities, "chat", "vision", "summarization", "reasoning")
		case shared.ChatModelGPT4, openai.ChatModelGPT4:
			maxTokens = 8192 // Or 32768 for -32k variants, but List API doesn't distinguish well
			capabilities = append(capabilities, "chat", "vision", "summarization", "reasoning")
		case shared.ChatModelGPT3_5Turbo, openai.ChatModelGPT3_5Turbo: // Covers variants like -0125
			maxTokens = 16385
			capabilities = append(capabilities, "chat", "summarization")
		default:
			// Generic fallback for other models
			if strings.Contains(model.ID, "gpt-4")) {
				maxTokens = 8192 // Generic GPT-4
				capabilities = append(capabilities, "chat", "summarization")
			} else if strings.Contains(model.ID, "gpt-3.5")) {
				maxTokens = 4096 // Older GPT-3.5 or generic
				capabilities = append(capabilities, "chat", "summarization")
			} else if strings.Contains(model.ID, "text-embedding")) {
				maxTokens = 8191 // Common for ada-002
				capabilities = []string{"embedding"}
			} else if strings.Contains(model.ID, "dall-e")) {
				maxTokens = 0 // Not applicable
				capabilities = []string{"image-generation"}
			} else if strings.Contains(model.ID, "whisper")) {
				maxTokens = 0 // Not applicable
				capabilities = []string{"audio-transcription"}
			} else {
				maxTokens = 4096 // A general fallback
			}
		}

		modelInfos = append(modelInfos, ModelInfo{
			ID:           model.ID,
			Name:         model.ID,
			Description:  fmt.Sprintf("Owned by: %s", model.OwnedBy),
			MaxTokens:    maxTokens,
			Capabilities: capabilities,
			ProviderName: p.GetName(),
		})
	}

	p.models = modelInfos
	Logger.Debug().Int("count", len(p.models)).Msg("Successfully fetched and cached OpenAI models.")
	return p.models
}

// Complete generates a completion from the prompt
func (p *OpenAIProvider) Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error) {
	if p.client == nil {
		return CompletionResponse{}, errors.New("openai client not initialized")
	}
	if p.apiKey == "" {
		return CompletionResponse{}, errors.New("openai provider not initialized: missing API key")
	}

	var messages []openai.ChatCompletionMessageParamUnion
	if request.SystemPrompt != "" {
		messages = append(messages, openai.SystemMessage(request.SystemPrompt))
	}
	if request.Prompt == "" {
		return CompletionResponse{}, fmt.Errorf("%w: prompt cannot be empty", ErrInvalidRequest)
	}
	messages = append(messages, openai.UserMessage(request.Prompt))

	modelID := request.Model
	if modelID == "" {
		modelID = string(openai.ChatModelGPT4o) // Updated default model
		Logger.Debug().Str("model", modelID).Msg("No model specified in request, using default OpenAI model.")
	}

	chatReqParams := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(modelID),
		Messages: messages,
	}

	if request.MaxTokens > 0 {
		chatReqParams.MaxTokens = param.NewOptInt(int64(request.MaxTokens))
	}
	// Temperature can be 0, so check if it's explicitly set (assuming 0 is a valid unset value for our llms.CompletionRequest)
	// OpenAI default is 1. If our request.Temperature is 0 and meant "not set", we should not send it.
	// For simplicity, if Temperature is provided (even 0), we send it.
	// A more robust llms.CompletionRequest might use *float64 for Temperature.
	if request.Temperature >= 0 && request.Temperature <= 2 { // OpenAI range is 0 to 2
		chatReqParams.Temperature = param.NewOptFloat(request.Temperature)
	}
	if request.TopP > 0 && request.TopP <= 1 { // OpenAI range is 0 to 1
		chatReqParams.TopP = param.NewOptFloat(request.TopP)
	}
	if request.N > 0 {
		chatReqParams.N = param.NewOptInt(int64(request.N))
	}
	if request.FrequencyPenalty != 0 { // Range -2.0 to 2.0
		chatReqParams.FrequencyPenalty = param.NewOptFloat(request.FrequencyPenalty)
	}
	if request.PresencePenalty != 0 { // Range -2.0 to 2.0
		chatReqParams.PresencePenalty = param.NewOptFloat(request.PresencePenalty)
	}
	if request.Seed != 0 { // Assuming 0 means not set for our llms.CompletionRequest
		chatReqParams.Seed = param.NewOptInt(int64(request.Seed))
	}

	if len(request.StopSequences) > 0 {
		if len(request.StopSequences) == 1 {
			chatReqParams.Stop = openai.NewChatCompletionStopString(request.StopSequences[0])
		} else {
			chatReqParams.Stop = openai.NewChatCompletionStopArray(request.StopSequences)
		}
	}
	if request.User != "" {
		chatReqParams.User = param.NewOpt(request.User)
	}

	if request.Stream {
		if request.IncludeUsage {
			chatReqParams.StreamOptions = openai.ChatCompletionStreamOptionsParam{
				IncludeUsage: param.NewOpt(true),
			}
		}
		Logger.Debug().Interface("params", chatReqParams).Msg("Requesting streaming chat completion from OpenAI.")
		stream, err := p.client.Chat.Completions.NewStreaming(ctx, chatReqParams)
		if err != nil {
			var apiErr *openai.Error
			if errors.As(err, &apiErr) {
				Logger.Error().Str("type", apiErr.Type).Str("code", fmt.Sprintf("%v", apiErr.Code)).Str("param", apiErr.Param).Msg("OpenAI API error during streaming request")
			}
			return CompletionResponse{}, fmt.Errorf("failed to start streaming chat completion: %w", err)
		}
		defer stream.Close()

		var fullText strings.Builder
		var finalResponse CompletionResponse
		var lastChunk openai.ChatCompletionChunk // To get usage from the very last chunk

		for {
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				Logger.Debug().Msg("Stream finished.")
				break
			}
			if err != nil {
				Logger.Error().Err(err).Msg("Error receiving stream chunk")
				return CompletionResponse{}, fmt.Errorf("stream error: %w", err)
			}
			lastChunk = chunk // Keep track of the last chunk for potential usage data
			if len(chunk.Choices) > 0 {
				fullText.WriteString(chunk.Choices[0].Delta.Content)
			}
		}

		finalResponse.Text = fullText.String()
		if len(lastChunk.Choices) > 0 { // FinishReason comes from the last choice in the last content-bearing chunk
			finalResponse.FinishReason = lastChunk.Choices[0].FinishReason
		}
		finalResponse.Model = lastChunk.Model // Model is consistent across chunks
		finalResponse.Provider = p.GetName()

		// Usage is typically in the very last event if requested via StreamOptions
		if lastChunk.Usage.IsPresent() {
			finalResponse.Usage = TokenUsage{
				PromptTokens:     int(lastChunk.Usage.PromptTokens),
				CompletionTokens: int(lastChunk.Usage.CompletionTokens),
				TotalTokens:      int(lastChunk.Usage.TotalTokens),
			}
		} else {
			Logger.Debug().Msg("Usage data not present in the final stream chunk. Ensure StreamOptions.IncludeUsage was set if needed.")
		}
		return finalResponse, nil

	} else {
		Logger.Debug().Interface("params", chatReqParams).Msg("Requesting non-streaming chat completion from OpenAI.")
		resp, err := p.client.Chat.Completions.New(ctx, chatReqParams)
		if err != nil {
			var apiErr *openai.Error
			if errors.As(err, &apiErr) {
				Logger.Error().Str("type", apiErr.Type).Str("code", fmt.Sprintf("%v", apiErr.Code)).Str("param", apiErr.Param).Msg("OpenAI API error")
			}
			return CompletionResponse{}, fmt.Errorf("chat completion failed: %w", err)
		}

		if len(resp.Choices) == 0 {
			return CompletionResponse{}, errors.New("no choices returned from OpenAI completion")
		}

		choice := resp.Choices[0]
		var usage TokenUsage
		if resp.Usage.IsPresent() {
			usage.PromptTokens = int(resp.Usage.PromptTokens)
			usage.CompletionTokens = int(resp.Usage.CompletionTokens)
			usage.TotalTokens = int(resp.Usage.TotalTokens)
		}

		return CompletionResponse{
			Text:         choice.Message.Content,
			FinishReason: choice.FinishReason,
			Usage:        usage,
			Model:        resp.Model,
			Provider:     p.GetName(),
		}, nil
	}
}