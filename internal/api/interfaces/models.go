package interfaces

// STTModelProvider interface for Speech-to-Text model operations
type STTModelProvider interface {
	// GetAllSTTModels returns all available STT models
	// Returns interface{} to avoid importing voice package types
	GetAllSTTModels() interface{}
	
	// UpdateSTTFactory updates the STT factory after settings change
	UpdateSTTFactory()
}

// LLMRegistryProvider interface for LLM registry operations
type LLMRegistryProvider interface {
	// GetLLMRegistry returns the LLM registry instance
	// Returns interface{} to avoid importing llms package types
	GetLLMRegistry() interface{}
	
	// GetSummaryService returns the summary service instance
	// Returns interface{} to avoid importing summary package types
	GetSummaryService() interface{}
}