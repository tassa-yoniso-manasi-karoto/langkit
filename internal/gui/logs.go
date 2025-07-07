package gui

import (
	"encoding/json"

	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)

// SetTraceLogs forwards the call to the GUI handler to toggle trace logs.
func (a *App) SetTraceLogs(enable bool) {
	if handler != nil {
		handler.SetTraceLogs(enable)
	}
}

// GetTraceLogs returns the current state of the trace log setting.
func (a *App) GetTraceLogs() bool {
	if handler != nil {
		return handler.GetTraceLogs()
	}
	return false
}

// getLogger returns the appropriate logger - handler's logger if available, otherwise app's logger
func (a *App) getLogger() *zerolog.Logger {
	if handler != nil {
		return handler.ZeroLog()
	}
	return a.logger
}

// BackendLoggerBatch handles batched log entries from the frontend
func (a *App) BackendLoggerBatch(component string, logsJson string) {
	// Validate input size
	if len(logsJson) > 1024*1024 { // 1MB max batch size
		a.getLogger().Error().
			Str("component", component).
			Int("size", len(logsJson)).
			Msg("Rejected oversized log batch")
		return
	}

	var logEntries []map[string]interface{}

	if err := json.Unmarshal([]byte(logsJson), &logEntries); err != nil {
		a.getLogger().Error().
			Err(err).
			Str("component", component).
			Msg("Failed to parse frontend log batch")
		return
	}

	// Only log batch processing for actual batches (more than 1 entry)
	if len(logEntries) > 1 {
		a.getLogger().Debug().
			Str("component", component).
			Int("count", len(logEntries)).
			Msg("Processing frontend log batch")
	}

	// Process each log entry
	for _, logEntry := range logEntries {
		a.processLogEntry(component, logEntry)
	}
}

// BackendLogger receives and processes individual log entries from the frontend
func (a *App) BackendLogger(component string, logJson string) {
	// Validate input size
	if len(logJson) > 100*1024 { // 100KB max for individual log
		a.getLogger().Error().
			Str("component", component).
			Int("size", len(logJson)).
			Msg("Rejected oversized log entry")
		return
	}

	var logEntry map[string]interface{}

	if err := json.Unmarshal([]byte(logJson), &logEntry); err != nil {
		a.getLogger().Error().
			Err(err).
			Str("component", component).
			Msg("Failed to parse frontend log entry")
		return
	}

	// Process the single log entry using the same logic as batch processing
	a.processLogEntry(component, logEntry)
}

// processLogEntry handles the common logic for processing a single log entry
func (a *App) processLogEntry(component string, logEntry map[string]interface{}) {
	// Map frontend log levels to zerolog levels
	level := zerolog.InfoLevel
	if levelVal, ok := logEntry["lvl"].(float64); ok {
		switch int(levelVal) {
		case -1: // TRACE
			level = zerolog.TraceLevel
		case 0: // DEBUG
			level = zerolog.DebugLevel
		case 1: // INFO
			level = zerolog.InfoLevel
		case 2: // WARN
			level = zerolog.WarnLevel
		case 3, 4: // ERROR, CRITICAL
			level = zerolog.ErrorLevel
		case 5: // FATAL
			level = zerolog.FatalLevel
		}
	}

	fields := map[string]interface{}{
		"origin": "gui", // Mark as frontend-originated to prevent feedback loop
	}

	// Extract component from log entry, fallback to function parameter
	if comp, ok := logEntry["comp"].(string); ok {
		fields["component"] = comp
	} else {
		fields["component"] = component
	}

	// Extract context information
	if context, ok := logEntry["ctx"].(map[string]interface{}); ok {
		for k, v := range context {
			fields["fe_"+k] = v
		}
	}

	// Add operation if present
	if operation, ok := logEntry["op"].(string); ok {
		fields["operation"] = operation
	}

	// Add session ID if present
	if sessionId, ok := logEntry["sid"].(string); ok {
		fields["sessionId"] = sessionId
	}

	// Get message and prepend FRONT: prefix
	message := "FRONT: "
	if msg, ok := logEntry["msg"].(string); ok {
		message += msg
	}

	// Log through the handler's logger to use its filtering pipeline
	if handler != nil {
		event := handler.ZeroLog().WithLevel(zerolog.Level(level))
		// Add fields
		event = event.Fields(fields)
		// Log the message
		event.Msg(message)
	} else {
		// Fallback to app logger if handler not initialized
		event := a.getLogger().WithLevel(level)
		event = event.Fields(fields)
		event.Msg(message)
	}
}

// RecordWasmState stores WebAssembly state in the crash reporter for diagnostics
func (a *App) RecordWasmState(stateJson string) {
	// Save state in crash reporter for diagnostic purposes
	if crash.Reporter != nil {
		crash.Reporter.SaveSnapshot("wasm_state", stateJson)
	}

	// Log state changes at debug level (to avoid spamming logs)
	a.getLogger().Debug().Msg("WebAssembly state updated")

	// Optional: Parse and log specific state changes (init status changes, errors, etc.)
	var state map[string]interface{}
	if err := json.Unmarshal([]byte(stateJson), &state); err != nil {
		a.getLogger().Error().Err(err).Msg("Failed to parse WebAssembly state")
		return
	}

	if status, ok := state["initStatus"].(string); ok {
		// Log status changes at info level
		a.getLogger().Info().Str("status", status).Msg("WebAssembly status updated")
	}

	// If there's an error, log it
	if lastError, ok := state["lastError"].(map[string]interface{}); ok {
		if errMsg, ok := lastError["message"].(string); ok {
			a.getLogger().Error().Str("source", "wasm").Msg(errMsg)
		}
	}
}

// RequestWasmState requests the WebAssembly state from the frontend for crash reports
func (a *App) RequestWasmState() {
	// Send an event to the frontend requesting the WebAssembly state
	a.getLogger().Debug().Msg("Requesting WebAssembly state from frontend")
	runtime.EventsEmit(a.ctx, "request-wasm-state")
}