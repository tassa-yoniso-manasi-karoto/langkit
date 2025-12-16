<script lang="ts">
    import { createEventDispatcher, onMount } from 'svelte';
    import { clickOutside } from '../lib/clickOutside';
    import Hovertip from './Hovertip.svelte';
    import Portal from 'svelte-portal/src/Portal.svelte';
    import { logger } from '../lib/logger';

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
    let highlightedIndex = -1;
    let isOpen = false;
    let dropdownRef: HTMLDivElement;
    let buttonRef: HTMLButtonElement;
    let optionsContainerRef: HTMLDivElement;
    let hasRenderedOptions = false;
    let dropdownPosition = { top: 0, left: 0, width: 0 };
    const dispatch = createEventDispatcher();
    
    // Track the options reference to detect changes - critical fix for reactivity
    let previousOptionsRef = options;
    $: if (options !== previousOptionsRef) {
        previousOptionsRef = options;
        
        // Reset active option when options change
        highlightedIndex = -1;
        
        // Force re-render next time dropdown opens
        if (isOpen) {
            // If dropdown is open, update immediately
            updateDropdownPosition();
            // Force reflow
            if (optionsContainerRef) {
                optionsContainerRef.style.display = 'none';
                setTimeout(() => {
                    if (optionsContainerRef) optionsContainerRef.style.display = '';
                }, 0);
            }
        } else {
            // Reset flag to ensure next open causes a fresh render
            hasRenderedOptions = false;
        }
    }

    function toggleDropdown() {
        if (disabled) return;
        isOpen = !isOpen;
        if (isOpen) {
            hasRenderedOptions = true;
            updateDropdownPosition();
            
            // Focus the dropdown container and setup initial highlight
            setTimeout(() => {
                if (optionsContainerRef) {
                    optionsContainerRef.focus();
                    logger.trace('dropdown', 'Dropdown opened - container focused');
                    
                    // Set initial highlighted item - either current value or first item
                    const findIndex = options.findIndex(option => 
                        optionKey && typeof option === 'object' 
                            ? option[optionKey] === value 
                            : option === value
                    );
                    
                    // Use the found index, or 0 if not found and options exist
                    highlightedIndex = findIndex >= 0 ? findIndex : (options.length > 0 ? 0 : -1);
                    logger.trace('dropdown', `Initial highlight index: ${highlightedIndex}`);
                    
                    // Apply the highlight visually
                    updateHighlightedOption(true);
                }
            }, 50);
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
    function handleScroll(event) {
        if (isOpen) {
            // Don't close dropdown when scrolling inside the options container
            if (optionsContainerRef && 
                (optionsContainerRef.contains(event.target as Node) || 
                 optionsContainerRef === event.target)) {
                // Allow scrolling within dropdown
                return;
            }
            
            // Close for scrolls outside dropdown
            closeDropdown();
        }
    }

    function handleResize() {
        if (isOpen) {
            closeDropdown();
        }
    }
    
    // Simple keydown handler - navigation is fully handled by global keydown handler
    function handleKeydown(event: KeyboardEvent) {
        // When dropdown is closed, handle opening keys
        if (!isOpen) {
            if (event.key === 'Enter' || event.key === ' ' || event.key === 'ArrowDown') {
                event.preventDefault();
                toggleDropdown();
            }
            return;
        }
        
        // For open dropdown, just prevent default for navigation keys
        // The actual navigation is handled by the global handler
        if (event.key === 'ArrowUp' || event.key === 'ArrowDown' ||
            event.key === 'Enter' || event.key === ' ' ||
            event.key === 'Home' || event.key === 'End' ||
            event.key === 'PageUp' || event.key === 'PageDown') {
            event.preventDefault();
        }
    }
    
    // Update the highlighted option in the UI
    function updateHighlightedOption(forceScroll = false) {
        if (!optionsContainerRef || highlightedIndex < 0) return;
        
        // Make sure dropdown is fully rendered
        if (!isOpen || !hasRenderedOptions) return;
        
        const optionElements = Array.from(optionsContainerRef.querySelectorAll('[role="option"]'));
        if (!optionElements.length) {
            logger.trace('dropdown', 'No option elements found');
            return;
        }
        
        logger.trace('dropdown', `Updating highlighted option: ${highlightedIndex}, total options: ${optionElements.length}`);
        
        // Ensure the highlighted index is valid
        if (highlightedIndex >= optionElements.length) {
            highlightedIndex = optionElements.length - 1;
            logger.trace('dropdown', `Adjusted highlightedIndex to ${highlightedIndex}`);
        }
        
        // Get the target element to highlight
        const targetElement = optionElements[highlightedIndex];
        
        // Update all elements - remove highlighting from all, then add to the target
        optionElements.forEach((element) => {
            element.classList.remove('keyboard-highlighted');
            element.removeAttribute('data-highlighted');
            // Add a clear visual marker that this item isn't selected
            element.style.backgroundColor = '';
            element.style.color = '';
            element.style.fontWeight = '';
        });
        
        // Apply highlighting to the target element
        if (targetElement) {
            logger.trace('dropdown', `Highlighting option: ${targetElement.textContent?.trim()}`);
            
            // Add CSS class for styling
            targetElement.classList.add('keyboard-highlighted');
            
            // Add data attribute for ARIA
            targetElement.setAttribute('data-highlighted', 'true');
            
            // Apply direct styles using theme variables
            targetElement.style.backgroundColor = 'hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.5)';
            targetElement.style.color = 'white';
            targetElement.style.fontWeight = 'bold';
            
            // Ensure the highlighted option is visible by scrolling to it
            if (forceScroll) {
                // Use a small delay to ensure the DOM has updated
                setTimeout(() => {
                    targetElement.scrollIntoView({ 
                        block: 'nearest',
                        behavior: 'smooth'
                    });
                }, 10);
            }
        }
        
        // Update aria-activedescendant on the listbox
        if (optionsContainerRef) {
            optionsContainerRef.setAttribute('aria-activedescendant', `option-${highlightedIndex}`);
        }
    }
    
    // Reset highlighted state when dropdown opens/closes
    function resetHighlight() {
        highlightedIndex = -1;
        if (optionsContainerRef) {
            const optionElements = optionsContainerRef.querySelectorAll('[role="option"]');
            optionElements.forEach(opt => {
                opt.removeAttribute('data-highlighted');
                opt.classList.remove('keyboard-highlighted');
            });
        }
    }

    onMount(() => {
        // Add listeners for events that should close the dropdown
        // Must NOT use passive: true for scroll event to allow checking the target element
        window.addEventListener('scroll', handleScroll, { passive: false, capture: true });
        window.addEventListener('resize', handleResize);
        
        // Global keydown handler to ensure keyboard navigation always works
        // regardless of which element has focus
        const handleGlobalKeydown = (e: KeyboardEvent) => {
            if (!isOpen) return;
            
            // Handle all navigation keys
            if (e.key === 'ArrowUp' || e.key === 'ArrowDown' || 
                e.key === 'Enter' || e.key === 'Escape' || 
                e.key === 'Home' || e.key === 'End' ||
                e.key === 'PageUp' || e.key === 'PageDown') {
                
                // ALWAYS capture these keydown events when dropdown is open
                e.preventDefault();
                e.stopPropagation();
                
                if (options.length === 0) return;
                
                // Calculate the jump size for PageUp/PageDown (approximately 10 items)
                const pageJumpSize = 10;
                
                // Handle navigation keys
                if (e.key === 'ArrowDown') {
                    // Move highlight down by 1
                    if (highlightedIndex === -1) {
                        highlightedIndex = 0;
                    } else {
                        highlightedIndex = (highlightedIndex + 1) % options.length;
                    }
                    logger.trace('dropdown', `ArrowDown: New index ${highlightedIndex}`);
                    updateHighlightedOption(true);
                    
                } else if (e.key === 'ArrowUp') {
                    // Move highlight up by 1
                    if (highlightedIndex === -1) {
                        highlightedIndex = options.length - 1;
                    } else {
                        highlightedIndex = (highlightedIndex - 1 + options.length) % options.length;
                    }
                    logger.trace('dropdown', `ArrowUp: New index ${highlightedIndex}`);
                    updateHighlightedOption(true);
                    
                } else if (e.key === 'PageDown') {
                    // Move highlight down by pageJumpSize
                    if (highlightedIndex === -1) {
                        highlightedIndex = 0;
                    } else {
                        // Jump down by pageJumpSize items, but don't go past the end
                        highlightedIndex = Math.min(highlightedIndex + pageJumpSize, options.length - 1);
                    }
                    logger.trace('dropdown', `PageDown: New index ${highlightedIndex}`);
                    updateHighlightedOption(true);
                    
                } else if (e.key === 'PageUp') {
                    // Move highlight up by pageJumpSize
                    if (highlightedIndex === -1) {
                        highlightedIndex = 0;
                    } else {
                        // Jump up by pageJumpSize items, but don't go before the start
                        highlightedIndex = Math.max(highlightedIndex - pageJumpSize, 0);
                    }
                    logger.trace('dropdown', `PageUp: New index ${highlightedIndex}`);
                    updateHighlightedOption(true);
                    
                } else if (e.key === 'Enter') {
                    // Select the currently highlighted option
                    if (highlightedIndex >= 0 && highlightedIndex < options.length) {
                        logger.trace('dropdown', `Selecting option: ${highlightedIndex}`);
                        selectOption(options[highlightedIndex]);
                    }
                    
                } else if (e.key === 'Escape') {
                    // Close the dropdown
                    closeDropdown();
                    buttonRef?.focus();
                    
                } else if (e.key === 'Home') {
                    // Go to the first option
                    highlightedIndex = 0;
                    updateHighlightedOption(true);
                    
                } else if (e.key === 'End') {
                    // Go to the last option
                    highlightedIndex = options.length - 1;
                    updateHighlightedOption(true);
                }
            }
        };
        
        window.addEventListener('keydown', handleGlobalKeydown, { capture: true });
        
        return () => {
            window.removeEventListener('scroll', handleScroll, { capture: true });
            window.removeEventListener('resize', handleResize);
            window.removeEventListener('keydown', handleGlobalKeydown, { capture: true });
        };
    });
</script>

<div class="relative w-full text-sm {className}" bind:this={dropdownRef} use:clickOutside on:clickoutside={closeDropdown}>
    {#if options.length > 0}
        <button 
            type="button" 
            bind:this={buttonRef}
            class="dropdown-button w-full flex justify-between items-center rounded-md h-[42px] px-3 py-2 font-medium {error ? 'border-error-hard/70' : ''} {disabled ? 'opacity-50 cursor-not-allowed' : 'focus:ring-offset-[3px] focus:ring-2 focus:ring-primary/30'} transition-all duration-200 relative"
            aria-haspopup="listbox" 
            aria-expanded={isOpen} 
            aria-labelledby={label ? `${label}-label` : undefined} 
            on:click={toggleDropdown}
            on:keydown={handleKeydown}
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
                class="dropdown-options fixed max-h-60 overflow-auto focus:outline focus:outline-2 focus:outline-primary/50"
                bind:this={optionsContainerRef} 
                role="listbox"
                aria-activedescendant={highlightedIndex >= 0 ? `option-${highlightedIndex}` : undefined}
                style="top: {dropdownPosition.top}px; left: {dropdownPosition.left}px; width: {dropdownPosition.width}px;"
                tabindex="0"
                on:keydown={handleKeydown}
            >
                {#each options as option, i (optionKey && typeof option === 'object' ? option[optionKey] : i)}
                    {@const isSelected = optionKey && typeof option === 'object' 
                        ? option[optionKey] === value 
                        : option === value}
                    {@const displayText = getOptionDisplayText(option)}
                    {@const tooltipText = getOptionTooltip(option)}
                    {#if tooltipText}
                        <Hovertip message={tooltipText} position="right">
                            <div 
                                slot="trigger" 
                                class="cursor-pointer font-semibold text-center truncate"
                                id={`option-${i}`}
                                on:click={() => selectOption(option)} 
                                on:mouseover={() => { highlightedIndex = i; updateHighlightedOption(); }}
                                role="option" 
                                aria-selected={isSelected}
                            >
                                {displayText}
                            </div>
                        </Hovertip>
                    {:else}
                        <div 
                            class="cursor-pointer text-center truncate" 
                            id={`option-${i}`}
                            on:click={() => selectOption(option)} 
                            on:mouseover={() => { highlightedIndex = i; updateHighlightedOption(); }}
                            role="option" 
                            aria-selected={isSelected}
                        >
                            {displayText}
                        </div>
                    {/if}
                {/each}
            </div>
        </Portal>
    {/if}

    {#if error}
        <p class="mt-1 text-xs text-error-hard">{error}</p>
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
    border-color: var(--input-border-focus);
    box-shadow: var(--input-shadow-focus);
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

  /* Keyboard focus - visual cue for the currently highlighted item */
  .dropdown-options div[role="option"][data-highlighted="true"],
  .dropdown-options div[role="option"].keyboard-highlighted {
    background-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.5) !important;
    outline: 2px solid hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.6) !important;
    color: white !important;
    font-weight: 700 !important;
    transform: scale(1.02);
    transition: all 0.1s ease-in-out;
    position: relative;
    z-index: 1;
    box-shadow: 0 0 4px rgba(0, 0, 0, 0.3);
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