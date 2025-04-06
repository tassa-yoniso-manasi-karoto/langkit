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
    // WebAssembly settings
    useWasm: boolean;
    wasmSizeThreshold: number;
    forceWasmMode: 'auto' | 'enabled' | 'disabled'; // Add force override mode
    eventThrottling: boolean; // Make required
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
    // Default values for WebAssembly settings
    useWasm: false,
    wasmSizeThreshold: 500, // Default from spec
    forceWasmMode: 'auto', // Default to automatic decision
    // Default values for internal settings
    appStartCount: 0,
    hasSeenLogViewerTooltip: false,
    // Add defaults for missing properties
    eventThrottling: false, // Assuming default
    convertValues: false    // Assuming default
};

type showSettings = boolean;

export const settings = writable<Settings>(initSettings);
export const showSettings = writable(false);

// Add a reactive store to track when WASM is actively being used
export const wasmActive = writable<boolean>(false);