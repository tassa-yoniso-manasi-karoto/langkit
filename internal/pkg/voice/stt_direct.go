package voice

import (
	"context"
)

// This file contains backward-compatible functions for direct use of specific STT models.

func Whisper(ctx context.Context, filepath string, maxTry, timeout int, lang, initialPrompt string) (string, error) {
	provider, err := GetSpeechToTextProvider("whisper")
	if err != nil {
		return "", err
	}
	return provider.TranscribeAudio(ctx, filepath, lang, initialPrompt, maxTry, timeout)
}

func InsanelyFastWhisper(ctx context.Context, filepath string, maxTry, timeout int, lang string) (string, error) {
	provider, err := GetSpeechToTextProvider("incredibly-fast-whisper")
	if err != nil {
		return "", err
	}
	return provider.TranscribeAudio(ctx, filepath, lang, "", maxTry, timeout)
}

func Universal1(ctx context.Context, filepath string, maxTry, timeout int, lang string) (string, error) {
	provider, err := GetSpeechToTextProvider("universal-1")
	if err != nil {
		return "", err
	}
	return provider.TranscribeAudio(ctx, filepath, lang, "", maxTry, timeout)
}

func GPT4oTranscribe(ctx context.Context, filepath string, maxTry, timeout int, lang, initialPrompt string) (string, error) {
	provider, err := GetSpeechToTextProvider("gpt-4o-transcribe")
	if err != nil {
		return "", err
	}
	return provider.TranscribeAudio(ctx, filepath, lang, initialPrompt, maxTry, timeout)
}

func GPT4oMiniTranscribe(ctx context.Context, filepath string, maxTry, timeout int, lang, initialPrompt string) (string, error) {
	provider, err := GetSpeechToTextProvider("gpt-4o-mini-transcribe")
	if err != nil {
		return "", err
	}
	return provider.TranscribeAudio(ctx, filepath, lang, initialPrompt, maxTry, timeout)
}

func ElevenLabsScribe(ctx context.Context, filepath string, maxTry, timeout int, lang string) (string, error) {
	provider, err := GetSpeechToTextProvider("scribe")
	if err != nil {
		return "", err
	}
	return provider.TranscribeAudio(ctx, filepath, lang, "", maxTry, timeout)
}
