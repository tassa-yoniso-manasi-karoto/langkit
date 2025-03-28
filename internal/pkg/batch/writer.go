package batch

import (
	"context"
	"io"
)

// ThrottledLogWriter is an io.Writer that writes logs both to a console output
// for crash reports and to the throttler for frontend updates
type ThrottledLogWriter struct {
	ctx         context.Context
	throttler   *AdaptiveEventThrottler
	consoleOut  io.Writer
}

// NewThrottledLogWriter creates a new throttled log writer
func NewThrottledLogWriter(ctx context.Context, throttler *AdaptiveEventThrottler, consoleOut io.Writer) *ThrottledLogWriter {
	return &ThrottledLogWriter{
		ctx:        ctx,
		throttler:  throttler,
		consoleOut: consoleOut,
	}
}

// Write implements the io.Writer interface
// It writes logs to both the console output (for crash reports)
// and sends them to the throttler (for frontend updates)
func (w *ThrottledLogWriter) Write(p []byte) (n int, err error) {
	// Always write to buffer immediately for crash reports
	n, err = w.consoleOut.Write(p)
	if err != nil {
		return n, err
	}
	
	// Send to throttler for frontend updates if available
	if w.throttler != nil {
		w.throttler.AddLog(string(p))
	}
	
	return len(p), nil
}