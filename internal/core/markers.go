package core

import (
	"strings"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
)


func langkitMadeDubtitlesMarker(STTModel string) string {
	return "." + strings.ToUpper(STTModel)
}

func langkitMadeMergedMarker() string {
	return ".MERGED"
}


func langkitMadeVocalsOnlyMarker(SeparationLib string) string {
	return ".VOCALS." +  strings.ToUpper(SeparationLib) + "."
}


func langkitMadeEnhancedMarker() string {
	return ".VOICES.ENHANCED"
}

// allows rejecting those files during subfile screening (see lang.go)
func isLangkitMadeDubtitles(s string) bool {
	for _, model := range voice.GetAllSTTModels() {
		if strings.Contains(s, langkitMadeDubtitlesMarker(model.Name)) {
			return true
		}
	}
	
	return false
}
// allows rejecting those files during subfile screening (see lang.go)
// Checks for the transliteration suffix pattern (e.g., "_tokenized", "_romanized")
// regardless of file extension.
func isLangkitMadeTranslit(s string) bool {
	for _, t := range []TranslitType{Tokenize, Romanize, Selective, TokenizedSelective} {
		// Check for the transliteration type suffix (e.g., "_tokenized", "_romanized")
		if strings.Contains(s, "_"+t.String()) {
			return true
		}
	}
	return false
}

func isLangkitMadeMergedOutput(s string) bool {
	return strings.Contains(s, langkitMadeMergedMarker())
}

// isLangkitMadeAudioArtifact returns true if the filename matches any
// Langkit-generated audio output (vocals-only, enhanced, or merged).
func isLangkitMadeAudioArtifact(s string) bool {
	upper := strings.ToUpper(s)
	if strings.Contains(upper, ".VOCALS.") {
		return true
	}
	if strings.Contains(upper, langkitMadeEnhancedMarker()) {
		return true
	}
	if strings.Contains(upper, langkitMadeMergedMarker()) {
		return true
	}
	return false
}