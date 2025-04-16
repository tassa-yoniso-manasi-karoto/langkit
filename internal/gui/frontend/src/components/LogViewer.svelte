<script lang="ts">
    import { onMount, onDestroy, tick, afterUpdate } from 'svelte';
    import { get } from 'svelte/store';
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
            
            // CRITICAL FIX: Ensure we're at the bottom when processing starts (if auto-scroll is enabled)
            if (isProcessing && !prevIsProcessing && autoScroll && scrollContainer) {
                if (debug) console.log("Processing started - forcing scroll to bottom");
                
                // Force to bottom immediately
                scrollContainer.scrollTop = 0;
                
                // Also schedule some follow-up checks during processing
                const processingChecks = [100, 500, 1000];
                processingChecks.forEach(delay => {
                    setTimeout(() => {
                        if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                            if (scrollContainer.scrollTop !== 0) {
                                if (debug) console.log(`Mid-processing scroll check (${delay}ms): fixing scrollTop=${scrollContainer.scrollTop}`);
                                scrollContainer.scrollTop = 0;
                            }
                        }
                    }, delay);
                });
            }
            
            // When processing ends, schedule final scroll checks
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
                if (debug) console.log(`Force scrollTop=0 for auto-scroll (current=${scrollContainer.scrollTop})`);
                scrollContainer.scrollTop = 0;
            }
        }
        // If auto-scroll is on but we're not at the bottom, this is inconsistent
        else if (autoScroll && !exactlyAtBottom && !isUserScrolling) {
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
                    if (scrollContainer && autoScroll && !isUserScrolling && !manualScrollLock) {
                        scrollContainer.scrollTop = 0; // Direct DOM manipulation is most reliable
                    }
                }
                
                // CRITICAL: For non-virtualized mode + auto-scroll, ensure we're at bottom after animation
                if (!virtualEnabled && autoScroll && !isUserScrolling && !manualScrollLock) {
                    if (debug) console.log("Force to bottom after animation in non-virtualized mode");
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
                                if (debug) console.log("Final safety scroll to bottom");
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
        if (debug) console.log(`SCROLL TRIGGER: ${triggerId}`);
        
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
        if (debug) console.log(`Auto-scroll ${newValue ? 'enabled' : 'disabled'} via ${source}`);
        
        // Track this state change
        trackScrollTrigger(`setAutoScroll:${newValue ? 'ON' : 'OFF'}:${source}`);
        
        // IMPORTANT: If trying to enable auto-scroll but manual lock is active, refuse
        if (newValue && manualScrollLock && source !== 'userPreference') {
            if (debug) console.warn(`Auto-scroll enable BLOCKED due to active manual lock`);
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
                                if (debug) console.log(`setAutoScroll retry (${delay}ms): forcing scrollTop=0`);
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
            if (debug) console.warn(`Forced scroll BLOCKED: autoScroll=${autoScroll}, userScrolling=${isUserScrolling}, manualLock=${manualScrollLock}`);
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
                        if (debug) console.log("Post-animation scroll safety check");
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
                        if (debug) console.log(`Direct force to bottom in scrollToBottom (was ${scrollContainer.scrollTop})`);
                        
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
                        if (debug) console.log(`Final force to bottom in scrollToBottom (was still ${scrollContainer.scrollTop})`);
                        
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
                    if (debug) console.log(`Post-processing scroll check #${index + 1} at t+${delay}ms`);
                    
                    // Check if we're not already at the bottom
                    if (scrollContainer && scrollContainer.scrollTop !== 0) {
                        if (debug) console.log(`Fixing scroll position in post-processing: ${scrollContainer.scrollTop} -> 0`);
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
                                        if (debug) console.log("Double-checking post-processing scroll");
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
                if (debug) console.log("Executing force scroll after high volume");
                forceScrollToBottom();
            }
            scheduleForceScrollTimer = null;
        }, delay);
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
            console.log(`Return to bottom button visible (scrollTop=${scrollContainer.scrollTop})`);
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
        // CRITICAL DEBUGGING - Add this console log to verify scroll events are detected
        if (debug) console.log("⚡ SCROLL EVENT DETECTED");
        
        // Always ignore programmatic scrolling
        if (isProgrammaticScroll) {
            if (debug) console.log("Ignoring programmatic scroll event");
            return;
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
            if (debug) console.log("Manual scroll lock timeout expired");
            manualScrollLock = false;
            manualScrollLockTimer = null;
        }, 3000);
        
        // CRITICAL FIX: Handle virtualization update immediately in a separate animation frame
        // This is critical to ensure the virtualization is updated properly during scrolling
        if (virtualEnabled && virtualizationReady) {
            requestAnimationFrame(() => {
                // Skip if no longer scrolling (e.g., if event was cancellation)
                if (!isUserScrolling) return;
                
                // Update the virtualization window
                updateVirtualization();
                
                // Schedule another update after a short delay
                // This ensures smoother scrolling with multiple updates
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
            
            if (debug) console.log(`Handling scroll: scrollTop=${scrollTop}px, height=${scrollContainer.scrollHeight}px, client=${scrollContainer.clientHeight}px`);
            
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
                    if (debug) console.log("User scrolling flag cleared after timeout");
                    isUserScrolling = false;
                    
                    // Do one final virtualization update when scrolling ends
                    if (virtualizationReady && virtualEnabled) {
                        updateVirtualization();
                    }
                } else {
                    if (debug) console.log("Keeping user scrolling flag due to active manual lock");
                }
                
                // Get final scroll position
                const finalScrollTop = scrollContainer?.scrollTop || 0;
                
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
        
        if (debug) console.log("Resetting virtualization");
        
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
                    if (debug) console.log(`Virtualization reset complete: ${virtualStart}-${virtualEnd}`);
                } else {
                    // Mark virtualization as ready after this initial setup
                    virtualizationReady = true;
                    updateVirtualization();
                    
                    if (debug) console.log("Virtualization marked as ready during reset");
                }
            }
            
            // If auto-scroll is enabled, ensure we're at the bottom
            if (autoScroll && scrollContainer) {
                if (debug) console.log("Scrolling to bottom after virtualization reset");
                scrollToBottomWithStrategy();
            }
        }, 100); // Slightly longer timeout to ensure complete DOM updates
    }
    
    // Toggle virtualization manually (dev mode)
    function toggleVirtualization(): void {
        manualVirtualToggle = true;
        virtualEnabled = !virtualEnabled;
        
        if (debug) console.log(`Toggling virtualization: ${virtualEnabled ? 'ON' : 'OFF'}`);
        
        // Reset all virtualization state completely
        virtualStart = 0;
        virtualEnd = virtualEnabled ? Math.min(100, filteredLogs.length - 1) : 0;
        
        // When toggling OFF, ensure we clean up any absolute positioning
        if (!virtualEnabled) {
            if (debug) console.log("Disabling virtualization - cleaning up positioning");
            
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
            if (debug) console.log("Enabling virtualization - initializing view");
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
                console.warn(`Skipping unreasonable average height update: ${newAvg}px`);
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
            console.log(`Height stats: logs=${filteredLogs.length}, samples=${heightCount}, avg=${safeAvgHeight.toFixed(1)}px, min=${minHeight}px, max=${maxHeight}px`);
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
            if (debug) console.log("Skipping virtualization update - not ready or enabled");
            return; 
        }
        
        const { scrollTop, clientHeight, scrollHeight } = scrollContainer;
        
        // DIAGNOSTIC LOG - Add only when debugging scroll issues
        if (debug) console.log(`VIRTUALIZATION UPDATE - scrollTop=${scrollTop}, clientHeight=${clientHeight}, scrollHeight=${scrollHeight}`);
        
        viewportHeight = clientHeight;
        
        // Special case for empty logs
        if (filteredLogs.length === 0) {
            virtualStart = 0;
            virtualEnd = 0;
            visibleLogCount = 0;
            return;
        }
        
        // CRITICAL SAFETY CHECK - Ensure we're always showing at least SOME logs
        // If filteredLogs has content but we're not showing anything, this is a critical error
        if (filteredLogs.length > 0 && (virtualStart > virtualEnd || virtualEnd < 0 || virtualStart >= filteredLogs.length)) {
            if (debug) console.error(`CRITICAL: Invalid virtualization window detected - resetting to show newest logs`);
            
            // Force to show the latest logs as a fallback
            const totalLogs = filteredLogs.length;
            virtualEnd = totalLogs - 1;
            virtualStart = Math.max(0, virtualEnd - 60); // Show at least 60 latest logs
            visibleLogCount = virtualEnd - virtualStart + 1;
            
            // Force a recalculation of positions
            recalculatePositions();
            return;
        }
        
        // When auto-scroll is enabled, ensure we prioritize latest logs (newest, at the bottom in column-reverse)
        if (autoScroll && isScrolledToBottom()) {
            // Start from the newest logs (end of the array for filteredLogs which are in timestamp order)
            const lastLogIndex = filteredLogs.length - 1;
            virtualEnd = lastLogIndex;
            
            // Determine how many logs fit in the viewport plus buffer
            const safeAvgHeight = Math.max(25, avgLogHeight); // Use a reasonable minimum
            const visibleCount = Math.max(60, Math.ceil(clientHeight / safeAvgHeight)); // Show at least 60 logs
            virtualStart = Math.max(0, lastLogIndex - visibleCount);
            
            // Update visible log count
            visibleLogCount = virtualEnd - virtualStart + 1;
            
            if (debug) console.log(`Auto-scroll virtualization: showing ${virtualStart}-${virtualEnd} (${visibleLogCount} logs)`);
        } else {
            // For regular scrolling, we need to determine which logs are visible
            // SIMPLIFIED APPROACH: Use a simple ratio calculation that's more reliable
            
            const totalLogs = filteredLogs.length;
            
            // Calculate available scroll space and normalized scroll position
            const maxScrollPosition = Math.max(1, scrollHeight - clientHeight); // Ensure non-zero
            
            // Handle extreme cases (very small/large scrollTop)
            if (scrollTop <= 0) {
                // At the bottom (newest logs) in column-reverse
                virtualEnd = totalLogs - 1;
                virtualStart = Math.max(0, virtualEnd - 60);
            } else if (scrollTop >= maxScrollPosition) {
                // At the top (oldest logs) in column-reverse
                virtualStart = 0;
                virtualEnd = Math.min(totalLogs - 1, 60);
            } else {
                // Somewhere in the middle - calculate based on scroll percentage
                // For column-reverse: 0% = at bottom (newest), 100% = at top (oldest)
                const scrollPercentage = scrollTop / maxScrollPosition;
                
                // Map the percentage to an index in the logs array
                // If scrollPercentage = 0, we want the newest logs (end of array)
                // If scrollPercentage = 1, we want the oldest logs (start of array)
                const centerIndex = Math.floor((1 - scrollPercentage) * totalLogs);
                
                // Calculate a window around this center point
                const halfWindow = 30; // Half the number of logs we want to show
                virtualStart = Math.max(0, centerIndex - halfWindow);
                virtualEnd = Math.min(totalLogs - 1, centerIndex + halfWindow);
            }
            
            // SAFETY: Ensure window is at least 60 logs if possible
            if (virtualEnd - virtualStart < 60 && totalLogs > 60) {
                if (virtualStart === 0) {
                    virtualEnd = Math.min(totalLogs - 1, 60);
                } else if (virtualEnd === totalLogs - 1) {
                    virtualStart = Math.max(0, totalLogs - 61);
                }
            }
            
            // Update visible log count
            visibleLogCount = virtualEnd - virtualStart + 1;
            
            if (debug) {
                console.log(`Simple scroll virtualization: scrollTop=${scrollTop}, scrollHeight=${scrollHeight}`);
                console.log(`Showing logs ${virtualStart}-${virtualEnd} (${visibleLogCount} logs)`);
            }
        }
        
        // FINAL SAFETY CHECK - Make absolutely sure we're showing something
        if (virtualStart > virtualEnd || virtualEnd < 0 || virtualStart >= filteredLogs.length) {
            if (debug) console.error("CRITICAL: Invalid virtualization window after calculation - showing latest logs");
            
            // Force to show the latest logs as a last resort
            const totalLogs = filteredLogs.length;
            virtualEnd = totalLogs - 1;
            virtualStart = Math.max(0, virtualEnd - 60);
            visibleLogCount = virtualEnd - virtualStart + 1;
        }
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
        
        if (debug) console.log("Saved viewport anchor:", viewportAnchor);
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
                if (debug) console.warn(`Anchor log (sequence=${sequence}) not found in current filtered set`);
                
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
            console.error("Error during restoreViewportAnchor:", error);
        }
    }
    
    // Measure individual log height using full bounding rect
    function measureLogEntry(node: HTMLElement, log: LogMessage) {
        const sequence = log._sequence || 0;
        
        // IMPORTANT: Skip for logs without a valid sequence
        if (sequence === undefined || sequence === null) {
            if (debug) console.warn("Skipping measurement for log with invalid sequence");
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
                    console.warn(`Height increase detected for log ${sequence}: ${currentHeight}px -> ${height}px`);
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
        if (debug) console.log(`Debug overlay ${debugAutoScroll ? 'enabled' : 'disabled'}`);
    }

    onMount(async () => {
        if (debug) console.log("LogViewer component mounting");
        
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
            console.log("Debug mode active - enabling verbose logging");
        }
        
        // CRITICAL: Set up scroll monitor interval to guarantee scroll events are always caught
        // This helps when normal scroll events might be missed due to browser performance issues
        scrollMonitorInterval = window.setInterval(() => {
            if (!scrollContainer) return;
            
            // Store current scrollTop
            const currentScrollTop = scrollContainer.scrollTop;
            
            // Check if scrollTop has changed since last check
            if (lastKnownScrollTop !== currentScrollTop) {
                if (debug) console.log(`Scroll monitor detected change: ${lastKnownScrollTop} -> ${currentScrollTop}`);
                
                // Update last known value
                lastKnownScrollTop = currentScrollTop;
                
                // If virtualization is enabled, update it directly
                if (virtualEnabled && virtualizationReady) {
                    updateVirtualization();
                }
            }
        }, 100); // Check every 100ms
        
        // Add scroll event listener ASAP
        if (scrollContainer) {
            if (debug) console.log("Adding scroll event listener");
            scrollContainer.addEventListener('scroll', handleScroll, { passive: true });
            // Store initial scrollTop
            lastKnownScrollTop = scrollContainer.scrollTop;
        } else {
            if (debug) console.warn("No scroll container yet - will retry scroll listener setup");
            // Retry after a short delay if container isn't available yet
            setTimeout(() => {
                if (scrollContainer) {
                    if (debug) console.log("Adding scroll event listener (retry)");
                    scrollContainer.addEventListener('scroll', handleScroll, { passive: true });
                    // Store initial scrollTop
                    lastKnownScrollTop = scrollContainer.scrollTop;
                } else {
                    console.error("CRITICAL: Failed to set up scroll listener - no container");
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
            if (debug) console.log("Resize detected");
            
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
            if (debug) console.warn("No scroll container for ResizeObserver - will observe document only");
            resizeObserver.observe(document.documentElement);
        }
        
        // Set a timeout to enable virtualization after initial rendering
        setTimeout(async () => {
            if (debug) console.log("Enabling virtualization after initial render");
            
            // By this point, some logs should have been measured
            await tick();
            
            // Start the virtualization system
            initialMeasurementsComplete = true;
            virtualizationReady = true;
            
            // Recalculate positions and update virtualization
            recalculatePositions();
            
            if (virtualEnabled) {
                if (debug) console.log("Initializing virtual display");
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
                    
                    if (debug) console.log(`Final virtualization setup: displaying logs ${virtualStart}-${virtualEnd}`);
                }
            }, 300);
        }, 200);
        
        // onMount cleanup function
        return () => {
            if (debug) console.log("LogViewer component unmounting - cleaning up resources");
            
            // Clear all timers and listeners
            if (velocityDecayTimer) clearInterval(velocityDecayTimer);
            if (scrollMonitorInterval) clearInterval(scrollMonitorInterval);
            if (scrollContainer) scrollContainer.removeEventListener('scroll', handleScroll);
            if (userScrollTimeout) clearTimeout(userScrollTimeout);
            if (scrollRAF) cancelAnimationFrame(scrollRAF);
            if (batchMeasurementTimer) clearTimeout(batchMeasurementTimer);
            if (manualScrollLockTimer) clearTimeout(manualScrollLockTimer);
            if (autoScrollToastTimer) clearTimeout(autoScrollToastTimer);
            if (scheduleForceScrollTimer) clearTimeout(scheduleForceScrollTimer);
            cancelPostProcessingChecks();
            
            // Disconnect observers
            resizeObserver.disconnect();
            
            // Remove key listeners
            document.removeEventListener('keydown', toggleDebugOverlay);
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
                    <!-- Visual indicators for virtualization mode -->
                    {#if virtualEnabled}
                        <!-- Show virtual range indicator for easier debugging -->
                        <div 
                            class="fixed top-2 right-2 z-50 bg-black/80 text-primary text-xs px-2 py-1 rounded pointer-events-none flex items-center gap-2"
                        >
                            <span class="text-green-400 font-bold">VIRTUAL MODE</span>
                            <span>Showing {visibleLogCount} of {filteredLogs.length} logs ({virtualStart}-{virtualEnd})</span>
                        </div>
                    {/if}
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
                        <!-- CRITICAL FIX: Use a fixed-positioned dummy spacer at top and bottom 
                             instead of absolute positioning each element -->
                        <div 
                            class="w-full"
                            style="height: {virtualStart * Math.max(10, avgLogHeight)}px;"
                        ></div>
                        
                        <!-- Show virtualized logs in simple, scrollable list -->
                        {#each filteredLogs.slice(virtualStart, virtualEnd + 1) as log, index (log._sequence + '-' + index)}
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
                        
                        <!-- Bottom spacer -->
                        <div 
                            class="w-full"
                            style="height: {(filteredLogs.length - virtualEnd - 1) * Math.max(10, avgLogHeight)}px;"
                        ></div>
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