package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// Mock FileScanner for testing
type MockFileScanner struct {
	mock.Mock
}

func (m *MockFileScanner) ScanForContent(filePath, pattern string) (bool, error) {
	args := m.Called(filePath, pattern)
	return args.Bool(0), args.Error(1)
}

func (m *MockFileScanner) GetLastProcessedIndex(filePath string) (int, error) {
	args := m.Called(filePath)
	return args.Int(0), args.Error(1)
}

// We'll use the MockResumptionHandler from test_helpers.go

// ResumptionServiceSuite is a test suite for resumption service
type ResumptionServiceSuite struct {
	suite.Suite
	service      ResumptionService
	fileScanner  *MockFileScanner
	handler      *MockResumptionHandler
	fs           afero.Fs
	outputFile   string
	separator    string
}

func (suite *ResumptionServiceSuite) SetupTest() {
	suite.fileScanner = new(MockFileScanner)
	suite.handler = NewMockResumptionHandlerWithDefaults()
	suite.fs = afero.NewMemMapFs()
	suite.outputFile = "/output/file.txt"
	suite.separator = "\t"
	
	// Setup handler to return output file path
	suite.handler.On("GetOutputFilePath").Return(suite.outputFile)
	
	suite.service = NewResumptionService(suite.fileScanner, suite.separator, suite.handler)
}

func TestResumptionServiceSuite(t *testing.T) {
	suite.Run(t, new(ResumptionServiceSuite))
}

func (suite *ResumptionServiceSuite) TestIsAlreadyProcessed_FileExists() {
	// Configure mock to indicate item is in file
	suite.fileScanner.On("ScanForContent", suite.outputFile, "test-identifier").Return(true, nil)
	
	// Test when item is already processed
	result, err := suite.service.IsAlreadyProcessed("test-identifier")
	
	suite.True(result)
	suite.NoError(err)
	suite.fileScanner.AssertExpectations(suite.T())
}

func (suite *ResumptionServiceSuite) TestIsAlreadyProcessed_FileDoesNotExist() {
	// Configure mock to simulate file not existing
	suite.fileScanner.On("ScanForContent", suite.outputFile, "test-identifier").Return(false, nil)
	
	// Test when item is not processed
	result, err := suite.service.IsAlreadyProcessed("test-identifier")
	
	suite.False(result)
	suite.NoError(err)
	suite.fileScanner.AssertExpectations(suite.T())
}

func (suite *ResumptionServiceSuite) TestIsAlreadyProcessed_Error() {
	expectedErr := fmt.Errorf("file scan error")
	
	// Configure mock to return an error
	suite.fileScanner.On("ScanForContent", suite.outputFile, "test-identifier").Return(false, expectedErr)
	
	// Test when scan encounters an error
	result, err := suite.service.IsAlreadyProcessed("test-identifier")
	
	suite.False(result)
	suite.Equal(expectedErr, err)
	suite.fileScanner.AssertExpectations(suite.T())
}

func (suite *ResumptionServiceSuite) TestIsAlreadyProcessed_NoOutputFile() {
	// Configure handler to return empty output file
	noOutputHandler := NewMockResumptionHandlerWithDefaults()
	noOutputHandler.On("GetOutputFilePath").Return("")
	
	// Create service with new handler
	service := NewResumptionService(suite.fileScanner, suite.separator, noOutputHandler)
	
	// Test when no output file is specified
	result, err := service.IsAlreadyProcessed("test-identifier")
	
	suite.False(result)
	suite.NoError(err)
	// FileScan should not be called when there's no output file
	suite.fileScanner.AssertNotCalled(suite.T(), "ScanForContent", mock.Anything, mock.Anything)
}

func (suite *ResumptionServiceSuite) TestGetResumePoint() {
	// Configure mock for file with some processed items
	suite.fileScanner.On("GetLastProcessedIndex", suite.outputFile).Return(42, nil)
	
	// Test getting resume point
	index, err := suite.service.GetResumePoint(suite.outputFile)
	
	suite.Equal(42, index)
	suite.NoError(err)
	suite.fileScanner.AssertExpectations(suite.T())
}

func (suite *ResumptionServiceSuite) TestGetResumePoint_Error() {
	expectedErr := fmt.Errorf("file scan error")
	
	// Configure mock to return an error
	suite.fileScanner.On("GetLastProcessedIndex", suite.outputFile).Return(0, expectedErr)
	
	// Test when scan encounters an error
	index, err := suite.service.GetResumePoint(suite.outputFile)
	
	suite.Equal(0, index)
	suite.Equal(expectedErr, err)
	suite.fileScanner.AssertExpectations(suite.T())
}

func (suite *ResumptionServiceSuite) TestGetResumePoint_NoOutputFile() {
	// Test when no output file is specified
	index, err := suite.service.GetResumePoint("")
	
	suite.Equal(0, index)
	suite.NoError(err)
	// FileScan should not be called when there's no output file
	suite.fileScanner.AssertNotCalled(suite.T(), "GetLastProcessedIndex", mock.Anything)
}

// Test DefaultFileScanner implementation with real files
func TestDefaultFileScanner(t *testing.T) {
	// Use in-memory filesystem
	fs := afero.NewMemMapFs()
	
	// Setup a mock handler for the scanner
	mockHandler := NewMockResumptionHandlerWithDefaults()
	
	// Create the DefaultFileScanner
	scanner := NewFileScanner(mockHandler)
	
	// Create a test file
	testDir := "/test/output"
	fs.MkdirAll(testDir, 0755)
	testFilePath := filepath.Join(testDir, "processed.txt")
	
	// Test when file doesn't exist
	found, err := scanner.ScanForContent(testFilePath, "content to find")
	assert.False(t, found)
	assert.NoError(t, err)
	
	// Create a file with some content
	testFile, _ := fs.Create(testFilePath)
	testContent := "line1\nline2\nline with content to find\nline4"
	testFile.WriteString(testContent)
	testFile.Close()
	
	// Write file to real filesystem for scanning
	tmpDir, err := os.MkdirTemp("", "resumption_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	tmpFile := filepath.Join(tmpDir, "processed.txt")
	err = os.WriteFile(tmpFile, []byte(testContent), 0644)
	assert.NoError(t, err)
	
	// Test scanning for content that exists
	found, err = scanner.ScanForContent(tmpFile, "content to find")
	assert.True(t, found)
	assert.NoError(t, err)
	
	// Test scanning for content that doesn't exist
	found, err = scanner.ScanForContent(tmpFile, "not in the file")
	assert.False(t, found)
	assert.NoError(t, err)
	
	// Test getting last processed index
	index, err := scanner.GetLastProcessedIndex(tmpFile)
	assert.Equal(t, 4, index) // 4 lines, so index should be 4
	assert.NoError(t, err)
	
	// Test error case with invalid file path
	_, err = scanner.ScanForContent("/nonexistent/file", "test")
	assert.NoError(t, err) // Should handle gracefully
	
	// Test with empty file
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	err = os.WriteFile(emptyFile, []byte{}, 0644)
	assert.NoError(t, err)
	
	found, err = scanner.ScanForContent(emptyFile, "anything")
	assert.False(t, found)
	assert.NoError(t, err)
	
	index, err = scanner.GetLastProcessedIndex(emptyFile)
	assert.Equal(t, 0, index)
	assert.NoError(t, err)
}

// TestIntegrationWithDefaultImplementations tests the complete ResumptionService with DefaultFileScanner
func TestIntegrationWithDefaultImplementations(t *testing.T) {
	// Setup temporary test directory
	tmpDir, err := os.MkdirTemp("", "resumption_integration")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	// Create output file with some processed items
	outputFile := filepath.Join(tmpDir, "output.tsv")
	content := "item1\t00:00:10,000\tprocessed1\nitem2\t00:00:20,000\tprocessed2\n"
	err = os.WriteFile(outputFile, []byte(content), 0644)
	assert.NoError(t, err)
	
	// Setup mock handler that returns our output file
	mockHandler := NewMockResumptionHandlerWithDefaults()
	mockHandler.On("GetOutputFilePath").Return(outputFile)
	
	// Create real FileScanner
	fileScanner := NewFileScanner(mockHandler)
	
	// Create ResumptionService with tab separator
	service := NewResumptionService(fileScanner, "\t", mockHandler)
	
	// Test for item that is already processed
	alreadyProcessed, err := service.IsAlreadyProcessed("\t00:00:10,000\t")
	assert.True(t, alreadyProcessed)
	assert.NoError(t, err)
	
	// Test for item that is not processed
	alreadyProcessed, err = service.IsAlreadyProcessed("\t00:00:30,000\t")
	assert.False(t, alreadyProcessed)
	assert.NoError(t, err)
	
	// Test getting resume point
	index, err := service.GetResumePoint(outputFile)
	assert.Equal(t, 2, index) // 2 lines, so we should have processed 2 items
	assert.NoError(t, err)
}

// Test marking an item as processed - this is a no-op in our implementation
func TestMarkAsProcessed(t *testing.T) {
	fileScanner := new(MockFileScanner)
	mockHandler := new(MockResumptionHandler)
	
	service := NewResumptionService(fileScanner, "\t", mockHandler)
	
	// This is a no-op in our implementation, but should return no error
	err := service.MarkAsProcessed("test-identifier")
	assert.NoError(t, err)
}

// Test DefaultFileScanner with unicode content
func TestDefaultFileScanner_Unicode(t *testing.T) {
	// Setup temporary test directory
	tmpDir, err := os.MkdirTemp("", "unicode_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	// Create file with unicode content
	unicodeFile := filepath.Join(tmpDir, "unicode.txt")
	unicodeContent := "普通话\n日本語\nEnglish\n한국어\n"
	err = os.WriteFile(unicodeFile, []byte(unicodeContent), 0644)
	assert.NoError(t, err)
	
	// Setup mock handler
	mockHandler := NewMockResumptionHandlerWithDefaults()
	
	// Create scanner
	scanner := NewFileScanner(mockHandler)
	
	// Test scanning for unicode content
	found, err := scanner.ScanForContent(unicodeFile, "日本語")
	assert.True(t, found)
	assert.NoError(t, err)
	
	// Test getting last processed index
	index, err := scanner.GetLastProcessedIndex(unicodeFile)
	assert.Equal(t, 4, index) // 4 lines
	assert.NoError(t, err)
}

// TestResumptionWithEmptyFile tests handling resumption when the output file exists but is empty
func TestResumptionWithEmptyFile(t *testing.T) {
	// Setup temporary test directory
	tmpDir, err := os.MkdirTemp("", "empty_file_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	// Create empty output file
	outputFile := filepath.Join(tmpDir, "empty.tsv")
	err = os.WriteFile(outputFile, []byte{}, 0644)
	assert.NoError(t, err)
	
	// Setup mock handler
	mockHandler := NewMockResumptionHandlerWithDefaults()
	mockHandler.On("GetOutputFilePath").Return(outputFile)
	
	// Create real FileScanner
	fileScanner := NewFileScanner(mockHandler)
	
	// Create ResumptionService
	service := NewResumptionService(fileScanner, "\t", mockHandler)
	
	// Test for any item - nothing should be processed
	alreadyProcessed, err := service.IsAlreadyProcessed("anything")
	assert.False(t, alreadyProcessed)
	assert.NoError(t, err)
	
	// Test getting resume point - should be 0 for empty file
	index, err := service.GetResumePoint(outputFile)
	assert.Equal(t, 0, index)
	assert.NoError(t, err)
}