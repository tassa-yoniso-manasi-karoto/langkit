# Enhanced Design Specification: LogViewer Scrolling System with flex-direction: column-reverse CSS property

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

### 1.2 Architectural Philosophy
- **Hybrid Approach**: Leverage native browser behaviors where reliable, implement explicit controls where needed
- **Separation of Concerns**: Clearly delineate between auto-scroll logic, viewport anchoring, and user input handling
- **Defensive Implementation**: Anticipate and gracefully handle race conditions, rapid state changes, and edge cases
- **Clear State Management**: Maintain unambiguous internal state that accurately reflects the visual representation
- **State Isolation**: UI state (e.g., checkbox visibility) should be decoupled from behavioral state (e.g., auto-scroll functionality)
- **Command-Based Updates**: Use a command pattern for critical state changes to ensure proper sequencing
- **Reactive Statement Discipline**: Exercise extreme caution with reactive statements (`$:`) that modify scroll position or DOM state

## 2. System Components & Relationships

### 2.1 Core Conceptual Model
- **Auto-Scroll Mode**: A binary state determining whether the view should automatically follow newest logs
- **Viewport Anchoring System (VAS)**: A position preservation mechanism that maintains stable viewing experience
- **Scroll Event Management**: A system for differentiating between user and programmatic scroll events
- **Virtualization Integration**: Specialized behavior modifications when dealing with virtualized content

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
- **CRITICAL CAUTION**: Avoid circular dependencies where state changes trigger DOM updates that trigger further state changes

### 2.4 Reactive Statement Safety
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
          isUserScrolling = false;
        }, 1000); // Longer timeout for reliable user reading
        
        // Continue with scroll handling...
      }
      ```

- **OFF → ON (Optional Consideration)**:
  - When user manually scrolls to bottom with auto-scroll OFF:
    - *Option 1*: Maintain OFF state (requires explicit user checkbox action)
    - *Option 2*: Automatically re-enable auto-scroll (more automated but potentially unexpected)
    - Design recommendation: Implement Option 1 for predictability, with clear visual cues to re-enable
    - **CRITICAL**: If implementing Option 2, apply a subtle timing delay to avoid accidental triggers

### 3.2 Viewport Anchoring System (VAS) Behavior

#### 3.2.1 Fundamental Operation
- **When Active**: Only when auto-scroll is OFF
- **Purpose**: Maintain stable viewing position during log additions and container changes
- **Core Process**:
  1. Before DOM updates: Capture position reference relative to a stable element
  2. After DOM updates: Calculate new position and restore viewport to equivalent position
  3. Apply position preservation only when appropriate (not during user scrolling)
- **CRITICAL RISK**: In a column-reverse layout, unconditional viewport restoration can create unintended auto-scroll behavior
- **MITIGATION**: Explicitly check auto-scroll state before applying viewport anchoring:
  ```javascript
  // SAFER APPROACH
  $: if (filteredLogs.length > 0) {
    if (autoScroll) {
      // Use direct approach for auto-scroll ON
      viewportAnchor = null; // Clear any anchor
      if (!isUserScrolling && !manualScrollLock) {
        scrollToBottom(); // Simple direct scroll
      }
    } else {
      // Use viewport anchoring only when auto-scroll is OFF
      saveViewportAnchor();
      // ...later after DOM updates...
      if (viewportAnchor && !isUserScrolling && !manualScrollLock) {
        restoreViewportAnchor();
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
- **PERFORMANCE OPTIMIZATION**: For WebAssembly functions, transform coordinates once before passing data rather than transforming each coordinate individually

### 3.3 Log Update & Rendering Flow

#### 3.3.1 With Auto-Scroll ON
- **Expected Behavior**: View remains at newest logs (bottom)
- **Primary Mechanism**: Browser's natural tendency to maintain scrollTop=0 in column-reverse layout
- **Safety Mechanism**: Explicit scrollTop=0 enforcement after updates when necessary, particularly:
  - After filtering operations
  - With virtualization enabled
  - After significant layout changes
  - Following browser inconsistencies
- **TIMING CONSIDERATION**: Use `requestAnimationFrame` for scroll position enforcement to ensure it happens after rendering

#### 3.3.2 With Auto-Scroll OFF
- **Expected Behavior**: View maintains stable position relative to existing content
- **Primary Mechanism**: VAS captures position before update, restores equivalent position after
- **Critical Timing**: Position restoration must occur after DOM updates are complete (Svelte tick)
- **Interference Prevention**: Skip position restoration during active user scrolling
- **ANIMATION AWARENESS**: Track animation state to defer scroll operations until animations complete:
  ```javascript
  let animationInProgress = false;
  let pendingScrollToBottom = false;
  
  function handleTransitionStart() {
    animationInProgress = true;
  }
  
  function handleTransitionEnd() {
    animationInProgress = false;
    
    // Execute deferred operations
    if (pendingScrollToBottom && autoScroll && !isUserScrolling) {
      pendingScrollToBottom = false;
      executeScrollToBottom();
    }
  }
  ```

## 4. Event Handling & Coordination

### 4.1 Scroll Event Management

#### 4.1.1 Event Categorization
- **User-Initiated Scrolling**: Direct interaction requiring state changes
- **Programmatic Scrolling**: System-initiated scroll requiring exclusion from feedback loops
- **Momentum/Inertial Scrolling**: Post-interaction scrolling requiring special timing considerations
- **CRITICAL SEPARATION**: Distinguish between user and programmatic scrolling with explicit flags:
  ```javascript
  let isProgrammaticScroll = false;
  
  function withProgrammaticScroll(callback) {
    isProgrammaticScroll = true;
    try {
      callback();
    } finally {
      // Use RAF to ensure flag is cleared after browser processes the scroll
      requestAnimationFrame(() => {
        isProgrammaticScroll = false;
      });
    }
  }
  
  function scrollToBottom() {
    withProgrammaticScroll(() => {
      scrollContainer.scrollTop = 0; // In column-reverse, 0 is bottom
    });
  }
  ```

#### 4.1.2 Scroll Cycle Behavior
- **Start**: Mark active scrolling, prevent competing operations
- **During**: Update internal state, track direction and extent
- **End (Debounced)**: Re-enable normal operations, evaluate position for potential state changes
- **Threshold Values**: Use small tolerance (1px) for "at bottom" detection to account for precision issues
- **PERFORMANCE CRITICAL**: Throttle scroll handlers for better performance:
  ```javascript
  function handleScroll() {
    // Skip if programmatic
    if (isProgrammaticScroll) return;
    
    // Set user scrolling flag
    isUserScrolling = true;
    
    // Throttle with requestAnimationFrame
    if (scrollRAF) cancelAnimationFrame(scrollRAF);
    scrollRAF = requestAnimationFrame(() => {
      // Implement scroll logic here
      const { scrollTop } = scrollContainer;
      
      // Check auto-scroll conditions
      // ...
      
      scrollRAF = null;
    });
    
    // Set timeout to mark scrolling complete
    if (userScrollTimeout) clearTimeout(userScrollTimeout);
    userScrollTimeout = setTimeout(() => {
      isUserScrolling = false;
      // Check final position
    }, 300);
  }
  ```

### 4.2 Resize Event Handling
- **Container Resizing**: Recalculate dimensions and maintain appropriate scroll position
- **Window Resizing**: Adjust virtualization parameters while preserving view stability
- **Content Height Changes**: Recalculate total height and adjust scroll position proportionally
- **MEMORY MANAGEMENT**: Properly disconnect observers to prevent memory leaks:
  ```javascript
  const cleanupFunctions = [];
  
  function registerCleanup(fn) {
    cleanupFunctions.push(fn);
  }
  
  onMount(() => {
    const resizeObserver = new ResizeObserver(handleResize);
    resizeObserver.observe(element);
    
    registerCleanup(() => resizeObserver.disconnect());
  });
  
  onDestroy(() => {
    // Clean everything up
    cleanupFunctions.forEach(cleanup => cleanup());
  });
  ```

### 4.3 DOM Lifecycle Integration
- **Before DOM Updates**: Capture position references
- **After DOM Updates**: Apply position restoration only after rendering complete
- **Batched Operations**: Group multiple operations to reduce performance impact
- **ASYNCHRONOUS AWARENESS**: Always use `await tick()` before measuring or manipulating scroll position after state changes
- **SVELTE REACTIVITY CAUTION**: Be aware of prop reactivity limitations in Svelte components

## 5. Edge Cases & Robustness Measures

### 5.1 Race Conditions & Timing Issues

#### 5.1.1 Rapid Interaction Sequences
- **Rapid Checkbox Toggling**: Deduplicate closely-timed transitions, honor latest user intent
- **Scrolling During Transitions**: Prioritize direct user interaction, cancel competing operations
- **Updates During Scrolling**: Delay position-affecting operations until scrolling stabilizes
- **Concurrent Operations**: Establish clear operation precedence hierarchy with user actions at highest priority:
  1. User scrolling takes highest priority
  2. Programmatic operations are deferred during user interaction
  3. Position maintenance happens only after rendering is complete

#### 5.1.2 Animation & Transition Timing
- **CSS Transitions**: Account for active transitions when calculating positions
- **Svelte Animations**: Ensure animations complete before applying critical position logic
- **Browser Painting**: Allow sufficient time between measurement and positioning (requestAnimationFrame)
- **TRANSITION TRACKING**: Implement explicit counters for tracking active transitions:
  ```javascript
  let activeTransitions = 0;
  
  function handleTransitionStart() {
    activeTransitions++;
    animationInProgress = true;
  }
  
  function handleTransitionEnd() {
    activeTransitions--;
    
    // Only set animation complete when all transitions are done
    if (activeTransitions <= 0) {
      activeTransitions = 0;
      animationInProgress = false;
      
      // Execute pending operations
      if (pendingScrollToBottom && autoScroll && !isUserScrolling) {
        pendingScrollToBottom = false;
        executeScrollToBottom();
      }
    }
  }
  ```

### 5.2 Initialization & Edge States

#### 5.2.1 Component Initialization
- **Initial Rendering**: Establish default auto-scroll state (ON recommended)
- **Asynchronous Loading**: Handle gracefully when logs arrive after initial render
- **Empty State**: Manage properly when log container starts empty
- **Cold Start**: Apply default behaviors before user has expressed preference
- **LIFECYCLE AWARENESS**: Handle component mounting/unmounting gracefully:
  ```javascript
  onMount(() => {
    // Initial state setup
    autoScroll = true; // Default to ON
    
    // Initial DOM setup
    setTimeout(async () => {
      await tick();
      if (autoScroll && scrollContainer && !isUserScrolling) {
        scrollContainer.scrollTop = 0; // Initial scroll to bottom
      }
    }, 100);
  });
  
  onDestroy(() => {
    // Clean up all resources
    cleanupFunctions.forEach(fn => fn());
    
    // Clear all timers
    if (userScrollTimeout) clearTimeout(userScrollTimeout);
    if (scrollRAF) cancelAnimationFrame(scrollRAF);
    if (batchMeasurementTimer) clearTimeout(batchMeasurementTimer);
    // Other cleanup...
  });
  ```

#### 5.2.2 Focus & Accessibility Considerations
- **Keyboard Navigation**: Maintain expected behavior with keyboard scrolling
- **Screen Readers**: Ensure proper ARIA attributes and focus management
- **Tab Visibility**: Handle background tab updates appropriately
- **ACCESSIBILITY ENHANCEMENT**: Add appropriate ARIA roles and states to scrollable container and controls

### 5.3 Browser & Environment Variations

#### 5.3.1 Browser-Specific Behaviors
- **Scroll Position Maintenance**: Some browsers may handle column-reverse differently
- **Event Timing**: Different browsers have different event ordering for scroll, resize
- **Rendering Optimizations**: Account for browser-specific layout and paint timing
- **BROWSER DETECTION**: Implement browser-specific workarounds only where absolutely necessary and with clear documentation

#### 5.3.2 Performance Degradation Scenarios
- **Large Log Volumes**: Maintain responsiveness with thousands of entries
- **Limited Resources**: Degrade gracefully on low-power devices
- **Slow Connections**: Handle properly when logs arrive in bursts
- **PERFORMANCE MONITORING**: Add optional performance tracking in development mode

## 6. Feature Integration Specifications

### 6.1 Virtualization Compatibility

#### 6.1.1 Auto-Scroll with Virtualization
- **Scroll Position Management**: Explicit bottom positioning required after virtualization updates
- **Render Window Adjustment**: Ensure newest logs are within virtual window when auto-scroll ON
- **Performance Considerations**: Minimize layout thrashing during rapid updates
- **STATE CONSISTENCY**: Ensure virtualization changes don't interfere with auto-scroll state

#### 6.1.2 VAS with Virtualization
- **Anchor Strategy Adaptation**: Use index-based anchoring rather than DOM elements
- **Calculations Adjustment**: Account for virtual content that doesn't exist in DOM
- **Position Estimation**: Handle gracefully when exact positions cannot be determined
- **COORDINATE TRANSFORMATION**: Handle column-reverse coordinate systems consistently:
  ```javascript
  function findLogAtScrollPosition(scrollTop, scrollMetrics) {
    // In column-reverse, we need to adjust the scrollTop value
    // Convert from scrollTop to a position from the top of content
    const adjustedScrollPosition = scrollContainer ? 
        (totalLogHeight - scrollContainer.clientHeight - scrollTop) : 
        scrollTop;
        
    // Use adjusted position for calculations...
  }
  ```

### 6.2 Log Filtering & Manipulation

#### 6.2.1 Filter Application
- **Position Handling**: Re-evaluate scroll position after filter changes
- **Auto-Scroll Behavior**: Maintain current auto-scroll state through filter operations
- **Anchor Invalidation**: Reset anchors that may no longer be valid after filtering
- **CRITICAL TIMING**: Apply special timing considerations for filter changes:
  ```javascript
  $: if (selectedLogLevel !== previousLogLevel) {
    filterTransitionRunning = true;
    
    // Save viewport anchor before filter change
    if (!autoScroll) {
      saveViewportAnchor();
    }
    
    previousLogLevel = selectedLogLevel;
    
    // After animation completes, restore scroll position
    setTimeout(async () => {
      await tick(); // Ensure DOM is updated
      filterTransitionRunning = false;
      recalculatePositions();
      
      // Apply appropriate scrolling based on auto-scroll state
      if (autoScroll && !isUserScrolling) {
        scrollToBottom();
      } else if (viewportAnchor) {
        restoreViewportAnchor();
      }
    }, 300); // Match with animation duration
  }
  ```

#### 6.2.2 Log Truncation & Clearing
- **State Preservation**: Maintain auto-scroll setting through content clearing
- **Position Recalculation**: Adjust scroll position when logs are truncated
- **Empty State Handling**: Manage gracefully when all logs are cleared/filtered
- **USER NOTIFICATION**: Provide clear visual feedback for major content changes

## 7. User Experience Design

### 7.1 Control Design & Placement

#### 7.1.1 Auto-Scroll Toggle
- **Checkbox Presentation**: Clear labeling indicating purpose
- **Placement**: Easily accessible within log viewer controls
- **State Indication**: Visually represent current state beyond checkbox
- **CRITICAL IMPLEMENTATION**: Use correct HTML structure for reliable behavior:
  ```svelte
  <!-- CORRECT: Single update path -->
  <div class="flex items-center gap-1">
    <input 
      id="auto-scroll-checkbox"
      type="checkbox" 
      checked={autoScroll}
      on:change={(e) => setAutoScroll(e.target.checked, 'userInteraction')}
      class="accent-primary"
    />
    <label for="auto-scroll-checkbox">
      Auto-scroll
    </label>
  </div>
  ```

#### 7.1.2 Supplementary Controls
- **Scroll to Bottom Button**: Present when auto-scroll OFF and not at bottom
- **Log Navigation Aids**: Consider additional navigation controls for large log sets
- **Visual Indicators**: Subtle indications of log additions and state changes
- **USER FEEDBACK**: Provide toast notifications for state changes:
  ```javascript
  function showAutoScrollToastMessage(message) {
    // Clear any existing timer
    if (autoScrollToastTimer) {
      clearTimeout(autoScrollToastTimer);
    }
    
    // Set message and show toast
    autoScrollToastMessage = message;
    showAutoScrollToast = true;
    
    // Hide after 2 seconds
    autoScrollToastTimer = window.setTimeout(() => {
      showAutoScrollToast = false;
      autoScrollToastTimer = null;
    }, 2000);
  }
  ```

### 7.2 Scrolling Aesthetics
- **Scrolling Animation**: Use instant scrolling for auto-scroll operations to avoid conflicts
- **Scrollbar Appearance**: Standard scrollbar for consistent user expectations
- **Visual Feedback**: Subtle highlighting for new logs (especially when auto-scroll OFF)
- **ANIMATION CONFLICTS**: Explicitly disable smooth scrolling to prevent animation conflicts:
  ```css
  .log-scroll-container {
    /* Override default smooth scrolling */
    scroll-behavior: auto !important;
  }
  ```

### 7.3 Performance Perception
- **Responsiveness Priority**: Maintain UI responsiveness even during heavy log processing
- **Progressive Loading**: Consider incremental rendering for very large log sets
- **Background Processing**: Perform expensive operations off the main thread when possible
- **VIRTUALIZATION THRESHOLD**: Automatically enable virtualization at appropriate log count

## 8. Implementation Approach

### 8.1. Unified Auto-Scroll State Management

```javascript
// Single source of truth
let autoScroll = true; // Start enabled by default

// The ONLY function that should modify auto-scroll state
function setAutoScroll(newValue: boolean, source: string = 'direct'): void {
  // Skip if no change
  if (newValue === autoScroll) return;
  
  // Debug logging
  if (debug) console.log(`Auto-scroll ${newValue ? 'enabled' : 'disabled'} via ${source}`);
  
  // Update our state variable
  autoScroll = newValue;
  
  // Sync UI checkbox
  const checkbox = document.getElementById('auto-scroll-checkbox') as HTMLInputElement;
  if (checkbox) {
    checkbox.checked = newValue;
  }
  
  // Additional state updates when needed
  if (newValue) {
    // When enabling auto-scroll:
    viewportAnchor = null; // Clear any saved position
    
    // Force scroll to bottom with direct DOM manipulation
    if (scrollContainer && !isUserScrolling && !manualScrollLock) {
      withProgrammaticScroll(() => {
        scrollContainer.scrollTop = 0; // In column-reverse, 0 is bottom
      });
    }
  } else {
    // When disabling auto-scroll, save position for restoration
    if (source !== 'userScrollAway') { // Don't save if already handled
      saveViewportAnchor();
    }
  }
  
  // Show visual confirmation
  showAutoScrollToastMessage(newValue ? 
    "Auto-scroll enabled" : 
    "Auto-scroll disabled - scroll to bottom to re-enable");
}
```

### 8.2. Scroll Event Handling with Proper Throttling

```javascript
function handleScroll(): void {
  // Always ignore programmatic scrolling
  if (isProgrammaticScroll) return;
  
  // Mark as user scrolling - SET THIS FLAG IMMEDIATELY
  isUserScrolling = true;
  
  // IMPORTANT: Set manual scroll lock to prevent auto-scroll from fighting with user
  manualScrollLock = true;
  
  // Cancel any existing timers/animations
  if (scrollRAF) cancelAnimationFrame(scrollRAF);
  if (manualScrollLockTimer) {
    clearTimeout(manualScrollLockTimer);
  }
  
  // Reset the manual scroll lock after a period (3 seconds)
  // This gives user plenty of time to read without auto-scroll interfering
  manualScrollLockTimer = window.setTimeout(() => {
    manualScrollLock = false;
    manualScrollLockTimer = null;
  }, 3000);
  
  // Use RAFrame for precise timing
  scrollRAF = requestAnimationFrame(() => {
    if (!scrollContainer) {
      scrollRAF = null;
      return;
    }
    
    // Get absolute value of scrollTop
    const scrollTop = Math.abs(scrollContainer.scrollTop);
    
    // IMPORTANT: If user has scrolled away from bottom and auto-scroll is on,
    // immediately disable auto-scroll before doing anything else
    if (scrollTop > 1 && autoScroll) {
      if (debug) console.warn(`Disabling auto-scroll due to scrollTop=${scrollTop}px`);
      setAutoScroll(false, 'userScrollAway');
    }
    
    // Update scroll metrics and virtualization as needed
    updateScrollMetrics(scrollTop);
    
    // Virtualization update if needed
    if (virtualizationReady && virtualEnabled) {
      updateVirtualization();
    }
    
    // Set a timeout to mark user scrolling complete
    if (userScrollTimeout) clearTimeout(userScrollTimeout);
    userScrollTimeout = window.setTimeout(() => {
      isUserScrolling = false;
      
      // Check if we're exactly at the bottom (scrollTop = 0 in column-reverse)
      if (scrollContainer && Math.abs(scrollContainer.scrollTop) === 0 && !autoScroll) {
        // Option 2: Auto-enable when scrolled to bottom
        setAutoScroll(true, 'scrolledToBottom');
      }
    }, 800); // Longer timeout to ensure truly stable position
    
    scrollRAF = null;
  });
}
```

### 8.3. Reactive Log Addition Handling

```javascript
// Reactive statement - runs when logs change
$: if (filteredLogs.length > 0 && scrollContainer) {
  // Different behavior based on auto-scroll state
  if (autoScroll) {
    // With auto-scroll ON: Let browser maintain position, but enforce bottom
    // position after DOM update for consistency
    if (!isUserScrolling && !manualScrollLock && !animationInProgress) {
      setTimeout(async () => {
        await tick(); // Ensure DOM is updated
        withProgrammaticScroll(() => {
          scrollContainer.scrollTop = 0; // Force to bottom
        });
      }, 10);
    }
  } else {
    // With auto-scroll OFF: Use viewport anchoring to maintain position
    saveViewportAnchor();
    
    // Mark new logs for animation if not virtualizing
    if (!virtualEnabled) {
      const currentTime = Date.now();
      filteredLogs.forEach(log => {
        if (log._unix_time && currentTime - log._unix_time < 500) {
          recentlyAddedLogs.add(log._sequence || 0);
          setTimeout(() => {
            recentlyAddedLogs.delete(log._sequence || 0);
          }, 1000);
        }
      });
      
      // Set animation in progress flag
      animationInProgress = true;
      setTimeout(() => {
        animationInProgress = false;
      }, 350);
    }
    
    // Schedule a batch update
    if (!pendingMeasurements) {
      pendingMeasurements = true;
      if (batchMeasurementTimer) {
        clearTimeout(batchMeasurementTimer);
      }
      
      batchMeasurementTimer = window.setTimeout(async () => {
        await tick(); // Ensure DOM is updated
        recalculatePositions();
        
        // Update virtualization if needed
        if (virtualizationReady && virtualEnabled) {
          updateVirtualization();
        }
        
        // Restore position only when auto-scroll is OFF
        if (!autoScroll && viewportAnchor && !isUserScrolling && !manualScrollLock) {
          restoreViewportAnchor();
        }
        
        pendingMeasurements = false;
        batchMeasurementTimer = null;
      }, 10);
    }
  }
}
```

## 9. Testing & Validation Requirements

### 9.1 Critical Test Scenarios
- **Rapid Log Addition**: Test with high-frequency log additions (50+ per second)
- **Browser Compatibility**: Verify across modern browsers with special attention to Safari
- **Interaction Combinations**: Test rapid scrolling while logs are being added
- **Low-Resource Scenarios**: Test on limited hardware and with constrained memory
- **Long-Running Sessions**: Verify stability with hours of continuous log additions
- **MEMORY LEAK TESTING**: Test for memory leaks by repeatedly mounting/unmounting component
- **CIRCULAR DEPENDENCY TESTS**: Verify no circular update patterns by logging state changes
- **COMPONENT INTERACTION TESTS**: Verify behavior when interacting with other components

### 9.2 User Interaction Testing
- **Natural Usage Patterns**: Test realistic usage sequences
- **Edge Interaction Sequences**: Test rapid toggling, scrolling during transitions
- **Accessibility Testing**: Verify behavior with keyboard navigation and screen readers
- **ASSUMPTION TESTING**: Explicitly test assumptions about browser behavior with column-reverse layouts
- **STATE SYNC VERIFICATION**: Verify checkbox state and auto-scroll behavior stay synchronized

## 10. Success Criteria

### 10.1 Functional Requirements
- Auto-scroll functionality correctly follows newest logs when enabled
- Viewport remains stable at user-selected position when auto-scroll disabled
- Transitions between states are reliable and predictable
- System behaves correctly across supported browsers
- Checkbox state accurately reflects actual auto-scroll behavior at all times
- Scrolling behavior remains intuitive in column-reverse layout

### 10.2 Performance Requirements
- UI remains responsive during rapid log additions (50+ logs/second)
- Scroll operations complete in under 16ms (60fps)
- Memory usage remains stable over long sessions
- CPU utilization remains reasonable during heavy log activity
- No memory leaks from observers or event listeners
- Clear debuggability with intentional logging and state tracking

### 10.3 User Experience Requirements
- Controls are intuitive and match user expectations
- Visual feedback clearly indicates system state
- No unexpected scroll position changes
- Smooth, jank-free scrolling experience
- Auto-scroll behavior feels natural and predictable
- User always maintains ultimate control over viewing position
