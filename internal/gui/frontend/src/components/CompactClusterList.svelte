<script lang="ts">
    import { slide } from 'svelte/transition';
    import { liteModeStore } from '../lib/stores';
    import type { ValidationIssue } from '../api/generated/api.gen';
    import {
        categoryLabel,
        getAvailableCategories,
        getClusters,
        sourceLabel,
        type Cluster,
    } from '../lib/preflightDataUtils';

    export let issues: ValidationIssue[] = [];
    export let compact: boolean = true;
    $: isLite = $liteModeStore.enabled;

    $: clusters = getClusters(issues);
    $: availableCategories = getAvailableCategories(clusters);
    $: filteredClusters = clusters.filter(function(cluster) {
        if (activeCategories.length === 0) return true;
        return activeCategories.indexOf(cluster.category) >= 0;
    });

    let activeCategories: string[] = [];
    let clusterOpen: Record<string, boolean> = {};
    let compactExpandedFiles: Record<string, boolean> = {};
    let lastClusterKey = '';

    $: {
        var nextKey = filteredClusters.map(function(cluster) {
            return cluster.code + ':' + cluster.fileCount;
        }).join('|') + '|' + String(compact);

        if (nextKey !== lastClusterKey) {
            lastClusterKey = nextKey;
            var nextOpen: Record<string, boolean> = {};
            var nextExpandedFiles: Record<string, boolean> = {};
            for (var i = 0; i < filteredClusters.length; i++) {
                var code = filteredClusters[i].code;
                if (clusterOpen[code] === undefined) {
                    nextOpen[code] = !compact;
                } else {
                    nextOpen[code] = clusterOpen[code];
                }
                nextExpandedFiles[code] = compactExpandedFiles[code] === true;
            }
            clusterOpen = nextOpen;
            compactExpandedFiles = nextExpandedFiles;
        }
    }

    function severityStripeClass(severity: string): string {
        var normalized = (severity || 'info').toLowerCase();
        if (normalized === 'error') return 'bg-red-400';
        if (normalized === 'warning') return 'bg-amber-400';
        return 'bg-blue-400';
    }

    function severityTextClass(severity: string): string {
        var normalized = (severity || 'info').toLowerCase();
        if (normalized === 'error') return 'text-red-300';
        if (normalized === 'warning') return 'text-amber-300';
        return 'text-blue-300';
    }

    function severityIcon(severity: string): string {
        var normalized = (severity || 'info').toLowerCase();
        if (normalized === 'error') return 'error';
        if (normalized === 'warning') return 'warning';
        return 'info';
    }

    function sourceClass(source: string): string {
        if (source === 'profile') return 'border-primary/30 bg-primary/15 text-primary';
        if (source === 'auto') return 'border-secondary/30 bg-secondary/15 text-secondary';
        if (source === 'structural') return 'border-white/20 bg-white/10 text-white/70';
        if (source === 'mixed') return 'border-amber-400/35 bg-amber-400/15 text-amber-200';
        return 'border-white/20 bg-white/10 text-white/60';
    }

    function toggleCategory(category: string) {
        if (activeCategories.indexOf(category) >= 0) {
            activeCategories = activeCategories.filter(function(c) { return c !== category; });
        } else {
            activeCategories = activeCategories.concat([category]);
        }
    }

    function clearCategoryFilters() {
        activeCategories = [];
    }

    function hasActiveCategoryFilters(): boolean {
        return activeCategories.length > 0;
    }

    function isCategoryActive(category: string): boolean {
        if (activeCategories.length === 0) return true;
        return activeCategories.indexOf(category) >= 0;
    }

    function toggleCluster(code: string) {
        clusterOpen = {
            ...clusterOpen,
            [code]: !clusterOpen[code],
        };
    }

    function toggleCompactFileExpansion(code: string) {
        compactExpandedFiles = {
            ...compactExpandedFiles,
            [code]: !compactExpandedFiles[code],
        };
    }

    function visibleFiles(cluster: Cluster) {
        if (!compact) return cluster.files;
        if (compactExpandedFiles[cluster.code]) return cluster.files;
        return cluster.files.slice(0, 5);
    }

    function hiddenFileCount(cluster: Cluster): number {
        if (!compact || compactExpandedFiles[cluster.code]) return 0;
        if (cluster.files.length <= 5) return 0;
        return cluster.files.length - 5;
    }

    function uniqueMessages(fileIssues: ValidationIssue[]): string[] {
        var seen: Record<string, boolean> = {};
        var out: string[] = [];
        for (var i = 0; i < fileIssues.length; i++) {
            var msg = fileIssues[i].message || '';
            if (!msg || seen[msg]) continue;
            seen[msg] = true;
            out.push(msg);
        }
        return out;
    }

    function firstUniqueMessage(fileIssues: ValidationIssue[]): string {
        var unique = uniqueMessages(fileIssues);
        if (unique.length === 0) return '';
        return unique[0];
    }

    function additionalUniqueMessageCount(fileIssues: ValidationIssue[]): number {
        var unique = uniqueMessages(fileIssues);
        if (unique.length <= 1) return 0;
        return unique.length - 1;
    }
</script>

<div class="space-y-2">
    {#if availableCategories.length > 0}
        <div class="flex flex-wrap items-center gap-1.5 pb-1">
            <button
                class="px-2 py-1 rounded-full text-[11px] border transition-colors
                       {hasActiveCategoryFilters()
                           ? 'border-white/10 bg-white/5 text-white/55 hover:text-white/75 hover:bg-white/10'
                           : 'border-primary/30 bg-primary/15 text-primary'}"
                on:click={clearCategoryFilters}
            >
                All
            </button>
            {#each availableCategories as cat}
                <button
                    class="px-2 py-1 rounded-full text-[11px] border transition-colors
                           {isCategoryActive(cat)
                               ? 'border-secondary/35 bg-secondary/15 text-secondary'
                               : 'border-white/10 bg-white/5 text-white/55 hover:text-white/75 hover:bg-white/10'}"
                    on:click={() => toggleCategory(cat)}
                >
                    {categoryLabel(cat)}
                </button>
            {/each}
        </div>
    {/if}

    {#if filteredClusters.length === 0 && issues.length === 0}
        <div class="text-xs text-white/40 text-center py-4">No findings</div>
    {:else if filteredClusters.length === 0}
        <div class="text-xs text-white/40 text-center py-4">No findings for selected categories</div>
    {:else}
        {#each filteredClusters as cluster}
            <section class="rounded-lg border border-white/10 overflow-hidden bg-white/5">
                <button
                    class="w-full px-3 py-2 text-left hover:bg-white/5 transition-colors
                           flex items-center justify-between gap-3"
                    on:click={() => toggleCluster(cluster.code)}
                >
                    <div class="flex items-center gap-2 min-w-0">
                        <span class="w-1.5 h-8 rounded-full shrink-0 {severityStripeClass(cluster.severity)}"></span>
                        <span class="material-icons text-sm shrink-0 {severityTextClass(cluster.severity)}">
                            {severityIcon(cluster.severity)}
                        </span>
                        <div class="min-w-0">
                            <div class="text-sm text-white/90 font-medium truncate">
                                {cluster.label}
                            </div>
                            <div class="text-[11px] text-white/50 flex items-center gap-1.5">
                                <span class="px-1.5 py-0.5 rounded-full border {sourceClass(cluster.source)}">
                                    {sourceLabel(cluster.source)}
                                </span>
                                <span>•</span>
                                <span>{categoryLabel(cluster.category)}</span>
                            </div>
                        </div>
                    </div>
                    <div class="flex items-center gap-1.5 shrink-0">
                        <span class="px-2 py-0.5 rounded-full text-[11px] border border-white/15 bg-white/10 text-white/75">
                            {cluster.fileCount} files
                        </span>
                        <span class="px-2 py-0.5 rounded-full text-[11px] border border-white/15 bg-white/10 text-white/75">
                            {cluster.issueCount}
                        </span>
                        <span class="material-icons text-sm text-white/35 transition-transform"
                              class:rotate-180={clusterOpen[cluster.code]}>
                            expand_more
                        </span>
                    </div>
                </button>

                {#if clusterOpen[cluster.code]}
                    <div class="px-3 pb-3 border-t border-white/8"
                         transition:slide={{ duration: isLite ? 0 : 180 }}>
                        <div class="space-y-2 pt-2">
                            {#each visibleFiles(cluster) as file}
                                <div class="rounded-md border border-white/8 bg-black/15 px-2.5 py-2">
                                    <div class="flex items-center justify-between gap-2">
                                        <span class="font-mono text-xs text-white/80 truncate">
                                            {file.name}
                                        </span>
                                        <span class="text-[10px] text-white/45">
                                            {file.issues.length} issue{file.issues.length !== 1 ? 's' : ''}
                                        </span>
                                    </div>
                                    <div class="mt-1.5 space-y-1">
                                        {#if firstUniqueMessage(file.issues)}
                                            <div class="text-[11px] text-white/55 leading-4">
                                                {firstUniqueMessage(file.issues)}
                                            </div>
                                        {/if}
                                        {#if additionalUniqueMessageCount(file.issues) > 0}
                                            <div class="text-[10px] text-white/35">
                                                +{additionalUniqueMessageCount(file.issues)} more
                                            </div>
                                        {/if}
                                    </div>
                                </div>
                            {/each}
                        </div>

                        {#if hiddenFileCount(cluster) > 0}
                            <div class="pt-2">
                                <button
                                    class="w-full text-[11px] text-white/55 hover:text-white/80
                                           border border-white/10 rounded-md py-1.5 bg-white/5 hover:bg-white/10 transition-colors"
                                    on:click={() => toggleCompactFileExpansion(cluster.code)}
                                >
                                    {#if compactExpandedFiles[cluster.code]}
                                        Show fewer files
                                    {:else}
                                        Show {hiddenFileCount(cluster)} more file{hiddenFileCount(cluster) !== 1 ? 's' : ''}
                                    {/if}
                                </button>
                            </div>
                        {/if}
                    </div>
                {/if}
            </section>
        {/each}
    {/if}
</div>
