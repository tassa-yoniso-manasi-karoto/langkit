// Feature Group Store
// Centralized management of feature groups and their shared options

import { writable, derived, get } from 'svelte/store';
import { invalidationErrorStore } from './invalidationErrorStore';
import type { FeatureDefinition, RomanizationScheme } from './featureModel';
import { logger } from './logger';

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
    // Canonical ordering of features as defined in the model
    canonicalOrder: string[];
    // Derived canonical order for each group
    groupCanonicalOrder: Record<string, string[]>;
    // Legacy field kept for backward compatibility, not used anymore
    displayOrder: Record<string, string[]>;
    // Maps option IDs to their owning group IDs
    optionGroups: Record<string, string>;
}

function createFeatureGroupStore() {
    // Initial state
    const initialState: GroupState = {
        groups: {},
        groupOptions: {},
        enabledFeatures: {},
        activeDisplayFeature: {},
        validationErrors: {},
        displayOrder: {},
        // New fields for canonical feature ordering
        canonicalOrder: [],
        groupCanonicalOrder: {},
        // Option to group mapping
        optionGroups: {}
    };

    const store = writable<GroupState>(initialState);

    // Return public API
    return {
        subscribe: store.subscribe,

        /**
         * Register a new feature group
         */
        registerGroup(group: FeatureGroup) {
            logger.trace('store/featureGroupStore', 'Registering group', { groupId: group.id, group });
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
            
            // If we already have a canonical order, update the group's canonical order
            const state = get(store);
            if (state.canonicalOrder.length > 0) {
                this.updateGroupCanonicalOrder(group.id);
            }
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
            logger.trace('store/featureGroupStore', 'Updating feature enabled state', { groupId, featureId, enabled });
            store.update(state => {
                if (!state.groups[groupId]) return state;
                
                const newState = { ...state };
                const enabledList = [...newState.enabledFeatures[groupId]];
                
                if (enabled && !enabledList.includes(featureId)) {
                    enabledList.push(featureId);
                    logger.trace('store/featureGroupStore', 'Added feature to enabled features', { featureId, groupId });
                } else if (!enabled) {
                    const index = enabledList.indexOf(featureId);
                    if (index !== -1) {
                        enabledList.splice(index, 1);
                        logger.trace('store/featureGroupStore', 'Removed feature from enabled features', { featureId, groupId });
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
         * Always uses the canonical order to determine topmost feature
         */
        updateActiveDisplayFeature(groupId: string, groupFeatures: string[], enabledFeatures: string[]) {
            logger.trace('store/featureGroupStore', 'Updating active display feature', { groupId });
            
            // Get the topmost enabled feature using canonical order
            const state = get(store);
            
            // Get canonical order for this group
            let orderToUse = state.groupCanonicalOrder[groupId];
            if (!orderToUse || orderToUse.length === 0) {
                // Fall back to group features order if canonical order not available
                orderToUse = groupFeatures;
            }
            
            // Find the first enabled feature according to canonical order
            const topmostFeature = orderToUse.find(id => enabledFeatures.includes(id));
            
            store.update(state => {
                const newState = { ...state };
                const oldActiveFeature = newState.activeDisplayFeature[groupId];
                
                if (topmostFeature) {
                    if (oldActiveFeature !== topmostFeature) {
                        newState.activeDisplayFeature[groupId] = topmostFeature;
                        logger.trace('store/featureGroupStore', 'Active display feature changed', { groupId, oldActiveFeature, topmostFeature });
                    }
                } else {
                    newState.activeDisplayFeature[groupId] = null;
                    logger.trace('store/featureGroupStore', 'No active display feature - no enabled features', { groupId });
                }
                
                return newState;
            });
            
            // Validate the group after updating
            this.validateGroup(groupId);
        },

        /**
         * Set a group option value - central store of all option values
         */
        setGroupOption(groupId: string, optionId: string, value: any) {
            logger.trace('store/featureGroupStore', 'Setting group option', { groupId, optionId, value });
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
            
            // Check if any features in the group are enabled
            const enabledFeaturesInGroup = state.enabledFeatures[groupId] || [];
            const anyFeatureEnabled = enabledFeaturesInGroup.length > 0;
            
            // Skip validation if no features in the group are enabled
            if (!anyFeatureEnabled) {
                // Remove any existing errors
                if (group.validationRules) {
                    group.validationRules
                        .filter(rule => rule.optionId === optionId)
                        .forEach(rule => {
                            invalidationErrorStore.removeError(`group-${groupId}-${rule.id}`);
                        });
                }
                return;
            }
            
            // Find validation rules for this option
            const rules = group.validationRules?.filter(rule => rule.optionId === optionId) || [];
            const optionValue = state.groupOptions[groupId]?.[optionId];
            
            logger.trace('store/featureGroupStore', 'Validating option', { groupId, optionId, value: optionValue, ruleCount: rules.length });
            
            // Apply each validation rule
            rules.forEach(rule => {
                // FIXED: Ensure the validator function is called correctly
                const isValid = rule.validator(optionValue);
                logger.trace('store/featureGroupStore', 'Validation result', { ruleId: rule.id, isValid });
                
                if (!isValid) {
                    // Register error in invalidationErrorStore
                    invalidationErrorStore.addError({
                        id: `group-${groupId}-${rule.id}`,
                        message: rule.errorMessage,
                        severity: rule.severity
                    });
                } else {
                    // Remove error if it exists
                    invalidationErrorStore.removeError(`group-${groupId}-${rule.id}`);
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
            
            // Check if any features in the group are enabled
            const enabledFeaturesInGroup = state.enabledFeatures[groupId] || [];
            const anyFeatureEnabled = enabledFeaturesInGroup.length > 0;
            
            logger.trace('store/featureGroupStore', 'Validating all options in group', { groupId, featuresEnabled: anyFeatureEnabled });
            
            // If no features are enabled, clear all validation errors for this group
            if (!anyFeatureEnabled) {
                if (group.validationRules) {
                    group.validationRules.forEach(rule => {
                        invalidationErrorStore.removeError(`group-${groupId}-${rule.id}`);
                    });
                }
                // Also clear the browser URL error
                invalidationErrorStore.removeError(`group-${groupId}-browser-url`);
                return;
            }
            
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
            
            // Get the active display feature for this group
            const activeFeature = state.activeDisplayFeature[groupId];
            
            // Check if this feature is the active one
            const isActive = activeFeature === featureId;
            
            logger.trace('store/featureGroupStore', 
                `isActiveDisplayFeature check for ${groupId}: Feature: ${featureId}, Active feature: ${activeFeature}, Is active: ${isActive}`
            );
            
            return isActive;
        },
        
        /**
         * Update the display order of features for a group
         * This method is kept for backward compatibility but now delegates to canonical order
         */
        updateFeatureDisplayOrder(groupId: string, orderedFeatureIds: string[]) {
            logger.trace('store/featureGroupStore', 'Updating display order - using canonical order', { groupId });
            
            // This method no longer updates the displayOrder but reuses the
            // canonical order for consistency
            
            // Update the active display feature based on current canonical order
            const state = get(store);
            const enabledFeatures = state.enabledFeatures[groupId] || [];
            const groupFeatures = state.groups[groupId]?.featureIds || [];
            
            this.updateActiveDisplayFeature(groupId, groupFeatures, enabledFeatures);
        },
        
        // Removed isTopmostDisplayedFeature - obsolete method has been replaced by isTopmostInGroup
        
        /**
         * Get all registered groups
         */
        getGroups(): Record<string, FeatureGroup> {
            const state = get(store);
            return state.groups;
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
         * Initialize the canonical ordering of features based on the feature model
         */
        initializeCanonicalOrder(orderedFeatureIds: string[]) {
            store.update(state => {
                const newState = { ...state };
                newState.canonicalOrder = orderedFeatureIds;
                return newState;
            });
            
            logger.trace('store/featureGroupStore', 'Initialized canonical feature order', { orderedFeatureIds });
            
            // Update canonical order for all groups
            const state = get(store);
            Object.keys(state.groups).forEach(groupId => {
                this.updateGroupCanonicalOrder(groupId);
            });
        },
        
        /**
         * Update the canonical order for a specific group
         */
        updateGroupCanonicalOrder(groupId: string) {
            const state = get(store);
            
            if (!state.groups[groupId]) {
                logger.error('store/featureGroupStore', `Group ${groupId} doesn't exist`);
                return;
            }
            
            store.update(state => {
                const newState = { ...state };
                
                // Filter the canonical order to only include features in this group
                const groupFeatures = state.groups[groupId].featureIds;
                const filteredOrder = state.canonicalOrder.filter(id => 
                    groupFeatures.includes(id)
                );
                
                newState.groupCanonicalOrder[groupId] = filteredOrder;
                return newState;
            });
            
            logger.trace('store/featureGroupStore', `Updated canonical order for group ${groupId}`);
        },
        
        /**
         * Check if a feature is the topmost in its group (the active display feature)
         */
        isTopmostInGroup(groupId: string, featureId: string): boolean {
            const state = get(store);
            
            // The topmost feature is the one marked as activeDisplayFeature for this group
            return state.activeDisplayFeature[groupId] === featureId;
        },
        
        /**
         * Register an option as belonging to a specific group
         */
        registerOptionToGroup(groupId: string, optionId: string) {
            store.update(state => {
                const newState = { ...state };
                newState.optionGroups[optionId] = groupId;
                return newState;
            });
        },
        
        /**
         * Get the group that an option belongs to
         */
        getGroupForOption(optionId: string): string | null {
            const state = get(store);
            return state.optionGroups[optionId] || null;
        },
        
        /**
         * Check if a feature is the topmost for a specific option
         * This is the key method that enables proper handling of options
         * that belong to different groups
         */
        isTopmostForOption(featureId: string, optionId: string): boolean {
            // Get the group this option belongs to
            const groupId = this.getGroupForOption(optionId);
            if (!groupId) {
                return true;
            }
            
            // Check if this feature is the topmost in the option's group
            return this.isTopmostInGroup(groupId, featureId);
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
         * Handle browser URL validation
         */
        validateBrowserUrl(url: string, needsScraper: boolean, groupId: string) {
            // Always clear any existing browser URL errors
            invalidationErrorStore.removeError(`group-${groupId}-browser-url`);
            invalidationErrorStore.removeError('invalid-browser-url'); // Legacy error
            
            // Check if any subtitle features are actually enabled/selected
            const state = get(store);
            const enabledFeaturesInGroup = state.enabledFeatures[groupId] || [];
            const anyFeatureEnabled = enabledFeaturesInGroup.length > 0;
            
            // If no subtitle features are enabled, skip validation entirely
            if (!anyFeatureEnabled) {
                logger.trace('store/featureGroupStore', `No features in group ${groupId} are enabled, skipping browser URL validation`);
                return true;
            }
            
            // If URL is empty, it's valid as Go-Rod will handle browser automatically
            if (!url || url.trim() === '') {
                logger.trace('store/featureGroupStore', `Empty browser URL, automatic browser management will be used`);
                return true;
            }
            
            // If URL is provided, check if it starts with ws:// (but don't create errors)
            const isValidURL = url.trim().startsWith('ws://');
            
            // Just log the validation result without creating errors
            if (isValidURL) {
                logger.trace('store/featureGroupStore', `✓ Valid browser URL: "${url}" will be used`);
            } else {
                logger.trace('store/featureGroupStore', `Non-standard browser URL: "${url}". If connection fails, automatic browser management will be used.`);
            }
            
            // Always return true - never block the process button
            return true;
        },

        /**
         * Clear all validation errors for a group
         */
        clearGroupErrors(groupId: string) {
            const state = get(store);
            const group = state.groups[groupId];
            
            if (!group || !group.validationRules) return;
            
            logger.trace('store/featureGroupStore', 'Clearing errors for group', { groupId });
            
            // Clear all validation errors
            group.validationRules.forEach(rule => {
                invalidationErrorStore.removeError(`group-${groupId}-${rule.id}`);
            });
            
            // Also clear browser URL error
            invalidationErrorStore.removeError(`group-${groupId}-browser-url`);
        }
    };
}

// Create stores for romanization data
export const romanizationSchemesStore = writable<RomanizationScheme[]>([]);
export const languageRequirementsStore = writable<{
    needsScraper: boolean;
    needsDocker: boolean;
}>({
    needsScraper: false,
    needsDocker: false
});

// Create the main feature group store
const baseFeatureGroupStore = createFeatureGroupStore();

// Create derived stores for reactive state
export const currentSchemeStore = derived(
    [baseFeatureGroupStore, romanizationSchemesStore],
    ([$store, $schemes]) => {
        // Get the currently selected style from the subtitle group
        const selectedStyle = $store.groupOptions['subtitle']?.['style'];
        if (!selectedStyle || $schemes.length === 0) {
            return null;
        }
        
        // Find the matching scheme
        return $schemes.find(s => s.name === selectedStyle) || null;
    }
);

// Derived store for whether scraper is needed (language-level, kept for backward compatibility)
export const needsScraperStore = derived(
    languageRequirementsStore,
    ($requirements) => $requirements.needsScraper
);

// Derived store for whether docker is needed (language-level, kept for backward compatibility)
export const needsDockerStore = derived(
    languageRequirementsStore,
    ($requirements) => $requirements.needsDocker
);

// Scheme-specific derived stores
export const currentSchemeNeedsScraperStore = derived(
    currentSchemeStore,
    ($scheme) => $scheme?.needsScraper || false
);

export const currentSchemeNeedsDockerStore = derived(
    currentSchemeStore,
    ($scheme) => $scheme?.needsDocker || false
);

// Export the store with additional derived stores
export const featureGroupStore = {
    ...baseFeatureGroupStore,
    // Add references to the derived stores
    needsScraper: needsScraperStore,
    needsDocker: needsDockerStore,
    currentScheme: currentSchemeStore,
    currentSchemeNeedsScraper: currentSchemeNeedsScraperStore,
    currentSchemeNeedsDocker: currentSchemeNeedsDockerStore
};

// Derived store for checking if any feature in a group is enabled
export const groupHasEnabledFeature = (groupId: string) => 
    derived(baseFeatureGroupStore, ($store) => {
        return $store.enabledFeatures[groupId]?.length > 0 || false;
    });