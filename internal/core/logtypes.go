package core

import (
	"github.com/rs/zerolog"
)

type ProcessingError struct {
	Err      error
	Behavior string
	//Level    int8				// probably unneeded
	//Message  string			// probably unneeded
	//Context  map[string]interface{}	// probably unneeded
}
// probably unneeded, right now, using ProcessingError.Err directly because
// it is enough to signal the existence of an error to the processing logic
//func (e *ProcessingError) Error() string {
//	return e.Message
//}

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


