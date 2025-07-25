import { ProcessingService } from '../generated/api.gen';
import type { ProcessRequest, ProcessingStatus } from '../generated/api.gen';
import { getAPIBaseUrl, defaultFetch } from '../client';

// Singleton instance
let processingServiceInstance: ProcessingService | null = null;

/**
 * Get or create the processing service instance
 */
async function getProcessingService(): Promise<ProcessingService> {
    if (!processingServiceInstance) {
        const baseUrl = await getAPIBaseUrl();
        processingServiceInstance = new ProcessingService(baseUrl, defaultFetch);
    }
    return processingServiceInstance;
}

/**
 * SendProcessingRequest - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function SendProcessingRequest(request: ProcessRequest): Promise<void> {
    const service = await getProcessingService();
    
    try {
        const response = await service.sendProcessingRequest({ request });
        
        // If there's an error in the response, throw it
        if (response.status.error) {
            throw new Error(response.status.error);
        }
    } catch (error) {
        console.error('SendProcessingRequest error:', error);
        throw error;
    }
}

/**
 * CancelProcessing - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function CancelProcessing(): Promise<void> {
    const service = await getProcessingService();
    
    try {
        await service.cancelProcessing();
    } catch (error) {
        console.error('CancelProcessing error:', error);
        throw error;
    }
}

/**
 * GetProcessingStatus - Additional method to check processing status
 * Not in original Wails API but useful for the new architecture
 */
export async function GetProcessingStatus(): Promise<ProcessingStatus> {
    const service = await getProcessingService();
    
    try {
        const response = await service.getProcessingStatus();
        return response.status;
    } catch (error) {
        console.error('GetProcessingStatus error:', error);
        throw error;
    }
}