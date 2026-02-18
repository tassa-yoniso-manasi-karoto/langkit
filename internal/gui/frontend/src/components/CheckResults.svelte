<script lang="ts">
    import { slide } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import { liteModeStore } from '../lib/stores';
    import { checkResultStore, checkState } from '../lib/checkResultStore';
    import type { ValidationReport, ValidationIssue } from '../api/services/expectation';

    // Severity filter state
    let showErrors = true;
    let showWarnings = true;
    let showInfos = false;

    // Source filter state (for combined auto+profile mode)
    let showProfile = true;
    let showAuto = true;

    // Collapsible sections
    let summariesOpen = false;
    let rawOpen = false;
    let consensusOpen = false;

    // Expand tracking for per-file detail
    let expandedFiles: Record<string, boolean> = {};

    $: report = $checkResultStore.report;
    $: isLite = $liteModeStore.enabled;

    // Auto-open summaries if there are errors
    $: if (report && report.errorCount > 0) {
        summariesOpen = true;
    }

    $: hasBothSources = report != null &&
        report.issues.some(i => i.source === 'profile') &&
        report.issues.some(i => i.source === 'auto');

    $: filteredIssues = report ? report.issues.filter(issue => {
        if (issue.severity === 'error' && !showErrors) return false;
        if (issue.severity === 'warning' && !showWarnings) return false;
        if (issue.severity === 'info' && !showInfos) return false;
        if (hasBothSources) {
            if (issue.source === 'profile' && !showProfile) return false;
            if (issue.source === 'auto' && !showAuto) return false;
        }
        return true;
    }) : [];

    $: filteredSummaries = report ? (report.interpretedSummaries || []) : [];

    // Group filtered issues by file
    $: issuesByFile = groupByFile(filteredIssues);

    function groupByFile(issues: ValidationIssue[]): Record<string, ValidationIssue[]> {
        var grouped: Record<string, ValidationIssue[]> = {};
        for (var i = 0; i < issues.length; i++) {
            var fp = issues[i].filePath;
            if (!grouped[fp]) grouped[fp] = [];
            grouped[fp].push(issues[i]);
        }
        return grouped;
    }

    function toggleFile(fp: string) {
        expandedFiles[fp] = !expandedFiles[fp];
        expandedFiles = expandedFiles;
    }

    function severityColor(severity: string): string {
        if (severity === 'error') return 'text-red-400';
        if (severity === 'warning') return 'text-yellow-400';
        return 'text-blue-400';
    }

    function severityBadge(severity: string): string {
        if (severity === 'error') return 'bg-red-500/20 text-red-300 border-red-500/30';
        if (severity === 'warning') return 'bg-yellow-500/20 text-yellow-300 border-yellow-500/30';
        return 'bg-blue-500/20 text-blue-300 border-blue-500/30';
    }

    function formatDuration(ms: number): string {
        if (ms < 1000) return ms + 'ms';
        return (ms / 1000).toFixed(1) + 's';
    }

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

{#if $checkResultStore.isRunning}
    <div class="px-4 py-3 rounded-xl border border-white/10 bg-white/5 flex items-center gap-3"
         transition:slide={{ duration: isLite ? 0 : 200 }}>
        <span class="material-icons text-lg text-white/60 animate-spin">refresh</span>
        <span class="text-sm text-white/70">Checking media files...</span>
    </div>
{/if}

{#if report}
    <div class="space-y-3" transition:slide={{ duration: isLite ? 0 : 300, easing: cubicOut }}>
        <!-- Tier 1: Summary Bar -->
        <div class="px-4 py-3 rounded-xl border
            {report.errorCount > 0
                ? 'bg-red-500/10 border-red-500/20'
                : report.warningCount > 0
                    ? 'bg-yellow-500/10 border-yellow-500/20'
                    : 'bg-emerald-500/10 border-emerald-500/20'}">
            <div class="flex items-center justify-between">
                <div class="flex items-center gap-3">
                    <span class="material-icons text-lg
                        {report.errorCount > 0 ? 'text-red-400' : report.warningCount > 0 ? 'text-yellow-400' : 'text-emerald-400'}">
                        {report.errorCount > 0 ? 'error' : report.warningCount > 0 ? 'warning' : 'check_circle'}
                    </span>
                    <span class="text-sm text-white/90">
                        {report.totalFiles} files checked
                        {#if report.errorCount > 0 || report.warningCount > 0}
                            — {#if report.errorCount > 0}{report.errorCount} error{report.errorCount !== 1 ? 's' : ''}{/if}{#if report.errorCount > 0 && report.warningCount > 0}, {/if}{#if report.warningCount > 0}{report.warningCount} warning{report.warningCount !== 1 ? 's' : ''}{/if}
                        {:else}
                            — all clean
                        {/if}
                    </span>
                </div>
                <span class="text-xs text-white/40">{formatDuration(report.durationMs)}</span>
            </div>
        </div>

        <!-- Tier 1.5: Consensus Overview (auto mode) -->
        {#if report.consensusSummaries && report.consensusSummaries.length > 0}
            <div class="rounded-xl border border-white/10 bg-white/5 overflow-hidden">
                <button
                    class="w-full px-4 py-2.5 flex items-center justify-between text-left hover:bg-white/5 transition-colors"
                    on:click={() => consensusOpen = !consensusOpen}
                >
                    <span class="text-sm font-medium text-white/80">Consensus Overview</span>
                    <span class="material-icons text-white/40 text-base transition-transform"
                          class:rotate-180={consensusOpen}>
                        expand_more
                    </span>
                </button>

                {#if consensusOpen}
                    <div class="px-4 pb-3 space-y-2 border-t border-white/5"
                         transition:slide={{ duration: isLite ? 0 : 200 }}>
                        {#each report.consensusSummaries as cs}
                            <div class="pt-2 text-xs text-white/60">
                                <span class="text-white/80 font-medium">{basename(cs.directory)}</span>
                                <span class="ml-1">({cs.fileCount} files{cs.bonusExcluded > 0 ? ', ' + cs.bonusExcluded + ' bonus excluded' : ''})</span>
                                <div class="ml-4 mt-1 space-y-0.5">
                                    {#if cs.consensusAudioLangs && cs.consensusAudioLangs.length > 0}
                                        <div>audio: [{cs.consensusAudioLangs.join(', ')}]</div>
                                    {/if}
                                    {#if cs.consensusSubLangs && cs.consensusSubLangs.length > 0}
                                        <div>subtitles: [{cs.consensusSubLangs.join(', ')}]</div>
                                    {/if}
                                    {#if cs.medianDurationSec > 0}
                                        <div>median duration: {formatMedianDuration(cs.medianDurationSec)}</div>
                                    {/if}
                                    {#if cs.consensusAudioTrackCount >= 0}
                                        <div>audio tracks: {cs.consensusAudioTrackCount}</div>
                                    {/if}
                                </div>
                            </div>
                        {/each}
                    </div>
                {/if}
            </div>
        {/if}

        <!-- Filter Bar -->
        <div class="flex items-center gap-2 flex-wrap">
            <button
                class="px-2.5 py-1 text-xs rounded-md border transition-colors
                    {showErrors ? 'bg-red-500/20 text-red-300 border-red-500/30' : 'bg-white/5 text-white/40 border-white/10'}"
                on:click={() => showErrors = !showErrors}
            >
                Errors ({report.errorCount})
            </button>
            <button
                class="px-2.5 py-1 text-xs rounded-md border transition-colors
                    {showWarnings ? 'bg-yellow-500/20 text-yellow-300 border-yellow-500/30' : 'bg-white/5 text-white/40 border-white/10'}"
                on:click={() => showWarnings = !showWarnings}
            >
                Warnings ({report.warningCount})
            </button>
            <button
                class="px-2.5 py-1 text-xs rounded-md border transition-colors
                    {showInfos ? 'bg-blue-500/20 text-blue-300 border-blue-500/30' : 'bg-white/5 text-white/40 border-white/10'}"
                on:click={() => showInfos = !showInfos}
            >
                Info ({report.infoCount})
            </button>

            {#if hasBothSources}
                <span class="text-white/20 mx-1">|</span>
                <button
                    class="px-2.5 py-1 text-xs rounded-md border transition-colors
                        {showProfile ? 'bg-purple-500/20 text-purple-300 border-purple-500/30' : 'bg-white/5 text-white/40 border-white/10'}"
                    on:click={() => showProfile = !showProfile}
                >
                    Profile
                </button>
                <button
                    class="px-2.5 py-1 text-xs rounded-md border transition-colors
                        {showAuto ? 'bg-cyan-500/20 text-cyan-300 border-cyan-500/30' : 'bg-white/5 text-white/40 border-white/10'}"
                    on:click={() => showAuto = !showAuto}
                >
                    Auto
                </button>
            {/if}
        </div>

        <!-- Tier 2: Interpreted Summaries -->
        {#if filteredSummaries.length > 0}
            <div class="rounded-xl border border-white/10 bg-white/5 overflow-hidden">
                <button
                    class="w-full px-4 py-2.5 flex items-center justify-between text-left hover:bg-white/5 transition-colors"
                    on:click={() => summariesOpen = !summariesOpen}
                >
                    <span class="text-sm font-medium text-white/80">Summaries ({filteredSummaries.length})</span>
                    <span class="material-icons text-white/40 text-base transition-transform"
                          class:rotate-180={summariesOpen}>
                        expand_more
                    </span>
                </button>

                {#if summariesOpen}
                    <div class="px-4 pb-3 border-t border-white/5"
                         transition:slide={{ duration: isLite ? 0 : 200 }}>
                        <ul class="mt-2 space-y-1">
                            {#each filteredSummaries as summary}
                                <li class="text-xs text-white/70 py-0.5 pl-3 border-l-2 border-white/10">
                                    {summary}
                                </li>
                            {/each}
                        </ul>
                    </div>
                {/if}
            </div>
        {/if}

        <!-- Tier 3: Raw Findings (per-file) -->
        {#if filteredIssues.length > 0}
            <div class="rounded-xl border border-white/10 bg-white/5 overflow-hidden">
                <button
                    class="w-full px-4 py-2.5 flex items-center justify-between text-left hover:bg-white/5 transition-colors"
                    on:click={() => rawOpen = !rawOpen}
                >
                    <span class="text-sm font-medium text-white/80">Details ({filteredIssues.length} findings)</span>
                    <span class="material-icons text-white/40 text-base transition-transform"
                          class:rotate-180={rawOpen}>
                        expand_more
                    </span>
                </button>

                {#if rawOpen}
                    <div class="border-t border-white/5 max-h-80 overflow-y-auto"
                         transition:slide={{ duration: isLite ? 0 : 200 }}>
                        {#each Object.keys(issuesByFile).sort() as fp}
                            <div class="border-b border-white/5 last:border-b-0">
                                <button
                                    class="w-full px-4 py-2 flex items-center justify-between text-left hover:bg-white/5 transition-colors"
                                    on:click={() => toggleFile(fp)}
                                >
                                    <span class="text-xs text-white/70 truncate flex-1 mr-2">{basename(fp)}</span>
                                    <div class="flex items-center gap-1.5 shrink-0">
                                        {#each issuesByFile[fp] as issue}
                                            <span class="w-1.5 h-1.5 rounded-full
                                                {issue.severity === 'error' ? 'bg-red-400' : issue.severity === 'warning' ? 'bg-yellow-400' : 'bg-blue-400'}">
                                            </span>
                                        {/each}
                                        <span class="material-icons text-white/30 text-sm ml-1 transition-transform"
                                              class:rotate-180={expandedFiles[fp]}>
                                            expand_more
                                        </span>
                                    </div>
                                </button>

                                {#if expandedFiles[fp]}
                                    <div class="px-4 pb-2 space-y-1"
                                         transition:slide={{ duration: isLite ? 0 : 150 }}>
                                        {#each issuesByFile[fp] as issue}
                                            <div class="flex items-start gap-2 py-1">
                                                <span class="inline-block px-1.5 py-0.5 text-[10px] rounded border shrink-0 {severityBadge(issue.severity)}">
                                                    {issue.severity}
                                                </span>
                                                <span class="text-xs text-white/60">{issue.message}</span>
                                            </div>
                                        {/each}
                                    </div>
                                {/if}
                            </div>
                        {/each}
                    </div>
                {/if}
            </div>
        {/if}
    </div>
{/if}
