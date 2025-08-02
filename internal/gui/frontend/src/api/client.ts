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
            
            if (apiPort === currentPort || config.runtime === 'browser' || config.runtime === 'anki') {
                // Single-port mode or server mode - use relative URLs
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
 * @param timeout - Timeout in milliseconds. Use 0 or negative value for no timeout.
 */
export function createFetch(timeout: number = 30000): typeof fetch {
    return async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
        const controller = new AbortController();
        let timeoutId: NodeJS.Timeout | undefined;
        
        // Only set timeout if it's a positive number
        if (timeout > 0) {
            timeoutId = setTimeout(() => controller.abort(), timeout);
        }

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
            
            if (timeoutId) {
                clearTimeout(timeoutId);
            }
            return response;
        } catch (error) {
            if (timeoutId) {
                clearTimeout(timeoutId);
            }
            if (error.name === 'AbortError' && timeout > 0) {
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

/**
 * Fetch instance for interactive operations (file dialogs, etc) with no timeout
 */
export const interactiveFetch = createFetch(0);