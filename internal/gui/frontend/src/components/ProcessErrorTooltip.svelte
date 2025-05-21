<script lang="ts">
    import { onDestroy } from 'svelte';
    import Portal from 'svelte-portal/src/Portal.svelte';
    import { fade, slide } from 'svelte/transition';
    import { flip } from 'svelte/animate';
    import { errorStore, type ErrorMessage, type ErrorSeverity } from '../lib/errorStore';
    import { logger } from '../lib/logger';

    // Position is passed in from the ProcessButton.
    export let position = { x: 0, y: 0 };

    let errors: ErrorMessage[] = [];
    const unsubscribe = errorStore.subscribe((val) => {
        errors = val;
    });

    // Group all errors together for display.
    interface ErrorGroup {
        errors: ErrorMessage[];
    }

    $: groupedErrors = errors.reduce((groups, error) => {
        if (groups.length === 0) {
            groups.push({ errors: [error] });
        } else {
            groups[0].errors.push(error);
        }
        return groups;
    }, [] as ErrorGroup[]);

    // Return an icon based on error severity.
    function getSeverityIcon(severity: ErrorSeverity): string {
        switch (severity) {
            case 'critical':
                return 'error';
            case 'warning':
                return 'warning';
            case 'info':
                return 'info';
        }
    }

    // When the user clicks on an error item, if an action exists, perform it.
    function handleErrorClick(error: ErrorMessage) {
        return () => {
            if (error.action) {
                if (error.id === 'no-media') {
                    logger.debug('processErrorTooltip', 'Suggestion: Please click the drop zone to select a media file.');
                }
                error.action.handler();
            }
        };
    }

    onDestroy(() => {
        unsubscribe();
    });
</script>

<Portal target="body">
    <div
        class="fixed transform -translate-x-1/2 -translate-y-full z-[1000] pointer-events-none"
        style="left: {position.x}px; top: {position.y}px;"
        in:fade={{ duration: 300, easing: (t) => t < 0.5 ? 2 * t * t : -1 + (4 - 2 * t) * t }}
        out:fade={{ duration: 300, easing: (t) => t * (2-t) }}
    >
        <div class="backdrop-blur-md bg-tooltip-bg/60 bg-gradient-to-br from-primary/30 to-secondary/20 text-white border border-primary/20 rounded-lg p-4 min-w-[280px] max-w-[400px] transition-all duration-200 font-sans shadow-lg shadow-primary/20 pointer-events-auto">
            <div class="text-sm font-medium mb-3 text-gray-300 flex items-center gap-2">
                {#if groupedErrors.length > 0 && groupedErrors[0].errors.length > 0}
                    <span class="material-icons text-primary text-xl">notification_important</span>
                    <span>
                        {groupedErrors[0].errors.length} {groupedErrors[0].errors.length === 1 ? 'item' : 'items'} to address
                    </span>
                {/if}
            </div>
            <ul class="list-none p-0 m-0 pointer-events-auto">
                {#each groupedErrors[0]?.errors || [] as error (error.id)}
                    <li class="bg-error-card-bg/60 backdrop-blur-sm border border-secondary/30 rounded-md p-3 mb-3 transition-all duration-200 cursor-pointer relative hover:bg-error-card-hover/70 shadow-md hover:shadow-secondary/20"
                        on:click={handleErrorClick(error)}
                        in:slide|local={{ duration: 200, easing: (t) => t * (2-t) }}
                        out:slide|local={{ duration: 150, easing: (t) => t * t }}
                        animate:flip={{ duration: 200 }}>
                        <div class="flex items-center gap-2">
                            <span class="material-icons text-[18px] text-secondary">
                                {getSeverityIcon(error.severity)}
                            </span>
                            <span class="text-sm font-medium text-gray-300">{error.message}</span>
                        </div>
                        {#if error.action}
                            <div class="mt-2 text-xs flex items-center gap-1 text-gray-400 font-medium">
                                <span class="text-[10px] material-icons text-secondary/80">touch_app</span>
                                {#if error.id === 'no-media'}
                                    Please click the drop zone to select a media file.
                                {:else}
                                    {error.action.label}
                                {/if}
                            </div>
                        {/if}
                        {#if error.dismissible}
                            <button class="absolute top-2 right-2 bg-transparent border-none cursor-pointer opacity-70 hover:opacity-100 hover:text-secondary" on:click|stopPropagation={() => errorStore.removeError(error.id)}>
                                <span class="material-icons text-[16px]">close</span>
                            </button>
                        {/if}
                    </li>
                {/each}
            </ul>
            <div class="absolute left-1/2 bottom-[-6px] transform -translate-x-1/2 rotate-45 w-3 h-3 bg-tooltip-bg/60 backdrop-blur-md border-l border-l-primary/20 border-b border-b-primary/20"></div>
        </div>
    </div>
</Portal>

<style>
    /* Add a subtle pulse animation to the tooltip to draw attention */
    @keyframes subtle-pulse {
        0% { box-shadow: 0 0 0 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.15); }
        70% { box-shadow: 0 0 0 10px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0); }
        100% { box-shadow: 0 0 0 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0); }
    }
    
    /* Add a subtle float animation for a more dynamic feel */
    @keyframes subtle-float {
        0% { transform: translateY(0); }
        50% { transform: translateY(-2px); }
        100% { transform: translateY(0); }
    }
    
    /* Apply both animations to the tooltip container */
    div > div {
        animation: subtle-pulse 4s infinite, subtle-float 5s ease-in-out infinite;
        will-change: transform, box-shadow;
        transform-origin: center bottom;
    }
    
    /* Add a subtle hover effect to list items */
    li {
        transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    }
    
    li:hover {
        transform: translateY(-2px);
        box-shadow: 0 4px 12px hsla(var(--secondary-hue), var(--secondary-saturation), var(--secondary-lightness), 0.2);
    }
    
    /* Smooth scrolling for the tooltip content */
    ul {
        scroll-behavior: smooth;
        max-height: 65vh;
        overflow-y: auto;
        scrollbar-width: thin;
        scrollbar-color: hsla(var(--secondary-hue), var(--secondary-saturation), var(--secondary-lightness), 0.3) transparent;
    }
    
    /* Custom scrollbar styling */
    ul::-webkit-scrollbar {
        width: 5px;
    }
    
    ul::-webkit-scrollbar-track {
        background: transparent;
    }
    
    ul::-webkit-scrollbar-thumb {
        background-color: hsla(var(--secondary-hue), var(--secondary-saturation), var(--secondary-lightness), 0.3);
        border-radius: 20px;
    }
</style>