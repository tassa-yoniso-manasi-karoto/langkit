package gui

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/batch"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

var handler *core.GUIHandler
var appThrottler *batch.AdaptiveEventThrottler

type App struct {
	ctx         context.Context
	procCancel  context.CancelFunc
	throttler   *batch.AdaptiveEventThrottler
	logger      *zerolog.Logger  // Only for early initialization before handler is ready
	llmRegistry *llms.Registry   // LLM Registry for async provider management
	wsServer    *WebSocketServer // WebSocket server for state updates
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

	// Initialize the throttler with default settings
	// These will be updated when settings are loaded
	a.throttler = batch.NewAdaptiveEventThrottler(
		ctx,
		0,                    // minInterval - will be updated from settings
		250*time.Millisecond, // maxInterval - will be updated from settings
		500*time.Millisecond, // rateWindow for measuring event frequency
		true,                 // enabled by default
		a.getLogger(),        // Logger for throttler
	)

	// Store throttler references for global access
	appThrottler = a.throttler

	// Initialize handler with throttler
	handler = core.NewGUIHandler(ctx, a.throttler)

	a.getLogger().Debug().Msg("Event throttler initialized")

	// Create WebSocket server for LLM state updates
	wsServer, err := NewWebSocketServer(*a.getLogger())
	if err != nil {
		a.getLogger().Fatal().Err(err).Msg("Failed to create WebSocket server")
	}
	a.wsServer = wsServer
	a.getLogger().Info().Int("port", wsServer.GetPort()).Msg("WebSocket server created")
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

