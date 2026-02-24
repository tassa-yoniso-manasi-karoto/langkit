package core

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

// DefaultVideoExtensions are the extensions that Routing() processes.
var DefaultVideoExtensions = []string{".mp4", ".mkv"}

// DiscoverMediaFiles walks the given path and returns all media files
// matching the provided extensions, applying the same skip rules as
// Routing(): .media directories are skipped, and Langkit-generated
// merged outputs are excluded.
//
// If path is a regular file, it is returned as-is (no filtering).
// If extensions is nil, DefaultVideoExtensions is used.
func DiscoverMediaFiles(root string, extensions []string, log zerolog.Logger) ([]string, error) {
	stat, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return []string{root}, nil
	}

	if extensions == nil {
		extensions = DefaultVideoExtensions
	}

	extSet := make(map[string]bool, len(extensions))
	for _, ext := range extensions {
		extSet[strings.ToLower(ext)] = true
	}

	var files []string
	skippedMediaDirs := 0
	skippedMerged := 0
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && strings.HasSuffix(info.Name(), ".media") {
			skippedMediaDirs++
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		filename := info.Name()
		ext := strings.ToLower(filepath.Ext(filename))
		if !extSet[ext] {
			return nil
		}
		if isLangkitMadeMergedOutput(filename) {
			skippedMerged++
			return nil
		}
		files = append(files, path)
		return nil
	})

	log.Debug().
		Str("root", root).
		Strs("extensions", extensions).
		Int("accepted", len(files)).
		Int("skippedMediaDirs", skippedMediaDirs).
		Int("skippedMergedOutputs", skippedMerged).
		Msg("Discovery walk complete")

	return files, err
}
