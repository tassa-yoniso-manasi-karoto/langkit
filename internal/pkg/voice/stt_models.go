package voice

import (
	"context"
	"fmt"
)

// DefaultSpeechToTextRegistry is a global instance for convenience
var DefaultSpeechToTextRegistry = NewSpeechToTextRegistry()

// STTModelInfo contains information about a speech-to-text model
type STTModelInfo struct {
	// Name is the unique identifier for the model
	Name string
	
	// DisplayName is a user-friendly name for the model
	DisplayName string
	
	// Description provides details about the model's capabilities and use cases
	Description string
	
	// ProviderName identifies which provider supplies this model
	ProviderName string
	
	IsDepreciated, IsRecommended bool
}

// GetAvailableSTTModels returns a slice of all available speech-to-text models
// The function checks if models have valid API keys before including them
func GetAvailableSTTModels() []STTModelInfo {
	// Define all supported models with their metadata
	allModels := []STTModelInfo{
		{
			Name:        "gpt-4o-transcribe",
			DisplayName: "GPT-4o Transcribe",
			Description: "OpenAI's premium transcription model with high accuracy",
			ProviderName: "openai",
			IsRecommended: true,
		},
		{
			Name:        "gpt-4o-mini-transcribe",
			DisplayName: "GPT-4o Mini Transcribe",
			Description: "Lightweight and cost-effective version of GPT-4o transcription",
			ProviderName: "openai",
			IsRecommended: true,
		},
		{
			Name:        "scribe",
			DisplayName: "Scribe",
			Description: "ElevenLabs' specialized speech-to-text model",
			ProviderName: "elevenlabs",
			IsRecommended: true,
		},
		{
			Name:        "whisper",
			DisplayName: "OpenAI Whisper V3 Large",
			Description: "High-accuracy speech recognition model with broad language support",
			ProviderName: "replicate",
			IsDepreciated: true,
		},
		{
			Name:        "incredibly-fast-whisper",
			DisplayName: "Incredibly Fast Whisper",
			Description: "Optimized version of Whisper with faster processing time",
			ProviderName: "replicate",
			IsDepreciated: true,
		},
		{
			Name:        "universal-1",
			DisplayName: "Universal-1",
			Description: "AssemblyAI's speech recognition model with strong performance across languages",
			ProviderName: "assemblyai",
		},
	}

	// Filter models to only include those with available API keys
	availableModels := []STTModelInfo{}
	for _, model := range allModels {
		provider, err := GetSpeechToTextProvider(model.Name)
		if err == nil && provider.IsAvailable() {
			availableModels = append(availableModels, model)
		}
	}

	return availableModels
}

// GetAvailableSTTModelNames returns a slice of names of all available speech-to-text models
func GetAvailableSTTModelNames() []string {
	models := GetAvailableSTTModels()
	names := make([]string, len(models))
	for i, model := range models {
		names[i] = model.Name
	}
	return names
}

// TranscribeAudioWithModel is a convenience function that gets the specified model and transcribes the audio
func TranscribeAudioWithModel(ctx context.Context, modelName, audioFile, language, initialPrompt string, maxTry, timeout int) (string, error) {
	// Get the provider for the specified model (already handles aliases)
	provider, err := GetSpeechToTextProvider(modelName)
	if err != nil {
		return "", fmt.Errorf("failed to get STT model '%s': %w", modelName, err)
	}

	// Use the provider to transcribe the audio
	return provider.TranscribeAudio(ctx, audioFile, language, initialPrompt, maxTry, timeout)
}

// IsSTTModelAvailable checks if a specific STT model is available
func IsSTTModelAvailable(modelName string) bool {
	provider, err := GetSpeechToTextProvider(modelName)
	return err == nil && provider.IsAvailable()
}

// SpeechToTextRegistry provides central access to STT model capabilities
type SpeechToTextRegistry struct {
	// providers is a map from model name to provider
	providers map[string]SpeechToTextProvider
}

// NewSpeechToTextRegistry creates a new registry with all available providers
func NewSpeechToTextRegistry() *SpeechToTextRegistry {
	registry := &SpeechToTextRegistry{
		providers: make(map[string]SpeechToTextProvider),
	}
	
	// Pre-load all available providers to avoid repeated instantiation
	modelNames := GetAvailableSTTModelNames()
	for _, name := range modelNames {
		provider, err := GetSpeechToTextProvider(name)
		if err == nil {
			registry.providers[name] = provider
		}
	}
	
	return registry
}

// GetProvider returns the provider for the given model name
func (r *SpeechToTextRegistry) GetProvider(modelName string) (SpeechToTextProvider, bool) {
	provider, exists := r.providers[modelName]
	return provider, exists
}

// Transcribe is a convenience method that uses the specified model to transcribe audio
func (r *SpeechToTextRegistry) Transcribe(ctx context.Context, modelName, audioFile, language, initialPrompt string, maxTry, timeout int) (string, error) {
	provider, exists := r.GetProvider(modelName)
	if !exists {
		// Try to get it on demand if not pre-loaded
		var err error
		provider, err = GetSpeechToTextProvider(modelName)
		if err != nil {
			return "", err
		}
	}
	
	return provider.TranscribeAudio(ctx, audioFile, language, initialPrompt, maxTry, timeout)
}
