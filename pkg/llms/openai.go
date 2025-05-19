package llms

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	client   *openai.Client // Official OpenAI Go client
	apiKey   string
	models   []ModelInfo // Cached list of available models
	modelsMu sync.RWMutex
}

// NewOpenAIProvider creates a new OpenAI provider with the given API key
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	if apiKey == "" {
		if Logger.Debug().Enabled() {
			Logger.Debug().Msg("Empty API key provided to OpenAI provider, cannot initialize.")
		}
		return nil
	}

	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithHTTPClient(httpClient),
	)

	provider := &OpenAIProvider{
		client: &client,
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
	return "OpenAI API for modern GPT and O-series models."
}

// RequiresAPIKey indicates if the provider needs an API key
func (p *OpenAIProvider) RequiresAPIKey() bool {
	return true
}

var datePatternRegex = regexp.MustCompile(`\d{4}[-_]\d{2}[-_]\d{2}`)

// isAllowedOpenAIModel applies strict filtering for current and future text/chat models.
func isAllowedOpenAIModel(modelID string) bool {
	idLower := strings.ToLower(modelID)

	// 1. Exclude models containing a date snapshot pattern
	if datePatternRegex.MatchString(idLower) {
		return false
	}

	// 2. Define prefixes for models we generally want to consider (current and anticipated future)
	// These are for the "latest" or non-dated versions.
	allowedSeriesPrefixes := []string{
		"gpt-4o",  // Includes gpt-4o, gpt-4o-mini (but specific exclusions below will catch sub-types)
		"gpt-4.1", // Includes gpt-4.1, gpt-4.1-mini, gpt-4.1-nano
		"o1",      // Includes o1, o1-mini, o1-pro
		"o3",      // Includes o3, o3-mini
		"o4-mini", // Specific allowed model
		"o5",      // Future o-series (generic)
		"gpt-5",   // Future GPT-series (generic)
		"chatgpt-4o-latest", // An alias
	}

	// 3. Define substrings or exact IDs for model types/functionalities to explicitly EXCLUDE
	// This helps filter out specialized versions of allowed series.
	strictlyExcludedSubstrings := []string{
		"embedding",    // All embedding models
		"dall-e",       // All DALL-E models
		"tts",          // All Text-to-Speech models
		"whisper",      // All Whisper (transcription) models
		"transcribe",   // Models specifically for transcription (e.g., gpt-4o-transcribe)
		"moderation",   // All moderation models
		"search",       // Tool-specific models like gpt-4o-search-preview
		"computer-use", // Tool-specific models
		"codex",        // Code-specific, might be too specialized
		"audio",        // General audio models like gpt-4o-audio-preview (unless it's a primary model like gpt-4o itself)
		"realtime",     // Realtime-focused previews
		"instruct",     // Older instruct models
		// Explicitly exclude all gpt-3.5 and base gpt-4 variants by not including them in allowedSeriesPrefixes
		// and ensuring specific older versions are caught if any broader rule accidentally includes them.
		"gpt-3.5-turbo",
		"gpt-4-turbo",         // Exclude base GPT-4 Turbo (dated snapshots are already excluded by regex)
		"gpt-4-preview",       // Exclude generic GPT-4 previews
		"gpt-4-0",             // Catches gpt-4-0125-preview, gpt-4-0314, gpt-4-0613
		"gpt-4-32k",           // Exclude all gpt-4-32k variants
		"babbage-002",
		"davinci-002",
		"curie",
		"ada",
		"o1-mini-2024-09-12", // Specifically deprecated o1-mini
	}

	// Check for strict exclusions first
	for _, sub := range strictlyExcludedSubstrings {
		if strings.Contains(idLower, sub) {
			// Special case: allow "gpt-4o" and "gpt-4o-mini" even if they contain "audio" if that's part of their primary name
			// and not a specific "audio-preview" variant.
			// However, "gpt-4o-audio-preview" should be excluded.
			// The current logic: "audio" in strictlyExcludedSubstrings will catch "gpt-4o-audio-preview".
			// "gpt-4o" itself will pass this exclusion. This seems fine.
			return false
		}
	}

	// Check if the model starts with one of the allowed series prefixes
	for _, prefix := range allowedSeriesPrefixes {
		if strings.HasPrefix(idLower, prefix) {
			return true // If it matches an allowed series and wasn't strictly excluded, it's allowed.
		}
	}

	// If it didn't match any allowed series prefix after passing exclusions, it's not allowed.
	return false
}

// GetAvailableModels returns the list of allowed text generation/chat models,
// sorted by release date (most recent first).
func (p *OpenAIProvider) GetAvailableModels() []ModelInfo {
	p.modelsMu.RLock()
	if len(p.models) > 0 {
		p.modelsMu.RUnlock()
		return p.models
	}
	p.modelsMu.RUnlock()

	p.modelsMu.Lock()
	defer p.modelsMu.Unlock()
	if len(p.models) > 0 { // Double check after acquiring write lock
		return p.models
	}

	if p.client == nil {
		Logger.Warn().Msg("OpenAI client not initialized in GetAvailableModels")
		return nil
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
		if !isAllowedOpenAIModel(model.ID) {
			Logger.Trace().Str("model_id", model.ID).Msg("Skipping disallowed OpenAI model")
			continue
		}

		releaseDate := time.Unix(model.Created, 0)
		capabilities := []string{"chat", "text-generation"}
		modelIDLower := strings.ToLower(model.ID)

		// Add vision capability for known vision-enabled series
		if strings.Contains(modelIDLower, "gpt-4o") || strings.Contains(modelIDLower, "gpt-4.1") || strings.HasPrefix(modelIDLower, "gpt-5") { // Assuming gpt-5 will have vision
			capabilities = append(capabilities, "vision")
		}
		// Note: `MaxTokens` in `llms.ModelInfo` refers to context window.
		// The OpenAI API's `/models` endpoint does not provide context window size directly.
		// We will set it to 0, indicating the information is not available from this endpoint.
		// The actual context limit will be enforced by the API during the call.
		// The `max_completion_tokens` or `max_tokens` parameter in the request controls output length.
		modelInfos = append(modelInfos, ModelInfo{
			ID:            model.ID,
			Name:          model.ID,
			Description:   fmt.Sprintf("Owned by: %s. API Registered: %s", model.OwnedBy, releaseDate.Format("Jan 2006")),
			MaxTokens:     0, // Indicate unknown/API-enforced context window from this listing
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
	Logger.Debug().Int("count", len(p.models)).Msg("Successfully fetched, filtered, and cached allowed OpenAI models.")
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
		modelID = string(openai.ChatModelGPT4o) // Default to GPT-4o
		Logger.Debug().Str("model", modelID).Msg("No model specified in request, using default OpenAI model.")
	}

	chatReqParams := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(modelID),
		Messages: messages,
	}

	// Only set parameters if they are meaningfully provided by llms.CompletionRequest
	if request.MaxTokens > 0 { // MaxTokens for output generation
		chatReqParams.MaxTokens = openai.Int(int64(request.MaxTokens))
	}
	if request.Temperature >= 0 && request.Temperature <= 2 {
		chatReqParams.Temperature = openai.Float(request.Temperature)
	}
	if request.TopP > 0 && request.TopP <= 1 { // OpenAI range is 0 to 1, typically >0
		chatReqParams.TopP = openai.Float(request.TopP)
	}
	if request.N > 0 {
		chatReqParams.N = openai.Int(int64(request.N))
	}
	// OpenAI penalties range from -2.0 to 2.0. 0 is the neutral default.
	if request.FrequencyPenalty != 0.0 {
		chatReqParams.FrequencyPenalty = openai.Float(request.FrequencyPenalty)
	}
	if request.PresencePenalty != 0.0 {
		chatReqParams.PresencePenalty = openai.Float(request.PresencePenalty)
	}
	if request.Seed != 0 { // Assuming 0 means "not set" or "let API decide"
		chatReqParams.Seed = openai.Int(int64(request.Seed))
	}

	if len(request.StopSequences) > 0 {
		if len(request.StopSequences) == 1 {
			chatReqParams.Stop.OfString = openai.String(request.StopSequences[0])
		} else {
			var stopArray []string
			for _, s := range request.StopSequences {
				stopArray = append(stopArray, s)
			}
			chatReqParams.Stop.OfChatCompletionNewsStopArray = stopArray
		}
	}
	if request.User != "" {
		chatReqParams.User = openai.String(request.User)
	}

	if request.Stream {
		if request.IncludeUsage {
			chatReqParams.StreamOptions = openai.ChatCompletionStreamOptionsParam{
				IncludeUsage: openai.Bool(true),
			}
		}
		Logger.Debug().Str("model", modelID).Msg("Requesting streaming chat completion from OpenAI.")
		stream := p.client.Chat.Completions.NewStreaming(ctx, chatReqParams)
		defer stream.Close()

		var fullText strings.Builder
		var finalResponse CompletionResponse
		var lastChunk openai.ChatCompletionChunk

		for stream.Next() {
			chunk := stream.Current()
			lastChunk = chunk
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.JSON.Content.IsPresent() {
				fullText.WriteString(chunk.Choices[0].Delta.Content)
			}
		}
		
		if err := stream.Err(); err != nil {
			Logger.Error().Err(err).Msg("Error receiving stream chunk")
			return CompletionResponse{}, fmt.Errorf("stream error: %w", err)
		}
		
		Logger.Debug().Msg("Stream finished.")

		finalResponse.Text = fullText.String()
		if len(lastChunk.Choices) > 0 {
			finalResponse.FinishReason = string(lastChunk.Choices[0].FinishReason)
		}
		finalResponse.Model = lastChunk.Model
		finalResponse.Provider = p.GetName()

		if lastChunk.JSON.Usage.IsPresent() {
			finalResponse.Usage = TokenUsage{
				PromptTokens:     int(lastChunk.Usage.PromptTokens),
				CompletionTokens: int(lastChunk.Usage.CompletionTokens),
				TotalTokens:      int(lastChunk.Usage.TotalTokens),
			}
		}
		return finalResponse, nil

	} else {
		Logger.Debug().Str("model", modelID).Msg("Requesting non-streaming chat completion from OpenAI.")
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
		if resp.JSON.Usage.IsPresent() {
			usage.PromptTokens = int(resp.Usage.PromptTokens)
			usage.CompletionTokens = int(resp.Usage.CompletionTokens)
			usage.TotalTokens = int(resp.Usage.TotalTokens)
		}

		return CompletionResponse{
			Text:         choice.Message.Content,
			FinishReason: string(choice.FinishReason),
			Usage:        usage,
			Model:        resp.Model,
			Provider:     p.GetName(),
		}, nil
	}
}