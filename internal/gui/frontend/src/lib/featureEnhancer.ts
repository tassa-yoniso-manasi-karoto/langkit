// src/lib/featureEnhancer.ts
import type { FeatureDefinition, FeatureOption } from './featureModel';
import { groupOptionDefinitions, convertToFeatureOption } from './groupOptions';
import { logStore } from './logStore';

/**
 * Enhances features with their group options from centralized definitions
 * @param features The original feature definitions
 * @returns Enhanced features with group options applied
 */
export function enhanceFeaturesWithGroupOptions(features: FeatureDefinition[]): FeatureDefinition[] {
  return features.map(feature => {
    // Skip features without group membership
    if (!feature.featureGroups || feature.featureGroups.length === 0) {
      return feature;
    }
    
    // Clone the feature to avoid mutation
    const enhancedFeature = { ...feature };
    
    // Initialize required fields if they don't exist
    if (!enhancedFeature.groupSharedOptions) {
      enhancedFeature.groupSharedOptions = {};
    }
    
    if (!enhancedFeature.options) {
      enhancedFeature.options = {};
    }
    
    // Create a new options object to avoid mutation
    const enhancedOptions = { ...enhancedFeature.options };
    
    // For each group this feature belongs to
    feature.featureGroups.forEach(groupId => {
      // Skip if group doesn't exist in centralized definitions
      if (!groupOptionDefinitions[groupId]) {
        logStore.addLog({
          level: 'WARN',
          message: `Feature ${feature.id} belongs to undefined group: ${groupId}`,
          time: new Date().toISOString()
        });
        return;
      }
      
      // Initialize group shared options tracking (ensure groupSharedOptions exists)
      if (enhancedFeature.groupSharedOptions && !enhancedFeature.groupSharedOptions[groupId]) {
        enhancedFeature.groupSharedOptions[groupId] = [];
      }
      
      // Get all options for this group
      const groupOptions = groupOptionDefinitions[groupId];
      
      // Apply each group option
      Object.entries(groupOptions).forEach(([optionId, optionDef]) => {
        // Add option ID to group tracking if not already there (ensure groupSharedOptions exists)
        if (enhancedFeature.groupSharedOptions && !enhancedFeature.groupSharedOptions[groupId]?.includes(optionId)) {
          enhancedFeature.groupSharedOptions[groupId]?.push(optionId); // Use optional chaining for push
        }
        
        // If feature already defines this option, respect its definition
        // This allows features to override group defaults when needed
        if (!enhancedOptions[optionId]) {
          // Convert group option to feature option format
          enhancedOptions[optionId] = convertToFeatureOption(optionDef);
        }
      });
    });
    
    // Apply enhanced options
    enhancedFeature.options = enhancedOptions;
    
    // Ensure optionOrder includes group options
    if (enhancedFeature.optionOrder) {
      const currentOrder = new Set(enhancedFeature.optionOrder);
      const allOptions = Object.keys(enhancedOptions);
      
      // Find options not in current order
      const missingOptions = allOptions.filter(opt => !currentOrder.has(opt));
      
      // Add missing options to the end of the order
      if (missingOptions.length > 0) {
        enhancedFeature.optionOrder = [...enhancedFeature.optionOrder, ...missingOptions];
      }
    }
    
    return enhancedFeature;
  });
}