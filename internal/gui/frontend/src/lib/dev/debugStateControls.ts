import { llmStateStore, userActivityState as userActivityStateStore, dockerStatusStore, internetStatusStore, ffmpegStatusStore, mediainfoStatusStore } from '../stores';
import { logger } from '../logger';

// Store the last real LLM state before forcing
let lastRealLLMState: any = null;
let isLLMStateForced = false;

// Subscribe to LLM state changes to capture real states
llmStateStore.subscribe((state) => {
    // Only save as "last real state" if it's not a forced state
    if (state && !state?.message?.startsWith('Debug: Forced')) {
        lastRealLLMState = state;
        isLLMStateForced = false;
    }
});

// LLM state control functions
export function forceLLMState(state: 'initializing' | 'ready' | 'error' | 'updating') {
    // Mark that we're now in forced state
    isLLMStateForced = true;
    
    const mockStateChange = {
        timestamp: new Date().toISOString(),
        globalState: state,
        providerStatesSnapshot: {},
        message: state === 'error' ? 'Debug: Forced error state' : `Debug: Forced ${state} state`
    };
    
    llmStateStore.set(mockStateChange);
    logger.debug('devDashboard', `Forced LLM state to: ${state}`);
}

export function resetLLMState() {
    // Restore the last known real state if we have one
    if (lastRealLLMState) {
        llmStateStore.set(lastRealLLMState);
        logger.debug('devDashboard', 'Reset LLM state to last known real state', { 
            globalState: lastRealLLMState.globalState 
        });
    } else {
        // If no real state was captured yet, set to null
        llmStateStore.set(null);
        logger.debug('devDashboard', 'Reset LLM state (no previous real state available)');
    }
    isLLMStateForced = false;
}

// User activity state control functions
export function forceUserActivityState(state: 'active' | 'idle' | 'afk') {
    userActivityStateStore.set(state, true); // true = forced
    logger.debug('devDashboard', `Forced user activity state to: ${state}`);
}

export function resetUserActivityState() {
    userActivityStateStore.reset();
    logger.debug('devDashboard', 'Reset user activity state to automatic detection');
}

// Docker control functions
export function forceDockerStatus(available: boolean) {
    dockerStatusStore.set({
        available: available,
        version: available ? 'Debug Mode' : '',
        error: available ? '' : 'Debug: Forced state',
        checked: true
    });
    logger.debug('devDashboard', `Forced Docker status to: ${available ? 'available' : 'unavailable'}`);
}

export function resetDockerStatus() {
    // Re-run the actual check by importing and calling the function from App.svelte
    import('../../api/services/deps').then(({ CheckDockerAvailability }) => {
        CheckDockerAvailability().then(status => {
            dockerStatusStore.set({
                available: status.available || false,
                version: status.version,
                engine: status.engine,
                error: status.error,
                checked: true
            });
            logger.debug('devDashboard', 'Reset Docker status to real state', status);
        });
    });
}

// Internet control functions
export function forceInternetStatus(online: boolean) {
    internetStatusStore.set({
        online: online,
        latency: online ? 50 : 0,
        error: online ? '' : 'Debug: Forced state',
        checked: true
    });
    logger.debug('devDashboard', `Forced Internet status to: ${online ? 'online' : 'offline'}`);
}

export function resetInternetStatus() {
    // Re-run the actual check by importing and calling the function from App.svelte
    import('../../api/services/deps').then(({ CheckInternetConnectivity }) => {
        CheckInternetConnectivity().then(status => {
            internetStatusStore.set({
                online: status.online || false,
                latency: status.latency,
                error: status.error,
                checked: true
            });
            logger.debug('devDashboard', 'Reset Internet status to real state', status);
        });
    });
}

// FFmpeg control functions
export function forceFFmpegStatus(available: boolean) {
    ffmpegStatusStore.set({
        available: available,
        version: available ? 'Debug Mode' : '',
        path: available ? '/debug/ffmpeg' : '',
        error: available ? '' : 'Debug: Forced state',
        checked: true
    });
    logger.debug('devDashboard', `Forced FFmpeg status to: ${available ? 'available' : 'unavailable'}`);
}

export function resetFFmpegStatus() {
    import('../../api/services/deps').then(({ CheckFFmpegAvailability }) => {
        CheckFFmpegAvailability().then(status => {
            ffmpegStatusStore.set({
                available: status.available || false,
                version: status.version,
                path: status.path,
                error: status.error,
                checked: true
            });
            logger.debug('devDashboard', 'Reset FFmpeg status to real state', status);
        });
    });
}

// MediaInfo control functions
export function forceMediaInfoStatus(available: boolean) {
    mediainfoStatusStore.set({
        available: available,
        version: available ? 'Debug Mode' : '',
        path: available ? '/debug/mediainfo' : '',
        error: available ? '' : 'Debug: Forced state',
        checked: true
    });
    logger.debug('devDashboard', `Forced MediaInfo status to: ${available ? 'available' : 'unavailable'}`);
}

export function resetMediaInfoStatus() {
    import('../../api/services/deps').then(({ CheckMediaInfoAvailability }) => {
        CheckMediaInfoAvailability().then(status => {
            mediainfoStatusStore.set({
                available: status.available || false,
                version: status.version,
                path: status.path,
                error: status.error,
                checked: true
            });
            logger.debug('devDashboard', 'Reset MediaInfo status to real state', status);
        });
    });
}

// Debounce base value state
let baseDebounceOverride: number | undefined = undefined;

// Debounce control functions
export function setBaseDebounceValue(value: number) {
    baseDebounceOverride = value;
    logger.debug('devDashboard', `Set base debounce override to: ${value}ms`);
}

export function getBaseDebounceValue(): number | undefined {
    return baseDebounceOverride;
}

export function resetBaseDebounceValue() {
    baseDebounceOverride = undefined;
    logger.debug('devDashboard', 'Reset base debounce to default (200ms)');
}

// Legacy functions for backward compatibility (will be removed later)
export function setDebounceOverride(os: 'windows' | 'other', value: number) {
    // Map to base debounce value
    setBaseDebounceValue(value);
}

export function getDebounceOverride(os: 'windows' | 'other'): number | undefined {
    // Return base debounce for any OS
    return baseDebounceOverride;
}

export function resetDebounceOverride(os: 'windows' | 'other') {
    // Reset base debounce
    resetBaseDebounceValue();
}

export function resetAllDebounceOverrides() {
    resetBaseDebounceValue();
}

// Get current debounce state for UI display
export function getDebounceState() {
    return {
        baseOverride: baseDebounceOverride,
        baseDefault: 200,
        // Legacy compatibility
        windowsOverride: baseDebounceOverride,
        otherOverride: baseDebounceOverride,
        windowsDefault: 200,
        otherDefault: 200
    };
}