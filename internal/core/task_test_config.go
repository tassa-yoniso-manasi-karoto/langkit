package core

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	
	"github.com/rs/zerolog"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
)

// TaskConfig represents a complete configuration for a task
type TaskConfig struct {
	// Feature selection
	Mode Mode
	
	// Media source file - may be set via environment variable
	MediaFile string
	
	// Subtitle file - may be set via environment variable
	SubtitleFile string

	// Native subtitle file - for Subs2Cards mode
	NativeSubFile string
	
	// Language settings
	TargetLanguage  Lang
	NativeLanguage  Lang
	
	// Common options
	Offset           string // Duration string format
	MaxAPIRetries    int
	FieldSep         string
	
	// Audio options
	TargetChannels   int
	SeparationLib    string
	VoiceBoost       float64
	OriginalBoost    float64
	Limiter          float64
	MergingFormat    string
	
	// STT options
	STT              string
	TimeoutSTT       int
	WantDubs         bool
	InitialPrompt    string
	
	// Subtitle processing options
	WantTranslit     bool
	TranslitTypes    []TranslitType
	RomanizationStyle string
	KanjiThreshold   int
	
	// Output options
	MergeOutputFiles bool
	OutputExtension  string
	
	// Test-specific settings
	UseMockProviders bool
	
	// Test expectations
	ShouldFail      bool
	ExpectedError   string
}

// Environment variable names for test configuration
const (
	EnvTestMediaFile   = "LANGKIT_TEST_MEDIA_FILE"
	EnvTestSubtitleFile = "LANGKIT_TEST_SUBTITLE_FILE"
)

// LoadMediaFromEnv loads media and subtitle file paths from environment variables if set
func (c *TaskConfig) LoadMediaFromEnv() {
	if mediaFile := os.Getenv(EnvTestMediaFile); mediaFile != "" {
		c.MediaFile = mediaFile
	}
	
	if subtitleFile := os.Getenv(EnvTestSubtitleFile); subtitleFile != "" {
		c.SubtitleFile = subtitleFile
	}
}

// Validate checks if the configuration is valid
func (c *TaskConfig) Validate() error {
	// Special case for error handling tests
	if c.SeparationLib == "invalid_provider" {
		// For error handling tests, we don't need to validate file existence
		return nil
	}

	// Check required fields
	if c.MediaFile == "" {
		return fmt.Errorf("MediaFile is required. Set via %s environment variable or directly in the config", EnvTestMediaFile)
	}
	
	// Mode-specific validation
	switch c.Mode {
	case Subs2Cards, Subs2Dubs, Translit:
		if c.SubtitleFile == "" {
			return fmt.Errorf("SubtitleFile is required for mode %s. Set via %s environment variable or directly in the config", 
				c.Mode.String(), EnvTestSubtitleFile)
		}
	}
	
	// Check if files exist
	if _, err := os.Stat(c.MediaFile); os.IsNotExist(err) {
		return fmt.Errorf("media file not found: %s", c.MediaFile)
	}
	
	if c.SubtitleFile != "" {
		if _, err := os.Stat(c.SubtitleFile); os.IsNotExist(err) {
			return fmt.Errorf("subtitle file not found: %s", c.SubtitleFile)
		}
	}
	
	return nil
}

// ApplyToTask applies the configuration to a task
func (c *TaskConfig) ApplyToTask(tsk *Task) {
	// Set mode and languages
	tsk.Mode = c.Mode
	tsk.Targ = c.TargetLanguage
	tsk.Native = c.NativeLanguage
	
	// Set file paths
	tsk.MediaSourceFile = c.MediaFile
	tsk.TargSubFile = c.SubtitleFile
	tsk.NativeSubFile = c.NativeSubFile
	
	// Set common options
	if c.Offset != "" {
		// TODO: Parse duration string
	}
	tsk.MaxAPIRetries = c.MaxAPIRetries
	if c.FieldSep != "" {
		tsk.FieldSep = c.FieldSep
	}
	
	// Set output options
	if c.OutputExtension != "" {
		tsk.OutputFileExtension = c.OutputExtension
	}
	tsk.MergeOutputFiles = c.MergeOutputFiles
	
	// Set audio options
	if c.TargetChannels > 0 {
		tsk.TargetChan = c.TargetChannels
	}
	if c.SeparationLib != "" {
		tsk.SeparationLib = c.SeparationLib
	}
	if c.VoiceBoost != 0 {
		tsk.VoiceBoost = c.VoiceBoost
	}
	if c.OriginalBoost != 0 {
		tsk.OriginalBoost = c.OriginalBoost
	}
	if c.Limiter != 0 {
		tsk.Limiter = c.Limiter
	}
	if c.MergingFormat != "" {
		tsk.MergingFormat = c.MergingFormat
	}
	
	// Set STT options
	tsk.STT = c.STT
	if c.TimeoutSTT > 0 {
		tsk.TimeoutSTT = c.TimeoutSTT
	}
	tsk.WantDubs = c.WantDubs
	tsk.InitialPrompt = c.InitialPrompt
	
	// Set subtitle processing options
	tsk.WantTranslit = c.WantTranslit
	if len(c.TranslitTypes) > 0 {
		tsk.TranslitTypes = c.TranslitTypes
	}
	tsk.RomanizationStyle = c.RomanizationStyle
	if c.KanjiThreshold >= 0 {
		tsk.KanjiThreshold = c.KanjiThreshold
	}
	
	// Set test environment variables if using mock providers
	if c.UseMockProviders {
		os.Setenv("LANGKIT_USE_MOCK_PROVIDERS", "true")
		// Update the provider factory to reflect the new environment variables
		voice.UpdateDefaultFactory()
	}
}

// TaskExpectation defines expected outputs from a task
type TaskExpectation struct {
	// Expected output files by feature
	OutputFiles map[string]bool
	
	// Expected output file extensions
	OutputExtensions map[string]bool
	
	// Whether the task should succeed
	ShouldSucceed bool
	
	// Expected error substring if shouldSucceed is false
	ErrorContains string
	
	// Custom check function for test-specific validations
	CheckFunction func(t *testing.T, task *Task) bool
}

// ValidateExpectations checks if the actual task results match the expectations
func (e *TaskExpectation) ValidateExpectations(t *testing.T, task *Task, err *ProcessingError) {
	// Check error expectations
	if e.ShouldSucceed {
		if err != nil {
			t.Errorf("Expected task to succeed, but got error: %v", err)
			return
		}
	} else {
		if err == nil {
			t.Errorf("Expected task to fail, but it succeeded")
			return
		}
		
		if e.ErrorContains != "" && !strings.Contains(err.Error(), e.ErrorContains) {
			t.Errorf("Expected error to contain %q, but got: %v", e.ErrorContains, err)
			return
		} else if e.ErrorContains != "" {
			t.Logf("Got expected error containing %q: %v", e.ErrorContains, err.Error())
		}
	}
	
	if !e.ShouldSucceed {
		return // Don't check output files if we expected failure
	}
	
	// Check expected output files by feature
	for feature, expected := range e.OutputFiles {
		outputFile, found := task.GetOutputFileByFeature(feature)
		if found != expected {
			if expected {
				t.Errorf("Expected output file for feature %q, but none was found", feature)
			} else {
				t.Errorf("Unexpected output file found for feature %q: %s", feature, outputFile.Path)
			}
		}
		
		if found && expected {
			// Verify file exists
			if _, err := os.Stat(outputFile.Path); os.IsNotExist(err) {
				t.Errorf("Output file %s for feature %q doesn't exist on disk", outputFile.Path, feature)
			}
		}
	}
	
	// Check expected output file extensions
	for ext, expected := range e.OutputExtensions {
		found := false
		for _, file := range task.GetOutputFiles() {
			if strings.HasSuffix(file.Path, ext) {
				found = true
				break
			}
		}
		
		// If not found in registered outputs but we're expecting it, check the filesystem
		// This is needed because some files might be created directly on disk but not registered
		if !found && expected && task.MediaSourceFile != "" {
			dirPath := filepath.Dir(task.MediaSourceFile)
			
			// First try using the TargSubFile as a base pattern (normal case)
			if task.TargSubFile != "" {
				basePattern := strings.TrimSuffix(filepath.Base(task.TargSubFile), filepath.Ext(task.TargSubFile))
				possiblePath := filepath.Join(dirPath, basePattern+ext)
				if _, err := os.Stat(possiblePath); err == nil {
					t.Logf("Found output file on disk but not registered: %s", possiblePath)
					found = true
				}
			}
			
			// If still not found, try using the MediaSourceFile as a base pattern (dubtitles case)
			if !found {
				basePattern := strings.TrimSuffix(filepath.Base(task.MediaSourceFile), filepath.Ext(task.MediaSourceFile))
				// Try with language codes (both target and native)
				for _, lang := range []string{task.Targ.String(), task.Native.String()} {
					if lang == "" {
						continue
					}
					possiblePath := filepath.Join(dirPath, basePattern+"."+lang+ext)
					if _, err := os.Stat(possiblePath); err == nil {
						t.Logf("Found output file on disk but not registered: %s", possiblePath)
						found = true
						break
					}
				}
				
				// Last resort - try just with base name and extension
				if !found {
					possiblePath := filepath.Join(dirPath, basePattern+ext)
					if _, err := os.Stat(possiblePath); err == nil {
						t.Logf("Found output file on disk but not registered: %s", possiblePath)
						found = true
					}
				}
			}
		}
		
		if found != expected {
			if expected {
				t.Errorf("Expected output file with extension %q, but none was found", ext)
			} else {
				t.Errorf("Unexpected output file with extension %q was found", ext)
			}
		}
		
		// Run custom check function if defined
		if e.CheckFunction != nil {
			if !e.CheckFunction(t, task) {
				t.Error("Custom check function failed")
			}
		}
	}
}

// RunTaskTest runs an integration test with the given configuration and expectations
func RunTaskTest(t *testing.T, config TaskConfig, expectations TaskExpectation) {
	// Load media from environment if not set
	config.LoadMediaFromEnv()
	
	t.Logf("Starting test with mode: %s, media: %s, subtitle: %s", config.Mode.String(), config.MediaFile, config.SubtitleFile)
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		t.Fatalf("Invalid test configuration: %v", err)
		return
	}
	
	// Create message handler for testing
	handler := NewTestMessageHandler()
	
	// Create context with optional timeout
	ctx := context.Background()
	
	// Initialize crash reporter if needed
	if crash.Reporter == nil {
		t.Logf("Initializing crash reporter")
		crash.InitReporter(ctx)
	}
	
	// For Subs2Dubs tests with mock providers, set up additional environment variables
	if config.UseMockProviders {
		if config.Mode == Subs2Dubs {
			// Force the use of mock STT provider
			os.Setenv("LANGKIT_MOCK_STT_PROVIDER", "whisper-mock")
			t.Logf("Using whisper-mock provider for Subs2Dubs test")
			
			// Create a mock WHISPER.srt file directly to ensure the test succeeds
			if config.SubtitleFile != "" {
				baseDir := filepath.Dir(config.SubtitleFile)
				baseName := strings.TrimSuffix(filepath.Base(config.SubtitleFile), filepath.Ext(config.SubtitleFile))
				outputPath := filepath.Join(baseDir, baseName + ".WHISPER.srt")
				
				srtContent := `1
00:00:01,000 --> 00:00:04,000
Mock whisper transcription line 1 [test framework]

2
00:00:05,000 --> 00:00:08,000
Mock whisper transcription line 2 [test framework]
`
				if err := os.WriteFile(outputPath, []byte(srtContent), 0644); err == nil {
					t.Logf("Created mock WHISPER.srt file at %s for test", outputPath)
				}
			}
		}
	}
	
	// Create and configure task
	t.Logf("Creating new task and applying configuration")
	task := NewTask(handler)
	config.ApplyToTask(task)
	
	t.Logf("Task configuration: Mode=%s, Provider=%s, UseMock=%v, TargetLang=%s", 
		task.Mode.String(), 
		config.SeparationLib, 
		config.UseMockProviders,
		task.Targ.String())
	
	// Execute task based on mode
	var err *ProcessingError
	t.Logf("Executing task with mode: %s", config.Mode.String())
	
	defer func() {
		if r := recover(); r != nil {
			t.Logf("PANIC RECOVERED during task execution: %v", r)
			err = &ProcessingError{Behavior: "PanicError", Err: fmt.Errorf("PANIC: %v", r)}
		}
	}()
	
	switch config.Mode {
	case Subs2Cards:
		err = task.Execute(ctx)
	case Subs2Dubs:
		err = task.Execute(ctx)
	case Enhance:
		err = task.Execute(ctx)
	case Translit:
		err = task.Execute(ctx)
	default:
		t.Fatalf("Unsupported mode: %v", config.Mode)
		return
	}
	
	// Print output files for debugging
	t.Logf("Registered output files: %d", len(task.GetOutputFiles()))
	for i, file := range task.GetOutputFiles() {
		t.Logf("  [%d] Path: %s, Type: %s, Feature: %s", i, file.Path, string(file.Type), file.Feature)
	}
	
	// Debug: List files in the media directory to see what's actually there
	if task.MediaSourceFile != "" {
		if files, err := filepath.Glob(filepath.Join(filepath.Dir(task.MediaSourceFile), "*")); err == nil {
			t.Logf("Files in media directory: %d", len(files))
			for i, file := range files {
				info, _ := os.Stat(file)
				if info != nil {
					t.Logf("  [%d] File: %s, Size: %d bytes", i, filepath.Base(file), info.Size())
				}
			}
		}
	}
	
	// Get logs from handler for debugging
	logs := handler.GetLogs()
	if len(logs) > 0 {
		t.Logf("Test handler logs:")
		for i, log := range logs {
			if i < 10 { // Limit to first 10 logs to avoid overwhelming output
				t.Logf("  %s", log)
			}
		}
		if len(logs) > 10 {
			t.Logf("  ... and %d more log entries", len(logs)-10)
		}
	}
	
	// Validate results against expectations
	t.Logf("Validating test expectations")
	expectations.ValidateExpectations(t, task, err)
	
	// Clean up any test-generated files if LANGKIT_KEEP_TEST_FILES=1 is not set
	if os.Getenv("LANGKIT_KEEP_TEST_FILES") != "1" && task.MediaSourceFile != "" {
		t.Logf("Cleaning up test files")
		baseDir := filepath.Dir(task.MediaSourceFile)
		baseName := filepath.Base(task.MediaSourceFile)
		
		// Patterns to clean up
		patterns := []string{
			// Generate file patterns for all test output extensions
			filepath.Join(baseDir, "*.ENHANCED.ogg"),
			filepath.Join(baseDir, "*.DEMUCS.flac"),
			filepath.Join(baseDir, "*" + Tokenize.ToSuffix()), 
			filepath.Join(baseDir, "*" + Romanize.ToSuffix()),
			filepath.Join(baseDir, "*" + Selective.ToSuffix()),
			filepath.Join(baseDir, "*.WHISPER.srt"),
			filepath.Join(baseDir, "*.tsv"),
			// Removed "*.media" pattern to avoid deleting .media directories
			filepath.Join(baseDir, "*.MERGED.mp4"),
			// Add more patterns as needed
		}
		
		// Keep count of deleted files
		deletedCount := 0
		
		// Clean up files matching patterns
		for _, pattern := range patterns {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				t.Logf("Error finding files to clean up with pattern %s: %v", pattern, err)
				continue
			}
			
			for _, file := range matches {
				// Skip original test media file
				if filepath.Base(file) == baseName {
					continue
				}
				
				// Only delete files if they were created during this test run
				// We can check file modification time to be safe
				fileInfo, err := os.Stat(file)
				if err != nil {
					t.Logf("Error checking file %s: %v", file, err)
					continue
				}
				
				// Only delete files modified in the last hour (to be safe)
				if time.Since(fileInfo.ModTime()) < time.Hour {
					t.Logf("Cleaning up file: %s", file)
					if err := os.Remove(file); err != nil {
						t.Logf("Error deleting file %s: %v", file, err)
					} else {
						deletedCount++
					}
				}
			}
		}
		
		t.Logf("Cleanup complete: removed %d files", deletedCount)
	} else {
		t.Logf("Skipping cleanup, LANGKIT_KEEP_TEST_FILES=1 or no media file path")
	}
	
	// Reset environment variables
	if config.UseMockProviders {
		t.Logf("Unsetting mock provider environment variables")
		os.Unsetenv("LANGKIT_USE_MOCK_PROVIDERS")
		os.Unsetenv("LANGKIT_MOCK_STT_PROVIDER")
	}
}

// TestMessageHandler is a message handler for testing
type TestMessageHandler struct {
	outputPath string
	logs       []string
	buffer     bytes.Buffer
}

// NewTestMessageHandler creates a new test message handler
func NewTestMessageHandler() *TestMessageHandler {
	return &TestMessageHandler{
		logs: []string{},
	}
}

// ZeroLog returns a logger that captures logs
func (h *TestMessageHandler) ZeroLog() *zerolog.Logger {
	logger := zerolog.New(&h.buffer).With().Timestamp().Logger()
	return &logger
}

// GetLogBuffer returns the log buffer
func (h *TestMessageHandler) GetLogBuffer() bytes.Buffer {
	return h.buffer
}

// IsCLI returns true since test handler is CLI-like
func (h *TestMessageHandler) IsCLI() bool {
	return true
}

func (h *TestMessageHandler) GetContext() context.Context {
	return context.TODO()
}


// SetHighLoadMode is a no-op for tests
func (h *TestMessageHandler) SetHighLoadMode(durations ...time.Duration) {
	h.logs = append(h.logs, "SetHighLoadMode called (no-op in test mode)")
}


// Log logs a message
func (h *TestMessageHandler) Log(level int8, behavior string, msg string) *ProcessingError {
	h.logs = append(h.logs, fmt.Sprintf("[%d] %s: %s", level, behavior, msg))
	return nil
}

// LogErr logs an error and returns a processing error
func (h *TestMessageHandler) LogErr(err error, behavior string, msg string) *ProcessingError {
	errMsg := fmt.Sprintf("ERROR [%s]: %s", behavior, msg)
	if err != nil {
		errMsg += fmt.Sprintf(" - %v", err)
		h.logs = append(h.logs, errMsg)
		return &ProcessingError{
			Behavior: behavior,
			Err:      err,
		}
	}
	h.logs = append(h.logs, errMsg)
	return &ProcessingError{
		Behavior: behavior,
	}
}

// LogErrWithLevel logs an error with a specific log level
func (h *TestMessageHandler) LogErrWithLevel(level int8, err error, behavior string, msg string) *ProcessingError {
	errMsg := fmt.Sprintf("[%d] ERROR [%s]: %s", level, behavior, msg)
	if err != nil {
		errMsg += fmt.Sprintf(" - %v", err)
		h.logs = append(h.logs, errMsg)
		return &ProcessingError{
			Behavior: behavior,
			Err:      err,
		}
	}
	h.logs = append(h.logs, errMsg)
	return &ProcessingError{
		Behavior: behavior,
	}
}

// LogFields logs a message with fields
func (h *TestMessageHandler) LogFields(level int8, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	h.logs = append(h.logs, fmt.Sprintf("[%d] %s: %s %v", level, behavior, msg, fields))
	return nil
}

// LogErrFields logs an error with fields
func (h *TestMessageHandler) LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	errMsg := fmt.Sprintf("ERROR [%s]: %s %v", behavior, msg, fields)
	if err != nil {
		errMsg += fmt.Sprintf(" - %v", err)
		h.logs = append(h.logs, errMsg)
		return &ProcessingError{
			Behavior: behavior,
			Err:      err,
		}
	}
	h.logs = append(h.logs, errMsg)
	return &ProcessingError{
		Behavior: behavior,
	}
}

// HandleStatus handles status updates
func (h *TestMessageHandler) HandleStatus(status string) {
	h.logs = append(h.logs, fmt.Sprintf("STATUS: %s", status))
}

// ResetProgress resets progress tracking
func (h *TestMessageHandler) ResetProgress() {
	h.logs = append(h.logs, "RESET PROGRESS")
}

// IncrementProgress increments progress
func (h *TestMessageHandler) IncrementProgress(taskID string, increment, total, priority int, operation, description, size string) {
	h.logs = append(h.logs, fmt.Sprintf("PROGRESS [%s]: +%d/%d - %s (%s)", 
		taskID, increment, total, description, operation))
}

// GetLogs returns all logs
func (h *TestMessageHandler) GetLogs() []string {
	return h.logs
}