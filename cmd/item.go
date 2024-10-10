package cmd

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"path/filepath"

	astisub "github.com/asticode/go-astisub"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/subs"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/voice"
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

// ExportedItemWriter should write an exported item in whatever format is // selected by the user.
type ExportedItemWriter func(*ExportedItem)


// ExportItems calls the write function for each foreign subtitle item.
// FIXME get rid of useless params
func (tsk *Task) ExportItems(foreignSubs, nativeSubs *subs.Subtitles, outputBase, mediaSourceFile, mediaPrefix string, write ExportedItemWriter) {
	// Create channels
	inputChan := make(chan string)
	resultChan := make(chan bool)
	defer close(inputChan)
	defer close(resultChan)
	go checkStringsInFile(tsk.outputFile(), inputChan, resultChan)
	total := 0
	skipped := 0
	for i, foreignItem := range foreignSubs.Items {
		item, audiofile, err := tsk.ExportItem(foreignItem, nativeSubs, outputBase, mediaSourceFile, mediaPrefix)
		total += 1
		// skip lines already in file, if the previous run wasn't completed
		if inputChan <- tsk.FieldSep + item.Time + tsk.FieldSep; <-resultChan {
			skipped += 1
			continue
		}
		if err != nil {
			tsk.Log.Error().
				Int("srt row", i).
				Str("item", foreignItem.String()).
				Err(err).
				Msg("can't export item")
		}
		lang := tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language
		switch tsk.STT {
		case "whisper":
			b, err := voice.Whisper(audiofile, 5, tsk.TimeoutSTT, lang.Part1, "")
			if err != nil {
				tsk.Log.Error().Err(err).
					Str("item", foreignItem.String()).
					Msg("Whisper error")
			}
			item.ForeignCurr = string(b)
		case "insanely-fast-whisper":
			b, err := voice.InsanelyFastWhisper(audiofile, 5, tsk.TimeoutSTT, lang.Part1)
			if err != nil {
				tsk.Log.Error().Err(err).
					Str("item", foreignItem.String()).
					Msg("InsanelyFastWhisper error")
			}
			item.ForeignCurr = string(b)
		case "universal-1":
			s, err := voice.Universal1(audiofile, 5, tsk.TimeoutSTT, lang.Part1)
			if err != nil {
				tsk.Log.Error().Err(err).
					Str("item", foreignItem.String()).
					Msg("Universal1 error")
			}
			item.ForeignCurr = s
		}
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
	if skipped != 0 {
		fmt.Printf("%.1f%% of items were already done and skipped (%d/%d)\n",
			float64(skipped)/float64(total)*100, skipped, total)
	}
	tsk.ConcatWAVstoOGG("CONDENSED", mediaPrefix)
	return
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
	audioFile, err := media.ExtractAudio("ogg", tsk.UseAudiotrack, tsk.Offset, foreignItem.StartAt, foreignItem.EndAt, mediaFile, mediaPrefix, tsk.DubsOnly)
	if err != nil {
		tsk.Log.Error().Err(err).Msg("can't extract ogg audio")
	}
	if !tsk.DubsOnly {
		_, err = media.ExtractAudio("wav", tsk.UseAudiotrack, time.Duration(0), foreignItem.StartAt, foreignItem.EndAt, mediaFile, mediaPrefix, false)
		if err != nil {
			tsk.Log.Error().Err(err).Msg("can't extract wav audio")
		}
	}
	imageFile, err := media.ExtractImage(foreignItem.StartAt, foreignItem.EndAt, mediaFile, mediaPrefix, tsk.DubsOnly)
	if err != nil {
		tsk.Log.Error().Err(err).Msg("can't extract image")
	}

	item.Time = timePosition(foreignItem.StartAt)
	item.Image = fmt.Sprintf("<img src=\"%s\">", path.Base(imageFile))
	item.Sound = fmt.Sprintf("[sound:%s]", path.Base(audioFile))

	return item, audioFile, nil
}





func (tsk *Task) ConcatWAVstoOGG(suffix, mediaPrefix string) {
	out := fmt.Sprint(mediaPrefix, ".", suffix,".ogg")
	if  _, err := os.Stat(out); err == nil {
		return
	}
	wavFiles, err := filepath.Glob(mediaPrefix+ "_*.wav")
	if err != nil {
		tsk.Log.Error().Err(err).
			Str("mediaOutputDir", tsk.mediaOutputDir()).
			Msg("Error searching for .wav files")
	}

	if len(wavFiles) == 0 {
		tsk.Log.Warn().
			Str("mediaOutputDir", tsk.mediaOutputDir()).
			Msg("No .wav files found")
	}
	// Generate the concat list for ffmpeg
	concatFile, err := media.CreateConcatFile(wavFiles)
	if err != nil {
		tsk.Log.Error().Err(err).Msg("Error creating temporary concat file")
	}
	defer os.Remove(concatFile)

	// Run FFmpeg to concatenate and create the audio file
	media.RunFFmpegConcat(concatFile, mediaPrefix+".wav")

	// Convert WAV to OPUS using FFmpeg
	media.RunFFmpegConvert(mediaPrefix+".wav", out)
	// Clean up
	os.Remove(mediaPrefix+".wav")
	for _, f := range wavFiles {
		if err := os.Remove(f); err != nil {
			tsk.Log.Warn().Str("file", f).Msg("Removing file failed")
		}
	}
}


// Function that runs in a goroutine to check multiple strings in a file
func checkStringsInFile(filepath string, inputChan <-chan string, resultChan chan<- bool) error {
	content, _ := os.ReadFile(filepath)
	fileContent := string(content)
	for searchString := range inputChan {
		resultChan <- strings.Contains(fileContent, searchString)
	}
	return nil
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



func placeholder4() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}


