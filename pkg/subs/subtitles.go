package subs

import (
	"os"
	"io"
	"regexp"
	"strings"
	"encoding/csv"

	"github.com/k0kubun/pp"
	astisub "github.com/asticode/go-astisub"
)

// Subtitles represents a collection of subtitles corresponding to some media.
type Subtitles struct {
	*astisub.Subtitles
}

// OpenFile opens the given subtitles file for reading.
func OpenFile(filename string, clean bool) (*Subtitles, error) {
	subs, err := astisub.OpenFile(filename)
	if err != nil {
		return nil, err
	}

	subs.Items = merge(subs.Items)

	return &Subtitles{subs}, nil
}

func (subs *Subtitles) Subs2Dubs(outputFile string, FieldSep rune, idx int) {
	file, _ := os.Open(outputFile)
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = FieldSep
	dubbings := []string{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if len(row) < idx {
			//println("row=",len(row))
			continue
		}
		//fmt.Printf("%#v\n", row)
		pp.Println(row) // FIXME
		os.Exit(0)// FIXME
		whispered := row[10]
		dubbings = append(dubbings, whispered)
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
		(*subs).Items[i].Lines[0].Items[0].Text = dubbings[i]
	}
	// TODO subs.Write(strings.Replace(outputFile, ---, ----", 1))
}

func (subs *Subtitles) DumbDown2Dubs() *Subtitles {
	re := regexp.MustCompile(`^[\p{Z}\p{P}]*\[.*\][\p{P}\p{Z}]*$`) // TODO add "♪" → lyrics of BG music
	for _, item := range subs.Items {
		item.Lines = filterLines(item.Lines, re)
	}
	subs.Items = filterItems(subs.Items)
	return subs
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
