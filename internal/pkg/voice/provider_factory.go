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
	
	// modelCache caches provider instances to avoid repeated creation
	modelCache map[string]SpeechToTextProvider
}

// NewProviderFactory creates a provider factory with default settings
func NewProviderFactory() *ProviderFactory {
	factory := &ProviderFactory{
		UseMocks:      false,
		MockSTTName:   "default-mock",
		MockAudioName: "default-mock",
		modelCache:    make(map[string]SpeechToTextProvider),
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
	// Check cache first
	if provider, exists := f.modelCache[name]; exists {
		return provider, nil
	}
	
	var provider SpeechToTextProvider
	
	// Return mock provider if mocks are enabled
	if f.UseMocks {
		// For specific providers in mock mode, use specialized mocks
		switch strings.ToLower(name) {
		case "whisper":
			provider = NewMockWhisperProvider()
		case "gpt-4o-transcribe":
			provider = NewMockOpenAIProvider("gpt-4o-transcribe")
		case "gpt-4o-mini-transcribe":
			provider = NewMockOpenAIProvider("gpt-4o-mini-transcribe")
		case "scribe":
			provider = NewMockElevenLabsSTTProvider()
		default:
			// For all other providers, use the generic mock
			provider = GetMockSpeechToTextProvider(f.MockSTTName)
		}
		
		// Cache the provider
		f.modelCache[name] = provider
		return provider, nil
	} else {
		// Return real provider based on name
		switch strings.ToLower(name) {
		case "whisper":
			provider = NewWhisperProvider()
		case "incredibly-fast-whisper":
			provider = NewFastWhisperProvider()
		case "gpt-4o-transcribe":
			provider = NewOpenAIProvider("gpt-4o-transcribe")
		case "gpt-4o-mini-transcribe":
			provider = NewOpenAIProvider("gpt-4o-mini-transcribe")
		case "scribe":
			provider = NewElevenLabsSTTProvider()
		case "custom":
			provider = NewCustomSTTProvider()
		default:
			return nil, fmt.Errorf("unknown speech-to-text provider: %s", name)
		}
		
		// Cache the provider
		f.modelCache[name] = provider
		return provider, nil
	}
}

// GetSpeechToTextProviderWithAliases handles common aliases for STT models
func (f *ProviderFactory) GetSpeechToTextProviderWithAliases(name string) (SpeechToTextProvider, error) {
	// Normalize the model name
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	
	// Handle shortcuts/aliases
	switch normalizedName {
	case "wh":
		normalizedName = "whisper"
	case "fast", "ifw":
		normalizedName = "incredibly-fast-whisper"
	case "4o":
		normalizedName = "gpt-4o-transcribe"
	case "4o-mini":
		normalizedName = "gpt-4o-mini-transcribe"
	case "11", "el":
		normalizedName = "scribe"
	}
	
	return f.GetSpeechToTextProvider(normalizedName)
}

// GetAudioSeparationProvider gets an appropriate audio separation provider based on current settings
// (This remains the same as before)
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
	case "custom":
		return NewCustomSeparationProvider(), nil
	case "docker-demucs":
		return NewDockerDemucsProvider(false), nil
	case "docker-demucs_ft":
		return NewDockerDemucsProvider(true), nil
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
	return DefaultFactory.GetSpeechToTextProviderWithAliases(name)
}

// GetAudioSeparationProvider gets an audio separation provider from the default factory
// Always updates the factory first to ensure environment changes are reflected
func GetAudioSeparationProvider(name string) (AudioSeparationProvider, error) {
	// Update factory first to ensure we have the latest environment variables
	UpdateDefaultFactory()
	return DefaultFactory.GetAudioSeparationProvider(name)
}