//go:build !windows

package executils

import (
	"context"
	"os/exec"
)

// NewCommand creates a standard *exec.Cmd for non-Windows platforms.
func NewCommand(name string, arg ...string) *exec.Cmd {
	return exec.Command(name, arg...)
}

// CommandContext creates an *exec.Cmd with context support for timeouts.
func CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, arg...)
}
