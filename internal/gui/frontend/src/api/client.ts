import { GetAPIPort } from '../../wailsjs/go/gui/App';

// Singleton for API configuration
let apiPort: number | null = null;
let apiBaseUrl: string | null = null;
let initPromise: Promise<void> | null = null;

/**
 * Initialize the API client by discovering the port from the backend
 */
async function initializeAPI(): Promise<void> {
    if (initPromise) {
        return initPromise;
    }

    initPromise = (async () => {
        try {
            const result = await GetAPIPort();
            if (typeof result === 'object' && result !== null && 'error' in result) {
                throw new Error(result.error);
            }
            apiPort = result as number;
            apiBaseUrl = `http://localhost:${apiPort}`;
            console.log(`WebRPC API initialized on port ${apiPort}`);
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