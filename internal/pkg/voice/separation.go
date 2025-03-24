package voice

import (
	"context"
	"fmt"
	"time"
	
	"github.com/tassa-yoniso-manasi-karoto/elevenlabs-go"
	"github.com/failsafe-go/failsafe-go"
	replicate "github.com/replicate/replicate-go"
)


// ElevenlabsIsolator provides direct access to ElevenLabs voice isolation
func ElevenlabsIsolator(ctx context.Context, filePath string, timeout int) ([]byte, error) {
	return defaultElevenLabsProvider.SeparateVoice(ctx, filePath, "", 3, timeout)
}

// Spleeter provides direct access to the Spleeter voice separation model
func Spleeter(ctx context.Context, filepath string, maxTry, timeout int) ([]byte, error) {
	return defaultSpleeterProvider.SeparateVoice(ctx, filepath, "wav", maxTry, timeout)
}

// Demucs provides direct access to the Demucs voice separation model
func Demucs(ctx context.Context, filepath, ext string, maxTry, timeout int, wantFinetuned bool) ([]byte, error) {
	if wantFinetuned {
		return defaultDemucsFinetunedProvider.SeparateVoice(ctx, filepath, ext, maxTry, timeout)
	}
	return defaultDemucsProvider.SeparateVoice(ctx, filepath, ext, maxTry, timeout)
}


// --- Provider implementations for audio separation ---

// ElevenLabsProvider implements AudioSeparationProvider using the ElevenLabs API
type ElevenLabsProvider struct {}

// GetName returns the provider name
func (p *ElevenLabsProvider) GetName() string {
	return "elevenlabs"
}

// IsAvailable checks if the ElevenLabs API is available
func (p *ElevenLabsProvider) IsAvailable() bool {
	apiKeyValue, found := APIKeys.Load("elevenlabs")
	if !found {
		return false
	}
	APIKey, ok := apiKeyValue.(string)
	return ok && APIKey != ""
}

// SeparateVoice extracts voice from audio using ElevenLabs
func (p *ElevenLabsProvider) SeparateVoice(ctx context.Context, audioFile, _ string, maxTry, timeout int) ([]byte, error) {
	// Verify API key
	apiKeyValue, found := APIKeys.Load("elevenlabs")
	if !found {
		return nil, fmt.Errorf("No Elevenlabs API key was provided")
	}
	APIKey, ok := apiKeyValue.(string)
	if !ok || APIKey == "" {
		return nil, fmt.Errorf("Invalid Elevenlabs API key format")
	}

	// Build a generic retry policy for the API call
	policy := buildRetryPolicy[[]byte](maxTry)

	// Execute the API call with the retry policy
	audio, err := failsafe.Get(func() ([]byte, error) {
		// Create a fresh context for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		
		// Create a new client with the fresh context
		client := elevenlabs.NewClient(attemptCtx, APIKey, time.Duration(timeout)*time.Second)
		return client.VoiceIsolator(audioFile)
	}, policy)
	if err != nil {
		return nil, fmt.Errorf("API query failed after retries: %w", err)
	}
	return audio, nil
}

// Default provider instance for standard use
var defaultElevenLabsProvider = &ElevenLabsProvider{}

// SpleeterProvider implements AudioSeparationProvider using Spleeter via Replicate
type SpleeterProvider struct {
	ReplicateProvider
}

// NewSpleeterProvider creates a new SpleeterProvider
func NewSpleeterProvider() *SpleeterProvider {
	return &SpleeterProvider{
		ReplicateProvider: ReplicateProvider{
			Owner:     "soykertje",
			ModelName: "spleeter",
		},
	}
}

// SeparateVoice separates voice from audio using Spleeter
func (p *SpleeterProvider) SeparateVoice(ctx context.Context, audioFile, _ string, maxTry, timeout int) ([]byte, error) {
	NoMoreInput := func(input replicate.PredictionInput) replicate.PredictionInput {
		return input
	}
	params := r8RunParams{
		Ctx:      ctx,
		Filepath: audioFile,
		MaxTry:   maxTry,
		Timeout:  timeout,
		Owner:    p.Owner,
		Name:     p.ModelName,
		InitRun:  NoMoreInput,
		Parser:   spleeterDemucsParser,
	}
	return r8RunWithAudioFileAndGET(params)
}

// Default provider instance for standard use
var defaultSpleeterProvider = NewSpleeterProvider()

// DemucsProvider implements AudioSeparationProvider using Demucs via Replicate
type DemucsProvider struct {
	ReplicateProvider
	UseFinetuned bool
}

// NewDemucsProvider creates a new DemucsProvider
func NewDemucsProvider(useFinetuned bool) *DemucsProvider {
	return &DemucsProvider{
		ReplicateProvider: ReplicateProvider{
			Owner:     "ryan5453",
			ModelName: "demucs",
		},
		UseFinetuned: useFinetuned,
	}
}

// SeparateVoice separates voice from audio using Demucs
func (p *DemucsProvider) SeparateVoice(ctx context.Context, audioFile, outputFormat string, maxTry, timeout int) ([]byte, error) {
	initRun := func(input replicate.PredictionInput) replicate.PredictionInput {
		input["output_format"] = outputFormat
		input["stems"] = "vocals"
		if p.UseFinetuned {
			input["model"] = "htdemucs_ft"
		}
		return input
	}
	params := r8RunParams{
		Ctx:      ctx,
		Filepath: audioFile,
		MaxTry:   maxTry,
		Timeout:  timeout,
		Owner:    p.Owner,
		Name:     p.ModelName,
		InitRun:  initRun,
		Parser:   spleeterDemucsParser,
	}
	return r8RunWithAudioFileAndGET(params)
}

// Default provider instances for standard use
var defaultDemucsProvider = NewDemucsProvider(false)
var defaultDemucsFinetunedProvider = NewDemucsProvider(true)



func spleeterDemucsParser (predictionOutput replicate.PredictionOutput) (string, error) {
	vocals, ok := predictionOutput.(map[string]interface{})["vocals"].(string)
	if !ok {
		return "", fmt.Errorf("vocals key is missing or not a string")
	}
	return vocals, nil
}


func whisperParser (predictionOutput replicate.PredictionOutput) (string, error) {
	transcription, ok := predictionOutput.(map[string]interface{})["transcription"].(string)
	if !ok {
		return "", fmt.Errorf("transcription key is missing or not a string")
	}
	return transcription, nil
}
