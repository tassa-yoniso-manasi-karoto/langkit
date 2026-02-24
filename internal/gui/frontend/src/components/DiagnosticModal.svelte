<script lang="ts">
    import { createEventDispatcher } from 'svelte';
    import { fade, slide } from 'svelte/transition';
    import { liteModeStore } from '../lib/stores';
    import type { ValidationReport, ExpectationProfile, ValidationIssue } from '../api/generated/api.gen';
    import { checkResultStore } from '../lib/checkResultStore';
    import { formatLanguageNames } from '../lib/languageDisplay';
    import ProfileManager from './ProfileManager.svelte';
    import { getClusters, getTriageFiles, sourceLabel, type Cluster, type TriageFile } from '../lib/preflightDataUtils';

    export let open: boolean = false;
    export let profiles: ExpectationProfile[] = [];
    export let report: ValidationReport | null = null;
    export let selectedProfileName: string = '';

    $: isLite = $liteModeStore.enabled;

    var dispatch = createEventDispatcher<{
        close: void;
        runCheck: void;
        saveProfile: { profile: ExpectationProfile };
        deleteProfile: { name: string };
    }>();

    let activeTriageFilePath = '';

    $: clusters = report ? getClusters(report.issues) : [];
    $: topClusters = clusters.slice(0, 3);
    $: triageFiles = report ? getTriageFiles(report.issues) : [];
    $: passedCount = report ? Math.max(0, report.totalFiles - triageFiles.length) : 0;

    $: {
        if (triageFiles.length === 0) {
            activeTriageFilePath = '';
        } else {
            var exists = false;
            for (var i = 0; i < triageFiles.length; i++) {
                if (triageFiles[i].path === activeTriageFilePath) {
                    exists = true;
                    break;
                }
            }
            if (!exists) activeTriageFilePath = triageFiles[0].path;
        }
    }

    $: activeTriageFile = getActiveTriageFile(triageFiles, activeTriageFilePath);

    function getActiveTriageFile(files: TriageFile[], path: string): TriageFile | null {
        if (!path) return null;
        for (var i = 0; i < files.length; i++) {
            if (files[i].path === path) return files[i];
        }
        return null;
    }

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

    function severityPillClass(kind: 'error' | 'warning' | 'info'): string {
        if (kind === 'error') return 'border-red-400/35 bg-red-500/15 text-red-200';
        if (kind === 'warning') return 'border-amber-400/35 bg-amber-500/15 text-amber-200';
        return 'border-blue-400/35 bg-blue-500/15 text-blue-200';
    }

    function summaryCardClass(kind: 'neutral' | 'error' | 'warning' | 'success'): string {
        if (kind === 'error') return 'text-red-300';
        if (kind === 'warning') return 'text-amber-300';
        if (kind === 'success') return 'text-green-300';
        return 'text-white/90';
    }

    function clusterBorderClass(cluster: Cluster): string {
        if (cluster.severity === 'error') return 'border-l-red-400';
        if (cluster.severity === 'warning') return 'border-l-amber-400';
        return 'border-l-blue-400';
    }

    function clusterSeverityTextClass(cluster: Cluster): string {
        if (cluster.severity === 'error') return 'text-red-300';
        if (cluster.severity === 'warning') return 'text-amber-300';
        return 'text-blue-300';
    }

    function clusterSeverityIcon(cluster: Cluster): string {
        if (cluster.severity === 'error') return 'error';
        if (cluster.severity === 'warning') return 'warning';
        return 'info';
    }

    function statusBadgeClass(status: 'critical' | 'review' | 'clean'): string {
        if (status === 'critical') return 'border-red-400/35 bg-red-500/15 text-red-200';
        if (status === 'review') return 'border-amber-400/35 bg-amber-500/15 text-amber-200';
        return 'border-green-400/35 bg-green-500/15 text-green-200';
    }

    function statusLabel(status: 'critical' | 'review' | 'clean'): string {
        if (status === 'critical') return 'Critical';
        if (status === 'review') return 'Review';
        return 'Clean';
    }

    function issueSeverityClass(issue: ValidationIssue): string {
        var severity = (issue.severity || 'info').toLowerCase();
        if (severity === 'error') return 'border-red-400/35 bg-red-500/10 text-red-200';
        if (severity === 'warning') return 'border-amber-400/35 bg-amber-500/10 text-amber-200';
        return 'border-blue-400/35 bg-blue-500/10 text-blue-200';
    }

    function languageNames(tags: string[]): string[] {
        return formatLanguageNames(tags);
    }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open}
    <div class="fixed inset-0 z-50 {isLite ? 'bg-black/70' : 'backdrop-blur-lg bg-black/30'}"
         transition:fade={{ duration: isLite ? 0 : 200 }}
         on:click={handleClose}>
    </div>

    <div class="fixed inset-0 z-50 overflow-y-auto" on:click|stopPropagation>
        <div class="container mx-auto max-w-6xl p-4 min-h-screen flex items-center"
             transition:slide={{ duration: isLite ? 0 : 220 }}>
            <div class="relative w-full rounded-xl border border-white/10 shadow-2xl overflow-hidden
                        {isLite ? 'bg-bgold-900/95' : 'bg-bgold-900/80 backdrop-blur-xl'}">

                <div class="flex items-center justify-between px-5 py-3 border-b border-white/10">
                    <div>
                        <h2 class="text-lg font-medium text-white/90">Library Diagnostics</h2>
                        <p class="text-xs text-white/45">
                            Audit files and tune expectations before processing.
                        </p>
                    </div>
                    <div class="flex items-center gap-2">
                        <button
                            class="px-3 py-1.5 text-xs rounded-md border border-white/10 bg-white/5
                                   text-white/70 hover:bg-white/10 hover:text-white/85 transition-colors"
                            on:click={() => dispatch('runCheck')}
                        >
                            <span class="material-icons text-sm align-middle mr-1">refresh</span>
                            Re-run Check
                        </button>
                        <button
                            class="w-8 h-8 flex items-center justify-center rounded-md
                                   text-white/40 hover:text-white/80 hover:bg-white/10 transition-colors"
                            on:click={handleClose}
                        >
                            <span class="material-icons text-base">close</span>
                        </button>
                    </div>
                </div>

                <div class="flex min-h-[68vh] max-h-[86vh]">
                    <div class="w-80 shrink-0 border-r border-white/10 p-4 overflow-y-auto space-y-3
                                {isLite ? 'bg-white/5' : 'bg-white/5 backdrop-blur-sm'}">
                        <div class="rounded-lg border border-white/10 bg-white/5 p-3">
                            <div class="text-[11px] text-white/50 uppercase tracking-wide mb-1">
                                Profile Workspace
                            </div>
                            <div class="text-xs text-white/65">
                                Create, edit, and delete expectation profiles.
                            </div>
                            <div class="mt-2 text-[11px] text-white/50">
                                Saved profiles: {profiles.length}
                            </div>
                        </div>

                        <ProfileManager bind:selectedProfileName />
                    </div>

                    <div class="relative flex-1 p-4 overflow-y-auto transition-opacity"
                         class:opacity-50={$checkResultStore.stale}
                         class:pointer-events-none={$checkResultStore.stale}>
                        {#if $checkResultStore.isRunning && !report}
                            <div class="h-full flex items-center justify-center">
                                <div class="text-center">
                                    <span class="material-icons text-white/50 animate-spin">refresh</span>
                                    <div class="mt-2 text-sm text-white/60">Checking library...</div>
                                </div>
                            </div>
                        {:else if report}
                            <div class="space-y-3 pb-2">
                                <section class="grid grid-cols-1 md:grid-cols-4 gap-3">
                                    <article class="rounded-xl border border-white/10 bg-white/5 p-4">
                                        <div class="text-[11px] text-white/50 uppercase tracking-wide">Total Files</div>
                                        <div class="mt-2 text-2xl font-semibold {summaryCardClass('neutral')}">
                                            {report.totalFiles}
                                        </div>
                                    </article>
                                    <article class="rounded-xl border border-white/10 bg-white/5 p-4">
                                        <div class="text-[11px] text-white/50 uppercase tracking-wide">Errors</div>
                                        <div class="mt-2 text-2xl font-semibold {summaryCardClass('error')}">
                                            {report.errorCount}
                                        </div>
                                    </article>
                                    <article class="rounded-xl border border-white/10 bg-white/5 p-4">
                                        <div class="text-[11px] text-white/50 uppercase tracking-wide">Warnings</div>
                                        <div class="mt-2 text-2xl font-semibold {summaryCardClass('warning')}">
                                            {report.warningCount}
                                        </div>
                                    </article>
                                    <article class="rounded-xl border border-white/10 bg-white/5 p-4">
                                        <div class="text-[11px] text-white/50 uppercase tracking-wide">Passed</div>
                                        <div class="mt-2 text-2xl font-semibold {summaryCardClass('success')}">
                                            {passedCount}
                                        </div>
                                    </article>
                                </section>

                                <section class="grid grid-cols-1 xl:grid-cols-[1.2fr_1fr] gap-3">
                                    <article class="rounded-xl border border-white/10 bg-white/5 p-3">
                                        <div class="text-[11px] text-white/50 uppercase tracking-wide mb-2">
                                            Top Problem Clusters
                                        </div>

                                        {#if topClusters.length === 0}
                                            <div class="text-xs text-white/50">No clustered findings</div>
                                        {:else}
                                            <div class="space-y-2">
                                                {#each topClusters as cluster}
                                                    <div class="rounded-lg border border-white/10 border-l-4 {clusterBorderClass(cluster)} bg-black/20 p-2.5">
                                                        <div class="flex items-center justify-between gap-2">
                                                            <div class="min-w-0">
                                                                <div class="text-sm text-white/90 font-medium truncate">
                                                                    {cluster.label}
                                                                </div>
                                                                {#if cluster.fileCount > 1}
                                                                    <div class="text-[11px] text-white/55">
                                                                        {cluster.fileCount} files · {cluster.issueCount} findings
                                                                    </div>
                                                                {/if}
                                                            </div>
                                                            <div class="flex items-center gap-1.5 text-[11px]">
                                                                <span class="material-icons text-sm {clusterSeverityTextClass(cluster)}">
                                                                    {clusterSeverityIcon(cluster)}
                                                                </span>
                                                                <span class="px-1.5 py-0.5 rounded-full border border-white/15 bg-white/10 text-white/70">
                                                                    {sourceLabel(cluster.source)}
                                                                </span>
                                                            </div>
                                                        </div>
                                                        {#if cluster.files.length > 0}
                                                            <div class="mt-1 text-left">
                                                                {#if cluster.fileCount === 1}
                                                                    <div class="text-[11px] text-white/60 leading-4">
                                                                        {cluster.files[0].name}
                                                                    </div>
                                                                {:else}
                                                                    <div class="text-[10px] uppercase tracking-wide text-white/45 mb-0.5">
                                                                        Examples
                                                                    </div>
                                                                    {#each cluster.files.slice(0, 3) as file}
                                                                        <div class="text-[11px] text-white/60 leading-4">
                                                                            {file.name}
                                                                        </div>
                                                                    {/each}
                                                                    {#if cluster.files.length > 3}
                                                                        <div class={clusterSeverityTextClass(cluster) + ' text-[11px]'}>
                                                                            +{cluster.files.length - 3} more
                                                                        </div>
                                                                    {/if}
                                                                {/if}
                                                            </div>
                                                        {/if}
                                                    </div>
                                                {/each}
                                            </div>
                                        {/if}
                                    </article>

                                    <article class="rounded-xl border border-white/10 bg-white/5 p-3">
                                        <div class="text-[11px] text-white/50 uppercase tracking-wide mb-2">
                                            Consensus Snapshot
                                        </div>

                                        {#if report.consensusSummaries && report.consensusSummaries.length > 0}
                                            <div class="space-y-2">
                                                {#each report.consensusSummaries as cs}
                                                    <div class="rounded-lg border border-white/10 bg-black/20 p-2.5">
                                                        <div class="text-xs text-white/80 font-medium mb-1">
                                                            {basename(cs.directory)} ({cs.fileCount} files)
                                                        </div>
                                                        <div class="space-y-2">
                                                            {#if cs.consensusAudioLangs && cs.consensusAudioLangs.length > 0}
                                                                <div>
                                                                    <div class="text-[10px] uppercase tracking-wide text-primary/80 mb-1">Audio</div>
                                                                    <div class="flex flex-wrap gap-1">
                                                                        {#each languageNames(cs.consensusAudioLangs) as name}
                                                                            <span class="text-[10px] px-2 py-0.5 rounded-full border border-primary/30 bg-primary/15 text-primary">
                                                                                {name}
                                                                            </span>
                                                                        {/each}
                                                                    </div>
                                                                </div>
                                                            {/if}
                                                            {#if cs.consensusSubLangs && cs.consensusSubLangs.length > 0}
                                                                <div>
                                                                    <div class="text-[10px] uppercase tracking-wide text-secondary/80 mb-1">Subs</div>
                                                                    <div class="flex flex-wrap gap-1">
                                                                        {#each languageNames(cs.consensusSubLangs) as name}
                                                                            <span class="text-[10px] px-2 py-0.5 rounded-full border border-secondary/30 bg-secondary/15 text-secondary">
                                                                                {name}
                                                                            </span>
                                                                        {/each}
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
                                        {:else}
                                            <div class="text-xs text-white/50">No consensus summaries available</div>
                                        {/if}
                                    </article>
                                </section>

                                <section class="rounded-xl border border-white/10 bg-white/5 p-3">
                                    <div class="flex items-center justify-between gap-2 mb-2">
                                        <h3 class="text-sm font-medium text-white/90">File Triage (fix first)</h3>
                                        <span class="text-[11px] text-white/50">Sorted by severity score</span>
                                    </div>

                                    <div class="overflow-x-auto rounded-lg border border-white/10">
                                        <table class="w-full text-xs">
                                            <thead class="bg-black/20 text-white/70">
                                                <tr>
                                                    <th class="text-left px-3 py-2 font-medium">File Name</th>
                                                    <th class="text-left px-3 py-2 font-medium">Status</th>
                                                    <th class="text-left px-3 py-2 font-medium">Errors</th>
                                                    <th class="text-left px-3 py-2 font-medium">Warnings</th>
                                                    <th class="text-left px-3 py-2 font-medium">Top Issue Summary</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {#if triageFiles.length === 0}
                                                    <tr>
                                                        <td class="px-3 py-3 text-white/45" colspan="5">No files with findings</td>
                                                    </tr>
                                                {:else}
                                                    {#each triageFiles as triage}
                                                        <tr
                                                            class={(triage.path === activeTriageFilePath
                                                                ? 'border-t border-white/10 cursor-pointer transition-colors hover:bg-white/5 bg-primary/10'
                                                                : 'border-t border-white/10 cursor-pointer transition-colors hover:bg-white/5')}
                                                            on:click={() => activeTriageFilePath = triage.path}>
                                                            <td class="px-3 py-2.5 font-mono text-white/80">{triage.name}</td>
                                                            <td class="px-3 py-2.5">
                                                                <span class="inline-flex px-2 py-0.5 rounded-full border text-[10px] uppercase tracking-wide {statusBadgeClass(triage.status)}">
                                                                    {statusLabel(triage.status)}
                                                                </span>
                                                            </td>
                                                            <td class="px-3 py-2.5 text-red-200">{triage.errorCount}</td>
                                                            <td class="px-3 py-2.5 text-amber-200">{triage.warningCount}</td>
                                                            <td class="px-3 py-2.5 text-white/65">{triage.topIssueSummary}</td>
                                                        </tr>
                                                    {/each}
                                                {/if}
                                            </tbody>
                                        </table>
                                    </div>

                                    <div class="mt-3 rounded-lg border border-white/10 bg-black/20 p-3">
                                        {#if activeTriageFile}
                                            <div class="text-sm text-white/90 font-medium mb-2">
                                                Details: {activeTriageFile.name}
                                            </div>
                                            <div class="space-y-1.5 max-h-52 overflow-y-auto pr-1">
                                                {#each activeTriageFile.issues as issue}
                                                    <div class="rounded-md border px-2 py-1.5 text-xs {issueSeverityClass(issue)}">
                                                        <span class="mr-2 inline-flex rounded-full border border-white/20 px-1.5 py-0.5 text-[10px] uppercase tracking-wide">
                                                            {issue.severity}
                                                        </span>
                                                        {issue.message}
                                                    </div>
                                                {/each}
                                            </div>
                                        {:else}
                                            <div class="text-xs text-white/50">Select a row to inspect file-level details</div>
                                        {/if}
                                    </div>
                                </section>
                            </div>
                        {:else}
                            <div class="flex items-center justify-center h-full text-sm text-white/40">
                                Click "Re-run Check" to scan your library
                            </div>
                        {/if}
                    </div>

                    {#if $checkResultStore.stale}
                        <div class={(isLite ? 'bg-black/70' : 'bg-black/55 backdrop-blur-sm') + ' absolute inset-y-0 right-0 left-[20rem] flex items-center justify-center p-4'}>
                            <div class="max-w-sm w-full rounded-lg border border-amber-400/25 bg-amber-500/10 p-4 text-center"
                                 transition:slide={{ duration: isLite ? 0 : 200 }}>
                                <div class="text-sm text-amber-200 font-medium mb-1">Results are out of date</div>
                                <div class="text-xs text-amber-200/80 mb-3">
                                    Profile or check settings changed. Re-run to refresh diagnostics.
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
            </div>
        </div>
    </div>
{/if}
