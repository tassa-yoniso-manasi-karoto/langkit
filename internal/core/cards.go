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

var AstisubSupportedExt = []string{".srt", ".ass", ".ssa", "vtt", ".stl", ".ttml"}

// Path utility functions
func (tsk *Task) outputBase() string {
	base := strings.TrimSuffix(filepath.Base(tsk.TargSubFile), filepath.Ext(tsk.TargSubFile))
	return strings.ReplaceAll(base, "'", " ")
}

func (tsk *Task) outputFile() string {
	return filepath.Join(filepath.Dir(tsk.MediaSourceFile), tsk.outputBase()+tsk.OutputFileExtension)
}

func (tsk *Task) mediaOutputDir() string {
	return filepath.Join(filepath.Dir(tsk.MediaSourceFile), tsk.outputBase()+".media")
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
			tsk.processClosedCaptions()
			
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

	tsk.processClosedCaptions()
	
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
				tsk.MediaPrefix = filepath.Join(mediaOutDir, tsk.outputBase())
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
				if err := tsk.ConcatWAVsToAudio("CONDENSED"); err != nil {
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
	files, err := os.ReadDir(filepath.Dir(tsk.MediaSourceFile))
	tsk.Handler.ZeroLog().Debug().
		Int("num_files", len(files)).
		Str("MediaSourceFile", tsk.MediaSourceFile).
		Msgf("Reading parent dir of media file: \"%s\"", filepath.Dir(tsk.MediaSourceFile))

	fileNames := make([]string, len(files))
	for i, entry := range files {
		fileNames[i] = entry.Name()
	}

	tsk.Handler.ZeroLog().Trace().
		Strs("files", fileNames).
		Msg("File list of parent dir of media file")

	if err != nil {
		return tsk.Handler.LogErr(err, AbortTask, "autosub: failed to read directory")
	} else if len(files) == 0 {
		return tsk.Handler.LogErr(err, AbortTask, "autosub: read directory but retrieved file list is empty")
	}
	trimmedMedia := strings.TrimSuffix(filepath.Base(tsk.MediaSourceFile), filepath.Ext(tsk.MediaSourceFile))

	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Name()))
		trimmed := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))

		if file.IsDir() {
			tsk.Handler.ZeroLog().Debug().
				Str("directory", file.Name()).
				Msg("Skipping: Entry is a directory")
			continue
		}
		
		if isLangkitMadeDubtitles(file.Name()) {
			tsk.Handler.ZeroLog().Debug().
				Str("file", file.Name()).
				Msg("Skipping: Is a Langkit-generated dubtitle file")
			continue
		}

		if isLangkitMadeTranslit(file.Name()) {
			tsk.Handler.ZeroLog().Debug().
				Str("file", file.Name()).
				Msg("Skipping: Is a Langkit-generated transliteration file")
			continue
		}
		
		if !slices.Contains(AstisubSupportedExt, ext) {
			tsk.Handler.ZeroLog().Debug().
				Str("file", file.Name()).
				Str("extension", ext).
				Msg("Skipping: Unsupported subtitle extension")
			continue
		}

		if strings.Contains(strings.ToLower(trimmed), "forced") {
			tsk.Handler.ZeroLog().Debug().
				Str("file", file.Name()).
				Msg("Skipping: Filename contains 'forced'")
			continue
		}

		if !strings.HasPrefix(trimmed, trimmedMedia) {
			tsk.Handler.ZeroLog().Debug().
				Str("file", file.Name()).
				Str("media_prefix", trimmedMedia).
				Msg("Skipping: Filename does not match the media file's prefix")
			continue
		}
		
		l, err := GuessLangFromFilename(file.Name())
		if err != nil {
			tsk.Handler.ZeroLog().Debug().Err(err).Msg("guessing lang")
			continue
		}
		
		tsk.Handler.ZeroLog().Debug().
			Str("Guessed lang", l.Part3).
			Str("Subtag", l.Subtag).
			Msgf("File: %s", file.Name())

		// Check if subtitle name matches our target language
		tsk.SetPreferred([]Lang{tsk.Targ}, l, tsk.Targ, file.Name(), &tsk.TargSubFile, &tsk.Targ)

		// Check if subtitle name matches any of our native/reference languages
		for _, RefLang := range tsk.RefLangs {
			tsk.IsCCorDubs = tsk.SetPreferred(tsk.RefLangs, l, RefLang, file.Name(), &tsk.NativeSubFile, &tsk.Native)
		}
	}
	tsk.Handler.ZeroLog().Info().Str("Automatically chosen Target subtitle", tsk.TargSubFile).Msg("")
	tsk.NativeSubFile = Base2Absolute(tsk.NativeSubFile, filepath.Dir(tsk.MediaSourceFile))
	tsk.TargSubFile = Base2Absolute(tsk.TargSubFile, filepath.Dir(tsk.MediaSourceFile))
	if tsk.TargSubFile == "" {
		return tsk.Handler.LogErrFields(fmt.Errorf("no subtitle file in %s was found", tsk.Targ.Name), AbortTask,
			"autosubs failed", map[string]interface{}{"video": filepath.Base(tsk.MediaSourceFile)})
	}
	if tsk.Mode != Subs2Cards {
		return nil
	}
	if tsk.NativeSubFile == "" {
		tsk.Handler.ZeroLog().Warn().Str("video", filepath.Base(tsk.MediaSourceFile)).Msg("No sub file for reference/native language was found")
	} else {
		tsk.Handler.ZeroLog().Info().Str("Automatically chosen Native subtitle", tsk.NativeSubFile).Msg("")
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
	
	if !tsk.IsBulkProcess {
		totalItems = len(tsk.TargSubs.Items)
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
		tsk.MediaPrefix = filepath.Join(tsk.mediaOutputDir(), tsk.outputBase())
		
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
	tsk.MediaPrefix = filepath.Join(tsk.mediaOutputDir(), tsk.outputBase())

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

// processClosedCaptions handles closed caption detection and processing
func (tsk *Task) processClosedCaptions() {
	// Check if subtitles are closed captions and process accordingly
	if isClosedCaptions(tsk.TargSubFile) {
		tsk.Handler.ZeroLog().Warn().Msg("Foreign subs are detected as closed captions and will be trimmed into dubtitles.")
		tsk.TargSubs.TrimCC2Dubs()
	} else {
		tsk.Handler.ZeroLog().Debug().Msg("Foreign subs are NOT detected as closed captions.")
	}
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

func isClosedCaptions(file string) bool {
	return strings.Contains(strings.ToLower(file), "closedcaption")
}

func placeholder() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
