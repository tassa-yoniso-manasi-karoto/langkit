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
	"github.com/rs/zerolog"
)

var AppName string

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
		Timeout: 30 * time.Second,
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
	return "OpenRouter: Access to multiple LLMs. Models sorted by weekly popularity."
}

// RequiresAPIKey indicates if the provider needs an API key
func (p *OpenRouterProvider) RequiresAPIKey() bool {
	return true
}

// Structs for parsing OpenRouter /models response (official endpoint)
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

type OpenRouterOfficialModelData struct {
	ID                  string                       `json:"id"`    // This is the ID to use for API calls (e.g., "openai/gpt-4o-mini")
	Name                string                       `json:"name"`  // This is the full display name (e.g., "OpenAI: GPT-4o-mini")
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

type OpenRouterOfficialModelsResponse struct {
	Data []OpenRouterOfficialModelData `json:"data"`
}

// Structs for parsing OpenRouter /frontend/models/find?order=top-weekly response
type OpenRouterPopularityModelData struct {
	Slug          string    `json:"slug"` // This is an ID, often matches official ID.
	Name          string    `json:"name"` // This is the full display name, used for matching.
	ContextLength int       `json:"context_length"`
	// ... other fields from the provided JSON, but we primarily need 'name' for ordering.
}

type OpenRouterPopularityResponseData struct {
	Models []OpenRouterPopularityModelData `json:"models"`
}

type OpenRouterPopularityResponse struct {
	Data OpenRouterPopularityResponseData `json:"data"`
}

// GetAvailableModels fetches models from the official OpenRouter endpoint,
// then attempts to fetch a popularity-ordered list from an alternative endpoint
// and re-sorts the official list accordingly using the 'Name' field for matching.
func (p *OpenRouterProvider) GetAvailableModels(ctx context.Context) []ModelInfo {
	if !p.isInitialized {
		Logger.Warn().Msg("OpenRouterProvider not initialized in GetAvailableModels")
		return nil
	}

	p.modelsMu.RLock()
	if len(p.models) > 0 {
		Logger.Trace().Msg("Returning cached OpenRouter models.")
		p.modelsMu.RUnlock()
		return p.models
	}
	p.modelsMu.RUnlock()

	p.modelsMu.Lock()
	defer p.modelsMu.Unlock()
	if len(p.models) > 0 {
		Logger.Trace().Msg("Returning cached OpenRouter models (double check).")
		return p.models
	}

	Logger.Debug().Msg("Fetching official model list from OpenRouter API (v1/models)...")
	officialAPIURL := "https://openrouter.ai/api/v1/models"
	officialModelsData, err := p.fetchOpenRouterEndpoint(officialAPIURL)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to fetch or parse official OpenRouter models list.")
		return nil // Critical failure if official list can't be fetched
	}

	var officialModelsResp OpenRouterOfficialModelsResponse
	if err := json.Unmarshal(officialModelsData, &officialModelsResp); err != nil {
		Logger.Error().Err(err).Str("url", officialAPIURL).Msg("Failed to unmarshal official OpenRouter models response")
		return nil
	}
	Logger.Trace().Int("official_model_count_raw", len(officialModelsResp.Data)).Msg("Fetched official models")

	// Fetch popularity-ordered list (names)
	popularModelNames := p.fetchPopularModelNames() // Helper function

	// Transform official models into llms.ModelInfo
	var modelInfos []ModelInfo
	officialModelNameMap := make(map[string]OpenRouterOfficialModelData) // For quick lookup by name

	for _, orModel := range officialModelsResp.Data {
		officialModelNameMap[orModel.Name] = orModel // Store by full name

		canGenerateText := false
		for _, outMod := range orModel.Architecture.OutputModalities {
			if strings.ToLower(outMod) == "text" {
				canGenerateText = true
				break
			}
		}
		if !canGenerateText {
			Logger.Trace().Str("model_id", orModel.ID).Str("model_name", orModel.Name).Msg("Skipping non-text generating model")
			continue
		}

		capabilities := []string{"text-generation", "chat"}
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
			ID:            orModel.ID,   // Use the official ID for API calls
			Name:          orModel.Name, // Use the official Name for display and matching popularity
			Description:   orModel.Description,
			MaxTokens:     contextLength,
			Capabilities:  uniqueStrings(capabilities),
			ProviderName:  p.GetName(),
			ReleaseDate:   releaseDate,
		})
	}
	Logger.Trace().Int("text_gen_model_count", len(modelInfos)).Msg("Transformed official models to ModelInfo")
	if Logger.GetLevel() <= zerolog.TraceLevel {
		var initialModelNames []string
		for _, mi := range modelInfos {
			initialModelNames = append(initialModelNames, mi.Name)
		}
		Logger.Trace().Strs("initial_model_names_preview", initialModelNames[:min(10, len(initialModelNames))]).Msg("Preview of ModelInfo names before popularity sort")
	}


	// Re-sort modelInfos based on popularity if available
	if len(popularModelNames) > 0 {
		popularityRank := make(map[string]int)
		for i, name := range popularModelNames {
			popularityRank[name] = i
		}
		Logger.Trace().Int("popular_model_name_count", len(popularModelNames)).Int("rank_map_size", len(popularityRank)).Msg("Popularity rank map created using model names")
		
		foundInPopular := 0
		notFoundInPopular := 0

		sort.SliceStable(modelInfos, func(i, j int) bool {
			nameI := modelInfos[i].Name
			nameJ := modelInfos[j].Name
			rankI, inPopI := popularityRank[nameI]
			rankJ, inPopJ := popularityRank[nameJ]

			if inPopI && inPopJ {
				return rankI < rankJ // Sort by popularity rank (lower index is better)
			}
			if inPopI { // Popular models always come before non-popular ones
				return true
			}
			if inPopJ { // Popular models always come before non-popular ones
				return false
			}
			// If neither is in the popularity list, maintain their original relative order.
			// SliceStable preserves this.
			return false
		})

		// Log how many models from the official list were found in the popular list
		for _, mi := range modelInfos {
			if _, ok := popularityRank[mi.Name]; ok {
				foundInPopular++
			} else {
				notFoundInPopular++
			}
		}
		Logger.Trace().Int("found_in_popular_list", foundInPopular).Int("not_found_in_popular_list", notFoundInPopular).Msg("Matching official models against popular list")
		Logger.Debug().Msg("Sorted OpenRouter models by weekly popularity (using Name for matching).")

	} else {
		Logger.Warn().Msg("Popularity list for OpenRouter models was empty or failed to fetch. Falling back to sorting by release date only.")
		sort.SliceStable(modelInfos, func(i, j int) bool {
			if !modelInfos[i].ReleaseDate.Equal(modelInfos[j].ReleaseDate) {
				return modelInfos[i].ReleaseDate.After(modelInfos[j].ReleaseDate)
			}
			return modelInfos[i].ID < modelInfos[j].ID // Use ID as secondary sort for release date
		})
		Logger.Debug().Msg("Sorted OpenRouter models by release date.")
	}
	
	if Logger.GetLevel() <= zerolog.TraceLevel {
		sortedNames := make([]string, 0, len(modelInfos))
		for _, mi := range modelInfos {
			sortedNames = append(sortedNames, mi.Name + " (ID: " + mi.ID + ")")
		}
		Logger.Trace().Strs("final_sorted_model_names_preview", sortedNames[:min(10, len(sortedNames))]).Msg("Preview of final sorted model names and IDs")
	}

	p.models = modelInfos
	Logger.Debug().Int("final_model_count", len(p.models)).Msg("Successfully fetched, filtered, and cached OpenRouter models.")
	return p.models
}

// fetchOpenRouterEndpoint is a helper to fetch and read body from an OpenRouter endpoint.
func (p *OpenRouterProvider) fetchOpenRouterEndpoint(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", url, err)
	}
	// No auth needed for public model listings typically

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetching %s returned status %d: %s", url, resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body from %s: %w", url, err)
	}
	return body, nil
}


// fetchPopularModelNames fetches the list of model names ordered by popularity.
func (p *OpenRouterProvider) fetchPopularModelNames() []string {
	Logger.Trace().Msg("Fetching popular model order from OpenRouter frontend API (find?order=top-weekly)...")
	popularityAPIURL := "https://openrouter.ai/api/frontend/models/find?order=top-weekly"
	
	bodyPopular, err := p.fetchOpenRouterEndpoint(popularityAPIURL)
	if err != nil {
		Logger.Warn().Err(err).Msg("Failed to fetch popular model list from frontend API.")
		return nil
	}

	var popularModelsResp OpenRouterPopularityResponse
	if err := json.Unmarshal(bodyPopular, &popularModelsResp); err != nil {
		Logger.Error().Err(err).Str("url", popularityAPIURL).Str("body_snippet", string(bodyPopular[:min(200, len(bodyPopular))])).Msg("Failed to unmarshal popular OpenRouter models response")
		return nil
	}

	var popularNames []string
	if popularModelsResp.Data.Models != nil {
		for _, model := range popularModelsResp.Data.Models {
			popularNames = append(popularNames, model.Name) // Use 'Name' for matching
		}
	}
	Logger.Trace().Int("popular_model_name_count", len(popularNames)).Msg("Successfully fetched popular model names.")
	return popularNames
}


type StreamDelta struct {
	Content string `json:"content,omitempty"`
	Role    string `json:"role,omitempty"`
}

// OpenRouterStreamChoice is used for unmarshalling choices from OpenRouter SSE stream
type OpenRouterStreamChoice struct {
	Index        int                       `json:"index"`
	Delta        StreamDelta               `json:"delta"`
	FinishReason openrouter.FinishReason   `json:"finish_reason,omitempty"`
	LogProbs     *openrouter.LogProbs      `json:"logprobs,omitempty"`
}

// OpenRouterStreamChunk is used for unmarshalling data events from OpenRouter SSE stream
type OpenRouterStreamChunk struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Choices []OpenRouterStreamChoice `json:"choices"`
	Usage   *openrouter.Usage        `json:"usage,omitempty"`
}

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
		httpReq.Header.Set("HTTP-Referer", AppName) 
		httpReq.Header.Set("X-Title", AppName) 


		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			return CompletionResponse{}, fmt.Errorf("OpenRouter stream HTTP request failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			Logger.Error().Int("status_code", resp.StatusCode).Str("body", string(bodyBytes)).Msg("OpenRouter stream request failed: non-200 status")
			var apiErr openrouter.APIError
			if json.Unmarshal(bodyBytes, &apiErr) == nil {
				return CompletionResponse{}, &apiErr
			}
			return CompletionResponse{}, fmt.Errorf("OpenRouter stream API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
		}

		sseDecoder := ssestream.NewDecoder(resp)
		stream := ssestream.NewStream[OpenRouterStreamChunk](sseDecoder, nil)
		defer stream.Close()

		var fullText strings.Builder
		var finalResponse CompletionResponse
		var lastChunk OpenRouterStreamChunk

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

// min helper for slicing log previews
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}