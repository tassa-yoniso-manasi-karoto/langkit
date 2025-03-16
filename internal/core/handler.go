package core

import (
	"fmt"
	"os"
	"io"
	"time"
	"context"
	"bytes"
	"encoding/json"
	
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
	"github.com/tassa-yoniso-manasi-karoto/translitkit/common"
)


type MessageHandler interface {
	IsCLI() bool

	// TODO log methods don't actually require an interface:
	// could use a Handler.Std() method to access Log* and thus regroup the Log* funcs under Std()
	Log(level int8, behavior string, msg string) *ProcessingError
	// this is a helper that logs to LevelError by default as most err log correspond to LevelError
	LogErr(err error, behavior string, msg string) *ProcessingError
	// this is a helper that returns an err but doesn't use a LevelError,
	// it is meant to be used to handle ctx.Err following user-requested context cancelation
	LogErrWithLevel(level int8, err error, behavior string, msg string) *ProcessingError
	
	LogFields(level int8, behavior string, msg string, fields map[string]interface{}) *ProcessingError
	LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError
	
	ZeroLog() *zerolog.Logger
	GetLogBuffer() bytes.Buffer
	HandleStatus(status string) //TODO
	
	IncrementProgress(taskID string, increment, total, priority int, operation, descr, size string)
	ResetProgress()
}

// #############################################################################
// #############################################################################
// #############################################################################


// CLI implementation
type CLIHandler struct {
	ctx	context.Context
	logger *zerolog.Logger
	buffer bytes.Buffer
	
	progressBars map[string]*progressbar.ProgressBar
}

func NewCLIHandler(ctx context.Context) *CLIHandler {
	h := &CLIHandler{ 
		ctx: ctx,
		progressBars: make(map[string]*progressbar.ProgressBar),
	}
	crash.InitReporter(ctx)
	
	multiOut := io.MultiWriter(os.Stderr, &h.buffer)
	
	writer := zerolog.ConsoleWriter{
		Out: multiOut,
		TimeFormat: time.TimeOnly,
	}
	logger := zerolog.New(writer).With().Timestamp().Logger()
	h.logger = &logger
	common.Log = logger.With().Timestamp().Str("module", "translitkit").Logger()
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

func (h *CLIHandler) LogErrWithLevel(level int8, err error, behavior string, msg string) *ProcessingError {
	return log(h, int8(level), err, behavior, msg, nil)
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


func (h *CLIHandler) HandleStatus(status string) {
	h.logger.Info().Msg(status)
}


func (h *CLIHandler) ResetProgress() {
}



func (h *CLIHandler) IncrementProgress(taskID string, increment, total, priority int, operation, desc, size string) {
	if h.progressBars == nil {
		h.progressBars = make(map[string]*progressbar.ProgressBar)
	}

	bar, exists := h.progressBars[taskID]
	if !exists {
		// Create a new progress bar for this ID
		bar = progressbar.NewOptions(
			total,
			progressbar.OptionSetDescription(desc),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(31),
			progressbar.OptionClearOnFinish(),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionSetWriter(os.Stdout),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "#",
				SaucerPadding: "-",
				BarStart:      "[",
				BarEnd:        "]",
			}),
		)
		h.progressBars[taskID] = bar
		fmt.Printf("\n%s\n", operation) // Show an extra line with the name of the operation if you like
	}
	
	// If the total changed, adjust the bar's max
	if bar.GetMax() != total {
		bar.ChangeMax(total)
		bar.Describe(desc) // update text if you want
	}
	
	// Increment by the specified amount
	bar.Add(increment)

	// If we're done, clear & remove from map so we don’t keep unused bars
	if bar.State().CurrentPercent >= 1.0 {
		bar.Clear()
		delete(h.progressBars, taskID)
	}
}


// #############################################################################
// #############################################################################
// #############################################################################



// GUI implementation
type GUIHandler struct {
	ctx	context.Context
	logger  *zerolog.Logger
	buffer  bytes.Buffer
	
	progressMap map[string]int
}

// LogWriter must implement io.Writer for zerolog.MultiLevelWriter
type LogWriter struct {
	ctx context.Context
}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	// Parse the log message
	var logMessage map[string]interface{}
	if err := json.Unmarshal(p, &logMessage); err != nil {
		return 0, err
	}

	// Check the log level
	if level, ok := logMessage["level"]; ok {
		// If it's TRACE (-1), don't send to frontend
		if level == -1 {
			// Return the original length to satisfy the Writer interface
			return len(p), nil
		}
	}

	// Send non-TRACE logs to the frontend
	runtime.EventsEmit(w.ctx, "log", string(p))
	return len(p), nil
}


func NewGUIHandler(ctx context.Context) *GUIHandler {
	h := &GUIHandler{
		ctx:         ctx,
		progressMap: make(map[string]int),
	}
	crash.InitReporter(ctx)
	
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
	common.Log = logger.With().Timestamp().Str("module", "translitkit").Logger()
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

func (h *GUIHandler) LogErrWithLevel(level int8, err error, behavior string, msg string) *ProcessingError {
	return log(h, int8(level), err, behavior, msg, nil)
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

	if err != nil {
		return &ProcessingError{
			Behavior: behavior,
			Err:      err,
		}
	}
	return nil
}

func (h *GUIHandler) ZeroLog() *zerolog.Logger {
	return h.logger
}

// ResetProgress clears all progress bars and resets the progress tracking map
func (h *GUIHandler) ResetProgress() {
	// Clear the progress map
	h.progressMap = make(map[string]int)
	
	// Emit event to frontend to reset all progress bars
	runtime.EventsEmit(h.ctx, "progress-reset", true)
}


func (h *GUIHandler) IncrementProgress(
	taskID string, 
	increment, total, priority int, 
	operation, descr, size string,
) {
	if _, found := h.progressMap[taskID]; !found {
		h.ZeroLog().Trace().Msgf("handler: creating bar %s for  \"%s\"", taskID, operation)
	} else {
		h.ZeroLog().Trace().Msgf("handler: reusing bar %s for \"%s\"", taskID, operation)
	}
	h.progressMap[taskID] += increment
	current := h.progressMap[taskID]

	percent := 0.0
	if total > 0 {
		percent = (float64(current) / float64(total)) * 100.0
	} else {
		// fallback if total=0
		percent = float64(current)
	}

	payload := map[string]interface{}{
		"id":          taskID,
		"progress":    percent,
		"current":     current,
		"total":       total,
		"operation":   operation,
		"description": descr,
		"color":       "",
		"size":        size,
		"striped":     true,
		"animated":    true,
		"priority":    priority,
	}
	runtime.EventsEmit(h.ctx, "progress", payload)

	if total > 0 && current >= total {
		delete(h.progressMap, taskID)
	}
}

func (h *GUIHandler) HandleStatus(status string) {
	runtime.EventsEmit(h.ctx, "status", status)
}


func placeholder3456() {
	fmt.Println("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
