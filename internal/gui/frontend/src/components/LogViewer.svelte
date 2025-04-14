<script lang="ts">
    import { onMount, onDestroy, tick, afterUpdate } from 'svelte';
    import { settings } from '../lib/stores';
    import { logStore, type LogMessage } from '../lib/logStore';
    import { slide, fade } from 'svelte/transition';
    import { backOut } from 'svelte/easing';
    import {
        isWasmEnabled,
        shouldUseWasm,
        findLogAtScrollPositionWasm,
        recalculatePositionsWasm,
        handleWasmError,
        isOperationBlacklisted,
        WasmOperationError
    } from '../lib/wasm';
    import { get } from 'svelte/store';

    // Optional version prop to handle dev vs. prod initialization
    export let version: string = "dev";
    // Add isProcessing prop to receive processing state from App.svelte
    export let isProcessing: boolean = false;
    let prevIsProcessing = false;

    // Decide initial log filter
    let selectedLogLevel = version === "dev" ? "DEBUG" : "INFO";
    let previousLogLevel = selectedLogLevel;
    
    // Log levels available
    const logLevels = ['DEBUG', 'INFO', 'WARN', 'ERROR'];

    // Priority map for numeric comparisons
    const logLevelPriority: Record<string, number> = {
        'debug': 1,
        'info': 2,
        'warn': 3,
        'error': 4,
    };

    // Match certain behaviors to text colors using the centralized colors
    const behaviorColors: Record<string, string> = {
        'abort_task': 'text-error-task log-behavior-abort-task',
        'abort_all': 'text-error-all log-behavior-abort-all',
        'user_cancel': 'text-user-cancel log-behavior-cancel',
        'probe': 'text-log-warn log-behavior-probe'
    };

    // DOM references
    let scrollContainer: HTMLElement;
    
    // Track direct scrolling by wrapping direct calls
    function setScrollTop(value: number, source: string = 'unknown') {
        if (!scrollContainer) return;
        
        if (value === 0) {
            // Track the direct calls to scrollTop = 0
            trackScrollTrigger(`setScrollTop:${source}`);
        }
        
        // Actually perform the scroll
        scrollContainer.scrollTop = value;
    }
    
    // Disable debug logs in the log viewer and use console instead
    function debugLog(message: string, level: string = "DEBUG"): void {
        // Only log in dev mode
        if (version === "dev") {
            // Use console logs instead of logStore to avoid the duplicate key issue
            if (level === "ERROR") {
                console.error(`[AUTO-SCROLL] ${message}`);
            } else if (level === "WARN") {
                console.warn(`[AUTO-SCROLL] ${message}`);
            } else if (level === "INFO") {
                console.info(`[AUTO-SCROLL] ${message}`);
            } else {
                console.debug(`[AUTO-SCROLL] ${message}`);
            }
        }
    }
    
    // AUTO-SCROLL STATE MANAGEMENT - SINGLE SOURCE OF TRUTH
    // We use a simple boolean with a controlled setter function
    let autoScroll = true; // Start with auto-scroll enabled
    
    // Debug variables to track auto-scroll state
    let lastAutoScrollChangeSource = 'initial';
    let autoScrollToggleCount = 0;
    
    // Scroll state management - ENHANCED
    let isUserScrolling = false;
    let userScrollTimeout: number | null = null;
    let scrollRAF: number | null = null;
    let lastScrollTop = 0;
    let lastScrollTime = Date.now();
    let scrollVelocity = 0;
    let scrollSamples: number[] = []; // Keep a history of recent scroll deltas
    let stableAtBottomTimer: number | null = null;
    let manualScrollLock = false; // Lock to prevent auto-scroll from fighting with user
    let manualScrollLockTimer: number | null = null;
    let velocityDecayTimer: number | null = null; // Timer for aggressive velocity decay
    
    // DEBUG: Tracking which parts of the code trigger scrolls
    let scrollTriggerHistory: {id: string, timestamp: number}[] = [];
    const MAX_SCROLL_HISTORY = 5; // Only keep last 5 events
    
    // Track a scroll trigger for debug overlay - FIXED version
    function trackScrollTrigger(triggerId: string) {
        console.log(`SCROLL TRIGGER: ${triggerId}`);
        
        // Create a new array with the new trigger at the beginning
        scrollTriggerHistory = [
            {
                id: triggerId,
                timestamp: Date.now()
            },
            ...scrollTriggerHistory.slice(0, MAX_SCROLL_HISTORY - 1)
        ];
    }
    
    // Tracked scroll to bottom function - use this instead of direct scrollTop = 0
    function trackedScrollToBottom(triggerId: string) {
        if (!scrollContainer) return;
        
        // ONLY scroll if auto-scroll is on and user is not actively scrolling
        if (!autoScroll || isUserScrolling || manualScrollLock) {
            if (debug) console.warn(`BLOCKED scroll from ${triggerId}: autoScroll=${autoScroll}, isUserScrolling=${isUserScrolling}, manualLock=${manualScrollLock}`);
            return;
        }
        
        // Track the source of this scroll event
        trackScrollTrigger(triggerId);
        
        // Perform the scroll using our tracked setter
        setScrollTop(0, triggerId);
    }
    // Removed isStableAtBottom as it's handled within checkStableAtBottom
    
    // Animation state tracking
    let animationInProgress = false;
    let pendingScrollToBottom = false;
    let forceScrollTimer: number | null = null;
    
    // Timer management
    let postProcessingTimers: number[] = [];
    
    // Mass log addition detection
    let recentLogAdditions = 0;
    let lastLogRateCheck = Date.now();
    
    // Viewport anchoring for stable scrolling
    let viewportAnchor: { 
        index: number,
        sequence: number, 
        offsetTop: number
    } | null = null;
    
    // User intent tracking (Variables from new plan)
    let scrollDirectionToBottom = false; // Keep existing
    // Removed significantScrollTowardBottom as it's not used in new plan
    let lastDirectionChangeTime = 0; // Keep existing
    let consistentDirectionDuration = 0; // Keep existing
    let intentToReturnToBottom = false; // Keep existing
    
    // Virtualization
    let virtualStart = 0;
    let virtualEnd = 0;
    const BUFFER_SIZE = 50; // How many logs to render above/below viewport
    let viewportHeight = 0;
    let virtualContainerHeight = 0;
    let visibleLogCount = 0;
    
    // Virtualization settings and state
    let virtualEnabled = false; // Start with virtualization disabled
    let virtualizationReady = false;
    let initialMeasurementsComplete = false;
    let manualVirtualToggle = false; // Track if user manually toggled virtualization
    
    // Individual log height tracking
    let logHeights: Map<number, number> = new Map(); // Maps sequence -> actual height
    let logPositions: Map<number, number> = new Map(); // Maps sequence -> Y position
    let totalLogHeight = 0;
    let avgLogHeight = 25; // Initial estimate, will be refined
    
    // Positioning buffer (to prevent overlap)
    const POSITION_BUFFER = 2; // Add 2px buffer between entries
    
    // Animation and filter state
    let filterTransitionRunning = false;
    let filterChangeTimestamp = Date.now();
    let recentlyAddedLogs: Set<number> = new Set();
    
    // Track measurement batches
    let pendingMeasurements = false;
    let batchMeasurementTimer: number | null = null;
    
    // Debugging and development logging
    let debug = version === 'dev';
    let debugAutoScroll = false; // Developer option for auto-scroll debug overlay
    
    // Auto-scroll debug stats 
    let lastAutoScrollTime = Date.now();
    let autoScrollTriggerCount = 0;
    let logsBatchedSinceLastScroll = 0;
    
    // Track active transitions for non-virtualized mode
    let activeTransitions = 0;

    // Visual feedback state
    let showReturnToBottomButton = false;
    let showAutoScrollToast = false;
    let autoScrollToastMessage = "";
    let autoScrollToastTimer: number | null = null;

    // Filter logs by level
    $: filteredLogs = $logStore.filter(log => 
        logLevelPriority[log.level?.toLowerCase() || 'info'] >= 
        logLevelPriority[selectedLogLevel.toLowerCase()]
    );
    
    // Use the centralized threshold from settings
    $: {
        if (!manualVirtualToggle) { // Only auto-toggle if user hasn't manually set it
            // Get threshold from settings with fallback to 2000
            const threshold = ($settings?.logViewerVirtualizationThreshold !== undefined)
                ? $settings.logViewerVirtualizationThreshold
                : 2000;
                
            const shouldVirtualize = filteredLogs.length > threshold;
            
            if (shouldVirtualize !== virtualEnabled) {
                if (debug) {
                    console.log(`Auto-toggling virtualization: ${shouldVirtualize ? 'ON' : 'OFF'} (log count: ${filteredLogs.length}, threshold: ${threshold})`);
                }
                virtualEnabled = shouldVirtualize;
                
                // Reset virtualization when toggling
                if (virtualEnabled) {
                    // Allow time to render first
                    setTimeout(() => {
                        resetVirtualization();
                    }, 50);
                }
            }
        }
    }
    
    // Track when log level changes for animations
    $: if (selectedLogLevel !== previousLogLevel) {
        filterChangeTimestamp = Date.now();
        filterTransitionRunning = true;
        
        // Save viewport anchor before filter change
        saveViewportAnchor();
        
        previousLogLevel = selectedLogLevel;
        
        // After animation completes, restore scroll position
        setTimeout(() => {
            filterTransitionRunning = false;
            recalculatePositions();
            // Use the reactive autoScroll boolean here
            if (!autoScroll) {
                restoreViewportAnchor();
            } else {
                scrollToBottomWithStrategy();
            }
        }, 300); // Match with animation duration
    }
    
    // Monitor isProcessing changes from App.svelte
    $: {
        if (isProcessing !== prevIsProcessing) {
            if (debug) console.log(`Processing state changed: ${prevIsProcessing} -> ${isProcessing}`);
            
            // When processing ends, schedule final scroll checks
            // Use the reactive autoScroll boolean here
            if (!isProcessing && prevIsProcessing && autoScroll) {
                if (debug) console.log("Processing ended - scheduling final scroll checks");
                schedulePostProcessingScrolls();
            }
            
            prevIsProcessing = isProcessing;
        }
    }
    
    // Check for high log volume to handle scroll appropriately
    $: {
        if (filteredLogs.length > 0) {
            recentLogAdditions++;
            
            // Check log rate periodically
            const now = Date.now();
            if (now - lastLogRateCheck > 1000) { // Check every second
                const currentRate = recentLogAdditions;
                
                // Schedule forced scroll for high volume scenarios
                // Use the reactive autoScroll boolean here
                if (currentRate > 30 && autoScroll) {
                    scheduleForceScroll();
                }
                
                // Reset counters
                recentLogAdditions = 0;
                lastLogRateCheck = now;
            }
        }
    }
    
    // React to new logs being added
    $: if (filteredLogs.length > 0 && scrollContainer) {
        // Check auto-scroll state and scroll position
        const exactlyAtBottom = scrollContainer.scrollTop === 0;
        
        // If auto-scroll is on but we're not at the bottom, this is inconsistent
        if (autoScroll && !exactlyAtBottom && !isUserScrolling) {
            if (debug) console.log(`Detected inconsistency: auto-scroll ON but scrollTop=${scrollContainer.scrollTop}px`);
            // Turn off auto-scroll
            setAutoScroll(false, 'reactiveInconsistency');
        }
        
        // Save viewport position if not at bottom and auto-scroll is off
        if (!exactlyAtBottom && !autoScroll) {
            saveViewportAnchor();
        }
        
        // Mark new logs for animation (only if not virtualizing)
        if (!virtualEnabled) {
            const currentTime = Date.now();
            filteredLogs.forEach(log => {
                if (log._unix_time && currentTime - log._unix_time < 500) {
                    recentlyAddedLogs.add(log._sequence || 0);
                    // Clear after animation completes
                    setTimeout(() => {
                        recentlyAddedLogs.delete(log._sequence || 0);
                    }, 1000);
                }
            });
            
            // Set animation in progress flag
            animationInProgress = true;
            
            // Schedule animation end after transition duration
            setTimeout(() => {
                animationInProgress = false;
                
                // If scroll was requested during animation, execute it now
                if (pendingScrollToBottom && autoScroll) {
                    pendingScrollToBottom = false;
                    // Verify we're still at bottom and auto-scroll is on before scrolling
                    if (scrollContainer && scrollContainer.scrollTop === 0 && autoScroll) {
                        scrollContainer.scrollTop = 0; // Direct DOM manipulation is most reliable
                    }
                }
            }, 350); // Slightly longer than transition duration to ensure completion
        }
        
        // Schedule a batch update to allow DOM to catch up
        if (!pendingMeasurements) {
            pendingMeasurements = true;
            
            // Cancel any existing timer
            if (batchMeasurementTimer) {
                clearTimeout(batchMeasurementTimer);
            }
            
            // Increment logs batched counter for debugging
            logsBatchedSinceLastScroll++;
            
            // Schedule delayed recalculation to allow DOM to update first
            batchMeasurementTimer = window.setTimeout(async () => {
                await tick(); // Ensure DOM is updated
                recalculatePositions();
                
                // Only update virtualization if it's ready
                if (virtualizationReady && virtualEnabled) {
                    updateVirtualization();
                }
                
                // Verify state again after DOM update
                const stillAtBottom = scrollContainer && scrollContainer.scrollTop === 0;
                
                // Restore scroll position using the appropriate strategy
                if (stillAtBottom && autoScroll && !isUserScrolling && !manualScrollLock) {
                    // If at bottom, maintain position at bottom with tracked scrolling
                    setScrollTop(0, 'batchMeasurementAtBottom');
                } else if (!autoScroll && viewportAnchor) {
                    // Otherwise restore anchor position
                    restoreViewportAnchor();
                } else if (autoScroll && !stillAtBottom && !isUserScrolling) {
                    // Fix inconsistency: we're supposed to be at bottom but aren't
                    setAutoScroll(false, 'measurementInconsistency');
                }
                
                pendingMeasurements = false;
                batchMeasurementTimer = null;
                
                // Verify checkbox state matches our internal state
                const checkbox = document.getElementById('auto-scroll-checkbox') as HTMLInputElement;
                if (checkbox && checkbox.checked !== autoScroll) {
                    checkbox.checked = autoScroll; // Force sync
                }
            }, 10); // Small delay to batch updates
        }
    }
    
    // Minimal, direct DOM approach after updates
    afterUpdate(() => {
        // Generate update ID for tracing
        const updateId = Date.now().toString().slice(-6);
        debugLog(`AFTER-UPDATE triggered [id=${updateId}]`, "INFO");
        
        // Get current states
        const svelteState = autoScroll;
        const visibleCheckbox = document.getElementById('auto-scroll-checkbox') as HTMLInputElement;
        const uiState = visibleCheckbox ? visibleCheckbox.checked : 'unknown';
        
        // Log all current states
        debugLog(`States: Svelte=${svelteState}, UI=${uiState} [id=${updateId}]`, "INFO");
        
        // Synchronize UI checkbox with svelte state if needed
        if (visibleCheckbox && visibleCheckbox.checked !== autoScroll) {
            visibleCheckbox.checked = autoScroll;
        }
        
        // Skip during active user scrolling
        if (isUserScrolling) {
            debugLog(`Skipping auto-scroll checks - user is actively scrolling [id=${updateId}]`, "DEBUG");
            return;
        }
        
        // Skip during active measurements
        if (pendingMeasurements) {
            debugLog(`Skipping auto-scroll checks - measurements pending [id=${updateId}]`, "DEBUG");
            return;
        }
        
        // If auto-scroll is off, just verify UI consistency
        if (!autoScroll) {
            // If UI is out of sync, update it
            if (uiState !== 'false' && uiState !== false) {
                debugLog(`UI checkbox out of sync: UI=${uiState}, actual=false - fixing [id=${updateId}]`, "WARN");
                if (visibleCheckbox) visibleCheckbox.checked = false;
            }
            return;
        }
        
        // If auto-scroll is ON, verify we're at bottom
        if (scrollContainer) {
            const currentScrollTop = scrollContainer.scrollTop;
            debugLog(`Checking scroll position: ${currentScrollTop}px with auto-scroll ON [id=${updateId}]`, "INFO");
            
            // Auto-scroll is on but we're not at bottom - inconsistency
            if (currentScrollTop !== 0 && autoScroll) {
                debugLog(`INCONSISTENCY: Auto-scroll ON but not at bottom (${currentScrollTop}px) [id=${updateId}]`, "WARN");
                
                if (currentScrollTop < 5 && !isUserScrolling && !manualScrollLock) {
                    // Very close to bottom - just scroll
                    trackScrollTrigger('afterUpdate:closeToBottom');
                    scrollContainer.scrollTop = 0;
                    debugLog(`Very close to bottom, forcing to 0px [id=${updateId}]`, "INFO");
                } else {
                    // Significantly away - disable auto-scroll
                    debugLog(`Not at bottom (${currentScrollTop}px), DISABLING auto-scroll [id=${updateId}]`, "WARN");
                    
                    // Use our setter function to ensure consistency
                    setAutoScroll(false, 'afterUpdateInconsistency');
                }
            } else if (currentScrollTop === 0 && autoScroll && !isUserScrolling && !manualScrollLock) {
                // Normal case - at bottom with auto-scroll on
                // We're already at bottom (scrollTop = 0), no need to scroll again
                debugLog(`At exact bottom (0px), position already correct [id=${updateId}]`, "DEBUG");
            }
        }
        
        // Update return to bottom button visibility
        updateReturnToBottomButtonVisibility();
        
        // No need to update checkbox state as we now have a single source of truth
    });
    
    // Schedule scroll checks after processing completes
    function schedulePostProcessingScrolls() {
        // Cancel any existing timers
        cancelPostProcessingChecks();
        
        // Use staggered timing to catch all rendering phases
        const checkTimes = [100, 300, 600, 1000, 1500];
        
        checkTimes.forEach((delay, index) => {
            const timerId = window.setTimeout(() => {
                // Only auto-scroll if it's enabled and user isn't manually scrolling
                // This prevents forcing auto-scroll when it was disabled by user action
                if (autoScroll && !isUserScrolling) {
                    if (debug) console.log(`Post-processing scroll check #${index + 1} at t+${delay}ms`);
                    executeScrollToBottom(index === checkTimes.length - 1); // Force on last check
                }
            }, delay);
            
            postProcessingTimers.push(timerId);
        });
    }
    
    // Cancel any pending post-processing checks
    function cancelPostProcessingChecks() {
        postProcessingTimers.forEach(timerId => {
            window.clearTimeout(timerId);
        });
        postProcessingTimers = [];
    }
    
    // Force scroll after a delay - use this for high volume scenarios
    function scheduleForceScroll(delay = 300) {
        // Cancel any existing timer
        if (forceScrollTimer) {
            clearTimeout(forceScrollTimer);
        }
        
        // Set a new timer for force scroll
        forceScrollTimer = window.setTimeout(() => {
            // Use the reactive autoScroll boolean here
            if (autoScroll) {
                // Force scroll regardless of other state
                if (debug) console.log("Executing force scroll after high volume");
                forceScrollToBottom();
            }
            forceScrollTimer = null;
        }, delay);
    }
    
    // Force scroll to bottom - COMPLETELY REWRITTEN WITH CRITICAL FIXES
    function forceScrollToBottom() {
        // CRITICAL: Don't scroll if auto-scroll is off, user is scrolling, or manual lock active
        if (!scrollContainer || !autoScroll || isUserScrolling || manualScrollLock) {
            if (debug) console.warn(`Forced scroll BLOCKED: autoScroll=${autoScroll}, userScrolling=${isUserScrolling}, manualLock=${manualScrollLock}`);
            return;
        }
        
        // Set flag to avoid our scroll handler treating this as user scroll
        isProgrammaticScroll = true;
        
        try {
            // Record this scroll attempt
            trackScrollTrigger('forceScrollToBottom:main');
            
            // In column-reverse, set scrollTop to 0 to get to the bottom (newest logs)
            setScrollTop(0, 'forceScrollToBottom:initial');
            
            // Use multiple techniques with escalating forcefulness
            requestAnimationFrame(() => {
                // CRITICAL CHECK: Only if still eligible for scrolling 
                if (!scrollContainer || !autoScroll || isUserScrolling || manualScrollLock) {
                    isProgrammaticScroll = false;
                    return;
                }
                
                // Record this scroll attempt
                trackScrollTrigger('forceScrollToBottom:rAF');
                
                // Try direct assignment first, but tracked
                setScrollTop(0, 'forceScrollToBottom:rAF');
                
                // Then use scrollTo with instant behavior
                try {
                    scrollContainer.scrollTo({ top: 0, behavior: 'instant' });
                } catch (e) {
                    // No fallback needed - we already did scrollTop = 0
                }
                
                // ONE retry is sufficient - remove the multiple retries
                setTimeout(() => {
                    // Final check before scrolling
                    if (!scrollContainer || !autoScroll || isUserScrolling || manualScrollLock) {
                        isProgrammaticScroll = false;
                        return;
                    }
                    
                    trackScrollTrigger('forceScrollToBottom:finalCheck');
                    
                    // Still maintain programmatic flag
                    isProgrammaticScroll = true;
                    
                    // One final scroll attempt with tracking
                    setScrollTop(0, 'forceScrollToBottom:finalAttempt');
                    
                    // End programmatic scroll state
                    setTimeout(() => {
                        isProgrammaticScroll = false;
                    }, 10);
                }, 50);
            });
        } catch (e) {
            isProgrammaticScroll = false;
            console.error("Error in force scroll:", e);
        }
    }
    
    // Ultra-simplified scroll to bottom function
    function scrollToBottomWithStrategy(): void {
        // CRITICAL FIX: If auto-scroll is off, DO NOT SCROLL under any circumstances
        if (!autoScroll || !scrollContainer) return;
        
        // If user is actively scrolling or manual lock is active, DO NOT SCROLL
        if (isUserScrolling || manualScrollLock) {
            if (debug) console.warn("Blocked auto-scroll due to user scrolling or manual lock");
            return;
        }

        // Track this scroll trigger
        trackScrollTrigger('scrollToBottomWithStrategy:main');
        
        // Direct DOM access but with tracking
        setScrollTop(0, 'scrollToBottomWithStrategy:main');
        
        // Special cases
        if (!virtualEnabled && animationInProgress) {
            // For animations, queue for later
            pendingScrollToBottom = true;
            return;
        }
        
        // For virtualized mode, ensure range includes newest logs
        if (virtualEnabled && virtualizationReady) {
            const lastLogIndex = filteredLogs.length - 1;
            if (virtualEnd < lastLogIndex) {
                // Update range to include newest logs
                virtualEnd = lastLogIndex;
                virtualStart = Math.max(0, virtualEnd - visibleLogCount - BUFFER_SIZE);
                
                // Force scroll after range update
                tick().then(() => {
                    // Check again if auto-scroll is still on
                    if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                        trackScrollTrigger('scrollToBottomWithStrategy:tick');
                        scrollContainer.scrollTop = 0;
                    }
                });
            }
        }
    }
    
    // Helper function to get log level styling
    function getLevelClass(level: string): string {
        switch (level?.toUpperCase()) {
            case 'DEBUG':
                return 'text-log-debug log-level-debug';
            case 'INFO':
                return 'text-log-info log-level-info';
            case 'WARN':
                return 'text-log-warn log-level-warn';
            case 'ERROR':
                return 'text-log-error log-level-error';
            default:
                return 'text-log-info log-level-info';
        }
    }
    
    // Helper function to determine if a log is new (for animation)
    function isNewLog(log: LogMessage): boolean {
        return recentlyAddedLogs.has(log._sequence || 0);
    }
    
    // Helper function to get flash animation class based on log level
    function getFlashClass(level: string): string {
        switch (level?.toUpperCase()) {
            case 'DEBUG': return 'flash-debug';
            case 'INFO': return 'flash-info';
            case 'WARN': return 'flash-warn';
            case 'ERROR': return 'flash-error';
            default: return 'flash-info';
        }
    }

    // Helper function: format additional fields
    function formatFields(fields: Record<string, any> | undefined): string {
        if (!fields) return '';
        
        const excluded = ['level', 'message', 'time', 'behavior', '_sequence', '_unix_time', '_original_time', '_visible', '_height'];
        return Object.entries(fields)
            .filter(([key]) => !excluded.includes(key))
            .map(([key, value]) => {
                if (typeof value === 'object') {
                    return key + "=" + JSON.stringify(value);
                }
                return key + "=" + value;
            })
            .join(' ');
    }
    
    // Check if we're exactly at the bottom (scrollTop = 0) of the scroll container
    // For terminal-style layout with column-reverse, this is a strict check
    function isScrolledToBottom(tolerance = 0): boolean {
        if (!scrollContainer) return true;
        
        // Check if scrolling is even possible
        const canScrollFurther = scrollContainer.scrollHeight > scrollContainer.clientHeight;
        if (!canScrollFurther) return true; // Can't scroll, so we're at the "bottom"
        
        // In column-reverse, we're only at bottom when scrollTop is EXACTLY 0
        // Only use tolerance for special cases where explicitly requested
        return scrollContainer.scrollTop <= tolerance;
    }
    
    // Track transition start/end events for non-virtualized mode
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
            
            // Execute any pending scrolls
            // Use the reactive autoScroll boolean here
            if (pendingScrollToBottom && autoScroll) {
                pendingScrollToBottom = false;
                executeScrollToBottom();
            }
        }
    }
    
    // Enhanced auto-scroll state management with single source of truth
    function setAutoScroll(newValue: boolean, source: string = 'direct') {
        // Skip if no change
        if (newValue === autoScroll) return;
        
        // Debug logging
        if (debug) console.log(`Auto-scroll ${newValue ? 'enabled' : 'disabled'} via ${source}`);
        
        // Track this state change
        trackScrollTrigger(`setAutoScroll:${newValue ? 'ON' : 'OFF'}:${source}`);
        
        // IMPORTANT: If trying to enable auto-scroll but manual lock is active, refuse
        if (newValue && manualScrollLock && source !== 'userPreference') {
            if (debug) console.warn(`Auto-scroll enable BLOCKED due to active manual scroll lock`);
            return; // Early return - don't enable against user's wishes
        }
        
        // Update our state variable
        autoScroll = newValue;
        
        // Sync UI checkbox - CRITICAL for consistency
        const checkbox = document.getElementById('auto-scroll-checkbox') as HTMLInputElement;
        if (checkbox) {
            checkbox.checked = newValue;
            
            if (debug) console.log(`UI checkbox synced to ${newValue ? 'checked' : 'unchecked'}`);
        } else {
            if (debug) console.warn(`Could not find UI checkbox to update!`);
        }
        
        // Additional state updates when needed
        if (newValue) {
            // When enabling auto-scroll:
            viewportAnchor = null; // Clear any saved position
            
            // SCROLL ONLY IF the user toggled it on or it came from certain sources
            // that are expected to scroll
            if (source === 'userPreference' || source === 'scrolledToBottom' || source === 'stableAtBottom') {
                trackScrollTrigger(`setAutoScroll:scroll:${source}`);
                
                // Force scroll to bottom with direct DOM manipulation
                if (scrollContainer && !isUserScrolling && !manualScrollLock) {
                    // Direct scrolling once - column-reverse layout needs scrollTop = 0 for bottom
                    scrollContainer.scrollTop = 0;
                    
                    // One retry is sufficient
                    setTimeout(() => {
                        if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                            trackScrollTrigger(`setAutoScroll:retry:${source}`);
                            scrollContainer.scrollTop = 0;
                        }
                    }, 50);
                }
            }
        } else {
            // When disabling auto-scroll, save position for restoration
            if (source !== 'userScrollAway') { // Don't save if already handled
                saveViewportAnchor();
            }
        }
        
        // Show visual confirmation - customized message based on source
        let message;
        if (newValue) {
            message = "Auto-scroll enabled";
        } else {
            if (source === 'userScrollAway' || source === 'inconsistencyFix') {
                message = "Auto-scroll disabled - scroll to bottom to re-enable";
            } else {
                message = "Auto-scroll disabled";
            }
        }
        
        showAutoScrollToastMessage(message);
    }
    
    
    // Show a temporary toast message about auto-scroll state
    function showAutoScrollToastMessage(message: string) {
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
    
    // Update visibility of the "Return to Bottom" button
    function updateReturnToBottomButtonVisibility() {
        // Show button when:
        // 1. Auto-scroll is disabled
        // 2. User is not at the bottom
        // 3. There are logs to view
        const notAtBottom = scrollContainer ? scrollContainer.scrollTop > 0 : false;
        
        showReturnToBottomButton = 
            !autoScroll && 
            notAtBottom &&
            filteredLogs.length > 0;
    }
    
    // Measure individual log height using full bounding rect
    function measureLogEntry(node: HTMLElement, log: LogMessage) {
        const sequence = log._sequence || 0;
        
        // Create ResizeObserver to measure the actual height
        const resizeObserver = new ResizeObserver(entries => {
            // Use getBoundingClientRect for complete height including padding/borders
            const rect = node.getBoundingClientRect();
            const height = Math.max(Math.ceil(rect.height), 20) + POSITION_BUFFER;
            
            // Only update if height changed significantly (>1px)
            if (Math.abs((logHeights.get(sequence) || 0) - height) > 1) {
                logHeights.set(sequence, height);
                
                // Mark measurements as having started
                if (!initialMeasurementsComplete && logHeights.size >= Math.min(10, filteredLogs.length)) {
                    initialMeasurementsComplete = true;
                    
                    // Update virtualization after a short delay to ensure UI is ready
                    setTimeout(async () => {
                        await tick(); // Ensure DOM update
                        virtualizationReady = true;
                        recalculatePositions();
                        
                        if (virtualEnabled) {
                            updateVirtualization();
                        }
                        
                        // Track if we were at bottom before height change
                        const wasAtBottom = isScrolledToBottom();
                        
                        // Maintain scroll position
                        // Use the reactive autoScroll boolean here
                        if (wasAtBottom && autoScroll) {
                            scrollToBottomWithStrategy();
                        } else if (!autoScroll && viewportAnchor) {
                            restoreViewportAnchor();
                        }
                    }, 100);
                } else if (virtualizationReady) {
                    // Queue a position recalculation for the next animation frame
                    // to batch multiple height changes together
                    if (!pendingMeasurements) {
                        pendingMeasurements = true;
                        
                        requestAnimationFrame(async () => {
                            recalculatePositions();
                            
                            if (virtualEnabled) {
                                updateVirtualization();
                            }
                            
                            // Maintain scroll position
                            // Use the reactive autoScroll boolean here
                            const wasAtBottom = isScrolledToBottom();
                            if (wasAtBottom && autoScroll) {
                                scrollToBottomWithStrategy();
                            } else if (!autoScroll && viewportAnchor) {
                                restoreViewportAnchor();
                            }
                            
                            pendingMeasurements = false;
                        });
                    }
                }
            }
        });
        
        resizeObserver.observe(node);
        
        // Set initial height estimate if not already set
        if (!logHeights.has(sequence)) {
            const initialHeight = avgLogHeight > 0 ? avgLogHeight : 25;
            logHeights.set(sequence, initialHeight + POSITION_BUFFER);
        }
        
        return {
            destroy() {
                resizeObserver.disconnect();
            }
        };
    }
    
    // Calculate positions for all logs based on individual heights (with WASM optimization)
    function recalculatePositions(): void {
        // Get current settings
        const $settings = get(settings);
        
        // Check for WebAssembly availability and blacklist status
        if (isWasmEnabled() && !isOperationBlacklisted('recalculatePositions')) {
            // IMPORTANT: Use different threshold logic for position calculation
            // This function is called less frequently but processes the entire dataset
            if ($settings.forceWasmMode === 'enabled' || 
                (filteredLogs.length > 500 && 
                 shouldUseWasm(filteredLogs.length, 'recalculatePositions'))) {
                
                try {
                    // Track start time for performance comparison
                    let tsTime = 0;
                    
                    // Occasionally benchmark TypeScript for comparison (5% of operations)
                    if (Math.random() < 0.05) {
                        const tsStartTime = performance.now();
                        
                        // Execute TS version for comparison but don't use its results
                        // This is just for measurement
                        let currentPosition = 0;
                        let totalHeightTs = 0;
                        
                        for (const log of filteredLogs) {
                            const sequence = log._sequence || 0;
                            // Don't actually set positions - just simulate the work
                            const height = logHeights.get(sequence) || avgLogHeight + POSITION_BUFFER;
                            currentPosition += height;
                            totalHeightTs += height;
                        }
                        
                        const tsEndTime = performance.now();
                        tsTime = tsEndTime - tsStartTime;
                    }
                    
                    // Use WebAssembly implementation, passing TS comparison time if available
                    const result = recalculatePositionsWasm(
                        filteredLogs,
                        logHeights,
                        avgLogHeight,
                        POSITION_BUFFER,
                        tsTime // Pass measured TS time for accurate comparison
                    );

                    // Update state with results
                    logPositions = result.positions;
                    totalLogHeight = result.totalHeight;
                    virtualContainerHeight = totalLogHeight;

                    // Recalculate average height based on measurements
                    if (logHeights.size > 0) {
                        let total = 0;
                        logHeights.forEach(height => total += height);
                        avgLogHeight = (total / logHeights.size) - POSITION_BUFFER;
                    }

                    return; // Exit early on success
                } catch (error: any) {
                    // Handle error and fall back to TypeScript
                    handleWasmError(error, 'recalculatePositions', {
                        logCount: filteredLogs.length
                    });
                    
                    // Fall back to TypeScript implementation
                }
            }
        }

        // Original TypeScript implementation (unchanged - fallback)
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

        // Recalculate average height
        if (logHeights.size > 0) {
            let total = 0;
            logHeights.forEach(height => total += height);
            avgLogHeight = (total / logHeights.size) - POSITION_BUFFER;
        }
    }
    
    // Find which log corresponds to a scroll position using binary search (with WASM optimization)
    function findLogAtScrollPosition(scrollTop: number, scrollMetrics?: {
        frequency?: number,
        visibleLogs?: number
    }): number {
        // Early short-circuit for empty logs
        if (filteredLogs.length === 0) return 0;
        
        // In column-reverse, we need to adjust the scrollTop value
        // Convert from scrollTop to a position from the top of content
        const adjustedScrollPosition = scrollContainer ? 
            (totalLogHeight - scrollContainer.clientHeight - scrollTop) : 
            scrollTop;
        
        // Get current settings
        const $settings = get(settings);
        
        // Check for WebAssembly availability and blacklist status
        if (isWasmEnabled() && !isOperationBlacklisted('findLogAtScrollPosition')) {
            if ($settings.forceWasmMode === 'enabled' || 
                (filteredLogs.length > 100 && 
                 shouldUseWasm(Math.min(filteredLogs.length, 1000), 'findLogAtScrollPosition'))) {
                
                try {
                    // Track scroll metrics for performance monitoring
                    const metrics = {
                        frequency: scrollMetrics?.frequency,
                        visibleLogs: scrollMetrics?.visibleLogs || 
                                    Math.ceil(viewportHeight / (avgLogHeight + POSITION_BUFFER))
                    };
                    
                    // Use adjusted position for WebAssembly function
                    return findLogAtScrollPositionWasm(
                        filteredLogs,
                        logPositions,
                        logHeights,
                        adjustedScrollPosition, // Use adjusted position
                        avgLogHeight,
                        POSITION_BUFFER,
                        metrics
                    );
                } catch (error: any) {
                    // Handle error and fall back to TypeScript
                    handleWasmError(error, 'findLogAtScrollPosition', {
                        logCount: filteredLogs.length,
                        scrollTop: adjustedScrollPosition
                    });
                    
                    // Fall back to TypeScript implementation
                }
            }
        }
        
        // Original TypeScript implementation with adjusted scroll position
        let low = 0;
        let high = filteredLogs.length - 1;
        
        while (low <= high) {
            const mid = Math.floor((low + high) / 2);
            const sequence = filteredLogs[mid]._sequence || 0;
            const pos = logPositions.get(sequence) || mid * (avgLogHeight + POSITION_BUFFER);
            const height = logHeights.get(sequence) || avgLogHeight + POSITION_BUFFER;
            
            if (adjustedScrollPosition >= pos && adjustedScrollPosition < (pos + height)) {
                return mid; // Found exact log
            }
            
            if (adjustedScrollPosition < pos) {
                high = mid - 1;
            } else {
                low = mid + 1;
            }
        }
        
        // Return the closest log index
        return Math.max(0, Math.min(filteredLogs.length - 1, low));
    }
    
    // Track scroll performance metrics
    let scrollCallCounter = 0;
    let scrollStartTime = Date.now();
    let scrollCallFrequency = 0;

    // Update virtualization calculations for column-reverse layout
    function updateVirtualization(): void {
        if (!scrollContainer || !virtualEnabled || !virtualizationReady) return;
        
        const { scrollTop, clientHeight } = scrollContainer;
        viewportHeight = clientHeight;
        
        // Calculate scroll metrics for WASM optimization
        const scrollMetrics = {
            frequency: scrollCallFrequency,
            visibleLogs: Math.ceil(clientHeight / (avgLogHeight + POSITION_BUFFER))
        };
        
        // When auto-scroll is enabled, ensure we prioritize latest logs
        // Use the reactive autoScroll boolean here
        if (autoScroll && isScrolledToBottom()) {
            // Start from the end of the list and work backwards
            const lastLogIndex = filteredLogs.length - 1;
            virtualEnd = lastLogIndex;
            
            // Determine how many logs fit in the viewport plus buffer
            const visibleCount = Math.ceil(clientHeight / (avgLogHeight + POSITION_BUFFER));
            virtualStart = Math.max(0, lastLogIndex - visibleCount - BUFFER_SIZE);
            
            // Update visible log count
            visibleLogCount = virtualEnd - virtualStart + 1;
        } else {
            // For scrolled state, determine visible range
            // Convert scrollTop for column-reverse layout
            const adjustedScrollTop = totalLogHeight - clientHeight - scrollTop;
            
            // Find log at top and bottom of viewport with adjusted positions
            const topLogIndex = findLogAtScrollPosition(adjustedScrollTop, scrollMetrics);
            const bottomLogIndex = findLogAtScrollPosition(adjustedScrollTop + clientHeight, scrollMetrics);
            
            // Set virtual range with buffer
            virtualStart = Math.max(0, topLogIndex - BUFFER_SIZE);
            virtualEnd = Math.min(filteredLogs.length - 1, bottomLogIndex + BUFFER_SIZE);
            
            // Update visible log count
            visibleLogCount = virtualEnd - virtualStart + 1;
        }
    }
    
    // Save current viewport position as an anchor - updated for column-reverse
    function saveViewportAnchor(): void {
        if (!scrollContainer) return;
        
        // If already at bottom and auto-scroll is enabled, don't create an anchor
        // Use the reactive autoScroll boolean here
        if (isScrolledToBottom() && autoScroll) {
            viewportAnchor = null;
            return;
        }
        
        const { scrollTop, clientHeight } = scrollContainer;
        
        // In column-reverse, convert scrollTop to position from top
        const scrollFromTop = totalLogHeight - clientHeight - scrollTop;
        
        // Calculate scroll metrics for optimization
        const scrollMetrics = {
            frequency: scrollCallFrequency,
            visibleLogs: Math.ceil(viewportHeight / (avgLogHeight + POSITION_BUFFER))
        };
        
        // Find which log is at the top of the viewport with adjusted scroll position
        const logIndex = findLogAtScrollPosition(scrollFromTop, scrollMetrics);
        if (logIndex < 0 || logIndex >= filteredLogs.length) return;
        
        const log = filteredLogs[logIndex];
        const sequence = log._sequence || 0;
        const logTop = logPositions.get(sequence) || 0;
        
        // Save anchor with offset from log top
        viewportAnchor = {
            index: logIndex,
            sequence: sequence,
            offsetTop: scrollFromTop - logTop
        };
    }
    
    // Restore scroll position based on saved anchor - updated for column-reverse
    function restoreViewportAnchor(): void {
        if (!viewportAnchor || !scrollContainer) return;
        
        // Find the log position now
        const sequence = viewportAnchor.sequence;
        const logTop = logPositions.get(sequence) || 0;
        
        // Calculate adjusted position with the same offset
        const positionFromTop = logTop + viewportAnchor.offsetTop;
        
        // In column-reverse, convert back to scrollTop
        const scrollTopValue = totalLogHeight - scrollContainer.clientHeight - positionFromTop;
        
        // Restore scroll position with the calculated scrollTop
        withProgrammaticScroll(() => {
            scrollContainer.scrollTop = Math.max(0, scrollTopValue);
        });
    }
    
    // Reset virtualization settings
    function resetVirtualization(): void {
        virtualStart = 0;
        virtualEnd = 0;
        viewportAnchor = null;
        
        setTimeout(async () => {
            await tick(); // Ensure DOM update
            recalculatePositions();
            
            if (virtualEnabled && virtualizationReady) {
                updateVirtualization();
            }
            
            // If auto scroll enabled, scroll to bottom
            // Use the reactive autoScroll boolean here
            if (autoScroll) {
                scrollToBottomWithStrategy();
            }
        }, 50);
    }
    
    // Wrapper to mark scroll operations as programmatic
    let isProgrammaticScroll = false; // Keep this flag
    function withProgrammaticScroll(callback: () => void): void {
        isProgrammaticScroll = true;
        try {
            callback();
        } finally {
            // Use a timeout to account for any scroll events that might be triggered
            // Use requestAnimationFrame for potentially better timing relative to rendering
            requestAnimationFrame(() => {
                isProgrammaticScroll = false;
            });
        }
    }
    
    // Execute scroll to bottom with programmatic flag - updated for column-reverse
    function executeScrollToBottom(force: boolean = false): void {
        // CRITICAL: Never scroll if auto-scroll is off or user is actively scrolling
        if (!scrollContainer || !autoScroll || isUserScrolling || manualScrollLock) {
            if (debug) console.warn(`Execute scroll BLOCKED: autoScroll=${autoScroll}, userScrolling=${isUserScrolling}, manualLock=${manualScrollLock}`);
            return;
        }
        
        // Skip during animations unless forced
        if (animationInProgress && !force) {
            pendingScrollToBottom = true;
            return;
        }
        
        // Track this scroll request
        trackScrollTrigger(`executeScrollToBottom:${force ? 'forced' : 'normal'}`);
        
        // Use rAF for better performance
        requestAnimationFrame(() => {
            // Check again in case conditions changed
            if (!autoScroll || isUserScrolling || manualScrollLock) return;
            
            withProgrammaticScroll(() => {
                if (scrollContainer) {
                    trackScrollTrigger('executeScrollToBottom:rAF');
                    
                    // In column-reverse, bottom is at scrollTop = 0
                    scrollContainer.scrollTop = 0;
                    
                    // For critical scrolls, add a second scroll after a tiny delay
                    if (force) {
                        setTimeout(() => {
                            // Final check before scrolling
                            if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                                trackScrollTrigger('executeScrollToBottom:forceRetry');
                                scrollContainer.scrollTop = 0;
                            }
                        }, 50);
                    }
                }
            });
        });
    }
    
    // Removed old toggleAutoScroll function, replaced by setAutoScroll
    
    // Toggle virtualization manually (dev mode)
    function toggleVirtualization(): void {
        manualVirtualToggle = true;
        virtualEnabled = !virtualEnabled;
        resetVirtualization();
    }
    
    // Clear logs while preserving auto-scroll state
    function clearLogsPreserveAutoScroll(): void {
        // Save current auto-scroll state (using the boolean)
        const wasAutoScrollEnabled = autoScroll;
        
        // Clear logs
        logStore.clearLogs();
        
        // Reset key tracking data
        viewportAnchor = null;
        
        // Restore auto-scroll state using the setter
        setAutoScroll(wasAutoScrollEnabled, 'clearLogs');
        
        // If it was enabled, ensure we scroll to bottom after clearing
        if (wasAutoScrollEnabled) {
            setTimeout(() => {
                executeScrollToBottom(true);
            }, 50); // Delay slightly to allow logStore clear to process
        }
    }
    
    // FIXED: Velocity tracking with aggressive decay
    function updateScrollMetrics(currentScrollTop: number): void {
        const now = Date.now();
        const timeDelta = Math.max(1, now - lastScrollTime); // Ensure non-zero denominator
        
        // CRITICAL FIX: If it's been more than 100ms since the last update,
        // clear all samples to prevent old momentum from persisting
        if (timeDelta > 100) {
            scrollSamples = [];
        }
        
        // Calculate the scroll delta (change in scroll position)
        const scrollDelta = currentScrollTop - lastScrollTop;
        
        // IMPORTANT: Only add non-zero deltas to the samples
        if (Math.abs(scrollDelta) > 0.01) {
            // Add to scroll samples for smoothing (keep last 3 samples - REDUCED)
            scrollSamples.push(scrollDelta);
            if (scrollSamples.length > 3) scrollSamples.shift();
        } else {
            // If there's no movement, add a zero to quickly decay the average
            scrollSamples.push(0);
            if (scrollSamples.length > 3) scrollSamples.shift();
        }
        
        // Calculate average delta for smoother velocity
        const avgDelta = scrollSamples.length > 0 ? 
            scrollSamples.reduce((sum, delta) => sum + delta, 0) / scrollSamples.length : 0;
        
        // ULTRA-FAST DECAY: If average delta is very small, reset to exactly zero
        // This prevents tiny values from keeping velocity non-zero
        const cleanedAvgDelta = Math.abs(avgDelta) < 0.1 ? 0 : avgDelta;
        
        // DRAMATICALLY amplify velocity (2000x) so it registers meaningful values
        // This helps detect even slight user movements
        scrollVelocity = (cleanedAvgDelta / timeDelta) * 2000;
        
        // DECAY VELOCITY: If velocity is very small, force it to exactly zero
        if (Math.abs(scrollVelocity) < 0.5) {
            scrollVelocity = 0;
        }
        
        // Determine scroll direction (toward bottom or not)
        // In column-reverse with positive values:
        // - Negative delta = scrolling toward bottom
        // - Positive delta = scrolling away from bottom
        const newDirectionToBottom = cleanedAvgDelta < 0;
        
        // Track direction changes
        if (newDirectionToBottom !== scrollDirectionToBottom) {
            lastDirectionChangeTime = now;
            consistentDirectionDuration = 0;
        } else {
            consistentDirectionDuration = now - lastDirectionChangeTime;
        }
        
        // Update direction state
        scrollDirectionToBottom = newDirectionToBottom;
        
        // IMPORTANT: If velocity is exactly zero, we're not moving in ANY direction
        if (scrollVelocity === 0) {
            intentToReturnToBottom = false;
        } else {
            // Detect intent to return to bottom - more sensitive parameters
            intentToReturnToBottom =
                scrollDirectionToBottom &&
                consistentDirectionDuration > 50 && // Only need 50ms of consistent direction
                Math.abs(scrollVelocity) > 10; // Lower threshold for detection
        }
        
        // Always log scroll metrics in debug mode, even for small movements
        if (debug && Math.abs(scrollDelta) > 0.01) {
            console.log(`🔍 SCROLL METRICS:
                - Delta: ${scrollDelta.toFixed(2)}
                - Avg Delta: ${cleanedAvgDelta.toFixed(2)}
                - Raw Velocity: ${(scrollDelta/timeDelta).toFixed(3)}
                - Amplified Velocity: ${scrollVelocity.toFixed(2)}
                - Direction: ${scrollDirectionToBottom ? '⬇️ TO BOTTOM' : '⬆️ TO TOP'}
                - Consistent Duration: ${consistentDirectionDuration}ms
                - Intent to Bottom: ${intentToReturnToBottom}`);
        }
        
        // Store current values for next calculation
        lastScrollTop = currentScrollTop;
        lastScrollTime = now;
    }
    
    // Add function to check if position is stable at bottom - ENHANCED
    function checkStableAtBottom(): void {
        if (stableAtBottomTimer) {
            clearTimeout(stableAtBottomTimer);
        }
        
        stableAtBottomTimer = window.setTimeout(() => {
            // Only re-enable auto-scroll if ALL of these conditions are met:
            // 1. We're EXACTLY at the bottom (scrollTop = 0) - super strict here
            // 2. User was deliberately scrolling toward the bottom
            // 3. Auto-scroll is currently off
            // 4. User is not actively scrolling
            // 5. Manual scroll lock is not active
            if (scrollContainer && 
                Math.abs(scrollContainer.scrollTop) === 0 && 
                scrollDirectionToBottom && 
                !autoScroll &&
                !isUserScrolling &&
                !manualScrollLock) {
                if (debug) console.log('Position stable at exact bottom with no manual locks - enabling auto-scroll');
                setAutoScroll(true, 'stableAtBottom');
            } else if (scrollContainer && Math.abs(scrollContainer.scrollTop) === 0 && !autoScroll) {
                // Log why we're not re-enabling for debugging
                if (debug) {
                    console.log(`At bottom but not re-enabling auto-scroll because:
                      - Direction to bottom: ${scrollDirectionToBottom}
                      - User scrolling: ${isUserScrolling}
                      - Manual lock: ${manualScrollLock}`);
                }
            }
            stableAtBottomTimer = null;
        }, 500); // 500ms timeout to ensure truly stable position
    }
    
    // Low-level scroll event handler with direct DOM manipulation
    // Completely rewritten scroll handler - ultra simplified for maximum reliability
    function handleScroll(): void {
        // Always ignore programmatic scrolling
        if (isProgrammaticScroll) return;
        
        // Mark as user scrolling - SET THIS FLAG IMMEDIATELY
        isUserScrolling = true;
        
        // IMPORTANT: Set manual scroll lock to prevent auto-scroll from fighting with user
        manualScrollLock = true;
        
        // Cancel any existing timers/animations
        if (scrollRAF) cancelAnimationFrame(scrollRAF);
        if (stableAtBottomTimer) {
            clearTimeout(stableAtBottomTimer);
            stableAtBottomTimer = null;
        }
        if (manualScrollLockTimer) {
            clearTimeout(manualScrollLockTimer);
        }
        
        // Reset the manual scroll lock after a LONG period (3 seconds)
        // This gives user plenty of time to read without auto-scroll interfering
        manualScrollLockTimer = window.setTimeout(() => {
            if (debug) console.log("Manual scroll lock timeout expired");
            manualScrollLock = false;
            manualScrollLockTimer = null;
        }, 3000);
        
        // Use RAFrame for precise timing
        scrollRAF = requestAnimationFrame(() => {
            if (!scrollContainer) {
                scrollRAF = null;
                return;
            }
            
            // CRITICAL FIX: Get absolute value of scrollTop
            // In column-reverse layout, scrollTop is negative when scrolled up
            // We use absolute value for easier handling
            const scrollTop = Math.abs(scrollContainer.scrollTop);
            
            // IMPORTANT: If user has scrolled away from bottom and auto-scroll is on,
            // immediately disable auto-scroll before doing anything else
            if (scrollTop > 1 && autoScroll) {
                if (debug) console.warn(`CRITICAL: Disabling auto-scroll due to scrollTop=${scrollTop}px`);
                // Force auto-scroll off
                setAutoScroll(false, 'userScrollAway');
                // Save position for restoration
                saveViewportAnchor();
                
                // Show toast notification
                showAutoScrollToastMessage("Auto-scroll disabled - scroll to bottom to re-enable");
            }
            
            // CRITICAL: Check if we're debugging mode
            if (debug) {
                // Get scroll container details for analysis
                const { scrollTop: rawScrollTop, scrollHeight, clientHeight } = scrollContainer;
                console.log("📊 SCROLL ANALYSIS 📊");
                console.log(`- raw scrollTop: ${rawScrollTop}px (negative in column-reverse)`);
                console.log(`- abs scrollTop: ${scrollTop}px (converted to positive)`);
                console.log(`- scrollHeight: ${scrollHeight}px`);
                console.log(`- clientHeight: ${clientHeight}px`);
                console.log(`- max scrollTop: ${scrollHeight - clientHeight}px`);
                console.log(`- percentage scrolled: ${(scrollTop / (scrollHeight - clientHeight) * 100).toFixed(1)}%`);
                console.log(`- current auto-scroll: ${autoScroll}`);
                console.log(`- user scrolling: ${isUserScrolling}`);
                console.log(`- manual scroll lock: ${manualScrollLock}`);
            }
            
            // Update scroll metrics with corrected scrollTop value
            updateScrollMetrics(scrollTop); // We pass the positive value
            
            // Virtualization update if needed
            if (virtualizationReady && virtualEnabled) {
                updateVirtualization();
            }
            
            // Set a timeout to mark user scrolling complete - MUCH LONGER TIMEOUT
            if (userScrollTimeout) clearTimeout(userScrollTimeout);
            userScrollTimeout = window.setTimeout(() => {
                // Only clear the flag if we're not in a locked state
                // This prevents clearing too early when user is actively reading
                if (!manualScrollLock) {
                    isUserScrolling = false;
                    
                    if (debug) console.log("User scrolling flag cleared - manual scroll lock: " + manualScrollLock);
                } else {
                    if (debug) console.log("Keeping user scrolling flag due to active manual scroll lock");
                }
                
                // Get final scroll position - absolute value for easier comparison
                const finalScrollTop = Math.abs(scrollContainer?.scrollTop || 0);
                
                if (debug) {
                    console.log(`User scrolling timeout finished:
                        - Position: ${finalScrollTop}px
                        - Auto-scroll: ${autoScroll}
                        - Direction: ${scrollDirectionToBottom ? 'TO BOTTOM' : 'TO TOP'}
                        - User scrolling: ${isUserScrolling}
                        - Manual lock: ${manualScrollLock}`);
                }
                
                // Check if we're exactly at the bottom (scrollTop = 0 in column-reverse)
                if (scrollContainer && finalScrollTop === 0) {
                    if (debug) console.log("User at EXACT bottom position after scrolling");
                    
                    // ONLY re-enable auto-scroll if:
                    // 1. Auto-scroll is currently off
                    // 2. User is scrolling toward the bottom
                    // 3. Manual scroll lock is not active (user isn't actively reading)
                    if (!autoScroll && scrollDirectionToBottom && !manualScrollLock) {
                        if (debug) console.warn("Re-enabling auto-scroll - user at bottom and not locked");
                        
                        // Use setAutoScroll to ensure UI sync
                        setAutoScroll(true, 'scrolledToBottom');
                    } else if (!autoScroll && manualScrollLock) {
                        if (debug) console.log("Not re-enabling auto-scroll despite being at bottom - manual lock active");
                    }
                } else {
                    if (debug) console.log(`Not at bottom after scroll: ${finalScrollTop}px`);
                    
                    // SAFETY CHECK: if not at bottom but auto-scroll is on, fix it
                    if (autoScroll) {
                        if (debug) console.error("INCONSISTENCY: Not at bottom but auto-scroll is on - fixing");
                        
                        // Use setAutoScroll to ensure UI sync
                        setAutoScroll(false, 'inconsistencyFix');
                    }
                }
            }, 800); // MUCH longer timeout (800ms instead of 200ms)
            
            scrollRAF = null;
        });
    }
    
    // For testing - toggle debug overlay
    function toggleDebugOverlay() {
        debugAutoScroll = !debugAutoScroll;
        if (debug) console.log(`Debug overlay ${debugAutoScroll ? 'enabled' : 'disabled'}`);
    }

    onMount(async () => {
        // CRITICAL FIX: Set up a velocity decay timer that aggressively decays velocity
        // This ensures velocity goes to zero quickly when scrolling stops
        velocityDecayTimer = window.setInterval(() => {
            // If no scrolling happens, velocity should decay toward zero
            if (!isUserScrolling && Math.abs(scrollVelocity) > 0) {
                // Extremely aggressive decay - 90% reduction every 50ms
                scrollVelocity *= 0.1;
                
                // If very small, snap to exactly zero
                if (Math.abs(scrollVelocity) < 0.5) {
                    scrollVelocity = 0;
                    scrollSamples = []; // Clear samples history too
                }
                
                if (debug && Math.abs(scrollVelocity) > 0) {
                    console.log(`Velocity decay: ${scrollVelocity.toFixed(2)}`);
                }
            }
        }, 50); // Decay every 50ms
        // Set up key press listener for toggling debug (press 'd' key)
        document.addEventListener('keydown', (e) => {
            if (e.key === 'd' && e.ctrlKey) {
                toggleDebugOverlay();
            }
        });
        // Start with auto-scroll enabled
        
        // Initial update
        await tick();
        recalculatePositions();
        
        // Start with virtualization disabled until measurements are ready
        virtualizationReady = false;
        
        // Initial scroll to bottom if needed - in column-reverse, this means scrollTop = 0
        if (autoScroll && scrollContainer) {
            // Mark this initial scroll
            trackScrollTrigger('onMount:initialScroll');
            scrollContainer.scrollTop = 0;
        }
        
        // Set a timeout to enable virtualization after initial rendering
        setTimeout(async () => {
            // By this point, some logs should have been measured
            await tick();
            
            initialMeasurementsComplete = true;
            virtualizationReady = true;
            
            recalculatePositions();
            
            if (virtualEnabled) {
                updateVirtualization();
            }
            
            // Double-check auto-scroll state - but be careful to check for user interaction
            if (autoScroll && scrollContainer && !isUserScrolling && !manualScrollLock) {
                // Only do this initial setup scroll if we're supposed to auto-scroll
                trackScrollTrigger('onMount:setupComplete');
                scrollContainer.scrollTop = 0;
            }
        }, 200);
        
        // Set up ResizeObserver to detect size changes
        const resizeObserver = new ResizeObserver(() => {
            // Save scroll position
            const wasAtBottom = isScrolledToBottom(0); // Use strict check (exactly at bottom)
            
            if (!wasAtBottom && !autoScroll) {
                saveViewportAnchor();
            }
            
            // Update layout
            recalculatePositions();
            
            if (virtualEnabled && virtualizationReady) {
                updateVirtualization();
            }
            
            // Restore scroll position - but only if user isn't actively scrolling
            if (wasAtBottom && autoScroll && !isUserScrolling && !manualScrollLock) {
                // Record this resize-triggered scroll
                trackScrollTrigger('resizeObserver:wasAtBottom');
                
                // Scroll directly instead of using forceScrollToBottom (simpler)
                if (scrollContainer) {
                    scrollContainer.scrollTop = 0;
                }
            } else if (viewportAnchor) {
                restoreViewportAnchor();
            }
            
            // Update return to bottom button visibility
            updateReturnToBottomButtonVisibility();
        });
        
        if (scrollContainer) {
            resizeObserver.observe(scrollContainer);
            resizeObserver.observe(document.documentElement);
        }
        
        // Log store subscription with detailed logging
        const unsubscribeLogStore = logStore.subscribe((logs) => {
            // Generate unique ID for log events
            const logEventId = Date.now().toString().slice(-6);
            
            // Basic requirements
            if (!scrollContainer || logs.length === 0) return;
            
            debugLog(`LOG UPDATE DETECTED [id=${logEventId}], logs=${logs.length}`, "INFO");
            
            // Check auto-scroll state
            // Get current auto-scroll state from Svelte
            const autoScrollEnabled = autoScroll;
            
            // Get visible checkbox state
            const visibleCheckbox = document.getElementById('auto-scroll-checkbox') as HTMLInputElement;
            const uiCheckboxState = visibleCheckbox ? visibleCheckbox.checked : 'unknown';
            
            // Log current states
            debugLog(`Auto-scroll states: DOM=${autoScrollEnabled}, UI=${uiCheckboxState}, Svelte=${autoScroll} [id=${logEventId}]`, "INFO");
            
            // If auto-scroll is disabled, don't do anything
            if (!autoScrollEnabled) {
                debugLog(`Auto-scroll disabled, not scrolling [id=${logEventId}]`, "DEBUG");
                return;
            }
            
            // Never interrupt active user scrolling or manual lock
            if (isUserScrolling || manualScrollLock) {
                debugLog(`User is actively scrolling or manual lock active, won't auto-scroll [id=${logEventId}]`, "DEBUG");
                return;
            }
            
            // Get current scroll position
            const scrollPos = scrollContainer.scrollTop;
            debugLog(`Current scroll position: ${scrollPos}px [id=${logEventId}]`, "INFO");
            
            // CRITICAL CONSISTENCY CHECK:
            // If we think auto-scroll is on but we're not at bottom, we need to fix this
            if (scrollPos > 0 && autoScrollEnabled) {
                debugLog(`INCONSISTENCY DETECTED: scrollTop=${scrollPos}px but auto-scroll=true [id=${logEventId}]`, "WARN");
                
                // If very close to bottom, just force it down
                if (scrollPos < 5 && !isUserScrolling && !manualScrollLock) {
                    trackScrollTrigger('logUpdate:closeToBottom');
                    scrollContainer.scrollTop = 0;
                    debugLog(`Very close to bottom (${scrollPos}px), forced to bottom [id=${logEventId}]`, "INFO");
                } else {
                    // Otherwise disable auto-scroll
                    debugLog(`Not at bottom (${scrollPos}px), DISABLING auto-scroll [id=${logEventId}]`, "WARN");
                    
                    // Use our setter function to ensure consistency
                    setAutoScroll(false, 'logUpdateInconsistency');
                }
                return;
            }
            
            // If we're at bottom and auto-scroll is on, scroll to display new logs
            if (scrollPos === 0 && autoScrollEnabled && !isUserScrolling && !manualScrollLock) {
                debugLog(`At exact bottom with auto-scroll on, scrolling to bottom [id=${logEventId}]`, "INFO");
                
                // Direct DOM manipulation for reliability
                trackScrollTrigger('logUpdate:exactBottom');
                scrollContainer.scrollTop = 0;
                
                // One retry is sufficient
                setTimeout(() => {
                    if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                        trackScrollTrigger('logUpdate:followUp');
                        scrollContainer.scrollTop = 0;
                        debugLog(`Follow-up scroll to bottom (scrollTop=${scrollContainer.scrollTop}px) [id=${logEventId}]`, "DEBUG");
                    }
                }, 10);
            }
        }); // Correctly close subscribe call
        
        // onMount cleanup function
        const cleanup = () => {
            resizeObserver.disconnect();
            unsubscribeLogStore();
            
            if (userScrollTimeout) clearTimeout(userScrollTimeout);
            if (scrollRAF) cancelAnimationFrame(scrollRAF);
            if (batchMeasurementTimer) clearTimeout(batchMeasurementTimer);
            if (forceScrollTimer) clearTimeout(forceScrollTimer);
            cancelPostProcessingChecks();
            if (stableAtBottomTimer) clearTimeout(stableAtBottomTimer);
            if (autoScrollToastTimer) clearTimeout(autoScrollToastTimer);
        };
        
        return cleanup; // Return the synchronous cleanup function
    });
    
    onDestroy(() => {
        // Clean up velocity decay timer
        if (velocityDecayTimer) {
            clearInterval(velocityDecayTimer);
            velocityDecayTimer = null;
        }
        // Clean up any remaining timeouts
        if (userScrollTimeout) {
            clearTimeout(userScrollTimeout);
            userScrollTimeout = null;
        }
        
        if (scrollRAF) {
            cancelAnimationFrame(scrollRAF);
            scrollRAF = null;
        }
        
        if (batchMeasurementTimer) {
            clearTimeout(batchMeasurementTimer);
            batchMeasurementTimer = null;
        }
        
        if (forceScrollTimer) {
            clearTimeout(forceScrollTimer);
            forceScrollTimer = null;
        }
        
        // Clean up any post-processing timers
        cancelPostProcessingChecks();
        
        // Clean up stable bottom checking timer
        if (stableAtBottomTimer) {
            clearTimeout(stableAtBottomTimer);
            stableAtBottomTimer = null;
        }
        
        // Clean up auto-scroll toast timer
        if (autoScrollToastTimer) {
            clearTimeout(autoScrollToastTimer);
            autoScrollToastTimer = null;
        }
    });
</script>

<!-- Main container for the log viewer with glassmorphism -->
<div class="flex flex-col h-full bg-logbg/60 text-white font-[DM_Mono] text-[11px] rounded-lg border-r border-b border-primary/20 shadow-log"
     role="log"
     aria-label="Application logs"
     aria-live="polite">
    <!-- Top controls row -->
    <div class="px-3 py-2 border-b border-primary/20 bg-bgold-800/60 backdrop-blur-md h-10 flex items-center justify-between rounded-t-lg">
        <div class="flex items-center gap-6">
            <!-- Log level filter -->
            <div class="flex items-center gap-2 whitespace-nowrap">
                <span class="text-xs uppercase tracking-wider font-medium text-primary-100/60" id="log-level-label">
                    Log Level:
                </span>
                <select
                    bind:value={selectedLogLevel}
                    class="w-20 h-7 bg-[#333] text-white text-[11px] font-medium uppercase tracking-wider
                           border border-primary/20 rounded px-2 py-1.5
                           focus:ring-1 focus:ring-primary/50 focus:border-primary outline-none focus:shadow-input-focus
                           hover:border-primary/55 hover:shadow-input transition-all duration-200
                           appearance-none
                           [background-image:url('data:image/svg+xml,%3Csvg%20xmlns%3D%22http%3A//www.w3.org/2000/svg%22%20fill%3D%22none%22%20viewBox%3D%220%200%2024%2024%22%20stroke%3D%22white%22%3E%3Cpath%20stroke-linecap%3D%22round%22%20stroke-linejoin%3D%22round%22%20stroke-width%3D%222%22%20d%3D%22M19%209l-7%207-7-7%22/%3E%3C/svg%3E')] 
                           bg-no-repeat bg-[right_0.5rem_center] bg-[length:1em]"
                    aria-labelledby="log-level-label"
                >
                    {#each logLevels as level}
                        <option value={level}>{level}</option>
                    {/each}
                </select>
            </div>

            <!-- Auto-scroll toggle with completely manual control -->
            <div class="flex items-center gap-1 px-3 py-1 bg-[#333] h-7 rounded hover:bg-primary/10 hover:border-primary/55 hover:shadow-input transition-all duration-200">
                <input
                    id="auto-scroll-checkbox"
                    type="checkbox"
                    checked={autoScroll}
                    on:change={(e) => {
                        // Stop event propagation
                        e.stopPropagation();
                        
                        // Get directly from the DOM
                        const target = e.target as HTMLInputElement;
                        const newValue = target.checked;
                        
                        // Log the event detail
                        debugLog(`Checkbox changed: ${newValue}, id=${e.timeStamp}`, "INFO");
                        
                        // Log the current state before changing
                        debugLog(`Before change - autoScroll: ${autoScroll}`, "INFO");
                        
                        // CRITICAL: When user directly toggles checkbox, clear the manual scroll lock
                        // This gives precedence to explicit user preference
                        if (manualScrollLock) {
                            debugLog(`Clearing manual scroll lock due to explicit user checkbox toggle`, "INFO");
                            manualScrollLock = false;
                            if (manualScrollLockTimer) {
                                clearTimeout(manualScrollLockTimer);
                                manualScrollLockTimer = null;
                            }
                        }
                        
                        // Use our central setter with special source to indicate user preference
                        setAutoScroll(newValue, 'userPreference');
                        
                        // Schedule check to verify state is maintained
                        setTimeout(() => {
                            const visibleCheckbox = document.getElementById('auto-scroll-checkbox') as HTMLInputElement;
                            const uiState = visibleCheckbox ? visibleCheckbox.checked : 'unknown';
                            debugLog(`50ms after change - autoScroll: ${autoScroll}, checkbox UI: ${uiState}`, "INFO");
                        }, 50);
                    }}
                    class="w-3.5 h-3.5 accent-primary m-0 cursor-pointer"
                    aria-label="Toggle auto-scroll"
                />
                <label 
                    for="auto-scroll-checkbox"
                    class="cursor-pointer text-text text-[11px] uppercase tracking-wider whitespace-nowrap flex-shrink-0 hover:text-white transition-colors duration-200"
                    on:click={(e) => {
                        // We'll let the label handle naturally through the 'for' attribute
                        // which will trigger the checkbox change event
                    }}
                >
                    Auto-scroll
                </label>
            </div>

            <!-- Clear button -->
            <button 
                on:click={clearLogsPreserveAutoScroll}
                class="px-3 py-1 h-7 bg-[#333] text-text rounded whitespace-nowrap 
                       flex-shrink-0 text-[11px] uppercase tracking-wider 
                       hover:bg-primary/10 hover:text-white hover:border-primary/55 hover:shadow-input 
                       transition-all duration-200"
                aria-label="Clear logs"
            >
                Clear
            </button>
            
            <!-- Virtual Rendering Toggle (for debugging) -->
            {#if version === 'dev'}
                <button 
                    on:click={toggleVirtualization}
                    class="px-3 py-1 h-7 bg-[#333] text-text rounded whitespace-nowrap 
                           flex-shrink-0 text-[11px] uppercase tracking-wider 
                           hover:bg-primary/10 hover:text-white hover:border-primary/55 hover:shadow-input 
                           transition-all duration-200"
                    aria-pressed={virtualEnabled}
                    aria-label="Toggle virtualization"
                >
                    {virtualEnabled ? 'Virt: ON' : 'Virt: OFF'}
                </button>
                
                <!-- Debug info -->
                <span class="text-xs text-primary/50" aria-live="polite">
                    {filteredLogs.length} logs {virtualEnabled ? '| ' + visibleLogCount + ' visible' : ''} {isProcessing ? '| PROCESSING' : ''}
                </span>
            {/if}

            <!-- Debug Auto-Scroll Button -->
            {#if version === 'dev'}
                <button
                    on:click={() => debugAutoScroll = !debugAutoScroll}
                    class="px-3 py-1 h-7 bg-[#444] text-text rounded whitespace-nowrap
                           flex-shrink-0 text-[11px] uppercase tracking-wider
                           hover:bg-primary/20 hover:text-white hover:border-primary/55 hover:shadow-input
                           transition-all duration-200"
                    aria-pressed={debugAutoScroll}
                >
                    Debug Scroll
                </button>
            {/if}
        </div>
    </div>
    
    <!-- Content area with virtualization -->
    <div class="relative flex flex-col flex-1 min-h-0">
        <!-- The scrollable container for log entries with terminal-mode (column-reverse) -->
        <div 
            class="flex-1 overflow-y-auto overflow-x-hidden min-h-0 log-scroll-container terminal-mode"
            class:autoscroll-active={autoScroll}
            bind:this={scrollContainer}
            on:scroll={handleScroll}
            role="region"
            aria-label="Log entries"
        >
            {#if filteredLogs.length === 0}
                <!-- Empty state -->
                <div class="absolute top-0 left-0 w-full h-full flex items-center justify-center">
                    <span class="bg-black/10 backdrop-blur-sm border border-primary/30 text-primary/60 italic text-sm px-6 py-3 rounded-lg" aria-live="polite">
                        No logs to display
                    </span>
                </div>
            {:else}
                <!-- Virtual scroller container -->
                <div 
                    class="relative w-full" 
                    style="height: {virtualEnabled && virtualizationReady ? `${virtualContainerHeight}px` : 'auto'}"
                    aria-hidden={virtualEnabled ? "true" : "false"}
                >
                    <!-- Initial loading state before virtualization is ready -->
                    {#if virtualEnabled && !virtualizationReady}
                        <!-- Show the first 50 logs in non-virtualized mode until virtualization is ready -->
                        {#each filteredLogs.slice(0, 50) as log (log._sequence)}
                            <div 
                                class="log-entry {log.behavior ? behaviorColors[log.behavior] : 'text-white/90'}
                                py-1.5 px-3 border-b border-primary/10 
                                flex items-start justify-start text-left w-full hover:bg-white/5 transition-colors duration-200"
                                data-log-sequence={log._sequence}
                                data-unix-time={log._unix_time ?? 0}
                                use:measureLogEntry={log}
                                role="listitem"
                                aria-label={`${log.level} log: ${log.message || ''}`}
                            >
                                <!-- Timestamp -->
                                <span class="text-primary/60 mr-2 mt-0.5 text-xs flex-shrink-0" aria-label="Log time">
                                    {log.time}
                                </span>
                                
                                <!-- Log level -->
                                <span class={"font-bold mt-0.5 text-sm mr-2 flex-shrink-0 min-w-[40px] " + getLevelClass(log.level)} aria-label={`Log level: ${log.level}`}>
                                    {log.level}
                                </span>
                                
                                <!-- Content column for message and/or fields -->
                                <div class="flex-grow">
                                    <!-- Message (if present) -->
                                    {#if log.message}
                                        <span class="text-sm text-left leading-relaxed whitespace-pre-wrap break-words">
                                            {log.message}
                                        </span>
                                    {/if}
                                    
                                    <!-- Show structured fields inline if no message, otherwise in next line -->
                                    {#if formatFields(log)}
                                        <span class="{log.message ? 'block mt-0.5 ' : ''}text-primary/50 text-[11px] leading-relaxed whitespace-pre-wrap break-words" aria-label="Additional log details">
                                            {formatFields(log)}
                                        </span>
                                    {/if}
                                </div>
                            </div>
                        {/each}
                    <!-- Virtualized rendering once measurements are ready -->
                    {:else if virtualEnabled && virtualizationReady}
                        {#each filteredLogs.slice(virtualStart, virtualEnd + 1) as log (log._sequence)}
                            <div 
                                class="log-entry {log.behavior ? behaviorColors[log.behavior] : 'text-white/90'}
                                py-1.5 px-3 border-b border-primary/10 
                                flex items-start justify-start text-left w-full hover:bg-white/5 transition-colors duration-200"
                                style="position: absolute; bottom: {totalLogHeight - (logPositions.get(log._sequence) || 0) - (logHeights.get(log._sequence) || 0)}px; left: 0; right: 0;"
                                data-log-sequence={log._sequence}
                                data-unix-time={log._unix_time ?? 0}
                                use:measureLogEntry={log}
                                role="listitem"
                                aria-label={`${log.level} log: ${log.message || ''}`}
                            >
                                <!-- Timestamp -->
                                <span class="text-primary/60 mr-2 mt-0.5 text-xs flex-shrink-0" aria-label="Log time">
                                    {log.time}
                                </span>
                                
                                <!-- Log level -->
                                <span class={"font-bold mt-0.5 text-sm mr-2 flex-shrink-0 min-w-[40px] " + getLevelClass(log.level)} aria-label={`Log level: ${log.level}`}>
                                    {log.level}
                                </span>
                                
                                <!-- Content column for message and/or fields -->
                                <div class="flex-grow">
                                    <!-- Message (if present) -->
                                    {#if log.message}
                                        <span class="text-sm text-left leading-relaxed whitespace-pre-wrap break-words">
                                            {log.message}
                                        </span>
                                    {/if}
                                    
                                    <!-- Show structured fields inline if no message, otherwise in next line -->
                                    {#if formatFields(log)}
                                        <span class="{log.message ? 'block mt-0.5 ' : ''}text-primary/50 text-[11px] leading-relaxed whitespace-pre-wrap break-words" aria-label="Additional log details">
                                            {formatFields(log)}
                                        </span>
                                    {/if}
                                </div>
                            </div>
                        {/each}
                    {:else}
                        <!-- Non-virtualized rendering (all logs) with animations -->
                        {#each filteredLogs as log (log._sequence)}
                            <div 
                                class="log-entry {log.behavior ? behaviorColors[log.behavior] : 'text-white/90'}
                                py-1.5 px-3 border-b border-primary/10 
                                flex items-start justify-start text-left w-full hover:bg-white/5 transition-colors duration-200
                                {isNewLog(log) ? 'new-log ' + getFlashClass(log.level) : ''}"
                                data-log-sequence={log._sequence}
                                data-unix-time={log._unix_time ?? 0}
                                use:measureLogEntry={log}
                                transition:fade|local={{ duration: autoScroll ? 150 : 300, easing: autoScroll ? undefined : backOut }}
                                on:introstart={handleTransitionStart}
                                on:introend={handleTransitionEnd}
                                on:outrostart={handleTransitionStart}
                                on:outroend={handleTransitionEnd}
                                role="listitem"
                                aria-label={`${log.level} log: ${log.message || ''}`}
                            >
                                <!-- Timestamp -->
                                <span class="text-primary/60 mr-2 mt-0.5 text-xs flex-shrink-0" aria-label="Log time">
                                    {log.time}
                                </span>
                                
                                <!-- Log level -->
                                <span class={"font-bold mt-0.5 text-sm mr-2 flex-shrink-0 min-w-[40px] " + getLevelClass(log.level)} aria-label={`Log level: ${log.level}`}>
                                    {log.level}
                                </span>
                                
                                <!-- Content column for message and/or fields -->
                                <div class="flex-grow">
                                    <!-- Message (if present) -->
                                    {#if log.message}
                                        <span class="text-sm text-left leading-relaxed whitespace-pre-wrap break-words">
                                            {log.message}
                                        </span>
                                    {/if}
                                    
                                    <!-- Show structured fields inline if no message, otherwise in next line -->
                                    {#if formatFields(log)}
                                        <span class="{log.message ? 'block mt-0.5 ' : ''}text-primary/50 text-[11px] leading-relaxed whitespace-pre-wrap break-words" aria-label="Additional log details">
                                            {formatFields(log)}
                                        </span>
                                    {/if}
                                </div>
                            </div>
                        {/each}
                    {/if}
                </div>
            {/if}
        </div>
    </div>

    <!-- Debug Auto-Scroll Overlay (Updated) -->
    {#if version === 'dev' && scrollContainer}
        <!-- Debug overlay for auto-scroll investigation -->
        <div class="fixed bottom-4 left-4 bg-black/80 p-3 text-white text-xs rounded shadow-lg z-50 pointer-events-none border-2 border-red-500">
            <div class="text-sm font-bold mb-1 text-red-400">AUTO-SCROLL DEBUGGER</div>
            <div class="flex flex-col space-y-0.5">
                <div>Auto-Scroll: {autoScroll ? 'ON' : 'OFF'}</div>
                <div>UI Checkbox: {document.getElementById('auto-scroll-checkbox')?.checked ? 'ON' : 'OFF'}</div>
                <div class="border-t border-gray-600 my-1 pt-1"></div>
                <div>ScrollTop: {Math.abs(scrollContainer?.scrollTop || 0).toFixed(1)}px</div>
                <div>At Bottom: {Math.abs(scrollContainer?.scrollTop || 0) === 0 ? 'YES' : 'NO'}</div>
                <div>Direction: {scrollDirectionToBottom ? '⬇️ TO BOTTOM' : '⬆️ TO TOP'}</div>
                <div>Velocity: {scrollVelocity.toFixed(2)}</div>
                <div class="font-bold {isUserScrolling ? 'text-green-400' : 'text-red-400'}">User Scrolling: {isUserScrolling ? 'YES' : 'NO'}</div>
                <div class="font-bold {manualScrollLock ? 'text-green-400' : ''}">Manual Lock: {manualScrollLock ? 'YES' : 'NO'}</div>
                
                <!-- SCROLL TRIGGER TRACERS -->
                <div class="border-t border-gray-600 my-1 pt-1"></div>
                <div class="text-red-400 font-bold">Last Scroll Triggers:</div>
                {#each scrollTriggerHistory as trace, i}
                    <div class="text-xs">
                        {i+1}. {trace.id} ({(Date.now() - trace.timestamp < 1000) ? 
                              ((Date.now() - trace.timestamp) + 'ms ago') : 
                              ((Date.now() - trace.timestamp)/1000).toFixed(1) + 's ago'})
                    </div>
                {/each}
                {#if scrollTriggerHistory.length === 0}
                    <div class="text-gray-400 text-xs">No scroll triggers yet</div>
                {/if}
            </div>
        </div>
    {/if}
</div>

<style>
    /* Custom scrollbar styling for log viewer - only Y axis visible */
    .log-scroll-container {
        scrollbar-width: thin;
        scrollbar-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4) transparent;
        scroll-behavior: smooth;
    }

    /* Terminal mode - column-reverse layout for fixed scrollbar position */
    .terminal-mode {
        display: flex;
        flex-direction: column-reverse;
    }

    /* Autoscroll active class */
    .autoscroll-active {
        /* Disable smooth scrolling when auto-scroll is active */
        scroll-behavior: auto !important;
    }

    .autoscroll-active::-webkit-scrollbar-thumb {
        opacity: 0.3;
        transition: opacity 0.3s ease;
    }

    .autoscroll-active:hover::-webkit-scrollbar-thumb {
        opacity: 1;
    }

    .log-scroll-container::-webkit-scrollbar {
        width: 6px;
        height: 0; /* Hide horizontal scrollbar */
    }

    .log-scroll-container::-webkit-scrollbar-track {
        background: transparent;
        margin: 4px 0;
    }

    .log-scroll-container::-webkit-scrollbar-thumb {
        background-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4);
        border-radius: 3px;
        transition: background-color 0.2s ease;
    }

    .log-scroll-container::-webkit-scrollbar-thumb:hover {
        background-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.7);
    }

    /* Hide horizontal scrollbar */
    .log-scroll-container::-webkit-scrollbar-horizontal {
        display: none;
    }

    /* LogViewer border glow effect, right and bottom edges as specified */
    :global(.shadow-log) {
        box-shadow: 
            /* Main box shadow for depth */
            0 10px 30px 0 rgba(0, 0, 0, 0.3),
            /* Right glow - simulating light hitting from bottom right */
            3px 0 15px 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.2),
            /* Bottom glow - stronger to simulate the light source */
            0 3px 15px 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.3),
            /* Subtle inner glow for depth */
            inset 0 0 30px 0 rgba(0, 0, 0, 0.2);
    }
    
    /* Hover effect for log entries */
    :global(.hover\:bg-white\/5:hover) {
        background-color: rgba(255, 255, 255, 0.05);
        box-shadow: 0 0 1px 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.1);
    }
    
    /* Subtle styling for log entries */
    :global(.border-primary\/10) {
        background-color: rgba(0, 0, 0, 0.05);
        transition: background-color 0.2s ease;
    }
    
    :global(.border-primary\/10:hover) {
        background-color: rgba(0, 0, 0, 0.1);
    }
    
    /* Styled log levels with enhanced visual treatment */
    .log-level-debug {
        text-shadow: 0 0 6px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4);
        font-weight: 500;
    }
    
    .log-level-info {
        text-shadow: 0 0 6px rgba(104, 231, 150, 0.4);
        font-weight: 500;
    }
    
    .log-level-warn {
        text-shadow: 0 0 6px rgba(255, 243, 142, 0.5);
        font-weight: 600;
    }
    
    .log-level-error {
        text-shadow: 0 0 8px rgba(239, 68, 68, 0.5);
        font-weight: 700;
        letter-spacing: 0.5px;
    }
    
    /* Add subtle hover animation to log timestamps */
    :global(.text-primary\/60) {
        transition: color 0.2s ease, text-shadow 0.2s ease;
    }
    
    :global(.border-primary\/10:hover .text-primary\/60) {
        color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.8);
        text-shadow: 0 0 3px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.3);
    }
    
    /* Enhanced styling for behavior-specific log entries using HSL variables */
    .log-behavior-abort-task {
        background: linear-gradient(
            to right,
            hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.08) 0%,
            rgba(0, 0, 0, 0) 70%
        );
        border-left: 2px solid hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.6);
    }
    
    .log-behavior-abort-all {
        background: linear-gradient(
            to right,
            hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.1) 0%,
            rgba(0, 0, 0, 0) 70%
        );
        border-left: 2px solid hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.6);
    }
    
    .log-behavior-cancel {
        background: linear-gradient(
            to right,
            hsla(var(--user-cancel-hue), var(--user-cancel-saturation), var(--user-cancel-lightness), 0.08) 0%,
            rgba(0, 0, 0, 0) 70%
        );
        border-left: 2px solid hsla(var(--user-cancel-hue), var(--user-cancel-saturation), var(--user-cancel-lightness), 0.5);
    }
    
    .log-behavior-probe {
        background: linear-gradient(
            to right,
            hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.08) 0%,
            rgba(0, 0, 0, 0) 70%
        );
        border-left: 2px solid hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.4);
    }
    
    /* ====== MICRO-INTERACTIONS ====== */
    
    /* 1. Log Entry Arrival Animation */
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
    
    /* Flash border animations for each log level */
    .flash-debug {
        animation: flashDebugBorder 1s ease-out;
    }
    
    .flash-info {
        animation: flashInfoBorder 1s ease-out;
    }
    
    .flash-warn {
        animation: flashWarnBorder 1s ease-out;
    }
    
    .flash-error {
        animation: flashErrorBorder 1s ease-out;
    }
    
    @keyframes flashDebugBorder {
        0%, 10% {
            box-shadow: 0 0 0 2px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.6);
        }
        100% {
            box-shadow: 0 0 0 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0);
        }
    }
    
    @keyframes flashInfoBorder {
        0%, 10% {
            box-shadow: 0 0 0 2px rgba(104, 231, 150, 0.6);
        }
        100% {
            box-shadow: 0 0 0 0 rgba(104, 231, 150, 0);
        }
    }
    
    @keyframes flashWarnBorder {
        0%, 10% {
            box-shadow: 0 0 0 2px rgba(255, 243, 142, 0.7);
        }
        100% {
            box-shadow: 0 0 0 0 rgba(255, 243, 142, 0);
        }
    }
    
    @keyframes flashErrorBorder {
        0%, 10% {
            box-shadow: 0 0 0 2px rgba(239, 68, 68, 0.7);
        }
        100% {
            box-shadow: 0 0 0 0 rgba(239, 68, 68, 0);
        }
    }
</style>