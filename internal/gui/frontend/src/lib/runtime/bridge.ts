/**
 * Runtime Bridge - Provides safe wrappers for runtime-specific functions
 * Allows the app to work in both Wails and Qt/server modes
 */

import { logger } from '../logger';
import { getCurrentRuntime } from './stores';

// Cache for loaded Wails runtime module
let wailsRuntime: any = null;

/**
 * Load Wails runtime module if in Wails mode
 */
async function loadWailsRuntime() {
    if (wailsRuntime) return wailsRuntime;
    
    const runtime = getCurrentRuntime();
    if (runtime === 'wails') {
        try {
            wailsRuntime = await import('../../../wailsjs/runtime/runtime');
            return wailsRuntime;
        } catch (error) {
            logger.error('runtime-bridge', 'Failed to load Wails runtime', { error });
            return null;
        }
    }
    return null;
}

/**
 * Safe wrapper for WindowIsMinimised
 * Returns false in non-Wails mode
 */
export async function safeWindowIsMinimised(): Promise<boolean> {
    const runtime = await loadWailsRuntime();
    if (runtime?.WindowIsMinimised) {
        return await runtime.WindowIsMinimised();
    }
    return false;
}

/**
 * Safe wrapper for WindowIsMaximised
 * Returns false in non-Wails mode
 */
export async function safeWindowIsMaximised(): Promise<boolean> {
    const runtime = await loadWailsRuntime();
    if (runtime?.WindowIsMaximised) {
        return await runtime.WindowIsMaximised();
    }
    return false;
}

