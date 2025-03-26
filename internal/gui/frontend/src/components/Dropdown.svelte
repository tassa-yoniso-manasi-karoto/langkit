<script lang="ts">
    export let options: Array<any> = [];
    export let value: string = '';
    export let label: string = '';
    export let optionKey: string = '';
    export let optionLabel: string = '';
    export let labelFunction: ((option: string) => string) | null = null;
    export let tooltipFunction: ((option: string) => string) | null = null;
    export let disabledFunction: ((option: string) => boolean) | null = null;
    
    import { createEventDispatcher, onMount } from 'svelte';
    const dispatch = createEventDispatcher();
    
    // Track selection state internally
    let selectElement: HTMLSelectElement;
    let internalValue: string = '';
    let initialRender = true;
    
    // Flag to prevent circular updates
    let isProcessingChange = false;

    onMount(() => {
        // Initialize internal value on mount
        internalValue = value;
        initialRender = false;
        
        // Set the select element value directly to match
        if (selectElement && value) {
            selectElement.value = value;
        }
    });

    function getValue(option: any): string {
        if (optionKey && typeof option === 'object') {
            return option[optionKey];
        }
        return option;
    }

    function getLabel(option: any): string {
        if (labelFunction && typeof option === 'string') {
            return labelFunction(option);
        }
        
        if (optionLabel && typeof option === 'object') {
            return option[optionLabel] || option[optionKey] || option;
        }
        return option;
    }

    function getTooltip(option: any): string {
        if (tooltipFunction && typeof option === 'string') {
            return tooltipFunction(option);
        }
        return '';
    }

    // We're not using the disabled state anymore
    function isDisabled(option: any): boolean {
        return false;
    }

    function handleSelect(event: Event) {
        // Prevent processing if we're already handling a change
        if (isProcessingChange) return;
        
        isProcessingChange = true;
        
        try {
            const target = event.target as HTMLSelectElement;
            const newValue = target.value;
            
            // Only dispatch if actually different
            if (newValue !== internalValue) {
                console.log(`Dropdown change: previous=${internalValue}, new=${newValue}`);
                
                // Update internal value first
                internalValue = newValue;
                
                // Then dispatch the change event
                dispatch('change', newValue);
            }
        } finally {
            // Always reset the flag when done
            isProcessingChange = false;
        }
    }
    
    // When external value changes, update our internal state
    $: if (!initialRender && value !== internalValue && !isProcessingChange) {
        // Set flag to prevent circular updates
        isProcessingChange = true;
        
        // Update our internal value
        internalValue = value;
        
        // Update the DOM element if it exists
        if (selectElement) {
            selectElement.value = value;
        }
        
        // Reset flag
        isProcessingChange = false;
    }
    
    // Default value behavior - only run once after initial render
    $: if (!initialRender && options.length > 0 && !internalValue && !isProcessingChange) {
        // Set flag to prevent circular updates
        isProcessingChange = true;
        
        try {
            // Use first option as default
            const defaultValue = getValue(options[0]);
            internalValue = defaultValue;
            
            // Only dispatch if actually needed
            if (defaultValue !== value) {
                // Use setTimeout to avoid update during render
                setTimeout(() => {
                    dispatch('change', defaultValue);
                }, 0);
            }
        } finally {
            // Always reset flag
            isProcessingChange = false;
        }
    }
</script>

<div class="relative w-full">
    <div class="relative flex items-center">
        <select
            bind:this={selectElement}
            on:change={handleSelect}
            class="w-full h-[42px] bg-sky-dark/50 border-2 border-primary/30 rounded-md
                   focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/30 
                   hover:border-primary/50 transition-all duration-200 text-sm font-medium
                   appearance-none cursor-pointer select-centered"
        >
            {#each options as option}
                <option 
                    value={getValue(option)} 
                    class="bg-bgold" 
                    title={getTooltip(option)}
                >
                    {getLabel(option)}
                </option>
            {/each}
        </select>
        <span class="material-icons text-primary/70 absolute right-3 pointer-events-none">
            expand_more
        </span>
    </div>
</div>

<style>
    select option:disabled {
        color: rgba(255, 255, 255, 0.5);
        font-style: italic;
    }

    .select-centered {
        text-align: center;
        text-align-last: center;
        -moz-text-align-last: center;
        -webkit-appearance: none;
        -moz-appearance: none;
        padding-left: 24px !important;
        padding-right: 24px !important;
    }

    /* Hide default arrow in Firefox */
    .select-centered {
        text-indent: 0;
        text-overflow: '';
    }

    /* Hide default arrow in IE/Edge */
    .select-centered::-ms-expand {
        display: none;
    }

    /* Center text in options */
    .select-centered option {
        text-align: center;
    }

    /* Firefox specific centering */
    @-moz-document url-prefix() {
        .select-centered {
            text-align: center !important;
            text-align-last: center !important;
        }
        .select-centered option {
            text-align: center !important;
        }
    }

    /* Webkit browsers specific centering */
    @media screen and (-webkit-min-device-pixel-ratio:0) {
        .select-centered {
            text-align: center !important;
            text-align-last: center !important;
        }
        .select-centered option {
            text-align: center !important;
        }
    }
</style>