package version

import (
	"encoding/json"
	"fmt"
)

// These variables are intended to be set via -ldflags at build time.
var (
	Version = "dev"
	Commit = "unknown"
	Branch = "unknown"
)

type Info struct {
	Version   string    `json:"version"`
	Commit    string    `json:"commit"`
	Branch    string    `json:"branch"`
}

func GetInfo() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		Branch:    Branch,
	}
}

func GetVersionInfo() string {
	info := GetInfo()
	return fmt.Sprintf(
		"Version: %s\nCommit: %s\nBranch: %s\n",
		info.Version,
		info.Commit,
		info.Branch,
	)
}

// ToJSON marshals the version info into a pretty-printed JSON string.
func ToJSON() (string, error) {
	info := GetInfo()
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
