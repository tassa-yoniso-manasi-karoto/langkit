# Svelte with TypeScript: Best Practices Guide

This document outlines specific best practices for developing robust applications with Svelte and TypeScript, covering both Svelte 4 and Svelte 5 patterns. Based on real-world experience addressing performance issues, memory leaks, reactivity challenges, and UI inconsistencies.

> **⚠️ CRITICAL BUILD COMPATIBILITY WARNING**
> 
> Never use function calls in template conditionals like `{#if someFunction()}`. This pattern can work in development but break in production builds due to how different bundlers optimize code. Always use reactive variables instead: `$: condition = someFunction()` then `{#if condition}`.
> 
> This is the #1 cause of "works on my machine" bugs in Svelte applications.

## Paradigm differences between backend and frontend

Early abstraction mindset:
- Backend:
  - "Design your abstractions upfront"
  - Interfaces and contracts are sacred
  - Changing signatures is expensive
  - The compiler enforces your architecture
  - DRY is almost religious

- Frontend:
  - "Feel your way to the right abstraction"
  - Iteration is cheap, visual feedback is immediate
  - The "right" abstraction often only emerges after you see it working
  - Components can create more problems than they solve
  - Sometimes WET (Write Everything Twice) is better than the wrong abstraction


State ownership philosophy:
- Backend: "Who owns this data?" is crystal clear - database, service, cache
- Frontend: State is everywhere - DOM, component state, stores, URL, localStorage. The question becomes "where SHOULD this live?" and the answer is often "it depends"

Error handling mentality:
- Backend: Errors are exceptional. Handle them, log them, fail gracefully
- Frontend: Errors are Tuesday. Users will refresh. The browser will recover. "undefined is not a function" is just part of life

Performance optimization:
- Backend: Algorithm complexity, database queries, caching strategies
- Frontend: "Perceived performance" > actual performance. A 300ms delay with a spinner feels faster than 100ms of frozen UI

Testing confidence:
- Backend: "If tests pass, it works"
- Frontend: "Tests pass, but does it FEEL right?" Automated tests can't catch "this animation feels janky"

Debugging approach:
- Backend: Logs, stack traces, debugger
- Frontend: "Let me add a border: 1px solid red" or console.log literally everywhere

Versioning reality:
- Backend: Deploy version 2.0, deprecate 1.0
- Frontend: Your code runs on Karen's 2015 iPad with iOS 12 that she'll never update

Code permanence:
- Backend: That function you wrote will probably exist for years
- Frontend: That component has a 50% chance of being completely rewritten when design gets bored


## Table of Contents
1. [Preventing Data Races](#1-preventing-data-races)
2. [Avoiding Circular Dependencies](#2-avoiding-circular-dependencies)
3. [Managing Reactivity](#3-managing-reactivity)
4. [Mitigating Memory Leaks](#4-mitigating-memory-leaks)
5. [Optimizing for Performance](#5-optimizing-for-performance)
6. [GUI-Specific Patterns](#6-gui-specific-patterns)
7. [Understanding Prop Reactivity Patterns](#7-understanding-prop-reactivity-patterns)
8. [Architecture Design Principles](#8-architecture-design-principles)
9. [Reactivity Boundaries and Edge Cases](#9-reactivity-boundaries-and-edge-cases)
10. [Svelte 5 Migration Patterns](#10-svelte-5-migration-patterns)

## 1. Preventing Data Races

### DO:
- **✅ Implement version tracking for async operations**
  ```typescript
  // Svelte 4
  let currentProcessId = 0;
  async function processData(data) {
    const processId = ++currentProcessId;
    const result = await fetchData(data);
    if (processId !== currentProcessId) return; // Abandon if newer process started
    updateUI(result);
  }
  
  // Svelte 5
  let currentProcessId = $state(0);
  async function processData(data) {
    const processId = ++currentProcessId;
    const result = await fetchData(data);
    if (processId !== currentProcessId) return;
    updateUI(result);
  }
  ```

- **✅ Use single points of truth for state**
  ```typescript
  // Svelte 4 - Store is source of truth
  import { writable, derived } from 'svelte/store';
  const mainStore = writable({ value: 0 });
  const derivedValue = derived(mainStore, $store => $store.value * 2);
  
  // Svelte 5 - Direct state management
  let mainState = $state({ value: 0 });
  let derivedValue = $derived(mainState.value * 2);
  ```

- **✅ Track readiness state for async initialization**
  ```typescript
  // Svelte 4
  let isInitialized = false;
  let isLoading = true;
  let error: Error | null = null;
  
  // Svelte 5 with better type safety
  type LoadingState = 
    | { status: 'idle' }
    | { status: 'loading' }
    | { status: 'success'; data: unknown }
    | { status: 'error'; error: Error };
    
  let loadingState = $state<LoadingState>({ status: 'idle' });
  ```

- **✅ Use AbortController for cancellable operations**
  ```typescript
  // Enhanced pattern for both Svelte 4 & 5
  class AsyncOperation<T> {
    private controller?: AbortController;
    
    async execute(fn: (signal: AbortSignal) => Promise<T>): Promise<T | null> {
      this.cancel();
      this.controller = new AbortController();
      
      try {
        return await fn(this.controller.signal);
      } catch (error) {
        if (error.name === 'AbortError') return null;
        throw error;
      }
    }
    
    cancel() {
      this.controller?.abort();
    }
  }
  ```

### DON'T:
- **❌ Use multiple uncoordinated timeouts for the same state**
  ```typescript
  // BAD: Creates race conditions
  setTimeout(() => { state = 'A'; }, 50);
  setTimeout(() => { state = 'B'; }, 100);
  ```

- **❌ Modify store values directly within render cycle**
  ```svelte
  <!-- BAD: Direct modification within rendering -->
  <div>
    {#if condition}
      {store.setValue(newValue)}
      {newValue}
    {/if}
  </div>
  ```

- **❌ Assume async operations complete in original order**
  ```typescript
  // BAD: No guarantee of order
  async function loadData() {
    const dataA = await fetchA();
    const dataB = await fetchB();
    // DataB might arrive before dataA is processed
    updateState(dataA, dataB);
  }
  ```

- **❌ Use get(store) for values that need to stay current**
  ```typescript
  // BAD: Only gets value once, not reactive
  const currentValue = get(myStore);
  
  // GOOD: Reactive subscription with $ syntax
  $: reactiveValue = $myStore;
  ```

## 2. Avoiding Circular Dependencies

### DO:
- **✅ Establish clear unidirectional data flow**
  ```typescript
  // Svelte 4 - Traditional approach
  // Parent → Store → Child → Events → Parent
  
  // Svelte 5 - Cleaner with runes
  class FeatureState {
    value = $state(0);
    derived = $derived(this.value * 2);
    
    updateValue(newValue: number) {
      this.value = newValue;
    }
  }
  ```

- **✅ Use flags to guard against circular updates**
  ```typescript
  // Pattern works for both Svelte 4 & 5
  class UpdateGuard {
    private updating = false;
    
    async safeUpdate(fn: () => void | Promise<void>) {
      if (this.updating) return;
      this.updating = true;
      
      try {
        await fn();
      } finally {
        this.updating = false;
      }
    }
  }
  ```

- **✅ Implement proper event delegation**
  ```svelte
  <!-- Svelte 4 & 5 compatible pattern -->
  <div on:click={handleClick}>
    {#each items as item}
      <button data-action="select" data-id={item.id}>
        {item.name}
      </button>
    {/each}
  </div>
  
  <script>
    function handleClick(event) {
      const target = event.target as HTMLElement;
      if (target.dataset.action === 'select') {
        selectItem(target.dataset.id);
      }
    }
  </script>
  ```

### DON'T:
- **❌ Update the store from within store subscriptions**
  ```typescript
  // BAD: Creates update loops
  storeA.subscribe(value => {
    // This can create circular updates
    storeB.update(state => ({ ...state, value }));
  });
  ```

- **❌ Create interdependent reactive statements**
  ```typescript
  // BAD: Circular dependency
  $: valueA = calculate(valueB);
  $: valueB = process(valueA);
  ```

- **❌ Use both bind:value AND on:change on the same element**
  ```svelte
  <!-- BAD: Dual binding pattern -->
  <select 
    bind:value={selectedValue}
    on:change={e => handleChange(e.target.value)}
  >
    <!-- This creates the "dual binding" issue where 
         Svelte updates the value before the change handler runs -->
  </select>
  ```

- **❌ Dispatch the same event type that you're listening for**
  ```svelte
  <!-- BAD: Can create event loops -->
  <ChildComponent
    on:change={handleChange}
    on:click={() => {
      // This can create loops if ChildComponent
      // dispatches 'change' in its click handler
      dispatch('change', newValue);
    }}
  />
  ```

## 3. Managing Reactivity

### DO:
- **✅ Use reactive variables instead of function calls in templates**
  ```typescript
  // ❌ BAD: Function calls in templates are fragile
  function shouldShowContent() {
    return isEnabled && hasPermission && data.length > 0;
  }
  
  // Template
  {#if shouldShowContent()}  // Fragile across builds!
    <div>Content</div>
  {/if}
  
  // ✅ GOOD: Reactive variable
  $: shouldShowContent = isEnabled && hasPermission && data.length > 0;
  
  // Template
  {#if shouldShowContent}  // Guaranteed reactivity
    <div>Content</div>
  {/if}
  ```
  
  **Why this matters:**
  - Function calls in templates rely on Svelte's ability to track dependencies inside the function
  - This tracking can break with different build tools, minification, or optimization settings
  - Different versions of bundlers (Wails 2.9 vs 2.10) can produce different behavior
  - Reactive statements (`$:`) are guaranteed to work across all build configurations
  - This is especially critical for production builds where aggressive optimization occurs

- **✅ Understand Svelte's reactivity boundaries**
  ```typescript
  // Svelte 4 - Compile-time reactivity
  let obj = { count: 0, nested: { value: 1 } };
  
  // ✅ Tracked (assignment)
  obj = { ...obj, count: 1 };
  obj.count = 1; // Top-level property only!
  
  // ❌ Not tracked (nested mutation)
  obj.nested.value = 2;
  
  // Svelte 5 - Runtime reactivity with Proxies
  let obj = $state({ count: 0, nested: { value: 1 } });
  
  // ✅ All tracked!
  obj.count = 1;
  obj.nested.value = 2;
  obj.nested.newProp = 3;
  ```

- **✅ Use proper patterns for imported values**
  ```typescript
  // Svelte 4 - Imports aren't reactive
  import { features } from './config';
  
  // ❌ Won't update
  $: selected = features.find(f => f.active);
  
  // ✅ Solution 1: Use stores
  import { featuresStore } from './config';
  $: selected = $featuresStore.find(f => f.active);
  
  // ✅ Solution 2: Local reactive copy
  let features = [...importedFeatures];
  
  // ✅ Solution 3: Force update
  features.push(newItem);
  features = features;
  
  // Svelte 5 - Use $state in config
  // config.js
  export const features = $state([...]);
  
  // component.svelte
  import { features } from './config';
  // Now it's reactive!
  ```

- **✅ Handle array/object mutations correctly**
  ```typescript
  // Svelte 4 patterns
  // Arrays
  items = [...items, newItem]; // ✅ Reassignment
  items = items.filter(i => i.id !== id); // ✅
  items.push(newItem); items = items; // ✅ Force update
  
  // Objects  
  user = { ...user, name: 'New' }; // ✅
  Object.assign(user, updates); user = user; // ✅
  
  // Svelte 5 - Just mutate!
  let items = $state([]);
  items.push(newItem); // ✅ Automatically reactive
  
  let user = $state({ name: '' });
  user.name = 'New'; // ✅ Automatically reactive
  ```

### DON'T:
- **❌ Call functions directly in template conditionals**
  ```svelte
  <!-- BAD: This can break in production builds -->
  {#if hasMessages()}
    <div>Messages</div>
  {/if}
  
  <!-- BAD: Even with simple conditions -->
  {#if isValid() && canProceed()}
    <button>Continue</button>
  {/if}
  
  <!-- GOOD: Use reactive variables -->
  <script>
    $: hasMessages = checkMessages();
    $: canContinue = isValid() && canProceed();
  </script>
  
  {#if hasMessages}
    <div>Messages</div>
  {/if}
  
  {#if canContinue}
    <button>Continue</button>
  {/if}
  ```

- **❌ Mix reactive and imperative updates**
- **❌ Rely on object mutation to trigger updates (Svelte 4)**
- **❌ Use get(store) within reactive statements**
- **❌ Create side effects in reactive declarations without guards**
  ```typescript
  // BAD: Side effect without guard
  $: {
    console.log(`Value changed to ${value}`);
    localStorage.setItem('value', value);
  }
  
  // GOOD: With guard
  $: if (value !== prevValue) {
    prevValue = value;
    console.log(`Value changed to ${value}`);
    localStorage.setItem('value', value);
  }
  ```

## 4. Mitigating Memory Leaks

### DO:
- **✅ Create a comprehensive cleanup registry**
  ```typescript
  // Works for both Svelte 4 & 5
  class CleanupManager {
    private cleanups: Array<() => void> = [];
    
    register(cleanup: () => void): void {
      this.cleanups.push(cleanup);
    }
    
    registerTimeout(id: ReturnType<typeof setTimeout>): void {
      this.register(() => clearTimeout(id));
    }
    
    registerInterval(id: ReturnType<typeof setInterval>): void {
      this.register(() => clearInterval(id));
    }
    
    registerAbortController(controller: AbortController): void {
      this.register(() => controller.abort());
    }
    
    cleanup(): void {
      this.cleanups.forEach(fn => fn());
      this.cleanups = [];
    }
  }
  
  // Usage
  const cleanup = new CleanupManager();
  
  onMount(() => {
    const id = setInterval(() => {}, 1000);
    cleanup.registerInterval(id);
    
    const controller = new AbortController();
    cleanup.registerAbortController(controller);
  });
  
  onDestroy(() => cleanup.cleanup());
  ```

- **✅ Use WeakMap/WeakSet for DOM references**
  ```typescript
  // Prevents memory leaks with DOM elements
  const elementMetadata = new WeakMap<HTMLElement, Metadata>();
  const observedElements = new WeakSet<HTMLElement>();
  
  function trackElement(element: HTMLElement, data: Metadata) {
    elementMetadata.set(element, data);
    observedElements.add(element);
    // No cleanup needed - garbage collected automatically
  }
  ```

### DON'T:
- **❌ Create DOM elements just for side effects**
- **❌ Forget to remove event listeners**
- **❌ Keep references to DOM elements after destruction**
- **❌ Use closures that capture component state without cleanup**

## 5. Optimizing for Performance

### DO:
- **✅ Use CSS containment for complex components**
  ```svelte
  <div class="complex-component">
    <!-- Content -->
  </div>
  
  <style>
    .complex-component {
      contain: layout style paint;
      content-visibility: auto;
    }
  </style>
  ```

- **✅ Implement virtual scrolling for large lists**
  ```typescript
  // Svelte 5 pattern with signals
  class VirtualList<T> {
    items = $state<T[]>([]);
    scrollTop = $state(0);
    itemHeight = 50;
    containerHeight = 600;
    
    visibleItems = $derived(() => {
      const start = Math.floor(this.scrollTop / this.itemHeight);
      const end = start + Math.ceil(this.containerHeight / this.itemHeight);
      return this.items.slice(start, end + 1).map((item, i) => ({
        item,
        y: (start + i) * this.itemHeight
      }));
    });
  }
  ```

- **✅ Use intersection observer for lazy loading**
  ```typescript
  // Reusable action for both Svelte 4 & 5
  export function lazyLoad(node: HTMLElement, callback: () => void) {
    const observer = new IntersectionObserver(entries => {
      if (entries[0].isIntersecting) {
        callback();
        observer.disconnect();
      }
    });
    
    observer.observe(node);
    
    return {
      destroy() {
        observer.disconnect();
      }
    };
  }
  ```

### DON'T:
- **❌ Subscribe to entire store when only specific values needed**
- **❌ Create new closures in render loops**
  ```svelte
  <!-- BAD: Creates new function on every render -->
  {#each items as item}
    <button on:click={() => handleClick(item.id)}>
      {item.name}
    </button>
  {/each}
  
  <!-- GOOD: Use event delegation or bind -->
  {#each items as item}
    <button on:click={handleClick} data-id={item.id}>
      {item.name}
    </button>
  {/each}
  ```

- **❌ Perform DOM measurements in render loops**
- **❌ Use synchronous operations in animation callbacks**

## 6. GUI-Specific Patterns

### DO:
- **✅ Implement proper focus management with TypeScript**
  ```typescript
  interface FocusContext {
    trapFocus(container: HTMLElement): void;
    releaseFocus(): void;
  }
  
  class FocusManager implements FocusContext {
    private stack: HTMLElement[] = [];
    private previousFocus: HTMLElement | null = null;
    
    trapFocus(container: HTMLElement): void {
      this.previousFocus = document.activeElement as HTMLElement;
      this.stack.push(container);
      
      const focusableElements = container.querySelectorAll<HTMLElement>(
        'a[href], button, textarea, input, select, [tabindex]:not([tabindex="-1"])'
      );
      
      const firstElement = focusableElements[0];
      const lastElement = focusableElements[focusableElements.length - 1];
      
      container.addEventListener('keydown', (e: KeyboardEvent) => {
        if (e.key !== 'Tab') return;
        
        if (e.shiftKey && document.activeElement === firstElement) {
          e.preventDefault();
          lastElement?.focus();
        } else if (!e.shiftKey && document.activeElement === lastElement) {
          e.preventDefault();
          firstElement?.focus();
        }
      });
      
      firstElement?.focus();
    }
    
    releaseFocus(): void {
      this.stack.pop();
      this.previousFocus?.focus();
    }
  }
  ```

- **✅ Create type-safe error boundaries**
  ```typescript
  // Svelte 5 with error boundaries
  interface ErrorBoundaryState {
    error: Error | null;
    errorInfo: { componentStack: string } | null;
  }
  
  class ErrorBoundary {
    state = $state<ErrorBoundaryState>({ error: null, errorInfo: null });
    
    static getDerivedStateFromError(error: Error): Partial<ErrorBoundaryState> {
      return { error };
    }
    
    componentDidCatch(error: Error, errorInfo: { componentStack: string }) {
      console.error('Error caught by boundary:', error, errorInfo);
      this.state.errorInfo = errorInfo;
    }
    
    reset() {
      this.state = { error: null, errorInfo: null };
    }
  }
  ```

### DON'T:
- **❌ Mix form validation with UI state**
- **❌ Create deep component hierarchies with prop drilling**
- **❌ Overuse modals without proper management**
- **❌ Directly manipulate DOM in response to prop changes**

## 7. Understanding Prop Reactivity Patterns

### DO:
- **✅ Use explicit reactivity contracts**
  ```typescript
  // Svelte 4 - Make reactivity explicit
  interface ReactiveProps {
    value: number;
    onChange?: (value: number) => void;
    // Explicit reactive binding
    bindTo?: { subscribe: Readable<number>['subscribe'] };
  }
  
  // Svelte 5 - Props are reactive by default
  interface Props {
    value: number;
    onChange?: (value: number) => void;
  }
  
  let { value, onChange }: Props = $props();
  ```

- **✅ Handle prop updates appropriately per version**
  ```svelte
  <!-- Svelte 4: Props don't auto-update deeply -->
  <script>
    export let config;
    
    // Won't update when parent's config.theme changes
    $: theme = config.theme;
    
    // Solution: Use key block
  </script>
  
  <!-- Svelte 5: Props are reactive -->
  <script>
    let { config } = $props();
    // Automatically updates!
    $: theme = config.theme;
  </script>
  ```

### DON'T:
- **❌ Assume props automatically update in Svelte 4**
- **❌ Overuse key directives without performance consideration**
- **❌ Create component update cycles with bidirectional bindings**

## 8. Architecture Design Principles

### State Management Architecture

#### DO:
- **✅ Design type-safe stores with clear interfaces**
  ```typescript
  // Svelte 4 - Type-safe store factory
  interface StoreFactory {
    <T>(initial: T, validator?: (value: T) => boolean): Writable<T> & {
      reset(): void;
      validate(): boolean;
    };
  }
  
  // Svelte 5 - Class-based state with built-in reactivity
  class DomainStore<T> {
    private _value = $state<T>();
    private _history = $state<T[]>([]);
    
    get value() { return this._value; }
    set value(v: T) {
      this._history.push(this._value);
      this._value = v;
    }
    
    undo() {
      const prev = this._history.pop();
      if (prev !== undefined) this._value = prev;
    }
  }
  ```

- **✅ Implement proper state machines**
  ```typescript
  // Type-safe state machine pattern
  type State = 'idle' | 'loading' | 'success' | 'error';
  type Event = 
    | { type: 'FETCH' }
    | { type: 'SUCCESS'; data: unknown }
    | { type: 'ERROR'; error: Error };
    
  class StateMachine {
    state = $state<State>('idle');
    
    transition(event: Event) {
      switch (this.state) {
        case 'idle':
          if (event.type === 'FETCH') this.state = 'loading';
          break;
        case 'loading':
          if (event.type === 'SUCCESS') this.state = 'success';
          if (event.type === 'ERROR') this.state = 'error';
          break;
      }
    }
  }
  ```

### Component Organization

#### DO:
- **✅ Create clear component contracts with TypeScript**
  ```typescript
  // Svelte 5 - Comprehensive component interface
  interface ComponentProps {
    // Required props
    id: string;
    value: number;
    
    // Optional with defaults
    label?: string;
    disabled?: boolean;
    
    // Callbacks
    onChange?: (value: number) => void;
    onFocus?: (event: FocusEvent) => void;
    
    // Slots
    children?: Snippet;
    header?: Snippet<[{ title: string }]>;
  }
  
  // Usage in component
  let {
    id,
    value,
    label = 'Default Label',
    disabled = false,
    onChange,
    onFocus,
    children,
    header
  }: ComponentProps = $props();
  ```

## 9. Reactivity Boundaries and Edge Cases

### Understanding Reactivity Boundaries

**Definition**: A reactivity boundary is the scope within which Svelte tracks and responds to state changes. Understanding these boundaries is crucial for predictable behavior.

### Compile-Time vs Runtime Boundaries (Svelte 4 vs 5)

```typescript
// Svelte 4 - Compile-time boundaries
function createCounter() {
  let count = 0; // Not reactive outside component!
  return {
    increment: () => count++,
    get value() { return count; }
  };
}

// Svelte 5 - Runtime boundaries with runes
function createCounter() {
  let count = $state(0); // Reactive anywhere!
  return {
    increment: () => count++,
    get value() { return count; }
  };
}
```

### Module vs Instance Boundaries

```svelte
<script context="module">
  // Svelte 4 & 5 - Module context
  // Shared across ALL component instances
  // NOT reactive - runs once at module load
  let sharedState = 0; // Changes won't trigger updates
  
  // Svelte 5 - Can use $state.frozen for shared immutable
  const sharedConfig = $state.frozen({
    apiUrl: 'https://api.example.com'
  });
</script>

<script>
  // Instance context - reactive
  let instanceState = 0; // Svelte 4
  let instanceState = $state(0); // Svelte 5
</script>
```

### Store Subscription Boundaries

```typescript
// Svelte 4 - Subscription boundary gotchas
import { writable, get } from 'svelte/store';

const store = writable(0);

// ❌ Outside reactive context - won't update
const value = get(store); 

// ✅ Inside reactive context
$: reactiveValue = $store;

// ⚠️ Manual subscription - must clean up!
const unsubscribe = store.subscribe(value => {
  console.log(value);
});
onDestroy(unsubscribe);
```

### Component Lifecycle Boundaries

```typescript
// Lifecycle functions must be called during component initialization
onMount(() => {
  // ✅ Valid
});

// ❌ Invalid - conditional lifecycle
if (condition) {
  onMount(() => {}); // Error!
}

// ✅ Conditional logic inside lifecycle
onMount(() => {
  if (condition) {
    // Do something
  }
});
```

### Async Operation Boundaries

```typescript
// Svelte 4 & 5 - Async reactivity boundaries
let data = null;

// ❌ Async in reactive statement creates issues
$: loadData(id); // Returns promise, not data

// ✅ Proper async handling
$: if (id) {
  loadData(id).then(result => {
    data = result; // Trigger update after async
  });
}

// Svelte 5 - Better with effects
$effect(() => {
  loadData(id).then(result => {
    data = result;
  });
});
```

### Cross-Component Boundaries

```typescript
// Parent-Child boundary patterns

// Svelte 4 - Props don't cross boundaries reactively
// Parent
let config = { theme: 'dark' };
config.theme = 'light'; // Child won't see this change!

// Solution: Reassign or use stores
config = { ...config, theme: 'light' };

// Svelte 5 - Props are reactive across boundaries
let config = $state({ theme: 'dark' });
config.theme = 'light'; // Child updates automatically!
```

### Event Handler Boundaries

```typescript
// Event handlers create closure boundaries
let count = $state(0);

function createHandler() {
  // Captures current value of count
  return () => {
    console.log(count); // Always logs value at creation time
  };
}

// Better: Access current value in handler
function handleClick() {
  console.log(count); // Always current value
}
```

### Build Tool Boundaries

**Critical**: Different build tools and optimization levels can create different reactivity behavior!

```typescript
// ❌ DANGEROUS: Works in dev, breaks in production
function shouldRender() {
  return someCondition && otherState > 0;
}

{#if shouldRender()}  // May lose reactivity after minification!
  <Component />
{/if}

// ✅ SAFE: Guaranteed to work across all builds
$: shouldRender = someCondition && otherState > 0;

{#if shouldRender}  // Always reactive
  <Component />
{/if}
```

**Why this happens:**
- Development builds preserve more debugging information
- Production builds aggressively optimize and minify code
- Function inlining can break Svelte's dependency tracking
- Different bundler versions (e.g., Wails 2.9 vs 2.10) have different optimization strategies
- WebKit and WebView2 may execute optimized code differently

**Best Practice**: Always use reactive statements (`$:`) for any computed values used in templates

## 10. Svelte 5 Migration Patterns

### Core Migration Strategies

#### State Migration
```typescript
// Svelte 4
let count = 0;
let items = [];
let user = { name: '', email: '' };

// Svelte 5
let count = $state(0);
let items = $state<Item[]>([]);
let user = $state({ name: '', email: '' });
```

#### Reactive Statements to Derivations
```typescript
// Svelte 4
$: doubled = count * 2;
$: filtered = items.filter(item => item.active);
$: fullName = `${user.firstName} ${user.lastName}`;

// Svelte 5
let doubled = $derived(count * 2);
let filtered = $derived(items.filter(item => item.active));
let fullName = $derived(`${user.firstName} ${user.lastName}`);
```

#### Side Effects Migration
```typescript
// Svelte 4
$: if (count > 10) {
  console.log('Count exceeded 10');
  localStorage.setItem('highCount', count);
}

// Svelte 5
$effect(() => {
  if (count > 10) {
    console.log('Count exceeded 10');
    localStorage.setItem('highCount', count);
  }
});
```

#### Props Migration
```typescript
// Svelte 4
export let value;
export let optional = 'default';

// Svelte 5
let { value, optional = 'default' } = $props<{
  value: string;
  optional?: string;
}>();
```

#### Store Migration Strategy
```typescript
// Gradual migration approach
// Step 1: Keep stores for shared state
import { userStore } from './stores';

// Step 2: Create parallel rune-based state
class UserState {
  user = $state<User | null>(null);
  loading = $state(false);
  
  async login(credentials: Credentials) {
    this.loading = true;
    try {
      this.user = await api.login(credentials);
    } finally {
      this.loading = false;
    }
  }
}

// Step 3: Gradually migrate components
// Can use both patterns during migration
```

### Advanced Migration Patterns

#### Custom Store to Rune Pattern
```typescript
// Svelte 4 custom store
function createCounter() {
  const { subscribe, set, update } = writable(0);
  
  return {
    subscribe,
    increment: () => update(n => n + 1),
    reset: () => set(0)
  };
}

// Svelte 5 equivalent
class Counter {
  value = $state(0);
  
  increment() {
    this.value++;
  }
  
  reset() {
    this.value = 0;
  }
}
```

#### Context Migration
```typescript
// Svelte 4
import { setContext, getContext } from 'svelte';

// Parent
setContext('user', userData);

// Child
const user = getContext('user');

// Svelte 5 - Same API, but can use with runes
// Parent
const userState = new UserState();
setContext('user', userState);

// Child
const userState = getContext<UserState>('user');
// Now userState.user is reactive!
```

### Performance Considerations

#### When to Migrate

**Migrate to Svelte 5 when:**
- You need deep reactivity without boilerplate
- Working with complex nested state
- Building new features from scratch
- Performance isn't ultra-critical (games, animations)

**Stay with Svelte 4 when:**
- Application is stable and working well
- You have extensive test coverage for current behavior
- Performance is absolutely critical
- Team needs time to learn new patterns

#### Hybrid Approach
```typescript
// You can use both patterns during migration
// Svelte 4 stores for shared state
import { userStore } from './stores';

// Svelte 5 runes for new component state
let localState = $state({ 
  isEditing: false,
  tempValue: ''
});

// They can work together
$: currentUser = $userStore;
$effect(() => {
  if (currentUser) {
    localState.tempValue = currentUser.name;
  }
});
```

## Conclusion

This guide represents battle-tested patterns for building robust Svelte applications. Key takeaways:

1. **Always use reactive statements for template conditions** - Never call functions directly in `{#if}` blocks
2. **Understand reactivity boundaries** - Know what Svelte can and cannot track
3. **Choose the right pattern for your Svelte version** - Svelte 4 and 5 have different strengths
4. **Prioritize explicit over implicit** - Clear data flow prevents bugs
5. **Design for maintainability** - Consistent patterns trump clever optimizations
6. **Test with production builds** - Development behavior doesn't guarantee production behavior
7. **Measure before optimizing** - Not all performance optimizations are necessary

**Most Critical Rule**: If a value is used in a template (especially in conditionals), it MUST be a reactive variable (`$: value = ...`), never a function call. This is the single most important pattern for cross-platform, cross-build-tool compatibility.

