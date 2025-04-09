<script lang="ts">
    import { createEventDispatcher, onMount, onDestroy } from 'svelte';
    import { clickOutside } from '../lib/clickOutside';
    import Hovertip from './Hovertip.svelte';
    import Portal from 'svelte-portal';
    import { type Readable } from 'svelte/store';

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
    export let className: string = "";
    export let invalid: boolean = false;
    export let errorMessage: string = '';
    // Add new prop for direct store access
    export let storeBinding: {groupId?: string, optionId?: string} | null = null;

    // Internal state
    let isOpen = false;
    let dropdownRef: HTMLDivElement;
    let buttonRef: HTMLButtonElement;
    let optionsContainerRef: HTMLDivElement;
    let hasRenderedOptions = false;
    let dropdownPosition = { top: 0, left: 0, width: 0 };
    const dispatch = createEventDispatcher();
    
    // Local value tracking
    let internalValue = value;
    let unsubscribeFromStore: (() => void) | null = null;
    
    // CRITICAL FIX: Make reactive statements directly depend on props
    $: {
        // This will run whenever 'value' changes
        if (value !== internalValue) {
            console.log(`Dropdown value prop changed: ${internalValue} → ${value}`);
            internalValue = value;
        }
    }

    // CRITICAL FIX: Make all relevant variables reactive dependencies
    $: selectedDisplayText = getSelectedDisplayText(internalValue, options);
    $: selectedOption = getSelectedOptionObject(internalValue, options);

    function toggleDropdown() {
        if (disabled) return;
        isOpen = !isOpen;
        if (isOpen) {
            hasRenderedOptions = true;
            updateDropdownPosition();
        }
    }

    function updateDropdownPosition() {
        if (!buttonRef) return;
        
        const rect = buttonRef.getBoundingClientRect();
        
        dropdownPosition = {
            top: rect.bottom + window.scrollY,
            left: rect.left + window.scrollX,
            width: rect.width
        };
    }

    function closeDropdown() {
        isOpen = false;
    }

    function selectOption(option: any) {
        const selectedValue = optionKey && typeof option === 'object' ? option[optionKey] : option;
        if (internalValue !== selectedValue) {
            internalValue = selectedValue;
            value = selectedValue; // Update external value
            dispatch('change', selectedValue);
        }
        closeDropdown();
    }

    /**
     * Get the formatted display text for any option
     */
    function getOptionDisplayText(option: any): string {
        if (!option) return '';
        
        // First priority: Use the labelFunction if provided
        if (labelFunction) {
            return labelFunction(option);
        }
        
        // Second priority: For objects, use optionLabel if specified
        if (typeof option === 'object' && optionLabel && option[optionLabel] !== undefined) {
            return option[optionLabel];
        }
        
        // Third priority: For objects with no matching optionLabel, convert to string
        return option.toString();
    }

    /**
     * Consistent method to find the currently selected option object
     * FIXED: Explicitly accept value and options as parameters to track dependencies
     */
    function getSelectedOptionObject(currentValue: any, currentOptions: any[]): any {
        if (!currentValue || !currentOptions || !currentOptions.length) return null;
        
        // For object options with optionKey
        if (optionKey && currentOptions.length > 0 && typeof currentOptions[0] === 'object') {
            return currentOptions.find(opt => opt[optionKey] === currentValue) || null;
        } 
        
        // For primitive options (strings, numbers, etc.)
        return currentOptions.find(opt => opt === currentValue) || null;
    }
    
    /**
     * Get the display text for the currently selected value
     * FIXED: Explicitly accept value and options as parameters to track dependencies
     */
    function getSelectedDisplayText(currentValue: any, currentOptions: any[]): string {
        if (!currentValue) return placeholder || `Select ${label || 'option'}...`;
        
        // Find the selected option object using the current values
        const selectedOpt = getSelectedOptionObject(currentValue, currentOptions);
        
        // If we found it, use the same display formatting used in dropdown
        if (selectedOpt) {
            return getOptionDisplayText(selectedOpt);
        }
        
        // Fallback if the option isn't found in the list
        return currentValue.toString();
    }

    function getOptionTooltip(option: any): string | null {
        return tooltipFunction ? tooltipFunction(option) : null;
    }

    // Event handlers to close dropdown
    function handleScroll() {
        if (isOpen) {
            closeDropdown();
        }
    }

    function handleResize() {
        if (isOpen) {
            closeDropdown();
        }
    }

    function handleClickOutside(event: CustomEvent) {
        closeDropdown();
    }

    onMount(() => {
        // Add listeners for events that should close the dropdown
        window.addEventListener('scroll', handleScroll, { passive: true, capture: true });
        window.addEventListener('resize', handleResize);
        
        // Subscribe to store if store binding is provided
        if (storeBinding?.groupId && storeBinding?.optionId && typeof window.featureGroupStore !== 'undefined') {
            const optionStore = window.featureGroupStore.createOptionSubscription(
                storeBinding.groupId,
                storeBinding.optionId
            );
            
            unsubscribeFromStore = optionStore.subscribe(storeValue => {
                if (storeValue !== undefined && storeValue !== internalValue) {
                    internalValue = storeValue;
                    // Not updating external value to avoid loops
                }
            });
        }
    });
    
    onDestroy(() => {
        window.removeEventListener('scroll', handleScroll, { capture: true });
        window.removeEventListener('resize', handleResize);
        
        if (unsubscribeFromStore) unsubscribeFromStore();
    });
</script>

<div 
    class="relative w-full text-sm {className}" 
    bind:this={dropdownRef} 
    use:clickOutside 
    on:clickoutside={handleClickOutside}
>
    {#if options.length > 0}
        <button 
            type="button" 
            bind:this={buttonRef}
            class="dropdown-button w-full flex justify-between items-center rounded-md h-[42px] px-3 py-2 font-medium {invalid ? 'border-error-task' : (error ? 'border-error-all/70' : '')} {disabled ? 'opacity-50 cursor-not-allowed' : 'focus:ring-offset-[3px] focus:ring-2 focus:ring-primary/30'} transition-all duration-200 relative"
            aria-haspopup="listbox" 
            aria-expanded={isOpen} 
            aria-labelledby={label ? `${label}-label` : undefined} 
            on:click={toggleDropdown} 
            {disabled}
        >
            <!-- Fixed-width container for text -->
            <div class="flex-grow min-w-0 overflow-hidden text-center">
                <span class="block truncate {!internalValue ? 'text-gray-400' : ''}">
                    {selectedDisplayText}
                </span>
            </div>
            <!-- Icon with flex-shrink-0 to prevent it from being compressed -->
            <span class="material-icons text-primary/70 ml-1 transition-transform duration-200 flex-shrink-0" class:rotate-180={isOpen}>expand_more</span>
        </button>
    {:else}
        <button type="button" class="dropdown-button w-full flex justify-between items-center rounded-md h-[42px] px-3 py-2 font-medium text-center text-gray-400 cursor-not-allowed opacity-70" disabled>
            <div class="flex-grow min-w-0 overflow-hidden text-center">
                <span class="block truncate">No options available</span>
            </div>
            <span class="material-icons text-primary/70 ml-1 flex-shrink-0">expand_more</span>
        </button>
    {/if}

    {#if isOpen && hasRenderedOptions}
        <Portal target="body">
            <div 
                class="dropdown-options fixed max-h-60 overflow-auto focus:outline-none" 
                bind:this={optionsContainerRef} 
                role="listbox"
                style="top: {dropdownPosition.top}px; left: {dropdownPosition.left}px; width: {dropdownPosition.width}px; z-index: 9999;"
            >
                {#each options as option, i (optionKey && typeof option === 'object' ? option[optionKey] : i)}
                    {@const isSelected = optionKey && typeof option === 'object' 
                        ? option[optionKey] === value 
                        : option === value}
                    {@const displayText = getOptionDisplayText(option)}
                    {@const tooltipText = getOptionTooltip(option)}
                    {#if tooltipText}
                        <Hovertip message={tooltipText} position="right">
                            <div slot="trigger" class="cursor-pointer font-semibold text-center truncate" on:click={() => selectOption(option)} role="option" aria-selected={isSelected}>
                                {displayText}
                            </div>
                        </Hovertip>
                    {:else}
                        <div class="cursor-pointer text-center truncate" on:click={() => selectOption(option)} role="option" aria-selected={isSelected}>
                            {displayText}
                        </div>
                    {/if}
                {/each}
            </div>
        </Portal>
    {/if}

    {#if invalid && errorMessage}
        <p class="mt-1 text-xs text-error-task">{errorMessage}</p>
    {:else if error}
        <p class="mt-1 text-xs text-error-all">{error}</p>
    {/if}
</div>

<style>
  /* Base dropdown styles */
  .dropdown-button {
    width: 100%;
    border: 2px solid var(--input-border);
    background-color: hsla(var(--input-bg), 0.4);
    box-shadow: var(--input-shadow);
  }
  
  /* Hover styles */
  .dropdown-button:hover:not(:disabled) {
    background-color: hsla(var(--input-bg-hover), 0.45);
    border-color: var(--input-border-hover);
  }
  
  /* Focus styles */
  .dropdown-button:focus:not(:disabled) {
    background-color: hsla(var(--input-bg-focus), 0.5);
    /* Don't override border color on focus if invalid */
    border-color: var(--input-border-focus);
    box-shadow: var(--input-shadow-focus);
  }
  
  /* Ensure invalid border takes precedence on focus */
  .dropdown-button.border-error-task:focus {
      border-color: var(--error-task-color);
  }

  /* Dropdown options styling */
  .dropdown-options {
    background: linear-gradient(135deg, var(--dropdown-primary-color), var(--dropdown-base-color), var(--dropdown-secondary-color));
    
    /* Control blur effect */
    backdrop-filter: blur(var(--dropdown-backdrop-blur, 12px));
    -webkit-backdrop-filter: blur(var(--dropdown-backdrop-blur, 12px));
    
    /* Appearance */
    border: 1px solid var(--dropdown-border, var(--input-border));
    border-radius: 8px;
    box-shadow: var(--dropdown-shadow, 0 8px 24px rgba(0, 0, 0, 0.2), 0 0 12px rgba(159, 110, 247, 0.15));
    padding: 2px 0;
    margin-top: 4px;
    font-size: 11px;
    font-weight: 500;
    transition: all 0.2s ease;
  }

  /* Dropdown option items */
  .dropdown-options div[role="option"] {
    padding: 4px 10px !important;
    margin: 0 !important;
    border-radius: 0 !important;
    white-space: nowrap !important;
    overflow: hidden !important;
    text-overflow: ellipsis !important;
    transition: all 0.15s ease;
    opacity: 0.95;
    text-align: center;
    max-width: 100% !important;
  }

  /* Hover effect */
  .dropdown-options div[role="option"]:hover {
    background-color: var(--dropdown-base-color);
    opacity: 1;
  }

  /* Selected option background */
  .dropdown-options div[role="option"][aria-selected="true"] {
    background-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4);
    color: white;
    font-weight: 600;
  }

  /* Hover state - needs to be more visible */
  .dropdown-options div[role="option"]:hover {
    background-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.25);
    opacity: 1;
  }
  
  /* Rounded corners for first and last items */
  .dropdown-options div[role="option"]:first-child {
    border-top-left-radius: 7px !important;
    border-top-right-radius: 7px !important;
  }

  .dropdown-options div[role="option"]:last-child {
    border-bottom-left-radius: 7px !important;
    border-bottom-right-radius: 7px !important;
  }

  /* Custom scrollbar styling */
  .dropdown-options::-webkit-scrollbar {
    width: 4px;
  }

  .dropdown-options::-webkit-scrollbar-track {
    background: transparent;
  }

  .dropdown-options::-webkit-scrollbar-thumb {
    background-color: rgba(159, 110, 247, 0.3);
    border-radius: 10px;
  }

  /* Dropdown appearance animation */
  @keyframes dropdown-appear {
    from {
      opacity: 0;
      transform: translateY(-5px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .dropdown-options {
    animation: dropdown-appear 0.2s forwards;
  }
</style>