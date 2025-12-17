# Lite Mode and Blur Effects

This document explains the Lite Mode feature, why it exists, and how to create UI components that properly support it.

## Overview

Lite Mode is a reduced visual effects mode that disables `backdrop-filter: blur()` CSS effects throughout the application. When enabled, components use solid or semi-transparent backgrounds instead of glassmorphism blur effects.

## Why Lite Mode Exists

### 1. Qt WebEngine Flickering on Windows

Qt WebEngine (used by Anki's embedded WebView on Windows) has a bug where `backdrop-filter: blur()` causes severe visual flickering. This is a compositor-level issue in Qt's GPU integration on Windows.

**Symptoms:**
- Rapid visual artifacts on elements using `backdrop-filter`
- Flickering triggered by user interaction, external GPU activity, or screen recording
- Severity increases with stacked blur layers and larger blurred areas

**Confirmed behavior:**
- Same frontend renders fine in WebView2 (Windows) and Qt WebEngine (Linux)
- Only Qt WebEngine on Windows exhibits the issue
- `--disable-gpu-compositing` eliminates flickering but makes the app unusable

### 2. No Hardware Acceleration

When WebGL/WebGPU hardware acceleration is unavailable (software rendering), blur effects are computationally expensive and cause poor performance. Lite mode is automatically enabled in this scenario.

## When Lite Mode is Forced

Lite Mode is automatically forced (user cannot disable) in these scenarios:

| Scenario | Detection | Reason Code |
|----------|-----------|-------------|
| Anki add-on on Windows | `isAnkiMode && os === 'windows'` | `'qt-windows'` |
| No hardware acceleration | `!graphicsInfo.hardwareAccelerated` | `'no-hw-accel'` |

Users can also manually enable Lite Mode in Settings for performance reasons on low-end hardware.

## The Problematic CSS Pattern

```css
/* This causes flickering on Qt WebEngine Windows */
.element {
    background: rgba(255, 255, 255, 0.05);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
}
```

**Severity factors:**
1. Number of stacked blur layers (multiple overlapping blurs are worst)
2. Viewport coverage (full-screen blur effects are worst)
3. Blur radius (larger radius = more GPU work)

## Recommended Styling Patterns

### Pattern 1: Conditional Class Strings

Use Svelte's class interpolation to switch between blur and solid backgrounds:

```svelte
<script lang="ts">
    import { liteModeStore } from '../lib/stores';

    $: liteMode = $liteModeStore.enabled;
</script>

<!-- Button with glassmorphism -->
<button class="{liteMode ? 'bg-white/15' : 'bg-white/5 backdrop-blur-md'}
               border border-white/10 rounded-lg">
    Click me
</button>
```

### Pattern 2: Conditional Style Attribute

For more complex backgrounds, use inline styles:

```svelte
<div class="rounded-xl"
     style="{liteMode ? 'background-color: rgba(35, 35, 45, 0.97);' : ''}">
    {#if !liteMode}
        <div class="absolute inset-0 backdrop-blur-xl rounded-xl"></div>
    {/if}
    <div class="relative">Content here</div>
</div>
```

### Pattern 3: CSS Custom Properties with Data Attributes

For components with many blur instances, use data attributes:

```svelte
<div class="my-component" data-lite-mode={liteMode}>
    <!-- Content -->
</div>

<style>
    .my-component input {
        background-color: hsla(var(--input-bg), 0.5);
        backdrop-filter: blur(10px);
    }

    .my-component[data-lite-mode="true"] input {
        background-color: hsla(var(--input-bg), 0.85);
        backdrop-filter: none;
    }
</style>
```

## Background Opacity Guidelines

When replacing blur with solid backgrounds, increase opacity to compensate for lost frosted-glass effect:

| Original (with blur) | Lite Mode (no blur) | Notes |
|---------------------|---------------------|-------|
| `bg-white/5 backdrop-blur-md` | `bg-white/15` | Light overlay buttons |
| `bg-black/10 backdrop-blur-sm` | `bg-black/30` | Dark overlay cards |
| `bg-black/50 backdrop-blur-3xl` | `rgba(35, 35, 45, 0.97)` | Modal panels |
| `bg-input-bg/50 backdrop-blur-sm` | `bg-input-bg/80` | Form inputs |
| `bg-primary/90 backdrop-blur-sm` | `bg-primary` | Primary action buttons |

**General rule:** When removing blur, increase background opacity by 2-3x to maintain visual hierarchy and ensure content readability.

## Component Checklist

When creating new components, check for these blur-related classes and ensure lite mode alternatives exist:

- [ ] `backdrop-blur-sm` / `backdrop-blur-md` / `backdrop-blur-lg` / `backdrop-blur-xl` / `backdrop-blur-3xl`
- [ ] `backdrop-filter: blur()`
- [ ] `-webkit-backdrop-filter: blur()`

## Implementation Template

Here's a complete template for a component that supports Lite Mode:

```svelte
<script lang="ts">
    import { liteModeStore } from '../lib/stores';

    // Track lite mode state
    $: liteMode = $liteModeStore.enabled;

    // Your component logic here
</script>

<!-- Example: A card component -->
<div class="card {liteMode ? 'card--lite' : 'card--full'}">
    <slot />
</div>

<style>
    .card {
        border-radius: 0.75rem;
        border: 1px solid rgba(255, 255, 255, 0.1);
        padding: 1rem;
    }

    /* Full effects mode */
    .card--full {
        background: rgba(255, 255, 255, 0.05);
        backdrop-filter: blur(16px);
        -webkit-backdrop-filter: blur(16px);
    }

    /* Lite mode - no blur, more opaque */
    .card--lite {
        background: rgba(255, 255, 255, 0.12);
    }
</style>
```

## Testing Lite Mode

### On Linux/macOS (Development)

Use the Developer Dashboard to test Lite Mode:

1. Open Developer Dashboard (available when `version === "dev"` or developer mode is enabled)
2. Go to the "Style" tab
3. Toggle "Reduced Effects Mode" / "Lite Mode" switch

### On Windows with Anki

Lite Mode is automatically enabled. To verify:

1. Check the Settings panel - the "Lite mode" toggle should be forced ON and greyed out
2. The explanation text should say "Forced on due to Qt bug on Windows"

## Related Files

- `internal/gui/frontend/src/lib/stores.ts` - `liteModeStore` definition
- `internal/gui/frontend/src/App.svelte` - Auto-detection logic for Qt+Windows and hardware acceleration
- `internal/gui/frontend/src/components/Settings.svelte` - User-facing toggle

## Store API Reference

```typescript
import { liteModeStore } from '../lib/stores';

// Reactive subscription
$: liteMode = $liteModeStore.enabled;
$: isForced = $liteModeStore.isForced;
$: reason = $liteModeStore.reason; // 'qt-windows' | 'no-hw-accel' | 'user' | 'debug-override' | 'none'

// Methods (typically called from App.svelte, not components)
liteModeStore.setAuto(isAnkiMode, os);           // Called on startup
liteModeStore.setNoHardwareAcceleration();       // Called when GPU check fails
liteModeStore.setUserPreference(enabled);        // Called when user toggles setting
liteModeStore.setDebugOverride(enabled);         // Called from dev dashboard
```
