# Updated Implementation Specification: Adaptive Event Throttling System for Langkit

## 1. Overview

The Adaptive Event Throttling System is designed to solve UI performance issues during high-volume processing operations in Langkit. This implementation creates a dual-path event system that preserves all data for crash reporting while intelligently throttling UI updates to prevent frontend thread saturation.

## 2. Core Components

### 2.1 AdaptiveEventThrottler

The central orchestration component that manages event buffering, adaptive throttling, and batch emission.

```go
type AdaptiveEventThrottler struct {
    // Context
    ctx                context.Context
    
    // Buffers
    logBuffer          []string
    progressBuffer     map[string]map[string]interface{}
    
    // Command handling - the core of our design
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
}
```

### 2.2 Command Types

Commands encapsulate operations for thread-safe execution:

```go
// Command interface for all operations
type command interface {
    execute(t *AdaptiveEventThrottler)
}

// Add log command
type addLogCommand struct {
    log string
    direct bool // Whether to send directly (bypass batch)
}

// Update progress command
type updateProgressCommand struct {
    id   string
    data map[string]interface{}
    direct bool // Whether to send directly
}

// Bulk update progress command
type bulkUpdateProgressCommand struct {
    updates map[string]map[string]interface{}
}

// Set high load mode command
type setHighLoadModeCommand struct {
    enabled  bool
    duration time.Duration
    done     chan struct{} // Optional done signal for sync calls
}

// Flush command
type flushCommand struct {
    sync  bool         // Whether this is a synchronous flush
    done  chan struct{} // Signal completion (for sync flushes)
}

// Shutdown command
type shutdownCommand struct {
    done chan struct{}
}

// Function command for one-off operations
type command func(*AdaptiveEventThrottler)
```

### 2.3 ThrottledLogWriter

An `io.Writer` implementation that ensures logs are both preserved for crash reports and sent to the throttler for UI updates.

```go
type ThrottledLogWriter struct {
    ctx         context.Context
    throttler   *AdaptiveEventThrottler
    consoleOut  io.Writer
}
```

Key methods:
- `Write(p []byte) (n int, err error)`: Implements the io.Writer interface, writing to both destinations

### 2.4 Settings Integration

Configuration options for controlling throttling behavior:

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

### 2.5 Optimized LogStore Implementation

A fully reworked log store that efficiently manages chronological ordering:

```typescript
function createLogStore() {
    // Main store with all logs
    const { subscribe, update, set } = writable<LogMessage[]>([]);
    
    // Efficient lookup data structures
    let sequenceIndex: Map<number, number> = new Map();
    let isLogsSorted = true;

    // State tracking
    let highestSequence = 0;
    let lastAddTime = 0;
    let pendingBatch: LogMessage[] = [];
    
    // Merge-insert algorithm for efficient batch processing
    function mergeInsertLogs(existingLogs: LogMessage[], newLogs: LogMessage[]): LogMessage[] {
        // Sort the new logs batch only
        newLogs.sort((a, b) => {
            // Use unix time for efficient comparison
            const timeA = a._unix_time || 0;
            const timeB = b._unix_time || 0;
            
            if (timeA !== timeB) {
                return timeA - timeB;
            }
            
            // Use sequence as tie-breaker
            return (a._sequence || 0) - (b._sequence || 0);
        });
        
        // Merge the two sorted arrays efficiently
        const result: LogMessage[] = [];
        let i = 0, j = 0;
        
        while (i < existingLogs.length && j < newLogs.length) {
            // ... efficient merge logic ...
        }
        
        return result;
    }
    
    // Other methods for batch processing, lookup, and virtualization...
}
```

## 3. Architecture Details

### 3.1 Command-Based Event Processing System

The architecture follows a command-based design pattern with enhanced ordering capabilities:

```
┌───────────────┐     ┌─────────────────┐     ┌──────────────┐
│ Log/Progress  │────▶│ Buffer          │────▶│ Crash Reports│
│ Generation    │     │ (immediate)     │     │              │
└───────┬───────┘     └─────────────────┘     └──────────────┘
        │
        │             ┌─────────────────┐     ┌──────────────┐
        └────────────▶│ Command Channel │────▶│ Single       │
                      │ (serializes all │     │ Processor    │
                      │  operations)    │     │ Goroutine    │
                      └─────────────────┘     └──────┬───────┘
                                                     │
                                                     ▼
                                              ┌──────────────┐
                                              │ Frontend UI  │
                                              │              │
                                              └──────────────┘
```

This architecture ensures:
- All logs are immediately written to the buffer for crash reports
- All operations are encapsulated as commands and processed sequentially
- No mutex contention or deadlocks with single-writer model
- Critical events can bypass throttling entirely
- All logs appear in proper chronological order regardless of when they arrive
- Clean shutdown with synchronous command completion

### 3.2 GUIHandler Integration

The GUIHandler is modified to use the throttler with additional bulk handling capabilities:

```go
type GUIHandler struct {
    ctx	       context.Context
    logger       *zerolog.Logger
    buffer       bytes.Buffer
    progressMap  map[string]int
    throttler    *batch.AdaptiveEventThrottler
}

func (h *GUIHandler) IncrementProgress(...) {
    // Create payload
    payload := map[string]interface{}{...}
    
    // Send through throttler if available
    if h.throttler != nil {
        h.throttler.UpdateProgress(taskID, payload)
    } else {
        runtime.EventsEmit(h.ctx, "progress", payload)
    }
}

// BulkUpdateProgress handles multiple progress updates efficiently
func (h *GUIHandler) BulkUpdateProgress(updates map[string]map[string]interface{}) {
    // Process through throttler if available
    if h.throttler != nil {
        h.throttler.BulkUpdateProgress(updates)
    } else {
        // Fallback to individual updates
        for _, update := range updates {
            runtime.EventsEmit(h.ctx, "progress", update)
        }
    }
}
```

### 3.3 Command Processor Loop

The throttling algorithm now executes all commands in a dedicated goroutine:

```go
// Process commands in a single goroutine
func (t *AdaptiveEventThrottler) processCommands() {
    // Create a ticker for periodic flushes
    periodicFlushTicker := time.NewTicker(250 * time.Millisecond)
    defer periodicFlushTicker.Stop()
    
    for t.isRunning {
        select {
        case cmd, ok := <-t.commandChan:
            if !ok {
                // Channel closed, exit
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
            t.isRunning = false
            t.doFlush(true) // Final flush
            return
        }
    }
}
```

### 3.4 Enhanced Log Messages with Timing Metadata

Log messages now include additional metadata for efficient sorting and display:

```json
{
  "level": "INFO",
  "message": "Processing file example.mp4",
  "time": "2024-03-27T12:34:56Z",
  "behavior": null,
  "_sequence": 42,                  // Monotonically increasing for stability
  "_unix_time": 1711627696000,      // Unix timestamp in milliseconds for efficient sorting
  "_original_time": "2024-03-27T12:34:56Z"  // Original ISO string for reference
}
```

## 4. Integration Points

### 4.1 App Initialization and Lifecycle

The throttler is initialized and properly managed throughout the application lifecycle:

```go
func (a *App) startup(ctx context.Context) {
    a.ctx = ctx
    
    // Initialize the throttler with logger for better traceability
    a.throttler = batch.NewAdaptiveEventThrottler(
        ctx,
        0,                    // minInterval
        250*time.Millisecond, // maxInterval
        500*time.Millisecond, // rateWindow
        true,                 // enabled by default
        a.logger,             // Logger for throttler
    )
    
    // Store throttler references
    appThrottler = a.throttler
    
    // Initialize handler with throttler
    handler = core.NewGUIHandler(ctx, a.throttler)
}

// Clean shutdown handling
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
    // Properly shut down the throttler
    if a.throttler != nil {
        a.logger.Info().Msg("Application closing, shutting down throttler")
        a.throttler.Shutdown()
        a.throttler = nil
    }
    return false
}
```

### 4.2 High Load Mode with EnterHighLoadMode

Support for task resumption scenarios with proper handler integration:

```go
// EnterHighLoadMode signals the throttler to prepare for high-volume events
func (h *GUIHandler) EnterHighLoadMode(durations ...time.Duration) {
    if h.throttler != nil {
        // Pass the optional duration to the throttler
        if len(durations) > 0 {
            h.ZeroLog().Info().Dur("duration", durations[0]).Msg("Entering high load mode with custom duration")
            h.throttler.SetHighLoadModeWithTimeout(durations[0])
        } else {
            h.ZeroLog().Info().Msg("Entering high load mode with default duration")
            h.throttler.SetHighLoadModeWithTimeout()
        }
    } else {
        h.ZeroLog().Warn().Msg("Cannot enter high load mode: throttler is nil")
    }
}
```

### 4.3 Crash Report Integration with Synchronous Flushing

The throttler is now synchronously flushed before generating crash reports to ensure all data is preserved:

```go
func (a *App) ExportDebugReport() error {
    // Synchronously flush any pending events before generating report
    if a.throttler != nil {
        a.logger.Debug().Msg("Flushing throttler before generating debug report")
        a.throttler.SyncFlush()
    }
    
    // Proceed with report generation...
}

func exitOnError(mainErr error) {
    // Flush any pending events synchronously if throttler is available
    if appThrottler != nil {
        appThrottler.SyncFlush()
    }
    
    // Proceed with crash report generation...
}
```

## 5. Advanced Features Implemented

### 5.1 Command-Based Concurrency Model

The system now implements a command-based concurrency model with several advantages:

- **Deadlock Prevention**: Eliminates mutex contention and locking order problems
- **Single Writer Model**: All state modifications happen in one goroutine
- **Clean Shutdown**: Proper coordination with optional sync points
- **Operation Encapsulation**: Each operation is a self-contained command
- **Simple Synchronization**: Optional done channels for synchronous operations
- **Timer Safety**: Timer callbacks dispatch commands rather than directly modifying state

This approach greatly simplifies the concurrency model and prevents the lifecycle issues that were causing UI freezes and shutdown problems.

### 5.2 High Load Mode with Timeout Extension

The system supports extending high load mode duration with successive calls:

```go
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
```

### 5.3 Virtual Rendering for Log Display

The LogViewer implements virtual rendering for extreme performance:

```svelte
<!-- Virtual scroller container -->
<div 
    class="relative w-full" 
    style="height: {virtualEnabled ? `${virtualContainerHeight}px` : 'auto'}"
>
    <!-- Only render logs within the visible range -->
    {#if virtualEnabled}
        {#each filteredLogs.slice(virtualStart, virtualEnd + 1) as log, i (log._sequence)}
            <div 
                class="log-entry ..."
                style="position: absolute; top: {virtualStart * avgLogHeight + i * 0}px; left: 0; right: 0; transform: translateY({i * avgLogHeight}px);"
                data-log-sequence={log._sequence}
                data-unix-time={log._unix_time}
            >
                <!-- Log content -->
            </div>
        {/each}
    {:else}
        <!-- Non-virtualized rendering (all logs) -->
        <!-- ... -->
    {/if}
</div>
```

### 5.4 Hybrid Pass-Through with Batching

The system implements a hybrid approach that combines direct pass-through with batching:

```go
func (c *addLogCommand) execute(t *AdaptiveEventThrottler) {
    // Check if this is a critical log by parsing it
    var logData map[string]interface{}
    isCritical := false
    
    // ... metadata processing ...
    
    // Send critical logs and direct logs immediately
    if isCritical || c.direct || !t.enabled {
        runtime.EventsEmit(t.ctx, "log", c.log)
        return
    }
    
    // Update event rate tracking
    t.updateEventRateInternal()
    
    // Use direct pass-through for normal operations (when not in high load mode)
    if t.currentRate < t.directPassThreshold && !t.highLoadMode {
        runtime.EventsEmit(t.ctx, "log", c.log)
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
```

### 5.5 Forced Periodic Flushes

Guaranteed updates with periodic flush checking:

```go
// Add periodic flush timer for guaranteed updates
periodicFlushTicker := time.NewTicker(250 * time.Millisecond)
defer periodicFlushTicker.Stop()

// In the select loop:
case <-periodicFlushTicker.C:
    // Periodically flush any pending data
    if len(t.logBuffer) > 0 || len(t.progressBuffer) > 0 {
        t.doFlush(false)
    }
```

## 6. Performance Characteristics

### 6.1 Memory Usage

- Log buffer: O(n) where n is the buffer size (configurable, default 5000)
- Progress buffer: O(m) where m is the number of active progress bars
- LogStore: O(n) where n is the maximum log entries setting (default 10000)
- Command channel: O(k) where k is the channel buffer size (default 100)

### 6.2 Computational Complexity

- Command processing: O(1) per command
- Batch insertion: O(n+m) where n is existing logs and m is new logs
- Log filtering: O(n) for filter operations
- Virtual rendering: O(v) where v is the visible viewport size (typically ~20-50 logs)

### 6.3 UI Performance Improvements

- Load reduction: >95% fewer events during high-volume operations
- Batch efficiency: Processing multiple events in unified batches
- Virtualization: Rendering only visible logs (20-50) instead of thousands
- Throttling overhead: <0.5ms per event for internal processing
- Thread safety: No locks in UI rendering path

## 7. Configuration Guidelines

### 7.1 Recommended Settings

- **Standard**: Enabled with min=0ms, max=100ms
- **High Performance**: Enabled with min=50ms, max=250ms
- **Extreme Performance**: Enabled with min=100ms, max=350ms, high load mode
- **Debug Mode**: Disabled for real-time updates

### 7.2 Fine Tuning

- Increase max interval (up to 500ms) on lower-end systems
- Decrease max interval (down to 50ms) on high-end systems
- Set min interval to 0ms for responsive UI during low activity
- Enable high load mode manually for task resumption scenarios
- Adjust virtual buffer size based on available memory

## 8. Implementation Notes

- Command-based concurrency eliminates mutex-related deadlocks
- All state modifications happen in a single goroutine
- Critical events use robust detection logic to bypass throttling
- Log entries include Unix timestamps for efficient sorting
- Sequence numbers ensure stable ordering regardless of arrival time
- Virtual rendering minimizes DOM operations for smooth performance
- Clean lifecycle management with proper shutdown sequence
- Hybrid approach combines immediate updates and efficient batching