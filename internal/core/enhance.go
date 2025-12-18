package core

import (
	"fmt"
	"os"
	"strings"
	"path/filepath"
	"errors"
	"context"

	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/dustin/go-humanize"
	iso "github.com/barbashov/iso639-3"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)



var extPerProvider = map[string]string{
	// Docker-based (local CPU)
	"docker-demucs":              "flac",
	"docker-demucs_ft":           "flac",
	// Docker-based (local GPU/NVIDIA)
	"docker-nvidia-demucs":       "flac",
	"docker-nvidia-demucs_ft":    "flac",
	// Replicate API-based (cloud)
	"replicate-demucs":           "flac",
	"replicate-demucs_ft":        "flac",
	"replicate-spleeter":         "wav",
	// Other
	"elevenlabs":                 "mp3",
}

// CAVEAT: All popular lossy encoder I have tried messed up the timings (except Opus),
// even demuxing with -c:a copy to keep the original encoding somehow did too!
// Using flac or opus is critical to keep video, audio and sub in sync.
func (tsk *Task) enhance(ctx context.Context) (procErr *ProcessingError) {
	reporter := crash.Reporter
	reporter.SaveSnapshot("Starting audio enhancement", tsk.DebugVals()) // necessity: high
	
	if len(tsk.Meta.MediaInfo.AudioTracks) == 0 {
		reporter.SaveSnapshot("No audio tracks found", tsk.DebugVals()) // necessity: high
		return tsk.Handler.LogErr(fmt.Errorf("No audio tracks found"), AbortTask, "No audio tracks found in media file")
	}
	
	// Ensure UseAudiotrack is within bounds
	if tsk.UseAudiotrack < 0 || tsk.UseAudiotrack >= len(tsk.Meta.MediaInfo.AudioTracks) {
		return tsk.Handler.LogErr(
			fmt.Errorf("Audio track index %d out of bounds (tracks: %d)", tsk.UseAudiotrack, len(tsk.Meta.MediaInfo.AudioTracks)),
			AbortTask, "Invalid audio track index")
	}
	
	// Ensure the track has a language
	if tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language == nil {
		// Set a default language if none is specified
		tsk.Handler.ZeroLog().Warn().Msg("Audio track has no language tag, using 'und' (undefined)")
		// Use the FromPart3Code method to get a Language for 'und' (undefined)
		tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language = iso.FromPart3Code("und")
	}
	
	langCode := Str(tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language)
	audioPrefix := filepath.Join(filepath.Dir(tsk.MediaSourceFile), tsk.audioBase()+"."+langCode)
	OriginalAudio := filepath.Join(os.TempDir(), tsk.audioBase() + "." + langCode + ".ORIGINAL.opus")
	VoiceFile := audioPrefix + langkitMadeVocalsOnlyMarker(tsk.SeparationLib) + extPerProvider[tsk.SeparationLib]
	
	// Check if a recompressed version exists (from previous run with recompress mode)
	ext := filepath.Ext(VoiceFile)
	recompressedVoiceFile := strings.TrimSuffix(VoiceFile, ext) + ".RECOMPRESSED.opus"
	if _, err := os.Stat(recompressedVoiceFile); err == nil {
		tsk.Handler.ZeroLog().Info().
			Str("recompressed", recompressedVoiceFile).
			Msg("Using existing recompressed voice file")
		VoiceFile = recompressedVoiceFile
	}
	
	tsk.Handler.ZeroLog().Debug().
		Str("originalAudio", OriginalAudio).
		Str("vocalsFile", VoiceFile).
		Msg("Audio files for enhancement")
		
	reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
		es.SelectedAudioTrack = tsk.UseAudiotrack
		es.AudioTrackLanguage = langCode
		es.SeparationProvider = tsk.SeparationLib
	}) // necessity: high
	
	stat, errOriginal := os.Stat(OriginalAudio)
	_, errVoice := os.Stat(VoiceFile)
	// Already register the voice file with the file manager for optional cleanup to support resumption scenarios
	if tsk.fileManager != nil {
		tsk.fileManager.RegisterFile(VoiceFile, "audio")
	}
	// 			no need to demux if isolate vocalfile exists already
	if errors.Is(errOriginal, os.ErrNotExist) {
		tsk.Handler.ZeroLog().Info().Msg("Demuxing the audiotrack...")
		err := media.FFmpeg(
			[]string{"-loglevel", "error", "-y", "-i", tsk.MediaSourceFile,
					"-map", fmt.Sprint("0:a:", tsk.UseAudiotrack), "-vn",
						"-af", "aresample=resampler=soxr:out_sample_rate=44100", // High-quality resample to 44.1kHz for demucs
						"-acodec", "libopus", "-b:a", media.OpusBitrate, OriginalAudio,
			}...)
		if err != nil {
			return tsk.Handler.LogErr(err, AbortTask, "Failed to demux the desired audiotrack.")
		}
	} else if errOriginal == nil {
		tsk.Handler.ZeroLog().Debug().Msg("Reusing demuxed audiotrack.")
	}
	if stat, errOriginal = os.Stat(OriginalAudio); errOriginal == nil {
		tsk.Handler.ZeroLog().Trace().Str("filesize", humanize.Bytes(uint64(stat.Size()))).Msg("Stat of OriginalAudio to enhance")
	}
	
	if errors.Is(errVoice, os.ErrNotExist) {
		// Get the appropriate provider for audio separation
		provider, err := voice.GetAudioSeparationProvider(tsk.SeparationLib)
		if err != nil {
			return tsk.Handler.LogErr(err, AbortAllTasks, "Failed to get audio separation provider. Check for typo in provider name.")
		}

		// Check if provider is available
		if !provider.IsAvailable() {
			return tsk.Handler.LogErr(nil, AbortTask, fmt.Sprintf("Provider %s is not available. Check API key configuration.", provider.GetName()))
		}

		// Log appropriate message based on provider type
		if strings.HasPrefix(provider.GetName(), "docker-") {
			tsk.Handler.ZeroLog().Info().Msg("Separating vocals from the rest of the audiotrack using local Docker processing...")
		} else {
			tsk.Handler.ZeroLog().Info().Msg("Separating vocals from the rest of the audiotrack: sending request to remote API for processing. Please wait...")
		}
		tsk.Handler.ZeroLog().Debug().Str("provider", provider.GetName()).Msg("Using vocals separation provider")

		// Create a new context with TimeoutDL value for download operations
		// and pass the handler for progress reporting (used by Docker-based providers)
		ctxWithTimeoutDL := context.WithValue(ctx, "TimeoutDL", tsk.TimeoutDL)
		ctxWithHandler := context.WithValue(ctxWithTimeoutDL, voice.ProgressHandlerKey, tsk.Handler)
		ctxWithRecreate := context.WithValue(ctxWithHandler, voice.DockerRecreateKey, tsk.DemucsDockerRecreate)

		audio, err := provider.SeparateVoice(ctxWithRecreate, OriginalAudio, extPerProvider[tsk.SeparationLib], tsk.MaxAPIRetries, tsk.TimeoutSep)
		
		if err != nil {
			reporter.SaveSnapshot("Voice separation failed", tsk.DebugVals()) // necessity: high
			reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
				es.LastErrorOperation = "voice_separation"
				es.LastErrorProvider = provider.GetName()
			}) // necessity: high
		        if errors.Is(err, context.Canceled) {
				return tsk.Handler.LogErrWithLevel(Debug, ctx.Err(), AbortAllTasks, "enhance: operation canceled by user")
		        } else if errors.Is(err, context.DeadlineExceeded) {
				return tsk.Handler.LogErr(err, AbortTask, "enhance: Operation timed out.")
			}
			return tsk.Handler.LogErr(err, AbortAllTasks, "Voice separation processing error.")
		}
		
		// Must write to disk so that it can be reused if ft error
		if err := os.WriteFile(VoiceFile, audio, 0644); err != nil {
			tsk.Handler.ZeroLog().Error().Err(err).Msg("File of separated vocals couldn't be written.")
		}
	} else {
		tsk.Handler.ZeroLog().Info().Msg("Previously separated vocals audio was found and will be reused.")
	}
	// MERGE THE ORIGINAL AUDIOTRACK WITH THE VOICE AUDIO FILE
	// Using a lossless audio file in the video could induce A-V desync will playing
	// because these format aren't designed to be audio tracks of videos, unlike opus.
	MergedFile := audioPrefix + langkitMadeEnhancedMarker() + ".opus"
	// if the user requested a merged video file then this enhanced audio file is in fact an intermediary file, costless to recreate
	settings, err := config.LoadSettings()
	if err == nil && tsk.MergeOutputFiles && settings.IntermediaryFileMode != config.KeepIntermediaryFiles {
		defer func() {
			tsk.Handler.ZeroLog().Trace().Msg("Deleting merged enhanced opus audiofile.")
			os.Remove(MergedFile)
		}()
	}
	if strings.ToLower(tsk.SeparationLib) == "elevenlabs" {
		tsk.Handler.ZeroLog().Info().Msg("No automatic merging possible with Elevenlabs. " +
			"You may synchronize both tracks and merge them using an audio editor (ie. Audacity).")
		return
	}
	tsk.Handler.ZeroLog().Debug().Msg("Merging original and separated vocals track into an enhanced voice track...")
	// Apply positive gain on Voicefile and negative gain on Original, and add a limiter in case
	err = media.FFmpeg(
		[]string{"-loglevel", "error", "-y", "-i", VoiceFile, "-i", OriginalAudio, "-filter_complex",
				fmt.Sprintf("[0:a]volume=%fdB[a1];", tsk.VoiceBoost) +
				fmt.Sprintf("[1:a]volume=%fdB[a2];", tsk.OriginalBoost) +
				"[a1][a2]amix=inputs=2[amixed];" +
				fmt.Sprintf("[amixed]alimiter=limit=%f[final]", tsk.Limiter),
				"-map", "[final]", "-metadata:s:a:0", "language=" + tsk.Targ.String(),
				"-acodec", "libopus", "-b:a", media.OpusBitrate,
				MergedFile,
		}...)
	if err != nil {
		reporter.SaveSnapshot("Audio merging failed", tsk.DebugVals()) // necessity: high
		return tsk.Handler.LogErr(err, AbortTask, "Failed to merge original with separated vocals track.")
	}
	tsk.Handler.ZeroLog().Trace().Msg("Audio merging success.")
	
	// Register the enhanced audio for final output merging if merging is enabled
	if tsk.MergeOutputFiles {
		tsk.RegisterOutputFile(MergedFile, OutputEnhanced, tsk.Targ, "voiceEnhancing", 100)
	}
	
	return nil
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