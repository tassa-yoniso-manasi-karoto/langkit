package interfaces

// MediaProvider interface for media operations
type MediaProvider interface {
	// GetVideosInDirectory returns video files in a directory
	// Returns []VideoInfo as interface{} to avoid import cycle
	GetVideosInDirectory(dirPath string) ([]interface{}, error)
	
	// CheckMediaLanguageTags checks if media has language tags
	// Returns MediaLanguageInfo as interface{} to avoid import cycle
	CheckMediaLanguageTags(path string) (interface{}, error)
}