package media

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
)

// ParseMediaInfoDuration parses a mediainfo duration string into float64
// seconds. Mediainfo JSON output normally reports seconds (e.g.
// "1424.576"), but some configurations emit milliseconds. Values above
// 86400 (24 hours in seconds) are assumed to be milliseconds and
// converted accordingly.
// Returns (0, false) if the value is empty, "N/A", or unparseable.
func ParseMediaInfoDuration(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "N/A") {
		return 0, false
	}
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	if val <= 0 {
		return 0, false
	}
	// Heuristic: if the value exceeds 24 hours in seconds, it is
	// almost certainly milliseconds (a 24-min episode = ~1440s but
	// ~1440000ms). Convert to seconds.
	if val > 86400 {
		val /= 1000.0
	}
	return val, true
}

// ProbeDuration uses ffprobe to get a file's duration in seconds.
// Falls back to the existing GetAudioDurationSeconds if ffprobe is
// not available.
func ProbeDuration(path string) (float64, error) {
	// Try ffprobe first (machine-readable output)
	ffprobePath, err := exec.LookPath("ffprobe")
	if err == nil {
		cmd := executils.NewCommand(ffprobePath,
			"-v", "error",
			"-show_entries", "format=duration",
			"-of", "default=noprint_wrappers=1:nokey=1",
			path,
		)
		out, err := cmd.Output()
		if err == nil {
			s := strings.TrimSpace(string(out))
			if val, err := strconv.ParseFloat(s, 64); err == nil && val > 0 {
				return val, nil
			}
		}
	}
	// Fallback to ffmpeg stderr parsing
	return GetAudioDurationSeconds(path)
}

// FormatDuration formats seconds into a human-readable string like
// "23m 45s" or "1h 12m 34s".
func FormatDuration(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	return fmt.Sprintf("%dm %ds", m, s)
}
