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

	// 1. Header
	if mode == ModeCrash {
		fmt.Fprintln(w, "LANGKIT CRASH REPORT")
	} else {
		fmt.Fprintln(w, "LANGKIT DEBUG REPORT")
	}
	fmt.Fprintln(w, "==================")
	fmt.Fprintln(w, "This file has syntax highlighting through ANSI escape codes and is best viewed")
	fmt.Fprintln(w, "in a terminal using 'cat'.")
	fmt.Fprintf(w, "Timestamp: %s\n\n", time.Now().Format(time.RFC3339))

	// 2. Basic app info
	fmt.Fprintln(w, version.GetInfo().String())
	fmt.Fprintf(w, "Interface mode: ")
	if isCLI {
		fmt.Fprintln(w, "CLI")
	} else {
		fmt.Fprintln(w, "GUI / Wails")
	}
	fmt.Fprint(w, "\n\n")

	// 3. Error details (only if crash mode and mainErr != nil)
	if mode == ModeCrash {
		fmt.Fprintln(w, "ERROR DETAILS")
		fmt.Fprintln(w, "============")
		if mainErr == nil {
			fmt.Fprintln(w, "WARNING: No mainErr provided, but mode is crash.")
		} else {
			fmt.Fprintf(w, "Error: %v\n", mainErr)
			if unwrappable, ok := mainErr.(interface{ Unwrap() error }); ok {
				fmt.Fprintf(w, "Unwrapped Error Chain:\n")
				err := unwrappable.Unwrap()
				for err != nil {
					fmt.Fprintf(w, "  → %v\n", err)
					if next, ok := err.(interface{ Unwrap() error }); ok {
						err = next.Unwrap()
					} else {
						err = nil
					}
				}
			}
		}
		fmt.Fprint(w, "\n")

		// 4. Stack trace
		fmt.Fprintln(w, "STACK TRACE")
		fmt.Fprintln(w, "===========")
		fmt.Fprintf(w, "%s\n\n", string(debug.Stack()))
	} else {
		fmt.Fprintln(w, "User-triggered debug report.")
	}

	// 5. Crash reporter scopes (if any)
	if Reporter != nil {
		globalScope, execScope := Reporter.GetScopes()
			
		fmt.Fprintln(w, "PARENT DIR OF MEDIA FILE")
		fmt.Fprintln(w, "========================")
		runWithTimeout(5*time.Second, "FormatDirectoryListing", w, func() {
			err := FormatDirectoryListing(w, execScope.ParentDirPath)
			if err != nil {
				fmt.Fprintf(w, "Error listing directory: %v\n", err)
			}
		})
		fmt.Fprint(w, "\n\n")

		fmt.Fprintln(w, "GLOBAL SCOPE")
		fmt.Fprintln(w, "============")
		fmt.Fprintf(w, "Program Start Time: %s\n", globalScope.StartTime.Format(time.RFC3339))
		fmt.Fprintf(w, "FFmpeg Path: %s\n", globalScope.FFmpegPath)
		fmt.Fprintf(w, "FFmpeg Version: %s\n", globalScope.FFmpegVersion)
		fmt.Fprintf(w, "MediaInfo Version: %s\n\n", globalScope.MediaInfoVer)

		if execScope.MediaInfoDump != "" {
			fmt.Fprintln(w, "CURRENT MEDIA INFORMATION")
			fmt.Fprintln(w, "=========================")
			fmt.Fprintf(w, "Processing Start Time: %s\n", execScope.StartTime.Format(time.RFC3339))
			fmt.Fprintf(w, "MediaInfo Dump:\n%s\n\n", execScope.MediaInfoDump)
		}

		// 6. Execution snapshots
		fmt.Fprintf(w, "%s\n", Reporter.GetSnapshotsString())
	}
	
	// 6.5 WebAssembly status (if available)
	if Reporter != nil {
		fmt.Fprintln(w, "WEBASSEMBLY STATUS")
		fmt.Fprintln(w, "==================")
		
		// Check for WebAssembly snapshot
		wasmSnapshot := Reporter.GetSnapshot("wasm_state")
		if wasmSnapshot != "" {
			var state map[string]interface{}
			if err := json.Unmarshal([]byte(wasmSnapshot), &state); err == nil {
				fmt.Fprintf(w, "Status: %v\n", state["initStatus"])
				
				if metrics, ok := state["performanceMetrics"].(map[string]interface{}); ok {
					fmt.Fprintf(w, "Operations: %v\n", state["totalOperations"])
					fmt.Fprintf(w, "Speed Ratio: %.2fx\n", metrics["speedupRatio"])
				}
				
				if memUsage, ok := state["memoryUsage"].(map[string]interface{}); ok {
					fmt.Fprintf(w, "Memory Usage: %.1f%%\n", memUsage["utilization"].(float64)*100)
				}
				
				if err, ok := state["lastError"].(map[string]interface{}); ok {
					fmt.Fprintf(w, "Last Error: %s\n", err["message"])
				}
			} else {
				fmt.Fprintln(w, "WebAssembly state available but failed to parse")
			}
		} else {
			fmt.Fprintln(w, "WebAssembly: Not initialized or state not available")
		}
		fmt.Fprintln(w, "")
	}

	// 7. System / runtime info
	fmt.Fprintln(w, "RUNTIME INFORMATION")
	fmt.Fprintln(w, "==================")
	runWithTimeout(10*time.Second, "RuntimeInfo", w, func() {
		fmt.Fprintln(w, NewRuntimeInfo().String())
	})

	// 8. Environment
	fmt.Fprintln(w, "ENVIRONMENT")
	fmt.Fprintln(w, "===========")
	printEnvironment(w)
	fmt.Fprint(w, "\n")

	// 9. Settings (mask API keys)
	fmt.Fprintln(w, "SETTINGS")
	fmt.Fprintln(w, "========")
	sanitizedSettings := settings
	sanitizedSettings.APIKeys.Replicate = MaskAPIKey(settings.APIKeys.Replicate)
	sanitizedSettings.APIKeys.ElevenLabs = MaskAPIKey(settings.APIKeys.ElevenLabs)
	sanitizedSettings.APIKeys.OpenAI = MaskAPIKey(settings.APIKeys.OpenAI)
	sanitizedSettings.APIKeys.OpenRouter = MaskAPIKey(settings.APIKeys.OpenRouter)
	sanitizedSettings.APIKeys.Google = MaskAPIKey(settings.APIKeys.Google)
	fmt.Fprintln(w, pp.Sprint(sanitizedSettings))

	// 10. Log history
	fmt.Fprintln(w, "LOG HISTORY")
	fmt.Fprintln(w, "===========")
	writeLogs(w, &logBuffer)

	// 11. Docker log history
	fmt.Fprintln(w, "DOCKER LOG HISTORY")
	fmt.Fprintln(w, "==================")
	writeLogs(w, &dockerutil.DockerLogBuffer)
	
	fmt.Fprintln(w, "DOCKER INFORMATION")
	fmt.Fprintln(w, "==================")
	runWithTimeout(15*time.Second, "DockerInfo", w, func() {
		captureDockerInfo(w)
	})
	fmt.Fprint(w, "\n")

	// 12. Connectivity status
	fmt.Fprintln(w, "CONNECTIVITY STATUS")
	fmt.Fprintln(w, "==================")
	// Not sure if some AI API services have georestrictions but when in doubt
	runWithTimeout(5*time.Second, "GetUserCountry", w, func() {
		if country, err := GetUserCountry(); err == nil {
			fmt.Fprintln(w, "Requests originate from:", country)
		}
	})
	
	runWithTimeout(5*time.Second, "CheckReplicate", w, func() {
		checkEndpointConnectivity(w, "https://replicate.com", "Replicate")
	})
	
	runWithTimeout(5*time.Second, "CheckElevenLabs", w, func() {
		checkEndpointConnectivity(w, "https://elevenlabs.io", "ElevenLabs")
	})
	
	runWithTimeout(10*time.Second, "DockerNslookup", w, func() {
		DockerNslookupCheck(w, "example.com")
	})
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

