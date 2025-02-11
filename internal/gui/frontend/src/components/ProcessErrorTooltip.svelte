<script lang="ts">
    import { onDestroy } from 'svelte';
    import Portal from 'svelte-portal/src/Portal.svelte';
    import { fade, slide } from 'svelte/transition';
    import { flip } from 'svelte/animate';
    import { errorStore, type ErrorMessage, type ErrorSeverity } from '../lib/errorStore';

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
                    console.debug('Suggestion: Please click the drop zone to select a media file.');
                } else {
                    error.action.handler();
                }
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
        in:fade={{ duration: 150 }}
        out:fade={{ duration: 100 }}
    >
        <div class="backdrop-blur-sm bg-[rgba(100,0,0,0.85)] text-white border border-[rgba(255,255,255,0.2)] rounded p-4 min-w-[280px] max-w-[400px] transition-colors duration-200 font-sans">
            <div class="text-sm font-bold mb-2">
                {#if groupedErrors.length > 0 && groupedErrors[0].errors.length > 0}
                    {groupedErrors[0].errors.length} {groupedErrors[0].errors.length === 1 ? 'problem' : 'problems'} to resolve:
                {/if}
            </div>
            <ul class="list-none p-0 m-0">
                {#each groupedErrors[0].errors as error (error.id)}
                    <li class="bg-[rgba(150,0,0,0.6)] border border-[rgba(255,255,255,0.2)] rounded p-2 mb-2 transition-colors duration-200 cursor-pointer relative hover:bg-[rgba(150,0,0,0.45)]"
                        on:click={handleErrorClick(error)}
                        in:slide|local={{ duration: 150 }}
                        animate:flip={{ duration: 200 }}>
                        <div class="flex items-center gap-2">
                            <span class="material-icons text-[18px]">
                                {getSeverityIcon(error.severity)}
                            </span>
                            <span class="text-sm">{error.message}</span>
                        </div>
                        {#if error.action}
                            <div class="mt-1 text-xs text-[#ffcccb]">
                                {#if error.id === 'no-media'}
                                    Please click the drop zone to select a media file.
                                {:else}
                                    {error.action.label}
                                {/if}
                            </div>
                        {/if}
                        {#if error.dismissible}
                            <button class="absolute top-1 right-1 bg-transparent border-none cursor-pointer hover:opacity-80" on:click|stopPropagation={() => errorStore.removeError(error.id)}>
                                <span class="material-icons">close</span>
                            </button>
                        {/if}
                    </li>
                {/each}
            </ul>
            <div class="absolute left-1/2 bottom-[-6px] transform -translate-x-1/2 rotate-45 w-3 h-3 bg-[rgba(150,0,0,0.35)] border-l border-l-[rgba(255,255,255,0.2)] border-b border-b-[rgba(255,255,255,0.2)]"></div>
        </div>
    </div>
</Portal>

<style>
</style>
