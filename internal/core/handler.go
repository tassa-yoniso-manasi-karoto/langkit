package core

import (
	"os"
	"time"
	"context"
	
	"fmt"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
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
	Log(level LogLevel, behavior string, msg string) *ProcessingError
	LogErr(err error, behavior string, msg string) *ProcessingError
	LogFields(level LogLevel, behavior string, msg string, fields map[string]interface{}) *ProcessingError
	LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError
	
	ZeroLog() *zerolog.Logger
	HandleProgress(current, total int, description string)
	HandleStatus(status string)
}


// #############################################################################
// #############################################################################
// #############################################################################


// CLI implementation
type CLIHandler struct {
	logger *zerolog.Logger
}

func NewCLIHandler() *CLIHandler {
	return &CLIHandler{ NewLogger() }
}




func (h *CLIHandler) Log(level LogLevel, behavior string, msg string) *ProcessingError {
	return log(h, int8(level), nil, behavior, msg, nil)
}

func (h *CLIHandler) LogErr(err error, behavior string, msg string) *ProcessingError {
	return log(h, int8(zerolog.ErrorLevel), err, behavior, msg, nil)
}

func (h *CLIHandler) LogFields(level LogLevel, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	return log(h, int8(level), nil, behavior, msg, fields)
}

func (h *CLIHandler) LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	return log(h, int8(zerolog.ErrorLevel), err, behavior, msg, fields)
}




func (h *CLIHandler) ZeroLog() *zerolog.Logger {
	return h.logger
}

func (h *CLIHandler) HandleProgress(current, total int, description string) {
	// TODO Implementation for CLI progress bar
}

func (h *CLIHandler) HandleStatus(status string) {
	h.logger.Info().Msg(status)
}



// #############################################################################
// #############################################################################
// #############################################################################



// GUI implementation
type GUIHandler struct {
	ctx	context.Context
	logger  *zerolog.Logger
}

// LogWriter must implement io.Writer for zerolog.MultiLevelWriter
type LogWriter struct {
	ctx context.Context
}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	// Emit the raw JSON directly to frontend
	runtime.EventsEmit(w.ctx, "log", string(p))
	return len(p), nil
}

func NewGUIHandler(ctx context.Context) *GUIHandler {
	// Create MultiLevelWriter with both GUI and console output
	multiWriter := zerolog.MultiLevelWriter(
		&LogWriter{ctx: ctx},
		zerolog.ConsoleWriter{
			Out:        os.Stderr,
			NoColor:    true,
			TimeFormat: time.TimeOnly,
		},
	)
	
	logger := zerolog.New(multiWriter).With().Timestamp().Logger()
	
	return &GUIHandler{
		ctx:    ctx,
		logger: &logger,
	}
}



func (h *GUIHandler) Log(level LogLevel, behavior string, msg string) *ProcessingError {
	return log(h, int8(level), nil, behavior, msg, nil)
}

func (h *GUIHandler) LogErr(err error, behavior string, msg string) *ProcessingError {
	return log(h, int8(zerolog.ErrorLevel), err, behavior, msg, nil)
}

func (h *GUIHandler) LogFields(level LogLevel, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	return log(h, int8(level), nil, behavior, msg, fields)
}

func (h *GUIHandler) LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	return log(h, int8(zerolog.ErrorLevel), err, behavior, msg, fields)
}

func log(h MessageHandler, level int8, err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	event := h.ZeroLog().WithLevel(zerolog.Level(level))
	if err != nil {
		event.Err(err)
		msg = fmt.Sprint("%s: %v", msg, err)
	}
	if fields != nil {
		event = event.Fields(fields)
	} // TODO check if need to make fields when they are nil
	event.Str("behavior", behavior).Msg(msg)

	if level >= int8(Error) {
		return &ProcessingError{
			Behavior: behavior,
			Level:    level,
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
		"current":     current,
		"total":       total,
		"description": description,
	})
}

func (h *GUIHandler) HandleStatus(status string) {
	runtime.EventsEmit(h.ctx, "status", status)
}


func placeholder3456() {
	fmt.Println("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
