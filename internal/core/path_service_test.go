package core

import (
	"path"
	"path/filepath"
	"testing"
	
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/google/go-cmp/cmp"
)

// MockPathSanitizer mocks the PathSanitizer interface for testing
type MockPathSanitizer struct {
	mock.Mock
}

func (m *MockPathSanitizer) SanitizeForFileSystem(input string) string {
	args := m.Called(input)
	return args.String(0)
}

func (m *MockPathSanitizer) SanitizeForFFmpeg(input string) string {
	args := m.Called(input)
	return args.String(0)
}

// PathServiceSuite is a test suite for path service functions
type PathServiceSuite struct {
	suite.Suite
	service           PathService
	mockSanitizer     *MockPathSanitizer
	defaultSanitizer  PathSanitizer
	fs                afero.Fs  // In-memory filesystem for testing
}

func (suite *PathServiceSuite) SetupTest() {
	suite.mockSanitizer = new(MockPathSanitizer)
	suite.defaultSanitizer = NewPathSanitizer()
	suite.service = NewPathService(suite.mockSanitizer)
	suite.fs = afero.NewMemMapFs() // In-memory filesystem
}

func TestPathServiceSuite(t *testing.T) {
	suite.Run(t, new(PathServiceSuite))
}

func (suite *PathServiceSuite) TestOutputBasePath() {
	// Setup mock expectations
	suite.mockSanitizer.On("SanitizeForFileSystem", "movie").Return("movie")
	suite.mockSanitizer.On("SanitizeForFileSystem", "movie.en").Return("movie.en")
	suite.mockSanitizer.On("SanitizeForFileSystem", "movie with'apostrophe").Return("movie with apostrophe")
	
	tests := []struct {
		name         string
		subtitlePath string
		expected     string
	}{
		{
			name:         "Simple Path",
			subtitlePath: "/path/to/movie.srt",
			expected:     "movie",
		},
		{
			name:         "Path with Language Extension",
			subtitlePath: "/path/to/movie.en.srt",
			expected:     "movie.en",
		},
		{
			name:         "Path with Apostrophe",
			subtitlePath: "/path/to/movie with'apostrophe.srt",
			expected:     "movie with apostrophe",
		},
	}
	
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			result := suite.service.OutputBasePath(tc.subtitlePath)
			suite.Equal(tc.expected, result)
		})
	}
	
	// Verify mock was called as expected
	suite.mockSanitizer.AssertExpectations(suite.T())
}

func (suite *PathServiceSuite) TestOutputFilePath() {
	// Create a service with the default sanitizer since this test doesn't need mocking
	service := NewPathService(suite.defaultSanitizer)
	
	tests := []struct {
		name         string
		mediaSource  string
		base         string
		extension    string
		expected     string
	}{
		{
			name:        "Simple Path",
			mediaSource: "/path/to/video.mp4",
			base:        "movie",
			extension:   ".tsv",
			expected:    "/path/to/movie.tsv",
		},
		{
			name:        "Different Directory",
			mediaSource: "/videos/movie.mp4",
			base:        "subtitles",
			extension:   ".srt",
			expected:    "/videos/subtitles.srt",
		},
		{
			name:        "Complex Base",
			mediaSource: "/path/to/complex.video.mp4",
			base:        "movie.with.dots",
			extension:   ".txt",
			expected:    "/path/to/movie.with.dots.txt",
		},
		{
			name:        "With Extension Already in Base",
			mediaSource: "/path/to/video.mp4",
			base:        "movie.base",
			extension:   ".ext",
			expected:    "/path/to/movie.base.ext",
		},
	}
	
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			result := service.OutputFilePath(tc.mediaSource, tc.base, tc.extension)
			suite.Equal(tc.expected, result)
		})
	}
}

func (suite *PathServiceSuite) TestMediaOutputDir() {
	// Create a service with the default sanitizer
	service := NewPathService(suite.defaultSanitizer)
	
	tests := []struct {
		name        string
		mediaSource string
		base        string
		expected    string
	}{
		{
			name:        "Simple Path",
			mediaSource: "/path/to/video.mp4",
			base:        "movie",
			expected:    "/path/to/movie.media",
		},
		{
			name:        "Different Directory",
			mediaSource: "/videos/movie.mp4",
			base:        "subtitles",
			expected:    "/videos/subtitles.media",
		},
		{
			name:        "Complex Path",
			mediaSource: "/complex/path with spaces/video.mp4",
			base:        "output-base",
			expected:    "/complex/path with spaces/output-base.media",
		},
	}
	
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			result := service.MediaOutputDir(tc.mediaSource, tc.base)
			suite.Equal(tc.expected, result)
		})
	}
}

func (suite *PathServiceSuite) TestAudioBasePath() {
	// Setup mock expectations
	suite.mockSanitizer.On("SanitizeForFileSystem", "video").Return("video")
	suite.mockSanitizer.On("SanitizeForFileSystem", "movie with'apostrophe").Return("movie with apostrophe")
	suite.mockSanitizer.On("SanitizeForFileSystem", "complex.name.with.dots").Return("complex.name.with.dots")
	
	tests := []struct {
		name        string
		mediaSource string
		expected    string
	}{
		{
			name:        "Simple Path",
			mediaSource: "/path/to/video.mp4",
			expected:    "video",
		},
		{
			name:        "Path with Apostrophe",
			mediaSource: "/path/to/movie with'apostrophe.mp4",
			expected:    "movie with apostrophe",
		},
		{
			name:        "Multiple Extensions",
			mediaSource: "/path/to/complex.name.with.dots.mp4",
			expected:    "complex.name.with.dots",
		},
	}
	
	for _, tc := range tests {
		suite.Run(tc.name, func() {
			result := suite.service.AudioBasePath(tc.mediaSource)
			suite.Equal(tc.expected, result)
		})
	}
	
	// Verify mock was called as expected
	suite.mockSanitizer.AssertExpectations(suite.T())
}

// Test with real filesystem operations using afero
func (suite *PathServiceSuite) TestWithVirtualFileSystem() {
	// Setup test paths
	testDir := "/test/directory"
	suite.fs.MkdirAll(testDir, 0755)
	
	// Create a test file
	testFilePath := path.Join(testDir, "test's file.mp4")
	testFile, err := suite.fs.Create(testFilePath)
	suite.Require().NoError(err)
	testFile.Close()
	
	// Create a service with real sanitizer
	service := NewPathService(NewPathSanitizer())
	
	// Test outputBasePath
	basePathResult := service.OutputBasePath(testFilePath)
	suite.Equal("test s file", basePathResult)
	
	// Test outputFilePath
	outputPath := service.OutputFilePath(testFilePath, basePathResult, ".srt")
	suite.Equal(path.Join(testDir, "test s file.srt"), outputPath)
	
	// Test mediaOutputDir
	mediaDir := service.MediaOutputDir(testFilePath, basePathResult)
	suite.Equal(path.Join(testDir, "test s file.media"), mediaDir)
	
	// Verify media directory can be created
	err = suite.fs.MkdirAll(mediaDir, 0755)
	suite.NoError(err)
	
	// Verify directory exists
	info, err := suite.fs.Stat(mediaDir)
	suite.NoError(err)
	suite.True(info.IsDir())
}

func TestPathSanitizer_Implementation(t *testing.T) {
	sanitizer := NewPathSanitizer()
	
	// Test case table for SanitizeForFileSystem
	fsTests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Apostrophes",
			input:    "file's name",
			expected: "file s name",
		},
		{
			name:     "Special Characters",
			input:    "file:name?with*weird|chars",
			expected: "file_name_with_weird_chars",
		},
		{
			name:     "Path Separators",
			input:    "file/name\\with/separators",
			expected: "file_name_with_separators",
		},
		{
			name:     "Unicode Characters",
			input:    "файл с пробелами",
			expected: "файл с пробелами", // Unicode chars should remain unchanged
		},
		{
			name:     "Empty String",
			input:    "",
			expected: "",
		},
		{
			name:     "Already Safe String",
			input:    "safe_filename",
			expected: "safe_filename",
		},
	}
	
	for _, tc := range fsTests {
		t.Run("FileSystem_"+tc.name, func(t *testing.T) {
			result := sanitizer.SanitizeForFileSystem(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
	
	// Test case table for SanitizeForFFmpeg
	ffmpegTests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Apostrophes",
			input:    "file's name",
			expected: "file\\\\'s name",
		},
		{
			name:     "Special Characters",
			input:    "file:name,with[brackets]",
			expected: "file\\\\:name\\,with\\[brackets\\]",
		},
		{
			name:     "Backslashes",
			input:    "file\\with\\backslashes",
			expected: "file\\\\with\\\\backslashes",
		},
		{
			name:     "Semicolons",
			input:    "command;injection;attempt",
			expected: "command\\;injection\\;attempt",
		},
		{
			name:     "Empty String",
			input:    "",
			expected: "",
		},
		{
			name:     "Already Safe String",
			input:    "safe_filename",
			expected: "safe_filename",
		},
	}
	
	for _, tc := range ffmpegTests {
		t.Run("FFmpeg_"+tc.name, func(t *testing.T) {
			result := sanitizer.SanitizeForFFmpeg(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Mock that integrates PathService with real filesystems
func TestPathServiceWithRealFileSystem(t *testing.T) {
	// Use in-memory filesystem
	fs := afero.NewMemMapFs()
	
	// Create test directory structure
	testDir := "/media/videos"
	fs.MkdirAll(testDir, 0755)
	
	// Create test files
	testFiles := []string{
		"movie.en.srt",
		"movie with spaces.mp4",
		"special:character?file.avi",
	}
	
	for _, filename := range testFiles {
		file, err := fs.Create(filepath.Join(testDir, filename))
		assert.NoError(t, err)
		file.Close()
	}
	
	// Create path service
	pathService := NewPathService(NewPathSanitizer())
	
	// Test paths for each file
	for _, filename := range testFiles {
		fullPath := filepath.Join(testDir, filename)
		
		// Get base path
		basePath := pathService.OutputBasePath(fullPath)
		
		// Test output file path
		outputPath := pathService.OutputFilePath(fullPath, basePath, ".txt")
		
		// Verify we can create file at the output path
		outputFile, err := fs.Create(outputPath)
		assert.NoError(t, err, "Failed to create file at output path: %s", outputPath)
		outputFile.Close()
		
		// Test media output directory
		mediaDir := pathService.MediaOutputDir(fullPath, basePath)
		
		// Verify we can create the media directory
		err = fs.MkdirAll(mediaDir, 0755)
		assert.NoError(t, err, "Failed to create media directory: %s", mediaDir)
		
		// Verify the directory exists
		info, err := fs.Stat(mediaDir)
		assert.NoError(t, err)
		assert.True(t, info.IsDir())
		
		// Create a file in the media directory to verify it works
		mediaFile, err := fs.Create(filepath.Join(mediaDir, "test.txt"))
		assert.NoError(t, err)
		mediaFile.Close()
	}
}

// Test for Path equality using go-cmp
func TestPathEquality(t *testing.T) {
	type PathPair struct {
		Media  string
		Output string
	}
	
	paths1 := PathPair{
		Media:  "/path/to/file.mp4",
		Output: "/path/to/file.txt",
	}
	
	paths2 := PathPair{
		Media:  "/path/to/file.mp4",
		Output: "/path/to/file.txt",
	}
	
	paths3 := PathPair{
		Media:  "/path/to/file.mp4",
		Output: "/path/to/different.txt",
	}
	
	// Equal paths should have no diff
	if diff := cmp.Diff(paths1, paths2); diff != "" {
		t.Errorf("Paths should be equal, but got diff: %s", diff)
	}
	
	// Different paths should have a diff
	if diff := cmp.Diff(paths1, paths3); diff == "" {
		t.Errorf("Paths should be different")
	} else {
		// This is expected - verify the diff contains the output path
		assert.Contains(t, diff, "Output")
	}
}

// Test with a mock path service
func TestWithMockPathService(t *testing.T) {
	// Mock path service
	mockService := new(MockPathService)
	
	// Setup expectations
	mockService.On("OutputBasePath", "/test/file.srt").Return("file")
	mockService.On("OutputFilePath", "/test/video.mp4", "file", ".txt").Return("/test/file.txt")
	mockService.On("MediaOutputDir", "/test/video.mp4", "file").Return("/test/file.media")
	
	// Use the mock in a function that would process a file
	processFile := func(service PathService, videoPath, subtitlePath string) (outputPath, mediaDir string) {
		// Get base name from subtitle path
		basePath := service.OutputBasePath(subtitlePath)
		
		// Create output path
		outputPath = service.OutputFilePath(videoPath, basePath, ".txt")
		
		// Create media directory path
		mediaDir = service.MediaOutputDir(videoPath, basePath)
		
		return outputPath, mediaDir
	}
	
	// Test our function with the mock
	outputPath, mediaDir := processFile(mockService, "/test/video.mp4", "/test/file.srt")
	
	// Verify results
	assert.Equal(t, "/test/file.txt", outputPath)
	assert.Equal(t, "/test/file.media", mediaDir)
	
	// Verify expectations were met
	mockService.AssertExpectations(t)
}

// MockPathService implements PathService for testing
type MockPathService struct {
	mock.Mock
}

func (m *MockPathService) OutputBasePath(subtitlePath string) string {
	args := m.Called(subtitlePath)
	return args.String(0)
}

func (m *MockPathService) OutputFilePath(mediaSource, base, extension string) string {
	args := m.Called(mediaSource, base, extension)
	return args.String(0)
}

func (m *MockPathService) MediaOutputDir(mediaSource, base string) string {
	args := m.Called(mediaSource, base)
	return args.String(0)
}

func (m *MockPathService) AudioBasePath(mediaSource string) string {
	args := m.Called(mediaSource)
	return args.String(0)
}