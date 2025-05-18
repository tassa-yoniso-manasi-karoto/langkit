package llms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/teilomillet/gollm"
	gollm_config "github.com/teilomillet/gollm/config"
	gollm_llm "github.com/teilomillet/gollm/llm"
)

/* NOTE ON GOLLM:
gollm provide direct support for Google and OpenAI's models but it doesn't rely
on their official SDK so I chose to use gollm only for OpenRouter support and
use official SDKs whenever possible.
*/


// OpenRouterProvider implements the Provider interface for OpenRouter,
// leveraging the teilomillet/gollm library for completions and making
// direct API calls for model listing.
type OpenRouterProvider struct {
	gollmInstance gollm.LLM // Instance of the gollm library's LLM
	apiKey        string
	defaultModel  string
	models        []ModelInfo // Cached list of available models
	modelsMu      sync.RWMutex
	httpClient    *http.Client
}

// NewOpenRouterProvider creates a new OpenRouter provider.
func NewOpenRouterProvider(apiKey string) *OpenRouterProvider {
	if apiKey == "" {
		if Logger.Debug().Enabled() {
			Logger.Debug().Msg("Empty API key provided to OpenRouterProvider, cannot initialize.")
		}
		return nil
	}

	defaultOpenRouterModel := "openrouter/auto"

	gollmLogLevel := gollm.LogLevelWarn
	currentLogLevel := Logger.GetLevel()

	switch currentLogLevel {
	case zerolog.DebugLevel:
		gollmLogLevel = gollm.LogLevelDebug
	case zerolog.InfoLevel: 
		gollmLogLevel = gollm.LogLevelInfo
	case zerolog.ErrorLevel:
		gollmLogLevel = gollm.LogLevelError
	}

	llmInstance, err := gollm.NewLLM(
		gollm_config.SetProvider("openrouter"),
		gollm_config.SetAPIKey(apiKey),
		gollm_config.SetModel(defaultOpenRouterModel),
		gollm_config.SetLogLevel(gollmLogLevel),
		gollm_config.SetMaxRetries(3),
		gollm_config.SetRetryDelay(2*time.Second),
	)

	if err != nil {
		Logger.Error().Err(err).Msg("Failed to create gollm.LLM instance for OpenRouter")
		return nil
	}

	return &OpenRouterProvider{
		gollmInstance: llmInstance,
		apiKey:        apiKey, // Store API key for direct calls if needed (like /models)
		defaultModel:  defaultOpenRouterModel,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Timeout for HTTP requests like model listing
		},
	}
}

// GetName returns the provider's name
func (p *OpenRouterProvider) GetName() string {
	return "openrouter"
}

// GetDescription returns the provider's description
func (p *OpenRouterProvider) GetDescription() string {
	return "OpenRouter: Access to multiple LLMs, using teilomillet/gollm for completions."
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
	InstructType     *string  `json:"instruct_type"` // Optional
}

type OpenRouterTopProvider struct {
	IsModerated         bool     `json:"is_moderated"`
	ContextLength       *float64 `json:"context_length"`        // Optional
	MaxCompletionTokens *float64 `json:"max_completion_tokens"` // Optional
}

type OpenRouterPricing struct {
	Prompt             string `json:"prompt"`
	Completion         string `json:"completion"`
	Image              string `json:"image"`
	Request            string `json:"request"`
	InputCacheRead     string `json:"input_cache_read"`    // Corrected field name
	InputCacheWrite    string `json:"input_cache_write"`   // Corrected field name
	WebSearch          string `json:"web_search"`          // Corrected field name
	InternalReasoning  string `json:"internal_reasoning"`  // Corrected field name
}


type OpenRouterModel struct {
	ID                  string                       `json:"id"`
	Name                string                       `json:"name"`
	Created             float64                      `json:"created"`
	Description         string                       `json:"description"`
	Architecture        OpenRouterModelArchitecture  `json:"architecture"`
	TopProvider         OpenRouterTopProvider        `json:"top_provider"`
	Pricing             OpenRouterPricing            `json:"pricing"`
	ContextLength       *float64                     `json:"context_length"` // Optional, top-level
	HuggingFaceID       *string                      `json:"hugging_face_id"` // Optional
	PerRequestLimits    map[string]interface{}       `json:"per_request_limits"` // Optional
	SupportedParameters []string                     `json:"supported_parameters"` // Optional
}

type OpenRouterModelsResponse struct {
	Data []OpenRouterModel `json:"data"`
}

// GetAvailableModels fetches and returns a list of available models from OpenRouter.
// It caches the result to avoid repeated API calls.
func (p *OpenRouterProvider) GetAvailableModels() []ModelInfo {
	p.modelsMu.RLock()
	if len(p.models) > 0 {
		p.modelsMu.RUnlock()
		return p.models
	}
	p.modelsMu.RUnlock()

	p.modelsMu.Lock()
	defer p.modelsMu.Unlock()
	// Double check after acquiring write lock
	if len(p.models) > 0 {
		return p.models
	}

	Logger.Debug().Msg("Fetching available models from OpenRouter API (https://openrouter.ai/api/v1/models)...")

	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to create request for OpenRouter models")
		return nil
	}
	// OpenRouter doesn't require Auth for /models, but it's good practice if it changes
	// req.Header.Set("Authorization", "Bearer "+p.apiKey) // Usually not needed for /models

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
		if canGenerateText {
			capabilities = append(capabilities, "text-generation", "chat")
		}
		for _, inMod := range orModel.Architecture.InputModalities {
			if strings.ToLower(inMod) == "image" {
				capabilities = append(capabilities, "vision")
				break
			}
		}
		if len(capabilities) == 0 {
			capabilities = append(capabilities, "unknown")
		}


		var contextLength int
		if orModel.ContextLength != nil {
			contextLength = int(*orModel.ContextLength)
		} else if orModel.TopProvider.ContextLength != nil {
			contextLength = int(*orModel.TopProvider.ContextLength)
		} else {
			contextLength = 4096 // A generic fallback if not specified
		}


		modelInfos = append(modelInfos, ModelInfo{
			ID:           orModel.ID,
			Name:         orModel.Name,
			Description:  orModel.Description,
			MaxTokens:    contextLength,
			Capabilities: uniqueStrings(capabilities), // Ensure unique capabilities
			ProviderName: p.GetName(),
		})
	}

	p.models = modelInfos
	Logger.Debug().Int("count", len(p.models)).Msg("Successfully fetched and cached OpenRouter models.")
	return p.models
}

// Complete generates a completion using the OpenRouter provider via gollm.
func (p *OpenRouterProvider) Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error) {
	if p.gollmInstance == nil {
		return CompletionResponse{}, errors.New("openrouter provider (gollm instance) not initialized")
	}

	modelID := request.Model
	if modelID == "" {
		modelID = p.defaultModel
		Logger.Debug().Str("model", modelID).Msg("No model specified in request, using OpenRouterProvider default model.")
	}

	// Set options on the gollmInstance for this specific call.
	// gollm's OpenRouterProvider reads these from the options map passed to PrepareRequest.
	// The gollm.LLMImpl passes its l.Options to the provider.
	p.gollmInstance.SetOption("model", modelID) // This is crucial for gollm's OpenRouterProvider

	if request.MaxTokens > 0 {
		p.gollmInstance.SetOption("max_tokens", request.MaxTokens)
	}
	if request.Temperature >= 0 { // Assuming 0 is a valid explicit temperature
		p.gollmInstance.SetOption("temperature", request.Temperature)
	}
	if request.TopP > 0 {
		p.gollmInstance.SetOption("top_p", request.TopP)
	}
	if request.N > 0 {
		p.gollmInstance.SetOption("n", int(request.N))
	}
	if request.FrequencyPenalty != 0.0 {
		p.gollmInstance.SetOption("frequency_penalty", request.FrequencyPenalty)
	}
	if request.PresencePenalty != 0.0 {
		p.gollmInstance.SetOption("presence_penalty", request.PresencePenalty)
	}
	if request.Seed != 0 {
		p.gollmInstance.SetOption("seed", int(request.Seed))
	}
	if len(request.StopSequences) > 0 {
		p.gollmInstance.SetOption("stop", request.StopSequences)
	}
	// User field is not directly supported by gollm's SetOption in a generic way for OpenRouter payload.
	// It might be part of OpenRouter's specific headers or request body structure that gollm handles.

	var promptOpts []gollm_llm.PromptOption
	if request.SystemPrompt != "" {
		// gollm's OpenRouterProvider uses "system_message" in options for system prompt
		p.gollmInstance.SetOption("system_message", request.SystemPrompt)
		// Or, if gollm.WithSystemPrompt is preferred and gollm's OpenRouter handles it:
		// promptOpts = append(promptOpts, gollm_llm.WithSystemPrompt(request.SystemPrompt, gollm_llm.CacheTypeEphemeral))
	}

	gollmPrompt := gollm.NewPrompt(request.Prompt, promptOpts...)

	if request.Stream {
		// For streaming, gollm's SetOption("stream", true) is usually how it's enabled.
		p.gollmInstance.SetOption("stream", true)
		if request.IncludeUsage {
			// OpenRouter streaming doesn't have a direct "include_usage" like OpenAI.
			// Usage might come in headers or a final event, which gollm might not expose.
			Logger.Debug().Msg("OpenRouter streaming usage statistics might not be available via gollm's stream interface.")
		}

		Logger.Debug().Str("model", modelID).Msg("Requesting streaming completion from OpenRouter via gollm.")
		stream, err := p.gollmInstance.Stream(ctx, gollmPrompt)
		if err != nil {
			return CompletionResponse{}, fmt.Errorf("gollm stream start failed for OpenRouter: %w", err)
		}
		defer stream.Close()

		var fullText strings.Builder
		for {
			token, err := stream.Next(ctx)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return CompletionResponse{}, fmt.Errorf("gollm stream error for OpenRouter: %w", err)
			}
			if token != nil {
				fullText.WriteString(token.Text)
			}
		}

		// FinishReason and TokenUsage are not directly available from gollm.TokenStream
		Logger.Debug().Msg("Streaming completed. FinishReason and TokenUsage are not available from gollm's stream interface for OpenRouter.")
		return CompletionResponse{
			Text:     fullText.String(),
			Model:    modelID,
			Provider: p.GetName(),
			// FinishReason: "", // Not available from gollm.StreamToken
			// Usage: TokenUsage{}, // Not available
		}, nil

	} else {
		p.gollmInstance.SetOption("stream", false) // Ensure stream is false for non-streaming
		Logger.Debug().Str("model", modelID).Msg("Requesting non-streaming completion from OpenRouter via gollm.")
		
		responseText, err := p.gollmInstance.Generate(ctx, gollmPrompt)
		if err != nil {
			return CompletionResponse{}, fmt.Errorf("gollm generation failed for OpenRouter: %w", err)
		}

		// FinishReason and TokenUsage are not directly available from gollm.Generate
		// The underlying gollm OpenRouterProvider *does* parse ID and Model from response.
		Logger.Debug().Msg("Non-streaming completed. FinishReason and TokenUsage are not available from gollm.Generate interface for OpenRouter.")
		return CompletionResponse{
			Text:     responseText,
			Model:    modelID,
			Provider: p.GetName(),
			// FinishReason: "", // Not available
			// Usage: TokenUsage{}, // Not available
		}, nil
	}
}
