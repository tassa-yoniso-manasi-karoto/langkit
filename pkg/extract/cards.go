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

	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/rs/zerolog"
	
	"github.com/tassa-yoniso-manasi-karoto/subs2cards/pkg/subs"
	"github.com/tassa-yoniso-manasi-karoto/subs2cards/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/subs2cards/pkg/voice"
	//"github.com/schollz/progressbar/v3"
)

type Task struct {
	Meta                 MediaInfo
	OriginalLang         string
	TargetLang           string
	Separation           string
	TargetChan           int
	UseAudiotrack        int
	Timeout              int
	Offset               time.Duration
	STT, IsCC            bool
	Log                  zerolog.Logger
	ForeignSubtitlesFile string
	NativeSubtitlesFile  string
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
	if strings.Contains(filepath.Dir(tsk.ForeignSubtitlesFile), "'") {
		tsk.Log.Fatal().Msg(
			"Due to ffmpeg limitations, the path of the directory in which the files are located must not contain an apostrophe (')." +
			"Apostrophe in the names of the files themselves are supported using a workaround.",
		)
	}
	base := strings.TrimSuffix(path.Base(tsk.ForeignSubtitlesFile), path.Ext(tsk.ForeignSubtitlesFile))
	return strings.ReplaceAll(base, "'", " ")
}

func (tsk *Task) outputFile() string {
	return path.Join(path.Dir(tsk.ForeignSubtitlesFile), tsk.outputBase()+"."+tsk.OutputFileExtension)
}

func (tsk *Task) mediaOutputDir() string {
	return path.Join(path.Dir(tsk.ForeignSubtitlesFile), tsk.outputBase()+".media")
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

	tsk.setDefaults()
	foreignSubs, err := subs.OpenFile(tsk.ForeignSubtitlesFile, false)
	if err != nil {
		tsk.Log.Fatal().Err(err).Msg("can't read foreign subtitles")
	}
	if tsk.IsCC || strings.Contains(strings.ToLower(tsk.ForeignSubtitlesFile), "closedcaption") {
		foreignSubs = subs.DumbDown2Dubs(foreignSubs)
		tsk.Log.Info().Msg("Foreign subs are closed captions.")
	}

	if tsk.NativeSubtitlesFile != "" {
		nativeSubs, err = subs.OpenFile(tsk.NativeSubtitlesFile, false)
		if err != nil {
			tsk.Log.Fatal().Err(err).Msg("can't read native subtitles")
		}
	}

	outStream, err := os.Create(tsk.outputFile())
	if err != nil {
		tsk.Log.Fatal().Err(err).Msg(fmt.Sprintf("can't create output file: %s", tsk.outputFile()))
	}
	defer outStream.Close()

	if err := os.MkdirAll(tsk.mediaOutputDir(), os.ModePerm); err != nil {
		tsk.Log.Fatal().Err(err).Msg(fmt.Sprintf("can't create output directory: %s", tsk.mediaOutputDir()))
	}
	mediaPrefix := path.Join(tsk.mediaOutputDir(), tsk.outputBase())
	tsk.Meta = mediainfo(tsk.MediaSourceFile)
	for _, track := range tsk.Meta.AudioTracks {
		if strings.Contains(strings.ToLower(track.Title), "original") {
			tsk.OriginalLang = track.Language
		}
	}
	if tsk.OriginalLang != "" {
		tsk.Log.Info().Msg("Detected original lang:"+ tsk.OriginalLang)
	}
	// FIXME range over func instead of calling it 3 times
	tsk.ChooseAudio(func(i int, track AudioTrack) {
		num, _ := strconv.Atoi(track.Channels)
		if track.Language == tsk.TargetLang && num == tsk.TargetChan {
			tsk.UseAudiotrack = i
		}
	})
	tsk.ChooseAudio(func(i int, track AudioTrack) {
		if track.Language == tsk.TargetLang {
			tsk.UseAudiotrack = i
		}
	})
	if tsk.UseAudiotrack < 0 {
		tsk.UseAudiotrack = 0
	}
	tsk.Log.Info().
		Int("UseAudiotrack", tsk.UseAudiotrack).
		Str("track lang", tsk.Meta.AudioTracks[tsk.UseAudiotrack].Language).
		Str("chan num", tsk.Meta.AudioTracks[tsk.UseAudiotrack].Channels)

	if tsk.Separation != "" {
		// CAVEAT: All popular lossy encoder I have tried messed up the timings except Opus,
		// even demuxing with -c:a copy to keep the original encoding somehow did too!
		// Using flac or opus is critical to keep video, audio and sub in sync.
		audiobase := NoSub(tsk.outputBase())
		OriginalAudio := filepath.Join(os.TempDir(), audiobase + ".flac")
		if  _, err := os.Stat(OriginalAudio); errors.Is(err, os.ErrNotExist){
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
		extPerProvider := map[string]string{
			"demucs":     "flac",
			"demucs_ft":  "flac",
			"spleeter":   "wav",
			"elevenlabs": "mp3",
		}
		VoiceFile :=  filepath.Join(tsk.mediaOutputDir(), audiobase + "." +  strings.ToUpper(tsk.Separation) + "." + extPerProvider[tsk.Separation])
		//SyncVoiceFile :=  filepath.Join(tsk.mediaOutputDir(), audiobase + "." +  strings.ToUpper(tsk.Separation) + ".SYNC.wav")
		if  _, err := os.Stat(VoiceFile); errors.Is(err, os.ErrNotExist) {
			tsk.Log.Info().Msg("Separating voice from the rest of the audiotrack...")
			var audio []byte
			switch strings.ToLower(tsk.Separation) {
			case "demucs":
				audio, err = voice.Demucs(OriginalAudio, extPerProvider[tsk.Separation], tsk.Timeout, false)
			case "demucs_ft":
				audio, err = voice.Demucs(OriginalAudio, extPerProvider[tsk.Separation], tsk.Timeout, true)
			case "spleeter":
				audio, err = voice.Spleeter(OriginalAudio, tsk.Timeout)
			case "elevenlabs":
				audio, err = voice.ElevenlabsIsolator(OriginalAudio, tsk.Timeout)
			default:
				tsk.Log.Fatal().Msg("An unknown source separation library was passed. Check for typo.")
			}
			if err != nil {
				tsk.Log.Fatal().Err(err).Msg("Voice separation processing error.")
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
		if strings.ToLower(tsk.Separation) != "elevenlabs" {
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

func placeholder() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}



