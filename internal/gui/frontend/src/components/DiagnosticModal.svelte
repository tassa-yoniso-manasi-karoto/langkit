<script lang="ts">
    import { createEventDispatcher } from 'svelte';
    import { fade, slide } from 'svelte/transition';
    import { liteModeStore } from '../lib/stores';
    import type { ValidationReport, ExpectationProfile, ValidationIssue } from '../api/generated/api.gen';
    import { checkResultStore } from '../lib/checkResultStore';
    import { formatLanguageNames } from '../lib/languageDisplay';
    import ProfileManager from './ProfileManager.svelte';
    import CloseButton from './CloseButton.svelte';
    import {
        getClusters, getTriageFiles, sourceLabel, codeLabelMap,
        normalizeSeverity, severityRank,
        type Cluster, type TriageFile,
    } from '../lib/preflightDataUtils';

    export let open: boolean = false;
    export let profiles: ExpectationProfile[] = [];
    export let report: ValidationReport | null = null;
    export let selectedProfileName: string = '';
    export let mediaPath: string = '';

    $: isLite = $liteModeStore.enabled;

    var dispatch = createEventDispatcher<{
        close: void;
        runCheck: void;
        saveProfile: { profile: ExpectationProfile };
        deleteProfile: { name: string };
    }>();

    let activeTriageFilePath = '';

    $: clusters = report ? getClusters(report.issues) : [];
    $: topClusters = clusters.slice(0, 5);
    $: triageFiles = report ? getTriageFiles(report.issues) : [];

    $: maxClusterFiles = (function() {
        var max = 1;
        for (var i = 0; i < topClusters.length; i++) {
            if (topClusters[i].fileCount > max) max = topClusters[i].fileCount;
        }
        return max;
    })();

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
    $: activeIssueGroups = activeTriageFile ? groupIssuesByCode(activeTriageFile.issues) : [];

    function getActiveTriageFile(files: TriageFile[], path: string): TriageFile | null {
        if (!path) return null;
        for (var i = 0; i < files.length; i++) {
            if (files[i].path === path) return files[i];
        }
        return null;
    }

    // Group a single file's issues by issueCode for the tree view
    interface IssueItem {
        message: string;
        subjectLabel: string;
        messagePrefix: string;
        messageSuffix: string;
    }

    interface IssueCodeGroup {
        code: string;
        label: string;
        severity: string;
        count: number;
        items: IssueItem[];
    }

    function groupIssuesByCode(issues: ValidationIssue[]): IssueCodeGroup[] {
        var byCode: Record<string, { issues: ValidationIssue[]; severity: string }> = {};
        var order: string[] = [];
        for (var i = 0; i < issues.length; i++) {
            var code = issues[i].issueCode || 'unknown';
            if (!byCode[code]) {
                byCode[code] = { issues: [], severity: issues[i].severity || 'info' };
                order.push(code);
            }
            byCode[code].issues.push(issues[i]);
            if (severityRank(issues[i].severity) < severityRank(byCode[code].severity)) {
                byCode[code].severity = issues[i].severity || 'info';
            }
        }
        var groups: IssueCodeGroup[] = [];
        for (var j = 0; j < order.length; j++) {
            var c = order[j];
            var entry = byCode[c];
            var seen: Record<string, boolean> = {};
            var items: IssueItem[] = [];
            for (var k = 0; k < entry.issues.length; k++) {
                var msg = entry.issues[k].message || '';
                if (msg && !seen[msg]) {
                    seen[msg] = true;
                    items.push({
                        message: msg,
                        subjectLabel: entry.issues[k].subjectLabel || '',
                        messagePrefix: entry.issues[k].messagePrefix || '',
                        messageSuffix: entry.issues[k].messageSuffix || '',
                    });
                }
            }
            groups.push({
                code: c,
                label: codeLabelMap[c] || humanize(c),
                severity: normalizeSeverity(entry.severity),
                count: entry.issues.length,
                items: items,
            });
        }
        groups.sort(function(a, b) {
            return severityRank(a.severity) - severityRank(b.severity);
        });
        return groups;
    }

    function humanize(code: string): string {
        var parts = code.split('_');
        var result = '';
        for (var i = 0; i < parts.length; i++) {
            if (i > 0) result += ' ';
            result += parts[i].charAt(0).toUpperCase() + parts[i].slice(1);
        }
        return result || 'Unknown';
    }

    // Episode-aware filename truncation: preserves "S02E31" markers
    // while truncating the series title prefix. Returns structured
    // parts so the episode marker can be styled independently.
    interface BasenameParts {
        prefix: string;
        episode: string;
        suffix: string;
    }

    var episodePattern = /(?:S\d+E\d+|E\d+|\d+x\d+)/i;

    function smartBasename(name: string): BasenameParts {
        var match = episodePattern.exec(name);
        if (!match || match.index === undefined) {
            return { prefix: name, episode: '', suffix: '' };
        }

        var markerStart = match.index;
        var prefix = name.slice(0, markerStart);
        var episode = match[0];
        var suffix = name.slice(markerStart + episode.length);

        // Clean separators at boundaries
        prefix = prefix.replace(/[\s._-]+$/, '');
        suffix = suffix.replace(/^[\s._-]+/, '');

        var maxPrefix = 18;
        if (prefix.length > maxPrefix) {
            prefix = prefix.slice(0, maxPrefix) + '\u2026';
        }

        var maxSuffix = 24;
        if (suffix.length > maxSuffix) {
            suffix = suffix.slice(0, maxSuffix) + '\u2026';
        }

        return { prefix: prefix, episode: episode, suffix: suffix };
    }

    // Severity dots for file list (one per issue code, sorted by severity)
    function fileSeverityDots(issues: ValidationIssue[]): string[] {
        var byCode: Record<string, string> = {};
        var order: string[] = [];
        for (var i = 0; i < issues.length; i++) {
            var code = issues[i].issueCode || 'unknown';
            if (!byCode[code]) {
                byCode[code] = issues[i].severity || 'info';
                order.push(code);
            } else if (severityRank(issues[i].severity) < severityRank(byCode[code])) {
                byCode[code] = issues[i].severity || 'info';
            }
        }
        var dots: { sev: string; rank: number }[] = [];
        for (var j = 0; j < order.length; j++) {
            dots.push({ sev: normalizeSeverity(byCode[order[j]]), rank: severityRank(byCode[order[j]]) });
        }
        dots.sort(function(a, b) { return a.rank - b.rank; });
        var result: string[] = [];
        for (var k = 0; k < Math.min(dots.length, 4); k++) {
            result.push(dots[k].sev);
        }
        return result;
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

    function dotBgClass(severity: string): string {
        var s = normalizeSeverity(severity);
        if (s === 'error') return 'bg-red-400';
        if (s === 'warning') return 'bg-amber-400';
        return 'bg-blue-400';
    }

    function dotColorStyle(severity: string): string {
        var s = normalizeSeverity(severity);
        if (s === 'error') return 'background-color: #f87171';
        if (s === 'warning') return 'background-color: #fbbf24';
        return 'background-color: #60a5fa';
    }

    function impactFillClass(severity: string): string {
        var s = normalizeSeverity(severity);
        if (s === 'error') return 'bg-red-400';
        if (s === 'warning') return 'bg-amber-400';
        return 'bg-blue-400';
    }

    function statusBadgeClass(status: string): string {
        if (status === 'critical') return 'border-red-400/35 bg-red-500/15 text-red-200';
        if (status === 'review') return 'border-amber-400/35 bg-amber-500/15 text-amber-200';
        return 'border-green-400/35 bg-green-500/15 text-green-200';
    }

    function statusText(status: string): string {
        if (status === 'critical') return 'Critical';
        if (status === 'review') return 'Review';
        return 'Clean';
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

    <div class="fixed inset-0 z-50 overflow-y-auto" on:click={handleClose}>
        <div class="container mx-auto max-w-7xl p-4 min-h-screen flex items-center"
             transition:slide={{ duration: isLite ? 0 : 220 }}>
            <div class="relative w-full rounded-xl border border-white/10 shadow-2xl overflow-hidden
                        {isLite ? 'bg-bgold-900/95' : 'bg-bgold-900/80 backdrop-blur-xl'}"
                 on:click|stopPropagation>

                <!-- Header -->
                <div class="flex items-center justify-between px-5 py-3 border-b border-white/10">
                    <div>
                        <h2 class="text-lg font-semibold text-white/90">Library Diagnostics</h2>
                        {#if mediaPath}
                            <p class="text-xs text-white/35 font-mono mt-0.5">{mediaPath}</p>
                        {/if}
                    </div>
                    <div class="flex items-center gap-2">
                        <button
                            class="px-3 py-1.5 text-sm rounded-md border border-white/10 bg-white/5
                                   text-white/70 hover:bg-white/10 hover:text-white/85 transition-colors
                                   disabled:opacity-40 disabled:cursor-not-allowed"
                            disabled={$checkResultStore.isRunning}
                            on:click={() => dispatch('runCheck')}
                        >
                            <span class="material-icons text-sm align-middle mr-1">{$checkResultStore.isRunning ? 'hourglass_empty' : 'refresh'}</span>
                            {$checkResultStore.isRunning ? 'Checking...' : 'Re-run Check'}
                        </button>
                        <CloseButton size="sm" on:click={handleClose} />
                    </div>
                </div>

                <!-- Body -->
                <div class="flex min-h-[68vh] max-h-[86vh]">
                    <!-- Left sidebar: Profile Manager -->
                    <div class="w-72 shrink-0 border-r border-white/10 p-4 overflow-y-auto
                                {isLite ? 'bg-white/5' : 'bg-white/5 backdrop-blur-sm'}">
                        <ProfileManager bind:selectedProfileName />
                    </div>

                    <!-- Right: Results -->
                    <div class="relative flex-1 p-4 overflow-y-auto">

                        {#if $checkResultStore.isRunning && !report}
                            <div class="h-full flex items-center justify-center">
                                <div class="text-center">
                                    <span class="material-icons text-white/50 animate-spin">refresh</span>
                                    <div class="mt-2 text-sm text-white/60">Checking library...</div>
                                </div>
                            </div>
                        {:else if report}
                            <div class="space-y-3.5 pb-2">

                                {#if $checkResultStore.stale}
                                    <div class="flex items-center gap-2.5 px-3.5 py-2.5 rounded-lg border border-amber-400/35 bg-amber-500/[0.12]"
                                         transition:slide={{ duration: isLite ? 0 : 150 }}>
                                        <span class="material-icons text-amber-300" style="font-size:18px;">info</span>
                                        <span class="text-sm text-amber-200/90 flex-1">Results may be out of date</span>
                                        <button
                                            class="px-3 py-1 text-xs font-medium rounded-md border border-amber-400/35
                                                   bg-amber-500/20 text-amber-200 hover:bg-amber-500/30 transition-colors"
                                            on:click={() => dispatch('runCheck')}
                                        >
                                            <span class="material-icons text-xs align-middle mr-0.5">refresh</span>
                                            Re-run
                                        </button>
                                    </div>
                                {/if}

                                <!-- Telemetry strip -->
                                <div class="flex rounded-xl border border-white/10 bg-white/5 overflow-hidden">
                                    <div class="flex-1 px-4 py-3 border-r border-white/[0.06]">
                                        <div class="text-[11px] uppercase tracking-[0.08em] text-white/35 font-medium">Total Files</div>
                                        <div class="text-3xl font-semibold text-white/90 mt-1">{report.totalFiles}</div>
                                    </div>
                                    <div class="flex-1 px-4 py-3 border-r border-white/[0.06] bg-red-500/[0.04]">
                                        <div class="text-[11px] uppercase tracking-[0.08em] text-red-300/60 font-medium">Errors</div>
                                        <div class="text-3xl font-semibold text-red-300 mt-1">{report.errorCount}</div>
                                    </div>
                                    <div class="flex-1 px-4 py-3 border-r border-white/[0.06] bg-amber-500/[0.04]">
                                        <div class="text-[11px] uppercase tracking-[0.08em] text-amber-300/60 font-medium">Warnings</div>
                                        <div class="text-3xl font-semibold text-amber-300 mt-1">{report.warningCount}</div>
                                    </div>
                                    <div class="flex-1 px-4 py-3 bg-blue-500/[0.04]">
                                        <div class="text-[11px] uppercase tracking-[0.08em] text-blue-300/60 font-medium">Info</div>
                                        <div class="text-3xl font-semibold text-blue-300 mt-1">{report.infoCount}</div>
                                    </div>
                                </div>

                                <!-- Clusters + Consensus bento -->
                                <div class="grid grid-cols-1 xl:grid-cols-[1.3fr_1fr] gap-3">
                                    <!-- Top Problem Clusters (with impact bars) -->
                                    <div class={'rounded-xl border bg-white/5 p-3 '
                                        + (report.errorCount > 0
                                            ? 'border-red-400/20 border-l-2 border-l-red-400/40 shadow-[0_0_20px_rgba(248,113,113,0.06)]'
                                            : report.warningCount > 0
                                                ? 'border-amber-400/20 border-l-2 border-l-amber-400/40'
                                                : 'border-white/10')}>
                                        <div class="text-[11px] uppercase tracking-[0.08em] text-white/35 font-medium mb-2">
                                            Top Problem Clusters
                                        </div>
                                        {#if topClusters.length === 0}
                                            <div class="text-sm text-white/40">No clustered findings</div>
                                        {:else}
                                            <div class="space-y-1.5">
                                                {#each topClusters as cluster}
                                                    <div class="flex items-center gap-2.5 px-2.5 py-2 rounded-md border border-white/[0.07] bg-white/[0.03] hover:bg-white/[0.06] transition-colors">
                                                        <div class={'w-1.5 h-1.5 rounded-full shrink-0 ' + dotBgClass(cluster.severity)}
                                                             class:severity-dot-glow={cluster.severity === 'error'}></div>
                                                        <div class="flex-1 min-w-0">
                                                            <div class="text-base font-medium text-white/85 truncate">{cluster.label}</div>
                                                            <div class="text-xs text-white/35">
                                                                {cluster.fileCount} file{cluster.fileCount !== 1 ? 's' : ''} · {sourceLabel(cluster.source)}
                                                            </div>
                                                        </div>
                                                        <div class="w-20 h-1 rounded-full bg-black/40 overflow-hidden shrink-0">
                                                            <div class={'h-full rounded-full ' + impactFillClass(cluster.severity)}
                                                                 style={'width: ' + Math.round(cluster.fileCount / maxClusterFiles * 100) + '%'}></div>
                                                        </div>
                                                        <span class="font-mono text-xs text-white/55 w-8 text-right shrink-0">{cluster.fileCount}</span>
                                                    </div>
                                                {/each}
                                            </div>
                                        {/if}
                                    </div>

                                    <!-- Consensus Snapshot -->
                                    <div class="rounded-xl border border-white/10 bg-white/5 p-3">
                                        <div class="text-[11px] uppercase tracking-[0.08em] text-white/35 font-medium mb-2">
                                            Consensus Snapshot
                                        </div>
                                        {#if report.consensusSummaries && report.consensusSummaries.length > 0}
                                            <div class="space-y-2">
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
                                                                    {#each languageNames(cs.consensusAudioLangs) as name}
                                                                        <span class="text-[11px] px-2 py-0.5 rounded-full border border-primary/30 bg-primary/15 text-primary">{name}</span>
                                                                    {/each}
                                                                </div>
                                                            </div>
                                                        {/if}
                                                        {#if cs.consensusSubLangs && cs.consensusSubLangs.length > 0}
                                                            <div class="flex items-baseline gap-1.5 mb-1">
                                                                <span class="text-[11px] uppercase tracking-[0.08em] text-secondary/80 font-medium w-12 shrink-0">Subs</span>
                                                                <div class="flex flex-wrap gap-1">
                                                                    {#each languageNames(cs.consensusSubLangs) as name}
                                                                        <span class="text-[11px] px-2 py-0.5 rounded-full border border-secondary/30 bg-secondary/15 text-secondary">{name}</span>
                                                                    {/each}
                                                                </div>
                                                            </div>
                                                        {/if}
                                                        {#if cs.medianDurationSec > 0 || cs.consensusAudioTrackCount >= 0}
                                                            <div class="flex items-center gap-2 mt-4 text-xs text-white/55">
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
                                        {:else}
                                            <div class="text-sm text-white/40">No consensus summaries available</div>
                                        {/if}
                                    </div>
                                </div>

                                <!-- File Triage — Master-Detail split -->
                                <div class="rounded-xl border border-white/10 bg-white/5 overflow-hidden">
                                    <div class="flex items-center justify-between px-3.5 py-2.5">
                                        <h3 class="text-base font-semibold text-white/85">File Triage</h3>
                                        <span class="text-[11px] text-white/35">Select a file to inspect · sorted by severity</span>
                                    </div>
                                    <div class="flex border-t border-white/[0.06]" style="min-height: 240px; max-height: 320px;">
                                        <!-- Left: File list -->
                                        <div class="w-[38%] border-r border-white/10 overflow-y-auto bg-black/20">
                                            {#if triageFiles.length === 0}
                                                <div class="px-3 py-4 text-sm text-white/40">No files with findings</div>
                                            {:else}
                                                {#each triageFiles as triage}
                                                    {@const bn = smartBasename(triage.name)}
                                                    <button
                                                        class={'w-full flex items-center justify-between px-3 py-2 '
                                                            + 'border-b border-white/[0.06] border-l-2 text-left transition-colors '
                                                            + (triage.path === activeTriageFilePath
                                                                ? 'bg-primary/[0.08] border-l-primary text-white/90'
                                                                : 'border-l-transparent text-white/55 hover:bg-white/5')}
                                                        on:click={() => activeTriageFilePath = triage.path}
                                                    >
                                                        <span class="text-xs truncate" title={triage.name}
                                                            >{#if bn.episode}<span class="text-white/50">{bn.prefix}</span>{' '}<span class="font-mono font-medium text-white/75">{bn.episode}</span>{#if bn.suffix}{' '}<span class="text-white/50">{bn.suffix}</span>{/if}{:else}{triage.name}{/if}</span>
                                                        <div class="flex gap-1 shrink-0 ml-2">
                                                            {#each fileSeverityDots(triage.issues) as sev}
                                                                <div class="w-1.5 h-1.5 rounded-full" style={dotColorStyle(sev)}></div>
                                                            {/each}
                                                        </div>
                                                    </button>
                                                {/each}
                                            {/if}
                                        </div>

                                        <!-- Right: Detail pane (issue tree) -->
                                        <div class="flex-1 p-3.5 overflow-y-auto bg-black/[0.35]">
                                            {#if activeTriageFile}
                                                {@const hdr = smartBasename(activeTriageFile.name)}
                                                <div class="flex items-center gap-2 mb-3">
                                                    <span class="text-xs text-white/60" title={activeTriageFile.name}
                                                        >{#if hdr.episode}<span class="text-white/45">{hdr.prefix}</span>{' '}<span class="font-mono font-medium text-white/70">{hdr.episode}</span>{#if hdr.suffix}{' '}<span class="text-white/45">{hdr.suffix}</span>{/if}{:else}{activeTriageFile.name}{/if}</span>
                                                    <span class="text-[10px] px-2 py-0.5 rounded-full border font-medium {statusBadgeClass(activeTriageFile.status)}">
                                                        {statusText(activeTriageFile.status)}
                                                    </span>
                                                </div>
                                                <div class="space-y-3.5">
                                                    {#each activeIssueGroups as group}
                                                        <div>
                                                            <div class="flex items-center gap-1.5 text-xs font-medium">
                                                                <div class={'w-[5px] h-[5px] rounded-full shrink-0 ' + dotBgClass(group.severity)}></div>
                                                                <span class="text-white/85">{group.label}</span>
                                                                <span class="text-white/40 font-normal">× {group.count}</span>
                                                            </div>
                                                            <div class="mt-1.5 space-y-1.5">
                                                                {#each group.items as item}
                                                                    <div class="ml-3 pl-2.5 py-0.5 text-xs text-white/65 leading-relaxed border-l border-white/[0.08]">
                                                                        {#if item.subjectLabel}{item.messagePrefix}<code class="inline-code">{item.subjectLabel}</code>{item.messageSuffix}{:else}{item.message}{/if}
                                                                    </div>
                                                                {/each}
                                                            </div>
                                                        </div>
                                                    {/each}
                                                </div>
                                            {:else}
                                                <div class="text-sm text-white/40">Select a file to inspect</div>
                                            {/if}
                                        </div>
                                    </div>
                                </div>

                            </div>
                        {:else}
                            <div class="flex items-center justify-center h-full text-sm text-white/40">
                                Click "Re-run Check" to scan your library
                            </div>
                        {/if}
                    </div>

                </div>
            </div>
        </div>
    </div>
{/if}

<style>
    .severity-dot-glow {
        box-shadow: 0 0 8px rgba(248, 113, 113, 0.6);
    }
    .inline-code {
        font-family: 'DM Mono', monospace;
        font-size: 0.7rem;
        padding: 0.1rem 0.35rem;
        border-radius: 0.25rem;
        background: rgba(255, 255, 255, 0.08);
        border: 1px solid rgba(255, 255, 255, 0.1);
        color: rgba(255, 255, 255, 0.75);
    }
</style>
