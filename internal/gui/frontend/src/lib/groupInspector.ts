// src/lib/groupInspector.ts
import { features } from './featureModel';
import { featureGroupStore } from './featureGroupStore';
import { groupOptionDefinitions } from './groupOptions';

/**
 * Group system inspector for developer tools
 */
export class GroupInspector {
  /**
   * Get a detailed overview of the group system's current state
   * @returns Detailed group system state
   */
  getGroupSystemState() {
    return {
      groups: this.getGroupsInfo(),
      features: this.getFeaturesInfo(),
      options: this.getOptionsInfo(),
      activeFeatures: this.getActiveFeaturesInfo()
    };
  }
  
  /**
   * Get information about all defined groups
   */
  getGroupsInfo() {
    return Object.keys(groupOptionDefinitions).map(groupId => {
      const groupOpts = groupOptionDefinitions[groupId];
      const optionCount = Object.keys(groupOpts).length;
      const enabledFeatures = featureGroupStore.getEnabledFeatures(groupId);
      const activeFeature = featureGroupStore.getActiveDisplayFeature(groupId);
      
      return {
        id: groupId,
        optionCount,
        options: Object.keys(groupOpts),
        featureCount: features.filter(f => f.featureGroups?.includes(groupId)).length,
        enabledFeatures,
        enabledCount: enabledFeatures.length,
        activeFeature
      };
    });
  }
  
  /**
   * Get information about features' group memberships
   */
  getFeaturesInfo() {
    return features.map(feature => {
      const groupCount = feature.featureGroups?.length || 0;
      const isEnabledInAny = feature.featureGroups?.some(gId => 
        featureGroupStore.isFeatureEnabled(gId, feature.id)
      ) || false;
      
      const isTopmostInAny = feature.featureGroups?.some(gId => 
        featureGroupStore.isTopmostInGroup(gId, feature.id)
      ) || false;
      
      return {
        id: feature.id,
        groupCount,
        groups: feature.featureGroups || [],
        enabled: isEnabledInAny,
        isTopmost: isTopmostInAny
      };
    });
  }
  
  /**
   * Get information about option visibility and values
   */
  getOptionsInfo() {
    const result: any[] = [];
    
    // Check each group's options
    Object.entries(groupOptionDefinitions).forEach(([groupId, groupOpts]) => {
      Object.entries(groupOpts).forEach(([optionId, optionDef]) => {
        // Find which features should show this option
        const eligibleFeatures = features.filter(f => 
          f.featureGroups?.includes(groupId) && 
          f.groupSharedOptions?.[groupId]?.includes(optionId)
        );
        
        // Find which feature is actually showing this option
        const visibleInFeature = eligibleFeatures.find(f => 
          featureGroupStore.isFeatureEnabled(groupId, f.id) &&
          featureGroupStore.isTopmostForOption(f.id, optionId)
        )?.id || null;
        
        // Get current value
        const currentValue = featureGroupStore.getGroupOption(groupId, optionId);
        
        result.push({
          groupId,
          optionId,
          type: optionDef.type,
          default: optionDef.default,
          currentValue,
          eligibleFeatureCount: eligibleFeatures.length,
          visibleInFeature,
          hasConditionalDisplay: !!optionDef.conditionalDisplay
        });
      });
    });
    
    return result;
  }
  
  /**
   * Get information about which features are active for displaying options
   */
  getActiveFeaturesInfo() {
    return Object.keys(groupOptionDefinitions).map(groupId => {
      const activeFeature = featureGroupStore.getActiveDisplayFeature(groupId);
      const enabledFeatures = featureGroupStore.getEnabledFeatures(groupId);
      
      // Need to access the store's internal method or state for canonical order
      // This might require exposing getCanonicalOrder or similar from the store
      // For now, returning an empty array as a placeholder
      const canonicalOrder: string[] = (featureGroupStore as any).getCanonicalOrder?.(groupId) || []; 
      
      return {
        groupId,
        activeFeature,
        enabledFeatures,
        activeIsFirst: activeFeature === enabledFeatures[0],
        canonicalOrder 
      };
    });
  }
}

/**
 * Global inspector instance for easy access
 */
export const groupInspector = new GroupInspector();

// For console debugging, expose to window in development
if (import.meta.env.DEV) {
  (window as any).__groupInspector = groupInspector;
  (window as any).__featureGroupStore = featureGroupStore;
  (window as any).__groupOptionDefinitions = groupOptionDefinitions;
}