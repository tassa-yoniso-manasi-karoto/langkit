// src/lib/featureMixins.ts
import type { FeatureDefinition } from './featureModel';

/**
 * Feature mixin for subtitle processing features
 */
export const subtitleFeatureMixin = {
  featureGroups: ['subtitle', 'merge'],
  requiresLanguage: true,
  requiresDocker: true,
  requiresScraper: true,
  optionOrder: ['style', 'provider', 'dockerRecreate', 'browserAccessURL', 'mergeOutputFiles', 'mergingFormat'],
};

/**
 * Feature mixin for merge-only features
 */
export const mergeFeatureMixin = {
  featureGroups: ['merge'],
  optionOrder: ['mergeOutputFiles', 'mergingFormat'],
};

/**
 * Apply mixins to a feature definition
 * @param feature Base feature definition
 * @param mixins Array of mixins to apply
 * @returns Enhanced feature with mixins applied
 */
export function applyFeatureMixins(feature: FeatureDefinition, ...mixins: Partial<FeatureDefinition>[]): FeatureDefinition {
  let result = { ...feature };
  
  // Apply each mixin
  mixins.forEach(mixin => {
    // Merge simple properties
    result = { ...result, ...mixin };
    
    // Merge arrays (unique values only)
    if (mixin.featureGroups && result.featureGroups) {
      result.featureGroups = Array.from(new Set([...result.featureGroups, ...mixin.featureGroups]));
    }
    
    if (mixin.optionOrder && result.optionOrder) {
      result.optionOrder = Array.from(new Set([...result.optionOrder, ...mixin.optionOrder]));
    }
  });
  
  return result;
}