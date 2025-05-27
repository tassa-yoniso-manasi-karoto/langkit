<script lang="ts">
    import { createEventDispatcher } from 'svelte';
    
    export let value: number;
    export let min: number = 0;
    export let max: number = 100;
    export let step: number = 1;
    export let label: string = '';
    export let disabled: boolean = false;
    export let showValue: boolean = true;
    export let formatValue: (value: number) => string = (v) => v.toString();
    export let className: string = '';
    
    const dispatch = createEventDispatcher();
    
    function handleInput(event: Event) {
        const target = event.target as HTMLInputElement;
        value = parseFloat(target.value);
        dispatch('change', value);
        dispatch('input', value);
    }
    
    // Calculate percentage for gradient background
    $: percentage = ((value - min) / (max - min)) * 100;
</script>

<div class="slider-container flex items-center gap-3 pl-6 {className}">    
    <div class="w-[60%]">
        <input
            type="range"
            {min}
            {max}
            {step}
            {value}
            {disabled}
            on:input={handleInput}
            class="slider w-full h-2 rounded-lg appearance-none cursor-pointer
                   disabled:opacity-50 disabled:cursor-not-allowed
                   focus:outline-none focus:ring-2 focus:ring-primary/30"
            style="background: linear-gradient(to right, 
                   var(--color-primary) 0%, 
                   var(--color-primary) {percentage}%, 
                   var(--slider-bg) {percentage}%, 
                   var(--slider-bg) 100%)"
        />
    </div>
    
    {#if showValue}
        <div class="text-sm font-medium text-primary whitespace-nowrap text-left">
            {formatValue(value)}
        </div>
    {/if}
</div>

<style>
    /* Custom slider thumb */
    .slider::-webkit-slider-thumb {
        appearance: none;
        width: 20px;
        height: 20px;
        border-radius: 50%;
        background: var(--color-primary);
        cursor: pointer;
        transition: all 0.15s ease-in-out;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
    }
    
    .slider::-webkit-slider-thumb:hover {
        transform: scale(1.1);
        box-shadow: 0 0 0 8px rgba(159, 110, 247, 0.1);
    }
    
    .slider::-webkit-slider-thumb:active {
        transform: scale(0.95);
    }
    
    .slider::-moz-range-thumb {
        width: 20px;
        height: 20px;
        border-radius: 50%;
        background: var(--color-primary);
        cursor: pointer;
        transition: all 0.15s ease-in-out;
        border: none;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
    }
    
    .slider::-moz-range-thumb:hover {
        transform: scale(1.1);
        box-shadow: 0 0 0 8px rgba(159, 110, 247, 0.1);
    }
    
    .slider::-moz-range-thumb:active {
        transform: scale(0.95);
    }
    
    /* Disabled state */
    .slider:disabled::-webkit-slider-thumb {
        cursor: not-allowed;
    }
    
    .slider:disabled::-moz-range-thumb {
        cursor: not-allowed;
    }
    
    /* Define CSS variables */
    :global(:root) {
        --color-primary: #9f6ef7;
        
        /* Slider background derived from input background variables */
        --slider-bg: hsl(
            var(--input-bg-hue), 
            var(--input-bg-saturation), 
            calc(var(--input-bg-lightness) + 8%)
        );
    }
</style>