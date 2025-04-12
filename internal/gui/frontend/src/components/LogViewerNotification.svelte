<!-- LogViewerNotification.svelte -->
<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { fade, fly } from 'svelte/transition';
    import Portal from 'svelte-portal/src/Portal.svelte';
    import { settings } from '../lib/stores';
    import { logStore, type LogMessage } from '../lib/logStore';

    export let processingStartTime: number = 0;
    // Position is passed in from the parent component
    export let position = { x: 0, y: 0 };
    // Mode determines the type of notification to show
    export let mode: 'processing' | 'error' = 'processing';
    // Function to toggle the log viewer
    export let onOpenLogViewer: () => void;
    // Function to call when dismissing the notification
    export let onDismiss: () => void = () => {};

    // Track logs with different error types
    let abortTaskLogs: LogMessage[] = [];
    let abortAllLogs: LogMessage[] = [];
    let errorLevelLogs: LogMessage[] = [];
    let processingActive = false;
    let shouldShowTooltip = false;
    
    // Keep track of the current app start count
    let appStartCount = 0;
    let hasSeenTooltip = false;

    // Calculate if we should show the processing tooltip based on app start count
    $: shouldShowProcessingTooltip = mode === 'processing' && 
        processingActive && 
        appStartCount <= 5 && 
        !hasSeenTooltip;
    
    // Determine the most severe error type
    $: errorType = abortAllLogs.length > 0 ? 'error_all' : 
                   abortTaskLogs.length > 0 ? 'error_task' : 
                   'none';
    
    // Calculate if we should show the error tooltip based on different error types
    // Only show if there are task abortions or error level logs (NOT user cancellations)
    $: shouldShowErrorTooltip = mode === 'error' && 
        (abortTaskLogs.length > 0 || abortAllLogs.length > 0 || errorLevelLogs.length > 0);
    
    // Combined condition to determine if we should show the tooltip
    $: shouldShowTooltip = shouldShowProcessingTooltip || shouldShowErrorTooltip;
    
    // Count total errors - exclude user_cancel behaviors
    $: totalErrorCount = abortTaskLogs.length + abortAllLogs.length + errorLevelLogs.length;

    // Animation duration - longer for errors to draw more attention
    $: animationDuration = mode === 'error' ? 400 : 300;
    
    // Auto-hide timeout for processing tooltip (not for errors)
    let autoHideTimeout: number | null = null;
    
    // Add a visibility variable to control the transition
    let visible = false;

    // Subscribe to settings to get app start count
    const unsubscribeSettings = settings.subscribe(val => {
        appStartCount = val.appStartCount || 0;
        hasSeenTooltip = val.hasSeenLogViewerTooltip || false;
    });
    
    // Subscribe to logs to detect different types of errors
    const unsubscribeLogs = logStore.subscribe(logs => {
        abortTaskLogs = logs.filter(log => 
            log._unix_time >= processingStartTime &&
            log.behavior === 'abort_task' && 
            log.level.toUpperCase() === 'ERROR'
        );
        
        abortAllLogs = logs.filter(log => 
            log._unix_time >= processingStartTime &&
            log.behavior === 'abort_all' && 
            log.level.toUpperCase() === 'ERROR'
        );
        
        // Count ERROR level logs that don't have a user_cancel behavior
        // Also exclude errors with cancellation messages
        errorLevelLogs = logs.filter(log => 
            log._unix_time >= processingStartTime &&
            log.level.toUpperCase() === 'ERROR' && 
            (!log.behavior || log.behavior !== 'user_cancel') &&
            (!log.message || !log.message.toLowerCase().includes('cancel'))
        );
        
        // Check for recent logs to detect active processing
        checkForProcessingLogs(logs);
    });

    // Handle click to open log viewer
    function handleOpenClick() {
        // Mark that the user has seen the tooltip
        if (mode === 'processing' && !hasSeenTooltip) {
            settings.update(s => ({
                ...s,
                hasSeenLogViewerTooltip: true
            }));
        }
        
        // Start fade out animation
        visible = false;
        
        // Wait for animation to complete, then open log viewer
        setTimeout(() => {
            // Hide the tooltip
            shouldShowTooltip = false;
            
            // Open the log viewer
            onOpenLogViewer();
        }, 300);
    }

    // Set processing status based on log activity
    function checkForProcessingLogs(logs: LogMessage[]) {
        // Look for logs within the last 5 seconds that indicate processing
        const now = Date.now();
        const recentLogs = logs.filter(log => {
            // Parse the time from the log (e.g., "14:23:45")
            const logParts = log.time.split(':');
            if (logParts.length !== 3) return false;
            
            const today = new Date();
            const logDate = new Date(
                today.getFullYear(),
                today.getMonth(),
                today.getDate(),
                parseInt(logParts[0]),
                parseInt(logParts[1]),
                parseInt(logParts[2])
            );
            
            // Check if log is within last 5 seconds
            return now - logDate.getTime() < 5000;
        });
        
        // Set processing active if there are recent logs
        processingActive = recentLogs.length > 0;
    }

    onMount(() => {
        // Set visible after a short delay to trigger transition
        setTimeout(() => {
            visible = true;
        }, 50);
        
        // No auto-hide for notifications - user must dismiss them
        // Only fade in the notification
        visible = true;
    });

    onDestroy(() => {
        unsubscribeSettings();
        unsubscribeLogs();
        
        if (autoHideTimeout) {
            clearTimeout(autoHideTimeout);
        }
    });
</script>

{#if shouldShowTooltip}
    <Portal target="body">
        <div
            class="fixed transform -translate-x-1/2 -translate-y-full z-[1000] transition-opacity duration-300 ease-in-out {visible ? 'opacity-100' : 'opacity-0'}"
            style="left: {position.x}px; top: {position.y}px; transform: translate(-50%, -100%) translateY({visible ? '0' : '-10px'});"
        >
            <!-- Dynamic background and border based on error type -->
            <div class="backdrop-blur-md 
                       {errorType === 'error_all' ? 'bg-error-all/10' : 
                         errorType === 'error_task' ? 'bg-error-task/10' : 
                         'bg-primary/20'} 
                       bg-gradient-to-br 
                       {errorType === 'error_all' 
                         ? 'from-hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.15) to-secondary/10' 
                         : errorType === 'error_task'
                           ? 'from-hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.15) to-secondary/10'
                           : 'from-primary/30 to-secondary/20'
                       } 
                       text-white border 
                       {errorType === 'error_all' 
                         ? 'border-error-all/20' 
                         : errorType === 'error_task'
                           ? 'border-error-task/20'
                           : 'border-primary/30'
                       } 
                       rounded-lg p-4 min-w-[280px] max-w-[400px] 
                       transition-all duration-200 font-sans 
                       shadow-lg 
                       {errorType === 'error_all' 
                         ? 'shadow-error-all/15' 
                         : errorType === 'error_task'
                           ? 'shadow-error-task/15'
                           : 'shadow-primary/20'
                       } 
                       cursor-pointer notification-container"
                 on:click={() => {
                     visible = false;
                     setTimeout(() => {
                         shouldShowTooltip = false;
                         onDismiss();
                     }, 300);
                 }}>
                
                <div class="text-sm font-medium mb-3 text-gray-300 flex items-center gap-2">
                    <span class="material-icons text-xl 
                          {errorType === 'error_all' 
                            ? 'text-error-all/70' 
                            : errorType === 'error_task'
                              ? 'text-error-task/70'
                              : 'text-primary'}">
                        {mode === 'processing' ? 'info' : 'warning'}
                    </span>
                    <span>
                        {#if mode === 'processing'}
                            Processing in progress
                        {:else}
                            {#if abortTaskLogs.length > 0 || abortAllLogs.length > 0}
                                Errors occured with cancelation
                            {:else}
                                Errors occured (no task canceled)
                            {/if}
                        {/if}
                    </span>
                </div>
                
                <div class="bg-black/20 backdrop-blur-sm 
                            border 
                            {errorType === 'error_all' 
                              ? 'border-error-all/20' 
                              : errorType === 'error_task'
                                ? 'border-error-task/20'
                                : 'border-primary/30'
                            } 
                            rounded-md p-3 transition-all duration-200 
                            hover:bg-black/30 cursor-pointer"
                     on:click|stopPropagation={handleOpenClick}>
                    <div class="flex items-center gap-2">
                        <span class="material-icons text-[18px] 
                              {errorType === 'error_all' 
                                ? 'text-error-all/80' 
                                : errorType === 'error_task'
                                  ? 'text-error-task/80'
                                  : 'text-primary'}">
                            {mode === 'processing' ? 'info' : 'error'}
                        </span>
                        <span class="text-sm font-medium text-gray-300">
                            {#if mode === 'processing'}
                                Open the Log Viewer to see ongoing processing details
                            {:else}
                                {#if abortAllLogs.length > 0}
                                    <span class="gradient-text-all">
                                        {abortTaskLogs.length !== 1 ? 'Media processing' : 'All media processing task'} aborted following {abortAllLogs.length} critical error{abortAllLogs.length !== 1 ? 's' : ''}
                                    </span>
                                {:else if abortTaskLogs.length > 0}
                                    <span class="gradient-text-task">
                                        {abortTaskLogs.length} media processing task{abortTaskLogs.length !== 1 ? 's' : ''} aborted with {errorLevelLogs.length} error{errorLevelLogs.length !== 1 ? 's' : ''}
                                    </span>
                                {:else}
                                    {errorLevelLogs.length} error{errorLevelLogs.length !== 1 ? 's' : ''} detected during processing
                                {/if}
                            {/if}
                        </span>
                    </div>
                    
                    <div class="mt-2 text-xs flex items-center gap-1 text-gray-400 font-medium">
                        <span class="text-[10px] material-icons 
                              {errorType === 'error_all' 
                                ? 'text-error-all/60' 
                                : errorType === 'error_task'
                                  ? 'text-error-task/60'
                                  : 'text-secondary/80'}">
                            touch_app
                        </span>
                        Click to open Log Viewer
                    </div>
                </div>
                
                <div class="absolute left-1/2 bottom-[-6px] transform -translate-x-1/2 rotate-45 w-3 h-3 
                           {errorType === 'error_all' 
                             ? 'bg-error-all/10' 
                             : errorType === 'error_task'
                               ? 'bg-error-task/10'
                               : 'bg-primary/20'
                           } 
                           backdrop-blur-md 
                           {errorType === 'error_all' 
                             ? 'border-l-error-all/20 border-b-error-all/20' 
                             : errorType === 'error_task'
                               ? 'border-l-error-task/20 border-b-error-task/20'
                               : 'border-l-primary/30 border-b-primary/30'
                           } 
                           border-l border-b"></div>
            </div>
        </div>
    </Portal>
{/if}

<style>
    /* Add a pulsating animation to draw attention */
    @keyframes pulsate {
        0% { transform: scale(1); opacity: 1; }
        50% { transform: scale(1.02); opacity: 0.9; }
        100% { transform: scale(1); opacity: 1; }
    }
    
    @keyframes glow-processing {
        0% { box-shadow: 0 0 5px 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4); }
        50% { box-shadow: 0 0 15px 5px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.6); }
        100% { box-shadow: 0 0 5px 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4); }
    }
    
    @keyframes glow-error-task {
        0% { box-shadow: 0 0 5px 0 hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.2); }
        50% { box-shadow: 0 0 15px 5px hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.3); }
        100% { box-shadow: 0 0 5px 0 hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.2); }
    }
    
    @keyframes glow-error-all {
        0% { box-shadow: 0 0 5px 0 hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.2); }
        50% { box-shadow: 0 0 15px 5px hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.3); }
        100% { box-shadow: 0 0 5px 0 hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.2); }
    }
    
    /* Basic notification container pulsating */
    .notification-container {
        animation: pulsate 3s ease-in-out infinite;
        will-change: transform, opacity;
    }
    
    /* Different glow effect for different notification types */
    :global(.bg-primary\/20.notification-container) {
        animation: pulsate 3s ease-in-out infinite, glow-processing 4s infinite;
    }
    
    :global(.bg-error-task\/10.notification-container) {
        animation: pulsate 2.5s ease-in-out infinite, glow-error-task 3.5s infinite;
    }
    
    :global(.bg-error-all\/10.notification-container) {
        animation: pulsate 2s ease-in-out infinite, glow-error-all 3s infinite;
    }
    
    /* Base gradient text styles with transitions */
    .gradient-text-base {
        position: relative;
        transition: color var(--error-transition-duration, 1.5s) ease-in-out;
    }

    /* Gradient text with transitions - reuse same classes from ProgressManager */
    .gradient-text-task {
        position: relative;
        color: transparent;
        background: var(--error-task-gradient, linear-gradient(to right, hsl(45, 100%, 60%), hsl(30, 100%, 50%)));
        -webkit-background-clip: text;
        background-clip: text;
        transition: background var(--error-transition-duration, 1.5s) ease-in-out,
                    color var(--error-transition-duration, 1.5s) ease-in-out;

        /* Fallback for browsers that don't support background-clip: text */
        @supports not (background-clip: text) {
            color: theme('colors.error-task');
            transition: color var(--error-transition-duration, 1.5s) ease-in-out;
        }
    }

    .gradient-text-all {
        position: relative;
        color: transparent;
        background: var(--error-all-gradient, linear-gradient(to right, hsl(320, 70%, 25%), hsl(335, 85%, 40%)));
        -webkit-background-clip: text;
        background-clip: text;
        transition: background var(--error-transition-duration, 1.5s) ease-in-out,
                    color var(--error-transition-duration, 1.5s) ease-in-out;

        /* Fallback for browsers that don't support background-clip: text */
        @supports not (background-clip: text) {
            color: theme('colors.error-all');
            transition: color var(--error-transition-duration, 1.5s) ease-in-out;
        }
    }
    
    /* Smooth transition on hover */
    .notification-container:hover {
        transform: translateY(-2px);
        box-shadow: 0 8px 20px rgba(0, 0, 0, 0.2);
    }
</style>