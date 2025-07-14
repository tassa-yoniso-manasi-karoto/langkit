package batch

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Default high load mode timeout
var defaultHighLoadTimeout = 5 * time.Second

// Command interface for all operations
type command interface {
	execute(t *AdaptiveEventThrottler)
}

// Add log command
type addLogCommand struct {
	log string
	direct bool // Whether to send directly (bypass batch)
}

func (c *addLogCommand) execute(t *AdaptiveEventThrottler) {
	// Check if this is a critical log by parsing it
	var logData map[string]interface{}
	isCritical := false
	
	if err := json.Unmarshal([]byte(c.log), &logData); err == nil {
		// Add metadata to the log
		t.logSequence++
		logData["_sequence"] = t.logSequence
		
		// Add timestamp for sorting
		if timeStr, ok := logData["time"].(string); ok {
			if timeVal, err := time.Parse(time.RFC3339, timeStr); err == nil {
				logData["_unix_time"] = timeVal.UnixNano() / int64(time.Millisecond)
			} else {
				logData["_unix_time"] = time.Now().UnixNano() / int64(time.Millisecond)
			}
		} else {
			logData["_unix_time"] = time.Now().UnixNano() / int64(time.Millisecond)
		}
		
		// Check if this is a critical log
		isCritical = t.isCriticalLog(logData)
		
		// Re-serialize with metadata
		if modifiedLog, err := json.Marshal(logData); err == nil {
			c.log = string(modifiedLog)
		}
	}
	
	// Send critical logs and direct logs immediately
	if isCritical || c.direct || !t.enabled {
		if t.broadcaster != nil {
			t.broadcaster("log.entry", c.log)
		}
		return
	}
	
	// Update event rate tracking
	t.updateEventRateInternal()
	
	// Use direct pass-through for normal operations (when not in high load mode)
	if t.currentRate < t.directPassThreshold && !t.highLoadMode {
		if t.broadcaster != nil {
			t.broadcaster("log.entry", c.log)
		}
		return
	}
	
	// Add to buffer with overflow protection
	if len(t.logBuffer) >= t.maxBufferSize {
		// Force a flush if buffer is getting full
		if float64(len(t.logBuffer)) > float64(t.maxBufferSize)*0.8 {
			t.doFlush(false)
		}
		
		// Keep only newer logs
		t.logBuffer = append(t.logBuffer[len(t.logBuffer)/5:], c.log)
	} else {
		t.logBuffer = append(t.logBuffer, c.log)
	}
	
	// Adjust throttling
	t.adjustThrottlingInternal()
}

// Update progress command
type updateProgressCommand struct {
	id   string
	data map[string]interface{}
	direct bool // Whether to send directly
}

func (c *updateProgressCommand) execute(t *AdaptiveEventThrottler) {
	// Direct send if throttling disabled
	if !t.enabled {
		if t.broadcaster != nil {
			t.broadcaster("progress.updated", c.data)
		}
		return
	}
	
	// Update rate tracking
	t.updateEventRateInternal()
	
	// Use direct pass-through for normal operations
	if (t.currentRate < t.directPassThreshold && !t.highLoadMode) || c.direct {
		if t.broadcaster != nil {
			t.broadcaster("progress.updated", c.data)
		}
		return
	}
	
	// Store only the latest update for this ID
	t.progressBuffer[c.id] = c.data
	
	// Adjust throttling
	t.adjustThrottlingInternal()
}

// Bulk update progress command
type bulkUpdateProgressCommand struct {
	updates map[string]map[string]interface{}
}

func (c *bulkUpdateProgressCommand) execute(t *AdaptiveEventThrottler) {
	if !t.enabled {
		// Send all updates directly
		progressUpdates := make([]map[string]interface{}, 0, len(c.updates))
		for _, update := range c.updates {
			progressUpdates = append(progressUpdates, update)
		}
		if t.broadcaster != nil {
			t.broadcaster("progress.batch", progressUpdates)
		}
		return
	}
	
	// Auto-enable high load mode for bulk updates
	if !t.highLoadMode && len(c.updates) > 20 {
		t.setHighLoadModeInternal(defaultHighLoadTimeout)
	}
	
	// Merge updates into buffer
	for id, data := range c.updates {
		t.progressBuffer[id] = data
	}
	
	// Force flush if many updates
	if len(c.updates) > 50 {
		t.doFlush(false)
	} else {
		t.adjustThrottlingInternal()
	}
}

// Set high load mode command
type setHighLoadModeCommand struct {
	enabled  bool
	duration time.Duration
	done     chan struct{} // Optional done signal for sync calls
}

func (c *setHighLoadModeCommand) execute(t *AdaptiveEventThrottler) {
	if c.enabled {
		t.setHighLoadModeInternal(c.duration)
	} else {
		// Cancel any existing timer
		if t.highLoadModeTimer != nil {
			t.highLoadModeTimer.Stop()
			t.highLoadModeTimer = nil
		}
		
		// Disable high load mode if it was enabled
		if t.highLoadMode {
			t.highLoadMode = false
			t.eventCounter = 0
			t.lastRateReset = time.Now()
			t.eventTimeWindow = t.eventTimeWindow[:0]
			
			t.logger.Debug().Msg("High load mode manually disabled")
		}
	}
	
	// Signal completion if awaiting
	if c.done != nil {
		close(c.done)
	}
}

// Flush command
type flushCommand struct {
	sync  bool         // Whether this is a synchronous flush
	done  chan struct{} // Signal completion (for sync flushes)
}

func (c *flushCommand) execute(t *AdaptiveEventThrottler) {
	// Perform the flush
	t.doFlush(c.sync)
	
	// Signal completion if synchronous
	if c.sync && c.done != nil {
		close(c.done)
	}
}

// Shutdown command
type shutdownCommand struct {
	done chan struct{}
}

func (c *shutdownCommand) execute(t *AdaptiveEventThrottler) {
	// Final flush to ensure no data is lost
	t.doFlush(true)
	
	// Cleanup
	if t.highLoadModeTimer != nil {
		t.highLoadModeTimer.Stop()
		t.highLoadModeTimer = nil
	}
	
	// Signal completion
	if c.done != nil {
		close(c.done)
	}
}

// Set enabled command
type setEnabledCommand struct {
	enabled bool
	done    chan struct{}
}

func (c *setEnabledCommand) execute(t *AdaptiveEventThrottler) {
	if t.enabled != c.enabled {
		t.enabled = c.enabled
		
		if c.enabled {
			// Reset counters when enabling
			t.eventCounter = 0
			t.lastRateReset = time.Now()
			t.eventTimeWindow = t.eventTimeWindow[:0]
		} else {
			// Flush pending events when disabling
			t.doFlush(false)
		}
		
		t.logger.Debug().Bool("enabled", c.enabled).Msg("Event throttling state changed")
	}
	
	if c.done != nil {
		close(c.done)
	}
}

// Set min interval command
type setMinIntervalCommand struct {
	interval time.Duration
}

func (c *setMinIntervalCommand) execute(t *AdaptiveEventThrottler) {
	if t.minInterval != c.interval {
		t.minInterval = c.interval
		t.logger.Debug().Dur("minInterval", c.interval).Msg("Updated minimum throttling interval")
	}
}

// Set max interval command
type setMaxIntervalCommand struct {
	interval time.Duration
}

func (c *setMaxIntervalCommand) execute(t *AdaptiveEventThrottler) {
	if t.maxInterval != c.interval {
		t.maxInterval = c.interval
		t.logger.Debug().Dur("maxInterval", c.interval).Msg("Updated maximum throttling interval")
	}
}

// Set direct pass threshold command
type setDirectPassThresholdCommand struct {
	threshold float64
}

func (c *setDirectPassThresholdCommand) execute(t *AdaptiveEventThrottler) {
	if t.directPassThreshold != c.threshold {
		t.directPassThreshold = c.threshold
		t.logger.Debug().Float64("threshold", c.threshold).Msg("Updated direct pass-through threshold")
	}
}

// Get status command
type getStatusCommand struct {
	result chan map[string]interface{}
}

func (c *getStatusCommand) execute(t *AdaptiveEventThrottler) {
	status := map[string]interface{}{
		"enabled":            t.enabled,
		"highLoadMode":       t.highLoadMode,
		"currentRate":        t.currentRate,
		"currentInterval":    t.currentInterval.Milliseconds(),
		"pendingLogs":        len(t.logBuffer),
		"pendingProgress":    len(t.progressBuffer),
		"maxBufferSize":      t.maxBufferSize,
		"directPassThreshold": t.directPassThreshold,
		"running":            t.isRunning,
	}
	
	c.result <- status
}

// AdaptiveEventThrottler manages the buffering and throttling of events to the frontend
type AdaptiveEventThrottler struct {
	// Context
	ctx                context.Context
	
	// Buffers
	logBuffer          []string
	progressBuffer     map[string]map[string]interface{}
	
	// Command handling - the core of our new design
	commandChan        chan command
	isRunning          bool
	
	// Event rate tracking
	eventCounter       int
	rateWindow         time.Duration
	lastRateReset      time.Time
	currentRate        float64
	eventTimeWindow    []time.Time
	directPassThreshold float64
	
	// Throttling state
	highLoadMode       bool
	highLoadModeTimer  *time.Timer
	currentInterval    time.Duration
	lastEmitTime       time.Time
	
	// Configuration
	enabled            bool
	minInterval        time.Duration
	maxInterval        time.Duration
	logSequence        int64
	maxBufferSize      int
	logger             *zerolog.Logger
	
	// Event broadcaster (WebSocket)
	broadcaster        func(msgType string, data interface{})
}

// NewAdaptiveEventThrottler creates a new throttler instance with the given parameters
func NewAdaptiveEventThrottler(
	ctx context.Context,
	minInterval time.Duration,
	maxInterval time.Duration,
	rateWindow time.Duration,
	enabled bool,
	logger *zerolog.Logger,
	broadcaster func(msgType string, data interface{}),
) *AdaptiveEventThrottler {
	t := &AdaptiveEventThrottler{
		ctx:                ctx,
		logBuffer:          make([]string, 0, 1000),
		progressBuffer:     make(map[string]map[string]interface{}),
		commandChan:        make(chan command, 100), // Buffered channel for commands
		isRunning:          true,
		rateWindow:         rateWindow,
		lastRateReset:      time.Now(),
		minInterval:        minInterval,
		maxInterval:        maxInterval,
		lastEmitTime:       time.Now(),
		enabled:            enabled,
		directPassThreshold: 20.0, // Direct pass-through for < 20 events/sec
		eventTimeWindow:    make([]time.Time, 0, 100),
		maxBufferSize:      5000,
		logger:             logger,
		broadcaster:        broadcaster,
	}
	
	// Start command processor
	go t.processCommands()
	
	return t
}

// processCommands runs in a separate goroutine and handles all commands
func (t *AdaptiveEventThrottler) processCommands() {
	t.logger.Debug().Msg("Command processor started")
	
	// Create a ticker for periodic flushes
	periodicFlushTicker := time.NewTicker(250 * time.Millisecond)
	defer periodicFlushTicker.Stop()
	
	for t.isRunning {
		select {
		case cmd, ok := <-t.commandChan:
			if !ok {
				// Channel closed, exit
				t.logger.Debug().Msg("Command channel closed, exiting processor")
				return
			}
			
			// Execute the command
			cmd.execute(t)
			
		case <-periodicFlushTicker.C:
			// Periodically check if we need to flush
			if len(t.logBuffer) > 0 || len(t.progressBuffer) > 0 {
				t.doFlush(false)
			}
			
		case <-t.ctx.Done():
			// Context canceled, shut down
			t.logger.Debug().Msg("Context canceled, shutting down command processor")
			t.isRunning = false
			t.doFlush(true) // Final flush
			return
		}
	}
	
	t.logger.Debug().Msg("Command processor exited")
}

// AddLog adds a log entry - public API
func (t *AdaptiveEventThrottler) AddLog(log string) {
	if t.isRunning {
		t.commandChan <- &addLogCommand{log: log, direct: false}
	}
}

// UpdateProgress updates a progress bar - public API
func (t *AdaptiveEventThrottler) UpdateProgress(id string, data map[string]interface{}) {
	if t.isRunning {
		t.commandChan <- &updateProgressCommand{id: id, data: data, direct: false}
	}
}

// BulkUpdateProgress handles multiple progress updates - public API
func (t *AdaptiveEventThrottler) BulkUpdateProgress(updates map[string]map[string]interface{}) {
	if t.isRunning {
		t.commandChan <- &bulkUpdateProgressCommand{updates: updates}
	}
}

// SetHighLoadMode enables/disables high load mode - public API
func (t *AdaptiveEventThrottler) SetHighLoadMode(enabled bool) {
	if t.isRunning {
		t.commandChan <- &setHighLoadModeCommand{
			enabled:  enabled,
			duration: defaultHighLoadTimeout,
		}
	}
}

// SetHighLoadModeWithTimeout sets high load mode with a custom timeout - public API
func (t *AdaptiveEventThrottler) SetHighLoadModeWithTimeout(durations ...time.Duration) {
	if !t.isRunning {
		return
	}
	
	// Determine duration
	duration := defaultHighLoadTimeout
	if len(durations) > 0 && durations[0] > 0 {
		duration = durations[0]
	}
	
	t.commandChan <- &setHighLoadModeCommand{
		enabled:  true,
		duration: duration,
	}
}

// Flush asynchronously flushes pending events - public API
func (t *AdaptiveEventThrottler) Flush() {
	if t.isRunning {
		t.commandChan <- &flushCommand{sync: false}
	}
}

// SyncFlush synchronously flushes pending events - public API
func (t *AdaptiveEventThrottler) SyncFlush() {
	if !t.isRunning {
		return
	}
	
	// Create a channel to wait for completion
	done := make(chan struct{})
	
	t.commandChan <- &flushCommand{
		sync: true,
		done: done,
	}
	
	// Wait for the flush to complete
	<-done
}

// Shutdown stops the command processor - public API
func (t *AdaptiveEventThrottler) Shutdown() {
	if !t.isRunning {
		return // Already shut down
	}
	
	t.logger.Debug().Msg("Shutting down throttler")
	
	// Create a channel to wait for completion
	done := make(chan struct{})
	
	// Send shutdown command
	t.commandChan <- &shutdownCommand{done: done}
	
	// Wait for shutdown to complete
	<-done
	
	// Mark as not running and close command channel
	t.isRunning = false
	close(t.commandChan)
	
	t.logger.Debug().Msg("Throttler shutdown complete")
}

// SetEnabled toggles throttling on/off - public API
func (t *AdaptiveEventThrottler) SetEnabled(enabled bool) {
	if t.isRunning {
		// Use a synchronous call to ensure it completes
		done := make(chan struct{})
		
		t.commandChan <- &setEnabledCommand{
			enabled: enabled,
			done:    done,
		}
		
		<-done
	}
}

// SetMinInterval sets the minimum throttling interval - public API
func (t *AdaptiveEventThrottler) SetMinInterval(interval time.Duration) {
	if t.isRunning {
		t.commandChan <- &setMinIntervalCommand{interval: interval}
	}
}

// SetMaxInterval sets the maximum throttling interval - public API
func (t *AdaptiveEventThrottler) SetMaxInterval(interval time.Duration) {
	if t.isRunning {
		t.commandChan <- &setMaxIntervalCommand{interval: interval}
	}
}

// SetDirectPassThreshold sets the threshold for direct pass-through - public API
func (t *AdaptiveEventThrottler) SetDirectPassThreshold(threshold float64) {
	if t.isRunning {
		t.commandChan <- &setDirectPassThresholdCommand{threshold: threshold}
	}
}

// GetStatus returns the current throttling status - public API
func (t *AdaptiveEventThrottler) GetStatus() map[string]interface{} {
	if !t.isRunning {
		return map[string]interface{}{
			"enabled":  false,
			"running":  false,
			"error":    "Throttler not running",
		}
	}
	
	// Use a synchronous call to get status
	result := make(chan map[string]interface{})
	
	t.commandChan <- &getStatusCommand{result: result}
	
	// Wait for result
	return <-result
}

// Internal methods below - these run in the command processor goroutine

// setHighLoadModeInternal enables high load mode with a timeout
func (t *AdaptiveEventThrottler) setHighLoadModeInternal(duration time.Duration) {
	// Only log if state is changing or timer is being reset
	shouldLog := !t.highLoadMode || t.highLoadModeTimer != nil
	
	// Set high load mode
	t.highLoadMode = true
	t.currentInterval = t.maxInterval
	
	// Cancel existing timer if there is one
	if t.highLoadModeTimer != nil {
		t.highLoadModeTimer.Stop()
	}
	
	// Set the new timer
	t.highLoadModeTimer = time.AfterFunc(duration, func() {
		// Create a command to disable high load mode when timer fires
		if t.isRunning {
			t.commandChan <- &setHighLoadModeCommand{enabled: false}
		}
	})
	
	// Log only if state changed or timer reset
	if shouldLog {
		t.logger.Info().Dur("duration", duration).Msg("High load mode activated with timeout")
	}
}

// updateEventRateInternal updates the event rate tracking
func (t *AdaptiveEventThrottler) updateEventRateInternal() {
	const highLoadThreshold = 100.0 // events per second
	
	now := time.Now()
	
	// Add current time to sliding window
	t.eventTimeWindow = append(t.eventTimeWindow, now)
	
	// Keep only events within rate window
	cutoff := now.Add(-t.rateWindow)
	for len(t.eventTimeWindow) > 0 && t.eventTimeWindow[0].Before(cutoff) {
		t.eventTimeWindow = t.eventTimeWindow[1:]
	}
	
	// Calculate current rate (events per second)
	if len(t.eventTimeWindow) > 1 {
		windowDuration := now.Sub(t.eventTimeWindow[0]).Seconds()
		if windowDuration > 0 {
			t.currentRate = float64(len(t.eventTimeWindow)) / windowDuration
			
			// Auto-enable high load mode if rate exceeds threshold
			if t.currentRate > highLoadThreshold && !t.highLoadMode {
				t.setHighLoadModeInternal(defaultHighLoadTimeout)
			}
		}
	}
}

// adjustThrottlingInternal adjusts the throttling timing
func (t *AdaptiveEventThrottler) adjustThrottlingInternal() {
	// Skip adaptive adjustments if in high load mode
	if t.highLoadMode {
		// In high load mode, always use maximum throttling
		t.currentInterval = t.maxInterval
		
		// Check if it's time to emit
		now := time.Now()
		if now.Sub(t.lastEmitTime) >= t.currentInterval {
			t.doFlush(false)
		}
		return
	}
	
	// Normal adaptive throttling
	now := time.Now()
	
	// Count this event
	t.eventCounter++
	
	// Reset counter if rate window has passed
	if now.Sub(t.lastRateReset) > t.rateWindow {
		// Calculate events per second
		windowDuration := now.Sub(t.lastRateReset).Seconds()
		if windowDuration > 0 {
			t.currentRate = float64(t.eventCounter) / windowDuration
		}
		t.eventCounter = 0
		t.lastRateReset = now
		
		// Adjust throttling interval based on rate
		switch {
		case t.currentRate < 10:
			t.currentInterval = t.minInterval // Low rate: minimal/no throttling
		case t.currentRate < 100:
			t.currentInterval = 50 * time.Millisecond
		case t.currentRate < 500:
			t.currentInterval = 100 * time.Millisecond
		default:
			t.currentInterval = t.maxInterval // Very high rate: max throttling
		}
	}
	
	// Check if it's time to emit with special handling for zero interval
	if (t.currentInterval > 0 && now.Sub(t.lastEmitTime) >= t.currentInterval) || 
	   (t.currentInterval == 0 && len(t.progressBuffer) > 0 && now.Sub(t.lastEmitTime) >= 100*time.Millisecond) {
		t.doFlush(false)
	}
}

// doFlush performs the actual flush operation
func (t *AdaptiveEventThrottler) doFlush(sync bool) {
	// Update last emit time
	t.lastEmitTime = time.Now()
	
	// Send logs if there are any
	if len(t.logBuffer) > 0 {
		// Make a copy of the buffer
		logsCopy := make([]string, len(t.logBuffer))
		copy(logsCopy, t.logBuffer)
		
		// Clear the buffer
		t.logBuffer = t.logBuffer[:0]
		
		// Send the batch event
		if t.broadcaster != nil {
			if sync {
				// Synchronous emission
				t.broadcaster("log.batch", logsCopy)
			} else {
				// Asynchronous emission
				go t.broadcaster("log.batch", logsCopy)
			}
		}
	}
	
	// Send progress updates if there are any
	if len(t.progressBuffer) > 0 {
		// Convert map to slice
		progressUpdates := make([]map[string]interface{}, 0, len(t.progressBuffer))
		for _, update := range t.progressBuffer {
			progressUpdates = append(progressUpdates, update)
		}
		
		// Clear the buffer
		t.progressBuffer = make(map[string]map[string]interface{})
		
		// Send the batch event
		if t.broadcaster != nil {
			if sync {
				// Synchronous emission
				t.broadcaster("progress.batch", progressUpdates)
			} else {
				// Asynchronous emission
				go t.broadcaster("progress.batch", progressUpdates)
			}
		}
	}
}

// isCriticalLog determines if a log should bypass throttling
func (t *AdaptiveEventThrottler) isCriticalLog(logData map[string]interface{}) bool {
	// Check log level
	if level, ok := logData["level"]; ok {
		levelStr, isString := level.(string)
		if isString && (levelStr == "error" || levelStr == "ERROR") {
			// Check behavior field
			if behavior, ok := logData["behavior"]; ok {
				behaviorStr, isString := behavior.(string)
				if isString && (behaviorStr == "abort_task" || behaviorStr == "abort_all") {
					return true
				}
			}
		}
	}
	
	// Check message content for cancel/abort keywords
	if message, ok := logData["message"]; ok {
		msgStr, isString := message.(string)
		if isString {
			lowMsg := strings.ToLower(msgStr)
			if strings.Contains(lowMsg, "cancel") || strings.Contains(lowMsg, "abort") {
				return true
			}
		}
	}
	
	return false
}