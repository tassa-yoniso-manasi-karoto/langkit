package core

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/progress"
)

// TestComputeImportanceMap_FromDocument parses the ground truth document
// and verifies the algorithm produces correct results for all combinations.
func TestComputeImportanceMap_FromDocument(t *testing.T) {
	// Find the document relative to this test file
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Could not determine test file location")
	}
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..")
	docPath := filepath.Join(projectRoot, "docs", "progress_bar_importance_mapping.md")

	file, err := os.Open(docPath)
	if err != nil {
		t.Fatalf("Could not open document: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Parse state
	var currentMode Mode
	var currentBulk bool
	var headers []string
	inTable := false

	// Regex patterns
	metadataRE := regexp.MustCompile(`<!--\s*MODE:\s*(\w+),\s*BULK:\s*(\w+)\s*-->`)
	tableRowRE := regexp.MustCompile(`^\|(.+)\|$`)

	lineNum := 0
	hasMetadata := false // Track if current section has metadata
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Check for metadata comment
		if matches := metadataRE.FindStringSubmatch(line); matches != nil {
			modeStr := strings.ToLower(matches[1])
			bulkStr := strings.ToLower(matches[2])

			currentMode = parseModeString(modeStr)
			currentBulk = bulkStr == "true"
			inTable = false
			headers = nil
			hasMetadata = true // This section has metadata
			continue
		}

		// Check for section headers (##) which reset metadata state
		if strings.HasPrefix(line, "##") {
			hasMetadata = false
			inTable = false
			headers = nil
			continue
		}

		// Skip tables that don't have metadata (like the rule examples)
		if !hasMetadata {
			continue
		}

		// Check for table row
		if matches := tableRowRE.FindStringSubmatch(line); matches != nil {
			cells := splitTableRow(matches[1])

			// Skip separator rows (containing only dashes)
			if isTableSeparator(cells) {
				continue
			}

			// First non-separator row after metadata is the header
			if headers == nil {
				headers = cells
				inTable = true
				continue
			}

			// This is a data row
			if inTable && len(cells) == len(headers) {
				testTableRow(t, lineNum, currentMode, currentBulk, headers, cells)
			}
		} else {
			// Non-table line resets table state
			if inTable {
				inTable = false
				headers = nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading document: %v", err)
	}
}

func parseModeString(s string) Mode {
	switch s {
	case "enhance":
		return Enhance
	case "translit":
		return Translit
	case "condense":
		return Condense
	case "subs2dubs":
		return Subs2Dubs
	case "subs2cards":
		return Subs2Cards
	default:
		return Subs2Cards // Default
	}
}

func splitTableRow(row string) []string {
	parts := strings.Split(row, "|")
	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

func isTableSeparator(cells []string) bool {
	for _, c := range cells {
		trimmed := strings.Trim(c, "- ")
		if trimmed != "" {
			return false
		}
	}
	return true
}

func testTableRow(t *testing.T, lineNum int, mode Mode, isBulk bool, headers, cells []string) {
	t.Helper()

	if len(cells) == 0 {
		return
	}

	// First cell is the combination name
	combination := cells[0]
	features := parseCombination(combination)

	// Compute the importance map
	importanceMap := ComputeImportanceMap(mode, isBulk, features)

	// Check each bar column
	for i := 1; i < len(headers) && i < len(cells); i++ {
		barID := normalizeBarID(headers[i])
		expectedEmoji := strings.TrimSpace(cells[i])
		expectedLevel := ParseImportanceLevel(expectedEmoji)

		// Skip NotApplicable bars (marked with "-")
		if expectedLevel == NotApplicable {
			continue
		}

		actualLevel := importanceMap.GetImportanceLevel(barID)

		if actualLevel != expectedLevel {
			t.Errorf("Line %d: %s (mode=%s, bulk=%v) bar %q: expected %s (%s), got %s (%s)",
				lineNum, combination, mode.String(), isBulk,
				barID, expectedLevel.String(), expectedLevel.HeightClass(),
				actualLevel.String(), actualLevel.HeightClass())
		}
	}
}

func parseCombination(combo string) FeatureSet {
	parts := strings.Split(combo, "+")
	features := FeatureSet{}

	for _, p := range parts {
		switch strings.TrimSpace(p) {
		case "enhance":
			features.HasEnhance = true
		case "translit":
			features.HasTranslit = true
		case "condense":
			features.HasCondense = true
		case "stt":
			features.HasSTT = true
		}
	}

	return features
}

func normalizeBarID(header string) string {
	// Map table headers to bar ID constants
	headerMap := map[string]string{
		"media-bar":          progress.BarMediaBar,
		"item-bar":           progress.BarItemBar,
		"demucs-process":     progress.BarDemucsProcess,
		"demucs-docker-dl":   progress.BarDemucsDockerDL,
		"demucs-model-dl":    progress.BarDemucsModelDL,
		"audiosep-process":   progress.BarAudioSepProcess,
		"audiosep-docker-dl": progress.BarAudioSepDockerDL,
		"audiosep-model-dl":  progress.BarAudioSepModelDL,
		"translit-process":   progress.BarTranslitProcess,
		"translit-docker-dl": progress.BarTranslitDockerDL,
		"translit-init":      progress.BarTranslitInit,
	}

	normalized := strings.TrimSpace(header)
	if barID, ok := headerMap[normalized]; ok {
		return barID
	}
	return normalized
}

// TestImportanceLevelHeightClass verifies height class mappings.
func TestImportanceLevelHeightClass(t *testing.T) {
	tests := []struct {
		level    ImportanceLevel
		expected string
	}{
		{VeryImportant, "h-5"},
		{Important, "h-4"},
		{Normal, "h-3"},
		{LowImportance, "h-2"},
		{VeryLowImportance, "h-1"},
		{NotApplicable, "h-3"}, // Default to Normal
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			if got := tt.level.HeightClass(); got != tt.expected {
				t.Errorf("HeightClass() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestImportanceMapGetHeightClass verifies prefix matching.
func TestImportanceMapGetHeightClass(t *testing.T) {
	m := ImportanceMap{
		progress.BarDemucsProcess: VeryImportant,
		progress.BarItemBar:       Important,
	}

	tests := []struct {
		barID    string
		expected string
	}{
		{progress.BarDemucsProcess, "h-5"},
		{"demucs-process-12345", "h-5"},       // Prefix match
		{progress.BarItemBar, "h-4"},
		{"unknown-bar", "h-3"},                // Default
	}

	for _, tt := range tests {
		t.Run(tt.barID, func(t *testing.T) {
			if got := m.GetHeightClass(tt.barID); got != tt.expected {
				t.Errorf("GetHeightClass(%q) = %v, want %v", tt.barID, got, tt.expected)
			}
		})
	}
}

// TestParseImportanceLevel verifies emoji parsing.
func TestParseImportanceLevel(t *testing.T) {
	tests := []struct {
		emoji    string
		expected ImportanceLevel
	}{
		{"🟥", VeryImportant},
		{"🟧", Important},
		{"🟨", Normal},
		{"🟩", LowImportance},
		{"🟦", VeryLowImportance},
		{"-", NotApplicable},
		{"", NotApplicable},
		{"unknown", NotApplicable},
	}

	for _, tt := range tests {
		t.Run(tt.emoji, func(t *testing.T) {
			if got := ParseImportanceLevel(tt.emoji); got != tt.expected {
				t.Errorf("ParseImportanceLevel(%q) = %v, want %v", tt.emoji, got, tt.expected)
			}
		})
	}
}

// TestComputeImportanceMap_SingleEnhance verifies the simplest case.
func TestComputeImportanceMap_SingleEnhance(t *testing.T) {
	m := ComputeImportanceMap(Enhance, false, FeatureSet{HasEnhance: true})

	// Single file Enhance mode: demucs-process should be VeryImportant
	if level := m[progress.BarDemucsProcess]; level != VeryImportant {
		t.Errorf("demucs-process: expected %s, got %s", VeryImportant.String(), level.String())
	}

	// demucs-docker-dl should be Important (one below processing)
	if level := m[progress.BarDemucsDockerDL]; level != Important {
		t.Errorf("demucs-docker-dl: expected %s, got %s", Important.String(), level.String())
	}

	// demucs-model-dl should be Normal (one below docker-dl)
	if level := m[progress.BarDemucsModelDL]; level != Normal {
		t.Errorf("demucs-model-dl: expected %s, got %s", Normal.String(), level.String())
	}
}

// TestComputeImportanceMap_BulkEnhance verifies bulk mode shifts.
func TestComputeImportanceMap_BulkEnhance(t *testing.T) {
	m := ComputeImportanceMap(Enhance, true, FeatureSet{HasEnhance: true})

	// Bulk mode: media-bar should be VeryImportant
	if level := m[progress.BarMediaBar]; level != VeryImportant {
		t.Errorf("media-bar: expected %s, got %s", VeryImportant.String(), level.String())
	}

	// demucs-process should be Important (shifted down from VeryImportant)
	if level := m[progress.BarDemucsProcess]; level != Important {
		t.Errorf("demucs-process: expected %s, got %s", Important.String(), level.String())
	}

	// demucs-docker-dl should be Normal
	if level := m[progress.BarDemucsDockerDL]; level != Normal {
		t.Errorf("demucs-docker-dl: expected %s, got %s", Normal.String(), level.String())
	}

	// demucs-model-dl should be LowImportance (one below docker-dl)
	if level := m[progress.BarDemucsModelDL]; level != LowImportance {
		t.Errorf("demucs-model-dl: expected %s, got %s", LowImportance.String(), level.String())
	}
}
