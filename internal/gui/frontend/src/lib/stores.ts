import { writable } from 'svelte/store';

type Settings = {
    apiKeys: {
        replicate: string;
        assemblyAI: string;
        elevenLabs: string;
    };
    targetLanguage: string;
    nativeLanguages: string;
    enableGlow: boolean;
    showLogViewerByDefault: boolean;
    maxLogEntries: number;
    maxAPIRetries: number;
    maxWorkers: number;
};

/* these values are irrelevant, only the default values of the backend matter */
const initSettings: Settings = {
    apiKeys: {
        replicate: '',
        assemblyAI: '',
        elevenLabs: ''
    },
    targetLanguage: '',
    nativeLanguages: 'en, en-US',
    enableGlow: true,
    showLogViewerByDefault: false,
    maxLogEntries: 10000,
    maxAPIRetries: 10,
    maxWorkers: 1,
};

type showSettings = boolean;

export const settings = writable<Settings>(initSettings);
export const showSettings = writable(false);