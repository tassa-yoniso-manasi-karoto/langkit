import { DryRunService } from '../generated/api.gen';
import { getAPIBaseUrl, defaultFetch } from '../client';

// Singleton instance
let dryRunServiceInstance: DryRunService | null = null;

/**
 * Get or create the dry run service instance
 */
async function getDryRunService(): Promise<DryRunService> {
    if (!dryRunServiceInstance) {
        const baseUrl = await getAPIBaseUrl();
        dryRunServiceInstance = new DryRunService(baseUrl, defaultFetch);
    }
    return dryRunServiceInstance;
}

/**
 * SetDryRunConfig - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function SetDryRunConfig(config: Record<string, any>): Promise<void> {
    const service = await getDryRunService();
    
    try {
        // Convert the config object to match the generated types
        await service.setConfig({
            config: {
                enabled: config.enabled || false,
                delayMs: config.delayMs || 1000,
                processedCount: config.processedCount || 0,
                nextErrorIndex: config.nextErrorIndex || -1,
                nextErrorType: config.nextErrorType,
                errorPoints: config.errorPoints || {}
            }
        });
    } catch (error) {
        console.error('SetDryRunConfig error:', error);
        throw error;
    }
}

/**
 * InjectDryRunError - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function InjectDryRunError(errorType: string): Promise<void> {
    const service = await getDryRunService();
    
    try {
        await service.injectError({ errorType });
    } catch (error) {
        console.error('InjectDryRunError error:', error);
        throw error;
    }
}

/**
 * GetDryRunStatus - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function GetDryRunStatus(): Promise<Record<string, any>> {
    const service = await getDryRunService();
    
    try {
        const response = await service.getStatus();
        
        // Convert the response to match the expected format
        const status: Record<string, any> = {
            enabled: response.status.enabled,
            delayMs: response.status.delayMs,
            processedCount: response.status.processedCount,
            nextErrorIndex: response.status.nextErrorIndex
        };
        
        if (response.status.nextErrorType) {
            status.nextErrorType = response.status.nextErrorType;
        }
        
        if (response.status.errorPoints) {
            status.errorPoints = response.status.errorPoints;
        }
        
        return status;
    } catch (error) {
        console.error('GetDryRunStatus error:', error);
        throw error;
    }
}