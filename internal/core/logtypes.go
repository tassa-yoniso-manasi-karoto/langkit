package core

import (
	"github.com/rs/zerolog"
)

type ProcessingError struct {
	Behavior string
	Level    int8			// probably unneeded
	Message  string			// probably unneeded
	Err      error
	Context  map[string]interface{}	// probably unneeded
}
func (e *ProcessingError) Error() string {
	return e.Message
}

const (
	ContinueProcessing	= "continue"
	ContinueWithWarning	= "warning"
	ProbeUser		= "probe"
	AbortTask		= "abort_task"
	AbortAllTasks		= "abort_all"
)

type LogLevel int8

const (
	Trace = zerolog.TraceLevel
	Debug = zerolog.DebugLevel
	Info  = zerolog.InfoLevel
	Warn  = zerolog.WarnLevel
	Error = zerolog.ErrorLevel
	Fatal = zerolog.FatalLevel
	Panic = zerolog.PanicLevel
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


