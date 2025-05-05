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
func isLangkitMadeTranslit(s string) bool {
	for _, t := range []TranslitType{Tokenize, Romanize, Selective, TokenizedSelective} {
		if strings.Contains(s, t.ToSuffix()) {
			return true
		}
	}
	return false
}

func isLangkitMadeMergedOutput(s string) bool {
	return strings.Contains(s, langkitMadeMergedMarker())
}