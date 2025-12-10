//go:build windows

package executils

import (
	"bufio"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// DetectGPUs returns a list of detected GPU names using wmic on Windows.
func DetectGPUs() []string {
	var gpus []string

	cmd := exec.Command("wmic", "path", "win32_VideoController", "get", "name")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := runWithTimeout(cmd, 5*time.Second)
	if err != nil {
		return []string{"unknown"}
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip header and empty lines
		if line != "" && strings.ToLower(line) != "name" {
			gpus = append(gpus, line)
		}
	}

	if len(gpus) == 0 {
		return []string{"unknown"}
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
