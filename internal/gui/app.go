package gui

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/services"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/batch"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

var handler *core.GUIHandler
var appThrottler *batch.AdaptiveEventThrottler
var errHandlerNotInitialized = fmt.Errorf("handler not initialized")

type App struct {
	ctx         context.Context
	procCancel  context.CancelFunc
	throttler   *batch.AdaptiveEventThrottler
	logger      *zerolog.Logger  // Only for early initialization before handler is ready
	llmRegistry *llms.Registry   // LLM Registry for async provider management
	wsServer    *WebSocketServer // WebSocket server for state updates
	apiServer   *api.Server      // WebRPC API server
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
	a.getLogger().Debug().Msg("Binding environment variables to config")

	// Set environment prefix and automatic env
	viper.SetEnvPrefix("LANGKIT")
	viper.AutomaticEnv()

	// Bind specific environment variables to their config counterparts
	envBindings := map[string]string{
		"REPLICATE_API_KEY": "api_keys.replicate",

		"ELEVENLABS_API_KEY": "api_keys.elevenlabs",
		"OPENAI_API_KEY":     "api_keys.openai",
		"OPENROUTER_API_KEY": "api_keys.openrouter",
		"GOOGLE_API_KEY":     "api_keys.google",
	}

	for env, conf := range envBindings {
		if err := viper.BindEnv(conf, env); err != nil {
			a.getLogger().Error().Str("env", env).Err(err).Msg("Failed to bind environment variable")
		} else {
			a.getLogger().Debug().Str("env", env).Str("config", conf).Msg("Bound environment variable to config")
		}
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	a.getLogger().Info().Msg("Application starting up")

	// Create WebSocket server first (needed by throttler)
	wsServer, err := NewWebSocketServer(*a.getLogger())
	if err != nil {
		a.getLogger().Fatal().Err(err).Msg("Failed to create WebSocket server")
	}
	a.wsServer = wsServer
	a.getLogger().Info().Int("port", wsServer.GetPort()).Msg("WebSocket server created")

	// Create broadcaster function for throttler
	broadcaster := func(msgType string, data interface{}) {
		if a.wsServer != nil {
			a.wsServer.Broadcast(msgType, data)
		}
	}

	// Initialize the throttler with WebSocket broadcaster
	// These settings will be updated when settings are loaded
	a.throttler = batch.NewAdaptiveEventThrottler(
		ctx,
		0,                    // minInterval - will be updated from settings
		250*time.Millisecond, // maxInterval - will be updated from settings
		500*time.Millisecond, // rateWindow for measuring event frequency
		true,                 // enabled by default
		a.getLogger(),        // Logger for throttler
		broadcaster,          // WebSocket broadcaster
	)

	// Store throttler references for global access
	appThrottler = a.throttler

	// Initialize handler with throttler and WebSocket server
	handler = core.NewGUIHandler(ctx, a.throttler, a.wsServer)

	a.getLogger().Debug().Msg("Event throttler initialized")
	
	// Create WebRPC API server
	apiServer, err := api.NewServer(api.DefaultConfig(), *a.getLogger())
	if err != nil {
		a.getLogger().Fatal().Err(err).Msg("Failed to create API server")
	}
	a.apiServer = apiServer
	
	// Register language service
	langSvc := services.NewLanguageService(*a.getLogger())
	if err := apiServer.RegisterService(langSvc); err != nil {
		a.getLogger().Fatal().Err(err).Msg("Failed to register language service")
	}
	
	// Start API server
	if err := apiServer.Start(); err != nil {
		a.getLogger().Fatal().Err(err).Msg("Failed to start API server")
	}
	a.getLogger().Info().Int("port", apiServer.GetPort()).Msg("WebRPC API server started")
}

func (a *App) domReady(ctx context.Context) {
	a.getLogger().Debug().Msg("DOM ready, initializing settings")

	// Bind environment variables to config
	a.bindEnvironmentVariables()

	// Load settings
	settings, err := config.LoadSettings()
	if err != nil {
		a.getLogger().Error().Err(err).Msg("Failed to load settings")
	}

	if err := config.InitConfig(""); err != nil {
		a.getLogger().Error().Err(err).Msg("Failed to initialize config")
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
	a.getLogger().Info().Msg("LLM registry initialized")
	
	// Set up WebSocket connection callback to send initial LLM state
	a.wsServer.SetOnConnect(func() {
		if a.llmRegistry != nil {
			stateSnapshot := a.llmRegistry.GetCurrentStateSnapshot()
			a.wsServer.Broadcast("llm.state.changed", stateSnapshot)
			a.getLogger().Debug().Msg("Sent initial LLM state to new WebSocket client")
		}
	})

	a.getLogger().Info().Msg("Application initialization complete")
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
		a.getLogger().Info().Msg("Application closing, shutting down LLM registry")
		core.ShutdownLLM(handler)
		a.llmRegistry = nil
	}

	// Properly shut down the WebSocket server
	if a.wsServer != nil {
		a.getLogger().Info().Msg("Application closing, shutting down WebSocket server")
		if err := a.wsServer.Shutdown(); err != nil {
			a.getLogger().Error().Err(err).Msg("Failed to shutdown WebSocket server")
		}
		a.wsServer = nil
	}
	
	// Properly shut down the API server
	if a.apiServer != nil {
		a.getLogger().Info().Msg("Application closing, shutting down API server")
		if err := a.apiServer.Shutdown(); err != nil {
			a.getLogger().Error().Err(err).Msg("Failed to shutdown API server")
		}
		a.apiServer = nil
	}

	// Properly shut down the throttler
	if a.throttler != nil {
		a.getLogger().Info().Msg("Application closing, shutting down throttler")
		a.throttler.Shutdown()
		a.throttler = nil
	}

	return false
}




// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	a.getLogger().Info().Msg("Application shutdown")
}

// Dry run testing methods

// SetDryRunConfig stores the dry run configuration for the next processing run
func (a *App) SetDryRunConfig(config map[string]interface{}) error {
	if handler == nil {
		return errHandlerNotInitialized
	}
	
	// Convert map to DryRunConfig struct
	dryRunConfig := &core.DryRunConfig{
		Enabled:        getBoolFromMap(config, "enabled", false),
		DelayMs:        getIntFromMap(config, "delayMs", 1000),
		ProcessedCount: getIntFromMap(config, "processedCount", 0),
		NextErrorIndex: getIntFromMap(config, "nextErrorIndex", -1),
		NextErrorType:  getStringFromMap(config, "nextErrorType", ""),
		ErrorPoints:    make(map[int]string),
	}
	
	// Convert errorPoints from map
	if errorPoints, ok := config["errorPoints"].(map[string]interface{}); ok {
		for indexStr, errorType := range errorPoints {
			if index, err := parseStringToInt(indexStr); err == nil {
				if errorTypeStr, ok := errorType.(string); ok {
					dryRunConfig.ErrorPoints[index] = errorTypeStr
				}
			}
		}
	}
	
	handler.SetDryRunConfig(dryRunConfig)
	return nil
}

// InjectDryRunError schedules an error injection at the next task
func (a *App) InjectDryRunError(errorType string) error {
	if handler == nil {
		return errHandlerNotInitialized
	}
	
	return handler.InjectDryRunError(errorType)
}

// GetDryRunStatus returns the current dry run status
func (a *App) GetDryRunStatus() (map[string]interface{}, error) {
	if handler == nil {
		return nil, errHandlerNotInitialized
	}
	
	return handler.GetDryRunStatus(), nil
}

// Helper functions for map conversion
func getBoolFromMap(m map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := m[key].(bool); ok {
		return val
	}
	return defaultValue
}

func getIntFromMap(m map[string]interface{}, key string, defaultValue int) int {
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	if val, ok := m[key].(int); ok {
		return val
	}
	return defaultValue
}

func getStringFromMap(m map[string]interface{}, key string, defaultValue string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return defaultValue
}

func parseStringToInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}


// GetAPIPort returns the port the WebRPC API server is listening on
func (a *App) GetAPIPort() (int, error) {
	if a.apiServer == nil {
		return 0, fmt.Errorf("API server not initialized")
	}
	return a.apiServer.GetPort(), nil
}
