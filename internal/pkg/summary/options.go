package summary

// Options contains configuration for generating summaries
type Options struct {
	// Required fields
	Provider string // "openai", "langchain", "openrouter"
	Model    string // Model ID e.g., "gpt-4", "claude-3-opus"
	
	// Optional parameters with defaults
	MaxLength    int     // Target summary length in words
	Temperature  float64 // Controls randomness (0.0-1.0)
	Style        string  // Summary style: "brief", "detailed", "character-focused"
	Language     string  // Target language (empty = same as source)
	
	// Advanced options
	IncludeCharacters bool   // Include character descriptions
	IncludePlot       bool   // Include plot summary
	IncludeThemes     bool   // Include thematic analysis
	ToneStyle         string // "neutral", "analytical", "casual"
}

// DefaultOptions returns a new Options struct with default values
func DefaultOptions() Options {
	return Options{
		Provider:          "openai",
		Model:             "gpt-3.5-turbo",
		MaxLength:         500,
		Temperature:       0.7,
		Style:             "brief",
		IncludeCharacters: true,
		IncludePlot:       true,
		IncludeThemes:     false,
		ToneStyle:         "neutral",
	}
}