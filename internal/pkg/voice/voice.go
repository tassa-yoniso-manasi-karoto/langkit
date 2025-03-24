package voice

import (
	"context"
	"os"
	"time"
	"strings"
	"fmt"
	"io"
	"net/http"
	"sync"
	"errors"
	
	"github.com/tassa-yoniso-manasi-karoto/elevenlabs-go"
	
	"github.com/schollz/progressbar/v3"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	//"github.com/rivo/uniseg"
	//"github.com/sergi/go-diff/diffmatchpatch"
	replicate "github.com/replicate/replicate-go"
	aai "github.com/AssemblyAI/assemblyai-go-sdk"
	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

// Provider interfaces for better testing and flexibility

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

var (
	APIKeys = &sync.Map{}
	STTModels = []string{"whisper", "incredibly-fast-whisper", "universal-1", "gpt-4o-transcribe", "gpt-4o-mini-transcribe"}
)

func init() {
	APIKeys.Store("elevenlabs", "")
	APIKeys.Store("assemblyai", "")
	APIKeys.Store("replicate", "")
	APIKeys.Store("openai", "")
}


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

// Default provider instance for backward compatibility
var defaultElevenLabsProvider = &ElevenLabsProvider{}

// ElevenlabsIsolator is kept for backward compatibility
// It delegates to the default ElevenLabsProvider
func ElevenlabsIsolator(ctx context.Context, filePath string, timeout int) ([]byte, error) {
	return defaultElevenLabsProvider.SeparateVoice(ctx, filePath, "", 3, timeout)
}



// AssemblyAIProvider implements SpeechToTextProvider using the AssemblyAI API
type AssemblyAIProvider struct{}

// GetName returns the provider name
func (p *AssemblyAIProvider) GetName() string {
	return "assemblyai"
}

// IsAvailable checks if the AssemblyAI API is available
func (p *AssemblyAIProvider) IsAvailable() bool {
	apiKeyValue, found := APIKeys.Load("assemblyai")
	if !found {
		return false
	}
	APIKey, ok := apiKeyValue.(string)
	return ok && APIKey != ""
}

// TranscribeAudio converts audio to text using AssemblyAI
func (p *AssemblyAIProvider) TranscribeAudio(ctx context.Context, audioFile, language, _ string, maxTry, timeout int) (string, error) {
	// Verify API key
	apiKeyValue, found := APIKeys.Load("assemblyai")
	if !found {
		return "", fmt.Errorf("No AssemblyAI API key was provided")
	}
	APIKey, ok := apiKeyValue.(string)
	if !ok || APIKey == "" {
		return "", fmt.Errorf("Invalid AssemblyAI API key format")
	}
	client := aai.NewClient(APIKey)

	// Open the audio file
	f, err := os.Open(audioFile)
	if err != nil {
		return "", fmt.Errorf("Couldn't open audio file: %w", err)
	}
	defer f.Close()

	// Setup transcription parameters
	params := &aai.TranscriptOptionalParams{
		LanguageCode: aai.TranscriptLanguageCode(language),
		SpeechModel:  aai.SpeechModelBest,
	}

	// Build retry policy for transcription attempts
	policy := buildRetryPolicy[aai.Transcript](maxTry)

	// Execute the transcription with the retry policy
	transcript, err := failsafe.Get(func() (aai.Transcript, error) {
		// Create a new timeout context for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		// Reset file pointer to the beginning for each attempt
		if _, err := f.Seek(0, 0); err != nil {
			return aai.Transcript{}, err
		}

		// Attempt to transcribe the audio
		return client.Transcripts.TranscribeFromReader(attemptCtx, f, params)
	}, policy)
	if err != nil {
		return "", fmt.Errorf("Failed Universal-1 prediction after %d attempts: %w", maxTry, err)
	}

	// Return the transcription text
	return *transcript.Text, nil
}

// Default provider instance for backward compatibility
var defaultAssemblyAIProvider = &AssemblyAIProvider{}

// Universal1 is kept for backward compatibility
// It delegates to the default AssemblyAIProvider
func Universal1(ctx context.Context, filepath string, maxTry, timeout int, lang string) (string, error) {
	return defaultAssemblyAIProvider.TranscribeAudio(ctx, filepath, lang, "", maxTry, timeout)
}


type initRunT = func(input replicate.PredictionInput) replicate.PredictionInput
type parserT = func(predictionOutput replicate.PredictionOutput) (string, error)

// ReplicateProvider is a base struct for providers using the Replicate platform
type ReplicateProvider struct {
	Owner        string
	ModelName    string
	ModelVersion string // Optional, uses latest if empty
}

// GetName returns a formatted provider name
func (p *ReplicateProvider) GetName() string {
	return fmt.Sprintf("replicate:%s/%s", p.Owner, p.ModelName)
}

// IsAvailable checks if the Replicate API is available
func (p *ReplicateProvider) IsAvailable() bool {
	apiKeyValue, found := APIKeys.Load("replicate")
	if !found {
		return false
	}
	APIKey, ok := apiKeyValue.(string)
	return ok && APIKey != ""
}

// createClient creates a new Replicate client
func (p *ReplicateProvider) createClient() (*replicate.Client, error) {
	apiKeyValue, found := APIKeys.Load("replicate")
	if !found {
		return nil, fmt.Errorf("No Replicate API key was provided")
	}
	APIKey, ok := apiKeyValue.(string)
	if !ok || APIKey == "" {
		return nil, fmt.Errorf("Invalid Replicate API key format")
	}
	return replicate.NewClient(replicate.WithToken(APIKey))
}

// WhisperProvider implements SpeechToTextProvider using OpenAI's Whisper model via Replicate
type WhisperProvider struct {
	ReplicateProvider
}

// NewWhisperProvider creates a new WhisperProvider
func NewWhisperProvider() *WhisperProvider {
	return &WhisperProvider{
		ReplicateProvider: ReplicateProvider{
			Owner:     "openai",
			ModelName: "whisper",
		},
	}
}

// TranscribeAudio transcribes audio using Whisper
func (p *WhisperProvider) TranscribeAudio(ctx context.Context, audioFile, language, initialPrompt string, maxTry, timeout int) (string, error) {
	initRun := func(input replicate.PredictionInput) replicate.PredictionInput {
		input["language"] = language
		if initialPrompt != "" {
			input["initial_prompt"] = initialPrompt
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
		Parser:   whisperParser,
	}
	return r8RunWithAudioFile(params)
}

// Default provider instance for backward compatibility
var defaultWhisperProvider = NewWhisperProvider()

// Whisper is kept for backward compatibility
func Whisper(ctx context.Context, filepath string, maxTry, timeout int, lang, initialPrompt string) (string, error) {
	return defaultWhisperProvider.TranscribeAudio(ctx, filepath, lang, initialPrompt, maxTry, timeout)
}

// FastWhisperProvider implements SpeechToTextProvider using Incredibly Fast Whisper via Replicate
type FastWhisperProvider struct {
	ReplicateProvider
}

// NewFastWhisperProvider creates a new FastWhisperProvider
func NewFastWhisperProvider() *FastWhisperProvider {
	return &FastWhisperProvider{
		ReplicateProvider: ReplicateProvider{
			Owner:     "vaibhavs10",
			ModelName: "incredibly-fast-whisper",
		},
	}
}

// TranscribeAudio transcribes audio using Fast Whisper
func (p *FastWhisperProvider) TranscribeAudio(ctx context.Context, audioFile, language, _ string, maxTry, timeout int) (string, error) {
	initRun := func(input replicate.PredictionInput) replicate.PredictionInput {
		input["language"] = language
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
		Parser:   whisperParser,
	}
	return r8RunWithAudioFile(params)
}

// Default provider instance for backward compatibility
var defaultFastWhisperProvider = NewFastWhisperProvider()

// InsanelyFastWhisper is kept for backward compatibility
func InsanelyFastWhisper(ctx context.Context, filepath string, maxTry, timeout int, lang string) (string, error) {
	return defaultFastWhisperProvider.TranscribeAudio(ctx, filepath, lang, "", maxTry, timeout)
}

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

// Default provider instance for backward compatibility
var defaultSpleeterProvider = NewSpleeterProvider()

// Spleeter is kept for backward compatibility
func Spleeter(ctx context.Context, filepath string, maxTry, timeout int) ([]byte, error) {
	return defaultSpleeterProvider.SeparateVoice(ctx, filepath, "wav", maxTry, timeout)
}

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

// Default provider instances for backward compatibility
var defaultDemucsProvider = NewDemucsProvider(false)
var defaultDemucsFinetunedProvider = NewDemucsProvider(true)

// Demucs is kept for backward compatibility
func Demucs(ctx context.Context, filepath, ext string, maxTry, timeout int, wantFinetuned bool) ([]byte, error) {
	if wantFinetuned {
		return defaultDemucsFinetunedProvider.SeparateVoice(ctx, filepath, ext, maxTry, timeout)
	}
	return defaultDemucsProvider.SeparateVoice(ctx, filepath, ext, maxTry, timeout)
}


// r8RunParams holds parameters controlling how to run the model and parse results.
type r8RunParams struct {
	Ctx      context.Context
	Filepath string
	MaxTry   int
	Timeout  int
	Owner    string
	Name     string
	InitRun  initRunT
	Parser   parserT
}

// buildRetryPolicy is a single function that builds a generic retry policy for any type R.
//
// Make retries ignore all errors unless we have hit the max attempts, in which case
// the last error is returned. The only early abort condition is context.Canceled.
func buildRetryPolicy[R any](maxTry int) failsafe.Policy[R] {
	return retrypolicy.Builder[R]().
		// Handle any error for retry, except context.Canceled which we abort on.
		HandleIf(func(_ R, err error) bool {
			return err != nil && !errors.Is(err, context.Canceled)
		}).
		// Abort if context was canceled.
		AbortOnErrors(context.Canceled).
		// Retry up to maxTry attempts before returning last error.
		WithMaxAttempts(maxTry).
		// Return the last error upon exceeding attempts (instead of a special ExceededError).
		ReturnLastFailure().
		// Example exponential backoff from 500 ms up to 5 s, doubling each time.
		WithBackoffFactor(500*time.Millisecond, 5*time.Second, 2.0).
		// Log each failed attempt with more detailed error information.
		OnRetry(func(evt failsafe.ExecutionEvent[R]) {
			fmt.Fprintf(os.Stderr, "WARN: Attempt %d failed with error: %v; retrying...\n", 
				evt.Attempts(), evt.LastError())
		}).
		Build()
}

// r8RunWithAudioFile runs a Replicate model with file input and returns the parsed result.
//
// 1) Uploads the file with repeated attempts if needed, ignoring all errors except context.Canceled
//    until maxTry is reached.
//
// 2) Calls r8.Run with repeated attempts if needed, ignoring all errors except context.Canceled
//    until maxTry is reached.
//
// 3) Processes the final result.
func r8RunWithAudioFile(params r8RunParams) (string, error) {
	// Verify API key.
	apiKeyValue, found := APIKeys.Load("replicate")
	if !found {
		return "", fmt.Errorf("No Replicate API key was provided")
	}
	APIKey, ok := apiKeyValue.(string)
	if !ok || APIKey == "" {
		return "", fmt.Errorf("Invalid Replicate API key format")
	}

	// Create a new client.
	r8, err := replicate.NewClient(replicate.WithToken(APIKey))
	if err != nil {
		return "", fmt.Errorf("failed to create Replicate client: %w", err)
	}

	// Create a parent context for the whole operation - this shouldn't time out
	// but will be inherited by each individual operation
	parentCtx, parentCancel := context.WithCancel(params.Ctx)
	defer parentCancel()

	// First, retrieve model info.
	var model *replicate.Model
	modelPolicy := buildRetryPolicy[*replicate.Model](params.MaxTry)
	model, err = failsafe.Get(func() (*replicate.Model, error) {
		// Create a separate context for model retrieval
		modelCtx, modelCancel := context.WithTimeout(parentCtx, time.Duration(params.Timeout)*time.Second)
		defer modelCancel()
		
		return r8.GetModel(modelCtx, params.Owner, params.Name)
	}, modelPolicy)
	
	if err != nil {
		return "", fmt.Errorf("failed retrieving %s's information: %w", params.Name, err)
	}

	// --- File Upload Retry Policy ---
	// Build a policy for r8.CreateFileFromPath.
	uploadPolicy := buildRetryPolicy[*replicate.File](params.MaxTry)

	// Execute the upload with the retry policy.
	var uploadResult *replicate.File
	uploadResult, err = failsafe.Get(func() (*replicate.File, error) {
		// Create a fresh context for this upload attempt
		uploadCtx, uploadCancel := context.WithTimeout(parentCtx, time.Duration(params.Timeout)*time.Second)
		defer uploadCancel()
		
		return r8.CreateFileFromPath(uploadCtx, params.Filepath, nil)
	}, uploadPolicy)
	
	if err != nil {
		return "", fmt.Errorf("CreateFileFromPath failed for \"%s\": %w", params.Filepath, err)
	}

	// --- Prediction Retry Policy ---
	// Build a policy for the overall prediction call.
	predictionPolicy := buildRetryPolicy[replicate.PredictionOutput](params.MaxTry)

	// Execute the prediction call within the retry policy.
	var predictionOutput replicate.PredictionOutput
	predictionOutput, err = failsafe.Get(func() (replicate.PredictionOutput, error) {
		// Create a fresh context for this prediction attempt
		predictionCtx, predictionCancel := context.WithTimeout(parentCtx, time.Duration(params.Timeout)*time.Second)
		defer predictionCancel()
		
		// Build the input with the successfully uploaded file.
		input := replicate.PredictionInput{
			"audio": uploadResult,
		}
		input = params.InitRun(input)
		return r8.Run(predictionCtx, params.Owner+"/"+params.Name+":"+model.LatestVersion.ID, input, nil)
	}, predictionPolicy)
	
	if err != nil {
		// If it's a replicate.ModelError, print logs and modify the error message.
		if me, ok := err.(*replicate.ModelError); ok {
			logs := ""
			if me.Prediction.Logs != nil {
				logs = *me.Prediction.Logs
			}
			color.Redln(strings.ReplaceAll(logs, "\n", "\n\t"))
			if logs == me.Prediction.Error && strings.Contains(logs, ":") {
				e := strings.TrimPrefix(me.Prediction.Error.(string), "model error: ")
				me.Prediction.Error, _, _ = strings.Cut(e, ":")
			}
			s := "see logs above"
			me.Prediction.Logs = &s
		}
		return "", fmt.Errorf("Failed %s prediction after %d attempts: %w", params.Name, params.MaxTry, err)
	}

	// Process the prediction output.
	resultStr, err := params.Parser(predictionOutput)
	if err != nil {
		pp.Println(err)
		return "", fmt.Errorf("Parser failed: %w", err)
	}
	return resultStr, nil
}

// makeRequestWithRetry performs an HTTP GET using failsafe-go with the same style of retrypolicy.
func makeRequestWithRetry(url string, ctx context.Context, timeout, maxTry int) (*http.Response, error) {
	// Use the same generic policy creation function, specialized for *http.Response.
	policy := buildRetryPolicy[*http.Response](maxTry)
	
	return failsafe.Get(func() (*http.Response, error) {
		// Create a fresh context for each request attempt
		reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		
		// Create a request with context
		req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		return http.DefaultClient.Do(req)
	}, policy)
}

// r8RunWithAudioFileAndGET runs the model prediction with repeated attempts, then
// downloads the resulting URL with repeated attempts. We adopt retrypolicy for the GET as well.
func r8RunWithAudioFileAndGET(params r8RunParams) ([]byte, error) {
	// 1. Run the model to get a URL.
	URL, err := r8RunWithAudioFile(params)
	if err != nil {
		return nil, err
	}

	// 2. Download the result with repeated attempts.
	resp, err := makeRequestWithRetry(URL, params.Ctx, params.Timeout, params.MaxTry)
	if err != nil {
		return nil, fmt.Errorf("Failed request on prediction output after %d attempts: %w", params.MaxTry, err)
	}
	defer resp.Body.Close()

	// 3. Read (and track) the downloaded content.
	bar := progressbar.DefaultBytes(-1, "downloading")
	reader := io.TeeReader(resp.Body, bar)

	body, err := io.ReadAll(reader)
	if err != nil {
		pp.Println(err)
		return nil, fmt.Errorf("Error reading body of the response: %w", err)
	}
	fmt.Print("\n")
	return body, nil
}


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


// OpenAIProvider implements SpeechToTextProvider using OpenAI API
type OpenAIProvider struct {
	ModelName string // "gpt-4o-transcribe" or "gpt-4o-mini-transcribe"
}

// NewOpenAIProvider creates a new OpenAIProvider
func NewOpenAIProvider(modelName string) *OpenAIProvider {
	return &OpenAIProvider{
		ModelName: modelName,
	}
}

// GetName returns the provider name
func (p *OpenAIProvider) GetName() string {
	return "openai:" + p.ModelName
}

// IsAvailable checks if the OpenAI API is available
func (p *OpenAIProvider) IsAvailable() bool {
	apiKeyValue, found := APIKeys.Load("openai")
	if !found {
		return false
	}
	APIKey, ok := apiKeyValue.(string)
	return ok && APIKey != ""
}

// TranscribeAudio converts audio to text using OpenAI GPT-4o models
func (p *OpenAIProvider) TranscribeAudio(ctx context.Context, audioFile, language, initialPrompt string, maxTry, timeout int) (string, error) {
	// Verify API key
	apiKeyValue, found := APIKeys.Load("openai")
	if !found {
		return "", fmt.Errorf("No OpenAI API key was provided")
	}
	APIKey, ok := apiKeyValue.(string)
	if !ok || APIKey == "" {
		return "", fmt.Errorf("Invalid OpenAI API key format")
	}

	// Open the audio file
	f, err := os.Open(audioFile)
	if err != nil {
		return "", fmt.Errorf("Couldn't open audio file: %w", err)
	}
	defer f.Close()

	// Create OpenAI client with API key
	client := openai.NewClient(option.WithAPIKey(APIKey))

	// Determine which model to use
	var model openai.AudioModel
	switch p.ModelName {
	case "gpt-4o-transcribe":
		model = openai.AudioModelGPT4oTranscribe
	case "gpt-4o-mini-transcribe":
		model = openai.AudioModelGPT4oMiniTranscribe
	default:
		return "", fmt.Errorf("unsupported OpenAI model: %s", p.ModelName)
	}

	// Build a retry policy for transcription attempts
	policy := buildRetryPolicy[*openai.Transcription](maxTry)

	// Execute the transcription with the retry policy
	transcription, err := failsafe.Get(func() (*openai.Transcription, error) {
		// Create a new timeout context for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		// Reset file pointer to the beginning for each attempt
		if _, err := f.Seek(0, 0); err != nil {
			return nil, err
		}

		// Setup transcription params
		params := openai.AudioTranscriptionNewParams{
			Model: model,
			File:  f,
			ResponseFormat: openai.AudioResponseFormatJSON,
		}

		// Add language if specified
		if language != "" {
			params.Language = param.NewOpt(language)
		}

		// Add prompt if specified
		if initialPrompt != "" {
			params.Prompt = param.NewOpt(initialPrompt)
		}

		// Attempt to transcribe the audio
		return client.Audio.Transcriptions.New(attemptCtx, params)
	}, policy)
	if err != nil {
		return "", fmt.Errorf("Failed %s prediction after %d attempts: %w", p.ModelName, maxTry, err)
	}

	// Return the transcription text
	return transcription.Text, nil
}

// Default provider instances for backward compatibility
var defaultGPT4oTranscribeProvider = NewOpenAIProvider("gpt-4o-transcribe")
var defaultGPT4oMiniTranscribeProvider = NewOpenAIProvider("gpt-4o-mini-transcribe")

// GPT4oTranscribe is for backward compatibility
func GPT4oTranscribe(ctx context.Context, filepath string, maxTry, timeout int, lang, initialPrompt string) (string, error) {
	return defaultGPT4oTranscribeProvider.TranscribeAudio(ctx, filepath, lang, initialPrompt, maxTry, timeout)
}

// GPT4oMiniTranscribe is for backward compatibility
func GPT4oMiniTranscribe(ctx context.Context, filepath string, maxTry, timeout int, lang, initialPrompt string) (string, error) {
	return defaultGPT4oMiniTranscribeProvider.TranscribeAudio(ctx, filepath, lang, initialPrompt, maxTry, timeout)
}

func placeholder5() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}