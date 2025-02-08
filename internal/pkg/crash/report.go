package crash

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
	"sort"

	"github.com/klauspost/compress/zstd"
	"github.com/k0kubun/pp"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
)

func WriteReport(
	mainErr error,
	runtimeInfo string,
	settings config.Settings,
	logBuffer io.Reader,
	snapshots string,
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

	if err := writeReport(crashFile, mainErr, runtimeInfo, settings, logBuffer, snapshots); err != nil {
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


// writeReport writes all sections of the crash report to the given writer
func writeReport(
	w io.Writer,
	mainErr error,
	runtimeInfo string,
	settings config.Settings,
	logBuffer io.Reader,
	snapshots string,
) error {
	// Write header
	fmt.Fprintf(w, "LANGKIT CRASH REPORT\n")
	fmt.Fprintf(w, "==================\n")
	fmt.Fprintf(w, "This file has syntax highlighting through ANSI escape codes and is best viewed in a terminal using 'cat'.\n")
	fmt.Fprintf(w, "Timestamp: %s\n\n", time.Now().Format(time.RFC3339))

	// Write error information with unwrapping
	fmt.Fprintf(w, "ERROR DETAILS\n")
	fmt.Fprintf(w, "============\n")
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
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "STACK TRACE\n")
	fmt.Fprintf(w, "===========\n")
	fmt.Fprintf(w, "%s\n\n", string(debug.Stack()))

	fmt.Fprintf(w, "RUNTIME INFORMATION\n")
	fmt.Fprintf(w, "==================\n")
	fmt.Fprintf(w, "%s\n\n", runtimeInfo)

	fmt.Fprintf(w, "ENVIRONMENT\n")
	fmt.Fprintf(w, "===========\n")
	for _, env := range os.Environ() {
		if !containsSensitiveInfo(env) {
			fmt.Fprintf(w, "%s\n", env)
		}
	}
	fmt.Fprintf(w, "\n")
	
	if snapshots != "" {
		fmt.Fprintf(w, "PROCESSING SNAPSHOTS\n")
		fmt.Fprintf(w, "===================\n")
		fmt.Fprintf(w, "%s\n", snapshots)
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "SETTINGS\n")
	fmt.Fprintf(w, "========\n")
	sanitizedSettings := settings
	sanitizedSettings.APIKeys.Replicate = MaskAPIKey(settings.APIKeys.Replicate)
	sanitizedSettings.APIKeys.AssemblyAI = MaskAPIKey(settings.APIKeys.AssemblyAI)
	sanitizedSettings.APIKeys.ElevenLabs = MaskAPIKey(settings.APIKeys.ElevenLabs)
	fmt.Fprintf(w, pp.Sprint(sanitizedSettings))
	fmt.Fprintf(w, "\n\n")

	// Write log history
	fmt.Fprintf(w, "LOG HISTORY\n")
	fmt.Fprintf(w, "===========\n")
	if logBuffer != nil {
		if _, err := io.Copy(w, logBuffer); err != nil {
			return fmt.Errorf("failed to write log history: %w", err)
		}
	} else {
		fmt.Fprintf(w, "No log history available")
	}
	fmt.Fprintf(w, "\n\n")

	// Takes the longest, keep it last
	fmt.Fprintf(w, "CONNECTIVITY STATUS\n")
	fmt.Fprintf(w, "==================\n")
	checkEndpointConnectivity(w, "https://replicate.com", "Replicate")
	checkEndpointConnectivity(w, "https://www.assemblyai.com/", "AssemblyAI")
	checkEndpointConnectivity(w, "https://elevenlabs.io", "ElevenLabs")
	fmt.Fprintf(w, "\n")
	
	return nil
}


// compressReport compresses the crash report using zstd
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


// checkEndpointConnectivity tests connectivity to a given endpoint
func checkEndpointConnectivity(w io.Writer, url, name string) {
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	start := time.Now()
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		fmt.Fprintf(w, "%s: Failed to create request - %v\n", name, err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(w, "%s: Failed to connect - %v\n", name, err)
		return
	}
	defer resp.Body.Close()

	latency := time.Since(start)
	
	fmt.Fprintf(w, "%s: Status %s (latency: %s)\n",
		name,
		resp.Status,
		formatDuration(latency),
	)
}


func CleanUpReportsOnDisk(crashDir string) {
	pattern := filepath.Join(crashDir, "crash_*.zst")
	
	matches, _ := filepath.Glob(pattern)
	
	// Only keep last 5 crash reports
	if len(matches) >= 5 {
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


func GetCrashDir() string  {
	dir, _ := config.GetConfigDir()
	dir = filepath.Join(dir, "crashes")
	os.MkdirAll(dir, 0755)
	return dir
}
