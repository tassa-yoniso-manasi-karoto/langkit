package crash

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"net"
	"time"

	"github.com/klauspost/compress/zip"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/rs/zerolog"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/version"
)

var log zerolog.Logger

func init() {
	writer := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.TimeOnly,
	}
	writer.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf("[crashWriter] %s", i)
	}
	log = zerolog.New(writer).With().Timestamp().Logger()
	
	// On a 4G network far from the antenna I was facing intermittent hangs during connectivity checks. 
	// I tried many different ways to fix the program hanging during connectivity checks.
	// 
	// - First, I wrapped HTTP requests in a goroutine with context cancellation 
	//   and a timeout, but sometimes it still got stuck.
	// - Then, I tweaked net/http settings (DialTimeout, TLSHandshakeTimeout, etc.), 
	//   but the issue persisted.
	// - I even switched to Resty (third-party HTTP client) hoping it would handle 
	//   timeouts better, but no luck.
	// 
	// After all these failed attempts, what actually worked was forcing Go’s built-in 
	// DNS resolver (PreferGo: true). Using Go’s native resolver made everything reliable.
	//
	// DNS. It's always DNS.
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
	}
}

// TODO use FormatDirectoryListing on dir of current media file

func WriteReport(
	mainErr error,
	runtimeInfo string,
	settings config.Settings,
	logBuffer io.Reader,
) (string, error) {
	startTime := time.Now()
	dir := GetCrashDir()
	CleanUpReportsOnDisk(dir)
	
	timestamp := startTime.Format("20060102_150405")
	tempPath := filepath.Join(dir, fmt.Sprintf("temp_crash_%s.log", timestamp))
	
	log.Debug().Msg("creating temp file for crash file")
	crashFile, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary crash file: %w", err)
	}
	defer func() {
		crashFile.Close()
		os.Remove(tempPath)
	}()

	log.Debug().Msg("starting to write report")
	if err := writeReport(crashFile, mainErr, runtimeInfo, settings, logBuffer); err != nil {
		return "", fmt.Errorf("failed to write crash report: %w", err)
	}

	genTime := time.Since(startTime)
	finalPath := filepath.Join(dir, fmt.Sprintf("crash_%s_gen%s.txt.zip", 
		timestamp,
		formatDuration(genTime),
	))

	log.Debug().Msg("starting to compress report")
	if err := compressReport(tempPath, finalPath); err != nil {
		return "", fmt.Errorf("failed to compress crash report: %w", err)
	}
	log.Debug().Msg("compressing report done")
	return finalPath, nil
}

func writeReport(
	w io.Writer,
	mainErr error,
	runtimeInfo string,
	settings config.Settings,
	logBuffer io.Reader,
) error {
	log.Debug().Msg("writing Header")
	fmt.Fprintln(w, "LANGKIT CRASH REPORT")
	fmt.Fprintln(w, "==================")
	fmt.Fprintf(w, "This file has syntax highlighting through ANSI escape codes and is best viewed in a terminal using 'cat'.\n")
	fmt.Fprintf(w, "Timestamp: %s\n\n", time.Now().Format(time.RFC3339))

	fmt.Fprintln(w, "Langkit:")
	fmt.Fprintln(w, version.GetVersionInfo())
	fmt.Fprint(w, "\n")

	
	log.Debug().Msg("writing ERROR DETAILS")
	fmt.Fprintln(w, "ERROR DETAILS")
	fmt.Fprintln(w, "============")
	fmt.Fprintf(w, "Error: %v\n", mainErr)
	if err, ok := mainErr.(interface{ Unwrap() error }); ok {
		fmt.Fprintf(w, "Unwrapped Error Chain:\n")
		for err := err.Unwrap(); err != nil; {
			if unwrappable, ok := err.(interface{ Unwrap() error }); ok {
				fmt.Fprintf(w, "  → %v\n", err)
				err = unwrappable.Unwrap()
			} else {
				fmt.Fprintf(w, "  → %v\n", err)
				break
			}
		}
	}
	fmt.Fprint(w, "\n")

	log.Debug().Msg("writing STACK TRACE")
	fmt.Fprintln(w, "STACK TRACE")
	fmt.Fprintln(w, "===========")
	fmt.Fprintf(w, "%s\n\n", string(debug.Stack()))

	// Write execution context from reporter
	if Reporter != nil {
		globalScope, execScope := Reporter.GetScopes()
		
		fmt.Fprintln(w, "GLOBAL SCOPE")
		fmt.Fprintln(w, "============")
		fmt.Fprintf(w, "Program Start Time: %s\n", globalScope.StartTime.Format(time.RFC3339))
		fmt.Fprintf(w, "FFmpeg Path: %s\n", globalScope.FFmpegPath)
		fmt.Fprintf(w, "FFmpeg Version: %s\n", globalScope.FFmpegVersion)
		fmt.Fprintf(w, "MediaInfo Version: %s\n\n", globalScope.MediaInfoVer)

		if execScope.MediaInfoDump != "" {
			fmt.Fprintln(w, "MEDIA INFORMATION")
			fmt.Fprintln(w, "=================")
			fmt.Fprintf(w, "Processing Start Time: %s\n", execScope.StartTime.Format(time.RFC3339))
			fmt.Fprintf(w, "MediaInfo Dump:\n%s\n\n", execScope.MediaInfoDump)
		}

		// Write execution snapshots
		fmt.Fprintf(w, "%s\n", Reporter.GetSnapshotsString())
	}

	log.Debug().Msg("writing RUNTIME INFO")
	fmt.Fprintln(w, "RUNTIME INFORMATION")
	fmt.Fprint(w, "==================\n\n")
	fmt.Fprint(w, runtimeInfo + "\n\n")

	log.Debug().Msg("writing ENVIRONMENT")
	fmt.Fprintln(w, "ENVIRONMENT")
	fmt.Fprintln(w, "===========")
	printEnvironment(w)
	fmt.Fprint(w, "\n")

	log.Debug().Msg("writing SETTINGS")
	fmt.Fprintln(w, "SETTINGS")
	fmt.Fprintln(w, "========")
	sanitizedSettings := settings
	sanitizedSettings.APIKeys.Replicate = MaskAPIKey(settings.APIKeys.Replicate)
	sanitizedSettings.APIKeys.AssemblyAI = MaskAPIKey(settings.APIKeys.AssemblyAI)
	sanitizedSettings.APIKeys.ElevenLabs = MaskAPIKey(settings.APIKeys.ElevenLabs)
	fmt.Fprintln(w, pp.Sprint(sanitizedSettings), "\n")

	log.Debug().Msg("writing LOG HISTORY")
	fmt.Fprintln(w, "LOG HISTORY")
	fmt.Fprintln(w, "===========")
	if logBuffer != nil {
		n, err := io.Copy(w, logBuffer)
		if err != nil && n != 0 {
			return fmt.Errorf("failed to write log history: %w", err)
		}
		if n == 0 {
			fmt.Fprintln(w, "No logs")
		}
	} else {
		fmt.Fprintln(w, "No logs")
	}
	fmt.Fprint(w, "\n")


	log.Debug().Msg("writing CONNECTIVITY STATUS")
	// Takes the longest, keep it last, and in some scenarios the DNS still hangs the program forever
	fmt.Fprintln(w, "CONNECTIVITY STATUS")
	fmt.Fprintln(w, "==================")
	// Not sure if some AI API services have georestrictions but when in doubt
	if country, err := GetUserCountry(); err == nil {
		fmt.Fprintln(w, "Requests originate from:", country)
	}
	log.Trace().Msg("Country OK")
	DockerNslookupCheck(w, "example.com")
	log.Debug().Msg("DockerNslookupCheck done")
	checkEndpointConnectivity(w, "https://replicate.com", "Replicate")
	checkEndpointConnectivity(w, "https://www.assemblyai.com/", "AssemblyAI")
	checkEndpointConnectivity(w, "https://elevenlabs.io", "ElevenLabs")
	fmt.Fprint(w, "\n")
	
	
	fmt.Fprintln(w, "==================")
	fmt.Fprintln(w, "END OF REPORT")
	fmt.Fprintln(w, "==================")
	return nil
}

func compressReport(sourcePath, destPath string) error {
	zipFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer source.Close()

	// Create a file entry in the zip archive using the source filename
	writer, err := zipWriter.Create(filepath.Base(sourcePath))
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	// Copy the source file contents to the zip archive
	if _, err := io.Copy(writer, source); err != nil {
		return fmt.Errorf("failed to compress file: %w", err)
	}

	return nil
}

func CleanUpReportsOnDisk(crashDir string) {
	pattern := filepath.Join(crashDir, "crash_*.zst")
	
	matches, _ := filepath.Glob(pattern)
	
	// Only keep last 10 crash reports
	if len(matches) >= 10 {
		// Sort by modification time
		sort.Slice(matches, func(i, j int) bool {
			iInfo, _ := os.Stat(matches[i])
			jInfo, _ := os.Stat(matches[j])
			return iInfo.ModTime().After(jInfo.ModTime())
		})
		
		// Remove older crash reports
		for _, path := range matches[10:] {
			os.Remove(path)
		}
	}
}


func placeholder354() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}

