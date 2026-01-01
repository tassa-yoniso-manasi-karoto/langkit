package core

import (
	"strings"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"
)

// ImportanceLevel represents the visual importance of a progress bar.
// Higher importance means larger/more prominent display in the UI.
type ImportanceLevel int

const (
	VeryImportant     ImportanceLevel = iota // 🟥 → h-5 (SVG waves, maximum prominence)
	Important                                // 🟧 → h-4
	Normal                                   // 🟨 → h-3
	LowImportance                            // 🟩 → h-2
	VeryLowImportance                        // 🟦 → h-1
	NotApplicable                            // Bar doesn't appear for this combination
)

// HeightClass returns the Tailwind height class for this importance level.
func (l ImportanceLevel) HeightClass() string {
	switch l {
	case VeryImportant:
		return "h-5"
	case Important:
		return "h-4"
	case Normal:
		return "h-3"
	case LowImportance:
		return "h-2"
	case VeryLowImportance:
		return "h-1"
	default:
		return "h-3" // Default to Normal
	}
}

// String returns the emoji representation for debugging/testing.
func (l ImportanceLevel) String() string {
	switch l {
	case VeryImportant:
		return "🟥"
	case Important:
		return "🟧"
	case Normal:
		return "🟨"
	case LowImportance:
		return "🟩"
	case VeryLowImportance:
		return "🟦"
	case NotApplicable:
		return "-"
	default:
		return "?"
	}
}

// ParseImportanceLevel converts an emoji string to ImportanceLevel.
func ParseImportanceLevel(emoji string) ImportanceLevel {
	switch strings.TrimSpace(emoji) {
	case "🟥":
		return VeryImportant
	case "🟧":
		return Important
	case "🟨":
		return Normal
	case "🟩":
		return LowImportance
	case "🟦":
		return VeryLowImportance
	case "-":
		return NotApplicable
	default:
		return NotApplicable
	}
}

// FeatureSet represents the features enabled for a processing run.
type FeatureSet struct {
	HasEnhance  bool // voiceEnhancing feature enabled
	HasTranslit bool // any subtitle romanization/tokenization feature enabled
	HasCondense bool // condensedAudio feature enabled
	HasSTT      bool // STT model selected (for subs2cards+stt combinations)
}

// ImportanceMap maps bar ID prefixes to their importance levels.
type ImportanceMap map[string]ImportanceLevel

// GetHeightClass returns the height class for a bar ID.
// Supports prefix matching: "demucs-process-12345" matches "demucs-process".
// Returns "h-3" (Normal) as default if no match found.
func (m ImportanceMap) GetHeightClass(barID string) string {
	// First try exact match
	if level, ok := m[barID]; ok {
		return level.HeightClass()
	}

	// Try prefix matching for bar IDs with suffixes
	for prefix, level := range m {
		if strings.HasPrefix(barID, prefix) {
			return level.HeightClass()
		}
	}

	// Default to Normal
	return Normal.HeightClass()
}

// GetImportanceLevel returns the importance level for a bar ID.
// Supports prefix matching. Returns NotApplicable if no match found.
func (m ImportanceMap) GetImportanceLevel(barID string) ImportanceLevel {
	// First try exact match
	if level, ok := m[barID]; ok {
		return level
	}

	// Try prefix matching
	for prefix, level := range m {
		if strings.HasPrefix(barID, prefix) {
			return level
		}
	}

	return NotApplicable
}

// demoteLevel reduces importance by one level, respecting minimum constraints.
// Processing bars never go below Normal (h-3).
// Download bars can go down to VeryLowImportance (h-1).
func demoteLevel(level ImportanceLevel, isDownload bool) ImportanceLevel {
	if level >= VeryLowImportance {
		return VeryLowImportance
	}
	demoted := level + 1
	// Processing bars never go below Normal
	if !isDownload && demoted > Normal {
		return Normal
	}
	return demoted
}

// ComputeImportanceMap implements the algorithm from docs/progress_bar_importance_mapping.md.
// It computes importance levels for all progress bars based on:
// - mode: The task mode (Subs2Cards, Subs2Dubs, Enhance, Translit, Condense)
// - isBulk: Whether processing multiple files (IsBulkProcess)
// - features: Which features are enabled
func ComputeImportanceMap(mode Mode, isBulk bool, features FeatureSet) ImportanceMap {
	m := make(ImportanceMap)

	// Check if there's a competing secondary feature that needs its own visual space
	hasCompeting := hasCompetingSecondaryFeature(mode, features)

	// Base importance levels depend on bulk mode and whether secondary features exist
	var (
		primaryProcLevel     ImportanceLevel
		primaryDLLevel       ImportanceLevel
		secondaryProcLevel   ImportanceLevel
		secondaryDLLevel     ImportanceLevel
	)

	if isBulk {
		// Bulk mode: media-bar takes top spot, everything shifts down
		m[progress.BarMediaBar] = VeryImportant
		primaryProcLevel = Important       // 🟧
		if hasCompeting {
			// When competing features exist, primary downloads get demoted further
			primaryDLLevel = LowImportance // 🟩
			secondaryProcLevel = Normal    // 🟨
			secondaryDLLevel = LowImportance // 🟩
		} else {
			primaryDLLevel = Normal        // 🟨
			secondaryProcLevel = Normal    // 🟨
			secondaryDLLevel = LowImportance // 🟩
		}
	} else {
		// Single file mode: primary feature gets maximum prominence
		primaryProcLevel = VeryImportant   // 🟥
		if hasCompeting {
			// When competing features exist, primary downloads drop to make room
			primaryDLLevel = Normal        // 🟨
			secondaryProcLevel = Important // 🟧
			secondaryDLLevel = Normal      // 🟨
		} else {
			// Single feature: primary downloads get Important
			primaryDLLevel = Important     // 🟧
			secondaryProcLevel = Important // 🟧
			secondaryDLLevel = Normal      // 🟨
		}
	}

	// Determine primary feature bars based on mode
	switch mode {
	case Enhance:
		// Demucs bars
		m[progress.BarDemucsProcess] = primaryProcLevel
		m[progress.BarDemucsDockerDL] = primaryDLLevel
		m[progress.BarDemucsModelDL] = demoteLevel(primaryDLLevel, true)
		// Audio-separator bars (alternative to Demucs, same importance levels)
		m[progress.BarAudioSepProcess] = primaryProcLevel
		m[progress.BarAudioSepDockerDL] = primaryDLLevel
		m[progress.BarAudioSepModelDL] = demoteLevel(primaryDLLevel, true)
	case Translit:
		m[progress.BarTranslitProcess] = primaryProcLevel
		m[progress.BarTranslitDockerDL] = primaryDLLevel
		m[progress.BarTranslitInit] = primaryDLLevel
	case Condense, Subs2Dubs, Subs2Cards:
		m[progress.BarItemBar] = primaryProcLevel
		// No downloads for item-bar based modes
	}

	// Handle secondary features (features enabled but not the primary mode)

	// If enhance is enabled but not primary (mode != Enhance)
	if features.HasEnhance && mode != Enhance {
		// Demucs bars
		m[progress.BarDemucsProcess] = secondaryProcLevel
		m[progress.BarDemucsDockerDL] = secondaryDLLevel
		m[progress.BarDemucsModelDL] = demoteLevel(secondaryDLLevel, true)
		// Audio-separator bars (alternative to Demucs, same importance levels)
		m[progress.BarAudioSepProcess] = secondaryProcLevel
		m[progress.BarAudioSepDockerDL] = secondaryDLLevel
		m[progress.BarAudioSepModelDL] = demoteLevel(secondaryDLLevel, true)
	}

	// If translit is enabled but not primary (mode != Translit)
	if features.HasTranslit && mode != Translit {
		m[progress.BarTranslitProcess] = secondaryProcLevel
		m[progress.BarTranslitDockerDL] = secondaryDLLevel
		m[progress.BarTranslitInit] = secondaryDLLevel
	}

	// If condense is enabled with Translit mode, item-bar appears at primary level
	if features.HasCondense && mode == Translit {
		m[progress.BarItemBar] = primaryProcLevel
	}

	return m
}

// hasCompetingSecondaryFeature checks if there's a secondary feature that requires
// its own visual space (i.e., not a sequential feature like condense with translit).
// This affects whether primary downloads get demoted.
func hasCompetingSecondaryFeature(mode Mode, features FeatureSet) bool {
	switch mode {
	case Enhance:
		// Translit processing is concurrent/competing, condense's item-bar is sequential
		return features.HasTranslit
	case Translit:
		// Enhance processing is concurrent/competing, condense's item-bar is sequential
		return features.HasEnhance
	case Condense:
		// Enhance processing is concurrent/competing
		return features.HasEnhance
	case Subs2Dubs:
		// Enhance and translit processing compete for visual space
		return features.HasEnhance || features.HasTranslit
	case Subs2Cards:
		// Enhance and translit processing compete for visual space
		return features.HasEnhance || features.HasTranslit
	default:
		return false
	}
}
