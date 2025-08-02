import { MediaService } from '../generated/api.gen';
import { getAPIBaseUrl, interactiveFetch } from '../client';

// Singleton instance
let mediaServiceInstance: MediaService | null = null;

/**
 * Get or create the media service instance
 */
async function getMediaService(): Promise<MediaService> {
    if (!mediaServiceInstance) {
        const baseUrl = await getAPIBaseUrl();
        mediaServiceInstance = new MediaService(baseUrl, interactiveFetch);
    }
    return mediaServiceInstance;
}

/**
 * OpenVideoDialog - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function OpenVideoDialog(): Promise<string> {
    const service = await getMediaService();
    
    try {
        const response = await service.openVideoDialog();
        return response.path;
    } catch (error) {
        console.error('OpenVideoDialog error:', error);
        throw error;
    }
}

/**
 * OpenDirectoryDialog - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function OpenDirectoryDialog(): Promise<string> {
    const service = await getMediaService();
    
    try {
        const response = await service.openDirectoryDialog();
        return response.path;
    } catch (error) {
        console.error('OpenDirectoryDialog error:', error);
        throw error;
    }
}

/**
 * OpenExecutableDialog - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function OpenExecutableDialog(title: string): Promise<string> {
    const service = await getMediaService();
    
    try {
        const response = await service.openExecutableDialog({ title });
        return response.path;
    } catch (error) {
        console.error('OpenExecutableDialog error:', error);
        throw error;
    }
}

/**
 * GetVideosInDirectory - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function GetVideosInDirectory(dirPath: string): Promise<VideoInfo[]> {
    const service = await getMediaService();
    
    try {
        const response = await service.getVideosInDirectory({ dirPath });
        return response.videos;
    } catch (error) {
        console.error('GetVideosInDirectory error:', error);
        throw error;
    }
}

/**
 * CheckMediaLanguageTags - Drop-in replacement for Wails method
 * Maintains exact same signature as the Wails version
 */
export async function CheckMediaLanguageTags(path: string): Promise<MediaLanguageInfo> {
    const service = await getMediaService();
    
    try {
        const response = await service.checkMediaLanguageTags({ path });
        return response.info;
    } catch (error) {
        console.error('CheckMediaLanguageTags error:', error);
        throw error;
    }
}