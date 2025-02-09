package crash

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/k0kubun/pp"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/version"
)

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
	
	crashFile, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary crash file: %w", err)
	}
	defer func() {
		crashFile.Close()
		os.Remove(tempPath)
	}()

	if err := writeReport(crashFile, mainErr, runtimeInfo, settings, logBuffer); err != nil {
		return "", fmt.Errorf("failed to write crash report: %w", err)
	}

	genTime := time.Since(startTime)
	finalPath := filepath.Join(dir, fmt.Sprintf("crash_%s_gen%s.txt.zst", 
		timestamp,
		formatDuration(genTime),
	))

	if err := compressReport(tempPath, finalPath); err != nil {
		return "", fmt.Errorf("failed to compress crash report: %w", err)
	}

	return finalPath, nil
}

func writeReport(
	w io.Writer,
	mainErr error,
	runtimeInfo string,
	settings config.Settings,
	logBuffer io.Reader,
) error {
	// Write header
	fmt.Fprintln(w, "LANGKIT CRASH REPORT")
	fmt.Fprintln(w, "==================")
	fmt.Fprintf(w, "This file has syntax highlighting through ANSI escape codes and is best viewed in a terminal using 'cat'.\n")
	fmt.Fprintf(w, "Timestamp: %s\n\n", time.Now().Format(time.RFC3339))

	fmt.Fprintln(w, "Langkit:")
	fmt.Fprintln(w, version.GetVersionInfo())
	fmt.Fprint(w, "\n")

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

	fmt.Fprintln(w, "RUNTIME INFORMATION")
	fmt.Fprint(w, "==================\n\n")
	fmt.Fprint(w, runtimeInfo + "\n\n")

	fmt.Fprintln(w, "ENVIRONMENT")
	fmt.Fprintln(w, "===========")
	printEnvironment(w)
	fmt.Fprint(w, "\n")

	fmt.Fprintln(w, "SETTINGS")
	fmt.Fprintln(w, "========")
	sanitizedSettings := settings
	sanitizedSettings.APIKeys.Replicate = MaskAPIKey(settings.APIKeys.Replicate)
	sanitizedSettings.APIKeys.AssemblyAI = MaskAPIKey(settings.APIKeys.AssemblyAI)
	sanitizedSettings.APIKeys.ElevenLabs = MaskAPIKey(settings.APIKeys.ElevenLabs)
	fmt.Fprintln(w, pp.Sprint(sanitizedSettings), "\n")

	fmt.Fprintln(w, "LOG HISTORY")
	fmt.Fprintln(w, "===========")
	if logBuffer != nil {
		if _, err := io.Copy(w, logBuffer); err != nil {
			return fmt.Errorf("failed to write log history: %w", err)
		}
	} else {
		fmt.Fprintln(w, "No log history available")
	}
	fmt.Fprint(w, "\n")

	// Takes the longest, keep it last
	fmt.Fprintln(w, "CONNECTIVITY STATUS")
	fmt.Fprintln(w, "==================")
	// Not sure if some AI API services have georestrictions but when in doubt
	if country, err := GetUserCountry(); err == nil {
		fmt.Fprintln(w, "Requests originate from:", country)
	}
	checkEndpointConnectivity(w, "https://replicate.com", "Replicate")
	checkEndpointConnectivity(w, "https://www.assemblyai.com/", "AssemblyAI")
	checkEndpointConnectivity(w, "https://elevenlabs.io", "ElevenLabs")
	fmt.Fprint(w, "\n")
	
	return nil
}

func compressReport(sourcePath, destPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer source.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create compressed file: %w", err)
	}
	defer dest.Close()

	enc, err := zstd.NewWriter(dest, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		return fmt.Errorf("failed to create zstd encoder: %w", err)
	}
	defer enc.Close()

	if _, err := io.Copy(enc, source); err != nil {
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
