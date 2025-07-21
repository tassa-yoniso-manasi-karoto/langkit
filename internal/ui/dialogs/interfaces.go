package dialogs

// FileDialog interface provides runtime-agnostic file dialog operations
type FileDialog interface {
	SaveFile(options SaveFileOptions) (string, error)
	OpenFile(options OpenFileOptions) (string, error)
	OpenDirectory(options OpenDirectoryOptions) (string, error)
}

// FileFilter represents file type filters for dialogs
type FileFilter struct {
	DisplayName string // Filter information EG: "Image Files (*.jpg, *.png)"
	Pattern     string // semicolon separated list of extensions, EG: "*.jpg;*.png"
}

// SaveFileOptions contains options for save file dialogs
type SaveFileOptions struct {
	Title           string
	DefaultFilename string
	Filters         []FileFilter
}

// OpenFileOptions contains options for open file dialogs
type OpenFileOptions struct {
	Title   string
	Filters []FileFilter
}

// OpenDirectoryOptions contains options for open directory dialogs
type OpenDirectoryOptions struct {
	Title string
}