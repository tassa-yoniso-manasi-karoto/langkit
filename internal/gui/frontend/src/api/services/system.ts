import { SystemService } from '../generated/api.gen';
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
            version: response.info.version,
            commit: response.info.commit,
            branch: response.info.branch,
            newerVersionAvailable: response.info.newerVersionAvailable
        };
    } catch (error) {
        console.error('GetVersion error:', error);
        throw error;
    }
}