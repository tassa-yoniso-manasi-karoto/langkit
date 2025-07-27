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
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/ui"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/ui/browser"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/ui/dialogs"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

var handler *core.GUIHandler
var appThrottler *batch.AdaptiveEventThrottler
var errHandlerNotInitialized = fmt.Errorf("handler not initialized")

type App struct {
	ctx         context.Context
	procCancel  context.CancelFunc
	throttler   *batch.AdaptiveEventThrottler
	logger      *zerolog.Logger   // Only for early initialization before handler is ready
	llmRegistry *llms.Registry    // LLM Registry for async provider management
	wsServer    WebSocketEmitter  // WebSocket server for state updates
	apiServer   *api.Server       // WebRPC API server
	
	// Pre-initialized servers (when using shared initialization)
	preInitialized bool
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

// NewAppWithServers creates an App with pre-initialized servers
func NewAppWithServers(servers *ServerComponents) *App {
	app := NewApp()
	app.wsServer = servers.WSServer
	app.apiServer = servers.APIServer
	app.throttler = servers.Throttler
	app.preInitialized = true
	
	// Set the global handler
	handler = servers.Handler
	
	return app
}


func (a *App) bindEnvironmentVariables() {
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

	// Initialize UI manager with Wails implementation
	ui.Initialize(
		dialogs.NewWailsFileDialog(ctx),
		browser.NewWailsURLOpener(ctx),
	)

	a.getLogger().Info().Msg("Application starting up")

	// If servers are not pre-initialized, create them now
	if !a.preInitialized {
		// Initialize servers using shared logic
		servers, err := InitializeServers(ctx, *a.getLogger())
		if err != nil {
			a.getLogger().Fatal().Err(err).Msg("Failed to initialize servers")
		}
		
		// Store the initialized components
		a.wsServer = servers.WSServer
		a.apiServer = servers.APIServer
		a.throttler = servers.Throttler
		handler = servers.Handler
		appThrottler = servers.Throttler
		
		// Register settings service (needs App reference, so done here)
		settingsProvider := &settingsProviderAdapter{app: a}
		settingsSvc := services.NewSettingsService(*a.getLogger(), settingsProvider)
		if err := a.apiServer.RegisterService(settingsSvc); err != nil {
			a.getLogger().Fatal().Err(err).Msg("Failed to register settings service")
		}
	} else {
		// Servers are pre-initialized, just set the global handler
		// (handler should have been created during InitializeServers)
		if handler == nil {
			a.getLogger().Fatal().Msg("Handler not initialized with pre-initialized servers")
		}
		appThrottler = a.throttler
		
		// Register settings service (needs App reference, so done here)
		settingsProvider := &settingsProviderAdapter{app: a}
		settingsSvc := services.NewSettingsService(*a.getLogger(), settingsProvider)
		if err := a.apiServer.RegisterService(settingsSvc); err != nil {
			a.getLogger().Fatal().Err(err).Msg("Failed to register settings service")
		}
	}
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
	if a.wsServer != nil {
		a.wsServer.Emit("settings.loaded", settings)
	}

	if settings.ShowLogViewerByDefault {
		runtime.WindowMaximise(ctx)
	}

	// Initialize LLM system with async registry and WebSocket server
	a.llmRegistry = core.InitLLM(handler, a.ctx, a.wsServer)
	a.getLogger().Info().Msg("LLM registry initialized")
	
	// Set the LLM registry in the handler so it can be accessed by services
	handler.SetLLMRegistry(a.llmRegistry)
	
	// Set up WebSocket connection callback to send initial LLM state
	a.wsServer.SetOnConnect(func() {
		if a.llmRegistry != nil {
			stateSnapshot := a.llmRegistry.GetCurrentStateSnapshot()
			a.wsServer.Emit("llm.state.changed", stateSnapshot)
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
		a.wsServer.Shutdown()
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

// settingsProviderAdapter implements interfaces.SettingsProvider
type settingsProviderAdapter struct {
	app *App
}

// UpdateThrottlerSettings implements interfaces.SettingsProvider
func (s *settingsProviderAdapter) UpdateThrottlerSettings(settings interface{}) {
	// Type assert to config.Settings
	if configSettings, ok := settings.(config.Settings); ok {
		s.app.updateThrottlerSettings(configSettings)
	} else {
		s.app.getLogger().Error().Msg("UpdateThrottlerSettings: invalid settings type")
	}
}

// TriggerLLMRegistryUpdate implements interfaces.SettingsProvider  
func (s *settingsProviderAdapter) TriggerLLMRegistryUpdate(settings interface{}) {
	// Type assert to config.Settings
	if configSettings, ok := settings.(config.Settings); ok {
		if s.app.llmRegistry != nil {
			s.app.getLogger().Info().Msg("Triggering LLM registry update with new settings")
			s.app.llmRegistry.TriggerUpdate(configSettings)
		}
	} else {
		s.app.getLogger().Error().Msg("TriggerLLMRegistryUpdate: invalid settings type")
	}
}
