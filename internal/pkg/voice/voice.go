package voice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
	"github.com/schollz/progressbar/v3"
	"github.com/gookit/color"
	"github.com/k0kubun/pp"
	
	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	
	replicate "github.com/replicate/replicate-go"
	
)

var (
	APIKeys         = &sync.Map{}
	CustomEndpoints = &sync.Map{}
)

func init() {
	APIKeys.Store("elevenlabs", "")
	APIKeys.Store("replicate", "")
	APIKeys.Store("openai", "")

	// Initialize custom endpoint defaults
	CustomEndpoints.Store("stt_enabled", false)
	CustomEndpoints.Store("stt_endpoint", "")
	CustomEndpoints.Store("stt_model", "")
	CustomEndpoints.Store("voice_isolation_enabled", false)
	CustomEndpoints.Store("voice_isolation_endpoint", "")
}

// ElevenLabsSTTProvider implements SpeechToTextProvider using ElevenLabs Scribe API
type ElevenLabsSTTProvider struct{}

// NewElevenLabsSTTProvider creates a new ElevenLabsSTTProvider
func NewElevenLabsSTTProvider() *ElevenLabsSTTProvider {
	return &ElevenLabsSTTProvider{}
}

// GetName returns the provider name
func (p *ElevenLabsSTTProvider) GetName() string {
	return "scribe"
}

// IsAvailable checks if the ElevenLabs API is available
func (p *ElevenLabsSTTProvider) IsAvailable() bool {
	apiKeyValue, found := APIKeys.Load("elevenlabs")
	if !found {
		return false
	}
	APIKey, ok := apiKeyValue.(string)
	return ok && APIKey != ""
}

// TranscribeAudio converts audio to text using ElevenLabs Scribe API
func (p *ElevenLabsSTTProvider) TranscribeAudio(ctx context.Context, audioFile, language, initialPrompt string, maxTry, timeout int) (string, error) {
	// Verify API key
	apiKeyValue, found := APIKeys.Load("elevenlabs")
	if !found {
		return "", fmt.Errorf("No ElevenLabs API key was provided")
	}
	APIKey, ok := apiKeyValue.(string)
	if !ok || APIKey == "" {
		return "", fmt.Errorf("Invalid ElevenLabs API key format")
	}

	// Build a generic retry policy for the API call
	policy := buildRetryPolicy[string](maxTry)

	// Execute the API call with the retry policy
	transcription, err := failsafe.Get(func() (string, error) {
		// Create a fresh context for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		// Prepare the file for upload
		file, err := os.Open(audioFile)
		if err != nil {
			return "", fmt.Errorf("couldn't open audio file: %w", err)
		}
		defer file.Close()

		// Create a pipe for streaming the file
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)

		// Start a goroutine to write the file data to the pipe
		go func() {
			defer pw.Close()

			// Add form fields
			_ = writer.WriteField("model_id", "scribe_v1")

			// Add language_code field if specified
			if language != "" {
				_ = writer.WriteField("language_code", language)
			}

			// Add other optional parameters with default values
			_ = writer.WriteField("tag_audio_events", "true")
			_ = writer.WriteField("timestamps_granularity", "word")
			_ = writer.WriteField("diarize", "false")

			// Add the file
			part, err := writer.CreateFormFile("file", filepath.Base(audioFile))
			if err != nil {
				pw.CloseWithError(fmt.Errorf("error creating form file: %w", err))
				return
			}

			// Copy the file data to the multipart form
			_, err = io.Copy(part, file)
			if err != nil {
				pw.CloseWithError(fmt.Errorf("error copying file data: %w", err))
				return
			}

			// Close the writer to finalize the form data
			err = writer.Close()
			if err != nil {
				pw.CloseWithError(fmt.Errorf("error closing multipart writer: %w", err))
				return
			}
		}()

		// Create the request
		req, err := http.NewRequestWithContext(attemptCtx, "POST", "https://api.elevenlabs.io/v1/speech-to-text", pr)
		if err != nil {
			return "", fmt.Errorf("error creating request: %w", err)
		}

		// Set headers
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("xi-api-key", APIKey)

		// Execute the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("error sending request: %w", err)
		}
		defer resp.Body.Close()

		// Check for non-2xx status code
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}

		// Parse the response
		var result struct {
			Text         string  `json:"text"`
			LanguageCode string  `json:"language_code"`
			LanguageProb float64 `json:"language_probability"`
			Words        []any   `json:"words"` // We only need the text, so don't parse the full structure
		}

		err = json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			return "", fmt.Errorf("error decoding response: %w", err)
		}

		return result.Text, nil
	}, policy)

	if err != nil {
		return "", fmt.Errorf("API query failed after retries: %w", err)
	}

	return transcription, nil
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

// r8RunWithAudioFile runs a Replicate model with file input and returns the parsed result.
//
//  1. Uploads the file with repeated attempts if needed, ignoring all errors except context.Canceled
//     until maxTry is reached.
//
//  2. Calls r8.Run with repeated attempts if needed, ignoring all errors except context.Canceled
//     until maxTry is reached.
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

// makeRequestWithRetry performs an HTTP GET using failsafe-go, reads the body, and returns it.
func makeRequestWithRetry(url string, ctx context.Context, timeout, maxTry int) ([]byte, error) {
	// Use the same generic policy creation function, specialized for []byte.
	policy := buildRetryPolicy[[]byte](maxTry)

	return failsafe.Get(func() ([]byte, error) {
		reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		// Create a request with context
		req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Execute request
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err // Let failsafe handle retry based on this error
		}
		defer resp.Body.Close()

		// Check for non-2xx status code before reading body
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// Read a small part of the body for error context, avoid reading huge bodies on error
			bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(bodyBytes))
		}

		// Read the entire body, optionally with progress bar
		var reader io.Reader
		if false { // Keep progress bar logic disabled by default
			// Use ContentLength if available, otherwise -1 for indeterminate progress
			contentLength := resp.ContentLength
			bar := progressbar.DefaultBytes(contentLength, "downloading")
			reader = io.TeeReader(resp.Body, bar)
		} else {
			reader = resp.Body
		}

		body, readErr := io.ReadAll(reader)
		if readErr != nil {
			// Check if the read error is due to context cancellation/deadline
			if errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded) {
				fmt.Fprintf(os.Stderr, "DEBUG: Body read failed with context error: %v\n", readErr)
			}
			return nil, fmt.Errorf("error reading response body: %w", readErr)
		}
		return body, nil
	}, policy)
}

// r8RunWithAudioFileAndGET runs the model prediction with repeated attempts, then
// downloads the resulting URL with repeated attempts.
func r8RunWithAudioFileAndGET(params r8RunParams) ([]byte, error) {
	// 1. Run the model to get a URL.
	URL, err := r8RunWithAudioFile(params)
	if err != nil {
		return nil, err
	}

	// For download operations, we should use a dedicated download timeout if available
	// Check for TimeoutDL in the context
	var dlTimeout int
	if timeoutVal := params.Ctx.Value("TimeoutDL"); timeoutVal != nil {
		if timeout, ok := timeoutVal.(int); ok && timeout > 0 {
			dlTimeout = timeout
		}
	}
	
	// If no dedicated download timeout was found, fall back to the regular timeout
	if dlTimeout == 0 {
		dlTimeout = params.Timeout
	}
	
	// 2. Download the result (including body read) with repeated attempts.
	body, err := makeRequestWithRetry(URL, params.Ctx, dlTimeout, params.MaxTry)
	if err != nil {
		color.Redln("couldn't download/read body from " + URL)
		pp.Println(err)
		return nil, fmt.Errorf("failed request or body read for prediction output after %d attempts: %w", params.MaxTry, err)
	}
	return body, nil
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
			Model:          model,
			File:           f,
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

func placeholder5() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
