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

The store provides these key methods:
- `initializeCanonicalOrder()`: Sets the definitive ordering of features based on the feature model
- `isTopmostInGroup()`: Determines if a feature is the first enabled one in its group
- `registerOptionToGroup()`: Associates an option with a specific group
- `getGroupForOption()`: Retrieves which group an option belongs to
- `isTopmostForOption()`: Determines if a feature should display a specific option based on its group
- `syncOptionsToFeatures()`: Ensures all features in a group have the same option values
- `updateFeatureEnabled()`: Updates which features are enabled in each group

### 2. Group Option Component (`GroupOption.svelte`)

A specialized component for rendering and managing group-shared options with:

- Authority-based value synchronization
- User input priority handling
- Validation integration
- Value persistence across feature toggling

### 3. Feature Selector Component

Manages feature initialization and UI coordination:

- Initializes canonical feature ordering from the feature model
- Registers feature groups and their shared options
- Registers options to their respective groups
- Manages feature enable/disable events
- Coordinates value synchronization across the system

### 4. Feature Card Component

Manages individual feature display including:

- Context-aware option visibility determination 
- Automatic option-to-group registration
- Conditional rendering of group options
- Handling user inputs for option values
- Propagating changes to the central store

## Implementation Approach

### Option-Group Relationship Management

The system maintains a clear mapping between options and their owning groups:

```typescript
// In featureGroupStore.ts
interface GroupState {
    // ... existing fields
    optionGroups: Record<string, string>; // Maps option IDs to their owning group IDs
}

registerOptionToGroup(groupId: string, optionId: string) {
    store.update(state => {
        const newState = { ...state };
        newState.optionGroups[optionId] = groupId;
        return newState;
    });
}

getGroupForOption(optionId: string): string | null {
    const state = get(store);
    return state.optionGroups[optionId] || null;
}
```

### Context-Aware Option Visibility

The system uses option-specific context to determine visibility:

```typescript
// In FeatureCard.svelte
function shouldShowOption(optionId: string, optionDef: any): boolean {
    // Find which group this option belongs to (if any)
    let optionGroup = null;
    if (feature.groupSharedOptions) {
        for (const [groupId, options] of Object.entries(feature.groupSharedOptions)) {
            if (options.includes(optionId)) {
                optionGroup = groupId;
                
                // Register this option with the group store
                featureGroupStore.registerOptionToGroup(groupId, optionId);
                break;
            }
        }
    }
    
    // Use the option-specific topmost check
    let isTopmostForThisOption = false;
    if (enabled) {
        isTopmostForThisOption = featureGroupStore.isTopmostForOption(feature.id, optionId);
    }
    
    // Context for conditional rendering
    const context = {
        // ...other context properties
        isTopmostForOption: isTopmostForThisOption
    };
}
```

### Option-Specific Topmost Check

```typescript
// In featureGroupStore.ts
isTopmostForOption(featureId: string, optionId: string): boolean {
    // Get the group this option belongs to
    const groupId = this.getGroupForOption(optionId);
    if (!groupId) {
        console.warn(`Option ${optionId} is not registered with any group`);
        return true; // Default to showing if not registered
    }
    
    // Check if this feature is the topmost in the option's group
    return this.isTopmostInGroup(groupId, featureId);
}
```

### Canonical Order Approach

The system uses canonical ordering (not DOM position) to determine the topmost feature:

```typescript
// In FeatureSelector.svelte
onMount(() => {
  // Get ordered feature IDs from the original features array
  const canonicalOrder = features.map(f => f.id);

  // Initialize feature group store with this canonical order
  featureGroupStore.initializeCanonicalOrder(canonicalOrder);
});
```

Special handling is provided for the merge group:

```typescript
// In featureGroupStore.ts
isTopmostInGroup(groupId: string, featureId: string): boolean {
    // ... basic checks

    // Special case for merge group
    if (groupId === 'merge') {
        // Use global canonical order for merge group
        const globalOrder = state.canonicalOrder || [];
        
        // Find first enabled feature in the merge group using global order
        const topmostFeature = globalOrder.find(id => enabledFeatures.includes(id));
        
        return topmostFeature === featureId;
    }
    
    // Standard group handling...
}
```

### Feature Model Option Visibility

Options use the new `isTopmostForOption` context variable:

```typescript
// In featureModel.ts
options: {
    mergeOutputFiles: {
        type: 'boolean',
        label: 'Merge all processed outputs',
        default: false,
        hovertip: "When enabled, all processed outputs will be merged into a single video file.",
        showCondition: "context.isTopmostForOption"
    },
    // ...
}
```

## Group Initialization

During initialization, groups and their options are registered:

```typescript
// In FeatureSelector.svelte
function initializeFeatureGroups() {
    // Register subtitle group
    const subtitleGroup = {
        id: 'subtitle',
        label: 'Subtitle Processing',
        featureIds: [...],
        sharedOptions: ['style', 'provider', 'dockerRecreate', 'browserAccessURL']
    };
    
    featureGroupStore.registerGroup(subtitleGroup);
    
    // Register options to their groups
    ['style', 'provider', 'dockerRecreate', 'browserAccessURL'].forEach(optionId => {
        featureGroupStore.registerOptionToGroup('subtitle', optionId);
    });
    
    // Register merge group and its options
    featureGroupStore.registerOptionToGroup('merge', 'mergeOutputFiles');
    featureGroupStore.registerOptionToGroup('merge', 'mergingFormat');
}
```

## Value Synchronization Logic

The system ensures values are properly synchronized through the central store:

```typescript
// Store's sync method ensures all features have identical values
syncOptionsToFeatures(groupId, currentOptions) {
  const newOptions = { ...currentOptions };
  const groupOptions = this.getGroupOptions(groupId);
  
  // Apply group option values to all features in the group
  this.getGroupFeatures(groupId).forEach(featureId => {
    // Apply each shared option value
    this.getSharedOptions(groupId).forEach(optionId => {
      if (groupOptions[optionId] !== undefined) {
        newOptions[featureId][optionId] = groupOptions[optionId];
      }
    });
  });
  
  return newOptions;
}
```

## Multi-Group Feature Configuration

Features can belong to multiple groups:

```typescript
// In featureModel.ts
{
  id: 'subtitleRomanization',
  // ...
  featureGroups: ['subtitle', 'merge'],
  groupSharedOptions: {
    'subtitle': ['style', 'provider', 'dockerRecreate', 'browserAccessURL'],
    'merge': ['mergeOutputFiles', 'mergingFormat']
  }
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

## Best Practices for Working with the Group System

1. **Adding new shared options**:
   - Add option definition to feature model
   - Add to groupSharedOptions array for each feature in the group
   - Register option with its group during initialization
   - Use `context.isTopmostForOption` in showCondition

2. **Creating a new group**:
   - Define group with ID, features, and shared options
   - Register with featureGroupStore
   - Register options to the group
   - Update feature definitions with group membership
   - Provide default values for shared options

3. **Implementing conditional visibility**:
   - Use the option-specific context variable:
   ```typescript
   showCondition: "context.isTopmostForOption && context.needsDocker"
   ```

4. **Features in multiple groups**:
   - Ensure each option is registered with exactly one group
   - Organize groupSharedOptions by group ID
   - Test visibility when multiple group features are enabled

## Summary

The Feature Group system provides a robust mechanism for sharing options across related features, even when features belong to multiple groups. By tracking which option belongs to which group and using context-aware visibility checks, the system ensures consistent display of shared options regardless of which combination of features is enabled.