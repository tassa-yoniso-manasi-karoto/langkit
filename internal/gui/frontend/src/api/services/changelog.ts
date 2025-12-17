import { ChangelogService } from '../generated/api.gen';
import type { ChangelogResponse, UpgradeInfo } from '../generated/api.gen';
import { getAPIBaseUrl, defaultFetch } from '../client';

// Singleton instance
let changelogServiceInstance: ChangelogService | null = null;

/**
 * Get or create the changelog service instance
 */
async function getChangelogService(): Promise<ChangelogService> {
    if (!changelogServiceInstance) {
        const baseUrl = await getAPIBaseUrl();
        changelogServiceInstance = new ChangelogService(baseUrl, defaultFetch);
    }
    return changelogServiceInstance;
}

/**
 * GetChangelog - Get changelog entries, optionally filtered by sinceVersion
 */
export async function GetChangelog(sinceVersion?: string): Promise<ChangelogResponse> {
    const service = await getChangelogService();

    try {
        const response = await service.getChangelog({ sinceVersion });
        return response.response;
    } catch (error) {
        console.error('GetChangelog error:', error);
        throw error;
    }
}

/**
 * CheckUpgrade - Check if user has upgraded and should see changelog
 */
export async function CheckUpgrade(): Promise<UpgradeInfo> {
    const service = await getChangelogService();

    try {
        const response = await service.checkUpgrade();
        return response.info;
    } catch (error) {
        console.error('CheckUpgrade error:', error);
        throw error;
    }
}

/**
 * MarkVersionSeen - Mark the current version as seen
 */
export async function MarkVersionSeen(): Promise<void> {
    const service = await getChangelogService();

    try {
        await service.markVersionSeen();
    } catch (error) {
        console.error('MarkVersionSeen error:', error);
        throw error;
    }
}
