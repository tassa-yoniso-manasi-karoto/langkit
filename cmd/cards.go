package cmd

import (
	"fmt"
	"os"
	"path"
	"strings"
	"strconv"
	"path/filepath"
	"slices"
	"bufio"

	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	//"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/subs"
	//"github.com/schollz/progressbar/v3"
)

var AstisubSupportedExt = []string{".srt", ".ass", ".ssa", "vtt", ".stl", ".ttml"}

func (tsk *Task) outputBase() string {
	// "'" in filename will break the format that ffmpeg's concat filter requires.
	// in their file format, no escaping is supported → must be trimmed from mediaPrefix.
	if strings.Contains(filepath.Dir(tsk.TargSubFile), "'") {
		tsk.Log.Fatal().Msg(
			"Due to ffmpeg limitations, the path of the directory in which the files are located must not contain an apostrophe (')." +
				"Apostrophe in the names of the files themselves are supported using a workaround.",
		)
	}
	base := strings.TrimSuffix(path.Base(tsk.TargSubFile), path.Ext(tsk.TargSubFile))
	return strings.ReplaceAll(base, "'", " ")
}

func (tsk *Task) outputFile() string {
	return path.Join(path.Dir(tsk.TargSubFile), tsk.outputBase()+"."+tsk.OutputFileExtension)
}

func (tsk *Task) mediaOutputDir() string {
	return path.Join(path.Dir(tsk.TargSubFile), tsk.outputBase()+".media")
}

func escape(s string) string {
	// https://datatracker.ietf.org/doc/html/rfc4180.html#section-2
	if strings.Contains(s, `"`) || strings.Contains(s, "\t") || strings.Contains(s, "\n") {
		var quoted = strings.ReplaceAll(s, `"`, `""`)
		return fmt.Sprintf(`"%s"`, quoted)
	}
	return s
}


func (tsk *Task) Execute() {
	//if tsk.Langs != nil {
	//	tsk.PrepareLangs()
	//}
	//pp.Println(tsk.Targ)
	//pp.Println(tsk.RefLangs)
	if tsk.TargSubFile == "" {
		tsk.Autosub()
	}
	foreignSubs, err := subs.OpenFile(tsk.TargSubFile, false)
	if err != nil {
		tsk.Log.Fatal().Err(err).Msg("can't read foreign subtitles")
	}
	if !tsk.DubsOnly && tsk.NativeSubFile == "" {
		tsk.Log.Warn().Str("video", path.Base(tsk.MediaSourceFile)).Msg("No sub file for any of the desired reference language(s) were found")
	}
	if tsk.NativeSubFile != "" {
		tsk.NativeSubs, err = subs.OpenFile(tsk.NativeSubFile, false)
		if err != nil {
			tsk.Log.Fatal().Err(err).Msg("can't read native subtitles")
		}
	}
	outStream, err := os.OpenFile(tsk.outputFile(), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		tsk.Log.Fatal().Err(err).Msg(fmt.Sprintf("can't create output file: %s", tsk.outputFile()))
	}
	defer outStream.Close()

	if err := os.MkdirAll(tsk.mediaOutputDir(), os.ModePerm); err != nil {
		tsk.Log.Fatal().Err(err).Msg(fmt.Sprintf("can't create output directory: %s", tsk.mediaOutputDir()))
	}
	tsk.MediaPrefix = path.Join(tsk.mediaOutputDir(), tsk.outputBase())
	tsk.Meta.MediaInfo = mediainfo(tsk.MediaSourceFile)
	/*	MediaInfo
	for _, track := range tsk.Meta.AudioTracks {
		if strings.Contains(strings.ToLower(track.Title), "original") {
			tsk.OriginalLang = track.Language
		}
	}
	if tsk.OriginalLang != "" {
		tsk.Log.Info().Msg("Detected original lang:"+ tsk.OriginalLang)
	}*/
	// FIXME range over func instead of calling it 3 times
	tsk.ChooseAudio(func(i int, track AudioTrack) {
		num, _ := strconv.Atoi(track.Channels)
		if track.Language == tsk.Targ.Language && num == tsk.TargetChan {
			tsk.UseAudiotrack = i
		}
	})
	tsk.ChooseAudio(func(i int, track AudioTrack) {
		if track.Language == tsk.Targ.Language {
			tsk.UseAudiotrack = i
		}
	})
	if tsk.UseAudiotrack < 0 {
		tsk.UseAudiotrack = 0
	}
	tsk.Log.Debug().
		Int("UseAudiotrack", tsk.UseAudiotrack).
		Str("trackLang", tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language.Part3).
		Str("chanNum", tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Channels).Msg("")

	if tsk.SeparationLib != "" {
		tsk.enhance()
	}
	if strings.Contains(strings.ToLower(tsk.TargSubFile), "closedcaption") {
		tsk.Log.Info().Msg("Foreign subs are detected as closed captions and will be trimmed into dubtitles.")
		foreignSubs.TrimCC2Dubs()
	} else {
		tsk.Log.Debug().Msg("Foreign subs are NOT detected as closed captions.")
	}
	// NOTE: this warning won't occur if the sub file are passed as arg
	if tsk.IsCCorDubs && tsk.STT != "" {
		tsk.Log.Warn().Msg("Speech-to-Text is requested but closed captions or dubtitles are available for the target language," +
			" which are usually reliable transcriptions of dubbings.")
		if !userConfirmed() {
			os.Exit(0)
		}	
	}
	switch tsk.STT {
	case "wh":
		tsk.STT = "whisper"
	case "fast", "incredibly-fast-whisper":
		tsk.STT = "insanely-fast-whisper"
	case "u1":
		tsk.STT = "universal-1"
	}
	tsk.Supervisor(foreignSubs, outStream, write)
	
	if tsk.STT != "" && tsk.WantDubs {
		err = foreignSubs.Subs2Dubs(tsk.outputFile(), tsk.FieldSep)
		if err != nil {
			tsk.Log.Fatal().Err(err).Msg("error making dubtitles")
		}
		dubs := strings.ReplaceAll(tsk.outputFile(), "subtitles", "DUBTITLES")
		dubs = strings.TrimSuffix(dubs, ".tsv")
		dubs = path.Join(dubs + "." + strings.ToUpper(tsk.STT) + filepath.Ext(tsk.TargSubFile))
		
		if err = foreignSubs.Write(dubs); err != nil {
			tsk.Log.Fatal().Err(err).Msg("error making dubtitles")
		}
	}
}


func (tsk *Task) Autosub() {
	files, err := os.ReadDir(filepath.Dir(tsk.MediaSourceFile))
	if err != nil {
		tsk.Log.Fatal().Err(err).Msg("Failed to read directory")
	}
	trimmedMedia := strings.TrimSuffix(path.Base(tsk.MediaSourceFile), path.Ext(tsk.MediaSourceFile))
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Name()))
		trimmed := strings.TrimSuffix(file.Name(), path.Ext(file.Name()))
		if file.IsDir() ||
			!slices.Contains(AstisubSupportedExt, ext) ||
				!strings.HasPrefix(trimmed, trimmedMedia) ||
					strings.Contains(trimmed, "forced") {
			continue
		}
		l, err := GuessLangFromFilename(file.Name())
		if err != nil {
			continue
		}
		//fmt.Printf("Guessed lang: %s\tSubtag: %s\tFile: %s\n", l.Part3, l.Subtag, file.Name())
		
		SetPrefered([]Lang{tsk.Targ}, l, tsk.Targ, file.Name(), &tsk.TargSubFile)
		for _, RefLang := range tsk.RefLangs {
			tsk.IsCCorDubs = SetPrefered(tsk.RefLangs, l, RefLang, file.Name(), &tsk.NativeSubFile)
		}
	}
	tsk.Log.Info().Str("Automatically chosen Target subtitle", tsk.TargSubFile).Msg("")
	if !tsk.DubsOnly {
		tsk.Log.Info().Str("Automatically chosen Native subtitle", tsk.NativeSubFile).Msg("")
	}
	tsk.NativeSubFile  = Base2Absolute(tsk.NativeSubFile, path.Dir(tsk.MediaSourceFile))
	tsk.TargSubFile = Base2Absolute(tsk.TargSubFile, path.Dir(tsk.MediaSourceFile))
	if tsk.TargSubFile == "" {
		tsk.Log.Fatal().Str("video", path.Base(tsk.MediaSourceFile)).Msg("No sub file for desired target language was found")
	}
}


func (tsk *Task) PrepareLangs() {
	if len(tsk.Langs) == 0 && tsk.TargSubFile == "" {
		tsk.Log.Fatal().Msg("Neither languages and nor subtitle files were specified.")
	} else if len(tsk.Langs) == 1 && !tsk.DubsOnly {
		tsk.Log.Fatal().Msg("Passed languages are improperly formatted or incomplete.")
	}
	if len(tsk.Langs) > 0 {
		tmp, err := ReadStdLangCode([]string{tsk.Langs[0]})
		if err != nil {
			tsk.Log.Fatal().Err(err).Msg("Language parsing error")
		}
		tsk.Targ = tmp[0]
	}
	if len(tsk.Langs) > 1 {
		tmp, err := ReadStdLangCode(tsk.Langs[1:])
		if err != nil {
			tsk.Log.Fatal().Err(err).Msg("Language parsing error")
		}
		tsk.RefLangs = tmp
	}
	tsk.Langs = nil
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


func (tsk *Task) ChooseAudio(f func(i int, track AudioTrack)) {
	if tsk.UseAudiotrack < 0 {
		for i, track := range tsk.Meta.MediaInfo.AudioTracks {
			f(i, track)
		}
	}
}


func Base2Absolute(s, dir string) string {
	if s != "" {
		return path.Join(dir, s)
	}
	return ""
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

func placeholder() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
