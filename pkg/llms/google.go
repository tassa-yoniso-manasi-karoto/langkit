package llms

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"google.golang.org/api/iterator"
	"google.golang.org/genai"
)

// GeminiProvider implements the Provider interface for Google Gemini
type GeminiProvider struct {
	client *genai.Client // Official Google Gen AI Go client
	apiKey string
	models []ModelInfo // Cached list of available models
}

// NewGeminiProvider creates a new Google Gemini provider with the given API key.
// Currently focuses on BackendGeminiAPI.
func NewGeminiProvider(apiKey string) *GeminiProvider {
	if apiKey == "" {
		if Logger.Debug().Enabled() {
			Logger.Debug().Msg("Empty API key provided to Gemini provider, cannot initialize.")
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Timeout for client creation
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI, // Using Gemini API backend
	})

	if err != nil {
		Logger.Error().Err(err).Msg("Failed to create Google Gen AI client")
		return nil
	}

	provider := &GeminiProvider{
		client: client,
		apiKey: apiKey,
	}
	return provider
}

// GetName returns the provider's name
func (p *GeminiProvider) GetName() string {
	return "google-gemini"
}

// GetDescription returns the provider's description
func (p *GeminiProvider) GetDescription() string {
	return "Google Gemini API for models like Gemini Pro and Flash"
}

// RequiresAPIKey indicates if the provider needs an API key
func (p *GeminiProvider) RequiresAPIKey() bool {
	return true
}

// GetAvailableModels returns the list of available models that support "generateContent".
// It fetches from the API and caches the result.
func (p *GeminiProvider) GetAvailableModels() []ModelInfo {
	if p.client == nil {
		Logger.Warn().Msg("Gemini client not initialized in GetAvailableModels")
		return nil
	}

	if len(p.models) > 0 {
		return p.models // Return cached models
	}

	Logger.Debug().Msg("Fetching available models from Google Gemini API...")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Longer timeout for model listing
	defer cancel()

	var applicableModelInfos []ModelInfo // Store only models suitable for content generation
	iter := p.client.Models.All(ctx)     // Uses an iterator that handles pagination
	for {
		model, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			Logger.Error().Err(err).Msg("Failed to iterate over Gemini models")
			return nil // Or return partially fetched models if preferred
		}

		// Filter for models that support "generateContent"
		supportsGenerateContent := false
		rawCapabilities := []string{} // Store all supported actions for potential broader use
		for _, action := range model.SupportedActions {
			rawCapabilities = append(rawCapabilities, action)
			if action == "generateContent" {
				supportsGenerateContent = true
			}
		}

		if !supportsGenerateContent {
			Logger.Debug().Str("model", model.Name).Msg("Skipping model as it does not support 'generateContent'")
			continue // Skip models not suitable for text generation
		}

		// Infer capabilities for llms.ModelInfo based on generateContent support
		capabilities := []string{"chat", "text-generation"}
		// Could add more specific capabilities if inferable from model name or other SupportedActions

		applicableModelInfos = append(applicableModelInfos, ModelInfo{
			ID:           model.Name,
			Name:         model.DisplayName,
			Description:  model.Description,
			MaxTokens:    int(model.InputTokenLimit), // Using InputTokenLimit as context window size
			Capabilities: capabilities,
			ProviderName: p.GetName(),
		})
	}

	p.models = applicableModelInfos
	Logger.Debug().Int("count", len(p.models)).Msg("Successfully fetched and cached applicable Google Gemini models.")
	return p.models
}

// Complete generates a completion from the prompt
func (p *GeminiProvider) Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error) {
	if p.client == nil {
		return CompletionResponse{}, errors.New("gemini client not initialized")
	}
	if p.apiKey == "" {
		return CompletionResponse{}, errors.New("gemini provider not initialized: missing API key")
	}

	if request.Prompt == "" {
		return CompletionResponse{}, fmt.Errorf("%w: prompt cannot be empty for Gemini", ErrInvalidRequest)
	}

	modelID := request.Model
	if modelID == "" {
		// Default to a capable and recent model that supports generateContent
		// Ensure this default is one that would pass the GetAvailableModels filter
		modelID = "models/gemini-2.5-flash-latest"
		Logger.Debug().Str("model", modelID).Msg("No model specified in request, using default Gemini model.")
	}

	// Construct content parts
	parts := []*genai.Part{genai.NewPartFromText(request.Prompt)}
	contents := []*genai.Content{{Parts: parts, Role: genai.RoleUser}}

	// Prepare GenerationConfig
	genConfig := &genai.GenerateContentConfig{}
	var systemInstruction *genai.Content

	if request.SystemPrompt != "" {
		systemInstruction = genai.NewContentFromText(request.SystemPrompt, genai.RoleModel)
		genConfig.SystemInstruction = systemInstruction
	}

	if request.MaxTokens > 0 {
		genConfig.MaxOutputTokens = genai.Ptr(int32(request.MaxTokens))
	}
	if request.Temperature >= 0 {
		genConfig.Temperature = genai.Ptr(float32(request.Temperature))
	}
	if request.TopP > 0 {
		genConfig.TopP = genai.Ptr(float32(request.TopP))
	}
	if request.N > 0 {
		genConfig.CandidateCount = genai.Ptr(int32(request.N))
	}
	if request.Seed != 0 {
		genConfig.Seed = genai.Ptr(int32(request.Seed))
	}
	if request.PresencePenalty != 0.0 {
		genConfig.PresencePenalty = genai.Ptr(float32(request.PresencePenalty))
	}
	if request.FrequencyPenalty != 0.0 {
		genConfig.FrequencyPenalty = genai.Ptr(float32(request.FrequencyPenalty))
	}

	if len(request.StopSequences) > 0 {
		genConfig.StopSequences = request.StopSequences
	}

	if request.Stream {
		Logger.Debug().Str("model", modelID).Interface("config", genConfig).Msg("Requesting streaming content generation from Gemini.")
		stream := p.client.Models.GenerateContentStream(ctx, modelID, contents, genConfig)
		// defer stream.Close() // genai.GenerateContentStream does not return a struct with Close()

		var fullText strings.Builder
		var finalResponse CompletionResponse
		var lastResp *genai.GenerateContentResponse

		for {
			resp, err := stream.Next() // stream is iter.Seq2[*GenerateContentResponse, error]
			if errors.Is(err, iterator.Done) { // Corrected end-of-stream check
				Logger.Debug().Msg("Gemini stream finished.")
				break
			}
			if err != nil {
				var genaiErr *genai.APIError
				if errors.As(err, &genaiErr) {
					Logger.Error().Int("code", genaiErr.Code).Str("status", genaiErr.Status).Msg("Gemini API error during streaming")
				}
				return CompletionResponse{}, fmt.Errorf("gemini stream error: %w", err)
			}
			lastResp = resp
			if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
				for _, part := range resp.Candidates[0].Content.Parts {
					if textPart, ok := part.Text(); ok {
						fullText.WriteString(textPart)
					}
				}
			}
		}

		finalResponse.Text = fullText.String()
		if lastResp != nil && len(lastResp.Candidates) > 0 {
			finalResponse.FinishReason = string(lastResp.Candidates[0].FinishReason)
		}
		finalResponse.Model = modelID
		finalResponse.Provider = p.GetName()

		if lastResp != nil && lastResp.UsageMetadata != nil {
			finalResponse.Usage = TokenUsage{
				PromptTokens:     int(lastResp.UsageMetadata.PromptTokenCount),
				CompletionTokens: int(lastResp.UsageMetadata.CandidatesTokenCount),
				TotalTokens:      int(lastResp.UsageMetadata.TotalTokenCount),
			}
		} else {
			Logger.Debug().Msg("Usage data not present in the final Gemini stream response.")
		}
		return finalResponse, nil

	} else {
		Logger.Debug().Str("model", modelID).Interface("config", genConfig).Msg("Requesting non-streaming content generation from Gemini.")
		resp, err := p.client.Models.GenerateContent(ctx, modelID, contents, genConfig)
		if err != nil {
			var genaiErr *genai.APIError
			if errors.As(err, &genaiErr) {
				Logger.Error().Int("code", genaiErr.Code).Str("status", genaiErr.Status).Msg("Gemini API error")
			}
			return CompletionResponse{}, fmt.Errorf("gemini content generation failed: %w", err)
		}

		if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			return CompletionResponse{}, errors.New("no content returned from Gemini completion")
		}

		var responseTextBuilder strings.Builder
		for _, part := range resp.Candidates[0].Content.Parts {
			if textPart, ok := part.Text(); ok {
				responseTextBuilder.WriteString(textPart)
			}
		}

		var usage TokenUsage
		if resp.UsageMetadata != nil {
			usage.PromptTokens = int(resp.UsageMetadata.PromptTokenCount)
			usage.CompletionTokens = int(resp.UsageMetadata.CandidatesTokenCount)
			usage.TotalTokens = int(resp.UsageMetadata.TotalTokenCount)
		}

		return CompletionResponse{
			Text:         responseTextBuilder.String(),
			FinishReason: string(resp.Candidates[0].FinishReason),
			Usage:        usage,
			Model:        modelID,
			Provider:     p.GetName(),
		}, nil
	}
}
