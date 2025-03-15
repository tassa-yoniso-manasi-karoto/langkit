package subs

import (
	"fmt"
	"os"
	"io"
	"regexp"
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


// Parsing the TSV instead of processing on the fly ensure dubtitle integrity
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
	// \p{Z} → unicode whiteshape/invisible; \p{P} → unicode punct
	re := `[\p{Z}\p{P}]*[([][^()[\]]*[)\]][\p{Z}\p{P}]*`
	
	// Remove leading group matches like "(...)" or "[...]" at the beginning
	reLeadingGroup := regexp.MustCompile("^" + re)
	// Remove trailing group matches like "(...)" or "[...]" at the end
	reTrailingGroup := regexp.MustCompile(re + "$")
	
	filterFunc := func(s string) string {
		// maybe users want lyrics
		// if strings.Contains(s, "♪") {
		// 	return ""
		// }
		s = reLeadingGroup.ReplaceAllString(s, "")
		return reTrailingGroup.ReplaceAllString(s, "")
	}
	for i, item := range subs.Items {
		subs.Items[i].Lines = filterLines(item.Lines, filterFunc)
	}
	subs.Items = filterItems(subs.Items)
	return
}

func filterLines(lines []astisub.Line, fn func(string) string) []astisub.Line {
	var filtered []astisub.Line
	for _, line := range lines {
		text := fn(line.String())
		text = strings.TrimSpace(text)
		if text != "" {
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


func placeholder() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}

