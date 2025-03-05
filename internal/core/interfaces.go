package core

import (
	"context"
	"io/fs"

	"github.com/asticode/go-astisub"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)

// LanguageDetector provides methods for detecting and parsing languages from various sources
type LanguageDetector interface {
	// GuessLangFromFilename attempts to detect language from a file name
	GuessLangFromFilename(filename string) (Lang, error)
	// ParseLanguageTags parses language tags from a string
	ParseLanguageTags(langTag string) []Lang
}

// FileLister abstracts filesystem operations needed for subtitle and language detection
type FileLister interface {
	// ReadDir reads a directory and returns a list of directory entries
	ReadDir(dirname string) ([]fs.DirEntry, error)
	// Exists checks if a path exists
	Exists(path string) bool
}

// TrackSelector provides methods for selecting audio tracks
type TrackSelector interface {
	// SelectTrack selects the best audio track based on given criteria
	SelectTrack(tracks []AudioTrack, targetLang Lang) (int, error)
}

// AudioCriteria contains criteria for audio track selection
type AudioCriteria struct {
	TargetLanguage Lang
	TargetChannels int
	ExcludeDescriptions bool
}

// WorkerPool manages concurrent processing of subtitle items
type WorkerPool interface {
	// Submit adds a subtitle item to the processing queue
	Submit(item IndexedSubItem) error
	// ProcessItems processes a batch of subtitle items concurrently
	ProcessItems(ctx context.Context, items []*astisub.Item) ([]ProcessedItem, error)
	// Shutdown gracefully shuts down the worker pool
	Shutdown() error
}

// ProgressTracker tracks progress for long-running operations
type ProgressTracker interface {
	// UpdateProgress updates the overall progress
	UpdateProgress(completed, total int, description string)
	// MarkCompleted marks a specific item as completed
	MarkCompleted(id string)
	// MarkFailed marks a specific item as failed
	MarkFailed(id string, err error)
}

// ResumptionService handles recovery and resumption of interrupted processing
type ResumptionService interface {
	// IsAlreadyProcessed checks if an item has already been processed
	IsAlreadyProcessed(identifier string) (bool, error)
	// MarkAsProcessed marks an item as processed
	MarkAsProcessed(identifier string) error
	// GetResumePoint finds the point at which to resume processing
	GetResumePoint(outputFile string) (int, error)
}

// FileScanner provides file content scanning capabilities
type FileScanner interface {
	// ScanForContent scans a file for a specific pattern
	ScanForContent(filePath, pattern string) (bool, error)
	// GetLastProcessedIndex gets the index of the last processed item
	GetLastProcessedIndex(filePath string) (int, error)
}

// PathService provides file path construction utilities
type PathService interface {
	// OutputBasePath gets the base path for output files
	OutputBasePath(subtitlePath string) string
	// OutputFilePath constructs full output file paths
	OutputFilePath(mediaSource, base, extension string) string
	// MediaOutputDir gets the directory for media output
	MediaOutputDir(mediaSource, base string) string
	// AudioBasePath gets the base path for audio files
	AudioBasePath(mediaSource string) string
}

// PathSanitizer sanitizes paths for different contexts
type PathSanitizer interface {
	// SanitizeForFileSystem sanitizes a path for the file system
	SanitizeForFileSystem(input string) string
	// SanitizeForFFmpeg sanitizes a path for FFmpeg commands
	SanitizeForFFmpeg(input string) string
}

// MediaInfoProvider provides media file information
type MediaInfoProvider interface {
	// GetMediaInfo gets media information for a file
	GetMediaInfo(filePath string) (MediaInfo, error)
}

// SubtitleProvider provides subtitle handling capabilities
type SubtitleProvider interface {
	// OpenFile opens a subtitle file
	OpenFile(path string, clean bool) (*Subtitles, error)
	// TrimCC2Dubs trims closed captions for dubbing
	TrimCC2Dubs(subs *Subtitles)
	// Subs2Dubs converts subtitles to dubbing format
	Subs2Dubs(subs *Subtitles, path, sep string) error
	// Write writes subtitles to a file
	Write(subs *Subtitles, path string) error
}

// Reporter provides crash reporting capabilities
type Reporter interface {
	// Record updates either global or execution scope information
	Record(update func(*crash.GlobalScope, *crash.ExecutionScope))
}

// Logger is a subset of zerolog.Logger needed for our interfaces
type Logger interface {
	Debug() LogEvent
	Info() LogEvent
	Warn() LogEvent
	Error() LogEvent
	Fatal() LogEvent
	Trace() LogEvent
}

// LogEvent represents a logging event
type LogEvent interface {
	Err(err error) LogEvent
	Str(key, val string) LogEvent
	Int(key string, val int) LogEvent
	Bool(key string, val bool) LogEvent
	Msg(msg string)
	Msgf(format string, v ...interface{})
}

// MessageHandlerEx extends MessageHandler with additional methods needed for testing
type MessageHandlerEx interface {
	MessageHandler

	// Added methods for compatibility with tests
	GetOutputFilePath() string
	UpdateProgress(barName string, progress, total int, description string)
}

// TaskInterface defines the required methods for worker pool processing
type TaskInterface interface {
	// ProcessItem processes a single subtitle item
	ProcessItem(ctx context.Context, indexedSub IndexedSubItem) (ProcessedItem, *ProcessingError)
}