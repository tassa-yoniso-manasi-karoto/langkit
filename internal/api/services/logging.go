package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/generated"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/interfaces"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/batch"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)

// Compile-time check that LoggingService implements api.Service
var _ api.Service = (*LoggingService)(nil)

// LoggingService implements the WebRPC LoggingService interface
type LoggingService struct {
	logger     zerolog.Logger
	provider   interfaces.LoggingProvider
	wsServer   interfaces.WebsocketService
	throttler  *batch.AdaptiveEventThrottler
	handler    http.Handler
	guiHandler *core.GUIHandler
	appContext context.Context
}

// NewLoggingService creates a new logging service instance
func NewLoggingService(
	logger zerolog.Logger,
	provider interfaces.LoggingProvider,
	wsServer interfaces.WebsocketService,
	throttler *batch.AdaptiveEventThrottler,
	guiHandler *core.GUIHandler,
	appContext context.Context,
) *LoggingService {
	svc := &LoggingService{
		logger:     logger,
		provider:   provider,
		wsServer:   wsServer,
		throttler:  throttler,
		guiHandler: guiHandler,
		appContext: appContext,
	}
	
	// Create the WebRPC handler
	svc.handler = generated.NewLoggingServiceServer(svc)
	
	return svc
}

// Name implements api.Service
func (s *LoggingService) Name() string {
	return "LoggingService"
}

// Handler implements api.Service
func (s *LoggingService) Handler() http.Handler {
	return s.handler
}

// Description implements api.Service
func (s *LoggingService) Description() string {
	return "Logging and diagnostics service"
}

// BackendLogger receives and processes individual log entries from the frontend
func (s *LoggingService) BackendLogger(ctx context.Context, component string, logJson string) error {
	var logEntry map[string]interface{}

	if err := json.Unmarshal([]byte(logJson), &logEntry); err != nil {
		s.logger.Error().
			Err(err).
			Str("component", component).
			Msg("Failed to parse frontend log entry")
		return nil // Don't return error to frontend
	}

	// Process the single log entry
	s.processLogEntry(component, logEntry)
	return nil
}

// BackendLoggerBatch handles batched log entries from the frontend
func (s *LoggingService) BackendLoggerBatch(ctx context.Context, component string, logsJson string) error {
	var logEntries []map[string]interface{}

	if err := json.Unmarshal([]byte(logsJson), &logEntries); err != nil {
		s.logger.Error().
			Err(err).
			Str("component", component).
			Msg("Failed to parse frontend log batch")
		return nil // Don't return error to frontend
	}

	// Only log batch processing for actual batches (more than 1 entry)
	if len(logEntries) > 1 {
		s.logger.Debug().
			Str("component", component).
			Int("count", len(logEntries)).
			Msg("Processing frontend log batch")
	}

	// Process each log entry
	for _, logEntry := range logEntries {
		s.processLogEntry(component, logEntry)
	}

	return nil
}

// processLogEntry handles the common logic for processing a single log entry
func (s *LoggingService) processLogEntry(component string, logEntry map[string]interface{}) {
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

	// Log through the provider's logger
	event := s.provider.ZeroLog().WithLevel(level)
	event = event.Fields(fields)
	event.Msg(message)
}

// SetTraceLogs enables or disables sending trace-level logs to the GUI
func (s *LoggingService) SetTraceLogs(ctx context.Context, enable bool) error {
	s.provider.SetTraceLogs(enable)
	return nil
}

// GetTraceLogs returns the current state of the trace log setting
func (s *LoggingService) GetTraceLogs(ctx context.Context) (bool, error) {
	return s.provider.GetTraceLogs(), nil
}

// RecordWasmState stores WebAssembly state in the crash reporter for diagnostics
func (s *LoggingService) RecordWasmState(ctx context.Context, stateJson string) error {
	// Save state in crash reporter for diagnostic purposes
	if crash.Reporter != nil {
		crash.Reporter.SaveSnapshot("wasm_state", stateJson)
	}

	// Log state changes at debug level (to avoid spamming logs)
	s.logger.Debug().Msg("WebAssembly state updated")

	// Optional: Parse and log specific state changes (init status changes, errors, etc.)
	var state map[string]interface{}
	if err := json.Unmarshal([]byte(stateJson), &state); err != nil {
		s.logger.Error().Err(err).Msg("Failed to parse WebAssembly state")
		return nil // Don't return error to frontend
	}

	if status, ok := state["initStatus"].(string); ok {
		// Log status changes at info level
		s.logger.Info().Str("status", status).Msg("WebAssembly status updated")
	}

	// If there's an error, log it
	if lastError, ok := state["lastError"].(map[string]interface{}); ok {
		if errMsg, ok := lastError["message"].(string); ok {
			s.logger.Error().Str("source", "wasm").Msg(errMsg)
		}
	}

	return nil
}

// RequestWasmState requests the WebAssembly state from the frontend for crash reports
func (s *LoggingService) RequestWasmState(ctx context.Context) error {
	// Send an event to the frontend requesting the WebAssembly state
	s.logger.Debug().Msg("Requesting WebAssembly state from frontend")
	if s.wsServer != nil {
		s.wsServer.Emit("wasm.state.request", nil)
	}
	return nil
}

// ExportDebugReport compiles and prompts the user to save a debug report
func (s *LoggingService) ExportDebugReport(ctx context.Context) error {
	s.logger.Info().Msg("Exporting debug report")

	// Flush any pending events before generating report
	if s.throttler != nil {
		s.logger.Debug().Msg("Flushing throttler before generating debug report")
		s.throttler.SyncFlush()
	}

	// Request WebAssembly state for the report
	s.RequestWasmState(ctx)

	// Small delay to allow frontend to respond with state
	s.logger.Debug().Msg("Waiting for WebAssembly state response...")
	time.Sleep(300 * time.Millisecond)

	settings, err := config.LoadSettings()
	if err != nil {
		// Continue with empty settings if loading fails
		s.logger.Warn().Err(err).Msg("Failed to load settings for debug report")
		fmt.Printf("Warning: Failed to load settings: %v\n", err)
	}

	zipPath, err := crash.WriteReport(
		crash.ModeDebug,
		nil,
		settings,
		s.guiHandler.GetLogBuffer(),
		false,
	)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to write debug report")
		return err
	}

	// For now, just log the path and let the GUI layer handle the file dialog
	s.logger.Info().Str("path", zipPath).Msg("Debug report created successfully")
	if s.wsServer != nil {
		s.wsServer.Emit("debug.report.created", zipPath)
	}
	return nil
}

// SetEventThrottling enables or disables the event throttling
func (s *LoggingService) SetEventThrottling(ctx context.Context, enabled bool) error {
	if s.throttler != nil {
		s.throttler.SetEnabled(enabled)
		s.logger.Debug().Bool("enabled", enabled).Msg("Event throttling toggled")
	} else {
		s.logger.Warn().Msg("Cannot set throttling state: throttler is nil")
	}
	return nil
}

// GetEventThrottlingStatus returns the current throttling status
func (s *LoggingService) GetEventThrottlingStatus(ctx context.Context) (*generated.EventThrottlingStatus, error) {
	if s.throttler == nil {
		s.logger.Warn().Msg("Cannot get throttling status: throttler is nil")
		errorMsg := "Throttler not initialized"
		return &generated.EventThrottlingStatus{
			Enabled:         false,
			CurrentRate:     0.0,
			CurrentInterval: 0,
			Error:           &errorMsg,
		}, nil
	}

	status := s.throttler.GetStatus()
	
	result := &generated.EventThrottlingStatus{
		Enabled:         status["enabled"].(bool),
		CurrentRate:     status["currentRate"].(float64),
		CurrentInterval: int32(status["currentInterval"].(int)),
	}

	if err, ok := status["error"].(string); ok && err != "" {
		result.Error = &err
	}

	return result, nil
}

