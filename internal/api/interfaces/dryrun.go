package interfaces

// DryRunProvider interface for dry run testing and debugging operations
type DryRunProvider interface {
	// SetDryRunConfig sets the dry run configuration
	// The config parameter should be *core.DryRunConfig
	SetDryRunConfig(config interface{})
	InjectDryRunError(errorType string) error
	GetDryRunStatus() map[string]interface{}
}