//go:build !windows

package executils

import (
	"bufio"
	"os/exec"
	"regexp"
	"runtime"
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
	var gpus []string

	// Try lspci
	cmd := exec.Command("lspci", "-nn")
	cmd.Env = append(cmd.Env, "LC_ALL=C") // Force English output

	output, err := runWithTimeout(cmd, 5*time.Second)
	if err != nil {
		return gpus
	}

	// Match VGA, 3D controller, Display controller
	scanner := bufio.NewScanner(strings.NewReader(output))
	deviceRegex := regexp.MustCompile(`:\s*(.+)$`)

	for scanner.Scan() {
		line := strings.ToLower(scanner.Text())
		if strings.Contains(line, "vga") ||
			strings.Contains(line, "3d controller") ||
			strings.Contains(line, "display controller") {
			// Extract device name after the last colon
			if match := deviceRegex.FindStringSubmatch(scanner.Text()); len(match) > 1 {
				gpus = append(gpus, strings.TrimSpace(match[1]))
			}
		}
	}

	return gpus
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
