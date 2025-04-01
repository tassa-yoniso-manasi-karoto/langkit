<script lang="ts">
    import { createEventDispatcher, onMount } from 'svelte';
    import { clickOutside } from '../lib/clickOutside';
    import Hovertip from './Hovertip.svelte';
    import Portal from 'svelte-portal/src/Portal.svelte';

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
    export let className: string = ""; // Added className prop for consistency

    // Internal state
    let isOpen = false;
    let dropdownRef: HTMLDivElement;
    let buttonRef: HTMLButtonElement;
    let optionsContainerRef: HTMLDivElement;
    let hasRenderedOptions = false;
    let dropdownPosition = { top: 0, left: 0, width: 0 };
    const dispatch = createEventDispatcher();

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
        
        // Get button position relative to viewport
        const rect = buttonRef.getBoundingClientRect();
        
        // Calculate positions
        dropdownPosition = {
            top: rect.bottom + window.scrollY, // Position below button
            left: rect.left + window.scrollX,
            width: rect.width
        };
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
     */
    function getSelectedOptionObject() {
        if (!value || !options || !options.length) return null;
        
        // For object options with optionKey
        if (optionKey && typeof options[0] === 'object') {
            return options.find(opt => opt[optionKey] === value) || null;
        } 
        
        // For primitive options (strings, numbers, etc.)
        return options.find(opt => opt === value) || null;
    }
    
    /**
     * Get the display text for the currently selected value
     */
    function getSelectedDisplayText(): string {
        if (!value) return placeholder || `Select ${label || 'option'}...`;
        
        // Find the selected option object
        const selectedOption = getSelectedOptionObject();
        
        // If we found it, use the same display formatting used in dropdown
        if (selectedOption) {
            return getOptionDisplayText(selectedOption);
        }
        
        // Fallback if the option isn't found in the list
        return value.toString();
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

    onMount(() => {
        // Add listeners for events that should close the dropdown
        window.addEventListener('scroll', handleScroll, { passive: true, capture: true });
        window.addEventListener('resize', handleResize);
        
        return () => {
            window.removeEventListener('scroll', handleScroll, { capture: true });
            window.removeEventListener('resize', handleResize);
        };
    });
</script>

<div class="relative w-full text-sm {className}" bind:this={dropdownRef} use:clickOutside on:clickoutside={closeDropdown}>
    {#if options.length > 0}
        <button 
            type="button" 
            bind:this={buttonRef}
            class="dropdown-button w-full flex justify-between items-center rounded-md h-[42px] px-3 py-2 font-medium {error ? 'border-error-all/70' : ''} {disabled ? 'opacity-50 cursor-not-allowed' : 'focus:ring-offset-[3px] focus:ring-2 focus:ring-primary/30'} transition-all duration-200 relative"
            aria-haspopup="listbox" 
            aria-expanded={isOpen} 
            aria-labelledby={label ? `${label}-label` : undefined} 
            on:click={toggleDropdown} 
            {disabled}
        >
            <!-- Fixed-width container for text -->
            <div class="flex-grow min-w-0 overflow-hidden text-center">
                <span class="block truncate {!value ? 'text-gray-400' : ''}">
                    {getSelectedDisplayText()}
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
                            <div slot="trigger" class="cursor-pointer text-center truncate" on:click={() => selectOption(option)} role="option" aria-selected={isSelected}>
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

    {#if error}
        <p class="mt-1 text-xs text-error-all">{error}</p>
    {/if}
</div>

<style>
  /* Base dropdown styles */
  .dropdown-button {
    width: 100%;
    border: 2px solid var(--input-border);
    background-color: var(--input-bg);
    box-shadow: var(--input-shadow);
  }
  
  /* Hover styles */
  .dropdown-button:hover:not(:disabled) {
    background-color: var(--input-bg-hover);
    border-color: var(--input-border-hover);
  }
  
  /* Focus styles */
  .dropdown-button:focus:not(:disabled) {
    background-color: var(--input-bg-focus);
    border-color: var(--input-border-focus);
    box-shadow: var(--input-shadow-focus);
  }

  /* Dropdown options styling */
  .dropdown-options {
    /* Add a solid base color with transparency */
    background-color: var(--dropdown-base-color);
    
    /* Then layer the gradient on top */
    background-image: var(--dropdown-base-color, linear-gradient(135deg, var(--dropdown-primary-color), var(--dropdown-secondary-color)));
    
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