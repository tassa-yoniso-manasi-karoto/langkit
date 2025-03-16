# Feature Group System Architecture

## Overview

The Feature Group system allows multiple features to share common options and settings. This provides a consistent user experience while reducing redundancy in the UI. When multiple features belong to the same group (e.g., subtitle processing features), certain options (like browser URL, provider, etc.) are shared across all features in the group.

## Key Components

### 1. Feature Group Store (`featureGroupStore.ts`)

The central store that manages all group-related state, including:

- Group definitions and membership
- Shared option values
- Active display feature tracking
- Validation rules and error handling

### 2. Group Option Component (`GroupOption.svelte`)

A specialized component for rendering and managing group-shared options with:

- Authority-based value synchronization
- User input priority handling
- Validation integration
- Value persistence across feature toggling

### 3. Feature Selector Integration

The `FeatureSelector.svelte` component integrates with the group system by:

- Initializing feature groups
- Managing feature enable/disable events
- Determining which feature displays group options
- Synchronizing values across all features

## Critical Implementation Details

### Value Authority System

The most critical aspect of the system is the "authority" mechanism that determines which value source to trust:

```typescript
// Track timing of updates to determine authority
let lastUserUpdateTime = 0;
let lastExternalUpdateTime = Date.now();

// User input always gets a future timestamp to ensure priority
function handleUserInput(newValue) {
    lastUserUpdateTime = Date.now() + 1000; // Future timestamp
    localValue = newValue;
    // ...propagate changes
}
```

**Key Points:**
- User-entered values have higher priority than system/store values
- Timestamps are used to determine which update is more recent
- Empty values don't update the external timestamp to prevent overriding valid user inputs

### Protecting Critical Values

For critical values like the WebSocket URL, additional protection is needed:

```typescript
// Special case handling for Browser URLs
if (optionId === 'browserAccessURL' && !value && localValue && localValue.startsWith('ws://')) {
    // Block empty values from overriding valid WebSocket URLs
    propagateUserValue(localValue);
}
```

### Value Propagation Workflow

The correct flow for propagating values is crucial:

1. Update local component state
2. Update the central group store
3. Validate if needed (e.g., browser URL)
4. Notify parent components
5. Sync to all other features in the group

### Proper Reactivity Structure

Svelte's reactivity system requires careful structuring:

```svelte
// Incorrect - using return inside reactive statement
$: if (condition) {
    // do something
    return; // Error!
}

// Correct - using block structure
$: {
    if (condition) {
        // do something
    } else {
        // alternative path
    }
}
```

## Common Pitfalls

1. **Circular Updates**: Reactive statements can trigger cascading updates that override user inputs

2. **Empty Value Propagation**: Empty values from initialization or reset operations can override valid user inputs

3. **Timing Issues**: Component mounting and initialization can race with user inputs

4. **Context Switching**: When switching between features (disabling one, enabling another), values can be lost

## Best Practices

### Implementing New Group Options

When adding a new option to a group:

1. Define the option in the `FeatureDefinition` interface
2. Add it to the group's `sharedOptions` array
3. Implement any necessary validation rules
4. Ensure proper initialization in all relevant features

```typescript
const newGroup = {
    id: 'myGroup',
    featureIds: ['feature1', 'feature2'],
    sharedOptions: ['option1', 'option2', 'newOption'],
    validationRules: [
        {
            id: 'validation-rule',
            optionId: 'newOption',
            validator: (value) => Boolean(value), // Custom validation logic
            errorMessage: 'Error message',
            severity: 'critical'
        }
    ]
};
```

### Creating a New Group

To create a new feature group:

1. Define the group with ID, features, and shared options
2. Register it in the `featureGroupStore`
3. Update the feature definitions with group membership
4. Define shared options for each feature
5. Implement initialization in `FeatureSelector`

```typescript
// In FeatureSelector.svelte
function initializeFeatureGroups() {
    // Define the new group
    const newGroup = {
        id: 'newGroup',
        label: 'New Group Label',
        featureIds: ['feature1', 'feature2'],
        sharedOptions: ['option1', 'option2']
    };
    
    // Register with store
    featureGroupStore.registerGroup(newGroup);
    
    // Set up features
    features.filter(f => newGroup.featureIds.includes(f.id))
        .forEach(feature => {
            if (!feature.featureGroups) {
                feature.featureGroups = [];
            }
            feature.featureGroups.push(newGroup.id);
            
            if (!feature.groupSharedOptions) {
                feature.groupSharedOptions = {};
            }
            feature.groupSharedOptions[newGroup.id] = newGroup.sharedOptions;
        });
        
    // Initialize values
    // ...
}
```

## Value Synchronization Logic

When a feature is enabled/disabled, the appropriate shared option values must be preserved:

```typescript
// When a feature is enabled
if (enabled) {
    // Update group store
    featureGroupStore.updateFeatureEnabled(groupId, id, true);
    
    // Determine if this feature should display group options
    const enabledOrderedFeatures = orderedGroupFeatures
        .filter(fId => enabledFeaturesInGroup.includes(fId));
        
    if (enabledOrderedFeatures[0] === id) {
        // Make this feature the active display feature
        featureGroupStore.updateActiveDisplayFeature(groupId, orderedGroupFeatures, enabledFeaturesInGroup);
    }
    
    // Sync values from store to all features in group
    currentFeatureOptions = featureGroupStore.syncOptionsToFeatures(groupId, currentFeatureOptions);
}
```

## Option Display Logic

Only one feature in a group should display the shared options:

```svelte
{#if feature.featureGroups && feature.groupSharedOptions && 
     feature.featureGroups.some(groupId => feature.groupSharedOptions[groupId]?.includes(optionId)) &&
     feature.featureGroups.some(groupId => featureGroupStore.isActiveDisplayFeature(groupId, feature.id))}
    <!-- Show group option only for the active display feature -->
    <GroupOption {...props} />
{:else}
    <!-- Show regular option -->
    <RegularOption {...props} />
{/if}
```

## Error Handling

The group system integrates with the error store for option validation:

```typescript
// In the validator
if (!isValid) {
    errorStore.addError({
        id: `group-${groupId}-${rule.id}`,
        message: rule.errorMessage,
        severity: rule.severity
    });
}
```

## Summary

The Feature Group system provides a robust way to share options across related features. The key to a reliable implementation lies in:

1. Proper value authority management (user input priority)
2. Special case handling for critical values
3. Correct propagation of values through the system
4. Preventing empty values from overriding valid user inputs
5. Properly structured reactive statements
6. Centralized store as the source of truth

By following these guidelines, new groups and shared options can be added reliably without encountering synchronization issues.