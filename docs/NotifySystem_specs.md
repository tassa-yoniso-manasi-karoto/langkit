# Langkit Notification System Architecture

This document provides a comprehensive guide to the notification and display system in Langkit. The system consists of several interconnected components that handle error reporting, log management, progress tracking, and user notifications.

## 1. System Components Overview

### 1.1 Core Components

1. **Log Store (`logStore`):**
   - Central repository for all application logs
   - Categorizes logs by level (DEBUG, INFO, WARN, ERROR)
   - Tracks special log behaviors (abort_task, abort_all, user_cancel, probe)
   - Uses efficient merge-sort algorithms for chronological ordering
   - Supports batch processing for high-volume scenarios
   - Preserves all logs without capping by quantity

2. **Error Store (`invalidationErrorStore`):**
   - Manages application errors and warnings
   - Supports different error severities (critical, warning, info)
   - Provides auto-dismissal based on severity
   - Allows action handlers for error resolution

3. **Progress Bars Store (`progressBarsStore`):**
   - Tracks multiple concurrent process progress states
   - Supports error states for individual progress bars
   - Handles priority-based error state propagation
   - Provides prioritized sorting of progress indicators

4. **Settings Store (`settings`):**
   - Stores user preferences and application settings
   - Contains internal counters like `appStartCount`
   - Tracks user interaction history (`hasSeenLogViewerTooltip`)

### 1.2 UI Components

1. **LogViewer:**
   - Displays filtered log entries with color-coded styling
   - Supports both virtualized and non-virtualized rendering modes
   - Auto-toggles virtualization based on log volume
   - Provides intelligent auto-scroll with processing awareness
   - Handles animations and visual effects for new log entries
   - Uses individual log height measurements for precise positioning
   - Receives `isProcessing` state for smart post-processing behavior

2. **LogViewerNotification:**
   - Appears above the LogViewer toggle button
   - Shows different messages for processing vs. errors
   - Displays detailed error counts by type
   - Employs elegant transitions and animations
   - Provides direct access to the LogViewer

3. **Progress Manager:**
   - Visualizes ongoing processes with animated progress bars
   - Shows error states with dedicated gradients and animations
   - Supports collapsing/expanding for space efficiency
   - Auto-cleans completed progress indicators
   - Provides high-level status summary of processing state

4. **Process Error Tooltip:**
   - Displays actionable error messages
   - Provides context-specific actions
   - Uses animations and visual feedback for user interaction
   - Organizes errors into dismissible groups

## 2. Data Flow and Interactions

### 2.1 Log Event Flow

```
Backend → EventsOn("log"/"log-batch") → logStore.addLog()/addLogBatch() → UI Components
```

1. Backend sends log events (single or batched)
2. App.svelte captures events and delegates to appropriate log store methods
3. logStore processes, sorts, and stores logs chronologically 
4. UI components reactively update:
   - LogViewer shows entries with virtualization as needed
   - LogViewerNotification detects specific error types
   - Log button shows error indicator when needed

### 2.2 Processing State Flow

```
App Component → isProcessing prop → LogViewer → Post-Processing Behavior
```

1. App maintains global processing state
2. LogViewer receives processing state via props
3. LogViewer monitors transitions between processing states:
   - Active processing → scroll normally during log reception
   - Processing end → schedule staggered post-processing scrolls
   - User scroll up → disable auto-scroll and preserve position
4. Post-processing scrolls ensure auto-scroll behavior functions after processing ends

### 2.3 Progress Tracking Flow

```
Backend → EventsOn("progress"/"progress-batch") → progressBarsStore → UI Components
```

1. Backend sends progress events with numeric values and IDs
2. App.svelte captures and batches events for efficiency
3. progressBarsStore updates with error state priority enforcement
4. UI components reactively update:
   - Progress Manager shows visual progress with animations
   - Error indicators show with appropriate styling on failure

### 2.4 Error Handling Flow

```
Error Detection → invalidationErrorStore.addError() → UI Components
```

1. Errors detected from:
   - API responses
   - Log messages with ERROR level and specific behaviors
   - Progress failures
2. invalidationErrorStore categorizes and stores errors
3. UI components reactively update:
   - Process Error Tooltip shows error messages
   - LogViewerNotification shows error counts
   - Progress Manager reflects error states visually

## 3. Color System and Theme Variables

### 3.1 HSL Variables for Consistent Theming

The system uses HSL color variables defined in multiple layers:

1. **Tailwind Config (Base Definitions):**
   ```javascript
   // Base colors
   const primaryHue = 261;
   const primarySaturation = '90%';
   const primaryLightness = '70%';
   
   // Error state colors
   const errorTaskHue = 50;  // Yellow for task errors
   const errorAllHue = 0;    // Red for critical errors
   const userCancelHue = 220; // Blue-gray for cancellations
   ```

2. **CSS Variables (App-wide Access):**
   ```css
   :root {
     /* Core color variables */
     --primary-hue: 261;
     --primary-saturation: 90%;
     --primary-lightness: 70%;
     
     /* Error state colors */
     --error-soft-hue: 50;
     --error-soft-saturation: 90%;
     --error-soft-lightness: 75%;
     
     --error-hard-hue: 0;
     --error-hard-saturation: 85%;
     --error-hard-lightness: 60%;
     
     --user-cancel-hue: 220;
     --user-cancel-saturation: 10%;
     --user-cancel-lightness: 45%;
   }
   ```

3. **Gradient Definitions:**
   ```css
   :root {
     /* Error gradient definitions */
     --error-soft-gradient: linear-gradient(to right, 
         hsl(45, 100%, 60%), /* Bright yellow/orange */
         hsl(30, 100%, 50%)  /* Deep orange */
     );

     --error-hard-gradient: linear-gradient(to right, 
         hsl(323, 85%, 30%),   /* Deep pink */
         hsl(355, 90%, 45%)    /* Vibrant crimson */
     );

     --user-cancel-gradient: linear-gradient(to right, 
       hsl(220, 15%, 40%),  /* Slate blue-gray */
       hsl(210, 20%, 50%)   /* Lighter blue-gray */
     );
   }
   ```

### 3.2 Behavior Color Mapping

| Behavior    | Color         | Variable Base        | Usage                            |
|-------------|---------------|----------------------|----------------------------------|
| abort_task  | Yellow        | --error-soft-*       | Task-specific failures           |
| abort_all   | Red           | --error-hard-*        | Critical/global failures         |
| user_cancel | Gray-blue     | --user-cancel-*      | User-initiated cancellations     |
| probe       | Yellow        | --error-soft-*       | Non-critical warnings            |

## 4. LogViewer Component Details

### 4.1 Core Functionality

- **Log Filtering:** Filters logs by level (DEBUG, INFO, WARN, ERROR)
- **Auto-scroll:** Features an intelligent system that:
  - Tracks user versus programmatic scrolling
  - Detects deliberate user scroll-up to disable auto-scroll
  - Performs strategic post-processing scrolls when processing ends
  - Uses multiple scroll strategies based on context
- **Virtualization:** Implements adaptive virtualization that:
  - Auto-enables when log count exceeds threshold
  - Measures individual log heights for precise rendering
  - Renders only visible logs plus buffer for performance
  - Preserves scroll position during filter changes
- **Animation System:** Provides visual feedback with:
  - Entry animations for new logs
  - Level-specific flash animations
  - Transition handling for smooth UI updates

### 4.2 Processing-Aware Auto-Scroll

The LogViewer now uses the `isProcessing` prop to implement an enhanced auto-scroll system:

```typescript
// Monitor isProcessing changes
$: {
    if (isProcessing !== prevIsProcessing) {
        // When processing ends, schedule final scroll checks
        if (!isProcessing && prevIsProcessing && autoScroll) {
            schedulePostProcessingScrolls();
        }
        prevIsProcessing = isProcessing;
    }
}

// Schedule scroll checks after processing completes
function schedulePostProcessingScrolls() {
    // Use staggered timing to catch all rendering phases
    const checkTimes = [100, 300, 600, 1000, 1500];
    
    checkTimes.forEach((delay, index) => {
        const timerId = window.setTimeout(() => {
            if (autoScroll && !isUserScrolling) {
                executeScrollToBottom(index === checkTimes.length - 1);
            }
        }, delay);
        postProcessingTimers.push(timerId);
    });
}
```

This ensures that auto-scroll continues working after processing completes, while still respecting user scrolling preferences.

### 4.3 Virtualization System

The LogViewer implements an adaptive virtualization system that:

1. **Measures Individual Log Heights:**
   ```typescript
   function measureLogEntry(node: HTMLElement, log: LogMessage) {
       const sequence = log._sequence || 0;
       const resizeObserver = new ResizeObserver(entries => {
           const rect = node.getBoundingClientRect();
           const height = Math.max(Math.ceil(rect.height), 20) + POSITION_BUFFER;
           // Update height in tracking map
           logHeights.set(sequence, height);
           // Recalculate positions as needed
       });
       resizeObserver.observe(node);
   }
   ```

2. **Calculates Precise Log Positions:**
   ```typescript
   function recalculatePositions(): void {
       let currentPosition = 0;
       totalLogHeight = 0;
       
       // Calculate positions for filteredLogs
       for (const log of filteredLogs) {
           const sequence = log._sequence || 0;
           logPositions.set(sequence, currentPosition);
           
           // Use actual height if measured, otherwise use average
           const height = logHeights.get(sequence) || avgLogHeight + POSITION_BUFFER;
           currentPosition += height;
           totalLogHeight += height;
       }
       
       // Update container height
       virtualContainerHeight = totalLogHeight;
   }
   ```

3. **Auto-Toggles Based on Log Volume:**
   ```typescript
   $: {
       if (!manualVirtualToggle) {
           const shouldVirtualize = $logStore.exceededMaxEntries;
           
           if (shouldVirtualize !== virtualEnabled) {
               virtualEnabled = shouldVirtualize;
               // Reset virtualization when toggling
               if (virtualEnabled) {
                   setTimeout(() => {
                       resetVirtualization();
                   }, 50);
               }
           }
       }
   }
   ```

### 4.4 Styling System

```css
/* Examples of the styling system */

/* Log levels with enhanced visual treatment */
.log-level-debug {
    text-shadow: 0 0 6px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4);
    font-weight: 500;
}

.log-level-error {
    text-shadow: 0 0 8px rgba(239, 68, 68, 0.5);
    font-weight: 700;
    letter-spacing: 0.5px;
}

/* Behavior-specific styling */
.log-behavior-abort-task {
    background: linear-gradient(
        to right,
        hsla(var(--error-soft-hue), var(--error-soft-saturation), var(--error-soft-lightness), 0.08) 0%,
        rgba(0, 0, 0, 0) 70%
    );
    border-left: 2px solid hsla(var(--error-soft-hue), var(--error-soft-saturation), var(--error-soft-lightness), 0.6);
}

/* Animation for new logs */
.new-log {
    animation: slideUpEffect 0.3s ease-out;
}

@keyframes slideUpEffect {
    0% {
        transform: translateY(5px);
        opacity: 0.6;
    }
    100% {
        transform: translateY(0);
        opacity: 1;
    }
}
```

## 5. Progress Manager Component Details

### 5.1 Progress Visualization

The ProgressManager visualizes ongoing processes with:

1. **Animated Progress Bars:**
   ```css
   /* Progress bar animations */
   @keyframes sweep {
       0% { 
           transform: translateX(-100%);
           animation-timing-function: cubic-bezier(0.45, 0.3, 0.45, 0.7);
       }
       50% { 
           transform: translateX(-50%);
           animation-timing-function: cubic-bezier(0.4, 0, 0.6, 0.8);
       }
       100% { 
           transform: translateX(100%);
       }
   }
   ```

2. **Error State Visualization:**
   ```html
   <!-- Error task gradient - orange/yellow -->
   <div class="absolute inset-0 layer-error-soft animate-fade-in"></div>
   
   <!-- Error all gradient - red -->
   <div class="absolute inset-0 layer-error-hard animate-fade-in"></div>
   
   <!-- User cancel gradient - gray/blue -->
   <div class="absolute inset-0 layer-user-cancel animate-fade-in"></div>
   ```

3. **Status Text with Gradient Effects:**
   ```css
   .gradient-text-task {
       position: relative;
       color: transparent;
       background: var(--error-soft-gradient);
       -webkit-background-clip: text;
       background-clip: text;
       transition: background var(--error-transition-duration) ease-in-out,
                   color var(--error-transition-duration) ease-in-out;
   }
   ```

### 5.2 Error Handling

The ProgressManager handles errors from logs with the following priorities:

1. **User Cancellation** - Highest priority, immediately shown
2. **Global Aborts** - Critical errors that affect the entire process
3. **Task Aborts** - Errors that affect specific tasks but allow others to continue

```typescript
function handleLogBehavior(log: LogMessage) {
    const taskId = log.task_id || '';
    const isUserCancelled = log.message && log.message.toLowerCase().includes("canceled");
    
    // Check for user cancellation FIRST
    if (isUserCancelled) {
        $progressBars.forEach(bar => {
            updateErrorStateForTask(bar.id, 'user_cancel');
        });
        statusText = "Processing canceled by user";
        return;
    }
    
    // Handle abort_task
    if (log.behavior === 'abort_task') {
        taskErrors.set(targetTaskId, log.message);
        abortedTasksCount++;
        
        $progressBars.forEach(bar => {
            updateErrorStateForTask(bar.id, 'abort_task');
        });
        
        statusText = `Continuing with errors (${abortedTasksCount} ${abortedTasksCount === 1 ? 'task' : 'tasks'})`;
    } 
    // Handle abort_all
    else if (log.behavior === 'abort_all') {
        isGlobalAbort = true;
        
        $progressBars.forEach(bar => {
            updateErrorStateForTask(bar.id, 'abort_all');
        });
        
        statusText = "Processing aborted due to critical error";
    }
}
```

## 6. Implementation Details and Best Practices

### 6.1 Log Behavior Handling

- **User Cancellations:** Logs with `user_cancel` behavior should NOT be treated as errors
  ```typescript
  errorLevelLogs = logs.filter(log => 
      log.level.toUpperCase() === 'ERROR' && 
      (!log.behavior || log.behavior !== 'user_cancel') &&
      (!log.message || !log.message.toLowerCase().includes('cancel'))
  );
  ```

- **Abort Task vs. Abort All:** 
  - `abort_task` affects specific tasks but allows others to continue
  - `abort_all` is a more severe global error that halts all processing

- **Prioritized Error States:**
  ```typescript
  // Priority rules for error states:
  // 1. abort_all (error_all) always overrides anything
  // 2. abort_task (error_task) can be overridden by abort_all but not by regular updates
  // 3. user_cancel can be overridden by abort_all but not by regular updates
  ```

### 6.2 Auto-Scroll Best Practices

1. **Respect User Intent:**
   - Disable auto-scroll when user deliberately scrolls up
   - Re-enable auto-scroll when user scrolls to bottom
   - Preserve scroll position during log filtering

2. **Handle Processing Transitions:**
   - Use the `isProcessing` prop to detect when processing ends
   - Schedule staggered post-processing scrolls to handle final logs
   - Force scroll at end of processing to ensure visibility

3. **Animation Awareness:**
   - Defer scrolling during animations to prevent visual glitches
   - Use a pending scroll flag for requests during animations
   - Execute pending scrolls after animations complete

### 6.3 Virtualization Best Practices

1. **Performance Optimization:**
   - Only render logs visible in the viewport plus a buffer
   - Measure actual log heights for precise positioning
   - Use binary search to find logs at specific scroll positions

2. **User Experience:**
   - Auto-toggle virtualization based on log volume
   - Allow manual override for development testing
   - Maintain scroll anchoring during virtualization changes

3. **DOM Efficiency:**
   - Use absolute positioning for virtualized logs
   - Batch DOM updates for improved performance
   - Use RequestAnimationFrame for scroll-related updates

## 7. Critical Rules Summary

1. **User Cancellations:**
   - NEVER treat user cancellations as errors
   - Logs with `user_cancel` behavior should be visually distinct

2. **Color System:**
   - ALWAYS use HSL variables for consistency
   - Add new colors through the tailwind.config.js → app.css → component pipeline

3. **Error Detection:**
   - CHECK both log level AND behavior for proper error handling
   - Filter out cancellation-related messages in error detection

4. **Processing Awareness:**
   - ALWAYS pass isProcessing to LogViewer for proper auto-scroll behavior
   - Implement post-processing scrolls to handle end-of-processing logs

5. **Component Styling:**
   - MAINTAIN glassmorphic styling across components
   - Preserve the special glow effect on LogViewer's right and bottom edge

6. **Virtualization Control:**
   - AUTO-TOGGLE virtualization based on log volume
   - MEASURE log heights accurately for stable rendering

## 8. Future Considerations

1. **Search and Filtering:**
   - Implement search within logs
   - Add custom filtering by multiple criteria

2. **Log Export:**
   - Allow exporting filtered logs
   - Support multiple formats (JSON, TXT)

3. **Improved Error Context:**
   - Expand log context for errors
   - Link related logs for better debugging

4. **Enhanced Analytics:**
   - Track error patterns
   - Provide error frequency analytics

5. **Performance Optimizations:**
   - Further optimize virtualization for extreme log volumes
   - Implement worker-based log processing for UI thread relief