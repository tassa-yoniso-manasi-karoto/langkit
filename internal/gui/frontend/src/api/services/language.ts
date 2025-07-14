import { LanguageService } from '../generated/api.gen';
import { getAPIBaseUrl, defaultFetch } from '../client';

// Singleton instance
let languageServiceInstance: LanguageService | null = null;

/**
 * Get or create the language service instance
 */
async function getLanguageService(): Promise<LanguageService> {
    if (!languageServiceInstance) {
        const baseUrl = await getAPIBaseUrl();
        languageServiceInstance = new LanguageService(baseUrl, defaultFetch);
    }
    return languageServiceInstance;
}

// Type definitions matching Wails API
interface LanguageCheckResponse {
    standardTag: string;
    isValid: boolean;
    error?: string;
}

/**
 * ValidateLanguageTag - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function ValidateLanguageTag(tagsString: string, maxOne: boolean): Promise<LanguageCheckResponse> {
    const service = await getLanguageService();
    
    try {
        const response = await service.validateLanguage({
            validation: {
                tag: tagsString,
                single: maxOne
            }
        });
        
        return {
            standardTag: response.response.standardTag,
            isValid: response.response.valid,
            error: response.response.error || undefined
        };
    } catch (error) {
        console.error('ValidateLanguageTag error:', error);
        return {
            standardTag: '',
            isValid: false,
            error: error instanceof Error ? error.message : 'Unknown error'
        };
    }
}

/**
 * GetRomanizationStyles - WebRPC version
 */
export async function GetRomanizationStyles(languageCode: string) {
    const service = await getLanguageService();
    
    try {
        const response = await service.getRomanizationStyles({ languageCode });
        return response.styles;
    } catch (error) {
        console.error('GetRomanizationStyles error:', error);
        throw error;
    }
}

/**
 * NeedsTokenization - WebRPC version
 */
export async function NeedsTokenization(language: string): Promise<boolean> {
    const service = await getLanguageService();
    
    try {
        const response = await service.needsTokenization({ language });
        return response.needed;
    } catch (error) {
        console.error('NeedsTokenization error:', error);
        return false;
    }
}

/**
 * GetLanguageRequirements - WebRPC version
 */
export async function GetLanguageRequirements(languageTag: string) {
    const service = await getLanguageService();
    
    try {
        const response = await service.getLanguageRequirements({ languageTag });
        return response.requirements;
    } catch (error) {
        console.error('GetLanguageRequirements error:', error);
        throw error;
    }
}