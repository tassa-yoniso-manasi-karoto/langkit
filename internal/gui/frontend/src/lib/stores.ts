import { writable } from 'svelte/store';

type Settings = {
    apiKeys: {
        replicate: string;
        assemblyAI: string;
        elevenLabs: string;
    };
    targetLanguage: string;
    nativeLanguage: string;
    enableGlow: boolean;
};

const defaultSettings: Settings = {
    apiKeys: {
        replicate: '',
        assemblyAI: '',
        elevenLabs: ''
    },
    targetLanguage: '',
    nativeLanguage: '',
    enableGlow: true
};

export const settings = writable<Settings>(defaultSettings);