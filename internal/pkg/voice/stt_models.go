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
	
	IsDepreciated, IsRecommended, TakesInitialPrompt bool
	IsAvailable bool
}

// Add a new function to get all models regardless of API key status
func GetAllSTTModels() []STTModelInfo {
    allModels := []STTModelInfo{
        {
            Name:               "gpt-4o-transcribe",
            DisplayName:        "GPT-4o Transcribe",
            Description:        "OpenAI's premium transcription model with very high accuracy",
            ProviderName:       "openai",
            IsRecommended:      true,
        },
        {
            Name:               "gpt-4o-mini-transcribe",
            DisplayName:        "GPT-4o Mini Transcribe",
            Description:        "Lightweight and cost-effective version of GPT-4o transcription. Outperforms Whisper V3 Large in most languages",
            ProviderName:       "openai",
            IsRecommended:      true,
        },
        {
            Name:               "scribe",
            DisplayName:        "Scribe",
            Description:        "ElevenLabs' recently released speech-to-text model",
            ProviderName:       "elevenlabs",
            IsRecommended:      true,
        },
        {
            Name:               "whisper",
            DisplayName:        "Whisper V3 Large",
            Description:        "OpenAI's open source, high-accuracy speech recognition model with broad language support",
            ProviderName:       "replicate",
            IsDepreciated:      true,
            TakesInitialPrompt: true,
        },
        {
            Name:               "incredibly-fast-whisper",
            DisplayName:        "Incredibly Fast Whisper",
            Description:        "Community-made optimized version of Whisper with faster processing time",
            ProviderName:       "replicate",
            IsDepreciated:      true,
            TakesInitialPrompt: false,
        },
    }
    
    // Check API key availability for each model
    for i := range allModels {
        provider, err := GetSpeechToTextProvider(allModels[i].Name)
        if err == nil {
            allModels[i].IsAvailable = provider.IsAvailable()
        } else {
            allModels[i].IsAvailable = false
        }
    }
    
    return allModels
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

// NewSpeechToTextRegistry creates a new registry with all providers
func NewSpeechToTextRegistry() *SpeechToTextRegistry {
	registry := &SpeechToTextRegistry{
		providers: make(map[string]SpeechToTextProvider),
	}
	
	allModels := GetAllSTTModels()
	
	// Pre-load all providers to avoid repeated instantiation
	for _, model := range allModels {
		provider, err := GetSpeechToTextProvider(model.Name)
		if err == nil {
			registry.providers[model.Name] = provider
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
