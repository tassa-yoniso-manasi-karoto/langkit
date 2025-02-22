package core

import (
	"fmt"
	"os"
	"path"
	"strings"
	"strconv"
	"path/filepath"
	"slices"
	"bufio"
	"context"

	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	//"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)

var AstisubSupportedExt = []string{".srt", ".ass", ".ssa", "vtt", ".stl", ".ttml"}

func (tsk *Task) outputBase() string {
	base := strings.TrimSuffix(path.Base(tsk.TargSubFile), path.Ext(tsk.TargSubFile))
	return strings.ReplaceAll(base, "'", " ")
}

func (tsk *Task) outputFile() string {
	return path.Join(path.Dir(tsk.MediaSourceFile), tsk.outputBase() + tsk.OutputFileExtension)
}

func (tsk *Task) mediaOutputDir() string {
	return path.Join(path.Dir(tsk.MediaSourceFile), tsk.outputBase()+".media")
}

func (tsk *Task) audioBase() string {
	base := strings.TrimSuffix(path.Base(tsk.MediaSourceFile), path.Ext(tsk.MediaSourceFile))
	return base
}


// TODO Wait for Claude 4 release to break large functions and write some tests.
func (tsk *Task) Execute(ctx context.Context) *ProcessingError {
	var err error // compiler shenanigans FIXME rm later
	
	reporter := crash.Reporter
	reporter.ClearExecutionRecords()
	
	reporter.SaveSnapshot("Starting execution", tsk.DebugVals())
	reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
		es.ParentDirPath = path.Dir(tsk.MediaSourceFile)
	})
	
	// "'" in filename will break the format that ffmpeg's concat filter requires.
	// in their file format, no escaping is supported → must be trimmed from mediaPrefix.
	if strings.Contains(filepath.Dir(tsk.TargSubFile), "'") {
		return tsk.Handler.Log(Error, AbortTask,
			"Due to ffmpeg limitations, the path of the directory in which the files are located must not contain an apostrophe (')." +
				"Apostrophe in the names of the files themselves are supported using a workaround.")
	}
	
	if len(tsk.Langs) == 0 && tsk.TargSubFile == "" {
		return tsk.Handler.Log(Error, AbortAllTasks,
			"Neither languages and nor subtitle files were specified.")
	}
	if tsk.TargSubFile == "" {
		reporter.SaveSnapshot("Running Autosub", tsk.DebugVals())
		if procErr:= tsk.Autosub(); procErr != nil {
			reporter.SaveSnapshot("Autosub failed", tsk.DebugVals())
			return procErr
		}
	} else {
		tsk.Targ, err = GuessLangFromFilename(tsk.TargSubFile)
		if err != nil {
                       tsk.Handler.ZeroLog().Warn().Err(err).
                               Str("TargSubFile", tsk.TargSubFile).
                               Msg("Couldn't guess the language of foreign subtitle file")
		}
		// NOTE: Native subtitle declared in CLI must trail foreign subtitle declaration,
		// thus if the native subtitle's lang needs guessing, TargSubFile can't be empty
		// = no need for a new if
		tsk.Native, err = GuessLangFromFilename(tsk.NativeSubFile)
		if tsk.NativeSubFile != "" && err != nil {
                       tsk.Handler.ZeroLog().Warn().Err(err).
                               Str("NativeSubFile", tsk.NativeSubFile).
                               Msg("Couldn't guess the language of native subtitle file")
		}
		tsk.Handler.ZeroLog().Trace().
			Str("Targ", tsk.Targ.String()).
			Str("Native", tsk.Native.String()).
			Msg("No language flag passed. Attempted to guess language from filename.")
	}
	var outStream *os.File
	tsk.TargSubs, err = subs.OpenFile(tsk.TargSubFile, false)
	if err != nil {
		return tsk.Handler.LogErr(err, AbortTask, "can't read foreign subtitles")
	}
	switch tsk.Mode { // TODO remove when I rewrite Execute with proper functions
	case Enhance, Translit:
		goto ResumeEnhance
	}
	// if not in bulk mode then it wasn't assigned yet
	if totalItems == 0 {
		totalItems = len(tsk.TargSubs.Items)
	}
	if tsk.Mode == Subs2Cards {
		if len(tsk.Langs) < 2 && tsk.NativeSubFile == "" {
			return tsk.Handler.LogErr(err, AbortTask, "Neither native language and nor native subtitle file was specified.")
		}
		if tsk.NativeSubFile == "" { // FIXME maybe redundant
			tsk.Handler.ZeroLog().Warn().
				Str("video", path.Base(tsk.MediaSourceFile)).
				Msg("No sub file for any of the desired reference language(s) were found")
		}
	}
	if tsk.NativeSubFile != "" {
		tsk.NativeSubs, err = subs.OpenFile(tsk.NativeSubFile, false)
		if err != nil {
			return tsk.Handler.LogErr(err, AbortTask, "can't read native subtitles")
		}
	}
	outStream, err = os.OpenFile(tsk.outputFile(), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return tsk.Handler.LogErr(err, AbortTask,
			fmt.Sprintf("can't create output file: %s", tsk.outputFile()))
	} else {
		tsk.Handler.ZeroLog().Debug().
			Str("outStream", tsk.outputFile()).
			Msg("outStream file successfully open")
	}
	defer outStream.Close()

	if err := os.MkdirAll(tsk.mediaOutputDir(), os.ModePerm); err != nil {
		return tsk.Handler.LogErr(err, AbortTask,
			fmt.Sprintf("can't create output directory: %s", tsk.mediaOutputDir()))
	}
	tsk.MediaPrefix = path.Join(tsk.mediaOutputDir(), tsk.outputBase())
ResumeEnhance:
	tsk.Meta.MediaInfo = Mediainfo(tsk.MediaSourceFile)
	/* MediaInfo // FIXME use this to cancel susb2dubs if *tsk.Targ == *tsk.OriginalLang
	for _, track := range tsk.Meta.AudioTracks {
		if strings.Contains(strings.ToLower(track.Title), "original") {
			tsk.OriginalLang = track.Language
		}
	}
	if tsk.OriginalLang != "" {
		tsk.Handler.ZeroLog().Info().Msg("Detected original lang:"+ tsk.OriginalLang)
	}*/

	if tsk.Mode != Translit {
		for _, fn := range []SelectionHelper{getIdealTrack, getAnyTargLangMatch, getFirstTrack} {
			if err := tsk.ChooseAudio(fn); err != nil {
				return tsk.Handler.LogErr(err, AbortAllTasks, "selecting audiotrack")
			}
		}
		tsk.Handler.ZeroLog().Debug().
			Int("UseAudiotrack", tsk.UseAudiotrack).
			Str("trackLang", tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language.Part3).
			Str("chanNum", tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Channels).Msg("")
	}
	
	if tsk.Mode != Enhance {
		if strings.Contains(strings.ToLower(tsk.TargSubFile), "closedcaption") {
			tsk.Handler.ZeroLog().Warn().Msg("Foreign subs are detected as closed captions and will be trimmed into dubtitles.")
			tsk.TargSubs.TrimCC2Dubs()
		} else {
			tsk.Handler.ZeroLog().Debug().Msg("Foreign subs are NOT detected as closed captions.")
		}
	}
	// FIXME this warning won't occur if the sub file are passed as arg
	// FIXME HANDLE GUI
	if tsk.IsCCorDubs && tsk.STT != "" && tsk.Handler.IsCLI() {
		tsk.Handler.ZeroLog().Warn().Msg("Speech-to-Text is requested but closed captions or dubtitles are available for the target language," +
			" which are usually reliable transcriptions of dubbings.")
		if !userConfirmed() {
			os.Exit(0)
		}	
	}
	if tsk.Mode == Subs2Cards || tsk.Mode == Subs2Dubs {
		if err := tsk.Supervisor(ctx, outStream, write); err != nil {
			return err
		}
	}
	
	// subs is the path of the reference subtitle/dubtitle file to use when transliterating
	subs := tsk.TargSubFile
	if tsk.STT != "" && tsk.WantDubs {
		// Subs2Dubs uses the TSV file containing transcriptions to
		// transform the subtitles into dubtitles in place
		err = tsk.TargSubs.Subs2Dubs(tsk.outputFile(), tsk.FieldSep)
		if err != nil {
			return tsk.Handler.LogErr(err, AbortTask, "making dubtitles")
		}
		subs = strings.ReplaceAll(tsk.outputFile(), "subtitles", "DUBTITLES")
		subs = strings.TrimSuffix(subs, ".tsv")
		subs = subs + langkitMadeDubtitlesMarker(tsk.STT) + filepath.Ext(tsk.TargSubFile)
		
		if err = tsk.TargSubs.Write(subs); err != nil {
			return tsk.Handler.LogErr(err, AbortTask, "writing dubtitle file")
		}
	}
	if tsk.WantTranslit {
		// TODO: find a way to provide transliteration in the TSV as well
		if err := tsk.Translit(ctx, subs); err != nil {
			return err
		}
	}
	if tsk.SeparationLib != "" {
		if err := tsk.enhance(ctx); err != nil {
			return err
		}
	} else if tsk.Mode == Enhance {
		tsk.Handler.ZeroLog().Error().Msg("No separation API to isolate the voice's audio was specified.")
	}
	
	tsk.Handler.ZeroLog().Info().Msg("Processing completed")
	return nil
}


func (tsk *Task) Autosub() *ProcessingError {
	files, err := os.ReadDir(filepath.Dir(tsk.MediaSourceFile))
	if err != nil {
		return tsk.Handler.LogErr(err, AbortTask, "autosub: failed to read directory")
	}
	trimmedMedia := strings.TrimSuffix(path.Base(tsk.MediaSourceFile), path.Ext(tsk.MediaSourceFile))
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Name()))
		trimmed := strings.TrimSuffix(file.Name(), path.Ext(file.Name()))
		if file.IsDir() ||
			!slices.Contains(AstisubSupportedExt, ext) ||
				!strings.HasPrefix(trimmed, trimmedMedia) ||
					strings.Contains(strings.ToLower(trimmed), "forced") {
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
	if tsk.TargSubFile == "" {
		return tsk.Handler.LogFields(Error, AbortTask,
				"autosub: no sub file for desired target language was found",
					map[string]interface{}{"video": path.Base(tsk.MediaSourceFile)})
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


func Base2Absolute(s, dir string) string {
	if s != "" {
		return path.Join(dir, s)
	}
	return ""
}

func langkitMadeDubtitlesMarker(STT string) string {
	return "." + strings.ToUpper(STT)
}

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
