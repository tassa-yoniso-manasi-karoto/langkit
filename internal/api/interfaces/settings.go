package interfaces

// SettingsProvider interface for settings service side effects
type SettingsProvider interface {
	// UpdateThrottlerSettings updates the throttler with new settings
	// The settings parameter should be config.Settings but we use interface{} to avoid import cycle
	UpdateThrottlerSettings(settings interface{})
	
	// TriggerLLMRegistryUpdate triggers the LLM registry to update with new settings
	// The settings parameter should be config.Settings but we use interface{} to avoid import cycle
	TriggerLLMRegistryUpdate(settings interface{})
}