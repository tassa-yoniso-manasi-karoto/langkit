import { get } from 'svelte/store';
import { systemInfoStore } from './stores';
import { getBaseDebounceValue } from './dev/debugStateControls';

/**
 * Debounce factor definitions
 * These factors are multiplied by the base debounce value
 */
const DEBOUNCE_FACTORS = {
    instant: 0,        // No debounce (0ms) - for immediate actions
    tiny: 0.1,        // Quick UI updates (base * 0.1)
    small: 0.25,      // Responsive actions (base * 0.25)
    medium: 0.5,      // Standard operations (base * 0.5)
    normal: 1.0,      // Default level (base * 1.0)
    large: 2.0,       // Expensive operations (base * 2.0)
    huge: 4.0,        // Very expensive operations (base * 4.0)
} as const;

/**
 * Default base debounce value in milliseconds
 * This can be overridden via the debug dashboard
 */
const DEFAULT_BASE_DEBOUNCE = 200;

/**
 * Get the base debounce value, considering debug overrides
 */
function getBaseDebounce(): number {
    const override = getBaseDebounceValue();
    return override !== undefined ? override : DEFAULT_BASE_DEBOUNCE;
}

/**
 * Get tiny debounce delay (base * 0.1)
 * Use for: Store updates, DOM measurements, progress updates
 */
export function getTinyDebounce(): number {
    return Math.round(getBaseDebounce() * DEBOUNCE_FACTORS.tiny);
}

/**
 * Get small debounce delay (base * 0.25)
 * Use for: UI reactivity, hover effects, topmost feature checks
 */
export function getSmallDebounce(): number {
    return Math.round(getBaseDebounce() * DEBOUNCE_FACTORS.small);
}

/**
 * Get medium debounce delay (base * 0.5)
 * Use for: API calls, validation, search operations
 */
export function getMediumDebounce(): number {
    return Math.round(getBaseDebounce() * DEBOUNCE_FACTORS.medium);
}

/**
 * Get normal debounce delay (base * 1.0)
 * Use for: General operations, default debounce
 */
export function getNormalDebounce(): number {
    return Math.round(getBaseDebounce() * DEBOUNCE_FACTORS.normal);
}

/**
 * Get large debounce delay (base * 2.0)
 * Use for: Expensive operations, file operations
 */
export function getLargeDebounce(): number {
    return Math.round(getBaseDebounce() * DEBOUNCE_FACTORS.large);
}

/**
 * Get huge debounce delay (base * 4.0)
 * Use for: Very expensive operations, batch processing
 */
export function getHugeDebounce(): number {
    return Math.round(getBaseDebounce() * DEBOUNCE_FACTORS.huge);
}

/**
 * Get all debounce values for debugging/display purposes
 */
export function getAllDebounceValues(): Record<string, number> {
    const base = getBaseDebounce();
    return {
        base,
        tiny: getTinyDebounce(),
        small: getSmallDebounce(),
        medium: getMediumDebounce(),
        normal: getNormalDebounce(),
        large: getLargeDebounce(),
        huge: getHugeDebounce(),
    };
}