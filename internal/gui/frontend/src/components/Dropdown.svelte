<script lang="ts">
    export let options: Array<any> = [];
    export let value: string = '';
    export let label: string = '';
    export let optionKey: string = '';
    export let optionLabel: string = '';
    export let labelFunction: ((option: string) => string) | null = null;
    export let tooltipFunction: ((option: string) => string) | null = null;
    export let disabledFunction: ((option: string) => boolean) | null = null;
    
    import { createEventDispatcher } from 'svelte';
    const dispatch = createEventDispatcher();

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

    // Fixed isDisabled function to properly handle options
    function isDisabled(option: any): boolean {
        const optionValue = getValue(option);
        if (disabledFunction && typeof optionValue === 'string') {
            return disabledFunction(optionValue);
        }
        return false;
    }

    function handleSelect(event: Event) {
        const target = event.target as HTMLSelectElement;
        value = target.value;
        dispatch('change', target.value);
    }

    // Updated to use isDisabled properly
    $: if (options.length > 0 && (!value || !options.some(opt => getValue(opt) === value))) {
        // Try to find first non-disabled option
        const availableOption = options.find(opt => !isDisabled(opt));
        const defaultValue = availableOption ? getValue(availableOption) : getValue(options[0]);
        
        if (defaultValue !== value) {
            value = defaultValue;
            dispatch('change', defaultValue);
        }
    }
</script>

<div class="relative w-full">
    <div class="relative flex items-center">
        <select
            bind:value
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
                    disabled={isDisabled(getValue(option))}
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