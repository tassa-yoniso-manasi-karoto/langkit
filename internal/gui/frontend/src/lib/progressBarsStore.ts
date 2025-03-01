import { writable } from 'svelte/store';

export interface ProgressBarData {
    id: string;               // unique ID for each bar
    progress: number;         // 0..100
    current?: number;         // optional, e.g. the "nth" item out of "total"
    total?: number;
    operation: string;        // short label, e.g. "Subtitle Extraction"
    description?: string;     // short text, e.g. "Splitting audio track"
    color: string;           // flowbite color prop, e.g. "blue" | "red" | "purple"
    size: string;            // e.g. "h-2" | "h-4" 
    labelOutside?: string;
    priority?: number;      // lower number => higher (earlier) in the UI
}

export const progressBars = writable<ProgressBarData[]>([]);

export function updateProgressBar(bar: ProgressBarData) {
    progressBars.update((bars) => {
        const idx = bars.findIndex((b) => b.id === bar.id);
        if (idx > -1) {
            bars[idx] = { ...bars[idx], ...bar };
        } else {
            bars.push(bar);
        }
        return bars;
    });
}

export function removeProgressBar(id: string) {
    progressBars.update((bars) => bars.filter((b) => b.id !== id));
}
