import { ModelService } from '../generated/api.gen';
import { getAPIBaseUrl, defaultFetch } from '../client';

// Singleton instance
let modelServiceInstance: ModelService | null = null;

/**
 * Get or create the model service instance
 */
async function getModelService(): Promise<ModelService> {
    if (!modelServiceInstance) {
        const baseUrl = await getAPIBaseUrl();
        modelServiceInstance = new ModelService(baseUrl, defaultFetch);
    }
    return modelServiceInstance;
}

/**
 * GetAvailableSTTModelsForUI - Drop-in replacement for Wails method
 * Gets all available Speech-to-Text (STT) models for the UI
 */
export async function GetAvailableSTTModelsForUI(): Promise<STTModelsResponse> {
    const service = await getModelService();
    
    try {
        const response = await service.getAvailableSTTModelsForUI();
        return response.response;
    } catch (error) {
        console.error('GetAvailableSTTModelsForUI error:', error);
        throw error;
    }
}

/**
 * RefreshSTTModelsAfterSettingsUpdate - Drop-in replacement for Wails method
 * Refreshes the list of available STT models after API keys are updated
 */
export async function RefreshSTTModelsAfterSettingsUpdate(): Promise<STTModelsResponse> {
    const service = await getModelService();
    
    try {
        const response = await service.refreshSTTModelsAfterSettingsUpdate();
        return response.response;
    } catch (error) {
        console.error('RefreshSTTModelsAfterSettingsUpdate error:', error);
        throw error;
    }
}

/**
 * GetAvailableSummaryProviders - Drop-in replacement for Wails method
 * Gets a list of available LLM providers for text summarization
 * Returns a map-like object for backward compatibility
 */
export async function GetAvailableSummaryProviders(): Promise<{ [key: string]: any }> {
    const service = await getModelService();
    
    try {
        const response = await service.getAvailableSummaryProviders();
        const data = response.response;
        
        // Convert typed response to map structure for backward compatibility
        const result: { [key: string]: any } = {
            providers: data.providers.map(p => ({
                name: p.name,
                displayName: p.displayName,
                description: p.description,
                ...(p.status && { status: p.status }),
                ...(p.error && { error: p.error })
            })),
            names: data.names,
            available: data.available,
            suggested: data.suggested,
            status: data.status
        };
        
        if (data.message) {
            result.message = data.message;
        }
        
        return result;
    } catch (error) {
        console.error('GetAvailableSummaryProviders error:', error);
        throw error;
    }
}

/**
 * GetAvailableSummaryModels - Drop-in replacement for Wails method
 * Gets a list of available models for a specific summary provider
 * Returns a map-like object for backward compatibility
 */
export async function GetAvailableSummaryModels(providerName: string): Promise<{ [key: string]: any }> {
    const service = await getModelService();
    
    try {
        const response = await service.getAvailableSummaryModels({ providerName });
        const data = response.response;
        
        // Convert typed response to map structure for backward compatibility
        const result: { [key: string]: any } = {
            models: data.models.map(m => ({
                id: m.id,
                name: m.name,
                description: m.description,
                providerName: m.providerName
            })),
            names: data.names,
            available: data.available,
            suggested: data.suggested,
            status: data.status
        };
        
        if (data.message) {
            result.message = data.message;
        }
        
        return result;
    } catch (error) {
        console.error('GetAvailableSummaryModels error:', error);
        throw error;
    }
}