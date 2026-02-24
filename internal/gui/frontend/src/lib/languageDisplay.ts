let displayNames: Intl.DisplayNames | null = null;

function getDisplayNames(): Intl.DisplayNames | null {
    if (displayNames) return displayNames;
    try {
        var locale = 'en';
        if (typeof navigator !== 'undefined' && navigator.language) {
            locale = navigator.language;
        }
        displayNames = new Intl.DisplayNames([locale, 'en'], { type: 'language' });
        return displayNames;
    } catch {
        return null;
    }
}

export function formatLanguageName(tag: string): string {
    var raw = (tag || '').trim();
    if (!raw) return '';

    var normalized = raw.replace(/_/g, '-');
    var lower = normalized.toLowerCase();

    if (lower === 'und') return 'Undefined';

    var names = getDisplayNames();
    if (!names) return normalized;

    try {
        var direct = names.of(normalized);
        if (direct && direct.toLowerCase() !== 'root') return direct;

        // Fallback: if the tag carries script/region and direct resolution fails,
        // try the primary language subtag before giving up.
        var primary = normalized.split('-')[0];
        if (primary && primary !== normalized) {
            var fallback = names.of(primary);
            if (fallback && fallback.toLowerCase() !== 'root') return fallback;
        }
    } catch {
        return normalized;
    }

    return normalized;
}

export function formatLanguageList(tags: string[]): string {
    return formatLanguageNames(tags).join(', ');
}

export function formatLanguageNames(tags: string[]): string[] {
    if (!tags || tags.length === 0) return [];

    var names: string[] = [];
    for (var i = 0; i < tags.length; i++) {
        var name = formatLanguageName(tags[i]);
        if (!name) continue;
        names.push(name);
    }
    return names;
}
