package core

import (
	"fmt"
	"os"
	"strings"
	"path/filepath"
	"errors"

	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/dustin/go-humanize"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
)



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
	VoiceFile := audioPrefix + "." +  strings.ToUpper(tsk.SeparationLib) + "." + extPerProvider[tsk.SeparationLib]
	
	stat, errOriginal := os.Stat(OriginalAudio)
	_, errVoice := os.Stat(VoiceFile)
	// 				no need to demux if isolate voicefile exists already
	if errors.Is(errOriginal, os.ErrNotExist) && errors.Is(errVoice, os.ErrNotExist) {
		tsk.Handler.ZeroLog().Info().Msg("Demuxing the audiotrack...")
		err := media.FFmpeg(
			[]string{"-loglevel", "error", "-y", "-i", tsk.MediaSourceFile,
					"-map", fmt.Sprint("0:a:", tsk.UseAudiotrack), "-vn",
						"-acodec", "libopus", "-b:a", "128k", OriginalAudio,
		}...)
		if err != nil {
			tsk.Handler.ZeroLog().Fatal().Err(err).Msg("Failed to demux the desired audiotrack.")
		}
	} else if errOriginal == nil {
		tsk.Handler.ZeroLog().Debug().Msg("Reusing demuxed audiotrack.")
	}
	if stat, errOriginal = os.Stat(OriginalAudio); errOriginal == nil {
		tsk.Handler.ZeroLog().Trace().Str("filesize", humanize.Bytes(uint64(stat.Size()))).Msg("Stat of OriginalAudio to enhance")
	}
	tsk.Handler.ZeroLog().Trace().Str("VoiceFile", VoiceFile).Msg("")
	
	if errors.Is(errVoice, os.ErrNotExist) {
		tsk.Handler.ZeroLog().Info().Msg("Separating voice from the rest of the audiotrack: sending request to remote API for processing. Please wait...")
		var audio []byte
		var err error
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
			tsk.Handler.ZeroLog().Fatal().Msg("An unknown separation library was passed. Check for typo.")
		}
		if err != nil {
			tsk.Handler.ZeroLog().Fatal().Err(err).Msg("Voice SeparationLib processing error.\n\n" +
				"LANGKIT DEVELOPER NOTE: These voice separation libraries are originally meant" +
				"for songs (ie. tracks a few minutes long) and the GPUs allocated by Replicate" +
				"to these models are not the best. You may face OOM (out of memory) GPU errors" +
				"when trying to process audio tracks of movies.\n" +
				"As far as my testing goes, trying a few hours later solves the problem.\n")
		}
		// Must write to disk so that it can be reused if ft error
		if err := os.WriteFile(VoiceFile, audio, 0644); err != nil {
			tsk.Handler.ZeroLog().Error().Err(err).Msg("File of separated voice couldn't be written.")
		}
	} else {
		tsk.Handler.ZeroLog().Info().Msg("Previously separated voice audio was found and will be reused.")
	}
	// MERGE THE ORIGINAL AUDIOTRACK WITH THE VOICE AUDIO FILE
	// Using a lossless audio file in the video could induce A-V desync will playing
	// because these format aren't designed to be audio tracks of videos, unlike opus.
	MergedFile := audioPrefix + ".ENHANCED.ogg"
	_, err := os.Stat(MergedFile)
	if strings.ToLower(tsk.SeparationLib) != "elevenlabs" {
		tsk.Handler.ZeroLog().Info().Msg("No automatic merging possible with Elevenlabs. " +
			"You may synchronize both tracks and merge them using an audio editor (ie. Audacity).")
		return
	}
	if err == nil {
		tsk.Handler.ZeroLog().Debug().Msg("Merging original and separated voice track into an enhanced voice track...")
		// Apply positive gain on Voicefile and negative gain on Original, and add a limiter in case
		err := media.FFmpeg(
			[]string{"-loglevel", "error", "-y", "-i", VoiceFile, "-i", OriginalAudio, "-filter_complex",
					fmt.Sprintf("[0:a]volume=%ddB[a1];", 13) +
					fmt.Sprintf("[1:a]volume=%ddB[a2];", -9) +
					"[a1][a2]amix=inputs=2[amixed];" +
					fmt.Sprintf("[amixed]alimiter=limit=%f[final]", 0.9),
					"-map", "[final]", "-metadata:s:a:0", "language=" + tsk.Targ.String(),
					"-acodec", "libopus", "-b:a", "128k",
					MergedFile,
		}...)
		if err != nil {
			tsk.Handler.ZeroLog().Fatal().Err(err).Msg("Failed to merge original with separated voice track.")
		} else {
			tsk.Handler.ZeroLog().Trace().Msg("Audio merging success.")
		}
	}
	// CAVEAT: on my machine, HW decoder (VAAPI) doesn't accept Matrovska with Opus audio
	// and webm accepts only VP8/VP9/AV1 so must use mp4 by default // FIXME add flag to choose video fmt
	ext := "mp4"
	MergedVideo := audioPrefix + ".MERGED." + ext
	if _, err = os.Stat(MergedVideo); errors.Is(err, os.ErrNotExist) {
		tsk.Handler.ZeroLog().Debug().Msg("Merging newly created audiotrack with the video...")
		c := tsk.buildVideoMergingCmd(MergedFile, MergedVideo, ext)
		err = media.FFmpeg(c...)
		if err != nil {
			tsk.Handler.ZeroLog().Fatal().Err(err).Msg("Failed to merge video with merged audiotrack.")
		} else {
			tsk.Handler.ZeroLog().Trace().Msg("Video merging success.")
		}
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
	tsk.Handler.ZeroLog().Trace().Strs("mergeVideoCmd", c).Msg("")
	return c
}



func placeholder234567() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
