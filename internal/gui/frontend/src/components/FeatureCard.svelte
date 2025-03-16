<script lang="ts">
    import { get } from 'svelte/store';
    import { createEventDispatcher, onMount } from 'svelte';
    
    import { formatDisplayText, type FeatureDefinition } from '../lib/featureModel';
    import { errorStore } from '../lib/errorStore';
    import { showSettings } from '../lib/stores';
    import { featureGroupStore } from '../lib/featureGroupStore';
    
    import Dropdown from './Dropdown.svelte';
    import Hovertip from './Hovertip.svelte';
    import NumericInput from './NumericInput.svelte';
    import TextInput from './TextInput.svelte';
    import ExternalLink from './ExternalLink.svelte';
    import GroupOption from './GroupOption.svelte';
    
    export let feature: FeatureDefinition;
    export let enabled = false;
    export let options: any = {};
    export let anyFeatureSelected = false;
    
    export let romanizationSchemes = [];
    export let tokenizationAllowed = false;
    export let isRomanizationAvailable = true;
    export let needsDocker = false;
    export let dockerUnreachable = false;
    export let dockerEngine = 'Docker';
    export let needsScraper = false;
    export let standardTag = '';
    
    export let providerGithubUrls = {};
    export let selectedFeatures = {};
    export let providerGroups = {};
    export let outputMergeGroups = {};

    const dispatch = createEventDispatcher();
    
    // Animation references and state
    let optionsContainer: HTMLElement;
    let optionsWrapper: HTMLElement;
    let optionsHeight = 0;
    let animating = false;
    let showNonJpnMessage = false;
    let showNotAvailableMessage = false;
    
    onMount(() => {
        // Initial measurement of the options height if enabled
        if (enabled && optionsWrapper) {
            optionsHeight = optionsWrapper.offsetHeight;
        }
    });
    
    // Update options height when they change
    $: if (enabled && optionsWrapper && !animating) {
        // Small delay to ensure DOM is updated
        setTimeout(() => {
            optionsHeight = optionsWrapper.offsetHeight;
        }, 50);
    }
    
    // Helper function for text color classes
    function getTextColorClass(enabled: boolean, anyFeatureSelected: boolean): string {
        if (enabled) return 'text-white';
        if (!anyFeatureSelected) return 'text-white';
        return 'text-white/70';
    }
    
    // Handle feature click for toggling and unavailable features
    function handleFeatureClick(event: Event) {
        // Prevent toggling if clicking inside of the options drawer
        const targetEl = event.target as HTMLElement;
        const optionsEl = optionsContainer;
        
        // Don't toggle if clicking inside the options area or on checkbox
        if (optionsEl && (optionsEl.contains(targetEl) || targetEl.tagName === 'INPUT')) {
            return;
        }

        // Define feature availability conditions
        const isRomanizationUnavailable = feature.id === 'subtitleRomanization' && !isRomanizationAvailable;
        const isSelectiveTransliterationUnavailable = feature.id === 'selectiveTransliteration' && standardTag !== 'jpn';
        // For tokenization, it's unavailable if:
        // 1. The language doesn't need tokenization (tokenizationAllowed is false) OR
        // 2. No romanization scheme is available (needed for the provider)
        const isTokenizationUnavailable = feature.id === 'subtitleTokenization' && 
                                         (!tokenizationAllowed || !isRomanizationAvailable);
        
        // Check if this feature is unavailable
        const isFeatureUnavailable = isRomanizationUnavailable || 
                                     isSelectiveTransliterationUnavailable ||
                                     isTokenizationUnavailable;
        
        if (isFeatureUnavailable) {
            // Show appropriate message based on the feature
            if (isSelectiveTransliterationUnavailable) {
                showNonJpnMessage = true;
                setTimeout(() => showNonJpnMessage = false, 5000);
            } else if (isTokenizationUnavailable) {
                showNotAvailableMessage = true;
                setTimeout(() => showNotAvailableMessage = false, 5000);
            }
            
            // Trigger shake animation
            const element = event.currentTarget as HTMLElement;
            element.classList.remove('shake-animation');
            void element.offsetWidth; // Force reflow to restart animation
            element.classList.add('shake-animation');
            
            return; // Don't allow toggling unavailable features
        }

        // Add ripple effect on click for available features
        addRippleEffect(event);

        // Toggle the feature if it's available
        enabled = !enabled;
        dispatch('enabledChange', { id: feature.id, enabled });
    }
    
    // Add material design ripple effect on click with reusable ripple elements
    const MAX_RIPPLES = 3; // Maximum number of ripple elements to create
    const ripplePool: HTMLSpanElement[] = [];
    
    function addRippleEffect(event: MouseEvent) {
        const element = event.currentTarget as HTMLElement;
        const rect = element.getBoundingClientRect();
        const x = event.clientX - rect.left;
        const y = event.clientY - rect.top;
        
        // Reuse existing ripple or create a new one if pool isn't full
        let ripple: HTMLSpanElement;
        if (ripplePool.length < MAX_RIPPLES) {
            ripple = document.createElement('span');
            ripple.classList.add('ripple-element');
            ripplePool.push(ripple);
        } else {
            // Reuse the first ripple (oldest one)
            ripple = ripplePool[0];
            // Move to end of array to maintain FIFO order
            ripplePool.push(ripplePool.shift()!);
        }
        
        // Position the ripple
        ripple.style.left = `${x}px`;
        ripple.style.top = `${y}px`;
        ripple.style.transform = 'scale(0)';
        ripple.style.opacity = '0.5';
        
        // Add to DOM if not already there
        if (!element.contains(ripple)) {
            element.appendChild(ripple);
        }
        
        // Force reflow and trigger animation
        void ripple.offsetWidth;
        
        // Apply animation with JS instead of CSS
        ripple.style.transition = 'transform 0.6s linear, opacity 0.6s linear';
        ripple.style.transform = 'scale(3)';
        ripple.style.opacity = '0';
        
        // Hide but don't remove from DOM to allow reuse
        setTimeout(() => {
            ripple.style.display = 'none';
        }, 600);
    }
    
    // Memoization cache for expensive option evaluations
    const optionVisibilityCache = new Map<string, boolean>();
    let lastContextHash = '';
    
    // Check if option should be shown based on conditions
    function shouldShowOption(optionId: string, optionDef: any): boolean {
        if (!optionDef.showCondition) return true;
        
        // Create a cache key based on the condition and current context values that affect it
        const contextValues = {
            standardTag,
            needsDocker,
            needsScraper,
            optionValues: JSON.stringify(options),
            selectedFeatures: JSON.stringify(selectedFeatures),
            featureId: feature.id
        };
        
        const contextHash = JSON.stringify(contextValues);
        const cacheKey = `${optionId}-${optionDef.showCondition}`;
        
        // If context changed, clear cache
        if (lastContextHash !== contextHash) {
            optionVisibilityCache.clear();
            lastContextHash = contextHash;
        }
        
        // Check cache first
        if (optionVisibilityCache.has(cacheKey)) {
            return optionVisibilityCache.get(cacheKey);
        }
        
        // Context object for evaluating conditions
        const context = {
            standardTag,
            needsDocker,
            needsScraper,
            romanizationSchemes,
            selectedFeatures
        };
        
        // Feature options reference for conditions 
        const featureData = {
            [feature.id]: options,
            id: feature.id // Include the feature id directly for easier checking
        };
        
        // Simple expression evaluator
        try {
            // Replace context variables with their values
            const prepared = optionDef.showCondition
                .replace(/context\.([a-zA-Z0-9_]+)/g, (_, prop) => {
                    return JSON.stringify(context[prop]);
                })
                .replace(/feature\.([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)/g, (_, featureId, propId) => {
                    return JSON.stringify(featureData[featureId][propId]);
                })
                .replace(/feature\.id/g, () => {
                    return JSON.stringify(feature.id);
                });
            
            // Use Function constructor to evaluate the expression
            const result = new Function('return ' + prepared)();
            
            // Cache the result
            optionVisibilityCache.set(cacheKey, result);
            return result;
        } catch (error) {
            console.error('Error evaluating condition:', optionDef.showCondition, error);
            optionVisibilityCache.set(cacheKey, false);
            return false;
        }
    }
    
    // Handle dropdown changes
    function handleDropdownChange(optionId: string, value: string) {
        options[optionId] = value;
        dispatch('optionChange', { featureId: feature.id, optionId, value });
    }
    
    // Cached visible options
    let visibleOptionsCache: string[] = [];
    let visibleOptionsDirty = true;
    
    // Mark cache as dirty when dependencies change
    $: {
        if (feature || options || standardTag || selectedFeatures || outputMergeGroups) {
            visibleOptionsDirty = true;
        }
    }
    
    // Get visible options for this feature (excluding merge options if not active feature)
    function getVisibleOptions(): string[] {
        // Use cache if available and not dirty
        if (!visibleOptionsDirty && visibleOptionsCache.length > 0) {
            return visibleOptionsCache;
        }
        
        // Check if this feature is in a merge group and if it's the active feature for that group
        const isInMergeGroup = feature.outputMergeGroup;
        const isMergeActive = isInMergeGroup && 
                              outputMergeGroups[feature.outputMergeGroup] && 
                              outputMergeGroups[feature.outputMergeGroup].includes(feature.id);
                              
        // If the feature is in a merge group but not the active feature for showing merge options,
        // we need to filter out the merge options
        const shouldHideMergeOptions = isInMergeGroup && !options.mergeOutputFiles && !options.mergingFormat;
        
        let optionList;
        if (feature.optionOrder) {
            optionList = feature.optionOrder.filter(optionId => 
                feature.options[optionId] && shouldShowOption(optionId, feature.options[optionId])
            );
        } else {
            optionList = Object.keys(feature.options).filter(optionId => 
                shouldShowOption(optionId, feature.options[optionId])
            );
        }
        
        // Filter out merge options if this is not the active merge feature
        if (shouldHideMergeOptions) {
            optionList = optionList.filter(optionId => 
                optionId !== 'mergeOutputFiles' && optionId !== 'mergingFormat'
            );
        }
        
        // Update cache
        visibleOptionsCache = optionList;
        visibleOptionsDirty = false;
        
        return optionList;
    }
    
    // Memoized version of hasVisibleOptions
    let hasVisibleOptionsCache = false;
    
    // Check if the feature has any visible options
    function hasVisibleOptions(): boolean {
        if (!visibleOptionsDirty && visibleOptionsCache.length > 0) {
            return hasVisibleOptionsCache;
        }
        
        hasVisibleOptionsCache = getVisibleOptions().length > 0;
        return hasVisibleOptionsCache;
    }
    
    // When enabled status changes, animate the height
    $: {
        if (optionsContainer) {
            // Only animate if there are visible options
            if (hasVisibleOptions()) {
                animating = true;
                
                if (enabled) {
                    // Opening animation
                    // First measure height of the content
                    if (optionsWrapper) {
                        setTimeout(() => {
                            optionsHeight = optionsWrapper.offsetHeight;
                            optionsContainer.style.height = optionsHeight + 'px';
                            
                            // Animation complete
                            setTimeout(() => {
                                animating = false;
                            }, 350);
                        }, 10);
                    }
                } else {
                    // Closing animation
                    optionsContainer.style.height = '0px';
                    
                    // Animation complete
                    setTimeout(() => {
                        animating = false;
                    }, 350);
                }
            } else {
                // No options to show, keep container closed
                optionsContainer.style.height = '0px';
                animating = false;
            }
        }
    }
    
    $: displayLabel = feature.id === 'selectiveTransliteration' && standardTag === 'jpn' 
    ? 'Selective Kanji Transliteration' 
    : feature.label;

    // Determine if feature is disabled
    $: isFeatureDisabled = ((!isRomanizationAvailable && feature.id === 'subtitleRomanization') || 
                           (standardTag !== 'jpn' && feature.id === 'selectiveTransliteration') ||
                           (feature.id === 'subtitleTokenization' && (!tokenizationAllowed || !isRomanizationAvailable)));
</script>

<div class="feature-card bg-white/5 rounded-lg
           transition-all duration-300 ease-out transform ripple
           relative {enabled ? '' : 'overflow-hidden'}
           {isFeatureDisabled ? 'disabled opacity-50 cursor-not-allowed' : 'hover:shadow-glow-hover'}"
     class:shadow-glow-strong={enabled && !anyFeatureSelected}
     class:shadow-glow={enabled}
     class:opacity-30={anyFeatureSelected && !enabled}
     tabindex="0"
     role="region"
     aria-expanded={enabled}
     aria-labelledby={`feature-heading-${feature.id}`}
     aria-checked={enabled}
     on:keydown={(e) => {
         if (e.key === 'Enter' || e.key === ' ') {
             e.preventDefault();
             if (!isFeatureDisabled) {
                 handleFeatureClick(e);
             }
         }
     }}
     on:click={handleFeatureClick}
>
    <div class="p-4 border-b border-white/10">
        <div class="flex items-center gap-3 cursor-pointer group
                  {isFeatureDisabled ? 'cursor-not-allowed' : ''}">
            <input
                type="checkbox"
                class="w-4 h-4 accent-primary/90 hover:accent-primary transition-colors duration-200"
                bind:checked={enabled}
                disabled={isFeatureDisabled}
                on:change={(e) => {
                    e.stopPropagation();
                    dispatch('enabledChange', { id: feature.id, enabled });
                }}
            />
            <span class="text-lg transition-all duration-300 {getTextColorClass(enabled, anyFeatureSelected)}
                       group-hover:text-primary"
                  class:font-semibold={enabled || !anyFeatureSelected}>
                {displayLabel || formatDisplayText(feature.id)}
            </span>
        </div>
        
        {#if enabled && get(errorStore).some(e => e.id === `provider-${feature.id}`)}
            <div class="mt-2 flex items-center gap-2 text-red-400 text-xs pl-7">
                <span class="material-icons text-[14px]">warning</span>
                <span>
                    {get(errorStore).find(e => e.id === `provider-${feature.id}`)?.message}
                    <button 
                        class="ml-1 text-primary hover:text-primary/80 transition-colors"
                        on:click={() => $showSettings = true}
                    >
                        Configure API Keys
                    </button>
                </span>
            </div>
        {/if}
        
        <!-- Feature-specific messages -->
        {#if feature.id === 'subtitleRomanization'}
            <!-- Only show Docker status if this feature is showing provider options -->
            {#if enabled && hasVisibleOptions() && needsDocker && !dockerUnreachable}
                <div class="mt-2 flex items-left text-xs font-bold text-green-300 pl-7">
                    🟢 {dockerEngine} is running and reachable.	&nbsp;<span class="relative top-[-3px]"> 🐳</span>
                </div>
            {/if}
            {#if needsDocker && dockerUnreachable}
                <div class="mt-2 flex items-left text-xs font-bold text-red-500 pl-7">
                    🔴 {dockerEngine} is required but not reachable. Please make sure it is installed and running.
                </div>
            {:else if !standardTag}
                <div class="mt-2 flex items-left text-xs text-white/80 pl-7">
                    Please select a language to proceed.
                </div>
            {:else if !isRomanizationAvailable}
                <div class="mt-2 flex items-left text-xs text-white/80 pl-7">
                    Sorry, no transliteration scheme has been implemented for this language yet! 
                </div>
                <div class="mt-2 flex items-left text-xs text-white/80 pl-7">
                    <ExternalLink 
                        href="https://github.com/tassa-yoniso-manasi-karoto/translitkit"
                        className="hover:text-white/60 transition-colors duration-200"
                        target="_blank"
                        rel="noopener noreferrer">
                        Pull requests and feedback are welcome.
                    </ExternalLink>
                </div>
            {/if}
        {:else if feature.id === 'selectiveTransliteration'}
            <!-- Only show Docker status if this feature is showing provider options -->
            {#if enabled && hasVisibleOptions() && getVisibleOptions().includes('provider') && needsDocker && !dockerUnreachable}
                <div class="mt-2 flex items-left text-xs font-bold text-green-300 pl-7">
                    🟢 {dockerEngine} is running and reachable.	&nbsp;<span class="relative top-[-3px]"> 🐳</span>
                </div>
            {/if}
            {#if needsDocker && dockerUnreachable}
                <div class="mt-2 flex items-left text-xs font-bold text-red-500 pl-7">
                    🔴 {dockerEngine} is required but not reachable. Please make sure it is installed and running.
                </div>
            {:else if !standardTag}
                <div class="mt-2 flex items-left text-xs text-white/80 pl-7">
                    Please select a language to proceed.
                </div>
            {:else if standardTag !== 'jpn' && showNonJpnMessage}
                <div class="mt-2 flex items-left text-xs text-white/80 pl-7">
                    Sorry, selective transliteration is currently only available for Japanese Kanji transliteration!
                </div>
            {/if}
        {:else if feature.id === 'subtitleTokenization'}
            <!-- Only show Docker status if this feature is showing provider options -->
            {#if enabled && hasVisibleOptions() && getVisibleOptions().includes('provider') && needsDocker && !dockerUnreachable}
                <div class="mt-2 flex items-left text-xs font-bold text-green-300 pl-7">
                    🟢 {dockerEngine} is running and reachable.	&nbsp;<span class="relative top-[-3px]"> 🐳</span>
                </div>
            {/if}
            {#if needsDocker && dockerUnreachable}
                <div class="mt-2 flex items-left text-xs font-bold text-red-500 pl-7">
                    🔴 {dockerEngine} is required but not reachable. Please make sure it is installed and running.
                </div>
            {/if}
            {#if showNotAvailableMessage}
                <div class="mt-2 flex items-left text-xs text-white/80 pl-7">
                    Sorry, no tokenizer is implemented for this language at this time!
                </div>
            {/if}
        {/if}

        <!-- Dependency messages when a feature depends on dubtitles -->
        {#if feature.dependentFeature && selectedFeatures[feature.dependentFeature] && enabled}
            <div class="mt-2 flex items-left text-xs text-blue-300 pl-7">
                <span class="material-icons text-[14px] mr-1">info</span>
                {feature.dependencyMessage}
            </div>
        {/if}

        <!-- Output merge group banner (shown for all features in the merge group) -->
        {#if feature.outputMergeGroup && feature.showMergeBanner && enabled}
            <div class="mt-2 flex items-left text-xs text-green-300 pl-7">
                <span class="material-icons text-[14px] mr-1">merge_type</span>
                All processed outputs will be merged in the final video
            </div>
        {/if}
    </div>
    
    <!-- Options drawer with slide animation - only displayed if the feature has visible options -->
    {#if hasVisibleOptions()}
    <div
    bind:this={optionsContainer} 
    class="overflow-hidden" 
    style="height: {optionsHeight}px; transition: height 350ms cubic-bezier(0.25, 1, 0.5, 1)"
    hidden={!enabled}
    >
        <div bind:this={optionsWrapper} class="p-4">
            <div class="options-grid">
                {#each getVisibleOptions() as optionId}
                    {@const optionDef = feature.options[optionId]}
                    {@const value = options[optionId]}
                    
                    {#if feature.featureGroups && feature.groupSharedOptions && 
                         feature.featureGroups.some(groupId => {
                           // Ensure this feature is part of the group in featureGroupStore
                           if (!featureGroupStore.isFeatureEnabled(groupId, feature.id)) {
                             featureGroupStore.addFeatureToGroup(groupId, feature.id);
                           }
                           
                           // Check if this option is shared in the group
                           return feature.groupSharedOptions[groupId]?.includes(optionId);
                         }) &&
                         feature.featureGroups.some(groupId => featureGroupStore.isActiveDisplayFeature(groupId, feature.id))}
                        <!-- Group option takes the entire row - only shown for the active display feature -->
                        {@const groupId = feature.featureGroups.find(gId => 
                            feature.groupSharedOptions[gId]?.includes(optionId) && 
                            featureGroupStore.isActiveDisplayFeature(gId, feature.id)
                        )}
                        <div class="mb-4 w-full">
                            <GroupOption 
                                {groupId}
                                {optionId}
                                optionDef={optionDef}
                                value={featureGroupStore.getGroupOption(groupId, optionId) ?? options[optionId]}
                                {needsDocker}
                                {needsScraper}
                                {romanizationSchemes}
                                on:groupOptionChange={event => {
                                    const { groupId, optionId, value } = event.detail;
                                    options[optionId] = value;
                                    dispatch('optionChange', { 
                                        featureId: feature.id, 
                                        optionId, 
                                        value,
                                        isGroupOption: true,
                                        groupId
                                    });
                                }}
                            />
                        </div>
                    {:else}
                        <!-- Regular option with label and input -->
                        <div class="option-row">
                            <div class="option-label">
                                {#if optionId === 'provider'}
                                    <span class="text-gray-300 text-sm text-left">Provider</span>
                                {:else}
                                    <span class="text-gray-300 text-sm text-left flex items-center gap-2">
                                        {optionDef.label}
                                        {#if optionDef.hovertip}
                                            <Hovertip message={optionDef.hovertip}>
                                                <span slot="trigger" class="material-icons text-primary/70 cursor-help pr-1 leading-none material-icon-adjust">
                                                    help_outline
                                                </span>
                                            </Hovertip>
                                        {/if}
                                    </span>
                                {/if}
                            </div>
                            <div class="option-input">
                                {#if optionDef.type === 'number'}
                                    <NumericInput 
                                        bind:value={options[optionId]}
                                        step={optionDef.step || '1'}
                                        min={optionDef.min}
                                        max={optionDef.max}
                                        placeholder={optionDef.placeholder}
                                        on:change={() => dispatch('optionChange', { featureId: feature.id, optionId, value: options[optionId] })}
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
                                            bind:checked={options[optionId]}
                                            on:change={() => dispatch('optionChange', { featureId: feature.id, optionId, value: options[optionId] })}
                                        />
                                    </label>
                                {:else if optionDef.type === 'dropdown'}
                                    <Dropdown
                                        options={optionDef.choices || []}
                                        value={options[optionId]}
                                        on:change={(e) => handleDropdownChange(optionId, e.detail)}
                                        label={optionDef.label}
                                    />
                                {:else if optionDef.type === 'string'}
                                    {#if optionId === 'initialPrompt'}
                                        <textarea
                                            bind:value={options[optionId]}
                                            class="w-full bg-sky-dark/50 border-2 border-primary/30 rounded-md px-3 py-2 text-sm font-medium
                                                focus:border-primary focus:ring-2 focus:ring-primary/30 hover:border-primary/50 focus:outline-none
                                                transition-all duration-200 placeholder:text-gray-500"
                                            rows="3"
                                            maxlength="850"
                                            placeholder={optionDef.placeholder}
                                            on:input={() => dispatch('optionChange', { featureId: feature.id, optionId, value: options[optionId] })}
                                        />
                                    {:else}
                                        <TextInput
                                            bind:value={options[optionId]}
                                            placeholder={optionDef.placeholder}
                                            className="text-sm placeholder:text-gray-500"
                                            on:input={() =>
                                                dispatch('optionChange', {
                                                    featureId: feature.id,
                                                    optionId,
                                                    value: options[optionId]
                                                })
                                            }
                                        />
                                    {/if}
                                {:else if optionDef.type === 'romanizationDropdown'}
                                    <Dropdown
                                        options={romanizationSchemes}
                                        optionKey="name"
                                        optionLabel="description"
                                        value={options[optionId]}
                                        on:change={(e) => {
                                            handleDropdownChange(optionId, e.detail);
                                            const selectedScheme = romanizationSchemes.find(s => s.name === e.detail);
                                            if (selectedScheme) {
                                                options['provider'] = selectedScheme.provider;
                                                dispatch('optionChange', { featureId: feature.id, optionId: 'provider', value: selectedScheme.provider });
                                            }
                                        }}
                                        label="Select style"
                                    />
                                {:else if optionDef.type === 'provider'}
                                    {@const provider = options['style'] ? (romanizationSchemes.find(s => s.name === options['style'])?.provider || '') : 'ichiran'}
                                    <div class="w-full px-3 py-1 text-sm inline-flex font-bold text-white/90 items-center justify-center gap-2">
                                        {provider}
                                        {#if providerGithubUrls[provider]}
                                            <ExternalLink 
                                                href={providerGithubUrls[provider]}
                                                target="_blank"
                                                rel="noopener noreferrer"
                                                className="text-primary/70 hover:text-primary transition-colors duration-200"
                                                title="View provider repository">
                                                <svg viewBox="0 0 16 16" class="w-5 h-5 fill-primary">
                                                    <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                                                </svg>
                                            </ExternalLink>
                                        {/if}
                                    </div>
                                {/if}
                            </div>
                        </div>
                    {/if}
                {/each}
            </div>
        </div>
    </div>
    {/if}

    <!-- Ripple effect container -->
    <style>
        .ripple-element {
            position: absolute;
            border-radius: 50%;
            background-color: rgba(159, 110, 247, 0.1);
            width: 100px;
            height: 100px;
            margin-top: -50px;
            margin-left: -50px;
            animation: ripple 0.6s linear;
            pointer-events: none;
        }

        @keyframes ripple {
            0% {
                transform: scale(0);
                opacity: 0.5;
            }
            100% {
                transform: scale(3);
                opacity: 0;
            }
        }
    </style>
</div>

<style>
    @import './featureStyles.css';
    
    /* Standardized grid layout for consistent spacing and alignment */
    .options-grid {
        display: flex;
        flex-direction: column;
        gap: 0.75rem;
    }
    
    .option-row {
        display: grid;
        grid-template-columns: 1fr 1.5fr; /* Match with group option layout */
        gap: 1.5rem;
        align-items: center;
        /* Add very subtle left indentation to match with group options */
        padding-left: 0.125rem;
    }
    
    .option-label {
        display: flex;
        align-items: center;
    }
    
    .option-input {
        width: 100%; /* Ensure consistent width with group options */
    }
</style>