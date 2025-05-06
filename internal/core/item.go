package core

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"path/filepath"
	"io/fs"
	"errors"
	"context"

	//astisub "github.com/asticode/go-astisub"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)

// ProcessedItem represents the exported information of a single subtitle item.
type ProcessedItem struct {
	Index       int
	AlreadyDone bool
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
	StartTime   time.Duration // Used for tracking when the subtitle starts
}

func (tsk *Task) ProcessItem(ctx context.Context, indexedSub IndexedSubItem) (item ProcessedItem, procErr *ProcessingError) {
	reporter := crash.Reporter
	
	// CAVEAT: astisub.Item has an "index" field and so does our IndexedSubItem
	foreignItem := indexedSub.Item
	childCtx, childCancel := context.WithCancel(ctx)
	defer childCancel()
	
	item.Source = tsk.outputBase()
	item.ForeignCurr = joinLines(foreignItem.String())

	if tsk.NativeSubs != nil {
		// the nativeSubs have not been trimmed if targetSubs is CC but
		// it's fine because nativeSubs rely on matching timmings
		if nativeItem := tsk.NativeSubs.Translate(foreignItem); nativeItem != nil {
			item.NativeCurr = joinLines(nativeItem.String())
		}
	}
	audiofile, err := media.ExtractAudio("ogg", tsk.UseAudiotrack,
		tsk.Offset, foreignItem.StartAt, foreignItem.EndAt,
			tsk.MediaSourceFile, tsk.MediaPrefix, false)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("can't extract ogg audio")
	}
	if tsk.WantCondensedAudio {
		_, err = media.ExtractAudio("wav", tsk.UseAudiotrack,
			time.Duration(0), foreignItem.StartAt, foreignItem.EndAt,
				tsk.MediaSourceFile, tsk.MediaPrefix, false)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			tsk.Handler.ZeroLog().Error().Err(err).Msg("can't extract wav audio")
		}
	}
	dryRun := tsk.Mode != Subs2Cards
	imageFile, err := media.ExtractImage(foreignItem.StartAt, foreignItem.EndAt,
		tsk.MediaSourceFile, tsk.MediaPrefix, dryRun)
	if err != nil {
		// determining AlreadyDone is done on the AVIF because it is the most
		// computing intensive part of each item's processing
		if errors.Is(err, fs.ErrExist) {
			item.AlreadyDone = true
		} else {
			tsk.Handler.ZeroLog().Error().Err(err).Msg("can't extract image")
		}
	}
	item.Time = timePosition(foreignItem.StartAt)
	item.Image = fmt.Sprintf("<img src=\"%s\">", path.Base(imageFile))
	item.Sound = fmt.Sprintf("[sound:%s]", path.Base(audiofile))

	if tsk.STT != "" {
		reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
			es.CurrentSTTOperation = tsk.STT
			es.CurrentItemIndex = indexedSub.Index
			es.CurrentItemTimecode = timePosition(foreignItem.StartAt)
		}) // necessity: high
		
		tsk.Handler.ZeroLog().Trace().
			Int("idx", indexedSub.Index).
			Msgf("Requesting %s transcription...", tsk.STT)
		
		// Get language info from media track
		lang := tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language
		
		// Use the new transcription function that handles model selection
		dub, err := voice.TranscribeAudioWithModel(
			childCtx,
			tsk.STT,
			audiofile, 
			lang.Part1, 
			tsk.InitialPrompt,
			tsk.MaxAPIRetries, 
			tsk.TimeoutSTT,
		)
		
		item.ForeignCurr = dub
		if err != nil {
			reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
				es.LastErrorOperation = "speech_to_text"
				es.LastErrorProvider = tsk.STT
				es.FailedSubtitleIndex = indexedSub.Index
				es.FailedSubtitleTimecode = timePosition(foreignItem.StartAt)
			}) // necessity: critical
			
			if errors.Is(err, context.Canceled) {
				return item, tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "STT: Processing canceled")
			} else if errors.Is(err, context.DeadlineExceeded) {
				return item, tsk.Handler.LogErr(err, AbortTask, "STT: Operation timed out.")
			}
			
			return item, tsk.Handler.LogErrFields(err, AbortTask, tsk.STT+" error",
				map[string]interface{}{"item": foreignItem.String()})
		}
	}

	i := indexedSub.Index
	
	if i > 0 && i < len(tsk.TargSubs.Items) {
		item.ForeignPrev = tsk.TargSubs.Items[i-1].String()
	}
	if i > 0 && i < len(tsk.NativeSubs.Items) {
		item.NativePrev = tsk.NativeSubs.Items[i-1].String()
	}

	if i+1 < len(tsk.TargSubs.Items) {
		item.ForeignNext = tsk.TargSubs.Items[i+1].String()
	}
	if i+1 < len(tsk.NativeSubs.Items) {
		item.NativeNext = tsk.NativeSubs.Items[i+1].String()
	}
	return
}





func (tsk *Task) ConcatWAVstoOGG(suffix string) {
	out := fmt.Sprint(tsk.MediaPrefix, ".", suffix,".ogg")
	if  _, err := os.Stat(out); err == nil {
		return
	}
	wavFiles, err := filepath.Glob(tsk.MediaPrefix+ "_*.wav")
	if err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).
			Str("mediaOutputDir", tsk.mediaOutputDir()).
			Msg("Error searching for .wav files")
	}

	if len(wavFiles) == 0 {
		tsk.Handler.ZeroLog().Warn().
			Str("mediaOutputDir", tsk.mediaOutputDir()).
			Msg("No .wav files found")
	}
	// Generate the concat list for ffmpeg
	concatFile, err := media.CreateConcatFile(wavFiles)
	if err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).Msg("Error creating temporary concat file")
	}
	defer os.Remove(concatFile)

	// Run FFmpeg to concatenate and create the audio file
	media.RunFFmpegConcat(concatFile, tsk.MediaPrefix+".wav")

	// Convert WAV to OPUS using FFmpeg
	media.RunFFmpegConvert(tsk.MediaPrefix+".wav", out)
	// Clean up
	os.Remove(tsk.MediaPrefix+".wav")
	for _, f := range wavFiles {
		if err := os.Remove(f); err != nil {
			tsk.Handler.ZeroLog().Warn().Str("file", f).Msg("Removing file failed")
		}
	}
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


