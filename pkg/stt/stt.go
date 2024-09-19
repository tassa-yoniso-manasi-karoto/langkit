package stt

import (
	"context"
	"os"
	"time"
	"strings"
	
	replicate "github.com/replicate/replicate-go"
	"github.com/k0kubun/pp"
	"github.com/rs/zerolog"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/rivo/uniseg"
)

// google offers STT but it doesn't have a great reputation
// https://cloud.google.com/speech-to-text/docs/speech-to-text-client-libraries#client-libraries-usage-go

var logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()


func Replicate(filepath, lang, owner, name, initialPrompt string) string {
	apiToken := os.Getenv("REPLICATE_API_TOKEN")
	if apiToken == "" {
		logger.Fatal().Msg("Please set the REPLICATE_API_TOKEN environment variable")
	}

	r8, err := replicate.NewClient(replicate.WithToken(apiToken))
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
	defer cancel()
	
	model, err := r8.GetModel(ctx, owner, name)
	if err != nil {
		pp.Println(err)
		logger.Fatal().Msg("Failed retrieving model's information")
	}
	file, _ := r8.CreateFileFromPath(ctx, filepath, nil)

	input := replicate.PredictionInput{
		"audio": file,
		"language": lang,
	}
	if initialPrompt != "" {
		input["initial_prompt"] = initialPrompt
	}
	// TODO Loop in case it fails
	predictionOutput, err := r8.Run(ctx, owner+"/"+name+":"+model.LatestVersion.ID, input, nil)
	// these two are broken as far as I am concerned (err 422, 502):
	// 	→ prediction, err := r8.CreatePrediction(ctx, version, input, nil, false)
	// 	→ prediction, err := r8.CreatePredictionWithModel(ctx, "openai", "whisper", input, nil, false)
	if err != nil {
		pp.Println(err)
		logger.Fatal().Msg("Failed prediction")
	}
	transcription := predictionOutput.(map[string]interface{})["transcription"].(string)
	return transcription
}

/*
	logger.Debug().
		Int("len", len(tStack)).
		Int("scope", scope).
		Int("idxMax", idxMax).
		Msg("superior")
*/


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