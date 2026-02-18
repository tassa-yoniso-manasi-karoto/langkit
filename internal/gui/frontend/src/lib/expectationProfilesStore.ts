import { writable } from 'svelte/store';
import type { ExpectationProfile } from '../api/services/expectation';
import {
    ListExpectationProfiles,
    SaveExpectationProfile,
    DeleteExpectationProfile,
} from '../api/services/expectation';
import { logger } from './logger';

function createExpectationProfilesStore() {
    const { subscribe, set } = writable<ExpectationProfile[]>([]);

    let loaded = false;

    return {
        subscribe,

        /** Fetch profiles from backend and populate the store. */
        load: async () => {
            try {
                var profiles = await ListExpectationProfiles();
                set(profiles);
                loaded = true;
                logger.debug('expectationProfiles', 'Loaded profiles', {
                    count: profiles.length
                });
            } catch (e) {
                logger.error('expectationProfiles', 'Failed to load profiles', { error: e });
            }
        },

        /** Ensure profiles are loaded (idempotent). */
        ensureLoaded: async () => {
            if (!loaded) {
                try {
                    var profiles = await ListExpectationProfiles();
                    set(profiles);
                    loaded = true;
                } catch (e) {
                    logger.error('expectationProfiles', 'Failed to ensure profiles loaded', { error: e });
                }
            }
        },

        /** Save (create or update) a profile, then refresh the store. */
        save: async (profile: ExpectationProfile) => {
            await SaveExpectationProfile(profile);
            var profiles = await ListExpectationProfiles();
            set(profiles);
            logger.info('expectationProfiles', 'Profile saved', { name: profile.name });
        },

        /** Delete a profile by name, then refresh the store. */
        remove: async (name: string) => {
            await DeleteExpectationProfile(name);
            var profiles = await ListExpectationProfiles();
            set(profiles);
            logger.info('expectationProfiles', 'Profile deleted', { name: name });
        },
    };
}

export const expectationProfilesStore = createExpectationProfilesStore();
