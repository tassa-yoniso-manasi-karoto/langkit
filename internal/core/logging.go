package core

import (
	"github.com/rs/zerolog"
)

type LogLevel int

const (
	LevelTrace LogLevel = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
	LevelPanic
)

func (l LogLevel) String() string {
	return [...]string{
		"TRACE",
		"DEBUG",
		"INFO",
		"WARN",
		"ERROR",
		"FATAL",
		"PANIC",
	}[l]
}

func (l LogLevel) ZerologLevel() zerolog.Level {
	return zerolog.Level(l)
}

type LogMessage struct {
	Level    string                 `json:"level"`
	Message  string                 `json:"message"`
	Time     string                 `json:"time"`
	Fields   map[string]interface{} `json:"fields,omitempty"`
	Behavior string                 `json:"behavior,omitempty"`
}

type ErrorBehavior int

const (
	ContinueProcessing ErrorBehavior = iota
	AbortTask
	AbortAllTasks
	ContinueWithWarning
)

func (b ErrorBehavior) String() string {
	return [...]string{
		"continue",
		"abort_task",
		"abort_all",
		"warning",
	}[b]
}

type ProcessingError struct {
	Behavior ErrorBehavior
	Level    LogLevel
	Message  string
	Err      error
	Context  map[string]interface{}
}

func (e *ProcessingError) Error() string {
	return e.Message
}
