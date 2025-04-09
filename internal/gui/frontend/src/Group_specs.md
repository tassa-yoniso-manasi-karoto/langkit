# Feature Group System Architecture

## Overview

The Feature Group system allows multiple features to share common options and settings. This provides a consistent user experience while reducing redundancy in the UI. When multiple features belong to the same group (e.g., subtitle processing features), certain options (like browser URL, provider, etc.) are shared across all features in the group.

## System Requirements

1. **Single Instance Rule**: Group-shared options must appear only once in the UI, even when multiple features from the same group are enabled
2. **Topmost Display**: Shared options should be displayed in the topmost (first) feature of the group
3. **Consistent Values**: All features must use the same values for shared options
4. **Value Synchronization**: Changes to a shared option in one feature must propagate to all features in the group
5. **Reliable Order Determination**: The system must reliably determine which feature should show the options
6. **Multi-Group Support**: Features can belong to multiple groups, with proper handling of options from each group
7. **Validation**: The system validates configuration integrity to prevent common errors
8. **Performance Optimization**: Optimized for minimal re-renders and efficient state management

## File Structure and Organization

The feature group system is organized across several specialized files:

| File | Purpose |
|------|---------|
| `featureGroupStore.ts` | Central store that manages group state, membership, and coordination |
| `groupOptions.ts` | Single source of truth for all group option definitions |
| `GroupOption.svelte` | Component for rendering and managing shared group options with authority-based synchronization |
| `featureEnhancer.ts` | Enhances feature definitions with their group options during initialization |
| `groupValidation.ts` | Comprehensive validation system for group configuration |
| `featureGroupErrorHandling.ts` | Error handling utilities with safe operations and fallbacks |
| `groupInspector.ts` | Debugging tools for inspecting group system state |
| `groupSystem.ts` | Core concepts documentation and utilities |
| `featureMixins.ts` | Reusable feature configurations as composable mixins |
| `groupOptionsValidator.ts` | Validates option definitions and relationships |
| `migrationUtils.ts` | Utilities for migrating from old feature-based to centralized option definitions |
| `cleanupUtils.ts` | Tools for identifying and resolving redundant or inconsistent option definitions |
| `GroupDebugPanel.svelte` | Debug UI component for real-time group system inspection |

## Key Components

### 1. Feature Group Store (`featureGroupStore.ts`)

The central store manages all group-related state, including:

- Group definitions and membership
- Feature canonical ordering
- Shared option values
- Feature enabled status tracking
- Topmost feature determination
- Value validation
- Option-to-group mapping
- Performance metrics and optimization

Key methods include:
```typescript
// Initialize canonical feature order
initializeCanonicalOrder(orderedFeatureIds: string[]): void

// Determine if a feature is the topmost in its group
isTopmostInGroup(groupId: string, featureId: string): boolean

// Associate an option with a specific group
registerOptionToGroup(groupId: string, optionId: string): void

// Get the group an option belongs to
getGroupForOption(optionId: string): string | null

// Check if a feature should display a specific option
isTopmostForOption(featureId: string, optionId: string): boolean

// Synchronize option values across all features in a group
syncOptionsToFeatures(groupId: string, currentOptions: Record<string, Record<string, any>>): Record<string, Record<string, any>>

// Create a derived store for a specific option
createOptionSubscription(groupId: string, optionId: string): Readable<any>
```

### 2. Centralized Option Definitions (`groupOptions.ts`)

All group-shared options are defined in a central location:

```typescript
export const groupOptionDefinitions: Record<string, Record<string, GroupOptionDefinition>> = {
  // Subtitle Group Options
  subtitle: {
    style: {
      type: 'romanizationDropdown',
      label: 'Romanization Style',
      default: '',
      conditionalDisplay: "context.romanizationSchemes.length > 0"
    },
    provider: {
      type: 'provider',
      label: 'Provider',
      default: '',
      conditionalDisplay: "context.romanizationSchemes.length > 0"
    },
    // More options...
  },
  // Merge Group Options
  merge: {
    mergeOutputFiles: {
      type: 'boolean',
      label: 'Merge all processed outputs',
      default: false,
      hovertip: "When enabled, all processed outputs will be merged into a single video file."
    },
    // More options...
  }
};
```

### 3. Group Option Component (`GroupOption.svelte`)

A specialized component for rendering and managing group-shared options with:

- Authority-based value synchronization
- User input priority handling 
- Validation integration
- Input recovery for invalid values
- Automatic relationship management (e.g., style → provider)
- Performance optimization with debounced updates

### 4. Feature Enhancement (`featureEnhancer.ts`)

Features are enhanced with their group options during initialization:

```typescript
export function enhanceFeaturesWithGroupOptions(features: FeatureDefinition[]): FeatureDefinition[] {
  return features.map(feature => {
    // Skip features without group membership
    if (!feature.featureGroups || feature.featureGroups.length === 0) {
      return feature;
    }
    
    // Clone and enhance the feature with group options
    const enhancedFeature = { ...feature };
    
    // Apply group options from centralized definitions
    feature.featureGroups.forEach(groupId => {
      const groupOptions = groupOptionDefinitions[groupId];
      // Apply options to feature...
    });
    
    return enhancedFeature;
  });
}
```

### 5. Validation System (`groupValidation.ts`)

Comprehensive validation ensures system integrity:

```typescript
export class GroupSystemValidator {
  validateSystem(silent: boolean = false): ValidationError[] {
    this.errors = [];
    
    // Run all validation checks
    this.validateFeatureGroups();
    this.validateGroupOptions();
    this.validateOptionTypes();
    this.validateOptionOrder();
    this.validateRedundantOptions();
    
    // Return validation results
    return this.errors;
  }
  
  // Individual validation methods...
}
```

## Implementation Approach

### Centralized Option Definitions

Options are now defined centrally rather than in individual features:

```typescript
// OLD: Feature-based definitions
{
  id: 'subtitleRomanization',
  options: {
    style: {
      type: 'romanizationDropdown',
      label: 'Romanization Style',
      default: '',
      showCondition: "context.isTopmostInGroup && context.romanizationSchemes.length > 0"
    },
    // More options...
  }
}

// NEW: Centralized definitions with feature enhancement
// In groupOptions.ts:
subtitle: {
  style: {
    type: 'romanizationDropdown',
    label: 'Romanization Style',
    default: '',
    conditionalDisplay: "context.romanizationSchemes.length > 0"
  },
  // More options...
}

// In feature definition:
{
  id: 'subtitleRomanization',
  featureGroups: ['subtitle', 'merge'],
  // Options are applied automatically during enhancement
}
```

### Option-Specific Context Awareness

The system uses option-specific context to determine visibility:

```typescript
function shouldShowOption(optionId: string, optionDef: any): boolean {
  // Find which group this option belongs to
  let optionGroup = null;
  if (feature.groupSharedOptions) {
    for (const [groupId, options] of Object.entries(feature.groupSharedOptions)) {
      if (options.includes(optionId)) {
        optionGroup = groupId;
        featureGroupStore.registerOptionToGroup(groupId, optionId);
        break;
      }
    }
  }
  
  // Use the option-specific topmost check
  const isTopmostForThisOption = optionGroup && enabled ?
    featureGroupStore.isTopmostForOption(feature.id, optionId) :
    true;
  
  // Prepare context for condition evaluation
  const context = {
    standardTag,
    needsDocker,
    romanizationSchemes,
    isTopmostForOption: isTopmostForThisOption,
    // Other context properties...
  };
  
  // Evaluate condition safely
  try {
    const evaluator = new Function('context', 'feature', 'featureGroupStore', 
      `return ${optionDef.showCondition}`);
    return Boolean(evaluator(context, {[feature.id]: options}, featureGroupStore));
  } catch (error) {
    console.error('Error evaluating condition:', optionDef.showCondition, error);
    return false;
  }
}
```

### Feature Mixins for Common Configurations

Reusable feature configurations are provided as mixins:

```typescript
export const subtitleFeatureMixin = {
  featureGroups: ['subtitle', 'merge'],
  requiresLanguage: true,
  requiresDocker: true,
  requiresScraper: true,
  optionOrder: ['style', 'provider', 'dockerRecreate', 'browserAccessURL', 'mergeOutputFiles', 'mergingFormat'],
};

export function applyFeatureMixins(feature: FeatureDefinition, ...mixins: Partial<FeatureDefinition>[]): FeatureDefinition {
  let result = { ...feature };
  
  // Apply each mixin
  mixins.forEach(mixin => {
    // Merge properties...
  });
  
  return result;
}
```

### Performance Optimizations

The system includes several performance optimizations:

1. **Caching**: Topmost status and visible options are cached to reduce redundant calculations
2. **Debounced Updates**: Changes are batched and debounced to prevent excessive re-rendering
3. **Selective Re-rendering**: Components only re-render when their specific data changes
4. **Store Subscriptions**: Fine-grained subscriptions to only relevant state changes
5. **Metrics Tracking**: Performance monitoring for identifying bottlenecks

```typescript
// Example of option-specific subscription
createOptionSubscription(gId: string, oId: string): Readable<any> {
  return derived(store, ($store) => {
    return $store.groupOptions[gId]?.[oId];
  });
}

// Example of performance metrics tracking
export class GroupPerformanceMonitor {
  private operations: Map<string, { count: number; totalTime: number; maxTime: number }> = new Map();
  
  measure<T>(operationName: string, operation: () => T): T {
    const start = performance.now();
    
    try {
      return operation();
    } finally {
      const time = performance.now() - start;
      // Record metrics...
    }
  }
  
  // Methods for reporting metrics...
}
```

## Common Pitfalls and Solutions

1. **Group Conflicts**: 
   - Problem: Features in multiple groups could have conflicting topmost status
   - Solution: Options know which group they belong to via `optionGroups` mapping

2. **Mixed Option Visibility**: 
   - Problem: Generic topmost check doesn't work for features in multiple groups
   - Solution: Option-specific `isTopmostForOption` check that identifies the proper group

3. **Different Group Orders**: 
   - Problem: Each group can have a different canonical order
   - Solution: Global order for merge group, group-specific order for others

4. **Option Ownership Ambiguity**: 
   - Problem: Without explicit option-to-group mapping, conflicts arise
   - Solution: Register each option to exactly one group in the store

5. **Performance Issues**:
   - Problem: Too many re-renders when options change
   - Solution: Fine-grained subscriptions and debounced updates

6. **Value Synchronization Races**:
   - Problem: Race conditions between user input and programmatic updates
   - Solution: Authority-based synchronization with timestamp tracking

## Best Practices for Working with the Group System

### 1. Adding new shared options

```typescript
// 1. Define the option in groupOptions.ts
merge: {
  newOption: {
    type: 'boolean',
    label: 'New Option',
    default: false,
    conditionalDisplay: "context.someCondition"
  }
}

// 2. Update feature definitions to use the group
{
  id: 'myFeature',
  featureGroups: ['merge'],
  groupSharedOptions: {
    'merge': ['mergeOutputFiles', 'mergingFormat', 'newOption']
  }
}

// 3. Register the option with its group during initialization
featureGroupStore.registerOptionToGroup('merge', 'newOption');
```

### 2. Creating a new group

```typescript
// 1. Define group in groupOptions.ts
newGroup: {
  option1: {
    type: 'boolean',
    label: 'Option 1',
    default: false
  },
  option2: {
    type: 'string',
    label: 'Option 2',
    default: ''
  }
}

// 2. Register group with the store
const newGroup = {
  id: 'newGroup',
  label: 'New Group',
  featureIds: ['feature1', 'feature2'],
  sharedOptions: ['option1', 'option2']
};
featureGroupStore.registerGroup(newGroup);

// 3. Register options to the group
['option1', 'option2'].forEach(optionId => {
  featureGroupStore.registerOptionToGroup('newGroup', optionId);
});

// 4. Update feature definitions
{
  id: 'feature1',
  featureGroups: ['newGroup'],
  groupSharedOptions: {
    'newGroup': ['option1', 'option2']
  }
}
```

### 3. Implementing conditional visibility

Use the option-specific context variable with additional conditions:

```typescript
// In groupOptions.ts
option1: {
  type: 'boolean',
  label: 'Option 1',
  default: false,
  conditionalDisplay: "context.needsDocker && context.romanizationSchemes.length > 0"
}

// In GroupOption component, conditionalDisplay becomes:
showCondition: "context.isTopmostForOption && (context.needsDocker && context.romanizationSchemes.length > 0)"
```

### 4. Features in multiple groups

```typescript
{
  id: 'subtitleRomanization',
  featureGroups: ['subtitle', 'merge'],
  groupSharedOptions: {
    'subtitle': ['style', 'provider', 'dockerRecreate', 'browserAccessURL'],
    'merge': ['mergeOutputFiles', 'mergingFormat']
  }
}
```

## Debugging and Validation

The system includes comprehensive debugging and validation tools:

1. **GroupDebugPanel**: UI component for inspecting group system state in real-time
2. **GroupInspector**: Programmatic access to detailed group system information
3. **GroupSystemValidator**: Validates group system configuration integrity
4. **Console Utilities**: Helper functions for debugging via the browser console

```typescript
// Example console debugging
window.__groupInspector.getGroupSystemState();
window.__featureGroupStore.getGroupOptions('subtitle');
validateGroupSystem();
```
