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

// Compile-time check that WailsMessageDialog implements MessageDialog
var _ MessageDialog = (*WailsMessageDialog)(nil)

// WailsMessageDialog implements MessageDialog using Wails runtime
type WailsMessageDialog struct {
	ctx context.Context
}

// NewWailsMessageDialog creates a new Wails message dialog instance
func NewWailsMessageDialog(ctx context.Context) *WailsMessageDialog {
	return &WailsMessageDialog{ctx: ctx}
}

// ShowMessage displays a message dialog using Wails runtime
func (w *WailsMessageDialog) ShowMessage(title, message string, msgType MessageType) (bool, error) {
	var dialogType runtime.DialogType

	switch msgType {
	case MessageInfo:
		dialogType = runtime.InfoDialog
	case MessageWarning:
		dialogType = runtime.WarningDialog
	case MessageError:
		dialogType = runtime.ErrorDialog
	case MessageQuestion:
		dialogType = runtime.QuestionDialog
	default:
		dialogType = runtime.InfoDialog
	}

	options := runtime.MessageDialogOptions{
		Type:    dialogType,
		Title:   title,
		Message: message,
	}

	result, err := runtime.MessageDialog(w.ctx, options)
	if err != nil {
		return false, err
	}

	// For question dialogs, check if user clicked Yes/OK
	return result == "Yes" || result == "Ok", nil
}