// Runtime configuration injected by the backend
export interface LangkitConfig {
    apiPort: number;
    wsPort: number;
    mode: 'wails' | 'qt';
    runtime: 'wails' | 'anki';
}

// Extend Window interface to include our config
declare global {
    interface Window {
        __LANGKIT_CONFIG__?: LangkitConfig;
    }
}

/**
 * Get the runtime configuration injected by the backend
 * @throws Error if configuration is not found
 */
export function getConfig(): LangkitConfig {
    const config = window.__LANGKIT_CONFIG__;
    if (!config) {
        throw new Error('Runtime configuration not found. __LANGKIT_CONFIG__ is not defined.');
    }
    return config;
}

/**
 * Check if runtime configuration is available
 */
export function hasConfig(): boolean {
    return window.__LANGKIT_CONFIG__ !== undefined;
}