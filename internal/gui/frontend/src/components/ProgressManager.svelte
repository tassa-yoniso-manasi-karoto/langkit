<script lang="ts">
    import { onMount } from 'svelte';
    import { slide } from 'svelte/transition';
    import { quintOut } from 'svelte/easing';
    import { progressBars, removeProgressBar, type ProgressBarData } from '../lib/progressBarsStore';
    import { logStore, type LogMessage } from '../lib/logStore';

    // If user wants to collapse the bar list
    let isCollapsed: boolean = false;
    
    // Track errors by task ID
    let taskErrors: Map<string, string> = new Map();
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
        taskErrors = new Map();
        statusText = "In progress...";
        lastProcessingState = true;
    } else if (!isProcessing && lastProcessingState) {
        // When processing stops, update the status text based on state
        // But don't clear visual state immediately - let user see the error
        lastProcessingState = false;
        
        if (userCancelled) {
            statusText = "Processing canceled by user";
        } else if (isGlobalAbort) {
            statusText = "Processing failed due to an error";
        } else if (abortedTasksCount > 0) {
            statusText = `Partially completed (${abortedTasksCount} media processing ${abortedTasksCount === 1 ? 'task' : 'tasks'} aborted)`;
        } else {
            statusText = "Processing complete";
        }
    }
    
    // Monitor user cancellation globally across all bars
    $: userCancelled = $progressBars.some(bar => bar.errorState === 'user_cancel');

    // Determine if status text should have the wave effect (exclude user_cancel)
    $: statusHasWaves = (!isGlobalAbort && abortedTasksCount > 0 && !userCancelled) || (isGlobalAbort && !userCancelled);

    // Determine state class for status text container
     $: statusStateClass = statusHasWaves
        ? isGlobalAbort ? 'state-error-all' : 'state-error-task'
        : userCancelled ? 'state-user-cancel' // Apply cancel state for static gradient
        : 'state-normal'; // Default or normal state

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
                
                if (latestLog.behavior && !userCancelled) {
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
        } as const; // Use 'as const' for stricter typing of keys
        
        // Type guard to check if behavior is a valid key
        function isValidErrorKey(key: string): key is keyof typeof errorStateMap {
            return key in errorStateMap;
        }

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
                    // Set the error state using the mapped value *only if behavior is a valid key*
                    const newErrorState = isValidErrorKey(behavior) ? errorStateMap[behavior] : behavior; // Fallback to behavior itself if key invalid
                    
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
    
    // Function to clear all progress bars
    function clearAllBars() {
        // Use the store directly to trigger updates
        progressBars.update(bars => {
            bars.forEach(bar => removeProgressBar(bar.id)); // Call remove for each
            return []; // Return empty array to clear the store state
        });
        // Reset counters immediately as well
        abortedTasksCount = 0;
        isGlobalAbort = false;
        taskErrors.clear();
        statusText = isProcessing ? "In progress..." : "Processing Status";
    }
</script>

<!-- Only show if there's at least one bar. -->
{#if $progressBars.length > 0}
    <div class="flex flex-col max-w-2xl mx-auto w-full text-white rounded-lg p-3 
                transition-all duration-200 ease-out">

        <!-- A minimal top row with a label & action buttons -->
        <div class="flex items-center justify-between">
            <!-- Status Text Container -->
            
            <div class="status-text-container {statusStateClass}">
                {#if statusHasWaves}
                     <!-- SVG containing waves clipped by text -->
                     <svg class="status-svg"
                          viewBox="0 0 175 20"
                          preserveAspectRatio="xMidYMid meet"
                          xmlns="http://www.w3.org/2000/svg"
                          xmlns:xlink="http://www.w3.org/1999/xlink">
                         <defs>
                             <!-- Wave Path (Reduced wave height: -3 instead of -7) -->
                             <path id="gentle-wave-status" d="M-160 12 c30 0 58-3 88-3s 58 3 88 3 58-3 88-3 58 3 88 3 v14 h-352z" />
                             <!-- Clip Path using Text. IMPORTANT must redefine clipPath whenever statusText changes -->
                             {#key statusText}
                                 <clipPath id="status-text-clip">
                                     <!-- Text acts as the clipping shape -->
                                     <!-- Use x=0 for left alignment, adjust y for new viewBox -->
                                     <text x="0" y="15"
                                           dominant-baseline="middle"
                                           class="status-text-svg" style="font-weight: bold; font-size: 1rem;"> <!-- Restore inline style -->
                                         {statusText}
                                     </text>
                                 </clipPath>
                             {/key}
                         </defs>
                         <!-- Animated Waves (Clipped by Text) -->
                         <g class="parallax-progress" style="filter: blur(1.7px);" clip-path="url(#status-text-clip)">
                              <!-- Background rectangle inside the clip path -->
                              <!-- Make rect much wider than the new viewBox (175) -->
                              <rect x="-100" y="0" width="1500" height="1500" fill="var(--progress-bg-color)" />
                              <!-- Use status-specific wave fills -->
                              <!-- Increased y offset slightly to lower waves -->
                              <use xlink:href="#gentle-wave-status" x="48" y="2" fill="var(--status-wave-1-fill)" />
                              <use xlink:href="#gentle-wave-status" x="48" y="5" fill="var(--status-wave-2-fill)" />
                              <use xlink:href="#gentle-wave-status" x="48" y="7" fill="var(--status-wave-3-fill)" />
                              <use xlink:href="#gentle-wave-status" x="48" y="9" fill="var(--status-wave-4-fill)" />
                         </g>
                     </svg>
                {:else}
                     <!-- Fallback simple span for normal/cancel states -->
                     <span class="font-bold text-base {userCancelled ? 'gradient-text-cancel' : ''}">
                         {statusText}
                     </span>
                {/if}
            </div>

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
                    on:click={clearAllBars}
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
                    {@const hasError = !!bar.errorState}
                    {@const isComplete = bar.progress >= 100 && !hasError}
                    {@const stateClass = hasError 
                        ? bar.errorState === 'error_task' ? 'state-error-task' 
                        : bar.errorState === 'error_all' ? 'state-error-all' 
                        : bar.errorState === 'user_cancel' ? '' 
                        : 'state-normal' 
                        : isComplete ? 'state-complete' : 'state-normal'}
                    {@const showWaves = bar.errorState !== 'user_cancel'}

                    <!-- Single bar row -->
                    <div 
                        class="flex flex-col gap-1 p-2 rounded-md
                               transition-all duration-300 ease-in-out"
                    >
                        <div class="flex items-center justify-between text-sm text-white/90">
                            <span class="truncate max-w-[80%] font-medium">
                                {bar.operation}{#if bar.description} - {bar.description}{/if}
                            </span>
                            <span class="text-primary/80 text-xs whitespace-nowrap">
                                {#if bar.total}({bar.current}/{bar.total}) {/if}{Math.round(bar.progress)}%
                            </span>
                        </div>
                        <div class="relative w-full bg-black/20 rounded-full overflow-hidden {bar.size || 'h-2.5'}">
                            <!-- Progress bar fill -->
                            <div
                                class="progress-bar-fill absolute inset-0 rounded-full transition-all duration-300 {stateClass}"
                                style="width: {bar.progress}%;"
                            >
                                {#if showWaves}
                                    <!-- Sweep animation -->
                                    {#if isProcessing}
                                        <div id="gradient-{bar.id}" 
                                            class="animate-sweep-gradient absolute inset-0 w-full h-full" 
                                            style="opacity: var(--sweep-opacity, 0.5);">
                                        </div>
                                    {/if}
                                    <!-- Layered animated waves SVG -->
                                    <div class="waves-container" style="filter: blur(1.7px);">
                                        <svg class="waves-svg" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"
                                             viewBox="0 0 150 10" preserveAspectRatio="none" shape-rendering="auto">
                                            <defs>
                                                <!-- Adjusted path for ~10px height -->
                                                <path id="gentle-wave-progress" d="M-160 6 c30 0 58-4 88-4s 58 4 88 4 58-4 88-4 58 4 88 4 v10 h-352z" />
                                            </defs>
                                            <g class="parallax-progress">
                                                <use xlink:href="#gentle-wave-progress" x="48" y="0" fill="var(--wave-1-fill)" />
                                                <use xlink:href="#gentle-wave-progress" x="48" y="3" fill="var(--wave-2-fill)" />
                                                <use xlink:href="#gentle-wave-progress" x="48" y="5" fill="var(--wave-3-fill)" />
                                                <use xlink:href="#gentle-wave-progress" x="48" y="7" fill="var(--wave-4-fill)" />
                                            </g>
                                        </svg>
                                    </div>
                                {:else if bar.errorState === 'user_cancel'}
                                    <div class="absolute inset-0 layer-user-cancel animate-fade-in"></div>
                                {/if}
                                <!-- Edge glow (optional, might interfere visually) -->
                                {#if bar.progress > 5 && !hasError}
                                    <div class="absolute top-0 bottom-0 w-[1px] shadow-progress-edge" 
                                         style="right: 0">
                                    </div>
                                {/if}
                            </div>
                        </div>
                    </div>
                {/each}
            </div>
        {/if}
    </div>
{/if}


<style>
    .status-text-container {
        /* Ensure container has some height and allows SVG to align */
        min-height: 1.5em; /* Match approx text line height */
        display: inline-block; /* Allow SVG to size correctly */
        line-height: 1; /* Prevent extra space below SVG */
        /* Inherits state variables from statusStateClass */
        /* Ensure it aligns left in the flex container */
        margin-right: auto;
    }
    .status-svg {
        width: auto; /* Let SVG size based on text content */
        max-width: 100%; /* Prevent overflow if text is very long */
        height: 1.5em; /* Adjust height based on font size */
        vertical-align: middle; /* Align SVG nicely with buttons */
        overflow: visible; /* Allow text clipping to work */
    }
    .status-text-svg {
        /* Inherit font styles from parent */
        font-size: inherit;
        font-weight: inherit;
        /* Fill determines color *if* not clipped, but needed for shape */
        fill: white;
        /* Add stroke for potentially crisper edges (suggestion 7) */
        stroke: rgba(0,0,0,0.01); /* Almost transparent stroke */
        stroke-width: 0.1;
    }
    
    .gradient-text-cancel {
        position: relative;
        color: transparent;
        background: var(--user-cancel-gradient, linear-gradient(to right, hsl(220, 15%, 40%), hsl(210, 20%, 50%)));
        -webkit-background-clip: text;
        background-clip: text;
        transition: background var(--error-transition-duration, 1.5s) ease-in-out,
                    color var(--error-transition-duration, 1.5s) ease-in-out;

        /* Fallback for browsers that don't support background-clip: text */
        @supports not (background-clip: text) {
            color: theme('colors.user-cancel');
            transition: color var(--error-transition-duration, 1.5s) ease-in-out;
        }
    }

    .progress-bar-fill {
        background-color: var(--progress-bg-color); /* Base background */
        overflow: hidden; /* Important to clip the waves */
    }

    .waves-container {
        position: absolute;
        left: 0;
        right: 0;
        bottom: 0; /* Anchor waves to the bottom */
        height: 100%; /* Make container fill the bar height */
        pointer-events: none;
    }

    .waves-svg {
        position: absolute;
        left: 0;
        bottom: 0;
        width: 100%;
        height: 100%; /* SVG height relative to container */
    }
    
    /* Progress bar animations */
    @keyframes sweep {
        0% { 
            transform: translateX(-100%);
            animation-timing-function: cubic-bezier(0.45, 0.3, 0.45, 0.7); /* Ease-in-out */
        }
        50% { 
            transform: translateX(-50%);
            animation-timing-function: cubic-bezier(0.4, 0, 0.6, 0.8); /* Accelerating */
        }
        100% { 
            transform: translateX(100%);
        }
    }

    /* State-specific Color Themes (Inverted Wave Colors) */
    .state-normal {
        /* Darker background using primary color */
        --progress-bg-color: hsl(var(--primary-hue), var(--primary-saturation), calc(var(--primary-lightness) - 10%)); /* Slightly darker background */
        /* Waves use primary/secondary colors with increased opacity */
        --wave-1-fill: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.5); /* Top wave */
        --wave-2-fill: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.7);
        --wave-3-fill: hsla(var(--secondary-hue), var(--secondary-saturation), var(--secondary-lightness), 0.8); /* Introduce secondary */
        --wave-4-fill: hsla(var(--secondary-hue), var(--secondary-saturation), var(--secondary-lightness), 0.9); /* Bottom wave, almost opaque */
        /* Status text waves (can be same or different - using same for now) */
        --status-wave-1-fill: var(--wave-1-fill);
        --status-wave-2-fill: var(--wave-2-fill);
        --status-wave-3-fill: var(--wave-3-fill);
        --status-wave-4-fill: var(--wave-4-fill);
    }
    .state-complete {
        --progress-bg-color: hsl(var(--completion-hue), var(--completion-saturation), calc(var(--completion-lightness) - 25%));
        --wave-1-fill: hsla(var(--completion-hue), var(--completion-saturation), var(--completion-lightness), 0.5);
        --wave-2-fill: hsla(var(--completion-hue), var(--completion-saturation), var(--completion-lightness), 0.7);
        --wave-3-fill: hsla(var(--completion-hue), var(--completion-saturation), var(--completion-lightness), 0.8);
        --wave-4-fill: hsla(var(--completion-hue), var(--completion-saturation), var(--completion-lightness), 0.9);
        --status-wave-1-fill: var(--wave-1-fill);
        --status-wave-2-fill: var(--wave-2-fill);
        --status-wave-3-fill: var(--wave-3-fill);
        --status-wave-4-fill: var(--wave-4-fill);
    }
    .state-error-task {
        --progress-bg-color: hsl(var(--error-task-hue), var(--error-task-saturation), calc(var(--error-task-lightness) - 35%));
        --wave-1-fill: hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.5);
        --wave-2-fill: hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.7);
        --wave-3-fill: hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.8);
        --wave-4-fill: hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.9);
        --status-wave-1-fill: var(--wave-1-fill);
        --status-wave-2-fill: var(--wave-2-fill);
        --status-wave-3-fill: var(--wave-3-fill);
        --status-wave-4-fill: var(--wave-4-fill);
    }
    .state-error-all {
        --progress-bg-color: hsl(var(--error-all-hue), var(--error-all-saturation), calc(var(--error-all-lightness) - 25%));
        --wave-1-fill: hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.5);
        --wave-2-fill: hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.7);
        --wave-3-fill: hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.8);
        --wave-4-fill: hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.9);
        --status-wave-1-fill: var(--wave-1-fill);
        --status-wave-2-fill: var(--wave-2-fill);
        --status-wave-3-fill: var(--wave-3-fill);
        --status-wave-4-fill: var(--wave-4-fill);
    }
    .layer-user-cancel {
        /* No wave variables wanted, just a static gradient */
        background: var(--user-cancel-gradient, linear-gradient(to right, hsl(220, 15%, 40%), hsl(210, 20%, 50%)));
    }
    
    .animate-fade-in {
        animation: fadeIn var(--error-transition-duration, 1.5s) ease-in-out forwards;
    }

    /* Wave Animation */
    .parallax-progress > use {
      animation: move-forever 25s cubic-bezier(.55,.5,.45,.5) infinite;
      will-change: transform; /* Performance hint */
    }
    .parallax-progress > use:nth-child(1) {
      animation-delay: -2s;
      animation-duration: 7s;
    }
    .parallax-progress > use:nth-child(2) {
      animation-delay: -3s;
      animation-duration: 10s;
    }
    .parallax-progress > use:nth-child(3) {
      animation-delay: -4s;
      animation-duration: 13s;
    }
    .parallax-progress > use:nth-child(4) {
      animation-delay: -5s;
      animation-duration: 20s;
    }

    @keyframes move-forever {
      0% {
       transform: translate3d(-90px,0,0);
      }
      100% { 
        transform: translate3d(85px,0,0);
      }
    }

    /* Reduced motion preferences */
    @media (prefers-reduced-motion) {
      .parallax-progress > use {
        animation: none; /* Disable wave animation */
      }
    }

</style>