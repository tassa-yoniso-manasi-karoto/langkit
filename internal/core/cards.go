package core

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gookit/color"
	"github.com/k0kubun/pp"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
)

var SupportedExt = []string{".srt", ".ass", ".ssa"}

// collectStandaloneCandidates scans directory for standalone subtitle files
func (tsk *Task) collectStandaloneCandidates() []SubtitleCandidate {
	var candidates []SubtitleCandidate

	dir := filepath.Dir(tsk.MediaSourceFile)
	files, err := os.ReadDir(dir)
	if err != nil {
		tsk.Handler.ZeroLog().Debug().Err(err).Msg("collectStandaloneCandidates: failed to read directory")
		return candidates
	}

	trimmedMedia := strings.TrimSuffix(filepath.Base(tsk.MediaSourceFile), filepath.Ext(tsk.MediaSourceFile))

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(file.Name()))
		trimmed := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))

		// Skip non-subtitle files
		if !slices.Contains(SupportedExt, ext) {
			continue
		}

		// Skip langkit-generated files
		if isLangkitMadeDubtitles(file.Name()) || isLangkitMadeTranslit(file.Name()) {
			continue
		}

		// Skip forced subtitles
		if strings.Contains(strings.ToLower(trimmed), "forced") {
			continue
		}

		// Skip files that don't match media prefix (case-insensitive for
		// providers like Crunchyroll that may use different casing)
		if !strings.HasPrefix(strings.ToLower(trimmed), strings.ToLower(trimmedMedia)) {
			continue
		}

		// Guess language from filename
		lang, err := GuessLangFromFilename(file.Name())
		if err != nil {
			tsk.Handler.ZeroLog().Debug().Err(err).Str("file", file.Name()).Msg("guessing lang from filename")
			continue
		}

		candidate := SubtitleCandidate{
			Lang: lang,
			Source: SubtitleSource{
				Type:     SubSourceStandalone,
				FilePath: filepath.Join(dir, file.Name()),
			},
			Subtype: subtypeMatcher(file.Name()),
		}

		tsk.Handler.ZeroLog().Debug().
			Str("file", file.Name()).
			Str("lang", lang.Part3).
			Str("subtag", lang.Subtag).
			Msg("Found standalone subtitle candidate")

		candidates = append(candidates, candidate)
	}

	return candidates
}

// collectEmbeddedCandidates extracts subtitle track info from video container
func (tsk *Task) collectEmbeddedCandidates() []SubtitleCandidate {
	var candidates []SubtitleCandidate

	if tsk.MediaSourceFile == "" {
		return candidates
	}

	mediaInfo, err := Mediainfo(tsk.MediaSourceFile)
	if err != nil {
		tsk.Handler.ZeroLog().Debug().Err(err).Msg("collectEmbeddedCandidates: failed to get mediainfo")
		return candidates
	}

	for i, track := range mediaInfo.TextTracks {
		// Skip non-text-based formats (PGS, VobSub, etc.)
		if !isTextBasedFormat(track.Format) {
			tsk.Handler.ZeroLog().Debug().
				Str("format", track.Format).
				Str("title", track.Title).
				Msg("Skipping non-text subtitle track")
			continue
		}

		// Parse StreamOrder to int for FFmpeg extraction
		var streamIndex int
		fmt.Sscanf(track.StreamOrder, "%d", &streamIndex)

		candidate := SubtitleCandidate{
			Lang: track.Language,
			Source: SubtitleSource{
				Type:        SubSourceEmbedded,
				MediaFile:   tsk.MediaSourceFile,
				TrackIndex:  i,
				StreamIndex: streamIndex,
				Format:      track.Format,
				CodecID:     track.CodecID,
			},
			IsDefault: track.Default == "Yes",
			Title:     track.Title,
			Subtype:   subtypeMatcher(track.Title), // Derive subtype from Title
		}

		tsk.Handler.ZeroLog().Debug().
			Int("trackIndex", i).
			Int("streamIndex", streamIndex).
			Str("format", track.Format).
			Str("lang", track.Language.Part3).
			Str("subtag", track.Language.Subtag).
			Str("title", track.Title).
			Bool("default", candidate.IsDefault).
			Msg("Found embedded subtitle candidate")

		candidates = append(candidates, candidate)
	}

	return candidates
}

// collectAllCandidates combines standalone and embedded subtitle candidates
func (tsk *Task) collectAllCandidates() []SubtitleCandidate {
	standalone := tsk.collectStandaloneCandidates()
	embedded := tsk.collectEmbeddedCandidates()

	tsk.Handler.ZeroLog().Debug().
		Int("standalone", len(standalone)).
		Int("embedded", len(embedded)).
		Msg("Collected subtitle candidates")

	return append(standalone, embedded...)
}

// Path utility functions

// isEmbeddedSubtitle returns true if TargSubFile was extracted from container
// (i.e., it's not in the same directory as the video file)
func (tsk *Task) isEmbeddedSubtitle() bool {
	if tsk.TargSubFile == "" || tsk.MediaSourceFile == "" {
		return false
	}
	return filepath.Dir(tsk.TargSubFile) != filepath.Dir(tsk.MediaSourceFile)
}

// outputDir returns the directory for all output files (video's directory)
func (tsk *Task) outputDir() string {
	if tsk.MediaSourceFile != "" {
		return filepath.Dir(tsk.MediaSourceFile)
	}
	return filepath.Dir(tsk.TargSubFile)
}

// outputBase returns the base name for all output files, preserving the original filename.
// For standalone subtitles: uses TargSubFile (preserves existing naming & resumption)
// For embedded subtitles: derives from MediaSourceFile + lang suffix
func (tsk *Task) outputBase() string {
	// Standalone subtitle: use existing naming (preserves resumption compatibility)
	if tsk.TargSubFile != "" && !tsk.isEmbeddedSubtitle() {
		return strings.TrimSuffix(filepath.Base(tsk.TargSubFile), filepath.Ext(tsk.TargSubFile))
	}

	// Embedded subtitle: derive from MediaSourceFile + language suffix
	if tsk.MediaSourceFile != "" {
		base := strings.TrimSuffix(filepath.Base(tsk.MediaSourceFile), filepath.Ext(tsk.MediaSourceFile))
		// Add language suffix for mpv fuzzy matching
		if tsk.Targ.Language != nil {
			base += "." + tsk.Targ.String()
		}
		return base
	}

	// Ultimate fallback (shouldn't happen in normal use)
	return strings.TrimSuffix(filepath.Base(tsk.TargSubFile), filepath.Ext(tsk.TargSubFile))
}

// ffmpegSafeBase returns outputBase with characters that break FFmpeg concat
// (single quotes) replaced. Used only for MediaPrefix and media output paths.
func (tsk *Task) ffmpegSafeBase() string {
	return strings.ReplaceAll(tsk.outputBase(), "'", " ")
}

func (tsk *Task) outputFile() string {
	return filepath.Join(tsk.outputDir(), tsk.outputBase()+tsk.OutputFileExtension)
}

func (tsk *Task) mediaOutputDir() string {
	return filepath.Join(tsk.outputDir(), tsk.ffmpegSafeBase()+".media")
}

func (tsk *Task) audioBase() string {
	base := strings.TrimSuffix(filepath.Base(tsk.MediaSourceFile), filepath.Ext(tsk.MediaSourceFile))
	return base
}

// Execute is the main entry point for processing a task
func (tsk *Task) Execute(ctx context.Context) (procErr *ProcessingError) {
	var outStream *os.File
	defer func() {
		if outStream != nil {
			outStream.Close()
		}
	}()
	reporter := crash.Reporter
	reporter.ClearExecutionRecords()
	reporter.SaveSnapshot("Starting execution", tsk.DebugVals())
	reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
		es.ParentDirPath = filepath.Dir(tsk.MediaSourceFile)
	})

	if procErr := tsk.validateBasicRequirements(); procErr != nil {
		reporter.SaveSnapshot("Requirements validation failed", tsk.DebugVals()) // necessity: high
		return procErr
	}
	
	// Initialize the intermediary file manager
	tsk.fileManager = NewIntermediaryFileManager(
		tsk.IntermediaryFileMode,
		tsk.Handler,
		tsk.DeleteResumptionFiles,
	)
	
	if tsk.Mode != Enhance {
		// Handle subtitle detection and language determination
		if procErr := tsk.setupSubtitles(ctx, reporter); procErr != nil {
			return procErr
		}
	}
	
	// Register original subtitle files for merging if merging is enabled AND this is a merge-group feature
	// The frontend tells us this by setting MergeOutputFiles to true only for features in the mergeGroup
	if tsk.MergeOutputFiles {
		tsk.Handler.ZeroLog().Debug().Msg("Registering subtitle files for merging")
		if tsk.TargSubFile != "" {
			tsk.RegisterOutputFile(tsk.TargSubFile, OutputSubtitle, tsk.Targ, "original", 50)
		}
		if tsk.NativeSubFile != "" {
			tsk.RegisterOutputFile(tsk.NativeSubFile, OutputSubtitle, tsk.Native, "original", 50)
		}
	} else {
		tsk.Handler.ZeroLog().Trace().Msg("Skipping output merge registration (no merging requested)")
	}
	
	if procErr := tsk.processMediaInfo(); procErr != nil {
		return procErr
	}
	
	// case where no STT is involved
	if tsk.Mode == Enhance || tsk.Mode == Translit {
		if tsk.WantTranslit {
			tsk.prepareSubtitles()

			if err := tsk.processTransliteration(ctx); err != nil {
				return err
			}
		}
		
		if procErr := tsk.processAudioEnhancement(ctx); procErr != nil {
			return procErr
		}
		goto goodEnd
	}

	if outStream, procErr = tsk.prepareOutputDirectory(); procErr != nil {
		return procErr
	}
	reporter.SaveSnapshot("after output directory prep", tsk.DebugVals()) // necessity: high
	reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
		es.MediaInfoDump = fmt.Sprintf("%+v", tsk.Meta.MediaInfo) // necessity: high
	})

	tsk.prepareSubtitles()

	if procErr := tsk.handleUserConfirmation(); procErr != nil {
		return procErr
	}

	// Launch bulk of app logic: supervisor
	if tsk.Mode == Subs2Cards || tsk.Mode == Subs2Dubs || tsk.Mode == Condense {
		// Check if we can skip WAV extraction due to existing concatenated file
		tsk.CheckConcatenatedWAV()
		
		// For Condense mode, we don't need to write TSV output, just extract WAV segments
		if tsk.Mode == Condense {
			tsk.Handler.ZeroLog().Info().Msg("Running in Condense mode - extracting audio segments only")
			if err := tsk.Supervisor(ctx, nil, nil); err != nil {
				reporter.SaveSnapshot("Supervisor failed in Condense mode", tsk.DebugVals())
				return err
			}
			
			// Check if enhanced track is also requested for Condense mode
			if tsk.WantEnhancedTrack {
				tsk.Handler.ZeroLog().Info().Msg("Additionally creating enhanced audio track as part of Condense mode...")
				
				// Ensure we have a separation library specified
				if tsk.SeparationLib == "" {
					procErr = tsk.Handler.Log(Warn, AbortTask, "Cannot create enhanced track: No separation library specified for Condense mode with enhanced track option.")
					if procErr != nil { 
						return procErr
					}
				} else {
					// Process audio enhancement as an auxiliary feature for Condense mode
					if procErr := tsk.processAudioEnhancement(ctx); procErr != nil {
						// Error is already logged by processAudioEnhancement or its callees
						return procErr
					}
				}
			}
		} else {
			// Normal operation for Subs2Cards and Subs2Dubs
			if err := tsk.Supervisor(ctx, outStream, write); err != nil {
				reporter.SaveSnapshot("Supervisor failed", tsk.DebugVals()) // necessity: critical
				return err
			}
		}
	}

	if procErr := tsk.processDubtitles(ctx); procErr != nil {
		reporter.SaveSnapshot("Dubtitles processing failed", tsk.DebugVals()) // necessity: high
		return procErr
	}

	if procErr := tsk.processTransliteration(ctx); procErr != nil {
		reporter.SaveSnapshot("Transliteration failed", tsk.DebugVals()) // necessity: high
		return procErr
	}

	if procErr := tsk.processAudioEnhancement(ctx); procErr != nil {
		reporter.SaveSnapshot("Audio enhancement failed", tsk.DebugVals()) // necessity: high
		return procErr
	}

goodEnd:
	// Handle condensed audio generation for Translit mode if requested
	// Note: Enhance mode does not support condensed audio according to the design
	if tsk.Mode == Translit && tsk.WantCondensedAudio {
		if tsk.TargSubs == nil || len(tsk.TargSubs.Items) == 0 {
			procErr = tsk.Handler.Log(Warn, AbortTask, "Cannot generate condensed audio: Target subtitles are required but not available/loaded for Translit mode with condensed audio option.")
			if procErr != nil {
				return procErr
			}
		} else {
			// Ensure media output directory exists for WAV segments
			mediaOutDir := tsk.mediaOutputDir()
			if err := os.MkdirAll(mediaOutDir, os.ModePerm); err != nil {
				// This is a critical error for the requested feature
				procErr = tsk.Handler.LogErr(err, AbortTask, fmt.Sprintf("Failed to create media output directory for condensed audio: %s", mediaOutDir))
				if procErr != nil {
					return procErr
				}
				// Continue processing even if condensed audio creation fails
			} else {
				// Ensure MediaPrefix is set properly for Translit mode with condensed audio
				if tsk.TargSubFile == "" {
					// This is a critical error - we need TargSubFile for outputBase
					procErr = tsk.Handler.Log(Error, AbortTask, "Cannot generate condensed audio: TargSubFile is not set, cannot determine output base.")
					if procErr != nil {
						return procErr
					}
				}

				// Set MediaPrefix for extraction
				tsk.MediaPrefix = filepath.Join(mediaOutDir, tsk.ffmpegSafeBase())
				tsk.Handler.ZeroLog().Debug().
					Str("MediaPrefix", tsk.MediaPrefix).
					Msg("MediaPrefix set for Translit + WantCondensedAudio")

				// Check if we can skip WAV extraction due to existing concatenated file
				tsk.CheckConcatenatedWAV()

				if !tsk.SkipWAVExtraction {
					tsk.Handler.ZeroLog().Info().Msg("Extracting WAV segments for condensed audio (auxiliary output)...")

					// Extract WAV segments for each subtitle item
					for _, foreignItem := range tsk.TargSubs.Items {
						select {
						case <-ctx.Done():
							tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "Condensed audio WAV extraction canceled by user")
							goto mergeOutputs
						default:
							_, err := media.ExtractAudio("wav", tsk.UseAudiotrack,
								time.Duration(0), foreignItem.StartAt, foreignItem.EndAt,
								tsk.MediaSourceFile, tsk.MediaPrefix, false) // dryRun = false
							if err != nil && !os.IsExist(err) {
								tsk.Handler.ZeroLog().Error().Err(err).
									Str("time", timePosition(foreignItem.StartAt)).
									Msg("Failed to extract WAV segment for condensed audio")
							}
						}
					}
				}

				// Call concatenation to create the final condensed audio file
				tsk.Handler.ZeroLog().Info().Msg("Creating condensed audio file (auxiliary output)...")
				if err := tsk.ConcatWAVsToAudio(); err != nil {
					procErr = tsk.Handler.LogErr(err, AbortTask, "Failed to create condensed audio file as part of Translit mode.")
					if procErr != nil {
						return procErr
					}
				}
			}
		}
	}
mergeOutputs:
	// Only merge outputs when MergeOutputFiles is true (set by the frontend for merge group features)
	if tsk.MergeOutputFiles && len(tsk.OutputFiles) > 0 {
		tsk.Handler.ZeroLog().Debug().
			Bool("mergeOutputFiles", tsk.MergeOutputFiles).
			Int("outputFilesCount", len(tsk.OutputFiles)).
			Msg("Processing merge outputs from merge group feature")
			
		mergeResult, procErr := tsk.MergeOutputs(ctx)
			if procErr != nil {
				reporter.SaveSnapshot("Output merging failed", tsk.DebugVals()) // necessity: high
				return procErr
			}
			
			if mergeResult != nil && !mergeResult.Skipped {
				tsk.Handler.ZeroLog().Info().
					Str("outputPath", mergeResult.OutputPath).
					Bool("success", mergeResult.Success).
					Msg("Output files merged successfully")
			}
	} else {
		tsk.Handler.ZeroLog().Debug().
			Bool("mergeOutputFiles", tsk.MergeOutputFiles).
			Int("outputFilesCount", len(tsk.OutputFiles)).
			Msg("Skipping merge outputs - not part of merge group or no files to merge")
	}

	// Process intermediary files according to the configured mode
	if tsk.fileManager != nil {
		tsk.Handler.ZeroLog().Debug().Msg("Processing intermediary files")
		tsvFile := tsk.outputFile()
		if err := tsk.fileManager.ProcessFiles(tsvFile); err != nil {
			tsk.Handler.ZeroLog().Warn().Err(err).Msg("Error processing intermediary files")
		}
		
		// Clean up empty media directory if all files were deleted
		if tsk.Mode != Enhance && tsk.Mode != Translit {
			tsk.fileManager.CleanupMediaDirectory(tsk.mediaOutputDir())
		}
	}

	tsk.Handler.ZeroLog().Info().Msg("Processing completed")
	return nil
}

// Autosub automatically discovers subtitle files based on the media filename
func (tsk *Task) Autosub() *ProcessingError {
	// Collect candidates from both standalone files and embedded tracks
	candidates := tsk.collectAllCandidates()

	if len(candidates) == 0 {
		return tsk.Handler.LogErrFields(
			fmt.Errorf("no subtitle candidates found for %s", tsk.Targ.Name),
			AbortTask,
			"autosubs failed",
			map[string]interface{}{"video": filepath.Base(tsk.MediaSourceFile)},
		)
	}

	// Select best candidates for target and native languages
	targCandidate, nativeCandidate := selectBestCandidates(candidates, tsk.Targ, tsk.RefLangs)

	// Create temp directory for embedded subtitle extraction (uses tmpfs)
	tempDir, err := os.MkdirTemp("", "langkit-subs-*")
	if err != nil {
		return tsk.Handler.LogErr(err, AbortTask, "autosub: failed to create temp directory")
	}

	// Materialize target subtitle
	if targCandidate != nil {
		path, err := targCandidate.Materialize(tempDir)
		if err != nil {
			return tsk.Handler.LogErr(err, AbortTask, "autosub: failed to materialize target subtitle")
		}
		tsk.TargSubFile = path
		tsk.Targ = targCandidate.Lang

		// Check if it's CC or dubs based on subtype
		tsk.IsCCorDubs = targCandidate.Subtype == CC || targCandidate.Subtype == Dub

		tsk.Handler.ZeroLog().Info().
			Str("path", tsk.TargSubFile).
			Str("lang", tsk.Targ.Part3).
			Str("subtag", tsk.Targ.Subtag).
			Bool("embedded", targCandidate.Source.Type == SubSourceEmbedded).
			Msg("Automatically chosen Target subtitle")
	} else {
		return tsk.Handler.LogErrFields(
			fmt.Errorf("no subtitle matching target language %s was found", tsk.Targ.Name),
			AbortTask,
			"autosubs failed",
			map[string]interface{}{"video": filepath.Base(tsk.MediaSourceFile)},
		)
	}

	// Materialize native subtitle (if needed for Subs2Cards mode)
	if tsk.Mode == Subs2Cards {
		if nativeCandidate != nil {
			path, err := nativeCandidate.Materialize(tempDir)
			if err != nil {
				tsk.Handler.ZeroLog().Warn().Err(err).Msg("Failed to materialize native subtitle")
			} else {
				tsk.NativeSubFile = path
				tsk.Native = nativeCandidate.Lang

				// Update IsCCorDubs if native is CC or dubs
				if nativeCandidate.Subtype == CC || nativeCandidate.Subtype == Dub {
					tsk.IsCCorDubs = true
				}

				tsk.Handler.ZeroLog().Info().
					Str("path", tsk.NativeSubFile).
					Str("lang", tsk.Native.Part3).
					Str("subtag", tsk.Native.Subtag).
					Bool("embedded", nativeCandidate.Source.Type == SubSourceEmbedded).
					Msg("Automatically chosen Native subtitle")
			}
		} else {
			tsk.Handler.ZeroLog().Warn().
				Str("video", filepath.Base(tsk.MediaSourceFile)).
				Msg("No subtitle matching reference/native language was found")
		}
	}

	return nil
}

// writer function for processed items
func write(outStream *os.File, item *ProcessedItem) {
	fmt.Fprintf(outStream, "%s\t", escape(item.Sound))
	fmt.Fprintf(outStream, "%s\t", escape(item.Time))
	fmt.Fprintf(outStream, "%s\t", escape(item.Source))
	fmt.Fprintf(outStream, "%s\t", escape(item.Image))
	fmt.Fprintf(outStream, "%s\t", escape(item.ForeignCurr))
	fmt.Fprintf(outStream, "%s\t", escape(item.NativeCurr))
	fmt.Fprintf(outStream, "%s\t", escape(item.ForeignPrev))
	fmt.Fprintf(outStream, "%s\t", escape(item.NativePrev))
	fmt.Fprintf(outStream, "%s\t", escape(item.ForeignNext))
	fmt.Fprintf(outStream, "%s\n", escape(item.NativeNext))
}




// validateBasicRequirements validates the basic requirements for task execution
func (tsk *Task) validateBasicRequirements() *ProcessingError {
	// Check for apostrophe in directory path (ffmpeg limitation)
	if strings.Contains(filepath.Dir(tsk.TargSubFile), "'") {
		return tsk.Handler.Log(Error, AbortTask,
			"Due to ffmpeg limitations, the path of the directory in which the files are located must not contain an apostrophe ('). "+
				"Apostrophe in the names of the files themselves are supported using a workaround.")
	}
	
	if tsk.Mode == Enhance  {
		return nil
	}
	
	// Ensure either languages or subtitle files are specified
	if len(tsk.Langs) == 0 && tsk.TargSubFile == "" {
		return tsk.Handler.Log(Error, AbortAllTasks,
			"Neither languages and nor subtitle files were specified.")
	}

	return nil
}

// setupSubtitles handles finding and loading subtitle files
func (tsk *Task) setupSubtitles(ctx context.Context, reporter *crash.ReporterInstance) *ProcessingError {
	var err error

	// If no subtitle file was specified, try to find one automatically
	if tsk.TargSubFile == "" {
		reporter.SaveSnapshot("Running Autosub", tsk.DebugVals())
		if procErr := tsk.Autosub(); procErr != nil {
			reporter.SaveSnapshot("Autosub failed", tsk.DebugVals())
			return procErr
		}
	} else {
		// When subtitle file is specified explicitly, try to guess languages
		if len(tsk.Langs) == 0 {
			tsk.Handler.ZeroLog().Trace().
				Str("Targ", tsk.Targ.String()).
				Str("Native", tsk.Native.String()).
				Msg("No language flag passed. Attempted to guess language from filename.")
		}

		// Try to guess target language from filename
		tsk.Targ, err = GuessLangFromFilename(tsk.TargSubFile)
		if err != nil {
			tsk.Handler.ZeroLog().Warn().Err(err).
				Str("TargSubFile", tsk.TargSubFile).
				Msg("Couldn't guess the language of foreign subtitle file")
		}

		// Try to guess native language from filename if available
		tsk.Native, err = GuessLangFromFilename(tsk.NativeSubFile)
		if tsk.NativeSubFile != "" && err != nil {
			tsk.Handler.ZeroLog().Warn().Err(err).
				Str("NativeSubFile", tsk.NativeSubFile).
				Msg("Couldn't guess the language of native subtitle file")
		}
	}

	// Open target subtitle file
	tsk.TargSubs, err = subs.OpenFile(tsk.TargSubFile, false)
	if err != nil {
		return tsk.Handler.LogErr(err, AbortTask, "can't read foreign subtitles")
	}

	// For single-file mode, count items after applying same filters as routing.go
	// (CC trimming and ASS default-style filtering happen later in prepareSubtitles,
	// so we need a temporary copy to get accurate count)
	if !tsk.IsBulkProcess {
		countSubs, _ := subs.OpenFile(tsk.TargSubFile, false)
		if tsk.IsCCorDubs {
			countSubs.TrimCC2Dubs()
		}
		if isASSFormat(tsk.TargSubFile) {
			countSubs.FilterToDefaultStyle()
		}
		totalItems = len(countSubs.Items)
	}

	return nil
}

// prepareOutputDirectory creates necessary output directories and files
func (tsk *Task) prepareOutputDirectory() (*os.File, *ProcessingError) {
	var err error
	var outStream *os.File
	
	// Skip for Enhance or Translit modes
	if tsk.Mode == Enhance || tsk.Mode == Translit {
		return nil, nil
	}
	
	// Set totalItems if not in bulk mode
	if totalItems == 0 {
		totalItems = len(tsk.TargSubs.Items)
	}
	
	// For Condense mode, we only need to create the media output directory and set MediaPrefix
	// Skip native subtitles loading and TSV output file creation
	if tsk.Mode == Condense {
		tsk.Handler.ZeroLog().Debug().Msg("Preparing output directory for Condense mode")
		
		// Create media output directory
		if err := os.MkdirAll(tsk.mediaOutputDir(), os.ModePerm); err != nil {
			return nil, tsk.Handler.LogErr(err, AbortTask,
				fmt.Sprintf("can't create output directory: %s", tsk.mediaOutputDir()))
		}
		
 		// Set media prefix for file output
		tsk.MediaPrefix = filepath.Join(tsk.mediaOutputDir(), tsk.ffmpegSafeBase())
		
		return nil, nil
	}
	
	// Validate requirements for Subs2Cards mode
	if tsk.Mode == Subs2Cards {
		if len(tsk.Langs) < 2 && tsk.NativeSubFile == "" {
			return nil, tsk.Handler.LogErr(err, AbortTask, "Neither native language and nor native subtitle file was specified.")
		}
		if tsk.NativeSubFile == "" {
			tsk.Handler.ZeroLog().Warn().
				Str("video", filepath.Base(tsk.MediaSourceFile)).
				Msg("No sub file for any of the desired reference language(s) were found")
		}
	}

	// Load native subtitles if available
	if tsk.NativeSubFile != "" {
		tsk.NativeSubs, err = subs.OpenFile(tsk.NativeSubFile, false)
		if err != nil {
			return nil, tsk.Handler.LogErr(err, AbortTask, "can't read native subtitles")
		}
	}

	// Create output file (not needed for Condense mode)
	outStream, err = os.OpenFile(tsk.outputFile(), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return nil, tsk.Handler.LogErr(err, AbortTask,
			fmt.Sprintf("can't create output file: %s", tsk.outputFile()))
	}
	
	tsk.Handler.ZeroLog().Debug().
		Str("outStream", tsk.outputFile()).
		Msg("outStream file successfully open")

	// Create media output directory
	if err := os.MkdirAll(tsk.mediaOutputDir(), os.ModePerm); err != nil {
		return nil, tsk.Handler.LogErr(err, AbortTask,
			fmt.Sprintf("can't create output directory: %s", tsk.mediaOutputDir()))
	}

	// Set media prefix for file output
	tsk.MediaPrefix = filepath.Join(tsk.mediaOutputDir(), tsk.ffmpegSafeBase())

	return outStream, nil
}

// processMediaInfo handles media info extraction and audio track selection
func (tsk *Task) processMediaInfo() *ProcessingError {
	var err error
	tsk.Meta.MediaInfo, err = Mediainfo(tsk.MediaSourceFile)
	if err != nil {
		return tsk.Handler.LogErr(err, AbortTask, "failed to get media info")
	}

	if len(tsk.Meta.MediaInfo.AudioTracks) == 0 {
		return tsk.Handler.LogErr(fmt.Errorf("no audiotracks exists in file"),
			AbortTask, fmt.Sprintf("ignoring file '%s'", filepath.Base(tsk.MediaSourceFile)))
	}

	for _, fn := range []SelectionHelper{getIdealTrack, getAnyTargLangMatch, getFirstTrack} {
		if err := tsk.ChooseAudio(fn); err != nil {
			return tsk.Handler.LogErr(err, AbortAllTasks, "selecting audiotrack")
		}
	}

	tsk.Handler.ZeroLog().Debug().
		Int("UseAudiotrack", tsk.UseAudiotrack).
		Str("trackLang", tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language.Part3).
		Str("chanNum", tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Channels).Msg("")

	return nil
}

// prepareSubtitles handles subtitle preprocessing before feature processing.
// This includes:
// 1. CC bracket trimming for closed captions
// 2. Deep copy to TargSubsRaw for transliteration (preserves all ASS styles)
// 3. Filtering TargSubs to Default-style only for ASS/SSA files
func (tsk *Task) prepareSubtitles() {
	// Step 1: Check if subtitles are closed captions and trim brackets
	// Uses IsCCorDubs set by Autosub() - works for both standalone and embedded
	if tsk.IsCCorDubs {
		tsk.Handler.ZeroLog().Warn().Msg("Foreign subs are detected as closed captions and will be trimmed.")
		tsk.TargSubs.TrimCC2Dubs()
	} else {
		tsk.Handler.ZeroLog().Debug().Msg("Foreign subs are NOT detected as closed captions.")
	}

	// Step 2: Deep copy to TargSubsRaw for transliteration (preserves all styles)
	tsk.TargSubsRaw = subs.DeepCopy(tsk.TargSubs)

	// Step 3: Filter TargSubs to Default-style only for ASS/SSA files
	// Other features (subs2cards, condense) only need dialogue, not positioned signs/animations
	if isASSFormat(tsk.TargSubFile) {
		beforeCount := len(tsk.TargSubs.Items)
		tsk.TargSubs.FilterToDefaultStyle()
		afterCount := len(tsk.TargSubs.Items)
		if beforeCount != afterCount {
			tsk.Handler.ZeroLog().Info().
				Int("before", beforeCount).
				Int("after", afterCount).
				Msg("Filtered ASS subtitles to Default style only")

			// Adjust totalItems for accurate progress bar (single-file mode only)
			// In bulk mode, filtering was already done in routing.go during the scan
			if !tsk.IsBulkProcess {
				totalItems -= (beforeCount - afterCount)
			}
		}
	}
}

// isASSFormat checks if the file is an ASS or SSA subtitle file
func isASSFormat(file string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	return ext == ".ass" || ext == ".ssa"
}

// handleUserConfirmation prompts the user for confirmation in specific cases
func (tsk *Task) handleUserConfirmation() *ProcessingError {
	if tsk.IsCCorDubs && tsk.STT != "" && tsk.Handler.IsCLI() {
		tsk.Handler.ZeroLog().Warn().Msg("Speech-to-Text is requested but closed captions or dubtitles are available for the target language," +
			" which are usually reliable transcriptions of dubbings.")
		if !userConfirmed() {
			os.Exit(0)
		}
	}
	return nil
}

// processDubtitles handles the generation of dubtitles from STT results
func (tsk *Task) processDubtitles(ctx context.Context) *ProcessingError {
	if tsk.STT == "" || !tsk.WantDubs {
		return nil
	}

	// Create dubtitles from TSV
	err := tsk.TargSubs.Subs2Dubs(tsk.outputFile(), tsk.FieldSep)
	if err != nil {
		return tsk.Handler.LogErr(err, AbortTask, "making dubtitles")
	}

	// Generate output file path
	subsPath := strings.TrimSuffix(tsk.outputFile(), tsk.OutputFileExtension)
	if re := regexp.MustCompile(`(?i)subtitles?`); re.MatchString(subsPath) {
		subsPath = re.ReplaceAllString(subsPath, "DUBTITLES")
	} else {
		subsPath += ".DUBTITLES"
	}
	tsk.TargSubFile = subsPath + langkitMadeDubtitlesMarker(tsk.STT) + filepath.Ext(tsk.TargSubFile)

	// Write dubtitles to file
	if err = tsk.TargSubs.Write(tsk.TargSubFile); err != nil {
		return tsk.Handler.LogErr(err, AbortTask, "writing dubtitle file")
	}
	
	// Register the dubtitle file for final output merging if merging is enabled
	if tsk.MergeOutputFiles {
		tsk.RegisterOutputFile(tsk.TargSubFile, OutputDubtitle, tsk.Targ, "dubtitles", 90)
	}

	return nil
}

// processTransliteration handles transliteration of subtitles
func (tsk *Task) processTransliteration(ctx context.Context) *ProcessingError {
	if !tsk.WantTranslit {
		return nil
	}
	
	// TODO: find a way to provide transliteration in the TSV as well
	return tsk.Transliterate(ctx)
}

// processAudioEnhancement handles audio enhancement if requested
func (tsk *Task) processAudioEnhancement(ctx context.Context) *ProcessingError {
	if tsk.SeparationLib != "" {
		if err := tsk.enhance(ctx); err != nil {
			return err
		}
	} else if tsk.Mode == Enhance || (tsk.Mode == Condense && tsk.WantEnhancedTrack) {
		return tsk.Handler.LogErr(fmt.Errorf("tsk.SeparationLib is empty"),
			AbortAllTasks, "No separation API to isolate the voice's audio was specified.")
	}
	return nil
}

// Utility functions
func Base2Absolute(s, dir string) string {
	if s != "" {
		return filepath.Join(dir, s)
	}
	return ""
}

// FIXME No gui support
func userConfirmed() bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("Continue? (y/n): ")

		// Read user input
		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			return false
		}

		// Convert to lowercase and trim spaces/newlines
		response = strings.ToLower(strings.TrimSpace(response))

		// Check for confirmation
		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" {
			return false
		} else {
			fmt.Println("Please type 'y' or 'n' and press enter.")
		}
	}
}

func escape(s string) string {
	// https://datatracker.ietf.org/doc/html/rfc4180.html#section-2
	if strings.Contains(s, `"`) || strings.Contains(s, "\t") || strings.Contains(s, "\n") {
		var quoted = strings.ReplaceAll(s, `"`, `""`)
		return fmt.Sprintf(`"%s"`, quoted)
	}
	return s
}

func placeholder() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
