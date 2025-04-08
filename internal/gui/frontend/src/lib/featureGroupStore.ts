// Feature Group Store
// Centralized management of feature groups and their shared options

import { writable, derived, get } from 'svelte/store';
import { errorStore } from './errorStore';
import { features, type FeatureDefinition } from './featureModel'; // Import features
import { debounce } from 'lodash'; // Import debounce
import { groupOptionDefinitions, type GroupOptionDefinition } from './groupOptions';
// Import metrics tracking functions
import { trackStoreUpdate, trackSubscription } from './metrics';
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
    // Add state version for tracking changes
    stateVersion: number;
    // Track pending updates by group+option
    pendingUpdates: Record<string, boolean>;
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
        optionGroups: {},
        // Initialize state version
        stateVersion: 0,
        pendingUpdates: {} // Initialize pending updates
    };
const store = writable<GroupState>(initialState);
let batchUpdateTimeout: number | null = null;

    // Add caching for expensive calculations
    const topmostCache = new Map<string, Map<string, boolean>>();

    // Clear cache when features are enabled/disabled
    function clearTopmostCache(groupId?: string) {
      if (groupId) {
        topmostCache.delete(groupId);
      } else {
        topmostCache.clear();
      }
    }

    // Return public API
    const publicApi = { // Assign to variable to allow self-reference in debounced function
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
            // Clear topmost cache for this group before updating state
            clearTopmostCache(groupId);
            
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
            const currentValue = get(store).groupOptions[groupId]?.[optionId];
            
            // Only update if value actually changed
            if (value !== currentValue) {
                // Track the update in metrics before updating the store
                trackStoreUpdate(groupId, optionId, value);
                
                store.update(state => {
                    const newState = { ...state };
                    if (!newState.groupOptions[groupId]) {
                        newState.groupOptions[groupId] = {};
                    }
                    
                    newState.groupOptions[groupId][optionId] = value;
                    
                    // Track that this option was updated (for batching version)
                    if (!newState.pendingUpdates) newState.pendingUpdates = {};
                    newState.pendingUpdates[`${groupId}.${optionId}`] = true;
                    
                    return newState;
                });
                
                // Schedule batched version update
                if (batchUpdateTimeout === null) {
                    batchUpdateTimeout = window.setTimeout(() => {
                        this.batchProcessUpdates(); // Use 'this' to call the method
                        batchUpdateTimeout = null;
                    }, 50);
                }
                
                // Still validate immediately
                this.validateOption(groupId, optionId);
            }
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
            
            console.log(
                `isActiveDisplayFeature check for ${groupId}:`,
                `Feature: ${featureId}`,
                `Active feature: ${activeFeature}`,
                `Is active: ${isActive}`
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
            
            console.log('Initialized canonical feature order:', orderedFeatureIds);
            
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
                console.error(`Group ${groupId} doesn't exist`);
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
            
            console.log(`Updated canonical order for group ${groupId}`);
        },
        
        /**
         * Check if a feature is the topmost in its group based on canonical order
         */
        isTopmostInGroup(groupId: string, featureId: string): boolean {
            // Check if result is cached
            if (topmostCache.has(groupId)) {
              const groupCache = topmostCache.get(groupId);
              if (groupCache && groupCache.has(featureId)) {
                return groupCache.get(featureId) || false;
              }
            }
            
            const state = get(store);
            
            // First check if the feature is enabled
            if (!state.enabledFeatures[groupId]?.includes(featureId)) {
                // Cache negative result
                if (!topmostCache.has(groupId)) {
                  topmostCache.set(groupId, new Map());
                }
                topmostCache.get(groupId)?.set(featureId, false);
                return false;
            }
            
            // Get all enabled features for this group
            const enabledFeatures = state.enabledFeatures[groupId] || [];
            
            // Standard handling for all groups (e.g., subtitle, merge)
            const groupOrder = state.groupCanonicalOrder[groupId] || [];
            let isTopmost = false;
            if (groupOrder.length === 0) {
                console.warn(`No canonical order for group ${groupId}, falling back to feature definition order`);
                
                // Fallback to group feature order if no canonical order is available
                const groupFeatures = state.groups[groupId]?.featureIds || [];
                
                // Find the first enabled feature in group definition order
                const topmostFeature = groupFeatures.find(id => enabledFeatures.includes(id));
                isTopmost = topmostFeature === featureId;
            } else {
                // Find the first enabled feature according to this group's canonical order
                const topmostFeature = groupOrder.find(id => enabledFeatures.includes(id));
                
                // This feature is the topmost if it matches the first enabled feature in canonical order
                isTopmost = topmostFeature === featureId;
            }
            
            // Cache the result
            if (!topmostCache.has(groupId)) {
              topmostCache.set(groupId, new Map());
            }
            topmostCache.get(groupId)?.set(featureId, isTopmost);
            
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
                console.warn(`Option ${optionId} is not registered with any group`);
                return true; // Default to showing if not registered
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
                console.log(`No features in group ${groupId} are enabled, skipping browser URL validation`);
                return true;
            }
            
            // If URL is empty, it's valid as Go-Rod will handle browser automatically
            if (!url || url.trim() === '') {
                console.log(`Empty browser URL, automatic browser management will be used`);
                return true;
            }
            
            // If URL is provided, check if it starts with ws:// (but don't create errors)
            const isValidURL = url.trim().startsWith('ws://');
            
            // Just log the validation result without creating errors
            if (isValidURL) {
                console.log(`✓ Valid browser URL: "${url}" will be used`);
            } else {
                console.log(`Non-standard browser URL: "${url}". If connection fails, automatic browser management will be used.`);
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
        },

        /**
         * Initialize a feature's group options with defaults
         * @param featureId Feature ID
         * @param currentOptions Current options object
         * @returns Updated options with defaults applied
         */
        initializeFeatureGroupOptions(featureId: string, currentOptions: Record<string, any> = {}): Record<string, any> {
          // Find the feature definition using the imported 'features'
          const feature = features.find(f => f.id === featureId);
          if (!feature || !feature.featureGroups) return currentOptions;
          
          // Placeholder implementation removed
          // const feature: FeatureDefinition | undefined = undefined; // Replace with actual lookup
          // if (!feature || !feature.featureGroups) return currentOptions;
          
          // Create a copy of the current options
          const updatedOptions = { ...currentOptions };
          
          // For each group the feature belongs to
          feature.featureGroups.forEach(groupId => {
            // Get the shared options for this group
            const sharedOptions = feature.groupSharedOptions?.[groupId] || [];
            
            // For each shared option
            sharedOptions.forEach(optionId => {
              // Get the option definition
              const optionDef = this.getGroupOptionDefinition(groupId, optionId);
              if (!optionDef) return;
              
              // Only set if not already defined
              if (updatedOptions[optionId] === undefined) {
                updatedOptions[optionId] = optionDef.default;
              }
            });
          });
          
          return updatedOptions;
        },

        /**
         * Get the original definition of a group option
         * @param groupId The group ID
         * @param optionId The option ID
         * @returns The group option definition or null if not found
         */
        getGroupOptionDefinition(groupId: string, optionId: string): GroupOptionDefinition | null {
          return groupOptionDefinitions[groupId]?.[optionId] || null;
        },
        
        /**
         * Check if an option belongs to a specific group
         * @param groupId The group ID to check
         * @param optionId The option ID to check
         * @returns True if the option belongs to the group
         */
        groupHasOption(groupId: string, optionId: string): boolean {
          return Boolean(groupOptionDefinitions[groupId]?.[optionId]);
        },
        
        /**
         * Get all option IDs for a specific group
         * @param groupId The group ID
         * @returns Array of option IDs
         */
        getGroupOptionIds(groupId: string): string[] {
          return Object.keys(groupOptionDefinitions[groupId] || {});
        },
        
        /**
         * Check if an option has additional display conditions beyond isTopmostForOption
         * @param groupId The group ID
         * @param optionId The option ID
         * @returns True if the option has additional display conditions
         */
        hasAdditionalDisplayConditions(groupId: string, optionId: string): boolean {
          return Boolean(groupOptionDefinitions[groupId]?.[optionId]?.conditionalDisplay);
        },
        
        /**
         * Get the additional display condition for an option
         * @param groupId The group ID
         * @param optionId The option ID
         * @returns The additional display condition or null if not found
         */
        getAdditionalDisplayCondition(groupId: string, optionId: string): string | null {
          return groupOptionDefinitions[groupId]?.[optionId]?.conditionalDisplay || null;
        },
        
        /**
         * Check if a group exists in the centralized definitions
         * @param groupId The group ID to check
         * @returns True if the group exists
         */
        isValidGroup(groupId: string): boolean {
          return Boolean(groupOptionDefinitions[groupId]);
        },

        /**
         * Debounced version of syncOptionsToFeatures to reduce frequent updates
         */
        debouncedSyncOptionsToFeatures: debounce(
          function(this: typeof publicApi, groupId: string, currentOptions: Record<string, Record<string, any>>) {
            return this.syncOptionsToFeatures(groupId, currentOptions);
          },
          50,
          { leading: true }
      ),
      
      /**
       * Debounced version update
       */
      batchProcessUpdates() {
          store.update(state => {
              const newState = { ...state };
              newState.stateVersion++;
              newState.pendingUpdates = {};
              return newState;
          });
      },
      
      /**
       * Get the current state version number
       */
      getStateVersion(): number {
          return get(store).stateVersion || 0;
      }
  };

    // Wrap the original subscribe method to track subscriptions
    const originalSubscribe = publicApi.subscribe;
    publicApi.subscribe = (callback: (value: GroupState) => void) => {
        // Track new subscription
        trackSubscription(true);
        
        const unsubscribe = originalSubscribe(callback);
        
        // Return enhanced unsubscribe that tracks removal
        return () => {
            trackSubscription(false);
            unsubscribe();
        };
    };

    return publicApi; // Return the assigned variable
}

// Create the store
export const featureGroupStore = createFeatureGroupStore();

// Derived store for checking if any feature in a group is enabled
export const groupHasEnabledFeature = (groupId: string) =>
    derived(featureGroupStore, ($store) => {
        return $store.enabledFeatures[groupId]?.length > 0 || false;
    });