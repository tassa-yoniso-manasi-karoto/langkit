package metadata

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2" // Official import path for the ID3 library
	"github.com/rs/zerolog"  // Assuming a logger is available, e.g., from llms pkg
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executil"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms" // For Logger, adjust if logger is elsewhere
)

// Logger instance - assuming it's initialized similarly to how it's done in pkg/llms
// If you have a central logging setup, use that.
// For now, this will use the one from pkg/llms, assuming it's exported or accessible.
var logger zerolog.Logger
var FFmpegPath string

func init() {
	// A basic initialization for the logger.
	// In a real app, this would be part of a broader logging setup.
	// If llms.Logger is already initialized and exported, this could use that.
	// For now, creating a local one or assuming llms.Logger is available.
	if llms.Logger.GetLevel() == zerolog.Disabled { // Check if llms.Logger was initialized
		logger = zerolog.New(os.Stderr).With().Timestamp().Logger() // Basic fallback
	} else {
		logger = llms.Logger.With().Str("component", "media_metadata").Logger()
	}
}

// AddLyricsToAudioFile adds the provided text (summary/lyrics) to the specified audio file's metadata.
// It detects the file type (MP3 or M4A/AAC) and uses the appropriate tagging method.
// summaryLangISO639_2 should be the 3-letter ISO 639-2 code (e.g., "eng", "fra") for the language of the lyricsText.
// This is primarily used for the USLT frame in MP3 files.
func AddLyricsToAudioFile(filePath, lyricsText, summaryLangISO639_2 string) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	logger.Debug().Str("file", filePath).Str("extension", ext).Msg("Attempting to add lyrics/summary metadata")

	switch ext {
	case ".mp3":
		return addLyricsToMP3(filePath, lyricsText, summaryLangISO639_2)
	case ".m4a", ".aac", ".mp4": // .mp4 can also be an audio-only M4A container
		return addLyricsToM4A(filePath, lyricsText)
	default:
		logger.Warn().Str("file", filePath).Msg("Unsupported file type for adding lyrics metadata")
		return fmt.Errorf("unsupported file type for lyrics metadata: %s", ext)
	}
}

// addLyricsToMP3 uses the n10v/id3v2 library to add lyrics to an MP3 file.
func addLyricsToMP3(filePath, lyricsText, summaryLangISO639_2 string) error {
	logger.Debug().Str("file", filePath).Msg("Adding lyrics to MP3 using n10v/id3v2")

	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		// If opening fails, it might be because the file doesn't exist or isn't a valid MP3.
		// id3v2.Open creates a new tag if one isn't found, so an error here is likely serious.
		logger.Error().Err(err).Str("file", filePath).Msg("Error opening MP3 file or parsing existing tag")
		return fmt.Errorf("error opening/parsing MP3 tag for %s: %w", filePath, err)
	}
	defer tag.Close()

	// Ensure ID3v2.4 and UTF-8 encoding for new frames.
	// The library defaults to v2.4 for new tags, but being explicit is good.
	tag.SetVersion(4)
	tag.SetDefaultEncoding(id3v2.EncodingUTF8) // Ensures SetTitle etc. use UTF-8

	// Remove any existing USLT (unsynchronised lyrics) frames to avoid duplicates.
	// CommonID gets the frame ID (e.g., "USLT") from a descriptive name.
	usltFrameID := tag.CommonID("Unsynchronised lyrics/text transcription") // Should be "USLT"
	if tag.GetFrames(usltFrameID) != nil { // Check if any USLT frames exist
	logger.Debug().Str("file", filePath).Msg("Removing existing USLT frames before adding new summary.")
	tag.DeleteFrames(usltFrameID)
	}

	langCodeForUSLT := strings.ToLower(summaryLangISO639_2)
	if len(langCodeForUSLT) != 3 {
	logger.Warn().Str("provided_lang_code", summaryLangISO639_2).Str("file", filePath).Msg("Invalid ISO 639-2 language code for USLT frame; defaulting to 'und'")
	langCodeForUSLT = "und"
	}

	uslf := id3v2.UnsynchronisedLyricsFrame{
	Encoding:          id3v2.EncodingUTF8,
	Language:          langCodeForUSLT,
	ContentDescriptor: "", // Use empty string for maximum compatibility as default lyrics/text
	Lyrics:            lyricsText,
	}
	tag.AddUnsynchronisedLyricsFrame(uslf)
	logger.Debug().Str("file", filePath).Str("lang", uslf.Language).Msg("Added USLT frame with empty content descriptor.")

	// Save the changes to the file.
	if err = tag.Save(); err != nil {
		logger.Error().Err(err).Str("file", filePath).Msg("Error saving ID3 tag")
		return fmt.Errorf("error saving ID3 tag to %s: %w", filePath, err)
	}

	logger.Info().Str("file", filePath).Msg("Successfully added lyrics/summary to MP3 metadata")
	return nil
}

// addLyricsToM4A uses FFmpeg to add lyrics to an M4A or AAC file.
func addLyricsToM4A(filePath, lyricsText string) error {
	logger.Debug().Str("file", filePath).Msg("Adding lyrics to M4A/AAC using FFmpeg")

	// Keep the same extension as the original file for the temp file
	ext := filepath.Ext(filePath)
	tempOutFile := strings.TrimSuffix(filePath, ext) + "_tmp" + ext

	// Construct FFmpeg arguments using direct metadata approach
	// Use both ©lyr (standard) and lyrics for maximum compatibility
	args := []string{
		"-i", filePath,                           // Original input file
		"-c", "copy",                             // Copy all streams without re-encoding
		"-metadata", "©lyr=" + lyricsText,       // Standard M4A lyrics atom
		"-metadata", "lyrics=" + lyricsText,     // Alternative for compatibility
		"-brand", "mp42",                         // Use mp42 brand for better compatibility
		"-movflags", "+faststart",                // Optimize for streaming
		"-y",                                     // Overwrite output file if it exists
		tempOutFile,                              // Output to the temporary file
	}

	if err := runFFmpegCommand(args...); err != nil {
		_ = os.Remove(tempOutFile)
		return fmt.Errorf("ffmpeg command failed for M4A metadata: %w", err)
	}

	// Atomically replace the original file with the new one.
	if err := os.Rename(tempOutFile, filePath); err != nil {
		logger.Error().Err(err).Str("source", tempOutFile).Str("dest", filePath).Msg("Failed to replace original file with metadata-updated file")
		return fmt.Errorf("failed to replace original file %s with updated file %s: %w", filePath, tempOutFile, err)
	}

	logger.Info().Str("file", filePath).Msg("Successfully added lyrics/summary to M4A/AAC metadata")
	return nil
}

// runFFmpegCommand executes an FFmpeg command.
func runFFmpegCommand(args ...string) error {
	// Prepend -loglevel error to reduce verbosity, unless already specified.
	hasLogLevel := false
	for _, arg := range args {
		if arg == "-loglevel" {
			hasLogLevel = true
			break
		}
	}
	finalArgs := []string{}
	if !hasLogLevel {
		finalArgs = append(finalArgs, "-loglevel", "error")
	}
	finalArgs = append(finalArgs, args...)
	finalArgs = append(finalArgs, "-hide_banner") // Suppress version banner

	logger.Debug().Strs("ffmpeg_args", finalArgs).Msg("Executing FFmpeg command")

	cmd := executil.NewCommand(FFmpegPath, finalArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr // Capture stderr for error reporting

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = "no stderr output"
		}
		logger.Error().Err(err).Str("ffmpeg_stderr", errMsg).Strs("args", finalArgs).Msg("FFmpeg command execution failed")
		return fmt.Errorf("ffmpeg command %v failed: %w (stderr: %s)", args, err, errMsg)
	}
	return nil
}

