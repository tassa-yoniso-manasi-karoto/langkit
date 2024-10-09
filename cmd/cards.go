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
	// No escaping is supported → must be trimmed from mediaPrefix.
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
	if tsk.MediaSourceFile == "" {
		tsk.Log.Fatal().Msg("A media file must be specified.")
	}
	var nativeSubs *subs.Subtitles
	
	if len(tsk.Langs) == 0 && tsk.TargSubFile == "" {
		tsk.Log.Fatal().Msg("Neither languages and nor subtitle files were specified.")
	} else if len(tsk.Langs) == 1 {
		tsk.Log.Fatal().Msg("Passed languages are improperly formatted or incomplete.")
	} else if len(tsk.Langs) > 1 {
		tmp, err := ReadStdLangCode([]string{tsk.Langs[0]})
		if err != nil {
			tsk.Log.Fatal().Err(err).Msg("Language parsing error")
		}
		tsk.Targ = tmp[0]
		tsk.RefLangs, err = ReadStdLangCode(tsk.Langs[1:])
		if err != nil {
			tsk.Log.Fatal().Err(err).Msg("Language parsing error")
		}
	}
	//pp.Println(tsk.Targ)
	//pp.Println(tsk.RefLangs)
	//### AUTOSUB ##########################
	if tsk.TargSubFile == "" {
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
				tsk.IsCCorDubs = SetPrefered(tsk.RefLangs, l, RefLang, file.Name(), &tsk.RefSubFile)
			}
		}
		tsk.RefSubFile  = Base2Absolute(tsk.RefSubFile, path.Dir(tsk.MediaSourceFile))
		tsk.TargSubFile = Base2Absolute(tsk.TargSubFile, path.Dir(tsk.MediaSourceFile))
		
	}
	if tsk.TargSubFile == "" {
		tsk.Log.Fatal().Str("video", path.Base(tsk.MediaSourceFile)).Msg("No sub file for desired target language were found")
	}
	if tsk.RefSubFile == "" {
		tsk.Log.Fatal().Str("video", path.Base(tsk.MediaSourceFile)).Msg("No sub file for any of the desired reference language(s) were found")
	}
	color.Redln("TARG:", tsk.TargSubFile)
	color.Redln("REF:", tsk.RefSubFile) // FIXME
	//color.Greenln("WIP!")
	//os.Exit(0)
	//#######################################
	foreignSubs, err := subs.OpenFile(tsk.TargSubFile, false)
	if err != nil {
		tsk.Log.Fatal().Err(err).Msg("can't read foreign subtitles")
	}
	nativeSubs, err = subs.OpenFile(tsk.RefSubFile, false)
	if err != nil {
		tsk.Log.Fatal().Err(err).Msg("can't read native subtitles")
	}
	outStream, err := os.OpenFile(tsk.outputFile(), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		tsk.Log.Fatal().Err(err).Msg(fmt.Sprintf("can't create output file: %s", tsk.outputFile()))
	}
	defer outStream.Close()

	if err := os.MkdirAll(tsk.mediaOutputDir(), os.ModePerm); err != nil {
		tsk.Log.Fatal().Err(err).Msg(fmt.Sprintf("can't create output directory: %s", tsk.mediaOutputDir()))
	}
	mediaPrefix := path.Join(tsk.mediaOutputDir(), tsk.outputBase())
	tsk.Meta = mediainfo(tsk.MediaSourceFile)
	/*for _, track := range tsk.Meta.AudioTracks {
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
	tsk.Log.Info().
		Int("UseAudiotrack", tsk.UseAudiotrack).
		Str("track lang", tsk.Meta.AudioTracks[tsk.UseAudiotrack].Language.Part3).
		Str("chan num", tsk.Meta.AudioTracks[tsk.UseAudiotrack].Channels)

	if tsk.SeparationLib != "" {
		tsk.enhance()
	}
	if strings.Contains(strings.ToLower(tsk.TargSubFile), "closedcaption") {
		foreignSubs.DumbDown2Dubs()
		tsk.Log.Info().Msg("Foreign subs are closed captions.")
	}
	// NOTE: this warning won't occur if the sub file are passed as arg
	if tsk.IsCCorDubs && tsk.STT != "" {
		tsk.Log.Warn().Msg("Speech-to-Text is requested but closed captions or dubtitles are available for the target language," +
			" which are usually reliable transcriptions of dubbings.")
		if !userConfirmed() {
			os.Exit(0)
		}	
	}
	tsk.ExportItems(foreignSubs, nativeSubs, tsk.outputBase(), tsk.MediaSourceFile, mediaPrefix, func(item *ExportedItem) {
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
	})
}

func (tsk *Task) ChooseAudio(f func(i int, track AudioTrack)) {
	if tsk.UseAudiotrack < 0 {
		for i, track := range tsk.Meta.AudioTracks {
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

func NoSub(s string) string {
	s = strings.ReplaceAll(s, ".closedcaptions", "")
	s = strings.ReplaceAll(s, ".subtitles", "")
	s = strings.ReplaceAll(s, ".dubtitles", "")
	s = strings.ReplaceAll(s, ".dialog", "")
	s = strings.ReplaceAll(s, ".STRIPPED_SDH.subtitles", "")
	s = strings.ReplaceAll(s, ".DUBTITLE.subtitles", "")
	//s = strings.ReplaceAll(s, ".", "")
	return s
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
