<script lang="ts">
    import { get } from 'svelte/store';
    import { createEventDispatcher, onMount, tick, afterUpdate } from 'svelte';
    
    import { formatDisplayText, type FeatureDefinition } from '../lib/featureModel';
    import { errorStore } from '../lib/errorStore';
    import { showSettings } from '../lib/stores';
    import { featureGroupStore } from '../lib/featureGroupStore';
    
    // Class for message items to keep styling consistent
    const messageItemClass = "flex items-center gap-2 py-2 px-3 first:pt-2 last:pb-2 hover:bg-white/5 transition-colors duration-200 group";
    
    import Dropdown from './Dropdown.svelte';
    import Hovertip from './Hovertip.svelte';
    import NumericInput from './NumericInput.svelte';
    import TextInput from './TextInput.svelte';
    import ExternalLink from './ExternalLink.svelte';
    import GroupIcon from './icons/GroupIcon.svelte';
    import DockerIcon from './icons/DockerIcon.svelte';
    import DockerUnavailableIcon from './icons/DockerUnavailableIcon.svelte';
    import GroupOption from './GroupOption.svelte';
    
    // No longer needed - direct DOM check is more reliable
    
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

    const dispatch = createEventDispatcher();
    
    // Animation references and state
    let optionsContainer: HTMLElement;
    let optionsWrapper: HTMLElement;
    let optionsHeight = 0;
    let animating = false;
    let showNonJpnMessage = false;
    let showNotAvailableMessage = false;
    
    // Store reference to the feature card element
    let featureCardElement: HTMLElement;
    
    // Store if this is the topmost feature for any of its groups
    let isTopmostFeatureForAnyGroup = false;
    
    // Function to check if this feature is the topmost for a given group using canonical order
    function checkTopmostFeatureStatus() {
        if (!feature.featureGroups || !feature.featureGroups.length || !enabled) {
            isTopmostFeatureForAnyGroup = false;
            return;
        }
        
        // For each group this feature belongs to, check if it's the topmost
        let foundTopmost = false;
        for (const groupId of feature.featureGroups) {
            // Use the store's isTopmostInGroup method instead of DOM queries
            if (featureGroupStore.isTopmostInGroup(groupId, feature.id)) {
                foundTopmost = true;
                break;
            }
        }
        
        isTopmostFeatureForAnyGroup = foundTopmost;
        console.log(`Feature ${feature.id} topmost status check: ${isTopmostFeatureForAnyGroup}`);
    }
    
    onMount(() => {
        // Initial measurement of the options height if enabled
        if (enabled && optionsWrapper) {
            optionsHeight = optionsWrapper.offsetHeight;
        }
    });
    
    // After all components are updated, check topmost status only when necessary
    let lastEnabledState = false;
    let lastCheckTime = 0;
    
    afterUpdate(() => {
        const now = Date.now();
        // Only check if enabled state changed or if it's been more than 300ms since last check
        if (enabled && feature.featureGroups?.length && 
            (enabled !== lastEnabledState || now - lastCheckTime > 300)) {
            
            lastEnabledState = enabled;
            lastCheckTime = now;
            // Schedule the check slightly later to ensure DOM is fully updated
            setTimeout(checkTopmostFeatureStatus, 50);
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
        
        // Find which group this option belongs to (if any)
        let optionGroup = null;
        if (feature.groupSharedOptions) {
            for (const [groupId, options] of Object.entries(feature.groupSharedOptions)) {
                if (options.includes(optionId)) {
                    optionGroup = groupId;
                    break;
                }
            }
        }
        
        // Check if this feature is the topmost in the specific group for this option
        let isTopmostForThisOption = false;
        if (optionGroup && enabled) {
            isTopmostForThisOption = featureGroupStore.isTopmostInGroup(optionGroup, feature.id);
            console.log(`Option ${optionId} belongs to group ${optionGroup}, isTopmost: ${isTopmostForThisOption}`);
        }
        
        // For backwards compatibility, also check topmost status in any group
        let isTopmostInAnyGroup = false;
        if (feature.featureGroups && feature.featureGroups.length > 0 && enabled) {
            for (const groupId of feature.featureGroups) {
                if (featureGroupStore.isTopmostInGroup(groupId, feature.id)) {
                    isTopmostInAnyGroup = true;
                    break;
                }
            }
        }
        
        const contextValues = {
            standardTag,
            needsDocker,
            needsScraper,
            optionValues: JSON.stringify(options),
            selectedFeatures: JSON.stringify(selectedFeatures),
            featureId: feature.id,
            isTopmostInGroup: optionGroup ? isTopmostForThisOption : isTopmostInAnyGroup
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
            selectedFeatures,
            isTopmostInGroup: optionGroup ? isTopmostForThisOption : isTopmostInAnyGroup
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
        if (feature || options || standardTag || selectedFeatures) {
            visibleOptionsDirty = true;
        }
    }
    
    // Get visible options for this feature
    function getVisibleOptions(): string[] {
        // Use cache if available and not dirty
        if (!visibleOptionsDirty && visibleOptionsCache.length > 0) {
            return visibleOptionsCache;
        }
        
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
    
    // When enabled status changes, animate the height and check topmost status
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
                                
                                // Check topmost status after animation completes
                                if (feature.featureGroups?.length) {
                                    checkTopmostFeatureStatus();
                                }
                            }, 350);
                        }, 10);
                    }
                } else {
                    // Closing animation
                    optionsContainer.style.height = '0px';
                    
                    // Reset topmost status when disabled
                    isTopmostFeatureForAnyGroup = false;
                    
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
    
    // Reset topmost status when disabled
    $: if (!enabled && isTopmostFeatureForAnyGroup) {
        isTopmostFeatureForAnyGroup = false;
    }
    
    $: displayLabel = feature.id === 'selectiveTransliteration' && standardTag === 'jpn' 
    ? 'Selective Kanji Transliteration' 
    : feature.label;

    // Determine if feature is disabled
    $: isFeatureDisabled = ((!isRomanizationAvailable && feature.id === 'subtitleRomanization') || 
                           (standardTag !== 'jpn' && feature.id === 'selectiveTransliteration') ||
                           (feature.id === 'subtitleTokenization' && (!tokenizationAllowed || !isRomanizationAvailable)));
    
    // Track option changes that need animation refresh
    $: if (options && Object.keys(options).some(key => key === 'mergeOutputFiles' || key.startsWith('docker'))) {
        // Schedule animation refresh after options change
        setTimeout(() => {
            const messageCard = document.querySelector('.glassmorphism-card');
            if (messageCard) {
                // Toggle animation class
                messageCard.classList.add('reanimating');
                setTimeout(() => {
                    messageCard.classList.remove('reanimating');
                }, 50);
            }
        }, 10);
    }
    
    // Helper function to determine if we should show feature messages
    function hasFeatureMessages() {
        // API Provider error messages
        if (enabled && get(errorStore).some(e => e.id === `provider-${feature.id}`)) {
            return true;
        }
        
        // Feature-specific messages
        if (feature.id === 'subtitleRomanization') {
            if ((enabled && hasVisibleOptions() && needsDocker && !dockerUnreachable) ||
                (needsDocker && dockerUnreachable) ||
                (!standardTag) ||
                (!isRomanizationAvailable)) {
                return true;
            }
        } else if (feature.id === 'selectiveTransliteration') {
            if ((enabled && hasVisibleOptions() && getVisibleOptions().includes('provider') && needsDocker && !dockerUnreachable) ||
                (needsDocker && dockerUnreachable) ||
                (!standardTag) ||
                (standardTag !== 'jpn' && showNonJpnMessage)) {
                return true;
            }
        } else if (feature.id === 'subtitleTokenization') {
            if ((enabled && hasVisibleOptions() && getVisibleOptions().includes('provider') && needsDocker && !dockerUnreachable) ||
                (needsDocker && dockerUnreachable) ||
                showNotAvailableMessage) {
                return true;
            }
        }
        
        // Dependency messages
        if (feature.dependentFeature && selectedFeatures[feature.dependentFeature] && enabled) {
            return true;
        }
        
        // Output merge group banner - only shown when merge option is enabled
        if (feature.outputMergeGroup && feature.showMergeBanner && enabled && options.mergeOutputFiles) {
            return true;
        }
        
        return false;
    }
</script>

<div class="feature-card bg-white/5 rounded-lg
           transition-all duration-300 ease-out transform ripple
           relative {enabled ? '' : 'overflow-hidden'}
           {isFeatureDisabled ? 'disabled opacity-50 cursor-not-allowed' : ''}"
     class:shadow-glow-strong={enabled && !anyFeatureSelected}
     class:shadow-glow={enabled}
     class:opacity-30={anyFeatureSelected && !enabled}
     tabindex="0"
     role="region"
     aria-expanded={enabled}
     aria-labelledby={`feature-heading-${feature.id}`}
     aria-checked={enabled}
     data-feature-id={feature.id}
     bind:this={featureCardElement}
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
    <div class="p-4 border-b border-white/10
                {(enabled && hasFeatureMessages()) ? 'pb-1' : 'pb-4'}"
    >
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
                       group-hover:text-primary-200"
                  class:font-semibold={enabled || !anyFeatureSelected}>
                {displayLabel || formatDisplayText(feature.id)}
            </span>
        </div>
        
        {#if hasFeatureMessages()}
        <div class="feature-message-card ml-7 w-auto animate-fadeIn">
                <div class="glassmorphism-card">
                    <!-- API Provider error messages -->
                    {#if enabled && get(errorStore).some(e => e.id === `provider-${feature.id}`)}
                        <div class={messageItemClass}>
                            <span class="material-icons text-[14px] text-error-all mt-0.5 group-hover:animate-subtlePulse">
                                warning
                            </span>
                            <div class="flex-1 text-xs text-white/90">
                                <span>{get(errorStore).find(e => e.id === `provider-${feature.id}`)?.message || ''}</span>
                                <button 
                                    class="ml-1 text-primary hover:text-primary-300 transition-colors duration-200 underline"
                                    on:click={() => $showSettings = true}>
                                    Configure API Keys
                                </button>
                            </div>
                        </div>
                    {/if}
                    
                    <!-- Feature-specific messages -->
                    {#if feature.id === 'subtitleRomanization'}
                        <!-- Docker status banners -->
                        {#if enabled && hasVisibleOptions() && needsDocker && !dockerUnreachable}
                            <div class={messageItemClass}>
                                <DockerIcon size="1.5em" className="text-blue-400" />
                                <div class="flex-1 text-xs text-white/90">
                                    <span>{dockerEngine} is running and reachable.</span>
                                </div>
                            </div>
                        {/if}
                        
                        {#if needsDocker && dockerUnreachable}
                            <div class={messageItemClass}>
                                <DockerUnavailableIcon size="1.5em" className="text-blue-400" />
                                <div class="flex-1 text-xs text-[#ff0000] font-bold">
                                    <span>{dockerEngine} is required but not reachable. Please make sure it is installed and running.</span>
                                </div>
                            </div>
                        {:else if !standardTag}
                            <div class={messageItemClass}>
                                <span class="material-icons text-[14px] text-primary mt-0.5 group-hover:animate-subtlePulse">
                                    info
                                </span>
                                <div class="flex-1 text-xs text-white/90">
                                    <span>Please select a language to proceed.</span>
                                </div>
                            </div>
                        {:else if !isRomanizationAvailable}
                            <div class={messageItemClass}>
                                <span class="material-icons text-[14px] text-primary mt-0.5 group-hover:animate-subtlePulse">
                                    info
                                </span>
                                <div class="flex-1 text-xs text-white/90">
                                    <span>Sorry, no transliteration scheme has been implemented for this language yet!</span>
                                </div>
                            </div>
                            <div class={messageItemClass}>
                                <span class="w-[14px] ml-3"></span>
                                <div class="flex-1 text-xs text-white/90">
                                    <span>Pull requests and feedback are welcome.</span>
                                    <a href="https://github.com/tassa-yoniso-manasi-karoto/translitkit" 
                                       class="ml-1 text-primary hover:text-primary-300 transition-colors duration-200"
                                       target="_blank" 
                                       rel="noopener noreferrer">
                                        Learn more
                                    </a>
                                </div>
                            </div>
                        {/if}
                    
                    {:else if feature.id === 'selectiveTransliteration'}
                        <!-- Docker status banners -->
                        {#if enabled && hasVisibleOptions() && getVisibleOptions().includes('provider') && needsDocker && !dockerUnreachable}
                            <div class={messageItemClass}>
                                <DockerIcon size="1.5em" className="text-blue-400" />
                                <div class="flex-1 text-xs text-white/90">
                                    <span>{dockerEngine} is running and reachable.</span>
                                </div>
                            </div>
                        {/if}
                        
                        {#if needsDocker && dockerUnreachable}
                            <div class={messageItemClass}>
                                <DockerUnavailableIcon size="1.5em" className="text-blue-400" />
                                <div class="flex-1 text-xs text-[#ff0000] font-bold">
                                    <span>{dockerEngine} is required but not reachable. Please make sure it is installed and running.</span>
                                </div>
                            </div>
                        {:else if !standardTag}
                            <div class={messageItemClass}>
                                <span class="material-icons text-[14px] text-primary mt-0.5 group-hover:animate-subtlePulse">
                                    info
                                </span>
                                <div class="flex-1 text-xs text-white/90">
                                    <span>Please select a language to proceed.</span>
                                </div>
                            </div>
                        {:else if standardTag !== 'jpn' && showNonJpnMessage}
                            <div class={messageItemClass}>
                                <span class="material-icons text-[14px] text-error-task mt-0.5 group-hover:animate-subtlePulse">
                                    warning
                                </span>
                                <div class="flex-1 text-xs text-white/90">
                                    <span>Sorry, selective transliteration is currently only available for Japanese Kanji transliteration!</span>
                                </div>
                            </div>
                        {/if}
                    
                    {:else if feature.id === 'subtitleTokenization'}
                        <!-- Docker status banners -->
                        {#if enabled && hasVisibleOptions() && getVisibleOptions().includes('provider') && needsDocker && !dockerUnreachable}
                            <div class={messageItemClass}>
                                <DockerIcon size="1.5em" className="text-blue-400" />
                                <div class="flex-1 text-xs text-white/90">
                                    <span>{dockerEngine} is running and reachable.</span>
                                </div>
                            </div>
                        {/if}
                        
                        {#if needsDocker && dockerUnreachable}
                            <div class={messageItemClass}>
                                <DockerUnavailableIcon size="1.5em" className="text-blue-400" />
                                <div class="flex-1 text-xs text-[#ff0000] font-bold">
                                    <span>{dockerEngine} is required but not reachable. Please make sure it is installed and running.</span>
                                </div>
                            </div>
                        {/if}
                        
                        {#if showNotAvailableMessage}
                            <div class={messageItemClass}>
                                <span class="material-icons text-[14px] text-error-task mt-0.5 group-hover:animate-subtlePulse">
                                    warning
                                </span>
                                <div class="flex-1 text-xs text-white/90">
                                    <span>Sorry, no tokenizer is implemented for this language at this time!</span>
                                </div>
                            </div>
                        {/if}
                    {/if}

                    <!-- Dependency messages when a feature depends on dubtitles -->
                    {#if feature.dependentFeature && selectedFeatures[feature.dependentFeature] && enabled}
                        <div class={messageItemClass}>
                            <span class="material-icons text-[14px] text-log-info mt-0.5 group-hover:animate-subtlePulse">
                                link
                            </span>
                            <div class="flex-1 text-xs text-white/90">
                                <span>{feature.dependencyMessage}</span>
                            </div>
                        </div>
                    {/if}

                    <!-- Output merge group banner (shown only when merge option is enabled) -->
                    {#if feature.outputMergeGroup && feature.showMergeBanner && enabled && options.mergeOutputFiles}
                        <div class={messageItemClass} key="{options.mergeOutputFiles}">
                            <span class="material-icons text-[14px] text-primary mt-0.5 group-hover:animate-subtlePulse">
                                merge_type
                            </span>
                            <div class="flex-1 text-xs text-white/90">
                                <span>All created content will be merged with originals in a new video</span>
                            </div>
                        </div>
                    {/if}
                </div>
        </div>
        {/if}
    </div>
    
    <!-- Options drawer with slide animation - only displayed if the feature has visible options -->
    {#if hasVisibleOptions()}
    {@const _logVisible = console.log('hasVisibleOptions() returned true')}
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
                    
                    <!-- Check if this option is a group shared option -->
                    {@const isGroupOption = feature.featureGroups && 
                        feature.groupSharedOptions && 
                        feature.featureGroups.some(groupId => 
                            feature.groupSharedOptions[groupId]?.includes(optionId)
                        )}
                    
                    <!-- Find the group that this option belongs to (if any) -->
                    {@const groupId = isGroupOption ? 
                        feature.featureGroups.find(gId => 
                            feature.groupSharedOptions[gId]?.includes(optionId)
                        ) : null}
                    
                    <!-- Ensure this feature is registered in the group -->
                    {#if isGroupOption && groupId}
                        {@const _ensureInGroup = 
                            featureGroupStore.addFeatureToGroup(groupId, feature.id)}
                    {/if}
                    
                    <!-- No additional checks needed - isTopmostInGroup is the source of truth -->
                    
                    {#if isGroupOption && groupId && featureGroupStore.isTopmostInGroup(groupId, feature.id)}
                        <!-- Using canonical ordering from the feature store -->
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
                                    // Update local option value for reactivity
                                    options[optionId] = value;
                                    
                                    // Dispatch to parent with all necessary metadata
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
                                    <span class="text-gray-300 text-sm text-left flex items-center gap-2">
                                        Provider
                                        <!-- Group icon if this is a shared option -->
                                        {#if feature.featureGroups && feature.groupSharedOptions && 
                                             feature.featureGroups.some(gId => feature.groupSharedOptions[gId]?.includes('provider'))}
                                            <Hovertip message="This option is shared across subtitle features">
                                                <span slot="trigger" class="cursor-help">
                                                    <GroupIcon size="1.5em" className="text-blue-400" />
                                                </span>
                                            </Hovertip>
                                        {/if}
                                    </span>
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
                                        
                                        <div class="flex items-center justify-center gap-2">
                                            <!-- GitHub link if available -->
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
    
    .feature-message-card {
        position: relative;
        left: 0;
        text-align: left;
    }
    
    /* Enhanced glassmorphism effect for feature message card 
    .glassmorphism-card {
        background-color: rgba(31, 41, 55, 0.15);
        backdrop-filter: blur(16px);
        -webkit-backdrop-filter: blur(16px);
        border-radius: 0.375rem;
        overflow: hidden;
        display: inline-block;
        max-width: max-content;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
        transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
    }
    
    .glassmorphism-card:hover {
        background-color: rgba(31, 41, 55, 0.22);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
        transform: translateY(-1px);
    }*/
    
    /* Style for the dividing dotted lines */
    .glassmorphism-card > div:not(:first-child) {
        border-top-width: 1px;
        border-color: rgba(255, 255, 255, 0.1);
        border-style: dotted;
    }
    
    /* Staggered message animations */
    .glassmorphism-card > div {
        animation: messageIn 0.3s cubic-bezier(0.16, 1, 0.3, 1) both;
        opacity: 0;
    }
    
    .glassmorphism-card > div:nth-child(1) {
        animation-delay: 0s;
    }
    
    .glassmorphism-card > div:nth-child(2) {
        animation-delay: 0.05s;
    }
    
    .glassmorphism-card > div:nth-child(3) {
        animation-delay: 0.1s;
    }
    
    .glassmorphism-card > div:nth-child(4) {
        animation-delay: 0.15s;
    }
    
    .glassmorphism-card > div:nth-child(5) {
        animation-delay: 0.2s;
    }
    
    @keyframes messageIn {
        0% {
            opacity: 0;
            transform: translateY(-2px);
        }
        100% {
            opacity: 1;
            transform: translateY(0);
        }
    }
    
    /* Force animation reset when the class is toggled */
    .glassmorphism-card.reanimating > div {
        opacity: 0 !important;
        animation-name: none !important;
    }
</style>