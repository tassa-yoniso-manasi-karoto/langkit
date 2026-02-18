import { writable, derived } from 'svelte/store';
import type { ValidationReport } from '../api/services/expectation';

export interface CheckResultState {
    report: ValidationReport | null;
    acknowledged: boolean;
    isRunning: boolean;
}

function createCheckResultStore() {
    const { subscribe, set, update } = writable<CheckResultState>({
        report: null,
        acknowledged: false,
        isRunning: false,
    });

    return {
        subscribe,

        /** Called when a check starts */
        setRunning: () => {
            set({ report: null, acknowledged: false, isRunning: true });
        },

        /** Called when a check completes */
        setReport: (report: ValidationReport) => {
            set({ report, acknowledged: false, isRunning: false });
        },

        /** Called when the user acknowledges errors */
        acknowledge: () => {
            update(state => ({ ...state, acknowledged: true }));
        },

        /** Called when the media path changes or user clears results */
        clear: () => {
            set({ report: null, acknowledged: false, isRunning: false });
        },
    };
}

export const checkResultStore = createCheckResultStore();

export type CheckState =
    | 'unchecked'
    | 'running'
    | 'checked_clean'
    | 'checked_with_errors_unacknowledged'
    | 'checked_with_errors_acknowledged';

export const checkState = derived(checkResultStore, ($store): CheckState => {
    if ($store.isRunning) return 'running';
    if ($store.report === null) return 'unchecked';
    if ($store.report.errorCount === 0) return 'checked_clean';
    if ($store.acknowledged) return 'checked_with_errors_acknowledged';
    return 'checked_with_errors_unacknowledged';
});
