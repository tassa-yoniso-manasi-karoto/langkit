package llms

// ModelInfo contains metadata about a model
type ModelInfo struct {
	ID           string   // Unique identifier for the model
	Name         string   // Display name for the model
	Description  string   // Brief description of the model
	MaxTokens    int      // Maximum context length supported
	Capabilities []string // Capabilities like "summarization", "creative", etc.
	ProviderName string   // The provider this model belongs to
}

// CompletionRequest contains parameters for a completion request
type CompletionRequest struct {
	Prompt           string   // The main prompt or user input
	MaxTokens        int      // Maximum tokens to generate
	Temperature      float64  // Controls randomness (0.0-1.0)
	TopP             float64  // Nucleus sampling parameter
	StopSequences    []string // Sequences to stop generation
	User             string   // Optional user identifier for billing/monitoring
	SystemPrompt     string   // System-level instruction prompt
	Model            string   // Model ID to use
	Stream           bool     // Whether to stream the response
	IncludeUsage     bool     // (New) For streaming, whether to request usage stats in the final chunk
	N                int64    // (New) How many chat completion choices to generate
	FrequencyPenalty float64  // (New) Penalize based on existing frequency
	PresencePenalty  float64  // (New) Penalize based on presence so far
	Seed             int64    // (New) For deterministic sampling if supported
}

// CompletionResponse contains the response from a completion request
type CompletionResponse struct {
	Text         string      // Generated text
	FinishReason string      // Why generation stopped
	Usage        TokenUsage  // Token usage statistics
	Model        string      // Model used for generation
	Provider     string      // Provider that served the request
}

// TokenUsage tracks token consumption
type TokenUsage struct {
	PromptTokens     int // Number of tokens in the prompt
	CompletionTokens int // Number of tokens in the completion
	TotalTokens      int // Total tokens used
}