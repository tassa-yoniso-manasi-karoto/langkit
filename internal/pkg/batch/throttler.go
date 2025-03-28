package batch

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AdaptiveEventThrottler manages the buffering and throttling of events to the frontend
// while ensuring all data is preserved for crash reporting.
type AdaptiveEventThrottler struct {
	ctx                context.Context
	logBuffer          []string          // Buffer for log events
	progressBuffer     map[string]map[string]interface{} // Buffer for progress events, keyed by task ID
	mutex              sync.RWMutex      // Protects all buffer operations
	
	// Adaptive parameters
	eventCounter       int               // Count of events in the current rate window
	rateWindow         time.Duration     // Duration window for rate calculation (e.g., 500ms)
	lastRateReset      time.Time         // When the rate window was last reset
	currentRate        float64           // Current events per second rate
	
	// Event rate tracking with sliding window
	eventTimeWindow    []time.Time       // Sliding window of event timestamps
	directPassThreshold float64          // Threshold below which to use direct pass-through
	
	// Throttling state
	lastEmitTime       time.Time         // When the last batch was emitted
	currentInterval    time.Duration     // Current throttling interval (dynamically adjusted)
	minInterval        time.Duration     // Minimum throttling interval (0 means no throttle when quiet)
	maxInterval        time.Duration     // Maximum throttling interval (upper bound)
	
	// Control
	enabled            bool              // Whether throttling is enabled
	isRunning          bool              // Whether the throttler is running
	flushChan          chan struct{}     // Signal channel for manual flush requests
	highLoadMode       bool              // Flag for high-volume situations (task resumption)
	
	// Sequence tracking for chronological ordering
	logSequence        int64             // Monotonically increasing sequence number
	sequenceMutex      sync.Mutex        // Dedicated mutex for sequence operations
	
	// Configuration
	maxBufferSize      int               // Maximum buffer size to prevent memory issues
	logger             *zerolog.Logger   // Logger for internal messages
	
	// High load mode timer management
	highLoadModeMutex     sync.Mutex
	highLoadModeTimer     *time.Timer
	highLoadModeDuration  time.Duration
}


var defaultHighLoadTimeout = 5 * time.Second


// NewAdaptiveEventThrottler creates a new throttler instance with the given parameters
func NewAdaptiveEventThrottler(
	ctx context.Context,
	minInterval time.Duration,
	maxInterval time.Duration,
	rateWindow time.Duration,
	enabled bool,
	logger *zerolog.Logger,
) *AdaptiveEventThrottler {
	t := &AdaptiveEventThrottler{
		ctx:                ctx,
		logBuffer:          make([]string, 0, 1000),
		progressBuffer:     make(map[string]map[string]interface{}),
		rateWindow:         rateWindow,
		lastRateReset:      time.Now(),
		minInterval:        minInterval,
		maxInterval:        maxInterval,
		lastEmitTime:       time.Now(),
		enabled:            enabled,
		highLoadModeDuration: defaultHighLoadTimeout,
		isRunning:          true,
		flushChan:          make(chan struct{}, 1),
		highLoadMode:       false,
		maxBufferSize:      5000, // Increased capacity for high-volume events
		logger:             logger,
		directPassThreshold: 20.0, // Direct pass-through for < 20 events/sec
		eventTimeWindow:    make([]time.Time, 0, 100),
	}
	
	// Start background processing goroutine
	go t.processBatches()
	return t
}

// AddLog adds a log entry to the buffer, or sends it immediately if it's critical or direct pass-through applies
func (t *AdaptiveEventThrottler) AddLog(log string) {
    // Early return if throttling is disabled
    if !t.enabled {
        runtime.EventsEmit(t.ctx, "log", log)
        return
    }

    // Try to parse the log to check if it's critical and add sequence number and unix timestamp
    var logData map[string]interface{}
    isCritical := false
    
    if err := json.Unmarshal([]byte(log), &logData); err == nil {
        // Check if this is a critical log
        isCritical = t.isCriticalLog(logData)
        
        // Add sequence number for tracking
        t.sequenceMutex.Lock()
        sequence := t.logSequence
        t.logSequence++
        t.sequenceMutex.Unlock()
        
        // Add sequence to log data
        logData["_sequence"] = sequence
        
        // Add unix timestamp for more efficient sorting
        if timeStr, ok := logData["time"].(string); ok {
            if timeVal, err := time.Parse(time.RFC3339, timeStr); err == nil {
                // Add unix timestamp in milliseconds
                logData["_unix_time"] = timeVal.UnixNano() / int64(time.Millisecond)
            } else {
                // If can't parse, use current time
                logData["_unix_time"] = time.Now().UnixNano() / int64(time.Millisecond)
            }
        } else {
            // No time field, use current time
            logData["_unix_time"] = time.Now().UnixNano() / int64(time.Millisecond)
        }
        
        // Re-serialize with added metadata
        if modifiedLog, err := json.Marshal(logData); err == nil {
            log = string(modifiedLog)
        }
    }

    // Critical logs bypass the throttling system
    if isCritical {
        runtime.EventsEmit(t.ctx, "log", log)
        return
    }
    
    // Update event rate and check for direct pass-through
    t.updateEventRate()
    
    // Use direct pass-through for normal operation (low event rates)
    if t.currentRate < t.directPassThreshold && !t.highLoadMode {
        runtime.EventsEmit(t.ctx, "log", log)
        return
    }

    // Lock for buffer modification
    t.mutex.Lock()
    defer t.mutex.Unlock()

    // Add to buffer with overflow protection
    if len(t.logBuffer) >= t.maxBufferSize {
        // Force a flush if buffer is getting full
        if float64(len(t.logBuffer)) > float64(t.maxBufferSize)*0.8 {
            go t.Flush()
        }
        
        // Only keep the most recent logs when buffer is full
        t.logBuffer = append(t.logBuffer[len(t.logBuffer)/5:], log)
    } else {
        t.logBuffer = append(t.logBuffer, log)
    }

    // Adjust throttling timing
    t.adjustThrottling()
}

// UpdateProgress stores only the latest update for each task ID
// or sends directly for normal operation
func (t *AdaptiveEventThrottler) UpdateProgress(id string, data map[string]interface{}) {
	// Early return if throttling is disabled
	if !t.enabled {
		runtime.EventsEmit(t.ctx, "progress", data)
		return
	}

    // Update event rate and check for direct pass-through
    t.updateEventRate()
    
    // Use direct pass-through for normal operation (low event rates)
    if t.currentRate < t.directPassThreshold && !t.highLoadMode {
        runtime.EventsEmit(t.ctx, "progress", data)
        return
    }

	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Store only the latest progress update
	t.progressBuffer[id] = data
	
	// Adjust throttling timing
	t.adjustThrottling()
}

// BulkUpdateProgress handles multiple progress updates efficiently
// Useful for task resumption scenarios with many simultaneous updates
func (t *AdaptiveEventThrottler) BulkUpdateProgress(updates map[string]map[string]interface{}) {
	if !t.enabled {
		// Send all updates directly
		progressUpdates := make([]map[string]interface{}, 0, len(updates))
		for _, update := range updates {
			progressUpdates = append(progressUpdates, update)
		}
		runtime.EventsEmit(t.ctx, "progress-batch", progressUpdates)
		return
	}
	
	// Automatically enable high load mode for bulk updates
	if !t.highLoadMode && len(updates) > 20 {
		// Use the centralized timeout method
		t.SetHighLoadModeWithTimeout()
	}
	
	t.mutex.Lock()
	defer t.mutex.Unlock()
	
	// Merge all updates into the buffer at once
	for id, data := range updates {
		t.progressBuffer[id] = data
	}
	
	// Force an immediate flush if many updates arrived
	if len(updates) > 50 {
		select {
		case t.flushChan <- struct{}{}:
			// Signal sent
		default:
			// Channel full, already pending flush
		}
	} else {
		t.adjustThrottling()
	}
}

// updateEventRate tracks the event rate using a sliding window approach
// and automatically detects high-load scenarios
func (t *AdaptiveEventThrottler) updateEventRate() {
    const highLoadThreshold = 100.0 // events per second
    
    now := time.Now()
    t.mutex.Lock()
    defer t.mutex.Unlock()
    
    // Add current time to a sliding window
    t.eventTimeWindow = append(t.eventTimeWindow, now)
    
    // Keep only events within the rate window
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
		t.SetHighLoadModeWithTimeout()
                t.logger.Info().Float64("rate", t.currentRate).Msg("Auto-enabled high load mode")
            }
            
            // Auto-disable when rate drops significantly
            if t.highLoadMode && t.currentRate < (highLoadThreshold * 0.5) {
                t.highLoadMode = false
                t.logger.Info().Float64("rate", t.currentRate).Msg("Auto-disabled high load mode")
            }
        }
    }
}

// adjustThrottling dynamically adjusts the throttling interval based on event frequency
func (t *AdaptiveEventThrottler) adjustThrottling() {
	// Skip adaptive adjustments if in high load mode
	if t.highLoadMode {
		// In high load mode, always use maximum throttling
		t.currentInterval = t.maxInterval
		
		// Check if it's time to emit
		now := time.Now()
		if now.Sub(t.lastEmitTime) >= t.currentInterval {
			select {
			case t.flushChan <- struct{}{}:
				// Signal sent
			default:
				// Channel full, already pending flush
			}
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
	
	// FIX: Check if it's time to emit with special handling for zero interval
	// This addresses the issue where progress bars don't update with zero interval
	if (t.currentInterval > 0 && now.Sub(t.lastEmitTime) >= t.currentInterval) || 
	   (t.currentInterval == 0 && len(t.progressBuffer) > 0 && now.Sub(t.lastEmitTime) >= 100*time.Millisecond) {
		select {
		case t.flushChan <- struct{}{}:
			// Signal sent
		default:
			// Channel full, already pending flush
		}
	}
}

// processBatches is a background goroutine that manages timed flushes
func (t *AdaptiveEventThrottler) processBatches() {
    // Add periodic flush timer for guaranteed updates
    periodicFlushTicker := time.NewTicker(250 * time.Millisecond)
    defer periodicFlushTicker.Stop()
    
	for t.isRunning {
		select {
		case <-t.ctx.Done():
			// Context was canceled, shutdown
			t.logger.Debug().Msg("Throttler context canceled, shutting down")
			return
			
		case <-t.flushChan:
			// Explicit flush requested
			t.emitBatches()
			
        case <-periodicFlushTicker.C:
            // Periodically flush any pending data regardless of other conditions
            t.mutex.RLock()
            hasData := len(t.logBuffer) > 0 || len(t.progressBuffer) > 0
            t.mutex.RUnlock()
            
            if hasData {
                t.emitBatches()
            }
			
		case <-time.After(100 * time.Millisecond):
			// Timeout check - see if we're due for a flush
			t.mutex.RLock()
			timeSinceLastEmit := time.Since(t.lastEmitTime)
			interval := t.currentInterval
			t.mutex.RUnlock()
			
			if interval > 0 && timeSinceLastEmit >= interval {
				t.emitBatches()
			}
		}
	}
}

// emitBatches sends all pending events to the frontend
func (t *AdaptiveEventThrottler) emitBatches() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	
	// Update last emit time
	t.lastEmitTime = time.Now()
	
	// Send logs if there are any
	if len(t.logBuffer) > 0 {
		// Make a copy of the buffer to prevent race conditions
		logsCopy := make([]string, len(t.logBuffer))
		copy(logsCopy, t.logBuffer)
		
		// Clear the buffer
		t.logBuffer = t.logBuffer[:0]
		
		// Send the batch event (outside the mutex lock)
		go runtime.EventsEmit(t.ctx, "log-batch", logsCopy)
	}
	
	// Send progress updates if there are any
	if len(t.progressBuffer) > 0 {
		// Convert map to slice for the event
		progressUpdates := make([]map[string]interface{}, 0, len(t.progressBuffer))
		for _, update := range t.progressBuffer {
			progressUpdates = append(progressUpdates, update)
		}
		
		// Clear the buffer
		t.progressBuffer = make(map[string]map[string]interface{})
		
		// Send the batch event (outside the mutex lock)
		go runtime.EventsEmit(t.ctx, "progress-batch", progressUpdates)
	}
}

// Flush manually sends all pending events to the frontend
// This should be called before generating crash reports
func (t *AdaptiveEventThrottler) Flush() {
	t.emitBatches()
}

// SyncFlush performs a synchronous flush - used for crash scenarios
// to ensure all data is visible to crash reporters
func (t *AdaptiveEventThrottler) SyncFlush() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	
	// Update last emit time
	t.lastEmitTime = time.Now()
	
	// For crash scenarios, we directly emit events synchronously
	// because we care more about data preservation than UI responsiveness
	
	// Send logs if there are any
	if len(t.logBuffer) > 0 {
		runtime.EventsEmit(t.ctx, "log-batch", t.logBuffer)
		t.logBuffer = t.logBuffer[:0]
	}
	
	// Send progress updates if there are any
	if len(t.progressBuffer) > 0 {
		progressUpdates := make([]map[string]interface{}, 0, len(t.progressBuffer))
		for _, update := range t.progressBuffer {
			progressUpdates = append(progressUpdates, update)
		}
		runtime.EventsEmit(t.ctx, "progress-batch", progressUpdates)
		t.progressBuffer = make(map[string]map[string]interface{})
	}
}

// SetEnabled toggles throttling on/off
func (t *AdaptiveEventThrottler) SetEnabled(enabled bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	
	// Only perform changes if the state is actually changing
	if t.enabled != enabled {
		t.enabled = enabled
		
		// If enabling, reset counters
		if enabled {
			t.eventCounter = 0
			t.lastRateReset = time.Now()
			t.eventTimeWindow = t.eventTimeWindow[:0]
		} else {
			// If disabling, flush any pending events
			go t.Flush()
		}
		
		t.logger.Debug().Bool("enabled", enabled).Msg("Event throttling state changed")
	}
}

// SetMinInterval sets the minimum throttling interval
func (t *AdaptiveEventThrottler) SetMinInterval(interval time.Duration) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	
	if t.minInterval != interval {
		t.minInterval = interval
		t.logger.Debug().Dur("minInterval", interval).Msg("Updated minimum throttling interval")
	}
}

// SetMaxInterval sets the maximum throttling interval
func (t *AdaptiveEventThrottler) SetMaxInterval(interval time.Duration) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	
	if t.maxInterval != interval {
		t.maxInterval = interval
		t.logger.Debug().Dur("maxInterval", interval).Msg("Updated maximum throttling interval")
	}
}

// SetDirectPassThreshold sets the threshold below which to use direct pass-through
func (t *AdaptiveEventThrottler) SetDirectPassThreshold(threshold float64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	
	if t.directPassThreshold != threshold {
		t.directPassThreshold = threshold
		t.logger.Debug().Float64("threshold", threshold).Msg("Updated direct pass-through threshold")
	}
}

// SetHighLoadModeWithTimeout activates high load mode for the specified duration
// Multiple calls will simply reset the timer rather than creating multiple instances
// If no duration is provided, it uses the default timeout
func (t *AdaptiveEventThrottler) SetHighLoadModeWithTimeout(durations ...time.Duration) {
	t.highLoadModeMutex.Lock()
	defer t.highLoadModeMutex.Unlock()

	// Determine the timeout duration (use provided or default)
	duration := defaultHighLoadTimeout
	if len(durations) > 0 && durations[0] > 0 {
		duration = durations[0]
	}

	// Only log if state is changing or timer is being reset
	shouldLog := !t.highLoadMode || t.highLoadModeTimer != nil

	// Set high load mode
	t.mutex.Lock()
	t.highLoadMode = true
	t.currentInterval = t.maxInterval
	t.mutex.Unlock()

	// Cancel existing timer if there is one
	if t.highLoadModeTimer != nil {
		t.highLoadModeTimer.Stop()
	}

	// Set the new timer
	t.highLoadModeDuration = duration
	t.highLoadModeTimer = time.AfterFunc(duration, func() {
		t.disableHighLoadMode()
	})

	// Log only if state changed or timer reset
	if shouldLog {
		t.logger.Info().Dur("duration", duration).Msg("High load mode activated with timeout")
	}
}



// SetHighLoadMode activates or deactivates high load mode for resumption scenarios
func (t *AdaptiveEventThrottler) SetHighLoadMode(enabled bool) {
	if enabled {
		t.SetHighLoadModeWithTimeout() // Use default timeout
	} else {
		t.highLoadModeMutex.Lock()
		defer t.highLoadModeMutex.Unlock()

		// Cancel any existing timer
		if t.highLoadModeTimer != nil {
			t.highLoadModeTimer.Stop()
			t.highLoadModeTimer = nil
		}

		// Disable high load mode
		t.mutex.Lock()
		wasEnabled := t.highLoadMode
		t.highLoadMode = false
		t.eventCounter = 0
		t.lastRateReset = time.Now()
		t.eventTimeWindow = t.eventTimeWindow[:0]
		t.mutex.Unlock()

		// Only log if state changed
		if wasEnabled {
			t.logger.Debug().Msg("High load mode manually disabled")
		}
	}
}

// disableHighLoadMode is called when the timer expires
func (t *AdaptiveEventThrottler) disableHighLoadMode() {
	t.highLoadModeMutex.Lock()
	defer t.highLoadModeMutex.Unlock()

	// Reset the timer reference
	t.highLoadModeTimer = nil

	// Switch back to normal mode
	t.mutex.Lock()
	wasEnabled := t.highLoadMode
	t.highLoadMode = false
	t.eventCounter = 0
	t.lastRateReset = time.Now()
	t.eventTimeWindow = t.eventTimeWindow[:0]
	t.mutex.Unlock()

	// Only log if state actually changed
	if wasEnabled {
		t.logger.Info().Msg("High load mode timeout expired - returning to adaptive throttling")
	}
}


// GetStatus returns the current throttling status
func (t *AdaptiveEventThrottler) GetStatus() map[string]interface{} {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	
	return map[string]interface{}{
		"enabled":         t.enabled,
		"highLoadMode":    t.highLoadMode,
		"currentRate":     t.currentRate,
		"currentInterval": t.currentInterval.Milliseconds(),
		"pendingLogs":     len(t.logBuffer),
		"pendingProgress": len(t.progressBuffer),
		"maxBufferSize":   t.maxBufferSize,
		"directPassThreshold": t.directPassThreshold,
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
			if contains(lowMsg, "cancel") || contains(lowMsg, "abort") {
				return true
			}
		}
	}
	
	return false
}

// isImportantLog determines if a log is important enough to keep
// when buffer is under pressure
func (t *AdaptiveEventThrottler) isImportantLog(logData map[string]interface{}) bool {
	// Check log level - higher levels are more important
	if level, ok := logData["level"]; ok {
		levelStr, isString := level.(string)
		if isString {
			levelLower := strings.ToLower(levelStr)
			// Prioritize error and warn logs
			if levelLower == "error" || levelLower == "warn" {
				return true
			}
		}
	}
	
	return false
}

// Shutdown gracefully stops the throttler
func (t *AdaptiveEventThrottler) Shutdown() {
	t.mutex.Lock()
	wasRunning := t.isRunning
	t.isRunning = false
	t.mutex.Unlock()
	
	// Only flush if we were running before
	if wasRunning {
		// Flush any remaining events
		t.SyncFlush()
		t.logger.Debug().Msg("Throttler shutdown complete")
	}
}

// Helper function to check if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}