//go:build windows

package executils

import (
	"os/exec"
	"syscall"
)

// NewCommand creates an *exec.Cmd that is configured to run without
// creating a new console window on Windows.
func NewCommand(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	return cmd
}
