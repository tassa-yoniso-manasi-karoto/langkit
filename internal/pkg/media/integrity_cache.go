package media

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/adrg/xdg"
)

const (
	cacheFileName = "decode_integrity.json"
	cacheVersion  = 1
)

// IntegrityCache is the persistent on-disk cache for decode integrity results.
type IntegrityCache struct {
	Version int                        `json:"version"`
	Entries map[string]*CacheFileEntry `json:"entries"` // keyed by filepath
}

// CacheFileEntry stores per-stream results for a single media file,
// keyed on mtime+size so stale entries are automatically invalidated.
type CacheFileEntry struct {
	ModTime int64                        `json:"modTime"` // UnixNano
	Size    int64                        `json:"size"`
	Streams map[int]*CacheStreamResult   `json:"streams"` // keyed by StreamIndex (-1 = video)
}

// CacheStreamResult records the outcome of a single stream decode.
type CacheStreamResult struct {
	Depth     IntegrityDepth `json:"depth"`     // "sampled" or "full"
	Corrupted bool           `json:"corrupted"`
	Error     string         `json:"error"`     // first line of FFmpeg stderr
}

var (
	cacheOnce  sync.Once
	cacheMu    sync.Mutex
	cache      *IntegrityCache
	cacheDirty bool
)

// cachePath returns the full path to the cache JSON file.
func cachePath() string {
	return filepath.Join(xdg.CacheHome, "langkit", cacheFileName)
}

// loadCache reads the cache from disk. Called once per process via sync.Once.
// If the file is missing or corrupt, starts with an empty cache.
// Also prunes entries for files that no longer exist on disk.
func loadCache() {
	cache = &IntegrityCache{
		Version: cacheVersion,
		Entries: make(map[string]*CacheFileEntry),
	}

	data, err := os.ReadFile(cachePath())
	if err != nil {
		return // missing file is fine, start empty
	}

	var loaded IntegrityCache
	if err := json.Unmarshal(data, &loaded); err != nil {
		return // corrupt file, start empty
	}

	if loaded.Version != cacheVersion || loaded.Entries == nil {
		return // incompatible version, start fresh
	}

	cache = &loaded
	pruneCache()
}

// pruneCache removes entries for files that no longer exist on disk.
func pruneCache() {
	for path := range cache.Entries {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			delete(cache.Entries, path)
			cacheDirty = true
		}
	}
}

// saveCache writes the cache to disk if it has been modified.
func saveCache() {
	if !cacheDirty {
		return
	}

	dir := filepath.Dir(cachePath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return // best effort
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return
	}

	_ = os.WriteFile(cachePath(), data, 0644)
	cacheDirty = false
}

// FlushIntegrityCache explicitly persists the cache to disk.
func FlushIntegrityCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheOnce.Do(loadCache)
	saveCache()
}

// depthSatisfies returns true if the cached depth is at least as thorough
// as the requested depth. A "full" cached result satisfies a "sampled"
// request (it's a superset), but not vice versa.
func depthSatisfies(cached, requested IntegrityDepth) bool {
	if cached == requested {
		return true
	}
	return cached == IntegrityFull && requested == IntegritySampled
}

// lookupCached returns a cached result for a specific stream if the
// cache entry is valid (mtime+size match) and the cached depth satisfies
// the requested depth.
func lookupCached(path string, mtime, size int64, streamIdx int, requestedDepth IntegrityDepth) (*DecodeCheckResult, bool) {
	cacheOnce.Do(loadCache)

	entry, ok := cache.Entries[path]
	if !ok {
		return nil, false
	}

	if entry.ModTime != mtime || entry.Size != size {
		// Stale entry — file was modified
		delete(cache.Entries, path)
		cacheDirty = true
		return nil, false
	}

	stream, ok := entry.Streams[streamIdx]
	if !ok {
		return nil, false
	}

	if !depthSatisfies(stream.Depth, requestedDepth) {
		return nil, false
	}

	return &DecodeCheckResult{
		Corrupted:   stream.Corrupted,
		ErrorOutput: stream.Error,
		StreamIndex: streamIdx,
	}, true
}

// storeCached writes a decode result into the cache for a specific stream.
func storeCached(path string, mtime, size int64, streamIdx int, depth IntegrityDepth, result DecodeCheckResult) {
	cacheOnce.Do(loadCache)

	entry, ok := cache.Entries[path]
	if !ok || entry.ModTime != mtime || entry.Size != size {
		// New or stale entry — create fresh
		entry = &CacheFileEntry{
			ModTime: mtime,
			Size:    size,
			Streams: make(map[int]*CacheStreamResult),
		}
		cache.Entries[path] = entry
	}

	// Only store the first line of error output to keep the cache compact
	errStr := result.ErrorOutput
	if idx := strings.Index(errStr, "\n"); idx != -1 {
		errStr = errStr[:idx]
	}

	entry.Streams[streamIdx] = &CacheStreamResult{
		Depth:     depth,
		Corrupted: result.Corrupted,
		Error:     errStr,
	}
	cacheDirty = true
}
