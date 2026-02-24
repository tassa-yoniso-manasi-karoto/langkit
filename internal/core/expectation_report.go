package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var episodePattern = regexp.MustCompile(`(?i)(S\d+E\d+|E\d+|\d+x\d+)`)

// FormatReportCLI formats a ValidationReport as a human-readable string
// for CLI output.
func FormatReportCLI(report *ValidationReport) string {
	var b strings.Builder

	// Summary line
	b.WriteString(fmt.Sprintf("%d files checked", report.TotalFiles))
	if report.ErrorCount > 0 || report.WarningCount > 0 {
		b.WriteString(": ")
		parts := []string{}
		if report.ErrorCount > 0 {
			parts = append(parts, fmt.Sprintf("%d errors", report.ErrorCount))
		}
		if report.WarningCount > 0 {
			parts = append(parts, fmt.Sprintf("%d warnings", report.WarningCount))
		}
		b.WriteString(strings.Join(parts, ", "))
	} else {
		b.WriteString(" -- all clean")
	}
	b.WriteString(fmt.Sprintf(" (%.1fs)\n", report.Duration.Seconds()))

	// Consensus overview (auto mode)
	if len(report.Consensus) > 0 {
		b.WriteString("\n")
		dirs := make([]string, 0, len(report.Consensus))
		for d := range report.Consensus {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		for _, dir := range dirs {
			dc := report.Consensus[dir]
			if dc.FileCount == 0 {
				continue
			}
			b.WriteString("  Consensus for " + filepath.Base(dir) +
				" (" + itoa(dc.FileCount) + " files)")
			if dc.BonusExcluded > 0 {
				b.WriteString(", " + itoa(dc.BonusExcluded) + " bonus excluded")
			}
			b.WriteString(":\n")
			if len(dc.QuorumAudioLangs) > 0 {
				b.WriteString("    audio: [" + strings.Join(dc.QuorumAudioLangs, ", ") + "]\n")
			}
			if len(dc.QuorumSubLangs) > 0 {
				b.WriteString("    subtitles: [" + strings.Join(dc.QuorumSubLangs, ", ") + "]\n")
			}
			if dc.MedianDuration > 0 {
				b.WriteString("    median duration: " + formatDur(dc.MedianDuration) + "\n")
			}
			if dc.ConsensusAudioCount >= 0 {
				b.WriteString("    audio tracks: " + itoa(dc.ConsensusAudioCount) + "\n")
			}
		}
	}

	if len(report.Issues) == 0 {
		return b.String()
	}

	// Interpreted summaries
	summaries := GenerateInterpretedSummaries(report)
	if len(summaries) > 0 {
		b.WriteString("\n")
		for _, s := range summaries {
			b.WriteString("  " + s.Message + "\n")
		}
	}

	// Per-file detail (errors and warnings only)
	b.WriteString("\nDetails:\n")
	fileIssues := groupByFile(report.Issues)
	// Sort files for stable output
	filePaths := make([]string, 0, len(fileIssues))
	for fp := range fileIssues {
		filePaths = append(filePaths, fp)
	}
	sort.Strings(filePaths)

	for _, fp := range filePaths {
		issues := fileIssues[fp]
		hasNonInfo := false
		for _, iss := range issues {
			if iss.Severity != SeverityInfo {
				hasNonInfo = true
				break
			}
		}
		if !hasNonInfo {
			continue
		}
		b.WriteString(fmt.Sprintf("  %s\n", displayName(fp)))
		for _, iss := range issues {
			if iss.Severity == SeverityInfo {
				continue
			}
			b.WriteString(fmt.Sprintf("    [%s] %s\n", iss.Severity.String(), iss.Message))
		}
	}

	return b.String()
}

// FormatReportJSON formats a ValidationReport as JSON.
func FormatReportJSON(report *ValidationReport) ([]byte, error) {
	type jsonIssue struct {
		Severity string `json:"severity"`
		Source   string `json:"source"`
		FilePath string `json:"filePath"`
		Category string `json:"category"`
		Code     string `json:"code"`
		Message  string `json:"message"`
	}
	type jsonConsensus struct {
		Directory              string   `json:"directory"`
		FileCount              int      `json:"fileCount"`
		BonusExcluded          int      `json:"bonusExcluded,omitempty"`
		ConsensusAudioLangs    []string `json:"consensusAudioLangs,omitempty"`
		ConsensusSubLangs      []string `json:"consensusSubLangs,omitempty"`
		ConsensusAudioTrackCount int    `json:"consensusAudioTrackCount"`
		MedianDurationSec      float64  `json:"medianDurationSec"`
	}
	type jsonSummary struct {
		Source  string `json:"source"`
		Message string `json:"message"`
	}
	type jsonReport struct {
		RootPath             string          `json:"rootPath"`
		TotalFiles           int             `json:"totalFiles"`
		ErrorCount           int             `json:"errorCount"`
		WarningCount         int             `json:"warningCount"`
		InfoCount            int             `json:"infoCount"`
		DurationMs           int64           `json:"durationMs"`
		Issues               []jsonIssue     `json:"issues"`
		InterpretedSummaries []jsonSummary   `json:"interpretedSummaries"`
		Consensus            []jsonConsensus `json:"consensus,omitempty"`
	}

	coreSummaries := GenerateInterpretedSummaries(report)
	jsonSummaries := make([]jsonSummary, len(coreSummaries))
	for i, s := range coreSummaries {
		jsonSummaries[i] = jsonSummary{
			Source:  string(s.Source),
			Message: s.Message,
		}
	}

	jr := jsonReport{
		RootPath:             report.RootPath,
		TotalFiles:           report.TotalFiles,
		ErrorCount:           report.ErrorCount,
		WarningCount:         report.WarningCount,
		InfoCount:            report.InfoCount,
		DurationMs:           report.Duration.Milliseconds(),
		InterpretedSummaries: jsonSummaries,
	}

	for _, iss := range report.Issues {
		jr.Issues = append(jr.Issues, jsonIssue{
			Severity: iss.Severity.String(),
			Source:   string(iss.Source),
			FilePath: iss.FilePath,
			Category: iss.Category,
			Code:     iss.Code,
			Message:  iss.Message,
		})
	}

	// Add consensus summaries for auto mode
	if len(report.Consensus) > 0 {
		dirs := make([]string, 0, len(report.Consensus))
		for d := range report.Consensus {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		for _, dir := range dirs {
			dc := report.Consensus[dir]
			jr.Consensus = append(jr.Consensus, jsonConsensus{
				Directory:              dir,
				FileCount:              dc.FileCount,
				BonusExcluded:          dc.BonusExcluded,
				ConsensusAudioLangs:    dc.QuorumAudioLangs,
				ConsensusSubLangs:      dc.QuorumSubLangs,
				ConsensusAudioTrackCount: dc.ConsensusAudioCount,
				MedianDurationSec:      dc.MedianDuration,
			})
		}
	}

	return json.MarshalIndent(jr, "", "  ")
}

// InterpretedSummary is an aggregated human-readable sentence derived
// from raw findings, tagged with the source that produced it.
type InterpretedSummary struct {
	Source  IssueSource
	Message string
}

// sourceOrder returns a sort key so that profile comes first,
// structural second, auto third.
func sourceOrder(s IssueSource) int {
	switch s {
	case SourceProfile:
		return 0
	case SourceStructural:
		return 1
	case SourceAuto:
		return 2
	default:
		return 3
	}
}

// codeLabel maps issue codes to human-readable cluster labels.
func codeLabel(code string) string {
	switch code {
	case CodeMediainfoFailed:
		return "MediaInfo Failures"
	case CodeNoMediaFiles:
		return "No Media Files"
	case CodeNoVideoTrack:
		return "Missing Video Track"
	case CodeAudioDecodeFailed:
		return "Audio Decode Failures"
	case CodeVideoDecodeFailed:
		return "Video Decode Failures"
	case CodeCorruptTrack:
		return "Corrupt Audio Tracks"
	case CodeAudioDurationMismatch:
		return "Audio Duration Mismatches"
	case CodeExtAudioDuration:
		return "External Audio Duration Issues"
	case CodeDurationUnavailable:
		return "Duration Unavailable"
	case CodeMissingAudioLang:
		return "Missing Audio Languages"
	case CodeMissingSubLang:
		return "Missing Subtitle Languages"
	case CodeUntaggedTrack:
		return "Untagged Tracks"
	case CodeSubParseFailed:
		return "Subtitle Parse Failures"
	case CodeSubEmpty:
		return "Empty Subtitles"
	case CodeSubEncoding:
		return "Subtitle Encoding Issues"
	case CodeSubLowCoverage:
		return "Low Subtitle Coverage"
	case CodeSubOverlap:
		return "Subtitle Overlap"
	case CodeAutoMissingAudio:
		return "Missing Consensus Audio"
	case CodeAutoMissingSub:
		return "Missing Consensus Subtitles"
	case CodeAutoAudioCount:
		return "Audio Track Count Anomalies"
	case CodeAutoSubCount:
		return "Subtitle Count Anomalies"
	case CodeAutoDurationOutlier:
		return "Duration Outliers"
	case CodeAutoGroupTooSmall:
		return "Group Too Small for Auto Checks"
	default:
		return code
	}
}

// GenerateInterpretedSummaries produces aggregated human-readable
// sentences from the raw findings, grouped by issue code and source.
func GenerateInterpretedSummaries(report *ValidationReport) []InterpretedSummary {
	var summaries []InterpretedSummary

	// Group issues by (code, source)
	type aggregateKey struct {
		code   string
		source IssueSource
	}
	type aggregateEntry struct {
		paths    []string
		issues   []Issue
		severity Severity // worst severity in the group
	}
	groups := make(map[aggregateKey]*aggregateEntry)

	for _, iss := range report.Issues {
		if iss.Severity == SeverityInfo {
			continue
		}
		key := aggregateKey{code: iss.Code, source: iss.Source}
		entry := groups[key]
		if entry == nil {
			entry = &aggregateEntry{severity: iss.Severity}
			groups[key] = entry
		}
		entry.paths = append(entry.paths, iss.FilePath)
		entry.issues = append(entry.issues, iss)
		if iss.Severity < entry.severity { // lower = more severe
			entry.severity = iss.Severity
		}
	}

	// Count files per directory for directory-level aggregation
	dirFileCount := make(map[string]int)
	for _, fr := range report.FileResults {
		dir := filepath.Dir(fr.VideoFile)
		dirFileCount[dir]++
	}

	for key, entry := range groups {
		if len(entry.paths) == 0 {
			continue
		}

		label := codeLabel(key.code)

		// Directory-level aggregation for language issues
		if key.code == CodeMissingAudioLang || key.code == CodeMissingSubLang ||
			key.code == CodeAutoMissingAudio || key.code == CodeAutoMissingSub {
			dirCounts := make(map[string]int)
			for _, p := range entry.paths {
				dirCounts[filepath.Dir(p)]++
			}
			emittedDirSummary := false
			for dir, count := range dirCounts {
				if total, ok := dirFileCount[dir]; ok && count >= total && total > 1 {
					// Use representative message from this group
					summaries = append(summaries, InterpretedSummary{
						Source:  key.source,
						Message: "Directory " + filepath.Base(dir) + ": " + entry.issues[0].Message,
					})
					emittedDirSummary = true
				}
			}
			if emittedDirSummary {
				continue
			}
		}

		// Deduplicate file paths (same file can have multiple issues
		// with the same code, e.g. multiple untagged tracks)
		uniquePaths := dedup(entry.paths)
		n := len(uniquePaths)

		var msg string
		if n > 3 {
			msg = label + " (" + itoa(n) + " files)"
		} else {
			names := make([]string, n)
			for i, p := range uniquePaths {
				names[i] = displayName(p)
			}
			msg = label + ": " + strings.Join(names, ", ")
		}
		summaries = append(summaries, InterpretedSummary{
			Source:  key.source,
			Message: msg,
		})
	}

	// Sort by source (profile → structural → auto), then by message
	sort.Slice(summaries, func(i, j int) bool {
		oi, oj := sourceOrder(summaries[i].Source), sourceOrder(summaries[j].Source)
		if oi != oj {
			return oi < oj
		}
		return summaries[i].Message < summaries[j].Message
	})
	return summaries
}

// dedup returns unique strings from a slice, preserving order.
func dedup(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func groupByFile(issues []Issue) map[string][]Issue {
	m := make(map[string][]Issue)
	for _, iss := range issues {
		m[iss.FilePath] = append(m[iss.FilePath], iss)
	}
	return m
}

// displayName returns a short name for a file path: episode ID if
// detectable, otherwise the base name without extension.
func displayName(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if m := episodePattern.FindString(name); m != "" {
		return m
	}
	return name
}
