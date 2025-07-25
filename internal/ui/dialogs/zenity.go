package dialogs

import (
	"strings"
	"github.com/ncruces/zenity"
)

// Compile-time check that ZenityFileDialog implements FileDialog
var _ FileDialog = (*ZenityFileDialog)(nil)

// ZenityFileDialog implements FileDialog using Zenity for cross-platform native dialogs
type ZenityFileDialog struct {
	// No state needed - zenity is stateless
}

// NewZenityFileDialog creates a new Zenity file dialog instance
func NewZenityFileDialog() *ZenityFileDialog {
	return &ZenityFileDialog{}
}

// SaveFile opens a save file dialog
func (z *ZenityFileDialog) SaveFile(options SaveFileOptions) (string, error) {
	opts := []zenity.Option{
		zenity.ConfirmOverwrite(), // Always confirm overwrite for safety
	}
	
	if options.Title != "" {
		opts = append(opts, zenity.Title(options.Title))
	}
	
	if options.DefaultFilename != "" {
		opts = append(opts, zenity.Filename(options.DefaultFilename))
	}
	
	// Convert and add file filters
	for _, filter := range options.Filters {
		zenityFilter := convertFileFilter(filter)
		opts = append(opts, zenityFilter)
	}
	
	return zenity.SelectFileSave(opts...)
}

// OpenFile opens an open file dialog
func (z *ZenityFileDialog) OpenFile(options OpenFileOptions) (string, error) {
	opts := []zenity.Option{}
	
	if options.Title != "" {
		opts = append(opts, zenity.Title(options.Title))
	}
	
	// Convert and add file filters
	for _, filter := range options.Filters {
		zenityFilter := convertFileFilter(filter)
		opts = append(opts, zenityFilter)
	}
	
	return zenity.SelectFile(opts...)
}

// OpenDirectory opens an open directory dialog
func (z *ZenityFileDialog) OpenDirectory(options OpenDirectoryOptions) (string, error) {
	opts := []zenity.Option{
		zenity.Directory(),
	}
	
	if options.Title != "" {
		opts = append(opts, zenity.Title(options.Title))
	}
	
	return zenity.SelectFile(opts...)
}

// convertFileFilter converts our FileFilter to zenity's FileFilter option
func convertFileFilter(filter FileFilter) zenity.Option {
	// Split semicolon-separated patterns into individual patterns
	patterns := strings.Split(filter.Pattern, ";")
	
	// Trim any whitespace from patterns
	for i := range patterns {
		patterns[i] = strings.TrimSpace(patterns[i])
	}
	
	// Create zenity FileFilter
	return zenity.FileFilter{
		Name:     filter.DisplayName,
		Patterns: patterns,
		CaseFold: false, // Match our FileFilter behavior
	}
}