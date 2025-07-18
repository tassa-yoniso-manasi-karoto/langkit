package media

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gookit/color"
	"github.com/k0kubun/pp"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
)

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

// Converts the WAV file to specified audio format using FFmpeg
func RunFFmpegConvert(inputWav, outputFile string) error {
	// Determine codec and parameters based on file extension
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(outputFile)), ".")
	
	args := []string{"-loglevel", "error", "-i", inputWav, "-acodec"}
	switch ext {
	case "m4a":
		args = append(args, []string{"aac", "-b:a", "192k", outputFile}...)
	case "opus", "ogg":
		args = append(args, []string{"libopus", "-b:a", "112k", outputFile}...)
	default:
		args = append(args, []string{"libmp3lame", "-b:a", "192k", outputFile}...)
	}

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


