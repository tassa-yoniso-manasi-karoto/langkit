import { writable, derived } from 'svelte/store';
import type { ValidationReport } from '../api/services/expectation';

export interface CheckResultState {
    report: ValidationReport | null;
    acknowledged: boolean;
    isRunning: boolean;
    stale: boolean;
    /** Monotonic token that ties an in-flight check to its result. */
    runToken: number;
}

function createCheckResultStore() {
    var nextToken = 0;

    const { subscribe, set, update } = writable<CheckResultState>({
        report: null,
        acknowledged: false,
        isRunning: false,
        stale: false,
        runToken: 0,
    });

    return {
        subscribe,

        /** Called when a check starts. Returns a token the caller must
         *  pass back to setReport so stale completions are discarded. */
        setRunning: (): number => {
            var token = ++nextToken;
            set({ report: null, acknowledged: false, isRunning: true, stale: false, runToken: token });
            return token;
        },

        /** Called when a check completes. Only writes if the token
         *  matches the current run — otherwise the result is stale. */
        setReport: (token: number, report: ValidationReport) => {
            update(function(state) {
                if (state.runToken !== token) return state;
                return { report: report, acknowledged: false, isRunning: false, stale: false, runToken: state.runToken };
            });
        },

        /** Called when the user acknowledges errors */
        acknowledge: () => {
            update(state => ({ ...state, acknowledged: true }));
        },

        /** Called when settings change but results should remain visible */
        markStale: () => {
            update(state => {
                if (!state.report) return state;
                return { ...state, stale: true };
            });
        },

        /** Called when the media path changes or user clears results */
        clear: () => {
            set({ report: null, acknowledged: false, isRunning: false, stale: false, runToken: ++nextToken });
        },

        /** Returns the current run token for conditional clearing. */
        currentToken: (): number => {
            var t = 0;
            var unsub = subscribe(function(s) { t = s.runToken; });
            unsub();
            return t;
        },
    };
}

export const checkResultStore = createCheckResultStore();

export type CheckState =
    | 'unchecked'
    | 'running'
    | 'stale'
    | 'checked_clean'
    | 'checked_with_errors_unacknowledged'
    | 'checked_with_errors_acknowledged';

export const checkState = derived(checkResultStore, ($store): CheckState => {
    if ($store.isRunning) return 'running';
    if ($store.report === null) return 'unchecked';
    if ($store.stale) return 'stale';
    if ($store.report.errorCount === 0) return 'checked_clean';
    if ($store.acknowledged) return 'checked_with_errors_acknowledged';
    return 'checked_with_errors_unacknowledged';
});
