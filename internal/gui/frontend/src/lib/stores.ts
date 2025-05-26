import { writable } from 'svelte/store';
import { logger } from './logger';

// IntermediaryFileMode defines how to handle intermediary files
type IntermediaryFileMode = 'keep' | 'recompress' | 'delete';

type Settings = {
    apiKeys: {
        replicate: string;
        elevenLabs: string;
        openAI: string;
        openRouter: string;
        google: string;
    };
    targetLanguage: string;
    nativeLanguages: string;
    enableGlow: boolean;
    showLogViewerByDefault: boolean;
    maxLogEntries: number;
    maxAPIRetries: number;
    maxWorkers: number;
    // Timeout settings
    timeoutSep: number;  // Voice separation timeout (seconds)
    timeoutSTT: number;  // Speech-to-text timeout (seconds)
    timeoutDL: number;   // Download operations timeout (seconds)
    // WebAssembly settings
    useWasm: boolean;
    wasmSizeThreshold: number;
    forceWasmMode: 'auto' | 'enabled' | 'disabled'; // Add force override mode

    // NEW: LogViewer settings
    logViewerVirtualizationThreshold: number;

    eventThrottling: { enabled: boolean; minInterval: number; maxInterval: number; }; // Expect object
    convertValues: boolean;   // Make required
    
    // Internal settings (not exposed in UI)
    hasSeenLogViewerTooltip?: boolean;
    
    // File handling settings
    intermediaryFileMode?: IntermediaryFileMode;
};

/* these values are irrelevant, only the default values of the backend matter */
const initSettings: Settings = {
    apiKeys: {
        replicate: '',
        elevenLabs: '',
        openAI: '',
        openRouter: '',
        google: ''
    },
    targetLanguage: '',
    nativeLanguages: 'en, en-US',
    enableGlow: true,
    showLogViewerByDefault: false,
    maxLogEntries: 10000,
    maxAPIRetries: 10,
    maxWorkers: 1,
    // Default values for timeout settings
    timeoutSep: 2100,  // 35 minutes for voice separation
    timeoutSTT: 90,    // 90 seconds for STT operations
    timeoutDL: 600,    // 10 minutes for downloads
    // Default values for WebAssembly settings
    useWasm: true,
    wasmSizeThreshold: 500, // Default from spec
    forceWasmMode: 'enabled', // Always use WebAssembly by default

    // NEW: LogViewer settings
    logViewerVirtualizationThreshold: 2000, // Default to 2000 logs for virtualization

    // Default values for internal settings
    hasSeenLogViewerTooltip: false,
    // Add defaults for missing properties
    eventThrottling: { enabled: true, minInterval: 0, maxInterval: 250 }, // Default object
    convertValues: false,    // Assuming default
    // Default file handling settings
    intermediaryFileMode: 'keep'
};

type showSettings = boolean;

// Create custom settings store with logging
function createSettingsStore() {
    const { subscribe, set, update } = writable<Settings>(initSettings);
    
    return {
        subscribe,
        set: (value: Settings) => {
            logger.trace('store/settings', 'Settings updated', {
                hasApiKeys: !!value.apiKeys,
                targetLanguage: value.targetLanguage,
                nativeLanguages: value.nativeLanguages,
                useWasm: value.useWasm,
                wasmSizeThreshold: value.wasmSizeThreshold
            });
            set(value);
        },
        update: (updater: (value: Settings) => Settings) => {
            update((current) => {
                const newValue = updater(current);
                logger.trace('store/settings', 'Settings updated via update()', {
                    hasApiKeys: !!newValue.apiKeys,
                    targetLanguage: newValue.targetLanguage,
                    useWasm: newValue.useWasm
                });
                return newValue;
            });
        }
    };
}

export const settings = createSettingsStore();

// Create show settings store with logging
function createShowSettingsStore() {
    const { subscribe, set, update } = writable(false);
    
    return {
        subscribe,
        set: (value: boolean) => {
            logger.trace('store/showSettings', 'Settings modal visibility changed', { visible: value });
            set(value);
        },
        update
    };
}

export const showSettings = createShowSettingsStore();

// Add a reactive store to track when WASM is actively being used
function createWasmActiveStore() {
    const { subscribe, set, update } = writable<boolean>(false);
    
    return {
        subscribe,
        set: (value: boolean) => {
            logger.trace('store/wasmActive', 'WASM active state changed', { active: value });
            set(value);
        },
        update
    };
}

export const wasmActive = createWasmActiveStore();

// LLM state management types and store
export interface LLMProviderState {
    status: 'not_attempted' | 'initializing_models' | 'ready' | 'error';
    error?: string;
    models?: Array<{ id: string; name: string }>;
    lastUpdated: string;
}

export interface LLMStateChange {
    timestamp: string;
    globalState: 'uninitialized' | 'initializing' | 'ready' | 'error' | 'updating';
    updatedProviderName?: string;
    providerStatesSnapshot: Record<string, LLMProviderState>;
    message?: string;
}

function createLLMStateStore() {
    const { subscribe, set, update } = writable<LLMStateChange | null>(null);
    
    return {
        subscribe,
        set: (value: LLMStateChange | null) => {
            if (value) {
                logger.trace('store/llmState', 'LLM state changed', {
                    globalState: value.globalState,
                    updatedProvider: value.updatedProviderName,
                    message: value.message
                });
            }
            set(value);
        },
        update,
        
        // Helper methods
        isReady: () => {
            let ready = false;
            subscribe(state => {
                ready = state?.globalState === 'ready';
            })();
            return ready;
        },
        
        getProviderStatus: (providerName: string): LLMProviderState | null => {
            let provider = null;
            subscribe(state => {
                provider = state?.providerStatesSnapshot[providerName] || null;
            })();
            return provider;
        }
    };
}

export const llmStateStore = createLLMStateStore();

// Statistics store
interface Statistics {
    [key: string]: any; // Flexible key-value structure for statistics
}

function createStatisticsStore() {
    const { subscribe, set, update } = writable<Statistics>({});
    
    return {
        subscribe,
        set: (value: Statistics) => {
            logger.trace('store/statistics', 'Statistics replaced', { 
                keys: Object.keys(value),
                keyCount: Object.keys(value).length 
            });
            set(value);
        },
        
        // Update specific statistics without overwriting the entire store
        updatePartial: (updates: Partial<Statistics>) => {
            logger.trace('store/statistics', 'Statistics partially updated', { 
                updatedKeys: Object.keys(updates),
                updates 
            });
            update(stats => ({
                ...stats,
                ...updates
            }));
        },
        
        // Get a specific statistic value
        get: (key: string): any => {
            let value: any;
            subscribe(stats => {
                value = stats[key];
            })();
            return value;
        },
        
        // Increment a counter statistic
        increment: (key: string): number => {
            let newValue = 0;
            update(stats => {
                const currentValue = typeof stats[key] === 'number' ? stats[key] : 0;
                newValue = currentValue + 1;
                logger.trace('store/statistics', 'Counter incremented', { 
                    key, 
                    oldValue: currentValue, 
                    newValue 
                });
                return {
                    ...stats,
                    [key]: newValue
                };
            });
            return newValue;
        }
    };
}

export const statisticsStore = createStatisticsStore();

// Welcome popup visibility state
export const welcomePopupVisible = writable<boolean>(false);

// User activity state with forced override support
interface UserActivityStateData {
    state: 'active' | 'idle' | 'afk';
    isForced: boolean;
}

function createUserActivityStateStore() {
    const { subscribe, set, update } = writable<UserActivityStateData>({
        state: 'active',
        isForced: false
    });
    
    return {
        subscribe,
        set: (state: 'active' | 'idle' | 'afk', forced: boolean = false) => {
            set({ state, isForced: forced });
        },
        reset: () => {
            set({ state: 'active', isForced: false });
        }
    };
}

export const userActivityState = createUserActivityStateStore();