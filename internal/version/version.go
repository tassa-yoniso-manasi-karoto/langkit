package version

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
)

const githubAPI = "https://api.github.com/repos/tassa-yoniso-manasi-karoto/langkit/releases/latest"

// Variables set via -ldflags at build time.
var (
	Version = "dev"
	Commit  = "unknown"
	Branch  = "unknown"
	
	infoInstance Info
	infoMutex    sync.RWMutex
)

type Info struct {
	Version               string `json:"version"`
	Commit                string `json:"commit"`
	Branch                string `json:"branch"`
	NewerVersionAvailable bool   `json:"newerVersionAvailable"`
}

func init() {
	// Initialize with local info immediately
	infoMutex.Lock()
	infoInstance = Info{
		Version: Version,
		Commit:  Commit,
		Branch:  Branch,
	}
	infoMutex.Unlock()
	
	// Check for updates asynchronously
	go func() {
		// Only check GitHub if not a dev version
		if Version != "dev" {
			remoteTag, err := getLatestVersionFromGithub()
			if err != nil {
				return
			}
			
			// Attempt to parse both local and remote versions as semver.
			localVer, errLocal := semver.NewVersion(Version)
			remoteVer, errRemote := semver.NewVersion(remoteTag)
			if errLocal == nil && errRemote == nil && remoteVer.GreaterThan(localVer) {
				infoMutex.Lock()
				infoInstance.NewerVersionAvailable = true
				infoMutex.Unlock()
			}
		}
	}()
}

func GetInfo() Info {
	infoMutex.RLock()
	defer infoMutex.RUnlock()
	return infoInstance
}

// GetInfoFromGithub() returns an Info struct populated with the current build details
// and determines whether a newer release tag exists on GitHub.
// If Version == "dev" or any error occurs, NewerVersionAvailable remains false.
func GetInfoFromGithub() Info {
	info := Info{
		Version: Version,
		Commit:  Commit,
		Branch:  Branch,
	}

	// If we're on a dev version, skip checking GitHub to avoid semver parse errors.
	if Version == "dev" {
		return info
	}

	remoteTag, err := getLatestVersionFromGithub()
	if err != nil {
		return info
	}

	// Attempt to parse both local and remote versions as semver.
	localVer, errLocal := semver.NewVersion(Version)
	remoteVer, errRemote := semver.NewVersion(remoteTag)
	if errLocal == nil && errRemote == nil && remoteVer.GreaterThan(localVer) {
		info.NewerVersionAvailable = true
	}

	return info
}

// String implements fmt.Stringer for pretty-printing on the terminal.
func (i Info) String() string {
	return fmt.Sprintf(
		"Version: %s\nCommit: %s\nBranch: %s\nNewerVersionAvailable: %v\n",
		i.Version,
		i.Commit,
		i.Branch,
		i.NewerVersionAvailable,
	)
}

// ToJSON returns the Info struct as a pretty-printed JSON string.
func (i Info) ToJSON() (string, error) {
	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}


func getLatestVersionFromGithub() (string, error) {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get(githubAPI)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}
