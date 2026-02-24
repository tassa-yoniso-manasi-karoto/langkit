import { ExpectationService } from '../generated/api.gen';
import type {
    CheckRequest,
    ValidationReport,
    ExpectationProfile,
    AutoCheckConfig,
    InterpretedSummary,
} from '../generated/api.gen';
import { getAPIBaseUrl, createFetch } from '../client';

// Re-export types for convenience
export type {
    CheckRequest,
    ValidationReport,
    ExpectationProfile,
    AutoCheckConfig,
    InterpretedSummary,
};

// Use a longer timeout for RunCheck since it scans many files
const checkFetch = createFetch(120000);

// Singleton instance
let serviceInstance: ExpectationService | null = null;

async function getService(): Promise<ExpectationService> {
    if (!serviceInstance) {
        const baseUrl = await getAPIBaseUrl();
        serviceInstance = new ExpectationService(baseUrl, checkFetch);
    }
    return serviceInstance;
}

/**
 * RunCheck performs the expectation check on a media path.
 */
export async function RunExpectationCheck(request: CheckRequest): Promise<ValidationReport> {
    const service = await getService();
    const response = await service.runCheck({ request });
    return response.report;
}

/**
 * ListExpectationProfiles returns all saved expectation profiles.
 */
export async function ListExpectationProfiles(): Promise<ExpectationProfile[]> {
    const service = await getService();
    const response = await service.listProfiles();
    return response.profiles || [];
}

/**
 * SaveExpectationProfile creates or updates an expectation profile.
 */
export async function SaveExpectationProfile(profile: ExpectationProfile): Promise<void> {
    const service = await getService();
    await service.saveProfile({ profile });
}

/**
 * DeleteExpectationProfile removes a profile by name.
 */
export async function DeleteExpectationProfile(name: string): Promise<void> {
    const service = await getService();
    await service.deleteProfile({ name });
}
