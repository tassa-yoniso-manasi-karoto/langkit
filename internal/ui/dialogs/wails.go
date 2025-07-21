package dialogs

import (
	"context"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Compile-time check that WailsFileDialog implements FileDialog
var _ FileDialog = (*WailsFileDialog)(nil)

// WailsFileDialog implements FileDialog using Wails runtime
type WailsFileDialog struct {
	ctx context.Context
}

// NewWailsFileDialog creates a new Wails file dialog instance
func NewWailsFileDialog(ctx context.Context) *WailsFileDialog {
	return &WailsFileDialog{ctx: ctx}
}

// SaveFile opens a save file dialog
func (w *WailsFileDialog) SaveFile(options SaveFileOptions) (string, error) {
	wailsOptions := runtime.SaveDialogOptions{
		Title:           options.Title,
		DefaultFilename: options.DefaultFilename,
		Filters:         convertFilters(options.Filters),
	}
	return runtime.SaveFileDialog(w.ctx, wailsOptions)
}

// OpenFile opens an open file dialog
func (w *WailsFileDialog) OpenFile(options OpenFileOptions) (string, error) {
	wailsOptions := runtime.OpenDialogOptions{
		Title:   options.Title,
		Filters: convertFilters(options.Filters),
	}
	return runtime.OpenFileDialog(w.ctx, wailsOptions)
}

// OpenDirectory opens an open directory dialog
func (w *WailsFileDialog) OpenDirectory(options OpenDirectoryOptions) (string, error) {
	wailsOptions := runtime.OpenDialogOptions{
		Title: options.Title,
	}
	return runtime.OpenDirectoryDialog(w.ctx, wailsOptions)
}

// convertFilters converts our FileFilter to Wails FileFilter
func convertFilters(filters []FileFilter) []runtime.FileFilter {
	wailsFilters := make([]runtime.FileFilter, len(filters))
	for i, filter := range filters {
		wailsFilters[i] = runtime.FileFilter{
			DisplayName: filter.DisplayName,
			Pattern:     filter.Pattern,
		}
	}
	return wailsFilters
}