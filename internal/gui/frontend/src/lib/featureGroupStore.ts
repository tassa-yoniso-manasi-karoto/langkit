// Feature Group Store
// Centralized management of feature groups and their shared options

import { writable, derived, get } from 'svelte/store';
import { errorStore } from './errorStore';
import type { FeatureDefinition } from './featureModel';

// Types for the group system
export interface FeatureGroup {
    id: string;
    label: string;
    description?: string;
    featureIds: string[];
    sharedOptions: string[];
    validationRules?: ValidationRule[];
}

export interface ValidationRule {
    id: string;
    optionId: string;
    validator: (value: any) => boolean;
    errorMessage: string;
    severity: 'critical' | 'warning' | 'info';
}

export interface GroupState {
    groups: Record<string, FeatureGroup>;
    // Options indexed by group ID then option ID - this is the centralized store of option values
    groupOptions: Record<string, Record<string, any>>;
    // Which features are enabled in each group
    enabledFeatures: Record<string, string[]>;
    // Which feature should display group options for each group
    activeDisplayFeature: Record<string, string | null>;
    // Validation state
    validationErrors: Record<string, string[]>;
}

function createFeatureGroupStore() {
    // Initial state
    const initialState: GroupState = {
        groups: {},
        groupOptions: {},
        enabledFeatures: {},
        activeDisplayFeature: {},
        validationErrors: {}
    };

    const store = writable<GroupState>(initialState);

    // Return public API
    return {
        subscribe: store.subscribe,

        /**
         * Register a new feature group
         */
        registerGroup(group: FeatureGroup) {
            // console.log(`Registering group: ${group.id}`, group);
            store.update(state => {
                // Initialize group state
                const newState = { ...state };
                newState.groups[group.id] = group;
                newState.enabledFeatures[group.id] = [];
                newState.activeDisplayFeature[group.id] = null;
                newState.groupOptions[group.id] = {}; 
                newState.validationErrors[group.id] = [];
                
                return newState;
            });
        },

        /**
         * Add a feature to a group
         */
        addFeatureToGroup(groupId: string, featureId: string) {
            store.update(state => {
                if (!state.groups[groupId]) return state;
                
                const newState = { ...state };
                if (!newState.groups[groupId].featureIds.includes(featureId)) {
                    newState.groups[groupId].featureIds.push(featureId);
                }
                
                return newState;
            });
        },

        /**
         * Update a feature's enabled status within a group
         */
        updateFeatureEnabled(groupId: string, featureId: string, enabled: boolean) {
            // console.log(`Updating feature enabled: ${groupId} - ${featureId} - ${enabled}`);
            store.update(state => {
                if (!state.groups[groupId]) return state;
                
                const newState = { ...state };
                const enabledList = [...newState.enabledFeatures[groupId]];
                
                if (enabled && !enabledList.includes(featureId)) {
                    enabledList.push(featureId);
                    console.log(`Added ${featureId} to enabled features in group ${groupId}`);
                } else if (!enabled) {
                    const index = enabledList.indexOf(featureId);
                    if (index !== -1) {
                        enabledList.splice(index, 1);
                        // console.log(`Removed ${featureId} from enabled features in group ${groupId}`);
                    }
                }
                
                newState.enabledFeatures[groupId] = enabledList;
                
                // Update which feature should display group options
                this.updateActiveDisplayFeature(groupId, state.groups[groupId].featureIds, enabledList);
                
                return newState;
            });
            
            // Validate the group after updating
            this.validateGroup(groupId);
        },

        /**
         * Update the active feature that should display group options
         */
        updateActiveDisplayFeature(groupId: string, groupFeatures: string[], enabledFeatures: string[]) {
            console.log(`Updating active display feature for group ${groupId}`, {
                groupFeatures,
                enabledFeatures,
            });
            
            store.update(state => {
                const newState = { ...state };
                const oldActiveFeature = newState.activeDisplayFeature[groupId];
                
                // Get the first enabled feature in order of definition
                const orderedEnabledFeatures = groupFeatures.filter(id => enabledFeatures.includes(id));
                
                console.log(`Group ${groupId} has ${orderedEnabledFeatures.length} enabled features in order`);
                
                if (orderedEnabledFeatures.length > 0) {
                    const newActiveFeature = orderedEnabledFeatures[0];
                    newState.activeDisplayFeature[groupId] = newActiveFeature;
                    
                    console.log(`Setting active display feature for ${groupId} to ${newActiveFeature}`);
                    
                    if (oldActiveFeature !== newActiveFeature) {
                        console.log(`Active display feature for ${groupId} changed from ${oldActiveFeature} to ${newActiveFeature}`);
                    }
                } else {
                    newState.activeDisplayFeature[groupId] = null;
                    console.log(`No active display feature for ${groupId} - no enabled features`);
                }
                
                return newState;
            });
            
            // After updating the state, validate the group to ensure errors are correctly shown
            this.validateGroup(groupId);
        },

        /**
         * Set a group option value - central store of all option values
         */
        setGroupOption(groupId: string, optionId: string, value: any) {
            // console.log(`Setting group option: ${groupId} - ${optionId}`, value);
            store.update(state => {
                if (!state.groups[groupId]) return state;
                
                const newState = { ...state };
                if (!newState.groupOptions[groupId]) {
                    newState.groupOptions[groupId] = {};
                }
                
                newState.groupOptions[groupId][optionId] = value;
                
                return newState;
            });
            
            // Validate the option that was just set
            this.validateOption(groupId, optionId);
        },

        /**
         * Get all options for a group
         */
        getGroupOptions(groupId: string) {
            const state = get(store);
            return state.groupOptions[groupId] || {};
        },

        /**
         * Get a specific option value for a group
         */
        getGroupOption(groupId: string, optionId: string) {
            const state = get(store);
            return state.groupOptions[groupId]?.[optionId];
        },

        /**
         * Validate a specific option within a group
         */
        validateOption(groupId: string, optionId: string) {
            const state = get(store);
            const group = state.groups[groupId];
            
            if (!group) return;
            
            // Find validation rules for this option
            const rules = group.validationRules?.filter(rule => rule.optionId === optionId) || [];
            const optionValue = state.groupOptions[groupId]?.[optionId];
            
            // console.log(`Validating option: ${groupId} - ${optionId}`, {value: optionValue, rules});
            
            // Apply each validation rule
            rules.forEach(rule => {
                // FIXED: Ensure the validator function is called correctly
                const isValid = rule.validator(optionValue);
                // console.log(`Validation result for ${rule.id}: ${isValid}`);
                
                if (!isValid) {
                    // Register error in errorStore
                    errorStore.addError({
                        id: `group-${groupId}-${rule.id}`,
                        message: rule.errorMessage,
                        severity: rule.severity
                    });
                } else {
                    // Remove error if it exists
                    errorStore.removeError(`group-${groupId}-${rule.id}`);
                }
            });
        },

        /**
         * Validate all options in a group
         */
        validateGroup(groupId: string) {
            const state = get(store);
            const group = state.groups[groupId];
            
            if (!group) return;
            
            // console.log(`Validating all options in group: ${groupId}`);
            
            // Get all shared options for this group
            const sharedOptions = group.sharedOptions || [];
            
            // Validate each shared option
            sharedOptions.forEach(optionId => {
                this.validateOption(groupId, optionId);
            });
        },

        /**
         * Get the active feature that should display group options
         */
        getActiveDisplayFeature(groupId: string): string | null {
            const state = get(store);
            return state.activeDisplayFeature[groupId] || null;
        },

        /**
         * Check if a feature is the active display feature for its group
         */
        isActiveDisplayFeature(groupId: string, featureId: string): boolean {
            const state = get(store);
            console.log(
              'isActiveDisplayFeature: groupID', groupId, 'activeDisplayFeature of group', state.activeDisplayFeature[groupId], 'feature currently examined', featureId, 'isActiveDisplay?', state.activeDisplayFeature[groupId] === featureId
            )
            return state.activeDisplayFeature[groupId] === featureId;
        },

        /**
         * Check if a feature is enabled in its group
         */
        isFeatureEnabled(groupId: string, featureId: string): boolean {
            const state = get(store);
            return state.enabledFeatures[groupId]?.includes(featureId) || false;
        },

        /**
         * Get all enabled features in a group
         */
        getEnabledFeatures(groupId: string): string[] {
            const state = get(store);
            return state.enabledFeatures[groupId] || [];
        },

        /**
         * Sync option values from the central store to all features
         * This ensures all features in a group have the same option values
         */
        syncOptionsToFeatures(
            groupId: string,
            currentOptions: Record<string, Record<string, any>>
        ): Record<string, Record<string, any>> {
            const state = get(store);
            const group = state.groups[groupId];
            
            if (!group) return currentOptions;
            
            // Make a copy of the options to avoid mutations
            const newOptions = { ...currentOptions };
            
            // Get the current group options from our central store
            const groupOptions = state.groupOptions[groupId] || {};
            
            // Update all enabled features with the group options
            group.featureIds.forEach(featureId => {
                if (!newOptions[featureId]) {
                    newOptions[featureId] = {};
                }
                
                // Apply each shared option value to the feature
                group.sharedOptions.forEach(optionId => {
                    if (groupOptions[optionId] !== undefined) {
                        newOptions[featureId][optionId] = groupOptions[optionId];
                    }
                });
            });
            
            return newOptions;
        },

        /**
         * Handle browser URL validation with improved checks
         */
        validateBrowserUrl(url: string, needsScraper: boolean, groupId: string) {
            // Clear any existing browser URL errors first to avoid duplicates
            errorStore.removeError(`group-${groupId}-browser-url`);
            errorStore.removeError('invalid-browser-url'); // Legacy error
            
            // Basic URL validation - must be a non-empty string starting with ws://
            const isValidURL = Boolean(url && typeof url === 'string' && url.trim().startsWith('ws://'));
            
            console.log(`Browser URL validation for group ${groupId}:`, {
                url: url,
                needsScraper: needsScraper, 
                isValidURL: isValidURL, 
                type: typeof url,
                isEmpty: !url
            });
            
            // Only validate if we need a scraper
            if (needsScraper) {
                if (!isValidURL) {
                    console.log(`❌ VALIDATION ERROR: Invalid browser URL: "${url}"`);
                    
                    // Register the error in the errorStore with a unique ID for this group
                    errorStore.addError({
                        id: `group-${groupId}-browser-url`,
                        message: 'Valid browser access URL is required for web scraping',
                        severity: 'critical'
                    });
                    return false;
                } else {
                    console.log(`✓ Valid browser URL: "${url}"`);
                    return true;
                }
            }
            
            return true;
        },

        /**
         * Clear all validation errors for a group
         */
        clearGroupErrors(groupId: string) {
            const state = get(store);
            const group = state.groups[groupId];
            
            if (!group || !group.validationRules) return;
            
            // console.log(`Clearing errors for group: ${groupId}`);
            
            // Clear all validation errors
            group.validationRules.forEach(rule => {
                errorStore.removeError(`group-${groupId}-${rule.id}`);
            });
            
            // Also clear browser URL error
            errorStore.removeError(`group-${groupId}-browser-url`);
        }
    };
}

// Create the store
export const featureGroupStore = createFeatureGroupStore();

// Derived store for checking if any feature in a group is enabled
export const groupHasEnabledFeature = (groupId: string) => 
    derived(featureGroupStore, ($store) => {
        return $store.enabledFeatures[groupId]?.length > 0 || false;
    });