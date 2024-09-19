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
		"-map", fmt.Sprint("0:", tracknum),
	}
	outArgs = append(outArgs, outFile)

	args := []string{
		"-loglevel", "error",
	}
	args = append(args, inArgs...)
	args = append(args, outArgs...)

	return ffmpeg(args...)
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

	return ffmpeg(args...)
}

func ffmpegPosition(d time.Duration) string {
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond
	return fmt.Sprintf("%d.%d", s, ms)
}

func ffmpeg(arg ...string) error {
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


