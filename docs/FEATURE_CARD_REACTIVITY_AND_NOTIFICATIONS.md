# FeatureCard Reactivity and Notifications

## Two Notification Systems

FeatureCard has two distinct ways to show messages to users:

### 1. Invalidation Error Store (Global)

Errors added to `invalidationErrorStore` with id pattern `provider-${feature.id}` are **automatically** displayed by the generic "API Provider error messages" section:

```svelte
{#if enabled && $invalidationErrorStore.some(e => e.id === `provider-${feature.id}`)}
    <!-- Warning icon + message + "Configure API Keys" button -->
{/if}
```

This is used for provider-specific token warnings like "OpenAI API token is required for GPT-4o Transcribe".

### 2. Feature-Specific Messages (Inline)

For messages that are specific to a feature's state (not just missing API keys), use the `hasFeatureMessages()` + template pattern:

**Step 1**: Add condition in `hasFeatureMessages()`:
```javascript
} else if (feature.id === 'dubtitles') {
    const availableSTTModels = currentSTTModels.models.filter(m => m.isAvailable);
    if (enabled && availableSTTModels.length === 0) {
        return true;
    }
}
```

**Step 2**: Add corresponding HTML in the feature message card:
```svelte
{:else if feature.id === 'dubtitles'}
    {@const availableSTTProviders = currentSTTModels.models.filter(m => m.isAvailable)}
    {#if enabled && availableSTTProviders.length === 0}
        <div class={messageItemClass}>
            <!-- info icon + helpful message -->
        </div>
    {/if}
{/if}
```

## STT Models Structure Gotcha

`currentSTTModels` contains:
- `models`: Array of **ALL** STT model objects (available and unavailable)
- `names`: Array of model names (may contain all names, not just available)
- `available`: Boolean
- `suggested`: String

**Critical**: To check if providers are available, **always filter by `isAvailable`**:

```javascript
// WRONG - names may include unavailable models
if (currentSTTModels.names.length === 0) { ... }

// CORRECT - explicitly check availability
const availableModels = currentSTTModels.models.filter(m => m.isAvailable);
if (availableModels.length === 0) { ... }
```

Each model object has:
- `name`: Internal identifier (e.g., 'gpt-4o-transcribe')
- `displayName`: Human-readable name
- `providerName`: Provider (e.g., 'OpenAI', 'Replicate')
- `isAvailable`: **Boolean** - true only if API key is configured
- `isRecommended`, `isDepreciated`: Display hints

## Coordinating Both Systems

When a condition should show a feature message instead of a provider error, you must:

1. **Prevent the error from being added** in `FeatureSelector.updateProviderWarnings()`:
```javascript
if (selectedFeatures.dubtitles && currentFeatureOptions.dubtitles) {
    const availableSTTModels = currentSTTModels.models.filter(m => m.isAvailable);
    if (availableSTTModels.length === 0) {
        // Skip token check - FeatureCard shows setup message instead
        invalidationErrorStore.removeError('provider-dubtitles');
    } else {
        // Normal token validation...
    }
}
```

2. **Show the feature message** in FeatureCard (as described above)

## Reactive Computations

### Store Subscriptions
```javascript
// In script - for reactive computations
$: voiceEnhancingNeedsDocker = feature.id === 'voiceEnhancing' &&
    options.sepLib?.startsWith('docker-');

// In template - auto-subscribes
{#if $invalidationErrorStore.some(e => e.id === 'provider-dubtitles')}
```

### Template-Local Constants
Use `{@const}` for computed values needed only in template:
```svelte
{@const availableSTTProviders = currentSTTModels.models.filter(m => m.isAvailable)}
{#if availableSTTProviders.length === 0}
```

## showCondition Pattern

Options can be conditionally visible via `showCondition` in featureModel.ts:
```javascript
useNvidiaGPU: {
    type: 'boolean',
    label: 'Use NVIDIA GPU acceleration',
    showCondition: "context.voiceEnhancingNeedsDocker"
}
```

The condition is evaluated in `evaluateShowCondition()` against a `context` object containing:
- `standardTag`, `needsDocker`, `needsScraper`
- `romanizationSchemes`, `selectedFeatures`, `sttModels`
- Custom computed properties like `voiceEnhancingNeedsDocker`

## Message Styling Classes

```javascript
const messageItemClass = "flex items-start gap-2 py-1.5 px-2 rounded-md ...";
```

Icon options:
- `text-primary` - info/neutral (blue)
- `text-log-warn` - warning (yellow)
- `text-log-info` - informational (cyan)
- `text-[#ff0000]` - critical error (red)

## Key Files

| File | Role |
|------|------|
| `FeatureCard.svelte` | Per-feature UI, message display |
| `FeatureSelector.svelte` | Parent, owns options, manages `invalidationErrorStore` |
| `invalidationErrorStore.ts` | Global error store with auto-dismiss |
| `featureModel.ts` | Feature definitions, STT model store |
