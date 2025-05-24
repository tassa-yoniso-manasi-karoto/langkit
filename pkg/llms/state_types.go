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
	Status      string       `json:"status"`      // "not_attempted", "initializing_models", "models_loaded", "ready", "error"
	Error       string       `json:"error,omitempty"`  // Changed from error to string for JSON serialization
	Models      []ModelInfo  `json:"models,omitempty"`
	LastUpdated time.Time    `json:"lastUpdated"`
}

// StateChange is the event payload sent when provider states change
type StateChange struct {
	Timestamp              time.Time                    `json:"timestamp"`
	GlobalState            GlobalServiceState           `json:"globalState"`
	UpdatedProviderName    string                       `json:"updatedProviderName,omitempty"`
	ProviderStatesSnapshot map[string]ProviderState     `json:"providerStatesSnapshot"`
	Message                string                       `json:"message,omitempty"`
}