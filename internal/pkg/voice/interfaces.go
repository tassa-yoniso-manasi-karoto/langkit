package voice

import (
	"context"
)

// AIServiceProvider is a common interface for all external AI service providers
type AIServiceProvider interface {
	// GetName returns the name of the provider
	GetName() string
	// IsAvailable checks if the provider is available with valid API keys
	IsAvailable() bool
}

// SpeechToTextProvider provides speech-to-text functionality
type SpeechToTextProvider interface {
	AIServiceProvider
	// TranscribeAudio converts audio to text
	TranscribeAudio(ctx context.Context, audioFile, language, initialPrompt string, maxTry, timeout int) (string, error)
}

// AudioSeparationProvider provides audio separation functionality
type AudioSeparationProvider interface {
	AIServiceProvider
	// SeparateVoice extracts voice from a mixed audio file
	SeparateVoice(ctx context.Context, audioFile, outputFormat string, maxTry, timeout int) ([]byte, error)
}