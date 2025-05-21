package llms

import (
	"time"
)

// GlobalServiceState represents the overall status of the LLM system
type GlobalServiceState string

const (
	GSUninitialized GlobalServiceState = "uninitialized"
	GSInitializing  GlobalServiceState = "initializing"
	GSReady         GlobalServiceState = "ready"
	GSError         GlobalServiceState = "error"
	GSUpdating      GlobalServiceState = "updating"
)

// String returns a string representation of GlobalServiceState
func (s GlobalServiceState) String() string {
	return string(s)
}

// ProviderState represents the status of a single LLM provider
type ProviderState struct {
	Status      string       // "not_attempted", "initializing_models", "models_loaded", "ready", "error"
	Error       error        // Error object if Status is "error"
	Models      []ModelInfo  // Available models for this provider
	LastUpdated time.Time    // Timestamp of last update
}

// StateChange is the event payload sent when provider states change
type StateChange struct {
	Timestamp              time.Time
	GlobalState            GlobalServiceState
	UpdatedProviderName    string                   // Optional: if specific to one provider
	ProviderStatesSnapshot map[string]ProviderState // Copy of current providerStates
	Message                string                   // Optional: details about the change
}