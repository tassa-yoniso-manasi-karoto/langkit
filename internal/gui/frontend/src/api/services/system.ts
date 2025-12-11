import { SystemService } from '../generated/api.gen';
import type { OpenURLArgs } from '../generated/api.gen';
import { getAPIBaseUrl, defaultFetch } from '../client';

// Singleton instance
let systemServiceInstance: SystemService | null = null;

/**
 * Get or create the system service instance
 */
async function getSystemService(): Promise<SystemService> {
    if (!systemServiceInstance) {
        const baseUrl = await getAPIBaseUrl();
        systemServiceInstance = new SystemService(baseUrl, defaultFetch);
    }
    return systemServiceInstance;
}

/**
 * GetSystemInfo - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function GetSystemInfo(): Promise<Record<string, string>> {
    const service = await getSystemService();
    
    try {
        const response = await service.getSystemInfo();
        return {
            os: response.info.os,
            arch: response.info.arch
        };
    } catch (error) {
        console.error('GetSystemInfo error:', error);
        throw error;
    }
}

/**
 * GetVersion - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function GetVersion(): Promise<any> {
    const service = await getSystemService();
    
    try {
        const response = await service.getVersion();
        return {
            version: response.info.version
        };
    } catch (error) {
        console.error('GetVersion error:', error);
        throw error;
    }
}

/**
 * CheckForUpdate - Check if a newer version is available
 */
export async function CheckForUpdate(): Promise<{ updateAvailable: boolean }> {
    const service = await getSystemService();
    
    try {
        const response = await service.checkForUpdate();
        return {
            updateAvailable: response.updateAvailable
        };
    } catch (error) {
        console.error('CheckForUpdate error:', error);
        throw error;
    }
}

/**
 * OpenURL - Drop-in replacement for Wails method
 * Opens a URL in the user's default browser
 * Maintains exact same signature as the Wails version
 */
export async function OpenURL(url: string): Promise<void> {
    const service = await getSystemService();

    try {
        await service.openURL({ url });
    } catch (error) {
        console.error('OpenURL error:', error);
        throw error;
    }
}

/**
 * ShowWarning - Display a warning dialog to the user
 * Returns true if the user acknowledged the warning
 */
export async function ShowWarning(title: string, message: string): Promise<boolean> {
    const service = await getSystemService();

    try {
        const response = await service.showWarning({ title, message });
        return response.acknowledged;
    } catch (error) {
        console.error('ShowWarning error:', error);
        throw error;
    }
}