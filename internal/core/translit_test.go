//go:build integration
// +build integration

package core

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	iso "github.com/barbashov/iso639-3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: This test now uses the shared MockHandler from test_helpers.go

// TestSelectiveTransliteration tests the Transliterate function with 
// Japanese selective transliteration enabled
func TestSelectiveTransliteration(t *testing.T) {
	// Use environment variable for subtitle file path or skip test
	subtitlePath := os.Getenv("LANGKIT_TEST_SUBTITLE_FILE")
	if subtitlePath == "" {
		t.Skip("LANGKIT_TEST_SUBTITLE_FILE environment variable not set, skipping test")
	}
	
	// Check if the file exists
	_, err := os.Stat(subtitlePath)
	if os.IsNotExist(err) {
		t.Skipf("Test subtitle file not found at %s, skipping test", subtitlePath)
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

	// Create a test task with appropriate language settings
	jpnLang := iso.FromPart3Code("jpn")
	
	// Create a context and handler
	ctx := context.Background()
	
	// Use real CLIHandler if LANGKIT_PROFILE_TRANSLIT is set to 1
	var handler MessageHandler
	if os.Getenv("LANGKIT_PROFILE_TRANSLIT") == "1" {
		// Use real handler for profiling
		handler = NewCLIHandler(ctx)
	} else {
		// Use TestMessageHandler instead of mock that requires setup
		handler = NewTestMessageHandler()
	}
	
	tsk := &Task{
		Handler:           handler,
		Targ:              Lang{Language: jpnLang},
		TargSubFile:       tempSubPath,
		RomanizationStyle: "hepburn",
		KanjiThreshold:    3, // Enable selective transliteration for kanji with >3 strokes
		DockerRecreate:    false,
		// Set all transliteration types for testing
		WantTranslit:      true,
		TranslitTypes:     []TranslitType{Tokenize, Romanize, Selective},
	}

	// Set up test start time
	startTime := time.Now()
	procErr := tsk.Transliterate(ctx)
	
	if procErr != nil && procErr.Err != nil {
		t.Fatalf("Expected no error from Transliterate, got: %v (type: %T)", procErr, procErr)
	}
	
	// Print success message
	t.Log("Transliterate function completed successfully")

	// Calculate test duration
	duration := time.Since(startTime)
	t.Logf("Transliteration completed in %v", duration)

	// Check that the expected output files exist - using TranslitType.ToSuffix() to get the correct paths
	tokenizedPath := strings.TrimSuffix(tempSubPath, ".srt") + Tokenize.ToSuffix()  // "_tokenized.srt"
	translitPath := strings.TrimSuffix(tempSubPath, ".srt") + Romanize.ToSuffix()   // "_romanized.srt"
	selectivePath := strings.TrimSuffix(tempSubPath, ".srt") + Selective.ToSuffix() // "_selective.srt"

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

	// Optional: Check actual content of files if LANGKIT_VERIFY_CONTENT is set
	if os.Getenv("LANGKIT_VERIFY_CONTENT") == "1" {
		// Read tokenized content
		tokenizedContent, err := os.ReadFile(tokenizedPath)
		require.NoError(t, err)
		tokenizedText := string(tokenizedContent)

		// Check for expected patterns in tokenized text using regex
		spacePattern := regexp.MustCompile(`\p{Han} \p{Han}`)
		assert.True(t, spacePattern.MatchString(tokenizedText), 
			"Tokenized text should have spaces between kanji characters")

		// Read romanized content
		romanizedContent, err := os.ReadFile(translitPath)
		require.NoError(t, err)
		romanizedText := string(romanizedContent)

		// Check for expected patterns in romanized text
		romaPattern := regexp.MustCompile(`[a-zA-Z]`)
		assert.True(t, romaPattern.MatchString(romanizedText),
			"Romanized text should contain Roman alphabet characters")

		// Read selective content
		selectiveContent, err := os.ReadFile(selectivePath)
		require.NoError(t, err)
		selectiveText := string(selectiveContent)

		// Check that selective text contains both kana and kanji
		kanaPattern := regexp.MustCompile(`[\p{Hiragana}\p{Katakana}]`)
		kanjiPattern := regexp.MustCompile(`\p{Han}`)
		assert.True(t, kanaPattern.MatchString(selectiveText) && kanjiPattern.MatchString(selectiveText),
			"Selective text should contain both kana and kanji")
	}
}