# Bug Report: Svelte 4 Reactivity Loss in Production Builds

**Date**: January 2025  
**Affected Components**: FeatureCard.svelte  
**Severity**: High - Feature messages completely fail to render in certain production builds  

## Executive Summary

A critical bug was discovered where Svelte 4's reactivity system fails when function calls are used in templates under certain production build configurations. The issue manifested as feature messages not rendering in production builds, despite working correctly in development and some production builds. The root cause was Svelte's fragile dependency tracking for function calls in templates, which breaks under certain build optimizations.

## Symptoms

### Primary Issue
Feature messages (merge banners, docker status, etc.) in FeatureCard components failed to display under specific conditions:

1. **WebView2 (Windows)**: 
   - Partial rendering - some messages displayed, others didn't
   - Padding changes indicated DOM was partially updating
   - Validation messages stayed in loading state indefinitely

2. **WebKit (Linux - GitHub Actions build)**:
   - Complete failure - no messages rendered at all
   - Only occurred with production builds from GitHub Actions
   - Local production builds worked correctly

### Observable Behavior
- The padding around feature cards changed (pb-4 → pb-1), confirming `hasFeatureMessages()` returned true
- The actual message content failed to render despite the container being created
- Issue was reproducible across different browser engines when using specific build tools

## Environment Details

### Working Configuration (Local Production Build)
```
go                              1.23.9 / 1.23.11
wails                           2.10.2
libwebkit2gtk                   2.46.6
github.com/wailsapp/wails/v2    2.10.2
```

### Failing Configuration (GitHub Actions)
```
go                              1.23.11
wails                           2.9.0
libwebkit2gtk                   2.48.3
github.com/wailsapp/wails/v2    2.9.0 / 2.10.2
```

## Root Cause Analysis

### The Problem Code

**Before (Broken in certain builds):**
```svelte
<script>
    function hasFeatureMessages() {
        // Complex logic checking multiple conditions
        if (enabled && $invalidationErrorStore.some(...)) return true;
        if (feature.outputMergeGroup && feature.showMergeBanner && enabled && mergeOutputFiles) return true;
        // ... more conditions
        return false;
    }
</script>

<!-- Function called directly in template -->
{#if hasFeatureMessages()}
    <div class="feature-message-card">
        <!-- Messages content -->
    </div>
{/if}
```

### Why It Failed

1. **Svelte's Compilation Process**: When Svelte compiles templates with function calls, it must:
   - Inject dependency tracking code
   - Determine when to re-execute the function
   - Track all reactive dependencies used inside the function

2. **Build Tool Interference**: Different build tools (Wails 2.9 vs 2.10) apply different optimizations:
   - Function inlining
   - Dead code elimination
   - Minification strategies
   - These optimizations can break Svelte's injected dependency tracking

3. **No Reactivity Guarantee**: Svelte 4 only guarantees reactivity for:
   - Reactive statements (`$:`)
   - Direct variable references in templates
   - NOT for function calls in templates

## The Solution

**After (Works in all builds):**
```svelte
<script>
    // Separate function for non-merge messages
    function hasNonMergeMessages() {
        if (enabled && $invalidationErrorStore.some(...)) return true;
        // ... other conditions
        return false;
    }
    
    // Direct reactive variable for merge message
    $: shouldShowMergeMessage = feature.outputMergeGroup && feature.showMergeBanner && enabled && mergeOutputFiles;
    
    // Reactive variable combining all checks
    $: hasFeatureMessages = hasNonMergeMessages() || shouldShowMergeMessage;
</script>

<!-- Reactive variable referenced directly -->
{#if hasFeatureMessages}
    <div class="feature-message-card">
        <!-- Messages content -->
    </div>
{/if}
```

### Why This Works

1. **Explicit Dependencies**: Svelte knows exactly when `hasFeatureMessages` needs to update
2. **Build Tool Resilient**: Reactive statements are preserved through all optimizations
3. **Guaranteed Reactivity**: This pattern is part of Svelte's core contract

## Investigation Timeline

### Phase 1: Initial WebView2 Hypothesis
- Assumed WebView2's single-threaded message pump was the issue
- Attempted fixes:
  - Debouncing (up to 800ms) - Failed
  - Double requestAnimationFrame - Failed
  - Force reflow with offsetHeight - Failed
  - Various DOM manipulation tricks - Failed

### Phase 2: Discovering the Pattern
- Noticed the issue also occurred on WebKit with specific builds
- Realized it wasn't engine-specific but build-specific
- Led to examining Svelte's compilation output

### Phase 3: Solution Discovery
- Implemented changes in fragments to isolate the fix
- Fragment 4 (converting to reactive variable) resolved all issues
- Confirmed across all builds and engines

## Key Learnings

### 1. Svelte Best Practices
- **Always use reactive statements** for computed values used in templates
- **Never call functions in templates** if they depend on reactive state
- Function calls in templates are fragile and build-tool dependent

### 2. Debugging Production Issues
- Don't assume engine-specific issues without testing multiple builds
- Build tool versions can dramatically affect Svelte's behavior
- Always test with the exact build pipeline used in production

### 3. Red Herrings
- WebView2's architecture was blamed but wasn't the issue
- The 10ms vs 300ms debouncing debate was irrelevant
- Complex architectural solutions (WebRPC migration) were unnecessary for this specific bug

## Recommendations

1. **Audit all Svelte components** for function calls in templates
2. **Convert to reactive variables** where computed values are used
3. **Establish coding standards** that prohibit function calls in template conditionals
4. **Test with production build tools** during development

## Code Pattern Reference

### ❌ Avoid This Pattern
```svelte
{#if someFunction()}
{#each getFilteredItems() as item}
<div class="{getClassName()}">
```

### ✅ Use This Pattern
```svelte
$: someCondition = someFunction();
$: filteredItems = getFilteredItems();
$: className = getClassName();

{#if someCondition}
{#each filteredItems as item}
<div class={className}>
```

## Conclusion

This bug highlights a fundamental limitation in Svelte 4's reactivity system when combined with modern build tools. While Svelte 5 may address these issues with its new fine-grained reactivity system, Svelte 4 applications must follow strict patterns to ensure reliable behavior across different build configurations.

The issue was not specific to WebView2 or any particular browser engine, but rather a consequence of how Svelte's compiled output interacts with different build optimization strategies. The solution - using reactive variables instead of function calls in templates - should be considered a mandatory pattern for production Svelte applications.