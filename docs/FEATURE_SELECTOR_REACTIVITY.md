# FeatureSelector / FeatureCard Reactivity Notes

## Component Hierarchy

```
FeatureSelector.svelte
  └── FeatureCard.svelte (one per feature)
        └── Dropdown, SepLibDropdown, NumericInput, etc.
```

## The Shared Reference Gotcha

`FeatureSelector` owns `currentFeatureOptions` and passes object references to each `FeatureCard`:

```javascript
// FeatureCard receives the SAME object, not a copy
options={currentFeatureOptions[feature.id]}
```

When FeatureCard mutates `options[optionId] = value`, it directly mutates the parent's object. This means:

1. **Reactive statements in FeatureSelector won't fire** - Svelte only tracks top-level assignments, not nested mutations
2. **Comparisons are always stale** - by the time the event reaches the parent, the value is already changed

```javascript
// THIS DOES NOT WORK in FeatureSelector:
$: if (currentFeatureOptions?.voiceEnhancing?.sepLib) {
    updateProviderWarnings();  // Never re-runs on change
}
```

## The Working Pattern

Use the `value` from the dispatched event directly:

```javascript
// FeatureSelector.svelte - handleOptionChange()
if (featureId === 'voiceEnhancing' && optionId === 'sepLib') {
    // Use `value` from event - guaranteed fresh
    const isReplicateProvider = value.startsWith('replicate-');
    // ...
}
```

To force UI updates after programmatic changes:
```javascript
currentFeatureOptions.voiceEnhancing.voiceBoost = 37;
currentFeatureOptions = {...currentFeatureOptions};  // Force reactivity
```

## Voice Separation Provider Naming

The `sepLib` option uses prefixed internal names:

| Prefix | Meaning | Example |
|--------|---------|---------|
| `docker-` | Local Docker (CPU) | `docker-demucs` |
| `docker-nvidia-` | Local Docker (GPU) | `docker-nvidia-mel-roformer-kim` |
| `replicate-` | Replicate API (cloud) | `replicate-spleeter` |

The `SepLibDropdown` component parses these prefixes and displays platform icons (Docker, NVIDIA, Replicate) instead of the raw text.

## Dynamic Behavior on Provider Change

In `handleOptionChange`, when `sepLib` changes:
- API token warning added/removed based on provider requirements
- `voiceBoost` adjusted (37 dB for docker, 13 dB for replicate)

## Key Files

| File | Role |
|------|------|
| `featureModel.ts` | Feature definitions, defaults, `sepLibDisplayNames` |
| `FeatureSelector.svelte` | Parent component, owns `currentFeatureOptions` |
| `FeatureCard.svelte` | Per-feature UI, mutates options directly |
| `SepLibDropdown.svelte` | Voice separation dropdown with platform icons |
