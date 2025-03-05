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
    
    // Reset counters and status when all progress bars are removed
    $: if ($progressBars.length === 0) {
        abortedTasksCount = 0;
        isGlobalAbort = false;
        taskErrors.clear();
        statusText = "Processing Status";
    }

    // Monitor user cancellation globally across all bars
    $: userCancelled = $progressBars.some(bar => bar.errorState === 'user_cancel');

    // Automatic removal of fully completed bars (not in error) after 2s
    onMount(() => {
        // Track progress bars
        const progressSub = progressBars.subscribe((bars) => {
            for (const bar of bars) {
                if (bar.progress >= 100 && !taskErrors.has(bar.id)) {
                    setTimeout(() => removeProgressBar(bar.id), 2000);
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
        };
    });
    
    function handleLogBehavior(log: LogMessage) {
        const taskId = log.task_id || '';
        const isUserCancelled = log.message && log.message.toLowerCase().includes("canceled");
        
        // Debug logging
        if (log.behavior) {
            console.log("Log with behavior detected:", log.behavior);
            console.log("Message:", log.message);
            console.log("Contains 'canceled':", isUserCancelled);
        }
        
        // Check for user cancellation FIRST, regardless of the behavior type
        if (isUserCancelled) {
            console.log("User cancellation detected!");
            
            // This is a user cancellation, not an error
            $progressBars.forEach(bar => {
                updateErrorStateForTask(bar.id, 'user_cancel');
            });
            
            // Update status text for cancellation
            statusText = "Processing canceled by user";
            return; // Skip other error processing
        }
        
        // Normal error handling continues if not a cancellation
        if (log.behavior === 'abort_task' && taskId) {
            // Mark this task as errored
            taskErrors.set(taskId, log.message);
            abortedTasksCount++;
            
            // Update progress bar color for this task
            updateErrorStateForTask(taskId, 'abort_task');
            
            // Update status text
            statusText = `Processing: ${abortedTasksCount} ${abortedTasksCount === 1 ? 'task' : 'tasks'} aborted`;
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
        progressBars.update(bars => {
            return bars.map(bar => {
                if (bar.id === taskId || behavior === 'abort_all' || behavior === 'user_cancel') {
                    // Set the error state directly from the behavior parameter
                    return { ...bar, errorState: behavior };
                }
                return bar;
            });
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
        
    // Determine bar color based on error state
    function getBarColor(bar: ProgressBarData) {
        if (!bar.errorState) return 'bg-primary';
        
        // Use inline style with calculated color instead of gradient
        return 'color-transition';
    }
    
    // Get bar color for a specific state
    function getBarColorByState(errorState: string) {
        switch(errorState) {
            case 'error_task':
                return getComputedStyle(document.documentElement).getPropertyValue('--error-task-color').trim() || '#f97316';
            case 'error_all':
                return getComputedStyle(document.documentElement).getPropertyValue('--error-all-color').trim() || '#ef4444';
            case 'user_cancel':
                return getComputedStyle(document.documentElement).getPropertyValue('--user-cancel-color').trim() || '#6b7280';
            default:
                return getComputedStyle(document.documentElement).getPropertyValue('--primary-color').trim() || '#9f6ef7';
        }
    }
    
    // Animated transition happens directly in the component with CSS animation
    let animationActive = false;
    
    function startColorTransition(bar: ProgressBarData) {
        if (!bar.errorState || bar.transitionComplete) return false;
        
        // Track animation state
        setTimeout(() => {
            progressBars.update(bars => {
                return bars.map(b => {
                    if (b.id === bar.id) {
                        return { ...b, transitionComplete: true };
                    }
                    return b;
                });
            });
        }, 1000); // Match the animation duration
        
        return true;
    }
</script>

<!-- Only show if there's at least one bar. -->
{#if $progressBars.length > 0}
    <div class="flex flex-col max-w-2xl mx-auto w-full bg-[#1a1a1a] text-white border-t border-[#2a2a2a] px-3 py-2 space-y-2 overflow-hidden">

        <!-- A minimal top row with a label & action buttons -->
        <div class="flex items-center justify-between">
            <span class="font-bold text-base" class:text-gray-300={!isGlobalAbort && abortedTasksCount === 0} 
                                              class:text-orange-500={!isGlobalAbort && abortedTasksCount > 0 && !userCancelled}
                                              class:text-red-600={isGlobalAbort && !userCancelled}
                                              class:text-gray-400={userCancelled}>
                {statusText}
            </span>
            <div class="flex items-center gap-2">
                <!-- Collapse/Expand toggle button -->
                <button 
                    class="flex items-center justify-center w-4 h-4
                           text-white/50 hover:text-white/80
                           transition-colors duration-200
                           focus:outline-none"
                    on:click={toggleCollapse}
                    aria-label="{isCollapsed ? 'Expand' : 'Collapse'} progress bars"
                >
                    <span class="material-icons text-[12px]">
                        {isCollapsed ? 'expand_more' : 'expand_less'}
                    </span>
                </button>
                
                <!-- Clear all progress bars button -->
                <button 
                    class="flex items-center justify-center w-4 h-4
                           text-red-400/50 hover:text-red-400
                           transition-colors duration-200
                           focus:outline-none"
                    on:click={() => {
                        $progressBars.forEach(bar => removeProgressBar(bar.id));
                    }}
                    aria-label="Clear all progress bars"
                >
                    <span class="material-icons text-[12px]">close</span>
                </button>
            </div>
        </div>

        <!-- Drawer-like animation for progress bars -->
        {#if !isCollapsed}
            <div 
                transition:slide={{ duration: 200, easing: quintOut }} 
                class="space-y-1"
            >
                {#each sortedBars as bar (bar.id)}
                    <!-- Single bar row -->
                    <div class="flex flex-col gap-0.5 p-1 rounded">
                        <div class="flex items-center justify-between text-sm text-gray-200">
                            <span class="truncate">
                                {bar.operation}{#if bar.description} - {bar.description}{/if}
                            </span>
                            <span>
                                ({bar.current}/{bar.total}) {Math.round(bar.progress)}%
                            </span>
                        </div>
                        <div class="relative w-full bg-[#333] rounded-full overflow-hidden {bar.size || 'h-2.5'}">
                            <div
                                class="absolute inset-0 bg-primary {bar.errorState === 'error_task' ? 'animate-to-error' : ''} 
                                                               {bar.errorState === 'error_all' ? 'animate-to-critical' : ''}
                                                               {bar.errorState === 'user_cancel' ? 'animate-to-cancel' : ''}"
                                style="width: {bar.progress}%;"
                            />
                        </div>
                    </div>
                {/each}
            </div>
        {/if}
    </div>
{/if}
