import { DependencyService } from '../generated/api.gen';
import { getAPIBaseUrl, defaultFetch, interactiveFetch } from '../client';

// Singleton instance for quick checks (with 30s timeout)
let dependencyServiceInstance: DependencyService | null = null;

// Singleton instance for downloads (no timeout)
let downloadServiceInstance: DependencyService | null = null;

/**
 * Get or create the dependency service instance for quick checks
 */
async function getDependencyService(): Promise<DependencyService> {
    if (!dependencyServiceInstance) {
        const baseUrl = await getAPIBaseUrl();
        dependencyServiceInstance = new DependencyService(baseUrl, defaultFetch);
    }
    return dependencyServiceInstance;
}

/**
 * Get or create the dependency service instance for downloads (no timeout)
 */
async function getDownloadService(): Promise<DependencyService> {
    if (!downloadServiceInstance) {
        const baseUrl = await getAPIBaseUrl();
        downloadServiceInstance = new DependencyService(baseUrl, interactiveFetch);
    }
    return downloadServiceInstance;
}

/**
 * CheckDockerAvailability - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function CheckDockerAvailability(): Promise<Record<string, any>> {
    const service = await getDependencyService();
    
    try {
        const response = await service.checkDockerAvailability({});
        const status = response.status;
        
        return {
            available: status.available,
            version: status.version,
            engine: status.engine,
            error: status.error || ""
        };
    } catch (error) {
        console.error('CheckDockerAvailability error:', error);
        return {
            available: false,
            version: "",
            engine: "",
            error: error instanceof Error ? error.message : 'Unknown error'
        };
    }
}

/**
 * CheckInternetConnectivity - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function CheckInternetConnectivity(): Promise<Record<string, any>> {
    const service = await getDependencyService();
    
    try {
        const response = await service.checkInternetConnectivity({});
        const status = response.status;
        
        return {
            online: status.online,
            latency: Number(status.latency), // Convert bigint to number
            error: status.error || ""
        };
    } catch (error) {
        console.error('CheckInternetConnectivity error:', error);
        return {
            online: false,
            latency: 0,
            error: error instanceof Error ? error.message : 'Unknown error'
        };
    }
}

/**
 * CheckFFmpegAvailability - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function CheckFFmpegAvailability(): Promise<Record<string, any>> {
    const service = await getDependencyService();
    
    try {
        const response = await service.checkFFmpegAvailability({});
        const status = response.status;
        
        return {
            available: status.available,
            version: status.version,
            path: status.path,
            error: status.error || ""
        };
    } catch (error) {
        console.error('CheckFFmpegAvailability error:', error);
        return {
            available: false,
            version: "",
            path: "",
            error: error instanceof Error ? error.message : 'Unknown error'
        };
    }
}

/**
 * CheckMediaInfoAvailability - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function CheckMediaInfoAvailability(): Promise<Record<string, any>> {
    const service = await getDependencyService();
    
    try {
        const response = await service.checkMediaInfoAvailability({});
        const status = response.status;
        
        return {
            available: status.available,
            version: status.version,
            path: status.path,
            error: status.error || ""
        };
    } catch (error) {
        console.error('CheckMediaInfoAvailability error:', error);
        return {
            available: false,
            version: "",
            path: "",
            error: error instanceof Error ? error.message : 'Unknown error'
        };
    }
}

/**
 * DownloadFFmpeg - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function DownloadFFmpeg(): Promise<string> {
    const service = await getDownloadService();
    
    try {
        const response = await service.downloadFFmpeg({});
        const result = response.result;
        
        if (result.error) {
            throw new Error(result.error);
        }
        
        return result.path;
    } catch (error) {
        console.error('DownloadFFmpeg error:', error);
        throw error;
    }
}

/**
 * DownloadMediaInfo - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function DownloadMediaInfo(): Promise<string> {
    const service = await getDownloadService();
    
    try {
        const response = await service.downloadMediaInfo({});
        const result = response.result;
        
        if (result.error) {
            throw new Error(result.error);
        }
        
        return result.path;
    } catch (error) {
        console.error('DownloadMediaInfo error:', error);
        throw error;
    }
}