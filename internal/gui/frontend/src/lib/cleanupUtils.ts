// src/lib/cleanupUtils.ts
import { features } from './featureModel';
import { groupOptionDefinitions } from './groupOptions';

/**
 * Find redundant option definitions (options defined in both feature and group)
 * @returns Array of redundancy infos
 */
export function findRedundantOptionDefinitions(): any[] {
  const redundancies: any[] = [];
  
  features.forEach(feature => {
    if (!feature.options || !feature.groupSharedOptions) return;
    
    // Check each option in the feature
    Object.entries(feature.options).forEach(([optionId, optionDef]) => {
      // Check if it's a group option
      for (const groupId of feature.featureGroups || []) {
        const sharedOptions = feature.groupSharedOptions?.[groupId] || []; // Optional chaining
        
        if (sharedOptions.includes(optionId)) {
          // Found a redundancy
          redundancies.push({
            featureId: feature.id,
            groupId,
            optionId,
            featureDefinition: optionDef,
            groupDefinition: groupOptionDefinitions[groupId]?.[optionId]
          });
          
          break;
        }
      }
    });
  });
  
  return redundancies;
}

/**
 * Find inconsistencies between feature and group option definitions
 * @returns Array of inconsistency infos
 */
export function findOptionInconsistencies(): any[] {
  const inconsistencies: any[] = [];
  
  features.forEach(feature => {
    if (!feature.options || !feature.groupSharedOptions) return;
    
    // Check each option in the feature
    Object.entries(feature.options).forEach(([optionId, optionDef]) => {
      // Check if it's a group option
      for (const groupId of feature.featureGroups || []) {
        const sharedOptions = feature.groupSharedOptions?.[groupId] || []; // Optional chaining
        
        if (sharedOptions.includes(optionId)) {
          // Get the group definition
          const groupDef = groupOptionDefinitions[groupId]?.[optionId];
          if (!groupDef) continue;
          
          // Compare properties
          const differences: string[] = [];
          
          // Common properties to check
          const commonProps = ['type', 'label', 'default', 'min', 'max', 'step', 'choices', 'placeholder'];
          
          commonProps.forEach(prop => {
            if (prop in optionDef && prop in groupDef && 
                JSON.stringify((optionDef as any)[prop]) !== JSON.stringify((groupDef as any)[prop])) { // Use any for dynamic access
              differences.push(prop);
            }
          });
          
          // Check hovertip separately (might be different formatting but same content)
          if ('hovertip' in optionDef && 'hovertip' in groupDef) {
            const featureHovertip = optionDef.hovertip?.replace(/\s+/g, ' ').trim();
            const groupHovertip = groupDef.hovertip?.replace(/\s+/g, ' ').trim();
            
            if (featureHovertip !== groupHovertip) {
              differences.push('hovertip');
            }
          }
          
          // Special handling for showCondition vs conditionalDisplay
          if ('showCondition' in optionDef && 'conditionalDisplay' in groupDef) {
            const featureCond = optionDef.showCondition?.replace('context.isTopmostForOption && ', '');
            const groupCond = groupDef.conditionalDisplay;
            
            if (featureCond !== groupCond) {
              differences.push('condition');
            }
          }
          
          if (differences.length > 0) {
            inconsistencies.push({
              featureId: feature.id,
              groupId,
              optionId,
              differences
            });
          }
          
          break;
        }
      }
    });
  });
  
  return inconsistencies;
}