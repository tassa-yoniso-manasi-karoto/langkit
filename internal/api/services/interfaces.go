package services

import "github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"

// ProgressBroadcaster interface for broadcasting download progress
type ProgressBroadcaster interface {
	Broadcast(event string, data interface{})
}

// DryRunProvider interface for dry run testing and debugging operations
type DryRunProvider interface {
	SetDryRunConfig(config *core.DryRunConfig)
	InjectDryRunError(errorType string) error
	GetDryRunStatus() map[string]interface{}
}