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
	
	"github.com/tassa-yoniso-manasi-karoto/elevenlabs-go"
	
	"github.com/schollz/progressbar/v3"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	//"github.com/rivo/uniseg"
	//"github.com/sergi/go-diff/diffmatchpatch"
	replicate "github.com/replicate/replicate-go"
	aai "github.com/AssemblyAI/assemblyai-go-sdk"
)


func ElevenlabsIsolator(ctx context.Context, filePath string, timeout int) ([]byte, error) {
	client := elevenlabs.NewClient(ctx, os.Getenv("ELEVENLABS_API_TOKEN"), time.Duration(timeout)*time.Second)
	audio, err := client.VoiceIsolator(filePath)
	if err != nil {
		return nil, fmt.Errorf("API query failed: %w", err)
	}
	return audio, nil
}


func Universal1(ctx context.Context, filepath string, maxTry, timeout int, lang string) (string, error) {
	apiKey := os.Getenv("ASSEMBLYAI_API_KEY")
	client := aai.NewClient(apiKey)

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
	b, err := r8RunWithAudioFile(ctx, filepath, maxTry, timeout, "openai", "whisper", initRun, whisperParser)
	return string(b), err
}

func InsanelyFastWhisper(ctx context.Context, filepath string, maxTry, timeout int, lang string) (string, error) {
	initRun := func(input replicate.PredictionInput) replicate.PredictionInput {
		input["language"] = lang
		return input
	}
	// model name is outdated on replicate
	b, err := r8RunWithAudioFile(ctx, filepath, maxTry, timeout, "vaibhavs10", "incredibly-fast-whisper", initRun, whisperParser)
	return string(b), err
}

func Spleeter(ctx context.Context, filepath string, maxTry, timeout int) ([]byte, error) {
	NoMoreInput := func(input replicate.PredictionInput) replicate.PredictionInput {
		return input
	}
	return r8RunWithAudioFile(ctx, filepath, maxTry, timeout, "soykertje", "spleeter", NoMoreInput, spleeterDemucsParser)
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
	return r8RunWithAudioFile(ctx, filepath, maxTry, timeout, "ryan5453", "demucs", initRun, spleeterDemucsParser)
}


func r8RunWithAudioFile(ctx context.Context, filepath string, maxTry, timeout int, owner, name string, initRun initRunT, parser parserT) ([]byte, error) {
	apiToken := os.Getenv("REPLICATE_API_TOKEN")
	if apiToken == "" {
		return nil, fmt.Errorf("Please set the REPLICATE_API_TOKEN environment variable")
	}
	var predictionOutput replicate.PredictionOutput
	baseDelay := time.Millisecond * 500
	for try := 0; try < maxTry; try++ {
		r8, err := replicate.NewClient(replicate.WithToken(apiToken))
		ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		
		model, err := r8.GetModel(ctx, owner, name)
		if err != nil {
			pp.Println(err)
			return nil, fmt.Errorf("Failed retrieving %s's information: %w", name, err)
		}
		file, err := r8.CreateFileFromPath(ctx, filepath, nil)
		if err != nil {
			return nil, fmt.Errorf("CreateFileFromPath failed when passed with \"%s\": %w", filepath, err)
		}
		input := replicate.PredictionInput{
			"audio": file,
		}
		input = initRun(input)
		predictionOutput, err = r8.Run(ctx, owner+"/"+name+":"+model.LatestVersion.ID, input, nil)
		// these two are broken as far as I am concerned (err 422, 502):
		// 	→ prediction, err := r8.CreatePrediction(ctx, version, input, nil, false)
		// 	→ prediction, err := r8.CreatePredictionWithModel(ctx, "openai", "whisper", input, nil, false)
		
		if err == nil {
			break
		} else if err == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "WARN: Timed out %s prediction (%d/%d)...\n", name, try, maxTry)
			if try+1 != maxTry {
				delay := calcExponentialBackoff(try, baseDelay)
				time.Sleep(delay)
				continue
			}
			return nil, fmt.Errorf("Timed out %s prediction after %d attempts: %w", name, maxTry, err)
		} else {
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
			return nil, fmt.Errorf("Failed %s prediction: %w", name, err)
		}
	}
	pp.Println(predictionOutput)
	URL, err := parser(predictionOutput)
	if err != nil {
		pp.Println(err)
		return nil, fmt.Errorf("Parser failed: %w", err)
	}
	// Download file made by API located at URL
	resp, err := makeRequestWithRetry(URL, maxTry)
	if err != nil {
		return nil, fmt.Errorf("Failed request on prediction output after %d attempts: %v", maxTry, err)
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
