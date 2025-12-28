<script lang="ts">
    import { createEventDispatcher, onMount } from 'svelte';
    import { clickOutside } from '../lib/clickOutside';
    import Portal from 'svelte-portal/src/Portal.svelte';
    import { logger } from '../lib/logger';
    import DockerIcon from './icons/DockerIcon.svelte';
    import NvidiaIcon from './icons/NvidiaIcon.svelte';
    import ReplicateIcon from './icons/ReplicateIcon.svelte';

    // Props (matching Dropdown.svelte interface)
    export let options: string[] = [];
    export let value: string = '';
    export let label: string = '';
    export let disabled: boolean = false;

    // Internal state
    let highlightedIndex = -1;
    let isOpen = false;
    let dropdownRef: HTMLDivElement;
    let buttonRef: HTMLButtonElement;
    let optionsContainerRef: HTMLDivElement;
    let hasRenderedOptions = false;
    let dropdownPosition = { top: 0, left: 0, width: 0 };
    const dispatch = createEventDispatcher();

    // Parse provider name to extract prefix and model name
    interface ParsedProvider {
        hasDocker: boolean;
        hasNvidia: boolean;
        hasReplicate: boolean;
        modelName: string;
    }

    function parseProvider(providerName: string): ParsedProvider {
        let remaining = providerName;
        let hasDocker = false;
        let hasNvidia = false;
        let hasReplicate = false;

        // Check for docker-nvidia- prefix first (order matters)
        if (remaining.startsWith('docker-nvidia-')) {
            hasDocker = true;
            hasNvidia = true;
            remaining = remaining.slice('docker-nvidia-'.length);
        } else if (remaining.startsWith('docker-')) {
            hasDocker = true;
            remaining = remaining.slice('docker-'.length);
        } else if (remaining.startsWith('replicate-')) {
            hasReplicate = true;
            remaining = remaining.slice('replicate-'.length);
        }

        return {
            hasDocker,
            hasNvidia,
            hasReplicate,
            modelName: remaining
        };
    }

    // Track the options reference to detect changes
    let previousOptionsRef = options;
    $: if (options !== previousOptionsRef) {
        previousOptionsRef = options;
        highlightedIndex = -1;
        if (isOpen) {
            updateDropdownPosition();
        } else {
            hasRenderedOptions = false;
        }
    }

    function toggleDropdown() {
        if (disabled) return;
        isOpen = !isOpen;
        if (isOpen) {
            hasRenderedOptions = true;
            updateDropdownPosition();
            setTimeout(() => {
                if (optionsContainerRef) {
                    optionsContainerRef.focus();
                    const findIndex = options.findIndex(opt => opt === value);
                    highlightedIndex = findIndex >= 0 ? findIndex : (options.length > 0 ? 0 : -1);
                    updateHighlightedOption(true);
                }
            }, 50);
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

    function selectOption(option: string) {
        if (value !== option) {
            value = option;
            dispatch('change', option);
        }
        closeDropdown();
    }

    function getSelectedDisplayText(): string {
        if (!value) return 'Select ' + (label || 'option') + '...';
        const parsed = parseProvider(value);
        return parsed.modelName;
    }

    function handleScroll(event: Event) {
        if (isOpen) {
            if (optionsContainerRef &&
                (optionsContainerRef.contains(event.target as Node) ||
                 optionsContainerRef === event.target)) {
                return;
            }
            closeDropdown();
        }
    }

    function handleResize() {
        if (isOpen) {
            closeDropdown();
        }
    }

    function handleKeydown(event: KeyboardEvent) {
        if (!isOpen) {
            if (event.key === 'Enter' || event.key === ' ' || event.key === 'ArrowDown') {
                event.preventDefault();
                toggleDropdown();
            }
            return;
        }

        if (event.key === 'ArrowUp' || event.key === 'ArrowDown' ||
            event.key === 'Enter' || event.key === ' ' ||
            event.key === 'Home' || event.key === 'End' ||
            event.key === 'PageUp' || event.key === 'PageDown') {
            event.preventDefault();
        }
    }

    function updateHighlightedOption(forceScroll = false) {
        if (!optionsContainerRef || highlightedIndex < 0) return;
        if (!isOpen || !hasRenderedOptions) return;

        const optionElements = Array.from(optionsContainerRef.querySelectorAll('[role="option"]'));
        if (!optionElements.length) return;

        if (highlightedIndex >= optionElements.length) {
            highlightedIndex = optionElements.length - 1;
        }

        const targetElement = optionElements[highlightedIndex];

        optionElements.forEach((element) => {
            element.classList.remove('keyboard-highlighted');
            element.removeAttribute('data-highlighted');
            (element as HTMLElement).style.backgroundColor = '';
            (element as HTMLElement).style.color = '';
            (element as HTMLElement).style.fontWeight = '';
        });

        if (targetElement) {
            targetElement.classList.add('keyboard-highlighted');
            targetElement.setAttribute('data-highlighted', 'true');
            (targetElement as HTMLElement).style.backgroundColor = 'hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.5)';
            (targetElement as HTMLElement).style.color = 'white';
            (targetElement as HTMLElement).style.fontWeight = 'bold';

            if (forceScroll) {
                setTimeout(() => {
                    targetElement.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
                }, 10);
            }
        }

        if (optionsContainerRef) {
            optionsContainerRef.setAttribute('aria-activedescendant', 'option-' + highlightedIndex);
        }
    }

    onMount(() => {
        window.addEventListener('scroll', handleScroll, { passive: false, capture: true });
        window.addEventListener('resize', handleResize);

        const handleGlobalKeydown = (e: KeyboardEvent) => {
            if (!isOpen) return;

            if (e.key === 'ArrowUp' || e.key === 'ArrowDown' ||
                e.key === 'Enter' || e.key === 'Escape' ||
                e.key === 'Home' || e.key === 'End' ||
                e.key === 'PageUp' || e.key === 'PageDown') {

                e.preventDefault();
                e.stopPropagation();

                if (options.length === 0) return;

                const pageJumpSize = 10;

                if (e.key === 'ArrowDown') {
                    highlightedIndex = highlightedIndex === -1 ? 0 : (highlightedIndex + 1) % options.length;
                    updateHighlightedOption(true);
                } else if (e.key === 'ArrowUp') {
                    highlightedIndex = highlightedIndex === -1 ? options.length - 1 : (highlightedIndex - 1 + options.length) % options.length;
                    updateHighlightedOption(true);
                } else if (e.key === 'PageDown') {
                    highlightedIndex = highlightedIndex === -1 ? 0 : Math.min(highlightedIndex + pageJumpSize, options.length - 1);
                    updateHighlightedOption(true);
                } else if (e.key === 'PageUp') {
                    highlightedIndex = highlightedIndex === -1 ? 0 : Math.max(highlightedIndex - pageJumpSize, 0);
                    updateHighlightedOption(true);
                } else if (e.key === 'Enter') {
                    if (highlightedIndex >= 0 && highlightedIndex < options.length) {
                        selectOption(options[highlightedIndex]);
                    }
                } else if (e.key === 'Escape') {
                    closeDropdown();
                    buttonRef?.focus();
                } else if (e.key === 'Home') {
                    highlightedIndex = 0;
                    updateHighlightedOption(true);
                } else if (e.key === 'End') {
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

    // Get parsed provider for current value (for button display)
    $: selectedParsed = value ? parseProvider(value) : null;
</script>

<div class="relative w-full text-sm" bind:this={dropdownRef} use:clickOutside on:clickoutside={closeDropdown}>
    {#if options.length > 0}
        <button
            type="button"
            bind:this={buttonRef}
            class="dropdown-button w-full flex justify-between items-center rounded-md h-[42px] px-3 py-2 font-medium {disabled ? 'opacity-50 cursor-not-allowed' : 'focus:ring-offset-[3px] focus:ring-2 focus:ring-primary/30'} transition-all duration-200 relative"
            aria-haspopup="listbox"
            aria-expanded={isOpen}
            aria-labelledby={label ? label + '-label' : undefined}
            on:click={toggleDropdown}
            on:keydown={handleKeydown}
            {disabled}
        >
            <!-- Icon(s) + text container -->
            <div class="flex-grow min-w-0 overflow-hidden flex items-center justify-center gap-2">
                {#if selectedParsed}
                    {#if selectedParsed.hasDocker}
                        <DockerIcon size="1.5em" className="text-[#2396ed] flex-shrink-0" noAnimation={true} />
                    {/if}
                    {#if selectedParsed.hasNvidia}
                        <NvidiaIcon size="1.6em" className="flex-shrink-0" />
                    {/if}
                    {#if selectedParsed.hasReplicate}
                        <ReplicateIcon size="1.3em" className="text-white/80 flex-shrink-0" />
                    {/if}
                    <span class="truncate">{selectedParsed.modelName}</span>
                {:else}
                    <span class="text-gray-400">Select {label || 'option'}...</span>
                {/if}
            </div>
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
                aria-activedescendant={highlightedIndex >= 0 ? 'option-' + highlightedIndex : undefined}
                style="top: {dropdownPosition.top}px; left: {dropdownPosition.left}px; width: {dropdownPosition.width}px;"
                tabindex="0"
                on:keydown={handleKeydown}
            >
                {#each options as option, i (option)}
                    {@const parsed = parseProvider(option)}
                    {@const isSelected = option === value}
                    <div
                        class="option-item cursor-pointer truncate flex items-center justify-center gap-2"
                        id={'option-' + i}
                        on:click={() => selectOption(option)}
                        on:mouseover={() => { highlightedIndex = i; updateHighlightedOption(); }}
                        role="option"
                        aria-selected={isSelected}
                    >
                        <span class="icon-group flex items-center gap-1">
                            {#if parsed.hasDocker}
                                <DockerIcon size="1.4em" className="text-[#2396ed] flex-shrink-0" noAnimation={true} />
                            {/if}
                            {#if parsed.hasNvidia}
                                <NvidiaIcon size="1.5em" className="flex-shrink-0" />
                            {/if}
                            {#if parsed.hasReplicate}
                                <ReplicateIcon size="1.2em" className="text-white/80 flex-shrink-0" />
                            {/if}
                        </span>
                        <span class="truncate">{parsed.modelName}</span>
                    </div>
                {/each}
            </div>
        </Portal>
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
    backdrop-filter: blur(var(--dropdown-backdrop-blur, 12px));
    -webkit-backdrop-filter: blur(var(--dropdown-backdrop-blur, 12px));
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

  /* Icon group micro-interactions */
  .icon-group {
    transition: transform 0.15s ease;
  }

  /* Scale up icons on hover */
  .dropdown-options div[role="option"]:hover .icon-group {
    transform: scale(1.15);
  }

  /* Scale up icons on click/active */
  .dropdown-options div[role="option"]:active .icon-group {
    transform: scale(1.25);
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

  /* Keyboard focus */
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

  /* Hover state */
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
