package cmd

import (
	"fmt"
	"os"
	"strings"
	"path/filepath"
	"errors"

	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	//"github.com/rs/zerolog"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/voice"
)


var enhanceCmd = &cobra.Command{
	Use:   "enhance <mediafile>",
	Short: sepDescr,

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(1)),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			logger.Fatal().Msg("this command requires at least one argument: the path to the media file to be processed")
		}
		tsk := DefaultTask(cmd)
		tsk.MediaSourceFile = args[0]
		
		tsk.SeparationLib = sep
		tsk.TimeoutSep, _ = cmd.Flags().GetInt("sep-to")

		tsk.enhance()
	},
}


// CAVEAT: All popular lossy encoder I have tried messed up the timings (except Opus),
// even demuxing with -c:a copy to keep the original encoding somehow did too!
// Using flac or opus is critical to keep video, audio and sub in sync.
func (tsk *Task) enhance() {
	audiobase := NoSub(tsk.outputBase())
	OriginalAudio := filepath.Join(os.TempDir(), audiobase+".ogg")
	stat, err := os.Stat(OriginalAudio)
	if errors.Is(err, os.ErrNotExist) {
		tsk.Log.Info().Msg("Demuxing the audiotrack...")
		err = media.FFmpeg(
			[]string{"-loglevel", "error", "-y", "-i", tsk.MediaSourceFile,
					"-map", fmt.Sprint("0:a:", tsk.UseAudiotrack), "-vn",
						"-acodec", "libopus", "-b:a", "128k", OriginalAudio,
		}...)
		if err != nil {
			tsk.Log.Fatal().Err(err).Msg("Failed to demux the desired audiotrack.")
		}
	} else {
		tsk.Log.Debug().Msg("Reusing demuxed audiotrack.")
	}
	stat, err = os.Stat(OriginalAudio)
	tsk.Log.Trace().Str("filesize", humanize.Bytes(uint64(stat.Size()))).Msg("Stat of OriginalAudio to enhance")
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
		tsk.Log.Info().Msg("Separating voice from the rest of the audiotrack: sending request to remote API for processing. Please wait...")
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
		tsk.Log.Info().Msg("Previously separated voice audio was found and will be reused.")
	}
	// MERGE THE ORIGINAL AUDIOTRACK WITH THE VOICE AUDIO FILE
	// Using a lossless file here could induce A-V desync will playing
	// because these format aren't designed to be audio tracks of videos, unlike opus.
	if strings.ToLower(tsk.SeparationLib) != "elevenlabs" {
		MergedFile :=  filepath.Join(tsk.mediaOutputDir(), audiobase + ".MERGED.ogg")
		tsk.Log.Debug().Msg("Merging original and separated voice track into an enhanced voice track...")
		// Apply positive gain on Voicefile and negative gain on Original, and add a limiter in case
		err := media.FFmpeg(
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
	} else {
		tsk.Log.Info().Msg("No automatic merging possible for Elevenlabs.")
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


func placeholder234567() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
