package browser

import (
	"runtime"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
)

// Compile-time check that ZenityURLOpener implements URLOpener
var _ URLOpener = (*ZenityURLOpener)(nil)

// ZenityURLOpener implements URLOpener using system commands
type ZenityURLOpener struct{}

// NewZenityURLOpener creates a new Zenity URL opener instance
func NewZenityURLOpener() *ZenityURLOpener {
	return &ZenityURLOpener{}
}

// OpenURL opens a URL in the user's default browser using system commands
func (z *ZenityURLOpener) OpenURL(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return executils.NewCommand("open", url).Start()
	case "windows":
		return executils.NewCommand("cmd", "/c", "start", url).Start()
	default: // linux and others
		return executils.NewCommand("xdg-open", url).Start()
	}
}