package dialogs

import "errors"

// Compile-time check that QtFileDialog implements FileDialog
var _ FileDialog = (*QtFileDialog)(nil)

// QtFileDialog implements FileDialog using Qt WebEngine
// This is a placeholder for future Qt implementation
type QtFileDialog struct {
	// TODO: Add Qt-specific fields when implementing
}

// NewQtFileDialog creates a new Qt file dialog instance
func NewQtFileDialog() *QtFileDialog {
	return &QtFileDialog{}
}

// SaveFile opens a save file dialog
func (q *QtFileDialog) SaveFile(options SaveFileOptions) (string, error) {
	// TODO: Implement Qt save dialog
	return "", errors.New("Qt file dialogs not yet implemented")
}

// OpenFile opens an open file dialog
func (q *QtFileDialog) OpenFile(options OpenFileOptions) (string, error) {
	// TODO: Implement Qt open dialog
	return "", errors.New("Qt file dialogs not yet implemented")
}

// OpenDirectory opens an open directory dialog
func (q *QtFileDialog) OpenDirectory(options OpenDirectoryOptions) (string, error) {
	// TODO: Implement Qt directory dialog
	return "", errors.New("Qt file dialogs not yet implemented")
}