package core

import (
	"path"
	"strings"
)

// DefaultPathService implements the PathService interface
type DefaultPathService struct {
	sanitizer PathSanitizer
}

// NewPathService creates a new DefaultPathService
func NewPathService(sanitizer PathSanitizer) PathService {
	if sanitizer == nil {
		sanitizer = NewPathSanitizer()
	}
	return &DefaultPathService{
		sanitizer: sanitizer,
	}
}

// OutputBasePath gets the base path for output files
func (p *DefaultPathService) OutputBasePath(subtitlePath string) string {
	base := strings.TrimSuffix(path.Base(subtitlePath), path.Ext(subtitlePath))
	return p.sanitizer.SanitizeForFileSystem(base)
}

// OutputFilePath constructs full output file paths
func (p *DefaultPathService) OutputFilePath(mediaSource, base, extension string) string {
	return path.Join(path.Dir(mediaSource), base+extension)
}

// MediaOutputDir gets the directory for media output
func (p *DefaultPathService) MediaOutputDir(mediaSource, base string) string {
	return path.Join(path.Dir(mediaSource), base+".media")
}

// AudioBasePath gets the base path for audio files
func (p *DefaultPathService) AudioBasePath(mediaSource string) string {
	base := strings.TrimSuffix(path.Base(mediaSource), path.Ext(mediaSource))
	return p.sanitizer.SanitizeForFileSystem(base)
}

// DefaultPathSanitizer implements the PathSanitizer interface
type DefaultPathSanitizer struct{}

// NewPathSanitizer creates a new DefaultPathSanitizer
func NewPathSanitizer() PathSanitizer {
	return &DefaultPathSanitizer{}
}

// SanitizeForFileSystem sanitizes a path for the file system
func (s *DefaultPathSanitizer) SanitizeForFileSystem(input string) string {
	// Replace apostrophes with spaces (common source of path issues)
	sanitized := strings.ReplaceAll(input, "'", " ")
	// Replace other problematic characters
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	sanitized = strings.ReplaceAll(sanitized, "\\", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, "*", "_")
	sanitized = strings.ReplaceAll(sanitized, "?", "_")
	sanitized = strings.ReplaceAll(sanitized, "\"", "_")
	sanitized = strings.ReplaceAll(sanitized, "<", "_")
	sanitized = strings.ReplaceAll(sanitized, ">", "_")
	sanitized = strings.ReplaceAll(sanitized, "|", "_")
	return sanitized
}

// SanitizeForFFmpeg sanitizes a path for FFmpeg commands
func (s *DefaultPathSanitizer) SanitizeForFFmpeg(input string) string {
	// FFmpeg requires special escaping for certain characters
	sanitized := strings.ReplaceAll(input, "'", "\\'")
	sanitized = strings.ReplaceAll(sanitized, ":", "\\:")
	sanitized = strings.ReplaceAll(sanitized, "\\", "\\\\")
	sanitized = strings.ReplaceAll(sanitized, "[", "\\[")
	sanitized = strings.ReplaceAll(sanitized, "]", "\\]")
	sanitized = strings.ReplaceAll(sanitized, ",", "\\,")
	sanitized = strings.ReplaceAll(sanitized, ";", "\\;")
	return sanitized
}