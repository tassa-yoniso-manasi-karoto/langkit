<script lang="ts">
    import { createEventDispatcher, onMount } from 'svelte';
    import { debounce } from 'lodash';
    
    import { featureGroupStore } from '../lib/featureGroupStore';
    
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
    export let romanizationSchemes = [];
    
    // Group indicator
    export let showGroupIndicator: boolean = true;
    
    // For handling special providers
    const providerGithubUrls = {
        'ichiran': 'https://github.com/tshatrov/ichiran',
        'aksharamukha': 'https://github.com/virtualvinodh/aksharamukha',
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
        
        // Validate if needed
        if (optionId === 'browserAccessURL') {
            featureGroupStore.validateBrowserUrl(value, needsScraper, groupId);
        }
        
        // Mark as initialized and track external update time
        isInitialized = true;
        lastExternalUpdateTime = Date.now();
    });
    
    // Handle external value changes (from parent or store)
    $: {
        if (isInitialized && value !== undefined) {
            // SPECIAL CASE: Empty value should never override a valid WebSocket URL
            if (optionId === 'browserAccessURL' && !value && localValue && localValue.startsWith('ws://')) {
                // Force re-propagation of the valid URL to preserve it
                propagateUserValue(localValue);
            } else {
                // Only update timestamp for non-empty values
                if (value) {
                    lastExternalUpdateTime = Date.now();
                }
                
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
                            
                            // Validate if needed
                            if (optionId === 'browserAccessURL') {
                                featureGroupStore.validateBrowserUrl(value, needsScraper, groupId);
                            }
                        }
                    }
                } else {
                    // No user input yet - accept external value
                    if (value !== localValue) {
                        localValue = value;
                        
                        // Validate if needed
                        if (optionId === 'browserAccessURL') {
                            featureGroupStore.validateBrowserUrl(value, needsScraper, groupId);
                        }
                    }
                }
            }
        }
    }
    
    // Force update provider value when romanization schemes change or style changes
    $: if (groupId === 'subtitle') {
        // When romanization schemes load or change
        if (optionId === 'provider' && romanizationSchemes.length > 0) {
            const styleValue = featureGroupStore.getGroupOption(groupId, 'style');
            if (styleValue) {
                const selectedScheme = romanizationSchemes.find(s => s.name === styleValue);
                if (selectedScheme && selectedScheme.provider !== localValue) {
                    // Update provider value directly
                    console.log(`Updating provider from ${localValue} to ${selectedScheme.provider} based on style ${styleValue}`);
                    localValue = selectedScheme.provider;
                    propagateUserValue(selectedScheme.provider);
                }
            }
        }
        // When style changes and this is a provider option
        else if (optionId === 'style' && localValue && romanizationSchemes.length > 0) {
            const selectedScheme = romanizationSchemes.find(s => s.name === localValue);
            if (selectedScheme) {
                // Update the provider in the group store
                const currentProvider = featureGroupStore.getGroupOption(groupId, 'provider');
                if (selectedScheme.provider !== currentProvider) {
                    console.log(`Style changed to ${localValue}, updating provider to ${selectedScheme.provider}`);
                    featureGroupStore.setGroupOption(groupId, 'provider', selectedScheme.provider);
                }
            }
        }
    }
    
    // Helper function to propagate user input to all necessary places
    function propagateUserValue(newValue: any) {
        // Store in group store
        featureGroupStore.setGroupOption(groupId, optionId, newValue);
        
        // Validate if needed
        if (optionId === 'browserAccessURL') {
            featureGroupStore.validateBrowserUrl(newValue, needsScraper, groupId);
        }
        
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
        
        // Browser URL needs immediate handling
        if (optionId === 'browserAccessURL') {
            handleUserInput(newValue);
        } else {
            // Other fields use debounced handling
            debouncedUserInput(newValue);
        }
    }

    // Handle romanization style changes with special provider update logic
    function handleRomanizationChange(event: any) {
        const newValue = event.detail;
        
        // Mark as user update with authority
        lastUserUpdateTime = Date.now() + 100;
        localValue = newValue;
        
        // Also update the provider if this is a style change
        if (optionId === 'style') {
            const selectedScheme = romanizationSchemes.find(s => s.name === newValue);
            if (selectedScheme) {
                // Update the provider in the group store so all features get updated
                featureGroupStore.setGroupOption(groupId, 'provider', selectedScheme.provider);
                
                console.log(`Updated provider to ${selectedScheme.provider} based on style ${newValue}`);
            }
        }
        
        // Propagate to store and parent
        propagateUserValue(newValue);
    }

    // Handle immediate changes like checkboxes
    function handleImmediateChange() {
        // Mark as user update with authority
        lastUserUpdateTime = Date.now() + 100;
        
        // Propagate to store and parent
        propagateUserValue(localValue);
    }
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
                    : groupId === 'finalOutput' 
                        ? "This option is shared across output merge features"
                        : `This option is shared across ${groupId} features`}
                {@const iconColorClass = groupId === 'subtitle' 
                    ? "text-group-subtitle" 
                    : groupId === 'finalOutput' 
                        ? "text-group-finalOutput"
                        : "text-blue-400"}
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
                on:change={handleImmediateChange}
            />
        {:else if optionDef.type === 'boolean'}
            <label class="inline-flex items-center cursor-pointer">
                <input 
                    type="checkbox" 
                    class="w-5 h-5 accent-primary rounded border-2 border-primary/50 
                           checked:bg-primary checked:border-primary
                           focus:ring-2 focus:ring-primary/30
                           transition-all duration-200
                           cursor-pointer"
                    bind:checked={localValue}
                    on:change={handleImmediateChange}
                />
            </label>
        {:else if optionDef.type === 'dropdown'}
            <!-- Remove label to avoid duplication -->
            <Dropdown
                options={optionDef.choices || []}
                value={localValue}
                on:change={handleChange}
                label=""
            />
        {:else if optionDef.type === 'string'}
            <!-- Special handling for browser URL for immediate validation -->
            {#if optionId === 'browserAccessURL'}
                <!-- Special handling for browser URL - immediate validation -->
                <TextInput
                    bind:value={localValue}
                    placeholder={optionDef.placeholder || "ws://127.0.0.1:9222/..."}
                    className="text-sm placeholder:text-gray-500"
                    on:input={(e) => {
                        // Get the input value directly from the event
                        const newValue = e.target.value;
                        
                        // Give user input a future timestamp to ensure it takes precedence
                        lastUserUpdateTime = Date.now() + 100;
                        
                        // Update local value immediately
                        localValue = newValue;
                        
                        // Store in group store and validate immediately
                        featureGroupStore.setGroupOption(groupId, optionId, newValue);
                        featureGroupStore.validateBrowserUrl(newValue, needsScraper, groupId);
                        
                        // Notify parent with user authority flag
                        dispatch('groupOptionChange', { 
                            groupId, 
                            optionId, 
                            value: newValue,
                            isUserInput: true
                        });
                    }}
                />
            {:else}
                <TextInput
                    bind:value={localValue}
                    placeholder={optionDef.placeholder}
                    className="text-sm placeholder:text-gray-500"
                    on:input={handleChange}
                />
            {/if}
        {:else if optionDef.type === 'romanizationDropdown'}
            <!-- Remove label to avoid duplication -->
            <Dropdown
                options={romanizationSchemes}
                optionKey="name"
                optionLabel="description"
                value={localValue}
                on:change={handleRomanizationChange}
                label=""
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
                    
                    <!-- Update the local value to match -->
                    {#if providerValue !== localValue && providerValue}
                        {@const ignored = featureGroupStore.setGroupOption(groupId, 'provider', providerValue)}
                        <!-- Also update local value for consistency -->
                        {#if providerValue !== localValue}
                            {@const update = (localValue = providerValue)}
                        {/if}
                    {/if}
                    
                    <!-- GitHub link if available -->
                    {#if providerGithubUrls[providerValue]}
                        <ExternalLink 
                            href={providerGithubUrls[providerValue]}
                            target="_blank"
                            rel="noopener noreferrer"
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
                    {#if providerGithubUrls[localValue]}
                        <ExternalLink 
                            href={providerGithubUrls[localValue]}
                            target="_blank"
                            rel="noopener noreferrer"
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
        grid-template-columns: 1fr 1.5fr;
        gap: 1.5rem;
        align-items: center;
        border-left: 2px solid; /* Color is applied dynamically */
        padding-left: 0.25rem; /* Reduced padding to minimize indentation */
        margin-left: 0;
    }
    
    /* Group-specific border colors using Tailwind color scheme */
    :global(.group-option[data-group-id='subtitle']) {
        border-left-color: rgba(65, 145, 250, 0.3);
    }
    
    :global(.group-option[data-group-id='finalOutput']) {
        border-left-color: rgba(0, 230, 100, 0.3);
    }
    
    .option-label {
        padding-left: 0.125rem;
    }
    
    .option-input {
        width: 100%; /* Ensure consistent width with regular options */
    }
</style>