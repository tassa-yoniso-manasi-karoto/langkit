# Specification: Adaptive Event Throttling System for Langkit

## 1. Introduction and Problem Statement

Langkit currently experiences significant UI lag during processing operations due to the high frequency of individual events (logs and progress updates) being sent to the frontend. Each event triggers UI updates, eventually overwhelming the JavaScript event loop and causing the interface to become unresponsive. While the application currently implements virtual log rendering and frontend debouncing, these optimizations alone are insufficient when thousands of events per second are being generated.

The goal is to implement an adaptive event throttling system that dramatically reduces the number of individual events sent to the frontend while preserving all data for crash reporting and maintaining critical event responsiveness.

## 2. Design Requirements and Constraints

### Core Requirements

1. **Reduce UI Lag**: Significantly decrease the number of individual events sent to the frontend to prevent UI thread saturation.
2. **Preserve Complete Log Data**: Ensure crash reports contain all log entries with no loss of information.
3. **Maintain Responsiveness for Critical Events**: Error messages and important state changes must still appear immediately.
4. **Adapt to Processing Load**: Dynamically adjust throttling intensity based on the current event generation rate.
5. **Enable Optional Disabling**: Allow disabling the throttling mechanism for debugging purposes.

### Constraints and Considerations

1. **Crash Package Compatibility**: Must seamlessly integrate with the existing crash reporting system and ensure all log data is available for crash reports.
2. **Temporal Consistency**: Event order must be preserved for accurate debugging and crash analysis.
3. **Concurrent Operation Safety**: Must be thread-safe during high-volume processing operations.
4. **Minimal Change to Frontend Logic**: Avoid extensive changes to existing frontend event handling.

## 3. System Architecture

### 3.1 Dual-Path Event System

The core architectural concept is a **dual-path event system**:

```
┌───────────────┐     ┌─────────────────┐     ┌──────────────┐
│ Log/Progress  │────▶│ Buffer          │────▶│ Crash Reports│
│ Generation    │     │ (immediate)     │     │              │
└───────┬───────┘     └─────────────────┘     └──────────────┘
        │
        │             ┌─────────────────┐     ┌──────────────┐
        └────────────▶│ Adaptive        │────▶│ Frontend UI  │
                      │ Throttler       │     │              │
                      └─────────────────┘     └──────────────┘
```

This architecture ensures:
- All log entries are immediately written to the buffer used by crash reports
- Only the transmission of events to the frontend is throttled
- Critical events can bypass the throttling mechanism entirely

### 3.2 Core Components

1. **AdaptiveEventThrottler**: Central component that manages event batching and throttling
2. **ThrottledLogWriter**: Modified log writer that maintains dual-path behavior
3. **GUIHandler Integration**: Updated handler that uses the throttler instead of direct event emission
4. **Frontend Batch Processors**: New event handlers that efficiently process batched events

## 4. Detailed Component Specifications

### 4.1 AdaptiveEventThrottler

```go
type AdaptiveEventThrottler struct {
    ctx                context.Context
    logBuffer          []string
    progressBuffer     map[string]map[string]interface{}
    mutex              sync.RWMutex
    
    // Adaptive parameters
    eventCounter       int
    rateWindow         time.Duration
    lastRateReset      time.Time
    currentRate        float64
    
    // Throttling state
    lastEmitTime       time.Time
    currentInterval    time.Duration
    minInterval        time.Duration
    maxInterval        time.Duration
    
    // Control
    enabled            bool
    isRunning          bool
    flushChan          chan struct{}
}
```

#### Constructor

```go
func NewAdaptiveEventThrottler(
    ctx context.Context,
    minInterval time.Duration,
    maxInterval time.Duration,
    rateWindow time.Duration,
    enabled bool,
) *AdaptiveEventThrottler {
    t := &AdaptiveEventThrottler{
        ctx:             ctx,
        logBuffer:       make([]string, 0, 1000),
        progressBuffer:  make(map[string]map[string]interface{}),
        rateWindow:      rateWindow,
        lastRateReset:   time.Now(),
        minInterval:     minInterval,
        maxInterval:     maxInterval,
        lastEmitTime:    time.Now(),
        enabled:         enabled,
        isRunning:       true,
        flushChan:       make(chan struct{}, 1),
    }
    
    go t.processBatches()
    return t
}
```

#### Key Methods

1. **`AddLog(log string)`**:
   - Checks if log is critical (by parsing JSON)
   - Sends critical logs immediately, bypassing throttling
   - Otherwise, adds to buffer and adjusts throttling timing

2. **`UpdateProgress(id string, data map[string]interface{})`**:
   - Stores only the latest update for each task ID
   - Triggers throttling adjustment

3. **`adjustThrottling()`**:
   - Counts events in the current rate window
   - Dynamically adjusts `currentInterval` based on event frequency
   - Schedules flush if interval has elapsed

4. **`processBatches()`**:
   - Background goroutine that manages timed flushes
   - Listens for flush signals or context cancellation

5. **`Flush()`**:
   - Sends all pending events to the frontend
   - Called manually before crash report generation

6. **`SetEnabled(enabled bool)`**:
   - Toggles throttling for debugging purposes

### 4.2 ThrottledLogWriter

```go
type ThrottledLogWriter struct {
    ctx         context.Context
    throttler   *AdaptiveEventThrottler
    consoleOut  io.Writer
}

func (w *ThrottledLogWriter) Write(p []byte) (n int, err error) {
    // Always write to buffer immediately for crash reports
    n, err = w.consoleOut.Write(p)
    if err != nil {
        return n, err
    }
    
    // Send to throttler for frontend updates
    // Skip TRACE level (-1) logs
    // ...parse logic...
    
    w.throttler.AddLog(string(p))
    return len(p), nil
}
```

### 4.3 GUIHandler Integration

Modify `GUIHandler` to use the throttler:

```go
type GUIHandler struct {
    ctx         context.Context
    logger      *zerolog.Logger
    buffer      bytes.Buffer
    progressMap map[string]int
    throttler   *AdaptiveEventThrottler
}

func NewGUIHandler(ctx context.Context, throttler *AdaptiveEventThrottler) *GUIHandler {
    h := &GUIHandler{
        ctx:        ctx,
        progressMap: make(map[string]int),
        throttler:  throttler,
    }
    
    // Configure log writer to use both buffer and throttler
    multiWriter := zerolog.MultiLevelWriter(
        // For crash reports
        zerolog.ConsoleWriter{Out: io.MultiWriter(os.Stderr, &h.buffer)},
        // For frontend updates
        &ThrottledLogWriter{ctx: ctx, throttler: throttler, consoleOut: &h.buffer},
    )
    
    logger := zerolog.New(multiWriter).With().Timestamp().Logger()
    h.logger = &logger
    
    return h
}

func (h *GUIHandler) IncrementProgress(...) {
    // Update local tracking
    h.progressMap[taskID] += increment
    current := h.progressMap[taskID]
    
    // Create payload
    payload := map[string]interface{}{
        "id":          taskID,
        "progress":    percent,
        // ...other fields...
    }
    
    // Send through throttler instead of direct emit
    h.throttler.UpdateProgress(taskID, payload)
    
    // Cleanup as before
    if total > 0 && current >= total {
        delete(h.progressMap, taskID)
    }
}
```

### 4.4 Frontend Event Handling

Add batch event handlers to `App.svelte`:

```javascript
// Add batch event listeners
EventsOn("log-batch", (logBatch) => {
    if (Array.isArray(logBatch) && logBatch.length > 0) {
        // Batch process all logs at once
        requestAnimationFrame(() => {
            logBatch.forEach(log => logStore.addLog(log));
        });
    }
});

EventsOn("progress-batch", (progressBatch) => {
    if (Array.isArray(progressBatch) && progressBatch.length > 0) {
        // Add all updates to pending queue
        pendingProgressUpdates.push(...progressBatch);
        
        // Process on next frame if not already scheduled
        if (!progressUpdateDebounceTimer) {
            progressUpdateDebounceTimer = window.requestAnimationFrame(() => {
                processProgressUpdates();
                progressUpdateDebounceTimer = null;
            });
        }
    }
});
```

## 5. Integration with Crash Reporting System

### 5.1 Buffer Integrity Preservation

The dual-path architecture ensures that all logs continue to be written to the buffer immediately, maintaining complete data for crash reports. The existing `GetLogBuffer()` method in `GUIHandler` will still return the complete buffer with all log entries.

### 5.2 Pre-Report Flushing

To ensure all pending events are processed before generating crash reports:

```go
// In App.ExportDebugReport (internal/gui/err.go)
func (a *App) ExportDebugReport() error {
    // Flush any pending events before generating report
    if a.throttler != nil {
        a.throttler.Flush()
    }
    
    settings, err := config.LoadSettings()
    // Rest of existing code...
}
```

### 5.3 Critical Event Paths

Events classified as critical bypass the throttling system entirely:

- Error logs with `abort_task` or `abort_all` behavior
- Logs containing cancellation or abort messages
- Any logs when throttling is disabled

This ensures critical information is immediately visible to users.

## 6. Configuration and Control

### 6.1 Settings Integration

Add throttling configuration to the settings structure in `internal/config/settings.go`:

```go
type Settings struct {
    // Existing fields...
    
    // Event throttling settings
    EventThrottling struct {
        Enabled     bool   `json:"enabled" mapstructure:"enabled"`
        MinInterval int    `json:"minInterval" mapstructure:"min_interval"` // milliseconds
        MaxInterval int    `json:"maxInterval" mapstructure:"max_interval"` // milliseconds
    } `json:"eventThrottling" mapstructure:"event_throttling"`
}
```

Update the `InitConfig` function to set defaults:

```go
func InitConfig(customPath string) error {
    // Existing configuration...
    
    // Default throttling settings
    viper.SetDefault("event_throttling.enabled", true)
    viper.SetDefault("event_throttling.min_interval", 0)     // 0ms = no throttle when quiet
    viper.SetDefault("event_throttling.max_interval", 250)   // 250ms max interval
    
    // Existing code...
}
```

### 6.2 Runtime Control

Add methods to control throttling at runtime:

```go
// In App struct
func (a *App) SetEventThrottling(enabled bool) {
    if a.throttler != nil {
        a.throttler.SetEnabled(enabled)
    }
}

func (a *App) GetEventThrottlingStatus() map[string]interface{} {
    if a.throttler == nil {
        return map[string]interface{}{
            "enabled": false,
            "currentRate": 0.0,
            "currentInterval": 0,
        }
    }
    
    return a.throttler.GetStatus()
}
```

### 6.3 UI Settings Integration

Add throttling controls to the Settings component:

```javascript
// In Settings.svelte
<div class="setting-row">
    <div class="setting-label">
        <span>Event Throttling</span>
        <div class="setting-description">
            Improves UI responsiveness during processing by batching updates
        </div>
    </div>
    <div class="setting-control">
        <label class="switch">
            <input type="checkbox" bind:checked={$settings.eventThrottling.enabled}
                   on:change={() => window.go.gui.App.SetEventThrottling($settings.eventThrottling.enabled)}>
            <span class="slider round"></span>
        </label>
    </div>
</div>
```

## 7. Adaptive Throttling Logic

The adaptive throttling system dynamically adjusts based on event frequency:

```go
func (t *AdaptiveEventThrottler) adjustThrottling() {
    now := time.Now()
    
    // Count this event
    t.eventCounter++
    
    // Reset counter if rate window has passed
    if now.Sub(t.lastRateReset) > t.rateWindow {
        // Calculate events per second
        t.currentRate = float64(t.eventCounter) / now.Sub(t.lastRateReset).Seconds()
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
    
    // Check if it's time to emit based on the adaptive interval
    if t.currentInterval > 0 && now.Sub(t.lastEmitTime) >= t.currentInterval {
        select {
        case t.flushChan <- struct{}{}:
            // Signal sent
        default:
            // Channel full, already pending flush
        }
    }
}
```

This approach:
1. Counts events in a sliding window (default 500ms)
2. Calculates the current event rate (events per second)
3. Adjusts the throttling interval based on the rate:
   - <10 events/sec: No throttling (immediate updates)
   - 10-100 events/sec: 50ms interval
   - 100-500 events/sec: 100ms interval
   - >500 events/sec: Maximum throttling (default 250ms)
4. Triggers a flush when the adaptive interval has elapsed

## 8. Implementation Considerations

### 8.1 Concurrency Safety

The implementation must be thread-safe, as events can be generated from multiple goroutines:

- Use mutexes for all buffer access
- Make critical sections as small as possible
- Use non-blocking channel operations for flush signals

### 8.2 Memory Management

To prevent unbounded memory growth:

- Cap log buffer size (e.g., to 1000 entries)
- Only store the latest progress update for each task ID
- Flush automatically if buffers exceed size thresholds

### 8.3 Critical Event Detection

Implement robust logic to detect critical events that should bypass throttling:

```go
func isCriticalLog(logData map[string]interface{}) bool {
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
```

### 8.4 Batch Event Format

Send batched events using consistent formats:

```go
// Log batch: array of log strings
runtime.EventsEmit(t.ctx, "log-batch", logs)

// Progress batch: array of progress update maps
runtime.EventsEmit(t.ctx, "progress-batch", progressUpdates)
```

## 9. Implementation Plan and Testing

### 9.1 Implementation Phases

1. **Phase 1: Core Throttler Implementation**
   - Implement `AdaptiveEventThrottler`
   - Add batch event handling to frontend

2. **Phase 2: Integration with Existing System**
   - Modify `GUIHandler` to use throttler
   - Integrate with crash reporting system

3. **Phase 3: User Settings and Controls**
   - Add configuration options
   - Implement UI controls

4. **Phase 4: Testing and Optimization**
   - Verify crash report integrity
   - Test performance under various loads
   - Fine-tune adaptive parameters

### 9.2 Testing Scenarios

1. **High Volume Testing**
   - Process large media files that generate thousands of log entries
   - Verify UI remains responsive

2. **Crash Reporting Verification**
   - Intentionally trigger errors during processing
   - Verify crash reports contain all log entries

3. **Critical Event Testing**
   - Verify error messages appear immediately
   - Test cancellation and user interactions during heavy processing

4. **Adaptive Behavior Testing**
   - Verify throttling adjusts based on event frequency
   - Measure UI responsiveness at different load levels

## 10. Conclusion

This adaptive event throttling system will significantly improve Langkit's UI responsiveness during intensive processing operations without compromising crash reporting capabilities or critical event visibility. The dual-path architecture ensures complete data preservation while dramatically reducing frontend event load. 

The adaptive nature of the throttling ensures optimal user experience across varying workloads, from light processing (minimal throttling) to intensive operations (maximum throttling). The system's toggle capability provides an easy fallback for debugging purposes when needed.

By implementing this system, Langkit will maintain its robust debugging and crash reporting capabilities while delivering a much smoother and more responsive user interface during heavy processing tasks.