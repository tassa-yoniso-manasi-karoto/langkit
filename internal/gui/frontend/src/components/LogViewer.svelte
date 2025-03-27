<script lang="ts">
    import { onMount, onDestroy, tick } from 'svelte';
    import { settings } from '../lib/stores';
    import { logStore, type LogMessage } from '../lib/logStore';

    // Optional version prop to handle dev vs. prod initialization
    export let version: string = "dev";

    // Decide initial log filter
    let selectedLogLevel = version === "dev" ? "DEBUG" : "INFO";
    
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
    
    // Scroll state
    let autoScroll = true;
    let isUserScrolling = false;
    let userScrollTimeout: number | null = null;
    let scrollResetTimeout: number | null = null;
    let scheduledScrollToBottom = false;
    
    // Viewport anchoring for stable scrolling
    let viewportAnchor: { 
        sequence: number, 
        offset: number,
        height: number 
    } | null = null;
    
    // Virtualization
    let virtualStart = 0;
    let virtualEnd = 0;
    const BUFFER_SIZE = 50; // How many logs to render above/below viewport
    let viewportHeight = 0;
    let avgLogHeight = 25; // Initial estimate, will be refined
    let totalHeight = 0;
    let virtualEnabled = true;
    let virtualContainerHeight = 0;
    let documentHeight = 0;
    let visibleLogCount = 0;
    
    // Log filtering
    let cachedFilteredLogs: LogMessage[] = [];

    // Filter logs by level
    $: filteredLogs = $logStore.filter(log => 
        logLevelPriority[log.level?.toLowerCase() || 'info'] >= 
        logLevelPriority[selectedLogLevel.toLowerCase()]
    );
    
    // Update virtual container height when logs change
    $: {
        // Only update if we have logs and container is mounted
        if (filteredLogs.length > 0 && scrollContainer) {
            updateVirtualization();
        }
    }
    
    // When log level changes, reset virtualization
    $: {
        if (selectedLogLevel) {
            resetVirtualization();
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

    // Helper function: format additional fields
    function formatFields(fields: Record<string, any> | undefined): string {
        if (!fields) return '';
        
        const excluded = ['level', 'message', 'time', 'behavior', '_sequence', '_unix_time', '_original_time', '_visible', '_height'];
        return Object.entries(fields)
            .filter(([key]) => !excluded.includes(key))
            .map(([key, value]) => {
                if (typeof value === 'object') {
                    return `${key}=${JSON.stringify(value)}`;
                }
                return `${key}=${value}`;
            })
            .join(' ');
    }
    
    // Check if we're at the bottom of the scroll container
    function isScrolledToBottom(tolerance = 50): boolean {
        if (!scrollContainer) return true;
        
        const { scrollTop, scrollHeight, clientHeight } = scrollContainer;
        return scrollHeight - scrollTop - clientHeight <= tolerance;
    }
    
    // Update virtualization calculations
    function updateVirtualization(): void {
        if (!scrollContainer || !virtualEnabled) return;
        
        const { scrollTop, clientHeight } = scrollContainer;
        viewportHeight = clientHeight;
        documentHeight = document.documentElement.clientHeight;
        
        // Calculate visible range based on scroll position
        const estimatedTotalLogs = filteredLogs.length;
        const estimatedTotalHeight = estimatedTotalLogs * avgLogHeight;
        
        // Calculate which logs should be visible
        const estimatedStartIndex = Math.floor(scrollTop / avgLogHeight);
        const estimatedVisibleCount = Math.ceil(clientHeight / avgLogHeight);
        
        // Add buffer for smoother scrolling
        virtualStart = Math.max(0, estimatedStartIndex - BUFFER_SIZE);
        virtualEnd = Math.min(estimatedTotalLogs - 1, estimatedStartIndex + estimatedVisibleCount + BUFFER_SIZE);
        
        // Calculate virtual container height
        virtualContainerHeight = estimatedTotalHeight;
        
        // Update visible log count for debugging
        visibleLogCount = virtualEnd - virtualStart + 1;
    }
    
    // Reset virtualization settings
    function resetVirtualization(): void {
        virtualStart = 0;
        virtualEnd = 0;
        viewportAnchor = null;
        
        setTimeout(() => {
            updateVirtualization();
            
            // If auto scroll enabled, scroll to bottom
            if (autoScroll) {
                scrollToBottom();
            }
        }, 0);
    }
    
    // Calculate and update log heights for more accurate virtualization
    function updateLogHeights(): void {
        if (!scrollContainer) return;
        
        let totalMeasuredHeight = 0;
        let measuredCount = 0;
        
        // Measure all visible log elements
        const logElements = scrollContainer.querySelectorAll('.log-entry');
        logElements.forEach(element => {
            const height = element.getBoundingClientRect().height;
            const sequenceAttr = element.getAttribute('data-log-sequence');
            
            if (sequenceAttr && height > 0) {
                const sequence = parseInt(sequenceAttr, 10);
                totalMeasuredHeight += height;
                measuredCount++;
            }
        });
        
        // Update average height if we measured logs
        if (measuredCount > 0) {
            avgLogHeight = totalMeasuredHeight / measuredCount;
        }
    }
    
    // Save viewport anchor for stable scrolling
    function saveScrollAnchor(): void {
        if (!scrollContainer) return;
        
        // If already at bottom, don't create an anchor
        if (isScrolledToBottom() && autoScroll) {
            viewportAnchor = null;
            return;
        }
        
        // Find a log element in the middle of the viewport
        const { scrollTop, clientHeight } = scrollContainer;
        const middleY = scrollTop + (clientHeight / 2);
        
        // Find log element closest to middle
        let closestElement: Element | null = null;
        let closestDistance = Infinity;
        
        const logElements = scrollContainer.querySelectorAll('.log-entry');
        logElements.forEach(element => {
            const rect = element.getBoundingClientRect();
            const elementMiddle = rect.top + (rect.height / 2);
            const distance = Math.abs(elementMiddle - middleY);
            
            if (distance < closestDistance) {
                closestDistance = distance;
                closestElement = element;
            }
        });
        
        // Save anchor if found
        if (closestElement) {
            const sequenceAttr = closestElement.getAttribute('data-log-sequence');
            if (sequenceAttr) {
                const sequence = parseInt(sequenceAttr, 10);
                const rect = closestElement.getBoundingClientRect();
                
                viewportAnchor = {
                    sequence,
                    offset: rect.top - scrollContainer.getBoundingClientRect().top,
                    height: rect.height
                };
            }
        }
    }
    
    // Restore scroll position based on viewport anchor
    async function restoreScrollAnchor(): Promise<boolean> {
        if (!scrollContainer || !viewportAnchor) return false;
        
        // Find the anchor element
        const anchorElement = scrollContainer.querySelector(`[data-log-sequence="${viewportAnchor.sequence}"]`);
        if (!anchorElement) return false;
        
        // Restore scroll position based on anchor
        const rect = anchorElement.getBoundingClientRect();
        const containerRect = scrollContainer.getBoundingClientRect();
        const targetScrollTop = scrollContainer.scrollTop + 
            (rect.top - containerRect.top) - viewportAnchor.offset;
        
        // Apply scroll
        scrollContainer.scrollTop = targetScrollTop;
        return true;
    }
    
    // Scroll to bottom with better performance
    function scrollToBottom(): void {
        if (!scrollContainer || !autoScroll || isUserScrolling) return;
        
        // Avoid multiple calls
        if (scheduledScrollToBottom) return;
        scheduledScrollToBottom = true;
        
        // Use rAF for better performance
        requestAnimationFrame(() => {
            if (scrollContainer) {
                // Use scrollIntoView on last element if possible
                const lastLog = scrollContainer.querySelector('.log-entry:last-child');
                if (lastLog) {
                    lastLog.scrollIntoView({ behavior: 'auto', block: 'end' });
                } else {
                    // Fallback to direct scrollTop
                    scrollContainer.scrollTop = scrollContainer.scrollHeight;
                }
            }
            scheduledScrollToBottom = false;
        });
    }
    
    // Toggle auto-scroll with proper cleanup
    function toggleAutoScroll(value: boolean): void {
        if (autoScroll === value) return;
        
        autoScroll = value;
        if (autoScroll) {
            viewportAnchor = null;
            scrollToBottom();
        } else {
            // Save scroll position before disabling auto-scroll
            saveScrollAnchor();
        }
    }
    
    // Enhanced scroll handler
    function handleScroll(event: Event): void {
        // Skip if programmatically scrolling
        if (scheduledScrollToBottom) return;
        
        // Clear any pending scroll timeouts
        if (scrollResetTimeout) {
            clearTimeout(scrollResetTimeout);
            scrollResetTimeout = null;
        }
        
        // Update virtualization
        updateVirtualization();
        
        // Handle user scrolling detection
        isUserScrolling = true;
        
        // Check if user scrolled to bottom
        const atBottom = isScrolledToBottom(20);
        
        // If not at bottom and auto-scroll is on, disable it
        if (!atBottom && autoScroll) {
            autoScroll = false;
            saveScrollAnchor();
        } 
        // If at bottom and auto-scroll is off, enable it
        else if (atBottom && !autoScroll) {
            autoScroll = true;
            viewportAnchor = null;
        }
        
        // Clear any existing user scroll timeout
        if (userScrollTimeout) {
            clearTimeout(userScrollTimeout);
        }
        
        // Set a timeout to detect when user finishes scrolling
        userScrollTimeout = window.setTimeout(() => {
            isUserScrolling = false;
            
            // Update log heights for better virtualization
            updateLogHeights();
            
            // If auto-scroll and at bottom, ensure we stay at bottom
            if (autoScroll && isScrolledToBottom()) {
                scrollToBottom();
            }
        }, 200);
    }
    
    // Setup observers and listeners on mount
    onMount(async () => {
        // Initial update
        updateVirtualization();
        
        if (autoScroll) {
            await tick();
            scrollToBottom();
        }
        
        // Set up ResizeObserver to detect size changes
        const resizeObserver = new ResizeObserver(() => {
            // Save scroll position
            const wasAtBottom = isScrolledToBottom();
            
            // Update layout
            updateVirtualization();
            updateLogHeights();
            
            // Restore scroll position
            if (wasAtBottom && autoScroll) {
                scrollToBottom();
            } else if (viewportAnchor) {
                restoreScrollAnchor();
            }
        });
        
        if (scrollContainer) {
            resizeObserver.observe(scrollContainer);
            resizeObserver.observe(document.documentElement);
        }
        
        // Set up interval to periodically update log heights
        const heightUpdateInterval = setInterval(() => {
            updateLogHeights();
        }, 5000);
        
        // Set up MutationObserver to detect new logs
        const mutationObserver = new MutationObserver((mutations) => {
            for (const mutation of mutations) {
                if (mutation.type === 'childList' && mutation.addedNodes.length > 0) {
                    // If auto-scroll, scroll to bottom when new logs arrive
                    if (autoScroll && !isUserScrolling) {
                        scrollToBottom();
                    }
                }
            }
        });
        
        // Observe changes to the log container
        if (scrollContainer) {
            mutationObserver.observe(scrollContainer, {
                childList: true,
                subtree: true
            });
        }
        
        return () => {
            resizeObserver.disconnect();
            mutationObserver.disconnect();
            clearInterval(heightUpdateInterval);
            
            if (userScrollTimeout) {
                clearTimeout(userScrollTimeout);
            }
            
            if (scrollResetTimeout) {
                clearTimeout(scrollResetTimeout);
            }
        };
    });
    
    onDestroy(() => {
        // Clean up any remaining timeouts
        if (userScrollTimeout) {
            clearTimeout(userScrollTimeout);
            userScrollTimeout = null;
        }
        
        if (scrollResetTimeout) {
            clearTimeout(scrollResetTimeout);
            scrollResetTimeout = null;
        }
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
                on:click={() => logStore.clearLogs()}
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
                    on:click={() => {
                        virtualEnabled = !virtualEnabled;
                        resetVirtualization();
                    }}
                    class="px-3 py-1 h-7 bg-[#333] text-text rounded whitespace-nowrap 
                           flex-shrink-0 text-[11px] uppercase tracking-wider 
                           hover:bg-primary/10 hover:text-white hover:border-primary/55 hover:shadow-input 
                           transition-all duration-200"
                >
                    {virtualEnabled ? 'Virt: ON' : 'Virt: OFF'}
                </button>
            {/if}
        </div>
        
        <!-- Debug info (dev only) -->
        {#if version === 'dev'}
            <div class="text-xs text-primary/50">
                {visibleLogCount}/{filteredLogs.length} logs | ~{avgLogHeight.toFixed(1)}px/log
            </div>
        {/if}
    </div>
    
    <!-- Content area with virtualization -->
    <div class="relative flex flex-col flex-1 min-h-0">
        <!-- The scrollable container for log entries -->
        <div 
            class="flex-1 overflow-y-auto min-h-0 log-scroll-container"
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
                    style="height: {virtualEnabled ? `${virtualContainerHeight}px` : 'auto'}"
                >
                    <!-- Only render logs within the visible range -->
                    {#if virtualEnabled}
                        {#each filteredLogs.slice(virtualStart, virtualEnd + 1) as log, i (log._sequence)}
                            <div 
                                class="log-entry {log.behavior ? behaviorColors[log.behavior] : 'text-white/90'}
                                py-1 px-3 border-b border-primary/10 whitespace-pre-wrap break-words leading-snug
                                flex items-baseline justify-start text-left w-full hover:bg-white/5 transition-colors duration-200"
                                style="position: absolute; top: {virtualStart * avgLogHeight + i * 0}px; left: 0; right: 0; transform: translateY({i * avgLogHeight}px);"
                                data-log-sequence={log._sequence}
                                data-unix-time={log._unix_time}
                            >
                                <!-- Timestamp -->
                                <span class="text-primary/60 mr-2 text-xs flex-shrink-0">
                                    {log.time}
                                </span>
                                
                                <!-- Log level -->
                                <span class={"font-bold text-sm mr-2 flex-shrink-0 min-w-[40px] " + getLevelClass(log.level)}>
                                    {log.level}
                                </span>
                                
                                <!-- Message with structured fields -->
                                <span class="flex-grow text-sm text-left overflow-x-auto">
                                    <!-- Show message if it exists -->
                                    {#if log.message}
                                        {log.message}
                                    {/if}
                                    
                                    <!-- Always show structured fields if they exist -->
                                    {#if formatFields(log)}
                                        <span class="{log.message ? 'ml-2' : ''} text-primary/50 text-[12px] font-[DM_Mono]">
                                            {formatFields(log)}
                                        </span>
                                    {/if}
                                </span>
                            </div>
                        {/each}
                    {:else}
                        <!-- Non-virtualized rendering (all logs) -->
                        {#each filteredLogs as log (log._sequence)}
                            <div 
                                class="log-entry {log.behavior ? behaviorColors[log.behavior] : 'text-white/90'}
                                py-1 px-3 border-b border-primary/10 whitespace-pre-wrap break-words leading-snug
                                flex items-baseline justify-start text-left w-full hover:bg-white/5 transition-colors duration-200"
                                data-log-sequence={log._sequence}
                                data-unix-time={log._unix_time}
                            >
                                <!-- Timestamp -->
                                <span class="text-primary/60 mr-2 text-xs flex-shrink-0">
                                    {log.time}
                                </span>
                                
                                <!-- Log level -->
                                <span class={"font-bold text-sm mr-2 flex-shrink-0 min-w-[40px] " + getLevelClass(log.level)}>
                                    {log.level}
                                </span>
                                
                                <!-- Message with structured fields -->
                                <span class="flex-grow text-sm text-left overflow-x-auto">
                                    <!-- Show message if it exists -->
                                    {#if log.message}
                                        {log.message}
                                    {/if}
                                    
                                    <!-- Always show structured fields if they exist -->
                                    {#if formatFields(log)}
                                        <span class="{log.message ? 'ml-2' : ''} text-primary/50 text-[12px] font-[DM_Mono]">
                                            {formatFields(log)}
                                        </span>
                                    {/if}
                                </span>
                            </div>
                        {/each}
                    {/if}
                </div>
            {/if}
        </div>
    </div>
</div>

<style>
    @keyframes spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
    }
    
    .animate-spin {
        animation: spin 1s linear infinite;
    }

    /* Custom scrollbar styling for log viewer */
    .log-scroll-container {
        scrollbar-width: thin;
        scrollbar-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4) transparent;
        scroll-behavior: smooth;
    }

    .log-scroll-container::-webkit-scrollbar {
        width: 6px;
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
</style>