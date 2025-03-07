package profiling

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
)

// getPprofDir creates and returns a directory for storing pprof data
func GetPprofDir() (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}
	
	// Create a dedicated pprof directory
	pprofDir := filepath.Join(configDir, "pprof")
	if err := os.MkdirAll(pprofDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create pprof directory: %w", err)
	}
	
	return pprofDir, nil
}

// IsCPUProfilingEnabled returns true if CPU profiling is enabled via environment variable
func IsCPUProfilingEnabled() bool {
	return os.Getenv("LANGKIT_PROFILE_CPU") == "1" || 
	       os.Getenv("LANGKIT_PROFILE_CPU") == "true" ||
	       os.Getenv("LANGKIT_PROFILE_CPU") == "yes"
}

// IsMemoryProfilingEnabled returns true if memory profiling is enabled via environment variable
func IsMemoryProfilingEnabled() bool {
	return os.Getenv("LANGKIT_PROFILE_MEMORY") == "1" || 
	       os.Getenv("LANGKIT_PROFILE_MEMORY") == "true" ||
	       os.Getenv("LANGKIT_PROFILE_MEMORY") == "yes"
}

// StartCPUProfile starts CPU profiling and returns the file and error
func StartCPUProfile(name string) (*os.File, error) {
	// Check if profiling is enabled via environment variable
	if !IsCPUProfilingEnabled() {
		return nil, nil
	}
	
	pprofDir, err := GetPprofDir()
	if err != nil {
		return nil, err
	}
	
	// Create a timestamped filename for the profile
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(pprofDir, fmt.Sprintf("cpu_%s_%s.pprof", name, timestamp))
	
	f, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create CPU profile file: %w", err)
	}
	
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to start CPU profile: %w", err)
	}
	
	return f, nil
}

// StopCPUProfile stops CPU profiling and closes the file
func StopCPUProfile(f *os.File) {
	if f != nil {
		pprof.StopCPUProfile()
		f.Close()
	}
}

// WriteMemoryProfile writes the current memory profile to a file
func WriteMemoryProfile(name string) error {
	// Check if profiling is enabled via environment variable
	if !IsMemoryProfilingEnabled() {
		return nil
	}
	
	pprofDir, err := GetPprofDir()
	if err != nil {
		return err
	}
	
	// Create a timestamped filename for the profile
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(pprofDir, fmt.Sprintf("memory_%s_%s.pprof", name, timestamp))
	
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create memory profile file: %w", err)
	}
	defer f.Close()
	
	if err := pprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("failed to write memory profile: %w", err)
	}
	
	return nil
}

// StartMemoryProfiler starts a goroutine that periodically writes memory profiles
// It returns a channel that can be closed to stop the profiler
func StartMemoryProfiler(name string, interval time.Duration) chan struct{} {
	// Check if profiling is enabled via environment variable
	if !IsMemoryProfilingEnabled() {
		return nil
	}
	
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				if err := WriteMemoryProfile(name); err != nil {
					// Log error but continue
					fmt.Fprintf(os.Stderr, "Error writing memory profile: %v\n", err)
				}
			case <-done:
				return
			}
		}
	}()
	
	return done
}