package media

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gookit/color"
	"github.com/k0kubun/pp"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
)

const OpusBitrate = "112k"

// Resample44100Soxr is a high-quality resample filter for 44.1kHz output
// Used to bypass demucs-next's poor linear interpolation resampler
var Resample44100Soxr = []string{"-af", "aresample=resampler=soxr:out_sample_rate=44100"}

var (
	FFmpegPath = "ffmpeg"
	MaxWidth   = 1000
	MaxHeight  = 562
)

func ffmpegExtractAudio(tracknum int, offset, startAt, endAt time.Duration, inFile, outFile string, outArgs []string) error {
	if exists(outFile) {
		return fs.ErrExist
	}
	/* https://stackoverflow.com/questions/18444194/cutting-multimedia-files-based-on-start-and-end-time-using-ffmpeg
	using -t after -i result in inaccurate cuts but using -to before -i fix it, resulting in the same timecode as subs2srs
	sub2srs uses -i "input.mp3" -ss 00:00:00.000 -t 00:00:01.900 format but used an old version of ffmpeg (v4)
	*/
	inArgs := []string{
		"-ss", ffmpegPosition(startAt-offset),
		"-to", ffmpegPosition(endAt+offset),
		"-i", inFile,
		"-map", fmt.Sprint("0:a:", tracknum),
	}
	outArgs = append(outArgs, outFile)

	args := []string{
		"-loglevel", "error",
	}
	args = append(args, inArgs...)
	args = append(args, outArgs...)

	return FFmpeg(args...)
}

func ffmpegExtractImage(startAt, endAt time.Duration, inFile string, outFile string) error {
	if exists(outFile) {
		return fs.ErrExist
	}

	var frameAt = startAt
	if endAt > startAt {
		frameAt = startAt + (endAt-startAt)/2
	}

	inArgs := []string{
		"-ss", ffmpegPosition(frameAt),
		"-i", inFile,
	}

	outArgs := []string{
		"-vf", fmt.Sprintf("scale=%d:%d", MaxWidth, MaxHeight),
		"-c:v", "libaom-av1",
		"-frames", "1",
		outFile,
	}
	if endAt > frameAt {
		outArgs = append([]string{"-t", ffmpegPosition(endAt - frameAt)}, outArgs...)
	}

	args := []string{
		"-loglevel", "error",
	}
	args = append(args, inArgs...)
	args = append(args, outArgs...)

	return FFmpeg(args...)
}

func ffmpegPosition(d time.Duration) string {
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond
	return fmt.Sprintf("%d.%d", s, ms)
}



// Creates the concat file for ffmpeg by listing .wav files in the directory
func CreateConcatFile(wavFiles []string) (string, error) {
	// Create a temporary file to store the concat list
	concatFile, err := os.CreateTemp("", "ffmpeg_concat_*.txt")
	if err != nil {
		return "", fmt.Errorf("error creating temporary file: %w", err)
	}
	defer concatFile.Close()

	// Write the list of .wav files in ffmpeg concat format
	for _, wavFile := range wavFiles {
		line := fmt.Sprintf("file '%s'\n", wavFile)
		if _, err := concatFile.WriteString(line); err != nil {
			return "", fmt.Errorf("error writing to concat file: %w", err)
		}
	}

	return concatFile.Name(), nil
}

// Runs FFmpeg concat command with the provided concat file and output wav file
func RunFFmpegConcat(concatFile, outputWav string) error {
	return FFmpeg([]string{"-loglevel", "error", "-f", "concat", "-safe", "0", "-i", concatFile, "-c", "copy", outputWav}...)
}

// GetAudioDurationSeconds returns the duration of an audio file in seconds using ffmpeg
func GetAudioDurationSeconds(filePath string) (float64, error) {
	// Use ffmpeg -i to get file info (duration is in stderr)
	cmd := executils.NewCommand(FFmpegPath, "-i", filePath, "-hide_banner", "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run() // ffmpeg returns error for -f null, ignore it

	// Parse "Duration: HH:MM:SS.ms" from output
	outputStr := stderr.String()
	durationIdx := strings.Index(outputStr, "Duration: ")
	if durationIdx == -1 {
		return 0, fmt.Errorf("could not find duration in ffmpeg output")
	}

	// Extract duration string (format: "HH:MM:SS.ms")
	durationStart := durationIdx + len("Duration: ")
	commaIdx := strings.Index(outputStr[durationStart:], ",")
	if commaIdx == -1 {
		return 0, fmt.Errorf("could not parse duration format")
	}
	durationStr := outputStr[durationStart : durationStart+commaIdx]

	// Parse HH:MM:SS.ms format
	parts := strings.Split(durationStr, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("unexpected duration format: %s", durationStr)
	}

	hours, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse hours: %w", err)
	}
	minutes, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse minutes: %w", err)
	}
	seconds, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse seconds: %w", err)
	}

	return hours*3600 + minutes*60 + seconds, nil
}

// SplitAudioFile splits an audio file into segments of specified duration in seconds.
// Returns paths to the segment files sorted in order.
func SplitAudioFile(inputPath string, segmentSeconds int, outputDir string) ([]string, error) {
	ext := filepath.Ext(inputPath)

	// Use simple prefix to avoid glob issues with special chars in original filename
	segmentPattern := filepath.Join(outputDir, "seg_%03d"+ext)

	args := []string{
		"-loglevel", "error",
		"-i", inputPath,
		"-f", "segment",
		"-segment_time", strconv.Itoa(segmentSeconds),
		"-c", "copy", // Copy codec, no re-encoding
		"-reset_timestamps", "1",
		segmentPattern,
	}

	if err := FFmpeg(args...); err != nil {
		return nil, fmt.Errorf("failed to split audio: %w", err)
	}

	// Find all segment files
	pattern := filepath.Join(outputDir, "seg_*"+ext)
	segments, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to find segments: %w", err)
	}

	if len(segments) == 0 {
		return nil, fmt.Errorf("no segments created")
	}

	// Sort segments to ensure correct order
	sort.Strings(segments)
	return segments, nil
}

// ExtractAudioTrack extracts an audio track from a media file to specified format
// Optional extraArgs (e.g., resample filters) are inserted before codec settings
func ExtractAudioTrack(inputFile string, trackIndex int, outputFile string, extraArgs ...string) error {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(outputFile)), ".")

	args := []string{"-loglevel", "error", "-i", inputFile,
		"-map", fmt.Sprintf("0:a:%d", trackIndex), "-vn"}

	// Extra args (filters like -af) go after input, before codec
	args = append(args, extraArgs...)

	// Codec and bitrate based on output format
	args = append(args, "-acodec")
	switch ext {
	case "m4a":
		args = append(args, "aac", "-b:a", "192k")
	case "opus", "ogg":
		args = append(args, "libopus", "-b:a", OpusBitrate)
	case "flac":
		args = append(args, "flac")
	case "wav":
		args = append(args, "pcm_s16le")
	default:
		args = append(args, "libmp3lame", "-b:a", "192k")
	}

	args = append(args, outputFile)
	return FFmpeg(args...)
}

// Converts audio file to specified format using FFmpeg
func RunFFmpegConvert(inputFile, outputFile string) error {
	return runFFmpegConvert(inputFile, outputFile, nil)
}

// RunFFmpegConvertArgs converts audio with additional FFmpeg args (e.g., filters)
// Extra args are inserted after input, before codec settings (proper FFmpeg ordering)
func RunFFmpegConvertArgs(inputFile, outputFile string, extraArgs ...string) error {
	return runFFmpegConvert(inputFile, outputFile, extraArgs)
}

// runFFmpegConvert is the internal implementation for audio conversion
func runFFmpegConvert(inputFile, outputFile string, extraArgs []string) error {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(outputFile)), ".")

	// Start with input
	args := []string{"-loglevel", "error", "-i", inputFile}

	// Extra args (filters like -af) go after input, before codec
	args = append(args, extraArgs...)

	// Codec and bitrate based on output format
	args = append(args, "-acodec")
	switch ext {
	case "m4a":
		args = append(args, "aac", "-b:a", "192k")
	case "opus", "ogg":
		args = append(args, "libopus", "-b:a", OpusBitrate)
	case "flac":
		args = append(args, "flac")
	case "wav":
		args = append(args, "pcm_s16le")
	default:
		args = append(args, "libmp3lame", "-b:a", "192k")
	}

	args = append(args, outputFile)
	return FFmpeg(args...)
}

func FFmpeg(arg ...string) error {
	arg = append(arg, "-hide_banner")
	arg = append(arg, "-y")
	cmd := executils.NewCommand(FFmpegPath, arg...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg command %v failed: %w", arg, err)
	}
	return nil
}

func GetFFmpegVersion() (string, error) {
	cmd := executils.NewCommand(FFmpegPath, "-version")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run ffmpeg: %w", err)
	}

	output := out.String()
	re := regexp.MustCompile(`ffmpeg version (\S+)`)
	match := re.FindStringSubmatch(output)

	if len(match) < 2 {
		return "", fmt.Errorf("failed to extract ffmpeg version from output")
	}

	return match[1], nil
}

func CheckValidData(filepath string) (bool, error) {
	cmd := executils.NewCommand(FFmpegPath,
		"-loglevel", "error",
		"-i", filepath,
		// all ↓ needed to suppress "At least one output file must be specified"
		"-t", "0",
		"-f", "null", "-",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	errorOutput := stderr.String()
	headerPatterns := []*regexp.Regexp{
		regexp.MustCompile(`Invalid data found when processing input`),
		regexp.MustCompile(`Error while decoding stream`),
		regexp.MustCompile(`could not find codec parameters`),
		regexp.MustCompile(`Failed to open input`),
		regexp.MustCompile(`Invalid header`),
		regexp.MustCompile(`error reading header`),
		regexp.MustCompile(`Invalid NAL`),
		regexp.MustCompile(`Error splitting the input into NAL units`),
	}

	dataPatterns := []*regexp.Regexp{
		regexp.MustCompile(`Sample size \d+ is too large`),
		regexp.MustCompile(`Invalid sample size`),
		regexp.MustCompile(`moov atom not found`),
		regexp.MustCompile(`Invalid chunk offset`),
		regexp.MustCompile(`Error while decoding frame`),
		regexp.MustCompile(`broken frame`),
		regexp.MustCompile(`Invalid index`),
		regexp.MustCompile(`invalid frame size`),
		regexp.MustCompile(`Invalid data found`),
	}

	for _, pattern := range headerPatterns {
		if pattern.MatchString(errorOutput) {
			return true, fmt.Errorf("%s", strings.TrimSuffix(stderr.String(), "\n"))
		}
	}

	for _, pattern := range dataPatterns {
		if pattern.MatchString(errorOutput) {
			return true, fmt.Errorf("%s", strings.TrimSuffix(stderr.String(), "\n"))
		}
	}
	
	if err != nil {
		return false, err
	}
	return false, nil
}



func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}


func placeholder3() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}


