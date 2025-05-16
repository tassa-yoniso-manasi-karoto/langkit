package summary

import (
	"strings"
	"unicode"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
)

// PrepareSubtitlesForSummary converts subtitle items to a format suitable for summarization
func PrepareSubtitlesForSummary(subtitles *subs.Subtitles) string {
	if subtitles == nil || len(subtitles.Items) == 0 {
		return ""
	}
	
	var builder strings.Builder
	
	// Process all items
	for i, item := range subtitles.Items {
		text := item.String()
		if text == "" {
			continue
		}
		
		// Clean and normalize the text
		text = cleanSubtitleText(text)
		
		// Add to builder with proper spacing
		if i > 0 && !endsWithPunctuation(builder.String()) {
			builder.WriteString(". ")
		} else if i > 0 {
			builder.WriteString(" ")
		}
		
		builder.WriteString(text)
	}
	
	return builder.String()
}

// cleanSubtitleText removes artifacts and normalizes text
func cleanSubtitleText(text string) string {
	// Remove HTML tags (simple approach)
	text = strings.ReplaceAll(text, "<i>", "")
	text = strings.ReplaceAll(text, "</i>", "")
	text = strings.ReplaceAll(text, "<b>", "")
	text = strings.ReplaceAll(text, "</b>", "")
	
	// Replace newlines with spaces
	text = strings.ReplaceAll(text, "\n", " ")
	
	// Remove multiple spaces
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	
	return strings.TrimSpace(text)
}

// endsWithPunctuation checks if the string ends with a punctuation mark
func endsWithPunctuation(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	
	lastChar := rune(s[len(s)-1])
	return strings.ContainsRune(".!?,:;", lastChar)
}

// GetCharacterList attempts to extract character names from subtitles
func GetCharacterList(subtitles *subs.Subtitles) []string {
	if subtitles == nil || len(subtitles.Items) == 0 {
		return nil
	}
	
	// Map to store potential character names and their occurrence count
	characterMap := make(map[string]int)
	
	for _, item := range subtitles.Items {
		text := item.String()
		
		// Look for patterns like "CHARACTER:" or "CHARACTER :"
		if parts := strings.Split(text, ":"); len(parts) > 1 {
			potential := strings.TrimSpace(parts[0])
			
			// Simple heuristic: all uppercase or title case, not too long
			if (isAllUpper(potential) || isTitleCase(potential)) && len(potential) < 30 {
				characterMap[potential]++
			}
		}
	}
	
	// Filter to characters with multiple occurrences
	var characters []string
	for char, count := range characterMap {
		if count >= 2 {
			characters = append(characters, char)
		}
	}
	
	return characters
}

// isAllUpper checks if a string is all uppercase
func isAllUpper(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) && !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

// isTitleCase checks if a string is in title case
func isTitleCase(s string) bool {
	words := strings.Fields(s)
	for _, word := range words {
		if len(word) > 0 {
			first := rune(word[0])
			if !unicode.IsUpper(first) {
				return false
			}
		}
	}
	return true
}