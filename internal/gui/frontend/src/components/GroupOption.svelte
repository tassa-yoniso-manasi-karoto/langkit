<script lang="ts">
    import { createEventDispatcher, onMount, onDestroy } from 'svelte';
    import { get, derived } from 'svelte/store'; // Import derived
    import type { RomanizationScheme } from '../lib/featureModel'; // Import the type
    import { debounce } from 'lodash';
    import { featureGroupStore } from '../lib/featureGroupStore';
    import { errorStore } from '../lib/errorStore';
    import { trackComponentMount, trackComponentDestroy } from '../lib/metrics'; // Import metrics tracking
    
    import Dropdown from './Dropdown.svelte';
    import Hovertip from './Hovertip.svelte';
    import NumericInput from './NumericInput.svelte';
    import TextInput from './TextInput.svelte';
    import GroupIcon from './icons/GroupIcon.svelte';
    import ExternalLink from './ExternalLink.svelte';
    
    // Props
    export let groupId: string;
    export let optionId: string;
    export let optionDef: any;
    export let value: any = optionDef.default;
    export let needsDocker: boolean = false;
    export let needsScraper: boolean = false;
    export let romanizationSchemes: RomanizationScheme[] = []; // Apply the type
    
    // Group indicator
    export let showGroupIndicator: boolean = true;
    
    // For handling special providers
    // Define type for provider URLs
    type ProviderUrls = {
        [key: string]: string;
    };
    const providerGithubUrls: ProviderUrls = {
        'ichiran': 'https://github.com/tshatrov/ichiran',
        'aksharamukha': 'https://github.com/virtualvinodh/aksharamukha',
        // Add other providers if needed
    };
    
    const dispatch = createEventDispatcher();
    
    // Value tracking with authority management
    let localValue = value;
    
    // Track last update times to determine authoritative source
    // Epoch timestamp: 0 means no user update has occurred yet
    let lastUserUpdateTime = 0;
    let lastExternalUpdateTime = Date.now();
    
    // Debug control
    const DEBUG = false;
    
    // Clean tracking of store changes
    let unsubscribeFromStore: () => void;
    
    // Flag to prevent initialization feedback loops
    let isInitialized = false;
    let isUpdating = false; // Add state for animation
    
    // Add flags to track update sources
    let updatingFromProvider = false;
    let updatingFromStyle = false;

    // Input validity tracking
    let isValid = true;
    let validationMessage = '';
    
    // Error handling
    function validateValue(val: any): { isValid: boolean; message: string } {
      // Get the group option definition for validation
      const optionDefinition = featureGroupStore.getGroupOptionDefinition(groupId, optionId);
      if (!optionDefinition) {
        return { isValid: true, message: '' };
      }
      
      // Type-specific validation
      switch (optionDefinition.type) {
        case 'number':
          // Ensure value is numeric
          const numValue = typeof val === 'string' ? parseFloat(val) : val;
          if (isNaN(numValue)) {
            return {
              isValid: false,
              message: 'Value must be a number'
            };
          }
          
          // Check numeric bounds
          if (typeof numValue === 'number') {
            if (optionDefinition.min !== undefined && numValue < optionDefinition.min) {
              return {
                isValid: false,
                message: `Value must be at least ${optionDefinition.min}`
              };
            }
            if (optionDefinition.max !== undefined && numValue > optionDefinition.max) {
              return {
                isValid: false,
                message: `Value must be at most ${optionDefinition.max}`
              };
            }
          }
          break;
        
        case 'string':
          // For browserAccessURL, validate WebSocket URL format if needed
          if (optionId === 'browserAccessURL' && val && !val.startsWith('ws://')) {
            return {
              isValid: false,
              message: 'Browser URL must start with ws://'
            };
          }
          break;
      }
      
      return { isValid: true, message: '' };
    }
    
    // Debounced update handler
    const updateFromStore = debounce((newValue: any) => {
        if (newValue !== undefined && newValue !== localValue) {
            if (DEBUG) console.log(`[GroupOption] Update ${groupId}.${optionId}: ${localValue} → ${newValue}`);
            localValue = newValue;
            
            // Validate but don't propagate back to store
            const validation = validateValue(localValue);
            isValid = validation.isValid;
            validationMessage = validation.message;
        }
    }, 50);
    
    let optionStore; // Declare optionStore

    onMount(() => {
        // Generate a unique ID for this component instance
        const componentId = `${groupId}.${optionId}`;
        
        // Track component mounting
        trackComponentMount(componentId);

        // Create a derived store that only tracks this specific option
        optionStore = featureGroupStore.createOptionSubscription(groupId, optionId);
        
        // Subscribe only to relevant changes
        unsubscribeFromStore = optionStore.subscribe(storeValue => {
            if (storeValue !== undefined && storeValue !== localValue) {
                // Update local value from store
                localValue = storeValue;
                 // Validate the new value
                const validation = validateValue(localValue);
                isValid = validation.isValid;
                validationMessage = validation.message;
            }
        });
        
        // Initialize with current value if needed
        const initialValue = featureGroupStore.getGroupOption(groupId, optionId);
        if (initialValue !== undefined && initialValue !== localValue) {
            localValue = initialValue;
        } else if (initialValue === undefined) {
             // Store initial value in group store if not already set by sync
             featureGroupStore.setGroupOption(groupId, optionId, localValue);
        }

        // Validate initial value
        const validation = validateValue(localValue);
        isValid = validation.isValid;
        validationMessage = validation.message;
        
        // Mark as initialized and track external update time
        isInitialized = true;
        lastExternalUpdateTime = Date.now();
        
        if (DEBUG) console.log(`GroupOption mounted: ${componentId}=${localValue}`);
    });
    
    // Handle external value changes (from parent or store) - Keep this for parent updates
    $: {
        if (isInitialized && value !== undefined) {
            // Always update timestamp regardless of value truthiness
            lastExternalUpdateTime = Date.now();
            
            // User input should take precedence when timestamps indicate it's newer
            if (lastUserUpdateTime > 0) {
                if (lastUserUpdateTime > lastExternalUpdateTime) {
                    // Only re-propagate if values differ
                    if (value !== localValue) {
                        propagateUserValue(localValue);
                    }
                } else {
                    // If external update is newer than user input
                    // Only apply the change if values actually differ
                    if (value !== localValue) {
                        localValue = value;
                        
                        // Validate the new value
                        const validation = validateValue(localValue);
                        isValid = validation.isValid;
                        validationMessage = validation.message;
                    }
                }
            } else {
                // No user input yet - accept external value
                if (value !== localValue) {
                    localValue = value;
                    
                    // Validate the new value
                    const validation = validateValue(localValue);
                    isValid = validation.isValid;
                    validationMessage = validation.message;
                }
            }
        }
    }
    
    // Debounced update function with guards for style -> provider
    const updateProviderForStyle = debounce((styleValue) => {
        // Skip if already handling a provider update or no style value
        if (updatingFromProvider || !styleValue) return;
        
        const selectedScheme = romanizationSchemes.find(s => s.name === styleValue);
        if (!selectedScheme) return;
        
        const currentProvider = featureGroupStore.getGroupOption(groupId, 'provider');
        if (selectedScheme.provider !== currentProvider) {
            // Set flag to prevent circular updates
            updatingFromStyle = true;
            
            // Update provider in store
            featureGroupStore.setGroupOption(groupId, 'provider', selectedScheme.provider);
            
            // Reset flag after a short delay to allow store update to complete
            setTimeout(() => {
                updatingFromStyle = false;
            }, 50);
        }
    }, 50);
    
    // Debounced handler for provider updates (if needed in future)
    const updateStyleForProvider = debounce((providerValue) => {
        // Skip if already handling a style update or no provider value
        if (updatingFromStyle || !providerValue) return;
        
        // Only for provider components
        if (optionId !== 'provider') return;
        
        // Logic if needed to update style based on provider
        // This is typically not needed but included for completeness
    }, 50);
    
    // Simplified reactive statements with guards
    $: if (optionId === 'style' && localValue) {
        updateProviderForStyle(localValue);
    }
    
    $: if (optionId === 'provider' && localValue) {
        updateStyleForProvider(localValue);
    }
    
    onDestroy(() => {
        const componentId = `${groupId}.${optionId}`;
        
        // Track component destruction
        trackComponentDestroy(componentId);
        
        // Original cleanup logic...
        if (unsubscribeFromStore) unsubscribeFromStore();
        if (updateFromStore && updateFromStore.cancel) updateFromStore.cancel(); // Check if cancel exists
        // Cancel new debounced functions
        updateProviderForStyle.cancel();
        updateStyleForProvider.cancel();
    });
    
    // Helper function to propagate user input to all necessary places
    function propagateUserValue(newValue: any) {
      // Validate before propagating
      const validation = validateValue(newValue);
      isValid = validation.isValid;
      validationMessage = validation.message;
      
      // Store in group store
      featureGroupStore.setGroupOption(groupId, optionId, newValue);
      
      // Add/remove error based on validity
      const errorId = `group-option-${groupId}-${optionId}`;
      if (!isValid) {
        errorStore.addError({
          id: errorId,
          message: `${optionDef.label}: ${validationMessage}`,
          severity: 'warning'
        });
      } else {
        errorStore.removeError(errorId);
      }
      
      // Add update animation
      isUpdating = true;
      setTimeout(() => { isUpdating = false; }, 500);

      // Notify parent component
      dispatch('groupOptionChange', {
        groupId,
        optionId,
        value: newValue,
        isUserInput: true,
        isValid
      });
    }
    
    // Handle direct user input with high authority
    function handleUserInput(newValue: any) {
        // Set user input timestamp slightly in the future to ensure it's authoritative
        lastUserUpdateTime = Date.now() + 10;
        
        // Update local value
        localValue = newValue;
        
        // User input is authoritative - propagate immediately
        propagateUserValue(newValue);
    }
    
    // Create a debounced version of user input handler
    const debouncedUserInput = debounce(handleUserInput, 100);
    
    // Handle option changes from UI events
    function handleChange(event: any) {
        const newValue = event.detail || event.target.value;
        debouncedUserInput(newValue);
    }

    // Handle romanization changes with debouncing
    const debouncedRomanizationChange = debounce((event: CustomEvent) => { // Add CustomEvent type
        const newValue = event.detail;
        if (newValue === localValue) return; // No change
        
        localValue = newValue;
        
        // Update the style in store
        featureGroupStore.setGroupOption(groupId, optionId, newValue);
        
        // Find matching scheme for provider update - This is now handled by the reactive statement
        // const selectedScheme = romanizationSchemes.find(s => s.name === newValue);
        // if (selectedScheme) {
        //     const newProvider = selectedScheme.provider;
        //     // Update provider in store
        //     featureGroupStore.setGroupOption(groupId, 'provider', newProvider);
        // }
        
        // Notify parent about style change
        dispatch('groupOptionChange', { 
            groupId, 
            optionId, 
            value: newValue
        });
    }, 50);
    
    function handleRomanizationChange(event: CustomEvent) { // Add CustomEvent type
        debouncedRomanizationChange(event);
    }

    // Handle immediate changes like checkboxes and numeric inputs
    function handleImmediateChange(event?: Event) { // Make event optional for direct calls
        // For checkboxes, get value directly from event if available
        const isCheckbox = event?.target instanceof HTMLInputElement && event.target.type === 'checkbox';
        const valueToPropagate = isCheckbox ? (event.target as HTMLInputElement).checked : localValue;
        
        if (DEBUG) console.log(`Immediate change: ${valueToPropagate} for ${groupId}.${optionId}`);
        
        // Mark as user update with authority
        lastUserUpdateTime = Date.now() + 10;
        
        // Propagate the value (from event for checkboxes, from localValue otherwise)
        propagateUserValue(valueToPropagate);
    }

    // Handle value recovery for invalid inputs
    function recoverFromInvalidInput() {
      // If current value is invalid, try to recover with a valid value
      if (!isValid && optionDef.default !== undefined) {
        if (DEBUG) console.log(`Recovering invalid input for ${groupId}.${optionId} with default value`);
        
        // Reset to default value
        localValue = optionDef.default;
        
        // Mark as external update
        lastUserUpdateTime = 0;
        lastExternalUpdateTime = Date.now();
        
        // Update the group store
        featureGroupStore.setGroupOption(groupId, optionId, optionDef.default);
        
        // Clear validation error
        isValid = true;
        validationMessage = '';
        
        // Remove error from store
        errorStore.removeError(`group-option-${groupId}-${optionId}`);
        
        // Notify parent
        dispatch('groupOptionChange', {
          groupId,
          optionId,
          value: optionDef.default,
          isUserInput: false,
          isValid: true,
          isRecovered: true
        });
      }
    }
    
    // Enhance romanization style handling - Removed redundant block, handled by reactive statements above
</script>

<div class="group-option" class:invalid={!isValid} class:updating={isUpdating} data-group-id={groupId}>
    <div class="option-label">
        <span class="text-gray-300 text-sm text-left flex items-center gap-2">
            {optionDef.label}
            {#if optionDef.hovertip}
                <Hovertip message={optionDef.hovertip}>
                    <span slot="trigger" class="material-icons text-primary/70 cursor-help leading-none material-icon-adjust">
                        help_outline
                    </span>
                </Hovertip>
            {/if}
            {#if showGroupIndicator}
                <Hovertip message={groupId === 'subtitle' 
                    ? "This option is shared across subtitle features" 
                    : groupId === 'merge' 
                        ? "This option is shared across output merge features"
                        : `This option is shared across ${groupId} features`}>
                    <!-- Simpler, more direct icon -->
                    <span class="group-badge">
                        <span class="inline-block w-4 h-4 bg-current rounded-full opacity-70 
                              hover:opacity-100 transition-opacity duration-200"
                              class:bg-subtitle={groupId === 'subtitle'}
                              class:bg-merge={groupId === 'merge'}>
                        </span>
                    </span>
                </Hovertip>
            {/if}
        </span>
    </div>
    
    <div class="option-input">
        {#if optionDef.type === 'number'}
            <NumericInput 
                bind:value={localValue}
                step={optionDef.step || '1'}
                min={optionDef.min}
                max={optionDef.max}
                placeholder={optionDef.placeholder}
                invalid={!isValid}
                errorMessage={validationMessage}
                on:change={(e) => handleImmediateChange(e)}
            />
        {:else if optionDef.type === 'boolean'}
            <label class="inline-flex items-center cursor-pointer">
                {#key localValue}
                    <input
                        type="checkbox"
                        class="w-5 h-5 accent-primary rounded border-2 border-primary/50
                               checked:bg-primary checked:border-primary
                               focus:ring-2 focus:ring-primary/30
                               transition-all duration-200
                               cursor-pointer"
                        checked={localValue}
                        on:click={(event) => {
                            // Prevent default behavior - we'll handle the state directly
                            event.preventDefault();
                            
                            // Directly toggle the value
                            const newValue = !localValue;
                            if (DEBUG) console.log(`Toggle checkbox: ${localValue} -> ${newValue} for ${groupId}.${optionId}`);
                            
                            // Update local state
                            localValue = newValue;
                            
                            // Mark as user update with authority
                            lastUserUpdateTime = Date.now() + 100;
                            
                            // Propagate to store and parent
                            propagateUserValue(newValue);
                        }}
                    />
                {/key}
            </label>
        {:else if optionDef.type === 'dropdown'}
            <!-- Remove label to avoid duplication -->
            <Dropdown
                options={optionDef.choices || []}
                value={localValue}
                on:change={handleChange}
                label=""
                placeholder={`Select ${optionDef.label}...`}
                invalid={!isValid}
                errorMessage={validationMessage}
            />
        {:else if optionDef.type === 'string'}
            {#if optionId === 'browserAccessURL'}
                <!-- Browser URL with optional badge -->
                <div class="relative">
                    <TextInput
                        bind:value={localValue}
                        placeholder={optionDef.placeholder || "ws://127.0.0.1:9222/... (optional)"}
                        className="text-sm placeholder:text-gray-500 pr-20"
                        minLength={undefined}
                        maxLength={undefined}
                        invalid={!isValid}
                        errorMessage={validationMessage}
                        on:input={() => {
                            // Set user input timestamp for authority
                            lastUserUpdateTime = Date.now() + 10;
                            
                            // Update the group option value
                            featureGroupStore.setGroupOption(groupId, optionId, localValue);
                            
                            // Validate the input
                            const validation = validateValue(localValue);
                            isValid = validation.isValid;
                            validationMessage = validation.message;
                            
                            // Add/remove error based on validity
                            const errorId = `group-option-${groupId}-${optionId}`;
                            if (!isValid && localValue) {  // Only show error if there's a value
                              errorStore.addError({
                                id: errorId,
                                message: `${optionDef.label}: ${validationMessage}`,
                                severity: 'warning'
                              });
                            } else {
                              errorStore.removeError(errorId);
                            }
                            
                            // Notify parent component
                            dispatch('groupOptionChange', {
                                groupId,
                                optionId,
                                value: localValue,
                                isUserInput: true,
                                isValid
                            });
                        }}
                    />
                    <!-- Add an "Optional" badge to the input -->
                    <div class="absolute right-2 top-1/2 transform -translate-y-1/2 text-xs px-2 py-0.5 bg-primary/20 text-primary-200 rounded-full pointer-events-none">
                        Optional
                    </div>
                </div>
            {:else}
                <TextInput
                    bind:value={localValue}
                    placeholder={optionDef.placeholder}
                    className="text-sm placeholder:text-gray-500"
                    minLength={undefined}
                    maxLength={undefined}
                    invalid={!isValid}
                    errorMessage={validationMessage}
                    on:input={handleChange}
                />
            {/if}
        {:else if optionDef.type === 'romanizationDropdown'}
            <!-- Force dropdown re-render when value changes -->
            {#key localValue + '-' + romanizationSchemes.length}
                <Dropdown
                    options={romanizationSchemes}
                    optionKey="name"
                    optionLabel="description"
                    value={localValue}
                    on:change={handleRomanizationChange}
                    label=""
                    placeholder="Select style..."
                    invalid={!isValid}
                    errorMessage={validationMessage}
                />
            {/key}
        {:else if optionDef.type === 'provider'}
            <!-- Simplified provider display -->
            <div class="w-full px-3 py-1 text-sm inline-flex font-bold text-white/90 items-center justify-center gap-2">
                {#if groupId === 'subtitle'}
                    <!-- Get fresh values from store carefully -->
                    {@const styleValue = featureGroupStore.getGroupOption(groupId, 'style')}
                    {@const selectedScheme = romanizationSchemes.find(s => s.name === styleValue)}
                    {@const providerValue = selectedScheme ? selectedScheme.provider : localValue}
                    
                    <!-- Display the provider value -->
                    <span>{providerValue || 'No provider selected'}</span>
                    
                    <!-- GitHub link if available -->
                    {#if providerValue && providerGithubUrls[providerValue]}
                        <ExternalLink href={providerGithubUrls[providerValue]} className="text-primary/70 hover:text-primary transition-colors duration-200">
                            <svg viewBox="0 0 16 16" class="w-5 h-5 fill-primary">
                                 <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                            </svg>
                        </ExternalLink>
                    {/if}
                {:else}
                    <!-- Regular provider display -->
                    {localValue || ''}
                {/if}
            </div>
        {/if}
    </div>
    
    {#if !isValid && validationMessage}
      <div class="validation-error text-xs text-error-task mt-1 ml-[122px] flex items-center">
        <span class="material-icons text-[14px] mr-1">error_outline</span>
        <span>{validationMessage}</span>
        <button
          class="ml-2 text-primary text-xs hover:text-primary-300 transition-colors duration-200"
          on:click={recoverFromInvalidInput}
        >
          Reset to default
        </button>
      </div>
    {/if}
</div>

<style>
    /* CSS remains largely the same with optimizations */
    .group-option {
      display: grid;
      grid-template-columns: minmax(120px, 1fr) minmax(0, 1.5fr);
      gap: 1.5rem;
      align-items: center;
      padding-left: 0.25rem;
      margin-left: 0;
      position: relative;
      border-left: 3px solid transparent;
    }
    
    /* Direct attribute selectors without :global() */
    .group-option[data-group-id='subtitle'] {
      border-left-color: hsla(210, 90%, 60%, 0.35);
    }
    
    .group-option[data-group-id='merge'] {
      border-left-color: hsla(130, 90%, 50%, 0.35);
    }
    
    /* Simple color classes */
    .bg-subtitle {
      background-color: hsla(210, 90%, 60%, 0.8) !important;
    }
    
    .bg-merge {
      background-color: hsla(130, 90%, 50%, 0.8) !important;
    }
  
  .group-badge {
    display: inline-flex;
    margin-left: 4px;
  }
  
  
  
  

    .group-badge:hover {
      opacity: 1;
    }

    /* Remove :global() selectors for icons as they are handled by class */
    /* Invalid state styling */
    .group-option.invalid :global(input),
    .group-option.invalid :global(select),
    .group-option.invalid :global(textarea) {
      border-color: var(--error-task-color);
    }

    /* Ensure invalid border takes precedence on focus */
    .group-option.invalid :global(input:focus),
    .group-option.invalid :global(select:focus),
    .group-option.invalid :global(textarea:focus) {
      border-color: var(--error-task-color);
      box-shadow: 0 0 0 2px rgba(var(--error-task-rgb), 0.25);
    }

    /* Updating animation */
    .group-option.updating {
      animation: option-update-pulse 0.5s ease;
    }
    
    @keyframes option-update-pulse {
      0% { background-color: transparent; }
      30% { background-color: rgba(159, 110, 247, 0.1); }
      100% { background-color: transparent; }
    }
</style>