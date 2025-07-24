package gui

import (
	goruntime "runtime"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/version"
)

// GetSystemInfo returns the user's operating system and architecture.
func (a *App) GetSystemInfo() map[string]string {
	return map[string]string{
		"os":   goruntime.GOOS,
		"arch": goruntime.GOARCH,
	}
}

// GetCurrentTimestamp returns the current timestamp in milliseconds since Unix epoch,
// using the same format as log timestamps in pkg batch/throttler.go
func (a *App) GetCurrentTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// OpenURL opens a URL in the default browser
func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// GetVersion returns version information
func (a *App) GetVersion() version.Info {
	return version.GetInfo(false)  // Don't wait for update check in GUI calls
}