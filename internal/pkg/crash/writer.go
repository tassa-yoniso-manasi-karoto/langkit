package crash

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"time"

	"github.com/gookit/color"
	"github.com/k0kubun/pp"
	"github.com/klauspost/compress/zip"
	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/dockerutil"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/version"
)

type ReportMode int

const (
	ModeCrash ReportMode = iota
	ModeDebug
)

var log zerolog.Logger

func init() {
	dockerutil.SetLogOutput(dockerutil.LogToBoth)
	writer := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.TimeOnly,
	}
	writer.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf("[crashWriter] %s", i)
	}
	log = zerolog.New(writer).With().Timestamp().Logger()
}

// runWithTimeout executes a function with a timeout. If the function doesn't complete
// within the timeout, it logs a warning and continues. This prevents any single
// operation from hanging the entire report generation.
func runWithTimeout(timeout time.Duration, name string, w io.Writer, fn func()) {
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Msgf("Panic in %s: %v", name, r)
				fmt.Fprintf(w, "%s: ERROR - panic occurred: %v\n", name, r)
			}
			close(done)
		}()
		fn()
	}()
	
	select {
	case <-done:
		// Operation completed successfully
	case <-time.After(timeout):
		log.Warn().Msgf("%s timed out after %v", name, timeout)
		fmt.Fprintf(w, "%s: TIMEOUT - operation did not complete within %v\n", name, timeout)
	}
}

func WriteReport(
	reportMode ReportMode,
	mainErr error, // may be nil if ModeDebug
	settings config.Settings,
	logBuffer bytes.Buffer,
	isCLI bool,
) (string, error) {

	startTime := time.Now()
	dir := GetCrashDir()
	CleanUpReportsOnDisk(dir)

	timestamp := startTime.Format("20060102_150405")

	// Distinguish the prefix + final name
	var prefix string
	if reportMode == ModeCrash {
		prefix = "crash"
	} else {
		prefix = "debug"
	}
	tempPath := filepath.Join(dir, fmt.Sprintf("%s_ZIP_ME_%s.log", prefix, timestamp))

	log.Debug().Msgf("creating temp file for %s file", prefix)
	reportFile, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary %s file: %w", prefix, err)
	}
	defer func() {
		reportFile.Close()
		os.Remove(tempPath) // we remove the intermediate .log once it's zipped
	}()

	log.Debug().Msg("starting to write report content")
	if err := writeReportContent(reportMode, reportFile, mainErr, settings, logBuffer, isCLI); err != nil {
		return "", fmt.Errorf("failed to write %s report: %w", prefix, err)
	}

	genTime := time.Since(startTime)
	finalPath := filepath.Join(dir, fmt.Sprintf("%s_%s_gen%s.txt.zip",
		prefix,
		timestamp,
		formatDuration(genTime),
	))

	log.Debug().Msg("starting to compress report")
	if err := compressReport(tempPath, finalPath); err != nil {
		return "", fmt.Errorf("failed to compress %s report: %w", prefix, err)
	}
	log.Debug().Msg("compressing report done")
	return finalPath, nil
}

// writeReportContent writes either the crash or debug data into an uncompressed file (which will
// later be zipped).
func writeReportContent(
	mode ReportMode,
	w io.Writer,
	mainErr error,      // might be nil for debug
	settings config.Settings,
	logBuffer bytes.Buffer,
	isCLI bool,
) error {
	// Create a buffer to capture all output before sanitization
	var buf bytes.Buffer
	
	// Use the buffer as the writer for all content
	bufWriter := &buf

	// 1. Header
	if mode == ModeCrash {
		fmt.Fprintln(bufWriter, "LANGKIT CRASH REPORT")
	} else {
		fmt.Fprintln(bufWriter, "LANGKIT DEBUG REPORT")
	}
	fmt.Fprintln(bufWriter, "==================")
	fmt.Fprintln(bufWriter, "This file has syntax highlighting through ANSI escape codes and is best viewed")
	fmt.Fprintln(bufWriter, "in a terminal using 'cat'.")
	fmt.Fprintf(bufWriter, "Timestamp: %s\n\n", time.Now().Format(time.RFC3339))

	// 2. Basic app info
	fmt.Fprintln(bufWriter, version.GetInfo().String())
	fmt.Fprintf(bufWriter, "Interface mode: ")
	if isCLI {
		fmt.Fprintln(bufWriter, "CLI")
	} else {
		fmt.Fprintln(bufWriter, "GUI / Wails")
	}
	fmt.Fprint(bufWriter, "\n\n")

	// 3. Error details (only if crash mode and mainErr != nil)
	if mode == ModeCrash {
		fmt.Fprintln(bufWriter, "ERROR DETAILS")
		fmt.Fprintln(bufWriter, "============")
		if mainErr == nil {
			fmt.Fprintln(bufWriter, "WARNING: No mainErr provided, but mode is crash.")
		} else {
			fmt.Fprintf(bufWriter, "Error: %v\n", mainErr)
			if unwrappable, ok := mainErr.(interface{ Unwrap() error }); ok {
				fmt.Fprintf(bufWriter, "Unwrapped Error Chain:\n")
				err := unwrappable.Unwrap()
				for err != nil {
					fmt.Fprintf(bufWriter, "  → %v\n", err)
					if next, ok := err.(interface{ Unwrap() error }); ok {
						err = next.Unwrap()
					} else {
						err = nil
					}
				}
			}
		}
		fmt.Fprint(bufWriter, "\n")

		// 4. Stack trace
		fmt.Fprintln(bufWriter, "STACK TRACE")
		fmt.Fprintln(bufWriter, "===========")
		fmt.Fprintf(bufWriter, "%s\n\n", string(debug.Stack()))
	} else {
		fmt.Fprintln(bufWriter, "User-triggered debug report.")
	}

	// 5. Crash reporter scopes (if any)
	if Reporter != nil {
		globalScope, execScope := Reporter.GetScopes()
			
		fmt.Fprintln(bufWriter, "PARENT DIR OF MEDIA FILE")
		fmt.Fprintln(bufWriter, "========================")
		runWithTimeout(5*time.Second, "FormatDirectoryListing", bufWriter, func() {
			err := FormatDirectoryListing(bufWriter, execScope.ParentDirPath)
			if err != nil {
				fmt.Fprintf(bufWriter, "Error listing directory: %v\n", err)
			}
		})
		fmt.Fprint(bufWriter, "\n\n")

		fmt.Fprintln(bufWriter, "GLOBAL SCOPE")
		fmt.Fprintln(bufWriter, "============")
		fmt.Fprintf(bufWriter, "Program Start Time: %s\n", globalScope.StartTime.Format(time.RFC3339))
		fmt.Fprintf(bufWriter, "FFmpeg Path: %s\n", globalScope.FFmpegPath)
		fmt.Fprintf(bufWriter, "FFmpeg Version: %s\n", globalScope.FFmpegVersion)
		fmt.Fprintf(bufWriter, "MediaInfo Version: %s\n\n", globalScope.MediaInfoVer)

		if execScope.MediaInfoDump != "" {
			fmt.Fprintln(bufWriter, "CURRENT MEDIA INFORMATION")
			fmt.Fprintln(bufWriter, "=========================")
			fmt.Fprintf(bufWriter, "Processing Start Time: %s\n", execScope.StartTime.Format(time.RFC3339))
			fmt.Fprintf(bufWriter, "MediaInfo Dump:\n%s\n\n", execScope.MediaInfoDump)
		}

		// 6. Execution snapshots
		fmt.Fprintf(bufWriter, "%s\n", Reporter.GetSnapshotsString())
	}
	
	// 6.5 WebAssembly status (if available)
	if Reporter != nil {
		fmt.Fprintln(bufWriter, "WEBASSEMBLY STATUS")
		fmt.Fprintln(bufWriter, "==================")
		
		// Check for WebAssembly snapshot
		wasmSnapshot := Reporter.GetSnapshot("wasm_state")
		if wasmSnapshot != "" {
			var state map[string]interface{}
			if err := json.Unmarshal([]byte(wasmSnapshot), &state); err == nil {
				fmt.Fprintf(bufWriter, "Status: %v\n", state["initStatus"])
				
				if metrics, ok := state["performanceMetrics"].(map[string]interface{}); ok {
					fmt.Fprintf(bufWriter, "Operations: %v\n", state["totalOperations"])
					fmt.Fprintf(bufWriter, "Speed Ratio: %.2fx\n", metrics["speedupRatio"])
				}
				
				if memUsage, ok := state["memoryUsage"].(map[string]interface{}); ok {
					fmt.Fprintf(bufWriter, "Memory Usage: %.1f%%\n", memUsage["utilization"].(float64)*100)
				}
				
				if err, ok := state["lastError"].(map[string]interface{}); ok {
					fmt.Fprintf(bufWriter, "Last Error: %s\n", err["message"])
				}
			} else {
				fmt.Fprintln(bufWriter, "WebAssembly state available but failed to parse")
			}
		} else {
			fmt.Fprintln(bufWriter, "WebAssembly: Not initialized or state not available")
		}
		fmt.Fprintln(bufWriter, "")
	}

	// 7. System / runtime info
	fmt.Fprintln(bufWriter, "RUNTIME INFORMATION")
	fmt.Fprintln(bufWriter, "==================")
	runWithTimeout(10*time.Second, "RuntimeInfo", bufWriter, func() {
		fmt.Fprintln(bufWriter, NewRuntimeInfo().String())
	})

	// 8. Environment
	fmt.Fprintln(bufWriter, "ENVIRONMENT")
	fmt.Fprintln(bufWriter, "===========")
	printEnvironment(bufWriter)
	fmt.Fprint(bufWriter, "\n")

	// 9. Settings (mask API keys)
	fmt.Fprintln(bufWriter, "SETTINGS")
	fmt.Fprintln(bufWriter, "========")
	sanitizedSettings := settings
	sanitizedSettings.APIKeys.Replicate = MaskAPIKey(settings.APIKeys.Replicate)
	sanitizedSettings.APIKeys.ElevenLabs = MaskAPIKey(settings.APIKeys.ElevenLabs)
	sanitizedSettings.APIKeys.OpenAI = MaskAPIKey(settings.APIKeys.OpenAI)
	sanitizedSettings.APIKeys.OpenRouter = MaskAPIKey(settings.APIKeys.OpenRouter)
	sanitizedSettings.APIKeys.Google = MaskAPIKey(settings.APIKeys.Google)
	fmt.Fprintln(bufWriter, pp.Sprint(sanitizedSettings))

	// 10. Log history
	fmt.Fprintln(bufWriter, "LOG HISTORY")
	fmt.Fprintln(bufWriter, "===========")
	writeLogs(bufWriter, &logBuffer)

	// 11. Docker log history
	fmt.Fprintln(bufWriter, "DOCKER LOG HISTORY")
	fmt.Fprintln(bufWriter, "==================")
	writeLogs(bufWriter, &dockerutil.DockerLogBuffer)
	
	fmt.Fprintln(bufWriter, "DOCKER INFORMATION")
	fmt.Fprintln(bufWriter, "==================")
	runWithTimeout(15*time.Second, "DockerInfo", bufWriter, func() {
		captureDockerInfo(bufWriter)
	})
	fmt.Fprint(bufWriter, "\n")

	// 12. Connectivity status
	fmt.Fprintln(bufWriter, "CONNECTIVITY STATUS")
	fmt.Fprintln(bufWriter, "==================")
	// Not sure if some AI API services have georestrictions but when in doubt
	runWithTimeout(5*time.Second, "GetUserCountry", bufWriter, func() {
		if country, err := GetUserCountry(); err == nil {
			fmt.Fprintln(bufWriter, "Requests originate from:", country)
		}
	})
	
	runWithTimeout(5*time.Second, "CheckReplicate", bufWriter, func() {
		checkEndpointConnectivity(bufWriter, "https://replicate.com", "Replicate")
	})
	
	runWithTimeout(5*time.Second, "CheckElevenLabs", bufWriter, func() {
		checkEndpointConnectivity(bufWriter, "https://elevenlabs.io", "ElevenLabs")
	})
	
	runWithTimeout(10*time.Second, "DockerNslookup", bufWriter, func() {
		DockerNslookupCheck(bufWriter, "example.com")
	})
	fmt.Fprint(bufWriter, "\n")

	fmt.Fprintln(bufWriter, "==================")
	fmt.Fprintln(bufWriter, "END OF REPORT")
	fmt.Fprintln(bufWriter, "==================")

	// Sanitize the buffer to remove all API keys
	sanitizedContent := SanitizeBuffer(buf.Bytes(), settings)
	
	// Write the sanitized content to the original writer
	if _, err := w.Write(sanitizedContent); err != nil {
		return fmt.Errorf("failed to write sanitized report: %w", err)
	}

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
	pattern := filepath.Join(crashDir, "crash_*")
	
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

