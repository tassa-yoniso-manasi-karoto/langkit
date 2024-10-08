package extract

import (
	"fmt"
	"os"
	"path"
	"strings"
	"strconv"
	"time"
	"path/filepath"
	"errors"
	"slices"
	"bufio"

	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/rs/zerolog"
	iso "github.com/barbashov/iso639-3"

	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/subs"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/voice"
	//"github.com/schollz/progressbar/v3"
)

var AstisubSupportedExt = []string{".srt", ".ass", ".ssa", "vtt", ".stl", ".ttml"}

type Task struct {
	Log                  zerolog.Logger
	Meta                 MediaInfo
	OriginalLang         string // FIXME what for?
	Langs                []string
	RefLangs             []Lang
	Targ                 Lang
	SeparationLib        string
	STT                  string
	TargetChan           int
	UseAudiotrack        int
	TimeoutSTT           int
	TimeoutSep           int
	Offset               time.Duration
	IsCCorDubs           bool
	TargSubFile          string
	RefSubFile           string
	MediaSourceFile      string
	OutputFieldSeparator string // defaults to "\t"
	OutputFileExtension  string // defaults to ".tsv" for "\t" and ".csv", otherwise
}

func (tsk *Task) setDefaults() {
	if tsk.OutputFieldSeparator == "" {
		tsk.OutputFieldSeparator = "\t"
	}

	if tsk.OutputFileExtension == "" {
		switch tsk.OutputFieldSeparator {
		case "\t":
			tsk.OutputFileExtension = ".tsv"
		default:
			tsk.OutputFileExtension = ".csv"
		}
	}
}


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


func readStdLangCode(arr []string) (langs []Lang) {
	for _, tmp := range arr {
		var lang Lang
		arr := strings.Split(tmp, "-")
		lang.Language = iso.FromAnyCode(arr[0])
		if len(arr) > 1 {
			lang.Subtag = arr[1]
		}
		langs = append(langs, lang)
	}
	return
}



func (tsk *Task) Execute() {
	if tsk.MediaSourceFile == "" {
		tsk.Log.Fatal().Msg("A media file must be specified.")
	}
	var nativeSubs *subs.Subtitles

	tsk.setDefaults()
	
	if tsk.TargSubFile == "" && tsk.Langs == nil {
		tsk.Log.Fatal().Msg("When no subtitle file is passed desired languages must be specified.")
	}
	if len(tsk.Langs) < 2 {
		tsk.Log.Fatal().Msg("Passed languages are improperly formatted or incomplete.")
	}
	tsk.Targ = readStdLangCode([]string{tsk.Langs[0]})[0]
	tsk.RefLangs = readStdLangCode(tsk.Langs[1:])
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
		// CAVEAT: All popular lossy encoder I have tried messed up the timings except Opus,
		// even demuxing with -c:a copy to keep the original encoding somehow did too!
		// Using flac or opus is critical to keep video, audio and sub in sync.
		audiobase := NoSub(tsk.outputBase())
		OriginalAudio := filepath.Join(os.TempDir(), audiobase+".flac")
		if _, err := os.Stat(OriginalAudio); errors.Is(err, os.ErrNotExist) {
			tsk.Log.Info().Msg("Demuxing the audiotrack...")
			err = media.Ffmpeg(
				[]string{"-loglevel", "error", "-y", "-i", tsk.MediaSourceFile,
						"-map", fmt.Sprint("0:a:", tsk.UseAudiotrack),
							"-vn", OriginalAudio,
			}...)
			if err != nil {
				tsk.Log.Fatal().Err(err).Msg("Failed to demux the desired audiotrack.")
			}
		}
		switch tsk.SeparationLib {
		case "de":
			tsk.SeparationLib = "demucs"
		case "ft":
			tsk.SeparationLib = "demucs_ft"
		case "sp":
			tsk.SeparationLib = "spleeter"
		case "11", "el":
			tsk.SeparationLib = "elevenlabs"
		}
		extPerProvider := map[string]string{
			"demucs":     "flac",
			"demucs_ft":  "flac",
			"spleeter":   "wav",
			"elevenlabs": "mp3",
		}
		VoiceFile :=  filepath.Join(tsk.mediaOutputDir(), audiobase + "." +  strings.ToUpper(tsk.SeparationLib) + "." + extPerProvider[tsk.SeparationLib])
		//SyncVoiceFile :=  filepath.Join(tsk.mediaOutputDir(), audiobase + "." +  strings.ToUpper(tsk.SeparationLib) + ".SYNC.wav")
		if  _, err := os.Stat(VoiceFile); errors.Is(err, os.ErrNotExist) {
			tsk.Log.Info().Msg("Separating voice from the rest of the audiotrack...")
			var audio []byte
			switch strings.ToLower(tsk.SeparationLib) {
			case "demucs":
				audio, err = voice.Demucs(OriginalAudio, extPerProvider[tsk.SeparationLib], 2, tsk.TimeoutSep, false)
			case "demucs_ft":
				audio, err = voice.Demucs(OriginalAudio, extPerProvider[tsk.SeparationLib], 2, tsk.TimeoutSep, true)
			case "spleeter":
				audio, err = voice.Spleeter(OriginalAudio, 2, tsk.TimeoutSep)
			case "elevenlabs":
				audio, err = voice.ElevenlabsIsolator(OriginalAudio, tsk.TimeoutSep)
			default:
				tsk.Log.Fatal().Msg("An unknown separation library was passed. Check for typo.")
			}
			if err != nil {
				tsk.Log.Fatal().Err(err).Msg("Voice SeparationLib processing error.")
			}
			// Must write to disk so that it can be reused if ft error
			if err := os.WriteFile(VoiceFile, audio, 0644); err != nil {
				tsk.Log.Error().Err(err).Msg("File of separated voice couldn't be written.")
			}
		} else {
			tsk.Log.Info().Msg("Previously separated voice audio was found.")
		}
		// MERGE THE ORIGINAL AUDIOTRACK WITH THE VOICE AUDIO FILE
		// Using a lossless file here could induce A-V desync will playing
		// because these format aren't designed to be audio tracks of videos, unlike opus.
		if strings.ToLower(tsk.SeparationLib) != "elevenlabs" {
			MergedFile :=  filepath.Join(tsk.mediaOutputDir(), audiobase + ".MERGED.ogg")
			tsk.Log.Info().Msg("Merging original and separated voice track into an enhanced voice track...")
			// Apply positive gain on Voicefile and negative gain on Original, and add a limiter in case
			err = media.Ffmpeg(
				[]string{"-loglevel", "error", "-y", "-i", VoiceFile, "-i", OriginalAudio, "-filter_complex",
						fmt.Sprintf("[0:a]volume=%ddB[a1];", 13) +
						fmt.Sprintf("[1:a]volume=%ddB[a2];", -9) +
						"[a1][a2]amix=inputs=2[amixed];" +
						fmt.Sprintf("[amixed]alimiter=limit=%f[final]", 0.9),
						"-map", "[final]", "-acodec", "libopus", "-b:a", "128k",
						MergedFile,
			}...)
			if err != nil {
				tsk.Log.Fatal().Err(err).Msg("Failed to merge original with separated voice track.")
			}
		}
	}
	if strings.Contains(strings.ToLower(tsk.TargSubFile), "closedcaption") {
		foreignSubs = subs.DumbDown2Dubs(foreignSubs)
		tsk.Log.Info().Msg("Foreign subs are closed captions.")
	}
	// NOTE: this warning won't occur if the sub file are passed as arg
	if tsk.IsCCorDubs && tsk.STT != "" {
		tsk.Log.Warn().Msg("Speech-to-Text is requested but closed captions or dubtitles are available for the target language," +
			" which are usually reliable transcriptions of dubbings.")
		if !askForConfirmation() {
			os.Exit(0)
		}	
	}
	tsk.ExportItems(outStream, foreignSubs, nativeSubs, tsk.outputBase(), tsk.MediaSourceFile, mediaPrefix, func(item *ExportedItem) {
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


func askForConfirmation() bool {
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
