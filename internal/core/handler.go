package core

import (
	"fmt"
	"os"
	"io"
	"time"
	"context"
	"bytes"
	"strings"
	
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)


type MessageHandler interface {
	IsCLI() bool

	Log(level int8, behavior string, msg string) *ProcessingError
	LogErr(err error, behavior string, msg string) *ProcessingError
	LogFields(level int8, behavior string, msg string, fields map[string]interface{}) *ProcessingError
	LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError
	
	ZeroLog() *zerolog.Logger
	GetLogBuffer() bytes.Buffer
	HandleProgress(current, total int, description string) //TODO
	HandleStatus(status string) //TODO
	SaveSnapshot(state string, task *Task)
}

type ProcessingSnapshot struct {
	Timestamp time.Time
	State     string
	TaskDump  string
}

// #############################################################################
// #############################################################################
// #############################################################################


// CLI implementation
type CLIHandler struct {
	logger *zerolog.Logger
	buffer bytes.Buffer
	snapshots []ProcessingSnapshot
}

func NewCLIHandler() *CLIHandler {
	h := &CLIHandler{}
	multiOut := io.MultiWriter(os.Stderr, &h.buffer)
	
	writer := zerolog.New(zerolog.ConsoleWriter{
		Out: multiOut,
		TimeFormat: time.TimeOnly,
	})
	logger := zerolog.New(writer).With().Timestamp().Logger()
	h.logger = &logger
	return h
}


func (h *CLIHandler) IsCLI() bool {
	return true
}

func (h *CLIHandler) GetLogBuffer() bytes.Buffer {
	return h.buffer
}

func (h *CLIHandler) Log(level int8, behavior string, msg string) *ProcessingError {
	return log(h, int8(level), nil, behavior, msg, nil)
}

func (h *CLIHandler) LogErr(err error, behavior string, msg string) *ProcessingError {
	return log(h, int8(zerolog.ErrorLevel), err, behavior, msg, nil)
}

func (h *CLIHandler) LogFields(level int8, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
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

func (h *CLIHandler) SaveSnapshot(state string, task *Task) {
	h.snapshots = append(h.snapshots, ProcessingSnapshot{
		Timestamp: time.Now(),
		State:     state,
		TaskDump:  pp.Sprint(task),
	})
}

func (h *CLIHandler) GetSnapshotsString() string {
	return getSnapshotsString(h.snapshots)
}


func getSnapshotsString(snapshots []ProcessingSnapshot) string {
	var b strings.Builder
	for i, snapshot := range snapshots {
		fmt.Fprintf(&b, "Snapshot #%d - %s\n", i+1, snapshot.Timestamp.Format(time.RFC3339))
		fmt.Fprintf(&b, "State: %s\n", snapshot.State)
		fmt.Fprintf(&b, "Task Dump:\n%s\n", snapshot.TaskDump)
		fmt.Fprintf(&b, "-------------------\n")
	}
	return b.String()
}



// #############################################################################
// #############################################################################
// #############################################################################



// GUI implementation
type GUIHandler struct {
	ctx	context.Context
	logger  *zerolog.Logger
	buffer  bytes.Buffer
	snapshots []ProcessingSnapshot
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
	h := &GUIHandler{ ctx: ctx }
	
	multiOut := io.MultiWriter(os.Stderr, &h.buffer)
	
	multiWriter := zerolog.MultiLevelWriter(
		// Raw JSON to send to the frontend directly
		&LogWriter{ctx: ctx},
		// Formatted output for console output & crash reports
		zerolog.ConsoleWriter{
			Out:        multiOut,
			TimeFormat: time.TimeOnly,
		},
	)
	
	logger := zerolog.New(multiWriter).With().Timestamp().Logger()
	h.logger = &logger
	return h
}


func (h *GUIHandler) IsCLI() bool {
	return false
}

func (h *GUIHandler) GetLogBuffer() bytes.Buffer {
	return h.buffer
}

func (h *GUIHandler) Log(level int8, behavior string, msg string) *ProcessingError {
	return log(h, int8(level), nil, behavior, msg, nil)
}
func (h *GUIHandler) LogErr(err error, behavior string, msg string) *ProcessingError {
	return log(h, Error, err, behavior, msg, nil)
}

func (h *GUIHandler) LogFields(level int8, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	return log(h, int8(level), nil, behavior, msg, fields)
}

func (h *GUIHandler) LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	return log(h, Error, err, behavior, msg, fields)
}

func log(h MessageHandler, level int8, err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	event := h.ZeroLog().WithLevel(zerolog.Level(level))
	if err != nil {
		//event.Err(err)
		msg = fmt.Sprintf("%s: %v", msg, err)
	}
	if fields != nil {
		event = event.Fields(fields)
	} // TODO check if need to make fields when they are nil
	event.Str("behavior", behavior).Msg(msg)

	if level >= int8(Error) {
		return &ProcessingError{
			Behavior: behavior,
			Err:      err,
			//Message:  msg,
			//Context:  fields,
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

func (h *GUIHandler) SaveSnapshot(state string, task *Task) {
	h.snapshots = append(h.snapshots, ProcessingSnapshot{
		Timestamp: time.Now(),
		State:     state,
		TaskDump:  pp.Sprint(task),
	})
}

func (h *GUIHandler) GetSnapshotsString() string {
	return getSnapshotsString(h.snapshots)
}

func placeholder3456() {
	fmt.Println("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
