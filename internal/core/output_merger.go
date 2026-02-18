package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"errors"
	"sort"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
)

// OutputRegistry defines an interface for tracking output files
type OutputRegistry interface {
	// RegisterOutputFile registers a file to be included in the merged output
	RegisterOutputFile(path string, typ MediaOutputType, language Lang, feature string, priority int) MediaOutputFile
	
	// GetOutputFiles returns all registered output files
	GetOutputFiles() []MediaOutputFile
	
	// GetOutputFilesByType returns all registered output files of a specific type
	GetOutputFilesByType(typ MediaOutputType) []MediaOutputFile
	
	// GetOutputFileByFeature returns the first output file for a specific feature
	GetOutputFileByFeature(feature string) (MediaOutputFile, bool)
	
	// GetMergedOutputPath returns the path to the merged output file
	GetMergedOutputPath() string
}

// RegisterOutputFile adds a file to the list of files to be included in the merged output
func (tsk *Task) RegisterOutputFile(path string, typ MediaOutputType, language Lang, feature string, priority int) MediaOutputFile {
	outputFile := MediaOutputFile{
		Path:        path,
		Type:        typ,
		Lang:        language,
		IsGenerated: true,
		Feature:     feature,
		Priority:    priority,
	}
	
	tsk.OutputFiles = append(tsk.OutputFiles, outputFile)
	tsk.Handler.ZeroLog().Debug().
		Str("path", path).
		Str("type", string(typ)).
		Str("lang", language.String()).
		Str("feature", feature).
		Int("priority", priority).
		Msg("Registered output file for merging")
		
	return outputFile
}

// GetOutputFiles returns all registered output files
func (tsk *Task) GetOutputFiles() []MediaOutputFile {
	return tsk.OutputFiles
}

// GetOutputFilesByType returns all registered output files of a specific type
func (tsk *Task) GetOutputFilesByType(typ MediaOutputType) []MediaOutputFile {
	var files []MediaOutputFile
	for _, file := range tsk.OutputFiles {
		if file.Type == typ {
			files = append(files, file)
		}
	}
	
	// Sort by priority (higher first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Priority > files[j].Priority
	})
	
	return files
}

// GetOutputFileByFeature returns the first output file for a specific feature
func (tsk *Task) GetOutputFileByFeature(feature string) (MediaOutputFile, bool) {
	for _, file := range tsk.OutputFiles {
		if file.Feature == feature {
			return file, true
		}
	}
	return MediaOutputFile{}, false
}

func (tsk *Task) GetMergedOutputPath() string {
	langCode := Str(tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language.Language)
	basePrefix := filepath.Join(filepath.Dir(tsk.MediaSourceFile), tsk.audioBase()+"."+langCode)
	ext := tsk.MergingFormat
	if ext == "" {
		ext = "mp4" // Default format
	}
	return basePrefix + ".MERGED." + ext
}

// MergeResult contains information about the merge operation
type MergeResult struct {
	OutputPath     string                            // Path to the merged output file
	Command        []string                          // FFmpeg command used for merging
	InputFilesByType map[MediaOutputType][]MediaOutputFile // Input files by type
	Success        bool                              // Whether the merge was successful
	Error          error                             // Error if the merge failed
	Skipped        bool                              // Whether the merge was skipped
	SkipReason     string                            // Reason for skipping the merge
}

// MergeOutputs combines all registered output files into a single video file
func (tsk *Task) MergeOutputs(ctx context.Context) (*MergeResult, *ProcessingError) {
	result := &MergeResult{
		OutputPath: tsk.GetMergedOutputPath(),
		Success:    false,
		Skipped:    false,
	}
	
	// Check if merging is disabled or no files to merge
	if !tsk.MergeOutputFiles || len(tsk.OutputFiles) == 0 {
		reason := "No files to merge"
		if !tsk.MergeOutputFiles {
			reason = "Merging disabled"
		}
		
		tsk.Handler.ZeroLog().Debug().Msg("Output merging skipped - " + reason)
		result.Skipped = true
		result.SkipReason = reason
		return result, nil
	}

	// Check if output already exists
	if _, err := os.Stat(result.OutputPath); err == nil {
		tsk.Handler.ZeroLog().Info().Str("path", result.OutputPath).Msg("Merged output file already exists, skipping merge")
		result.Skipped = true
		result.SkipReason = "Output file already exists"
		return result, nil
	}

	tsk.Handler.ZeroLog().Info().Msg("Merging all output files...")
	
	// Prepare files for merging
	filesToMerge, err := tsk.prepareFilesForMerging()
	if err != nil {
		result.Error = err
		return result, tsk.Handler.LogErr(err, AbortTask, "Failed to prepare files for merging")
	}
	
	result.InputFilesByType = filesToMerge

	// Build merge command
	mergeCmd := tsk.buildMergeCommand(filesToMerge, result.OutputPath, tsk.MergingFormat)
	result.Command = mergeCmd
	
	// Execute merge
	err = media.FFmpeg(mergeCmd...)
	if err != nil {
		result.Error = err
		return result, tsk.Handler.LogErr(err, AbortTask, "Failed to merge output files")
	}

	result.Success = true
	tsk.Handler.ZeroLog().Info().Str("output", result.OutputPath).Msg("Successfully merged all output files")
	
	// Register the merged output file
	tsk.RegisterOutputFile(result.OutputPath, "merged", tsk.Targ, "merging", 100)
	
	return result, nil
}

// prepareFilesForMerging organizes and filters the files to be merged
func (tsk *Task) prepareFilesForMerging() (map[MediaOutputType][]MediaOutputFile, error) {
	if len(tsk.OutputFiles) == 0 {
		return nil, errors.New("no output files to merge")
	}

	// Organize files by type
	filesByType := make(map[MediaOutputType][]MediaOutputFile)
	// lang is set by Mediainfo() no matter what, even if just "und"
	lang := tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language

	// Always include original video as the base
	videoFile := MediaOutputFile{
		Path:        tsk.MediaSourceFile,
		Type:        OutputVideo,
		Lang:        lang,
		IsGenerated: false,
		Feature:     "original",
		Priority:    0,
	}
	
	filesByType[OutputVideo] = []MediaOutputFile{videoFile}
	
	// Add all registered files to their appropriate type groups
	for _, file := range tsk.OutputFiles {
		if _, err := os.Stat(file.Path); os.IsNotExist(err) {
			tsk.Handler.ZeroLog().Warn().
				Str("path", file.Path).
				Str("type", string(file.Type)).
				Msg("File registered for merging doesn't exist, skipping")
			continue
		}
		
		filesByType[file.Type] = append(filesByType[file.Type], file)
	}
	
	// Sort each type group by priority
	for typ, files := range filesByType {
		sort.Slice(files, func(i, j int) bool {
			return files[i].Priority > files[j].Priority // Higher priority first
		})
		filesByType[typ] = files
	}
	
	return filesByType, nil
}

// buildMergeCommand constructs the FFmpeg command for merging all files
func (tsk *Task) buildMergeCommand(filesByType map[MediaOutputType][]MediaOutputFile, outputPath, format string) []string {
	var subfmt string
	switch format {
	case "mp4":
		subfmt = "mov_text"
	case "mkv":
		subfmt = "ass"
	case "webm":
		subfmt = "webvtt"
	}

	// Create a minimalist command focusing on getting language tags to work
	cmd := []string{"-loglevel", "error", "-y"}
	
	// Get original video file
	originalVideoPath := ""
	if videoFiles, ok := filesByType[OutputVideo]; ok && len(videoFiles) > 0 {
		originalVideoPath = videoFiles[0].Path
		cmd = append(cmd, "-i", originalVideoPath)
	}
	
	// Collect subtitle files in priority order
	subtitleFiles := []MediaOutputFile{}
	subtitleTypes := []MediaOutputType{
		OutputDubtitle,
		OutputRomanized,
		OutputTranslit,
		OutputTokenized,
		OutputSubtitle,
	}
	
	for _, subType := range subtitleTypes {
		if subFiles, ok := filesByType[subType]; ok {
			subtitleFiles = append(subtitleFiles, subFiles...)
		}
	}
	
	// Add subtitle files to command
	for _, subtitle := range subtitleFiles {
		cmd = append(cmd, "-i", subtitle.Path)
	}
	
	// Add enhanced audio if available
	enhancedAudioPath := ""
	if audioFiles, ok := filesByType[OutputEnhanced]; ok && len(audioFiles) > 0 {
		enhancedAudioPath = audioFiles[0].Path
		cmd = append(cmd, "-i", enhancedAudioPath)
	}
	
	// Add mapping options - start with video
	cmd = append(cmd, "-map", "0:v")
	
	// Map audio streams
	audioTracks := tsk.Meta.MediaInfo.AudioTracks
	for i := range audioTracks {
		cmd = append(cmd, "-map", fmt.Sprintf("0:a:%d", i))
	}
	
	// Map enhanced audio if available
	if enhancedAudioPath != "" {
		enhancedIndex := len(subtitleFiles) + 1
		cmd = append(cmd, "-map", fmt.Sprintf("%d:a:0", enhancedIndex))
	}
	
	// Map subtitle streams
	for i := range subtitleFiles {
		cmd = append(cmd, "-map", fmt.Sprintf("%d:s", i+1))
	}
	
	// Set codec options
	cmd = append(cmd, "-c:v", "copy", "-c:a", "copy", "-c:s", subfmt)
	
	// Explicitly set language metadata for audio tracks first
	for i, track := range audioTracks {
		if track.Language.Language != nil {
			langCode := Str(track.Language.Language)
			cmd = append(cmd, 
				"-metadata:s:a:"+fmt.Sprint(i),
				"language="+langCode,
			)
		}
	}
	
	// Set enhanced audio language if available
	if enhancedAudioPath != "" {
		audioIndex := len(audioTracks)
		cmd = append(cmd, 
			"-metadata:s:a:"+fmt.Sprint(audioIndex),
			"language="+tsk.Targ.String(),
		)
	}
	
	// Set subtitle languages
	for i, subtitle := range subtitleFiles {
		cmd = append(cmd, 
			"-metadata:s:s:"+fmt.Sprint(i),
			"language="+subtitle.Lang.String(),
		)
	}
	
	// Set disposition flags last
	// Default audio track
	for i := range audioTracks {
		disposition := "none"
		if i == tsk.UseAudiotrack && enhancedAudioPath == "" {
			disposition = "default"
		}
		cmd = append(cmd, "-disposition:a:"+fmt.Sprint(i), disposition)
	}
	
	// Enhanced audio disposition
	if enhancedAudioPath != "" {
		cmd = append(cmd, 
			"-disposition:a:"+fmt.Sprint(len(audioTracks)),
			"default",
		)
	}
	
	// Subtitle dispositions
	for i, subtitle := range subtitleFiles {
		disposition := "none"
		if i == 0 && subtitle.Lang.String() == tsk.Targ.String() {
			disposition = "default"
		}
		cmd = append(cmd, "-disposition:s:"+fmt.Sprint(i), disposition)
	}
	
	// Output file
	cmd = append(cmd, outputPath)
	
	tsk.Handler.ZeroLog().Debug().Strs("mergeCommand", cmd).Msg("Built merge command")
	return cmd
}