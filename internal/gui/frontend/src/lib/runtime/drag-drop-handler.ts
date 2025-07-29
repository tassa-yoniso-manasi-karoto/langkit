/**
 * Global drag and drop handler for different runtime modes
 */

import { getCurrentRuntime, runtimeInitialized } from './stores';
import { logger } from '../logger';
import { get } from 'svelte/store';

type WailsDropCallback = (x: number, y: number, paths: string[]) => void;

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
            logger.error('drag-drop', 'Failed to load Wails runtime', { error });
            return null;
        }
    }
    return null;
}

/**
 * Initialize drag and drop handlers based on runtime mode
 * @param onFileDrop Callback to handle dropped files (Wails signature for compatibility)
 */
export async function initializeDragDrop(onFileDrop: WailsDropCallback) {
    // Wait for runtime to be initialized
    if (!get(runtimeInitialized)) {
        await new Promise<void>((resolve) => {
            const unsubscribe = runtimeInitialized.subscribe(initialized => {
                if (initialized) {
                    unsubscribe();
                    resolve();
                }
            });
            
            // Timeout after 5 seconds
            setTimeout(() => {
                unsubscribe();
                resolve();
            }, 5000);
        });
    }
    
    const runtime = getCurrentRuntime();
    
    // For Anki mode, set up the window functions that Qt will call
    if (runtime === 'anki') {
        logger.info('drag-drop', 'Initializing Qt drag-drop handlers');
        
        // Handler for file drops from Qt
        (window as any).handleFileDrop = (filePath: string) => {
            logger.info('drag-drop', 'File dropped from Qt', { filePath });
            // Convert single file to Wails format (0, 0 for coordinates, array for paths)
            onFileDrop(0, 0, [filePath]);
        };
        
        // Visual feedback handlers
        (window as any).handleDragEnter = () => {
            document.body.classList.add('drag-over');
        };
        
        (window as any).handleDragLeave = () => {
            document.body.classList.remove('drag-over');
        };
    }
    
    // For browser mode, no drag-drop support
    else if (runtime === 'browser') {
        logger.info('drag-drop', 'Browser mode - drag-drop not supported, use file picker buttons');
    }
    
    // Wails mode uses its own drag-drop system
    else if (runtime === 'wails') {
        logger.info('drag-drop', 'Initializing Wails drag-drop handlers');
        const wailsRuntime = await loadWailsRuntime();
        if (wailsRuntime?.OnFileDrop) {
            wailsRuntime.OnFileDrop(onFileDrop, true);
            logger.trace('drag-drop', 'Wails file drop handler registered');
        } else {
            logger.error('drag-drop', 'Wails runtime OnFileDrop not available');
        }
    }
}

/**
 * Clean up drag and drop handlers
 */
export async function cleanupDragDrop() {
    const runtime = getCurrentRuntime();
    
    if (runtime === 'anki') {
        delete (window as any).handleFileDrop;
        delete (window as any).handleDragEnter;
        delete (window as any).handleDragLeave;
        document.body.classList.remove('drag-over');
        logger.trace('drag-drop', 'Qt drag-drop handlers cleaned up');
    }
    
    else if (runtime === 'wails') {
        const wailsRuntimeModule = await loadWailsRuntime();
        if (wailsRuntimeModule?.OnFileDropOff) {
            wailsRuntimeModule.OnFileDropOff();
            logger.trace('drag-drop', 'Wails file drop handler cleaned up');
        }
    }
    
    wailsRuntime = null;
}
