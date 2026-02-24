package core

import (
	"time"

	"github.com/rs/zerolog"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
)

// ExpectationProfile defines what the user expects from their media library.
type ExpectationProfile struct {
	Name                    string   `yaml:"name" json:"name"`
	ExpectedAudioLangs      []string `yaml:"expectedAudioLangs" json:"expectedAudioLangs"`
	ExpectedSubtitleLangs   []string `yaml:"expectedSubtitleLangs" json:"expectedSubtitleLangs"`
	RequireVideoTrack       bool     `yaml:"requireVideoTrack" json:"requireVideoTrack"`
	RequireLanguageTags     bool     `yaml:"requireLanguageTags" json:"requireLanguageTags"`
	DurationTolerancePct    float64  `yaml:"durationTolerancePercent" json:"durationTolerancePercent"`
	CheckExternalAudio      bool     `yaml:"checkExternalAudioFiles" json:"checkExternalAudioFiles"`
	VideoExtensions         []string `yaml:"videoExtensions" json:"videoExtensions"`
}

// DefaultProfile returns a profile with sensible defaults.
func DefaultProfile() ExpectationProfile {
	return ExpectationProfile{
		RequireVideoTrack:    true,
		RequireLanguageTags:  true,
		DurationTolerancePct: 2.0,
	}
}

// Severity levels for validation findings.
type Severity int

const (
	SeverityError   Severity = iota // Will certainly cause processing failure
	SeverityWarning                 // Might indicate a problem
	SeverityInfo                    // Informational observation
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	default:
		return "unknown"
	}
}

// IssueSource distinguishes profile-based from auto-detected findings.
type IssueSource string

const (
	SourceProfile    IssueSource = "profile"
	SourceAuto       IssueSource = "auto"
	SourceStructural IssueSource = "structural"
)

// Issue codes — stable, machine-readable keys for grouping findings
// by problem type regardless of per-file message variations.
const (
	// Structural
	CodeMediainfoFailed       = "mediainfo_failed"
	CodeNoMediaFiles          = "no_media_files"
	CodeNoVideoTrack          = "no_video_track"
	CodeAudioDecodeFailed     = "audio_decode_failed"
	CodeVideoDecodeFailed     = "video_decode_failed"
	CodeCorruptTrack          = "corrupt_track"          // merged decode+duration
	CodeAudioDurationMismatch = "audio_duration_mismatch"
	CodeExtAudioDuration      = "ext_audio_duration"
	CodeDurationUnavailable   = "duration_unavailable"
	// Language
	CodeMissingAudioLang = "missing_audio_lang"
	CodeMissingSubLang   = "missing_sub_lang"
	CodeUntaggedTrack    = "untagged_track"
	// Subtitle
	CodeSubParseFailed = "sub_parse_failed"
	CodeSubEmpty       = "sub_empty"
	CodeSubEncoding    = "sub_encoding"
	CodeSubLowCoverage = "sub_low_coverage"
	CodeSubOverlap     = "sub_overlap"
	// Auto / consistency
	CodeAutoMissingAudio    = "auto_missing_audio"
	CodeAutoMissingSub      = "auto_missing_sub"
	CodeAutoAudioCount      = "auto_audio_count"
	CodeAutoSubCount        = "auto_sub_count"
	CodeAutoDurationOutlier = "auto_duration_outlier"
	CodeAutoGroupTooSmall   = "auto_group_too_small"
)

// Issue represents a single validation finding.
type Issue struct {
	Severity Severity
	Source   IssueSource
	FilePath string
	Category string // "integrity", "language", "duration", "subtitle", "structure", "consistency"
	Code     string // stable grouping key (one of the Code* constants)
	Message  string
}

// AutoCheckConfig controls auto mode (consistency detection) behavior.
type AutoCheckConfig struct {
	Enabled      bool    `yaml:"enabled" json:"enabled"`
	QuorumPct    float64 `yaml:"quorumPct" json:"quorumPct"`         // default: 75.0
	SoftFloorPct float64 `yaml:"softFloorPct" json:"softFloorPct"`   // default: 20.0
	MinGroupSize int     `yaml:"minGroupSize" json:"minGroupSize"`   // default: 3
}

// DefaultAutoConfig returns an AutoCheckConfig with sensible defaults.
func DefaultAutoConfig() AutoCheckConfig {
	return AutoCheckConfig{
		Enabled:      true,
		QuorumPct:    75.0,
		SoftFloorPct: 20.0,
		MinGroupSize: 3,
	}
}

// DirectoryConsensus captures the inferred "norm" for one directory group.
type DirectoryConsensus struct {
	Directory       string
	FileCount       int
	BonusExcluded   int              // files excluded by bonus content heuristic
	AudioLangs      map[string]int   // lang code → file count
	SubtitleLangs   map[string]int   // lang code → file count
	AudioTrackCounts map[int]int     // track count → file count
	SubCountPerLang map[string]map[int]int // lang → sub source count → file count
	Durations       []float64        // video durations in seconds (one per eligible file)

	// Derived tiered language sets
	QuorumAudioLangs []string // confidence >= quorumPct
	SoftAudioLangs   []string // softFloor <= confidence < quorum
	QuorumSubLangs   []string // confidence >= quorumPct
	SoftSubLangs     []string // softFloor <= confidence < quorum

	ConsensusAudioCount int     // mode of audio track counts (-1 if tied)
	MedianDuration      float64 // median video duration in seconds
}

// ValidationReport is the complete output of a check run.
type ValidationReport struct {
	Profile      *ExpectationProfile
	AutoConfig   *AutoCheckConfig
	RootPath     string
	TotalFiles   int
	Issues       []Issue
	ErrorCount   int
	WarningCount int
	InfoCount    int
	Duration     time.Duration
	FileResults  map[string]*FileCheckResult
	Consensus    map[string]*DirectoryConsensus // dir path → consensus
}

// FileCheckResult stores all metadata gathered for a single media file.
type FileCheckResult struct {
	VideoFile       string
	MediaInfo       MediaInfo
	MediaInfoErr    error // non-nil if Mediainfo() failed
	SubCandidates   []SubtitleCandidate
	SubCheckResults []SubCheckResult // subset of SubCandidates; CandidateIdx maps back
	DecodeResults   []media.DecodeCheckResult
	DecodeCorrupted bool
	VideoDuration   float64 // seconds, 0 if unavailable
	AudioDurations  []float64
	ExternalAudio   []ExternalAudioFile
}

// ExternalAudioFile represents an audio file found alongside a video.
type ExternalAudioFile struct {
	Path     string
	Duration float64 // seconds, 0 if probe failed
}

// SubCheckResult stores the parse results for a single subtitle candidate.
// Only standalone subtitles are parsed; embedded tracks are intentionally
// excluded from parse-level checks to keep the preflight fast (extraction
// + parsing would be expensive and is not needed for structural validation).
type SubCheckResult struct {
	CandidateIdx int    // index into SubCandidates
	FilePath     string // resolved file path on disk
	LineCount    int    // number of parsed items (0 if parse failed)
	ParseErr     string // non-empty if subs.OpenFile() failed
	Parsed       bool   // true if parse was attempted
	EncodingIssue bool  // true if high proportion of U+FFFD or null bytes
	TailEndSec   float64 // median end-time of last k cues (robust tail coverage)
}

// AddIssue appends an issue and updates the report's severity counts.
func (r *ValidationReport) AddIssue(issue Issue) {
	r.Issues = append(r.Issues, issue)
	switch issue.Severity {
	case SeverityError:
		r.ErrorCount++
	case SeverityWarning:
		r.WarningCount++
	case SeverityInfo:
		r.InfoCount++
	}
}

// HasErrors returns true if the report contains any Error-severity issues.
func (r *ValidationReport) HasErrors() bool {
	return r.ErrorCount > 0
}

// CheckCallbacks carries optional logger and progress reporting into RunCheck.
type CheckCallbacks struct {
	Logger     zerolog.Logger
	OnProgress func(barID string, increment, total int, label string)
}
