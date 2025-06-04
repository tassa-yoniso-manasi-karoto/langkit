package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
	
	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	
	"github.com/tassa-yoniso-manasi-karoto/dockerutil"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/batch"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/summary"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

var handler *core.GUIHandler
var appThrottler *batch.AdaptiveEventThrottler

type App struct {
	ctx		     context.Context
	procCancel	 context.CancelFunc
	throttler    *batch.AdaptiveEventThrottler
	logger       *zerolog.Logger
	llmRegistry  *llms.Registry  // LLM Registry for async provider management
	wsServer     *WebSocketServer // WebSocket server for state updates
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

func (a *App) bindEnvironmentVariables() {
	a.logger.Debug().Msg("Binding environment variables to config")
	
	// Set environment prefix and automatic env
	viper.SetEnvPrefix("LANGKIT")
	viper.AutomaticEnv()
	
	// Bind specific environment variables to their config counterparts
	envBindings := map[string]string{
		"REPLICATE_API_KEY":  "api_keys.replicate",
 
		"ELEVENLABS_API_KEY": "api_keys.elevenlabs",
		"OPENAI_API_KEY":     "api_keys.openai",
		"OPENROUTER_API_KEY": "api_keys.openrouter",
		"GOOGLE_API_KEY":     "api_keys.google",
	}

	for env, conf := range envBindings {
		if err := viper.BindEnv(conf, env); err != nil {
			a.logger.Error().Str("env", env).Err(err).Msg("Failed to bind environment variable")
		} else {
			a.logger.Debug().Str("env", env).Str("config", conf).Msg("Bound environment variable to config")
		}
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
	
	// Create WebSocket server for LLM state updates
	wsServer, err := NewWebSocketServer(*a.logger)
	if err != nil {
		a.logger.Fatal().Err(err).Msg("Failed to create WebSocket server")
	}
	a.wsServer = wsServer
	a.logger.Info().Int("port", wsServer.GetPort()).Msg("WebSocket server created")
}

func (a *App) domReady(ctx context.Context) {
	a.logger.Debug().Msg("DOM ready, initializing settings")
	
	// Bind environment variables to config
	a.bindEnvironmentVariables()
	
	// Load settings
	settings, err := config.LoadSettings()
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to load settings")
	}
	
	if err := config.InitConfig(""); err != nil {
		a.logger.Error().Err(err).Msg("Failed to initialize config")
		runtime.LogError(ctx, "Failed to initialize config: "+err.Error())
		return
	}
	
	// Update throttler settings from config
	a.updateThrottlerSettings(settings)

	// Emit settings to frontend
	runtime.EventsEmit(ctx, "settings-loaded", settings)
	
	if settings.ShowLogViewerByDefault {
		runtime.WindowMaximise(ctx)
	}
	
	// Initialize LLM system with async registry and WebSocket server
	a.llmRegistry = core.InitLLM(handler, a.ctx, a.wsServer)
	a.logger.Info().Msg("LLM registry initialized")
	
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

// GetWebSocketPort returns the port the WebSocket server is listening on
func (a *App) GetWebSocketPort() (int, error) {
	if a.wsServer == nil {
		return 0, fmt.Errorf("WebSocket server not initialized")
	}
	return a.wsServer.GetPort(), nil
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

	// Only log batch processing for actual batches (more than 1 entry)
	if len(logEntries) > 1 {
		a.logger.Debug().
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
		a.logger.Error().
			Str("component", component).
			Int("size", len(logJson)).
			Msg("Rejected oversized log entry")
		return
	}

	var logEntry map[string]interface{}

	if err := json.Unmarshal([]byte(logJson), &logEntry); err != nil {
		a.logger.Error().
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
	
	fields := map[string]interface{}{}

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

	// Log through the handler or directly to zerolog
	event := a.logger.WithLevel(level)

	// Add fields
	event = event.Fields(fields)

	// Log the message
	event.Msg(message)
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

// GetInitialLLMState returns the current state of LLM providers
func (a *App) GetInitialLLMState() (map[string]interface{}, error) {
	a.logger.Debug().Msg("Getting initial LLM state")
	
	if a.llmRegistry == nil {
		err := fmt.Errorf("LLM registry not initialized")
		a.logger.Error().Err(err).Msg("Failed to get LLM state")
		return map[string]interface{}{
			"globalState": "error",
			"message":     "LLM registry not initialized",
		}, err
	}
	
	// Get the current state snapshot
	stateSnapshot := a.llmRegistry.GetCurrentStateSnapshot()
	
	// Convert to map for JSON serialization
	response := map[string]interface{}{
		"globalState":     stateSnapshot.GlobalState.String(),
		"timestamp":       stateSnapshot.Timestamp,
		"message":         stateSnapshot.Message,
		"providerStates":  make(map[string]interface{}),
	}
	
	// Convert provider states to serializable format
	providerStates := make(map[string]interface{})
	for name, state := range stateSnapshot.ProviderStatesSnapshot {
		providerState := map[string]interface{}{
			"status":       state.Status,
			"lastUpdated":  state.LastUpdated,
			"modelCount":   len(state.Models),
		}
		
		if state.Error != "" {
			providerState["error"] = state.Error
		}
		
		providerStates[name] = providerState
	}
	
	response["providerStates"] = providerStates
	
	return response, nil
}

// GetAvailableSummaryProviders returns a list of available LLM providers for summarization
func (a *App) GetAvailableSummaryProviders() (map[string]interface{}, error) {
	a.logger.Debug().Msg("Fetching available summary providers")
	
	// First check LLM registry state
	if a.llmRegistry == nil {
		a.logger.Warn().Msg("LLM registry not initialized")
		return map[string]interface{}{
			"providers": []map[string]string{},
			"names":     []string{},
			"available": false,
			"suggested": "",
			"status":    "registry_not_initialized",
			"message":   "LLM registry not initialized yet",
		}, nil
	}
	
	stateSnapshot := a.llmRegistry.GetCurrentStateSnapshot()
	
	// If registry is not ready, return appropriate status
	if stateSnapshot.GlobalState != llms.GSReady {
		a.logger.Info().
			Str("global_state", stateSnapshot.GlobalState.String()).
			Msg("LLM registry not ready yet")
			
		return map[string]interface{}{
			"providers": []map[string]string{},
			"names":     []string{},
			"available": false,
			"suggested": "",
			"status":    stateSnapshot.GlobalState.String(),
			"message":   "LLM providers are still initializing",
		}, nil
	}
	
	// Get the summary service
	summaryService := summary.GetDefaultService()
	if summaryService == nil {
		err := fmt.Errorf("summary service not initialized")
		a.logger.Error().Err(err).Msg("Failed to get summary providers")
		return map[string]interface{}{
			"providers": []map[string]string{},
			"names":     []string{},
			"available": false,
			"suggested": "",
			"status":    "summary_service_not_initialized",
			"message":   "Summary service not initialized yet",
		}, nil
	}
	
	// Get the list of providers
	providers := summaryService.ListProviders()
	
	// Create the response structure
	response := map[string]interface{}{
		"providers": []map[string]string{},
		"names":     []string{},
		"available": len(providers) > 0,
		"suggested": "",
		"status":    "ready",
	}
	
	// Add provider details
	providersList := make([]map[string]string, 0, len(providers))
	namesList := make([]string, 0, len(providers))
	
	for _, provider := range providers {
		providerName := provider.GetName()
		namesList = append(namesList, providerName)
		
		providerInfo := map[string]string{
			"name":        providerName,
			"displayName": displayNameForProvider(providerName),
			"description": descriptionForProvider(providerName),
		}
		
		// Add status information from provider states if available
		if providerState, exists := stateSnapshot.ProviderStatesSnapshot[providerName]; exists {
			providerInfo["status"] = providerState.Status
			if providerState.Status == "error" && providerState.Error != "" {
				providerInfo["error"] = providerState.Error
			}
		}
		
		providersList = append(providersList, providerInfo)
	}
	
	response["providers"] = providersList
	response["names"] = namesList
	
	// Set suggested provider - prioritize openrouter-free only
	// First check for openrouter-free
	for _, name := range namesList {
		if name == "openrouter-free" {
			response["suggested"] = "openrouter-free"
			break
		}
	}
	
	// If no openrouter-free and there's at least one available, use the first one
	if response["suggested"] == "" && len(namesList) > 0 {
		response["suggested"] = namesList[0]
	}
	
	return response, nil
}

// GetAvailableSummaryModels returns a list of available models for a specified provider
func (a *App) GetAvailableSummaryModels(providerName string) (map[string]interface{}, error) {
	a.logger.Debug().Str("provider", providerName).Msg("Fetching available summary models")
	
	// First check LLM registry state
	if a.llmRegistry != nil {
		stateSnapshot := a.llmRegistry.GetCurrentStateSnapshot()
		
		// If registry is not ready, return appropriate status
		if stateSnapshot.GlobalState != llms.GSReady {
			return map[string]interface{}{
				"models":    []map[string]interface{}{},
				"names":     []string{},
				"available": false,
				"suggested": "",
				"status":    stateSnapshot.GlobalState.String(),
				"message":   "LLM providers are still initializing",
			}, nil
		}
		
		// If this specific provider is in error state, return that info
		if providerState, exists := stateSnapshot.ProviderStatesSnapshot[providerName]; exists {
			if providerState.Status == "error" {
				errMsg := "Provider initialization failed"
				if providerState.Error != "" {
					errMsg = providerState.Error
				}
				
				return map[string]interface{}{
					"models":    []map[string]interface{}{},
					"names":     []string{},
					"available": false,
					"suggested": "",
					"status":    "error",
					"message":   errMsg,
				}, nil
			}
		}
	}
	
	// Get the summary service
	summaryService := summary.GetDefaultService()
	if summaryService == nil {
		err := fmt.Errorf("summary service not initialized")
		a.logger.Error().Err(err).Msg("Failed to get summary models")
		return nil, err
	}
	
	// Get models for the specified provider
	models, err := summaryService.GetModelsForProvider(providerName)
	if err != nil {
		a.logger.Error().Err(err).Str("provider", providerName).Msg("Failed to get models for provider")
		return nil, err
	}
	
	// Create the response structure
	response := map[string]interface{}{
		"models":    []map[string]interface{}{},
		"names":     []string{},
		"available": len(models) > 0,
		"suggested": "",
		"status":    "ready",
	}
	
	// Add model details
	modelsList := make([]map[string]interface{}, 0, len(models))
	namesList := make([]string, 0, len(models))
	
	for _, model := range models {
		namesList = append(namesList, model.ID)
		
		modelInfo := map[string]interface{}{
			"id":          model.ID,
			"name":        model.Name,
			"description": model.Description,
			"providerName": model.ProviderName,
		}
		modelsList = append(modelsList, modelInfo)
		
		// Look for GPT-4o or Claude models to set as suggested
		if response["suggested"] == "" {
			if strings.Contains(strings.ToLower(model.ID), "gpt-4o") ||
				strings.Contains(strings.ToLower(model.ID), "claude-3") {
				response["suggested"] = model.ID
			}
		}
	}
	
	response["models"] = modelsList
	response["names"] = namesList
	
	// If no suggested model yet and there's at least one available, use the first one
	if response["suggested"] == "" && len(namesList) > 0 {
		response["suggested"] = namesList[0]
	}
	
	return response, nil
}

// Helper function to provide friendly display names for providers
func displayNameForProvider(providerName string) string {
	switch providerName {
	case "openai":
		return "OpenAI"
	case "openrouter":
		return "OpenRouter"
	case "google":
		return "Google AI"
	default:
		// Capitalize first letter and return
		if len(providerName) > 0 {
			return strings.ToUpper(providerName[:1]) + providerName[1:]
		}
		return providerName
	}
}

// Helper function to provide descriptions for providers
func descriptionForProvider(providerName string) string {
	switch providerName {
	case "openai":
		return "OpenAI's GPT models including GPT-4o"
	case "openrouter":
		return "Access to multiple LLM providers through OpenRouter"
	case "google":
		return "Google's Gemini models"
	default:
		return "LLM provider for summarization"
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
	
	// Properly shut down the LLM registry
	if a.llmRegistry != nil {
		a.logger.Info().Msg("Application closing, shutting down LLM registry")
		core.ShutdownLLM(handler)
		a.llmRegistry = nil
	}
	
	// Properly shut down the WebSocket server
	if a.wsServer != nil {
		a.logger.Info().Msg("Application closing, shutting down WebSocket server")
		if err := a.wsServer.Shutdown(); err != nil {
			a.logger.Error().Err(err).Msg("Failed to shutdown WebSocket server")
		}
		a.wsServer = nil
	}
	
	// Properly shut down the throttler
	if a.throttler != nil {
		a.logger.Info().Msg("Application closing, shutting down throttler")
		a.throttler.Shutdown()
		a.throttler = nil
	}
	
	return false
}

// GenerateSummary generates a summary using the specified options
func (a *App) GenerateSummary(text string, inputLanguage string, options map[string]interface{}) (string, error) {
	a.logger.Debug().
		Str("input_language", inputLanguage).
		Int("text_length", len(text)).
		Msg("Generating summary")

	// First check if LLM registry is ready
	if a.llmRegistry == nil {
		return "", fmt.Errorf("LLM registry not initialized")
	}

	stateSnapshot := a.llmRegistry.GetCurrentStateSnapshot()
	if stateSnapshot.GlobalState != llms.GSReady {
		return "", fmt.Errorf("LLM providers not ready (current state: %s)", stateSnapshot.GlobalState.String())
	}

	// Convert map options to typed struct
	summaryOpts := summary.DefaultOptions()
	
	if provider, ok := options["provider"].(string); ok && provider != "" {
		summaryOpts.Provider = provider
	} else {
		return "", fmt.Errorf("provider is required")
	}
	
	if model, ok := options["model"].(string); ok && model != "" {
		summaryOpts.Model = model
	} else {
		return "", fmt.Errorf("model is required")
	}
	
	if outputLang, ok := options["outputLanguage"].(string); ok {
		summaryOpts.OutputLanguage = outputLang
	}
	
	if maxLength, ok := options["maxLength"].(float64); ok && maxLength > 0 {
		summaryOpts.MaxLength = int(maxLength)
	}
	
	if temperature, ok := options["temperature"].(float64); ok && temperature >= 0 {
		summaryOpts.Temperature = temperature
	}
	
	if customPrompt, ok := options["customPrompt"].(string); ok {
		summaryOpts.CustomPrompt = customPrompt
	}

	// Generate the summary
	summaryService := summary.GetDefaultService()
	if summaryService == nil {
		return "", fmt.Errorf("summary service not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := summaryService.GenerateSummary(ctx, text, inputLanguage, summaryOpts)
	if err != nil {
		a.logger.Error().Err(err).
			Str("provider", summaryOpts.Provider).
			Str("model", summaryOpts.Model).
			Msg("Summary generation failed")
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	a.logger.Info().
		Str("provider", summaryOpts.Provider).
		Str("model", summaryOpts.Model).
		Int("result_length", len(result)).
		Msg("Summary generated successfully")

	return result, nil
}

// CheckDockerAvailability checks if Docker is available on the system
func (a *App) CheckDockerAvailability() (map[string]interface{}, error) {
	a.logger.Debug().Msg("Checking Docker availability")
	
	// Try to run docker version command
	cmd := exec.Command("docker", "version", "--format", "json")
	output, err := cmd.Output()
	
	result := map[string]interface{}{
		"available": false,
		"version":   "",
		"engine":    "",
		"error":     "",
	}
	
	if err != nil {
		// Check if it's a command not found error
		if strings.Contains(err.Error(), "executable file not found") {
			result["error"] = "Docker is not installed"
		} else {
			result["error"] = "Cannot connect to Docker daemon"
		}
		a.logger.Debug().Err(err).Msg("Docker check failed")
		return result, nil
	}
	
	// Parse docker version output
	var versionInfo map[string]interface{}
	if err := json.Unmarshal(output, &versionInfo); err == nil {
		result["available"] = true
		if client, ok := versionInfo["Client"].(map[string]interface{}); ok {
			if version, ok := client["Version"].(string); ok {
				result["version"] = version
			}
		}
		
		// Get the actual Docker backend name using dockerutil
		engine := dockerutil.DockerBackendName()
		result["engine"] = engine
		a.logger.Debug().Str("engine", engine).Msg("Docker engine detected")
	}
	
	a.logger.Debug().Interface("result", result).Msg("Docker check completed")
	return result, nil
}

// CheckInternetConnectivity checks if the system has internet connectivity
func (a *App) CheckInternetConnectivity() (map[string]interface{}, error) {
	a.logger.Debug().Msg("Checking internet connectivity")
	
	result := map[string]interface{}{
		"online":   false,
		"latency":  0,
		"error":    "",
	}
	
	// Try to connect to multiple reliable hosts
	hosts := []string{
		"1.1.1.1:443",        // Cloudflare DNS
		"8.8.8.8:443",        // Google DNS
		"208.67.222.222:443", // OpenDNS
	}
	
	for _, host := range hosts {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", host, 3*time.Second)
		if err == nil {
			conn.Close()
			result["online"] = true
			result["latency"] = int(time.Since(start).Milliseconds())
			break
		}
	}
	
	if !result["online"].(bool) {
		result["error"] = "No internet connection detected"
		a.logger.Debug().Msg("Internet connectivity check failed")
	} else {
		a.logger.Debug().
			Bool("online", true).
			Int("latency", result["latency"].(int)).
			Msg("Internet connectivity check passed")
	}
	
	return result, nil
}

// LanguageRequiresDocker checks if a specific language requires Docker for linguistic processing
func (a *App) LanguageRequiresDocker(languageTag string) bool {
	// Languages that require Docker for linguistic processing
	dockerRequiredLanguages := map[string]bool{
		"jpn": true, // Japanese
		"hin": true, // Hindi
		"mar": true, // Marathi
		"ben": true, // Bengali
		"tam": true, // Tamil
		"tel": true, // Telugu
		"kan": true, // Kannada
		"mal": true, // Malayalam
		"guj": true, // Gujarati
		"pan": true, // Punjabi
		"ori": true, // Odia
		"urd": true, // Urdu
	}
	
	// Check if the language tag starts with any of the codes
	for code := range dockerRequiredLanguages {
		if strings.HasPrefix(languageTag, code) {
			return true
		}
	}
	
	return false
}

// LanguageRequiresInternet checks if a specific language requires Internet for linguistic processing
func (a *App) LanguageRequiresInternet(languageTag string) bool {
	// Languages that require Internet for linguistic processing
	internetRequiredLanguages := map[string]bool{
		"tha": true, // Thai
		"jpn": true, // Japanese
		"hin": true, // Hindi
		"mar": true, // Marathi
		"ben": true, // Bengali
		"tam": true, // Tamil
		"tel": true, // Telugu
		"kan": true, // Kannada
		"mal": true, // Malayalam
		"guj": true, // Gujarati
		"pan": true, // Punjabi
		"ori": true, // Odia
		"urd": true, // Urdu
	}
	
	// Check if the language tag starts with any of the codes
	for code := range internetRequiredLanguages {
		if strings.HasPrefix(languageTag, code) {
			return true
		}
	}
	
	return false
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	a.logger.Info().Msg("Application shutdown")
}