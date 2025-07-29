/**
 * Runtime module - Centralized runtime detection and utilities
 * 
 * Use the stores for reactive runtime detection in components:
 * - $isWailsMode
 * - $isBrowserMode  
 * - $isAnkiMode
 * - $runtimeInitialized
 * 
 * Also exports drag-drop handlers and safe wrappers for Wails-specific functions.
 */

import { writable, derived, type Readable } from 'svelte/store';

// Create stores first, before any other imports that might use them
// This prevents initialization order issues

// The runtime configuration store with safe defaults
const runtimeConfig = writable({
    runtime: 'browser' as 'wails' | 'browser' | 'anki',
    initialized: false
});

// Derived stores for each runtime mode
export const isWailsMode: Readable<boolean> = derived(
    runtimeConfig,
    ($config) => $config.runtime === 'wails'
);

export const isBrowserMode: Readable<boolean> = derived(
    runtimeConfig,
    ($config) => $config.runtime === 'browser'
);

export const isAnkiMode: Readable<boolean> = derived(
    runtimeConfig,
    ($config) => $config.runtime === 'anki'
);

export const runtimeInitialized: Readable<boolean> = derived(
    runtimeConfig,
    ($config) => $config.initialized
);

// Now we can safely import other modules
import { getConfig, hasConfig } from '../../config';
import { logger } from '../logger';

/**
 * Initialize runtime stores from config
 * Should be called once when the app starts
 */
export async function initializeRuntimeStores(): Promise<void> {
    logger.debug('runtime-stores', 'Initializing runtime stores...');
    
    // Wait for config to be available
    if (!hasConfig()) {
        logger.debug('runtime-stores', 'Waiting for config to be available...');
        await new Promise<void>((resolve) => {
            const checkInterval = setInterval(() => {
                if (hasConfig()) {
                    clearInterval(checkInterval);
                    resolve();
                }
            }, 100);
            
            // Timeout after 5 seconds
            setTimeout(() => {
                clearInterval(checkInterval);
                logger.warn('runtime-stores', 'Config not available after 5 seconds, defaulting to browser mode');
                resolve();
            }, 5000);
        });
    }
    
    // Get runtime from config
    let runtime: 'wails' | 'browser' | 'anki' = 'browser'; // Default to browser
    if (hasConfig()) {
        try {
            const config = getConfig();
            runtime = config.runtime;
            logger.info('runtime-stores', `Runtime detected: ${runtime}`);
        } catch (error) {
            logger.error('runtime-stores', 'Failed to get config', { error });
        }
    }
    
    // Update the store
    runtimeConfig.set({
        runtime,
        initialized: true
    });
    logger.debug('runtime-stores', 'Runtime stores initialized');
}

/**
 * Get current runtime mode (for non-reactive use)
 * Use this only when you need a synchronous value (e.g., in event handlers).
 * For reactive UI, use the $isWailsMode, $isBrowserMode, or $isAnkiMode stores instead.
 */
export function getCurrentRuntime(): 'wails' | 'browser' | 'anki' {
    let current: 'wails' | 'browser' | 'anki' = 'browser';
    const unsubscribe = runtimeConfig.subscribe(config => {
        current = config.runtime;
    });
    unsubscribe();
    return current;
}

// Export drag-drop handlers
export { initializeDragDrop, cleanupDragDrop } from './drag-drop-handler';

// Export safe wrappers for Wails-specific functions
export { safeWindowIsMinimised, safeWindowIsMaximised } from './bridge';