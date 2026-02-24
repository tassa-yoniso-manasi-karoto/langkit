package core

import (
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/rs/zerolog"
)

// Bonus content exclusion (PRD 5.3.3)
//
// All tokens use word-boundary matching: they must be delimited by
// separators (. - _ [ ] ( ) space) or string start/end.
// This prevents false exclusions from legitimate titles containing
// words like "special" or "extra" as substrings.

var bonusTokens = []string{
	// Short tokens (originally required boundaries, now all do)
	"sp", "pv", "ova", "oad", "nced", "ncop", "menu",
	// Long tokens (also boundary-matched to avoid false positives
	// on titles like "A Special Day" or "Disappearance")
	"extra", "bonus", "trailer", "special", "preview",
}

// bonusPattern matches bonus tokens at word boundaries.
// Boundaries are: string start/end, or separators (. - _ [ ] ( ) space).
var bonusPattern *regexp.Regexp

func init() {
	escaped := make([]string, len(bonusTokens))
	for i, t := range bonusTokens {
		escaped[i] = regexp.QuoteMeta(strings.ToUpper(t))
	}
	pattern := `(?i)(?:^|[.\-_ \[\]()])(?:` +
		strings.Join(escaped, "|") +
		`)(?:$|[.\-_ \[\]()])`
	bonusPattern = regexp.MustCompile(pattern)
}

// isBonusContent returns true if the filename matches common non-episode
// patterns (extras, trailers, specials, etc.). Only the base name
// (without extension) is tested.
func isBonusContent(filename string) bool {
	name := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	return bonusPattern.MatchString(name)
}

// isAutoEligible returns true if a file should participate in consensus
// building and auto consistency checks. Excludes bonus content, files
// with mediainfo failures, and corrupted files.
func isAutoEligible(fp string, r *FileCheckResult) bool {
	if isBonusContent(fp) {
		return false
	}
	if r == nil || r.MediaInfoErr != nil {
		return false
	}
	if r.DecodeCorrupted {
		return false
	}
	return true
}

// buildConsensus computes the DirectoryConsensus for a group of files.
// Only files passing isAutoEligible participate. Bonus content is
// counted separately in BonusExcluded.
func buildConsensus(dir string, filePaths []string, results map[string]*FileCheckResult, config *AutoCheckConfig, log zerolog.Logger) *DirectoryConsensus {
	dc := &DirectoryConsensus{
		Directory:        dir,
		AudioLangs:       make(map[string]int),
		SubtitleLangs:    make(map[string]int),
		AudioTrackCounts: make(map[int]int),
		SubCountPerLang:  make(map[string]map[int]int),
		ConsensusAudioCount: -1,
	}

	// Filter eligible files
	var eligible []string
	for _, fp := range filePaths {
		if isBonusContent(fp) {
			dc.BonusExcluded++
			continue
		}
		r := results[fp]
		if !isAutoEligible(fp, r) {
			continue
		}
		eligible = append(eligible, fp)
	}
	dc.FileCount = len(eligible)

	if dc.FileCount == 0 {
		return dc
	}

	for _, fp := range eligible {
		r := results[fp]

		// Audio languages (exclude "und" from consensus per PRD 6.5.1)
		audioLangSet := make(map[string]bool)
		for _, at := range r.MediaInfo.AudioTracks {
			code := langCode(at.Language)
			if code != "und" {
				audioLangSet[code] = true
			}
		}
		for code := range audioLangSet {
			dc.AudioLangs[code]++
		}

		// Audio track count
		dc.AudioTrackCounts[len(r.MediaInfo.AudioTracks)]++

		// Subtitle languages (both embedded and standalone via SubCandidates)
		subLangSet := make(map[string]bool)
		for _, sc := range r.SubCandidates {
			code := langCode(sc.Lang)
			if code != "und" {
				subLangSet[code] = true
			}
		}
		for code := range subLangSet {
			dc.SubtitleLangs[code]++
		}

		// Subtitle source count per language (SubCandidates only --
		// single source of truth, already includes embedded + standalone)
		subCountByLang := make(map[string]int)
		for _, sc := range r.SubCandidates {
			code := langCode(sc.Lang)
			if code != "und" {
				subCountByLang[code]++
			}
		}
		for lang, count := range subCountByLang {
			if dc.SubCountPerLang[lang] == nil {
				dc.SubCountPerLang[lang] = make(map[int]int)
			}
			dc.SubCountPerLang[lang][count]++
		}

		// Duration
		if r.VideoDuration > 0 {
			dc.Durations = append(dc.Durations, r.VideoDuration)
		}
	}

	// Classify languages into quorum/soft-floor tiers
	n := float64(dc.FileCount)
	quorumThreshold := config.QuorumPct / 100.0
	softFloorThreshold := config.SoftFloorPct / 100.0

	for lang, count := range dc.AudioLangs {
		confidence := float64(count) / n
		if confidence >= quorumThreshold {
			dc.QuorumAudioLangs = append(dc.QuorumAudioLangs, lang)
		} else if confidence >= softFloorThreshold {
			dc.SoftAudioLangs = append(dc.SoftAudioLangs, lang)
		}
	}
	sort.Strings(dc.QuorumAudioLangs)
	sort.Strings(dc.SoftAudioLangs)

	for lang, count := range dc.SubtitleLangs {
		confidence := float64(count) / n
		if confidence >= quorumThreshold {
			dc.QuorumSubLangs = append(dc.QuorumSubLangs, lang)
		} else if confidence >= softFloorThreshold {
			dc.SoftSubLangs = append(dc.SoftSubLangs, lang)
		}
	}
	sort.Strings(dc.QuorumSubLangs)
	sort.Strings(dc.SoftSubLangs)

	// Audio track count mode (PRD 6.5.4 mode tie handling)
	dc.ConsensusAudioCount = computeMode(dc.AudioTrackCounts)

	// Median duration (sort a copy -- median() requires sorted input)
	if len(dc.Durations) > 0 {
		sorted := make([]float64, len(dc.Durations))
		copy(sorted, dc.Durations)
		sort.Float64s(sorted)
		dc.MedianDuration = median(sorted)
	}

	log.Debug().
		Str("dir", filepath.Base(dir)).
		Int("eligibleCount", dc.FileCount).
		Int("bonusExcluded", dc.BonusExcluded).
		Strs("quorumAudioLangs", dc.QuorumAudioLangs).
		Strs("quorumSubLangs", dc.QuorumSubLangs).
		Int("consensusAudioCount", dc.ConsensusAudioCount).
		Float64("medianDuration", dc.MedianDuration).
		Msg("Built consensus")

	return dc
}

// runAutoChecks runs all consistency checks (PRD 6.5.1-6.5.5) for
// files in a single directory group against the computed consensus.
func runAutoChecks(report *ValidationReport, dc *DirectoryConsensus, filePaths []string, results map[string]*FileCheckResult, config *AutoCheckConfig, log zerolog.Logger) {
	issuesBefore := len(report.Issues)
	n := dc.FileCount

	for _, fp := range filePaths {
		r := results[fp]
		if !isAutoEligible(fp, r) {
			continue
		}

		// 6.5.1 Audio language consensus
		audioLangSet := make(map[string]bool)
		for _, at := range r.MediaInfo.AudioTracks {
			code := langCode(at.Language)
			if code != "und" {
				audioLangSet[code] = true
			}
		}
		checkLangConsensus(report, fp, audioLangSet, dc.AudioLangs, n,
			dc.QuorumAudioLangs, dc.SoftAudioLangs, "audio", config)

		// 6.5.2 Subtitle language consensus (SubCandidates as single source)
		subLangSet := make(map[string]bool)
		for _, sc := range r.SubCandidates {
			code := langCode(sc.Lang)
			if code != "und" {
				subLangSet[code] = true
			}
		}
		checkLangConsensus(report, fp, subLangSet, dc.SubtitleLangs, n,
			dc.QuorumSubLangs, dc.SoftSubLangs, "subtitle", config)

		// 6.5.4 Audio track count consistency
		if dc.ConsensusAudioCount >= 0 { // -1 means mode tie → skip
			trackCount := len(r.MediaInfo.AudioTracks)
			if trackCount != dc.ConsensusAudioCount {
				support := dc.AudioTrackCounts[dc.ConsensusAudioCount]
				confidence := float64(support) / float64(n)
				if confidence >= config.QuorumPct/100.0 {
					report.AddIssue(Issue{
						Severity: SeverityInfo,
						Source:   SourceAuto,
						FilePath: fp,
						Category: "consistency",
						Message: "has " + itoa(trackCount) +
							" audio tracks (most files have " +
							itoa(dc.ConsensusAudioCount) + ")",
					})
				}
			}
		}

		// 6.5.5 Subtitle count per language consistency
		// (SubCandidates only -- single source of truth)
		subCountByLang := make(map[string]int)
		for _, sc := range r.SubCandidates {
			code := langCode(sc.Lang)
			if code != "und" {
				subCountByLang[code]++
			}
		}
		for _, lang := range dc.QuorumSubLangs {
			countDist := dc.SubCountPerLang[lang]
			if countDist == nil {
				continue
			}
			modeCount := computeMode(countDist)
			if modeCount < 0 { // tie → skip
				continue
			}
			fileCount := subCountByLang[lang]
			if fileCount != modeCount {
				report.AddIssue(Issue{
					Severity: SeverityInfo,
					Source:   SourceAuto,
					FilePath: fp,
					Category: "consistency",
					Message: "has " + itoa(fileCount) + " " + lang +
						" subtitle source(s) (most files have " +
						itoa(modeCount) + ")",
				})
			}
		}
	}

	// 6.5.3 Duration outlier detection (requires n >= 6)
	if len(dc.Durations) >= 6 {
		checkDurationOutliers(report, dc, filePaths, results, log)
	}

	log.Debug().
		Str("dir", filepath.Base(dc.Directory)).
		Int("issuesEmitted", len(report.Issues)-issuesBefore).
		Msg("Auto checks complete for directory")
}

// checkLangConsensus checks a single file's language set against the
// consensus, emitting findings based on confidence tier.
// Messages are file-agnostic to allow aggregation in interpreted summaries.
func checkLangConsensus(report *ValidationReport, fp string, fileLangs map[string]bool, allLangCounts map[string]int, n int, quorumLangs, softLangs []string, trackType string, config *AutoCheckConfig) {
	// Check quorum-tier languages (missing → Warning)
	for _, lang := range quorumLangs {
		if !fileLangs[lang] {
			support := allLangCounts[lang]
			report.AddIssue(Issue{
				Severity: SeverityWarning,
				Source:   SourceAuto,
				FilePath: fp,
				Category: "consistency",
				Message: "missing " + lang + " " + trackType +
					" (present in " + itoa(support) +
					" of " + itoa(n) + " files)",
			})
		}
	}

	// Check soft-floor-tier languages (missing → Info)
	for _, lang := range softLangs {
		if !fileLangs[lang] {
			support := allLangCounts[lang]
			report.AddIssue(Issue{
				Severity: SeverityInfo,
				Source:   SourceAuto,
				FilePath: fp,
				Category: "consistency",
				Message: "missing " + lang + " " + trackType +
					" (present in " + itoa(support) +
					" of " + itoa(n) + " files)",
			})
		}
	}
}

// checkDurationOutliers uses Tukey fences to detect duration anomalies.
// Only called when n >= 6 (per PRD 6.5.3).
func checkDurationOutliers(report *ValidationReport, dc *DirectoryConsensus, filePaths []string, results map[string]*FileCheckResult, log zerolog.Logger) {
	sorted := make([]float64, len(dc.Durations))
	copy(sorted, dc.Durations)
	sort.Float64s(sorted)

	q1 := percentile(sorted, 25)
	q3 := percentile(sorted, 75)
	iqr := q3 - q1

	var lowerFence, upperFence float64
	if iqr == 0 {
		// IQR=0 fallback: use absolute + percentage floor
		absFloor := 120.0 // 2 minutes
		pctFloor := 0.05  // 5%
		threshold := math.Max(absFloor, pctFloor*dc.MedianDuration)
		lowerFence = dc.MedianDuration - threshold
		upperFence = dc.MedianDuration + threshold
	} else {
		lowerFence = q1 - 1.5*iqr
		upperFence = q3 + 1.5*iqr
	}

	log.Debug().
		Float64("q1", q1).
		Float64("q3", q3).
		Float64("iqr", iqr).
		Float64("lowerFence", lowerFence).
		Float64("upperFence", upperFence).
		Msg("Duration outlier fences")

	for _, fp := range filePaths {
		r := results[fp]
		if !isAutoEligible(fp, r) || r.VideoDuration <= 0 {
			continue
		}

		dur := r.VideoDuration
		if dur < lowerFence || dur > upperFence {
			var direction string
			if dur < dc.MedianDuration {
				direction = "short"
			} else {
				direction = "long"
			}
			report.AddIssue(Issue{
				Severity: SeverityWarning,
				Source:   SourceAuto,
				FilePath: fp,
				Category: "consistency",
				Message: "duration (" + formatDur(dur) +
					") is unusually " + direction +
					" compared to siblings (median: " +
					formatDur(dc.MedianDuration) + ")",
			})
		}
	}
}

// langCode returns a stable language code string for consensus counting.
// Uses Part3 (ISO 639-3) when available, falls back to "und".
func langCode(l Lang) string {
	if l.Language == nil {
		return "und"
	}
	if l.Part3 != "" {
		return l.Part3
	}
	return "und"
}

// computeMode returns the mode (most common value) from a frequency map.
// Returns -1 if there is a tie for the highest count (PRD mode-tie handling).
func computeMode(counts map[int]int) int {
	if len(counts) == 0 {
		return -1
	}

	maxCount := 0
	modeVal := -1
	tied := false

	for val, count := range counts {
		if count > maxCount {
			maxCount = count
			modeVal = val
			tied = false
		} else if count == maxCount {
			tied = true
		}
	}

	if tied {
		return -1
	}
	return modeVal
}

// median returns the median of a sorted slice.
func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2.0
	}
	return sorted[n/2]
}

// percentile returns the p-th percentile from a sorted slice using
// linear interpolation.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}

	rank := p / 100.0 * float64(n-1)
	lower := int(math.Floor(rank))
	upper := lower + 1
	if upper >= n {
		return sorted[n-1]
	}
	frac := rank - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

// formatDur formats seconds into a human-readable string.
func formatDur(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return itoa(h) + "h " + itoa(m) + "m " + itoa(s) + "s"
	}
	return itoa(m) + "m " + itoa(s) + "s"
}
