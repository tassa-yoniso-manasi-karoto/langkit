<script lang="ts">
    import type { ValidationReport } from '../api/generated/api.gen';
    import { checkResultStore } from '../lib/checkResultStore';
    import ClusterView from './ClusterView.svelte';

    export let report: ValidationReport | null = null;

    function formatMedianDuration(seconds: number): string {
        if (seconds <= 0) return '';
        var mins = Math.floor(seconds / 60);
        var secs = Math.round(seconds % 60);
        return mins + 'm ' + secs + 's';
    }

    function basename(path: string): string {
        var parts = path.split('/');
        var windowsParts = parts[parts.length - 1].split('\\');
        return windowsParts[windowsParts.length - 1];
    }
</script>

<!-- Shell: ChatGPT fills in consensus pills, stale overlay, compact cluster view -->
<div class="h-full flex flex-col overflow-hidden">
    {#if !report}
        <div class="flex-1 flex items-center justify-center text-sm text-white/40">
            No check results yet
        </div>
    {:else}
        <!-- Stale overlay -->
        <div class="flex-1 overflow-y-auto p-4 space-y-4"
             class:opacity-50={$checkResultStore.stale}>

            {#if $checkResultStore.stale}
                <div class="text-center py-4">
                    <div class="text-sm text-amber-400 mb-2">Results are out of date</div>
                    <!-- Re-run button wired by parent -->
                </div>
            {/if}

            <!-- Consensus pills -->
            {#if report.consensusSummaries && report.consensusSummaries.length > 0}
                <div class="space-y-2">
                    {#each report.consensusSummaries as cs}
                        <div class="text-xs text-white/60">
                            <span class="text-white/80 font-medium">{basename(cs.directory)}</span>
                            <span class="ml-1">({cs.fileCount} files)</span>
                            <!-- TODO: ChatGPT fills in language chips, duration badge -->
                        </div>
                    {/each}
                </div>
            {/if}

            <!-- Compact cluster view -->
            <ClusterView issues={report.issues} compact={true} />
        </div>
    {/if}
</div>
