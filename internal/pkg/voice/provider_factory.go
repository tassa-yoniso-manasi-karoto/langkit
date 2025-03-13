package voice

import (
	"fmt"
	"os"
	"strings"
)

// Environment variables that control provider selection
const (
	EnvUseMockProviders = "LANGKIT_USE_MOCK_PROVIDERS"
	EnvMockSTTProvider  = "LANGKIT_MOCK_STT_PROVIDER"
	EnvMockAudioProvider = "LANGKIT_MOCK_AUDIO_PROVIDER"
)

// ProviderFactory creates appropriate providers based on configuration
type ProviderFactory struct {
	// UseMocks determines if mock providers should be used globally
	UseMocks bool
	
	// MockSTTName is the name of the mock STT provider to use (if UseMocks is true)
	MockSTTName string
	
	// MockAudioName is the name of the mock audio separation provider to use (if UseMocks is true)
	MockAudioName string
}

// NewProviderFactory creates a provider factory with default settings
func NewProviderFactory() *ProviderFactory {
	factory := &ProviderFactory{
		UseMocks:      false,
		MockSTTName:   "default-mock",
		MockAudioName: "default-mock",
	}
	
	// Check environment variables
	if val := os.Getenv(EnvUseMockProviders); val != "" {
		factory.UseMocks = (strings.ToLower(val) == "true" || val == "1")
	}
	
	if val := os.Getenv(EnvMockSTTProvider); val != "" {
		factory.MockSTTName = val
	}
	
	if val := os.Getenv(EnvMockAudioProvider); val != "" {
		factory.MockAudioName = val
	}
	
	return factory
}

// GetSpeechToTextProvider gets an appropriate STT provider based on current settings
func (f *ProviderFactory) GetSpeechToTextProvider(name string) (SpeechToTextProvider, error) {
	// Return mock provider if mocks are enabled
	if f.UseMocks {
		// For "whisper" provider in mock mode, use our specific mock whisper provider
		if strings.ToLower(name) == "whisper" {
			return NewMockWhisperProvider(), nil
		}
		
		// For all other providers, use the generic mock
		return GetMockSpeechToTextProvider(f.MockSTTName), nil
	}
	
	// Return real provider based on name
	switch strings.ToLower(name) {
	case "whisper":
		return NewWhisperProvider(), nil
	case "incredibly-fast-whisper":
		return NewFastWhisperProvider(), nil
	case "universal-1":
		return &AssemblyAIProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown speech-to-text provider: %s", name)
	}
}

// GetAudioSeparationProvider gets an appropriate audio separation provider based on current settings
func (f *ProviderFactory) GetAudioSeparationProvider(name string) (AudioSeparationProvider, error) {
	// Return mock provider if mocks are enabled
	if f.UseMocks {
		return GetMockAudioSeparationProvider(f.MockAudioName), nil
	}
	
	// Return real provider based on name
	switch strings.ToLower(name) {
	case "elevenlabs":
		return &ElevenLabsProvider{}, nil
	case "spleeter":
		return NewSpleeterProvider(), nil
	case "demucs":
		return NewDemucsProvider(false), nil
	case "demucs_ft":
		return NewDemucsProvider(true), nil
	default:
		return nil, fmt.Errorf("unknown audio separation provider: %s", name)
	}
}

// DefaultFactory is the global provider factory instance
var DefaultFactory = NewProviderFactory()

// UpdateDefaultFactory updates the default factory to reflect current environment variables
// This should be called whenever environment variables might have changed
func UpdateDefaultFactory() {
	DefaultFactory = NewProviderFactory()
}

// GetSpeechToTextProvider gets a speech-to-text provider from the default factory
// Always updates the factory first to ensure environment changes are reflected
func GetSpeechToTextProvider(name string) (SpeechToTextProvider, error) {
	// Update factory first to ensure we have the latest environment variables
	UpdateDefaultFactory()
	return DefaultFactory.GetSpeechToTextProvider(name)
}

// GetAudioSeparationProvider gets an audio separation provider from the default factory
// Always updates the factory first to ensure environment changes are reflected
func GetAudioSeparationProvider(name string) (AudioSeparationProvider, error) {
	// Update factory first to ensure we have the latest environment variables
	UpdateDefaultFactory()
	return DefaultFactory.GetAudioSeparationProvider(name)
}