<script lang="ts">
    import { tweened } from 'svelte/motion';
    import { cubicOut } from 'svelte/easing';
    import type { ProgressBarData } from '../lib/progressBarsStore';

    // The progress bar data is passed as a single "bar" object.
    export let bar: ProgressBarData;
    
    // Create a tweened store for smooth animation
    const progress = tweened(0, {
        duration: 400,
        easing: cubicOut
    });
    
    // Update the progress whenever bar.progress changes
    $: progress.set(bar.progress);
    
    // Default size if not provided
    $: size = bar.size || 'h-2.5';
    
    // Handle animation based on animated flag
    $: animated = bar.animated === true;
    $: striped = bar.striped === true;
</script>

<div class="bg-white/5 p-2 rounded-lg text-xs flex flex-col gap-1 hover:bg-white/10 transition-colors">
    <!-- Header: Operation label and description (shown outside the bar) -->
    <div class="flex justify-between mb-1">
        <span class="text-base font-medium text-white">
            {bar.operation}
        </span>
        <span class="text-sm font-medium text-white opacity-80">
            {bar.description}
        </span>
    </div>

    <!-- Clean progress component using Tailwind classes -->
    <div class="w-full bg-gray-700 rounded-full overflow-hidden">
        <div 
            class="bg-accent rounded-full {size} {striped ? 'progress-striped' : ''} 
                  {animated ? 'progress-animated' : ''}"
            style="width: {$progress}%;"
        ></div>
    </div>

    <!-- Progress percentage and fraction info -->
    <div class="flex justify-between text-gray-300 text-[0.75rem]">
        <span>{$progress.toFixed(0)}%</span>
        {#if bar.total}
            <span>{bar.current} / {bar.total}</span>
        {/if}
    </div>
</div>

<style>
    /* Striped effect for progress bars */
    .progress-striped {
        background-image: linear-gradient(
            45deg,
            rgba(255, 255, 255, 0.15) 25%,
            transparent 25%,
            transparent 50%,
            rgba(255, 255, 255, 0.15) 50%,
            rgba(255, 255, 255, 0.15) 75%,
            transparent 75%,
            transparent
        );
        background-size: 1rem 1rem;
    }
    
    /* Animation for the progress bar */
    .progress-animated {
        animation: progress-stripes 1s linear infinite;
    }
    
    @keyframes progress-stripes {
        from {
            background-position: 1rem 0;
        }
        to {
            background-position: 0 0;
        }
    }
    
    /* Apply glow effect to accent progress bar */
    .bg-accent {
        box-shadow: 0 0 5px rgba(159, 110, 247, 0.5);
    }
</style>
