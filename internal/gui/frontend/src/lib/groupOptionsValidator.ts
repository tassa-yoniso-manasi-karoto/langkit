// src/lib/groupOptionsValidator.ts
import { groupOptionDefinitions } from './groupOptions';
import { features } from './featureModel';

/**
 * Validate that all group options referenced in features are properly defined
 * This is a development-time validation only, to be used during testing
 */
export function validateGroupOptions(): { valid: boolean; errors: string[] } {
  const errors: string[] = [];
  
  // Check each feature that has group memberships
  features.forEach(feature => {
    if (!feature.featureGroups || !feature.groupSharedOptions) return;
    
    // For each group the feature claims membership in
    feature.featureGroups.forEach(groupId => {
      // Check if the group exists in centralized definitions
      if (!groupOptionDefinitions[groupId]) {
        errors.push(`Feature ${feature.id} references undefined group: ${groupId}`);
        return;
      }
      
      // Check each option the feature claims is shared
      const sharedOptions = feature.groupSharedOptions?.[groupId] || []; // Use optional chaining
      sharedOptions.forEach(optionId => {
        // Check if the option exists in the group's centralized definitions
        if (!groupOptionDefinitions[groupId][optionId]) {
          errors.push(`Feature ${feature.id} claims option ${optionId} belongs to group ${groupId}, but it's not defined there`);
        }
      });
    });
  });
  
  return {
    valid: errors.length === 0,
    errors
  };
}

/**
 * Log validation errors to the console
 * This can be called during development to verify group option integrity
 */
export function logGroupOptionValidation(): void {
  const validation = validateGroupOptions();
  
  if (validation.valid) {
    console.log('All group options are valid!');
  } else {
    console.error('Group option validation failed:');
    validation.errors.forEach(error => {
      console.error(`- ${error}`);
    });
  }
}