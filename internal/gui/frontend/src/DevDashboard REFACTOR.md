# DevDashboard Refactoring Plan - PHASED APPROACH

## Directory Structure:
```
internal/gui/frontend/src/
├── components/dev/
│   ├── WASMDashboard.svelte (moved from WasmPerformanceDashboard)
│   ├── StateDebugDashboard.svelte
│   ├── LogsDebugDashboard.svelte
│   ├── DebugControlsDashboard.svelte
│   ├── StyleDebugDashboard.svelte
│   ├── DraggableContainer.svelte
│   └── style/
│       ├── MainStyleControls.svelte
│       ├── BackgroundGradientControls.svelte
│       ├── GlowEffectControls.svelte
│       ├── WelcomeStyleControls.svelte
│       ├── ProgressWaveControls.svelte
│       └── CoffeeMugControls.svelte
└── lib/
    └── dev/
        ├── debugStateControls.ts
        └── styleControlsDefaults.ts
```

## PHASE 1 - Very Safe (File moves & pure utilities)
**Risk: Minimal - Just moving files and extracting pure functions**

1. Create directories:
   - `internal/gui/frontend/src/components/dev/`
   - `internal/gui/frontend/src/lib/dev/`

2. Move and rename `WasmPerformanceDashboard.svelte` → `dev/WASMDashboard.svelte`
   - Update import in DevDashboard.svelte

3. Extract pure utility functions to `lib/dev/debugStateControls.ts`:
   - Move all the force/reset functions (forceLLMState, resetLLMState, etc.)
   - These are standalone functions that just call stores
   - Import them back into DevDashboard

4. Extract style defaults to `lib/dev/styleControlsDefaults.ts`:
   - Move defaultValues, defaultProgressWaveValues objects
   - Import them back

**Verification**: App should work exactly as before

## PHASE 2 - Safe (Self-contained component extraction)
**Risk: Low - Extracting well-isolated functionality**

1. Extract `DraggableContainer.svelte`:
   - Move all dragging logic (handleMouseDown, handleMouseMove, handleMouseUp)
   - Props: `position`, `isDragging`, `onPositionChange`
   - Wrap the icon bubble and expanded panel with this component

2. Extract `MemoryTestButton.svelte` import verification
   - Ensure it's properly imported from its current location

**Verification**: Dragging should work exactly as before

## PHASE 3 - Moderate (Simple tab extractions)
**Risk: Moderate - Moving larger chunks but keeping exact logic**

1. Extract `LogsDebugDashboard.svelte`:
   - Move the entire Logs tab content
   - Pass necessary props: enableTraceLogsStore, enableFrontendLoggingStore, displayFrontendLogsStore
   - Keep all functionality identical

2. Extract `StateDebugDashboard.svelte`:
   - Move the entire State tab content
   - Pass all necessary state variables as props
   - No logic changes, just moving JSX

**Verification**: Logs and State tabs work identically

## PHASE 4 - Moderate-High (Complex tab extractions)
**Risk: Moderate-High - More complex state interactions**

1. Extract `DebugControlsDashboard.svelte`:
   - Move entire Debug tab content
   - Pass all force/reset functions as props
   - Pass all current state values

2. Begin extracting `StyleDebugDashboard.svelte`:
   - Move the main Style tab container with sub-tabs
   - Keep all slider logic in place initially
   - Pass styleControls and all handlers as props

**Verification**: Debug controls and basic style tab navigation work

## PHASE 5 - High (Style sub-components)
**Risk: High - Complex state management for styles**

1. Extract style sub-components one by one:
   - `MainStyleControls.svelte`
   - `BackgroundGradientControls.svelte`
   - `GlowEffectControls.svelte`
   - `WelcomeStyleControls.svelte`
   - `ProgressWaveControls.svelte`
   - `CoffeeMugControls.svelte`

2. Each component receives relevant slice of styleControls and update handlers

**Verification**: All style controls still update CSS variables correctly

## PHASE 6 - Highest (Optimization - OPTIONAL)
**Risk: Highest - Code simplification and optimization**

1. Simplify DevDashboard.svelte:
   - Remove redundant code
   - Optimize store subscriptions
   - Consolidate similar patterns

2. Add lazy loading for tabs if beneficial

**Note**: This phase is optional and can be skipped if earlier phases work well

### Key Safety Measures:
- **No logic changes** in phases 1-4, only moving code
- **Test after each phase** before proceeding
- **Keep exact same variable names** initially
- **Preserve all comments** especially the important warnings about sliders
- **Git commit after each phase** for easy rollback
- **Run the app** after each major change to verify functionality