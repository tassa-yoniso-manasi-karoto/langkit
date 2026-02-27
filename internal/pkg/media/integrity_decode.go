package media

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
)

// IntegrityDepth controls how thoroughly decode integrity is checked.
type IntegrityDepth string

const (
	// IntegritySampled probes 3 sample windows (start, mid, end) per stream.
	IntegritySampled IntegrityDepth = "sampled"
	// IntegrityFull decodes the entire audio stream; video is still sampled.
	IntegrityFull IntegrityDepth = "full"
)

// ValidIntegrityDepth returns true if depth is a recognized value.
func ValidIntegrityDepth(depth IntegrityDepth) bool {
	return depth == IntegritySampled || depth == IntegrityFull
}

// NormalizeIntegrityDepth returns the depth if valid, otherwise IntegritySampled.
func NormalizeIntegrityDepth(raw string) IntegrityDepth {
	d := IntegrityDepth(raw)
	if ValidIntegrityDepth(d) {
		return d
	}
	return IntegritySampled
}

// DecodeScope specifies which streams to check.
type DecodeScope struct {
	AudioStreamIndices []int // 0-based StreamOrder values for -map
	CheckVideo         bool
}

// DecodeCheckResult holds the outcome of a single decode probe.
type DecodeCheckResult struct {
	Corrupted   bool
	ErrorOutput string
	StreamIndex int // which stream failed (-1 if video)
}

// sampleWindow describes one seek-based decode window.
type sampleWindow struct {
	seekSec     float64
	durationSec float64
}

// CheckDecodeIntegrity runs FFmpeg decode probes on the given file
// according to the requested depth and scope. Results are cached
// persistently on disk so repeated checks on unchanged files are free.
// Execution failures (e.g. FFmpeg binary missing) are surfaced as
// corrupted results with descriptive error output so they are never
// silently treated as clean.
func CheckDecodeIntegrity(ctx context.Context, path string, depth IntegrityDepth, scope DecodeScope) ([]DecodeCheckResult, error) {
	// Stat file for cache key
	stat, statErr := os.Stat(path)
	var mtime, size int64
	if statErr == nil {
		mtime = stat.ModTime().UnixNano()
		size = stat.Size()
	}
	canCache := statErr == nil

	var results []DecodeCheckResult

	// Get file duration for sample-point calculation (only needed for actual decodes).
	var totalDur float64
	var durErr error
	durProbed := false

	// Audio streams
	for _, idx := range scope.AudioStreamIndices {
		// Try cache first
		if canCache {
			cacheMu.Lock()
			cached, ok := lookupCached(path, mtime, size, idx, depth)
			cacheMu.Unlock()
			if ok {
				results = append(results, *cached)
				continue
			}
		}

		// Cache miss — probe duration lazily (only once per call)
		if !durProbed {
			totalDur, durErr = ProbeDuration(path)
			durProbed = true
		}

		var res DecodeCheckResult
		if depth == IntegrityFull {
			res = decodeFull(ctx, path, idx, false)
		} else {
			res = decodeStreamSampled(ctx, path, idx, false, totalDur, durErr)
		}
		results = append(results, res)

		// Cache the result (skip if context was cancelled — partial results are unreliable)
		if canCache && ctx.Err() == nil {
			cacheMu.Lock()
			storeCached(path, mtime, size, idx, depth, res)
			cacheMu.Unlock()
		}
	}

	// Video (always sampled, even in "full" mode)
	if scope.CheckVideo {
		// Try cache first
		if canCache {
			cacheMu.Lock()
			cached, ok := lookupCached(path, mtime, size, -1, IntegritySampled)
			cacheMu.Unlock()
			if ok {
				results = append(results, *cached)
				goto done
			}
		}

		// Cache miss
		if !durProbed {
			totalDur, durErr = ProbeDuration(path)
			durProbed = true
		}

		res := decodeStreamSampled(ctx, path, -1, true, totalDur, durErr)
		results = append(results, res)

		if canCache && ctx.Err() == nil {
			cacheMu.Lock()
			storeCached(path, mtime, size, -1, IntegritySampled, res)
			cacheMu.Unlock()
		}
	}

done:
	// Persist cache to disk
	if canCache {
		cacheMu.Lock()
		saveCache()
		cacheMu.Unlock()
	}

	return results, nil
}

// decodeStreamSampled runs sampled decode on a single stream (audio or video).
func decodeStreamSampled(ctx context.Context, path string, streamIdx int, isVideo bool, totalDur float64, durErr error) DecodeCheckResult {
	windows := buildSampleWindows(totalDur, durErr)
	for _, w := range windows {
		res := decodeSample(ctx, path, streamIdx, isVideo, w)
		if res.Corrupted {
			return res
		}
	}
	idx := streamIdx
	if isVideo {
		idx = -1
	}
	return DecodeCheckResult{StreamIndex: idx}
}

// buildSampleWindows returns the 3 sample points: start, mid, near-end.
// If duration is unknown, falls back to a single start window.
func buildSampleWindows(totalDur float64, durErr error) []sampleWindow {
	const windowLen = 20.0 // seconds per sample

	if durErr != nil || totalDur <= 0 {
		// Duration unknown: just probe the start
		return []sampleWindow{{seekSec: 0, durationSec: windowLen}}
	}

	if totalDur <= windowLen*3 {
		// Short file: just decode the whole thing
		return []sampleWindow{{seekSec: 0, durationSec: totalDur}}
	}

	midPoint := totalDur / 2.0
	endPoint := totalDur - windowLen
	if endPoint < 0 {
		endPoint = 0
	}

	return []sampleWindow{
		{seekSec: 0, durationSec: windowLen},
		{seekSec: midPoint, durationSec: windowLen},
		{seekSec: endPoint, durationSec: windowLen},
	}
}

// decodeFull runs a full decode of a single stream.
func decodeFull(ctx context.Context, path string, streamIdx int, isVideo bool) DecodeCheckResult {
	args := baseDecodeArgs()
	args = append(args, "-i", path)
	args = append(args, mapAndFilterArgs(streamIdx, isVideo)...)
	args = append(args, "-f", "null", "-")

	idx := streamIdx
	if isVideo {
		idx = -1
	}

	stderr, execErr := runFFmpegDecode(ctx, args)
	if execErr != nil {
		return DecodeCheckResult{
			Corrupted:   true,
			ErrorOutput: "FFmpeg execution failed: " + execErr.Error(),
			StreamIndex: idx,
		}
	}

	return DecodeCheckResult{
		Corrupted:   isCorruptionDetected(stderr),
		ErrorOutput: stderr,
		StreamIndex: idx,
	}
}

// decodeSample runs a windowed decode of a single stream.
func decodeSample(ctx context.Context, path string, streamIdx int, isVideo bool, w sampleWindow) DecodeCheckResult {
	args := baseDecodeArgs()
	// -ss before -i for fast seek
	args = append(args, "-ss", formatSeconds(w.seekSec))
	args = append(args, "-i", path)
	args = append(args, "-t", formatSeconds(w.durationSec))
	args = append(args, mapAndFilterArgs(streamIdx, isVideo)...)
	args = append(args, "-f", "null", "-")

	idx := streamIdx
	if isVideo {
		idx = -1
	}

	stderr, execErr := runFFmpegDecode(ctx, args)
	if execErr != nil {
		return DecodeCheckResult{
			Corrupted:   true,
			ErrorOutput: "FFmpeg execution failed: " + execErr.Error(),
			StreamIndex: idx,
		}
	}

	return DecodeCheckResult{
		Corrupted:   isCorruptionDetected(stderr),
		ErrorOutput: stderr,
		StreamIndex: idx,
	}
}

// baseDecodeArgs returns the common flags for decode probing.
func baseDecodeArgs() []string {
	return []string{
		"-hide_banner",
		"-v", "error",
		"-xerror",
		"-err_detect", "explode+crccheck",
	}
}

// mapAndFilterArgs returns -map and stream-type filter flags.
func mapAndFilterArgs(streamIdx int, isVideo bool) []string {
	if isVideo {
		return []string{
			"-map", "0:v:0",
			"-an", "-sn", "-dn",
		}
	}
	return []string{
		"-map", fmt.Sprintf("0:%d", streamIdx),
		"-vn", "-sn", "-dn",
	}
}

// runFFmpegDecode executes FFmpeg with the given args and returns
// (stderr output, exec error). A non-nil exec error means the binary
// could not be launched (missing, permission denied, etc.) — distinct
// from a non-zero exit code caused by corrupt input, which shows up
// as stderr content with a nil error.
func runFFmpegDecode(ctx context.Context, args []string) (string, error) {
	cmd := executils.CommandContext(ctx, FFmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	stderrStr := stderr.String()

	// Distinguish exec-level failure (binary missing, permission denied)
	// from FFmpeg returning non-zero due to corrupt input.
	// If stderr has content, FFmpeg ran and reported errors — that's a
	// decode finding, not an exec failure.
	if err != nil && strings.TrimSpace(stderrStr) == "" {
		return "", fmt.Errorf("ffmpeg process error: %w", err)
	}
	return stderrStr, nil
}

// isCorruptionDetected checks stderr for corruption indicators.
func isCorruptionDetected(stderr string) bool {
	if strings.TrimSpace(stderr) == "" {
		return false
	}
	// Any error output from -v error -xerror is a corruption signal
	return true
}

// formatSeconds formats a float64 as a string suitable for FFmpeg -ss/-t.
func formatSeconds(sec float64) string {
	return strconv.FormatFloat(sec, 'f', 3, 64)
}
