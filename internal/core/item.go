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
	"time"

	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/asticode/go-astisub" 
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/summary" 
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
	} else {
		item.NativeCurr = "" // Explicitly set to empty if no NativeSubs
	}
	// Only extract OGG audio if needed (for Subs2Cards or if STT is enabled)
	// For Condense mode, we don't need OGG audio for TSV output
	var audiofile string
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
	// This is always required for Condense mode or when WantCondensedAudio is true
	if tsk.Mode == Condense || tsk.WantCondensedAudio {
		var errWAV error // Declare error variable for WAV extraction
		_, errWAV = media.ExtractAudio("wav", tsk.UseAudiotrack,
			time.Duration(0), foreignItem.StartAt, foreignItem.EndAt,
				tsk.MediaSourceFile, tsk.MediaPrefix, false)
		if errWAV != nil && !errors.Is(errWAV, fs.ErrExist) {
			tsk.Handler.ZeroLog().Error().Err(errWAV).Msg("can't extract wav audio")
		}
		
		// Add this specifically for Condense mode to update AlreadyDone based on WAV
		if tsk.Mode == Condense && errors.Is(errWAV, fs.ErrExist) {
			item.AlreadyDone = true
			tsk.Handler.ZeroLog().Trace().
				Int("idx", indexedSub.Index).
				Msgf("Item marked AlreadyDone for Condense mode due to existing WAV segment.")
		}
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





func (tsk *Task) ConcatWAVstoOGG(suffix string) error {
	// Define output file path
	out := fmt.Sprint(tsk.MediaPrefix, ".", suffix, ".ogg")
	
	// Check if output file already exists
	if _, err := os.Stat(out); err == nil {
		tsk.Handler.ZeroLog().Info().
			Str("outFile", out).
			Msg("Condensed audio file already exists, skipping creation")
		return nil
	}
	
	// Find all WAV files that we need to concatenate
	wavPattern := tsk.MediaPrefix + "_*.wav"
	wavFiles, err := filepath.Glob(wavPattern)
	if err != nil {
		err = fmt.Errorf("failed to find WAV files: %w", err)
		tsk.Handler.ZeroLog().Error().Err(err).
			Str("pattern", wavPattern).
			Str("mediaOutputDir", tsk.mediaOutputDir()).
			Msg("Error searching for .wav files")
		return err
	}

	// Ensure we have files to process
	if len(wavFiles) == 0 {
		err = fmt.Errorf("no WAV files found to create condensed audio")
		tsk.Handler.ZeroLog().Warn().
			Str("pattern", wavPattern).
			Str("mediaOutputDir", tsk.mediaOutputDir()).
			Msg("No .wav files found for creating condensed audio")
		return err
	}
	
	tsk.Handler.ZeroLog().Info().
		Int("fileCount", len(wavFiles)).
		Str("outputFile", out).
		Msg("Creating condensed audio file from WAV segments")
	
	// Generate the concat list for ffmpeg
	concatFile, err := media.CreateConcatFile(wavFiles)
	if err != nil {
		err = fmt.Errorf("failed to create concatenation file: %w", err)
		tsk.Handler.ZeroLog().Error().Err(err).
			Msg("Error creating temporary concat file for FFmpeg")
		return err
	}
	defer os.Remove(concatFile)

	// Temporary WAV file path
	tempWavFile := tsk.MediaPrefix + ".wav"
	
	// Run FFmpeg to concatenate and create the intermediate WAV file
	if err := media.RunFFmpegConcat(concatFile, tempWavFile); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).
			Str("concatFile", concatFile).
			Str("outputWav", tempWavFile).
			Msg("Failed to concatenate WAV files")
		return err
	}

	// Convert WAV to OPUS/OGG using FFmpeg
	if err := media.RunFFmpegConvert(tempWavFile, out); err != nil {
		tsk.Handler.ZeroLog().Error().Err(err).
			Str("inputWav", tempWavFile).
			Str("outputOgg", out).
			Msg("Failed to convert WAV to OGG")
		return err
	}
	
	// Clean up intermediate WAV file
	if err := os.Remove(tempWavFile); err != nil {
		tsk.Handler.ZeroLog().Warn().
			Str("file", tempWavFile).
			Err(err).
			Msg("Failed to remove temporary WAV file")
	}
	
	// Clean up individual WAV segment files if configured to do so
	if tsk.IntermediaryFileMode != "keep" {
		for _, f := range wavFiles {
			if err := os.Remove(f); err != nil {
				tsk.Handler.ZeroLog().Warn().
					Str("file", f).
					Err(err).
					Msg("Failed to remove WAV segment file")
			}
		}
		tsk.Handler.ZeroLog().Debug().
			Int("removedFiles", len(wavFiles)).
			Msg("Removed WAV segment files after creating condensed audio")
	}
	
	// Generate and add summary to metadata if requested
	if tsk.WantSummary && tsk.TargSubs != nil && len(tsk.TargSubs.Items) > 0 {
		tsk.Handler.ZeroLog().Info().
			Str("provider", tsk.SummaryProvider).
			Str("model", tsk.SummaryModel).
			Msg("Attempting to generate media summary for condensed audio...")

		var astiSubs *astisub.Subtitles
		if tsk.TargSubs.Subtitles != nil {
			astiSubs = tsk.TargSubs.Subtitles // FIXME change this at some point because using this will likely cause LLM to be provided with the 'trimmed' close captions!
		}

		if astiSubs != nil {
			subtitleTextForLLM := summary.PrepareSubtitlesForSummary(astiSubs) // Only returns text now

			if subtitleTextForLLM != "" {
				inputLangName := ""
				if tsk.Targ != nil && tsk.Targ.Language != nil {
					inputLangName = tsk.Targ.Language.Name // e.g., "Japanese"
				}

				outputLangName := ""
				if tsk.NativeLang != nil && tsk.NativeLang.Language != nil {
					outputLangName = tsk.NativeLang.Language.Name // e.g., "English"
				}


				summaryOpts := summary.Options{
					Provider:          tsk.SummaryProvider,
					Model:             tsk.SummaryModel,
					OutputLanguage:    outputLangName,       // Use English name of native lang
					MaxLength:         tsk.SummaryMaxLength,   
					Temperature:       tsk.SummaryTemperature, 
					CustomPrompt:      tsk.SummaryCustomPrompt,
					// InputLanguageHint is no longer in summary.Options
				}
				
				ctxSummarize, cancelSummarize := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancelSummarize()

				// Pass inputLangName to the service's GenerateSummary method
				summaryText, err := summary.GetDefaultService().GenerateSummary(ctxSummarize, subtitleTextForLLM, inputLangName, summaryOpts)

				if err != nil {
					tsk.Handler.ZeroLog().Error().Err(err).
						Msg("Failed to generate summary for condensed audio")
				} else {
					if summaryText != "" {
						err = media.AddMetadataToAudio(out, "lyrics", summaryText) 
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
				Msg("Underlying astisub.Subtitles not available from tsk.TargSubs for summarization")
		}
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


