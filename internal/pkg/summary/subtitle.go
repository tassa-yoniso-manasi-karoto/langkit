package summary

import (
	"regexp"
	"strings"
	// "unicode" // Not strictly needed for the simplified cleaning

	"github.com/asticode/go-astisub"
	"github.com/rs/zerolog"
)

// Package-level logger, to be initialized by summary.Initialize()
var logger zerolog.Logger 

const (
	maxInputCharsForLLMWarning = 300000 
)

// PrepareSubtitlesForSummary converts astisub.Subtitles items to a single
// coherent string suitable for LLM summarization, retaining speaker labels and CC noise.
// Returns:
//   - concatenatedText: The prepared string of subtitle content.
func PrepareSubtitlesForSummary(originalSubtitles *astisub.Subtitles) (concatenatedText string) {
	if originalSubtitles == nil || len(originalSubtitles.Items) == 0 {
		return ""
	}

	var builder strings.Builder

	for _, item := range originalSubtitles.Items {
		itemText := item.String() 
		if itemText == "" {
			continue
		}

		cleanedItemText := cleanSubtitleTextForLLMSummary(itemText)
		if cleanedItemText == "" {
			continue
		}

		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(cleanedItemText)
	}

	finalText := builder.String()

	if len(finalText) > maxInputCharsForLLMWarning {
		logger.Warn().Int("char_count", len(finalText)).
			Msg("Prepared subtitle text is very long. Summarization might be slow, costly, or exceed LLM context limits.")
	}

	return strings.TrimSpace(finalText)
}

// cleanSubtitleTextForLLMSummary (definition remains the same as previously provided)
var (
	simpleStylingTagRegex = regexp.MustCompile(`</?(i|b|u|font)(\s+[^>]*)?>`)
	tabToSpaceRegex       = regexp.MustCompile(`\t+`)
	multipleSpacesRegex   = regexp.MustCompile(` {2,}`) 
	leadingTrailingNewlinesRegex = regexp.MustCompile(`^\s*\n|\n\s*$`)
	lineWhitespaceRegex = regexp.MustCompile(`^[ \t]+|[ \t]+$`)
)

func cleanSubtitleTextForLLMSummary(text string) string {
	text = simpleStylingTagRegex.ReplaceAllString(text, "")
	text = tabToSpaceRegex.ReplaceAllString(text, " ")
	lines := strings.Split(text, "\n")
	cleanedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmedLine := lineWhitespaceRegex.ReplaceAllString(line, "")
		normalizedLine := multipleSpacesRegex.ReplaceAllString(trimmedLine, " ")
		if normalizedLine != "" { 
			cleanedLines = append(cleanedLines, normalizedLine)
		}
	}
	text = strings.Join(cleanedLines, "\n")
	text = leadingTrailingNewlinesRegex.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}