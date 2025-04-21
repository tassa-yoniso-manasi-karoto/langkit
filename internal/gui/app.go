package gui

import (
	"context"
	"encoding/json"
	"os"
	"time"
	
	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/batch"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)

var handler *core.GUIHandler
var appThrottler *batch.AdaptiveEventThrottler

type App struct {
	ctx		context.Context
	procCancel	context.CancelFunc
	throttler   *batch.AdaptiveEventThrottler
	logger      *zerolog.Logger
}

func NewApp() *App {
	// Setup logger
	writer := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.TimeOnly,
	}
	logger := zerolog.New(writer).With().Timestamp().Str("module", "app").Logger()
	
	return &App{
		logger: &logger,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	
	a.logger.Info().Msg("Application starting up")
	
	// Initialize the throttler with default settings
	// These will be updated when settings are loaded
	a.throttler = batch.NewAdaptiveEventThrottler(
		ctx,
		0,                    // minInterval - will be updated from settings
		250*time.Millisecond, // maxInterval - will be updated from settings
		500*time.Millisecond, // rateWindow for measuring event frequency
		true,                 // enabled by default
		a.logger,             // Logger for throttler
	)
	
	// Store throttler references for global access
	appThrottler = a.throttler
	
	// Initialize handler with throttler
	handler = core.NewGUIHandler(ctx, a.throttler)
	
	a.logger.Debug().Msg("Event throttler initialized")
}

func (a *App) domReady(ctx context.Context) {
	a.logger.Debug().Msg("DOM ready, initializing settings")
	
	if err := config.InitConfig(""); err != nil {
		a.logger.Error().Err(err).Msg("Failed to initialize config")
		runtime.LogError(ctx, "Failed to initialize config: "+err.Error())
		return
	}

	// Load settings and emit to frontend
	settings, err := config.LoadSettings()
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to load settings")
		runtime.LogError(ctx, "Failed to load settings: "+err.Error())
		return
	}

	// Update throttler settings from config
	a.updateThrottlerSettings(settings)

	// Emit settings to frontend
	runtime.EventsEmit(ctx, "settings-loaded", settings)
	
	if settings.ShowLogViewerByDefault {
		runtime.WindowMaximise(ctx)
	}
	
	a.logger.Info().Msg("Application initialization complete")
}

// Update throttler settings when config changes
func (a *App) updateThrottlerSettings(settings config.Settings) {
	if a.throttler == nil {
		a.logger.Warn().Msg("Cannot update throttler settings: throttler is nil")
		return
	}
	
	// Convert milliseconds to time.Duration
	minInterval := time.Duration(settings.EventThrottling.MinInterval) * time.Millisecond
	maxInterval := time.Duration(settings.EventThrottling.MaxInterval) * time.Millisecond
	
	// Enforce reasonable limits
	if maxInterval < 50*time.Millisecond {
		maxInterval = 50 * time.Millisecond
	}
	if maxInterval > 1000*time.Millisecond {
		maxInterval = 1000 * time.Millisecond
	}
	
	// Set the throttler parameters
	a.throttler.SetMinInterval(minInterval)
	a.throttler.SetMaxInterval(maxInterval)
	a.throttler.SetEnabled(settings.EventThrottling.Enabled)
	
	a.logger.Debug().
		Bool("enabled", settings.EventThrottling.Enabled).
		Dur("minInterval", minInterval).
		Dur("maxInterval", maxInterval).
		Msg("Throttler settings updated")
}

// SetEventThrottling enables or disables the event throttling
func (a *App) SetEventThrottling(enabled bool) {
	if a.throttler != nil {
		a.throttler.SetEnabled(enabled)
		a.logger.Debug().Bool("enabled", enabled).Msg("Event throttling toggled")
	} else {
		a.logger.Warn().Msg("Cannot set throttling state: throttler is nil")
	}
}

// GetEventThrottlingStatus returns the current throttling status
func (a *App) GetEventThrottlingStatus() map[string]interface{} {
	if a.throttler == nil {
		a.logger.Warn().Msg("Cannot get throttling status: throttler is nil")
		return map[string]interface{}{
			"enabled": false,
			"currentRate": 0.0,
			"currentInterval": 0,
			"error": "Throttler not initialized",
		}
	}
	
	return a.throttler.GetStatus()
}


// BackendLoggerBatch handles batched log entries from the frontend
func (a *App) BackendLoggerBatch(component string, logsJson string) {
    // Validate input size
    if len(logsJson) > 1024*1024 { // 1MB max batch size
        a.logger.Error().
            Str("component", component).
            Int("size", len(logsJson)).
            Msg("Rejected oversized log batch")
        return
    }
    
    var logEntries []map[string]interface{}
    
    if err := json.Unmarshal([]byte(logsJson), &logEntries); err != nil {
        a.logger.Error().
            Err(err).
            Str("component", component).
            Msg("Failed to parse frontend log batch")
        return
    }
    
    a.logger.Debug().
        Str("component", component).
        Int("count", len(logEntries)).
        Msg("Processing frontend log batch")
    
    // Process each log entry
    for _, logEntry := range logEntries {
        // Map frontend log levels to zerolog levels
        level := zerolog.InfoLevel
        if levelVal, ok := logEntry["level"].(float64); ok {
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
            }
        }
        
        // Extract fields for structured logging
        fields := map[string]interface{}{
            "frontend": true,
            "component": component,
        }
        
        // Extract context information
        if context, ok := logEntry["context"].(map[string]interface{}); ok {
            for k, v := range context {
                fields["fe_"+k] = v
            }
        }
        
        // Add operation if present
        if operation, ok := logEntry["operation"].(string); ok {
            fields["operation"] = operation
        }
        
        // Add session ID if present
        if sessionId, ok := logEntry["sessionId"].(string); ok {
            fields["sessionId"] = sessionId
        }
        
        // Get message
        message := "Frontend log"
        if msg, ok := logEntry["message"].(string); ok {
            message = msg
        }
        
        // Log through the handler or directly to zerolog
        event := a.logger.WithLevel(level)
        
        // Add fields
        event = event.Fields(fields)
        
        // Log the message
        event.Msg(message)
    }
}

// RecordWasmLog receives and processes WebAssembly log entries from the frontend
func (a *App) RecordWasmLog(logJson string) {
	var logEntry map[string]interface{}
	
	if err := json.Unmarshal([]byte(logJson), &logEntry); err != nil {
		a.logger.Error().Err(err).Msg("Failed to parse WebAssembly log entry")
		return
	}
	
	// Convert to Zerolog level
	level := zerolog.InfoLevel
	if levelVal, ok := logEntry["level"].(float64); ok {
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
		}
	}
	
	// Extract fields for structured logging
	fields := map[string]interface{}{
		"origin": "gui",
	}
	
	if component, ok := logEntry["component"].(string); ok {
		fields["component"] = component
	}
	
	if metrics, ok := logEntry["metrics"].(map[string]interface{}); ok {
		for k, v := range metrics {
			fields["wasm_"+k] = v
		}
	}
	
	// Log through the throttler
	message := "WebAssembly log"
	if msg, ok := logEntry["message"].(string); ok {
		message = msg
	}
	
	// Use the handler to log the message with fields
	// FIXME USING ZEROLOG DIRECTLY IS LIKELY THE BEST WAY, NOT SURE.
	handler.LogFields(int8(level), "wasm", message, fields)
}

// RecordWasmState stores WebAssembly state in the crash reporter for diagnostics
func (a *App) RecordWasmState(stateJson string) {
	// Save state in crash reporter for diagnostic purposes
	if crash.Reporter != nil {
		crash.Reporter.SaveSnapshot("wasm_state", stateJson)
	}
	
	// Log state changes at debug level (to avoid spamming logs)
	a.logger.Debug().Msg("WebAssembly state updated")
	
	// Optional: Parse and log specific state changes (init status changes, errors, etc.)
	var state map[string]interface{}
	if err := json.Unmarshal([]byte(stateJson), &state); err != nil {
		a.logger.Error().Err(err).Msg("Failed to parse WebAssembly state")
		return
	}
	
	if status, ok := state["initStatus"].(string); ok {
		// Log status changes at info level
		a.logger.Info().Str("status", status).Msg("WebAssembly status updated")
	}
	
	// If there's an error, log it
	if lastError, ok := state["lastError"].(map[string]interface{}); ok {
		if errMsg, ok := lastError["message"].(string); ok {
			a.logger.Error().Str("source", "wasm").Msg(errMsg)
		}
	}
}

// RequestWasmState requests the WebAssembly state from the frontend for crash reports
func (a *App) RequestWasmState() {
	// Send an event to the frontend requesting the WebAssembly state
	a.logger.Debug().Msg("Requesting WebAssembly state from frontend")
	runtime.EventsEmit(a.ctx, "request-wasm-state")
}

// beforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue, false will continue shutdown as normal.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	// Request WebAssembly state for diagnostic purposes
	a.RequestWasmState()
	
	// Small delay to allow frontend to respond with state
	time.Sleep(100 * time.Millisecond)
	
	// Properly shut down the throttler
	if a.throttler != nil {
		a.logger.Info().Msg("Application closing, shutting down throttler")
		a.throttler.Shutdown()
		a.throttler = nil
	}
	return false
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	a.logger.Info().Msg("Application shutdown")
}