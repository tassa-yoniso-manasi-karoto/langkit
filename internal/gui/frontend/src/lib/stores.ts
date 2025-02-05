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
};

const defaultSettings: Settings = {
    apiKeys: {
        replicate: '',
        assemblyAI: '',
        elevenLabs: ''
    },
    targetLanguage: '',
    nativeLanguages: 'en, en-US',
    enableGlow: true,
    showLogViewerByDefault: false,
    maxLogEntries: 10000
};

type showSettings = boolean;

export const settings = writable<Settings>(defaultSettings);
export const showSettings = writable(false);