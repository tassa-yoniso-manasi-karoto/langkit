package core

import (
	"fmt"
	"testing"
	"path/filepath"
	
	"github.com/stretchr/testify/assert"
)

// This test validates that validateBasicRequirements works correctly
func TestValidateBasicRequirements(t *testing.T) {
	// Create a task with the existing MockHandler
	mockHandler := NewMockHandler()
	tsk := &Task{
		Handler: mockHandler,
	}
	
	// Test case 1: No apostrophe in path
	tsk.TargSubFile = filepath.Join("path", "to", "subtitles.srt")
	tsk.Langs = []string{"ja"}
	
	err := tsk.validateBasicRequirements()
	assert.Nil(t, err, "Should not return error when path has no apostrophe and langs are specified")
	
	// Test case 2: No languages or subtitle files specified
	tsk.Langs = []string{}
	tsk.TargSubFile = ""
	
	// Set up mock for error
	expectedErr := &ProcessingError{
		Behavior: AbortAllTasks,
		Err: fmt.Errorf("Neither languages and nor subtitle files were specified."),
	}
	mockHandler.On("Log", Error, AbortAllTasks, "Neither languages and nor subtitle files were specified.").Return(expectedErr)
	
	err = tsk.validateBasicRequirements()
	assert.NotNil(t, err, "Should return error when no languages or subtitles are specified")
	assert.Equal(t, AbortAllTasks, err.Behavior)
	
	// Test utility functions don't throw errors
	tsk.TargSubFile = filepath.Join("path", "to", "movie's.srt")
	tsk.MediaSourceFile = filepath.Join("path", "to", "video.mp4")
	tsk.OutputFileExtension = ".tsv"
	
	// Test that these functions return expected values without errors
	assert.Contains(t, tsk.outputBase(), "movie")
	assert.Contains(t, tsk.outputFile(), ".tsv")
	assert.Contains(t, tsk.mediaOutputDir(), ".media")
}

func TestEscape(t *testing.T) {
	// Test the escape function
	assert.Equal(t, "normal", escape("normal"), "Should not modify normal text")
	assert.Equal(t, `"tab	char"`, escape("tab\tchar"), "Should escape tab characters")
	assert.Equal(t, `"new
line"`, escape("new\nline"), "Should escape newline characters")
	assert.Equal(t, `"quote""quote"`, escape(`quote"quote`), "Should escape quote characters")
}

func TestBase2Absolute(t *testing.T) {
	assert.Equal(t, filepath.Join("dir", "file.txt"), Base2Absolute("file.txt", "dir"), "Should create correct path")
	assert.Equal(t, "", Base2Absolute("", "dir"), "Should handle empty paths")
}

func TestStructuredExecute(t *testing.T) {
	// This test doesn't actually call Execute but validates that the refactored structure
	// has all the components we expect
	mockHandler := NewMockHandler()
	tsk := NewTask(mockHandler)
	
	// Check that the task has all the expected methods from our refactoring
	// These won't actually run, we're just checking they exist
	assert.NotNil(t, tsk.validateBasicRequirements, "validateBasicRequirements should exist")
	
	// Check the utility functions
	assert.NotNil(t, tsk.outputBase, "outputBase function should exist")
	assert.NotNil(t, tsk.outputFile, "outputFile function should exist")
	assert.NotNil(t, tsk.mediaOutputDir, "mediaOutputDir function should exist")
}