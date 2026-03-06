package core

import (
	"github.com/rs/zerolog"
)

type ProcessingError struct {
	Err      error
	Behavior string
	//Level    int8			// probably unneeded
	//Message  string		// probably unneeded
	Context map[string]interface{} // probably unneeded
}

func (e *ProcessingError) Error() string {
	if e.Err == nil {
		return "ProcessingError: " + e.Behavior
	}
	return e.Err.Error()
}

const (
	//ContinueProcessing	= "continue"
	//ContinueWithWarning	= "warning"
	ProbeUser     = "probe"
	AbortTask     = "abort_task"
	AbortAllTasks = "abort_all"
)

const (
	ffmpegApostropheDirectoryPathError = "Due to ffmpeg limitations, the path of the directory in which the files are located must not contain an apostrophe ('). Apostrophe in the names of the files themselves are supported using a workaround."
)

const (
	Trace = int8(zerolog.TraceLevel)
	Debug = int8(zerolog.DebugLevel)
	Info  = int8(zerolog.InfoLevel)
	Warn  = int8(zerolog.WarnLevel)
	Error = int8(zerolog.ErrorLevel)
	Fatal = int8(zerolog.FatalLevel)
	Panic = int8(zerolog.PanicLevel)
)
