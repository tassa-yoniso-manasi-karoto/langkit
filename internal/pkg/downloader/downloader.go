package downloader

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// GitHubRelease represents the structure of a GitHub release API response.
type GitHubRelease struct {
	Assets []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// GetDownloadURLForAsset fetches the download URL for an asset from a GitHub repository's latest release based on a set of keywords.
func GetDownloadURLForAsset(repo string, keywords []string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := http.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("failed to query GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status from GitHub API: %s", resp.Status)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse GitHub release JSON: %w", err)
	}

	for _, asset := range release.Assets {
		matches := true
		for _, keyword := range keywords {
			if !strings.Contains(asset.Name, keyword) {
				matches = false
				break
			}
		}
		if matches {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("asset matching keywords '%v' not found in repo '%s'", keywords, repo)
}

// ProgressReader is a wrapper around an io.Reader that reports download progress.
type ProgressReader struct {
	Reader    io.Reader
	Total     int64
	Current   int64
	startTime time.Time
	Handler   func(p float64, read, total int64, speed float64)
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	if pr.startTime.IsZero() {
		pr.startTime = time.Now()
	}

	n, err := pr.Reader.Read(p)
	if n > 0 {
		pr.Current += int64(n)
		elapsed := time.Since(pr.startTime).Seconds()
		speed := float64(pr.Current) / elapsed
		progress := float64(pr.Current) / float64(pr.Total) * 100
		pr.Handler(progress, pr.Current, pr.Total, speed)
	}
	return n, err
}

// ExtractZip extracts specific files from a zip archive to a destination directory.
func ExtractZip(zipPath, destDir string, filesToExtract []string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip archive: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		for _, fileToExtract := range filesToExtract {
			// Check if the file inside the zip matches what we want to extract
			if filepath.Base(f.Name) == fileToExtract {
				outFile, err := os.OpenFile(
					filepath.Join(destDir, filepath.Base(f.Name)),
					os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
					f.Mode(),
				)
				if err != nil {
					return fmt.Errorf("failed to create destination file: %w", err)
				}

				rc, err := f.Open()
				if err != nil {
					return fmt.Errorf("failed to open file in zip: %w", err)
				}

				_, err = io.Copy(outFile, rc)

				// Close files
				outFile.Close()
				rc.Close()

				if err != nil {
					return fmt.Errorf("failed to copy file content: %w", err)
				}

				// On macOS/Linux, set executable permissions
				if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
					if err := os.Chmod(outFile.Name(), 0755); err != nil {
						return fmt.Errorf("failed to set executable permissions: %w", err)
					}
				}
			}
		}
	}
	return nil
}
