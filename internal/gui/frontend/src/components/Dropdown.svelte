<script lang="ts">
    import { createEventDispatcher, onMount } from 'svelte';
    import { clickOutside } from '../lib/clickOutside';
    import { debounce } from 'lodash';
    import { fly } from 'svelte/transition';
    import Hovertip from './Hovertip.svelte';

    // Props
    export let options: any[] = [];
    export let value: any = '';
    export let label: string = '';
    export let placeholder: string = '';
    export let disabled: boolean = false;
    export let error: string = '';
    export let optionKey: string = '';
    export let optionLabel: string = '';
    export let labelFunction: ((option: any) => string) | null = null;
    export let tooltipFunction: ((option: any) => string) | null = null;

    // Internal state
    let isOpen = false;
    let dropdownRef: HTMLDivElement;
    let optionsContainerRef: HTMLDivElement;
    let hasRenderedOptions = false;
    const dispatch = createEventDispatcher();

    function toggleDropdown() {
        if (disabled) return;
        isOpen = !isOpen;
        if (isOpen) {
            hasRenderedOptions = true;
        }
    }

    function closeDropdown() {
        isOpen = false;
    }

    function selectOption(option: any) {
        const selectedValue = optionKey && typeof option === 'object' ? option[optionKey] : option;
        if (value !== selectedValue) {
            value = selectedValue;
            dispatch('change', selectedValue);
        }
        closeDropdown();
    }

    function findSelectedOption() {
        if (!optionKey || !options.length || typeof options[0] !== 'object') {
            return null;
        }
        return options.find(opt => opt[optionKey] === value);
    }

    function getOptionDisplayText(option: any): string {
        if (labelFunction) {
            return labelFunction(option);
        }
        if (optionKey && optionLabel && typeof option === 'object') {
            return option[optionLabel];
        }
        return option.toString();
    }

    function getOptionTooltip(option: any): string | null {
        return tooltipFunction ? tooltipFunction(option) : null;
    }

    onMount(() => {});
</script>

<div class="relative w-full text-sm" bind:this={dropdownRef} use:clickOutside on:clickoutside={closeDropdown}>
    {#if options.length > 0}
        <button type="button" class="w-full flex justify-between items-center glassmorphic-button border-2 {error ? 'border-error-all/70' : 'border-primary/30'} rounded-md h-[42px] px-3 py-2 text-sm font-medium text-left {disabled ? 'opacity-50 cursor-not-allowed' : 'hover:border-primary/50 focus:ring-offset-[3px] focus:ring-2 focus:ring-primary/30 focus:border-primary'} transition-all duration-200 relative overflow-hidden" aria-haspopup="listbox" aria-expanded={isOpen} aria-labelledby={label ? `${label}-label` : undefined} on:click={toggleDropdown} {disabled}>
            <span class="truncate {!value ? 'text-gray-400' : ''}">
                {#if value}
                    {#if optionKey && optionLabel && typeof options[0] === 'object'}
                        {#if findSelectedOption()}
                            {getOptionDisplayText(findSelectedOption())}
                        {:else}
                            {value}
                        {/if}
                    {:else}
                        {value}
                    {/if}
                {:else}
                    {placeholder || `Select ${label || 'option'}...`}
                {/if}
            </span>
            <span class="material-icons text-primary/70 ml-2 transition-transform duration-200 relative z-10" class:rotate-180={isOpen}>expand_more</span>
        </button>
    {:else}
        <button type="button" class="w-full flex justify-between items-center glassmorphic-button border-2 border-primary/30 rounded-md h-[42px] px-3 py-2 text-sm font-medium text-gray-400 cursor-not-allowed opacity-70" disabled>
            No options available
            <span class="material-icons text-primary/70 ml-2">expand_more</span>
        </button>
    {/if}

    {#if isOpen && hasRenderedOptions}
        <div class="absolute z-50 w-full mt-1 glassmorphic-dropdown border-2 border-primary/30 rounded-md shadow-lg max-h-60 overflow-auto focus:outline-none py-1" bind:this={optionsContainerRef} transition:fly={{ duration: 200, y: -10 }} role="listbox">
            {#each options as option, i (optionKey && typeof option === 'object' ? option[optionKey] : i)}
                {@const isSelected = optionKey && typeof option === 'object' ? option[optionKey] === value : option === value}
                {@const displayText = getOptionDisplayText(option)}
                {@const tooltipText = getOptionTooltip(option)}
                {#if tooltipText}
                    <Hovertip message={tooltipText} position="right">
                        <div slot="trigger" class="cursor-pointer px-3 py-2 text-sm hover:bg-white/20 {isSelected ? 'bg-primary/20 text-white' : 'text-white'} transition-colors duration-150" on:click={() => selectOption(option)} role="option" aria-selected={isSelected}>
                            {displayText}
                        </div>
                    </Hovertip>
                {:else}
                    <div class="cursor-pointer px-3 py-2 text-sm hover:bg-white/20 {isSelected ? 'bg-primary/20 text-white' : 'text-white'} transition-colors duration-150" on:click={() => selectOption(option)} role="option" aria-selected={isSelected}>
                        {displayText}
                    </div>
                {/if}
            {/each}
        </div>
    {/if}

    {#if error}
        <p class="mt-1 text-xs text-error-all">{error}</p>
    {/if}
</div>


<style>
  /* Custom scrollbar for dropdown */
  div[role="listbox"] {
    scrollbar-width: thin;
    scrollbar-color: rgba(159, 110, 247, 0.4) transparent;
  }
  
  div[role="listbox"]::-webkit-scrollbar {
    width: 6px;
  }
  
  div[role="listbox"]::-webkit-scrollbar-track {
    background: transparent;
  }
  
  div[role="listbox"]::-webkit-scrollbar-thumb {
    background-color: rgba(159, 110, 247, 0.4);
    border-radius: 20px;
  }
</style>