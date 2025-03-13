<script lang="ts">
/* Large number of logs are a challenge for the web engine to handle.
Accordingly, the implementation is currently as follows:

Triple-layered auto-scroll mechanism:
	Normal reactive updates trigger scrolling
	A mutation observer watches for DOM changes and triggers scrolling
	A periodic interval check ensures we stay at the bottom even if the other mechanisms fail


Complete isolation of user vs. programmatic scrolling:
	User scrolling is tracked with its own flag and timeout
	Programmatic scrolling uses a separate timer and doesn't interfere with user actions


More aggressive force-scrolling:
	Uses both setTimeout and requestAnimationFrame for maximum reliability
	Adds higher timeouts to ensure operations complete
*/

    import { onMount, onDestroy } from 'svelte';
    import { settings } from '../lib/stores';
    import { logStore, type LogMessage } from '../lib/logStore';

    // Optional version prop to handle dev vs. prod initialization.
    export let version: string = "dev";

    // Decide initial log filter
    let selectedLogLevel = version === "dev" ? "DEBUG" : "INFO";
    
    let scrollContainer: HTMLElement;
    let autoScroll = true;
    let scrollTimer: number | null = null;
    let mutationObserver: MutationObserver | null = null;
    let isUserScrolling = false;
    let userScrollTimeout: number;
    
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
        'abort_task': 'text-error-task',
        'abort_all': 'text-error-all',
        'user_cancel': 'text-user-cancel',
        'probe': 'text-log-warn'
    };

    // Return a Tailwind class for each log level
    function getLevelClass(level: string): string {
        switch (level.toUpperCase()) {
            case 'DEBUG':
                return 'text-log-debug';
            case 'INFO':
                return 'text-log-info';
            case 'WARN':
                return 'text-log-warn';
            case 'ERROR':
                return 'text-log-error';
            default:
                return 'text-log-info';
        }
    }

    // Helper function: format additional fields
    function formatFields(fields: Record<string, any> | undefined): string {
        if (!fields) return '';
        return Object.entries(fields)
            .map(([key, value]) => `${key}=${JSON.stringify(value)}`)
            .join(' ');
    }

    // Helper function: format structured fields
    function formatStructuredFields(log: LogMessage): string {
        const excluded = ['level', 'message', 'time', 'behavior'];
        const fields = Object.entries(log)
            .filter(([key]) => !excluded.includes(key))
            .map(([key, value]) => {
                if (typeof value === 'object') {
                    return `${key}=${JSON.stringify(value)}`;
                }
                return `${key}=${value}`;
            })
            .join(' ');
        return fields;
    }

    // Config for log display
    const MAX_VISIBLE_LOGS = 500;
    let showAllLogs = false; // Track if user has explicitly chosen to see all logs
    let cachedFilteredLogs: LogMessage[] = [];
    let isLoadingAllLogs = false; // Track when logs are being loaded
    
    // Optimize log filtering using memoization
    $: {
        // Only recompute when the log store or filter changes
        const newFilteredLogs = $logStore.filter(log => 
            logLevelPriority[log.level.toLowerCase()] >= logLevelPriority[selectedLogLevel.toLowerCase()]
        );
        
        // Apply length limit to filtered logs only if auto-scroll is enabled and not showing all logs
        if (newFilteredLogs.length > MAX_VISIBLE_LOGS && autoScroll && !showAllLogs) {
            // Keep the most recent logs (where the index is highest)
            cachedFilteredLogs = newFilteredLogs.slice(-MAX_VISIBLE_LOGS);
        } else {
            // Show all logs when user is scrolling up or has explicitly chosen to see all logs
            if (!showAllLogs && isLoadingAllLogs) {
                // Simulate a brief loading delay for better UX
                setTimeout(() => {
                    cachedFilteredLogs = newFilteredLogs;
                    isLoadingAllLogs = false;
                    showAllLogs = true;
                }, 300);
            } else {
                cachedFilteredLogs = newFilteredLogs;
            }
        }
    }
    
    // Use the cached value to prevent unnecessary re-renders
    $: filteredLogs = cachedFilteredLogs;
    
    // Handle showing all logs
    function showAllLogHistory() {
        showAllLogs = true;
        autoScroll = false; // Disable auto-scroll when viewing full history
    }
    
    // Improved user scroll handler
    function handleUserScroll() {
        // If we're scrolling programmatically, don't interfere
        if (scrollTimer !== null) return;
        
        // Check scroll position
        const { scrollTop, scrollHeight, clientHeight } = scrollContainer;
        const atBottom = scrollHeight - scrollTop - clientHeight <= 50;
        const atTop = scrollTop < 50; // Near the top of the scroll container
        
        // Handle scroll away from bottom
        if (!atBottom && autoScroll) {
            // User scrolled away from bottom - disable auto-scroll
            autoScroll = false;
            isUserScrolling = true;
        }
        
        // Auto-load all logs when user scrolls to the top
        if (atTop && !showAllLogs && !isLoadingAllLogs && $logStore.length > MAX_VISIBLE_LOGS) {
            // Trigger loading spinner and load all logs
            isLoadingAllLogs = true;
            autoScroll = false;
            // Loading and actual display happens in the reactive statement
        }
        
        // Clear any existing timeout
        if (userScrollTimeout !== null) {
            clearTimeout(userScrollTimeout);
        }
        
        // Set a timeout to detect when user finishes scrolling
        userScrollTimeout = window.setTimeout(() => {
            if (isScrolledToBottom()) {
                // Re-enable auto-scroll only if they scrolled to the bottom
                autoScroll = true;
                
                // Reset to limit log display when auto-scroll is re-enabled
                if (autoScroll && $logStore.length > MAX_VISIBLE_LOGS) {
                    showAllLogs = false;
                    isLoadingAllLogs = false;
                }
            }
            isUserScrolling = false;
        }, 300);
    }

    // Optimized scroll position check
    function isScrolledToBottom(tolerance = 50): boolean {
        if (!scrollContainer) return true;
        
        // Use cached values for better performance
        const { scrollTop, scrollHeight, clientHeight } = scrollContainer;
        return scrollHeight - scrollTop - clientHeight <= tolerance;
    }

    // Optimized scroll to bottom with better performance characteristics
    function forceScrollToBottom() {
        if (!scrollContainer || !autoScroll || isUserScrolling) return;
        
        // Cancel any existing scroll operation
        if (scrollTimer !== null) {
            clearTimeout(scrollTimer);
        }
        
        // Use requestAnimationFrame for better performance
        scrollTimer = window.setTimeout(() => {
            // Use virtual scrolling technique for better performance
            if (scrollContainer && autoScroll) {
                // Use scrollIntoView for smoother scrolling
                const lastChild = scrollContainer.lastElementChild;
                if (lastChild) {
                    lastChild.scrollIntoView({ behavior: 'auto' });
                } else {
                    // Fallback to direct scrollTop if no elements
                    scrollContainer.scrollTop = scrollContainer.scrollHeight;
                }
                
                // Clear timer after scrolling is complete
                setTimeout(() => {
                    scrollTimer = null;
                }, 50);
            }
        }, 10);
    }

    // Function to toggle auto-scroll
    function toggleAutoScroll(value: boolean) {
        autoScroll = value;
        
        // When enabling auto-scroll
        if (autoScroll) {
            // Reset log view to truncated mode when enabling auto-scroll
            if ($logStore.length > MAX_VISIBLE_LOGS) {
                showAllLogs = false;
            }
            
            // Force scroll to bottom when enabling auto-scroll
            forceScrollToBottom();
        }
    }

    // Setup observers and listeners on mount
    onMount(() => {
        // Initial scroll
        if (autoScroll) {
            setTimeout(forceScrollToBottom, 100);
        }
        
        // Set up a mutation observer to detect when logs are added to the DOM
        if (scrollContainer) {
            mutationObserver = new MutationObserver((mutations) => {
                let logAdded = false;
                
                for (const mutation of mutations) {
                    if (mutation.type === 'childList' && mutation.addedNodes.length > 0) {
                        logAdded = true;
                        break;
                    }
                }
                
                if (logAdded && autoScroll) {
                    forceScrollToBottom();
                }
            });
            
            mutationObserver.observe(scrollContainer, { 
                childList: true, 
                subtree: true 
            });
        }
        
        // Additional interval to periodically check and force scroll if needed
        const scrollCheckInterval = setInterval(() => {
            if (autoScroll && !isUserScrolling && !isScrolledToBottom()) {
                forceScrollToBottom();
            }
        }, 1000);
        
        return () => {
            clearInterval(scrollCheckInterval);
            if (mutationObserver) {
                mutationObserver.disconnect();
            }
        };
    });

    onDestroy(() => {
        if (scrollTimer !== null) {
            clearTimeout(scrollTimer);
        }
        clearTimeout(userScrollTimeout);
        if (mutationObserver) {
            mutationObserver.disconnect();
        }
    });
    
    // Also force scroll when log store updates
    $: {
        if (filteredLogs.length > 0 && autoScroll) {
            forceScrollToBottom();
        }
    }
</script>

<!-- Main container for the log viewer -->
<div class="flex flex-col h-full bg-[#1e1e1e] text-white font-[DM_Mono] text-[11px]">
    <!-- Top controls row -->
    <div class="px-3 py-2 border-b border-gray-800 bg-[#252525] h-10 flex items-center justify-between">
        <div class="flex items-center gap-6">
            <!-- Log level filter -->
            <div class="flex items-center gap-2 whitespace-nowrap">
                <span class="text-xs uppercase tracking-wider font-medium text-gray-400">
                    Log Level:
                </span>
                <select
                    bind:value={selectedLogLevel}
                    class="w-20 h-7 bg-[#333] text-white text-[11px] font-medium uppercase tracking-wider
                           border-none rounded px-2 py-1.5
                           focus:ring-1 focus:ring-primary outline-none
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
                class="flex items-center gap-1 px-3 py-1 bg-[#333] h-7 text-gray-400 rounded whitespace-nowrap flex-shrink-0 text-[11px] uppercase tracking-wider hover:bg-[#444] hover:text-white transition-colors"
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
            class="px-3 py-1 h-7 bg-[#333] text-gray-400 rounded whitespace-nowrap flex-shrink-0 text-[11px] uppercase tracking-wider hover:bg-[#444] hover:text-white transition-colors"
        >
            Clear
        </button>
        </div>
    </div>
    
    <!-- Content area: logs and optional progress bar -->
    <div class="relative flex flex-col flex-1 min-h-0">
        <!-- The scrollable container for log entries -->
        <div 
            class="flex-1 overflow-y-auto min-h-0"
            bind:this={scrollContainer}
            on:scroll={handleUserScroll}
        >
            {#if filteredLogs.length === 0}
                <div class="absolute top-0 left-0 w-full h-full flex items-center justify-center text-gray-600 italic text-sm">
                    No logs to display
                </div>
            {:else}
                <!-- Show loading spinner when loading all logs -->
                {#if isLoadingAllLogs}
                    <div class="py-2 px-3 text-primary text-center text-xs bg-primary/10 border-b border-[#2a2a2a] flex items-center justify-center gap-2">
                        <div class="spinner w-4 h-4 border-2 border-primary/30 border-t-primary rounded-full animate-spin"></div>
                        <span>Loading all {$logStore.length} logs...</span>
                    </div>
                <!-- Show truncation message if we're limiting the number of logs -->
                {:else if $logStore.length > MAX_VISIBLE_LOGS && !showAllLogs}
                    <div class="py-1 px-3 text-yellow-500 text-center text-xs bg-yellow-500/10 border-b border-[#2a2a2a] flex items-center justify-center gap-2">
                        <span>Showing only the most recent {MAX_VISIBLE_LOGS} logs of {$logStore.length} total.</span>
                        <button 
                            class="underline hover:text-yellow-300 transition-colors"
                            on:click={showAllLogHistory}
                            aria-label="Show all logs"
                        >
                            Show all logs
                        </button>
                    </div>
                {/if}
                
                <!-- Use keyed each block for better performance -->
                {#each filteredLogs as log, i (i)}
                    <!-- Use contain property to isolate rendering -->
                    <div 
                    class="{log.behavior ? behaviorColors[log.behavior] : 'text-[#c1c1c1]'}
                    py-1 px-3 border-b border-[#2a2a2a] whitespace-pre-wrap break-words leading-snug
                    flex items-baseline justify-start text-left w-full hover:bg-[rgba(255,255,255,0.02)]"
                    style="contain: content;"
                    >
                        <!-- Timestamp -->
                        <span class="text-[#888] mr-2 text-xs flex-shrink-0">
                            {log.time}
                        </span>
                        
                        <!-- Log level with optimized class binding -->
                        <span class={"font-bold text-sm mr-2 flex-shrink-0 min-w-[40px] " + getLevelClass(log.level)}>
                            {log.level}
                        </span>
                        
                        <!-- Message with optimized rendering of structured fields -->
                        <span class="flex-grow text-sm text-left overflow-x-auto">
                            <!-- Show message if it exists -->
                            {#if log.message}
                                {log.message}
                            {/if}
                            
                            <!-- Always show structured fields if they exist, regardless of message -->
                            {#if formatStructuredFields(log)}
                                <span class="{log.message ? 'ml-2' : ''} text-[#666] text-[12px] font-[DM_Mono]">
                                    {formatStructuredFields(log)}
                                </span>
                            {/if}
                        </span>
                    </div>
                {/each}
            {/if}
        </div>
    </div>
</div>

<!--{#if import.meta.env.DEV}
    <div class="debug-overlay">
        Total logs: {logs.length}<br>
        Filtered logs: {filteredLogs.length}<br>
        Selected level: {selectedLogLevel}<br>
        Log levels present: {[...new Set(logs.map(l => l.level))].join(', ')}
    </div>
{/if}-->

<style>
    @keyframes spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
    }
    
    .animate-spin {
        animation: spin 1s linear infinite;
    }
</style>
