<script lang="ts">
    import type { ValidationIssue } from '../api/generated/api.gen';

    export let issues: ValidationIssue[] = [];
    export let compact: boolean = false;

    // Issue code to human-readable label
    var codeLabelMap: Record<string, string> = {
        'mediainfo_failed': 'MediaInfo Failures',
        'no_media_files': 'No Media Files',
        'no_video_track': 'Missing Video Track',
        'audio_decode_failed': 'Audio Decode Failures',
        'video_decode_failed': 'Video Decode Failures',
        'corrupt_track': 'Corrupt Audio Tracks',
        'audio_duration_mismatch': 'Audio Duration Mismatches',
        'ext_audio_duration': 'External Audio Duration Issues',
        'duration_unavailable': 'Duration Unavailable',
        'missing_audio_lang': 'Missing Audio Languages',
        'missing_sub_lang': 'Missing Subtitle Languages',
        'untagged_track': 'Untagged Tracks',
        'sub_parse_failed': 'Subtitle Parse Failures',
        'sub_empty': 'Empty Subtitles',
        'sub_encoding': 'Subtitle Encoding Issues',
        'sub_low_coverage': 'Low Subtitle Coverage',
        'sub_overlap': 'Subtitle Overlap',
        'auto_missing_audio': 'Missing Consensus Audio',
        'auto_missing_sub': 'Missing Consensus Subtitles',
        'auto_audio_count': 'Audio Track Count Anomalies',
        'auto_sub_count': 'Subtitle Count Anomalies',
        'auto_duration_outlier': 'Duration Outliers',
        'auto_group_too_small': 'Group Too Small for Auto Checks',
    };

    interface ClusterFile {
        path: string;
        name: string;
        issues: ValidationIssue[];
    }

    interface Cluster {
        code: string;
        label: string;
        severity: string;
        source: string;
        category: string;
        fileCount: number;
        files: ClusterFile[];
    }

    // Clustering logic: group by issueCode, sub-group by file
    $: clusters = buildClusters(issues);

    function buildClusters(allIssues: ValidationIssue[]): Cluster[] {
        // TODO: ChatGPT fills in clustering logic, rendering, category filters
        return [];
    }

    function basename(path: string): string {
        var parts = path.split('/');
        var windowsParts = parts[parts.length - 1].split('\\');
        return windowsParts[windowsParts.length - 1];
    }
</script>

<!-- Shell: ChatGPT fills in cluster cards, severity stripes, file lists, category filter chips -->
<div class="space-y-2">
    {#if clusters.length === 0 && issues.length === 0}
        <div class="text-xs text-white/40 text-center py-4">No findings</div>
    {:else if clusters.length === 0}
        <div class="text-xs text-white/40 text-center py-4">Clustering placeholder — {issues.length} issues</div>
    {:else}
        {#each clusters as cluster}
            <div class="text-xs text-white/60">{cluster.label} ({cluster.fileCount} files)</div>
        {/each}
    {/if}
</div>
