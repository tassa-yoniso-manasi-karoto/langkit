import { writable } from 'svelte/store';

// Developer mode toggle (activated by clicking the bug icon 7 times in the settings panel)
export const isDeveloperMode = writable(false);