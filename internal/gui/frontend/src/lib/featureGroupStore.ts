// Feature Group Store
// Centralized management of feature groups and their shared options

import { writable, derived, get } from 'svelte/store';
import { errorStore } from './errorStore';
import type { FeatureDefinition } from './featureModel';
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
         * Always uses the canonical order to determine topmost feature
         */
        updateActiveDisplayFeature(groupId: string, groupFeatures: string[], enabledFeatures: string[]) {
            console.log(`Updating active display feature for group ${groupId}`);
            
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
                        console.log(`Active display feature for ${groupId} changed from ${oldActiveFeature} to ${topmostFeature}`);
                    }
                } else {
                    newState.activeDisplayFeature[groupId] = null;
                    console.log(`No active display feature for ${groupId} - no enabled features`);
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
                            errorStore.removeError(`group-${groupId}-${rule.id}`);
                        });
                }
                return;
            }
            
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
            
            // Check if any features in the group are enabled
            const enabledFeaturesInGroup = state.enabledFeatures[groupId] || [];
            const anyFeatureEnabled = enabledFeaturesInGroup.length > 0;
            
            // console.log(`Validating all options in group: ${groupId}, features enabled: ${anyFeatureEnabled}`);
            
            // If no features are enabled, clear all validation errors for this group
            if (!anyFeatureEnabled) {
                if (group.validationRules) {
                    group.validationRules.forEach(rule => {
                        errorStore.removeError(`group-${groupId}-${rule.id}`);
                    });
                }
                // Also clear the browser URL error
                errorStore.removeError(`group-${groupId}-browser-url`);
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
            console.log(`Updating display order for group ${groupId} - using canonical order instead`);
            
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
         * Check if a feature is the topmost in its group based on canonical order
         */
        isTopmostInGroup(groupId: string, featureId: string): boolean {
            const state = get(store);
            
            // First check if the feature is enabled
            if (!state.enabledFeatures[groupId]?.includes(featureId)) {
                return false;
            }
            
            // Get all enabled features for this group
            const enabledFeatures = state.enabledFeatures[groupId] || [];
            
            // Standard handling for all groups (e.g., subtitle, merge)
            const groupOrder = state.groupCanonicalOrder[groupId] || [];
            if (groupOrder.length === 0) {
                logger.warn('store/featureGroupStore', `No canonical order for group ${groupId}, falling back to feature definition order`);
                
                // Fallback to group feature order if no canonical order is available
                const groupFeatures = state.groups[groupId]?.featureIds || [];
                
                // Find the first enabled feature in group definition order
                const topmostFeature = groupFeatures.find(id => enabledFeatures.includes(id));
                return topmostFeature === featureId;
            }
            
            // Find the first enabled feature according to this group's canonical order
            const topmostFeature = groupOrder.find(id => enabledFeatures.includes(id));
            
            // This feature is the topmost if it matches the first enabled feature in canonical order
            const isTopmost = topmostFeature === featureId;
            
            /*console.log(`isTopmostInGroup check for ${groupId}:`, {
                featureId,
                enabledFeatures,
                groupOrder,
                topmostFeature,
                isTopmost
            });*/ // TODO this log is spammed and causes memory leaks; restore it with throttling
            
            return isTopmost;
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
            errorStore.removeError(`group-${groupId}-browser-url`);
            errorStore.removeError('invalid-browser-url'); // Legacy error
            
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