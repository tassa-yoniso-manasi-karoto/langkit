package subs

import (
	"fmt"
	"os"
	"io"
	"unicode"
	"strings"
	"encoding/csv"

	"github.com/gookit/color"
	"github.com/k0kubun/pp"
	astisub "github.com/asticode/go-astisub"
)

type Subtitles struct {
	*astisub.Subtitles
}

func OpenFile(filename string, clean bool) (*Subtitles, error) {
	subs, err := astisub.OpenFile(filename)
	if err != nil {
		return nil, err
	}
	return &Subtitles{subs}, nil
}

func (subs *Subtitles) Write(filename string) error {
	if err := subs.Subtitles.Write(filename); err != nil {
		return err
	}
	return nil
}

func (subs *Subtitles) Subs2Dubs(outputFile, FieldSep string) (err error) {
	transcriptedLines := loadSTTfromTSV(outputFile, FieldSep)
	if len(transcriptedLines) != len((*subs).Items) {
		return fmt.Errorf("The number of STT transcriptions doesn't match the number of subtitle lines." +
				" This is a bug, please report it.\n" +
					"len transcriptions=%d\tlen sub lines=%d", len(transcriptedLines),len((*subs).Items))
	}
	for i, item := range (*subs).Items {
		if len(item.Lines) == 0 {
			continue
		}
		// clear the lines except the first
		(*subs).Items[i].Lines = []astisub.Line{item.Lines[0]}
		if len(item.Lines[0].Items) == 0 {
			continue
		}
		// clear the items of that first line except the first because
		// bunkai/subs2cards merge LineItems together in one field in outputFile
		(*subs).Items[i].Lines[0].Items = []astisub.LineItem{item.Lines[0].Items[0]}
		(*subs).Items[i].Lines[0].Items[0].Text = transcriptedLines[i]
	}
	return nil
}


// Parsing the TSV instead of processing on the fly ensure dubtitles integrity
// in cases where the processing is restarted after an interruption or crash
func loadSTTfromTSV(outputFile, FieldSep string) (transcriptedLines []string) {
	file, _ := os.Open(outputFile)
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = rune(FieldSep[0])
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if len(row) < 5 {
			continue
		}
		transcriptedLines = append(transcriptedLines, row[4])
	}
	return
}

// Translate generates a new subtitle from all subtitles which overlap with the given item.
// FIXME 
func (subs *Subtitles) Translate(item *astisub.Item) *astisub.Item {
	newItem := &astisub.Item{}

	for _, item2 := range subs.Items {
		if item2.StartAt >= item.EndAt {
			break
		}

		if item2.StartAt >= item.StartAt || item2.EndAt > item.StartAt {
			newItem.Lines = append(newItem.Lines, item2.Lines...)
		}
	}

	return newItem
}

func (subs *Subtitles) TrimCC2Dubs() {
	for i, item := range subs.Items {
		subs.Items[i].Lines = filterLines(item.Lines)
	}
	subs.Items = filterItems(subs.Items)
	return
}

func filterLines(lines []astisub.Line) []astisub.Line {
	var filtered []astisub.Line
	for _, line := range lines {
		text := removeLeadingGroup(line.String())
		text = removeTrailingGroup(text)
		text = strings.TrimSpace(text)
		
		if text != "" && !isNonLexicalContent(text) {
			filtered = append(filtered, astisub.Line{Items: []astisub.LineItem{{Text: text}}})
		}
	}
	return filtered
}

func filterItems(items []*astisub.Item) []*astisub.Item {
	var filtered []*astisub.Item
	for _, item := range items {
		if len(item.Lines) > 0 {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func isNonLexicalContent(s string) bool {
	// Check for strings consisting only of punctuation and/or symbols
	runes := []rune(s)
	onlyPunctAndSymbols := true
	for _, r := range runes {
		if !unicode.IsPunct(r) && !unicode.IsSymbol(r) && !unicode.IsSpace(r) {
			onlyPunctAndSymbols = false
			break
		}
	}
	if onlyPunctAndSymbols && len(s) > 0 {
		return true
	}
	return false
}

// Function to remove parenthetical groups at the beginning of a string
func removeLeadingGroup(s string) string {
	if len(s) == 0 {
		return s
	}

	runes := []rune(s)
	start := -1

	// Find first opening parenthesis after any whitespace/punctuation
	for i, r := range runes {
		if unicode.Is(unicode.Ps, r) {
			start = i
			break
		} else if !unicode.IsSpace(r) && !unicode.IsPunct(r) {
			// Stop if we hit non-whitespace, non-punct before an opening parenthesis
			return s
		}
	}

	// No opening parenthesis found at the beginning
	if start == -1 {
		return s
	}

	// Track nesting depth
	depth := 1
	end := -1

	// Find matching closing parenthesis
	for i := start + 1; i < len(runes); i++ {
		if unicode.Is(unicode.Ps, runes[i]) {
			depth++
		} else if unicode.Is(unicode.Pe, runes[i]) {
			depth--
			if depth == 0 {
				end = i
				break
			}
		}
	}

	// If no matching closing parenthesis found, be tolerant and return original
	if end == -1 {
		return s
	}

	// Find first non-whitespace/punct character after the closing parenthesis
	contentStart := -1
	for i := end + 1; i < len(runes); i++ {
		if !unicode.IsSpace(runes[i]) && !unicode.IsPunct(runes[i]) {
			contentStart = i
			break
		}
	}

	// If nothing after the parenthetical group, return empty string
	if contentStart == -1 {
		return ""
	}

	// Return everything after the parenthetical group
	return string(runes[contentStart:])
}

// Function to remove parenthetical groups at the end of a string
func removeTrailingGroup(s string) string {
	if len(s) == 0 {
		return s
	}

	runes := []rune(s)
	n := len(runes)
	end := -1

	// Find last closing parenthesis before any trailing whitespace/punctuation
	for i := n - 1; i >= 0; i-- {
		if unicode.Is(unicode.Pe, runes[i]) {
			end = i
			break
		} else if !unicode.IsSpace(runes[i]) && !unicode.IsPunct(runes[i]) {
			// Stop if we hit non-whitespace, non-punct before a closing parenthesis
			return s
		}
	}

	// No closing parenthesis found at the end
	if end == -1 {
		return s
	}

	// Track nesting depth
	depth := 1
	start := -1

	// Find matching opening parenthesis
	for i := end - 1; i >= 0; i-- {
		if unicode.Is(unicode.Pe, runes[i]) {
			depth++
		} else if unicode.Is(unicode.Ps, runes[i]) {
			depth--
			if depth == 0 {
				start = i
				break
			}
		}
	}

	// If no matching opening parenthesis found, be tolerant and return original
	if start == -1 {
		return s
	}

	// Find last non-whitespace/punct character before the opening parenthesis
	contentEnd := -1
	for i := start - 1; i >= 0; i-- {
		if !unicode.IsSpace(runes[i]) && !unicode.IsPunct(runes[i]) {
			contentEnd = i
			break
		}
	}

	// If nothing before the parenthetical group, return empty string
	if contentEnd == -1 {
		return ""
	}

	// Return everything before the parenthetical group
	return string(runes[:contentEnd+1])
}



func placeholder() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}

