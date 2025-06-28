package ffmpeg

import (
	"fmt"
	"io"
	"runtime"
	"time"
)

// GetDownloadURL returns the appropriate FFmpeg download URL for the current OS and architecture.
func GetDownloadURL() (string, error) {
	const baseURL = "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/"

	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return baseURL + "ffmpeg-master-latest-win64-gpl.zip", nil
		case "arm64":
			return baseURL + "ffmpeg-master-latest-winarm64-gpl.zip", nil
		}
	case "darwin":
		return "https://evermeet.cx/ffmpeg/get/zip", nil
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return baseURL + "ffmpeg-master-latest-linux64-gpl.tar.xz", nil
		case "arm64":
			return baseURL + "ffmpeg-master-latest-linuxarm64-gpl.tar.xz", nil
		}
	}
	return "", fmt.Errorf("unsupported OS/architecture for automatic FFmpeg download: %s/%s", runtime.GOOS, runtime.GOARCH)
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
