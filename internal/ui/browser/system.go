package browser

import (
	"runtime"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
)

// Compile-time check that SystemURLOpener implements URLOpener
var _ URLOpener = (*SystemURLOpener)(nil)

// SystemURLOpener implements URLOpener using system commands
type SystemURLOpener struct{}

// NewSystemURLOpener creates a new system URL opener instance
func NewSystemURLOpener() *SystemURLOpener {
	return &SystemURLOpener{}
}

// OpenURL opens a URL in the user's default browser using system commands
func (s *SystemURLOpener) OpenURL(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return executils.NewCommand("open", url).Start()
	case "windows":
		return executils.NewCommand("cmd", "/c", "start", url).Start()
	default: // linux and others
		return executils.NewCommand("xdg-open", url).Start()
	}
}