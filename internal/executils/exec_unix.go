//go:build !windows

package executils

import (
	"os/exec"
)

// NewCommand creates a standard *exec.Cmd for non-Windows platforms.
func NewCommand(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}
