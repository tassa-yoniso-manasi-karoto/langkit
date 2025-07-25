package browser

import (
	"context"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Compile-time check that WailsURLOpener implements URLOpener
var _ URLOpener = (*WailsURLOpener)(nil)

// WailsURLOpener implements URLOpener using Wails runtime
type WailsURLOpener struct {
	ctx context.Context
}

// NewWailsURLOpener creates a new Wails URL opener instance
func NewWailsURLOpener(ctx context.Context) *WailsURLOpener {
	return &WailsURLOpener{ctx: ctx}
}

// OpenURL opens a URL in the user's default browser
func (w *WailsURLOpener) OpenURL(url string) error {
	runtime.BrowserOpenURL(w.ctx, url)
	return nil
}