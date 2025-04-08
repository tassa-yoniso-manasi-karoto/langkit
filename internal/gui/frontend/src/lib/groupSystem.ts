/**
 * @file Feature Group System - Core Concepts and Usage
 * 
 * This file provides documentation and utilities for the Feature Group System.
 * The system enables sharing and synchronizing options across related features
 * while maintaining clean separation of concerns.
 * 
 * Core Concepts:
 * 1. Group Options: Defined centrally in groupOptions.ts, these are shared across features
 * 2. Feature Enhancement: Features are enhanced with group options during initialization
 * 3. Topmost Display: Only the first enabled feature in a group displays shared options
 * 4. Value Synchronization: Changes to one feature propagate to all features in the group
 * 
 * Usage Examples:
 * - Adding a feature to a group: feature.featureGroups = ['subtitle', 'merge']
 * - Defining a group option: see groupOptions.ts
 * - Using a group option: use the GroupOption component
 */

import { features } from './featureModel';
import { featureGroupStore } from './featureGroupStore';
import { groupOptionDefinitions } from './groupOptions';
import { logStore } from './logStore';

/**
 * Explains the group system architecture in a developer-friendly format.
 * Can be called from the browser console for debugging.
 */
export function explainGroupSystem(): void {
  console.group('Feature Group System Architecture');
  
  console.log('The Feature Group System consists of:');
  console.log('1. Centralized group definitions (groupOptions.ts)');
  console.log('2. Feature enhancement during initialization (featureEnhancer.ts)');
  console.log('3. Runtime group management (featureGroupStore.ts)');
  console.log('4. UI components for rendering group options (GroupOption.svelte)');
  
  console.log('\nCurrently defined groups:');
  console.table(Object.keys(groupOptionDefinitions).map(groupId => ({
    groupId,
    optionCount: Object.keys(groupOptionDefinitions[groupId]).length,
    features: features.filter(f => f.featureGroups?.includes(groupId)).map(f => f.id).join(', ')
  })));
  
  console.groupEnd();
}

/**
 * Displays the current group option values for all groups.
 * Useful for debugging and understanding the current state.
 */
export function inspectGroupValues(): void {
  const groupIds = Object.keys(groupOptionDefinitions);
  
  console.group('Current Group Option Values');
  
  groupIds.forEach(groupId => {
    const groupOptions = featureGroupStore.getGroupOptions(groupId);
    console.group(`Group: ${groupId}`);
    
    if (!groupOptions || Object.keys(groupOptions).length === 0) {
      console.log('No options set');
    } else {
      console.table(groupOptions);
    }
    
    // Show enabled features in this group
    const enabledFeatures = featureGroupStore.getEnabledFeatures(groupId);
    console.log(`Enabled features: ${enabledFeatures.join(', ') || 'none'}`);
    
    // Show topmost feature
    const topmostFeature = enabledFeatures.length > 0 
      ? featureGroupStore.getActiveDisplayFeature(groupId)
      : null;
    console.log(`Topmost feature: ${topmostFeature || 'none'}`);
    
    console.groupEnd();
  });
  
  console.groupEnd();
}