import { writable } from 'svelte/store';
import { logger } from './logger';

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
    priority?: number;       // lower number => higher (earlier) in the UI
    errorState?: string;     // 'error_task', 'error_all' or 'user_cancel'
    animated?: boolean;      // whether the progress bar should have animation 
    striped?: boolean;       // whether the progress bar should have striped pattern
    status?: string;         // custom status text (defaults to "Processing..." or "Complete")
}

export const progressBars = writable<ProgressBarData[]>([]);

export function updateProgressBar(bar: ProgressBarData) {
    progressBars.update((bars) => {
        const idx = bars.findIndex((b) => b.id === bar.id);
        if (idx > -1) {
            // Special error state handling
            const existingErrorState = bars[idx].errorState;
            const newErrorState = bar.errorState;

            if (existingErrorState) {
                // Priority rules for error states:
                // 1. abort_all (error_all) always overrides anything
                // 2. abort_task (error_task) can be overridden by abort_all but not by regular updates
                // 3. user_cancel can be overridden by abort_all but not by regular updates
                
                if (newErrorState === 'error_all') {
                    // Allow error_all to override any existing error state
                    bars[idx] = { ...bars[idx], ...bar };
                    logger.trace('store/progressBarsStore', 'Overriding error state with error_all', { 
                        barId: bar.id, 
                        previousState: existingErrorState, 
                        newState: 'error_all' 
                    });
                } else if (!newErrorState) {
                    // Regular update - preserve existing error state
                    bars[idx] = { 
                        ...bars[idx], 
                        ...bar,
                        errorState: existingErrorState
                    };
                    logger.trace('store/progressBarsStore', 'Preserving existing error state', { 
                        barId: bar.id, 
                        errorState: existingErrorState 
                    });
                } else {
                    // New error state but not error_all - check hierarchy
                    if (existingErrorState === 'error_task' && newErrorState === 'error_all') {
                        // Allow error_all to override error_task
                        bars[idx] = { ...bars[idx], ...bar };
                    } else if (existingErrorState === 'user_cancel' && 
                              (newErrorState === 'error_all' || newErrorState === 'error_task')) {
                        // Allow any error to override user_cancel
                        bars[idx] = { ...bars[idx], ...bar };
                    } else {
                        // Otherwise preserve existing error state
                        bars[idx] = { 
                            ...bars[idx], 
                            ...bar,
                            errorState: existingErrorState
                        };
                    }
                }
            } else {
                // No existing error state, normal update
                bars[idx] = { ...bars[idx], ...bar };
            }
        } else {
            // New bar
            bars.push(bar);
        }
        return bars;
    });
}

export function removeProgressBar(id: string) {
    progressBars.update((bars) => bars.filter((b) => b.id !== id));
}

export function resetAllProgressBars() {
    progressBars.set([]);
}
