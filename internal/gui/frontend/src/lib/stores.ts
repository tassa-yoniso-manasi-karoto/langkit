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
    liteMode: boolean;
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
    deleteResumptionFiles?: boolean;

    // Custom endpoints for local inference
    customEndpoints?: {
        stt: {
            enabled: boolean;
            endpoint: string;
            model: string;
        };
        llm: {
            enabled: boolean;
            endpoint: string;
            model: string;
        };
    };

    // Version tracking for changelog display
    lastSeenVersion?: string;
    changelogDisplayFrequency?: 'all' | 'minor_major' | 'major_only';
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
    liteMode: false,
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
    intermediaryFileMode: 'keep',
    deleteResumptionFiles: false,
    // Default custom endpoints settings
    customEndpoints: {
        stt: {
            enabled: false,
            endpoint: 'http://localhost:8080/v1/audio/transcriptions',
            model: ''
        },
        llm: {
            enabled: false,
            endpoint: 'http://localhost:11434/v1/chat/completions',
            model: ''
        }
    },
    // Default version tracking settings
    lastSeenVersion: '',
    changelogDisplayFrequency: 'minor_major'
};

type showSettings = boolean;

// Merge loaded settings with defaults, ensuring nested structures are always complete
export function mergeSettingsWithDefaults(loaded: Partial<Settings>): Settings {
    return {
        ...initSettings,
        ...loaded,
        // Ensure nested objects are properly merged (not overwritten with null/undefined)
        apiKeys: {
            ...initSettings.apiKeys,
            ...loaded.apiKeys
        },
        eventThrottling: {
            ...initSettings.eventThrottling,
            ...loaded.eventThrottling
        },
        customEndpoints: {
            stt: {
                enabled: loaded.customEndpoints?.stt?.enabled ?? initSettings.customEndpoints!.stt.enabled,
                endpoint: loaded.customEndpoints?.stt?.endpoint || initSettings.customEndpoints!.stt.endpoint,
                model: loaded.customEndpoints?.stt?.model ?? initSettings.customEndpoints!.stt.model
            },
            llm: {
                enabled: loaded.customEndpoints?.llm?.enabled ?? initSettings.customEndpoints!.llm.enabled,
                endpoint: loaded.customEndpoints?.llm?.endpoint || initSettings.customEndpoints!.llm.endpoint,
                model: loaded.customEndpoints?.llm?.model ?? initSettings.customEndpoints!.llm.model
            }
        },
        // Ensure scalar fields have proper defaults
        intermediaryFileMode: loaded.intermediaryFileMode || initSettings.intermediaryFileMode,
        deleteResumptionFiles: loaded.deleteResumptionFiles ?? initSettings.deleteResumptionFiles,
        forceWasmMode: (loaded.forceWasmMode || initSettings.forceWasmMode) as 'auto' | 'enabled' | 'disabled',
        // Version tracking fields
        lastSeenVersion: loaded.lastSeenVersion ?? initSettings.lastSeenVersion,
        changelogDisplayFrequency: (loaded.changelogDisplayFrequency || initSettings.changelogDisplayFrequency) as 'all' | 'minor_major' | 'major_only'
    };
}

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
        // Load settings from backend, merging with defaults
        load: (loaded: Partial<Settings>) => {
            const merged = mergeSettingsWithDefaults(loaded);
            logger.trace('store/settings', 'Settings loaded and merged with defaults', {
                hasApiKeys: !!merged.apiKeys,
                targetLanguage: merged.targetLanguage,
                hasCustomEndpoints: !!merged.customEndpoints
            });
            set(merged);
            return merged;
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

// Docker status store
export interface DockerStatus {
    available: boolean;
    version?: string;
    engine?: string;
    error?: string;
    checked: boolean;
}

function createDockerStatusStore() {
    const { subscribe, set, update } = writable<DockerStatus>({
        available: false,
        checked: false
    });
    
    return {
        subscribe,
        set: (value: DockerStatus) => {
            logger.trace('store/dockerStatus', 'Docker status updated', value);
            set(value);
        },
        update
    };
}

export const dockerStatusStore = createDockerStatusStore();

// Internet connectivity store
export interface InternetStatus {
    online: boolean;
    latency?: number;
    error?: string;
    checked: boolean;
}

function createInternetStatusStore() {
    const { subscribe, set, update } = writable<InternetStatus>({
        online: false,
        checked: false
    });
    
    return {
        subscribe,
        set: (value: InternetStatus) => {
            logger.trace('store/internetStatus', 'Internet status updated', value);
            set(value);
        },
        update
    };
}

export const internetStatusStore = createInternetStatusStore();

// FFmpeg status store
export interface FFmpegStatus {
    available: boolean;
    version?: string;
    path?: string;
    error?: string;
    checked: boolean;
}

function createFFmpegStatusStore() {
    const { subscribe, set, update } = writable<FFmpegStatus>({
        available: false,
        checked: false
    });
    
    return {
        subscribe,
        set: (value: FFmpegStatus) => {
            logger.trace('store/ffmpegStatus', 'FFmpeg status updated', value);
            set(value);
        },
        update
    };
}

export const ffmpegStatusStore = createFFmpegStatusStore();

// MediaInfo status store
export interface MediaInfoStatus {
    available: boolean;
    version?: string;
    path?: string;
    error?: string;
    checked: boolean;
}

function createMediaInfoStatusStore() {
    const { subscribe, set, update } = writable<MediaInfoStatus>({
        available: false,
        checked: false
    });
    
    return {
        subscribe,
        set: (value: MediaInfoStatus) => {
            logger.trace('store/mediainfoStatus', 'MediaInfo status updated', value);
            set(value);
        },
        update
    };
}

export const mediainfoStatusStore = createMediaInfoStatusStore();

// Store to control whether trace logs are sent to the frontend
export const enableTraceLogsStore = writable<boolean>(false);

// Store to control whether frontend logs are sent to the backend
export const enableFrontendLoggingStore = writable<boolean>(true);

// Store to control whether frontend logs are displayed in the LogViewer
export const displayFrontendLogsStore = writable<boolean>(false);

// Store to control whether frontend trace logs are sent to the backend
export const sendFrontendTraceLogsStore = writable<boolean>(false);

// System information store
export interface SystemInfo {
    os: string;
    arch: string;
}

export const systemInfoStore = writable<SystemInfo>({ os: '', arch: '' });

// Lite mode store
// When true, disables backdrop-filter blur effects to work around:
// - Qt WebEngine flickering issues on Windows
// - Lack of hardware acceleration (software rendering)
// Reasons:
// - 'qt-windows': Automatically enabled on Qt+Windows (cannot be disabled by user)
// - 'no-hw-accel': No hardware acceleration available (cannot be disabled by user)
// - 'user': User preference via liteMode setting
// - 'debug-override': Dev testing on non-Windows platforms
// - 'none': Full effects enabled
interface LiteModeState {
    enabled: boolean;
    reason: 'qt-windows' | 'no-hw-accel' | 'user' | 'debug-override' | 'none';
    isForced: boolean;  // Track if forced (Qt+Windows or no hw accel) for UI disabling
}

function createLiteModeStore() {
    const { subscribe, set, update } = writable<LiteModeState>({
        enabled: false,
        reason: 'none',
        isForced: false
    });

    let _isForced = false;
    let _forcedReason: 'qt-windows' | 'no-hw-accel' | null = null;

    return {
        subscribe,
        // Auto-set based on runtime detection (called once on startup)
        setAuto: (isAnkiMode: boolean, os: string) => {
            const isQtWindows = isAnkiMode && os === 'windows';
            logger.debug('store/liteMode', 'Auto-detecting lite mode', {
                isAnkiMode,
                os,
                isQtWindows
            });
            if (isQtWindows) {
                // Force enable on Qt+Windows
                _isForced = true;
                _forcedReason = 'qt-windows';
                set({
                    enabled: true,
                    reason: 'qt-windows',
                    isForced: true
                });
            } else {
                set({
                    enabled: false,
                    reason: 'none',
                    isForced: false
                });
            }
        },
        // Force enable when hardware acceleration is unavailable
        setNoHardwareAcceleration: () => {
            // Don't override if already forced for Qt+Windows reason
            if (_forcedReason === 'qt-windows') {
                logger.debug('store/liteMode', 'No hw accel ignored (already forced for Qt+Windows)');
                return;
            }
            logger.info('store/liteMode', 'Forcing lite mode due to no hardware acceleration');
            _isForced = true;
            _forcedReason = 'no-hw-accel';
            set({
                enabled: true,
                reason: 'no-hw-accel',
                isForced: true
            });
        },
        // Set based on user preference (liteMode setting)
        setUserPreference: (liteMode: boolean) => {
            // If forced for any reason, ignore user preference
            if (_isForced) {
                logger.debug('store/liteMode', 'User preference ignored (forced)', { liteMode, reason: _forcedReason });
                return;
            }
            logger.debug('store/liteMode', 'User preference set', { liteMode });
            set({
                enabled: liteMode,
                reason: liteMode ? 'user' : 'none',
                isForced: false
            });
        },
        // Debug override for testing on non-Windows platforms
        setDebugOverride: (enabled: boolean) => {
            logger.info('store/liteMode', 'Debug override set', { enabled });
            set({
                enabled,
                reason: enabled ? 'debug-override' : 'none',
                isForced: _isForced
            });
        },
        // Get just the enabled state (convenience)
        isEnabled: (): boolean => {
            let enabled = false;
            subscribe(state => {
                enabled = state.enabled;
            })();
            return enabled;
        }
    };
}

export const liteModeStore = createLiteModeStore();