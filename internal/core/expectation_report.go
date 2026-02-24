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

// GenerateInterpretedSummaries produces aggregated human-readable
// sentences from the raw findings, tagged by source.
func GenerateInterpretedSummaries(report *ValidationReport) []InterpretedSummary {
	var summaries []InterpretedSummary

	// Group issues by (message, source) → list of full file paths
	type aggregateKey struct {
		category string
		message  string
		source   IssueSource
	}
	pathsByKey := make(map[aggregateKey][]string)

	for _, iss := range report.Issues {
		if iss.Severity == SeverityInfo {
			continue
		}
		key := aggregateKey{
			category: iss.Category,
			message:  iss.Message,
			source:   iss.Source,
		}
		pathsByKey[key] = append(pathsByKey[key], iss.FilePath)
	}

	// Count files per directory for directory-level detection
	dirFileCount := make(map[string]int)
	for _, fr := range report.FileResults {
		dir := filepath.Dir(fr.VideoFile)
		dirFileCount[dir]++
	}

	for key, paths := range pathsByKey {
		if len(paths) == 0 {
			continue
		}

		// Check if all files in a specific directory share this issue
		dirCounts := make(map[string]int)
		for _, p := range paths {
			dirCounts[filepath.Dir(p)]++
		}

		emittedDirSummary := false
		if key.category == "language" {
			for dir, count := range dirCounts {
				if total, ok := dirFileCount[dir]; ok && count >= total && total > 1 {
					summaries = append(summaries, InterpretedSummary{
						Source:  key.source,
						Message: fmt.Sprintf("Directory %s: %s", filepath.Base(dir), key.message),
					})
					emittedDirSummary = true
				}
			}
		}

		if emittedDirSummary {
			continue
		}

		// Convert to display names for output
		names := make([]string, len(paths))
		for i, p := range paths {
			names[i] = displayName(p)
		}

		var msg string
		if len(names) > 3 {
			msg = fmt.Sprintf("%d files: %s", len(names), key.message)
		} else if len(names) > 1 {
			msg = fmt.Sprintf("%s: %s", strings.Join(names, ", "), key.message)
		} else {
			msg = fmt.Sprintf("%s: %s", names[0], key.message)
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
