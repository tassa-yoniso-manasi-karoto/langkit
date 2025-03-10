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
            class="rounded-full {size} transition-colors duration-1000 ease-in-out relative
                  {hasError ? bar.errorState : 'progress-gradient'} 
                  {isComplete && !hasError ? 'animate-pulse-soft' : ''}"
            style="width: {$progress}%; {!hasError ? `--gradient-position: ${gradientPosition}%` : ''}"
        >
            <!-- Direct simple progress bar animation -->
            {#if $progress < 100 && !hasError && isProcessing}
                <div class="progress-bar-animation absolute inset-0" style="width: {$progress}%;"></div>
            {/if}
        </div>
    </div>
    
    <!-- Progress percentage, fraction info and status -->
    <div class="flex justify-between text-gray-300 text-[0.75rem]">
        <div class="flex items-center gap-2">
            <span class="font-medium">{$progress.toFixed(0)}%</span>
            <span class="text-gray-400">{statusText}</span>
        </div>
        {#if bar.total}
            <span>{bar.current} / {bar.total}</span>
        {/if}
    </div>
</div>

<style>
    /* Dynamic gradient background with position based on progress */
    .progress-gradient {
        background: linear-gradient(to right, var(--primary-color, #9f6ef7), var(--pink-color, #ff6ec7));
        background-size: 200% 100%;
        background-position: calc(100% - var(--gradient-position, 0%)) 0;
        transition: background-position 0.4s ease, width 0.4s ease;
        box-shadow: 0 0 8px rgba(159, 110, 247, 0.6);
    }
    
    /* Shimmer effect overlay */
    .shimmer-overlay {
        background: linear-gradient(90deg, transparent, rgba(255,255,255,0.2), transparent);
        animation: shimmer 1.5s infinite;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
    }
    
    @keyframes shimmer {
        0% { transform: translateX(-100%); }
        100% { transform: translateX(100%); }
    }
    
    /* Simple progress bar animation */
    .progress-bar-animation {
        background-image: linear-gradient(
            -45deg, 
            rgba(255, 255, 255, 0.2) 25%, 
            transparent 25%, 
            transparent 50%, 
            rgba(255, 255, 255, 0.2) 50%, 
            rgba(255, 255, 255, 0.2) 75%, 
            transparent 75%, 
            transparent
        );
        background-size: 30px 30px;
        animation: move-stripes 2s linear infinite;
        z-index: 1;
        border-radius: inherit;
    }
    
    @keyframes move-stripes {
        0% { background-position: 0 0; }
        100% { background-position: 60px 0; }
    }
    
    /* Custom pulse animation for completed state */
    @keyframes pulse-soft {
        0% { box-shadow: 0 0 5px rgba(159, 110, 247, 0.5); }
        50% { box-shadow: 0 0 12px rgba(255, 110, 199, 0.8); }
        100% { box-shadow: 0 0 5px rgba(159, 110, 247, 0.5); }
    }
    
    .animate-pulse-soft {
        animation: pulse-soft 2s ease-in-out infinite;
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
        0% { background-color: theme('colors.primary'); }
        100% { background-color: theme('colors.error-task'); }
    }
    
    @keyframes fadeToRed {
        0% { background-color: theme('colors.primary'); }
        100% { background-color: theme('colors.error-all'); }
    }
    
    @keyframes fadeToGray {
        0% { background-color: theme('colors.primary'); }
        100% { background-color: theme('colors.user-cancel'); }
    }
</style>