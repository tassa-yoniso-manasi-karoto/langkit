package extract

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"strconv"

	astisub "github.com/asticode/go-astisub"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/tassa-yoniso-manasi-karoto/subs2cards/pkg/subs"
	"github.com/tassa-yoniso-manasi-karoto/subs2cards/pkg/stt"
)

// ExportedItem represents the exported information of a single subtitle item,
// where Time is the primary field which identifies the item and ForeignCurr is
// the actual text of the item. The fields NativeCurr, NativePrev and NativeNext
// will be empty unless a second subtitle file was specified for the export and
// that second subtitle file is sufficiently aligned with the first.
type ExportedItem struct {
	Sound       string
	Time        string
	Source      string
	Image       string
	ForeignCurr string
	NativeCurr  string
	ForeignPrev string
	NativePrev  string
	ForeignNext string
	NativeNext  string
}

type SequenceT struct {
	StartAt        time.Duration
	EndAt          time.Duration
	MusicAndEffect string
	Files           map[string]PathFileT
	Item           ExportedItem
}


type PathFileT struct {
	Orig, Voiceover stringS
}


// ExportedItemWriter should write an exported item in whatever format is
// selected by the user.
type ExportedItemWriter func(*ExportedItem) error

// ExportItems calls the write function for each foreign subtitle item.
func (tsk *Task) ExportItems(foreignSubs, nativeSubs *subs.Subtitles, outputBase, mediaSourceFile, mediaPrefix string, write ExportedItemWriter) error {
	var master []SequenceT
	last := time.Duration(0)
	filler := func(last, first time.Duration) {
		if !IsZeroLengthTimespan(last, first) {
			master = append(master, SequenceT{
				StartAt: last,
				EndAt: first,
			})
		}
	}
	for i, foreignItem := range foreignSubs.Items {
		filler(last, foreignItem.StartAt)
		last = foreignItem.EndAt
		//---------------------------
		item, audiofile, err := tsk.ExportItem(foreignItem, nativeSubs, outputBase, mediaSourceFile, mediaPrefix)
		if err != nil {
			return fmt.Errorf("can't export item #%d: %s: %v", i+1, foreignItem.String(), err)
		}
		if tsk.STT {
			lang := tsk.Meta.AudioTracks[tsk.UseAudiotrack].Language
			item.ForeignCurr = stt.Replicate(audiofile, lang, "openai", "whisper", "")
			// stt.Replicate(filepath, lang, "vaibhavs10", "incredibly-fast-whisper", "")
		}
		sq := SequenceT{
			StartAt: foreignItem.StartAt,
			EndAt: foreignItem.EndAt,
			Files: make(map[string]PathFileT),
			Item: *item,
		}
		master = append(master, sq)
		if i > 0 {
			prevItem := foreignSubs.Items[i-1]
			item.ForeignPrev = prevItem.String()
		}

		if i+1 < len(foreignSubs.Items) {
			nextItem := foreignSubs.Items[i+1]
			item.ForeignNext = nextItem.String()
		}
		write(item)
	}
	end, _ := strconv.ParseFloat(tsk.Meta.GeneralTrack.Duration, 64)
	filler(last, time.Duration(end*float64(time.Second)))
	return nil
}

func (tsk *Task) ExportItem(foreignItem *astisub.Item, nativeSubs *subs.Subtitles, subsBase, mediaFile, mediaPrefix string) (*ExportedItem, string, error) {
	item := &ExportedItem{}
	item.Source = subsBase
	item.ForeignCurr = joinLines(foreignItem.String())

	if nativeSubs != nil {
		if nativeItem := nativeSubs.Translate(foreignItem); nativeItem != nil {
			item.NativeCurr = joinLines(nativeItem.String())
		}
	}
	audioFile := "" // FIXME rm this if: providinga .mp4 should not be optional
	if false && mediaPrefix != "" {
		audioFile, err := tsk.ExtractAudio("ogg", tsk.UseAudiotrack, tsk.Offset, foreignItem.StartAt, foreignItem.EndAt, mediaFile, mediaPrefix)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: can't extract audio: %v\n", err)
			os.Exit(1)
		}

		imageFile, err := tsk.ExtractImage(foreignItem.StartAt, foreignItem.EndAt, mediaFile, mediaPrefix)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: can't extract image: %v\n", err)
			os.Exit(1)
		}

		item.Time = timePosition(foreignItem.StartAt)
		item.Image = fmt.Sprintf("<img src=\"%s\">", path.Base(imageFile))
		item.Sound = fmt.Sprintf("[sound:%s]", path.Base(audioFile))
	}

	return item, audioFile, nil
}

// timePosition formats the given time.Duration as a time code which can safely
// be used in file names on all platforms.
func timePosition(d time.Duration) string {
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func joinLines(s string) string {
	s = strings.Replace(s, "\t", " ", -1)
	return strings.Replace(s, "\n", " ", -1)
}

func IsZeroLengthTimespan(last, t time.Duration) (b bool) {
	if t - last == 0 {
		b = true
	}
	return
}


func (sq *SequenceT) HasDialog() (b bool) {
	if sq.Files != nil {
		b = true
	}
	return
}

func placeholder4() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}


