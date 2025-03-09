<script lang="ts">
    import { get } from 'svelte/store';
    import { createEventDispatcher, onMount } from 'svelte';
    
    import { formatDisplayText, type FeatureDefinition } from '../lib/featureModel';
    import { errorStore } from '../lib/errorStore';
    import { showSettings } from '../lib/stores';
    
    import Dropdown from './Dropdown.svelte';
    import Hovertip from './Hovertip.svelte';
    import NumericInput from './NumericInput.svelte';
    import ExternalLink from './ExternalLink.svelte';
    
    export let feature: FeatureDefinition;
    export let enabled = false;
    export let options: any = {};
    export let anyFeatureSelected = false;
    
    export let romanizationSchemes = [];
    export let isRomanizationAvailable = true;
    export let needsDocker = false;
    export let dockerUnreachable = false;
    export let dockerEngine = 'Docker';
    export let needsScraper = false;
    export let standardTag = '';
    
    export let providerGithubUrls = {};

    const dispatch = createEventDispatcher();
    
    // Animation references and state
    let optionsContainer: HTMLElement;
    let optionsWrapper: HTMLElement;
    let optionsHeight = 0;
    let animating = false;
    let showNonJpnMessage = false;
    
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
        // Check if the feature is unavailable based on language requirements
        const isFeatureUnavailable =
            (feature.id === 'subtitleRomanization' && !isRomanizationAvailable) ||
            (feature.id === 'selectiveTransliteration' && (standardTag !== 'jpn'));
        
        
        if (isFeatureUnavailable) {
            // If the user tries to enable selectiveTransliteration but standardTag != 'jpn',
            // show a 10-second message and do the shake animation, but do NOT enable.
            if (feature.id === 'selectiveTransliteration' && standardTag !== 'jpn') {
                showNonJpnMessage = true;
                setTimeout(() => {
                    showNonJpnMessage = false;
                }, 5000);
            }
            
            // Trigger shake animation
            const element = event.currentTarget as HTMLElement;
            element.classList.remove('shake-animation');
            void element.offsetWidth; // Force reflow to restart animation
            element.classList.add('shake-animation');
            return; // Don't allow toggling disabled features
        }
        
        // Toggle the feature directly
        enabled = !enabled;
        dispatch('enabledChange', { id: feature.id, enabled });
    }
    
    // Check if option should be shown based on conditions
    function shouldShowOption(optionId: string, optionDef: any): boolean {
        if (!optionDef.showCondition) return true;
        
        // Context object for evaluating conditions
        const context = {
            standardTag,
            needsDocker,
            needsScraper, 
            romanizationSchemes
        };
        
        // Feature options reference for conditions 
        const featureData = {
            [feature.id]: options
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
                });
            
            // Use Function constructor to evaluate the expression
            return new Function('return ' + prepared)();
        } catch (error) {
            console.error('Error evaluating condition:', optionDef.showCondition, error);
            return false;
        }
    }
    
    // Handle dropdown changes
    function handleDropdownChange(optionId: string, value: string) {
        options[optionId] = value;
        dispatch('optionChange', { featureId: feature.id, optionId, value });
    }
    
    // Get visible options for this feature
    function getVisibleOptions(): string[] {
        if (feature.optionOrder) {
            return feature.optionOrder.filter(optionId => 
                feature.options[optionId] && shouldShowOption(optionId, feature.options[optionId])
            );
        }
        
        return Object.keys(feature.options).filter(optionId => 
            shouldShowOption(optionId, feature.options[optionId])
        );
    }
    
    // When enabled status changes, animate the height
    $: {
        if (optionsContainer) {
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
        }
    }
    
    $: displayLabel = feature.id === 'selectiveTransliteration' && standardTag === 'jpn' 
    ? 'Selective Kanji Transliteration' 
    : feature.label;
</script>

<div class="bg-white/5 rounded-lg
           transition-all duration-300 ease-out transform
           hover:translate-y-[-2px]
         {((!isRomanizationAvailable && feature.id === 'subtitleRomanization') || 
           (standardTag !== 'jpn' && feature.id === 'selectiveTransliteration'))
            ? 'opacity-50 cursor-not-allowed' 
            : 'hover:translate-y-[-2px]'}"
     class:shadow-glow-strong={enabled && !anyFeatureSelected}
     class:shadow-glow={enabled}
     class:hover:shadow-glow-hover={!enabled && ((feature.id !== 'subtitleRomanization' || isRomanizationAvailable) && 
                                                (feature.id !== 'selectiveTransliteration' || standardTag === 'jpn'))}
     class:opacity-30={anyFeatureSelected && !enabled}
     on:click={handleFeatureClick}
>
    <div class="p-4 border-b border-white/10">
        <div class="flex items-center gap-3 cursor-pointer group
                  {((!isRomanizationAvailable && feature.id === 'subtitleRomanization') || 
                    (standardTag !== 'jpn' && feature.id === 'selectiveTransliteration'))
                    ? 'cursor-not-allowed' : ''}">
            <input
                type="checkbox"
                class="w-4 h-4 accent-primary/90 hover:accent-primary"
                bind:checked={enabled}
                disabled={((!isRomanizationAvailable && feature.id === 'subtitleRomanization') || 
                           (standardTag !== 'jpn' && feature.id === 'selectiveTransliteration'))}
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
        
        <!-- Specific messages for subtitleRomanization feature -->
        {#if feature.id === 'subtitleRomanization'}
            {#if enabled && needsDocker && !dockerUnreachable}
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
            {#if enabled && needsDocker && !dockerUnreachable}
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
        {/if}
    </div>
    
    <!-- Options drawer with slide animation -->
    <div bind:this={optionsContainer} class="overflow-hidden" style="height: 0px; transition: height 350ms cubic-bezier(0.25, 1, 0.5, 1)">
        <div bind:this={optionsWrapper} class="p-4">
            <div class="grid grid-cols-[1fr,1.5fr] gap-x-6 gap-y-3">
                {#each getVisibleOptions() as optionId}
                    {@const optionDef = feature.options[optionId]}
                    {@const value = options[optionId]}
                    
                    <div class="flex items-center">
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
                    <div>
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
                                    class="w-full bg-sky-dark/50 border border-primary/30 rounded px-3 py-2 text-sm font-medium 
                                           focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary
                                           transition-colors duration-200 placeholder:text-gray-500"
                                    rows="3"
                                    maxlength="850"
                                    placeholder={optionDef.placeholder}
                                    on:input={() => dispatch('optionChange', { featureId: feature.id, optionId, value: options[optionId] })}
                                />
                            {:else}
                                <input 
                                    type="text"
                                    bind:value={options[optionId]}
                                    class="w-full bg-sky-dark/50 border border-primary/30 rounded px-3 py-1
                                           focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary
                                           transition-colors duration-200 text-sm font-medium placeholder:text-gray-500"
                                    placeholder={optionDef.placeholder}
                                    on:input={() => dispatch('optionChange', { featureId: feature.id, optionId, value: options[optionId] })}
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
                {/each}
            </div>
        </div>
    </div>
</div>

<style>
    @import './featureStyles.css';
</style>