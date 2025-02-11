<script lang="ts">
    import { createEventDispatcher, onMount, onDestroy } from 'svelte';
    import { fade } from 'svelte/transition';
    import { errorStore } from '../lib/errorStore';
    import ProcessErrorTooltip from './ProcessErrorTooltip.svelte';

    const dispatch = createEventDispatcher<{ process: void }>();

    export let isProcessing: boolean;

    let showTooltip = false;
    let buttonRef: HTMLButtonElement;
    let tooltipPosition = { x: 0, y: 0 };

    // Subscribe to the error store to get the current errors.
    let errors = [];
    const unsubscribe = errorStore.subscribe((val) => {
        errors = val;
        console.debug('ProcessButton: current errors', errors);
    });

    // Determine if any critical error exists (which will lock the button)
    $: hasCriticalErrors = errors.some(e => e.severity === 'critical');
    // For the tooltip we display all errors (if any exist)
    $: hasAnyErrors = errors.length > 0;

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
            updateTooltipPositionFromEvent(event);
            console.debug('Mouse over - showing tooltip', tooltipPosition);
        }
    }

    function handleMouseMove(event: MouseEvent) {
        if (showTooltip) {
            updateTooltipPositionFromEvent(event);
        }
    }

    function handleMouseOut() {
        showTooltip = false;
        console.debug('Mouse out - hiding tooltip');
    }

    function handleClick() {
        // Only block processing when a critical error exists or processing is active.
        if (!hasCriticalErrors && !isProcessing) {
            dispatch('process');
        } else {
            console.debug('Button click blocked; hasCriticalErrors:', hasCriticalErrors, 'isProcessing:', isProcessing);
        }
    }

    function handleKeydown(event: KeyboardEvent) {
        if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            handleClick();
        } else if (event.key === 'Escape' && showTooltip) {
            showTooltip = false;
        }
    }

    function handleFocus(event: FocusEvent) {
        if (hasAnyErrors) {
            showTooltip = true;
        }
    }

    onMount(() => {
        window.addEventListener('resize', () => {
            if (showTooltip && buttonRef) {
                // Recalculate a default position (center of button) on resize.
                const rect = buttonRef.getBoundingClientRect();
                tooltipPosition = {
                    x: rect.left + rect.width / 2,
                    y: rect.top - 10
                };
            }
        });
    });

    onDestroy(() => {
        unsubscribe();
    });
</script>

<div class="relative inline-block">
    <button
        bind:this={buttonRef}
        class="h-12 px-4 bg-accent text-white rounded-lg font-bold transition-colors transition-shadow transform outline-none flex items-center justify-center hover:bg-[#8a5de5] hover:shadow-md hover:translate-y-[-2px] active:translate-y-0"
        class:opacity-50={hasCriticalErrors || isProcessing}
        class:cursor-not-allowed={hasCriticalErrors || isProcessing}
        on:click={handleClick}
        on:mouseover={handleMouseOver}
        on:mousemove={handleMouseMove}
        on:mouseout={handleMouseOut}
        on:focus={handleFocus}
        on:keydown={handleKeydown}
        aria-disabled={hasCriticalErrors || isProcessing}
        role="button"
        tabindex="0"
    >
        <div class="flex items-center gap-2">
            {#if isProcessing}
                <span class="material-icons animate-spin">refresh</span>
                <span>Processing...</span>
            {:else}
                <span>Process Files</span>
            {/if}
        </div>
    </button>

    {#if showTooltip}
        <div
            class="fixed z-[1000] pointer-events-none transform -translate-x-1/2 -translate-y-full"
            style="left: {tooltipPosition.x}px; top: {tooltipPosition.y}px;"
            in:fade={{ duration: 150 }}
            out:fade={{ duration: 100 }}
        >
            <ProcessErrorTooltip position={tooltipPosition} />
        </div>
    {/if}
</div>
