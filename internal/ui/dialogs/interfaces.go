package dialogs

// FileDialog interface provides runtime-agnostic file dialog operations
type FileDialog interface {
	SaveFile(options SaveFileOptions) (string, error)
	OpenFile(options OpenFileOptions) (string, error)
	OpenDirectory(options OpenDirectoryOptions) (string, error)
}

// MessageType represents the type of message dialog
type MessageType int

const (
	MessageInfo MessageType = iota
	MessageWarning
	MessageError
	MessageQuestion
)

// MessageDialog provides an interface for displaying message dialogs
type MessageDialog interface {
	// ShowMessage displays a message dialog with the given title, message, and type.
	// For MessageQuestion, returns true if user clicked OK/Yes, false otherwise.
	// For other types, the bool return value is not meaningful.
	ShowMessage(title, message string, msgType MessageType) (bool, error)
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