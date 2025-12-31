package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
)

// SubSourceType distinguishes standalone files from embedded tracks
type SubSourceType int

const (
	SubSourceStandalone SubSourceType = iota
	SubSourceEmbedded
)

// SubtitleSource represents where a subtitle comes from
type SubtitleSource struct {
	Type SubSourceType

	// Standalone fields
	FilePath string // Actual file path on disk

	// Embedded fields
	MediaFile   string // Container file (video) path
	TrackIndex  int    // Subtitle track index (0-based among subtitle tracks)
	StreamIndex int    // FFmpeg stream index for extraction (-map 0:streamIndex)
	Format      string // "ASS", "SRT", "SubRip", etc.
	CodecID     string // "S_TEXT/ASS", "S_TEXT/UTF8", etc.
}

// SubtitleCandidate represents a potential subtitle for selection
type SubtitleCandidate struct {
	Lang   Lang
	Source SubtitleSource

	// Quality factors for selection
	IsDefault bool   // Default flag from container metadata
	Title     string // Track title ("Dialogue", "Signs/Songs", etc.)
	Subtype   int    // CC, Dub, Sub from existing subtypeMatcher
}

// formatToExt maps mediainfo Format names to file extensions
var formatToExt = map[string]string{
	"ASS":    ".ass",
	"SSA":    ".ssa",
	"SRT":    ".srt",
	"SubRip": ".srt",
	"UTF-8":  ".srt", // Generic text, treat as SRT
}

// isTextBasedFormat returns true if the format can be processed as text
func isTextBasedFormat(format string) bool {
	ext, ok := formatToExt[format]
	if !ok {
		return false
	}
	// Check against existing SupportedExt
	for _, supported := range SupportedExt {
		if ext == supported {
			return true
		}
	}
	return false
}

// Materialize returns a usable file path, extracting embedded tracks if needed
func (c *SubtitleCandidate) Materialize(tempDir string) (string, error) {
	if c.Source.Type == SubSourceStandalone {
		return c.Source.FilePath, nil
	}

	// Embedded - extract to temp
	ext, ok := formatToExt[c.Source.Format]
	if !ok {
		ext = ".srt" // Fallback
	}
	outFile := filepath.Join(tempDir, fmt.Sprintf("track_%d%s", c.Source.StreamIndex, ext))

	// Skip if already extracted (idempotent)
	if _, err := os.Stat(outFile); err == nil {
		return outFile, nil
	}

	err := media.ExtractSubtitleTrack(c.Source.MediaFile, c.Source.StreamIndex, outFile)
	if err != nil {
		return "", fmt.Errorf("extracting subtitle track %d: %w", c.Source.StreamIndex, err)
	}

	return outFile, nil
}

// filterByLang filters candidates by language using getIdx for script-aware matching
func filterByLang(candidates []SubtitleCandidate, langs []Lang) []SubtitleCandidate {
	var matches []SubtitleCandidate
	for _, c := range candidates {
		// Use getIdx - it handles script subtags correctly
		// e.g., user requests "zho" → matches embedded "zh-Hans" (default script)
		if _, ok := getIdx(langs, c.Lang); ok {
			matches = append(matches, c)
		}
	}
	return matches
}

// candidateQuality returns a quality score for selection ranking
func candidateQuality(c SubtitleCandidate) int {
	score := 0

	// Default flag (user's stated preference)
	if c.IsDefault {
		score += 100
	}

	// Title-based: prefer dialogue over signs-only
	titleLower := strings.ToLower(c.Title)
	if strings.Contains(titleLower, "dialogue") || strings.Contains(titleLower, "dialog") {
		score += 50
	}
	if isSignsOnly(titleLower) {
		score -= 100 // Strong penalty for signs-only
	}

	// Existing subtype priority (CC > Dub > Sub)
	score += c.Subtype * 10

	return score
}

// isSignsOnly returns true if title indicates signs/songs only (no dialogue)
func isSignsOnly(title string) bool {
	hasSign := strings.Contains(title, "sign")
	hasDialogue := strings.Contains(title, "dialog") || strings.Contains(title, "dialogue")
	return hasSign && !hasDialogue
}

// pickBest selects the highest quality candidate from a slice
func pickBest(candidates []SubtitleCandidate) *SubtitleCandidate {
	if len(candidates) == 0 {
		return nil
	}
	// Sort by quality score (highest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidateQuality(candidates[i]) > candidateQuality(candidates[j])
	})
	return &candidates[0]
}

// selectBestCandidates selects the best target and native subtitle candidates
func selectBestCandidates(candidates []SubtitleCandidate, targLang Lang, refLangs []Lang) (targ, native *SubtitleCandidate) {
	// Filter candidates matching target language (uses getIdx for script-aware matching)
	targCandidates := filterByLang(candidates, []Lang{targLang})
	if len(targCandidates) > 0 {
		targ = pickBest(targCandidates)
	}

	// Filter candidates matching any reference language
	nativeCandidates := filterByLang(candidates, refLangs)
	if len(nativeCandidates) > 0 {
		native = pickBest(nativeCandidates)
	}

	return
}
