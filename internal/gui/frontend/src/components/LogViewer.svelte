<script lang="ts">
    import { onMount, onDestroy, tick, afterUpdate } from 'svelte';
    import { settings } from '../lib/stores';
    import { logStore, type LogMessage } from '../lib/logStore';
    import { slide } from 'svelte/transition';
    import { backOut } from 'svelte/easing';

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
    
    // Scroll state management
    let autoScroll = true;
    let isUserScrolling = false;
    let userScrollTimeout: number | null = null;
    let scrollRAF: number | null = null;
    let scheduledScrollToBottom = false;
    let isProgrammaticScroll = false;
    let lastScrollTop = 0;
    let scrollDirection: 'up' | 'down' | 'none' = 'none';
    let lastUserScrollTime = 0;
    let scrollEventCount = 0;
    let consecutiveSystemScrollEvents = 0;
    
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
    let logHeights = new Map<number, number>(); // Maps sequence -> actual height
    let logPositions = new Map<number, number>(); // Maps sequence -> Y position
    let totalLogHeight = 0;
    let avgLogHeight = 25; // Initial estimate, will be refined
    
    // Positioning buffer (to prevent overlap)
    const POSITION_BUFFER = 2; // Add 2px buffer between entries
    
    // Animation and filter state
    let filterTransitionRunning = false;
    let filterChangeTimestamp = Date.now();
    let recentlyAddedLogs = new Set<number>();
    
    // Track measurement batches
    let pendingMeasurements = false;
    let batchMeasurementTimer: number | null = null;
    
    // Debugging and development logging
    let debug = version === 'dev';
    
    // Auto-scroll debug stats 
    let lastAutoScrollTime = Date.now();
    let autoScrollTriggerCount = 0;
    let logsBatchedSinceLastScroll = 0;
    
    // Track active transitions for non-virtualized mode
    let activeTransitions = 0;
    
    // Filter logs by level
    $: filteredLogs = $logStore.filter(log => 
        logLevelPriority[log.level?.toLowerCase() || 'info'] >= 
        logLevelPriority[selectedLogLevel.toLowerCase()]
    );
    
    // Subscribe to log store exceededMaxEntries to toggle virtualization
    $: {
        if (!manualVirtualToggle) { // Only auto-toggle if user hasn't manually set it
            const shouldVirtualize = $logStore.exceededMaxEntries;
            
            if (shouldVirtualize !== virtualEnabled) {
                if (debug) {
                    console.log("Auto-toggling virtualization: " + (shouldVirtualize ? 'ON' : 'OFF') + " (log count: " + filteredLogs.length + ")");
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
        // Save scroll position before updating
        const wasAtBottom = isScrolledToBottom();
        
        // Track current viewport anchor if not at bottom
        if (!wasAtBottom && !autoScroll) {
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
                    scrollToBottomWithStrategy();
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
                
                // Restore scroll position using the appropriate strategy
                if (wasAtBottom && autoScroll) {
                    // If at bottom, maintain position at bottom
                    scrollToBottomWithStrategy();
                } else if (!autoScroll && viewportAnchor) {
                    // Otherwise restore anchor position
                    restoreViewportAnchor();
                }
                
                pendingMeasurements = false;
                batchMeasurementTimer = null;
            }, 10); // Small delay to batch updates
        }
    }
    
    // After DOM updates, handle any needed scrolling
    afterUpdate(() => {
        // If auto-scroll is on and we're not user scrolling, scroll to bottom
        if (autoScroll && !isUserScrolling && !pendingMeasurements) {
            scrollToBottomWithStrategy();
        }
    });
    
    // Schedule scroll checks after processing completes
    function schedulePostProcessingScrolls() {
        // Cancel any existing timers
        cancelPostProcessingChecks();
        
        // Use staggered timing to catch all rendering phases
        const checkTimes = [100, 300, 600, 1000, 1500];
        
        checkTimes.forEach((delay, index) => {
            const timerId = window.setTimeout(() => {
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
            if (autoScroll) {
                // Force scroll regardless of other state
                if (debug) console.log("Executing force scroll after high volume");
                forceScrollToBottom();
            }
            forceScrollTimer = null;
        }, delay);
    }
    
    // Force scroll to bottom - more aggressive than normal scroll
    function forceScrollToBottom() {
        if (!scrollContainer) return;
        
        // Avoid animations for this scroll
        isProgrammaticScroll = true;
        
        try {
            // Direct DOM manipulation for maximum reliability
            scrollContainer.scrollTop = scrollContainer.scrollHeight;
            
            // Schedule another scroll in the next frame to ensure it worked
            requestAnimationFrame(() => {
                scrollContainer.scrollTop = scrollContainer.scrollHeight;
                
                // And one more time after a short delay to really make sure
                setTimeout(() => {
                    isProgrammaticScroll = false;
                    scrollContainer.scrollTop = scrollContainer.scrollHeight;
                }, 50);
            });
        } catch (e) {
            isProgrammaticScroll = false;
            console.error("Error in force scroll:", e);
        }
    }
    
    // Helper function to intelligently scroll to bottom based on current state
    function scrollToBottomWithStrategy() {
        if (!autoScroll || !scrollContainer) return;
        
        // Cancel any existing animations if processing just ended
        if (!isProcessing && prevIsProcessing) {
            pendingScrollToBottom = false;
            
            // Be more aggressive when processing ends
            executeScrollToBottom(true);
            return;
        }
        
        // If animation is in progress in non-virtualized mode, defer scroll
        if (!virtualEnabled && animationInProgress) {
            pendingScrollToBottom = true;
            return;
        }
        
        // For virtualized mode, ensure virtualization range is updated before scrolling
        if (virtualEnabled && virtualizationReady) {
            // Ensure last logs are included in virtual range first
            const lastLogIndex = filteredLogs.length - 1;
            if (virtualEnd < lastLogIndex) {
                // Update virtual range
                virtualEnd = lastLogIndex;
                virtualStart = Math.max(0, virtualEnd - visibleLogCount - BUFFER_SIZE);
                visibleLogCount = virtualEnd - virtualStart + 1;
                
                // Need an extra tick to ensure DOM reflects the updated range
                tick().then(() => {
                    executeScrollToBottom(true); // Force scroll for virtualized mode
                });
                return;
            }
            
            // Even if range is OK, be more aggressive in virtualized mode
            executeScrollToBottom(true);
            return;
        }
        
        // For non-virtualized mode with no active animations, execute scroll
        executeScrollToBottom();
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
    
    // Check if we're at the bottom of the scroll container
    function isScrolledToBottom(tolerance = 20): boolean {
        if (!scrollContainer) return true;
        
        const { scrollTop, scrollHeight, clientHeight } = scrollContainer;
        return scrollHeight - scrollTop - clientHeight <= tolerance;
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
            if (pendingScrollToBottom && autoScroll) {
                pendingScrollToBottom = false;
                executeScrollToBottom();
            }
        }
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
            }
        };
    }
    
    // Calculate positions for all logs based on individual heights
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
        
        // Recalculate average height
        if (logHeights.size > 0) {
            let total = 0;
            logHeights.forEach(height => total += height);
            avgLogHeight = (total / logHeights.size) - POSITION_BUFFER;
        }
    }
    
    // Find which log corresponds to a scroll position using binary search
    function findLogAtScrollPosition(scrollTop: number): number {
        if (filteredLogs.length === 0) return 0;
        
        let low = 0;
        let high = filteredLogs.length - 1;
        
        while (low <= high) {
            const mid = Math.floor((low + high) / 2);
            const sequence = filteredLogs[mid]._sequence || 0;
            const pos = logPositions.get(sequence) || mid * (avgLogHeight + POSITION_BUFFER);
            const height = logHeights.get(sequence) || avgLogHeight + POSITION_BUFFER;
            
            if (scrollTop >= pos && scrollTop < (pos + height)) {
                return mid; // Found exact log
            }
            
            if (scrollTop < pos) {
                high = mid - 1;
            } else {
                low = mid + 1;
            }
        }
        
        // Return the closest log index
        return Math.max(0, Math.min(filteredLogs.length - 1, low));
    }
    
    // Update virtualization calculations
    function updateVirtualization(): void {
        if (!scrollContainer || !virtualEnabled || !virtualizationReady) return;
        
        const { scrollTop, clientHeight } = scrollContainer;
        viewportHeight = clientHeight;
        
        // Find log at top of viewport
        const topLogIndex = findLogAtScrollPosition(scrollTop);
        
        // Find log at bottom of viewport
        const bottomLogIndex = findLogAtScrollPosition(scrollTop + clientHeight);
        
        // Set virtual range with buffer
        virtualStart = Math.max(0, topLogIndex - BUFFER_SIZE);
        virtualEnd = Math.min(filteredLogs.length - 1, bottomLogIndex + BUFFER_SIZE);
        
        // Update visible log count
        visibleLogCount = virtualEnd - virtualStart + 1;
        
        // When auto-scroll is enabled, ensure we include the last logs in virtualized range
        if (autoScroll && filteredLogs.length > 0) {
            // Ensure the end of the list is always in the virtualized range
            const lastLogIndex = filteredLogs.length - 1;
            if (virtualEnd < lastLogIndex) {
                // Extend the virtual window to include last logs
                virtualEnd = lastLogIndex;
                
                // Adjust start to maintain reasonable window size
                virtualStart = Math.max(0, virtualEnd - visibleLogCount - BUFFER_SIZE);
                
                // Update visible count after adjustment
                visibleLogCount = virtualEnd - virtualStart + 1;
            }
        }
    }
    
    // Save current viewport position as an anchor
    function saveViewportAnchor(): void {
        if (!scrollContainer) return;
        
        // If already at bottom, don't create an anchor
        if (isScrolledToBottom() && autoScroll) {
            viewportAnchor = null;
            return;
        }
        
        const { scrollTop } = scrollContainer;
        
        // Find which log is at the top of the viewport
        const logIndex = findLogAtScrollPosition(scrollTop);
        if (logIndex < 0 || logIndex >= filteredLogs.length) return;
        
        const log = filteredLogs[logIndex];
        const sequence = log._sequence || 0;
        const logTop = logPositions.get(sequence) || 0;
        
        // Save anchor with offset from log top
        viewportAnchor = {
            index: logIndex,
            sequence: sequence,
            offsetTop: scrollTop - logTop
        };
    }
    
    // Restore scroll position based on saved anchor
    function restoreViewportAnchor(): void {
        if (!viewportAnchor || !scrollContainer) return;
        
        // Find the log position now
        const sequence = viewportAnchor.sequence;
        const logTop = logPositions.get(sequence) || 0;
        
        // Restore scroll position with the same offset
        withProgrammaticScroll(() => {
            scrollContainer.scrollTop = logTop + viewportAnchor.offsetTop;
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
            if (autoScroll) {
                scrollToBottomWithStrategy();
            }
        }, 50);
    }
    
    // Wrapper to mark scroll operations as programmatic
    function withProgrammaticScroll(callback: () => void): void {
        isProgrammaticScroll = true;
        try {
            callback();
        } finally {
            // Use a timeout to account for any scroll events that might be triggered
            setTimeout(() => {
                isProgrammaticScroll = false;
            }, 50);
        }
    }
    
    // Execute scroll to bottom with programmatic flag
    function executeScrollToBottom(force: boolean = false): void {
        if (!scrollContainer) return;
        
        // Skip during animations unless forced
        if (animationInProgress && !force) {
            pendingScrollToBottom = true;
            return;
        }
        
        // Avoid multiple calls in the same frame
        if (scheduledScrollToBottom) return;
        scheduledScrollToBottom = true;
        
        // For debugging - track autoscroll triggers
        if (debug) {
            const now = Date.now();
            autoScrollTriggerCount++;
            if (now - lastAutoScrollTime > 1000) {
                console.log("[AutoScroll] " + autoScrollTriggerCount + " triggers, " + logsBatchedSinceLastScroll + 
                          " log batches processed in last " + (Math.round((now - lastAutoScrollTime)/100)/10) + "s");
                lastAutoScrollTime = now;
                autoScrollTriggerCount = 0;
                logsBatchedSinceLastScroll = 0;
            }
        }
        
        // Use rAF for better performance and to ensure we execute after any DOM updates
        requestAnimationFrame(() => {
            withProgrammaticScroll(() => {
                if (scrollContainer) {
                    // Ensure we're scrolling to the actual bottom after all rendering is complete
                    scrollContainer.scrollTop = scrollContainer.scrollHeight;
                    
                    // For critical scrolls (like at processing end), add a second scroll after a tiny delay
                    if (force) {
                        setTimeout(() => {
                            if (scrollContainer) {
                                scrollContainer.scrollTop = scrollContainer.scrollHeight;
                            }
                        }, 50);
                    }
                }
            });
            scheduledScrollToBottom = false;
        });
    }
    
    // Toggle auto-scroll with proper cleanup
    function toggleAutoScroll(value: boolean): void {
        if (autoScroll === value) return;
        
        autoScroll = value;
        
        if (autoScroll) {
            // When turning on auto-scroll, clear any viewport anchor and scroll to bottom
            viewportAnchor = null;
            
            // Always use the intelligent strategy
            scrollToBottomWithStrategy();
            
            // Additionally, schedule a forced scroll to handle any edge cases
            scheduleForceScroll(100);
            
            // If processing just ended, schedule post-processing scrolls
            if (!isProcessing && prevIsProcessing === false) {
                schedulePostProcessingScrolls();
            }
        } else {
            // When turning off, save scroll position for stability
            saveViewportAnchor();
            
            // Cancel any pending post-processing checks
            cancelPostProcessingChecks();
        }
    }
    
    // Toggle virtualization manually (dev mode)
    function toggleVirtualization(): void {
        manualVirtualToggle = true;
        virtualEnabled = !virtualEnabled;
        resetVirtualization();
    }
    
    // Clear logs while preserving auto-scroll state
    function clearLogsPreserveAutoScroll(): void {
        // Save current auto-scroll state
        const currentAutoScroll = autoScroll;
        
        // Clear logs
        logStore.clearLogs();
        
        // Ensure auto-scroll state is preserved
        if (currentAutoScroll !== autoScroll) {
            toggleAutoScroll(currentAutoScroll);
        }
        
        // Force scroll to bottom if auto-scroll was enabled
        if (currentAutoScroll) {
            setTimeout(() => {
                executeScrollToBottom(true);
            }, 50);
        }
    }
    
    // Enhanced scroll handler with improved user vs system scroll detection
    function handleScroll(): void {
        // Skip if programmatically scrolling (explicit flag)
        if (isProgrammaticScroll) {
            consecutiveSystemScrollEvents++;
            return;
        }
        
        // Use requestAnimationFrame for smooth scrolling
        if (scrollRAF) cancelAnimationFrame(scrollRAF);
        
        scrollRAF = requestAnimationFrame(() => {
            // Only update virtualization if it's ready and enabled
            if (virtualizationReady && virtualEnabled) {
                updateVirtualization();
            }
            
            if (!scrollContainer) {
                scrollRAF = null;
                return;
            }
            
            // Get current scroll position
            const { scrollTop } = scrollContainer;
            
            // Determine scroll direction
            if (scrollTop > lastScrollTop) {
                scrollDirection = 'down';
            } else if (scrollTop < lastScrollTop) {
                scrollDirection = 'up';
            }
            lastScrollTop = scrollTop;
            
            // Detect if this is likely user scrolling
            const currentTime = Date.now();
            const timeSinceLastScroll = currentTime - lastUserScrollTime;

            // Consider scrolling as user-initiated if:
            // 1. It's been a significant time since the last scroll (human pauses)
            // 2. OR the direction changes and is not immediately after a programmatic scroll
            // 3. AND we haven't seen many consecutive system events
            const isLikelyUserScroll = 
                (timeSinceLastScroll > 100 || scrollEventCount === 0) && 
                consecutiveSystemScrollEvents < 3 &&
                // Don't consider as user scroll during active processing without significant movement
                !(isProcessing && Math.abs(scrollTop - lastScrollTop) < 10);
            
            // Reset system scroll counter
            consecutiveSystemScrollEvents = 0;
            
            // Update tracking
            lastUserScrollTime = currentTime;
            scrollEventCount++;
            
            // If likely user scroll, mark as user scrolling
            if (isLikelyUserScroll) {
                isUserScrolling = true;
                
                // If user scrolls up, disable auto-scroll
                // Be more strict about recognizing deliberate up scrolls
                if (scrollDirection === 'up' && 
                    autoScroll && 
                    // Ensure it's a significant scroll up (avoid disabling for tiny movements)
                    Math.abs(scrollTop - lastScrollTop) > 5) {
                    
                    autoScroll = false;
                    saveViewportAnchor();
                    
                    // Cancel any pending post-processing checks
                    cancelPostProcessingChecks();
                    
                    if (debug) console.log(`Auto-scroll disabled by user scroll up (delta: ${Math.round(scrollTop - lastScrollTop)}px)`);
                }
            }
            
            // Check if scrolled to bottom
            const atBottom = isScrolledToBottom();
            
            // If scrolling down to bottom, enable auto-scroll
            // But only if it was a deliberate user action
            if (atBottom && scrollDirection === 'down' && isLikelyUserScroll) {
                if (!autoScroll) {
                    if (debug) console.log(`Auto-scroll enabled by user scroll to bottom`);
                    autoScroll = true;
                    viewportAnchor = null;
                }
            }
            
            // Clear any existing user scroll timeout
            if (userScrollTimeout) {
                clearTimeout(userScrollTimeout);
            }
            
            // Set a timeout to detect when user finishes scrolling
            userScrollTimeout = window.setTimeout(() => {
                isUserScrolling = false;
                scrollEventCount = 0;
                
                // If auto-scroll is enabled and we're at the bottom, ensure we stay there
                if (autoScroll && isScrolledToBottom()) {
                    scrollToBottomWithStrategy();
                }
            }, 200);
            
            scrollRAF = null;
        });
    }
    
    // Setup observers and listeners on mount
    onMount(async () => {
        // Initial update
        await tick();
        recalculatePositions();
        
        // Start with virtualization disabled until measurements are ready
        virtualizationReady = false;
        
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
            
            if (autoScroll) {
                scrollToBottomWithStrategy();
            }
        }, 200);
        
        // Initial scroll to bottom
        if (autoScroll) {
            scrollToBottomWithStrategy();
            
            // Schedule a force scroll after a short delay just to be sure
            scheduleForceScroll(200);
        }
        
        // Set up ResizeObserver to detect size changes
        const resizeObserver = new ResizeObserver(() => {
            // Save scroll position
            const wasAtBottom = isScrolledToBottom();
            if (!wasAtBottom && !autoScroll) {
                saveViewportAnchor();
            }
            
            // Update layout
            recalculatePositions();
            
            if (virtualEnabled && virtualizationReady) {
                updateVirtualization();
            }
            
            // Restore scroll position
            if (wasAtBottom && autoScroll) {
                scrollToBottomWithStrategy();
            } else if (viewportAnchor) {
                restoreViewportAnchor();
            }
        });
        
        if (scrollContainer) {
            resizeObserver.observe(scrollContainer);
            resizeObserver.observe(document.documentElement);
        }
        
        // Set up observer to track log store changes
        const unsubscribeLogStore = logStore.subscribe((logs) => {
            // If auto-scroll is enabled and no user scrolling is happening,
            // ensure we stay at the bottom when new logs arrive
            if (autoScroll && !isUserScrolling && scrollContainer && logs.length > 0) {
                // Check if we're already at the bottom
                const wasAtBottom = isScrolledToBottom();
                
                // If at bottom or auto-scroll is on, scroll to bottom
                if (wasAtBottom || autoScroll) {
                    scrollToBottomWithStrategy();
                }
            }
        });
        
        return () => {
            resizeObserver.disconnect();
            unsubscribeLogStore();
            
            if (userScrollTimeout) {
                clearTimeout(userScrollTimeout);
            }
            
            if (scrollRAF) {
                cancelAnimationFrame(scrollRAF);
            }
            
            if (batchMeasurementTimer) {
                clearTimeout(batchMeasurementTimer);
            }
            
            if (scrollAfterAnimationTimer) {
                clearTimeout(scrollAfterAnimationTimer);
            }
            
            if (forceScrollTimer) {
                clearTimeout(forceScrollTimer);
            }
            
            // Clean up any post-processing timers
            cancelPostProcessingChecks();
        };
    });
    
    onDestroy(() => {
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
    });
</script>

<!-- Main container for the log viewer with glassmorphism -->
<div class="flex flex-col h-full bg-logbg/60 text-white font-[DM_Mono] text-[11px] rounded-lg border-r border-b border-primary/20 shadow-log">
    <!-- Top controls row -->
    <div class="px-3 py-2 border-b border-primary/20 bg-bgold-800/60 backdrop-blur-md h-10 flex items-center justify-between rounded-t-lg">
        <div class="flex items-center gap-6">
            <!-- Log level filter -->
            <div class="flex items-center gap-2 whitespace-nowrap">
                <span class="text-xs uppercase tracking-wider font-medium text-primary-100/60">
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
                >
                    {#each logLevels as level}
                        <option value={level}>{level}</option>
                    {/each}
                </select>
            </div>

            <!-- Auto-scroll toggle -->
            <button 
                type="button" 
                class="flex items-center gap-1 px-3 py-1 bg-[#333] h-7
                       text-text rounded whitespace-nowrap flex-shrink-0 text-[11px] uppercase tracking-wider 
                       hover:bg-primary/10 hover:text-white hover:border-primary/55 hover:shadow-input
                       transition-all duration-200"
                on:click={() => toggleAutoScroll(!autoScroll)}
            >
                <input 
                    type="checkbox" 
                    checked={autoScroll}
                    on:change={(e) => toggleAutoScroll(e.target.checked)}
                    class="w-3.5 h-3.5 accent-primary m-0"
                />
                Auto-scroll
            </button>

            <!-- Clear button -->
            <button 
                on:click={clearLogsPreserveAutoScroll}
                class="px-3 py-1 h-7 bg-[#333] text-text rounded whitespace-nowrap 
                       flex-shrink-0 text-[11px] uppercase tracking-wider 
                       hover:bg-primary/10 hover:text-white hover:border-primary/55 hover:shadow-input 
                       transition-all duration-200"
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
                >
                    {virtualEnabled ? 'Virt: ON' : 'Virt: OFF'}
                </button>
                
                <!-- Debug info -->
                <span class="text-xs text-primary/50">
                    {filteredLogs.length} logs {virtualEnabled ? '| ' + visibleLogCount + ' visible' : ''} {isProcessing ? '| PROCESSING' : ''}
                </span>
            {/if}
        </div>
    </div>
    
    <!-- Content area with virtualization -->
    <div class="relative flex flex-col flex-1 min-h-0">
        <!-- The scrollable container for log entries (hide X-axis overflow only) -->
        <div 
            class="flex-1 overflow-y-auto overflow-x-hidden min-h-0 log-scroll-container"
            bind:this={scrollContainer}
            on:scroll={handleScroll}
        >
            {#if filteredLogs.length === 0}
                <!-- Empty state -->
                <div class="absolute top-0 left-0 w-full h-full flex items-center justify-center">
                    <span class="bg-black/10 backdrop-blur-sm border border-primary/30 text-primary/60 italic text-sm px-6 py-3 rounded-lg">
                        No logs to display
                    </span>
                </div>
            {:else}
                <!-- Virtual scroller container -->
                <div 
                    class="relative w-full" 
                    style="height: {virtualEnabled && virtualizationReady ? `${virtualContainerHeight}px` : 'auto'}"
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
                                data-unix-time={log._unix_time}
                                use:measureLogEntry={log}
                            >
                                <!-- Timestamp -->
                                <span class="text-primary/60 mr-2 mt-0.5 text-xs flex-shrink-0">
                                    {log.time}
                                </span>
                                
                                <!-- Log level -->
                                <span class={"font-bold mt-0.5 text-sm mr-2 flex-shrink-0 min-w-[40px] " + getLevelClass(log.level)}>
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
                                        <span class="{log.message ? 'block mt-0.5 ' : ''}text-primary/50 text-[11px] leading-relaxed whitespace-pre-wrap break-words">
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
                                style="position: absolute; top: {logPositions.get(log._sequence) || 0}px; left: 0; right: 0;"
                                data-log-sequence={log._sequence}
                                data-unix-time={log._unix_time}
                                use:measureLogEntry={log}
                            >
                                <!-- Timestamp -->
                                <span class="text-primary/60 mr-2 mt-0.5 text-xs flex-shrink-0">
                                    {log.time}
                                </span>
                                
                                <!-- Log level -->
                                <span class={"font-bold mt-0.5 text-sm mr-2 flex-shrink-0 min-w-[40px] " + getLevelClass(log.level)}>
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
                                        <span class="{log.message ? 'block mt-0.5 ' : ''}text-primary/50 text-[11px] leading-relaxed whitespace-pre-wrap break-words">
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
                                data-unix-time={log._unix_time}
                                use:measureLogEntry={log}
                                transition:slide|local={{ duration: 300, easing: backOut }}
                                on:introstart={handleTransitionStart}
                                on:introend={handleTransitionEnd}
                                on:outrostart={handleTransitionStart}
                                on:outroend={handleTransitionEnd}
                            >
                                <!-- Timestamp -->
                                <span class="text-primary/60 mr-2 mt-0.5 text-xs flex-shrink-0">
                                    {log.time}
                                </span>
                                
                                <!-- Log level -->
                                <span class={"font-bold mt-0.5 text-sm mr-2 flex-shrink-0 min-w-[40px] " + getLevelClass(log.level)}>
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
                                        <span class="{log.message ? 'block mt-0.5 ' : ''}text-primary/50 text-[11px] leading-relaxed whitespace-pre-wrap break-words">
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
</div>

<style>
    /* Custom scrollbar styling for log viewer - only Y axis visible */
    .log-scroll-container {
        scrollbar-width: thin;
        scrollbar-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4) transparent;
        scroll-behavior: smooth;
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