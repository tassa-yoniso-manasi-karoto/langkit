# WebView2 Problematic Code Patterns in Langkit

This document lists code patterns that are known to cause issues with WebView2 on Windows, based on architectural constraints and observed behavior.

## Critical Issues (Highly Confident)

### 1. Modal Dialog in Event Handler - ExportDebugReport
**File:** `/internal/gui/err.go:52`
**Problem:** `runtime.SaveFileDialog` creates a modal dialog directly within a Wails event handler
**WebView2 Violation:** Modal dialogs create nested message loops, causing reentrancy violations
**Symptom:** Export Debug Report hangs after settings changes
```go
// PROBLEMATIC CODE
savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{...})
```

### 2. Synchronous File I/O in SaveSettings
**File:** `/internal/gui/settings.go:21`
**Problem:** `config.SaveSettings(settings)` performs synchronous file I/O on the UI thread
**WebView2 Violation:** Blocks the message pump, preventing async completion callbacks
**Symptom:** Settings panel doesn't close after save, subsequent async calls hang
```go
// PROBLEMATIC CODE
err := config.SaveSettings(settings)  // Synchronous file write
// ...
a.llmRegistry.TriggerUpdate(settings) // May also block
```

### 3. Unthrottled Rapid ValidateLanguageTag Calls
**File:** `/internal/gui/frontend/src/components/Settings.svelte:359-364`
**Problem:** Reactive statement triggers validation on every keystroke without debouncing
**WebView2 Violation:** Overwhelms the message queue with concurrent async calls
**Symptom:** Language validation hangs after rapid input or settings changes
```javascript
// PROBLEMATIC CODE
$: {
    if (currentSettings.targetLanguage !== undefined ||
        currentSettings.nativeLanguages !== undefined) {
        validateLanguages(); // Called on every change, no debounce
    }
}
```

## Additional Patterns to Watch

### 4. Multiple Sequential Async Calls
**File:** `/internal/gui/frontend/src/components/Settings.svelte:196-222`
**Pattern:** Two ValidateLanguageTag calls in sequence without batching
```javascript
const targetResponse = await ValidateLanguageTag(currentSettings.targetLanguage, true);
// ... 
const nativeResponse = await ValidateLanguageTag(currentSettings.nativeLanguages, false);
```

### 5. Settings Subscribe Pattern
**File:** `/internal/gui/frontend/src/components/Settings.svelte:368-382`
**Pattern:** Store subscription that triggers validation, can cause cascading calls
```javascript
settings.subscribe(value => {
    // ...
    validateLanguages(); // Can trigger during saves
});
```

### 6. RefreshSTTModelsAfterSettingsUpdate
**File:** `/internal/gui/frontend/src/components/Settings.svelte:249`
**Pattern:** Another async call immediately after SaveSettings
```javascript
await (window as any).go.gui.App.SaveSettings(currentSettings);
// ...
await (window as any).go.gui.App.RefreshSTTModelsAfterSettingsUpdate();
```

## Root Cause Summary

WebView2 requires:
1. **No blocking operations on UI thread** - All file I/O must be async
2. **No modal dialogs in event handlers** - Must defer execution
3. **Throttled async calls** - Message queue can be overwhelmed
4. **Single-threaded message pump** - Any blocking prevents all async completions

## Recommended Solutions

1. **For Modal Dialogs:** Use deferred execution pattern
```javascript
// Defer dialog to next tick
setTimeout(() => {
    // Show dialog here
}, 0);
```

2. **For File I/O:** Make all backend operations truly async
```go
// Use goroutines for file operations
go func() {
    err := config.SaveSettings(settings)
    // Send completion via channel or callback
}()
```

3. **For Rapid Calls:** Implement debouncing
```javascript
let debounceTimer;
function debouncedValidate() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
        validateLanguages();
    }, 300);
}
```

4. **Alternative:** Use WebSocket for critical operations (already implemented for LLM state)