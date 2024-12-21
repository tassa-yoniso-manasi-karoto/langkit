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

	subs.Items = merge(subs.Items)

	return &Subtitles{subs}, nil
}

func (subs *Subtitles) Subs2Dubs(outputFile, FieldSep string) (err error) {
	transcriptedLines := loadSTTfromTSV(outputFile, FieldSep)
	if len(transcriptedLines) != len((*subs).Items) {
		return fmt.Errorf("The number of STT transcriptions doesn't match the number of subtitle lines." +
				" This is most likely a bug, please report it.\n" +
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
// edit: atm on-the-fly writing to TSV is gone due to parallelization implementation
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
			//println("row=",len(row))
			continue
		}
		//fmt.Printf("%#v\n", row)
		//pp.Println(row)
		transcriptedLines = append(transcriptedLines, row[4])
	}
	return
}

func (subs *Subtitles) TrimCC2Dubs() {
	re := regexp.MustCompile(`^[\p{Z}\p{P}]*\[.*\][\p{P}\p{Z}]*$`) // TODO add "♪" → lyrics of BG music
	for i, item := range subs.Items {
		subs.Items[i].Lines = filterLines(item.Lines, re)
	}
	subs.Items = filterItems(subs.Items)
	return
}

// Translate generates a new subtitle from all subtitles which overlap with the given item.
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

func merge(items []*astisub.Item) []*astisub.Item {
	mergedItems := []*astisub.Item{}

	var last *astisub.Item
	for _, item := range items {
		// Entirely empty items may mean that a new sub section is about to begin
		if last != nil && len(last.Lines) == 0 {
			mergeWithPrev(last, item)
			continue
		}

		// Drop lines that are just repeated from the previous sub
		if last != nil && len(item.Lines) > 0 {
			removeOverlap(last, item)
			if len(item.Lines) == 0 {
				mergeWithPrev(last, item)
				continue
			}
		}

		mergedItems = append(mergedItems, item)
		last = item
	}

	return mergedItems
}


func filterLines(lines []astisub.Line, re *regexp.Regexp) []astisub.Line {
	var filtered []astisub.Line
	for _, line := range lines {
		if !re.MatchString(line.String()) {
			text := strings.TrimSpace(line.String())
			if text != "" {
				filtered = append(filtered, astisub.Line{Items: []astisub.LineItem{{Text: text}}})
			}
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


func mergeWithPrev(prev *astisub.Item, next *astisub.Item) {
	prev.Lines = append(prev.Lines, next.Lines...)
	prev.EndAt = next.EndAt
}

func removeOverlap(prev *astisub.Item, next *astisub.Item) {
	for n := min(len(prev.Lines), len(next.Lines)); n > 0; n-- {
		if checkOverlap(prev.Lines, next.Lines, n) {
			next.Lines = next.Lines[n:]
			return
		}
	}
}

func checkOverlap(prev, next []astisub.Line, n int) bool {
	for i, j := len(prev)-n, 0; i >= 0 && j <= len(next)-1; i, j = i-1, j+1 {
		if prev[i].String() != next[j].String() {
			return false
		}
	}
	return true
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}


func placeholder() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}

