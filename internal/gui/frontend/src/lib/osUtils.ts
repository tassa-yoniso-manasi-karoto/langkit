import { get } from 'svelte/store';
import { systemInfoStore } from './stores';
import { getDebounceOverride } from './dev/debugStateControls';

/**
 * Get the appropriate debounce delay based on the operating system.
 * Windows (WebView2) needs more delay due to message queue limitations.
 * 
 * @returns 200ms for Windows, 10ms for other platforms (or debug override if set)
 */
export function getOSDebounceDelay(): number {
    const systemInfo = get(systemInfoStore);
    
    // Check for debug override first
    const osKey = systemInfo.os === 'windows' ? 'windows' : 'other';
    const override = getDebounceOverride(osKey);
    
    if (override !== undefined) {
        return override;
    }
    
    // Default values
    if (systemInfo.os === 'windows') {
        return 200;
    }
    
    // WebKit on macOS/Linux can handle shorter delays
    return 10;
}