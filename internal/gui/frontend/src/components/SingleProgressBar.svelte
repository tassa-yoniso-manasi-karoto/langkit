<script lang="ts">
    import { tweened } from 'svelte/motion';
    import { cubicInOut, elasticOut } from 'svelte/easing';
    import type { ProgressBarData } from '../lib/progressBarsStore';
    
    // The progress bar data is passed as a single "bar" object.
    export let bar: ProgressBarData;
    
    // Application processing state
    export let isProcessing = true;
    
    // Create a tweened store with slower start and faster finish
    const progress = tweened(0, {
        duration: 800, // Longer duration for more pronounced effect
        easing: cubicInOut // Slow start, faster middle, slow end
    });
    
    // Reset progress when bar changes or processing starts/stops
    $: {
        // When processing starts, reset to 0 first, then animate to current value
        if (!isProcessing) {
            progress.set(0, { duration: 0 }); // Immediate reset, no animation
        } else {
            progress.set(bar.progress); // Animate to current value
        }
    }
    
    // Default size if not provided
    $: size = bar.size || 'h-2.5';
    
    // Handle animation based on animated flag and processing state
    $: animated = bar.animated === true && isProcessing;
    $: striped = bar.striped === true && isProcessing;
    
    // Determine if bar is complete
    $: isComplete = $progress >= 100 && !bar.errorState;
    
    // Determine status text
    $: statusText = bar.status || ($progress < 100 ? 'Processing...' : 'Complete');
    
    // Determine if we're in an error state
    $: hasError = !!bar.errorState;
    
    // Get gradient position based on progress for dynamic color effect
    $: gradientPosition = $progress <= 0 ? 0 : ($progress >= 100 ? 100 : $progress);
    
    // Set defaults for animation if not provided
    $: if (bar.animated === undefined) bar = { ...bar, animated: true };
</script>


<div class="bg-white/5 p-3 rounded-lg text-xs flex flex-col gap-1.5 hover:bg-white/10 transition-colors shadow-md">
    <!-- Header: Operation label and description -->
    <div class="flex justify-between items-center mb-1">
        <span class="text-base font-medium text-white flex items-center gap-2">
            {bar.operation}
            {#if isComplete}
                <span class="text-pale-green text-xs">✓</span>
            {/if}
        </span>
        <span class="text-sm font-medium text-white/80">
            {bar.description}
        </span>
    </div>
    
    <!-- Progress bar component -->
    <div class="w-full bg-gray-700/50 rounded-full overflow-hidden shadow-inner">
        <div 
            class="rounded-full {size} relative overflow-hidden transition-all duration-300"
            style="width: {$progress}%;"
        >
            <!-- Normal progress gradient - only shown in normal state -->
            {#if !hasError}
                <div class="absolute inset-0 progress-gradient" 
                     style="--gradient-position: {gradientPosition}%;"></div>
            <!-- Error task gradient - orange/yellow -->
            {:else if bar.errorState === 'error_task'}
                <div class="absolute inset-0 layer-error-task animate-fade-in"></div>
            <!-- Error all gradient - red -->
            {:else if bar.errorState === 'error_all'}
                <div class="absolute inset-0 layer-error-all animate-fade-in"></div>
            <!-- User cancel gradient - gray/blue -->
            {:else if bar.errorState === 'user_cancel'}
                <div class="absolute inset-0 layer-user-cancel animate-fade-in"></div>
            {/if}
            
            <!-- Animated sweeping gradient (only for normal state) -->
            {#if $progress < 100 && !hasError && isProcessing}
                <div class="absolute h-full w-full overflow-hidden">
                    <div class="absolute inset-0" style="width: calc(max(500px, 150%));">
                        <div id="gradient-{bar.id}" 
                             class="animate-sweep-gradient absolute inset-0 w-full h-full" 
                             style="opacity: var(--sweep-opacity, 0.5);">
                        </div>
                    </div>
                    
                    <!-- Edge glow -->
                    <div class="absolute top-0 bottom-0 w-[1px] shadow-progress-edge" 
                         style="right: 0">
                    </div>
                </div>
            {/if}
            
            <!-- Completion pulse effect (only for completed, non-error state) -->
            {#if isComplete && !hasError}
                <div class="absolute inset-0 animate-pulse-soft rounded-full"></div>
            {/if}
        </div>
    </div>
    
    <!-- Progress percentage, fraction info and status -->
    <div class="flex justify-between text-gray-300 text-[0.75rem]">
        <div class="flex items-center gap-2">
            <span class="font-medium">{$progress.toFixed(0)}%</span>
            <span class="{!hasError ? 'text-gray-400' : ''}
                         {bar.errorState === 'error_task' ? 'gradient-text-task' : ''}
                         {bar.errorState === 'error_all' ? 'gradient-text-all' : ''}
                         {bar.errorState === 'user_cancel' ? 'gradient-text-cancel' : ''}">
                {statusText}
            </span>
        </div>
        {#if bar.total}
            <span>{bar.current} / {bar.total}</span>
        {/if}
    </div>
</div>

<style>
    /* Base gradient text styles with transitions */
    .gradient-text-base {
        position: relative;
        transition: color var(--error-transition-duration, 1.5s) ease-in-out;
    }

    /* Gradient text with transitions */
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

    /* Normal text state with transition */
    .text-normal {
        transition: color var(--error-transition-duration, 1.5s) ease-in-out;
    }

    /* Dynamic gradient background with position based on progress */
    .progress-gradient {
        background: linear-gradient(to right, var(--primary-color, #9f6ef7), var(--secondary-color, #ff6ec7));
        background-size: 200% 100%;
        background-position: calc(100% - var(--gradient-position, 0%)) 0;
        box-shadow: 0 0 8px rgba(159, 110, 247, 0.6);
    }
    
    /* Error state gradients */
    .layer-error-task {
        background: var(--error-task-gradient, linear-gradient(to right, hsl(45, 100%, 60%), hsl(30, 100%, 50%)));
        box-shadow: 0 0 10px hsla(var(--error-task-hue), var(--error-task-saturation), var(--error-task-lightness), 0.7);
    }
    
    .layer-error-all {
        background: var(--error-all-gradient, linear-gradient(to right, hsl(0, 100%, 45%), hsl(350, 100%, 60%)));
        box-shadow: 0 0 10px hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.7);
    }
    
    .layer-user-cancel {
        background: var(--user-cancel-gradient, linear-gradient(to right, hsl(220, 15%, 40%), hsl(210, 20%, 50%)));
    }
    
    /* Animation utility class */
    .animate-fade-in {
        animation: fadeIn var(--error-transition-duration, 1.5s) ease-in-out forwards;
    }
    
    /* Custom pulse animation for completed state */
    @keyframes pulse-soft {
        0% { box-shadow: 0 0 5px rgba(159, 110, 247, 0.5); }
        50% { box-shadow: 0 0 12px rgba(255, 110, 199, 0.8); }
        100% { box-shadow: 0 0 5px rgba(159, 110, 247, 0.5); }
    }
    
    .animate-pulse-soft {
        animation: pulse-soft 2s ease-in-out infinite;
        background: transparent;
    }
    
    /* For backward compatibility - preserved but not used with the new approach */
    .error_task { 
        background-color: theme('colors.error-task');
        animation: fadeToOrange var(--error-transition-duration, 1.5s) ease-in-out forwards;
    }
    
    .error_all { 
        background-color: theme('colors.error-all');
        animation: fadeToRed var(--error-transition-duration, 1.5s) ease-in-out forwards;
    }
    
    .user_cancel { 
        background-color: theme('colors.user-cancel');
        animation: fadeToGray var(--error-transition-duration, 1.5s) ease-in-out forwards;
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