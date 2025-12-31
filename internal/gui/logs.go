package gui

import (
	"github.com/rs/zerolog"
)

// getLogger returns the appropriate logger - handler's logger if available, otherwise app's logger
func (a *App) getLogger() *zerolog.Logger {
	if handler != nil {
		return handler.ZeroLog()
	}
	return a.logger
}
