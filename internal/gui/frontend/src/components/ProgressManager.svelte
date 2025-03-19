<script lang="ts">
    import { onMount } from 'svelte';
    import { slide } from 'svelte/transition';
    import { quintOut } from 'svelte/easing';
    import { progressBars, removeProgressBar, type ProgressBarData } from '../lib/progressBarsStore';
    import { logStore, type LogMessage } from '../lib/logStore';

    // If user wants to collapse the bar list
    let isCollapsed: boolean = false;
    
    // Track errors by task ID
    let taskErrors = new Map<string, string>();
    let abortedTasksCount = 0;
    let isGlobalAbort = false;
    
    // Process status text
    let statusText = "Processing Status";
    
    // Track if user is active (for auto-removal of larger bars)
    let userActive = false;
    let userActivityTimer: ReturnType<typeof setTimeout> | null = null;
    
    // Application processing state
    export let isProcessing = false;
    
    // Only reset counters and status when all progress bars are removed, not when processing stops
    // This ensures error messages remain visible until bars are cleared
    $: if ($progressBars.length === 0) {
        setTimeout(() => {
            abortedTasksCount = 0;
            isGlobalAbort = false;
            taskErrors.clear();
            statusText = isProcessing ? "In progress..." : "Processing Status";
        }, 3000); // Delay reset to ensure user sees the error message
    }

    // Track last processing state to prevent immediate clearing of error states
    let lastProcessingState = false;
    let errorClearTimeout: ReturnType<typeof setTimeout> | null = null;
    
    // Reset state counters when processing starts
    $: if (isProcessing && !lastProcessingState) {
        // When processing starts after being stopped, reset all error states and counters
        abortedTasksCount = 0;
        isGlobalAbort = false;
        taskErrors = new Map<string, string>();
        statusText = "In progress...";
        lastProcessingState = true;
    } else if (!isProcessing && lastProcessingState) {
        // When processing stops, update the status text based on state
        // But don't clear visual state immediately - let user see the error
        lastProcessingState = false;
        
        if (userCancelled) {
            statusText = "Processing canceled by user";
        } else if (isGlobalAbort) {
            statusText = "Processing failed following an error";
        } else if (abortedTasksCount > 0) {
            statusText = `Continuing with errors (${abortedTasksCount} ${abortedTasksCount === 1 ? 'task' : 'tasks'})`;
        } else {
            statusText = "Processing complete";
        }
        
        // If user becomes active and then inactive again, set a timeout to clear the error state
        // This ensures user sees the error message before it gets cleared
        if (errorClearTimeout) {
            clearTimeout(errorClearTimeout);
        }
    }
    
    // Only set the auto-clear if the user becomes active first
    $: if (userActive && (!isProcessing)) {
        errorClearTimeout = setTimeout(() => {
            if (!isProcessing) {
                statusText = "Processing Status";
            }
        }, 5000); // Auto-clear after 5 seconds of user inactivity
    }

    // Monitor user cancellation globally across all bars
    $: userCancelled = $progressBars.some(bar => bar.errorState === 'user_cancel');

    // Check for large bars that need cleanup when user becomes active
    $: if (userActive) {
        checkForCompletedLargeBars();
    }
    
    // Function to handle cleanup of completed large bars
    function checkForCompletedLargeBars() {
        // Don't reset flags immediately when processing stops
        // This allows error messages to remain visible for the user
        // The reset will happen after all bars are removed with a timeout (see reactive statement above)
        
        $progressBars.forEach(bar => {
            if (bar.progress >= 100 && !taskErrors.has(bar.id)) {
                const barSize = bar.size || 'h-2.5';
                
                // Parse height size to determine if it's large
                const sizeMatch = barSize.match(/h-([0-9.]+)/);
                const sizeValue = sizeMatch ? parseFloat(sizeMatch[1]) : 2.5;
                const isLargeBar = sizeValue > 3;  // Larger than h-3
                
                if (isLargeBar && userActive) {
                    // Only remove completed large bars if user is active
                    // and wait a bit longer to ensure user sees completion
                    setTimeout(() => removeProgressBar(bar.id), 3000);
                }
            }
        });
    }
    
    // Watch for processing state changes to handle error visibility
    let processingWasActive = false;
    
    // Update processingWasActive whenever isProcessing changes
    $: {
        processingWasActive = isProcessing;
    }
    
    // Automatic removal of fully completed bars (not in error) after 2s
    onMount(() => {
        
        // Set up user activity detection
        const handleUserActivity = () => {
            const wasInactive = !userActive;
            userActive = true;
            
            // Reset timer on each activity
            if (userActivityTimer) {
                clearTimeout(userActivityTimer);
            }
            
            // If user was inactive before, check for large bars to cleanup
            if (wasInactive) {
                checkForCompletedLargeBars();
            }
            
            // Set user as inactive after 3 seconds of no movement
            userActivityTimer = setTimeout(() => {
                userActive = false;
            }, 3000);
        };
        
        // Add event listeners for user activity
        window.addEventListener('mousemove', handleUserActivity);
        window.addEventListener('keydown', handleUserActivity);
        window.addEventListener('mousedown', handleUserActivity);
        window.addEventListener('touchstart', handleUserActivity);
        
        // Track progress bars
        const progressSub = progressBars.subscribe((bars) => {
            for (const bar of bars) {
                if (bar.progress >= 100 && !taskErrors.has(bar.id) && !bar.errorState) {
                    // Only auto-remove small bars (h-3 or smaller), or larger bars if user is active
                    const barSize = bar.size || 'h-2.5';
                    
                    // Parse height size to determine if it's small
                    // h-3 or smaller is considered small, anything larger is considered large
                    const sizeMatch = barSize.match(/h-([0-9.]+)/);
                    const sizeValue = sizeMatch ? parseFloat(sizeMatch[1]) : 2.5;
                    const isSmallBar = sizeValue <= 3;
                    
                    if (isSmallBar && !bar.errorState) {
                        // Remove small bars after a delay whether user is active or not
                        setTimeout(() => removeProgressBar(bar.id), 3000);
                    }
                    // Larger bars will remain until user activity is detected (handled in checkForCompletedLargeBars)
                    // Bars with errors remain visible until manually cleared
                }
            }
        });
        
        // Listen for log messages with behavior fields
        const logSub = logStore.subscribe((logs) => {
            // Only look at the most recent log (last item in the array)
            if (logs.length > 0) {
                const latestLog = logs[logs.length - 1];
                
                if (latestLog.behavior) {
                    handleLogBehavior(latestLog);
                }
            }
        });
        
        return () => {
            progressSub();
            logSub();
            
            // Clean up event listeners
            window.removeEventListener('mousemove', handleUserActivity);
            window.removeEventListener('keydown', handleUserActivity);
            window.removeEventListener('mousedown', handleUserActivity);
            window.removeEventListener('touchstart', handleUserActivity);
            
            if (userActivityTimer) {
                clearTimeout(userActivityTimer);
            }
        };
    });
    
    function handleLogBehavior(log: LogMessage) {
        const taskId = log.task_id || '';
        const isUserCancelled = log.message && log.message.toLowerCase().includes("canceled");
        
        console.log(`Processing log behavior: ${log.behavior}, taskId: ${taskId}, message: ${log.message}`);
        console.log(`Current progress bars:`, $progressBars);
        
        // Check for user cancellation FIRST, regardless of the behavior type
        if (isUserCancelled) {
            console.log(`User cancellation detected`);
            // This is a user cancellation, not an error
            $progressBars.forEach(bar => {
                updateErrorStateForTask(bar.id, 'user_cancel');
            });
            
            // Update status text for cancellation
            statusText = "Processing canceled by user";
            return; // Skip other error processing
        }
        
        // Normal error handling continues if not a cancellation
        if (log.behavior === 'abort_task') {
            console.log(`ABORT_TASK behavior detected! Message: ${log.message}, taskId: ${taskId}`);
            
            // Use a fallback task ID if none is provided
            const targetTaskId = taskId || 'global-task';
            console.log(`Using targetTaskId: ${targetTaskId}`);
            
            // Mark this task as errored
            taskErrors.set(targetTaskId, log.message);
            abortedTasksCount++;
            console.log(`Updated abortedTasksCount to ${abortedTasksCount}`);
            
            // Force update all progress bars to make the error more visible
            if ($progressBars.length > 0) {
                console.log(`Setting error state on all progress bars`);
                $progressBars.forEach(bar => {
                    updateErrorStateForTask(bar.id, 'abort_task');
                });
            } else {
                console.log(`No progress bars found to update!`);
                // Just update the specific task
                updateErrorStateForTask(targetTaskId, 'abort_task');
            }
            
            // Update status text with warning color in the requested format
            statusText = `Continuing with errors (${abortedTasksCount} ${abortedTasksCount === 1 ? 'task' : 'tasks'})`;
            console.log(`Set status text to: ${statusText}`);
        } 
        else if (log.behavior === 'abort_all') {
            isGlobalAbort = true;
            
            // Update all progress bars with error state
            $progressBars.forEach(bar => {
                updateErrorStateForTask(bar.id, 'abort_all');
            });
            
            // Update status text
            statusText = "Processing aborted due to critical error";
        }
    }
    
    function updateErrorStateForTask(taskId: string, behavior: string) {
        // Map internal behavior names to errorState values
        const errorStateMap = {
            'abort_task': 'error_task',
            'abort_all': 'error_all',
            'user_cancel': 'user_cancel'
        };
        
        // Debug logging to help troubleshoot
        console.log(`Updating error state for task: ${taskId}, behavior: ${behavior}`);
        
        progressBars.update(bars => {
            // First, log the bars we're working with
            console.log(`Current progress bars before update: ${bars.length}`);
            
            // If it's a task abortion, make sure we update the UI
            if (behavior === 'abort_task') {
                // Update the status text immediately to ensure it's displayed
                statusText = `Continuing with errors (${abortedTasksCount} ${abortedTasksCount === 1 ? 'task' : 'tasks'})`;
            }
            
            // Create a modified array
            const updatedBars = bars.map(bar => {
                // For abort_task, let's update all bars or the specific one
                // For abort_all and user_cancel, update all bars
                if (bar.id === taskId || behavior === 'abort_all' || behavior === 'user_cancel' || behavior === 'abort_task') {
                    // Set the error state using the mapped value
                    const newErrorState = errorStateMap[behavior] || behavior;
                    console.log(`Updating bar ${bar.id} with error state: ${newErrorState}`);
                    
                    // For debugging, let's examine the bar before and after update
                    console.log(`Bar before update:`, bar);
                    
                    const updatedBar = { 
                        ...bar, 
                        errorState: newErrorState 
                    };
                    
                    console.log(`Bar after update:`, updatedBar);
                    return updatedBar;
                }
                return bar;
            });
            
            // Do a final check for the updated bars
            console.log(`Updated bars:`, updatedBars);
            
            // Return the updated array
            return updatedBars;
        });
    }

    function toggleCollapse() {
        isCollapsed = !isCollapsed;
    }

    // Sort in descending order of priority
    // (highest priority => top of list, lowest => bottom)
    $: sortedBars = $progressBars
        .slice()
        .sort((a, b) => (b.priority ?? 50) - (a.priority ?? 50));
    
    // Clear a specific progress bar
    function clearBar(barId: string) {
        removeProgressBar(barId);
    }
    
    // Helper function to get gradient position based on progress
    function getGradientPosition(progress) {
        return progress <= 0 ? 0 : (progress >= 100 ? 100 : progress);
    }
</script>

<!-- Only show if there's at least one bar. -->
{#if $progressBars.length > 0}
    <div class="flex flex-col max-w-2xl mx-auto w-full text-white rounded-lg p-3 
                transition-all duration-200 ease-out">

        <!-- A minimal top row with a label & action buttons -->
        <div class="flex items-center justify-between">
            <span class="font-bold text-base" class:text-gray-300={!isGlobalAbort && abortedTasksCount === 0} 
                                              class:text-error-task={!isGlobalAbort && abortedTasksCount > 0 && !userCancelled}
                                              class:text-error-all={isGlobalAbort && !userCancelled}
                                              class:text-user-cancel={userCancelled}>
                {statusText}
            </span>
            <div class="flex items-center gap-2">
                <!-- Collapse/Expand toggle button -->
                <button 
                    class="flex items-center justify-center w-6 h-6 
                           text-primary/70 hover:text-primary
                           transition-all duration-200 relative
                           overflow-hidden rounded-full
                           active:bg-primary/20
                           focus:outline-none focus:ring-1 focus:ring-primary/40"
                    on:click={toggleCollapse}
                    aria-label="{isCollapsed ? 'Expand' : 'Collapse'} progress bars"
                >
                    <span class="material-icons text-[16px] transform transition-transform duration-300"
                          class:rotate-180={isCollapsed}>
                        expand_less
                    </span>
                </button>
                
                <!-- Clear all progress bars button -->
                <button 
                    class="flex items-center justify-center w-6 h-6
                           text-secondary/70 hover:text-secondary
                           transition-all duration-200 relative
                           overflow-hidden rounded-full
                           active:bg-secondary/20
                           focus:outline-none focus:ring-1 focus:ring-secondary/40"
                    on:click={() => {
                        $progressBars.forEach(bar => removeProgressBar(bar.id));
                    }}
                    aria-label="Clear all progress bars"
                >
                    <span class="material-icons text-[16px]">close</span>
                </button>
            </div>
        </div>

        <!-- Drawer-like animation for progress bars -->
        {#if !isCollapsed}
            <div 
                transition:slide={{ duration: 200, easing: quintOut }} 
                class="space-y-2 mt-2"
            >
                {#each sortedBars as bar (bar.id)}
                    <!-- Single bar row with subtle styling matching MediaInput -->
                    <div 
                        class="flex flex-col gap-1 p-2 rounded-md
                               transition-all duration-300 ease-in-out 
                               bg-ui-element hover:bg-ui-element-hover"
                    >
                        <div class="flex items-center justify-between text-sm text-white/90">
                            <span class="truncate max-w-[80%] font-medium">
                                {bar.operation}{#if bar.description} - {bar.description}{/if}
                            </span>
                            <span class="text-primary/80 text-xs whitespace-nowrap">
                                ({bar.current}/{bar.total}) {Math.round(bar.progress)}%
                            </span>
                        </div>
                        <div class="relative w-full bg-black/20 rounded-full overflow-hidden {bar.size || 'h-2.5'}">
                            <!-- Optimized progress bar rendering -->
                            {#if !bar.errorState}
                                <div
                                    class="absolute inset-0 progress-gradient"
                                    style="width: {bar.progress}%; --gradient-position: {getGradientPosition(bar.progress)}%;"
                                />
                            {:else}
                                <!-- Error state coloring with !important to ensure it's applied -->
                                <div
                                    class="absolute inset-0 transition-colors duration-1000 ease-in-out {bar.errorState}"
                                    style="width: {bar.progress}%;"
                                />
                            {/if}
                            
                            <!-- Only render sweeping gradient animation when bar is active and visible -->
                            {#if bar.progress < 100 && !bar.errorState && bar.progress > 0}
                                <div class="absolute h-full w-full overflow-hidden will-change-transform">
                                    <!-- Main progress fill clipping container with contain property -->
                                    <div class="absolute inset-0 overflow-hidden" 
                                        style="width: {bar.progress}%; contain: strict;">
                                        <!-- Fixed-width gradient container with composite layers-->
                                        <div class="absolute inset-0" style="width: 500px; will-change: transform; contain: paint;">
                                            <!-- Optimize gradient animation -->
                                            <div id="gradient-{bar.id}" 
                                                class="animate-sweep-gradient absolute inset-0 w-full h-full bg-sweep-gradient" 
                                                style="opacity: var(--sweep-opacity, 0.5); will-change: transform;">
                                            </div>
                                        </div>
                                    </div>
                                    <!-- Only show edge glow if progress is significant -->
                                    {#if bar.progress > 5}
                                        <div class="absolute top-0 bottom-0 w-[1px] shadow-progress-edge" 
                                            style="left: {bar.progress}%">
                                        </div>
                                    {/if}
                                </div>
                            {/if}
                        </div>
                    </div>
                {/each}
            </div>
        {/if}
    </div>
{/if}

<style>
    /* Progress bar styles */
    
    /* Dynamic gradient for progress bars */
    .progress-gradient {
        background: linear-gradient(to right, 
            hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 1), 
            hsla(var(--secondary-hue), var(--secondary-saturation), var(--secondary-lightness), 1));
        background-size: 200% 100%;
        background-position: calc(100% - var(--gradient-position, 0%)) 0;
        transition: background-position 0.4s ease, width 0.4s ease;
        box-shadow: 0 0 8px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.6);
    }
    
    /* Error state styles with distinct class names matching errorState values */
    .error_task { 
        background-color: theme('colors.error-task') !important; /* Using orange color for task errors */
        animation: fadeToOrange 1.5s ease-in-out forwards;
    }
    .error_all { 
        background-color: theme('colors.error-all') !important; /* Using red color for critical errors */
        animation: fadeToRed 1.5s ease-in-out forwards;
    }
    .user_cancel { 
        background-color: theme('colors.user-cancel') !important; /* Using gray for user cancellations */
        animation: fadeToGray 1.5s ease-in-out forwards;
    }
    
    @keyframes fadeToOrange {
        0% { background-color: theme('colors.primary.DEFAULT'); }
        100% { background-color: theme('colors.error-task'); }
    }
    
    @keyframes fadeToRed {
        0% { background-color: theme('colors.primary.DEFAULT'); }
        100% { background-color: theme('colors.error-all'); }
    }
    
    @keyframes fadeToGray {
        0% { background-color: theme('colors.primary.DEFAULT'); }
        100% { background-color: theme('colors.user-cancel'); }
    }
</style>