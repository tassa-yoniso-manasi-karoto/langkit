<script lang="ts">
    import { createEventDispatcher, onMount } from 'svelte';
    import type { RomanizationScheme } from '../lib/featureModel'; // Import the type
    import { debounce } from 'lodash';
    
    import { featureGroupStore } from '../lib/featureGroupStore';
    import { logger } from '../lib/logger';
    
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
    
    // Flag to prevent initialization feedback loops
    let isInitialized = false;
    
    // Initialize from component props
    onMount(() => {
        // Set initial local value
        localValue = value;
        
        // Store initial value in group store
        featureGroupStore.setGroupOption(groupId, optionId, value);
        
        // Mark as initialized and track external update time
        isInitialized = true;
        lastExternalUpdateTime = Date.now();
        
        logger.trace('groupOption', `GroupOption mounted: ${groupId}.${optionId}=${value}`);
    });
    
    // Handle external value changes (from parent or store)
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
                    }
                }
            } else {
                // No user input yet - accept external value
                if (value !== localValue) {
                    localValue = value;
                }
            }
        }
    }
    
    // Update provider when style changes
    $: if (groupId === 'subtitle' && optionId === 'style' && localValue && romanizationSchemes.length > 0) {
        const selectedScheme = romanizationSchemes.find(s => s.name === localValue);
        if (selectedScheme) {
            // Update the provider in the group store
            const currentProvider = featureGroupStore.getGroupOption(groupId, 'provider');
            if (selectedScheme.provider !== currentProvider) {
                logger.trace('groupOption', `Style changed to ${localValue}, updating provider to ${selectedScheme.provider}`);
                featureGroupStore.setGroupOption(groupId, 'provider', selectedScheme.provider);
            }
        }
    }
    
    // Helper function to propagate user input to all necessary places
    function propagateUserValue(newValue: any) {
        // Store in group store
        featureGroupStore.setGroupOption(groupId, optionId, newValue);
        
        // Notify parent component
        dispatch('groupOptionChange', { 
            groupId, 
            optionId, 
            value: newValue,
            isUserInput: true
        });
    }
    
    // Handle direct user input with high authority
    function handleUserInput(newValue: any) {
        // Set user input timestamp slightly in the future to ensure it's authoritative
        lastUserUpdateTime = Date.now() + 100; // Small future timestamp
        
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

    // Handle romanization style changes with special provider update logic
    function handleRomanizationChange(event: any) {
        const newValue = event.detail;
        logger.trace('groupOption', `Romanization style change: ${newValue}`);
        
        // Mark as user update with authority
        lastUserUpdateTime = Date.now() + 100;
        localValue = newValue;
        
        // Update the style
        featureGroupStore.setGroupOption(groupId, optionId, newValue);
        
        // Also update the provider if a matching scheme is found
        const selectedScheme = romanizationSchemes.find(s => s.name === newValue);
        if (selectedScheme) {
            const newProvider = selectedScheme.provider;
            logger.trace('groupOption', `Updating provider to ${newProvider} based on style ${newValue}`);
            
            // Update the provider in the group store
            featureGroupStore.setGroupOption(groupId, 'provider', newProvider);
            
            // Notify about provider change too
            dispatch('groupOptionChange', { 
                groupId, 
                optionId: 'provider', 
                value: newProvider,
                isUserInput: true
            });
        }
        
        // Notify about style change
        dispatch('groupOptionChange', { 
            groupId, 
            optionId, 
            value: newValue,
            isUserInput: true
        });
    }

    // Handle immediate changes like checkboxes and numeric inputs
    function handleImmediateChange(event?: Event) { // Make event optional for direct calls
        // For checkboxes, get value directly from event if available
        const isCheckbox = event?.target instanceof HTMLInputElement && event.target.type === 'checkbox';
        const valueToPropagate = isCheckbox ? (event.target as HTMLInputElement).checked : localValue;
        
        logger.trace('groupOption', `Immediate change: ${valueToPropagate} for ${groupId}.${optionId}`);
        
        // Mark as user update with authority
        lastUserUpdateTime = Date.now() + 100;
        
        // Propagate the value (from event for checkboxes, from localValue otherwise)
        propagateUserValue(valueToPropagate);
    }

    // handleCheckboxChange function removed
</script>

<div class="group-option" data-group-id={groupId}>
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
                {@const groupMessage = groupId === 'subtitle' 
                    ? "This option is shared across subtitle features" 
                    : groupId === 'merge' 
                        ? "This option is shared across output merge features"
                        : `This option is shared across ${groupId} features`}
                {@const iconColorClass = groupId === 'subtitle' 
                    ? "text-group-subtitle" 
                    : groupId === 'merge' 
                        ? "text-group-merge"
                        : ""}
                <Hovertip message={groupMessage}>
                    <span slot="trigger" class="cursor-help">
                        <GroupIcon size="1.5em" className={iconColorClass} />
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
                            logger.trace('groupOption', `Toggle checkbox: ${localValue} -> ${newValue} for ${groupId}.${optionId}`);
                            
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
                        on:input={() => {
                            // Set user input timestamp for authority
                            lastUserUpdateTime = Date.now() + 100;
                            
                            // Update the group option value but don't validate
                            featureGroupStore.setGroupOption(groupId, optionId, localValue);
                            
                            // Notify parent component
                            dispatch('groupOptionChange', {
                                groupId,
                                optionId,
                                value: localValue,
                                isUserInput: true
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
                    on:input={handleChange}
                />
            {/if}
        {:else if optionDef.type === 'romanizationDropdown'}
            <!-- Special handling for romanization style dropdown -->
            <Dropdown
                options={romanizationSchemes}
                optionKey="name"
                optionLabel="description"
                value={localValue}
                on:change={handleRomanizationChange}
                label=""
                placeholder="Select style..."
            />
        {:else if optionDef.type === 'provider'}
            <!-- Show provider with GitHub link if available -->
            <div class="w-full px-3 py-1 text-sm inline-flex font-bold text-white/90 items-center justify-center gap-2">
                <!-- Force lookup of provider from style -->
                {#if groupId === 'subtitle' && optionId === 'provider'}
                    {@const styleValue = featureGroupStore.getGroupOption(groupId, 'style')}
                    {@const selectedScheme = romanizationSchemes.find(s => s.name === styleValue)}
                    {@const providerValue = selectedScheme ? selectedScheme.provider : (localValue || '')}
                    
                    <!-- Display the provider value -->
                    {providerValue}
                    
                    <!-- Update if needed -->
                    {#if providerValue !== localValue && providerValue}
                        {localValue = providerValue}
                        {featureGroupStore.setGroupOption(groupId, 'provider', providerValue)}
                    {/if}
                    
                    <!-- GitHub link if available -->
                    <!-- Ensure string even if providerValue is null/undefined -->
                    {@const providerKey = String(providerValue || '')}
                    {#if providerKey && providerGithubUrls[providerKey]}
                        <ExternalLink
                            href={providerGithubUrls[providerKey]}
                            className="text-primary/70 hover:text-primary transition-colors duration-200"
                            title="View provider repository">
                            <svg viewBox="0 0 16 16" class="w-5 h-5 fill-primary">
                                <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                            </svg>
                        </ExternalLink>
                    {/if}
                {:else}
                    <!-- Regular provider display for non-subtitle groups -->
                    {localValue || ''}
                    <!-- Ensure string -->
                    {@const localProviderKey = String(localValue || '')}
                    {#if localProviderKey && providerGithubUrls[localProviderKey]}
                        <ExternalLink
                            href={providerGithubUrls[localProviderKey]}
                            className="text-primary/70 hover:text-primary transition-colors duration-200"
                            title="View provider repository">
                            <svg viewBox="0 0 16 16" class="w-5 h-5 fill-primary">
                                <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                            </svg>
                        </ExternalLink>
                    {/if}
                {/if}
            </div>
        {/if}
    </div>
</div>

<style>
    /* Styling for group options */
    .group-option {
        display: grid;
        grid-template-columns: minmax(120px, 1fr) minmax(0, 1.5fr);
        gap: 1.5rem;
        align-items: center;
        border-left: 3px solid; /* Color is applied dynamically */
        padding-left: 0.25rem; /* Reduced padding to minimize indentation */
        margin-left: 0;
    }
    
    /* Group-specific border colors using Tailwind color scheme */
    :global(.group-option[data-group-id='subtitle']) {
        border-left-color: var(--group-subtitle-color);
    }
    
    :global(.group-option[data-group-id='merge']) {
        border-left-color: var(--group-merge-color);
    }
    
    .option-label {
        padding-left: 0.125rem;
    }
    
    .option-input {
        width: 100%; /* Ensure consistent width with regular options */
    }
</style>