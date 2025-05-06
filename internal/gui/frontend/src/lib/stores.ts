import { writable } from 'svelte/store';

type Settings = {
    apiKeys: {
        replicate: string;
        assemblyAI: string;
        elevenLabs: string;
        openAI: string;
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
    appStartCount?: number;
    hasSeenLogViewerTooltip?: boolean;
};

/* these values are irrelevant, only the default values of the backend matter */
const initSettings: Settings = {
    apiKeys: {
        replicate: '',
        assemblyAI: '',
        elevenLabs: '',
        openAI: ''
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
    forceWasmMode: 'auto', // Default to automatic decision

    // NEW: LogViewer settings
    logViewerVirtualizationThreshold: 2000, // Default to 2000 logs for virtualization

    // Default values for internal settings
    appStartCount: 0,
    hasSeenLogViewerTooltip: false,
    // Add defaults for missing properties
    eventThrottling: { enabled: true, minInterval: 0, maxInterval: 250 }, // Default object
    convertValues: false    // Assuming default
};

type showSettings = boolean;

export const settings = writable<Settings>(initSettings);
export const showSettings = writable(false);

// Add a reactive store to track when WASM is actively being used
export const wasmActive = writable<boolean>(false);