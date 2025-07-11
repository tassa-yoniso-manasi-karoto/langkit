import { get } from 'svelte/store';
import { systemInfoStore } from './stores';

/**
 * Get the appropriate debounce delay based on the operating system.
 * Windows (WebView2) needs more delay due to message queue limitations.
 * 
 * @returns 50ms for Windows, 10ms for other platforms
 */
export function getOSDebounceDelay(): number {
    const systemInfo = get(systemInfoStore);
    
    // Windows needs more delay for WebView2 stability
    if (systemInfo.os === 'windows') {
        return 50;
    }
    
    // WebKit on macOS/Linux can handle shorter delays
    return 10;
}