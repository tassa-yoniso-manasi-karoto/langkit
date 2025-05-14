<script lang="ts">
    import { onMount, onDestroy, tick, afterUpdate } from 'svelte';
    import { get } from 'svelte/store';
    import { settings } from '../lib/stores';
    import { logStore, type LogMessage } from '../lib/logStore';
    import { logger } from '../lib/logger';
    import { slide, fade } from 'svelte/transition';
    import { backOut } from 'svelte/easing';

    // Listen for dev dashboard events
    function handleToggleVirtualization() {
        if (version === 'dev') {
            manualVirtualToggle = true;
            virtualEnabled = !virtualEnabled;
            logger.trace('logViewer', `Virtualization ${virtualEnabled ? 'enabled' : 'disabled'} by dev dashboard`);

            if (virtualEnabled) {
                setTimeout(() => resetVirtualization(), 50);
            }
        }
    }

    function handleCheckVirtualization() {
        if (version === 'dev' && virtualEnabled && virtualizationReady) {
            logger.trace('logViewer', "Virtualization check triggered from dev dashboard");
            recalculatePositions();
            updateVirtualization();

            // Schedule additional checks
            [50, 200, 500].forEach(delay => {
                setTimeout(() => {
                    if (virtualEnabled && virtualizationReady) {
                        logger.trace('logViewer', `Check at t+${delay}ms`);
                        updateVirtualization();
                    }
                }, delay);
            });
        }
    }

    function handleToggleDebugScroll() {
        if (version === 'dev') {
            debugAutoScroll = !debugAutoScroll;
            logger.trace('logViewer', `Debug scroll overlay ${debugAutoScroll ? 'enabled' : 'disabled'} by dev dashboard`);
        }
    }

    function handleForceScrollBottom() {
        if (version === 'dev') {
            logger.trace('logViewer', "Force scroll to bottom triggered from dev dashboard");
            scrollToBottomWithStrategy();
        }
    }

    function handleClearLogs() {
        if (version === 'dev') {
            logger.trace('logViewer', "Clearing logs from dev dashboard");
            logStore.set([]);
        }
    }

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
        'abort_task': 'text-error-soft log-behavior-abort-task',
        'abort_all': 'text-error-hard log-behavior-abort-all',
        'user_cancel': 'text-user-cancel log-behavior-cancel',
        'probe': 'text-log-warn log-behavior-probe'
    };

    // DOM references
    let scrollContainer: HTMLElement;
    
    // Debugging and development logging
    let debug = version === 'dev';
    let debugAutoScroll = false; // Developer option for auto-scroll debug overlay
    
    // --- Core Auto-Scroll State ---
    let autoScroll = true; // Default: ON. Single source of truth.
    let viewportAnchor: { 
      type?: string,
      index: number,
      sequence: number, 
      offsetTop: number
    } | null = null; // Stores VAS data
    let isUserScrolling = false; // True during user scroll gestures
    let manualScrollLock = false; // Prevent auto-scroll from fighting with user
    let isProgrammaticScroll = false; // Flag to ignore programmatic scroll events
    
    // Scroll state management - ENHANCED
    let userScrollTimeout: number | null = null;
    let scrollRAF: number | null = null;
    let lastScrollTop = 0;
    let lastScrollTime = Date.now();
    let scrollVelocity = 0;
    let scrollSamples: number[] = []; // Keep a history of recent scroll deltas
    let velocityDecayTimer: number | null = null; // Timer for velocity decay
    let manualScrollLockTimer: number | null = null;
    
    // User intent tracking
    let scrollDirectionToBottom = false; // Keep existing
    let lastDirectionChangeTime = 0;
    let consistentDirectionDuration = 0;
    let intentToReturnToBottom = false;
    
    // Animation state tracking
    let animationInProgress = false;
    let pendingScrollToBottom = false;
    let activeTransitions = 0;
    
    // Timer management
    let postProcessingTimers: number[] = [];
    let scrollMonitorInterval: number | null = null; // For polling scrollTop changes
    let lastKnownScrollTop = 0; // Track last known scrollTop for monitoring
    let scheduleForceScrollTimer: number | null = null; // Timer for scheduleForceScroll
    
    // DEBUG: Tracking which parts of the code trigger scrolls
    let scrollTriggerHistory: {id: string, timestamp: number}[] = [];
    const MAX_SCROLL_HISTORY = 5; // Only keep last 5 events

    // --- Virtualization ---
    let virtualEnabled = false; // Start with virtualization disabled
    let virtualizationReady = false;
    let initialMeasurementsComplete = false;
    let manualVirtualToggle = false; // Track if user manually toggled virtualization
    
    // Virtual viewport tracking
    let virtualStart = 0;
    let virtualEnd = 0;
    const BUFFER_SIZE = 50; // How many logs to render above/below viewport
    let viewportHeight = 0;
    let virtualContainerHeight = 0;
    let visibleLogCount = 0;
    
    // Height tracking
    let logHeights: Map<number, number> = new Map(); // Maps sequence -> actual height
    let logPositions: Map<number, number> = new Map(); // Maps sequence -> Y position
    let totalLogHeight = 0;
    let avgLogHeight = 25; // Initial estimate, will be refined
    
    // Positioning buffer (to prevent overlap)
    const POSITION_BUFFER = 2; // Add 2px buffer between entries
    
    // Mass log addition detection
    let recentLogAdditions = 0;
    let lastLogRateCheck = Date.now();
    
    // Animation and filter state
    let filterTransitionRunning = false;
    let filterChangeTimestamp = Date.now();
    let recentlyAddedLogs: Set<number> = new Set();
    
    // Track measurement batches
    let pendingMeasurements = false;
    let batchMeasurementTimer: number | null = null;
    
    // Auto-scroll debug stats 
    let lastAutoScrollTime = Date.now();
    let autoScrollTriggerCount = 0;
    let logsBatchedSinceLastScroll = 0;

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
                    logger.trace('logViewer', `Auto-toggling virtualization: ${shouldVirtualize ? 'ON' : 'OFF'} (log count: ${filteredLogs.length}, threshold: ${threshold})`);
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
            if (debug) logger.trace('logViewer', `Processing state changed: ${prevIsProcessing} -> ${isProcessing}`);
            
            // CRITICAL FIX: Ensure we're at the bottom when processing starts (if auto-scroll is enabled)
            if (isProcessing && !prevIsProcessing && autoScroll && scrollContainer) {
                if (debug) logger.trace('logViewer', "Processing started - forcing scroll to bottom");
                
                // Force to bottom immediately
                scrollContainer.scrollTop = 0;
                
                // Also schedule some follow-up checks during processing
                const processingChecks = [100, 500, 1000];
                processingChecks.forEach(delay => {
                    setTimeout(() => {
                        if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                            if (scrollContainer.scrollTop !== 0) {
                                if (debug) logger.trace('logViewer', `Mid-processing scroll check (${delay}ms): fixing scrollTop=${scrollContainer.scrollTop}`);
                                scrollContainer.scrollTop = 0;
                            }
                        }
                    }, delay);
                });
            }
            
            // When processing ends, schedule final scroll checks
            if (!isProcessing && prevIsProcessing) {
                if (debug) logger.trace('logViewer', "Processing ended - scheduling post-processing actions");
                
                // For auto-scroll, we want to keep scrolled to bottom
                if (autoScroll) {
                    schedulePostProcessingScrolls();
                }
                
                // CRITICAL FIX: If virtualization is enabled, force full recalculation
                // This ensures we update the view properly when scrolling after processing ends
                if (virtualEnabled && virtualizationReady) {
                    // Schedule a series of virtualization updates after processing
                    // This helps ensure the view is correct even if scroll events are missed
                    const postProcessingChecks = [50, 200, 500, 1000, 2000, 3000];
                    
                    postProcessingChecks.forEach(delay => {
                        setTimeout(() => {
                            if (debug) logger.trace('logViewer', `Post-processing virtualization check at t+${delay}ms`);
                            
                            // Force update virtualization
                            if (virtualEnabled && virtualizationReady) {
                                // Recalculate positions first
                                recalculatePositions();
                                
                                // Then update virtualization based on current scroll position
                                updateVirtualization();
                                
                                // Force reactive updates by reassigning
                                virtualStart = virtualStart;
                                virtualEnd = virtualEnd;
                            }
                        }, delay);
                    });
                }
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
        
        // CRITICAL FIX: Always ensure we're at bottom if auto-scroll is enabled
        if (autoScroll && !isUserScrolling && !manualScrollLock) {
            // Force to bottom immediately - this is essential for both virtualized and non-virtualized modes
            if (scrollContainer.scrollTop !== 0) {
                if (debug) logger.trace('logViewer', `Force scrollTop=0 for auto-scroll (current=${scrollContainer.scrollTop})`);
                scrollContainer.scrollTop = 0;
            }
        }
        // If auto-scroll is on but we're not at the bottom, this is inconsistent
        else if (autoScroll && !exactlyAtBottom && !isUserScrolling) {
            if (debug) logger.trace('logViewer', `Detected inconsistency: auto-scroll ON but scrollTop=${scrollContainer.scrollTop}px`);
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
                    if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                        scrollContainer.scrollTop = 0; // Direct DOM manipulation is most reliable
                    }
                }
                
                // CRITICAL: For non-virtualized mode + auto-scroll, ensure we're at bottom after animation
                if (!virtualEnabled && autoScroll && !isUserScrolling && !manualScrollLock) {
                    if (debug) logger.trace('logViewer', "Force to bottom after animation in non-virtualized mode");
                    scrollContainer.scrollTop = 0;
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
                if (autoScroll && !isUserScrolling && !manualScrollLock) {
                    // IMPORTANT: Force to bottom if auto-scroll is enabled - regardless of current position
                    // This ensures auto-scroll always works even in complex DOM update scenarios
                    scrollContainer.scrollTop = 0;
                } else if (!autoScroll && viewportAnchor) {
                    // Otherwise restore anchor position
                    restoreViewportAnchor();
                } else if (autoScroll && !stillAtBottom && !isUserScrolling) {
                    // Fix inconsistency: we're supposed to be at bottom but aren't
                    setAutoScroll(false, 'measurementInconsistency');
                }
                
                pendingMeasurements = false;
                batchMeasurementTimer = null;
                
                // Update return to bottom button visibility
                updateReturnToBottomButtonVisibility();
                
                // FINAL SAFETY: One last scroll check for auto-scroll enabled
                if (autoScroll && !isUserScrolling && !manualScrollLock) {
                    setTimeout(() => {
                        if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                            if (scrollContainer.scrollTop !== 0) {
                                if (debug) logger.trace('logViewer', "Final safety scroll to bottom");
                                scrollContainer.scrollTop = 0;
                            }
                        }
                    }, 50);
                }
            }, 10); // Small delay to batch updates
        }
    }
    
    // --- Helper Functions ---
    
    // Track scroll triggers for debug overlay
    function trackScrollTrigger(triggerId: string) {
        if (debug) logger.trace('logViewer', `SCROLL TRIGGER: ${triggerId}`);
        
        // Create a new array with the new trigger at the beginning
        scrollTriggerHistory = [
            {
                id: triggerId,
                timestamp: Date.now()
            },
            ...scrollTriggerHistory.slice(0, MAX_SCROLL_HISTORY - 1)
        ];
    }
    
    // --- Core Auto-Scroll Setter ---
    function setAutoScroll(newValue: boolean, source: string = 'direct') {
        // Skip if no change
        if (newValue === autoScroll) return;
        
        // Debug logging
        if (debug) logger.trace('logViewer', `Auto-scroll ${newValue ? 'enabled' : 'disabled'} via ${source}`);
        
        // Track this state change
        trackScrollTrigger(`setAutoScroll:${newValue ? 'ON' : 'OFF'}:${source}`);
        
        // IMPORTANT: If trying to enable auto-scroll but manual lock is active, refuse
        if (newValue && manualScrollLock && source !== 'userPreference') {
            if (debug) logger.warn('logViewer', `Auto-scroll enable BLOCKED due to active manual lock`);
            return; // Early return - don't enable against user's wishes
        }
        
        // Update our state variable
        autoScroll = newValue;
        
        // Additional state updates when needed
        if (newValue) {
            // When enabling auto-scroll:
            viewportAnchor = null; // Clear any saved position
            
            // CRITICAL ENHANCEMENT: ALWAYS scroll to bottom when enabling auto-scroll
            // This ensures we move to the latest content immediately
            if (scrollContainer && !isUserScrolling) {
                // Track this scroll 
                trackScrollTrigger(`setAutoScroll:forcedScroll:${source}`);
                
                // First direct scrolling attempt
                withProgrammaticScroll(() => {
                    scrollContainer.scrollTop = 0;
                });
                
                // Attempt with scrollTo API for better browser compatibility
                try {
                    scrollContainer.scrollTo({
                        top: 0,
                        behavior: 'auto' // Instant, not smooth
                    });
                } catch (e) {
                    // Fallback if scrollTo isn't supported
                    scrollContainer.scrollTop = 0;
                }
                
                // For virtualized mode, also update the virtualization window
                if (virtualEnabled && virtualizationReady) {
                    // Update to show newest logs
                    const lastLogIndex = filteredLogs.length - 1;
                    virtualEnd = lastLogIndex; 
                    virtualStart = Math.max(0, lastLogIndex - 60);
                    
                    // Force update
                    updateVirtualization();
                }
                
                // Multiple retries for reliability - critical for processing state
                const retryDelays = [50, 100, 300, 500];
                retryDelays.forEach(delay => {
                    setTimeout(() => {
                        if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                            if (scrollContainer.scrollTop !== 0) {
                                if (debug) logger.trace('logViewer', `setAutoScroll retry (${delay}ms): forcing scrollTop=0`);
                                scrollContainer.scrollTop = 0;
                            }
                        }
                    }, delay);
                });
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
    
    // Wrapper to mark scroll operations as programmatic
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
    
    // Check if we're exactly at the bottom
    function isScrolledToBottom(tolerance = 0): boolean {
        if (!scrollContainer) return true;
        
        // Check if scrolling is even possible
        const canScrollFurther = scrollContainer.scrollHeight > scrollContainer.clientHeight;
        if (!canScrollFurther) return true; // Can't scroll, so we're at the "bottom"
        
        // In column-reverse, we're only at bottom when scrollTop is EXACTLY 0
        // Only use tolerance for special cases where explicitly requested
        return scrollContainer.scrollTop <= tolerance;
    }
    
    // Force scroll to bottom
    function forceScrollToBottom() {
        // CRITICAL: Don't scroll if auto-scroll is off, user is scrolling, or manual lock active
        if (!scrollContainer || !autoScroll || isUserScrolling || manualScrollLock) {
            if (debug) logger.warn('logViewer', `Forced scroll BLOCKED: autoScroll=${autoScroll}, userScrolling=${isUserScrolling}, manualLock=${manualScrollLock}`);
            return;
        }
        
        // Set flag to avoid our scroll handler treating this as user scroll
        isProgrammaticScroll = true;
        
        try {
            // Record this scroll attempt
            trackScrollTrigger('forceScrollToBottom:main');
            
            // In column-reverse, set scrollTop to 0 to get to the bottom (newest logs)
            scrollContainer.scrollTop = 0;
            
            // Use multiple techniques with escalating forcefulness
            requestAnimationFrame(() => {
                // CRITICAL CHECK: Only if still eligible for scrolling 
                if (!scrollContainer || !autoScroll || isUserScrolling || manualScrollLock) {
                    isProgrammaticScroll = false;
                    return;
                }
                
                // Record this scroll attempt
                trackScrollTrigger('forceScrollToBottom:rAF');
                
                // Try direct assignment first
                scrollContainer.scrollTop = 0;
                
                // Then use scrollTo with instant behavior
                try {
                    scrollContainer.scrollTo({ top: 0, behavior: 'instant' });
                } catch (e) {
                    // No fallback needed - we already did scrollTop = 0
                }
                
                // ONE retry is sufficient
                setTimeout(() => {
                    // Final check before scrolling
                    if (!scrollContainer || !autoScroll || isUserScrolling || manualScrollLock) {
                        isProgrammaticScroll = false;
                        return;
                    }
                    
                    trackScrollTrigger('forceScrollToBottom:finalCheck');
                    
                    // Still maintain programmatic flag
                    isProgrammaticScroll = true;
                    
                    // One final scroll attempt
                    scrollContainer.scrollTop = 0;
                    
                    // End programmatic scroll state
                    setTimeout(() => {
                        isProgrammaticScroll = false;
                    }, 10);
                }, 50);
            });
        } catch (e) {
            isProgrammaticScroll = false;
            logger.error('logViewer', "Error in force scroll", { error: e });
        }
    }
    
    // Ultra-simplified scroll to bottom function
    function scrollToBottomWithStrategy(): void {
        // CRITICAL FIX: If auto-scroll is off, DO NOT SCROLL under any circumstances
        if (!autoScroll || !scrollContainer) return;
        
        // If user is actively scrolling or manual lock is active, DO NOT SCROLL
        if (isUserScrolling || manualScrollLock) {
            if (debug) logger.warn('logViewer', "Blocked auto-scroll due to user scrolling or manual lock");
            return;
        }

        // Track this scroll trigger
        trackScrollTrigger('scrollToBottomWithStrategy:main');
        
        // First, handle the immediate scroll action
        withProgrammaticScroll(() => {
            scrollContainer.scrollTop = 0;
        });
        
        // Special treatment for virtualization mode
        if (virtualEnabled && virtualizationReady) {
            // For virtualized mode, first ensure range includes newest logs
            const lastLogIndex = filteredLogs.length - 1;
            if (virtualEnd < lastLogIndex) {
                // Update range to include newest logs
                virtualEnd = lastLogIndex;
                virtualStart = Math.max(0, lastLogIndex - 60); // Show at least 60 latest logs
                
                // Force scroll after range update
                tick().then(() => {
                    // Check again if auto-scroll is still on
                    if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                        trackScrollTrigger('scrollToBottomWithStrategy:tick');
                        scrollContainer.scrollTop = 0;
                    }
                });
            }
            
            // Add another safety check after a brief delay
            setTimeout(() => {
                if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                    scrollContainer.scrollTop = 0;
                    // Also update virtualization window again
                    updateVirtualization();
                }
            }, 50);
        }
        // Special treatment for non-virtualized mode with animations 
        else if (!virtualEnabled && animationInProgress) {
            // For animations, queue for later and also execute right away
            pendingScrollToBottom = true;
            
            // Try to scroll right away anyway
            withProgrammaticScroll(() => {
                scrollContainer.scrollTop = 0;
            });
            
            // Also set another safety check after animation should be done
            setTimeout(() => {
                if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                    withProgrammaticScroll(() => {
                        if (debug) logger.trace('logViewer', "Post-animation scroll safety check");
                        scrollContainer.scrollTop = 0;
                    });
                }
            }, 400); // Just after animation should be complete
        }
        // For all other cases
        else {
            // Add another safety check after a brief delay - this is critical for all cases
            setTimeout(() => {
                if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                    withProgrammaticScroll(() => {
                        scrollContainer.scrollTop = 0;
                    });
                }
            }, 50);
        }
    }
    
    // Scroll directly to the bottom
    function scrollToBottom() {
        withProgrammaticScroll(() => {
            if (scrollContainer) {
                // CRITICAL FIX: In column-reverse, scrollTop = 0 is the visual bottom
                // But in some browsers, with certain DOM structures, we need to set it repeatedly
                // to ensure it actually reaches the bottom.
                scrollContainer.scrollTop = 0;
                
                // Use setTimeout to double-check that we're really at bottom
                setTimeout(() => {
                    if (scrollContainer && scrollContainer.scrollTop !== 0 && !isUserScrolling) {
                        if (debug) logger.trace('logViewer', `Direct force to bottom in scrollToBottom (was ${scrollContainer.scrollTop})`);
                        
                        // Try with scrollTo API for more consistent behavior
                        try {
                            scrollContainer.scrollTo({
                                top: 0,
                                behavior: 'auto' // Use 'auto' not 'smooth' to ensure immediate effect
                            });
                        } catch (e) {
                            // Fallback if scrollTo API fails
                            scrollContainer.scrollTop = 0;
                        }
                    }
                }, 10);
                
                // Set a final check with a longer timeout, as sometimes scrolling takes time
                setTimeout(() => {
                    if (scrollContainer && scrollContainer.scrollTop !== 0 && !isUserScrolling) {
                        if (debug) logger.trace('logViewer', `Final force to bottom in scrollToBottom (was still ${scrollContainer.scrollTop})`);
                        
                        // Try one more time
                        scrollContainer.scrollTop = 0;
                    }
                }, 100);
            }
        });
    }
    
    // Schedule scroll checks after processing completes
    function schedulePostProcessingScrolls() {
        // Cancel any existing timers
        cancelPostProcessingChecks();
        
        // CRITICAL FIX: Use staggered timing to catch all rendering phases
        // Include more checks and longer times to ensure we catch all updates
        const checkTimes = [100, 300, 600, 1000, 1500, 2000, 2500];
        
        checkTimes.forEach((delay, index) => {
            const timerId = window.setTimeout(() => {
                // Only auto-scroll if it's enabled and user isn't manually scrolling
                // This prevents forcing auto-scroll when it was disabled by user action
                if (autoScroll && !isUserScrolling && !manualScrollLock) {
                    if (debug) logger.trace('logViewer', `Post-processing scroll check #${index + 1} at t+${delay}ms`);
                    
                    // Check if we're not already at the bottom
                    if (scrollContainer && scrollContainer.scrollTop !== 0) {
                        if (debug) logger.trace('logViewer', `Fixing scroll position in post-processing: ${scrollContainer.scrollTop} -> 0`);
                    }
                    
                    // Use most direct method - force scrolling on last few checks
                    if (index >= checkTimes.length - 3) {
                        // For the last few checks, use direct DOM scrolling for maximum reliability
                        withProgrammaticScroll(() => {
                            if (scrollContainer) {
                                scrollContainer.scrollTop = 0;
                                
                                // Force an additional check right after
                                setTimeout(() => {
                                    if (scrollContainer && scrollContainer.scrollTop !== 0 && autoScroll && !isUserScrolling) {
                                        if (debug) logger.trace('logViewer', "Double-checking post-processing scroll");
                                        scrollContainer.scrollTop = 0;
                                    }
                                }, 10);
                            }
                        });
                    } else {
                        // For earlier checks, use the regular method
                        executeScrollToBottom(index === checkTimes.length - 1);
                    }
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
        if (scheduleForceScrollTimer) {
            clearTimeout(scheduleForceScrollTimer);
        }
        
        // Set a new timer for force scroll
        scheduleForceScrollTimer = window.setTimeout(() => {
            // Use the reactive autoScroll boolean here
            if (autoScroll) {
                // Force scroll regardless of other state
                if (debug) logger.trace('logViewer', "Executing force scroll after high volume");
                forceScrollToBottom();
            }
            scheduleForceScrollTimer = null;
        }, delay);
    }
    
    // Execute scroll to bottom with programmatic flag - updated for column-reverse
    function executeScrollToBottom(force: boolean = false): void {
        // CRITICAL: Never scroll if auto-scroll is off or user is actively scrolling
        if (!scrollContainer || !autoScroll || isUserScrolling || manualScrollLock) {
            if (debug) logger.warn('logViewer', `Execute scroll BLOCKED: autoScroll=${autoScroll}, userScrolling=${isUserScrolling}, manualLock=${manualScrollLock}`);
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
    
    // --- Transition Handling ---
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
            if (pendingScrollToBottom && autoScroll) {
                pendingScrollToBottom = false;
                executeScrollToBottom();
            }
        }
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
        
        if (!scrollContainer || !filteredLogs.length) {
            showReturnToBottomButton = false;
            return;
        }
        
        // In column-reverse layout, bottom is at scrollTop = 0
        // Use a small tolerance (1px) to account for rounding errors
        const notAtBottom = scrollContainer.scrollTop > 1;
        
        showReturnToBottomButton = !autoScroll && notAtBottom;
        
        if (debug && showReturnToBottomButton) {
            logger.trace('logViewer', `Return to bottom button visible (scrollTop=${scrollContainer.scrollTop})`);
        }
    }
    
    // Handler for the return to bottom button
    function handleReturnToBottom() {
        scrollToBottom();
        // Optionally re-enable auto-scroll
        // setAutoScroll(true, 'returnToBottomClick');
        showAutoScrollToastMessage("Returned to bottom");
    }
    
    // --- Scroll Event Handling ---
    function handleScroll(): void {
        // Always ignore programmatic scrolling
        if (isProgrammaticScroll) {
            if (debug) logger.trace('logViewer', "Ignoring programmatic scroll event");
            return;
        }
        
        // Log scroll event for debugging
        if (debug) logger.trace('logViewer', `⚡ SCROLL EVENT - scrollTop=${scrollContainer.scrollTop}`);
        
        // SIMPLIFIED VIRTUALIZATION UPDATE
        // For virtualization, update the visible window immediately
        if (virtualEnabled && virtualizationReady) {
            // Update the visible window based on current scroll position
            updateVirtualization();
        }
        
        // Mark as user scrolling - SET THIS FLAG IMMEDIATELY
        isUserScrolling = true;
        
        // IMPORTANT: Set manual scroll lock to prevent auto-scroll from fighting with user
        manualScrollLock = true;
        
        // Cancel any existing timers/animations
        if (scrollRAF) cancelAnimationFrame(scrollRAF);
        if (manualScrollLockTimer) {
            clearTimeout(manualScrollLockTimer);
        }
        
        // Reset the manual scroll lock after a LONG period (3 seconds)
        // This gives user plenty of time to read without auto-scroll interfering
        manualScrollLockTimer = window.setTimeout(() => {
            if (debug) logger.trace('logViewer', "Manual scroll lock timeout expired");
            manualScrollLock = false;
            manualScrollLockTimer = null;
        }, 3000);
        
        // Schedule another virtualization update in the next frame
        // This gives the best responsiveness for virtual view
        if (virtualEnabled && virtualizationReady) {
            requestAnimationFrame(() => {
                // Update the virtualization window again
                updateVirtualization();
                
                // And schedule one more update for good measure
                setTimeout(() => {
                    if (virtualEnabled && virtualizationReady) {
                        updateVirtualization();
                    }
                }, 16); // ~1 frame at 60fps
            });
        }
        
        // Use RAFrame for precise timing of auto-scroll and metrics updates
        scrollRAF = requestAnimationFrame(() => {
            if (!scrollContainer) {
                scrollRAF = null;
                return;
            }
            
            // Get scrollTop directly
            const scrollTop = scrollContainer.scrollTop;
            
            if (debug) logger.trace('logViewer', `Handling scroll: scrollTop=${scrollTop}px, height=${scrollContainer.scrollHeight}px, client=${scrollContainer.clientHeight}px`);
            
            // IMPORTANT: If user has scrolled away from bottom and auto-scroll is on,
            // immediately disable auto-scroll before doing anything else
            if (scrollTop > 1 && autoScroll) {
                if (debug) logger.warn('logViewer', `CRITICAL: Disabling auto-scroll due to scrollTop=${scrollTop}px`);
                // Force auto-scroll off
                setAutoScroll(false, 'userScrollAway');
                // Save position for restoration
                saveViewportAnchor();
                
                // Show toast notification
                showAutoScrollToastMessage("Auto-scroll disabled - scroll to bottom to re-enable");
            }
            
            // Update scroll metrics
            updateScrollMetrics(scrollTop);
            
            // Update return to bottom button visibility
            updateReturnToBottomButtonVisibility();
            
            // Set a timeout to mark user scrolling complete - MUCH LONGER TIMEOUT
            if (userScrollTimeout) clearTimeout(userScrollTimeout);
            userScrollTimeout = window.setTimeout(() => {
                // Only clear the flag if we're not in a locked state
                // This prevents clearing too early when user is actively reading
                if (!manualScrollLock) {
                    if (debug) logger.trace('logViewer', "User scrolling flag cleared after timeout");
                    isUserScrolling = false;
                    
                    // Do one final virtualization update when scrolling ends
                    if (virtualizationReady && virtualEnabled) {
                        updateVirtualization();
                    }
                } else {
                    if (debug) logger.trace('logViewer', "Keeping user scrolling flag due to active manual lock");
                }
                
                // Get final scroll position
                const finalScrollTop = scrollContainer?.scrollTop || 0;
                
                // Check if we're exactly at the bottom (scrollTop = 0 in column-reverse)
                if (scrollContainer && finalScrollTop === 0) {
                    if (debug) logger.trace('logViewer', "User at EXACT bottom position after scrolling");
                    
                    // ONLY re-enable auto-scroll if:
                    // 1. Auto-scroll is currently off
                    // 2. User is scrolling toward the bottom
                    // 3. Manual scroll lock is not active (user isn't actively reading)
                    if (!autoScroll && scrollDirectionToBottom && !manualScrollLock) {
                        if (debug) logger.warn('logViewer', "Re-enabling auto-scroll - user at bottom and not locked");
                        
                        // Use setAutoScroll to ensure UI sync
                        setAutoScroll(true, 'scrolledToBottom');
                    } else if (!autoScroll && manualScrollLock) {
                        if (debug) logger.trace('logViewer', "Not re-enabling auto-scroll despite being at bottom - manual lock active");
                    }
                } else {
                    if (debug) logger.trace('logViewer', `Not at bottom after scroll: ${finalScrollTop}px`);
                    
                    // SAFETY CHECK: if not at bottom but auto-scroll is on, fix it
                    if (autoScroll) {
                        if (debug) logger.error('logViewer', "INCONSISTENCY: Not at bottom but auto-scroll is on - fixing");
                        
                        // Use setAutoScroll to ensure UI sync
                        setAutoScroll(false, 'inconsistencyFix');
                    }
                }
            }, 800); // MUCH longer timeout (800ms instead of 200ms)
            
            scrollRAF = null;
        });
    }
    
    // Update scroll metrics with velocity tracking
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
        
        // Store current values for next calculation
        lastScrollTop = currentScrollTop;
        lastScrollTime = now;
    }
    
    // --- Virtualization Functions ---
    
    // Reset virtualization settings
    function resetVirtualization(): void {
        // Reset view range
        virtualStart = 0;
        virtualEnd = Math.min(100, filteredLogs.length - 1); // Start with a reasonable number of logs
        viewportAnchor = null;
        
        if (debug) logger.trace('logViewer', "Resetting virtualization");
        
        // Use setTimeout with tick() to ensure DOM is updated
        setTimeout(async () => {
            await tick(); // Ensure DOM update first
            
            // Re-measure everything
            recalculatePositions();
            
            if (virtualEnabled) {
                if (virtualizationReady) {
                    // Real virtualization update
                    updateVirtualization();
                    
                    // Log the state if in debug mode
                    if (debug) logger.trace('logViewer', `Virtualization reset complete: ${virtualStart}-${virtualEnd}`);
                } else {
                    // Mark virtualization as ready after this initial setup
                    virtualizationReady = true;
                    updateVirtualization();
                    
                    if (debug) logger.trace('logViewer', "Virtualization marked as ready during reset");
                }
            }
            
            // If auto-scroll is enabled, ensure we're at the bottom
            if (autoScroll && scrollContainer) {
                if (debug) logger.trace('logViewer', "Scrolling to bottom after virtualization reset");
                scrollToBottomWithStrategy();
            }
        }, 100); // Slightly longer timeout to ensure complete DOM updates
    }
    
    // Toggle virtualization manually (dev mode)
    function toggleVirtualization(): void {
        manualVirtualToggle = true;
        virtualEnabled = !virtualEnabled;
        
        if (debug) logger.trace('logViewer', `Toggling virtualization: ${virtualEnabled ? 'ON' : 'OFF'}`);
        
        // Reset all virtualization state completely
        virtualStart = 0;
        virtualEnd = virtualEnabled ? Math.min(100, filteredLogs.length - 1) : 0;
        
        // When toggling OFF, ensure we clean up any absolute positioning
        if (!virtualEnabled) {
            if (debug) logger.trace('logViewer', "Disabling virtualization - cleaning up positioning");
            
            // Force immediate re-render with clean slate
            setTimeout(async () => {
                await tick();
                recalculatePositions();
                
                // If auto-scroll was on, ensure we're at the bottom
                if (autoScroll && scrollContainer) {
                    scrollContainer.scrollTop = 0;
                }
            }, 10);
        } else {
            // When enabling, do a complete initialization
            if (debug) logger.trace('logViewer', "Enabling virtualization - initializing view");
            virtualizationReady = true; // Mark ready immediately
            resetVirtualization();
        }
    }
    
    // Calculate positions for all logs based on individual heights
    function recalculatePositions(): void {
        // CRITICAL FIX: Simplify position calculation logic for more reliability

        // Safety check for empty logs
        if (filteredLogs.length === 0) {
            virtualContainerHeight = 0;
            totalLogHeight = 0;
            return;
        }

        // JavaScript implementation (fallback)
        // Calculate statistics only
        let heightsTotal = 0;
        let heightCount = 0;
        let minHeight = Infinity;
        let maxHeight = 0;

        // Scan measured logs to get average height
        for (const log of filteredLogs) {
            const sequence = log._sequence || 0;
            if (sequence === null || sequence === undefined) continue;

            const height = logHeights.get(sequence);
            if (height) {
                // Only include actual measured heights in statistics
                const cappedHeight = Math.min(height, 100); // Cap at 100px
                heightsTotal += cappedHeight;
                heightCount++;
                minHeight = Math.min(minHeight, cappedHeight);
                maxHeight = Math.max(maxHeight, cappedHeight);
            }
        }

        // Calculate new average height if we have enough samples
        if (heightCount > 10) {
            // Use a safe average that prevents unreasonable values
            const newAvg = heightsTotal / heightCount;

            // Only update if reasonable (not too small, not too large)
            if (newAvg >= 20 && newAvg <= 100) {
                avgLogHeight = newAvg;
            } else if (debug) {
                logger.warn('logViewer', `Skipping unreasonable average height update: ${newAvg}px`);
            }
        }

        // Simplified approach: Use average height × count for total height
        // This is much more reliable than summing individual heights
        const safeAvgHeight = Math.max(20, avgLogHeight);
        totalLogHeight = filteredLogs.length * safeAvgHeight;

        // Set virtual container height
        virtualContainerHeight = totalLogHeight;

        // Log statistics in debug mode, but only occasionally
        if (debug && heightCount > 0 && filteredLogs.length % 100 === 0) {
            logger.trace('logViewer', `Height stats: logs=${filteredLogs.length}, samples=${heightCount}, avg=${safeAvgHeight.toFixed(1)}px, min=${minHeight}px, max=${maxHeight}px`);
        }
    }
    
    // Find which log corresponds to a scroll position using binary search
    function findLogAtScrollPosition(scrollTop: number): number {
        // Early short-circuit for empty logs
        if (filteredLogs.length === 0) return 0;

        // In column-reverse, we need to adjust the scrollTop value
        // Convert from scrollTop to a position from the top of content
        const adjustedScrollPosition = scrollContainer ?
            (totalLogHeight - scrollContainer.clientHeight - scrollTop) :
            scrollTop;

        // JavaScript implementation (fallback)
        // Binary search for the log
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
    
    // Update virtualization calculations for column-reverse layout
    function updateVirtualization(): void {
        if (!scrollContainer || !virtualEnabled || !virtualizationReady) {
            if (debug) logger.trace('logViewer', "Skipping virtualization update - not ready or enabled");
            return; 
        }
        
        const { scrollTop, clientHeight, scrollHeight } = scrollContainer;
        
        // CRITICAL DEBUGGING - Always log scroll position in debug mode
        if (debug) logger.trace('logViewer', `⚠️ VIRTUALIZATION UPDATE - scrollTop=${scrollTop}, clientHeight=${clientHeight}, scrollHeight=${scrollHeight}`);
        
        viewportHeight = clientHeight;
        
        // Special case for empty logs
        if (filteredLogs.length === 0) {
            virtualStart = 0;
            virtualEnd = 0;
            visibleLogCount = 0;
            return;
        }
        
        // Store old values for comparison
        const oldStart = virtualStart;
        const oldEnd = virtualEnd;
        const totalLogs = filteredLogs.length;
        
        // COMPLETELY REDESIGNED APPROACH: Use a much simpler and more reliable algorithm
        // that correctly handles column-reverse layout with both positive and negative scrollTop values
        
        // Special handling for the auto-scroll case (show newest logs)
        if (autoScroll && isScrolledToBottom()) {
            virtualEnd = totalLogs - 1;
            virtualStart = Math.max(0, virtualEnd - 60);
        } else {
            // NOT auto-scrolling - calculate window based on scroll position
            
            // CRITICAL: In column-reverse layout, handle both signs of scrollTop consistently
            // scrollTop=0 means we're at the bottom (newest logs)
            // negative scrollTop means we've scrolled up from the bottom
            
            // First normalize scrollTop to always be non-negative for our calculations
            const normalizedScrollTop = Math.abs(scrollTop);
            
            // Calculate our position as a fraction of the scrollable area
            // For the scrollHeight calculation, subtract client height to get actual scrollable area
            const scrollableHeight = Math.max(1, scrollHeight - clientHeight);
            
            // Calculate scroll ratio (0 = bottom/newest, 1 = top/oldest)
            const scrollRatio = Math.min(1, Math.max(0, normalizedScrollTop / scrollableHeight));
            
            // Use the scroll ratio to determine which logs to show
            // We invert because in our data structure, oldest logs are at index 0
            // But in the UI with column-reverse, oldest logs are at the top (scroll=1)
            const targetIndex = Math.floor((1 - scrollRatio) * totalLogs);
            
            // Create a stable window size based on viewport
            // Use viewport height to calculate approximately how many logs would fit
            const visibleLines = Math.max(20, Math.ceil(clientHeight / Math.max(20, avgLogHeight)));
            
            // Add substantial buffer for smooth scrolling (3x visible area)
            const bufferSize = Math.ceil(visibleLines * 1.5);
            
            // Calculate stable window with buffer (avoid frequent size changes)
            const windowSize = Math.max(60, bufferSize * 2); // At least 60 logs total
            const halfWindow = Math.floor(windowSize / 2);
            
            // Create window centered on target with sufficient buffer
            virtualStart = Math.max(0, targetIndex - halfWindow);
            virtualEnd = Math.min(totalLogs - 1, targetIndex + halfWindow);
            
            // Ensure window size is consistent by extending one end if we hit a boundary
            if (virtualStart === 0 && totalLogs > windowSize) {
                // We hit the top boundary (oldest logs) - extend window downward
                virtualEnd = Math.min(totalLogs - 1, windowSize - 1);
            } else if (virtualEnd === totalLogs - 1 && totalLogs > windowSize) {
                // We hit the bottom boundary (newest logs) - extend window upward
                virtualStart = Math.max(0, totalLogs - windowSize);
            }
            
            // Debug logging
            if (debug) {
                logger.trace('logViewer', `Scroll ratio: ${scrollRatio.toFixed(4)}, target idx: ${targetIndex}, window: ${virtualStart}-${virtualEnd}`);
            }
        }
        
        // Ensure our window doesn't exceed log count (safety check)
        virtualStart = Math.max(0, Math.min(virtualStart, totalLogs - 1));
        virtualEnd = Math.max(virtualStart, Math.min(virtualEnd, totalLogs - 1));
        
        // Update visible log count
        visibleLogCount = virtualEnd - virtualStart + 1;
        
        // Log changes for debugging
        if (oldStart !== virtualStart || oldEnd !== virtualEnd) {
            if (debug) logger.trace('logViewer', `📊 Window updated: ${oldStart}-${oldEnd} → ${virtualStart}-${virtualEnd}`);
        } else if (scrollTop !== 0 && scrollTop !== lastScrollTop && debug) {
            logger.warn('logViewer', `⚠️ Scroll changed (${lastScrollTop} → ${scrollTop}) but window unchanged: ${virtualStart}-${virtualEnd}`);
        }
        
        // Update last scroll position for comparison
        lastScrollTop = scrollTop;
        
        // Force a reactive update by reassigning variables
        virtualStart = Math.floor(virtualStart);
        virtualEnd = Math.floor(virtualEnd);
    }
    
    // Save current viewport position as an anchor - updated for column-reverse
    function saveViewportAnchor(): void {
        if (!scrollContainer) return;
        
        // CRITICAL FIX: Only save anchor if auto-scroll is OFF
        if (autoScroll) return;
        
        // If already at bottom and auto-scroll is enabled, don't create an anchor
        if (isScrolledToBottom() && autoScroll) {
            viewportAnchor = null;
            return;
        }
        
        const { scrollTop, clientHeight } = scrollContainer;
        
        // In column-reverse, convert scrollTop to position from top
        const scrollFromTop = totalLogHeight - clientHeight - scrollTop;
        
        // Find which log is at the top of the viewport with adjusted position
        const logIndex = findLogAtScrollPosition(scrollFromTop);
        if (logIndex < 0 || logIndex >= filteredLogs.length) return;
        
        const log = filteredLogs[logIndex];
        const sequence = log._sequence || 0;
        const logTop = logPositions.get(sequence) || 0;
        
        // Save anchor with offset from log top
        viewportAnchor = {
            type: virtualEnabled ? 'virtual' : 'nonVirtual',
            index: logIndex,
            sequence: sequence,
            offsetTop: scrollFromTop - logTop
        };
        
        if (debug) logger.trace('logViewer', "Saved viewport anchor", { viewportAnchor });
    }
    
    // Restore scroll position based on saved anchor - updated for column-reverse
    function restoreViewportAnchor(): void {
        // Early returns - critical checks
        if (!viewportAnchor || !scrollContainer) return;
        
        // CRITICAL FIX: Only restore anchor if auto-scroll is OFF
        if (autoScroll) return;
        
        // Skip during active user scrolling or when manual lock is active
        if (isUserScrolling || manualScrollLock) return;
        
        try {
            // Find the log position now
            const sequence = viewportAnchor.sequence;
            
            // Check if the log still exists in the filtered set
            let logIndex = -1;
            for (let i = 0; i < filteredLogs.length; i++) {
                if (filteredLogs[i]._sequence === sequence) {
                    logIndex = i;
                    break;
                }
            }
            
            if (logIndex === -1) {
                // Anchored log not found, use fallback
                if (debug) logger.warn('logViewer', `Anchor log (sequence=${sequence}) not found in current filtered set`);
                
                // Try to use the original index if in bounds
                if (viewportAnchor.index >= 0 && viewportAnchor.index < filteredLogs.length) {
                    logIndex = viewportAnchor.index;
                } else {
                    // Use percentage-based positioning for fallback
                    const percentageIndex = Math.min(
                        filteredLogs.length - 1,
                        Math.floor((viewportAnchor.index / filteredLogs.length) * filteredLogs.length)
                    );
                    logIndex = Math.max(0, percentageIndex);
                }
            }
            
            if (logIndex >= 0) {
                const currentLog = filteredLogs[logIndex];
                const currentSequence = currentLog._sequence || 0;
                const logTop = logPositions.get(currentSequence) || 0;
                
                // Calculate adjusted position with the same offset
                const positionFromTop = logTop + viewportAnchor.offsetTop;
                
                // In column-reverse, convert back to scrollTop
                const scrollTopValue = totalLogHeight - scrollContainer.clientHeight - positionFromTop;
                
                // Restore scroll position with the calculated scrollTop
                withProgrammaticScroll(() => {
                    scrollContainer.scrollTop = Math.max(0, scrollTopValue);
                });
                
                // Update virtualization if needed
                if (virtualEnabled && virtualizationReady) {
                    updateVirtualization();
                }
            }
        } catch (error) {
            logger.error('logViewer', "Error during restoreViewportAnchor", { error });
        }
    }
    
    // Measure individual log height using full bounding rect
    function measureLogEntry(node: HTMLElement, log: LogMessage) {
        const sequence = log._sequence || 0;
        
        // IMPORTANT: Skip for logs without a valid sequence
        if (sequence === undefined || sequence === null) {
            if (debug) logger.warn('logViewer', "Skipping measurement for log with invalid sequence");
            return {};
        }
        
        // Only create one observer per unique sequence
        if (node.hasAttribute('data-observed')) {
            return {};
        }
        
        // Mark node as being observed
        node.setAttribute('data-observed', 'true');
        
        // Create ResizeObserver to measure the actual height
        const resizeObserver = new ResizeObserver(entries => {
            // Use getBoundingClientRect for complete height including padding/borders
            const rect = node.getBoundingClientRect();
            
            // CRITICAL FIX: Use a reasonable maximum height (100px) to prevent excessive growth
            // This prevents the ever-growing height issue
            const measuredHeight = Math.ceil(rect.height);
            const height = Math.min(Math.max(measuredHeight, 20), 100) + POSITION_BUFFER;
            
            // Only update if height changed significantly (>1px) and is reasonable
            const currentHeight = logHeights.get(sequence) || 0;
            if (Math.abs(currentHeight - height) > 1) {
                if (debug && currentHeight > 0 && height > currentHeight * 1.5) {
                    logger.warn('logViewer', `Height increase detected for log ${sequence}: ${currentHeight}px -> ${height}px`);
                }
                
                // Update height map
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
                if (node.hasAttribute('data-observed')) {
                    node.removeAttribute('data-observed');
                }
            }
        };
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
    
    // For testing - toggle debug overlay
    function toggleDebugOverlay() {
        debugAutoScroll = !debugAutoScroll;
        if (debug) logger.trace('logViewer', `Debug overlay ${debugAutoScroll ? 'enabled' : 'disabled'}`);
    }

    onMount(async () => {
        if (debug) logger.trace('logViewer', "LogViewer component mounting");
        
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
            }
        }, 50); // Decay every 50ms
        
        // Set up key press listener for toggling debug (press 'd' key)
        document.addEventListener('keydown', (e) => {
            if (e.key === 'd' && e.ctrlKey) {
                toggleDebugOverlay();
            }
        });
        
        // IMPORTANT: Set up a log position debug overlay in debug mode
        if (debug) {
            logger.debug('logViewer', "Debug mode active - enabling verbose logging");
        }
        
        // CRITICAL: Set up scroll monitor interval to guarantee scroll events are always caught
        // This helps when normal scroll events might be missed due to browser performance issues
        scrollMonitorInterval = window.setInterval(() => {
            if (!scrollContainer) return;
            
            // Store current scrollTop
            const currentScrollTop = scrollContainer.scrollTop;
            
            // Check if scrollTop has changed since last check
            if (lastKnownScrollTop !== currentScrollTop) {
                if (debug) logger.trace('logViewer', `Scroll monitor detected change: ${lastKnownScrollTop} -> ${currentScrollTop}`);
                
                // Update last known value
                lastKnownScrollTop = currentScrollTop;
                
                // If virtualization is enabled, update it directly
                if (virtualEnabled && virtualizationReady) {
                    updateVirtualization();
                    
                    // Schedule another update a little later for smoother updates
                    setTimeout(() => {
                        if (virtualEnabled && virtualizationReady) {
                            updateVirtualization();
                        }
                    }, 16);
                }
            } else {
                // Even if scrollTop hasn't changed, still periodically update virtualization
                // This is critical for post-processing scrolling to work correctly
                if (virtualEnabled && virtualizationReady && !autoScroll) {
                    // This ensures virtualization updates even if scroll events aren't detected
                    // Only do this every 5th interval (500ms) to avoid too much updating
                    const now = Date.now();
                    if (now % 500 < 100) { // Roughly every 500ms
                        updateVirtualization();
                    }
                }
            }
        }, 100); // Check every 100ms
        
        // EXTREME MEASURE: Add global wheel and touchmove event listeners
        // This is necessary because some browsers might not properly trigger scroll events in certain conditions
        logger.trace('logViewer', "Adding global wheel and touchmove event listeners for scroll detection");
        
        // SIMPLIFIED DOCUMENT WHEEL HANDLER
        // We don't need this anymore - the scroll event is sufficient
        // Let the native scroll events do their work instead
        
        // We don't need touchmove special handling either
        // The regular scroll event will capture this too
        
        // Also add keydown for arrow keys
        document.addEventListener('keydown', (e) => {
            // Only process arrow keys
            if (e.key === 'ArrowUp' || e.key === 'ArrowDown' || e.key === 'PageUp' || e.key === 'PageDown' || e.key === 'Home' || e.key === 'End') {
                // Only process if we have a scroll container and virtualization is enabled
                if (scrollContainer && virtualEnabled && virtualizationReady) {
                    logger.trace('logViewer', `KEY EVENT DETECTED (${e.key}) - Forcing virtualization update`);
                    
                    // Force recalculation and update
                    recalculatePositions();
                    updateVirtualization();
                    
                    // Force reactive updates
                    virtualStart = virtualStart;
                    virtualEnd = virtualEnd;
                }
            }
        });
        
        // Add scroll event listener ASAP
        if (scrollContainer) {
            if (debug) logger.trace('logViewer', "Adding scroll event listener to container");
            scrollContainer.addEventListener('scroll', handleScroll, { passive: true });
            // Store initial scrollTop
            lastKnownScrollTop = scrollContainer.scrollTop;
            
            // We don't need special wheel handling - the scroll event is reliable enough
            // Rely on the scroll event to update virtualization
        } else {
            if (debug) logger.warn('logViewer', "No scroll container yet - will retry scroll listener setup");
            // Retry after a short delay if container isn't available yet
            setTimeout(() => {
                if (scrollContainer) {
                    if (debug) logger.trace('logViewer', "Adding scroll event listener (retry)");
                    scrollContainer.addEventListener('scroll', handleScroll, { passive: true });
                    // Store initial scrollTop
                    lastKnownScrollTop = scrollContainer.scrollTop;
                } else {
                    logger.error('logViewer', "CRITICAL: Failed to set up scroll listener - no container");
                }
            }, 100);
        }
        
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
        
        // Set up ResizeObserver to detect size changes for the container and window
        const resizeObserver = new ResizeObserver(() => {
            if (debug) logger.trace('logViewer', "Resize detected");
            
            // Save scroll position
            const wasAtBottom = isScrolledToBottom(0); // Use strict check (exactly at bottom)
            
            if (!wasAtBottom && !autoScroll) {
                saveViewportAnchor();
            }
            
            // Update layout measurements
            recalculatePositions();
            
            // Update virtual window if needed
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
        
        // Observe both the scroll container and document
        if (scrollContainer) {
            resizeObserver.observe(scrollContainer);
            resizeObserver.observe(document.documentElement);
        } else {
            if (debug) logger.warn('logViewer', "No scroll container for ResizeObserver - will observe document only");
            resizeObserver.observe(document.documentElement);
        }
        
        // Set a timeout to enable virtualization after initial rendering
        setTimeout(async () => {
            if (debug) logger.trace('logViewer', "Enabling virtualization after initial render");
            
            // By this point, some logs should have been measured
            await tick();
            
            // Start the virtualization system
            initialMeasurementsComplete = true;
            virtualizationReady = true;
            
            // Recalculate positions and update virtualization
            recalculatePositions();
            
            if (virtualEnabled) {
                if (debug) logger.trace('logViewer', "Initializing virtual display");
                updateVirtualization();
            }
            
            // Double-check auto-scroll state - but be careful to check for user interaction
            if (autoScroll && scrollContainer && !isUserScrolling && !manualScrollLock) {
                // Only do this initial setup scroll if we're supposed to auto-scroll
                trackScrollTrigger('onMount:setupComplete');
                scrollContainer.scrollTop = 0;
            }
            
            // One final update after a bit more delay to catch any missed positions
            setTimeout(() => {
                if (virtualEnabled && virtualizationReady) {
                    recalculatePositions();
                    updateVirtualization();
                    
                    if (debug) logger.trace('logViewer', `Final virtualization setup: displaying logs ${virtualStart}-${virtualEnd}`);
                }
            }, 300);
        }, 200);
        
        // Define wheel and touch event handlers for later removal
        const wheelHandler = (e) => {
            if (scrollContainer && virtualEnabled && virtualizationReady) {
                logger.trace('logViewer', "WHEEL EVENT DETECTED - Forcing virtualization update");
                recalculatePositions();
                updateVirtualization();
                virtualStart = virtualStart;
                virtualEnd = virtualEnd;
            }
        };
        
        const touchHandler = (e) => {
            if (scrollContainer && virtualEnabled && virtualizationReady) {
                logger.trace('logViewer', "TOUCH MOVE EVENT DETECTED - Forcing virtualization update");
                recalculatePositions();
                updateVirtualization();
                virtualStart = virtualStart;
                virtualEnd = virtualEnd;
            }
        };
        
        const keyHandler = (e) => {
            if (e.key === 'ArrowUp' || e.key === 'ArrowDown' || e.key === 'PageUp' || e.key === 'PageDown' || e.key === 'Home' || e.key === 'End') {
                if (scrollContainer && virtualEnabled && virtualizationReady) {
                    logger.trace('logViewer', `KEY EVENT DETECTED (${e.key}) - Forcing virtualization update`);
                    recalculatePositions();
                    updateVirtualization();
                    virtualStart = virtualStart;
                    virtualEnd = virtualEnd;
                }
            } else if (e.key === 'd' && e.ctrlKey) {
                toggleDebugOverlay();
            }
        };
        
        // Register our handlers
        document.addEventListener('wheel', wheelHandler, { passive: true });
        document.addEventListener('touchmove', touchHandler, { passive: true });
        document.addEventListener('keydown', keyHandler);

        // Register dev dashboard event handlers
        document.addEventListener('dev:toggle-virtualization', handleToggleVirtualization);
        document.addEventListener('dev:toggle-debug-scroll', handleToggleDebugScroll);
        document.addEventListener('dev:force-scroll-bottom', handleForceScrollBottom);
        
        // onMount cleanup function - remove ALL event listeners
        return () => {
            if (debug) logger.trace('logViewer', "LogViewer component unmounting - cleaning up resources");
            
            // Clear all timers and listeners
            if (velocityDecayTimer) clearInterval(velocityDecayTimer);
            if (scrollMonitorInterval) clearInterval(scrollMonitorInterval);
            
            // Remove all scroll-related event listeners
            if (scrollContainer) {
                scrollContainer.removeEventListener('scroll', handleScroll);
            }

            // Remove dev dashboard event handlers
            document.removeEventListener('dev:toggle-virtualization', handleToggleVirtualization);
            document.removeEventListener('dev:toggle-debug-scroll', handleToggleDebugScroll);
            document.removeEventListener('dev:force-scroll-bottom', handleForceScrollBottom);

            // Remove other event listeners
            if (scrollContainer) {
                scrollContainer.removeEventListener('wheel', wheelHandler);
            }
            
            // Remove document event listeners
            document.removeEventListener('wheel', wheelHandler);
            document.removeEventListener('touchmove', touchHandler);
            document.removeEventListener('keydown', keyHandler);
            
            // Clear all timers
            if (userScrollTimeout) clearTimeout(userScrollTimeout);
            if (scrollRAF) cancelAnimationFrame(scrollRAF);
            if (batchMeasurementTimer) clearTimeout(batchMeasurementTimer);
            if (manualScrollLockTimer) clearTimeout(manualScrollLockTimer);
            if (autoScrollToastTimer) clearTimeout(autoScrollToastTimer);
            if (scheduleForceScrollTimer) clearTimeout(scheduleForceScrollTimer);
            cancelPostProcessingChecks();
            
            // Disconnect observers
            resizeObserver.disconnect();
            
            logger.trace('logViewer', "All LogViewer resources cleaned up");
        };
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

            <!-- Auto-scroll toggle -->
            <div class="flex items-center gap-1 px-3 py-1 bg-[#333] h-7 rounded hover:bg-primary/10 hover:border-primary/55 hover:shadow-input transition-all duration-200">
                <input
                    id="auto-scroll-checkbox"
                    type="checkbox"
                    checked={autoScroll}
                    on:change={(e) => {
                        const target = e.target as HTMLInputElement;
                        setAutoScroll(target.checked, 'userPreference');
                    }}
                    class="w-3.5 h-3.5 accent-primary m-0 cursor-pointer"
                    aria-label="Toggle auto-scroll"
                />
                <label 
                    for="auto-scroll-checkbox"
                    class="cursor-pointer text-text text-[11px] uppercase tracking-wider whitespace-nowrap flex-shrink-0 hover:text-white transition-colors duration-200"
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
                <!-- Debug info -->
            {#if version === 'dev'}
                <span class="text-xs text-primary/50" aria-live="polite">
                    {filteredLogs.length} logs {virtualEnabled ? '| ' + visibleLogCount + ' visible' : ''} {isProcessing ? '| PROCESSING' : ''}
                </span>
            {/if}
        </div>
    </div>
    
    <!-- Content area -->
    <div class="relative flex flex-col flex-1 min-h-0">
        <!-- The scrollable container for log entries with terminal-mode (column-reverse) -->
        <div 
            class="flex-1 overflow-y-auto overflow-x-hidden min-h-0 log-scroll-container terminal-mode"
            class:autoscroll-active={autoScroll}
            bind:this={scrollContainer}
            role="region"
            aria-label="Log entries"
            style="overscroll-behavior: contain; will-change: scroll-position;"
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
                    data-virtual-container="true"
                >
                    <!-- Initial loading state before virtualization is ready -->
                    {#if virtualEnabled && !virtualizationReady}
                        <!-- Show the first 50 logs in non-virtualized mode until virtualization is ready -->
                        {#each filteredLogs.slice(0, 50 + 1) as log, index (log._sequence + '-' + index)}
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
                        <!-- SIMPLIFIED VIRTUALIZATION: More stable with reliable spacers -->
                        <div>
                            <!-- Top spacer to maintain scroll position for logs above virtual window -->
                            <div 
                                class="w-full top-spacer"
                                style="height: {virtualStart * Math.max(20, avgLogHeight)}px;"
                                data-virtual-top-spacer="true"
                            ></div>
                            
                            <!-- Status indicator for debugging -->
                            {#if version === 'dev'}
                                <div class="sticky top-0 z-50 bg-black/80 text-white text-xs p-1 border-b border-primary/30">
                                    Showing logs {virtualStart} → {virtualEnd} of {filteredLogs.length}
                                    <span class="text-primary">| Scroll: {scrollContainer?.scrollTop || 0}</span>
                                </div>
                            {/if}
                            
                            <!-- Virtual window of logs -->
                            {#each filteredLogs.slice(virtualStart, virtualEnd + 1) as log, index (`${log._sequence || 0}-${virtualStart + index}`)}
                                <div 
                                    class="log-entry {log.behavior ? behaviorColors[log.behavior] : 'text-white/90'}
                                    py-1.5 px-3 border-b border-primary/10 
                                    flex items-start justify-start text-left w-full hover:bg-white/5 transition-colors duration-200"
                                    data-log-sequence={log._sequence}
                                    data-unix-time={log._unix_time ?? 0}
                                    data-virtual-index={virtualStart + index}
                                    use:measureLogEntry={log}
                                    role="listitem"
                                    aria-label={`${log.level} log: ${log.message || ''}`}
                                >
                                    <!-- Debug Index in dev mode -->
                                    {#if version === 'dev'}
                                        <span class="text-primary-400 mr-1 text-xs font-mono opacity-40" title="Virtual index">
                                            {virtualStart + index}
                                        </span>
                                    {/if}
                                    
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
                            
                            <!-- Bottom spacer to maintain scroll position for logs below virtual window -->
                            <div 
                                class="w-full bottom-spacer"
                                style="height: {(filteredLogs.length - virtualEnd - 1) * Math.max(20, avgLogHeight)}px;"
                                data-virtual-bottom-spacer="true"
                            ></div>
                        </div>
                    {:else}
                        <!-- Non-virtualized rendering (all logs) with animations -->
                        {#each filteredLogs as log, index (log._sequence + '-' + index)}
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

    <!-- Return to bottom button -->
    {#if showReturnToBottomButton}
        <div 
            class="fixed bottom-6 right-6 z-50 transform transition-all duration-200"
            transition:fade={{ duration: 200 }}
        >
            <button
                class="w-12 h-12 flex items-center justify-center rounded-full 
                      bg-primary text-white shadow-lg hover:shadow-xl
                      transition-all duration-200 hover:scale-105"
                on:click={handleReturnToBottom}
                aria-label="Return to bottom"
            >
                <span class="material-icons">arrow_downward</span>
            </button>
        </div>
    {/if}

    <!-- Auto-scroll toast notification -->
    {#if showAutoScrollToast}
        <div
            transition:fade={{ duration: 200 }}
            class="fixed bottom-4 left-1/2 transform -translate-x-1/2 bg-black/70 text-white px-3 py-1.5 rounded text-sm z-50"
            role="status"
            aria-live="polite"
        >
            {autoScrollToastMessage}
        </div>
    {/if}

    <!-- Debug Auto-Scroll Overlay -->
    {#if version === 'dev' && debugAutoScroll && scrollContainer}
        <!-- Debug overlay for auto-scroll and virtualization investigation -->
        <div class="fixed bottom-4 left-4 bg-black/80 p-3 text-white text-xs rounded shadow-lg z-50 pointer-events-none border-2 border-red-500 overflow-hidden max-h-[90vh] overflow-y-auto">
            <div class="text-sm font-bold mb-1 text-red-400 flex justify-between items-center">
                <span>LOG VIEWER DEBUGGER</span>
                <span class="text-xs opacity-60">{filteredLogs.length} logs total</span>
            </div>
            
            <div class="flex flex-col space-y-0.5">
                <!-- Auto-scroll section -->
                <div class="text-green-400 font-bold border-b border-gray-600 pb-1">AUTO-SCROLL STATE</div>
                <div>Auto-Scroll: {autoScroll ? 'ON' : 'OFF'}</div>
                <div>UI Checkbox: {document.getElementById('auto-scroll-checkbox')?.checked ? 'ON' : 'OFF'}</div>
                
                <!-- Scroll metrics -->
                <div class="border-t border-gray-600 my-1 pt-1"></div>
                <div>ScrollTop: {Math.abs(scrollContainer?.scrollTop || 0).toFixed(1)}px</div>
                <div>ScrollHeight: {scrollContainer?.scrollHeight || 0}px</div>
                <div>ClientHeight: {scrollContainer?.clientHeight || 0}px</div>
                <div>At Bottom: {Math.abs(scrollContainer?.scrollTop || 0) <= 1 ? 'YES' : 'NO'}</div>
                <div>Direction: {scrollDirectionToBottom ? '⬇️ TO BOTTOM' : '⬆️ TO TOP'}</div>
                <div>Velocity: {scrollVelocity.toFixed(2)}</div>
                <div class="font-bold {isUserScrolling ? 'text-green-400' : 'text-red-400'}">User Scrolling: {isUserScrolling ? 'YES' : 'NO'}</div>
                <div class="font-bold {manualScrollLock ? 'text-green-400' : ''}">Manual Lock: {manualScrollLock ? 'YES' : 'NO'}</div>
                
                <!-- Virtualization metrics -->
                <div class="text-blue-400 font-bold border-t border-b border-gray-600 my-1 py-1">VIRTUALIZATION STATE</div>
                <div>Virtualization: {virtualEnabled ? 'ON' : 'OFF'}</div>
                <div>Virtualization Ready: {virtualizationReady ? 'YES' : 'NO'}</div>
                <div>Range: {virtualStart} → {virtualEnd} ({visibleLogCount} logs)</div>
                <div>Avg Height: {avgLogHeight.toFixed(1)}px</div>
                <div>Container Height: {virtualContainerHeight}px</div>
                <div>Manual Toggle: {manualVirtualToggle ? 'YES' : 'NO'}</div>
                <div>Initial Measurements: {initialMeasurementsComplete ? 'COMPLETE' : 'INCOMPLETE'}</div>
                
                <!-- SCROLL TRIGGER TRACERS -->
                <div class="text-yellow-400 font-bold border-t border-b border-gray-600 my-1 py-1">SCROLL TRIGGERS</div>
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
                
                <!-- Return to bottom button status -->
                <div class="text-purple-400 font-bold border-t border-gray-600 mt-1 pt-1">UI STATE</div>
                <div>Return to Bottom Button: {showReturnToBottomButton ? 'VISIBLE' : 'HIDDEN'}</div>
                <div>Toast Visible: {showAutoScrollToast ? 'YES' : 'NO'}</div>
                <div>Toast Message: {autoScrollToastMessage || 'None'}</div>
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
            hsla(var(--error-soft-hue), var(--error-soft-saturation), var(--error-soft-lightness), 0.08) 0%,
            rgba(0, 0, 0, 0) 70%
        );
        border-left: 2px solid hsla(var(--error-soft-hue), var(--error-soft-saturation), var(--error-soft-lightness), 0.6);
    }
    
    .log-behavior-abort-all {
        background: linear-gradient(
            to right,
            hsla(var(--error-hard-hue), var(--error-hard-saturation), var(--error-hard-lightness), 0.1) 0%,
            rgba(0, 0, 0, 0) 70%
        );
        border-left: 2px solid hsla(var(--error-hard-hue), var(--error-hard-saturation), var(--error-hard-lightness), 0.6);
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
            hsla(var(--error-soft-hue), var(--error-soft-saturation), var(--error-soft-lightness), 0.08) 0%,
            rgba(0, 0, 0, 0) 70%
        );
        border-left: 2px solid hsla(var(--error-soft-hue), var(--error-soft-saturation), var(--error-soft-lightness), 0.4);
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