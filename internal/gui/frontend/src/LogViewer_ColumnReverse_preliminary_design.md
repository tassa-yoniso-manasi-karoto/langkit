# Preliminary Design Specification: LogViewer Scrolling System with flex-direction: column-reverse CSS property and WebAssembly Integration

## 1. Core Design Principles

### 1.1 Guiding Values
- **User Control Primacy**: User-initiated actions must always take precedence over automated behaviors
- **Predictable Behavior**: The system should behave consistently across browsers and user scenarios
- **Visual Stability**: Content should not unexpectedly shift or jump during user reading
- **Performance Efficiency**: Implementation must remain responsive even with large log volumes
- **Graceful Degradation**: The system should maintain core functionality across browsers and edge cases
- **Single Source of Truth**: Each state element (especially auto-scroll) must have exactly one authoritative source to prevent synchronization issues
- **Explicit State Transitions**: State changes should be deliberate, traceable, and flow through a single update path, never implicit or side-effect driven
- **Defensive Implementation**: Assume component state may be inconsistent during rapid updates and implement robust validation before critical operations
- **Progressive Enhancement**: WebAssembly optimizations should enhance existing functionality without creating dependencies, ensuring the application functions correctly when WebAssembly is unavailable
- **Risk Mitigation Priority**: Implement WebAssembly as an enhancement rather than a replacement, maintaining fallback mechanisms to TypeScript implementations

### 1.2 Architectural Philosophy
- **Hybrid Approach**: Leverage native browser behaviors where reliable, implement explicit controls where needed
- **Separation of Concerns**: Clearly delineate between auto-scroll logic, viewport anchoring, and user input handling
- **Defensive Implementation**: Anticipate and gracefully handle race conditions, rapid state changes, and edge cases
- **Clear State Management**: Maintain unambiguous internal state that accurately reflects the visual representation
- **State Isolation**: UI state (e.g., checkbox visibility) should be decoupled from behavioral state (e.g., auto-scroll functionality)
- **Command-Based Updates**: Use a command pattern for critical state changes to ensure proper sequencing
- **Reactive Statement Discipline**: Exercise extreme caution with reactive statements (`$:`) that modify scroll position or DOM state
- **Optimized Performance Path**: Delegate performance-critical operations to WebAssembly when appropriate, with automatic TypeScript fallbacks when necessary

## 2. Visual Representations

### Figure 1: Column-Reverse Coordinate System

```
┌───────────────────────────────┐  ◄── Top of Visual Display
│                               │      (scrollTop = maxScrollTop)
│  ┌───────────────────────┐    │
│  │ Older Log Entry #1    │    │
│  └───────────────────────┘    │
│                               │
│  ┌───────────────────────┐    │
│  │ Older Log Entry #2    │    │      DOM Order: Bottom-to-Top
│  └───────────────────────┘    │      Visual Order: Top-to-Bottom
│                               │
│  ┌───────────────────────┐    │      scrollTop increases ▲
│  │ Newer Log Entry #N-1  │    │      as you scroll UP to older logs
│  └───────────────────────┘    │
│                               │
│  ┌───────────────────────┐    │
│  │ Newest Log Entry #N   │    │
│  └───────────────────────┘    │
│                               │
└───────────────────────────────┘  ◄── Bottom of Visual Display
                                       (scrollTop = 0)
```

**Key Characteristics**:
- **DOM Structure**: In column-reverse, elements are added at the beginning of the container but appear visually at the bottom.
- **scrollTop = 0**: Corresponds to viewing the newest logs (visual bottom).
- **scrollTop > 0**: Corresponds to scrolling up to view older logs.
- **Coordinate Transformation**: When calculations designed for standard layouts are used with column-reverse, they must transform coordinates:
  - `scrollPositionFromTop = totalHeight - clientHeight - scrollTop`

### Figure 2: Auto-Scroll State Transitions

```
                          ┌────────────────────────────────┐
                          │     Auto-Scroll Enabled        │
                          │ (Newest Logs Visible, VAS Off) │
                          └──────────────┬─────────────────┘
                                         │
                ┌────────────────────────┼────────────────────────┐
                │                        │                        │
                ▼                        ▼                        ▼
┌────────────────────────┐    ┌─────────────────────┐    ┌────────────────────────┐
│ User Scrolls Away From │    │ User Toggles        │    │ Log Updates Arrive     │
│ Bottom (scrollTop > 0) │    │ Checkbox OFF        │    │                        │
└───────────┬────────────┘    └──────────┬──────────┘    └────────────┬───────────┘
            │                            │                            │
            ▼                            ▼                            ▼
┌────────────────────────┐    ┌─────────────────────┐    ┌────────────────────────┐
│ Auto-Scroll Disabled   │    │ Auto-Scroll Disabled│    │ Browser Maintains      │
│ VAS Enabled            │    │ VAS Enabled         │    │ Position at Bottom     │
│ Save Anchor Position   │    │ Save Anchor Position│    │ scrollTop = 0          │
└───────────┬────────────┘    └──────────┬──────────┘    └────────────────────────┘
            │                            │
            └────────────────┬───────────┘
                             │
                             ▼
        ┌─────────────────────────────────────────┐
        │                                         │
        │          Auto-Scroll Disabled           │
        │        (Stable View, VAS Active)        │
        │                                         │
        └───────────────────┬─────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌────────────────┐  ┌──────────────────┐  ┌────────────────────┐
│ User Scrolls To │  │ User Toggles     │  │ Log Updates Arrive │
│ Bottom          │  │ Checkbox ON      │  │                    │
└───────┬─────────┘  └────────┬─────────┘  └──────────┬─────────┘
        │                     │                       │
        │                     │                       │
        ▼                     ▼                       ▼
┌────────────────┐  ┌──────────────────┐  ┌────────────────────┐
│ Option 1: Stay │  │ Auto-Scroll ON   │  │ VAS Restores       │
│ in OFF state   │  │ VAS Disabled     │  │ Anchor Position    │
│ (Default)      │  │ Scroll to Bottom │  │ View Stays Stable  │
└────────────────┘  └──────────────────┘  └────────────────────┘
```

**Key States and Transitions**:
- When auto-scroll is **ON**: VAS is disabled; browser naturally keeps newest logs visible
- When auto-scroll is **OFF**: VAS is enabled to maintain stable view position as logs are added
- State transitions are initiated by: user scrolling, checkbox toggling, and content updates
- Each transition involves proper coordination of auto-scroll state and VAS behavior

### Figure 3: Component Interaction Diagram

```
┌─────────────────┐     ┌───────────────────┐     ┌───────────────────┐
│                 │     │                   │     │                   │
│  User Interface │     │  Svelte Component │     │  Backend Systems  │
│                 │     │                   │     │                   │
└───────┬─────────┘     └─────────┬─────────┘     └─────────┬─────────┘
        │                         │                         │
        │ ┌───────────────────────┴───────────────────┐    │
        │ │                                           │    │
        │ │         AutoScroll State (Boolean)        │◄───┘
        │ │                                           │    Event Throttling System
        │ └───┬───────────────────────────────┬───────┘    sends batched logs
        │     │                               │            via "log-batch" events
┌───────▼─────▼───┐                   ┌───────▼───────────┐
│               │                     │                   │
│ Checkbox UI   │                     │  Viewport         │
│               │◄────setAutoScroll───│  Anchoring        │
└───────┬───────┘                     │  System           │
        │                             │                   │
        │                             └─────────┬─────────┘
        │                                       │
        │                                       │
┌───────▼───────────────────────────────────────▼─────────┐
│                                                         │
│                    Scroll Container                     │
│                                                         │
└─────────────────────────────┬───────────────────────────┘
                              │
                              │
                 ┌────────────▼────────────┐
                 │                         │
                 │      DOM Events         │
                 │  (scroll, resize, etc)  │
                 │                         │
                 └─────────────────────────┘
```

**Key Interactions**:
- **Central State**: The `autoScroll` boolean is the single source of truth
- **Backend Integration**: The existing Adaptive Event Throttling System in the backend batches logs and sends them to the frontend via `log-batch` events
- **Coordination**: All components reference the central `autoScroll` state for decisions
- **User Input**: UI interactions (checkbox, scrolling) affect the central state through the `setAutoScroll` function
- **VAS Behavior**: Viewport Anchoring System is conditionally active based on `autoScroll` state

## 3. System Components & Relationships

### 3.1 Core Conceptual Model
- **Auto-Scroll Mode**: A binary state determining whether the view should automatically follow newest logs
- **Viewport Anchoring System (VAS)**: A position preservation mechanism that maintains stable viewing experience
- **Scroll Event Management**: A system for differentiating between user and programmatic scroll events
- **Virtualization Integration**: Specialized behavior modifications when dealing with virtualized content
- **WebAssembly Integration**: Performance optimization layer for computationally intensive operations with automatic fallbacks

### 3.2 State Management Architecture
- **Primary Control State**: `autoScroll` boolean - the single source of truth for tracking mode
- **Centralized State Modification**: `setAutoScroll()` function as the only method to modify auto-scroll state
- **Internal Operational Flags**:
  - User interaction tracking (e.g., active scrolling, programmatic operations)
  - Measurement and calculation coordination
  - Position anchoring data structures
  - Timing and debounce controls
  - Manual scroll locks to prevent state fighting
  - Animation tracking to coordinate transitions

### 3.3 System Interactions & Dependencies
- **UI → State**: User interactions with checkbox directly affect the `autoScroll` state through the central setter function
- **State → Behavior**: `autoScroll` state determines whether VAS is active and how scroll positions are maintained
- **Logs → Position**: Log additions trigger a position preservation flow depending on `autoScroll` state
- **Events → Flags**: Scroll, resize, and other events modify internal flags to coordinate behaviors
- **Performance → Delegation**: Log processing operations are delegated to WebAssembly or TypeScript based on performance metrics and thresholds
- **CRITICAL CAUTION**: Avoid circular dependencies where state changes trigger DOM updates that trigger further state changes

### 3.4 Integration with Existing Backend Systems

The LogViewer interacts with several backend systems, most notably the **Adaptive Event Throttling System** which already exists in the Go backend (`internal/pkg/batch/throttler.go`).

#### 3.4.1 Adaptive Event Throttling System

The backend already implements a sophisticated throttling system that:

1. **Batches Logs**: Collects multiple log entries before sending them to the frontend
2. **Adapts Frequency**: Dynamically adjusts batch frequency based on event volume
3. **Optimizes Performance**: Reduces frontend overhead during high-volume logging
4. **Maintains Order**: Preserves chronological ordering of logs

This system communicates with the frontend through `EventsOn` events, specifically:
- `log-batch`: Contains arrays of multiple log entries
- `progress-batch`: Contains progress updates (if applicable)

#### 3.4.2 Frontend Integration

The LogViewer should leverage this existing system rather than implementing its own throttling:

```javascript
// In component initialization
onMount(() => {
  // Subscribe to batched log updates
  window.go.events.on("log-batch", (batchedLogs) => {
    // Process the batch of logs from the backend throttling system
    processBatchedLogs(batchedLogs);
  });
});

// Process batched logs efficiently
function processBatchedLogs(batchedLogs) {
  // Before processing: Save viewport anchor if auto-scroll is OFF
  if (!autoScroll && !isUserScrolling) {
    saveViewportAnchor();
  }

  // Process logs through the WebAssembly-optimized merge function
  // or its TypeScript fallback for chronological ordering
  const mergedLogs = mergeInsertLogs(filteredLogs, batchedLogs);
  filteredLogs = applyCurrentFilter(mergedLogs); // Apply filtering after merging

  // After processing: Schedule scroll position management
  setTimeout(async () => {
    await tick(); // Wait for DOM update

    if (autoScroll && !isUserScrolling && !manualScrollLock && !animationInProgress) {
      scrollToBottom(); // Auto-scroll ON: Go to bottom
    } else if (!autoScroll && viewportAnchor && !isUserScrolling && !manualScrollLock && !animationInProgress) {
      restoreViewportAnchor(); // Auto-scroll OFF: Maintain position
    }
  }, 0);
}
```

#### 3.4.3 Throttling Configuration

The backend throttling system has configurable parameters that affect frontend behavior:

```go
// In internal/pkg/batch/throttler.go
type AdaptiveEventThrottler struct {
    minInterval time.Duration // Minimum time between events
    maxInterval time.Duration // Maximum time between events
    rateWindow  time.Duration // Window for measuring event frequency
    enabled     bool          // Whether throttling is active
    // ...other fields
}
```

These parameters can be adjusted either through settings in the UI or directly in the backend code:

```javascript
// Example: Setting throttling parameters from the frontend
async function updateThrottlingSettings(settings) {
  try {
    await window.go.gui.App.SetEventThrottling(settings.enabled);
    // Other parameters might need their own setters
  } catch (error) {
    console.error("Failed to update throttling settings:", error);
  }
}
```

**IMPORTANT**: The frontend should not duplicate the backend's throttling logic. Instead, it should focus on efficiently consuming and rendering the batched logs provided by the backend system.

### 3.5 WebAssembly Integration Architecture
- **Performance-Critical Functions**: Functions with high computational costs are candidates for WebAssembly optimization:
  - `mergeInsertLogs`: Log merging and chronological ordering
  - `findLogAtScrollPosition`: Virtualization scroll position calculation
  - `recalculatePositions`: Position calculations for virtualized rendering
- **Adaptive Threshold System**: Uses metrics-based decisions to determine when WebAssembly should be used:
    ```javascript
    // Function delegating to WebAssembly or TypeScript based on runtime conditions
    function shouldUseWasm(totalLogCount, operation = 'mergeInsertLogs') {
      // Basic checks first
      if (!isWasmEnabled() || !wasmModule) return false;
      if (isOperationBlacklisted(operation)) return false;

      // Honor forced mode settings
      const settings = get(settingsStore);
      if (settings.forceWasmMode === 'enabled') return true;
      if (settings.forceWasmMode === 'disabled') return false;

      // For auto mode, use threshold-based decision
      const threshold = getOperationThreshold(operation);

      // Check memory availability before using WebAssembly
      if (totalLogCount > threshold) {
        const memoryAvailable = checkMemoryAvailability(totalLogCount);
        if (!memoryAvailable) return false;

        return true;
      }

      // Default to TypeScript for small logs
      return false;
    }
    ```
- **Error Recovery System**: Automatically falls back to TypeScript implementations when WebAssembly encounters errors
- **Memory Management**: Monitors WebAssembly memory usage to prevent out-of-memory conditions
- **Performance Metrics Collection**: Tracks execution time and speedup ratios to optimize future decisions
- **IMPORTANT**: Utilize the existing backend Adaptive Event Throttling System (from `internal/pkg/batch/throttler.go`) for batched log handling, potentially influencing how logs are passed to the frontend for processing.

### 3.6 Reactive Statement Safety
- **CAUTION**: Svelte's reactive declarations (`$:`) can create hidden dependencies and circular update patterns
- **CAVEAT**: Multiple sources observing and updating the same state can cause "reactive loops"
- **PRACTICE**: Isolate DOM state (scrollTop position) from component state (auto-scroll flag)
- **DEFENSIVE APPROACH**: Use guard flags to prevent reactive statements from triggering multiple times during a single logical update

## 4. Detailed Behavioral Specifications

### 4.1 Auto-Scroll State Transitions

#### 4.1.1 User-Initiated Mode Changes (Checkbox Toggle)
- **OFF → ON**:
  - VAS must be immediately disabled
  - Any current viewport anchor must be discarded
  - If not already at the bottom, an explicit programmatic scroll must move view to bottom
  - Subsequent log additions should maintain bottom position (primarily through browser behavior)
  - This transition should feel responsive and immediate to the user
  - **CRITICAL**: Must only update state through the centralized setter function

- **ON → OFF**:
  - VAS must be immediately enabled
  - Current viewport position must be captured as anchor reference
  - Subsequent log additions must preserve the anchored position
  - No automatic scrolling should occur while in this state
  - **CRITICAL**: Must only update state through the centralized setter function

#### 4.1.2 Implicit State Changes (User Scrolling)
- **ON → OFF (Automatic)**:
  - When user scrolls away from bottom with auto-scroll ON, the system must:
    - Automatically transition to auto-scroll OFF through the centralized setter
    - Enable VAS and capture the new position
    - Provide subtle visual feedback that auto-scroll has been disabled
    - Ensure checkbox UI reflects the new state
    - **SAFETY MEASURE**: Apply manual "guard flags" to prevent recursive updates:
            ```javascript
            let isUserScrolling = false;
            let manualScrollLock = false;
            let scrollGuardTimer = null;

            function handleScroll() {
              // Skip if programmatic scrolling
              if (isProgrammaticScroll) return;

              // Set guard flags IMMEDIATELY to prevent recursive updates
              isUserScrolling = true;
              manualScrollLock = true;

              // Clear guards after sufficient delay
              if (scrollGuardTimer) clearTimeout(scrollGuardTimer);
              scrollGuardTimer = setTimeout(() => {
                manualScrollLock = false;
                isUserScrolling = false; // Might be better handled by scroll end detection
              }, 1000); // Longer timeout for reliable user reading

              // Continue with scroll handling...
            }
            ```

- **OFF → ON (Optional Consideration)**:
  - When user manually scrolls to bottom with auto-scroll OFF:
    - *Option 1*: Maintain OFF state (requires explicit user checkbox action)
    - *Option 2*: Automatically re-enable auto-scroll (more automated but potentially unexpected)
    - Design recommendation: Implement Option 1 for predictability, with clear visual cues to re-enable
    - **CRITICAL**: If implementing Option 2, apply a subtle timing delay to avoid accidental triggers (as shown in `handleScroll` implementation example `9.2`)

### 4.2 Viewport Anchoring System (VAS) Behavior

#### 4.2.1 Fundamental Operation
- **When Active**: Only when auto-scroll is OFF
- **Purpose**: Maintain stable viewing position during log additions and container changes
- **Core Process**:
    1.  Before DOM updates: Capture position reference relative to a stable element
    2.  After DOM updates: Calculate new position and restore viewport to equivalent position
    3.  Apply position preservation only when appropriate (not during user scrolling)
- **CRITICAL RISK**: In a column-reverse layout, unconditional viewport restoration can create unintended auto-scroll behavior
- **MITIGATION**: Explicitly check auto-scroll state before applying viewport anchoring:
    ```javascript
    // SAFER APPROACH (Illustrative reactive block)
    $: if (filteredLogs.length > 0 && scrollContainer) {
      // Schedule post-update actions
      setTimeout(async () => {
          await tick(); // Wait for DOM update

          // Re-check state *after* tick, as it could have changed
          if (autoScroll) {
            // Use direct approach for auto-scroll ON
            viewportAnchor = null; // Clear any anchor
            if (!isUserScrolling && !manualScrollLock && !animationInProgress) {
              scrollToBottom(); // Simple direct scroll (programmatically safe)
            }
          } else {
            // Use viewport anchoring only when auto-scroll is OFF
            // Save happens *before* DOM update trigger (e.g., before log processing)
            // saveViewportAnchor(); was called earlier
            // Restore happens *after* DOM update
            if (viewportAnchor && !isUserScrolling && !manualScrollLock && !animationInProgress) {
              restoreViewportAnchor();
            }
          }
      }, 0);
    }
    ```

#### 4.2.2 Anchor Selection Strategy
- **Primary Strategy**: Anchor to visible log entry near viewport center
- **Alternative Strategy**: Use scroll percentage or offset from top/bottom when specific elements aren't reliable
- **Fallback Mechanism**: When anchor elements are removed (filtering, virtualization), recalculate based on nearby elements
- **TIMING CRITICAL**: Always use `await tick()` before measuring positions to ensure DOM has updated

#### 4.2.3 Coordinate Calculations
- **Column-Reverse Transformation**: All position calculations must account for inverted coordinate system
- **Precision Considerations**: Use tolerance values (±1-2px) for position comparisons to account for rounding and subpixel rendering
- **Boundary Handling**: Ensure calculated positions remain within valid scroll range, particularly near content boundaries
- **WebAssembly Coordination**: For WASM-optimized functions (like those potentially used in `recalculatePositions` or `findLogAtScrollPosition`), transform coordinates in JavaScript *before* passing data to WebAssembly, rather than requiring Wasm to handle layout specifics:
    ```javascript
    // Example of coordinate transformation before WebAssembly delegation
    function findLogAtScrollPosition(scrollTop, scrollMetrics) {
      // In column-reverse, convert scrollTop to position from top of content
      const adjustedScrollPosition = scrollContainer ?
          (totalLogHeight - scrollContainer.clientHeight - scrollTop) :
          scrollTop;

      // Delegate to WebAssembly if appropriate
      if (shouldUseWasm(filteredLogs.length, 'findLogAtScrollPosition')) {
        try {
          // Pass the transformed coordinate to WebAssembly function
          // Example assumes Wasm function takes adjusted position directly
          return findLogAtScrollPositionWasm(
            // ... Wasm function arguments ...
            adjustedScrollPosition, // Send adjusted position
            // ... other arguments ...
          );
        } catch (error) {
          // Handle error and fall back to TypeScript implementation
          handleWasmError(error, 'findLogAtScrollPosition');
          // Continue with TypeScript implementation below...
        }
      }

      // TypeScript implementation (fallback)
      // Uses adjustedScrollPosition for calculations
      // [implementation details...]
    }
    ```

### 4.3 Log Update & Rendering Flow

#### 4.3.1 With Auto-Scroll ON
- **Expected Behavior**: View remains at newest logs (bottom)
- **Primary Mechanism**: Browser's natural tendency to maintain scrollTop=0 in column-reverse layout when content is added at the start (bottom visually).
- **Safety Mechanism**: Explicit `scrollTop=0` enforcement after updates when necessary, particularly:
  - After filtering operations
  - With virtualization enabled (where browser behavior might not suffice)
  - After significant layout changes
  - Following potential browser inconsistencies
- **TIMING CONSIDERATION**: Use `requestAnimationFrame` or `await tick()` followed by programmatic scroll for position enforcement to ensure it happens after rendering/layout.

#### 4.3.2 With Auto-Scroll OFF
- **Expected Behavior**: View maintains stable position relative to existing content
- **Primary Mechanism**: VAS captures position before update, restores equivalent position after
- **Critical Timing**: Position capture must occur *before* the DOM update begins. Position restoration must occur *after* DOM updates are complete (e.g., after `await tick()`).
- **Interference Prevention**: Skip position restoration during active user scrolling or immediately after user scroll completion (use `manualScrollLock`).
- **ANIMATION AWARENESS**: Track animation state to defer scroll operations until animations complete:
    ```javascript
    let animationInProgress = false;
    let pendingScrollToBottom = false; // Example flag for deferred operation

    function handleTransitionStart() {
      animationInProgress = true;
    }

    function handleTransitionEnd() { // Needs careful implementation, e.g., transition counter
      animationInProgress = false;

      // Execute deferred operations if conditions met
      if (pendingScrollToBottom && autoScroll && !isUserScrolling && !manualScrollLock) {
        pendingScrollToBottom = false;
        executeScrollToBottom(); // Function performing the actual scroll
      }
      // Potentially trigger anchor restoration if needed and conditions met
      // if (!autoScroll && viewportAnchor && !isUserScrolling && !manualScrollLock) { ... }
    }
    ```

## 5. Event Handling & Coordination

### 5.1 Scroll Event Management

#### 5.1.1 Event Categorization
- **User-Initiated Scrolling**: Direct interaction (mouse wheel, scrollbar drag, touch, keyboard) requiring state changes (potentially disabling auto-scroll).
- **Programmatic Scrolling**: System-initiated scroll (e.g., `scrollToBottom`, `restoreViewportAnchor`) requiring exclusion from user-scroll feedback loops.
- **Momentum/Inertial Scrolling**: Post-interaction scrolling (e.g., trackpad fling) requiring careful timing considerations for scroll end detection.
- **CRITICAL SEPARATION**: Distinguish between user and programmatic scrolling with explicit flags:
    ```javascript
    let isProgrammaticScroll = false;

    function withProgrammaticScroll(callback) {
      isProgrammaticScroll = true;
      try {
        callback();
      } finally {
        // Use RAF or setTimeout to ensure flag is cleared *after* browser processes the scroll
        requestAnimationFrame(() => {
          // Small delay might be needed if RAF isn't enough
          setTimeout(() => { isProgrammaticScroll = false; }, 0);
        });
      }
    }

    function scrollToBottom() {
      withProgrammaticScroll(() => {
        if (scrollContainer) {
          scrollContainer.scrollTop = 0; // In column-reverse, 0 is bottom
        }
      });
    }
    ```

#### 5.1.2 Scroll Cycle Behavior
- **Start**: Mark active scrolling (`isUserScrolling`), prevent competing operations (e.g., pause anchor restoration), set `manualScrollLock`.
- **During**: Update internal state (e.g., `currentScrollTop`), track direction/extent if needed, potentially trigger auto-scroll disable if scrolling away from bottom. Throttle handler execution.
- **End (Debounced/Timeout)**: Clear `isUserScrolling` flag, evaluate final position for potential state changes (e.g., re-enable auto-scroll if user scrolled back to bottom - Option 2). Clear `manualScrollLock` after a longer delay.
- **Threshold Values**: Use small tolerance (e.g., 1-2px) for "at bottom" detection (`Math.abs(scrollTop) <= tolerance`) to account for precision issues.
- **PERFORMANCE CRITICAL**: Throttle scroll handlers for better performance, typically using `requestAnimationFrame` for execution timing and a `setTimeout` for end detection:
    ```javascript
    let scrollRAF = null;
    let userScrollTimeout = null;
    let isUserScrolling = false; // Track if user is actively scrolling

    function handleScrollThrottled() { // The actual logic inside the throttle
      if (!scrollContainer) return;
      // Skip if programmatic
      if (isProgrammaticScroll) return;

      // Implement scroll logic here
      const { scrollTop } = scrollContainer;
      const absScrollTop = Math.abs(scrollTop);

      // Check auto-scroll conditions (disable if scrolling away)
      if (absScrollTop > 2 && autoScroll) { // Use a small threshold > 0
        if (debug) console.warn(`Disabling auto-scroll due to user scroll away: scrollTop=${scrollTop}px`);
        setAutoScroll(false, 'userScrollAway');
      }

      // Update scroll metrics, virtualization etc. as needed
      // updateScrollMetrics(absScrollTop);
      // ...

      // Reset the scroll end timeout
      if (userScrollTimeout) clearTimeout(userScrollTimeout);
      userScrollTimeout = setTimeout(() => {
        isUserScrolling = false;
        // Check final position after scrolling stops
        if (scrollContainer && Math.abs(scrollContainer.scrollTop) <= 1 && !autoScroll) {
           // Option 2: Consider auto-enabling when scrolled back to bottom
           // setAutoScroll(true, 'scrolledToBottom');
        }
        // Manual lock should persist longer via its own timer (see 9.2)
      }, 300); // Adjust timeout duration as needed
    }

    function handleScroll() { // The event listener attached to the element
      if (isProgrammaticScroll) return;

      // Mark user scrolling immediately
      isUserScrolling = true;
      manualScrollLock = true; // Set manual lock immediately (cleared by its own timer)

      // Throttle with requestAnimationFrame
      if (scrollRAF) cancelAnimationFrame(scrollRAF);
      scrollRAF = requestAnimationFrame(() => {
        handleScrollThrottled();
        scrollRAF = null;
      });

      // Reset manual lock timer (defined elsewhere, see 9.2)
      // resetManualScrollLockTimer();
    }
    ```

### 5.2 Resize Event Handling
- **Container Resizing**: Recalculate dimensions (`clientHeight`, etc.) and maintain appropriate scroll position (e.g., re-apply anchor if VAS is active, or scroll to bottom if auto-scroll is ON).
- **Window Resizing**: May trigger container resizing. Adjust virtualization parameters if applicable, while preserving view stability.
- **Content Height Changes**: Not a direct event, but occurs due to log additions/filtering/virtualization. Recalculate total height and adjust scroll position proportionally or via VAS/auto-scroll logic.
- **MEMORY MANAGEMENT**: Properly disconnect observers (`ResizeObserver`, `MutationObserver`) during component destruction to prevent memory leaks:
    ```javascript
    // Example using Svelte lifecycle
    import { onMount, onDestroy } from 'svelte';

    let resizeObserver;
    let mutationObserver; // If used

    onMount(() => {
      resizeObserver = new ResizeObserver(handleResize); // handleResize defined elsewhere
      if (scrollContainer) {
        resizeObserver.observe(scrollContainer);
      }
      // Initialize other observers if needed
    });

    onDestroy(() => {
      // Clean everything up
      if (resizeObserver) resizeObserver.disconnect();
      if (mutationObserver) mutationObserver.disconnect();
      // Clear all timers (scrollRAF, userScrollTimeout, manualScrollLockTimer, etc.)
      if (scrollRAF) cancelAnimationFrame(scrollRAF);
      if (userScrollTimeout) clearTimeout(userScrollTimeout);
      // ... other cleanup ...
    });
    ```

### 5.3 DOM Lifecycle Integration
- **Before DOM Updates**: Capture necessary state, primarily viewport position references for VAS (`saveViewportAnchor()`). This often happens just before triggering the state change that causes the update (e.g., processing new logs).
- **After DOM Updates**: Apply position restoration (`restoreViewportAnchor()`) or enforce scroll position (`scrollToBottom()`) only *after* the DOM has finished rendering the changes. Use `await tick()` in Svelte, potentially followed by `requestAnimationFrame` for positioning to ensure layout is stable.
- **Batched Operations**: Group multiple measurements or DOM manipulations where possible (e.g., using `requestAnimationFrame` or short `setTimeout` for batching) to reduce layout thrashing and improve performance.
- **ASYNCHRONOUS AWARENESS**: Always use `await tick()` in Svelte before measuring DOM elements (like `offsetTop`, `clientHeight`, `scrollHeight`) or manipulating scroll position (`scrollTop`) *after* a reactive state change that affects the DOM.
- **SVELTE REACTIVITY CAUTION**: Be aware that Svelte's reactivity updates the DOM asynchronously. Props passed down might not reflect immediately in the child component's DOM until the next tick.

## 6. Edge Cases & Robustness Measures

### 6.1 Race Conditions & Timing Issues

#### 6.1.1 Rapid Interaction Sequences
- **Rapid Checkbox Toggling**: Ensure the `setAutoScroll` function handles rapid calls gracefully, possibly debouncing slightly or ensuring the latest call takes precedence. State should only transition based on the final intended value.
- **Scrolling During Transitions**: Prioritize direct user interaction (scrolling). If a user starts scrolling while a programmatic scroll (like `scrollToBottom` after enabling auto-scroll) is happening, the user scroll should take over, and the system should react accordingly (e.g., immediately disable auto-scroll again). Guard flags (`isUserScrolling`, `manualScrollLock`) are crucial here.
- **Updates During Scrolling**: When auto-scroll is OFF and logs arrive while the user is scrolling, the VAS should ideally *not* try to restore position until the user scroll finishes. The `isUserScrolling` flag prevents `restoreViewportAnchor` from running. When auto-scroll is ON and logs arrive while the user is scrolling *away* from the bottom, auto-scroll should already be disabled by the scroll handler, so the new logs will just be added without forcing a scroll-to-bottom.
- **Concurrent Operations**: Establish clear operation precedence:
    1.  Direct User Scroll: Highest priority. Interrupts/cancels most programmatic actions. Sets `isUserScrolling` and `manualScrollLock`.
    2.  User Checkbox Toggle: High priority. Triggers `setAutoScroll` immediately.
    3.  Programmatic Scrolls (`scrollToBottom`, `restoreViewportAnchor`): Lower priority. Should check flags (`!isUserScrolling`, `!manualScrollLock`, `!animationInProgress`) before executing.
    4.  DOM Updates/Log Processing: Happen reactively, but subsequent positioning depends on flags and state.

#### 6.1.2 Animation & Transition Timing
- **CSS Transitions**: Be aware that CSS transitions on log elements (e.g., fade-in) can affect layout calculations temporarily. Measurements should ideally occur *after* transitions complete, or calculations need to account for the animated state if necessary. Often, anchoring logic is best applied after animations.
- **Svelte Animations**: Similar to CSS transitions, ensure Svelte `in:`/`out:` directives or `animate:` directives complete before relying on final element positions for critical logic like VAS restoration. Use animation/transition event listeners (`on:transitionend`, `on:animationend`) or track state via flags.
- **Browser Painting**: `requestAnimationFrame` is useful for ensuring code runs just before the browser paints, which is generally after layout calculation. Using `await tick()` followed by `requestAnimationFrame` can be a robust pattern for applying scroll changes after DOM updates.
- **TRANSITION TRACKING**: If complex animations involving multiple elements occur, use counters to track active transitions accurately:
    ```javascript
    let activeTransitions = 0;
    let animationInProgress = false; // Simplified flag derived from counter

    function handleTransitionStart() {
      activeTransitions++;
      animationInProgress = true;
    }

    function handleTransitionEnd() {
      activeTransitions--;
      if (activeTransitions < 0) activeTransitions = 0; // Safety check

      // Only set animation complete when all tracked transitions are done
      if (activeTransitions === 0) {
        animationInProgress = false;

        // Execute pending operations that were waiting for animation
        // Example: if (pendingRestoreAnchor && !autoScroll && ...) { ... }
      }
    }
    // Attach these handlers to relevant elements/transitions
    ```

### 6.2 Initialization & Edge States

#### 6.2.1 Component Initialization
- **Initial Rendering**: Establish default auto-scroll state (ON recommended for typical log viewing). `autoScroll = true;`
- **Asynchronous Loading**: Handle cases where logs arrive *after* the initial component mount. The initial `scrollToBottom` might need to happen after the first batch of logs arrives if the component mounts empty.
- **Empty State**: Manage gracefully when the log container starts empty. Scroll logic should not cause errors. `scrollHeight`, `scrollTop` etc. will be 0.
- **Cold Start**: Apply default behaviors (auto-scroll ON, scroll to bottom) after mount, potentially with a small delay or after the first log data arrives to ensure the container exists and has dimensions.
- **LIFECYCLE AWARENESS**: Handle component mounting/unmounting correctly using `onMount` and `onDestroy` for setup and cleanup.
    ```javascript
    onMount(() => {
      // Initial state setup
      // autoScroll = true; // Assuming default state

      // Initial DOM setup & scroll
      // Needs to run after container is rendered and potentially after first logs
      setTimeout(async () => {
        await tick(); // Ensure container exists in DOM
        if (scrollContainer && autoScroll && !isUserScrolling) {
          // Don't scroll if user interacted before this timeout
          scrollToBottom(); // Initial scroll to bottom
        }
        // Initialize observers, etc.
      }, 50); // Small delay can help ensure readiness
    });

    onDestroy(() => {
      // Clean up all resources: observers, timers, event listeners
      // ... (as shown in 5.2) ...
    });
    ```

#### 6.2.2 Focus & Accessibility Considerations
- **Keyboard Navigation**: Ensure standard keyboard scrolling (arrow keys, PgUp/PgDown, Home/End) works as expected. These actions count as user scrolls and should disable auto-scroll if active and moving away from the bottom.
- **Screen Readers**: Use appropriate ARIA roles (`role="log"`, `aria-live="polite"` or `assertive` depending on needs) for the log container. Ensure controls (`checkbox`, buttons) have proper labels and ARIA attributes. Manage focus appropriately, especially when new content loads or controls appear/disappear.
- **Tab Visibility**: Consider pausing intensive operations (like frequent background polling or complex rendering updates) when the browser tab is not visible using the Page Visibility API, unless required for background operation. Auto-scroll logic might not need to run if the tab is hidden.

### 6.3 Browser & Environment Variations

#### 6.3.1 Browser-Specific Behaviors
- **Scroll Position Maintenance**: Test `flex-direction: column-reverse` behavior rigorously across target browsers (Chrome, Firefox, Safari, Edge). Safari, in particular, sometimes has quirks with scroll anchoring and layout. Browser's native scroll anchoring might interfere or assist; understand its behavior.
- **Event Timing**: The exact order and timing of `scroll`, `resize`, and DOM mutation events can vary slightly between browsers. Robust logic should not depend on precise micro-timing between different event types.
- **Rendering Optimizations**: Be aware of browser-specific rendering optimizations (like content visibility) that might affect element measurements if not handled carefully.
- **BROWSER DETECTION**: Avoid browser-specific sniffing if possible. Rely on feature detection. Implement browser-specific workarounds *only* as a last resort for known, unavoidable bugs, with clear documentation and targeted application.

#### 6.3.2 Performance Degradation Scenarios
- **Large Log Volumes**: Ensure performance remains acceptable with tens or hundreds of thousands of entries. Virtualization is key here. Test Wasm thresholds and performance gains.
- **Limited Resources**: Test on lower-spec devices or use browser developer tools to simulate CPU throttling and reduced memory. Ensure the UI remains usable, even if slower. Graceful degradation might involve disabling cosmetic animations or using Wasm more conservatively.
- **Slow Connections/Bursty Logs**: Handle logs arriving infrequently or in large bursts without freezing the UI. Batch processing (via backend throttler) and asynchronous operations are important. Ensure scroll logic behaves correctly when many logs arrive at once.

### 6.4 WebAssembly-Specific Edge Cases
- **Feature Detection**: Check for WebAssembly support (`typeof WebAssembly === 'object'`) during initialization. Fall back entirely to TypeScript implementations if Wasm is not supported.
- **Module Loading Failures**: Handle errors during Wasm module fetching (`fetch`) or instantiation (`WebAssembly.instantiate`). Log the error and gracefully fall back to TypeScript, potentially disabling Wasm for the session.
- **Memory Constraints**: Monitor Wasm memory usage if performing operations that could significantly grow memory. Implement checks (`checkMemoryAvailability`) before attempting large Wasm operations. Handle potential `RangeError: WebAssembly.Memory.grow()` failures by catching the error, logging it, blacklisting the operation, and falling back to TypeScript.
- **Operation Blacklisting**: Implement a system to temporarily disable specific Wasm functions if they consistently fail or prove slower than TypeScript. Use an exponential backoff strategy for retrying blacklisted operations. Store blacklist state persistently (e.g., Session Storage) or reset it per session.
- **CRITICAL HANDLING**: Implement robust error handling around *every* call to a Wasm function. Use `try...catch` blocks.
    ```javascript
    // Example of WebAssembly error handling with blacklisting
    function handleWasmError(error, operation, context = {}) {
      const WasmLogLevel = { INFO: 0, WARNING: 1, ERROR: 2, CRITICAL: 3 }; // Example levels
      // Determine error severity based on error type or context
      const isCritical = (error instanceof RangeError); // Example: Memory errors are critical
      const logLevel = isCritical ? WasmLogLevel.CRITICAL : WasmLogLevel.ERROR;

      // Log error with context (using a dedicated logger is good practice)
      console.error(
        `Wasm Error [${operation}] (Level ${logLevel}): ${error.message}`,
        { context, error }
      );
      // wasmLogger.log(...) // Use a structured logger if available

      // Update shared Wasm error state if needed for UI feedback
      // setWasmErrorState({ lastError: error, operation });

      // Add operation to blacklist with exponential backoff
      addToOperationBlacklist(operation, error); // Implement blacklist logic separately

      // For critical errors, consider disabling WebAssembly entirely for the session
      const disableOnCritical = true; // Configuration option
      if (isCritical && disableOnCritical) {
        console.warn(`Disabling WebAssembly due to critical error in ${operation}.`);
        enableWasm(false); // Function to disable Wasm usage
      }
    }
    ```

## 7. Feature Integration Specifications

### 7.1 Virtualization Compatibility

#### 7.1.1 Auto-Scroll with Virtualization
- **Scroll Position Management**: When auto-scroll is ON, the browser's native behavior is insufficient with virtualization. After new logs are added (and potentially processed by Wasm), explicitly calculate the scroll position required to show the very latest items at the bottom (visually, so `scrollTop = 0` in column-reverse) and apply it programmatically. Ensure this happens *after* the virtualizer has updated the rendered items.
- **Render Window Adjustment**: The virtualization logic must ensure that when auto-scroll is ON, the "render window" (the subset of logs actually in the DOM) includes the newest log entries.
- **Performance Considerations**: Minimize layout thrashing during rapid updates. Ensure virtualization calculations (potentially using Wasm's `findLogAtScrollPosition` or `recalculatePositions`) are efficient.
- **STATE CONSISTENCY**: Ensure virtualization updates (changing the rendered items) don't inadvertently trigger scroll handlers that disable auto-scroll. Programmatic scrolls initiated by the virtualizer or auto-scroll logic must use the `isProgrammaticScroll` flag.

#### 7.1.2 VAS with Virtualization
- **Anchor Strategy Adaptation**: DOM element anchoring is unreliable because elements scroll out of the virtual window. Use index-based anchoring instead. Store the index of the log item the user was viewing (e.g., the one closest to the viewport center) and its offset relative to the viewport top/bottom.
- **Calculations Adjustment**: When logs are added/removed, recalculate the scroll position needed to bring the anchored log index back to its previous offset within the viewport. This involves knowing the estimated total height and the positions/heights of items (potentially calculated via Wasm `recalculatePositions`).
- **Position Estimation**: Handle gracefully when exact positions cannot be determined (e.g., using average item height for items outside the render window). Accept some potential jitter, especially with variable height content.
- **COORDINATE TRANSFORMATION**: Calculations involving `scrollTop` must consistently account for the `column-reverse` layout. When interfacing with virtualization logic or Wasm functions, ensure the coordinate system expected (e.g., pixels from top of total content vs. pixels from bottom of viewport) is clear and conversions are applied correctly in the JavaScript layer before calling Wasm.
    ```javascript
    // Example within virtualization logic using VAS data
    function restoreVirtualAnchor(anchor) { // anchor = { index: number, offset: number }
      // Calculate estimated total height (potentially with Wasm help)
      const totalHeight = estimateTotalHeight();

      // Calculate the estimated top offset of the anchor item
      // (potentially using cached/calculated positions, maybe from Wasm)
      const estimatedItemTop = estimateItemTopPosition(anchor.index);

      // Calculate target scrollTop for column-reverse
      // scrollTop = totalHeight - clientHeight - (estimatedItemTop - anchor.offset)
      // Need to adjust anchor.offset based on how it was captured (from top or bottom of viewport)
      const clientHeight = scrollContainer ? scrollContainer.clientHeight : 0;
      // Assuming offset was captured from top of viewport:
      let targetScrollTop = totalHeight - clientHeight - (estimatedItemTop - anchor.offset);


      // Clamp targetScrollTop to valid range [0, maxScrollTop]
      const maxScrollTop = Math.max(0, totalHeight - clientHeight);
      targetScrollTop = Math.max(0, Math.min(targetScrollTop, maxScrollTop));

      // Apply programmatically
      withProgrammaticScroll(() => {
        if (scrollContainer) {
          scrollContainer.scrollTop = targetScrollTop;
          // Trigger virtualization update based on new scroll position
          updateVirtualization(targetScrollTop);
        }
      });
    }
    ```

### 7.2 Log Filtering & Manipulation

#### 7.2.1 Filter Application
- **Position Handling**: When filters change, the set of visible logs changes dramatically.
  - If auto-scroll is ON: Maintain it, and scroll to the bottom (`scrollTop = 0`) of the newly filtered log set after the update.
  - If auto-scroll is OFF: Attempt to maintain the user's view using VAS. This can be challenging. Save the anchor *before* applying the filter. After filtering, try to find the anchored item (or a nearby item if the original is filtered out) in the new set and restore the view to it. If the anchor context is completely lost, falling back to scrolling to the top or bottom of the filtered view might be necessary, potentially with user notification.
- **Auto-Scroll Behavior**: The `autoScroll` state itself should persist across filter changes.
- **Anchor Invalidation**: Anchors based on specific DOM elements or indices might become invalid after filtering. The restoration logic needs to handle cases where the anchored item is no longer present.
- **CRITICAL TIMING**: Apply filtering logic, wait for DOM updates (`await tick()`), then apply scrolling/anchoring logic. Consider visual transitions for filter changes.
    ```javascript
    // Simplified reactive logic for filter change
    let previousFilter; // Store previous filter state
    $: if (currentFilter !== previousFilter) {
      let savedAnchor = null;
      if (!autoScroll) {
        savedAnchor = saveViewportAnchor(); // Save before filtering starts
      }
      previousFilter = currentFilter;

      // Apply filter logic which updates `filteredLogs` reactively...
      // updateFilteredLogs(currentFilter);

      // Schedule actions after DOM update
      setTimeout(async () => {
        await tick(); // Wait for DOM to reflect filtered logs

        if (autoScroll && !isUserScrolling && !manualScrollLock && !animationInProgress) {
          scrollToBottom();
        } else if (!autoScroll && savedAnchor) {
          // Attempt to restore anchor in the new filtered set
          restoreViewportAnchor(savedAnchor); // Needs robust logic for filtered items
        } else if (!autoScroll) {
          // Fallback if anchor is invalid or wasn't saved
          // Optionally scroll to top or do nothing
        }
      }, 0); // Use setTimeout 0 or requestAnimationFrame
    }
    ```

#### 7.2.2 Log Truncation & Clearing
- **State Preservation**: Maintain the `autoScroll` setting when logs are cleared or truncated.
- **Position Recalculation**:
  - If logs are cleared: Scroll position naturally goes to 0 (`scrollTop = 0`). If auto-scroll was OFF, it might remain OFF but the view is now empty or at the top/bottom.
  - If logs are truncated (e.g., keeping only the last N logs):
    - If auto-scroll ON: Remain at the bottom (`scrollTop = 0`).
    - If auto-scroll OFF: The current view might be removed. Use VAS to try and find a relevant position in the remaining logs. If the view was near the *end* of the truncated logs, try to maintain that relative position. If the view was in the *removed* portion, scrolling to the top (`scrollTop = scrollHeight - clientHeight`) of the remaining logs might be the most sensible default.
- **Empty State Handling**: Ensure the component functions correctly when all logs are cleared/filtered out.
- **USER NOTIFICATION**: Consider subtle visual feedback or a toast message when large-scale truncation or clearing occurs, especially if it significantly shifts the user's view when auto-scroll is OFF.

### 7.3 WebAssembly Performance Optimization

#### 7.3.1 Optimized Operations
- **Log Merging and Sorting**: Delegate `mergeInsertLogs` (or similar function handling insertion of new, potentially out-of-order logs into the existing sorted list) to WebAssembly when the number of logs involved (existing + new) exceeds a threshold (e.g., >500 logs).
- **Scroll Position Calculation**: Use WebAssembly (`findLogAtScrollPositionWasm`) for efficient searching (e.g., binary search) within large, potentially virtualized log lists to find the log index corresponding to a given `scrollTop`, especially when item heights are variable but calculable.
- **Position Recalculation**: Leverage WebAssembly (`recalculatePositionsWasm`) when virtualization requires recalculating the estimated top positions or total height of a large number of log entries, particularly after filtering or significant additions/removals.
- **ADAPTIVE THRESHOLDS**: Implement a system to dynamically adjust the log count threshold for using WebAssembly per operation, based on measured performance.
    ```javascript
    // Part of adaptive threshold system (conceptual)
    const WASM_CONFIG = { MIN_THRESHOLD: 100, MAX_THRESHOLD: 10000, DEFAULT_THRESHOLD: 500 };
    let operationThresholds = { mergeInsertLogs: WASM_CONFIG.DEFAULT_THRESHOLD, /* ... other ops */ };
    let wasmPerformanceMetrics = { operationTimings: {}, operationsCount: 0, /* ... */ };

    function getOperationThreshold(operation) { return operationThresholds[operation] || WASM_CONFIG.DEFAULT_THRESHOLD; }
    function setOperationThreshold(operation, threshold) { operationThresholds[operation] = threshold; }

    function updateOperationThresholds() {
      const metrics = wasmPerformanceMetrics; // Get current metrics
      if (!metrics.operationTimings || metrics.operationsCount < 10) return; // Need some data

      Object.entries(metrics.operationTimings).forEach(([operation, stats]) => {
        // Example stats: { count: number, wasmAvgMs: number, tsAvgMs: number }
        if (stats.count < 5 || !stats.tsAvgMs) return; // Need comparisons

        const currentThreshold = getOperationThreshold(operation);
        let newThreshold = currentThreshold;
        const operationSpeedup = stats.tsAvgMs / stats.wasmAvgMs;

        // Adjust threshold based on observed performance
        if (operationSpeedup > 2.5) { // Significant speedup? Lower threshold.
          newThreshold = Math.max(WASM_CONFIG.MIN_THRESHOLD, Math.round(currentThreshold * 0.85));
        } else if (operationSpeedup < 1.3) { // Minimal speedup? Raise threshold.
          newThreshold = Math.min(WASM_CONFIG.MAX_THRESHOLD, Math.round(currentThreshold * 1.15));
        }

        if (newThreshold !== currentThreshold) {
          console.log(`Adjusting Wasm threshold for ${operation}: ${currentThreshold} -> ${newThreshold} (Speedup: ${operationSpeedup.toFixed(1)}x)`);
          setOperationThreshold(operation, newThreshold);
        }
      });
      // Reset counters or schedule next update
    }
    // Call updateOperationThresholds periodically or after significant batches of operations
    ```

#### 7.3.2 Performance Monitoring
- **Execution Time Tracking**: Use `performance.now()` before and after both Wasm calls and their equivalent TypeScript fallback implementations (run TS version occasionally even when Wasm is used, or run both in dev mode) to gather timing data.
- **Speedup Ratio Analysis**: Calculate `tsExecutionTime / wasmExecutionTime` for specific operations and log/store these metrics. Account for data serialization/deserialization overhead for Wasm calls.
- **Memory Usage Monitoring**: If using Wasm memory features directly, monitor `WebAssembly.Memory.buffer.byteLength`. If memory grows significantly, investigate potential leaks or inefficient memory use in the Wasm module.
- **Diagnostic Reporting**: Include Wasm status (enabled, active, last error, performance metrics summary) in diagnostic bundles or crash reports sent to the backend.
- **USER FEEDBACK**: Optionally, provide subtle, non-intrusive feedback if Wasm significantly speeds up a noticeable operation (e.g., a one-time toast "Performance boost enabled!" after several successful, fast Wasm operations). Avoid being noisy.
    ```javascript
    // Example state for feedback (in Svelte component script)
    import { wasmState } from './wasmStore'; // Assuming a Svelte store for Wasm state
    let showPerformanceNotice = false;
    let lastSignificantPerformanceTimestamp = 0;

    $: {
      const state = $wasmState; // Reactive dependency on the store
      if (
        state.isActive && // Wasm is currently being used
        state.performanceMetrics?.overallSpeedup > 3.0 && // Example threshold
        state.performanceMetrics?.totalWasmCalls > 20 && // Example threshold
        Date.now() - lastSignificantPerformanceTimestamp > 300000 // Show max once per 5 mins
      ) {
        showPerformanceNotice = true;
        lastSignificantPerformanceTimestamp = Date.now();
        console.log("Wasm performance boost detected.");

        // Hide notice after a few seconds
        setTimeout(() => { showPerformanceNotice = false; }, 5000);
      }
    }

    // In template:
    // {#if showPerformanceNotice}
    //  <div class="performance-notice">Performance boost enabled ✨</div>
    // {/if}
    ```

## 8. User Experience Design

### 8.1 Control Design & Placement

#### 8.1.1 Auto-Scroll Toggle
- **Checkbox Presentation**: Standard HTML checkbox with a clear, concise label (e.g., "Auto-scroll", "Follow Logs").
- **Placement**: Logically grouped with other log view controls, easily visible and accessible near the log output area.
- **State Indication**: The checkbox state (`checked`/`unchecked`) is the primary indicator. Consider subtle secondary indicators if needed (e.g., a slightly different background/border when scrolling is paused due to user interaction).
- **CRITICAL IMPLEMENTATION**: Use a standard HTML `<input type="checkbox">` with a corresponding `<label>` for accessibility and reliable event handling. Bind its `checked` state to the `autoScroll` variable, but ensure changes *only* go through the `setAutoScroll` function via the `on:change` handler.
    ```svelte
    <!-- CORRECT: Single update path via central function -->
    <div class="control-item flex items-center gap-1">
      <input
        id="auto-scroll-checkbox"
        type="checkbox"
        class="accent-primary"
        aria-describedby="auto-scroll-description"
        bind:checked={autoScroll} {/* Note: This binding might need adjustment if direct binding causes issues. */}
                                   {/* Prefer on:change handler */}
        on:change={(event) => setAutoScroll(event.currentTarget.checked, 'userInteraction:checkbox')}
      />
      <label for="auto-scroll-checkbox" class="cursor-pointer">
        Auto-scroll
      </label>
    </div>
    <p id="auto-scroll-description" class="sr-only">Automatically scroll to the newest logs</p>
    ```
    *Self-Correction*: Using `bind:checked` directly might bypass the centralized `setAutoScroll` if not careful. The `on:change` handler triggering `setAutoScroll` is the safer pattern, ensuring all state changes go through the controlled path. The `checked={autoScroll}` attribute ensures the UI reflects the state.

#### 8.1.2 Supplementary Controls
- **Scroll to Bottom Button**: Consider adding a button (e.g., "Go to Bottom", "↓") that appears only when `autoScroll` is OFF and the user is scrolled up away from the bottom (`scrollTop > tolerance`). Clicking this button should scroll smoothly or instantly to the bottom and, optionally, re-enable auto-scroll (consistent with Option 2 behavior).
- **Log Navigation Aids**: For very large logs, consider adding jump-to-time or search functionality, separate from the core scrolling mechanism.
- **Visual Indicators**:
  - A subtle indicator (e.g., a thin line or temporary highlight) for newly arrived logs when `autoScroll` is OFF and the new logs are added below the current view.
  - A clear but unobtrusive notification/toast (as designed in `setAutoScroll`) when `autoScroll` is implicitly disabled by scrolling or explicitly toggled.
- **USER FEEDBACK (Toast Example)**:
    ```javascript
    // State for toast message
    let showAutoScrollToast = false;
    let autoScrollToastMessage = "";
    let autoScrollToastTimer = null;

    function showAutoScrollToastFeedback(message) {
      if (autoScrollToastTimer) clearTimeout(autoScrollToastTimer);

      autoScrollToastMessage = message;
      showAutoScrollToast = true;

      autoScrollToastTimer = window.setTimeout(() => {
        showAutoScrollToast = false;
        autoScrollToastTimer = null;
      }, 2500); // Show for 2.5 seconds
    }

    // Call this from setAutoScroll:
    // showAutoScrollToastFeedback(newValue ? "Auto-scroll enabled" : "Auto-scroll disabled");
    // Or more descriptive when disabled by scroll:
    // showAutoScrollToastFeedback("Auto-scroll paused. Scroll to bottom to resume.");
    ```

### 8.2 Scrolling Aesthetics
- **Scrolling Animation**:
  - Use **instant** scrolling (`scroll-behavior: auto` or direct `scrollTop` manipulation without smooth options) for programmatic scrolls related to auto-scroll ON (`scrollToBottom`) and potentially VAS restoration (`restoreViewportAnchor`) to ensure precise positioning without delay or interference from user actions.
  - Consider using smooth scrolling (`scroll-behavior: smooth`) *only* for explicit user actions like clicking a "Scroll to Bottom" button, if desired. Avoid global smooth scrolling on the container itself.
- **Scrollbar Appearance**: Use the browser's native scrollbar by default for platform consistency and accessibility. Custom scrollbars can introduce usability issues if not implemented carefully.
- **Visual Feedback**: Implement the subtle highlight for new logs when auto-scroll is OFF (e.g., a brief background fade-in animation on new log entries).
- **ANIMATION CONFLICTS**: Ensure CSS `scroll-behavior: smooth` is *not* applied to the scroll container, as it interferes with programmatic instant scrolling needed for auto-scroll and VAS. Control scroll behavior programmatically.
    ```css
    .log-scroll-container {
      /* Ensure programmatic scrolls are instant */
      scroll-behavior: auto !important;
      /* Other styles: overflow-y: scroll, height, etc. */
    }
    ```

### 8.3 Performance Perception
- **Responsiveness Priority**: The UI thread must remain responsive. Ensure log processing (parsing, filtering, preparing for render), especially large batches or Wasm operations, doesn't block the main thread for extended periods. Use web workers for complex parsing if necessary, or ensure Wasm operations yield if extremely long-running (though Wasm runs synchronously on the main thread unless in a worker).
- **Progressive Loading**: If dealing with extremely large logs loaded initially, fetch and render them incrementally rather than all at once.
- **Background Processing**: Defer non-critical work (like updating secondary indices or detailed metrics) using `requestIdleCallback` or `setTimeout(..., 0)`.
- **VIRTUALIZATION THRESHOLD**: Implement virtualization based on log count (e.g., enable above 500-1000 logs) to maintain performance.

## 9. Implementation Approach

### 9.1. Unified Auto-Scroll State Management

```javascript
// In Svelte component script
import { onMount, onDestroy, tick } from 'svelte';

// --- State Variables ---
let autoScroll = true; // Default: ON. Single source of truth.
let viewportAnchor = null; // Stores VAS data { index, offset } or { element, offset }
let isUserScrolling = false; // True during user scroll gestures
let manualScrollLock = false; // True shortly after user scroll to prevent interference
let isProgrammaticScroll = false; // True during system-initiated scrolls
let animationInProgress = false; // True during CSS/Svelte transitions
let scrollContainer = null; // Bound element ref: bind:this={scrollContainer}

// Timers
let manualScrollLockTimer = null;
let userScrollTimeout = null; // Detects scroll end
let scrollRAF = null; // Throttles scroll handler execution
let batchMeasurementTimer = null; // Batches post-update measurements/restores
let autoScrollToastTimer = null; // Manages toast visibility duration

// Flags / Settings
let debug = true; // Enable console logging for debugging
let virtualEnabled = false; // Is virtualization currently active?
let virtualizationReady = false; // Is virtualization library loaded/initialized?

// Toast state
let showAutoScrollToast = false;
let autoScrollToastMessage = "";

// --- Core AutoScroll Setter ---
function setAutoScroll(newValue, source = 'unknown') {
  if (newValue === autoScroll) return; // No change

  if (debug) console.log(`setAutoScroll: ${newValue ? 'ON' : 'OFF'} (Source: ${source})`);

  autoScroll = newValue;
  // Checkbox UI update is handled by Svelte's reactive binding: checked={autoScroll} attribute
  // We might need to trigger a reactivity update if direct binding isn't used:
  // autoScroll = autoScroll; // Force Svelte to see the change if needed

  if (newValue) {
    // --- Enabling Auto-Scroll ---
    viewportAnchor = null; // Clear any saved position anchor
    manualScrollLock = false; // Ensure lock is off if enabling programmatically
    if (manualScrollLockTimer) clearTimeout(manualScrollLockTimer);

    // If not already at bottom, scroll there (unless user is currently scrolling)
    if (scrollContainer && !isUserScrolling) {
      const absScrollTop = Math.abs(scrollContainer.scrollTop);
      if (absScrollTop > 1) { // Only scroll if not already at bottom
         if (debug) console.log(`AutoScroll ON: Scrolling to bottom from ${absScrollTop}px`);
         scrollToBottom(); // Uses withProgrammaticScroll
      }
    }
    showAutoScrollToastFeedback("Auto-scroll enabled");

  } else {
    // --- Disabling Auto-Scroll ---
    // Save anchor *only* if disabled explicitly (checkbox) or programmatically,
    // NOT if disabled implicitly by user scrolling away (anchor is captured by scroll handler).
    if (source !== 'userScrollAway' && !isUserScrolling) {
      // Capture current position immediately *before* user might scroll further
      saveViewportAnchor(); // Implement this function
      if (debug && viewportAnchor) console.log(`AutoScroll OFF: Saved anchor`, viewportAnchor);
    }
    showAutoScrollToastFeedback(source === 'userScrollAway' ?
      "Auto-scroll paused. Scroll to bottom to resume." :
      "Auto-scroll disabled");
  }
}

// --- Programmatic Scroll Wrapper ---
function withProgrammaticScroll(callback) {
  isProgrammaticScroll = true;
  try {
    callback();
  } finally {
    // Needs careful timing to clear *after* browser processes scroll event
    requestAnimationFrame(() => {
      setTimeout(() => { isProgrammaticScroll = false; }, 0);
    });
  }
}

// --- Scroll Functions ---
function scrollToBottom() {
  withProgrammaticScroll(() => {
    if (scrollContainer) {
      if (debug && scrollContainer.scrollTop !== 0) console.log("Programmatic scroll to bottom (scrollTop=0)");
      scrollContainer.scrollTop = 0; // column-reverse: 0 is bottom
      // If virtualized, may need to trigger virtualizer update here too
      // if (virtualizationReady && virtualEnabled) updateVirtualization(0);
    }
  });
}

function saveViewportAnchor() {
    if (!scrollContainer || !scrollContainer.clientHeight) {
        viewportAnchor = null;
        return;
    }

    if (virtualEnabled && virtualizationReady) {
        // Virtualization: Find center item index + offset
        const viewportCenterY = scrollContainer.scrollTop + scrollContainer.clientHeight / 2;
        const centerItem = findItemAtScrollPosition(viewportCenterY); // Needs implementation
        if (centerItem) {
            const itemTop = calculateItemTopPosition(centerItem.index); // Needs implementation
            // Offset from top of viewport to top of item
            const offset = itemTop - scrollContainer.scrollTop;
            viewportAnchor = { type: 'virtual', index: centerItem.index, offset: offset };
            if (debug) console.log(`Saved virtual anchor: index=${centerItem.index}, offset=${offset}`);
        } else {
            viewportAnchor = null; // Fallback or use scroll percentage?
        }
    } else {
        // Non-Virtualized: Find center element + offset
        const viewportCenterY = scrollContainer.scrollTop + scrollContainer.clientHeight / 2;
        let centerElement = null;
        let minDistance = Infinity;

        // Simplified approach: Find visible element closest to center
        const children = scrollContainer.children;
        for (let i = 0; i < children.length; i++) {
            const element = children[i];
            const elementRect = element.getBoundingClientRect();
            const scrollRect = scrollContainer.getBoundingClientRect();
            const elementCenter = elementRect.top - scrollRect.top + elementRect.height / 2;
            const distance = Math.abs(elementCenter - scrollContainer.clientHeight / 2); // Distance from visual center

            if (distance < minDistance) {
                minDistance = distance;
                centerElement = element;
            }
        }

        if (centerElement) {
            const elementRect = centerElement.getBoundingClientRect();
            const scrollRect = scrollContainer.getBoundingClientRect();
            // Offset from top of viewport to top of element
            const offset = elementRect.top - scrollRect.top;
            // Get a stable identifier (e.g., index, or a data-id attribute if available)
            const elementId = centerElement.dataset.logId || Array.from(scrollContainer.children).indexOf(centerElement);
            viewportAnchor = { type: 'element', identifier: elementId, offset: offset };
            if (debug) console.log(`Saved element anchor: identifier=${elementId}, offset=${offset}`);
        } else {
            viewportAnchor = null; // Fallback: use scroll percentage?
            const scrollPercentage = scrollContainer.scrollTop / (scrollContainer.scrollHeight - scrollContainer.clientHeight);
            viewportAnchor = { type: 'percentage', value: scrollPercentage };
             if (debug) console.log(`Saved percentage anchor: value=${scrollPercentage}`);
        }
    }
}

function restoreViewportAnchor() {
  if (!viewportAnchor || !scrollContainer || !scrollContainer.clientHeight) {
    if (debug && !viewportAnchor) console.log("Restore skipped: No anchor saved.");
    if (debug && !scrollContainer) console.log("Restore skipped: Scroll container not available.");
    return;
  }
  if (debug) console.log("Attempting to restore viewport anchor:", viewportAnchor);

  withProgrammaticScroll(async () => {
    // Ensure DOM is stable before calculations
    await tick();

    let targetScrollTop = scrollContainer.scrollTop; // Default to current if restore fails

    try {
        if (viewportAnchor.type === 'virtual') {
            const itemTop = calculateItemTopPosition(viewportAnchor.index); // Needs implementation
            if (itemTop !== null) {
                // Target scroll pos = item's top position - offset from viewport top
                targetScrollTop = itemTop - viewportAnchor.offset;
                if (debug) console.log(`Virtual Restore: index=${viewportAnchor.index}, itemTop=${itemTop}, offset=${viewportAnchor.offset} -> targetScrollTop=${targetScrollTop}`);
            } else {
                if(debug) console.warn(`Virtual Restore Failed: Could not find position for index ${viewportAnchor.index}`);
                // Fallback? Maybe use percentage if available?
            }
        } else if (viewportAnchor.type === 'element') {
            let targetElement = null;
            if (typeof viewportAnchor.identifier === 'number') { // Index-based identifier
                targetElement = scrollContainer.children[viewportAnchor.identifier];
            } else { // ID-based identifier
                targetElement = scrollContainer.querySelector(`[data-log-id="${viewportAnchor.identifier}"]`);
            }

            if (targetElement) {
                 const elementRect = targetElement.getBoundingClientRect();
                 const scrollRect = scrollContainer.getBoundingClientRect();
                 const currentElementTop = elementRect.top - scrollRect.top;
                 // Adjustment needed = current element top - desired element top (offset)
                 const scrollAdjustment = currentElementTop - viewportAnchor.offset;
                 targetScrollTop = scrollContainer.scrollTop + scrollAdjustment;
                 if (debug) console.log(`Element Restore: id=${viewportAnchor.identifier}, currentTop=${currentElementTop}, offset=${viewportAnchor.offset}, adjustment=${scrollAdjustment} -> targetScrollTop=${targetScrollTop}`);
            } else {
                 if(debug) console.warn(`Element Restore Failed: Could not find element with identifier ${viewportAnchor.identifier}`);
                 // Fallback? Use percentage?
            }
        } else if (viewportAnchor.type === 'percentage') {
             const maxScroll = scrollContainer.scrollHeight - scrollContainer.clientHeight;
             targetScrollTop = viewportAnchor.value * maxScroll;
             if (debug) console.log(`Percentage Restore: value=${viewportAnchor.value}, maxScroll=${maxScroll} -> targetScrollTop=${targetScrollTop}`);
        }

        // Clamp and apply
        const totalHeight = scrollContainer.scrollHeight;
        const clientHeight = scrollContainer.clientHeight;
        const maxScrollTop = Math.max(0, totalHeight - clientHeight);
        const clampedScrollTop = Math.max(0, Math.min(targetScrollTop, maxScrollTop));

        if (Math.abs(scrollContainer.scrollTop - clampedScrollTop) > 1) {
           if (debug) console.log(`Restoring scroll top from ${scrollContainer.scrollTop} to ${clampedScrollTop}`);
           scrollContainer.scrollTop = clampedScrollTop;
           // If virtualized, may need to trigger virtualizer update
           // if (virtualizationReady && virtualEnabled) updateVirtualization(clampedScrollTop);
        } else {
           if (debug) console.log(`Anchor restore skipped, already near target (${scrollContainer.scrollTop} ≈ ${clampedScrollTop})`);
        }
    } catch (error) {
        console.error("Error during restoreViewportAnchor:", error);
    }
  });
  // Keep the anchor until next save/disable, allows retries if needed.
}

// --- Toast Feedback ---
function showAutoScrollToastFeedback(message) {
  // (Implementation as shown in 8.1.2)
  if (autoScrollToastTimer) clearTimeout(autoScrollToastTimer);
  autoScrollToastMessage = message;
  showAutoScrollToast = true;
  autoScrollToastTimer = window.setTimeout(() => { showAutoScrollToast = false; }, 2500);
}

// --- Lifecycle ---
onMount(() => {
  // Bind scroll listener, resize observer etc.
  // Initial scroll check
  setTimeout(async () => {
    await tick();
    if (scrollContainer && autoScroll && !isUserScrolling) {
       scrollToBottom();
    }
    // Add scroll listener
    scrollContainer?.addEventListener('scroll', handleScroll, { passive: true });
    // Initialize observers...
  }, 50);
});

onDestroy(() => {
  // Cleanup listeners, timers, observers
  scrollContainer?.removeEventListener('scroll', handleScroll);
  if (manualScrollLockTimer) clearTimeout(manualScrollLockTimer);
  if (userScrollTimeout) clearTimeout(userScrollTimeout);
  if (scrollRAF) cancelAnimationFrame(scrollRAF);
  if (batchMeasurementTimer) clearTimeout(batchMeasurementTimer);
  if (autoScrollToastTimer) clearTimeout(autoScrollToastTimer);
  // Disconnect observers...
});
```

### 9.2. Scroll Event Handling with Proper Throttling

```javascript
// In Svelte component script

function handleScroll() {
  if (isProgrammaticScroll) {
    // If a programmatic scroll happened, we might receive a scroll event.
    // We should ignore it for state changes, but potentially allow
    // virtualization updates if needed. Resetting the flag happens later.
    // For simplicity here, we just return early.
    return;
  }

  // --- User is scrolling ---
  isUserScrolling = true;
  manualScrollLock = true; // Prevent auto-scroll/anchor restore interference

  // --- Reset manual lock timer ---
  // Ensures lock stays active while scrolling and for a period after
  if (manualScrollLockTimer) clearTimeout(manualScrollLockTimer);
  manualScrollLockTimer = window.setTimeout(() => {
    manualScrollLock = false;
    manualScrollLockTimer = null;
    if (debug) console.log("Manual scroll lock released.");
    // Check if we ended up at the bottom after lock release
    checkScrollPositionForAutoScrollEnable();
  }, 1500); // Lock duration - needs tuning (e.g., 1.5 seconds after last scroll event)

  // --- Throttle execution with RAF ---
  if (scrollRAF) cancelAnimationFrame(scrollRAF);
  scrollRAF = requestAnimationFrame(() => {
    if (!scrollContainer) {
      scrollRAF = null;
      return; // Component unmounted or container not ready
    }

    const scrollTop = scrollContainer.scrollTop; // Read once
    const absScrollTop = Math.abs(scrollTop);

    // --- Auto-scroll disable logic ---
    // If user scrolls away from the bottom (scrollTop > tolerance) while autoScroll is ON
    if (absScrollTop > 2 && autoScroll) {
      if (debug) console.warn(`User scrolled away (${scrollTop}px), disabling auto-scroll.`);
      // IMPORTANT: Call setAutoScroll *before* saving the anchor
      setAutoScroll(false, 'userScrollAway');
      // Capture anchor position *now* as user defined it by scrolling.
      saveViewportAnchor();
      if (debug && viewportAnchor) console.log(`Saved anchor immediately after user scroll away`, viewportAnchor);
    }

    // Update scroll metrics / virtualization view
    // updateScrollMetrics(absScrollTop);
    // if (virtualizationReady && virtualEnabled) { updateVirtualization(scrollTop); }

    // --- Scroll End Detection ---
    if (userScrollTimeout) clearTimeout(userScrollTimeout);
    userScrollTimeout = window.setTimeout(() => {
      isUserScrolling = false;
      if (debug) console.log("User scroll ended.");
      // Check position *after* scrolling truly stops
      checkScrollPositionForAutoScrollEnable();
    }, 200); // Shorter timeout for scroll end detection (e.g., 200ms)

    scrollRAF = null; // Mark RAF execution complete
  });
}

function checkScrollPositionForAutoScrollEnable() {
  // This function checks if we should re-enable auto-scroll (Option 2)
  // Only run if user is not actively scrolling AND manual lock is off
  if (!isUserScrolling && !manualScrollLock && scrollContainer) {
    const absScrollTop = Math.abs(scrollContainer.scrollTop);
    if (absScrollTop <= 1 && !autoScroll) {
      // Option 2 Implementation: If user scrolled to bottom, re-enable auto-scroll
      // This is currently DISABLED based on recommendation in 4.1.2
      // if (debug) console.log("User scrolled to bottom, re-enabling auto-scroll. (Option 2 - Currently Disabled)");
      // setAutoScroll(true, 'scrolledToBottom'); // UNCOMMENT FOR OPTION 2
    }
  }
}

// Add listener in onMount:
// scrollContainer?.addEventListener('scroll', handleScroll, { passive: true }); // Done in 9.1
// Remove listener in onDestroy:
// scrollContainer?.removeEventListener('scroll', handleScroll); // Done in 9.1
```

### 9.3. Reactive Log Addition Handling

```javascript
// In Svelte component script
import { tick } from 'svelte';

let filteredLogs = []; // Assume this is reactively updated from log processing
let recentlyAddedLogs = new Set(); // For highlighting new logs
let pendingMeasurements = false; // Flag for batching post-update work

// This logic should likely be triggered *after* processBatchedLogs updates filteredLogs
// or whatever upstream process updates the logs.
// Maybe turn processBatchedLogs into an async function and call this logic at the end.
async function handleLogUpdateCompletion() {
    if (!scrollContainer) return;

    // --- Identify condition leading to this call (e.g., new logs added) ---
    const logsJustChanged = true; // Assume logs were just updated before calling this

    if (logsJustChanged) {
        if (autoScroll) {
            // --- Auto-Scroll ON ---
            if (!isUserScrolling && !manualScrollLock && !animationInProgress) {
                // Schedule scroll to bottom after the next tick/paint
                await tick(); // Wait for Svelte DOM update
                requestAnimationFrame(() => {
                    // Double check state in case it changed during delays
                    if (autoScroll && !isUserScrolling && !manualScrollLock && scrollContainer) {
                        if (Math.abs(scrollContainer.scrollTop) > 1) {
                            if (debug) console.log("AutoScroll ON: Enforcing scroll to bottom after log update.");
                            scrollToBottom();
                        } else {
                            if (debug) console.log("AutoScroll ON: Already at bottom after log update.");
                        }
                    }
                });
            } else {
                if (debug) console.log("AutoScroll ON: Skipped enforcing bottom scroll due to user interaction/animation.");
            }
        } else {
            // --- Auto-Scroll OFF ---
            // Anchor should have been saved *before* the log update started.
            // Mark new logs for highlighting
            // markRecentlyAddedLogs(filteredLogs); // Needs implementation details

            // Track animations if applicable
            // if (usingLogEntryAnimations) { animationInProgress = true; ... }

            // Schedule restoration after DOM updates and potential animations
            if (!pendingMeasurements) {
                pendingMeasurements = true;
                batchMeasurementTimer = window.setTimeout(async () => {
                    await tick(); // Ensure Svelte DOM updates are finished

                    // Perform measurements if needed
                    // recalculatePositionsIfNeeded();

                    // Update virtualization
                    // if (virtualizationReady && virtualEnabled) { updateVirtualization(); }

                    // Restore position if conditions still hold
                    if (!autoScroll && viewportAnchor && !isUserScrolling && !manualScrollLock && !animationInProgress) {
                        restoreViewportAnchor();
                    } else {
                        if (debug) console.log("AutoScroll OFF: Skipped anchor restore due to state/interaction/animation.");
                    }

                    pendingMeasurements = false;
                    batchMeasurementTimer = null;
                }, 10); // Small delay for batching
            }
        }
    }
}

// Modify processBatchedLogs from 3.4.2 to call this handler:
async function processBatchedLogs(batchedLogs) {
  let savedAnchorBeforeUpdate = null;
  // Before processing: Save viewport anchor if auto-scroll is OFF
  if (!autoScroll && !isUserScrolling && !manualScrollLock) {
    // Need to save *synchronously* before logs change
    saveViewportAnchor();
    savedAnchorBeforeUpdate = viewportAnchor; // Keep track of the saved anchor
  } else {
     savedAnchorBeforeUpdate = null; // Ensure no stale anchor is used
  }

  // Process logs (this might be async if using Wasm workers, but assumed sync here)
  const mergedLogs = mergeInsertLogs(filteredLogs, batchedLogs);
  filteredLogs = applyCurrentFilter(mergedLogs); // Update the reactive variable

  // Now that filteredLogs is updated, Svelte will schedule a DOM update.
  // Call the completion handler to schedule post-update actions.
  // Use setTimeout to ensure it runs after the current synchronous flow.
  setTimeout(() => handleLogUpdateCompletion(), 0);
}

// Function to mark logs (example - needs refinement)
function markRecentlyAddedLogs(logs) {
  if (virtualEnabled || !logs) return; // Skip if virtualizing or no logs
  const now = Date.now();
  logs.forEach(log => {
    // Assuming log has a unique ID and timestamp
    if (log._internal_timestamp && now - log._internal_timestamp < 1000 && log.id) {
      if (!recentlyAddedLogs.has(log.id)) {
        recentlyAddedLogs.add(log.id);
        setTimeout(() => {
          recentlyAddedLogs.delete(log.id);
          // Trigger reactivity if CSS class depends on this set
          // recentlyAddedLogs = recentlyAddedLogs;
        }, 1500); // Match CSS animation duration
      }
    }
  });
  // Trigger reactivity if needed: recentlyAddedLogs = recentlyAddedLogs;
}
```

### 9.4. WebAssembly-Enhanced Log Processing

```javascript
// Example Wasm integration for a log processing function
// Assumes wasmStore provides state like: isEnabled, module, performanceMetrics, errors etc.
// Assumes wasmUtils provides: shouldUseWasm, handleWasmError, trackOperationStart/End etc.

import { wasmState, getWasmModule } from './wasmStore'; // Svelte store access
import { shouldUseWasm, handleWasmError, trackOperationStart, trackOperationEnd, updatePerformanceMetrics } from './wasmUtils';
import { mergeInsertLogsTS } from './logProcessingTS'; // TypeScript fallback implementation
import { serializeLogsForWasm, deserializeLogsFromWasm } from './wasmSerialization'; // Data conversion helpers

// Function called to merge new logs into existing ones
function mergeInsertLogs(existingLogs, newLogs) {
  const operation = 'mergeInsertLogs';
  trackOperationStart(operation); // For metrics

  const totalLogCount = (existingLogs?.length || 0) + (newLogs?.length || 0);

  // Handle trivial cases
  if (!newLogs || newLogs.length === 0) return existingLogs || [];
  if (!existingLogs || existingLogs.length === 0) {
     // Ensure new logs are sorted chronologically if needed (Wasm might do this)
     newLogs.sort((a, b) => (a._unix_time || 0) - (b._unix_time || 0));
     return newLogs;
  }


  let result = null;
  let wasmTime = -1, tsTime = -1, serializeTime = -1, deserializeTime = -1;
  let usedWasm = false;

  // --- Decide whether to use WebAssembly ---
  if (shouldUseWasm(totalLogCount, operation)) {
    try {
      const wasmModule = getWasmModule(); // Get loaded Wasm instance
      if (!wasmModule || !wasmModule.instance?.exports?.merge_insert_logs) { // Check function exists
        throw new Error("WebAssembly module or merge_insert_logs function not available");
      }
      const wasmExports = wasmModule.instance.exports;

      // --- Prepare Data for Wasm ---
      const serializeStart = performance.now();
      // Combine logs first might be simpler for some Wasm implementations
      const combinedLogs = [...existingLogs, ...newLogs];
      const { pointer, size } = serializeLogsForWasm(combinedLogs, wasmExports.memory, wasmExports.allocate_logs); // Pass memory/allocator if needed
      serializeTime = performance.now() - serializeStart;

      // --- Call Wasm Function ---
      const wasmStart = performance.now();
      // Example: Wasm function takes pointer/size, returns new pointer/size
      const resultPtrSize = wasmExports.merge_insert_logs(pointer, size);
      wasmTime = performance.now() - wasmStart;

      // --- Process Result from Wasm ---
      const deserializeStart = performance.now();
      const deserialized = deserializeLogsFromWasm(resultPtrSize, wasmExports.memory, wasmExports.free_logs); // Convert result back, free memory
      deserializeTime = performance.now() - deserializeStart;

      result = deserialized.logs;
      usedWasm = true;
      // Clear any previous error count for this operation on success
      // clearOperationErrorCount(operation);

    } catch (error) {
      // --- Wasm Error Handling ---
      console.error(`Wasm ${operation} failed, falling back to TypeScript.`, error);
      handleWasmError(error, operation, { logCount: totalLogCount });
      // Ensure result is null so TS fallback runs
      result = null;
      usedWasm = false;
    }
  }

  // --- TypeScript Fallback ---
  if (result === null) {
    if (debug && usedWasm) console.log(`Falling back to TS for ${operation}`); // Logged if Wasm was attempted but failed
    else if(debug && !usedWasm) console.log(`Using TS for ${operation} (Wasm threshold not met or Wasm disabled)`);

    const tsStart = performance.now();
    result = mergeInsertLogsTS(existingLogs, newLogs); // Call the TS version
    tsTime = performance.now() - tsStart;
    usedWasm = false; // Ensure flag is false
  }

  // --- Update Performance Metrics ---
  updatePerformanceMetrics({
    operation,
    wasmTime,
    tsTime, // Will be -1 if Wasm succeeded, or contain TS time if fallback ran
    logCount: totalLogCount,
    serializeTime,
    deserializeTime,
    usedWasm
  });

  trackOperationEnd(operation);
  return result;
}
```

## 10. Testing & Validation Requirements

### 10.1 Critical Test Scenarios
- **Rapid Log Addition**: Simulate adding logs at high frequency (e.g., 50-200 logs/sec) in bursts and continuously via the backend throttler's `log-batch` event. Verify UI responsiveness, correct auto-scroll behavior, and stable VAS positioning. Test with and without Wasm active.
- **Browser Compatibility**: Test extensively on latest versions of Chrome, Firefox, Safari, and Edge. Pay special attention to `column-reverse` layout, scroll event timing, and Wasm support/performance differences (especially Safari).
- **Interaction Combinations**:
  - Scroll manually (up/down) while logs are rapidly arriving (auto-scroll ON and OFF).
  - Toggle auto-scroll checkbox rapidly while logs are arriving.
  - Apply filters while logs are arriving.
  - Resize the container/window during log additions and user scrolling.
- **Low-Resource Scenarios**: Use browser dev tools to throttle CPU (4x-6x slowdown) and network. Test on actual low-spec hardware if possible. Verify graceful degradation (UI remains usable, Wasm might be disabled more readily).
- **Long-Running Sessions**: Run the log viewer for extended periods (hours) with continuous or periodic log additions. Monitor for memory leaks (using browser memory profiler) and performance degradation over time.
- **MEMORY LEAK TESTING**: Systematically mount/unmount the component hundreds of times while adding/removing logs. Check for detached DOM nodes and increasing memory usage in the profiler, ensuring all listeners/observers/timers are cleaned up `onDestroy`.
- **CIRCULAR DEPENDENCY TESTS**: Add verbose logging around state changes (`autoScroll`, `scrollTop`, `filteredLogs`) and function calls (`setAutoScroll`, `handleScroll`, reactive blocks, `scrollToBottom`, `restoreViewportAnchor`). Manually inspect logs during complex interactions to ensure no infinite loops or unintended chained reactions occur.
- **COMPONENT INTERACTION TESTS**: If the log viewer is part of a larger application, test interactions with other components that might affect its state or data (e.g., global filters, data sources).

### 10.2 User Interaction Testing
- **Natural Usage Patterns**: Simulate typical user behavior: scrolling up to read older logs, pausing, scrolling back down, enabling/disabling auto-scroll. Verify behavior matches expectations and principles (user control, predictability).
- **Edge Interaction Sequences**:
  - Start scrolling *immediately* after toggling auto-scroll ON/OFF.
  - Flick-scroll (inertial scrolling) and let it settle near/at the bottom; verify state transitions.
  - Click checkbox while momentum scrolling is active.
  - Hold down PgDown/PgUp keys.
- **Accessibility Testing**:
  - Navigate and operate the log viewer entirely using the keyboard (Tab, Shift+Tab, Spacebar on checkbox, Arrow keys, PgUp/PgDown, Home/End for scrolling).
  - Use screen readers (VoiceOver, NVDA, JAWS) to verify proper announcements for logs, controls, and state changes (e.g., auto-scroll paused/resumed toast). Check ARIA attributes.
- **ASSUMPTION TESTING**: Create minimal test cases specifically verifying browser behavior with `flex-direction: column-reverse`, `scrollTop` values (0 at bottom, positive values scrolling up), and how adding content at the beginning affects scroll position with/without native scroll anchoring.
- **STATE SYNC VERIFICATION**: Add assertions or visual checks in tests to ensure the `autoScroll` state variable is *always* perfectly synchronized with the checkbox UI `checked` state under all conditions (user interaction, programmatic changes, implicit changes).

### 10.3 WebAssembly Integration Testing
- **Feature Detection**: Test in environments where Wasm is disabled or unsupported (e.g., older browsers, specific browser settings). Verify the application loads correctly and all log processing functions fall back to TypeScript seamlessly without errors.
- **Threshold Behavior**: Design tests that feed varying numbers of logs (below, around, and above the default/adaptive thresholds) to Wasm-enhanced functions (`mergeInsertLogs`, etc.). Verify that the system correctly delegates to Wasm or TypeScript based on the threshold logic. Test `forceWasmMode` settings.
- **Error Recovery**: Simulate Wasm errors (e.g., by providing invalid data to a Wasm function, modifying the Wasm module to throw errors, or simulating memory allocation failures if possible). Verify that:
  - `handleWasmError` is called.
  - The specific operation is blacklisted temporarily.
  - The function falls back to the TypeScript implementation correctly.
  - The UI remains stable and functional.
  - Critical errors potentially disable Wasm globally for the session.
- **Memory Management**: If Wasm allocates significant memory, design tests that trigger large allocations. Monitor memory usage. Simulate `Memory.grow` failures (if possible in test environment) and verify recovery.
- **Performance Comparison**: Implement automated tests that run key operations (e.g., merging 10k logs into 50k logs) using both the Wasm and TS implementations multiple times. Measure and compare execution times to validate that Wasm provides the expected speedup under defined conditions. Track serialization/deserialization overhead.
- **DATA CONSISTENCY**: For every function implemented in both Wasm and TS (`mergeInsertLogs`, `findLogAtScrollPosition`, etc.), create test suites that run the *exact same input data* through both versions and assert that the outputs are *identical*. This is critical to ensure Wasm optimization doesn't introduce behavioral changes or bugs.
    ```javascript
    // Example Jest/Vitest test structure
    describe('mergeInsertLogs Consistency', () => {
      const testCases = generateLogTestCases(); // Function to create diverse log scenarios

      testCases.forEach((testCase, index) => {
        it(`should produce identical results for test case ${index}`, () => {
          const { existingLogs, newLogs } = testCase;

          // Mock wasmUtils and wasmStore as needed
          const wasmResult = runMergeWithWasmForced(true, [...existingLogs], [...newLogs]);
          const tsResult = runMergeWithWasmForced(false, [...existingLogs], [...newLogs]);

          // Deep comparison of the resulting log arrays
          // Ensure logs are compared by content and order
          expect(wasmResult).toEqual(tsResult);
        });
      });
    });

    // Helper function for testing
    function runMergeWithWasmForced(forceWasm, existing, news) {
        // Mock shouldUseWasm to return forceWasm
        // ... mock setup ...
        const result = mergeInsertLogs(existing, news);
        // ... mock cleanup ...
        return result;
    }
    ```

## 11. Phased Implementation Plan

### Phase 1: Core Auto-Scroll Fix

**Objective**: Resolve the primary issue where VAS runs unconditionally, causing unintended auto-scroll behavior.

**Key Tasks**:
1. Implement the centralized `autoScroll` state variable.
2. Create the `setAutoScroll()` function as the single update path.
3. Modify log update handling (`processBatchedLogs` / `handleLogUpdateCompletion`) to conditionally save anchor *before* updates and conditionally restore anchor / scroll to bottom *after* updates, based on `autoScroll` state.
4. Update `handleScroll` to correctly call `setAutoScroll(false, 'userScrollAway')` when scrolling away from the bottom while auto-scroll is ON.
5. Ensure checkbox `on:change` correctly calls `setAutoScroll`.
6. Implement basic toast notifications for state changes using `showAutoScrollToastFeedback`.

**Success Criteria**:
- Auto-scroll can be reliably toggled ON/OFF via checkbox.
- When disabled (OFF), the view remains stable (anchor restored correctly) when new logs are added.
- When enabled (ON), the view follows the newest logs (`scrollToBottom` works).
- Scrolling up while ON correctly disables auto-scroll (`setAutoScroll(false, 'userScrollAway')` is triggered).

### Phase 2: Enhanced User Experience

**Objective**: Improve usability with robust interaction handling and visual feedback.

**Key Tasks**:
1. Implement full scroll event handling (`handleScroll`) with RAF throttling and scroll end detection (`userScrollTimeout`).
2. Implement `manualScrollLock` logic with its timer to prevent state fighting after user scrolls.
3. Implement `isProgrammaticScroll` flag and `withProgrammaticScroll` wrapper for system scrolls.
4. Add "scroll to bottom" button (conditional visibility).
5. Implement `animationInProgress` flag and `handleTransitionStart/End` if using CSS/Svelte animations for logs. Ensure scroll actions wait for animations.
6. Refine toast notifications for clarity (e.g., "Auto-scroll paused").
7. Implement subtle highlighting for new logs when auto-scroll is OFF (`markRecentlyAddedLogs`).

**Success Criteria**:
- User scrolling away from bottom reliably disables auto-scroll and saves the correct anchor position.
- Programmatic scrolls don't trigger implicit state changes.
- Manual lock prevents VAS/auto-scroll interference immediately after user scroll.
- Users receive clear visual feedback about state changes and new logs.
- Scrolling feels natural and responsive without visual jumps.
- UI components update properly after animations complete (if used).

### Phase 3: Virtualization and Performance

**Objective**: Ensure the solution works efficiently with large log volumes and virtualization.

**Key Tasks**:
1. Integrate with the chosen virtualization library.
2. Adapt `saveViewportAnchor` and `restoreViewportAnchor` for index-based anchoring when virtualization is enabled.
3. Implement `findItemAtScrollPosition` and `calculateItemTopPosition` (potentially using estimated heights) for virtual anchors.
4. Ensure coordinate transformations (`scrollTop` vs. position from top) are correct in virtual calculations.
5. Implement the `virtualEnabled` flag and conditional logic.
6. Test and optimize performance with 10,000+ logs.
7. Ensure backend integration (`processBatchedLogs`) efficiently handles large batches.
8. Review and apply Memory Management Guidelines (Section 12).

**Success Criteria**:
- Solution performs well with 10,000+ logs (smooth scrolling, responsive UI).
- Virtual scrolling maintains correct view positions for both auto-scroll ON and OFF (VAS).
- Memory usage remains stable during extended use with large log sets.
- UI remains responsive during high-frequency log updates.

### Phase 4: WebAssembly & Final Polish

**Objective**: Integrate WebAssembly optimizations and ensure overall robustness.

**Key Tasks**:
1. Finalize WebAssembly integration for `mergeInsertLogs` (and potentially `findLogAtScrollPosition`, `recalculatePositions` if needed for virtualization).
2. Implement `shouldUseWasm` logic with adaptive thresholds and settings override.
3. Implement robust Wasm error handling (`handleWasmError`), blacklisting, and TypeScript fallbacks.
4. Implement performance monitoring (`updatePerformanceMetrics`) and data consistency checks between Wasm/TS versions.
5. Add comprehensive browser testing (Chrome, Firefox, Safari, Edge), focusing on `column-reverse` quirks and Wasm compatibility.
6. Add graceful degradation for Wasm loading failures or unsupported environments.
7. Perform thorough accessibility testing (keyboard navigation, screen readers).
8. Final code cleanup, documentation updates, and review.

**Success Criteria**:
- WebAssembly provides measurable performance benefits for target operations when enabled and appropriate.
- System gracefully falls back to TypeScript when Wasm fails or is unavailable.
- Implementation works consistently and reliably across all target browsers.
- Edge cases (empty logs, rapid interactions, filtering) are handled gracefully.
- Accessibility requirements are met.
- Code is clean, well-documented, and maintainable.

## 12. Memory Management Guidelines

1.  **Maximum Log Retention**:
    *   Adhere strictly to the user-configurable `maxLogEntries` setting (e.g., default 10,000).
    *   When new logs arrive via `processBatchedLogs` and the total count exceeds `maxLogEntries`, efficiently prune the *oldest* logs from the `filteredLogs` array *before* storing the result. For `column-reverse`, oldest logs are at the end of the array.
        ```javascript
        // Inside mergeInsertLogs or after its result
        if (result.length > maxLogEntries) {
            const numberToRemove = result.length - maxLogEntries;
            result = result.slice(numberToRemove); // Keep the latest maxLogEntries
        }
        ```
    *   Investigate virtual pagination or dynamic loading from storage/backend for accessing logs beyond the retention limit, rather than keeping them in memory.

2.  **DOM Element Efficiency**:
    *   Implement virtualization, triggering it when the number of logs rendered *could* exceed a threshold (e.g., ~500-1000 logs). Ensure the virtualizer correctly calculates the number of DOM nodes needed based on container height and estimated item height.
    *   Keep log entry components lightweight. Avoid unnecessary DOM nesting within each log entry. Pass only essential data as props.
    *   Apply CSS `contain: strict` or `contain: content` to the log container and potentially individual log entries (if heights are fixed or predictable) to help browser layout optimization.

3.  **Event Listener Management**:
    *   Attach core listeners (`scroll`, `resize`) only once to the main scroll container element (`scrollContainer`).
    *   For interactions specific to log entries (e.g., click to expand details), use event delegation on the scroll container instead of attaching listeners to every log entry. Check `event.target` to identify the specific log entry clicked.
    *   **Crucially**: Ensure all event listeners (`scroll`, delegated listeners) and observers (`ResizeObserver`, `MutationObserver`) are explicitly removed/disconnected in the Svelte component's `onDestroy` lifecycle hook to prevent leaks when the component is unmounted.

4.  **Garbage Collection Optimization**:
    *   Minimize object/array creation within frequently called functions like `handleScroll` or log processing loops. Reuse objects where feasible.
    *   When updating `filteredLogs`, ensure the old array is dereferenced so it can be garbage collected. Avoid patterns that keep references to previous large log arrays unintentionally.
    *   Batch DOM read/write operations. For example, in `restoreViewportAnchor`, perform all measurements (getBoundingClientRect) together before applying the final `scrollTop` change. Use `requestAnimationFrame` to schedule writes just before the paint.
    *   Be mindful of closures in event listeners or callbacks. Avoid capturing large scopes or large data structures (like the entire `filteredLogs` array if only a few properties are needed) within closures that persist longer than necessary. Pass specific needed values instead.
    *   Profile memory usage regularly during development and testing using browser developer tools, specifically looking for detached DOM nodes and heap snapshots showing unexpected object retention.