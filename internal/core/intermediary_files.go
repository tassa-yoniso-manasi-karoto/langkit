package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
)

// IntermediaryFileManager handles cleanup and recompression of intermediary files
type IntermediaryFileManager struct {
	mode        config.IntermediaryFileMode
	handler     MessageHandler
	deleteTSV   bool // Additional option to delete TSV/CSV files
	
	// Track files created during processing
	audioFiles  []string
	imageFiles  []string
	wavFiles    []string
	otherFiles  []string
}

// NewIntermediaryFileManager creates a new file manager
func NewIntermediaryFileManager(mode config.IntermediaryFileMode, handler MessageHandler, deleteTSV bool) *IntermediaryFileManager {
	return &IntermediaryFileManager{
		mode:        mode,
		handler:     handler,
		deleteTSV:   deleteTSV,
		audioFiles:  make([]string, 0),
		imageFiles:  make([]string, 0),
		wavFiles:    make([]string, 0),
		otherFiles:  make([]string, 0),
	}
}

// RegisterFile registers an intermediary file for later processing
func (ifm *IntermediaryFileManager) RegisterFile(filepath string, fileType string) {
	switch fileType {
	case "audio":
		ifm.audioFiles = append(ifm.audioFiles, filepath)
	case "image":
		ifm.imageFiles = append(ifm.imageFiles, filepath)
	case "wav":
		ifm.wavFiles = append(ifm.wavFiles, filepath)
	default:
		ifm.otherFiles = append(ifm.otherFiles, filepath)
	}
}

// ProcessFiles handles all registered intermediary files according to the configured mode
func (ifm *IntermediaryFileManager) ProcessFiles(tsvFile string) error {
	ifm.handler.ZeroLog().Debug().
		Str("mode", string(ifm.mode)).
		Bool("deleteTSV", ifm.deleteTSV).
		Int("audioFiles", len(ifm.audioFiles)).
		Int("imageFiles", len(ifm.imageFiles)).
		Int("wavFiles", len(ifm.wavFiles)).
		Msg("Processing intermediary files")
	
	switch ifm.mode {
	case config.KeepIntermediaryFiles:
		// Nothing to do, keep all files as-is
		ifm.handler.ZeroLog().Info().Msg("Keeping all intermediary files")
		return nil
		
	case config.RecompressIntermediaryFiles:
		// Recompress audio files to save space
		if err := ifm.recompressAudioFiles(); err != nil {
			return fmt.Errorf("failed to recompress audio files: %w", err)
		}
		// Delete WAV files as they're uncompressed and large
		if err := ifm.deleteFiles(ifm.wavFiles); err != nil {
			return fmt.Errorf("failed to delete WAV files: %w", err)
		}
		
	case config.DeleteIntermediaryFiles:
		// Delete all intermediary files
		allFiles := append(ifm.audioFiles, ifm.imageFiles...)
		allFiles = append(allFiles, ifm.wavFiles...)
		allFiles = append(allFiles, ifm.otherFiles...)
		
		if err := ifm.deleteFiles(allFiles); err != nil {
			return fmt.Errorf("failed to delete intermediary files: %w", err)
		}
	}
	
	// Handle TSV/CSV deletion if requested (independent of mode)
	if ifm.deleteTSV && tsvFile != "" {
		ifm.handler.ZeroLog().Info().Str("file", tsvFile).Msg("Deleting TSV/CSV resumption file")
		if err := os.Remove(tsvFile); err != nil && !os.IsNotExist(err) {
			ifm.handler.ZeroLog().Warn().Err(err).Msg("Failed to delete TSV/CSV file")
		}
	}
	
	return nil
}

// recompressAudioFiles recompresses audio files to save space
func (ifm *IntermediaryFileManager) recompressAudioFiles() error {
	for _, audioFile := range ifm.audioFiles {
		// Skip if file doesn't exist
		if _, err := os.Stat(audioFile); os.IsNotExist(err) {
			continue
		}
		
		// Skip if already compressed (check extension)
		ext := strings.ToLower(filepath.Ext(audioFile))
		if ext == ".opus" || ext == ".mp3" || ext == ".m4a" {
			ifm.handler.ZeroLog().Debug().
				Str("file", audioFile).
				Msg("Skipping recompression - already compressed format")
			continue
		}
		
		// Special handling for separated voice files from expensive APIs
		if strings.Contains(audioFile, ".VOCALS-ONLY.") {
			ifm.handler.ZeroLog().Info().
				Str("file", audioFile).
				Msg("Preserving expensive API-separated voice file - will not recompress")
			continue
		}
		
		// Check if recompressed version already exists
		recompressedFile := strings.TrimSuffix(audioFile, ext) + ".RECOMPRESSED.opus"
		if _, err := os.Stat(recompressedFile); err == nil {
			ifm.handler.ZeroLog().Debug().
				Str("file", recompressedFile).
				Msg("Recompressed file already exists, removing original")
			// Delete original, keep recompressed
			os.Remove(audioFile)
			continue
		}
		
		// Recompress to Opus at lower bitrate
		ifm.handler.ZeroLog().Info().
			Str("file", audioFile).
			Msg("Recompressing audio file to save space")
		
		err := media.FFmpeg(
			"-loglevel", "error", "-y",
			"-i", audioFile,
			"-c:a", "libopus",
			"-b:a", "64k", // Lower bitrate for space saving + enough for voice-only audio
			recompressedFile,
		)
		
		if err != nil {
			ifm.handler.ZeroLog().Warn().
				Err(err).
				Str("file", audioFile).
				Msg("Failed to recompress audio file")
			// Continue with other files even if one fails
			continue
		}
		
		// Delete original after successful recompression
		os.Remove(audioFile)
	}
	
	return nil
}

// deleteFiles deletes the specified files
func (ifm *IntermediaryFileManager) deleteFiles(files []string) error {
	deletedCount := 0
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			if !os.IsNotExist(err) {
				ifm.handler.ZeroLog().Debug().
					Err(err).
					Str("file", file).
					Msg("Failed to delete file")
			}
		} else {
			deletedCount++
		}
	}
	
	ifm.handler.ZeroLog().Info().
		Int("deleted", deletedCount).
		Int("total", len(files)).
		Msg("Deleted intermediary files")
	
	return nil
}

// CleanupMediaDirectory removes the .media directory if empty
func (ifm *IntermediaryFileManager) CleanupMediaDirectory(mediaDir string) {
	if ifm.mode != config.DeleteIntermediaryFiles {
		return
	}
	
	// Check if directory is empty
	entries, err := os.ReadDir(mediaDir)
	if err != nil {
		return
	}
	
	if len(entries) == 0 {
		ifm.handler.ZeroLog().Info().
			Str("dir", mediaDir).
			Msg("Removing empty media directory")
		os.Remove(mediaDir)
	}
}