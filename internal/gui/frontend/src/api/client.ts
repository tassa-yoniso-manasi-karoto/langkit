import { getConfig } from '../config';

// Singleton for API configuration
let apiPort: number | null = null;
let apiBaseUrl: string | null = null;
let initPromise: Promise<void> | null = null;

/**
 * Initialize the API client by getting the port from injected config
 */
async function initializeAPI(): Promise<void> {
    if (initPromise) {
        return initPromise;
    }

    initPromise = (async () => {
        try {
            const config = getConfig();
            apiPort = config.apiPort;
            
            // Check if we're in single-port mode (same origin)
            // In single-port mode, use relative URLs to avoid CORS
            const currentPort = window.location.port ? parseInt(window.location.port) : 
                              (window.location.protocol === 'https:' ? 443 : 80);
            
            if (apiPort === currentPort || config.mode === 'qt') {
                // Single-port mode or Qt mode - use relative URLs
                apiBaseUrl = '/api';
                console.log('WebRPC API using relative URLs (single-port mode)');
            } else {
                // Multi-port mode (Wails) - use absolute URLs with /api prefix
                apiBaseUrl = `http://localhost:${apiPort}/api`;
                console.log(`WebRPC API initialized on port ${apiPort}/api (multi-port mode)`);
            }
        } catch (error) {
            console.error('Failed to initialize API:', error);
            throw error;
        }
    })();

    return initPromise;
}

/**
 * Get the API base URL, initializing if necessary
 */
export async function getAPIBaseUrl(): Promise<string> {
    if (!apiBaseUrl) {
        await initializeAPI();
    }
    if (!apiBaseUrl) {
        throw new Error('API base URL not initialized');
    }
    return apiBaseUrl;
}

/**
 * Create a fetch function with common options for WebRPC
 */
export function createFetch(timeout: number = 30000): typeof fetch {
    return async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), timeout);

        try {
            const response = await fetch(input, {
                ...init,
                signal: controller.signal,
                headers: {
                    'Content-Type': 'application/json',
                    'Accept': 'application/json',
                    ...init?.headers,
                }
            });
            
            clearTimeout(timeoutId);
            return response;
        } catch (error) {
            clearTimeout(timeoutId);
            if (error.name === 'AbortError') {
                throw new Error(`Request timeout after ${timeout}ms`);
            }
            throw error;
        }
    };
}

/**
 * Default fetch instance with 30 second timeout
 */
export const defaultFetch = createFetch(30000);