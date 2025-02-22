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
)

var (
	APIKeys = &sync.Map{}
)

func init() {
	APIKeys.Store("elevenlabs", "")
	APIKeys.Store("assemblyai", "")
	APIKeys.Store("replicate", "")
}


func ElevenlabsIsolator(ctx context.Context, filePath string, timeout int) ([]byte, error) {
	apiKeyValue, found := APIKeys.Load("elevenlabs")
	if !found {
		return nil, fmt.Errorf("No Elevenlabs API key was provided")
	}
	APIKey, ok := apiKeyValue.(string)
	if !ok || APIKey == "" {
		return nil, fmt.Errorf("Invalid Elevenlabs API key format")
	}
	client := elevenlabs.NewClient(ctx, APIKey, time.Duration(timeout)*time.Second)
	audio, err := client.VoiceIsolator(filePath)
	if err != nil {
		return nil, fmt.Errorf("API query failed: %w", err)
	}
	return audio, nil
}


func Universal1(ctx context.Context, filepath string, maxTry, timeout int, lang string) (string, error) {
	apiKeyValue, found := APIKeys.Load("assemblyai")
	if !found {
		return "", fmt.Errorf("No AssemblyAI API key was provided")
	}
	APIKey, ok := apiKeyValue.(string)
	if !ok || APIKey == "" {
		return "", fmt.Errorf("Invalid AssemblyAI API key format")
	}
	client := aai.NewClient(APIKey)

	f, err := os.Open(filepath)
	if err != nil {
		return "", fmt.Errorf("Couldn't open audio file:", err)
	}
	defer f.Close()

	for try := 0; try < maxTry; try++ {
		ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		params := &aai.TranscriptOptionalParams{
			LanguageCode: aai.TranscriptLanguageCode(lang),
			SpeechModel: aai.SpeechModelBest,
		}
		transcript, err := client.Transcripts.TranscribeFromReader(ctx, f, params)
		if err == nil {
			return *transcript.Text, nil
		} else if err == context.DeadlineExceeded {
			fmt.Printf("Timed out Universal-1 prediction (%d/%d)...\n", try, maxTry)
			if try+1 != maxTry {
				continue
			}
			break
		} else {
			pp.Println(err)
			return "", fmt.Errorf("Failed Universal-1 prediction: %w", err)
		}
	}
	return "", fmt.Errorf("Timed out Universal-1 prediction after %d attempts: %w", maxTry, err)
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

func r8RunWithAudioFile(params r8RunParams) (string, error) {
	apiKeyValue, found := APIKeys.Load("replicate")
	if !found {
		return "", fmt.Errorf("No Replicate API key was provided")
	}
	APIKey, ok := apiKeyValue.(string)
	if !ok || APIKey == "" {
		return "", fmt.Errorf("Invalid Replicate API key format")
	}
	var predictionOutput replicate.PredictionOutput
	baseDelay := time.Millisecond * 500
	for try := 0; try < params.MaxTry; try++ {
		r8, err := replicate.NewClient(replicate.WithToken(APIKey))
		ctx, cancel := context.WithTimeout(params.Ctx, time.Duration(params.Timeout)*time.Second)
		
		model, err := r8.GetModel(ctx, params.Owner, params.Name)
		if err != nil {
			pp.Println(err)
			return "", fmt.Errorf("Failed retrieving %s's information: %w", params.Name, err)
		}
		file, err := r8.CreateFileFromPath(ctx, params.Filepath, nil)
		if err != nil {
			return "", fmt.Errorf("CreateFileFromPath failed when passed with \"%s\": %w", params.Filepath, err)
		}
		input := replicate.PredictionInput{
			"audio": file,
		}
		input = params.InitRun(input)
		predictionOutput, err = r8.Run(ctx, params.Owner+"/"+params.Name+":"+model.LatestVersion.ID, input, nil)
		// these two are broken as far as I am concerned (err 422, 502):
		// 	→ prediction, err := r8.CreatePrediction(ctx, version, input, nil, false)
		// 	→ prediction, err := r8.CreatePredictionWithModel(ctx, "openai", "whisper", input, nil, false)
		
		if err == nil {
			break
		} else if errors.Is(err, context.DeadlineExceeded) {
			fmt.Fprintf(os.Stderr, "WARN: Timed out %s prediction (%d/%d)...\n", params.Name, try, params.MaxTry)
			if try+1 != params.MaxTry {
				delay := calcExponentialBackoff(try, baseDelay)
				time.Sleep(delay)
				continue
			}
			return "", fmt.Errorf("Timed out %s prediction after %d attempts: %w", params.Name, params.MaxTry, err)
		} else if errors.Is(err, context.Canceled) {
			return "", fmt.Errorf("Abort %s prediction: context cancelled: %v", params.Name, err)
		} else {
			pp.Println("RawPredictionErr", err)
			err, ok := err.(*replicate.ModelError)
			logs := ""
			if ok {
				logs = *err.Prediction.Logs
				if logs == err.Prediction.Error && strings.Contains(logs, ":") {
					e := strings.TrimPrefix(err.Prediction.Error.(string), "model error: ")
					err.Prediction.Error, _, _ = strings.Cut(e, ":")
				}
				s := "see below"
				err.Prediction.Logs = &s
			}
			pp.Println(err)
			color.Redln(strings.ReplaceAll(logs, "\n", "\n\t"))
			cancel()
			return "", fmt.Errorf("Failed %s prediction: %v", params.Name, err)
		}
		cancel()
	}
	//pp.Println("predictionOutput:", predictionOutput)
	str, err := params.Parser(predictionOutput)
	if err != nil {
		pp.Println(err)
		return "", fmt.Errorf("Parser failed: %w", err)
	}
	return str, nil
}

func r8RunWithAudioFileAndGET(params r8RunParams) ([]byte, error) {
	URL, err := r8RunWithAudioFile(params)
	if err != nil {
		return nil, err
	}
	
	resp, err := makeRequestWithRetry(URL, params.MaxTry)
	if err != nil {
		return nil, fmt.Errorf("Failed request on prediction output after %d attempts: %v", params.MaxTry, err)
	}
	defer resp.Body.Close()

	bar := progressbar.DefaultBytes(
		-1,
		"downloading",
	)
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


func makeRequestWithRetry(URL string, maxTry int) (*http.Response, error) {
	var resp *http.Response
	
	baseDelay := time.Millisecond * 500
	
	for attempt := 1; attempt <= maxTry; attempt++ {
		req, err := http.NewRequest("GET", URL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		resp, err = http.DefaultClient.Do(req)
		if err == nil {
			return resp, nil
		}
		
		if attempt == maxTry {
			return nil, fmt.Errorf("failed after %d attempts, last error: %w", maxTry, err)
		}
		
		// Check if the response body exists before trying to close it
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		
		delay := calcExponentialBackoff(attempt, baseDelay)
		
		fmt.Fprintf(os.Stderr, "WARN: Request failed (attempt %d/%d): %v. Retrying in %v...\n", 
			attempt, maxTry, err, delay)
		
		time.Sleep(delay)
	}
	
	return nil, fmt.Errorf("unexpected error in retry logic")
}

func calcExponentialBackoff(attempt int, baseDelay time.Duration) time.Duration {
	return time.Duration(math.Pow(2, float64(attempt))) * baseDelay
}

func placeholder5() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
