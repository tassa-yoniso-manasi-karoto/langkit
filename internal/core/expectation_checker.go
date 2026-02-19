package core

import (
	"context"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
)

// RunCheck performs the expectation check on the given path.
// If profile is nil, only structural checks (integrity) run.
// If autoConfig is non-nil and Enabled, auto consistency checks run.
// Both can be combined. The function respects context cancellation.
func RunCheck(ctx context.Context, rootPath string, profile *ExpectationProfile, autoConfig *AutoCheckConfig) (*ValidationReport, error) {
	start := time.Now()

	report := &ValidationReport{
		Profile:     profile,
		AutoConfig:  autoConfig,
		RootPath:    rootPath,
		FileResults: make(map[string]*FileCheckResult),
		Consensus:   make(map[string]*DirectoryConsensus),
	}

	// Determine extensions
	var extensions []string
	if profile != nil && len(profile.VideoExtensions) > 0 {
		extensions = profile.VideoExtensions
	}

	files, err := DiscoverMediaFiles(rootPath, extensions)
	if err != nil {
		return nil, err
	}
	report.TotalFiles = len(files)

	if len(files) == 0 {
		report.AddIssue(Issue{
			Severity: SeverityInfo,
			Source:   SourceStructural,
			FilePath: rootPath,
			Category: "structure",
			Message:  "No media files found",
		})
		report.Duration = time.Since(start)
		return report, nil
	}

	// Parse expected languages if profile is given
	var expectedAudioLangs, expectedSubLangs []Lang
	if profile != nil {
		if len(profile.ExpectedAudioLangs) > 0 {
			expectedAudioLangs, err = ParseLanguageTags(profile.ExpectedAudioLangs)
			if err != nil {
				return nil, err
			}
		}
		if len(profile.ExpectedSubtitleLangs) > 0 {
			expectedSubLangs, err = ParseLanguageTags(profile.ExpectedSubtitleLangs)
			if err != nil {
				return nil, err
			}
		}
	}

	// Probe pass: gather metadata for every file
	for _, filePath := range files {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		checkExtAudio := profile != nil && profile.CheckExternalAudio
		result := probeFile(filePath, checkExtAudio)
		report.FileResults[filePath] = result

		// Structural checks (always run)
		checkIntegrityResult(report, filePath, result)

		// If mediainfo failed, emit a structural issue and skip
		// metadata-dependent checks for this file
		if result.MediaInfoErr != nil {
			report.AddIssue(Issue{
				Severity: SeverityError,
				Source:   SourceStructural,
				FilePath: filePath,
				Category: "structure",
				Message:  "Could not read media metadata: " + result.MediaInfoErr.Error(),
			})
			continue
		}

		if profile != nil {
			checkVideoTrack(report, filePath, result, profile)
		}
	}

	// Profile checks (only on files with successful mediainfo)
	if profile != nil {
		for _, filePath := range files {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			result := report.FileResults[filePath]
			if result.MediaInfoErr != nil {
				continue
			}

			checkAudioLanguages(report, filePath, result, expectedAudioLangs)
			checkSubtitleLanguages(report, filePath, result, expectedSubLangs)
			checkLanguageTags(report, filePath, result, profile, expectedAudioLangs, expectedSubLangs)
			checkDurationConsistency(report, filePath, result, profile)
			checkExternalAudioDuration(report, filePath, result, profile)
			checkSubtitleIntegrity(report, filePath, result)
			checkEmbeddedStandaloneOverlap(report, filePath, result)
		}
	}

	// Auto checks: group files by directory and run consistency checks
	if autoConfig != nil && autoConfig.Enabled {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		runAutoMode(report, files, autoConfig)
	}

	report.Duration = time.Since(start)
	return report, nil
}

// runAutoMode groups files by immediate parent directory and runs
// consistency checks for each group meeting the minimum size.
func runAutoMode(report *ValidationReport, files []string, config *AutoCheckConfig) {
	// Group files by immediate parent directory
	dirFiles := make(map[string][]string)
	for _, fp := range files {
		dir := filepath.Dir(fp)
		dirFiles[dir] = append(dirFiles[dir], fp)
	}

	for dir, fps := range dirFiles {
		// Build consensus for this directory
		dc := buildConsensus(dir, fps, report.FileResults, config)
		report.Consensus[dir] = dc

		if dc.FileCount < config.MinGroupSize {
			report.AddIssue(Issue{
				Severity: SeverityInfo,
				Source:   SourceAuto,
				FilePath: dir,
				Category: "consistency",
				Message: "Skipped auto-check for " + filepath.Base(dir) +
					" (" + itoa(dc.FileCount) + " eligible files, minimum is " +
					itoa(config.MinGroupSize) + ")",
			})
			continue
		}

		runAutoChecks(report, dc, fps, report.FileResults, config)
	}
}

// probeFile gathers all metadata for a single media file.
// If checkExternalAudio is true, also discovers and probes sidecar audio files.
func probeFile(filePath string, checkExternalAudio bool) *FileCheckResult {
	result := &FileCheckResult{
		VideoFile: filePath,
	}

	// Integrity check
	isCorrupted, err := media.CheckValidData(filePath)
	result.Integrity = !isCorrupted
	result.IntegrityErr = err

	// Mediainfo
	mi, err := Mediainfo(filePath)
	if err != nil {
		result.MediaInfoErr = err
		return result
	}
	result.MediaInfo = mi

	// Parse video duration
	if dur, ok := media.ParseMediaInfoDuration(mi.VideoTrack.Duration); ok {
		result.VideoDuration = dur
	}

	// Parse audio durations
	for _, at := range mi.AudioTracks {
		if dur, ok := media.ParseMediaInfoDuration(at.Duration); ok {
			result.AudioDurations = append(result.AudioDurations, dur)
		} else {
			result.AudioDurations = append(result.AudioDurations, 0)
		}
	}

	// Collect subtitle candidates (retain unknown-lang for completeness)
	result.SubCandidates = CollectAllSubs(filePath, mi, true)

	// Parse standalone subtitle candidates for integrity checking
	for i, sc := range result.SubCandidates {
		if sc.Source.Type != SubSourceStandalone {
			continue
		}
		scr := probeSubtitle(i, sc.Source.FilePath)
		result.SubCheckResults = append(result.SubCheckResults, scr)
	}

	// Discover external audio files alongside the video
	if checkExternalAudio {
		result.ExternalAudio = CollectExternalAudio(filePath)
	}

	return result
}

// probeSubtitle parses a standalone subtitle file and checks for issues.
func probeSubtitle(candidateIdx int, filePath string) SubCheckResult {
	scr := SubCheckResult{
		CandidateIdx: candidateIdx,
		FilePath:     filePath,
		Parsed:       true,
	}

	parsed, err := subs.OpenFile(filePath, false)
	if err != nil {
		scr.ParseErr = err.Error()
		return scr
	}

	n := len(parsed.Items)
	scr.LineCount = n

	// Compute robust tail end: median of last k cue end times.
	// Using a sample of last cues avoids sensitivity to a single
	// outlier cue (e.g. a late credit or sign that extends past the
	// actual dialogue).
	if n > 0 {
		k := 7
		if k > n {
			k = n
		}
		endTimes := make([]float64, k)
		for i := 0; i < k; i++ {
			endTimes[i] = parsed.Items[n-k+i].EndAt.Seconds()
		}
		sort.Float64s(endTimes)
		scr.TailEndSec = median(endTimes)
	}

	// Encoding sanity: check for high proportion of U+FFFD or null bytes
	totalChars := 0
	badChars := 0
	for _, item := range parsed.Items {
		for _, line := range item.Lines {
			for _, li := range line.Items {
				for _, r := range li.Text {
					totalChars++
					if r == '\uFFFD' || r == 0 {
						badChars++
					}
				}
			}
		}
	}
	if totalChars > 0 && float64(badChars)/float64(totalChars) > 0.02 {
		scr.EncodingIssue = true
	}

	return scr
}

func checkIntegrityResult(report *ValidationReport, filePath string, result *FileCheckResult) {
	if !result.Integrity {
		report.AddIssue(Issue{
			Severity: SeverityError,
			Source:   SourceStructural,
			FilePath: filePath,
			Category: "integrity",
			Message:  "Video file is corrupted or malformed",
		})
	} else if result.IntegrityErr != nil {
		report.AddIssue(Issue{
			Severity: SeverityWarning,
			Source:   SourceStructural,
			FilePath: filePath,
			Category: "integrity",
			Message:  "Could not fully verify integrity: " + result.IntegrityErr.Error(),
		})
	}
}

func checkVideoTrack(report *ValidationReport, filePath string, result *FileCheckResult, profile *ExpectationProfile) {
	if !profile.RequireVideoTrack {
		return
	}
	if result.MediaInfo.VideoTrack.Type == "" {
		report.AddIssue(Issue{
			Severity: SeverityError,
			Source:   SourceProfile,
			FilePath: filePath,
			Category: "structure",
			Message:  "No video track found",
		})
	}
}

// langMatchesExpected checks whether a track's language matches an
// expected language using getIdx() for script/subtag-aware matching.
func langMatchesExpected(track, expected Lang) bool {
	_, found := getIdx([]Lang{expected}, track)
	return found
}

func checkAudioLanguages(report *ValidationReport, filePath string, result *FileCheckResult, expected []Lang) {
	if len(expected) == 0 {
		return
	}
	for _, expLang := range expected {
		found := false
		for _, at := range result.MediaInfo.AudioTracks {
			if langMatchesExpected(at.Language, expLang) {
				found = true
				break
			}
		}
		if !found {
			report.AddIssue(Issue{
				Severity: SeverityError,
				Source:   SourceProfile,
				FilePath: filePath,
				Category: "language",
				Message:  "Missing expected audio language: " + langDisplayName(expLang),
			})
		}
	}
}

func checkSubtitleLanguages(report *ValidationReport, filePath string, result *FileCheckResult, expected []Lang) {
	if len(expected) == 0 {
		return
	}
	for _, expLang := range expected {
		found := false
		// Check embedded text tracks
		for _, tt := range result.MediaInfo.TextTracks {
			if langMatchesExpected(tt.Language, expLang) {
				found = true
				break
			}
		}
		// Check standalone subtitle candidates
		if !found {
			for _, sc := range result.SubCandidates {
				if langMatchesExpected(sc.Lang, expLang) {
					found = true
					break
				}
			}
		}
		if !found {
			report.AddIssue(Issue{
				Severity: SeverityError,
				Source:   SourceProfile,
				FilePath: filePath,
				Category: "language",
				Message:  "Missing expected subtitle language: " + langDisplayName(expLang),
			})
		}
	}
}

func checkLanguageTags(report *ValidationReport, filePath string, result *FileCheckResult, profile *ExpectationProfile, expectedAudioLangs, expectedSubLangs []Lang) {
	if !profile.RequireLanguageTags {
		return
	}

	// Determine which expected audio languages are already satisfied
	// by tagged tracks. An untagged track is only alarming (Warning)
	// if it might be the only way to satisfy an unsatisfied expectation.
	// Otherwise it's Info (tag hygiene).
	unsatisfiedAudioLangs := unsatisfiedLangs(expectedAudioLangs, result.MediaInfo.AudioTracks)
	hasUndAudio := false
	for i, at := range result.MediaInfo.AudioTracks {
		if at.Language.Part3 == "und" || at.Language.Language == nil {
			hasUndAudio = true
			severity := SeverityInfo
			if len(unsatisfiedAudioLangs) > 0 {
				severity = SeverityWarning
			}
			report.AddIssue(Issue{
				Severity: severity,
				Source:   SourceProfile,
				FilePath: filePath,
				Category: "language",
				Message:  "Audio track " + itoa(i+1) + " has no language tag",
			})
		}
	}
	_ = hasUndAudio

	unsatisfiedSubLangs := unsatisfiedSubtitleLangs(expectedSubLangs, result.MediaInfo.TextTracks, result.SubCandidates)
	for i, tt := range result.MediaInfo.TextTracks {
		if tt.Language.Part3 == "und" || tt.Language.Language == nil {
			severity := SeverityInfo
			if len(unsatisfiedSubLangs) > 0 {
				severity = SeverityWarning
			}
			report.AddIssue(Issue{
				Severity: severity,
				Source:   SourceProfile,
				FilePath: filePath,
				Category: "language",
				Message:  "Subtitle track " + itoa(i+1) + " has no language tag",
			})
		}
	}
}

// unsatisfiedLangs returns expected languages not matched by any tagged audio track.
func unsatisfiedLangs(expected []Lang, tracks []AudioTrack) []Lang {
	var unsatisfied []Lang
	for _, exp := range expected {
		found := false
		for _, at := range tracks {
			if at.Language.Part3 != "und" && langMatchesExpected(at.Language, exp) {
				found = true
				break
			}
		}
		if !found {
			unsatisfied = append(unsatisfied, exp)
		}
	}
	return unsatisfied
}

// unsatisfiedSubtitleLangs returns expected languages not matched by any tagged subtitle source.
func unsatisfiedSubtitleLangs(expected []Lang, tracks []TextTrack, candidates []SubtitleCandidate) []Lang {
	var unsatisfied []Lang
	for _, exp := range expected {
		found := false
		for _, tt := range tracks {
			if tt.Language.Part3 != "und" && langMatchesExpected(tt.Language, exp) {
				found = true
				break
			}
		}
		if !found {
			for _, sc := range candidates {
				if sc.Lang.Part3 != "und" && langMatchesExpected(sc.Lang, exp) {
					found = true
					break
				}
			}
		}
		if !found {
			unsatisfied = append(unsatisfied, exp)
		}
	}
	return unsatisfied
}

func checkDurationConsistency(report *ValidationReport, filePath string, result *FileCheckResult, profile *ExpectationProfile) {
	if result.VideoDuration == 0 {
		return
	}

	tolerancePct := profile.DurationTolerancePct
	if tolerancePct == 0 {
		tolerancePct = 2.0
	}
	const absoluteFloor = 2.0 // seconds

	for i, audioDur := range result.AudioDurations {
		if audioDur == 0 {
			report.AddIssue(Issue{
				Severity: SeverityInfo,
				Source:   SourceProfile,
				FilePath: filePath,
				Category: "duration",
				Message:  "Duration unavailable for audio track " + itoa(i+1),
			})
			continue
		}

		deviation := math.Abs(audioDur - result.VideoDuration)
		effectiveTolerance := math.Max(
			result.VideoDuration*tolerancePct/100.0,
			absoluteFloor,
		)

		if deviation > result.VideoDuration*0.10 {
			report.AddIssue(Issue{
				Severity: SeverityError,
				Source:   SourceProfile,
				FilePath: filePath,
				Category: "duration",
				Message: "Audio track " + itoa(i+1) + " duration (" +
					media.FormatDuration(audioDur) + ") deviates >10% from video (" +
					media.FormatDuration(result.VideoDuration) + ")",
			})
		} else if deviation > effectiveTolerance {
			report.AddIssue(Issue{
				Severity: SeverityWarning,
				Source:   SourceProfile,
				FilePath: filePath,
				Category: "duration",
				Message: "Audio track " + itoa(i+1) + " duration (" +
					media.FormatDuration(audioDur) + ") deviates from video (" +
					media.FormatDuration(result.VideoDuration) + ")",
			})
		}
	}
}

// checkExternalAudioDuration validates external audio file durations against
// the video duration using the same hybrid tolerance model as embedded tracks.
func checkExternalAudioDuration(report *ValidationReport, filePath string, result *FileCheckResult, profile *ExpectationProfile) {
	if !profile.CheckExternalAudio || result.VideoDuration == 0 {
		return
	}
	if len(result.ExternalAudio) == 0 {
		return
	}

	tolerancePct := profile.DurationTolerancePct
	if tolerancePct == 0 {
		tolerancePct = 2.0
	}
	const absoluteFloor = 2.0

	for _, ea := range result.ExternalAudio {
		if ea.Duration == 0 {
			report.AddIssue(Issue{
				Severity: SeverityInfo,
				Source:   SourceProfile,
				FilePath: filePath,
				Category: "duration",
				Message:  "Duration unavailable for external audio: " + filepath.Base(ea.Path),
			})
			continue
		}

		deviation := math.Abs(ea.Duration - result.VideoDuration)
		effectiveTolerance := math.Max(
			result.VideoDuration*tolerancePct/100.0,
			absoluteFloor,
		)

		extName := filepath.Base(ea.Path)
		if deviation > result.VideoDuration*0.10 {
			report.AddIssue(Issue{
				Severity: SeverityError,
				Source:   SourceProfile,
				FilePath: filePath,
				Category: "duration",
				Message: "External audio " + extName + " duration (" +
					media.FormatDuration(ea.Duration) + ") deviates >10% from video (" +
					media.FormatDuration(result.VideoDuration) + ")",
			})
		} else if deviation > effectiveTolerance {
			report.AddIssue(Issue{
				Severity: SeverityWarning,
				Source:   SourceProfile,
				FilePath: filePath,
				Category: "duration",
				Message: "External audio " + extName + " duration (" +
					media.FormatDuration(ea.Duration) + ") deviates from video (" +
					media.FormatDuration(result.VideoDuration) + ")",
			})
		}
	}
}

// checkSubtitleIntegrity validates parsed subtitle data per PRD 6.4.2.
func checkSubtitleIntegrity(report *ValidationReport, filePath string, result *FileCheckResult) {
	if len(result.SubCheckResults) == 0 {
		return
	}

	for _, scr := range result.SubCheckResults {
		subName := filepath.Base(scr.FilePath)

		// 6.4.2.1 Parse check
		if scr.ParseErr != "" {
			report.AddIssue(Issue{
				Severity: SeverityError,
				Source:   SourceStructural,
				FilePath: filePath,
				Category: "subtitle",
				Message:  "Subtitle file " + subName + " is unparseable: " + scr.ParseErr,
			})
			continue
		}

		// 6.4.2.3 Empty file check
		if scr.LineCount == 0 {
			report.AddIssue(Issue{
				Severity: SeverityError,
				Source:   SourceStructural,
				FilePath: filePath,
				Category: "subtitle",
				Message:  "Subtitle file " + subName + " has zero parseable lines",
			})
			continue
		}

		// 6.4.2.4 Encoding sanity
		if scr.EncodingIssue {
			report.AddIssue(Issue{
				Severity: SeverityWarning,
				Source:   SourceStructural,
				FilePath: filePath,
				Category: "subtitle",
				Message:  "Subtitle file " + subName + " may have encoding issues (replacement characters detected)",
			})
		}
	}

	// Tail-timestamp coverage: check if subtitles end too early
	checkSubtitleCoverage(report, filePath, result)
}

// checkSubtitleCoverage checks whether each subtitle file covers the
// video duration adequately by comparing the tail end (median of last
// k cue end-times) against the video duration.
//
// Tiered severity based on tail gap:
//   Info:    tailGap > max(90s,  4% of video)
//   Warning: tailGap > max(180s, 8% of video)
//   Error:   tailGap > max(420s, 15% of video)
//
// Skipped when video duration is unknown or cue count < 20.
func checkSubtitleCoverage(report *ValidationReport, filePath string, result *FileCheckResult) {
	if result.VideoDuration <= 0 {
		return
	}
	vd := result.VideoDuration

	for _, scr := range result.SubCheckResults {
		if scr.ParseErr != "" || scr.LineCount < 20 || scr.TailEndSec <= 0 {
			continue
		}

		tailGap := vd - scr.TailEndSec
		if tailGap <= 0 {
			continue
		}

		subName := filepath.Base(scr.FilePath)

		errorThreshold := math.Max(420, 0.15*vd)
		warnThreshold := math.Max(180, 0.08*vd)
		infoThreshold := math.Max(90, 0.04*vd)

		if tailGap > errorThreshold {
			report.AddIssue(Issue{
				Severity: SeverityError,
				Source:   SourceStructural,
				FilePath: filePath,
				Category: "subtitle",
				Message: "Subtitle file " + subName + " ends at " +
					media.FormatDuration(scr.TailEndSec) +
					" but video is " + media.FormatDuration(vd) +
					" -- likely truncated",
			})
		} else if tailGap > warnThreshold {
			report.AddIssue(Issue{
				Severity: SeverityWarning,
				Source:   SourceStructural,
				FilePath: filePath,
				Category: "subtitle",
				Message: "Subtitle file " + subName + " ends at " +
					media.FormatDuration(scr.TailEndSec) +
					" but video is " + media.FormatDuration(vd) +
					" -- may be truncated",
			})
		} else if tailGap > infoThreshold {
			report.AddIssue(Issue{
				Severity: SeverityInfo,
				Source:   SourceStructural,
				FilePath: filePath,
				Category: "subtitle",
				Message: "Subtitle file " + subName + " ends at " +
					media.FormatDuration(scr.TailEndSec) +
					", video is " + media.FormatDuration(vd),
			})
		}
	}
}

// checkEmbeddedStandaloneOverlap detects when both embedded and standalone
// subtitles exist for the same language (PRD 6.4.3).
func checkEmbeddedStandaloneOverlap(report *ValidationReport, filePath string, result *FileCheckResult) {
	// Collect languages with embedded tracks
	embeddedLangs := make(map[string]bool)
	for _, tt := range result.MediaInfo.TextTracks {
		if !isTextBasedFormat(tt.Format) {
			continue
		}
		code := langCode(tt.Language)
		if code != "und" {
			embeddedLangs[code] = true
		}
	}

	// Check standalone candidates for overlaps
	standaloneLangs := make(map[string]bool)
	for _, sc := range result.SubCandidates {
		if sc.Source.Type != SubSourceStandalone {
			continue
		}
		code := langCode(sc.Lang)
		if code != "und" {
			standaloneLangs[code] = true
		}
	}

	for lang := range standaloneLangs {
		if embeddedLangs[lang] {
			report.AddIssue(Issue{
				Severity: SeverityInfo,
				Source:   SourceStructural,
				FilePath: filePath,
				Category: "subtitle",
				Message:  "Both embedded and standalone subtitles exist for language " + lang,
			})
		}
	}
}

// langDisplayName returns a readable name for a language.
func langDisplayName(l Lang) string {
	if l.Language != nil && l.Language.Name != "" {
		name := l.Language.Name
		if l.Subtag != "" {
			name += " (" + l.Subtag + ")"
		}
		return name
	}
	return l.String()
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
