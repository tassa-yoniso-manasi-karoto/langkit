package changelog

import (
	"fmt"
	"testing"
)

// TestParseEmbeddedChangelog validates that the embedded CHANGELOG.md parses correctly
func TestParseEmbeddedChangelog(t *testing.T) {
	entries, err := Parse()
	if err != nil {
		t.Fatalf("Failed to parse embedded changelog: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("Changelog parsed but contains no entries")
	}

	// Validate each entry
	for i, entry := range entries {
		// Check version is not empty
		if entry.Version == "" {
			t.Errorf("Entry %d has empty version", i)
		}

		// Check date is not empty
		if entry.Date == "" {
			t.Errorf("Entry %d (%s) has empty date", i, entry.Version)
		}

		// Check sections have valid titles
		validSections := map[string]bool{
			"Added":      true,
			"Changed":    true,
			"Fixed":      true,
			"Deprecated": true,
			"Removed":    true,
			"Security":   true,
		}

		for _, section := range entry.Sections {
			if !validSections[section.Title] {
				t.Errorf("Entry %d (%s) has invalid section title: %q (must be Added, Changed, Fixed, Deprecated, Removed, or Security)",
					i, entry.Version, section.Title)
			}

			if len(section.Items) == 0 {
				t.Errorf("Entry %d (%s) section %q has no items",
					i, entry.Version, section.Title)
			}
		}

		// Print summary for visibility
		t.Logf("✓ Entry %d: [%s] - %s (%d sections)",
			i, entry.Version, entry.Date, len(entry.Sections))
	}
}

// TestParseStringFormat tests various format edge cases
func TestParseStringFormat(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantCount int
	}{
		{
			name: "valid single entry",
			input: `# Changelog

## [1.0.0] - 2024-01-15

### Added

- Feature one
- Feature two
`,
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "valid with unreleased",
			input: `# Changelog

## [1.1.0] - Unreleased

### Added

- New feature

## [1.0.0] - 2024-01-15

### Fixed

- Bug fix
`,
			wantErr:   false,
			wantCount: 2,
		},
		{
			name: "valid with prerelease tag",
			input: `# Changelog

## [1.0.0-alpha] - Unreleased

### Added

- Alpha feature
`,
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "empty changelog",
			input:     `# Changelog`,
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := ParseString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(entries) != tt.wantCount {
				t.Errorf("ParseString() got %d entries, want %d", len(entries), tt.wantCount)
			}
		})
	}
}

// TestUpgradeTypeDetection validates SemVer upgrade classification
func TestUpgradeTypeDetection(t *testing.T) {
	tests := []struct {
		oldVer   string
		newVer   string
		expected UpgradeType
	}{
		{"1.0.0", "2.0.0", UpgradeMajor},
		{"1.0.0", "1.1.0", UpgradeMinor},
		{"1.0.0", "1.0.1", UpgradePatch},
		{"1.0.0", "1.0.0", UpgradeNone},
		{"", "1.0.0", UpgradeMajor}, // First install
		{"1.0.0-alpha", "1.0.0", UpgradePatch}, // Prerelease to release
		{"0.9.0", "1.0.0", UpgradeMajor},
		{"1.2.3", "1.2.4", UpgradePatch},
		{"1.2.3", "1.3.0", UpgradeMinor},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s->%s", tt.oldVer, tt.newVer), func(t *testing.T) {
			got := DetermineUpgradeType(tt.oldVer, tt.newVer)
			if got != tt.expected {
				t.Errorf("DetermineUpgradeType(%q, %q) = %v, want %v",
					tt.oldVer, tt.newVer, got, tt.expected)
			}
		})
	}
}

// TestShouldShowChangelog validates display frequency logic
func TestShouldShowChangelog(t *testing.T) {
	tests := []struct {
		upgradeType UpgradeType
		frequency   string
		expected    bool
	}{
		{UpgradeMajor, "all", true},
		{UpgradeMinor, "all", true},
		{UpgradePatch, "all", true},
		{UpgradeMajor, "medium_major", true},
		{UpgradeMinor, "medium_major", true},
		{UpgradePatch, "medium_major", false},
		{UpgradeMajor, "major_only", true},
		{UpgradeMinor, "major_only", false},
		{UpgradePatch, "major_only", false},
		{UpgradeNone, "all", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v/%s", tt.upgradeType, tt.frequency), func(t *testing.T) {
			got := ShouldShowChangelog(tt.upgradeType, tt.frequency)
			if got != tt.expected {
				t.Errorf("ShouldShowChangelog(%v, %q) = %v, want %v",
					tt.upgradeType, tt.frequency, got, tt.expected)
			}
		})
	}
}
