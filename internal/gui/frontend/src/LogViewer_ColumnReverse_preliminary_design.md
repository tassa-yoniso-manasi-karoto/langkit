# Enhanced Design Specification: LogViewer Scrolling System with flex-direction: column-reverse CSS property and WebAssembly Integration

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

## 2. System Components & Relationships

### 2.1 Core Conceptual Model
- **Auto-Scroll Mode**: A binary state determining whether the view should automatically follow newest logs
- **Viewport Anchoring System (VAS)**: A position preservation mechanism that maintains stable viewing experience
- **Scroll Event Management**: A system for differentiating between user and programmatic scroll events
- **Virtualization Integration**: Specialized behavior modifications when dealing with virtualized content
- **WebAssembly Integration**: Performance optimization layer for computationally intensive operations with automatic fallbacks

### 2.2 State Management Architecture
- **Primary Control State**: `autoScroll` boolean - the single source of truth for tracking mode
- **Centralized State Modification**: `setAutoScroll()` function as the only method to modify auto-scroll state
- **Internal Operational Flags**:
  - User interaction tracking (e.g., active scrolling, programmatic operations)
  - Measurement and calculation coordination
  - Position anchoring data structures
  - Timing and debounce controls
  - Manual scroll locks to prevent state fighting
  - Animation tracking to coordinate transitions

### 2.3 System Interactions & Dependencies
- **UI → State**: User interactions with checkbox directly affect the `autoScroll` state through the central setter function
- **State → Behavior**: `autoScroll` state determines whether VAS is active and how scroll positions are maintained
- **Logs → Position**: Log additions trigger a position preservation flow depending on `autoScroll` state
- **Events → Flags**: Scroll, resize, and other events modify internal flags to coordinate behaviors
- **Performance → Delegation**: Log processing operations are delegated to WebAssembly or TypeScript based on performance metrics and thresholds
- **CRITICAL CAUTION**: Avoid circular dependencies where state changes trigger DOM updates that trigger further state changes

### 2.4 WebAssembly Integration Architecture
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

### 2.5 Reactive Statement Safety
- **CAUTION**: Svelte's reactive declarations (`$:`) can create hidden dependencies and circular update patterns
- **CAVEAT**: Multiple sources observing and updating the same state can cause "reactive loops"
- **PRACTICE**: Isolate DOM state (scrollTop position) from component state (auto-scroll flag)
- **DEFENSIVE APPROACH**: Use guard flags to prevent reactive statements from triggering multiple times during a single logical update

## 3. Detailed Behavioral Specifications

### 3.1 Auto-Scroll State Transitions

#### 3.1.1 User-Initiated Mode Changes (Checkbox Toggle)
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

#### 3.1.2 Implicit State Changes (User Scrolling)
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
    - **CRITICAL**: If implementing Option 2, apply a subtle timing delay to avoid accidental triggers (as shown in `handleScroll` implementation example `8.2`)

### 3.2 Viewport Anchoring System (VAS) Behavior

#### 3.2.1 Fundamental Operation
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
      if (autoScroll) {
        // Use direct approach for auto-scroll ON
        viewportAnchor = null; // Clear any anchor
        if (!isUserScrolling && !manualScrollLock && !animationInProgress) {
          scrollToBottom(); // Simple direct scroll (programmatically safe)
        }
      } else {
        // Use viewport anchoring only when auto-scroll is OFF
        // Save happens *before* DOM update trigger (e.g., before log processing)
        // saveViewportAnchor();
        // ...later after DOM updates...
        if (viewportAnchor && !isUserScrolling && !manualScrollLock && !animationInProgress) {
          restoreViewportAnchor(); // Restore happens *after* DOM update
        }
      }
    }
    ```

#### 3.2.2 Anchor Selection Strategy
- **Primary Strategy**: Anchor to visible log entry near viewport center
- **Alternative Strategy**: Use scroll percentage or offset from top/bottom when specific elements aren't reliable
- **Fallback Mechanism**: When anchor elements are removed (filtering, virtualization), recalculate based on nearby elements
- **TIMING CRITICAL**: Always use `await tick()` before measuring positions to ensure DOM has updated

#### 3.2.3 Coordinate Calculations
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

### 3.3 Log Update & Rendering Flow

#### 3.3.1 With Auto-Scroll ON
- **Expected Behavior**: View remains at newest logs (bottom)
- **Primary Mechanism**: Browser's natural tendency to maintain scrollTop=0 in column-reverse layout when content is added at the start (bottom visually).
- **Safety Mechanism**: Explicit `scrollTop=0` enforcement after updates when necessary, particularly:
  - After filtering operations
  - With virtualization enabled (where browser behavior might not suffice)
  - After significant layout changes
  - Following potential browser inconsistencies
- **TIMING CONSIDERATION**: Use `requestAnimationFrame` or `await tick()` followed by programmatic scroll for position enforcement to ensure it happens after rendering/layout.

#### 3.3.2 With Auto-Scroll OFF
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

## 4. Event Handling & Coordination

### 4.1 Scroll Event Management

#### 4.1.1 Event Categorization
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

#### 4.1.2 Scroll Cycle Behavior
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
        // Manual lock should persist longer via its own timer (see 8.2)
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

      // Reset manual lock timer (defined elsewhere, see 8.2)
      // resetManualScrollLockTimer();
    }
    ```

### 4.2 Resize Event Handling
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

### 4.3 DOM Lifecycle Integration
- **Before DOM Updates**: Capture necessary state, primarily viewport position references for VAS (`saveViewportAnchor()`). This often happens just before triggering the state change that causes the update (e.g., processing new logs).
- **After DOM Updates**: Apply position restoration (`restoreViewportAnchor()`) or enforce scroll position (`scrollToBottom()`) only *after* the DOM has finished rendering the changes. Use `await tick()` in Svelte, potentially followed by `requestAnimationFrame` for positioning to ensure layout is stable.
- **Batched Operations**: Group multiple measurements or DOM manipulations where possible (e.g., using `requestAnimationFrame` or short `setTimeout` for batching) to reduce layout thrashing and improve performance.
- **ASYNCHRONOUS AWARENESS**: Always use `await tick()` in Svelte before measuring DOM elements (like `offsetTop`, `clientHeight`, `scrollHeight`) or manipulating scroll position (`scrollTop`) *after* a reactive state change that affects the DOM.
- **SVELTE REACTIVITY CAUTION**: Be aware that Svelte's reactivity updates the DOM asynchronously. Props passed down might not reflect immediately in the child component's DOM until the next tick.

## 5. Edge Cases & Robustness Measures

### 5.1 Race Conditions & Timing Issues

#### 5.1.1 Rapid Interaction Sequences
- **Rapid Checkbox Toggling**: Ensure the `setAutoScroll` function handles rapid calls gracefully, possibly debouncing slightly or ensuring the latest call takes precedence. State should only transition based on the final intended value.
- **Scrolling During Transitions**: Prioritize direct user interaction (scrolling). If a user starts scrolling while a programmatic scroll (like `scrollToBottom` after enabling auto-scroll) is happening, the user scroll should take over, and the system should react accordingly (e.g., immediately disable auto-scroll again). Guard flags (`isUserScrolling`, `manualScrollLock`) are crucial here.
- **Updates During Scrolling**: When auto-scroll is OFF and logs arrive while the user is scrolling, the VAS should ideally *not* try to restore position until the user scroll finishes. The `isUserScrolling` flag prevents `restoreViewportAnchor` from running. When auto-scroll is ON and logs arrive while the user is scrolling *away* from the bottom, auto-scroll should already be disabled by the scroll handler, so the new logs will just be added without forcing a scroll-to-bottom.
- **Concurrent Operations**: Establish clear operation precedence:
    1.  Direct User Scroll: Highest priority. Interrupts/cancels most programmatic actions. Sets `isUserScrolling` and `manualScrollLock`.
    2.  User Checkbox Toggle: High priority. Triggers `setAutoScroll` immediately.
    3.  Programmatic Scrolls (`scrollToBottom`, `restoreViewportAnchor`): Lower priority. Should check flags (`!isUserScrolling`, `!manualScrollLock`, `!animationInProgress`) before executing.
    4.  DOM Updates/Log Processing: Happen reactively, but subsequent positioning depends on flags and state.

#### 5.1.2 Animation & Transition Timing
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

### 5.2 Initialization & Edge States

#### 5.2.1 Component Initialization
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
      // ... (as shown in 4.2) ...
    });
    ```

#### 5.2.2 Focus & Accessibility Considerations
- **Keyboard Navigation**: Ensure standard keyboard scrolling (arrow keys, PgUp/PgDown, Home/End) works as expected. These actions count as user scrolls and should disable auto-scroll if active and moving away from the bottom.
- **Screen Readers**: Use appropriate ARIA roles (`role="log"`, `aria-live="polite"` or `assertive` depending on needs) for the log container. Ensure controls (`checkbox`, buttons) have proper labels and ARIA attributes. Manage focus appropriately, especially when new content loads or controls appear/disappear.
- **Tab Visibility**: Consider pausing intensive operations (like frequent background polling or complex rendering updates) when the browser tab is not visible using the Page Visibility API, unless required for background operation. Auto-scroll logic might not need to run if the tab is hidden.

### 5.3 Browser & Environment Variations

#### 5.3.1 Browser-Specific Behaviors
- **Scroll Position Maintenance**: Test `flex-direction: column-reverse` behavior rigorously across target browsers (Chrome, Firefox, Safari, Edge). Safari, in particular, sometimes has quirks with scroll anchoring and layout. Browser's native scroll anchoring might interfere or assist; understand its behavior.
- **Event Timing**: The exact order and timing of `scroll`, `resize`, and DOM mutation events can vary slightly between browsers. Robust logic should not depend on precise micro-timing between different event types.
- **Rendering Optimizations**: Be aware of browser-specific rendering optimizations (like content visibility) that might affect element measurements if not handled carefully.
- **BROWSER DETECTION**: Avoid browser-specific sniffing if possible. Rely on feature detection. Implement browser-specific workarounds *only* as a last resort for known, unavoidable bugs, with clear documentation and targeted application.

#### 5.3.2 Performance Degradation Scenarios
- **Large Log Volumes**: Ensure performance remains acceptable with tens or hundreds of thousands of entries. Virtualization is key here. Test Wasm thresholds and performance gains.
- **Limited Resources**: Test on lower-spec devices or use browser developer tools to simulate CPU throttling and reduced memory. Ensure the UI remains usable, even if slower. Graceful degradation might involve disabling cosmetic animations or using Wasm more conservatively.
- **Slow Connections/Bursty Logs**: Handle logs arriving infrequently or in large bursts without freezing the UI. Batch processing and asynchronous operations are important. Ensure scroll logic behaves correctly when many logs arrive at once.

### 5.4 WebAssembly-Specific Edge Cases
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

## 6. Feature Integration Specifications

### 6.1 Virtualization Compatibility

#### 6.1.1 Auto-Scroll with Virtualization
- **Scroll Position Management**: When auto-scroll is ON, the browser's native behavior is insufficient with virtualization. After new logs are added (and potentially processed by Wasm), explicitly calculate the scroll position required to show the very latest items at the bottom (visually, so `scrollTop = 0` in column-reverse) and apply it programmatically. Ensure this happens *after* the virtualizer has updated the rendered items.
- **Render Window Adjustment**: The virtualization logic must ensure that when auto-scroll is ON, the "render window" (the subset of logs actually in the DOM) includes the newest log entries.
- **Performance Considerations**: Minimize layout thrashing during rapid updates. Ensure virtualization calculations (potentially using Wasm's `findLogAtScrollPosition` or `recalculatePositions`) are efficient.
- **STATE CONSISTENCY**: Ensure virtualization updates (changing the rendered items) don't inadvertently trigger scroll handlers that disable auto-scroll. Programmatic scrolls initiated by the virtualizer or auto-scroll logic must use the `isProgrammaticScroll` flag.

#### 6.1.2 VAS with Virtualization
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
      const clientHeight = scrollContainer ? scrollContainer.clientHeight : 0;
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

### 6.2 Log Filtering & Manipulation

#### 6.2.1 Filter Application
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

        if (autoScroll && !isUserScrolling && !manualScrollLock) {
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

#### 6.2.2 Log Truncation & Clearing
- **State Preservation**: Maintain the `autoScroll` setting when logs are cleared or truncated.
- **Position Recalculation**:
  - If logs are cleared: Scroll position naturally goes to 0 (`scrollTop = 0`). If auto-scroll was OFF, it might remain OFF but the view is now empty or at the top/bottom.
  - If logs are truncated (e.g., keeping only the last N logs):
    - If auto-scroll ON: Remain at the bottom (`scrollTop = 0`).
    - If auto-scroll OFF: The current view might be removed. Use VAS to try and find a relevant position in the remaining logs. If the view was near the *end* of the truncated logs, try to maintain that relative position. If the view was in the *removed* portion, scrolling to the top (`scrollTop = scrollHeight - clientHeight`) of the remaining logs might be the most sensible default.
- **Empty State Handling**: Ensure the component functions correctly when all logs are cleared/filtered out.
- **USER NOTIFICATION**: Consider subtle visual feedback or a toast message when large-scale truncation or clearing occurs, especially if it significantly shifts the user's view when auto-scroll is OFF.

### 6.3 WebAssembly Performance Optimization

#### 6.3.1 Optimized Operations
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

#### 6.3.2 Performance Monitoring
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

## 7. User Experience Design

### 7.1 Control Design & Placement

#### 7.1.1 Auto-Scroll Toggle
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
        checked={autoScroll}
        on:change={(event) => setAutoScroll(event.currentTarget.checked, 'userInteraction:checkbox')}
      />
      <label for="auto-scroll-checkbox" class="cursor-pointer">
        Auto-scroll
      </label>
    </div>
    <p id="auto-scroll-description" class="sr-only">Automatically scroll to the newest logs</p>
    ```

#### 7.1.2 Supplementary Controls
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

### 7.2 Scrolling Aesthetics
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

### 7.3 Performance Perception
- **Responsiveness Priority**: The UI thread must remain responsive. Ensure log processing (parsing, filtering, preparing for render), especially large batches or Wasm operations, doesn't block the main thread for extended periods. Use web workers for complex parsing if necessary, or ensure Wasm operations yield if extremely long-running (though Wasm runs synchronously on the main thread unless in a worker).
- **Progressive Loading**: If dealing with extremely large logs loaded initially, fetch and render them incrementally rather than all at once.
- **Background Processing**: Defer non-critical work (like updating secondary indices or detailed metrics) using `requestIdleCallback` or `setTimeout(..., 0)`.
- **VIRTUALIZATION THRESHOLD**: Implement virtualization based on log count (e.g., enable above 500-1000 logs) to maintain performance.

## 8. Implementation Approach

### 8.1. Unified Auto-Scroll State Management

```javascript
// In Svelte component script

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
  // NOTE: Checkbox UI update is handled by Svelte's reactive binding: checked={autoScroll}

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
      if (debug) console.log(`AutoScroll OFF: Saved anchor`, viewportAnchor);
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
      // updateVirtualization(0);
    }
  });
}

function saveViewportAnchor() {
  // Implementation depends on virtualization state
  // If virtual: Find center item index + offset
  // If not virtual: Find center element + offset from viewport top
  // Store in `viewportAnchor`
  // ... implementation needed ...
  viewportAnchor = { /* ... captured data ... */ };
}

function restoreViewportAnchor() {
  if (!viewportAnchor || !scrollContainer) return;
  if (debug) console.log("Attempting to restore viewport anchor:", viewportAnchor);

  withProgrammaticScroll(() => {
    // Implementation depends on virtualization state and anchor data
    // Calculate target scrollTop based on anchor data
    // ... implementation needed ...
    let targetScrollTop = calculateScrollTopForAnchor(viewportAnchor);

    // Clamp and apply
    const totalHeight = scrollContainer.scrollHeight;
    const clientHeight = scrollContainer.clientHeight;
    const maxScrollTop = Math.max(0, totalHeight - clientHeight);
    targetScrollTop = Math.max(0, Math.min(targetScrollTop, maxScrollTop));

    if (Math.abs(scrollContainer.scrollTop - targetScrollTop) > 1) {
       if (debug) console.log(`Restoring scroll top to ${targetScrollTop}`);
       scrollContainer.scrollTop = targetScrollTop;
       // If virtualized, may need to trigger virtualizer update
       // updateVirtualization(targetScrollTop);
    } else {
       if (debug) console.log(`Anchor restore skipped, already near target (${scrollContainer.scrollTop} ≈ ${targetScrollTop})`);
    }
  });
  // Clear anchor after successful restore? Maybe not, keep it until next save/disable.
}

// --- Toast Feedback ---
function showAutoScrollToastFeedback(message) {
  // (Implementation as shown in 7.1.2)
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
  }, 50);
});

onDestroy(() => {
  // Cleanup listeners, timers, observers
  if (manualScrollLockTimer) clearTimeout(manualScrollLockTimer);
  if (userScrollTimeout) clearTimeout(userScrollTimeout);
  if (scrollRAF) cancelAnimationFrame(scrollRAF);
  if (batchMeasurementTimer) clearTimeout(batchMeasurementTimer);
  if (autoScrollToastTimer) clearTimeout(autoScrollToastTimer);
  // Disconnect observers...
});
```

### 8.2. Scroll Event Handling with Proper Throttling

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
      setAutoScroll(false, 'userScrollAway');
      // IMPORTANT: Auto-scroll is now OFF, subsequent logic should respect this.
      // Capture anchor position *now* as user defined it by scrolling.
      saveViewportAnchor();
      if (debug) console.log(`Saved anchor immediately after user scroll away`, viewportAnchor);
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
      if (debug) console.log("User scrolled to bottom, re-enabling auto-scroll.");
      // setAutoScroll(true, 'scrolledToBottom'); // UNCOMMENT FOR OPTION 2
    }
  }
}

// Add listener in onMount:
// scrollContainer?.addEventListener('scroll', handleScroll, { passive: true });
// Remove listener in onDestroy:
// scrollContainer?.removeEventListener('scroll', handleScroll);
```

### 8.3. Reactive Log Addition Handling

```javascript
// In Svelte component script
import { tick } from 'svelte';

let filteredLogs = []; // Assume this is reactively updated
let recentlyAddedLogs = new Set(); // For highlighting new logs
let pendingMeasurements = false; // Flag for batching post-update work

// $: Reactive statement triggered when `filteredLogs` changes
$: if (filteredLogs && filteredLogs.length >= 0 && scrollContainer) { // Check length >= 0 for clear events
  const logsChanged = true; // More sophisticated check if needed (e.g., compare length or content hash)

  if (logsChanged) {
    if (autoScroll) {
      // --- Auto-Scroll ON ---
      // Browser might handle it, but enforce for reliability, especially with virt/filtering.
      // Only enforce if user isn't interacting and no animations are running.
      if (!isUserScrolling && !manualScrollLock && !animationInProgress) {
        // Needs to run *after* DOM update. Schedule it.
        setTimeout(async () => {
          await tick(); // Wait for Svelte DOM update
          // Double check state in case it changed during timeout/tick
          if (autoScroll && !isUserScrolling && !manualScrollLock && scrollContainer) {
            // Use RAF for final timing before paint
            requestAnimationFrame(() => {
              if (autoScroll && !isUserScrolling && !manualScrollLock && scrollContainer) {
                 // Check if we are already visually at the bottom due to browser behavior
                 if (Math.abs(scrollContainer.scrollTop) > 1) {
                    if(debug) console.log("AutoScroll ON: Enforcing scroll to bottom after log update.");
                    scrollToBottom();
                 } else {
                    if(debug) console.log("AutoScroll ON: Already at bottom after log update.");
                 }
              }
            });
          }
        }, 0); // Schedule slightly after current execution context
      } else {
        if(debug) console.log("AutoScroll ON: Skipped enforcing bottom scroll due to user interaction/animation.");
      }
    } else {
      // --- Auto-Scroll OFF ---
      // Need to maintain position using VAS. Anchor should have been saved *before*
      // the log update process began if the update wasn't triggered by user scroll.
      // Here, we schedule the *restoration* after the DOM updates.

      // Mark new logs for potential highlighting (if not virtualizing heavily)
      // (Implementation depends on log structure and highlighting approach)
      // markRecentlyAddedLogs(filteredLogs);

      // Set animation flag if using CSS transitions for new logs
      // if (usingLogEntryAnimations) { animationInProgress = true; ... setTimeout clear ... }

      // Schedule measurements and anchor restoration after DOM updates
      if (!pendingMeasurements) {
        pendingMeasurements = true;
        // Use setTimeout 0 or RAF to batch potential rapid updates
        batchMeasurementTimer = window.setTimeout(async () => {
          await tick(); // Ensure Svelte DOM updates are finished

          // Perform any necessary measurements after updates (e.g., total height)
          // recalculatePositionsIfNeeded();

          // Update virtualization if needed based on new content/height
          // if (virtualizationReady && virtualEnabled) { updateVirtualization(); }

          // Restore position *only if* conditions still hold
          if (!autoScroll && viewportAnchor && !isUserScrolling && !manualScrollLock && !animationInProgress) {
            restoreViewportAnchor();
          } else {
            if(debug) console.log("AutoScroll OFF: Skipped anchor restore due to state change/interaction/animation.");
          }

          pendingMeasurements = false;
          batchMeasurementTimer = null; // Clear timer ID
        }, 10); // Small delay for batching
      }
    }
  }
}

// Function to mark logs (example)
function markRecentlyAddedLogs(logs) {
  if (virtualEnabled) return; // Skip if virtualizing heavily
  const now = Date.now();
  logs.forEach(log => {
    // Assuming log has a timestamp field `_unix_time` and a unique `_sequence`
    if (log._unix_time && now - log._unix_time < 1000 && log._sequence) {
      if (!recentlyAddedLogs.has(log._sequence)) {
        recentlyAddedLogs.add(log._sequence);
        // Remove from set after animation duration
        setTimeout(() => {
          recentlyAddedLogs.delete(log._sequence);
          // Trigger reactivity if needed for CSS class removal
        }, 1500); // Match CSS animation duration
      }
    }
  });
  // Trigger reactivity if needed: recentlyAddedLogs = recentlyAddedLogs;
}
```

### 8.4. WebAssembly-Enhanced Log Processing

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

  const totalLogCount = existingLogs.length + newLogs.length;

  // Handle trivial cases
  if (newLogs.length === 0) return existingLogs;
  if (existingLogs.length === 0) return newLogs.sort((a, b) => a._unix_time - b._unix_time); // Ensure new logs are sorted if needed

  let result = null;
  let wasmTime = -1, tsTime = -1, serializeTime = -1, deserializeTime = -1;
  let usedWasm = false;

  // --- Decide whether to use WebAssembly ---
  if (shouldUseWasm(totalLogCount, operation)) {
    try {
      const wasmModule = getWasmModule(); // Get loaded Wasm instance
      if (!wasmModule || !wasmModule.merge_insert_logs) { // Check function exists
        throw new Error("WebAssembly module or merge_insert_logs function not available");
      }

      // --- Prepare Data for Wasm ---
      const serializeStart = performance.now();
      // Combine logs first might be simpler for some Wasm implementations
      const combinedLogs = [...existingLogs, ...newLogs];
      const serialized = serializeLogsForWasm(combinedLogs); // Convert logs to format Wasm expects (e.g., Float64Array)
      serializeTime = performance.now() - serializeStart;

      // --- Call Wasm Function ---
      const wasmStart = performance.now();
      // Example: Wasm function takes pointers/offsets into memory
      // Adjust call based on actual Wasm function signature
      const wasmResultPtr = wasmModule.merge_insert_logs(serialized.pointer, combinedLogs.length);
      wasmTime = performance.now() - wasmStart;

      // --- Process Result from Wasm ---
      const deserializeStart = performance.now();
      const deserialized = deserializeLogsFromWasm(wasmResultPtr, wasmModule.memory); // Convert result back
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
    if (debug && usedWasm) console.log(`Falling back to TS for ${operation}`);
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

## 9. Testing & Validation Requirements

### 9.1 Critical Test Scenarios
- **Rapid Log Addition**: Simulate adding logs at high frequency (e.g., 50-200 logs/sec) in bursts and continuously. Verify UI responsiveness, correct auto-scroll behavior, and stable VAS positioning. Test with and without Wasm active.
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

### 9.2 User Interaction Testing
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

### 9.3 WebAssembly Integration Testing
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

          // Force Wasm (mock shouldUseWasm or use settings)
          forceWasmUsage(true);
          const wasmResult = mergeInsertLogs([...existingLogs], [...newLogs]); // Use copies

          // Force TS (mock shouldUseWasm or use settings)
          forceWasmUsage(false);
          const tsResult = mergeInsertLogs([...existingLogs], [...newLogs]); // Use copies

          // Deep comparison of the resulting log arrays
          // Ensure logs are compared by content and order
          expect(wasmResult).toEqual(tsResult);

          // Reset Wasm usage forcing
          resetWasmUsageForcing();
        });
      });
    });
    ```

## 10. Success Criteria

### 10.1 Functional Requirements
- Auto-scroll correctly follows the newest logs (maintains `scrollTop` near 0) when enabled and user is not interacting.
- Viewport remains stable (within a few pixels tolerance) relative to the viewed content when auto-scroll is disabled and logs are added/removed outside the view.
- Transitions between auto-scroll ON/OFF (via checkbox or implicit user scroll) are reliable, immediate, and predictable.
- Core scrolling system behaves consistently and correctly across latest versions of Chrome, Firefox, Safari, and Edge.
- The auto-scroll checkbox UI state accurately reflects the internal `autoScroll` state variable at all times.
- User scrolling (mouse, keyboard, touch) feels intuitive and correctly interacts with the auto-scroll state in the `column-reverse` layout.

### 10.2 Performance Requirements
- UI thread remains responsive (no freezes > 100ms) during rapid log additions (target: 50-200 logs/second) and filtering operations on large datasets (e.g., 100k logs).
- Scroll event handling and associated logic (position calculations, state updates) complete well within a single frame budget (< 16ms).
- Memory usage remains stable and does not grow unbounded during long-running sessions with continuous log additions (no memory leaks from listeners, observers, state).
- CPU utilization remains reasonable during typical usage and does not spike excessively during background processing or Wasm operations.
- Clear debuggability: System provides useful console logs (when debug flag is enabled) for state transitions, event handling, Wasm decisions, and errors.
- **WebAssembly Optimization**: WebAssembly implementations provide statistically significant and measurable performance improvements for targeted operations on relevant data sizes (compared to optimized TS fallbacks):
  - Target Speedup (Examples - adjust based on profiling):
    - Small datasets (near threshold, e.g., 500-1k logs): > 1.5× faster
    - Medium datasets (e.g., 5k-10k logs): > 3× faster
    - Large datasets (e.g., 50k+ logs): > 5-8× faster
- **Adaptive Efficiency**: The adaptive threshold system demonstrates ability to adjust thresholds based on measured performance, favoring Wasm when it's significantly faster and TS when speedup is marginal or negative.

### 10.3 User Experience Requirements
- Controls (checkbox, potential buttons) are easily discoverable, understandable, and behave as expected.
- Visual feedback (toast messages, new log highlights) clearly and unobtrusively communicates the system's state and actions (auto-scroll enabled/disabled/paused).
- Scrolling feels smooth and predictable; no unexpected jumps or jarring movements, especially when auto-scroll is OFF (VAS active).
- Auto-scroll behavior feels natural – it stays on when expected, pauses intuitively when the user scrolls away, and (if Option 2 implemented) resumes predictably when scrolling back to the bottom.
- The user always feels in control; user-initiated scrolls immediately override automated behaviors.

### 10.4 Integration Requirements
- WebAssembly integration is seamless; the application functions identically (except for performance) whether Wasm is active or falling back to TypeScript.
- TypeScript fallbacks are robust and correctly handle all cases when WebAssembly is unavailable or fails.
- Wasm error recovery (blacklisting, fallback) works transparently to the user, maintaining application stability.
- Performance monitoring provides accurate data for diagnosing issues and verifying Wasm effectiveness. This data can be included in diagnostic reports.
- Coordinate transformations required for `column-reverse` layout are handled correctly and consistently in both JavaScript logic and data passed to/from WebAssembly functions.
- Wasm state (enabled, errors, performance summary) integrates with any existing application-wide state management and backend crash/error reporting systems.
