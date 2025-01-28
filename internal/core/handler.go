package core

import (
	"os"
	"time"
	"context"
	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func NewLogger() *zerolog.Logger {
	z := zerolog.New(zerolog.ConsoleWriter{
		Out: os.Stdout,
		TimeFormat: time.TimeOnly,
	}).With().Timestamp().Logger()
	return &z
}


type MessageHandler interface {
	Log(level LogLevel, behavior ErrorBehavior, msg string, fields map[string]interface{}) *ProcessingError
	ZeroLog() *zerolog.Logger
	HandleProgress(current, total int, description string)
	HandleStatus(status string)
}

// CLI implementation
type CLIHandler struct {
	logger *zerolog.Logger
}

func NewCLIHandler() *CLIHandler {
	return &CLIHandler{ NewLogger() }
}


func (h *CLIHandler) Log(level LogLevel, behavior ErrorBehavior, msg string, fields map[string]interface{}) *ProcessingError {
	event := h.logger.WithLevel(level.ZerologLevel())
	if fields != nil {
		event = event.Fields(fields)
	}
	event.Str("behavior", behavior.String()).Msg(msg)

	if level >= LevelError {
		return &ProcessingError{
			Behavior: behavior,
			Level:	level,
			Message:  msg,
			Context:  fields,
		}
	}
	return nil
}

func (h *CLIHandler) ZeroLog() *zerolog.Logger {
	return h.logger
}

func (h *CLIHandler) HandleProgress(current, total int, description string) {
	// Implementation for CLI progress bar
}

func (h *CLIHandler) HandleStatus(status string) {
	h.logger.Info().Msg(status)
}

// GUI implementation
type GUIHandler struct {
	ctx	context.Context
	logger *zerolog.Logger
}

func NewGUIHandler(ctx context.Context, logger *zerolog.Logger) *GUIHandler {
	return &GUIHandler{
		ctx:	ctx,
		logger: logger,
	}
}

func (h *GUIHandler) Log(level LogLevel, behavior ErrorBehavior, msg string, fields map[string]interface{}) *ProcessingError {
	// Log to console
	event := h.logger.WithLevel(level.ZerologLevel())
	if fields != nil {
		event = event.Fields(fields)
	}
	event.Str("behavior", behavior.String()).Msg(msg)

	// Emit to frontend
	runtime.EventsEmit(h.ctx, "log", LogMessage{
		Level:	level.String(),
		Message:  msg,
		Time:	 time.Now().Format(time.TimeOnly),
		Fields:   fields,
		Behavior: behavior.String(),
	})

	if level >= LevelError {
		return &ProcessingError{
			Behavior: behavior,
			Level:	level,
			Message:  msg,
			Context:  fields,
		}
	}
	return nil
}

func (h *GUIHandler) ZeroLog() *zerolog.Logger {
	return h.logger
}

func (h *GUIHandler) HandleProgress(current, total int, description string) {
	runtime.EventsEmit(h.ctx, "progress", map[string]interface{}{
		"current":	 current,
		"total":	   total,
		"description": description,
	})
}

func (h *GUIHandler) HandleStatus(status string) {
	runtime.EventsEmit(h.ctx, "status", status)
}