//go:build !windows

package executils

import (
	"bufio"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// DetectGPUs returns a list of detected GPU names using platform-specific methods.
func DetectGPUs() []string {
	var gpus []string

	switch runtime.GOOS {
	case "linux":
		gpus = detectGPUsLinux()
	case "darwin":
		gpus = detectGPUsMacOS()
	}

	if len(gpus) == 0 {
		return []string{"unknown"}
	}
	return gpus
}

func detectGPUsLinux() []string {
	var activeGPUs []string
	var inactiveGPUs []string

	// Try lspci
	cmd := exec.Command("lspci", "-nn")
	cmd.Env = append(cmd.Env, "LC_ALL=C") // Force English output

	output, err := runWithTimeout(cmd, 5*time.Second)
	if err != nil {
		return nil
	}

	// lspci -nn format: "01:00.0 VGA compatible controller [0300]: NVIDIA Corporation..."
	// Extract device name after "]: " (after the class code in brackets)
	deviceRegex := regexp.MustCompile(`\]:\s*(.+)$`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		lineLower := strings.ToLower(line)

		// Check if it's a GPU-related device
		isVGA := strings.Contains(lineLower, "vga compatible controller")
		is3D := strings.Contains(lineLower, "3d controller")
		isDisplay := strings.Contains(lineLower, "display controller")
		// MUX-switched inactive GPU shows as this on some systems
		isInactiveGPU := strings.Contains(lineLower, "non-vga unclassified") &&
			(strings.Contains(lineLower, "intel") || strings.Contains(lineLower, "nvidia") || strings.Contains(lineLower, "amd"))

		if isVGA || is3D || isDisplay || isInactiveGPU {
			if match := deviceRegex.FindStringSubmatch(line); len(match) > 1 {
				name := strings.TrimSpace(match[1])

				// VGA controller = active GPU (with MUX switch)
				if isVGA {
					activeGPUs = append(activeGPUs, name+" (ACTIVE)")
				} else if isInactiveGPU {
					inactiveGPUs = append(inactiveGPUs, name)
				} else {
					// 3D controller or display controller (could be active too)
					activeGPUs = append(activeGPUs, name)
				}
			}
		}
	}

	// Active GPUs first
	return append(activeGPUs, inactiveGPUs...)
}

func detectGPUsMacOS() []string {
	var gpus []string

	cmd := exec.Command("system_profiler", "SPDisplaysDataType")
	output, err := runWithTimeout(cmd, 10*time.Second)
	if err != nil {
		return gpus
	}

	// Look for "Chipset Model:" lines
	scanner := bufio.NewScanner(strings.NewReader(output))
	chipsetRegex := regexp.MustCompile(`Chipset Model:\s*(.+)$`)

	for scanner.Scan() {
		if match := chipsetRegex.FindStringSubmatch(scanner.Text()); len(match) > 1 {
			gpus = append(gpus, strings.TrimSpace(match[1]))
		}
	}

	return gpus
}

func runWithTimeout(cmd *exec.Cmd, timeout time.Duration) (string, error) {
	done := make(chan struct{})
	var output []byte
	var err error

	go func() {
		output, err = cmd.Output()
		close(done)
	}()

	select {
	case <-done:
		return string(output), err
	case <-time.After(timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return "", exec.ErrNotFound
	}
}

// GetNvidiaVRAMMiB returns the total VRAM in MiB for the first NVIDIA GPU.
// Returns 0 if nvidia-smi is not available or no GPU is found.
func GetNvidiaVRAMMiB() int {
	// Use nvidia-smi query for clean output: "12227 MiB" or just "12227"
	cmd := exec.Command("nvidia-smi", "--query-gpu=memory.total", "--format=csv,noheader,nounits")
	output, err := runWithTimeout(cmd, 5*time.Second)
	if err != nil {
		return 0
	}

	// Parse first line (first GPU)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return 0
	}

	vram, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return 0
	}

	return vram
}
