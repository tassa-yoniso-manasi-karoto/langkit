// invalidationErrorStore.ts
import { writable, derived, type Readable } from 'svelte/store';
import { logger } from './logger';

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
    let initial: ErrorMessage[] = storedErrors ? JSON.parse(storedErrors) : [];

    // Clear any stale "processing in progress" errors on startup
    // These can get stuck if the app was closed during processing
    initial = initial.filter(e => !e.id.includes('processing') && !e.message.toLowerCase().includes('processing'));

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
            logger.trace('store/invalidationErrorStore', 'Error tracking not available', { error: e });
        }
    }

    function addError(error: ErrorMessage) {
        logger.debug('store/invalidationErrorStore', 'Adding error', { errorId: error.id, error });
        trackErrorMetric(error);

        store.update(errors => {
            // Always remove existing error with same ID first for proper reactivity
            const filteredErrors = errors.filter(e => e.id !== error.id);
            
            // Create new error with timestamp
            const newError = {
                ...error,
                timestamp: Date.now(),
                dismissible: error.dismissible ?? (error.severity !== 'critical')
            };
            
            // Setup auto-dismiss if needed
            const timeout = AUTODISMISS_TIMEOUTS[error.severity];
            if (timeout > 0) {
                setTimeout(() => {
                    // Remove the error after the timeout.
                    removeError(error.id);
                }, timeout);
            }

            // Return updated errors array to trigger reactivity
            return [...filteredErrors, newError];
        });
    }

    function removeError(id: string) {
        logger.debug('store/invalidationErrorStore', 'Removing error', { errorId: id });
        store.update(errors => errors.filter(e => e.id !== id));
    }

    function clearErrors() {
        logger.debug('store/invalidationErrorStore', 'Clearing all errors');
        store.set([]);
    }

    function clearErrorsOfType(severity: ErrorSeverity) {
        logger.debug('store/invalidationErrorStore', 'Clearing errors of type', { severity });
        store.update(errors => errors.filter(e => e.severity !== severity));
    }

    // Return the public API
    return {
        subscribe: store.subscribe,
        addError,
        removeError,
        clearErrors,
        clearErrorsOfType,

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

export const invalidationErrorStore = createErrorStore();