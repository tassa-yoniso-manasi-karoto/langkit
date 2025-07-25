package interfaces

import "context"

// ProcessingProvider interface for the processing service
type ProcessingProvider interface {
	// SendProcessingRequest starts a new processing task
	// The context is managed internally and can be cancelled via CancelProcessing()
	SendProcessingRequest(ctx context.Context, request interface{}) error
	
	// CancelProcessing cancels the current processing task
	// For explicit cancel button in UI
	CancelProcessing()
	
	// IsProcessing returns whether processing is currently active
	IsProcessing() bool
}