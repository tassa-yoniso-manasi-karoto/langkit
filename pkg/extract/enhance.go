package extract

import (
	"fmt"
	"os"
	"strings"
	"path/filepath"
	"errors"

	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	//"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/voice"
)


func (tsk *Task) Enhance() {
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
		err := media.Ffmpeg(
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




func placeholder234567() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}

