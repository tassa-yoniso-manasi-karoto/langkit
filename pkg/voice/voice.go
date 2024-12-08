package voice

import (
	"context"
	"os"
	"time"
	"strings"
	"fmt"
	"io"
	"net/http"
	
	"github.com/tassa-yoniso-manasi-karoto/elevenlabs-go"
	
	"github.com/schollz/progressbar/v3"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/rivo/uniseg"
	"github.com/sergi/go-diff/diffmatchpatch"
	replicate "github.com/replicate/replicate-go"
	aai "github.com/AssemblyAI/assemblyai-go-sdk"
)


func ElevenlabsIsolator(filePath string, timeout int) ([]byte, error) {
	client := elevenlabs.NewClient(context.Background(), os.Getenv("ELEVENLABS_API_TOKEN"), time.Duration(timeout)*time.Second)
	audio, err := client.VoiceIsolator(filePath)
	if err != nil {
		return nil, fmt.Errorf("API query failed: %w", err)
	}
	return audio, nil
}


func Universal1(filepath string, maxTry, timeout int, lang string) (string, error) {
	apiKey := os.Getenv("ASSEMBLYAI_API_KEY")
	client := aai.NewClient(apiKey)

	f, err := os.Open(filepath)
	if err != nil {
		return "", fmt.Errorf("Couldn't open audio file:", err)
	}
	defer f.Close()

	for try := 0; try < maxTry; try++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
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

func Whisper(filepath string, maxTry, timeout int, lang, initialPrompt string) ([]byte, error) {
	initRun := func(input replicate.PredictionInput) replicate.PredictionInput {
		input["language"] = lang
		if initialPrompt != "" {
			input["initial_prompt"] = initialPrompt
		}
		return input
	}
	return r8RunWithAudioFile(filepath, maxTry, timeout, "openai", "whisper", initRun, whisperParser)
}

func InsanelyFastWhisper(filepath string, maxTry, timeout int, lang string) ([]byte, error) {
	initRun := func(input replicate.PredictionInput) replicate.PredictionInput {
		input["language"] = lang
		return input
	}
	// model name is outdated on replicate
	return r8RunWithAudioFile(filepath, maxTry, timeout, "vaibhavs10", "incredibly-fast-whisper", initRun, whisperParser)
}

func Spleeter(filepath string, maxTry, timeout int) ([]byte, error) {
	NoMoreInput := func(input replicate.PredictionInput) replicate.PredictionInput {
		return input
	}
	return r8RunWithAudioFile(filepath, maxTry, timeout, "soykertje", "spleeter", NoMoreInput, spleeterDemucsParser)
}


func Demucs(filepath, ext string, maxTry, timeout int, wantFinetuned bool) ([]byte, error) {
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
	return r8RunWithAudioFile(filepath, maxTry, timeout, "ryan5453", "demucs", initRun, spleeterDemucsParser)
}


func r8RunWithAudioFile(filepath string, maxTry, timeout int, owner, name string, initRun initRunT, parser parserT) ([]byte, error) {
	apiToken := os.Getenv("REPLICATE_API_TOKEN")
	if apiToken == "" {
		return nil, fmt.Errorf("Please set the REPLICATE_API_TOKEN environment variable")
	}
	var predictionOutput replicate.PredictionOutput
	for try := 0; try < maxTry; try++ {
		r8, err := replicate.NewClient(replicate.WithToken(apiToken))
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
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
			fmt.Printf("Timed out %s prediction (%d/%d)...\n", name, try, maxTry)
			if try+1 != maxTry {
				continue
			}
			return nil, fmt.Errorf("Timed out %s prediction after %d attempts: %w", name, maxTry, err)
		} else {
			pp.Println(err)
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
	req, _ := http.NewRequest("GET", URL, nil)
	resp, _ := http.DefaultClient.Do(req)
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


func placeholder5() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
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

const SEP = "𓃰"

// Compute Character Error Rate (CER)
func computeCER(ref, hyp string) float64 {
	refTokens := tokenizeChars(ref)
	hypTokens := tokenizeChars(hyp)
	refStr := strings.Join(refTokens, SEP)
	hypStr := strings.Join(hypTokens, SEP)

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(refStr, hypStr, false)
	distance := dmp.DiffLevenshtein(diffs)

	cer := float64(distance) / float64(len(refTokens))
	return cer
}


// Tokenize the input string into grapheme clusters (characters)
func tokenizeChars(s string) []string {
	var chars []string
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		chars = append(chars, gr.Str())
	}
	return chars
}
