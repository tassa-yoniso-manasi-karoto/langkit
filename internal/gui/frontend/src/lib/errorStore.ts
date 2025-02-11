// errorStore.ts
import { writable, derived, type Readable } from 'svelte/store';

export type ErrorSeverity = 'critical' | 'warning' | 'info';

export interface ErrorMessage {
    id: string;
    message: string;
    severity: ErrorSeverity;
    timestamp?: number;
    dismissible?: boolean;
    docsUrl?: string;
    action?: {
        label: string;
        handler: () => void;
    };
}

const AUTODISMISS_TIMEOUTS: Record<ErrorSeverity, number> = {
    critical: 0,    // Never auto-dismiss
    warning: 10000, // 10 seconds
    info: 5000      // 5 seconds
};

function createErrorStore() {
    const storedErrors = localStorage.getItem('langkit-errors');
    const initial = storedErrors ? JSON.parse(storedErrors) : [];

    // Store the entire writable store in a variable.
    const store = writable<ErrorMessage[]>(initial);

    // Subscribe to changes and persist only critical errors.
    store.subscribe(($errors) => {
        localStorage.setItem('langkit-errors',
            JSON.stringify($errors.filter(e => e.severity === 'critical'))
        );
    });

    function trackErrorMetric(error: ErrorMessage) {
        try {
            window.go.gui.App.TrackEvent({
                category: 'Error',
                action: error.id,
                label: error.message,
                value: error.severity === 'critical' ? 1 : 0
            });
        } catch (e) {
            console.debug('Error tracking not available:', e);
        }
    }

    return {
        subscribe: store.subscribe,

        addError: (error: ErrorMessage) => {
            console.debug(`[Error Store] Adding error: ${error.id}`, error);
            trackErrorMetric(error);

            store.update(errors => {
                // Remove existing error with the same ID, if present.
                const filteredErrors = errors.filter(e => e.id !== error.id);
                const timeout = AUTODISMISS_TIMEOUTS[error.severity];
                const newError = {
                    ...error,
                    timestamp: Date.now(),
                    dismissible: error.dismissible ?? (error.severity !== 'critical')
                };

                if (timeout > 0) {
                    setTimeout(() => {
                        // Remove the error after the timeout.
                        // (You can also call removeError if you want to reuse its logic)
                        store.update(errs => errs.filter(e => e.id !== error.id));
                    }, timeout);
                }

                return [...filteredErrors, newError];
            });
        },

        removeError: (id: string) => {
            console.debug(`[Error Store] Removing error: ${id}`);
            store.update(errors => errors.filter(e => e.id !== id));
        },

        clearErrors: () => {
            console.debug('[Error Store] Clearing all errors');
            store.set([]);
        },

        clearErrorsOfType: (severity: ErrorSeverity) => {
            console.debug(`[Error Store] Clearing errors of type: ${severity}`);
            store.update(errors => errors.filter(e => e.severity !== severity));
        },

        // Create derived stores using the entire store.
        hasErrors: derived(store, ($errors) =>
            $errors.some(e => e.severity === 'critical')
        ) as Readable<boolean>,

        hasWarnings: derived(store, ($errors) =>
            $errors.some(e => e.severity === 'warning')
        ) as Readable<boolean>,

        getErrors: derived(store, ($errors) =>
            $errors.filter(e => e.severity === 'critical')
        ) as Readable<ErrorMessage[]>,

        getWarnings: derived(store, ($errors) =>
            $errors.filter(e => e.severity === 'warning')
        ) as Readable<ErrorMessage[]>,

        getInfos: derived(store, ($errors) =>
            $errors.filter(e => e.severity === 'info')
        ) as Readable<ErrorMessage[]>
    };
}

export const errorStore = createErrorStore();
