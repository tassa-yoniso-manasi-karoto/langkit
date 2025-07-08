import { llmStateStore, userActivityState as userActivityStateStore, dockerStatusStore, internetStatusStore, ffmpegStatusStore, mediainfoStatusStore } from '../stores';
import { logger } from '../logger';

// LLM state control functions
export function forceLLMState(state: 'initializing' | 'ready' | 'error' | 'updating') {
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
    // Clear any debug forced state by setting a null/empty state
    // The WebSocket will then update with the real state
    llmStateStore.set(null);
    logger.debug('devDashboard', 'Reset LLM state to real backend state');
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
    import('../../../wailsjs/go/gui/App').then(({ CheckDockerAvailability }) => {
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
    import('../../../wailsjs/go/gui/App').then(({ CheckInternetConnectivity }) => {
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
    import('../../../wailsjs/go/gui/App').then(({ CheckFFmpegAvailability }) => {
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
    import('../../../wailsjs/go/gui/App').then(({ CheckMediaInfoAvailability }) => {
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