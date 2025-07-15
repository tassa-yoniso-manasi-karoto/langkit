import { SettingsService } from '../generated/api.gen';
import { getAPIBaseUrl, defaultFetch } from '../client';

// Singleton instance
let settingsServiceInstance: SettingsService | null = null;

/**
 * Get or create the settings service instance
 */
async function getSettingsService(): Promise<SettingsService> {
    if (!settingsServiceInstance) {
        const baseUrl = await getAPIBaseUrl();
        settingsServiceInstance = new SettingsService(baseUrl, defaultFetch);
    }
    return settingsServiceInstance;
}

/**
 * InitSettings - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function InitSettings(): Promise<void> {
    const service = await getSettingsService();
    
    try {
        await service.initSettings();
    } catch (error) {
        console.error('InitSettings error:', error);
        throw error;
    }
}

/**
 * LoadSettings - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function LoadSettings(): Promise<Settings> {
    const service = await getSettingsService();
    
    try {
        const response = await service.loadSettings();
        return response.settings;
    } catch (error) {
        console.error('LoadSettings error:', error);
        throw error;
    }
}

/**
 * SaveSettings - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function SaveSettings(settings: Settings): Promise<void> {
    const service = await getSettingsService();
    
    try {
        await service.saveSettings({ settings });
    } catch (error) {
        console.error('SaveSettings error:', error);
        throw error;
    }
}

/**
 * LoadStatistics - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function LoadStatistics(): Promise<Record<string, any>> {
    const service = await getSettingsService();
    
    try {
        const response = await service.loadStatistics();
        return response.statistics;
    } catch (error) {
        console.error('LoadStatistics error:', error);
        throw error;
    }
}

/**
 * UpdateStatistics - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function UpdateStatistics(updates: Record<string, any>): Promise<void> {
    const service = await getSettingsService();
    
    try {
        await service.updateStatistics({ updates });
    } catch (error) {
        console.error('UpdateStatistics error:', error);
        throw error;
    }
}

/**
 * IncrementStatistic - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function IncrementStatistic(key: string): Promise<number> {
    const service = await getSettingsService();
    
    try {
        const response = await service.incrementStatistic({ key });
        return response.result.newValue;
    } catch (error) {
        console.error('IncrementStatistic error:', error);
        throw error;
    }
}