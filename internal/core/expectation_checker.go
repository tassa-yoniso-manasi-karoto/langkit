package core

import (
	"context"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
)

// RunCheck performs the expectation check on the given path.
// If profile is nil, only structural checks (integrity) run.
// If autoConfig is non-nil and Enabled, auto consistency checks run.
// Both can be combined. The function respects context cancellation.
// decodeDepth overrides the settings value; pass "" to use the setting.
func RunCheck(ctx context.Context, rootPath string, profile *ExpectationProfile, autoConfig *AutoCheckConfig, decodeDepth media.IntegrityDepth, cb CheckCallbacks) (*ValidationReport, error) {
	start := time.Now()
	log := cb.Logger.With().Str("component", "expectation-checker").Logger()

	// Resolve decode depth: explicit param > settings > default sampled
	depthSource := "explicit"
	if decodeDepth == "" {
		settings, err := config.LoadSettings()
		if err == nil {
			decodeDepth = media.NormalizeIntegrityDepth(settings.IntegrityDecodeDepth)
			depthSource = "settings"
		} else {
			decodeDepth = media.IntegritySampled
			depthSource = "fallback"
		}
	}

	log.Info().
		Str("rootPath", rootPath).
		Bool("hasProfile", profile != nil).
		Bool("hasAutoConfig", autoConfig != nil && autoConfig.Enabled).
		Str("decodeDepth", string(decodeDepth)).
		Str("depthSource", depthSource).
		Msg("Starting expectation check")

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

	files, err := DiscoverMediaFiles(rootPath, extensions, log)
	if err != nil {
		log.Error().Err(err).Msg("Discovery failed")
		return nil, err
	}
	report.TotalFiles = len(files)

	log.Info().Int("fileCount", len(files)).Msg("Discovery complete")

	if len(files) == 0 {
		report.AddIssue(Issue{
			Severity: SeverityInfo,
			Source:   SourceStructural,
			FilePath: rootPath,
			Category: "structure",
			Code:     CodeNoMediaFiles,
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
				log.Error().Err(err).Strs("tags", profile.ExpectedAudioLangs).Msg("Failed to parse expected audio languages")
				return nil, err
			}
			log.Debug().Strs("audioLangs", profile.ExpectedAudioLangs).Msg("Parsed expected audio languages")
		}
		if len(profile.ExpectedSubtitleLangs) > 0 {
			expectedSubLangs, err = ParseLanguageTags(profile.ExpectedSubtitleLangs)
			if err != nil {
				log.Error().Err(err).Strs("tags", profile.ExpectedSubtitleLangs).Msg("Failed to parse expected subtitle languages")
				return nil, err
			}
			log.Debug().Strs("subLangs", profile.ExpectedSubtitleLangs).Msg("Parsed expected subtitle languages")
		}
	}

	// Pass 1: Probe – gather metadata for every file (no decode)
	log.Debug().Msg("Starting probe pass")
	for _, filePath := range files {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		checkExtAudio := profile != nil && profile.CheckExternalAudio
		result := probeFile(filePath, checkExtAudio)
		report.FileResults[filePath] = result

		// If mediainfo failed, emit a structural issue
		if result.MediaInfoErr != nil {
			log.Warn().Str("file", filepath.Base(filePath)).Err(result.MediaInfoErr).Msg("Mediainfo failed")
			report.AddIssue(Issue{
				Severity: SeverityError,
				Source:   SourceStructural,
				FilePath: filePath,
				Category: "structure",
				Code:     CodeMediainfoFailed,
				Message:  "Could not read media metadata: " + result.MediaInfoErr.Error(),
			})
			continue
		}

		log.Debug().
			Str("file", filepath.Base(filePath)).
			Int("audioTracks", len(result.MediaInfo.AudioTracks)).
			Int("textTracks", len(result.MediaInfo.TextTracks)).
			Int("subCandidates", len(result.SubCandidates)).
			Int("externalAudio", len(result.ExternalAudio)).
			Msg("Probed file")

		if profile != nil {
			checkVideoTrack(report, filePath, result, profile)
		}

		if cb.OnProgress != nil {
			cb.OnProgress(progress.BarCheckProbe, 1, len(files),
				"Probing: "+filepath.Base(filePath))
		}
	}

	// Structural checks: always run regardless of profile/auto mode.
	// Duration tolerance comes from the profile if available, else a
	// sensible default. The source tag reflects the origin.
	log.Debug().Msg("Running structural checks")
	tolerancePct := 2.0
	durationSource := SourceStructural
	if profile != nil && profile.DurationTolerancePct > 0 {
		tolerancePct = profile.DurationTolerancePct
		durationSource = SourceProfile
	}

	for _, filePath := range files {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		result := report.FileResults[filePath]
		if result.MediaInfoErr != nil {
			continue
		}
		checkDurationConsistency(report, filePath, result, tolerancePct, durationSource)
		checkSubtitleIntegrity(report, filePath, result)
		checkEmbeddedStandaloneOverlap(report, filePath, result)
	}

	// Pass 2: Decode integrity + domain checks.
	// Each mode runs its own scoped decode pass. The deduplication
	// inside runDecodeIntegrity ensures streams already checked by a
	// prior pass are not re-decoded.
	decodeRan := false

	// Profile checks (only on files with successful mediainfo)
	if profile != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Debug().Msg("Running profile decode integrity")
		// Decode integrity scoped to expected audio languages
		runDecodeIntegrity(ctx, report, files, report.FileResults,
			decodeDepth, expectedAudioLangs, log, cb)
		decodeRan = true

		log.Debug().Msg("Running profile domain checks")
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
			checkExternalAudioDuration(report, filePath, result, profile)
		}
	}

	// Auto checks: group files by directory and run consistency checks.
	// Always runs its own decode pass (scoped to consensus languages)
	// even when profile already ran — dedup skips already-checked streams.
	if autoConfig != nil && autoConfig.Enabled {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Debug().Msg("Running auto decode integrity")
		runAutoDecodeIntegrity(ctx, report, files, report.FileResults,
			decodeDepth, autoConfig, log, cb)
		decodeRan = true
		runAutoMode(report, files, autoConfig, log)
	}

	// Structural-only fallback: if neither profile nor auto ran decode
	if !decodeRan {
		log.Debug().Msg("Running structural-only decode integrity")
		runDecodeIntegrity(ctx, report, files, report.FileResults,
			decodeDepth, nil, log, cb)
	}

	// Post-processing: merge correlated decode + duration findings
	mergeCorrelatedFindings(report, log)

	report.Duration = time.Since(start)
	log.Info().
		Dur("elapsed", report.Duration).
		Int("totalFiles", report.TotalFiles).
		Int("errors", report.ErrorCount).
		Int("warnings", report.WarningCount).
		Int("infos", report.InfoCount).
		Msg("Expectation check complete")
	return report, nil
}

// runAutoMode groups files by immediate parent directory and runs
// consistency checks for each group meeting the minimum size.
func runAutoMode(report *ValidationReport, files []string, config *AutoCheckConfig, log zerolog.Logger) {
	// Group files by immediate parent directory
	dirFiles := make(map[string][]string)
	for _, fp := range files {
		dir := filepath.Dir(fp)
		dirFiles[dir] = append(dirFiles[dir], fp)
	}

	for dir, fps := range dirFiles {
		// Build consensus for this directory
		dc := buildConsensus(dir, fps, report.FileResults, config, log)
		report.Consensus[dir] = dc

		if dc.FileCount < config.MinGroupSize {
			log.Debug().
				Str("dir", filepath.Base(dir)).
				Int("fileCount", dc.FileCount).
				Int("minGroupSize", config.MinGroupSize).
				Msg("Skipped auto-check: group too small")
			report.AddIssue(Issue{
				Severity: SeverityInfo,
				Source:   SourceAuto,
				FilePath: dir,
				Category: "consistency",
				Code:     CodeAutoGroupTooSmall,
				Message: "Skipped auto-check for " + filepath.Base(dir) +
					" (" + itoa(dc.FileCount) + " eligible files, minimum is " +
					itoa(config.MinGroupSize) + ")",
			})
			continue
		}

		runAutoChecks(report, dc, fps, report.FileResults, config, log)
	}
}

// probeFile gathers all metadata for a single media file.
// If checkExternalAudio is true, also discovers and probes sidecar audio files.
// Decode integrity is NOT run here; it runs in a separate pass so that
// stream scoping can use language information from mediainfo or consensus.
func probeFile(filePath string, checkExternalAudio bool) *FileCheckResult {
	result := &FileCheckResult{
		VideoFile: filePath,
	}

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

// checkDecodeResults emits issues for any streams that failed decode.
// Corruption is always attributed to SourceStructural because it is a
// fundamental media defect, not an expectation mismatch.
func checkDecodeResults(report *ValidationReport, filePath string, result *FileCheckResult) {
	// Map container stream index → audio track position for labeling
	streamToPos := make(map[int]int)
	for i, at := range result.MediaInfo.AudioTracks {
		if idx, parseErr := strconv.Atoi(at.StreamOrder); parseErr == nil {
			streamToPos[idx] = i
		}
	}

	for _, dr := range result.DecodeResults {
		if !dr.Corrupted {
			continue
		}
		var msg string
		var code string
		var label string
		if dr.StreamIndex == -1 {
			msg = "Video stream decode failed"
			code = CodeVideoDecodeFailed
		} else {
			if pos, ok := streamToPos[dr.StreamIndex]; ok {
				label = audioTrackLabel(result.MediaInfo.AudioTracks, pos)
			} else {
				label = "Audio stream " + itoa(dr.StreamIndex)
			}
			msg = label + " decode failed"
			code = CodeAudioDecodeFailed
		}
		if dr.ErrorOutput != "" {
			// Trim to first line for conciseness
			firstLine := dr.ErrorOutput
			if idx := strings.Index(firstLine, "\n"); idx != -1 {
				firstLine = firstLine[:idx]
			}
			msg += ": " + strings.TrimSpace(firstLine)
		}
		report.AddIssue(Issue{
			Severity:     SeverityError,
			Source:       SourceStructural,
			FilePath:     filePath,
			Category:     "integrity",
			Code:         code,
			Message:      msg,
			SubjectLabel: label,
		})
	}
}

// mergeCorrelatedFindings replaces paired decode-failure + duration-deviation
// issues on the same audio track with a single CodeCorruptTrack issue.
// A corrupt stream typically appears as both "Audio stream N decode failed"
// and "Audio track M duration deviates" — these share a single root cause.
func mergeCorrelatedFindings(report *ValidationReport, log zerolog.Logger) {
	// Build per-file set of corrupt audio track positions (0-based).
	// StreamIndex (container stream order) → audio track position requires
	// the MediaInfo AudioTracks list for mapping.
	type corruptInfo struct {
		trackPos    int    // 0-based audio track position
		streamIndex int    // container stream index
		audioDur    float64
		videoDur    float64
	}

	fileCorrupt := make(map[string][]corruptInfo) // filePath → corrupt tracks

	for filePath, result := range report.FileResults {
		if result == nil || result.MediaInfoErr != nil || len(result.DecodeResults) == 0 {
			continue
		}

		// Map StreamIndex → audio track position
		streamToPos := make(map[int]int)
		for i, at := range result.MediaInfo.AudioTracks {
			idx, err := strconv.Atoi(at.StreamOrder)
			if err == nil {
				streamToPos[idx] = i
			}
		}

		for _, dr := range result.DecodeResults {
			if !dr.Corrupted || dr.StreamIndex == -1 {
				continue // skip video or non-corrupt
			}
			if pos, ok := streamToPos[dr.StreamIndex]; ok {
				ci := corruptInfo{
					trackPos:    pos,
					streamIndex: dr.StreamIndex,
					videoDur:    result.VideoDuration,
				}
				if pos < len(result.AudioDurations) {
					ci.audioDur = result.AudioDurations[pos]
				}
				fileCorrupt[filePath] = append(fileCorrupt[filePath], ci)
			}
		}
	}

	if len(fileCorrupt) == 0 {
		return
	}

	// Build lookup: (filePath, trackPos) → true for mergeable pairs
	type trackKey struct {
		filePath string
		trackPos int
	}
	mergeSet := make(map[trackKey]corruptInfo)
	for fp, tracks := range fileCorrupt {
		for _, ci := range tracks {
			mergeSet[trackKey{fp, ci.trackPos}] = ci
		}
	}

	// Filter issues: remove decode + duration pairs, collect merged issues
	var merged []Issue
	kept := make([]Issue, 0, len(report.Issues))
	removedErrors := 0
	removedWarnings := 0
	removedInfos := 0

	for _, iss := range report.Issues {
		switch iss.Code {
		case CodeAudioDecodeFailed:
			// Check if this decode issue maps to a known corrupt track
			fp := iss.FilePath
			if tracks, ok := fileCorrupt[fp]; ok {
				removed := false
				for _, ci := range tracks {
					// Match by stream index in the message
					if strings.Contains(iss.Message, "Audio stream "+itoa(ci.streamIndex)+" decode failed") {
						// Check if a corresponding duration issue exists
						if ci.audioDur > 0 && ci.videoDur > 0 {
							removed = true
							removedErrors++
							break
						}
					}
				}
				if removed {
					continue
				}
			}
			kept = append(kept, iss)

		case CodeAudioDurationMismatch:
			// Check if this duration issue corresponds to a corrupt track
			fp := iss.FilePath
			removed := false
			for trackPos := range mergeSet {
				if trackPos.filePath == fp {
					// Match by track label in message
					if strings.Contains(iss.Message, audioTrackLabel(report.FileResults[fp].MediaInfo.AudioTracks, trackPos.trackPos)+" duration") {
						removed = true
						switch iss.Severity {
						case SeverityError:
							removedErrors++
						case SeverityWarning:
							removedWarnings++
						}
						break
					}
				}
			}
			if !removed {
				kept = append(kept, iss)
			}

		default:
			kept = append(kept, iss)
		}
	}

	// Emit merged issues
	for fp, tracks := range fileCorrupt {
		for _, ci := range tracks {
			if ci.audioDur > 0 && ci.videoDur > 0 {
				merged = append(merged, Issue{
					Severity: SeverityError,
					Source:   SourceStructural,
					FilePath: fp,
					Category: "integrity",
					Code:     CodeCorruptTrack,
					Message: audioTrackLabel(report.FileResults[fp].MediaInfo.AudioTracks, ci.trackPos) +
						" corrupt — decoded " + media.FormatDuration(ci.audioDur) +
						" of " + media.FormatDuration(ci.videoDur) + " video",
					SubjectLabel: audioTrackLabel(report.FileResults[fp].MediaInfo.AudioTracks, ci.trackPos),
				})
			}
		}
	}

	if len(merged) > 0 {
		report.Issues = append(kept, merged...)
		// Recount severities
		report.ErrorCount = report.ErrorCount - removedErrors + len(merged)
		report.WarningCount -= removedWarnings
		report.InfoCount -= removedInfos
		log.Debug().
			Int("mergedPairs", len(merged)).
			Msg("Merged correlated decode+duration findings")
	}
}

// ResolveAudioStreamIndices returns 0-based StreamOrder values for audio
// tracks matching the given languages. If no languages are given or no
// matches are found, returns all audio stream indices.
func ResolveAudioStreamIndices(mi MediaInfo, langs ...Lang) []int {
	var matched []int
	for _, at := range mi.AudioTracks {
		for _, lang := range langs {
			if langMatchesExpected(at.Language, lang) {
				idx, err := strconv.Atoi(at.StreamOrder)
				if err == nil {
					matched = append(matched, idx)
				}
				break
			}
		}
	}
	if len(matched) > 0 {
		return matched
	}
	return allAudioStreamIndices(mi)
}

// allAudioStreamIndices returns StreamOrder values for all audio tracks.
func allAudioStreamIndices(mi MediaInfo) []int {
	var indices []int
	for _, at := range mi.AudioTracks {
		idx, err := strconv.Atoi(at.StreamOrder)
		if err == nil {
			indices = append(indices, idx)
		}
	}
	return indices
}

// runDecodeIntegrity runs decode checks on all files and stores results.
func runDecodeIntegrity(
	ctx context.Context,
	report *ValidationReport,
	files []string,
	fileResults map[string]*FileCheckResult,
	depth media.IntegrityDepth,
	scopeLangs []Lang,
	log zerolog.Logger,
	cb CheckCallbacks,
) {
	// Pre-compute which files will actually be decoded so the progress
	// bar total reflects real work rather than the full file list.
	type decodeWork struct {
		filePath    string
		indices     []int
		checkVideo  bool
		dedupSkipped int
	}
	var work []decodeWork
	for _, filePath := range files {
		result := fileResults[filePath]
		if result == nil || result.MediaInfoErr != nil {
			continue
		}
		if result.DecodeCorrupted {
			continue
		}

		indices := ResolveAudioStreamIndices(result.MediaInfo, scopeLangs...)

		dedupSkipped := 0
		if len(result.DecodeResults) > 0 {
			covered := make(map[int]bool)
			for _, dr := range result.DecodeResults {
				covered[dr.StreamIndex] = true
			}
			var novel []int
			for _, idx := range indices {
				if !covered[idx] {
					novel = append(novel, idx)
				} else {
					dedupSkipped++
				}
			}
			indices = novel
		}

		checkVideo := result.MediaInfo.VideoTrack.Type != ""
		for _, dr := range result.DecodeResults {
			if dr.StreamIndex == -1 {
				checkVideo = false
				break
			}
		}

		if len(indices) == 0 && !checkVideo {
			continue
		}

		work = append(work, decodeWork{filePath, indices, checkVideo, dedupSkipped})
	}

	for _, w := range work {
		result := fileResults[w.filePath]

		log.Debug().
			Str("file", filepath.Base(w.filePath)).
			Str("depth", string(depth)).
			Bool("checkVideo", w.checkVideo).
			Ints("audioStreams", w.indices).
			Int("dedupSkipped", w.dedupSkipped).
			Msg("Decode integrity starting")

		scope := media.DecodeScope{
			AudioStreamIndices: w.indices,
			CheckVideo:         w.checkVideo,
		}

		decodeResults, err := media.CheckDecodeIntegrity(ctx, w.filePath, depth, scope)
		if err != nil {
			log.Error().Str("file", filepath.Base(w.filePath)).Err(err).Msg("Decode integrity check failed")
			report.AddIssue(Issue{
				Severity: SeverityError,
				Source:   SourceStructural,
				FilePath: w.filePath,
				Category: "integrity",
				Code:     CodeAudioDecodeFailed,
				Message:  "Decode integrity check failed: " + err.Error(),
			})
			result.DecodeCorrupted = true
		}
		if err == nil {
			result.DecodeResults = append(result.DecodeResults, decodeResults...)

			for _, dr := range decodeResults {
				if dr.Corrupted {
					log.Warn().
						Str("file", filepath.Base(w.filePath)).
						Int("streamIndex", dr.StreamIndex).
						Msg("Corrupted stream detected")
					result.DecodeCorrupted = true
					break
				}
			}

			checkDecodeResults(report, w.filePath, result)
		}

		if cb.OnProgress != nil {
			cb.OnProgress(progress.BarCheckDecode, 1, len(work),
				"Verifying: "+filepath.Base(w.filePath))
		}
	}
}

// runAutoDecodeIntegrity builds preliminary consensus per directory group
// and scopes decode integrity to each group's quorum audio languages.
// This satisfies PRD 6.3: auto mode uses consensus-derived scope.
// Note: DecodeCorrupted is not yet set during preliminary consensus, so
// all files with valid mediainfo participate. runAutoMode will rebuild
// consensus afterward with decode results available.
func runAutoDecodeIntegrity(
	ctx context.Context,
	report *ValidationReport,
	files []string,
	fileResults map[string]*FileCheckResult,
	depth media.IntegrityDepth,
	autoConfig *AutoCheckConfig,
	log zerolog.Logger,
	cb CheckCallbacks,
) {
	// Group files by immediate parent directory
	dirFiles := make(map[string][]string)
	for _, fp := range files {
		dir := filepath.Dir(fp)
		dirFiles[dir] = append(dirFiles[dir], fp)
	}

	for dir, fps := range dirFiles {
		// Build preliminary consensus for language scoping
		dc := buildConsensus(dir, fps, fileResults, autoConfig, log)

		// Convert quorum audio languages to Lang for scoping
		var scopeLangs []Lang
		for _, code := range dc.QuorumAudioLangs {
			langs, err := ParseLanguageTags([]string{code})
			if err == nil {
				scopeLangs = append(scopeLangs, langs...)
			}
		}

		log.Debug().
			Str("dir", filepath.Base(dir)).
			Strs("quorumLangs", dc.QuorumAudioLangs).
			Int("fileCount", len(fps)).
			Msg("Auto decode integrity: scoped to consensus languages")

		// Run decode integrity for this directory group,
		// scoped to consensus languages (falls back to all
		// audio streams if no quorum languages were found).
		runDecodeIntegrity(ctx, report, fps, fileResults,
			depth, scopeLangs, log, cb)
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
			Code:     CodeNoVideoTrack,
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
				Code:     CodeMissingAudioLang,
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
				Code:     CodeMissingSubLang,
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
			label := "Audio track " + itoa(i+1)
			report.AddIssue(Issue{
				Severity:     severity,
				Source:       SourceProfile,
				FilePath:     filePath,
				Category:     "language",
				Code:         CodeUntaggedTrack,
				Message:      label + " has no language tag",
				SubjectLabel: label,
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
			label := "Subtitle track " + itoa(i+1)
			report.AddIssue(Issue{
				Severity:     severity,
				Source:       SourceProfile,
				FilePath:     filePath,
				Category:     "language",
				Code:         CodeUntaggedTrack,
				Message:      label + " has no language tag",
				SubjectLabel: label,
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

func checkDurationConsistency(report *ValidationReport, filePath string, result *FileCheckResult, tolerancePct float64, source IssueSource) {
	if result.VideoDuration == 0 {
		return
	}

	if tolerancePct == 0 {
		tolerancePct = 2.0
	}
	const absoluteFloor = 2.0 // seconds

	for i, audioDur := range result.AudioDurations {
		label := audioTrackLabel(result.MediaInfo.AudioTracks, i)
		if audioDur == 0 {
			report.AddIssue(Issue{
				Severity:     SeverityInfo,
				Source:       source,
				FilePath:     filePath,
				Category:     "duration",
				Code:         CodeDurationUnavailable,
				Message:      "Duration unavailable for " + label,
				SubjectLabel: label,
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
				Severity:     SeverityError,
				Source:       source,
				FilePath:     filePath,
				Category:     "duration",
				Code:         CodeAudioDurationMismatch,
				Message: label + " duration (" +
					media.FormatDuration(audioDur) + ") deviates >10% from video (" +
					media.FormatDuration(result.VideoDuration) + ")",
				SubjectLabel: label,
			})
		} else if deviation > effectiveTolerance {
			report.AddIssue(Issue{
				Severity:     SeverityWarning,
				Source:       source,
				FilePath:     filePath,
				Category:     "duration",
				Code:         CodeAudioDurationMismatch,
				Message: label + " duration (" +
					media.FormatDuration(audioDur) + ") deviates from video (" +
					media.FormatDuration(result.VideoDuration) + ")",
				SubjectLabel: label,
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
				Code:     CodeDurationUnavailable,
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
				Code:     CodeExtAudioDuration,
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
				Code:     CodeExtAudioDuration,
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
		subLabel := subtitleLabel(scr.FilePath)

		// 6.4.2.1 Parse check
		if scr.ParseErr != "" {
			report.AddIssue(Issue{
				Severity:     SeverityError,
				Source:       SourceStructural,
				FilePath:     filePath,
				Category:     "subtitle",
				Code:         CodeSubParseFailed,
				Message:      subLabel + " is unparseable: " + scr.ParseErr,
				SubjectLabel: subLabel,
			})
			continue
		}

		// 6.4.2.3 Empty file check
		if scr.LineCount == 0 {
			report.AddIssue(Issue{
				Severity:     SeverityError,
				Source:       SourceStructural,
				FilePath:     filePath,
				Category:     "subtitle",
				Code:         CodeSubEmpty,
				Message:      subLabel + " has zero parseable lines",
				SubjectLabel: subLabel,
			})
			continue
		}

		// 6.4.2.4 Encoding sanity
		if scr.EncodingIssue {
			report.AddIssue(Issue{
				Severity:     SeverityWarning,
				Source:       SourceStructural,
				FilePath:     filePath,
				Category:     "subtitle",
				Code:         CodeSubEncoding,
				Message:      subLabel + " may have encoding issues (replacement characters detected)",
				SubjectLabel: subLabel,
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
//   Info:    tailGap > max(300s, 10% of video)
//   Warning: tailGap > max(600s, 20% of video)
//   Error:   tailGap > max(900s, 30% of video)
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

		subLabel := subtitleLabel(scr.FilePath)

		errorThreshold := math.Max(900, 0.30*vd)
		warnThreshold := math.Max(600, 0.20*vd)
		infoThreshold := math.Max(300, 0.10*vd)

		if tailGap > errorThreshold {
			report.AddIssue(Issue{
				Severity:     SeverityError,
				Source:       SourceStructural,
				FilePath:     filePath,
				Category:     "subtitle",
				Code:         CodeSubLowCoverage,
				Message: subLabel + " ends at " +
					media.FormatDuration(scr.TailEndSec) +
					" but video is " + media.FormatDuration(vd) +
					" -- likely truncated",
				SubjectLabel: subLabel,
			})
		} else if tailGap > warnThreshold {
			report.AddIssue(Issue{
				Severity:     SeverityWarning,
				Source:       SourceStructural,
				FilePath:     filePath,
				Category:     "subtitle",
				Code:         CodeSubLowCoverage,
				Message: subLabel + " ends at " +
					media.FormatDuration(scr.TailEndSec) +
					" but video is " + media.FormatDuration(vd) +
					" -- may be truncated",
				SubjectLabel: subLabel,
			})
		} else if tailGap > infoThreshold {
			report.AddIssue(Issue{
				Severity:     SeverityInfo,
				Source:       SourceStructural,
				FilePath:     filePath,
				Category:     "subtitle",
				Code:         CodeSubLowCoverage,
				Message: subLabel + " ends at " +
					media.FormatDuration(scr.TailEndSec) +
					", video is " + media.FormatDuration(vd),
				SubjectLabel: subLabel,
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
				Code:     CodeSubOverlap,
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

// audioTrackLabel returns a human-readable label for an audio track,
// preferring the language tag over the 1-based track number.
func audioTrackLabel(tracks []AudioTrack, idx int) string {
	if idx < len(tracks) && tracks[idx].Language.Language != nil &&
		tracks[idx].Language.Part3 != "" && tracks[idx].Language.Part3 != "und" {
		return "Audio (" + tracks[idx].Language.Part3 + ")"
	}
	return "Audio track " + itoa(idx+1)
}

// subtitleLabel returns a human-readable label for a subtitle file,
// preferring the parsed language over the raw filename.
func subtitleLabel(filePath string) string {
	base := filepath.Base(filePath)
	if lang, err := GuessLangFromFilename(base); err == nil {
		label := lang.Part3
		if lang.Subtag != "" {
			label += "-" + lang.Subtag
		}
		return "Subtitle (" + label + ")"
	}
	return "Subtitle " + base
}
