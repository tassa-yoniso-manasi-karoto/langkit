package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	iso "github.com/barbashov/iso639-3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Simple mock for Handler to use in tests
type MockHandler struct {
	logger zerolog.Logger
	buffer bytes.Buffer
}

func NewMockHandler() *MockHandler {
	var buffer bytes.Buffer
	logger := zerolog.New(&buffer).With().Timestamp().Logger()
	return &MockHandler{
		logger: logger,
		buffer: buffer,
	}
}

func (m *MockHandler) ZeroLog() *zerolog.Logger {
	return &m.logger
}

func (m *MockHandler) LogErr(err error, behavior string, msg string) *ProcessingError {
	return &ProcessingError{
		Err:      err,
		Behavior: behavior,
	}
}

func (m *MockHandler) LogErrWithLevel(level int8, err error, behavior string, msg string) *ProcessingError {
	return &ProcessingError{
		Err:      err,
		Behavior: behavior,
	}
}

func (m *MockHandler) Log(level int8, behavior string, msg string) *ProcessingError {
	return &ProcessingError{
		Behavior: behavior,
	}
}

func (m *MockHandler) LogFields(level int8, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	return &ProcessingError{
		Behavior: behavior,
	}
}

func (m *MockHandler) LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	return &ProcessingError{
		Err:      err,
		Behavior: behavior,
	}
}

func (m *MockHandler) GetLogBuffer() bytes.Buffer {
	return m.buffer
}

func (m *MockHandler) HandleStatus(status string) {
	// Do nothing for test
}

func (m *MockHandler) IncrementProgress(taskID string, increment, total, priority int, operation, descr, size string) {
	// Do nothing for test
}

func (m *MockHandler) IsCLI() bool {
	return true
}

// TestSelectiveTransliteration tests the Transliterate function with 
// Japanese selective transliteration enabled
func TestSelectiveTransliteration(t *testing.T) {
	// Skip the test if the subtitle file doesn't exist
	subtitlePath := "/home/voiduser/go/src/langkit-test/Jujutsu Kaisen_AV1_S01E01_Ryomen Sukuna.ja.closedcaptions.srt"
	_, err := os.Stat(subtitlePath)
	if os.IsNotExist(err) {
		t.Skip("Test subtitle file not found, skipping test")
	}

	// Create a temp directory for output files
	tempDir, err := os.MkdirTemp("", "translit-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Copy the test subtitle file to the temp directory
	tempSubPath := filepath.Join(tempDir, "test_subtitle.srt")
	data, err := os.ReadFile(subtitlePath)
	require.NoError(t, err)
	err = os.WriteFile(tempSubPath, data, 0644)
	require.NoError(t, err)

	// Create a test task with a real CLIHandler instead of mock
	jpnLang := iso.FromPart3Code("jpn")
	
	// Create a context and a real CLIHandler
	ctx := context.Background()
	handler := NewCLIHandler(ctx)
	
	tsk := &Task{
		Handler:           handler,
		Targ:              Lang{Language: jpnLang},
		RomanizationStyle: "hepburn",
		KanjiThreshold:    3, // Enable selective transliteration for kanji with >3 strokes
		DockerRecreate:    false,
	}

	// Set up test start time
	startTime := time.Now()
	err = tsk.Transliterate(ctx, tempSubPath)
	
	// More detailed error checking instead of require.NoError
	if err != nil {
		t.Fatalf("Expected no error from Transliterate, got: %v (type: %T)", err, err)
	}
	
	// Print success message
	t.Log("Transliterate function completed successfully")

	// Calculate test duration
	duration := time.Since(startTime)
	t.Logf("Transliteration completed in %v", duration)

	// Check that the expected output files exist
	tokenizedPath := strings.TrimSuffix(tempSubPath, ".srt") + "_tokenized.srt"
	translitPath := strings.TrimSuffix(tempSubPath, ".srt") + "_translit.srt"
	selectivePath := strings.TrimSuffix(tempSubPath, ".srt") + "_selective.srt"

	// Check that all expected files exist
	assert.FileExists(t, tokenizedPath, "Tokenized subtitle file should exist")
	assert.FileExists(t, translitPath, "Transliterated subtitle file should exist")
	assert.FileExists(t, selectivePath, "Selective transliteration file should exist")

	// Verify that files are not empty
	tokenizedStat, err := os.Stat(tokenizedPath)
	require.NoError(t, err)
	assert.Greater(t, tokenizedStat.Size(), int64(0), "Tokenized file should not be empty")

	translitStat, err := os.Stat(translitPath)
	require.NoError(t, err)
	assert.Greater(t, translitStat.Size(), int64(0), "Transliterated file should not be empty")

	selectiveStat, err := os.Stat(selectivePath)
	require.NoError(t, err)
	assert.Greater(t, selectiveStat.Size(), int64(0), "Selective transliteration file should not be empty")

	// Read the files to check their content
	tokenizedContent, err := os.ReadFile(tokenizedPath)
	require.NoError(t, err)
	translitContent, err := os.ReadFile(translitPath)
	require.NoError(t, err)
	selectiveContent, err := os.ReadFile(selectivePath)
	require.NoError(t, err)

	// Verify the content of the files
	assert.NotEqual(t, string(tokenizedContent), string(translitContent), 
		"Tokenized and transliterated files should have different content")
	assert.NotEqual(t, string(selectiveContent), string(translitContent), 
		"Selective and full transliteration files should have different content")
	assert.NotEqual(t, string(tokenizedContent), string(selectiveContent), 
		"Tokenized and selective files should have different content")

	// Verify that selective transliteration contains both Japanese characters and romanized text
	// This is a key characteristic of selective transliteration - it keeps some kanji and romanizes others
	hasJapaneseChars := strings.ContainsAny(string(selectiveContent), "あいうえおカキクケコ漢字")
	hasRomanizedText := strings.ContainsAny(string(selectiveContent), "abcdefghijklmnopqrstuvwxyz")
	assert.True(t, hasJapaneseChars, "Selective transliteration should contain Japanese characters")
	assert.True(t, hasRomanizedText, "Selective transliteration should contain romanized text")

	// Full transliteration should have more romanized content than selective
	romanizedRatioFull := float64(len(strings.Join(findMatches(string(translitContent), "[a-zA-Z]+"), ""))) / float64(len(translitContent))
	romanizedRatioSelective := float64(len(strings.Join(findMatches(string(selectiveContent), "[a-zA-Z]+"), ""))) / float64(len(selectiveContent))
	
	assert.Greater(t, romanizedRatioFull, romanizedRatioSelective, 
		"Full transliteration should have a higher ratio of romanized text than selective")

	// Additional check: Selective transliteration should be a mix of the other two
	// It should share some content with both tokenized and transliterated files
	selectiveLines := strings.Split(string(selectiveContent), "\n")
	tokenizedLines := strings.Split(string(tokenizedContent), "\n")
	translitLines := strings.Split(string(translitContent), "\n")

	// Compare line counts
	assert.Equal(t, len(tokenizedLines), len(translitLines), "Line count should be the same in all files")
	assert.Equal(t, len(tokenizedLines), len(selectiveLines), "Line count should be the same in all files")

	t.Logf("Test completed successfully. %d lines processed.", len(tokenizedLines))
}

// Helper function to find all regex matches in a string
func findMatches(s, pattern string) []string {
	r := regexp.MustCompile(pattern)
	return r.FindAllString(s, -1)
}