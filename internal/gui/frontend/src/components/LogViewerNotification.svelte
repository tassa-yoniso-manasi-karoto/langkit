<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { fade } from 'svelte/transition';
    import Portal from 'svelte-portal/src/Portal.svelte';
    import { settings, statisticsStore, userActivityState } from '../lib/stores';
    import { logStore, type LogMessage } from '../lib/logStore';

    // Component props
    export let processingStartTime: number = 0;
    export let position = { x: 0, y: 0 };
    export let isProcessing: boolean = false;
    export let isVisible: boolean = false;
    export let onOpenLogViewer: () => void;
    export let onDismiss: () => void = () => {};

    // Track previous processingStartTime to detect changes
    let prevProcessingStartTime: number = 0;

    // State for logs
    let allLogs: LogMessage[] = [];
    let abortTaskLogs: LogMessage[] = [];
    let abortAllLogs: LogMessage[] = [];
    let errorLevelLogs: LogMessage[] = [];
    
    // Component state
    let hasSeenTooltip = false;
    let countAppStart = 0;
    let visible = false;
    
    // Subscribe to user activity state
    let currentUserActivityState = 'active';
    const unsubscribeUserActivity = userActivityState.subscribe(value => {
        currentUserActivityState = value.state;
    });

    // Subscribe to settings
    const unsubscribeSettings = settings.subscribe(val => {
        hasSeenTooltip = val.hasSeenLogViewerTooltip || false;
    });
    
    // Subscribe to statistics
    const unsubscribeStatistics = statisticsStore.subscribe(stats => {
        countAppStart = stats?.countAppStart || 0;
    });
    
    // Subscribe to logs
    const unsubscribeLogs = logStore.subscribe(logs => {
        allLogs = logs;
        performFiltering();
    });
    
    // Watch isVisible changes to trigger animation
    $: if (isVisible !== visible) {
        visible = isVisible;
    }
    
    // Filter logs based on processing start time
    function performFiltering() {
        abortTaskLogs = [];
        abortAllLogs = [];
        errorLevelLogs = [];
        
        // Determine time threshold based on processing state
        // For behavior errors, always use processingStartTime when available
        // For plain errors, use recent time window when not processing
        const plainErrorTimeThreshold = (processingStartTime > 0 || isProcessing) 
            ? processingStartTime 
            : Date.now() - (5 * 60 * 1000); // Last 5 minutes when not processing
        
        allLogs.forEach(log => {
            const logTime = log._unix_time || 0;
            
            // For behavior errors (abort_task, abort_all), only count from processingStartTime
            if (processingStartTime > 0 && logTime < processingStartTime) {
                // Skip behavior errors from before processing started
                if (log.behavior === 'abort_task' || log.behavior === 'abort_all') {
                    return;
                }
            }
            
            // For plain errors, use the appropriate time threshold
            if (log.level.toUpperCase() === 'ERROR' && 
                log.behavior !== 'abort_task' && 
                log.behavior !== 'abort_all' &&
                logTime < plainErrorTimeThreshold) {
                return;
            }
            
            if (log.behavior === 'abort_task' && log.level.toUpperCase() === 'ERROR') {
                abortTaskLogs.push(log);
            }
            
            if (log.behavior === 'abort_all' && log.level.toUpperCase() === 'ERROR') {
                abortAllLogs.push(log);
            }
            
            if (log.level.toUpperCase() === 'ERROR' && 
                log.behavior !== 'user_cancel' &&
                (!log.message || !log.message.toLowerCase().includes('cancel'))) {
                errorLevelLogs.push(log);
            }
        });
    }

    // Handle processingStartTime changes
    $: if (processingStartTime !== prevProcessingStartTime) {
        abortTaskLogs = [];
        abortAllLogs = [];
        errorLevelLogs = [];
        prevProcessingStartTime = processingStartTime;
        performFiltering();
    }
    
    // Determine error type based on log counts
    $: errorType = abortAllLogs.length > 0 ? 'error_all' : 
                   abortTaskLogs.length > 0 ? 'error_task' : 
                   'none';
    
    // Determine if we have any errors
    $: hasErrors = totalErrorCount > 0;
    
    // Content selection (not visibility)
    $: shouldShowProcessingTooltip = isProcessing && 
                                    !hasErrors &&
                                    countAppStart <= 5 && 
                                    !hasSeenTooltip;
    
    $: shouldShowErrorTooltip = hasErrors;
    
    // Count total unique errors (avoiding double-counting)
    $: totalErrorCount = (() => {
        const uniqueErrors = new Set<LogMessage>();
        abortTaskLogs.forEach(log => uniqueErrors.add(log));
        abortAllLogs.forEach(log => uniqueErrors.add(log));
        errorLevelLogs.forEach(log => uniqueErrors.add(log));
        return uniqueErrors.size;
    })();
    
    // Determine if we're in a processing context
    $: isInProcessingContext = isProcessing || processingStartTime > 0;

    function handleOpenClick() {
        if (isProcessing && !hasErrors && !hasSeenTooltip) {
            settings.update(s => ({
                ...s,
                hasSeenLogViewerTooltip: true
            }));
        }
        
        visible = false;
        setTimeout(() => {
            onOpenLogViewer();
        }, 300);
    }
    
    function handleDismiss() {
        visible = false;
        setTimeout(() => {
            onDismiss();
        }, 300);
    }

    onMount(() => {
        performFiltering();
    });

    onDestroy(() => {
        unsubscribeSettings();
        unsubscribeStatistics();
        unsubscribeLogs();
        unsubscribeUserActivity();
    });
</script>

{#if isVisible && currentUserActivityState !== 'afk'}
    <Portal target="body">
        <div
            class="fixed transform -translate-x-1/2 -translate-y-full transition-opacity duration-300 ease-in-out {visible ? 'opacity-100' : 'opacity-0'}"
            style="left: {position.x}px; top: {position.y}px; transform: translate(-50%, -100%) translateY({visible ? '0' : '-10px'}); z-index: var(--z-index-log-viewer-notification);"
        >
            <!-- Dynamic background and border based on error type -->
            <div class="backdrop-blur-md 
                       {errorType === 'error_all' ? 'bg-error-hard/10' : 
                         errorType === 'error_task' ? 'bg-error-soft/10' : 
                         'bg-primary/20'} 
                       bg-gradient-to-br 
                       {errorType === 'error_all' 
                         ? 'from-hsla(var(--error-hard-hue), var(--error-hard-saturation), var(--error-hard-lightness), 0.15) to-secondary/10' 
                         : errorType === 'error_task'
                           ? 'from-hsla(var(--error-soft-hue), var(--error-soft-saturation), var(--error-soft-lightness), 0.15) to-secondary/10'
                           : 'from-primary/30 to-secondary/20'
                       } 
                       text-white border 
                       {errorType === 'error_all' 
                         ? 'border-error-hard/20' 
                         : errorType === 'error_task'
                           ? 'border-error-soft/20'
                           : 'border-primary/30'
                       } 
                       rounded-lg p-4 min-w-[280px] max-w-[400px] 
                       transition-all duration-200 font-sans 
                       shadow-lg 
                       {errorType === 'error_all' 
                         ? 'shadow-error-hard/15' 
                         : errorType === 'error_task'
                           ? 'shadow-error-soft/15'
                           : 'shadow-primary/20'
                       } 
                       cursor-pointer notification-container"
                 class:animation-reduced={currentUserActivityState === 'idle'}
                 on:click={handleDismiss}>
                
                <div class="text-sm font-medium mb-3 text-gray-300 flex items-center gap-2">
                    <span class="material-icons text-xl 
                          {errorType === 'error_all' 
                            ? 'text-error-hard/70' 
                            : errorType === 'error_task'
                              ? 'text-error-soft/70'
                              : 'text-primary'}">
                        {hasErrors ? 'warning' : 'info'}
                    </span>
                    <span>
                        {#if hasErrors}
                            {#if abortTaskLogs.length > 0 || abortAllLogs.length > 0}
                                Errors occurred with cancelation
                            {:else}
                                Errors occurred{isInProcessingContext ? ' (no task canceled)' : ''}
                            {/if}
                        {:else if isProcessing}
                            Processing in progress
                        {:else}
                            <!-- This shouldn't happen given visibility logic -->
                            Log viewer available
                        {/if}
                    </span>
                </div>
                
                <div class="bg-black/20 backdrop-blur-sm 
                            border 
                            {errorType === 'error_all' 
                              ? 'border-error-hard/20' 
                              : errorType === 'error_task'
                                ? 'border-error-soft/20'
                                : 'border-primary/30'
                            } 
                            rounded-md p-3 transition-all duration-200 
                            hover:bg-black/30 cursor-pointer"
                     on:click|stopPropagation={handleOpenClick}>
                    <div class="flex items-center gap-2">
                        <span class="material-icons text-[18px] 
                              {errorType === 'error_all' 
                                ? 'text-error-hard/80' 
                                : errorType === 'error_task'
                                  ? 'text-error-soft/80'
                                  : 'text-primary'}">
                            {hasErrors ? 'error' : 'info'}
                        </span>
                        <span class="text-sm font-medium text-gray-300">
                            {#if hasErrors}
                                {#if abortAllLogs.length > 0}
                                    <span class="gradient-text-all">
                                        {abortTaskLogs.length !== 1 ? 'Media processing' : 'All media processing task'} aborted following {abortAllLogs.length} critical error{abortAllLogs.length !== 1 ? 's' : ''}
                                    </span>
                                {:else if abortTaskLogs.length > 0}
                                    <span class="gradient-text-task">
                                        {abortTaskLogs.length} media processing task{abortTaskLogs.length !== 1 ? 's' : ''} aborted with {totalErrorCount} error{totalErrorCount !== 1 ? 's' : ''}
                                    </span>
                                {:else}
                                    {totalErrorCount} error{totalErrorCount !== 1 ? 's' : ''} detected{isInProcessingContext ? ' during processing' : ''}
                                {/if}
                            {:else if isProcessing}
                                Open the Log Viewer to see ongoing processing details
                            {:else}
                                Open the Log Viewer
                            {/if}
                        </span>
                    </div>
                    
                    <div class="mt-2 text-xs flex items-center gap-1 text-gray-400 font-medium">
                        <span class="text-[10px] material-icons 
                              {errorType === 'error_all' 
                                ? 'text-error-hard/60' 
                                : errorType === 'error_task'
                                  ? 'text-error-soft/60'
                                  : 'text-secondary/80'}">
                            touch_app
                        </span>
                        Click to open Log Viewer
                    </div>
                </div>
                
                <div class="absolute left-1/2 bottom-[-6px] transform -translate-x-1/2 rotate-45 w-3 h-3 
                           {errorType === 'error_all' 
                             ? 'bg-error-hard/10' 
                             : errorType === 'error_task'
                               ? 'bg-error-soft/10'
                               : 'bg-primary/20'
                           } 
                           backdrop-blur-md 
                           {errorType === 'error_all' 
                             ? 'border-l-error-hard/20 border-b-error-hard/20' 
                             : errorType === 'error_task'
                               ? 'border-l-error-soft/20 border-b-error-soft/20'
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
    
    @keyframes glow-error-soft {
        0% { box-shadow: 0 0 5px 0 hsla(var(--error-soft-hue), var(--error-soft-saturation), var(--error-soft-lightness), 0.2); }
        50% { box-shadow: 0 0 15px 5px hsla(var(--error-soft-hue), var(--error-soft-saturation), var(--error-soft-lightness), 0.3); }
        100% { box-shadow: 0 0 5px 0 hsla(var(--error-soft-hue), var(--error-soft-saturation), var(--error-soft-lightness), 0.2); }
    }
    
    @keyframes glow-error-hard {
        0% { box-shadow: 0 0 5px 0 hsla(var(--error-hard-hue), var(--error-hard-saturation), var(--error-hard-lightness), 0.2); }
        50% { box-shadow: 0 0 15px 5px hsla(var(--error-hard-hue), var(--error-hard-saturation), var(--error-hard-lightness), 0.3); }
        100% { box-shadow: 0 0 5px 0 hsla(var(--error-hard-hue), var(--error-hard-saturation), var(--error-hard-lightness), 0.2); }
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
    
    :global(.bg-error-soft\/10.notification-container) {
        animation: pulsate 2.5s ease-in-out infinite, glow-error-soft 3.5s infinite;
    }
    
    :global(.bg-error-hard\/10.notification-container) {
        animation: pulsate 2s ease-in-out infinite, glow-error-hard 3s infinite;
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
        background: var(--error-soft-gradient, linear-gradient(to right, hsl(45, 100%, 60%), hsl(30, 100%, 50%)));
        -webkit-background-clip: text;
        background-clip: text;
        transition: background var(--error-transition-duration, 1.5s) ease-in-out,
                    color var(--error-transition-duration, 1.5s) ease-in-out;

        /* Fallback for browsers that don't support background-clip: text */
        @supports not (background-clip: text) {
            color: theme('colors.error-soft');
            transition: color var(--error-transition-duration, 1.5s) ease-in-out;
        }
    }

    .gradient-text-all {
        position: relative;
        color: transparent;
        background: var(--error-hard-gradient, linear-gradient(to right, hsl(320, 70%, 25%), hsl(335, 85%, 40%)));
        -webkit-background-clip: text;
        background-clip: text;
        transition: background var(--error-transition-duration, 1.5s) ease-in-out,
                    color var(--error-transition-duration, 1.5s) ease-in-out;

        /* Fallback for browsers that don't support background-clip: text */
        @supports not (background-clip: text) {
            color: theme('colors.error-hard');
            transition: color var(--error-transition-duration, 1.5s) ease-in-out;
        }
    }
    
    /* Smooth transition on hover */
    .notification-container:hover {
        transform: translateY(-2px);
        box-shadow: 0 8px 20px rgba(0, 0, 0, 0.2);
    }
    
    /* Reduce animations when user is idle */
    .notification-container.animation-reduced {
        animation: none !important;
    }
    
    .notification-container.animation-reduced:hover {
        transform: none;
    }
</style>