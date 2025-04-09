// Feature Group Store
// Centralized management of feature groups and their shared options

import { writable, derived, get, type Unsubscriber, type Subscriber, type Updater, type Readable } from 'svelte/store'; // Added missing types
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
    groupOptions: Record<string, Record<string, any>>;
    enabledFeatures: Record<string, string[]>;
    activeDisplayFeature: Record<string, string | null>;
    validationErrors: Record<string, string[]>;
    canonicalOrder: string[];
    groupCanonicalOrder: Record<string, string[]>;
    displayOrder: Record<string, string[]>;
    optionGroups: Record<string, string>; // Maps option IDs to their owning group IDs
    stateVersion: number; // Add version counter
    // pendingUpdates removed from interface
}

// Define the interface for the store's public API
export interface FeatureGroupStore extends Readable<GroupState> {
    registerGroup(group: FeatureGroup): void;
    addFeatureToGroup(groupId: string, featureId: string): void;
    updateFeatureEnabled(groupId: string, featureId: string, enabled: boolean): void;
    setGroupOption(groupId: string, optionId: string, value: any): void;
    batchSetGroupOptions(groupId: string, options: Record<string, any>): void;
    getGroupOptions(groupId: string): Record<string, any>;
    getGroupOption(groupId: string, optionId: string): any;
    validateOption(groupId: string, optionId: string): void;
    validateGroup(groupId: string): void;
    getActiveDisplayFeature(groupId: string): string | null;
    isActiveDisplayFeature(groupId: string, featureId: string): boolean;
    updateFeatureDisplayOrder(groupId: string, orderedFeatureIds: string[]): void;
    updateActiveDisplayFeature(groupId: string, groupFeatures: string[], enabledFeatures: string[]): void; // Added missing export
    getGroups(): Record<string, FeatureGroup>;
    isFeatureEnabled(groupId: string, featureId: string): boolean;
    getEnabledFeatures(groupId: string): string[];
    initializeCanonicalOrder(orderedFeatureIds: string[]): void;
    updateGroupCanonicalOrder(groupId: string): void;
    isTopmostInGroup(groupId: string, featureId: string): boolean;
    registerOptionToGroup(groupId: string, optionId: string): void;
    getGroupForOption(optionId: string): string | null;
    isTopmostForOption(featureId: string, optionId: string): boolean;
    syncOptionsToFeatures(groupId: string, currentOptions: Record<string, Record<string, any>>): Record<string, Record<string, any>>;
    validateBrowserUrl(url: string, needsScraper: boolean, groupId: string): boolean;
    clearGroupErrors(groupId: string): void;
    initializeFeatureGroupOptions(featureId: string, currentOptions?: Record<string, any>): Record<string, any>;
    getGroupOptionDefinition(groupId: string, optionId: string): GroupOptionDefinition | null;
    groupHasOption(groupId: string, optionId: string): boolean;
    getGroupOptionIds(groupId: string): string[];
    hasAdditionalDisplayConditions(groupId: string, optionId: string): boolean;
    getAdditionalDisplayCondition(groupId: string, optionId: string): string | null;
    isValidGroup(groupId: string): boolean;
    debouncedSyncOptionsToFeatures: (groupId: string, currentOptions: Record<string, Record<string, any>>) => Record<string, Record<string, any>>;
    // batchProcessUpdates removed from interface
    getStateVersion(): number;
    createOptionSubscription(groupId: string, optionId: string): Readable<any>; // Add new interface method
}


function createFeatureGroupStore(): FeatureGroupStore {
    // Initial state
    const initialState: GroupState = {
        groups: {},
        groupOptions: {},
        enabledFeatures: {},
        activeDisplayFeature: {},
        validationErrors: {},
        displayOrder: {},
        canonicalOrder: [],
        groupCanonicalOrder: {},
        optionGroups: {}, // Initialize optionGroups
        stateVersion: 0, // Initialize version
        // pendingUpdates fully removed from initialState
    };
    const store = writable<GroupState>(initialState);
    let batchUpdateTimeout: number | null = null;

    const topmostCache = new Map<string, Map<string, boolean>>();

    function clearTopmostCache(groupId?: string) {
      if (groupId) {
        topmostCache.delete(groupId);
      } else {
        topmostCache.clear();
      }
    }

    // --- Internal Helper Functions (Defined BEFORE publicApi) ---

    function updateGroupCanonicalOrder(groupId: string) {
        const state = get(store);
        if (!state.groups[groupId]) {
            console.error(`Group ${groupId} doesn't exist`);
            return;
        }
        store.update(state => {
            const newState = { ...state };
            const groupFeatures = state.groups[groupId].featureIds;
            const filteredOrder = state.canonicalOrder.filter(id => groupFeatures.includes(id));
            newState.groupCanonicalOrder[groupId] = filteredOrder;
            return newState;
        });
        // console.log(`Updated canonical order for group ${groupId}`); // Reduced logging
    }

    function validateOption(groupId: string, optionId: string) {
        const state = get(store);
        const group = state.groups[groupId];
        if (!group) return;
        const enabledFeaturesInGroup = state.enabledFeatures[groupId] || [];
        const anyFeatureEnabled = enabledFeaturesInGroup.length > 0;
        if (!anyFeatureEnabled) {
            if (group.validationRules) {
                group.validationRules
                    .filter(rule => rule.optionId === optionId)
                    .forEach(rule => errorStore.removeError(`group-${groupId}-${rule.id}`));
            }
            return;
        }
        const rules = group.validationRules?.filter(rule => rule.optionId === optionId) || [];
        const optionValue = state.groupOptions[groupId]?.[optionId];
        rules.forEach(rule => {
            const isValid = rule.validator(optionValue);
            if (!isValid) {
                errorStore.addError({ id: `group-${groupId}-${rule.id}`, message: rule.errorMessage, severity: rule.severity });
            } else {
                errorStore.removeError(`group-${groupId}-${rule.id}`);
            }
        });
    }

     function validateGroup(groupId: string) {
        const state = get(store);
        const group = state.groups[groupId];
        if (!group) return;
        const enabledFeaturesInGroup = state.enabledFeatures[groupId] || [];
        const anyFeatureEnabled = enabledFeaturesInGroup.length > 0;
        if (!anyFeatureEnabled) {
            if (group.validationRules) {
                group.validationRules.forEach(rule => errorStore.removeError(`group-${groupId}-${rule.id}`));
            }
            errorStore.removeError(`group-${groupId}-browser-url`);
            return;
        }
        const sharedOptions = group.sharedOptions || [];
        sharedOptions.forEach(optionId => validateOption(groupId, optionId)); // Use the internal validateOption
    }

    function updateActiveDisplayFeature(groupId: string, groupFeatures: string[], enabledFeatures: string[]) {
        const state = get(store);
        let orderToUse = state.groupCanonicalOrder[groupId];
        if (!orderToUse || orderToUse.length === 0) {
            orderToUse = groupFeatures;
        }
        const topmostFeature = orderToUse.find(id => enabledFeatures.includes(id));
        store.update(s => {
            const newState = { ...s };
            const oldActiveFeature = newState.activeDisplayFeature[groupId];
            if (topmostFeature) {
                if (oldActiveFeature !== topmostFeature) {
                    newState.activeDisplayFeature[groupId] = topmostFeature;
                    // console.log(`Active display feature for ${groupId} changed from ${oldActiveFeature} to ${topmostFeature}`); // Reduced logging
                }
            } else {
                newState.activeDisplayFeature[groupId] = null;
                // console.log(`No active display feature for ${groupId} - no enabled features`); // Reduced logging
            }
            return newState;
        });
        validateGroup(groupId); // Use the internal validateGroup
    }
    
    // Process pending updates in batches
    // batchProcessUpdates function definition fully removed

    function setGroupOption(groupId: string, optionId: string, value: any) {
        const currentValue = get(store).groupOptions[groupId]?.[optionId];
        if (value !== currentValue) {
            trackStoreUpdate(groupId, optionId, value);
            store.update(state => {
                const newState = { ...state };
                if (!newState.groupOptions[groupId]) newState.groupOptions[groupId] = {};
                newState.groupOptions[groupId][optionId] = value;
                newState.stateVersion++; // Increment version directly
                // pendingUpdates logic removed
                return newState;
            });
            // batchUpdateTimeout logic removed
            validateOption(groupId, optionId); // Use internal function
        }
    }

    // Correct implementation of batchSetGroupOptions
    function batchSetGroupOptions(groupId: string, options: Record<string, any>) {
        if (!options || Object.keys(options).length === 0) return;

        store.update(state => {
            const newState = { ...state };
            if (!newState.groupOptions[groupId]) newState.groupOptions[groupId] = {};

            let hasChanges = false;
            Object.entries(options).forEach(([optionId, value]) => {
                if (newState.groupOptions[groupId][optionId] !== value) {
                    newState.groupOptions[groupId][optionId] = value;
                    trackStoreUpdate(groupId, optionId, value); // Keep tracking for metrics
                    hasChanges = true;
                }
            });

            if (hasChanges) {
                newState.stateVersion++; // Increment version only if changes occurred
            }
            // pendingUpdates and batchUpdateTimeout logic removed
            return newState;
        });
        Object.keys(options).forEach(optionId => validateOption(groupId, optionId)); // Use internal function
    }

    function getGroupOptionDefinition(groupId: string, optionId: string): GroupOptionDefinition | null {
        return groupOptionDefinitions[groupId]?.[optionId] || null;
    }

    // Implementation of createOptionSubscription
    function createOptionSubscription(groupId: string, optionId: string): Readable<any> {
        return derived(store, ($store) => {
            // Ensure nested properties exist before accessing
            return $store?.groupOptions?.[groupId]?.[optionId];
        });
    }

     function syncOptionsToFeatures(
            groupId: string,
            currentOptions: Record<string, Record<string, any>>
        ): Record<string, Record<string, any>> {
            const state = get(store);
            const group = state.groups[groupId];
            if (!group) return currentOptions;

            // Create new options object (don't mutate existing)
            const newOptions = { ...currentOptions };
            const groupOptions = state.groupOptions[groupId] || {};

            // Update each feature's options with new references
            group.featureIds.forEach(featureId => {
                if (!newOptions[featureId]) {
                    newOptions[featureId] = {};
                } else {
                    // Create new reference for this feature's options
                    newOptions[featureId] = { ...newOptions[featureId] };
                }

                // Apply group options
                group.sharedOptions.forEach(optionId => {
                    if (groupOptions[optionId] !== undefined) {
                        newOptions[featureId][optionId] = groupOptions[optionId];
                    }
                });
            });

            return newOptions;
    }

    function getGroupForOption(optionId: string): string | null {
        const state = get(store);
        return state.optionGroups[optionId] || null;
    }

    function isTopmostInGroup(groupId: string, featureId: string): boolean {
         if (topmostCache.has(groupId)) {
           const groupCache = topmostCache.get(groupId);
           if (groupCache && groupCache.has(featureId)) {
             return groupCache.get(featureId) || false;
           }
         }
         const state = get(store);
         if (!state.enabledFeatures[groupId]?.includes(featureId)) {
             if (!topmostCache.has(groupId)) {
               topmostCache.set(groupId, new Map());
             }
             topmostCache.get(groupId)?.set(featureId, false);
             return false;
         }
         const enabledFeatures = state.enabledFeatures[groupId] || [];
         const groupOrder = state.groupCanonicalOrder[groupId] || [];
         let isTopmost = false;
         if (groupOrder.length === 0) {
             const groupFeatures = state.groups[groupId]?.featureIds || [];
             const topmostFeature = groupFeatures.find(id => enabledFeatures.includes(id));
             isTopmost = topmostFeature === featureId;
         } else {
             const topmostFeature = groupOrder.find(id => enabledFeatures.includes(id));
             isTopmost = topmostFeature === featureId;
         }
         if (!topmostCache.has(groupId)) {
           topmostCache.set(groupId, new Map());
         }
         topmostCache.get(groupId)?.set(featureId, isTopmost);
         return isTopmost;
    }

    // --- Public API ---
    // Define the methods object first
    // Implementation of getStateVersion
    function getStateVersion(): number {
        return get(store).stateVersion || 0;
    }

    // --- Public API ---
    // Define the methods object first
    const publicApiMethods = {
        registerGroup(group: FeatureGroup) {
            store.update(state => {
                const newState = { ...state };
                newState.groups[group.id] = group;
                newState.enabledFeatures[group.id] = [];
                newState.activeDisplayFeature[group.id] = null;
                newState.groupOptions[group.id] = {}; 
                newState.validationErrors[group.id] = [];
                return newState;
            });
            const state = get(store);
            if (state.canonicalOrder.length > 0) {
                updateGroupCanonicalOrder(group.id); // Use internal function
            }
        },

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

        updateFeatureEnabled(groupId: string, featureId: string, enabled: boolean) {
            clearTopmostCache(groupId);
            store.update(state => {
                if (!state.groups[groupId]) return state;
                const newState = { ...state };
                const enabledList = [...newState.enabledFeatures[groupId]];
                if (enabled && !enabledList.includes(featureId)) {
                    enabledList.push(featureId);
                } else if (!enabled) {
                    const index = enabledList.indexOf(featureId);
                    if (index !== -1) {
                        enabledList.splice(index, 1);
                    }
                }
                newState.enabledFeatures[groupId] = enabledList;
                // Defer updateActiveDisplayFeature call to ensure state is updated
                const currentGroupFeatures = newState.groups[groupId].featureIds;
                setTimeout(() => updateActiveDisplayFeature(groupId, currentGroupFeatures, enabledList), 0);
                return newState;
            });
             // Defer validation call
            setTimeout(() => validateGroup(groupId), 0);
        },

        setGroupOption, 
        batchSetGroupOptions, // Expose new method

        getGroupOptions(groupId: string) {
            const state = get(store);
            return state.groupOptions[groupId] || {};
        },

        getGroupOption(groupId: string, optionId: string) {
            const state = get(store);
            return state.groupOptions[groupId]?.[optionId];
        },

        validateOption, 
        validateGroup, 

        getActiveDisplayFeature(groupId: string): string | null {
            const state = get(store);
            return state.activeDisplayFeature[groupId] || null;
        },

        isActiveDisplayFeature(groupId: string, featureId: string): boolean {
            const state = get(store);
            const activeFeature = state.activeDisplayFeature[groupId];
            return activeFeature === featureId;
        },
        
        updateFeatureDisplayOrder(groupId: string, orderedFeatureIds: string[]) {
            // console.log(`Updating display order for group ${groupId} - using canonical order instead`); // Reduced logging
            const state = get(store);
            const enabledFeatures = state.enabledFeatures[groupId] || [];
            const groupFeatures = state.groups[groupId]?.featureIds || [];
            updateActiveDisplayFeature(groupId, groupFeatures, enabledFeatures);
        },

        updateActiveDisplayFeature, // Expose internal function
        
        getGroups(): Record<string, FeatureGroup> {
            const state = get(store);
            return state.groups;
        },

        isFeatureEnabled(groupId: string, featureId: string): boolean {
            const state = get(store);
            return state.enabledFeatures[groupId]?.includes(featureId) || false;
        },

        getEnabledFeatures(groupId: string): string[] {
            const state = get(store);
            return state.enabledFeatures[groupId] || [];
        },
        
        initializeCanonicalOrder(orderedFeatureIds: string[]) {
            store.update(state => {
                const newState = { ...state };
                newState.canonicalOrder = orderedFeatureIds;
                return newState;
            });
            // console.log('Initialized canonical feature order:', orderedFeatureIds); // Reduced logging
            const state = get(store);
            Object.keys(state.groups).forEach(groupId => {
                updateGroupCanonicalOrder(groupId);
            });
        },
        
        updateGroupCanonicalOrder, 

        isTopmostInGroup, // Expose internal function
        
        registerOptionToGroup(groupId: string, optionId: string) {
            store.update(state => {
                const newState = { ...state };
                newState.optionGroups[optionId] = groupId;
                return newState;
            });
        },
        
        getGroupForOption, // Expose internal function
        
        isTopmostForOption(featureId: string, optionId: string): boolean {
            const groupId = getGroupForOption(optionId); // Use internal function
            if (!groupId) {
                console.warn(`Option ${optionId} is not registered with any group`);
                return true; 
            }
            return isTopmostInGroup(groupId, featureId); // Use internal function
        },

        syncOptionsToFeatures, // Expose internal function

        validateBrowserUrl(url: string, needsScraper: boolean, groupId: string) {
            errorStore.removeError(`group-${groupId}-browser-url`);
            errorStore.removeError('invalid-browser-url'); 
            const state = get(store);
            const enabledFeaturesInGroup = state.enabledFeatures[groupId] || [];
            const anyFeatureEnabled = enabledFeaturesInGroup.length > 0;
            if (!anyFeatureEnabled) {
                return true;
            }
            if (!url || url.trim() === '') {
                return true;
            }
            const isValidURL = url.trim().startsWith('ws://');
            // if (isValidURL) { // Reduced logging
            //     console.log(`✓ Valid browser URL: "${url}" will be used`);
            // } else {
            //     console.log(`Non-standard browser URL: "${url}". If connection fails, automatic browser management will be used.`);
            // }
            return true;
        },

        clearGroupErrors(groupId: string) {
            const state = get(store);
            const group = state.groups[groupId];
            if (!group || !group.validationRules) return;
            group.validationRules.forEach(rule => {
                errorStore.removeError(`group-${groupId}-${rule.id}`);
            });
            errorStore.removeError(`group-${groupId}-browser-url`);
        },

        initializeFeatureGroupOptions(featureId: string, currentOptions: Record<string, any> = {}): Record<string, any> {
          const feature = features.find(f => f.id === featureId);
          if (!feature || !feature.featureGroups) return currentOptions;
          const updatedOptions = { ...currentOptions };
          feature.featureGroups.forEach(groupId => {
            const sharedOptions = feature.groupSharedOptions?.[groupId] || [];
            sharedOptions.forEach(optionId => {
              const optionDef = getGroupOptionDefinition(groupId, optionId); // Use internal function
              if (!optionDef) return;
              if (updatedOptions[optionId] === undefined) {
                updatedOptions[optionId] = optionDef.default;
              }
            });
          });
          return updatedOptions;
        },

        getGroupOptionDefinition, // Expose internal function
        
        groupHasOption(groupId: string, optionId: string): boolean {
          return Boolean(groupOptionDefinitions[groupId]?.[optionId]);
        },
        
        getGroupOptionIds(groupId: string): string[] {
          return Object.keys(groupOptionDefinitions[groupId] || {});
        },
        
        hasAdditionalDisplayConditions(groupId: string, optionId: string): boolean {
          return Boolean(groupOptionDefinitions[groupId]?.[optionId]?.conditionalDisplay);
        },
        
        getAdditionalDisplayCondition(groupId: string, optionId: string): string | null {
          return groupOptionDefinitions[groupId]?.[optionId]?.conditionalDisplay || null;
        },
        
        isValidGroup(groupId: string): boolean {
          return Boolean(groupOptionDefinitions[groupId]);
        },

        debouncedSyncOptionsToFeatures: debounce(
          // Correctly reference the internal function, not publicApi
          function(this: any, groupId: string, currentOptions: Record<string, Record<string, any>>) { 
            return syncOptionsToFeatures(groupId, currentOptions); 
          },
          50,
          { leading: true }
        ),
      
        // batchProcessUpdates removed from export
        createOptionSubscription: createOptionSubscription, // Expose new method

        getStateVersion: getStateVersion // Expose correct getStateVersion function
    };

    // Wrap the original subscribe method to track subscriptions and return the correct type
    const enhancedSubscribe = (callback: (value: GroupState) => void): Unsubscriber => {
        trackSubscription(true);
        const unsubscribe = store.subscribe(callback); // Subscribe to the base store
        return () => {
            trackSubscription(false);
            unsubscribe();
        };
    };

    // Return the public API combined with the enhanced subscribe method
    // Use 'as FeatureGroupStore' to ensure the returned object matches the interface
    return {
        ...publicApiMethods, // Spread the methods first
        subscribe: enhancedSubscribe, // Add the enhanced subscribe
    } as FeatureGroupStore; // Cast to the defined interface
}

// Create a typed window augmentation
declare global {
    interface Window {
        // Define minimal structure needed by Dropdown
        featureGroupStore: {
            createOptionSubscription: FeatureGroupStore['createOptionSubscription'];
            getGroupOption: FeatureGroupStore['getGroupOption'];
            getStateVersion: FeatureGroupStore['getStateVersion'];
            // Add other methods if needed by components directly accessing window.featureGroupStore
        };
    }
}

// Create the store instance before exposing it
const storeInstance = createFeatureGroupStore();

// Expose store or necessary methods to window
// Use a check for 'typeof window' to avoid errors during SSR or testing environments
if (typeof window !== 'undefined') {
    // In dev mode, expose the full store for debugging
    if (import.meta.env.DEV) {
        window.featureGroupStore = storeInstance;
    } else {
        // In production, only expose minimal needed methods
        window.featureGroupStore = {
            createOptionSubscription: storeInstance.createOptionSubscription,
            getGroupOption: storeInstance.getGroupOption,
            getStateVersion: storeInstance.getStateVersion
            // Add other minimal exports here if required
        };
    }
}

// Create the store
// Export the created store instance
export const featureGroupStore = storeInstance;

// Derived store for checking if any feature in a group is enabled
// Ensure derived subscribes to the final store object which has the correct subscribe method
export const groupHasEnabledFeature = (groupId: string) => 
    derived(featureGroupStore, ($store) => { 
        // Need to check if $store and $store.enabledFeatures exist before accessing length
        return $store?.enabledFeatures?.[groupId]?.length > 0 || false;
    });