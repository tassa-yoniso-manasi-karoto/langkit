//go:build windows

package executils

import (
	"bytes"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
)

// DetectGPUs returns a list of detected GPU names on Windows.
// Tries PowerShell first (modern), falls back to wmic (legacy).
func DetectGPUs() []string {
	// Try PowerShell first (more reliable on modern Windows)
	if gpus := detectGPUsPowerShell(); len(gpus) > 0 {
		return gpus
	}

	// Fall back to wmic for older systems
	if gpus := detectGPUsWmic(); len(gpus) > 0 {
		return gpus
	}

	return []string{"unknown"}
}

func detectGPUsPowerShell() []string {
	var gpus []string

	// PowerShell command to get GPU names
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"Get-CimInstance -ClassName Win32_VideoController | Select-Object -ExpandProperty Name")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := runWithTimeout(cmd, 5*time.Second)
	if err != nil {
		return gpus
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			gpus = append(gpus, line)
		}
	}

	return gpus
}

func detectGPUsWmic() []string {
	var gpus []string

	cmd := exec.Command("wmic", "path", "win32_VideoController", "get", "name")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := runWithTimeout(cmd, 5*time.Second)
	if err != nil {
		return gpus
	}

	// wmic outputs UTF-16 LE, try to decode it
	decoded := decodeUTF16IfNeeded([]byte(output))

	for _, line := range strings.Split(decoded, "\n") {
		line = strings.TrimSpace(line)
		// Skip header and empty lines
		if line != "" && strings.ToLower(line) != "name" {
			gpus = append(gpus, line)
		}
	}

	return gpus
}

// decodeUTF16IfNeeded attempts to decode UTF-16 LE to UTF-8 if BOM is present
func decodeUTF16IfNeeded(data []byte) string {
	// Check for UTF-16 LE BOM (0xFF 0xFE)
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		data = data[2:] // Skip BOM

		// Convert UTF-16 LE to string
		if len(data)%2 != 0 {
			data = data[:len(data)-1] // Ensure even length
		}

		u16s := make([]uint16, len(data)/2)
		for i := 0; i < len(u16s); i++ {
			u16s[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
		}

		runes := utf16.Decode(u16s)
		return string(runes)
	}

	// Also try without BOM (wmic sometimes omits it)
	// Check if it looks like UTF-16 (alternating null bytes)
	if len(data) >= 4 && data[1] == 0 && data[3] == 0 {
		if len(data)%2 != 0 {
			data = data[:len(data)-1]
		}

		u16s := make([]uint16, len(data)/2)
		for i := 0; i < len(u16s); i++ {
			u16s[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
		}

		runes := utf16.Decode(u16s)
		// Filter out null runes
		var buf bytes.Buffer
		for _, r := range runes {
			if r != 0 {
				buf.WriteRune(r)
			}
		}
		return buf.String()
	}

	return string(data)
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
