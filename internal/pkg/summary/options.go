package summary

// Options contains configuration for generating summaries
type Options struct {
	// Required fields for selecting the LLM
	Provider string // e.g., "openai", "google", "openrouter"
	Model    string // Model ID e.g., "gpt-4o", "models/gemini-1.5-pro-latest"

	// Core summarization parameters
	OutputLanguage    string  // Target language for the summary (e.g., "English", "French"). Empty = LLM default.
	MaxLength         int     // Approximate target summary length in words (for the output summary). 0 means no specific length constraint.
	Temperature       float64 // Controls randomness (0.0-1.0). Negative means use LLM default.

	// Customization
	CustomPrompt string // If provided, this prompt is used directly, and subtitle text is appended.

	// For future use: Context from previous content (not implemented yet)
	// PreviousContentContext string
}

// DefaultOptions returns a new Options struct with sensible default values
func DefaultOptions() Options {
	return Options{
		// Provider and Model must be set by the caller based on user selection or application defaults.
		OutputLanguage:    "",  // Let LLM decide by default.
		MaxLength:         300, 
		Temperature:       0.7, 
		CustomPrompt:      "",
	}
}