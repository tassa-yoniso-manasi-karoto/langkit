package voice

import (
	"context"
	"os"
	"time"
	"strings"
	"fmt"
	"io"
	"net/http"
	"math"
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
)

var (
	APIKeys = &sync.Map{}
	STTModels = []string{"whisper", "incredibly-fast-whisper", "universal-1"}
)

func init() {
	APIKeys.Store("elevenlabs", "")
	APIKeys.Store("assemblyai", "")
	APIKeys.Store("replicate", "")
}


func ElevenlabsIsolator(ctx context.Context, filePath string, timeout int) ([]byte, error) {
	// Verify API key.
	apiKeyValue, found := APIKeys.Load("elevenlabs")
	if !found {
		return nil, fmt.Errorf("No Elevenlabs API key was provided")
	}
	APIKey, ok := apiKeyValue.(string)
	if !ok || APIKey == "" {
		return nil, fmt.Errorf("Invalid Elevenlabs API key format")
	}

	// Create a new client with a timeout.
	client := elevenlabs.NewClient(ctx, APIKey, time.Duration(timeout)*time.Second)

	// Build a generic retry policy for the API call.
	// This policy ignores any error until maxTry (here assumed to be 3) is reached,
	// except for context.Canceled, which aborts immediately.
	policy := buildRetryPolicy[[]byte](3)

	// Execute the API call with the retry policy.
	audio, err := failsafe.Get(func() ([]byte, error) {
		return client.VoiceIsolator(filePath)
	}, policy)
	if err != nil {
		return nil, fmt.Errorf("API query failed after retries: %w", err)
	}
	return audio, nil
}



// Universal1 uses AssemblyAI's service to transcribe an audio file.
// It verifies the API key, opens the file, and then uses a retry policy to
// attempt transcription multiple times. For each attempt, a new timeout context
// is created and the file pointer is reset.
// Errors are ignored until the maximum number of attempts is reached.
func Universal1(ctx context.Context, filepath string, maxTry, timeout int, lang string) (string, error) {
	// Verify API key.
	apiKeyValue, found := APIKeys.Load("assemblyai")
	if !found {
		return "", fmt.Errorf("No AssemblyAI API key was provided")
	}
	APIKey, ok := apiKeyValue.(string)
	if !ok || APIKey == "" {
		return "", fmt.Errorf("Invalid AssemblyAI API key format")
	}
	client := aai.NewClient(APIKey)

	// Open the audio file.
	f, err := os.Open(filepath)
	if err != nil {
		return "", fmt.Errorf("Couldn't open audio file: %w", err)
	}
	defer f.Close()

	// Setup transcription parameters.
	params := &aai.TranscriptOptionalParams{
		LanguageCode: aai.TranscriptLanguageCode(lang),
		SpeechModel:  aai.SpeechModelBest,
	}

	// Build a generic retry policy for transcription attempts.
	// Make retries ignore any error until the maximum attempts is reached,
	// except for context.Canceled which aborts immediately.
	policy := buildRetryPolicy[aai.Transcript](maxTry)

	// Execute the transcription with the retry policy.
	transcript, err := failsafe.Get(func() (aai.Transcript, error) {
		// Create a new timeout context for this attempt.
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		// Reset file pointer to the beginning for each attempt.
		if _, err := f.Seek(0, 0); err != nil {
			return aai.Transcript{}, err
		}

		// Attempt to transcribe the audio from the file.
		return client.Transcripts.TranscribeFromReader(attemptCtx, f, params)
	}, policy)
	if err != nil {
		return "", fmt.Errorf("Failed Universal-1 prediction after %d attempts: %w", maxTry, err)
	}

	// Return the transcription text.
	return *transcript.Text, nil
}


type initRunT = func(input replicate.PredictionInput) replicate.PredictionInput
type parserT = func(predictionOutput replicate.PredictionOutput) (string, error)

func Whisper(ctx context.Context, filepath string, maxTry, timeout int, lang, initialPrompt string) (string, error) {
	initRun := func(input replicate.PredictionInput) replicate.PredictionInput {
		input["language"] = lang
		if initialPrompt != "" {
			input["initial_prompt"] = initialPrompt
		}
		return input
	}
	params := r8RunParams{
		Ctx:      ctx,
		Filepath: filepath,
		MaxTry:   maxTry,
		Timeout:  timeout,
		Owner:    "openai",
		Name:     "whisper",
		InitRun:  initRun,
		Parser:   whisperParser,
	}
	return r8RunWithAudioFile(params)
}

func InsanelyFastWhisper(ctx context.Context, filepath string, maxTry, timeout int, lang string) (string, error) {
	initRun := func(input replicate.PredictionInput) replicate.PredictionInput {
		input["language"] = lang
		return input
	}
	
	params := r8RunParams{
		Ctx:      ctx,
		Filepath: filepath,
		MaxTry:   maxTry,
		Timeout:  timeout,
		Owner:    "vaibhavs10",
		Name:     "incredibly-fast-whisper", // model name is outdated on replicate
		InitRun:  initRun,
		Parser:   whisperParser,
	}
	return r8RunWithAudioFile(params)
}

func Spleeter(ctx context.Context, filepath string, maxTry, timeout int) ([]byte, error) {
	NoMoreInput := func(input replicate.PredictionInput) replicate.PredictionInput {
		return input
	}
	params := r8RunParams{
		Ctx:      ctx,
		Filepath: filepath,
		MaxTry:   maxTry,
		Timeout:  timeout,
		Owner:    "soykertje",
		Name:     "spleeter",
		InitRun:  NoMoreInput,
		Parser:   spleeterDemucsParser,
	}
	return r8RunWithAudioFileAndGET(params)
}


func Demucs(ctx context.Context, filepath, ext string, maxTry, timeout int, wantFinetuned bool) ([]byte, error) {
	initRun := func(input replicate.PredictionInput) replicate.PredictionInput {
		input["output_format"] = ext
		input["stems"] = "vocals"
		return input
	}
	if wantFinetuned {
		initRun = func(input replicate.PredictionInput) replicate.PredictionInput {
			input["model"] = "htdemucs_ft"
			input["output_format"] = ext
			input["stems"] = "vocals"
			return input
		}
	}
	params := r8RunParams{
		Ctx:      ctx,
		Filepath: filepath,
		MaxTry:   maxTry,
		Timeout:  timeout,
		Owner:    "ryan5453",
		Name:     "demucs",
		InitRun:  initRun,
		Parser:   spleeterDemucsParser,
	}
	return r8RunWithAudioFileAndGET(params)
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
		// Log each failed attempt.
		OnRetry(func(evt failsafe.ExecutionEvent[R]) {
			fmt.Fprintf(os.Stderr, "WARN: Attempt %d failed; retrying...\n", evt.Attempts())
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

	// Create a fresh context with timeout for this overall attempt.
	ctx, cancel := context.WithTimeout(params.Ctx, time.Duration(params.Timeout)*time.Second)
	defer cancel()

	// First, retrieve model info.
	model, err := r8.GetModel(ctx, params.Owner, params.Name)
	if err != nil {
		return "", fmt.Errorf("failed retrieving %s's information: %w", params.Name, err)
	}

	// --- File Upload Retry Policy ---
	// Build a policy for r8.CreateFileFromPath.
	uploadPolicy := buildRetryPolicy[*replicate.File](params.MaxTry)

	// Execute the upload with the retry policy.
	uploadResult, err := failsafe.Get(func() (*replicate.File, error) {
		return r8.CreateFileFromPath(ctx, params.Filepath, nil)
	}, uploadPolicy)
	if err != nil {
		return "", fmt.Errorf("CreateFileFromPath failed for \"%s\": %w", params.Filepath, err)
	}

	// --- Prediction Retry Policy ---
	// Build a policy for the overall prediction call.
	predictionPolicy := buildRetryPolicy[replicate.PredictionOutput](params.MaxTry)

	// Execute the prediction call within the retry policy.
	predictionOutput, err := failsafe.Get(func() (replicate.PredictionOutput, error) {
		// Build the input with the successfully uploaded file.
		input := replicate.PredictionInput{
			"audio": uploadResult,
		}
		input = params.InitRun(input)
		return r8.Run(ctx, params.Owner+"/"+params.Name+":"+model.LatestVersion.ID, input, nil)
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
func makeRequestWithRetry(url string, maxTry int) (*http.Response, error) {
	// Use the same generic policy creation function, specialized for *http.Response.
	policy := buildRetryPolicy[*http.Response](maxTry)
	return failsafe.Get(func() (*http.Response, error) {
		return http.Get(url)
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
	resp, err := makeRequestWithRetry(URL, params.MaxTry)
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
		return nil, fmt.Errorf("Error reading body of the response")
	}
	fmt.Print("\n")
	return body, nil
}






func spleeterDemucsParser (predictionOutput replicate.PredictionOutput) (string, error) {
	vocals, ok := predictionOutput.(map[string]interface{})["vocals"].(string)
	if !ok {
		return "", fmt.Errorf("vyocals key is missing or not a string")
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


func calcExponentialBackoff(attempt int, baseDelay time.Duration) time.Duration {
	return time.Duration(math.Pow(1.3, float64(attempt))) * baseDelay
}

func placeholder5() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
