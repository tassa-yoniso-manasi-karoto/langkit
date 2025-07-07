package gui

import (
	"time"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
)

// updateThrottlerSettings when config changes
func (a *App) updateThrottlerSettings(settings config.Settings) {
	if a.throttler == nil {
		a.getLogger().Warn().Msg("Cannot update throttler settings: throttler is nil")
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

	a.getLogger().Debug().
		Bool("enabled", settings.EventThrottling.Enabled).
		Dur("minInterval", minInterval).
		Dur("maxInterval", maxInterval).
		Msg("Throttler settings updated")
}

// SetEventThrottling enables or disables the event throttling
func (a *App) SetEventThrottling(enabled bool) {
	if a.throttler != nil {
		a.throttler.SetEnabled(enabled)
		a.getLogger().Debug().Bool("enabled", enabled).Msg("Event throttling toggled")
	} else {
		a.getLogger().Warn().Msg("Cannot set throttling state: throttler is nil")
	}
}

// GetEventThrottlingStatus returns the current throttling status
func (a *App) GetEventThrottlingStatus() map[string]interface{} {
	if a.throttler == nil {
		a.getLogger().Warn().Msg("Cannot get throttling status: throttler is nil")
		return map[string]interface{}{
			"enabled":         false,
			"currentRate":     0.0,
			"currentInterval": 0,
			"error":           "Throttler not initialized",
		}
	}

	return a.throttler.GetStatus()
}