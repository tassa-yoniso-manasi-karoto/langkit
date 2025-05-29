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

	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/summary" 
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/metadata"
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
	} else {
		item.NativeCurr = "" // Explicitly set to empty if no NativeSubs
	}
	// Only extract OGG audio if needed (for Subs2Cards or if STT is enabled)
	// For Condense mode, we don't need OGG audio for TSV output
	var audiofile string
	var errWAV error

	if tsk.Mode != Condense && (tsk.Mode == Subs2Cards || tsk.STT != "") {
		var err error
		audiofile, err = media.ExtractAudio("ogg", tsk.UseAudiotrack,
			tsk.Offset, foreignItem.StartAt, foreignItem.EndAt,
				tsk.MediaSourceFile, tsk.MediaPrefix, false)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			tsk.Handler.ZeroLog().Error().Err(err).Msg("can't extract ogg audio")
		}
	}
	
	// Extract WAV for condensed audio if needed
	if tsk.Mode == Condense || tsk.WantCondensedAudio {
		// CAVEAT: Offset MUST be 0 to avoid duplicating audio at junctions of adjascent sublines
		_, errWAV = media.ExtractAudio("wav", tsk.UseAudiotrack,
			time.Duration(0), foreignItem.StartAt, foreignItem.EndAt,
			tsk.MediaSourceFile, tsk.MediaPrefix, false)
		if errWAV != nil && !errors.Is(errWAV, fs.ErrExist) {
			tsk.Handler.ZeroLog().Error().Err(errWAV).Msg("can't extract wav audio")
		}
	}

	if tsk.Mode == Condense && errors.Is(errWAV, fs.ErrExist) {
		item.AlreadyDone = true
		tsk.Handler.ZeroLog().Trace().
			Int("idx", indexedSub.Index).
			Msgf("Item marked AlreadyDone for Condense mode due to existing WAV segment.")
	}
	
	// Images are only needed for Subs2Cards mode
	dryRun := tsk.Mode != Subs2Cards
	
	// Skip actual image extraction in Condense mode
	var imageFile string
	var err error
	if tsk.Mode == Condense {
		// In Condense mode, we don't need images, just use a dummy path
		imageFile = tsk.MediaPrefix + "_dummy.avif"
	} else {
		imageFile, err = media.ExtractImage(foreignItem.StartAt, foreignItem.EndAt,
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
	}
	
	item.Time = timePosition(foreignItem.StartAt)
	item.Image = fmt.Sprintf("<img src=\"%s\">", path.Base(imageFile))
	
	// Only set Sound field if we extracted an OGG file
	if audiofile != "" {
		item.Sound = fmt.Sprintf("[sound:%s]", path.Base(audiofile))
	} else {
		item.Sound = ""
	}

	// Skip STT processing for Condense mode since it's not needed
	if tsk.STT != "" && tsk.Mode != Condense {
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
	if tsk.NativeSubs != nil && i > 0 && i < len(tsk.NativeSubs.Items) {
		item.NativePrev = tsk.NativeSubs.Items[i-1].String()
	}

	if i+1 < len(tsk.TargSubs.Items) {
		item.ForeignNext = tsk.TargSubs.Items[i+1].String()
	}
	if tsk.NativeSubs != nil && i+1 < len(tsk.NativeSubs.Items) {
		item.NativeNext = tsk.NativeSubs.Items[i+1].String()
	}
	return
}





func (tsk *Task) ConcatWAVsToAudio(suffix string) error {
	errFmt := fmt.Errorf("invalid or missing audio format")
	var ext string
	switch tsk.CondensedAudioFmt {
	case "MP3":
		ext = "mp3"
	case "AAC":
		ext = "m4a"
	case "Opus":
		ext = "opus"
	default:
		return tsk.Handler.LogErr(fmt.Errorf("%w: \"%s\" isn't recognized", errFmt, tsk.CondensedAudioFmt), AbortAllTasks, "")
	}
	
	out := fmt.Sprintf("%s.%s.%s", tsk.MediaPrefix, suffix, ext)
	tsk.Handler.ZeroLog().Debug().
		Str("outFile", out).
		Msg("Condensed audio initialized")

	if _, err := os.Stat(out); err == nil {
		tsk.Handler.ZeroLog().Info().
			Str("outFile", out).
			Msg("Condensed audio file already exists, skipping creation")
		if tsk.WantSummary {
			tsk.Handler.ZeroLog().Info().Msg("Summary was requested, but condensed audio file already exists. Summary will not be added to existing file in this run.")
		}
		return nil
	}

	wavPattern := tsk.MediaPrefix + "_*.wav"
	wavFiles, err := filepath.Glob(wavPattern)
	if err != nil {
		err = fmt.Errorf("failed to find WAV files with pattern '%s': %w", wavPattern, err)
		tsk.Handler.ZeroLog().Error().Err(err).
			Str("pattern", wavPattern).
			Msg("Error searching for .wav files for concatenation")
		return err
	}

	if len(wavFiles) == 0 {
		err = fmt.Errorf("no WAV files found with pattern '%s' to create condensed audio", wavPattern)
		tsk.Handler.ZeroLog().Warn().
			Str("pattern", wavPattern).
			Msg("No .wav files found for creating condensed audio. Condensed audio will not be created.")
		return err
	}

	tsk.Handler.ZeroLog().Info().
		Int("fileCount", len(wavFiles)).
		Str("outputFile", out).
		Msg("Creating condensed audio file from WAV segments")

	concatFile, err := media.CreateConcatFile(wavFiles)
	if err != nil {
		err = fmt.Errorf("failed to create concatenation file for FFmpeg: %w", err)
		tsk.Handler.ZeroLog().Error().Err(err).
			Msg("Error creating temporary concat file for FFmpeg")
		return err
	}
	defer os.Remove(concatFile)

	tempWavFile := tsk.MediaPrefix + ".concatenated.wav"

	if err := media.RunFFmpegConcat(concatFile, tempWavFile); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).
			Str("concatFile", concatFile).
			Str("outputWav", tempWavFile).
			Msg("Failed to concatenate WAV files")
		_ = os.Remove(tempWavFile)
		return err
	}
	defer os.Remove(tempWavFile)

	if err := media.RunFFmpegConvert(tempWavFile, out); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).
			Str("inputWav", tempWavFile).
			Str("outputFile", out).
			Str("format", tsk.CondensedAudioFmt).
			Msg("Failed to convert WAV to target format")
		_ = os.Remove(out)
		return err
	}

	tsk.Handler.ZeroLog().Trace().Msg("Removing WAV segment files after creating condensed audio...")
	for _, f := range wavFiles {
		if err := os.Remove(f); err != nil {
			tsk.Handler.ZeroLog().Warn().
				Str("file", f).
				Err(err).
				Msg("Failed to remove WAV segment file")
		}
	}

	// Generate and add summary to metadata if requested (not supported for Opus format)
	if tsk.WantSummary && tsk.CondensedAudioFmt != "Opus" {
		if tsk.TargSubs != nil && tsk.TargSubs.Subtitles != nil && len(tsk.TargSubs.Subtitles.Items) > 0 {
			tsk.Handler.ZeroLog().Info().
				Str("provider", tsk.SummaryProvider).
				Str("model", tsk.SummaryModel).
				Msg("Attempting to generate media summary for condensed audio...")

			astiSubs := tsk.TargSubs.Subtitles
			// FIXME: The comment about "trimmed close captions" is still relevant.
			// If tsk.TargSubs.Subtitles was modified by TrimCC2Dubs in cards.go,
			// the summary will be based on the trimmed version.
			// If original is needed, a copy must be made before TrimCC2Dubs.

			subtitleTextForLLM := summary.PrepareSubtitlesForSummary(astiSubs)

			if subtitleTextForLLM != "" {
				inputLangName := ""
				if tsk.Targ.Language != nil {
					inputLangName = tsk.Targ.Language.Name
				}

				outputLangName := ""
				summaryLangCodeISO639_2 := "und" // Default to "undetermined"

				if tsk.Native.Language != nil {
					outputLangName = tsk.Native.Language.Name
					if tsk.Native.Language.Part2T != "" {
						summaryLangCodeISO639_2 = tsk.Native.Language.Part2T
					} else if tsk.Native.Language.Part2B != "" {
						summaryLangCodeISO639_2 = tsk.Native.Language.Part2B
					} else {
						tsk.Handler.ZeroLog().Warn().Str("lang_name", tsk.Native.Language.Name).Msg("No ISO 639-2 (T or B) code found for native language, USLT language will be 'und'")
					}
				} else {
					tsk.Handler.ZeroLog().Warn().Msg("Native language not set for task, summary output language will be LLM default and USLT language tag will be 'und'")
				}

				summaryOpts := summary.Options{
					Provider:            tsk.SummaryProvider,
					Model:               tsk.SummaryModel,
					OutputLanguage:      outputLangName,
					MaxLength:           tsk.SummaryMaxLength,
					Temperature:         tsk.SummaryTemperature,
					CustomPrompt:        tsk.SummaryCustomPrompt,
					UseSymbolicEmphasis: tsk.UseSymbolicEmphasis,
				}

				ctxSummarize, cancelSummarize := context.WithTimeout(context.Background(), 3*time.Minute) // TODO: Make timeout configurable
				defer cancelSummarize()

				summaryText, err := summary.GetDefaultService().GenerateSummary(ctxSummarize, subtitleTextForLLM, inputLangName, summaryOpts)

				if err != nil {
					tsk.Handler.ZeroLog().Error().Err(err).
						Msg("Failed to generate summary for condensed audio")
				} else {
					if summaryText != "" {
						// Use the new AddLyricsToAudioFile function
						err = metadata.AddLyricsToAudioFile(out, summaryText, summaryLangCodeISO639_2)
						if err != nil {
							tsk.Handler.ZeroLog().Error().Err(err).
								Msg("Failed to add summary to condensed audio metadata")
						} else {
							tsk.Handler.ZeroLog().Info().
								Msg("Summary successfully generated and added to condensed audio file metadata")
						}
					} else {
						tsk.Handler.ZeroLog().Info().
							Msg("Summary generation resulted in empty text, not adding to metadata.")
					}
				}
			} else {
				tsk.Handler.ZeroLog().Warn().
					Msg("No subtitle text available for summarization after preparation")
			}
		} else {
			tsk.Handler.ZeroLog().Warn().
				Msg("Target subtitles (tsk.TargSubs or tsk.TargSubs.Subtitles) not available for summarization. Skipping summary.")
		}
	} else if tsk.WantSummary && tsk.CondensedAudioFmt == "Opus" {
		tsk.Handler.ZeroLog().Info().
			Msg("Summary generation is not supported for Opus format. Skipping summary.")
	}

	tsk.Handler.ZeroLog().Info().
		Str("outputFile", out).
		Msg("Successfully created condensed audio file")
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


