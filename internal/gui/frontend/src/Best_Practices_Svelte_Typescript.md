# Svelte with TypeScript: Best Practices Guide

This document outlines specific best practices for developing robust applications with Svelte and TypeScript, based on real-world experience addressing performance issues, memory leaks, reactivity challenges, and UI inconsistencies.

## 1. Preventing Data Races

### DO:
- **✅ Implement version tracking for async operations**
  ```typescript
  let currentProcessId = 0;
  async function processData(data) {
    const processId = ++currentProcessId;
    // Async processing...
    if (processId !== currentProcessId) return; // Abandon if newer process started
    // Continue with updates...
  }
  ```

- **✅ Use single points of truth for state**
  ```typescript
  // Store is source of truth, UI reads from store
  const value = featureGroupStore.getGroupOption(groupId, optionId);
  // User actions update store, not local state directly
  function handleChange(newValue) {
    featureGroupStore.setGroupOption(groupId, optionId, newValue);
  }
  ```

- **✅ Track readiness state for async initialization**
  ```typescript
  let isInitialized = false;
  let isLoading = true;
  let error = null;
  
  async function initialize() {
    isLoading = true;
    error = null;
    try {
      await loadData();
      isInitialized = true;
    } catch (e) {
      error = e;
    } finally {
      isLoading = false;
    }
  }
  ```

- **✅ Validate current state before applying updates**
  ```typescript
  function applyChanges(newValue) {
    const currentState = get(store);
    if (currentState.version !== expectedVersion) return;
    // Apply changes...
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
  // Store → Component → User action → Store
  const options = derived(store, $store => $store.options);
  function handleUserInput(value) {
    dispatch('update', { value });
  }
  ```

- **✅ Use flags to guard against circular updates**
  ```typescript
  let updatingFromStore = false;
  
  // In store subscription
  storeValue.subscribe(newValue => {
    updatingFromStore = true;
    localValue = newValue;
    setTimeout(() => { updatingFromStore = false; }, 0);
  });
  
  // In user input handler
  function handleInput(value) {
    if (updatingFromStore) return; // Skip if from store
    dispatch('update', { value });
  }
  ```

- **✅ Choose either binding OR events, not both**
  ```svelte
  <!-- GOOD: Event-based approach -->
  <select value={value} on:change={e => handleSelect(e.target.value)}>
    {#each options as option}
      <option value={option.value}>{option.label}</option>
    {/each}
  </select>
  
  <!-- OR: Binding approach -->
  <select bind:value>
    {#each options as option}
      <option value={option.value}>{option.label}</option>
    {/each}
  </select>
  ```

- **✅ Implement safe and explicit event handlers**
  ```typescript
  function handleChange(e) {
    const newValue = e.target.value;
    // Record old value before change
    const oldValue = currentValue;
    currentValue = newValue;
    
    // Only dispatch if actually changed
    if (oldValue !== newValue) {
      dispatch('change', { oldValue, newValue });
    }
  }
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
- **✅ Prefer $store syntax for reactive store values**
  ```svelte
  <script>
    import { myStore } from './stores';
    
    // Reactively updates when store changes
    $: doubledValue = $myStore * 2;
  </script>
  
  <div>{doubledValue}</div>
  ```

- **✅ Use proper reactive assignments for nested objects**
  ```typescript
  // To trigger reactivity when modifying nested properties
  function updateNestedProperty(id, value) {
    items = items.map(item => 
      item.id === id 
        ? { ...item, value } 
        : item
    );
  }
  ```

- **✅ Make component state changes explicit**
  ```typescript
  function handleModelChange(modelName) {
    // Explicitly update related state
    selectedModel = modelName;
    
    // Force error message refresh
    errorCheckCounter++;
    
    // Trigger side effects in a clear way
    validateProviderApiKey(modelName);
  }
  ```

- **✅ Assign to tracked variables to trigger reactivity**
  ```typescript
  // Changing property won't trigger reactivity
  function badUpdate() {
    obj.count += 1; // Won't trigger reactivity!
  }
  
  // Reassign whole object to trigger reactivity
  function goodUpdate() {
    obj = { ...obj, count: obj.count + 1 };
  }
  ```

### DON'T:
- **❌ Mix reactive and imperative updates**
  ```typescript
  // BAD: Mixing styles
  $: total = items.reduce((sum, item) => sum + item.value, 0);
  
  function addItem(item) {
    items.push(item); // Imperative update won't trigger reactivity
    // Should be: items = [...items, item];
  }
  ```

- **❌ Rely on object mutation to trigger updates**
  ```typescript
  // BAD: Mutation
  function updateCounter() {
    state.counter++; // Won't trigger reactivity
  }
  
  // GOOD: Assignment
  function updateCounter() {
    state = { ...state, counter: state.counter + 1 };
  }
  ```

- **❌ Use get(store) within reactive statements**
  ```typescript
  // BAD: Not reactive
  $: result = calculate(get(myStore));
  
  // GOOD: Reactive
  $: result = calculate($myStore);
  ```

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
- **✅ Clean up ALL subscriptions in onDestroy**
  ```typescript
  let unsubscribe;
  
  onMount(() => {
    unsubscribe = store.subscribe(value => {
      // Handle update
    });
  });
  
  onDestroy(() => {
    if (unsubscribe) unsubscribe();
  });
  ```

- **✅ Track and clear ALL timeouts**
  ```typescript
  let timeoutId = null;
  
  function delayedUpdate() {
    // Clear existing timeout
    if (timeoutId !== null) clearTimeout(timeoutId);
    
    timeoutId = setTimeout(() => {
      // Update logic
      timeoutId = null;
    }, 100);
  }
  
  onDestroy(() => {
    if (timeoutId !== null) clearTimeout(timeoutId);
  });
  ```

- **✅ Use a cleanup registry for multiple resources**
  ```typescript
  const cleanupFunctions = [];
  
  function registerCleanup(fn) {
    cleanupFunctions.push(fn);
  }
  
  onMount(() => {
    const intervalId = setInterval(() => {
      // Periodically check something
    }, 1000);
    
    registerCleanup(() => clearInterval(intervalId));
    
    // Register more cleanup functions as needed
  });
  
  onDestroy(() => {
    // Clean up all registered resources
    cleanupFunctions.forEach(cleanup => cleanup());
  });
  ```

- **✅ Use action functions for DOM-related cleanup**
  ```typescript
  // Create a reusable action
  function addResizeObserver(node, callback) {
    const observer = new ResizeObserver(callback);
    observer.observe(node);
    
    return {
      destroy() {
        observer.disconnect();
      }
    };
  }
  
  // Use in component
  <div use:addResizeObserver={handleResize}>
    Resizable content
  </div>
  ```

### DON'T:
- **❌ Create DOM elements just for side effects**
  ```svelte
  <!-- BAD: Creates DOM elements on every render -->
  <div style="display:none;">
    {console.log('Debug info:', someExpensiveCalculation())}
  </div>
  ```

- **❌ Forget to remove event listeners in animations**
  ```typescript
  // BAD: Missing cleanup
  function animateElement() {
    element.addEventListener('transitionend', () => {
      // Transition complete
    });
    element.style.opacity = 0;
  }
  ```

- **❌ Keep references to DOM elements after component destruction**
  ```typescript
  // BAD: Global reference that outlives component
  let elementRefs = [];
  
  onMount(() => {
    // Adding to global array
    elementRefs.push(myElement);
  });
  
  // No cleanup to remove reference when component is destroyed
  // Should add: onDestroy(() => { elementRefs = elementRefs.filter(el => el !== myElement); })
  ```

- **❌ Use closure variables that capture component state without cleanup**
  ```typescript
  // BAD: Closure captures component state
  const handleExternalEvent = (event) => {
    // References component state
    updateComponentState(event.detail);
  };
  
  onMount(() => {
    window.addEventListener('external-event', handleExternalEvent);
    // Missing cleanup
  });
  ```

## 5. Optimizing for Performance

### DO:

- **✅ Batch store updates**
  ```typescript
  // Better than multiple individual updates
  function updateRelatedOptions(values) {
    store.update(state => {
      const newState = { ...state };
      // Apply all updates at once
      Object.entries(values).forEach(([key, value]) => {
        newState[key] = value;
      });
      return newState;
    });
  }
  ```

- **✅ Use dynamic display to avoid unnecessary rendering**
  ```svelte
  <!-- Only render expensive component when needed -->
  {#if showComponent}
    <ExpensiveComponent />
  {/if}
  ```

- **✅ Optimize renders during window minimization**
  ```typescript
  // Check window state
  async function checkWindowState() {
    const minimized = await WindowIsMinimised();
    
    if (minimized !== isWindowMinimized) {
      isWindowMinimized = minimized;
      
      // Reduce animations and processing if minimized
      if (minimized) {
        showGlow = false;
        updateInterval = 100; // 10fps
      } else {
        showGlow = settings.enableGlow;
        updateInterval = 16; // 60fps
      }
    }
  }
  
  // Set up interval check
  onMount(() => {
    windowCheckInterval = setInterval(checkWindowState, 2000);
  });
  
  onDestroy(() => {
    clearInterval(windowCheckInterval);
  });
  ```

- **✅ Use requestAnimationFrame for smooth visual updates**
  ```typescript
  function updateVisuals() {
    // Schedule visual update for next frame
    requestAnimationFrame(() => {
      // Update DOM
      element.style.transform = `translateX(${position}px)`;
      
      // Continue animation if needed
      if (animating) {
        updateVisuals();
      }
    });
  }
  ```

### DON'T:
- **❌ Subscribe to the entire store when only specific values are needed**
  ```typescript
  // BAD: Processing all store changes
  store.subscribe(state => {
    // Component only needs state.user but processes all changes
  });
  
  // GOOD: Use derived stores
  const userData = derived(store, $store => $store.user);
  userData.subscribe(user => {
    // Only processes user changes
  });
  ```

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
  ```typescript
  // BAD: Causes layout thrashing
  function updateLayout() {
    items.forEach(item => {
      // Forced layout recalculation on each iteration
      const width = item.element.offsetWidth;
      item.element.style.height = `${width / 2}px`;
    });
  }
  
  // GOOD: Batch reads, then writes
  function updateLayout() {
    // Read phase
    const measurements = items.map(item => ({
      element: item.element,
      width: item.element.offsetWidth
    }));
    
    // Write phase
    measurements.forEach(item => {
      item.element.style.height = `${item.width / 2}px`;
    });
  }
  ```

- **❌ Use synchronous operations in animation/transition callbacks**
  ```typescript
  // BAD: Potentially blocking main thread during animation
  function onTransitionEnd() {
    // Expensive synchronous operation
    const result = heavyComputation();
    updateUI(result);
  }
  
  // GOOD: Defer heavy work
  function onTransitionEnd() {
    // Schedule work for next idle period
    setTimeout(() => {
      const result = heavyComputation();
      updateUI(result);
    }, 0);
  }
  ```

## 6. GUI-Specific Patterns

### DO:
- **✅ Implement proper error visualization hierarchies**
  ```typescript
  // Define error severity and display priority
  const ERROR_PRIORITY = {
    critical: 0,
    warning: 1,
    info: 2
  };
  
  function getHighestPriorityError() {
    return errors.sort((a, b) => 
      ERROR_PRIORITY[a.severity] - ERROR_PRIORITY[b.severity]
    )[0];
  }
  ```

- **✅ Use error tokens instead of messages for consistency**
  ```typescript
  // Define error tokens
  const ERROR_TOKENS = {
    MISSING_API_KEY: 'error.missing_api_key',
    INVALID_FORMAT: 'error.invalid_format'
  };
  
  // Add structured error
  errorStore.addError({
    id: 'provider-check',
    token: ERROR_TOKENS.MISSING_API_KEY,
    provider: 'openai',
    severity: 'critical'
  });
  
  // Render with consistent formatting
  <span>{formatError($errorStore.find(e => e.id === 'provider-check'))}</span>
  ```

- **✅ Centralize focus management**
  ```typescript
  // Focus manager for modals and dialogs
  const focusManager = {
    previousFocus: null,
    
    captureFocus(element) {
      this.previousFocus = document.activeElement;
      element.focus();
    },
    
    returnFocus() {
      if (this.previousFocus && typeof this.previousFocus.focus === 'function') {
        this.previousFocus.focus();
      }
    }
  };
  
  // Using in component
  onMount(() => {
    focusManager.captureFocus(dialogElement);
  });
  
  onDestroy(() => {
    focusManager.returnFocus();
  });
  ```

- **✅ Implement progressive enhancement for component loading**
  ```svelte
  <script>
    // State to track component loading
    let componentsLoaded = {
      core: false,
      features: false,
      settings: false
    };
    
    onMount(() => {
      // Load core immediately
      componentsLoaded.core = true;
      
      // Defer loading features
      setTimeout(() => {
        componentsLoaded.features = true;
      }, 100);
      
      // Defer loading settings
      setTimeout(() => {
        componentsLoaded.settings = true;
      }, 200);
    });
  </script>
  
  <!-- Show core UI immediately -->
  {#if componentsLoaded.core}
    <CoreInterface />
  {:else}
    <LoadingPlaceholder />
  {/if}
  
  <!-- Progressively load other components -->
  {#if componentsLoaded.features}
    <FeatureSelector />
  {/if}
  ```

### DON'T:
- **❌ Mix form validation with UI state**
  ```typescript
  // BAD: Mixing validation and UI state
  function validateForm() {
    if (!nameInput.value) {
      nameInput.classList.add('error');
      errorMessage.textContent = 'Name is required';
      return false;
    }
    // More validation...
  }
  
  // GOOD: Separate validation from UI
  function validateForm() {
    const errors = {};
    
    if (!nameInput.value) {
      errors.name = 'Name is required';
    }
    // More validation...
    
    return { valid: Object.keys(errors).length === 0, errors };
  }
  
  // Then update UI separately
  function updateUIWithValidation(validationResult) {
    if (validationResult.errors.name) {
      nameInput.classList.add('error');
      nameErrorMessage.textContent = validationResult.errors.name;
    }
  }
  ```

- **❌ Create deep component hierarchies with prop drilling**
  ```svelte
  <!-- BAD: Deep prop drilling -->
  <App {settings} {user} {theme} {preferences} />
    <Dashboard {settings} {user} {theme} />
      <Panel {settings} {theme} />
        <Widget {theme} />
  
  <!-- GOOD: Use context or stores -->
  <App>
    <Dashboard />
      <Panel />
        <Widget />
  </App>
  
  <!-- In Widget.svelte -->
  <script>
    import { getContext } from 'svelte';
    const theme = getContext('theme');
  </script>
  ```

- **❌ Overuse of modals and popups without proper management**
  ```typescript
  // BAD: Creating modals ad-hoc without management
  function showErrorModal() {
    // Create without tracking
    const modal = document.createElement('div');
    modal.innerHTML = '<div class="modal">Error occurred!</div>';
    document.body.appendChild(modal);
  }
  
  // GOOD: Use a modal manager
  function showErrorModal() {
    modalManager.show('error', {
      message: 'Error occurred!',
      onClose: () => {
        // Handle cleanup
      }
    });
  }
  ```

- **❌ Directly manipulate DOM in response to prop changes**
  ```typescript
  // BAD: Direct DOM manipulation in response to prop
  $: if (isVisible) {
    // Direct DOM manipulation
    element.classList.add('visible');
  } else {
    element.classList.remove('visible');
  }
  
  // GOOD: Use Svelte binding
  <div class:visible={isVisible}>Content</div>
  ```

## 7. Understanding Prop Reactivity Patterns

This section completes the best practices guide by addressing the critical issue of prop reactivity patterns in Svelte - a unique aspect of the framework that differs significantly from React or Vue and can lead to subtle bugs if not properly understood and handled.

### Background: Svelte's Prop Reactivity Model

Svelte intentionally doesn't automatically watch props for changes after initial component creation. This design choice:
- Optimizes performance by avoiding unnecessary watchers
- Creates a predictable reactivity model based on assignments
- Enables more efficient compile-time optimizations
- Reduces runtime overhead compared to frameworks with dirty-checking

This can create reactivity blindspots where child components don't update when parent data changes.

### DO:
- **✅ Use {#key} directive to force component recreation for critical UI elements**
  ```svelte
  <!-- Force recreation when value or related dependencies change -->
  {#key `${value}-${dependencyA}-${dependencyB}`}
    <ChildComponent {value} />
  {/key}
  ```

- **✅ Implement direct store subscriptions for shared state**
  ```svelte
  <script>
    // In child component
    export let storeBinding = null; // {groupId, optionId}
    export let value;
    
    // Internal state that stays updated
    let internalValue = value;
    let unsubscribe;
    
    onMount(() => {
      // Subscribe directly to store if binding provided
      if (storeBinding?.groupId && storeBinding?.optionId) {
        const optionStore = featureGroupStore.createOptionSubscription(
          storeBinding.groupId, storeBinding.optionId
        );
        
        unsubscribe = optionStore.subscribe(newValue => {
          if (newValue !== undefined && newValue !== internalValue) {
            internalValue = newValue; // Local state stays updated
          }
        });
      }
    });
    
    onDestroy(() => {
      if (unsubscribe) unsubscribe();
    });
    
    // Make display values reactive to BOTH props and internal state
    $: displayValue = internalValue ?? value;
  </script>
  ```

- **✅ Use combined approach for complex dependency scenarios**
  ```svelte
  <!-- Parent component -->
  {#if optionId === 'provider'}
    <!-- Special case with complex dependencies -->
    {#key `provider-${storeValue}-${dependentStyleValue}`}
      <Dropdown
        value={storeValue}
        storeBinding={{ groupId, optionId }}
        options={availableOptions}
      />
    {/key}
  {:else}
    <!-- Standard case with store binding -->
    <Dropdown
      value={storeValue}
      storeBinding={{ groupId, optionId }}
      options={availableOptions}
    />
  {/if}
  ```

- **✅ Choose approach based on update frequency and complexity**
  ```typescript
  // Decision guide for component updates
  function chooseUpdateStrategy(component) {
    if (component.updateFrequency === 'high') {
      // High-frequency changes: Avoid recreation, use store subscription
      return 'store-subscription';
    } else if (component.hasDependencies) {
      // Complex dependencies: Combined approach
      return 'combined';
    } else if (component.needsReset) {
      // Component needs clean slate: Key directive
      return 'key-directive';
    } else {
      // Default case: Simple props
      return 'standard-props';
    }
  }
  ```

### DON'T:
- **❌ Assume props will automatically update like in React or Vue**
  ```svelte
  <!-- BAD: Assuming child will update when myProp changes -->
  <ChildComponent myProp={dynamicValue} />
  
  <!-- GOOD: Force update with key directive if needed -->
  {#key dynamicValue}
    <ChildComponent myProp={dynamicValue} />
  {/key}
  ```

- **❌ Overuse key directives without performance consideration**
  ```svelte
  <!-- BAD: Unnecessary recreation for every iteration -->
  {#each items as item}
    {#key item.id}
      <ItemComponent {item} />
    {/key}
  {/each}
  
  <!-- GOOD: Only use key when truly needed -->
  {#each items as item (item.id)}
    <ItemComponent {item} />
  {/each}
  ```

- **❌ Mix prop updates and store updates without coordination**
  ```typescript
  // BAD: Uncoordinated updates from multiple sources
  function updateComponent() {
    // Direct prop update
    component.prop = newValue;
    
    // Simultaneous store update that affects same component
    store.update(state => ({
      ...state,
      value: newValue
    }));
  }
  
  // GOOD: Coordinated update through single source
  function updateComponent() {
    // Update store only, component reads from store
    store.update(state => ({
      ...state,
      value: newValue
    }));
  }
  ```

- **❌ Create component update cycles with bidirectional bindings**
  ```svelte
  <!-- BAD: Creating update cycles -->
  <ParentComponent bind:value={parentValue}>
    <ChildComponent bind:value={parentValue} />
  </ParentComponent>
  
  <!-- GOOD: Unidirectional flow -->
  <ParentComponent {value} on:change={e => parentValue = e.detail}>
    <ChildComponent {value} on:change />
  </ParentComponent>
  ```

### Performance Implications:

**Component Recreation (Key Directive)**
- **Pros**: Guarantees fresh state, simpler mental model
- **Cons**: More expensive, potential flickering, loses component state

**Store Subscriptions**
- **Pros**: More efficient for frequently changing values, maintains component state
- **Cons**: More complex implementation, potential memory leaks if not cleaned up

**Combined Approach**
- **Pros**: Handles complex dependencies, more reliable updates
- **Cons**: Highest implementation complexity, requires careful coordination

## 8. Architecture Design Principles

This section outlines architectural best practices for Svelte applications based on lessons learned from complex feature implementation experiences. These principles focus on organizing your application for maintainability, predictability, and performance.

### State Management Architecture

#### DO:
- **✅ Centralize shared state, decentralize UI state**
  
  Maintain a clear separation between global application state and component-specific UI state. Only state that needs to be shared across components or persisted should be in stores. Local component state should remain local.

- **✅ Design stores around domain concepts, not components**
  
  Organize stores based on logical domains of your application (users, settings, features) rather than mirroring your component structure. This creates a more intuitive mental model and prevents tight coupling between your UI and state management.

- **✅ Establish clear data ownership boundaries**
  
  Every piece of state should have exactly one owner (either a store or a component). Avoid duplicating state across multiple components or stores, which leads to synchronization issues and bugs.

- **✅ Implement intelligent derivation over redundant state**
  
  When data needs to be transformed or filtered, use derived stores rather than creating new state. This ensures that derived data stays in sync with its source without manual management.

#### DON'T:
- **❌ Create hybrid state management approaches**
  
  Don't mix different state management patterns inconsistently. Choose either component props, context, or stores for a given type of state and be consistent. Hybrid approaches lead to unpredictable behavior.

- **❌ Store UI-only state in global stores**
  
  Modal visibility, form input values, and hover states rarely need to be globally accessible. Keep this state local to components unless explicitly needed elsewhere.

- **❌ Create circular state dependencies**
  
  Avoid situations where Store A depends on Store B which depends on Store A. These circular dependencies create difficult-to-debug update cycles and race conditions.

### Component Organization

#### DO:
- **✅ Create clear component responsibility layers**
  
  Design your components with distinct responsibilities: container components manage state and logic, presentation components handle rendering and events, and utility components provide reusable functionality.

- **✅ Design components as independent units with explicit contracts**
  
  Components should clearly define their API through props, events, and slots. A component should be understandable and usable without needing to know its internal implementation.

- **✅ Favor composition over inheritance or extension**
  
  Build complex UIs by composing smaller, focused components rather than creating deeply nested component hierarchies or component extension patterns.

- **✅ Implement feature-based organization for large applications**
  
  Group related components by feature or domain rather than by component type. This improves discoverability and helps maintain logical boundaries.

#### DON'T:
- **❌ Create monolithic components with multiple responsibilities**
  
  Avoid components that manage state, handle business logic, perform validation, and render complex UI. These become difficult to test, reuse, and maintain.

- **❌ Allow components to access parent or sibling state directly**
  
  Components should not reach outside their boundaries to access or modify state. Use props, events, and stores for communication instead.

- **❌ Design components with hidden external dependencies**
  
  A component that silently expects certain context providers or store structures to exist is brittle. Make all dependencies explicit.

### Communication Patterns

#### DO:
- **✅ Establish unidirectional data flow throughout your application**
  
  Data should flow down through props and events should flow up. This creates a predictable mental model that makes the application easier to reason about.

- **✅ Separate reading from writing in your API design**
  
  Split your interfaces into read operations (getting values) and write operations (updating values). This clarifies intent and prevents accidental modifications.

- **✅ Design communication based on component purpose, not proximity**
  
  Choose communication patterns based on what components need to accomplish, not just how they're positioned in the tree. Sometimes distant components need direct communication paths.

- **✅ Create explicit coordination points for complex interactions**
  
  When multiple components need to interact in complex ways, create a dedicated coordination layer (manager, controller, or specialized store) rather than direct component-to-component communication.

#### DON'T:
- **❌ Create bidirectional update loops**
  
  Avoid situations where Component A updates Component B which then updates Component A. These create infinite loops and unpredictable state.

- **❌ Mix different communication paradigms without clear boundaries**
  
  Don't mix message passing, event dispatch, and direct method calls inconsistently. Create clear boundaries where different paradigms are used.

- **❌ Rely on implicit communication through DOM structure**
  
  Don't use parent-child DOM relationships to create implicit communication paths. Make all communication explicit through props, events, or stores.

### Application Structure Recommendations

#### DO:
- **✅ Create explicit initialization sequences**
  
  Define clear application startup processes that handle asynchronous data loading, authentication, and configuration in a coordinated way.

- **✅ Separate domain logic from UI components**
  
  Extract business logic, validation, and transformation into separate service modules that components can import and use.

- **✅ Design for progressive enhancement and selective loading**
  
  Structure your application to load and initialize in phases, delivering core functionality quickly and enhancing progressively.

- **✅ Implement comprehensive error boundaries**
  
  Design your architecture with explicit error handling at key boundaries. Components should fail gracefully without crashing the entire application.

#### DON'T:
- **❌ Create deep component hierarchies with fragile dependencies**
  
  Avoid deep nesting of components where changes to a parent component can easily break children deep in the tree.

- **❌ Assume synchronous initialization across the application**
  
  Don't design your application with the assumption that all data will be available at once. Handle asynchronous loading and partial state explicitly.

- **❌ Allow cross-cutting concerns to spread throughout components**
  
  Concerns like logging, analytics, permissions, and error handling should be abstracted into services rather than duplicated across components.

### Lessons from Real-World Application Development

Our experience developing complex feature systems yielded several key insights:

1. **Simplicity Trumps Cleverness**
   
   When we replaced our three-tiered reactivity strategy with a single consistent approach, bugs decreased and maintainability improved. Prefer straightforward solutions over complex ones, even if they seem less elegant.

2. **State Centralization Requires Discipline**
   
   While centralizing shared state is beneficial, it requires clear rules about what belongs in stores versus component state. Without these boundaries, stores become bloated and components lose autonomy.

3. **Reactivity Boundary Clarity Is Essential**
   
   Every component and store should have clearly defined reactivity boundaries: what triggers updates, which values are derived, and how changes propagate. Without this clarity, changes cause unpredictable cascading updates.

4. **UI-Domain Separation Improves Flexibility**
   
   When we separated our domain logic (option relationships, validation) from UI components, both became simpler and more reusable. Domain rules belonged in stores or services, UI rendering in components.

5. **Consistent Patterns Enable Collaboration**
   
   Teams benefit from consistent architectural patterns more than perfectly optimized ones. When developers can make reliable assumptions about how components communicate and state flows, productivity increases dramatically.

By applying these architectural principles, you can create Svelte applications that are more maintainable, predictable, and resilient to change.
