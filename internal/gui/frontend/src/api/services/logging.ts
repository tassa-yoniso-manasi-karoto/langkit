import { LoggingService } from '../generated/api.gen';
import { getAPIBaseUrl, defaultFetch } from '../client';

// Singleton instance
let loggingServiceInstance: LoggingService | null = null;

/**
 * Get or create the logging service instance
 */
async function getLoggingService(): Promise<LoggingService> {
    if (!loggingServiceInstance) {
        const baseUrl = await getAPIBaseUrl();
        loggingServiceInstance = new LoggingService(baseUrl, defaultFetch);
    }
    return loggingServiceInstance;
}

/**
 * BackendLogger - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function BackendLogger(component: string, logJson: string): Promise<void> {
    const service = await getLoggingService();
    
    try {
        await service.backendLogger({ component, logJson });
    } catch (error) {
        // Don't throw errors for logging - just log to console
        console.error('BackendLogger error:', error);
    }
}

/**
 * BackendLoggerBatch - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function BackendLoggerBatch(component: string, logsJson: string): Promise<void> {
    const service = await getLoggingService();
    
    try {
        await service.backendLoggerBatch({ component, logsJson });
    } catch (error) {
        // Don't throw errors for logging - just log to console
        console.error('BackendLoggerBatch error:', error);
    }
}

/**
 * SetTraceLogs - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function SetTraceLogs(enable: boolean): Promise<void> {
    const service = await getLoggingService();
    
    try {
        await service.setTraceLogs({ enable });
    } catch (error) {
        console.error('SetTraceLogs error:', error);
        throw error;
    }
}

/**
 * GetTraceLogs - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function GetTraceLogs(): Promise<boolean> {
    const service = await getLoggingService();
    
    try {
        const response = await service.getTraceLogs();
        return response.enabled;
    } catch (error) {
        console.error('GetTraceLogs error:', error);
        throw error;
    }
}

/**
 * RecordWasmState - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function RecordWasmState(stateJson: string): Promise<void> {
    const service = await getLoggingService();
    
    try {
        await service.recordWasmState({ stateJson });
    } catch (error) {
        // Don't throw errors for state recording - just log to console
        console.error('RecordWasmState error:', error);
    }
}

/**
 * RequestWasmState - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function RequestWasmState(): Promise<void> {
    const service = await getLoggingService();
    
    try {
        await service.requestWasmState();
    } catch (error) {
        console.error('RequestWasmState error:', error);
        throw error;
    }
}

/**
 * ExportDebugReport - Drop-in replacement for Wails method
 * Accepts optional graphics info string to include WebView GPU details in the report
 */
export async function ExportDebugReport(graphicsInfo?: string): Promise<void> {
    const service = await getLoggingService();

    try {
        await service.exportDebugReport({ graphicsInfo });
    } catch (error) {
        console.error('ExportDebugReport error:', error);
        throw error;
    }
}

/**
 * SetEventThrottling - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function SetEventThrottling(enabled: boolean): Promise<void> {
    const service = await getLoggingService();
    
    try {
        await service.setEventThrottling({ enabled });
    } catch (error) {
        console.error('SetEventThrottling error:', error);
        throw error;
    }
}

/**
 * GetEventThrottlingStatus - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function GetEventThrottlingStatus(): Promise<Record<string, any>> {
    const service = await getLoggingService();
    
    try {
        const response = await service.getEventThrottlingStatus();
        const status = response.status;
        
        // Convert to the expected format
        const result: Record<string, any> = {
            enabled: status.enabled,
            currentRate: status.currentRate,
            currentInterval: status.currentInterval
        };
        
        if (status.error) {
            result.error = status.error;
        }
        
        return result;
    } catch (error) {
        console.error('GetEventThrottlingStatus error:', error);
        throw error;
    }
}