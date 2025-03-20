# Feature Group System Architecture

## Overview

The Feature Group system allows multiple features to share common options and settings. This provides a consistent user experience while reducing redundancy in the UI. When multiple features belong to the same group (e.g., subtitle processing features), certain options (like browser URL, provider, etc.) are shared across all features in the group.

## System Requirements

1. **Single Instance Rule**: Group-shared options must appear only once in the UI, even when multiple features from the same group are enabled
2. **Topmost Display**: Shared options should be displayed in the topmost (first) feature of the group
3. **Consistent Values**: All features must use the same values for shared options
4. **Value Synchronization**: Changes to a shared option in one feature must propagate to all features in the group
5. **Reliable Order Determination**: The system must reliably determine which feature should show the options

## Key Components

### 1. Feature Group Store (`featureGroupStore.ts`)

The central store manages all group-related state, including:

- Group definitions and membership
- Feature canonical ordering
- Shared option values
- Feature enabled status tracking
- Topmost feature determination
- Value validation

The store provides these key methods:
- `initializeCanonicalOrder()`: Sets the definitive ordering of features based on the feature model
- `isTopmostInGroup()`: Determines if a feature is the first enabled one in its group
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
- Manages feature enable/disable events
- Coordinates value synchronization across the system

### 4. Feature Card Component

Manages individual feature display including:

- Checking if it should display group options (via `isTopmostInGroup`)
- Conditional rendering of group options
- Handling user inputs for option values
- Propagating changes to the central store

## Implementation Approach

The system uses a canonical order approach (instead of DOM queries) to determine the topmost feature:

1. During initialization, the feature model's original order is captured:
```typescript
// In FeatureSelector.svelte
onMount(() => {
  // Get ordered feature IDs from the original features array
  const canonicalOrder = features.map(f => f.id);

  // Initialize feature group store with this canonical order
  featureGroupStore.initializeCanonicalOrder(canonicalOrder);
});
```

2. The store maintains this canonical order and derives group-specific orders:
```typescript
// In featureGroupStore.ts
initializeCanonicalOrder(orderedFeatureIds: string[]) {
  store.update(state => {
    const newState = { ...state };
    newState.canonicalOrder = orderedFeatureIds;
    return newState;
  });
  
  // Update canonical order for all groups
  Object.keys(this.getGroups()).forEach(groupId => {
    this.updateGroupCanonicalOrder(groupId);
  });
}
```

3. A reliable method determines the topmost feature in each group:
```typescript
isTopmostInGroup(groupId: string, featureId: string): boolean {
  const state = get(store);
  
  // First check if the feature is enabled
  if (!state.enabledFeatures[groupId]?.includes(featureId)) {
    return false;
  }
  
  // Get canonical order for this group
  const groupOrder = state.groupCanonicalOrder[groupId] || [];
  
  // Get all enabled features
  const enabledFeatures = state.enabledFeatures[groupId] || [];
  
  // Find the first enabled feature according to canonical order
  const topmostFeature = groupOrder.find(id => enabledFeatures.includes(id));
  
  // This feature is the topmost if it matches the first enabled feature
  return topmostFeature === featureId;
}
```

4. Option visibility is controlled through the `showCondition` system:
```typescript
// In featureModel.ts, for each shared option
showCondition: "context.isTopmostInGroup && context.needsScraper"
```

5. The `isTopmostInGroup` context variable is provided to option conditions:
```typescript
// In FeatureCard.svelte
function shouldShowOption(optionId: string, optionDef: any): boolean {
  // Determine if this is the topmost feature in any groups
  let isTopmostInGroupForCache = false;
  if (feature.featureGroups && feature.featureGroups.length > 0 && enabled) {
    for (const groupId of feature.featureGroups) {
      if (featureGroupStore.isTopmostInGroup(groupId, feature.id)) {
        isTopmostInGroupForCache = true;
        break;
      }
    }
  }
  
  // Context object for evaluating conditions
  const context = {
    // ...other context properties
    isTopmostInGroup: isTopmostInGroupForCache
  };
  
  // Evaluate condition with this context
  // ...
}
```

## Value Synchronization Logic

The system ensures values are properly synchronized through the following mechanism:

1. When a user changes a shared option value, it's first updated in the central store:
```typescript
// In GroupOption.svelte
function handleValueChange(newValue) {
  // Update local state with user input
  localValue = newValue;
  lastUserUpdateTime = Date.now() + 1000; // Future timestamp for priority
  
  // Update the central store
  featureGroupStore.setGroupOption(groupId, optionId, newValue);
  
  // Notify parent component
  dispatch('groupOptionChange', { groupId, optionId, value: newValue });
}
```

2. The feature selector syncs values from the store to all features:
```typescript
// In FeatureSelector.svelte
function handleGroupOptionChange(event) {
  const { groupId, optionId, value } = event.detail;
  
  // Update central store first
  featureGroupStore.setGroupOption(groupId, optionId, value);
  
  // Sync to all features in the group
  currentFeatureOptions = featureGroupStore.syncOptionsToFeatures(
    groupId, 
    currentFeatureOptions
  );
}
```

3. The store's sync method ensures all features have identical values:
```typescript
syncOptionsToFeatures(groupId, currentOptions) {
  const newOptions = { ...currentOptions };
  const groupOptions = this.getGroupOptions(groupId);
  
  // Apply group option values to all features in the group
  this.getGroupFeatures(groupId).forEach(featureId => {
    if (!newOptions[featureId]) {
      newOptions[featureId] = {};
    }
    
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

## Value Authority System

To handle the potential for conflicting updates, the system implements an authority mechanism using timestamps:

```typescript
// In GroupOption.svelte
let lastUserUpdateTime = 0;
let lastExternalUpdateTime = Date.now();

// When receiving external updates
$: if (externalValue !== undefined && externalValue !== localValue) {
  // Only accept external changes if they're more recent than user changes
  if (lastExternalUpdateTime > lastUserUpdateTime) {
    localValue = externalValue;
  } else {
    // User's change takes precedence - propagate back to the system
    propagateUserValue(localValue);
  }
  lastExternalUpdateTime = Date.now();
}

// User input gets future timestamp to ensure priority
function handleUserInput(newValue) {
  lastUserUpdateTime = Date.now() + 1000;
  localValue = newValue;
  propagateUserValue(newValue);
}
```

## Feature Membership Configuration

Features are configured to belong to groups in the feature model:

```typescript
// In featureModel.ts
{
  id: 'subtitleTokenization',
  // ...other properties
  featureGroups: ['subtitle'],
  groupSharedOptions: {
    'subtitle': ['style', 'provider', 'dockerRecreate', 'browserAccessURL']
  }
}
```

The group itself is defined during initialization:

```typescript
// In FeatureSelector.svelte
function initializeFeatureGroups() {
  const subtitleGroup = {
    id: 'subtitle',
    label: 'Subtitle Processing',
    featureIds: ['subtitleRomanization', 'selectiveTransliteration', 'subtitleTokenization'],
    sharedOptions: ['style', 'provider', 'dockerRecreate', 'browserAccessURL'],
    validationRules: [
      // Rules for option validation
    ]
  };
  
  featureGroupStore.registerGroup(subtitleGroup);
  
  // Additional setup...
}
```

## Common Pitfalls and Solutions

1. **Circular Updates**: 
   - Problem: Reactive statements can trigger cascading updates
   - Solution: Use proper timestamp-based authority system

2. **Empty Value Propagation**: 
   - Problem: Empty values can override valid user inputs
   - Solution: Special handling for critical values
   ```typescript
   // Special case for important values like WebSocket URLs
   if (optionId === 'browserAccessURL' && !value && localValue && localValue.startsWith('ws://')) {
     // Block empty values from overriding valid WebSocket URLs
     propagateUserValue(localValue);
   }
   ```

3. **Timing Issues**: 
   - Problem: Component mounting and initialization can race
   - Solution: Wait for components to fully mount before propagating values

4. **Topmost Feature Determination**: 
   - Problem: DOM-based approaches are unreliable
   - Solution: Use canonical order from the feature model

## Best Practices for Working with the Group System

1. **Adding new shared options**:
   - Add option definition to feature model
   - Add to groupSharedOptions array for each feature in the group
   - Use `context.isTopmostInGroup` in showCondition

2. **Creating a new group**:
   - Define group with ID, features, and shared options
   - Register with featureGroupStore
   - Update feature definitions with group membership
   - Provide default values for shared options

3. **Implementing conditional visibility**:
   - Use the `isTopmostInGroup` context variable:
   ```typescript
   showCondition: "context.isTopmostInGroup && context.needsDocker"
   ```

4. **Adding features to a group**:
   - Update feature definition with group membership
   - Add group shared options configuration
   - Ensure feature is registered with the group store

## Summary

The Feature Group system provides a robust, reliable way to share options across related features. By using canonical feature ordering instead of DOM position, the system can consistently determine which feature should display the group's shared options. The topmost feature is derived from the original order in the feature model, ensuring consistent behavior regardless of DOM rendering or visual effects.