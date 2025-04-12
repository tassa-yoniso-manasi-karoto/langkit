package core

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"regexp"

	"github.com/gookit/color"
	"github.com/k0kubun/pp"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
)

var AstisubSupportedExt = []string{".srt", ".ass", ".ssa", "vtt", ".stl", ".ttml"}

// Path utility functions
func (tsk *Task) outputBase() string {
	base := strings.TrimSuffix(path.Base(tsk.TargSubFile), path.Ext(tsk.TargSubFile))
	return strings.ReplaceAll(base, "'", " ")
}

func (tsk *Task) outputFile() string {
	return path.Join(path.Dir(tsk.MediaSourceFile), tsk.outputBase()+tsk.OutputFileExtension)
}

func (tsk *Task) mediaOutputDir() string {
	return path.Join(path.Dir(tsk.MediaSourceFile), tsk.outputBase()+".media")
}

func (tsk *Task) audioBase() string {
	base := strings.TrimSuffix(path.Base(tsk.MediaSourceFile), path.Ext(tsk.MediaSourceFile))
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
		es.ParentDirPath = path.Dir(tsk.MediaSourceFile)
	})

	if procErr := tsk.validateBasicRequirements(); procErr != nil {
		reporter.SaveSnapshot("Requirements validation failed", tsk.DebugVals()) // necessity: high
		return procErr
	}

	// Handle subtitle detection and language determination
	if procErr := tsk.setupSubtitles(ctx, reporter); procErr != nil {
		return procErr
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
		tsk.Handler.ZeroLog().Debug().Msg("Skipping output merge registration - not a merge group feature")
	}

	// case where no dubtitle/STT is involved
	if tsk.Mode == Enhance || tsk.Mode == Translit {
		if procErr := tsk.processMediaInfo(); procErr != nil { // FIXME DRY
			return procErr
		}
		
		tsk.processClosedCaptions()
		
		if tsk.WantTranslit {
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

	if procErr := tsk.processMediaInfo(); procErr != nil {
		return procErr
	}
	reporter.SaveSnapshot("After media info processing", tsk.DebugVals()) // necessity: high
	reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
		es.MediaInfoDump = fmt.Sprintf("%+v", tsk.Meta.MediaInfo) // necessity: high
	})

	tsk.processClosedCaptions()
	
	if procErr := tsk.handleUserConfirmation(); procErr != nil {
		return procErr
	}

	// Launch main app logic: supervisor
	if tsk.Mode == Subs2Cards || tsk.Mode == Subs2Dubs {
		if err := tsk.Supervisor(ctx, outStream, write); err != nil {
			reporter.SaveSnapshot("Supervisor failed", tsk.DebugVals()) // necessity: critical
			return err
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

goodEnd:
	tsk.Handler.ZeroLog().Info().Msg("Processing completed")
	return nil
}

// Autosub automatically discovers subtitle files based on the media filename
func (tsk *Task) Autosub() *ProcessingError {
	files, err := os.ReadDir(filepath.Dir(tsk.MediaSourceFile))
	if err != nil {
		return tsk.Handler.LogErr(err, AbortTask, "autosub: failed to read directory")
	}
	trimmedMedia := strings.TrimSuffix(path.Base(tsk.MediaSourceFile), path.Ext(tsk.MediaSourceFile))
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Name()))
		trimmed := strings.TrimSuffix(file.Name(), path.Ext(file.Name()))
		if isLangkitMadeDubtitles(file.Name()) || isLangkitMadeTranslit(file.Name()) ||
			!slices.Contains(AstisubSupportedExt, ext) ||
				strings.Contains(strings.ToLower(trimmed), "forced") ||
					!strings.HasPrefix(trimmed, trimmedMedia) ||
						file.IsDir()  {
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
	tsk.NativeSubFile  = Base2Absolute(tsk.NativeSubFile, path.Dir(tsk.MediaSourceFile))
	tsk.TargSubFile = Base2Absolute(tsk.TargSubFile, path.Dir(tsk.MediaSourceFile))
	if tsk.Mode != Enhance && tsk.TargSubFile == "" {
		return tsk.Handler.LogErrFields(fmt.Errorf("no subtitle file in %s was found", tsk.Targ.Name), AbortTask,
			"autosubs failed", map[string]interface{}{"video": path.Base(tsk.MediaSourceFile)})
	}
	if tsk.Mode != Subs2Cards {
		return nil
	}
	if tsk.NativeSubFile == "" {
		tsk.Handler.ZeroLog().Warn().Str("video", path.Base(tsk.MediaSourceFile)).Msg("No sub file for reference/native language was found")
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

// Audio track selection functions
func (tsk *Task) ChooseAudio(helper func(tsk *Task, i int, track AudioTrack) error) (err error) {
	if tsk.UseAudiotrack < 0 {
		for i, track := range tsk.Meta.MediaInfo.AudioTracks {
			if err = helper(tsk, i, track); err != nil {
				return
			}
		}
	}
	return
}

type SelectionHelper func(*Task, int, AudioTrack) error

func getIdealTrack(tsk *Task, i int, track AudioTrack) error {
	num, _ := strconv.Atoi(track.Channels)
	tsk.Handler.ZeroLog().Trace().
		Bool("isTargLang?", *track.Language == *tsk.Targ.Language).
		Bool("isTargetChanNum?", num == tsk.TargetChan).
		Bool("track.Title_empty?", track.Title == "").
		Bool("track.Title_notEmpty_notAudioDescr", track.Title != "" && !strings.Contains(strings.ToLower(track.Title), "audio description")).
		Msg("getIdealTrack")
	if *track.Language == *tsk.Targ.Language && num == tsk.TargetChan &&
		(track.Title == "" || track.Title != "" && !strings.Contains(strings.ToLower(track.Title), "audio description")) {
			tsk.UseAudiotrack = i
			tsk.Handler.ZeroLog().Debug().Msg("getIdealTrack selected UseAudiotrack")
	}
	return nil
}

func getAnyTargLangMatch(tsk *Task, i int, track AudioTrack) error {
	tsk.Handler.ZeroLog().Trace().
		Bool("isTargLang?", *track.Language == *tsk.Targ.Language).Msg("getAnyTargLangMatch")
	if *track.Language == *tsk.Targ.Language {
		tsk.UseAudiotrack = i
		tsk.Handler.ZeroLog().Debug().Msg("getAnyTargLangMatch selected UseAudiotrack")
	}
	return nil
}

func getFirstTrack(tsk *Task, i int, track AudioTrack) error {
	tsk.Handler.ZeroLog().Trace().
		Bool("hasLang", track.Language != nil).
		Bool("lang_isn't_target", *track.Language != *tsk.Targ.Language).Msg("getFirstTrack")
	if track.Language != nil && *track.Language != *tsk.Targ.Language {
		return fmt.Errorf("No audiotrack tagged with the requested target language exists. " +
			"If it isn't a misinput please use the audiotrack override to set a track number manually.")
	}
	// Having found no audiotrack tagged with target language, we can
	// assume first audiotrack is the target if it doesn't have a language tag
	tsk.UseAudiotrack = i
	tsk.Handler.ZeroLog().Debug().Msg("getFirstTrack selected UseAudiotrack")
	return nil
}

// Utility functions
func Base2Absolute(s, dir string) string {
	if s != "" {
		return path.Join(dir, s)
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

// Helper functions for testability

// validateBasicRequirements validates the basic requirements for task execution
func (tsk *Task) validateBasicRequirements() *ProcessingError {
	// Check for apostrophe in directory path (ffmpeg limitation)
	if strings.Contains(filepath.Dir(tsk.TargSubFile), "'") {
		return tsk.Handler.Log(Error, AbortTask,
			"Due to ffmpeg limitations, the path of the directory in which the files are located must not contain an apostrophe ('). "+
				"Apostrophe in the names of the files themselves are supported using a workaround.")
	}
	
	// FIXME Probably return on mode.Enhance here
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
	
	// Validate requirements for Subs2Cards mode
	if tsk.Mode == Subs2Cards {
		if len(tsk.Langs) < 2 && tsk.NativeSubFile == "" {
			return nil, tsk.Handler.LogErr(err, AbortTask, "Neither native language and nor native subtitle file was specified.")
		}
		if tsk.NativeSubFile == "" {
			tsk.Handler.ZeroLog().Warn().
				Str("video", path.Base(tsk.MediaSourceFile)).
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

	// Create output file
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
	tsk.MediaPrefix = path.Join(tsk.mediaOutputDir(), tsk.outputBase())
	
	return outStream, nil
}

// processMediaInfo handles media info extraction and audio track selection
func (tsk *Task) processMediaInfo() *ProcessingError {
	// Extract media info
	tsk.Meta.MediaInfo = Mediainfo(tsk.MediaSourceFile)

	// Skip audio track selection for Translit mode
	if tsk.Mode == Translit {
		return nil
	}

	// Select appropriate audio track using selection helpers
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
	if tsk.Mode == Enhance {
		return
	}

	// Check if subtitles are closed captions and process accordingly
	if strings.Contains(strings.ToLower(tsk.TargSubFile), "closedcaption") {
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


func langkitMadeDubtitlesMarker(STTModel string) string {
	return "." + strings.ToUpper(STTModel)
}

// allows rejecting those files during subfile screening (see lang.go)
func isLangkitMadeDubtitles(s string) bool {
	for _, model := range voice.GetAllSTTModels() {
		if strings.Contains(s, langkitMadeDubtitlesMarker(model.Name)) {
			return true
		}
	}
	
	return false
}
// allows rejecting those files during subfile screening (see lang.go)
func isLangkitMadeTranslit(s string) bool {
	for _, t := range []TranslitType{Tokenize, Romanize, Selective} {
		if strings.Contains(s, t.ToSuffix()) {
			return true
		}
	}
	return false
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
	} else if tsk.Mode == Enhance {
		tsk.Handler.ZeroLog().Error().Msg("No separation API to isolate the voice's audio was specified.")
	}
	return nil
}

func placeholder() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
