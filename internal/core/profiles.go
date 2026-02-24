package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/rs/zerolog"
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
func LoadProfiles(log zerolog.Logger) ([]ExpectationProfile, error) {
	path, err := profilesPath()
	if err != nil {
		return nil, err
	}

	log.Debug().Str("path", path).Msg("Loading profiles")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug().Msg("No profiles file yet")
			return nil, nil
		}
		log.Error().Err(err).Str("path", path).Msg("Failed to read profiles")
		return nil, fmt.Errorf("reading profiles: %w", err)
	}

	var pf ProfilesFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to parse profiles")
		return nil, fmt.Errorf("parsing profiles: %w", err)
	}
	log.Debug().Int("count", len(pf.Profiles)).Msg("Profiles loaded")
	return pf.Profiles, nil
}

// SaveAllProfiles writes the full profiles list to disk.
func SaveAllProfiles(profiles []ExpectationProfile, log zerolog.Logger) error {
	path, err := profilesPath()
	if err != nil {
		return err
	}

	pf := ProfilesFile{Profiles: profiles}
	data, err := yaml.Marshal(&pf)
	if err != nil {
		return fmt.Errorf("marshaling profiles: %w", err)
	}

	log.Debug().Str("path", path).Int("count", len(profiles)).Msg("Saving profiles")
	return os.WriteFile(path, data, 0644)
}

// GetProfile returns a saved profile by name, or nil if not found.
func GetProfile(name string, log zerolog.Logger) (*ExpectationProfile, error) {
	profiles, err := LoadProfiles(log)
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
func SaveProfile(profile ExpectationProfile, log zerolog.Logger) error {
	if profile.Name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	profiles, err := LoadProfiles(log)
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

	action := "updated"
	if !found {
		action = "created"
	}
	log.Debug().Str("name", profile.Name).Str("action", action).Msg("Saving profile")
	return SaveAllProfiles(profiles, log)
}

// DeleteProfile removes a profile by name. Returns an error if the
// profile doesn't exist.
func DeleteProfile(name string, log zerolog.Logger) error {
	profiles, err := LoadProfiles(log)
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

	log.Debug().Str("name", name).Msg("Deleting profile")
	return SaveAllProfiles(remaining, log)
}
