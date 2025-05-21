package llms

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"google.golang.org/genai"
)

// GoogleProvider implements the Provider interface for Google AI Platform
type GoogleProvider struct {
	client *genai.Client // Official Google Gen AI Go client
	apiKey string
	models []ModelInfo // Cached list of available models
}

// NewGoogleProvider creates a new Google AI provider with the given API key.
// Currently focuses on BackendGeminiAPI.
func NewGoogleProvider(apiKey string) *GoogleProvider {
	if apiKey == "" {
		if Logger.Debug().Enabled() {
			Logger.Debug().Msg("Empty API key provided to Google provider, cannot initialize.")
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

	provider := &GoogleProvider{
		client: client,
		apiKey: apiKey,
	}
	return provider
}

// GetName returns the provider's name
func (p *GoogleProvider) GetName() string {
	return "google"
}

// GetDescription returns the provider's description
func (p *GoogleProvider) GetDescription() string {
	return "Google AI Platform for models like Gemini Pro and Flash"
}

// RequiresAPIKey indicates if the provider needs an API key
func (p *GoogleProvider) RequiresAPIKey() bool {
	return true
}

// GetAvailableModels returns the list of available models that support "generateContent".
// It fetches from the API and caches the result.
func (p *GoogleProvider) GetAvailableModels(ctx context.Context) []ModelInfo {
	if p.client == nil {
		Logger.Warn().Msg("Google client not initialized in GetAvailableModels")
		return nil
	}

	if len(p.models) > 0 {
		return p.models // Return cached models
	}

	Logger.Debug().Msg("Fetching available models from Google AI API...")
	// Create a new context with timeout or use the provided one
	fetchCtx, cancel := context.WithTimeout(ctx, 60*time.Second) // Longer timeout for model listing
	defer cancel()

	var applicableModelInfos []ModelInfo // Store only models suitable for content generation
	
	// Use modern iteration with the iter.Seq2 from Gemini SDK
	modelsIterator := p.client.Models.All(fetchCtx)
	var err error
	
	// Collect models using the iterator function pattern
	modelsIterator(func(model *genai.Model, e error) bool {
		name := strings.TrimPrefix(model.Name, "models/")
		if e != nil {
			err = e
			return false
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
			Logger.Debug().Str("model", name).Msg("Skipping model as it does not support 'generateContent'")
			return true
		} else if isOutdatedGoogleModel(name) {
			Logger.Debug().Str("model", name).Msg("Skipping OUTDATED model")
			return true
		} else {
			Logger.Debug().Str("model", name).Msg("Registering model")
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
		
		return true // Continue iteration
	})
	
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to iterate over Google models")
		return nil
	}

	p.models = applicableModelInfos
	Logger.Debug().Int("count", len(p.models)).Msg("Successfully fetched and cached applicable Google AI models.")
	return p.models
}

// Complete generates a completion from the prompt
func (p *GoogleProvider) Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error) {
	if p.client == nil {
		return CompletionResponse{}, errors.New("google client not initialized")
	}
	if p.apiKey == "" {
		return CompletionResponse{}, errors.New("google provider not initialized: missing API key")
	}

	if request.Prompt == "" {
		return CompletionResponse{}, fmt.Errorf("%w: prompt cannot be empty for Google", ErrInvalidRequest)
	}

	modelID := request.Model
	if modelID == "" {
		// Default to a capable and recent model that supports generateContent
		// Ensure this default is one that would pass the GetAvailableModels filter
		modelID = "models/gemini-2.5-flash-latest"
		Logger.Debug().Str("model", modelID).Msg("No model specified in request, using default Google model.")
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

	// Set configuration options with proper type conversion
	if request.MaxTokens > 0 {
		genConfig.MaxOutputTokens = int32(request.MaxTokens)
	}
	if request.Temperature >= 0 {
		genConfig.Temperature = genai.Ptr(float32(request.Temperature))
	}
	if request.TopP > 0 {
		genConfig.TopP = genai.Ptr(float32(request.TopP))
	}
	if request.N > 0 {
		genConfig.CandidateCount = int32(request.N)
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
		Logger.Debug().Str("model", modelID).Msg("Requesting streaming content generation from Google.")
		streamIterator := p.client.Models.GenerateContentStream(ctx, modelID, contents, genConfig)

		var fullText strings.Builder
		var finalResponse CompletionResponse
		var lastResp *genai.GenerateContentResponse
		var streamErr error

		// Use the newer iter.Seq2 pattern with a yield function
		streamIterator(func(resp *genai.GenerateContentResponse, err error) bool {
			if err != nil {
				streamErr = err
				return false // Stop iteration
			}
			
			lastResp = resp
			if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
				for _, part := range resp.Candidates[0].Content.Parts {
					// Access the Text field directly since part is a struct with a Text field, not an interface
					// According to docs, *genai.Part has a Text string field
					if part.Text != "" {
						fullText.WriteString(part.Text)
					}
				}
			}
			return true // Continue iteration
		})

		if streamErr != nil && streamErr != io.EOF {
			var genaiErr *genai.APIError
			if errors.As(streamErr, &genaiErr) {
				Logger.Error().Int("code", genaiErr.Code).Str("status", genaiErr.Status).Msg("Google API error during streaming")
			}
			return CompletionResponse{}, fmt.Errorf("google stream error: %w", streamErr)
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
				TotalTokens:      int(lastResp.UsageMetadata.PromptTokenCount + lastResp.UsageMetadata.CandidatesTokenCount),
			}
		}

		return finalResponse, nil
	} else {
		Logger.Debug().Str("model", modelID).Msg("Requesting content generation from Google.")
		
		resp, err := p.client.Models.GenerateContent(ctx, modelID, contents, genConfig)
		if err != nil {
			var genaiErr *genai.APIError
			if errors.As(err, &genaiErr) {
				Logger.Error().Int("code", genaiErr.Code).Str("status", genaiErr.Status).Msg("Google API error")
			}
			return CompletionResponse{}, fmt.Errorf("google content generation: %w", err)
		}

		if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			return CompletionResponse{}, errors.New("google returned empty response content")
		}

		var fullText strings.Builder
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.Text != "" {
				fullText.WriteString(part.Text)
			}
		}

		response := CompletionResponse{
			Text:         fullText.String(),
			FinishReason: string(resp.Candidates[0].FinishReason),
			Model:        modelID,
			Provider:     p.GetName(),
		}

		if resp.UsageMetadata != nil {
			response.Usage = TokenUsage{
				PromptTokens:     int(resp.UsageMetadata.PromptTokenCount),
				CompletionTokens: int(resp.UsageMetadata.CandidatesTokenCount),
				TotalTokens:      int(resp.UsageMetadata.PromptTokenCount + resp.UsageMetadata.CandidatesTokenCount),
			}
		}

		return response, nil
	}
}

// isOutdatedGoogleModel returns true if the model is outdated or an older preview
func isOutdatedGoogleModel(name string) bool {
	// If name has a specific date embedded, it's a dated snapshot, which may become outdated
	if strings.Contains(name, "-20") { // e.g., gemini-1.0-pro-20231115
		return true
	}
	
	// Explicitly outdated model series
	outdatedModels := []string{
		"palm", // All PaLM models are outdated
		"text-bison",
		"embedding-gecko",
		"gemini-1.0", // First generation Gemini, now superseded
		"gemini-pro",
		"gemini-ultra", // Original naming
		"gemini-1.5-", // These are now replaced by 2.5 family
	}
	
	// Check if model name contains any of the outdated model strings
	for _, outdated := range outdatedModels {
		if strings.Contains(name, outdated) {
			return true
		}
	}
	
	// Include older "preview" variants but allow newer preview models
	if strings.Contains(name, "preview") && 
	   (strings.Contains(name, "gemini-1.0") || strings.Contains(name, "gemini-1.5")) {
		return true
	}
	
	// Special case checks for specific models
	// Add more specific checks here if needed
	
	// If no outdated patterns match, it's likely current
	return false
}

// Ptr returns a pointer to the provided value (for SDK options)
func Ptr[T any](v T) *T {
	return &v
}