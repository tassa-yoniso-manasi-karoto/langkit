package media

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/k0kubun/pp"
	"github.com/gookit/color"
)

func ffmpegExtractAudio(tracknum int, offset, startAt, endAt time.Duration, inFile, outFile string, outArgs []string) error {
	if exists(outFile) {
		return nil
	}
	/* https://stackoverflow.com/questions/18444194/cutting-multimedia-files-based-on-start-and-end-time-using-ffmpeg
	using -t after -i result in inaccurate cuts but using -to before -i fix it, resulting in the same timecode as subs2srs
	sub2srs uses -i "input.mp3" -ss 00:00:00.000 -t 00:00:01.900 format but used an old version of ffmpeg (v4)
	*/
	inArgs := []string{
		"-ss", ffmpegPosition(startAt-offset),
		"-to", ffmpegPosition(endAt+offset),
		"-i", inFile,
		"-map", fmt.Sprint("0:", tracknum+1),
	}
	outArgs = append(outArgs, outFile)

	args := []string{
		"-loglevel", "error",
	}
	args = append(args, inArgs...)
	args = append(args, outArgs...)

	return Ffmpeg(args...)
}

func ffmpegExtractImage(startAt, endAt time.Duration, inFile string, outFile string) error {
	if exists(outFile) {
		return nil
	}

	var frameAt = startAt
	if endAt > startAt {
		frameAt = startAt + (endAt-startAt)/2
	}

	inArgs := []string{
		"-ss", ffmpegPosition(frameAt),
		"-i", inFile,
	}

	outArgs := []string{ // -45% in size but x2,3 processing time (pentium 4core)
		"-vf", "scale=1000:562",
		"-c:v", "libaom-av1",
		"-cpu-used", "6",
		"-aom-params", "aq-mode=1:enable-chroma-deltaq=1",
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

	return Ffmpeg(args...)
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
		return "", fmt.Errorf("error creating temporary file: %v", err)
	}
	defer concatFile.Close()

	// Write the list of .wav files in ffmpeg concat format
	for _, wavFile := range wavFiles {
		line := fmt.Sprintf("file '%s'\n", wavFile)
		if _, err := concatFile.WriteString(line); err != nil {
			return "", fmt.Errorf("error writing to concat file: %v", err)
		}
	}

	return concatFile.Name(), nil
}

// Runs FFmpeg concat command with the provided concat file and output wav file
func RunFFmpegConcat(concatFile, outputWav string) error {
	return Ffmpeg([]string{"-loglevel", "error", "-y", "-f", "concat", "-safe", "0", "-i", concatFile, "-c", "copy", outputWav}...)
}

// Converts the WAV file to OGG using FFmpeg
func RunFFmpegConvert(inputWav, outputOgg string) error {
	return Ffmpeg([]string{"-loglevel", "error", "-y", "-i", inputWav, "-acodec", "libopus", "-b:a", "112k", outputOgg}...)
}



func Ffmpeg(arg ...string) error {
	arg = append(arg, "-hide_banner")
	cmd := exec.Command("ffmpeg", arg...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg command %v failed: %v", arg, err)
	}
	return nil
}




func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}


func placeholder3() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}


