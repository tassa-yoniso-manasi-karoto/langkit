import type { ValidationIssue } from '../api/generated/api.gen';

export type IssueSeverity = 'error' | 'warning' | 'info';

export interface ClusterFile {
    path: string;
    name: string;
    issues: ValidationIssue[];
}

export interface Cluster {
    code: string;
    label: string;
    severity: IssueSeverity;
    source: string;
    category: string;
    fileCount: number;
    files: ClusterFile[];
    issueCount: number;
}

export interface TriageFile {
    path: string;
    name: string;
    issues: ValidationIssue[];
    errorCount: number;
    warningCount: number;
    infoCount: number;
    severityScore: number;
    status: 'critical' | 'review' | 'clean';
    topIssueSummary: string;
}

export const codeLabelMap: Record<string, string> = {
    mediainfo_failed: 'MediaInfo Failures',
    no_media_files: 'No Media Files',
    no_video_track: 'Missing Video Track',
    audio_decode_failed: 'Audio Decode Failures',
    video_decode_failed: 'Video Decode Failures',
    corrupt_track: 'Corrupt Audio Tracks',
    audio_duration_mismatch: 'Audio Duration Mismatches',
    ext_audio_duration: 'External Audio Duration Issues',
    duration_unavailable: 'Duration Unavailable',
    missing_audio_lang: 'Missing Audio Languages',
    missing_sub_lang: 'Missing Subtitle Languages',
    untagged_track: 'Untagged Tracks',
    sub_parse_failed: 'Subtitle Parse Failures',
    sub_empty: 'Empty Subtitles',
    sub_encoding: 'Subtitle Encoding Issues',
    sub_low_coverage: 'Low Subtitle Coverage',
    sub_overlap: 'Subtitle Overlap',
    auto_missing_audio: 'Missing Consensus Audio',
    auto_missing_sub: 'Missing Consensus Subtitles',
    auto_audio_count: 'Audio Track Count Anomalies',
    auto_sub_count: 'Subtitle Count Anomalies',
    auto_duration_outlier: 'Duration Outliers',
    auto_group_too_small: 'Group Too Small for Auto Checks',
};

export const categoryLabelMap: Record<string, string> = {
    integrity: 'Integrity',
    duration: 'Duration',
    language: 'Language',
    subtitle: 'Subtitle',
    consistency: 'Consistency',
    structure: 'Structure',
};

export function normalizeSeverity(severity: string): IssueSeverity {
    var value = (severity || 'info').toLowerCase();
    if (value !== 'error' && value !== 'warning' && value !== 'info') return 'info';
    return value;
}

export function severityRank(severity: string): number {
    var normalized = normalizeSeverity(severity);
    if (normalized === 'error') return 0;
    if (normalized === 'warning') return 1;
    return 2;
}

export function categoryLabel(category: string): string {
    return categoryLabelMap[category] || humanizeIssueCode(category);
}

export function sourceLabel(source: string): string {
    if (source === 'profile') return 'Profile';
    if (source === 'auto') return 'Auto';
    if (source === 'structural') return 'Structural';
    if (source === 'mixed') return 'Mixed';
    return 'Unknown';
}

export function getClusters(issues: ValidationIssue[]): Cluster[] {
    var byCode: Record<string, ValidationIssue[]> = {};
    for (var i = 0; i < issues.length; i++) {
        var issue = issues[i];
        var issueCode = issue.issueCode || 'unknown';
        if (!byCode[issueCode]) byCode[issueCode] = [];
        byCode[issueCode].push(issue);
    }

    var clusters: Cluster[] = [];
    var codes = Object.keys(byCode);
    for (var codeIndex = 0; codeIndex < codes.length; codeIndex++) {
        var clusterCode = codes[codeIndex];
        var codeIssues = byCode[clusterCode];
        if (!codeIssues || codeIssues.length === 0) continue;

        var byFile: Record<string, ValidationIssue[]> = {};
        var categoryCount: Record<string, number> = {};
        var sourceCount: Record<string, number> = {};
        var dominantSeverity: IssueSeverity = 'info';

        for (var issueIndex = 0; issueIndex < codeIssues.length; issueIndex++) {
            var entry = codeIssues[issueIndex];
            if (!byFile[entry.filePath]) byFile[entry.filePath] = [];
            byFile[entry.filePath].push(entry);

            var category = entry.category || 'other';
            categoryCount[category] = (categoryCount[category] || 0) + 1;

            var source = entry.source || 'profile';
            sourceCount[source] = (sourceCount[source] || 0) + 1;

            if (severityRank(entry.severity) < severityRank(dominantSeverity)) {
                dominantSeverity = normalizeSeverity(entry.severity);
            }
        }

        var filePaths = Object.keys(byFile).sort();
        var files: ClusterFile[] = [];
        for (var fileIndex = 0; fileIndex < filePaths.length; fileIndex++) {
            var path = filePaths[fileIndex];
            var fileIssues = byFile[path].slice().sort(function(a, b) {
                var severityDiff = severityRank(a.severity) - severityRank(b.severity);
                if (severityDiff !== 0) return severityDiff;
                return (a.message || '').localeCompare(b.message || '');
            });

            files.push({
                path: path,
                name: basename(path),
                issues: fileIssues,
            });
        }

        var source = dominantKey(sourceCount, 'profile');
        if (Object.keys(sourceCount).length > 1) source = 'mixed';

        clusters.push({
            code: clusterCode,
            label: codeLabelMap[clusterCode] || humanizeIssueCode(clusterCode),
            severity: dominantSeverity,
            source: source,
            category: dominantKey(categoryCount, 'other'),
            fileCount: files.length,
            files: files,
            issueCount: codeIssues.length,
        });
    }

    clusters.sort(function(a, b) {
        var severityDiff = severityRank(a.severity) - severityRank(b.severity);
        if (severityDiff !== 0) return severityDiff;
        if (a.fileCount !== b.fileCount) return b.fileCount - a.fileCount;
        return a.label.localeCompare(b.label);
    });

    return clusters;
}

export function getAvailableCategories(clusters: Cluster[]): string[] {
    var seen: Record<string, boolean> = {};
    var categories: string[] = [];
    for (var i = 0; i < clusters.length; i++) {
        var category = clusters[i].category || 'other';
        if (seen[category]) continue;
        seen[category] = true;
        categories.push(category);
    }

    categories.sort(function(a, b) {
        return categoryLabel(a).localeCompare(categoryLabel(b));
    });

    return categories;
}

function buildTriageFiles(issues: ValidationIssue[]): TriageFile[] {
    var byFile: Record<string, ValidationIssue[]> = {};
    for (var i = 0; i < issues.length; i++) {
        var issue = issues[i];
        if (!byFile[issue.filePath]) byFile[issue.filePath] = [];
        byFile[issue.filePath].push(issue);
    }

    var files: TriageFile[] = [];
    var paths = Object.keys(byFile).sort();
    for (var pathIndex = 0; pathIndex < paths.length; pathIndex++) {
        var path = paths[pathIndex];
        var fileIssues = byFile[path].slice().sort(function(a, b) {
            var severityDiff = severityRank(a.severity) - severityRank(b.severity);
            if (severityDiff !== 0) return severityDiff;
            return (a.message || '').localeCompare(b.message || '');
        });

        var errorCount = 0;
        var warningCount = 0;
        var infoCount = 0;
        for (var issueIndex = 0; issueIndex < fileIssues.length; issueIndex++) {
            var severity = normalizeSeverity(fileIssues[issueIndex].severity);
            if (severity === 'error') {
                errorCount++;
            } else if (severity === 'warning') {
                warningCount++;
            } else {
                infoCount++;
            }
        }

        var status: 'critical' | 'review' | 'clean' = 'clean';
        if (errorCount > 0) {
            status = 'critical';
        } else if (warningCount > 0 || infoCount > 0) {
            status = 'review';
        }

        // Weighting keeps "error-heavy" files at the top while still
        // differentiating warning/info clusters.
        var severityScore = errorCount * 100 + warningCount * 10 + infoCount;

        files.push({
            path: path,
            name: basename(path),
            issues: fileIssues,
            errorCount: errorCount,
            warningCount: warningCount,
            infoCount: infoCount,
            severityScore: severityScore,
            status: status,
            topIssueSummary: summarizeTopIssue(fileIssues),
        });
    }

    files.sort(function(a, b) {
        if (a.severityScore !== b.severityScore) return b.severityScore - a.severityScore;
        if (a.errorCount !== b.errorCount) return b.errorCount - a.errorCount;
        if (a.warningCount !== b.warningCount) return b.warningCount - a.warningCount;
        return a.name.localeCompare(b.name);
    });

    return files;
}

export function getTriageFiles(issues: ValidationIssue[]): TriageFile[] {
    return buildTriageFiles(issues);
}

function summarizeTopIssue(issues: ValidationIssue[]): string {
    if (issues.length === 0) return 'No findings';

    var byCode: Record<string, number> = {};
    var byCodeSeverity: Record<string, number> = {};
    for (var i = 0; i < issues.length; i++) {
        var code = issues[i].issueCode || 'unknown';
        byCode[code] = (byCode[code] || 0) + 1;

        var currentRank = severityRank(issues[i].severity);
        if (byCodeSeverity[code] === undefined || currentRank < byCodeSeverity[code]) {
            byCodeSeverity[code] = currentRank;
        }
    }

    var codes = Object.keys(byCode);
    if (codes.length === 0) return issues[0].message || 'Issue';

    codes.sort(function(a, b) {
        var severityDiff = (byCodeSeverity[a] || 2) - (byCodeSeverity[b] || 2);
        if (severityDiff !== 0) return severityDiff;
        var countDiff = (byCode[b] || 0) - (byCode[a] || 0);
        if (countDiff !== 0) return countDiff;
        return a.localeCompare(b);
    });

    var topCode = codes[0];
    var topLabel = codeLabelMap[topCode] || humanizeIssueCode(topCode);
    var topCount = byCode[topCode] || 0;

    if (codes.length === 1) {
        return topLabel + (topCount > 1 ? ' (' + topCount + ')' : '');
    }

    return topLabel + (topCount > 1 ? ' (' + topCount + ')' : '') + ' +' + (codes.length - 1) + ' more';
}

function dominantKey(counts: Record<string, number>, fallback: string): string {
    var keys = Object.keys(counts);
    if (keys.length === 0) return fallback;

    var bestKey = keys[0];
    var bestCount = counts[bestKey] || 0;
    for (var i = 1; i < keys.length; i++) {
        var key = keys[i];
        var count = counts[key] || 0;
        if (count > bestCount) {
            bestKey = key;
            bestCount = count;
        }
    }

    return bestKey;
}

function humanizeIssueCode(code: string): string {
    var parts = (code || '').split('_');
    var result: string[] = [];
    for (var i = 0; i < parts.length; i++) {
        if (!parts[i]) continue;
        result.push(parts[i].charAt(0).toUpperCase() + parts[i].slice(1));
    }
    if (result.length === 0) return 'Unknown';
    return result.join(' ');
}

function basename(path: string): string {
    var unixParts = path.split('/');
    var lastUnixPart = unixParts[unixParts.length - 1];
    var windowsParts = lastUnixPart.split('\\');
    return windowsParts[windowsParts.length - 1];
}
