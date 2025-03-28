package gui

import (
	"context"
	"os"
	"time"
	
	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/batch"
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

// beforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue, false will continue shutdown as normal.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	// Properly shut down the throttler
	if a.throttler != nil {
		a.logger.Info().Msg("Shutting down throttler")
		a.throttler.Shutdown()
	}
	return false
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	a.logger.Info().Msg("Application shutdown")
}