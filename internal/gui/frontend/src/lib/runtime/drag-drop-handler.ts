/**
 * Global drag and drop handler for different runtime modes
 */

import { isAnkiMode, isBrowserMode, isWailsMode } from './bridge';
import { logger } from '../logger';
import { hasConfig } from '../../config';

type FileDropCallback = (filePath: string) => void;
type WailsDropCallback = (x: number, y: number, paths: string[]) => void;

let wailsRuntime: any = null;

/**
 * Load Wails runtime module if in Wails mode
 */
async function loadWailsRuntime() {
    if (wailsRuntime) return wailsRuntime;
    
    if (isWailsMode()) {
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
    // Wait for config to be available if not already loaded
    if (!hasConfig()) {
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
                resolve();
            }, 5000);
        });
    }
    
    // For Anki mode, set up the window functions that Qt will call
    if (isAnkiMode()) {
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
    else if (isBrowserMode()) {
        logger.info('drag-drop', 'Browser mode - drag-drop not supported, use file picker buttons');
    }
    
    // Wails mode uses its own drag-drop system
    else if (isWailsMode()) {
        logger.info('drag-drop', 'Initializing Wails drag-drop handlers');
        const runtime = await loadWailsRuntime();
        if (runtime?.OnFileDrop) {
            runtime.OnFileDrop(onFileDrop, true);
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
    if (isAnkiMode()) {
        delete (window as any).handleFileDrop;
        delete (window as any).handleDragEnter;
        delete (window as any).handleDragLeave;
        document.body.classList.remove('drag-over');
        logger.trace('drag-drop', 'Qt drag-drop handlers cleaned up');
    }
    
    else if (isWailsMode()) {
        const runtime = await loadWailsRuntime();
        if (runtime?.OnFileDropOff) {
            runtime.OnFileDropOff();
            logger.trace('drag-drop', 'Wails file drop handler cleaned up');
        }
    }
    
    wailsRuntime = null;
}
