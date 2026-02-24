<script lang="ts">
    import { createEventDispatcher } from 'svelte';
    import { fade, slide } from 'svelte/transition';
    import { liteModeStore } from '../lib/stores';
    import type { ValidationReport, ExpectationProfile } from '../api/generated/api.gen';
    import { checkResultStore } from '../lib/checkResultStore';
    import ClusterView from './ClusterView.svelte';
    import ProfileManager from './ProfileManager.svelte';

    export let open: boolean = false;
    export let profiles: ExpectationProfile[] = [];
    export let report: ValidationReport | null = null;

    $: isLite = $liteModeStore.enabled;

    var dispatch = createEventDispatcher<{
        close: void;
        runCheck: void;
        saveProfile: { profile: ExpectationProfile };
        deleteProfile: { name: string };
    }>();

    function handleClose() {
        dispatch('close');
    }

    function handleKeydown(e: KeyboardEvent) {
        if (e.key === 'Escape') handleClose();
    }

    function basename(path: string): string {
        var parts = path.split('/');
        var windowsParts = parts[parts.length - 1].split('\\');
        return windowsParts[windowsParts.length - 1];
    }

    function formatMedianDuration(seconds: number): string {
        if (seconds <= 0) return '';
        var mins = Math.floor(seconds / 60);
        var secs = Math.round(seconds % 60);
        return mins + 'm ' + secs + 's';
    }
</script>

<svelte:window on:keydown={handleKeydown} />

<!-- Shell: ChatGPT fills in two-pane layout, profile editor, full cluster view, re-run button -->
{#if open}
    <!-- Backdrop -->
    <div class="fixed inset-0 z-50 {isLite ? 'bg-black/70' : 'backdrop-blur-lg bg-black/30'}"
         transition:fade={{ duration: 300 }}
         on:click={handleClose}>
    </div>

    <!-- Modal panel -->
    <div class="fixed inset-0 z-50 overflow-y-auto" on:click|stopPropagation>
        <div class="container mx-auto max-w-5xl p-4 min-h-screen flex items-center"
             transition:slide={{ duration: 300 }}>
            <div class="relative w-full bg-[#16161c]/95 rounded-xl border border-white/10
                        shadow-2xl overflow-hidden">

                <!-- Header -->
                <div class="flex items-center justify-between px-6 py-4 border-b border-white/10">
                    <h2 class="text-lg font-medium text-white/90">Library Diagnostics</h2>
                    <button
                        class="w-8 h-8 flex items-center justify-center rounded-md
                               text-white/40 hover:text-white/80 hover:bg-white/10 transition-colors"
                        on:click={handleClose}
                    >
                        <span class="material-icons text-base">close</span>
                    </button>
                </div>

                <!-- Two-pane body -->
                <div class="flex min-h-[60vh] max-h-[80vh]">
                    <!-- Left pane: Profile editor -->
                    <div class="w-80 shrink-0 border-r border-white/10 p-4 overflow-y-auto">
                        <!-- TODO: ChatGPT fills in profile editor (reuses ProfileManager internals) -->
                        <div class="text-xs text-white/40">Profile editor placeholder</div>
                    </div>

                    <!-- Right pane: Check results -->
                    <div class="flex-1 p-4 overflow-y-auto"
                         class:opacity-50={$checkResultStore.stale}>

                        {#if $checkResultStore.stale}
                            <div class="text-center py-8">
                                <div class="text-sm text-amber-400 mb-3">Results are out of date</div>
                                <button
                                    class="px-4 py-2 text-sm rounded-md bg-amber-500/20 text-amber-300
                                           border border-amber-500/30 hover:bg-amber-500/30 transition-colors"
                                    on:click={() => dispatch('runCheck')}
                                >
                                    <span class="material-icons text-sm align-middle mr-1">refresh</span>
                                    Re-run Check
                                </button>
                            </div>
                        {/if}

                        {#if report}
                            <!-- Consensus overview -->
                            {#if report.consensusSummaries && report.consensusSummaries.length > 0}
                                <div class="mb-4 space-y-2">
                                    {#each report.consensusSummaries as cs}
                                        <div class="text-xs text-white/60">
                                            <span class="text-white/80 font-medium">{basename(cs.directory)}</span>
                                            <span class="ml-1">({cs.fileCount} files)</span>
                                            <!-- TODO: ChatGPT fills in consensus detail -->
                                        </div>
                                    {/each}
                                </div>
                            {/if}

                            <!-- Full cluster view -->
                            <ClusterView issues={report.issues} compact={false} />
                        {:else}
                            <div class="flex items-center justify-center h-full text-sm text-white/40">
                                Click "Re-run Check" to scan your library
                            </div>
                        {/if}

                        <!-- Re-run button at bottom -->
                        {#if report && !$checkResultStore.stale}
                            <div class="mt-4 text-center">
                                <button
                                    class="px-4 py-2 text-sm rounded-md bg-white/5 text-white/60
                                           border border-white/10 hover:bg-white/10 transition-colors"
                                    on:click={() => dispatch('runCheck')}
                                >
                                    <span class="material-icons text-sm align-middle mr-1">refresh</span>
                                    Re-run Check
                                </button>
                            </div>
                        {/if}
                    </div>
                </div>
            </div>
        </div>
    </div>
{/if}
