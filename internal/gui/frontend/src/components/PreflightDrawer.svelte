<script lang="ts">
    import { createEventDispatcher } from 'svelte';
    import { slide } from 'svelte/transition';
    import type { ValidationReport } from '../api/generated/api.gen';
    import { liteModeStore } from '../lib/stores';
    import { checkResultStore } from '../lib/checkResultStore';
    import { formatLanguageNames } from '../lib/languageDisplay';
    import {
        getClusters, sourceLabel, categoryLabel, normalizeSeverity,
        type Cluster,
    } from '../lib/preflightDataUtils';

    export let report: ValidationReport | null = null;
    var dispatch = createEventDispatcher<{ runCheck: void }>();
    $: isLite = $liteModeStore.enabled;

    $: clusters = report ? getClusters(report.issues) : [];

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

    function severityPillClass(kind: string): string {
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

    function clusterCardClass(severity: string): string {
        if (severity === 'error') return 'rounded-lg border border-red-400/20 bg-red-500/[0.04]';
        return 'rounded-lg border border-white/10 bg-white/5';
    }

    function clusterLabelClass(severity: string): string {
        if (severity === 'error') return 'text-red-300';
        return 'text-white/85';
    }

    function dotBgClass(severity: string): string {
        var s = normalizeSeverity(severity);
        if (s === 'error') return 'bg-red-400';
        if (s === 'warning') return 'bg-amber-400';
        return 'bg-blue-400';
    }

    function clusterBadgeClass(severity: string): string {
        if (severity === 'error') return 'text-red-200 bg-red-500/15';
        if (severity === 'warning') return 'text-amber-200 bg-amber-500/15';
        return 'text-blue-200 bg-blue-500/15';
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
                    Run a media check from the Preflight bar.
                </div>
            </div>
        </div>
    {:else}
        <div class="h-full relative">
            <div class="flex-1 h-full overflow-y-auto p-4 space-y-4 transition-opacity"
                 class:opacity-50={$checkResultStore.stale}
                 class:pointer-events-none={$checkResultStore.stale}>

                <!-- Headline stat strip -->
                <div class="rounded-lg border border-white/10 bg-white/5 px-4 py-3 flex items-center gap-3">
                    <div>
                        <div class="text-2xl font-semibold text-white/90 leading-none">{report.totalFiles}</div>
                        <div class="text-xs text-white/55">files</div>
                    </div>
                    <div class="w-px h-7 bg-white/10 shrink-0"></div>
                    <div class="flex flex-wrap gap-1.5">
                        <span class="px-2.5 py-0.5 rounded-full text-[11px] font-medium border {severityPillClass('error')}">
                            {report.errorCount} errors
                        </span>
                        <span class="px-2.5 py-0.5 rounded-full text-[11px] font-medium border {severityPillClass('warning')}">
                            {report.warningCount} warnings
                        </span>
                        <span class="px-2.5 py-0.5 rounded-full text-[11px] font-medium border {severityPillClass('info')}">
                            {report.infoCount} info
                        </span>
                    </div>
                </div>

                <!-- Consensus (label-value rows) -->
                {#if report.consensusSummaries && report.consensusSummaries.length > 0}
                    <div class="rounded-lg border border-white/10 bg-white/5 p-3 space-y-2">
                        {#each report.consensusSummaries as cs}
                            <div class="rounded-md border border-white/[0.06] bg-black/20 p-2.5">
                                <div class="text-sm text-white/75 font-medium mb-1.5 flex items-center gap-1.5">
                                    {basename(cs.directory)}
                                    <span class="text-[11px] text-white/35 font-normal">{cs.fileCount} files</span>
                                </div>
                                {#if cs.consensusAudioLangs && cs.consensusAudioLangs.length > 0}
                                    <div class="flex items-baseline gap-1.5 mb-1">
                                        <span class="text-[11px] uppercase tracking-[0.08em] text-primary/80 font-medium w-12 shrink-0">Audio</span>
                                        <div class="flex flex-wrap gap-1">
                                            {#each visibleLanguageNames(cs.consensusAudioLangs, 6) as name}
                                                <span class="text-[11px] px-2 py-0.5 rounded-full border border-primary/30 bg-primary/15 text-primary">{name}</span>
                                            {/each}
                                            {#if hiddenLanguageCount(cs.consensusAudioLangs, 6) > 0}
                                                <span class="text-[11px] px-2 py-0.5 rounded-full border border-white/20 bg-white/10 text-white/70">
                                                    +{hiddenLanguageCount(cs.consensusAudioLangs, 6)} more
                                                </span>
                                            {/if}
                                        </div>
                                    </div>
                                {/if}
                                {#if cs.consensusSubLangs && cs.consensusSubLangs.length > 0}
                                    <div class="flex items-baseline gap-1.5 mb-1">
                                        <span class="text-[11px] uppercase tracking-[0.08em] text-secondary/80 font-medium w-12 shrink-0">Subs</span>
                                        <div class="flex flex-wrap gap-1">
                                            {#each visibleLanguageNames(cs.consensusSubLangs, 10) as name}
                                                <span class="text-[11px] px-2 py-0.5 rounded-full border border-secondary/30 bg-secondary/15 text-secondary">{name}</span>
                                            {/each}
                                            {#if hiddenLanguageCount(cs.consensusSubLangs, 10) > 0}
                                                <span class="text-[11px] px-2 py-0.5 rounded-full border border-white/20 bg-white/10 text-white/70">
                                                    +{hiddenLanguageCount(cs.consensusSubLangs, 10)} more
                                                </span>
                                            {/if}
                                        </div>
                                    </div>
                                {/if}
                                {#if cs.medianDurationSec > 0 || cs.consensusAudioTrackCount >= 0}
                                    <div class="flex items-center gap-2 mt-1 text-xs text-white/55">
                                        <span class="w-12 shrink-0"></span>
                                        {#if cs.medianDurationSec > 0}
                                            <span class="flex items-center gap-1">
                                                <span class="material-icons" style="font-size:14px;">schedule</span>
                                                {formatMedianDuration(cs.medianDurationSec)} median
                                            </span>
                                        {/if}
                                        {#if cs.consensusAudioTrackCount >= 0}
                                            <span class="flex items-center gap-1">
                                                <span class="material-icons" style="font-size:14px;">audiotrack</span>
                                                {cs.consensusAudioTrackCount} tracks
                                            </span>
                                        {/if}
                                    </div>
                                {/if}
                            </div>
                        {/each}
                    </div>
                {/if}

                <!-- Cluster cards (separate, severity-tinted) -->
                {#if clusters.length > 0}
                    <div class="space-y-1.5">
                        {#each clusters as cluster}
                            <div class={clusterCardClass(cluster.severity)}>
                                <div class="flex items-center gap-2.5 px-3 py-2.5">
                                    <div class={'w-2 h-2 rounded-full shrink-0 ' + dotBgClass(cluster.severity)}
                                         class:severity-dot-glow={cluster.severity === 'error'}></div>
                                    <div class="flex-1 min-w-0">
                                        <div class={'text-sm font-medium truncate ' + clusterLabelClass(cluster.severity)}>
                                            {cluster.label}
                                        </div>
                                        <div class="text-[11px] text-white/35">
                                            {sourceLabel(cluster.source)} · {categoryLabel(cluster.category)}
                                        </div>
                                    </div>
                                    <span class={'text-xs font-medium px-2 py-0.5 rounded-full ' + clusterBadgeClass(cluster.severity)}>
                                        {cluster.fileCount}
                                    </span>
                                </div>
                            </div>
                        {/each}
                    </div>
                {/if}
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

<style>
    .severity-dot-glow {
        box-shadow: 0 0 8px rgba(248, 113, 113, 0.6);
    }
</style>
