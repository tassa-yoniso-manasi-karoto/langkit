package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

const profilesFileName = "expectation_profiles.yaml"

// ProfilesFile is the top-level wrapper for the YAML file.
// Uses a `profiles:` key for future extensibility.
type ProfilesFile struct {
	Profiles []ExpectationProfile `yaml:"profiles"`
}

// profilesPath returns the full path to the profiles YAML file.
func profilesPath() (string, error) {
	dir := filepath.Join(xdg.ConfigHome, "langkit")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, profilesFileName), nil
}

// LoadProfiles reads all saved profiles from disk.
// Returns an empty slice (not error) if the file doesn't exist yet.
func LoadProfiles() ([]ExpectationProfile, error) {
	path, err := profilesPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading profiles: %w", err)
	}

	var pf ProfilesFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("parsing profiles: %w", err)
	}
	return pf.Profiles, nil
}

// SaveAllProfiles writes the full profiles list to disk.
func SaveAllProfiles(profiles []ExpectationProfile) error {
	path, err := profilesPath()
	if err != nil {
		return err
	}

	pf := ProfilesFile{Profiles: profiles}
	data, err := yaml.Marshal(&pf)
	if err != nil {
		return fmt.Errorf("marshaling profiles: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// GetProfile returns a saved profile by name, or nil if not found.
func GetProfile(name string) (*ExpectationProfile, error) {
	profiles, err := LoadProfiles()
	if err != nil {
		return nil, err
	}
	for i := range profiles {
		if profiles[i].Name == name {
			return &profiles[i], nil
		}
	}
	return nil, nil
}

// SaveProfile upserts a profile: updates if a profile with the same
// name exists, appends otherwise.
func SaveProfile(profile ExpectationProfile) error {
	if profile.Name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	profiles, err := LoadProfiles()
	if err != nil {
		return err
	}

	found := false
	for i := range profiles {
		if profiles[i].Name == profile.Name {
			profiles[i] = profile
			found = true
			break
		}
	}
	if !found {
		profiles = append(profiles, profile)
	}

	return SaveAllProfiles(profiles)
}

// DeleteProfile removes a profile by name. Returns an error if the
// profile doesn't exist.
func DeleteProfile(name string) error {
	profiles, err := LoadProfiles()
	if err != nil {
		return err
	}

	found := false
	var remaining []ExpectationProfile
	for _, p := range profiles {
		if p.Name == name {
			found = true
			continue
		}
		remaining = append(remaining, p)
	}

	if !found {
		return fmt.Errorf("profile %q not found", name)
	}

	return SaveAllProfiles(remaining)
}
