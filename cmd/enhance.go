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

		tsk.Mode = Enhance
		tsk.routing()
	},
}


var extPerProvider = map[string]string{
	"demucs":     "flac",
	"demucs_ft":  "flac",
	"spleeter":   "wav",
	"elevenlabs": "mp3",
}

// CAVEAT: All popular lossy encoder I have tried messed up the timings (except Opus),
// even demuxing with -c:a copy to keep the original encoding somehow did too!
// Using flac or opus is critical to keep video, audio and sub in sync.
func (tsk *Task) enhance() {
	langCode := Str(tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language)
	audioPrefix := filepath.Join(filepath.Dir(tsk.MediaSourceFile), tsk.audioBase()+"."+langCode)
	OriginalAudio := filepath.Join(os.TempDir(), tsk.audioBase() + "." + langCode + ".ORIGINAL.ogg")
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
	VoiceFile := audioPrefix + "." +  strings.ToUpper(tsk.SeparationLib) + "." + extPerProvider[tsk.SeparationLib]
	tsk.Log.Trace().Str("VoiceFile", VoiceFile).Msg("")
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
	// Using a lossless audio file in the video could induce A-V desync will playing
	// because these format aren't designed to be audio tracks of videos, unlike opus.
	if strings.ToLower(tsk.SeparationLib) != "elevenlabs" {
		MergedFile := audioPrefix + ".ENHANCED.ogg"
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
		} else {
			tsk.Log.Trace().Msg("Audio merging success.")
		}
		// CAVEAT: on my machine, HW decoder (VAAPI) doesn't accept Matrovska with Opus audio
		// and webm accepts only VP8/VP9/AV1 so must use mp4 by default
		MergedVideo := audioPrefix + ".MERGED."
		tsk.Log.Debug().Msg("Merging newly created audiotrack with the video...")
		c := tsk.buildVideoMergingCmd(MergedFile, MergedVideo, "mkv")
		err = media.FFmpeg(c...)
		if err != nil {
			tsk.Log.Fatal().Err(err).Msg("Failed to merge video with merged audiotrack.")
		} else {
			tsk.Log.Trace().Msg("Video merging success.")
		}
	} else {
		tsk.Log.Info().Msg("No automatic merging possible with Elevenlabs. " +
			"You may synchronize both tracks and merge them using an audio editor (ie. Audacity).")
	}
}

func (tsk *Task) buildVideoMergingCmd(MergedFile, MergedVideo, ext string) []string {
	var subfmt string
	switch ext {
		case "mp4":
			subfmt = "mov_text"
		case "mkv":
			subfmt = "ass"
		case "webm":
			subfmt = "webvtt"
	}
	// Start with base command
	c := []string{"-loglevel", "error", "-y"}
	
	// Collect input files and their corresponding maps
	inputs := []string{tsk.MediaSourceFile, MergedFile}
	maps := []string{
		"-map", "0:v",	// video from first input
		"-map", "1:a",	// audio from second input
		"-map", "0:a?",   // optional audio from first input
	}

	// Add metadata for the merged audio track (assuming it's the first audio track)
	metadata := []string{
		"-metadata:s:a:0", "language=" + tsk.Targ.String(),
	}

	// Add subtitle files if they exist
	subFiles := []struct {
		path string
		lang string
	}{
		{tsk.TargSubFile, tsk.Targ.String()},
		{tsk.NativeSubFile, tsk.Native.String()},
	}

	subIndex := 0
	for _, sub := range subFiles {
		if sub.path != "" {
			inputs = append(inputs, sub.path)
			maps = append(maps, "-map", fmt.Sprintf("%d:s", len(inputs)-1))
			metadata = append(metadata, 
				fmt.Sprintf("-metadata:s:s:%d", subIndex), 
				fmt.Sprintf("language=%s", sub.lang),
			)
			subIndex++
		}
	}

	// Add all input files
	for _, input := range inputs {
		c = append(c, "-i", input)
	}

	// Add all maps
	c = append(c, maps...)

	// Add all metadata
	c = append(c, metadata...)

	// Add the rest of the parameters
	c = append(c, []string{
		"-c:v", "copy",
		"-c:a", "copy",
		"-c:s", subfmt,
		"-disposition:a:0", "default",
		"-disposition:a:1", "none",
		MergedVideo + ext,
	}...)
	tsk.Log.Trace().Strs("mergeVideoCmd", c).Msg("")
	return c
}



func placeholder234567() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
