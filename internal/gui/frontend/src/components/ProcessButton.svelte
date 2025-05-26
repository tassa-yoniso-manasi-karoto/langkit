<script lang="ts">
    import { createEventDispatcher, onMount, onDestroy } from 'svelte';
    import { fade } from 'svelte/transition';
    import { errorStore } from '../lib/errorStore';
    import ProcessErrorTooltip from './ProcessErrorTooltip.svelte';
    import { logger } from '../lib/logger';
    import { userActivityState } from '../lib/stores';

    // Create event dispatcher without generic type parameters
    const dispatch = createEventDispatcher();

    export let isProcessing: boolean;
    
    // Track previous processing state
    let prevIsProcessing = isProcessing;

    let showTooltip = false;
    let buttonRef: HTMLButtonElement;
    let tooltipPosition = { x: 0, y: 0 };
    
    // Subscribe to user activity state
    let currentUserActivityState = 'active';
    const unsubscribeUserActivity = userActivityState.subscribe(value => {
        currentUserActivityState = value.state;
    });

    // Subscribe to the error store to get the current errors.
    let errors = [];
    const unsubscribe = errorStore.subscribe((val) => {
        const oldErrorCount = errors.length;
        errors = val;
        
        // Log error state changes
        if (oldErrorCount !== errors.length) {
            logger.trace('ProcessButton', 'Error count changed', { 
                from: oldErrorCount, 
                to: errors.length,
                hasCritical: errors.some(e => e.severity === 'critical')
            });
        }
        
        // Auto-update tooltip position based on button position when errors change
        // This ensures tooltip is properly positioned even with deferred loading
        if (buttonRef && errors.length > 0) {
            const rect = buttonRef.getBoundingClientRect();
            tooltipPosition = {
                x: rect.left + rect.width / 2,
                y: rect.top - 10
            };
        }
    });

    // Determine if any critical error exists (which will lock the button)
    $: hasCriticalErrors = errors.some(e => e.severity === 'critical');
    // For the tooltip we display all errors (if any exist)
    $: hasAnyErrors = errors.length > 0;
    
    // No automatic tooltip display
    // Only show on hover - handled by handleMouseOver and handleMouseOut

    // Update tooltip position based on mouse event coordinates.
    function updateTooltipPositionFromEvent(event: MouseEvent) {
        tooltipPosition = {
            x: event.clientX,
            y: event.clientY - 10  // position a bit above the cursor
        };
    }

    function handleMouseOver(event: MouseEvent) {
        if (hasAnyErrors) {
            showTooltip = true;
            logger.trace('ProcessButton', 'Showing error tooltip on hover');
            // First use button position for better initial positioning
            updateTooltipPositionFromButton();
            // Then use mouse position for fine-tuning
            updateTooltipPositionFromEvent(event);
        }
    }

    function handleMouseMove(event: MouseEvent) {
        if (showTooltip && hasAnyErrors) {
            updateTooltipPositionFromEvent(event);
        }
    }

    function handleMouseOut() {
        if (showTooltip) {
            logger.trace('ProcessButton', 'Hiding error tooltip');
        }
        showTooltip = false;
    }

    function handleClick() {
        // Only block processing when a critical error exists or processing is active.
        if (!hasCriticalErrors && !isProcessing) {
            logger.info('ProcessButton', 'Process button clicked - starting processing');
            dispatch('process');
        } else {
            logger.warn('ProcessButton', 'Button click blocked', { 
                hasCriticalErrors, 
                isProcessing,
                criticalErrors: errors.filter(e => e.severity === 'critical').map(e => e.message)
            });
        }
    }

    function handleKeydown(event: KeyboardEvent) {
        if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            logger.trace('ProcessButton', 'Keyboard activation', { key: event.key });
            handleClick();
        } else if (event.key === 'Escape' && showTooltip) {
            logger.trace('ProcessButton', 'Hiding tooltip via Escape key');
            showTooltip = false;
        }
    }

    function handleFocus(event: FocusEvent) {
        if (hasAnyErrors) {
            logger.trace('ProcessButton', 'Showing error tooltip on focus');
            showTooltip = true;
        }
    }

    // Helper function to position tooltip based on button
    function updateTooltipPositionFromButton() {
        if (buttonRef) {
            const rect = buttonRef.getBoundingClientRect();
            tooltipPosition = {
                x: rect.left + rect.width / 2,
                y: rect.top - 15  // Position tooltip further from button to prevent flicker
            };
        }
    }

    onMount(() => {
        logger.trace('ProcessButton', 'Component mounted');
        
        // Position tooltip initially
        setTimeout(updateTooltipPositionFromButton, 500);
        
        // Re-position on resize
        window.addEventListener('resize', () => {
            if (showTooltip && buttonRef) {
                updateTooltipPositionFromButton();
            }
        });
        
        // Initialize button position for tooltip if needed
        if (buttonRef) {
            updateTooltipPositionFromButton();
        }
    });

    onDestroy(() => {
        logger.trace('ProcessButton', 'Component unmounting');
        unsubscribe();
        unsubscribeUserActivity();
    });
    
    // React to processing state changes
    $: {
        if (isProcessing !== prevIsProcessing) {
            if (isProcessing) {
                logger.info('ProcessButton', 'Processing started');
            } else {
                logger.info('ProcessButton', 'Processing ended');
            }
            prevIsProcessing = isProcessing;
        }
    }
</script>

<div class="relative inline-block">
    <button
        bind:this={buttonRef}
        class="h-12 px-4 bg-primary text-white rounded-lg font-bold outline-none flex items-center justify-center overflow-hidden"
        class:opacity-50={hasCriticalErrors || isProcessing}
        class:cursor-not-allowed={hasCriticalErrors || isProcessing}
        class:hover:bg-opacity-80={!hasCriticalErrors && !isProcessing}
        class:hover:-translate-y-0.5={!hasCriticalErrors && !isProcessing}
        class:hover:shadow-lg={!hasCriticalErrors && !isProcessing}
        on:click={handleClick}
        on:mouseover={handleMouseOver}
        on:mousemove={handleMouseMove}
        on:mouseout={handleMouseOut}
        on:focus={handleFocus}
        on:keydown={handleKeydown}
        aria-disabled={hasCriticalErrors || isProcessing}
        role="button"
        tabindex="0"
        style="transition: all 0.2s ease;"
    >
        <div class="flex items-center gap-2">
            {#if isProcessing}
                <span class="material-icons w-6 h-6 flex items-center justify-center" 
                      class:animate-spin={currentUserActivityState !== 'afk'}>refresh</span>
                <span>Processing...</span>
            {:else}
                <span>Process Files</span>
            {/if}
        </div>
    </button>

    {#if showTooltip && hasAnyErrors}
        <div
            class="fixed z-[1000] transform -translate-x-1/2 -translate-y-full"
            style="left: {tooltipPosition.x}px; top: {tooltipPosition.y}px;"
        >
            <ProcessErrorTooltip position={tooltipPosition} />
        </div>
    {/if}
</div>