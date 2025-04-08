// src/lib/migrationUtils.ts
import type { FeatureDefinition } from './featureModel';
import { groupOptionDefinitions } from './groupOptions';

/**
 * Extract group option definitions from a feature for migration
 * @param feature The feature to extract group options from
 * @returns Option definitions mapped by group ID
 */
export function extractGroupOptionsFromFeature(feature: FeatureDefinition): Record<string, Record<string, any>> {
  if (!feature.featureGroups || !feature.groupSharedOptions || !feature.options) {
    return {};
  }
  
  const result: Record<string, Record<string, any>> = {};
  
  // For each group the feature belongs to
  feature.featureGroups.forEach(groupId => {
    if (!feature.groupSharedOptions?.[groupId]) return;
    
    result[groupId] = {};
    
    // For each shared option in this group
    feature.groupSharedOptions[groupId].forEach(optionId => {
      // Check if the feature defines this option
      if (feature.options?.[optionId]) {
        result[groupId][optionId] = feature.options[optionId];
      }
    });
  });
  
  return result;
}

/**
 * Generate migration code for group options
 * @param feature The feature to generate migration code for
 * @returns Code string for migrating the feature
 */
export function generateMigrationCode(feature: FeatureDefinition): string {
  if (!feature.featureGroups || !feature.groupSharedOptions || !feature.options) {
    return '// No group options found in this feature';
  }
  
  let code = `// Migration code for feature: ${feature.id}\n\n`;
  code += `// 1. Add to groupOptionDefinitions.ts:\n\n`;
  
  // Extract group options
  const extractedOptions = extractGroupOptionsFromFeature(feature);
  
  // Generate code for each group
  Object.entries(extractedOptions).forEach(([groupId, options]) => {
    code += `${groupId}: {\n`;
    
    Object.entries(options).forEach(([optionId, optionDef]) => {
      const existingDef = groupOptionDefinitions[groupId]?.[optionId];
      
      if (existingDef) {
        code += `  // Option ${optionId} already defined in group ${groupId}\n`;
      } else {
        code += `  ${optionId}: {\n`;
        
        // Convert each property
        Object.entries(optionDef).forEach(([prop, value]) => {
          if (prop === 'showCondition') {
            // Convert showCondition to conditionalDisplay
            const condition = String(value).replace('context.isTopmostForOption && ', '');
            code += `    conditionalDisplay: ${JSON.stringify(condition)},\n`;
          } else {
            // Normal property
            code += `    ${prop}: ${JSON.stringify(value)},\n`;
          }
        });
        
        code += `  },\n`;
      }
    });
    
    code += `},\n\n`;
  });
  
  // Generate feature refactoring code
  code += `// 2. Refactor feature definition:\n\n`;
  code += `{\n`;
  code += `  id: '${feature.id}',\n`;
  code += `  label: '${feature.label}',\n`;
  
  // Options with group options removed
  code += `  options: {\n`;
  Object.entries(feature.options).forEach(([optionId, optionDef]) => {
    // Skip if it's a group option
    let isGroupOption = false;
    for (const groupId of feature.featureGroups || []) {
      if (feature.groupSharedOptions?.[groupId]?.includes(optionId)) {
        isGroupOption = true;
        break;
      }
    }
    
    if (!isGroupOption) {
      code += `    ${optionId}: ${JSON.stringify(optionDef, null, 4).replace(/\n/g, '\n    ')},\n`;
    }
  });
  code += `  },\n`;
  
  // Option order (keep as is)
  if (feature.optionOrder) {
    code += `  optionOrder: [${feature.optionOrder.map(o => `'${o}'`).join(', ')}],\n`;
  }
  
  // Group memberships (keep as is)
  code += `  featureGroups: [${(feature.featureGroups || []).map(g => `'${g}'`).join(', ')}],\n`;
  
  // Other properties (copy as is)
  const excludedProps = ['id', 'label', 'options', 'optionOrder', 'featureGroups', 'groupSharedOptions'];
  Object.entries(feature).forEach(([prop, value]) => {
    if (!excludedProps.includes(prop)) {
      code += `  ${prop}: ${JSON.stringify(value, null, 2).replace(/\n/g, '\n  ')},\n`;
    }
  });
  
  code += `}\n`;
  
  return code;
}