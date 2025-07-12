# WebView2 - Possible Areas of Improvement

This document identifies code patterns in the frontend that are likely to overwhelm WebView2's single-threaded message pump, ordered by severity.

## Critical Issues

### 1. Settings.svelte - Cascading Reactive Validation (Lines 370-385)
```javascript
// Re-validate whenever relevant parts of currentSettings change
$: {
    if (currentSettings.targetLanguage !== undefined ||
        currentSettings.nativeLanguages !== undefined) {
        debouncedValidateLanguages();
    }
}

// Set validating state immediately when target language actually changes
$: if (currentSettings.targetLanguage !== previousTargetLanguage) {
    if (currentSettings.targetLanguage !== undefined) {
        isValidatingTarget = true;
        previousTargetLanguage = currentSettings.targetLanguage;
    }
}

// Set validating state immediately when native languages actually change
$: if (currentSettings.nativeLanguages !== previousNativeLanguages) {
    if (currentSettings.nativeLanguages !== undefined) {
        isValidatingNative = true;
        previousNativeLanguages = currentSettings.nativeLanguages;
    }
}
```
**Why problematic:** Three separate reactive blocks monitoring language changes. The first triggers on ANY currentSettings change (not just language fields), potentially causing validation calls when unrelated settings change. Combined with the two state-setting blocks, this creates multiple reactive evaluations per keystroke.

### 2. FeatureSelector.svelte - Duplicate Language Change Watchers (Lines 1034-1041, 1724-1735)
```javascript
// First reactive block
$: {
    if (quickAccessLangTag !== lastProcessedLangTag) {
        if (lastProcessedLangTag.toLowerCase() !== quickAccessLangTag.toLowerCase()) {
            debouncedProcessLanguageChange(quickAccessLangTag);
        }
        lastProcessedLangTag = quickAccessLangTag;
    }
}

// Second reactive block (1000+ lines later)
$: {
    if (quickAccessLangTag !== previousQuickAccessLangTag) {
        if (previousQuickAccessLangTag !== undefined) {
            debouncedProcessLanguageChange(quickAccessLangTag);
        }
        previousQuickAccessLangTag = quickAccessLangTag;
    }
}
```
**Why problematic:** Two reactive blocks watching the same variable (`quickAccessLangTag`) and calling the same function. This doubles the reactive evaluations and can cause race conditions where both blocks trigger simultaneously.

### 3. FeatureCard.svelte - Broad Settings Subscription (Line 236)
```javascript
settingsUnsubscribe = settings.subscribe(() => {
    debouncedCheckNativeLanguageIsEnglish();
});
```
**Why problematic:** Subscribes to ALL settings changes but only needs to react to `nativeLanguages` changes. Any settings modification (theme, API keys, etc.) triggers language validation, creating unnecessary backend calls.

## High Priority Issues

### 4. ProcessButton.svelte - Unthrottled Mouse Events (Lines 92-102)
```javascript
function handleMouseMove(event: MouseEvent) {
    if (!isPressed || isDisabled || !buttonElement) return;
    
    const rect = buttonElement.getBoundingClientRect();
    const x = ((event.clientX - rect.left) / rect.width) * 100;
    const y = ((event.clientY - rect.top) / rect.height) * 100;
    
    rippleX = x;
    rippleY = y;
}
```
**Why problematic:** Mouse move events fire continuously (potentially 100+ times per second) without throttling. Each event triggers Svelte's reactivity system to update ripple position variables.

### 5. ProgressManager.svelte - Continuous Array Sorting (Line 29)
```javascript
$: sortedBars = progressBars.slice().sort((a, b) => {
    const orderA = displayOrder[a.id] ?? 999;
    const orderB = displayOrder[b.id] ?? 999;
    return orderA - orderB;
});
```
**Why problematic:** Sorts the entire progress bars array on every change to either `progressBars` or `displayOrder`. With active media processing generating frequent updates, this creates constant re-sorting operations.

## Medium Priority Issues

### 6. Settings.svelte - High-Frequency WASM State Polling (Lines 364-366)
```javascript
wasmStateUpdateInterval = window.setInterval(() => {
    wasmState = getWasmState();
}, 1000); // Update every second
```
**Why problematic:** Polls WASM state every second regardless of whether the settings panel is visible or if WASM state has actually changed. Creates unnecessary reactive updates.

### 7. LogView.svelte - Window State Monitoring (Lines 368-370)
```javascript
setInterval(() => {
    checkIfMinimized();
}, 1000);
```
**Why problematic:** Another 1-second interval checking window state. Combined with other intervals, creates multiple periodic timers competing for the message pump.

### 8. FeatureSelector.svelte - Complex Option Visibility Calculation (Lines 559-566)
```javascript
function calculateVisibleOptions(featureId: string): { visibleOptions: Set<string>, debugInfo: any } {
    const feature = getFeatureById(featureId);
    if (!feature || !feature.options) {
        return { visibleOptions: new Set(), debugInfo: {} };
    }
    
    const visibleOptions = new Set<string>();
    const debugInfo: any = {};
    
    for (const [optionKey, optionDef] of Object.entries(feature.options)) {
        // ... complex visibility calculation
    }
}
```
**Why problematic:** Called reactively for multiple features whenever various dependencies change (settings, docker status, LLM state). The function performs complex object iterations and condition evaluations that can stack up during rapid state changes.

## Pattern Summary

The most severe issues involve:
1. **Overly broad reactive dependencies** - Reacting to entire objects when only specific properties matter
2. **Duplicate watchers** - Multiple reactive blocks monitoring the same state
3. **Unthrottled high-frequency events** - Mouse moves, scroll events without rate limiting
4. **Polling instead of event-driven updates** - Regular intervals checking for changes

These patterns are particularly problematic for WebView2 because its single-threaded message pump must process all JavaScript-to-native calls sequentially, making it vulnerable to being overwhelmed by rapid successive operations.