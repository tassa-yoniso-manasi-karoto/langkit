package changelog

import (
	"bufio"
	"embed"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

//go:embed CHANGELOG.md
var embeddedChangelog embed.FS

// Section represents a changelog section (Added, Changed, Fixed, etc.)
type Section struct {
	Title string   `json:"title"`
	Items []string `json:"items"`
}

// Entry represents a single version's changelog entry
type Entry struct {
	Version  string    `json:"version"`
	Date     string    `json:"date"`
	Sections []Section `json:"sections"`
}

// UpgradeType represents the type of version upgrade
type UpgradeType string

const (
	UpgradeNone  UpgradeType = "none"
	UpgradePatch UpgradeType = "patch"
	UpgradeMinor UpgradeType = "minor"
	UpgradeMajor UpgradeType = "major"
)

var (
	// Regex patterns for parsing
	versionHeaderRegex = regexp.MustCompile(`^## \[([^\]]+)\](?: - (.+))?$`)
	sectionHeaderRegex = regexp.MustCompile(`^### (.+)$`)
	listItemRegex      = regexp.MustCompile(`^- (.+)$`)
)

// Parse parses the embedded CHANGELOG.md file and returns all entries
func Parse() ([]Entry, error) {
	content, err := embeddedChangelog.ReadFile("CHANGELOG.md")
	if err != nil {
		return nil, err
	}

	return ParseString(string(content))
}

// ParseString parses a changelog string and returns all entries
func ParseString(content string) ([]Entry, error) {
	var entries []Entry
	var currentEntry *Entry
	var currentSection *Section

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		// Check for version header
		if matches := versionHeaderRegex.FindStringSubmatch(line); matches != nil {
			// Save previous entry if exists
			if currentEntry != nil {
				if currentSection != nil && len(currentSection.Items) > 0 {
					currentEntry.Sections = append(currentEntry.Sections, *currentSection)
				}
				entries = append(entries, *currentEntry)
			}

			date := "Unreleased"
			if len(matches) > 2 && matches[2] != "" {
				date = strings.TrimSpace(matches[2])
			}

			currentEntry = &Entry{
				Version:  matches[1],
				Date:     date,
				Sections: []Section{},
			}
			currentSection = nil
			continue
		}

		// Check for section header
		if matches := sectionHeaderRegex.FindStringSubmatch(line); matches != nil {
			// Save previous section if exists
			if currentSection != nil && len(currentSection.Items) > 0 && currentEntry != nil {
				currentEntry.Sections = append(currentEntry.Sections, *currentSection)
			}

			currentSection = &Section{
				Title: matches[1],
				Items: []string{},
			}
			continue
		}

		// Check for list item
		if matches := listItemRegex.FindStringSubmatch(line); matches != nil {
			if currentSection != nil {
				currentSection.Items = append(currentSection.Items, matches[1])
			}
			continue
		}
	}

	// Save last entry and section
	if currentEntry != nil {
		if currentSection != nil && len(currentSection.Items) > 0 {
			currentEntry.Sections = append(currentEntry.Sections, *currentSection)
		}
		entries = append(entries, *currentEntry)
	}

	return entries, scanner.Err()
}

// GetEntriesSinceVersion returns all entries since (but not including) the given version
func GetEntriesSinceVersion(entries []Entry, sinceVersion string) []Entry {
	if sinceVersion == "" {
		return entries
	}

	var result []Entry
	sinceSemver, errSince := semver.NewVersion(sinceVersion)

	for _, entry := range entries {
		entrySemver, errEntry := semver.NewVersion(entry.Version)

		// If we can parse both versions, compare them
		if errSince == nil && errEntry == nil {
			if entrySemver.GreaterThan(sinceSemver) {
				result = append(result, entry)
			}
		} else {
			// Fallback: include if versions don't match (string comparison)
			if entry.Version != sinceVersion {
				result = append(result, entry)
			}
		}
	}

	return result
}

// GetEntryForVersion returns the changelog entry for a specific version
func GetEntryForVersion(entries []Entry, version string) *Entry {
	for _, entry := range entries {
		if entry.Version == version {
			return &entry
		}
	}
	return nil
}

// DetermineUpgradeType compares two versions and returns the upgrade type
func DetermineUpgradeType(oldVersion, newVersion string) UpgradeType {
	if oldVersion == "" || newVersion == "" {
		if newVersion != "" {
			return UpgradeMajor // First install treated as major
		}
		return UpgradeNone
	}

	oldSemver, errOld := semver.NewVersion(oldVersion)
	newSemver, errNew := semver.NewVersion(newVersion)

	if errOld != nil || errNew != nil {
		// Can't parse, do string comparison
		if oldVersion != newVersion {
			return UpgradeMinor // Default to minor if can't determine
		}
		return UpgradeNone
	}

	if !newSemver.GreaterThan(oldSemver) {
		return UpgradeNone
	}

	// Compare major versions
	if newSemver.Major() > oldSemver.Major() {
		return UpgradeMajor
	}

	// Compare minor versions
	if newSemver.Minor() > oldSemver.Minor() {
		return UpgradeMinor
	}

	// Must be patch
	return UpgradePatch
}

// ShouldShowChangelog determines if changelog should be shown based on upgrade type and frequency setting
func ShouldShowChangelog(upgradeType UpgradeType, frequency string) bool {
	if upgradeType == UpgradeNone {
		return false
	}

	switch frequency {
	case "all":
		return true
	case "minor_major":
		return upgradeType == UpgradeMajor || upgradeType == UpgradeMinor
	case "major_only":
		return upgradeType == UpgradeMajor
	default:
		return upgradeType == UpgradeMajor || upgradeType == UpgradeMinor
	}
}
