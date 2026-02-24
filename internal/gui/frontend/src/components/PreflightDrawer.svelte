<script lang="ts">
    import { createEventDispatcher } from 'svelte';
    import { slide } from 'svelte/transition';
    import type { ValidationReport } from '../api/generated/api.gen';
    import { liteModeStore } from '../lib/stores';
    import { checkResultStore } from '../lib/checkResultStore';
    import { formatLanguageNames } from '../lib/languageDisplay';
    import CompactClusterList from './CompactClusterList.svelte';

    export let report: ValidationReport | null = null;
    var dispatch = createEventDispatcher<{ runCheck: void }>();
    $: isLite = $liteModeStore.enabled;

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

    function severityPillClass(kind: 'error' | 'warning' | 'info'): string {
        if (kind === 'error') return 'border-red-400/35 bg-red-500/15 text-red-200';
        if (kind === 'warning') return 'border-amber-400/35 bg-amber-500/15 text-amber-200';
        return 'border-blue-400/35 bg-blue-500/15 text-blue-200';
    }

    function visibleLanguageNames(tags: string[], limit: number): string[] {
        return formatLanguageNames(tags).slice(0, limit);
    }

    function hiddenLanguageCount(tags: string[], limit: number): number {
        var names = formatLanguageNames(tags);
        if (names.length <= limit) return 0;
        return names.length - limit;
    }
</script>

<div class="h-full flex flex-col overflow-hidden relative">
    {#if $checkResultStore.isRunning && !report}
        <div class="flex-1 flex items-center justify-center">
            <div class="text-center">
                <span class="material-icons text-white/50 animate-spin">refresh</span>
                <div class="mt-2 text-sm text-white/55">Checking media files...</div>
            </div>
        </div>
    {:else if !report}
        <div class="flex-1 flex items-center justify-center p-4">
            <div class="text-center max-w-xs">
                <div class="text-sm text-white/65 mb-1">No preflight results yet</div>
                <div class="text-xs text-white/45">
                    Run a library check from the Preflight bar.
                </div>
            </div>
        </div>
    {:else}
        <div class="h-full relative">
            <div class="flex-1 h-full overflow-y-auto p-3 space-y-3 transition-opacity"
                 class:opacity-50={$checkResultStore.stale}
                 class:pointer-events-none={$checkResultStore.stale}>

                <div class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
                    <div class="flex items-center justify-between gap-2">
                        <div class="text-xs text-white/65">
                            <span class="text-white/90 font-medium">{report.totalFiles}</span>
                            files checked
                        </div>
                        <div class="flex items-center gap-1.5">
                            <span class="px-2 py-0.5 rounded-full text-[10px] border {severityPillClass('error')}">
                                {report.errorCount} errors
                            </span>
                            <span class="px-2 py-0.5 rounded-full text-[10px] border {severityPillClass('warning')}">
                                {report.warningCount} warnings
                            </span>
                            <span class="px-2 py-0.5 rounded-full text-[10px] border {severityPillClass('info')}">
                                {report.infoCount} info
                            </span>
                        </div>
                    </div>
                </div>

                {#if report.consensusSummaries && report.consensusSummaries.length > 0}
                    <div class="rounded-lg border border-white/10 bg-white/5 p-2.5">
                        <div class="text-[11px] text-white/50 uppercase tracking-wide mb-2">
                            Consensus
                        </div>
                        <div class="space-y-2">
                            {#each report.consensusSummaries as cs}
                                <div class="rounded-md border border-white/10 bg-black/20 px-2 py-2">
                                    <div class="text-xs text-white/75 font-medium mb-1">
                                        {basename(cs.directory)} ({cs.fileCount} files)
                                    </div>
                                    <div class="space-y-2">
                                        {#if cs.consensusAudioLangs && cs.consensusAudioLangs.length > 0}
                                            <div>
                                                <div class="text-[10px] uppercase tracking-wide text-primary/80 mb-1">Audio</div>
                                                <div class="flex flex-wrap gap-1">
                                                    {#each visibleLanguageNames(cs.consensusAudioLangs, 6) as name}
                                                        <span class="text-[10px] px-2 py-0.5 rounded-full border border-primary/30 bg-primary/15 text-primary">
                                                            {name}
                                                        </span>
                                                    {/each}
                                                    {#if hiddenLanguageCount(cs.consensusAudioLangs, 6) > 0}
                                                        <span class="text-[10px] px-2 py-0.5 rounded-full border border-white/20 bg-white/10 text-white/70">
                                                            +{hiddenLanguageCount(cs.consensusAudioLangs, 6)} more
                                                        </span>
                                                    {/if}
                                                </div>
                                            </div>
                                        {/if}
                                        {#if cs.consensusSubLangs && cs.consensusSubLangs.length > 0}
                                            <div>
                                                <div class="text-[10px] uppercase tracking-wide text-secondary/80 mb-1">Subs</div>
                                                <div class="flex flex-wrap gap-1">
                                                    {#each visibleLanguageNames(cs.consensusSubLangs, 10) as name}
                                                        <span class="text-[10px] px-2 py-0.5 rounded-full border border-secondary/30 bg-secondary/15 text-secondary">
                                                            {name}
                                                        </span>
                                                    {/each}
                                                    {#if hiddenLanguageCount(cs.consensusSubLangs, 10) > 0}
                                                        <span class="text-[10px] px-2 py-0.5 rounded-full border border-white/20 bg-white/10 text-white/70">
                                                            +{hiddenLanguageCount(cs.consensusSubLangs, 10)} more
                                                        </span>
                                                    {/if}
                                                </div>
                                            </div>
                                        {/if}
                                        <div class="flex flex-wrap gap-1.5 pt-0.5">
                                            {#if cs.medianDurationSec > 0}
                                                <span class="text-[10px] px-2 py-0.5 rounded-full border border-white/15 bg-white/10 text-white/70">
                                                    Median: {formatMedianDuration(cs.medianDurationSec)}
                                                </span>
                                            {/if}
                                            {#if cs.consensusAudioTrackCount >= 0}
                                                <span class="text-[10px] px-2 py-0.5 rounded-full border border-white/15 bg-white/10 text-white/70">
                                                    Tracks: {cs.consensusAudioTrackCount}
                                                </span>
                                            {/if}
                                        </div>
                                    </div>
                                </div>
                            {/each}
                        </div>
                    </div>
                {/if}

                <CompactClusterList issues={report.issues} compact={true} />
            </div>

            {#if $checkResultStore.stale}
                <div class={(isLite ? 'bg-black/70' : 'bg-black/55 backdrop-blur-sm') + ' absolute inset-0 flex items-center justify-center p-4'}>
                    <div class="max-w-xs w-full rounded-lg border border-amber-400/25 bg-amber-500/10 p-4 text-center"
                         transition:slide={{ duration: isLite ? 0 : 200 }}>
                        <div class="text-sm text-amber-200 font-medium mb-1">Results are out of date</div>
                        <div class="text-xs text-amber-200/80 mb-3">
                            Re-run preflight to refresh this analysis.
                        </div>
                        <button
                            class="w-full px-3 py-2 rounded-md text-sm border border-amber-400/35
                                   bg-amber-500/15 text-amber-200 hover:bg-amber-500/25 transition-colors"
                            on:click={() => dispatch('runCheck')}
                        >
                            <span class="material-icons text-sm align-middle mr-1">refresh</span>
                            Re-run Check
                        </button>
                    </div>
                </div>
            {/if}
        </div>
    {/if}
</div>
