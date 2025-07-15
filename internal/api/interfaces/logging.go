package interfaces

import "github.com/rs/zerolog"

// LoggingProvider interface for the logging service
type LoggingProvider interface {
	// SetTraceLogs enables or disables sending trace-level logs to the GUI
	SetTraceLogs(enable bool)
	
	// GetTraceLogs returns the current state of the trace log setting
	GetTraceLogs() bool
	
	// ZeroLog returns the logger instance
	ZeroLog() *zerolog.Logger
}