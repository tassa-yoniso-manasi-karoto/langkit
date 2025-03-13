<script lang="ts">
    import { createEventDispatcher, onMount, onDestroy } from 'svelte';
    import { fly } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import { get } from 'svelte/store';
    import { debounce } from 'lodash';
    
    import { settings } from '../lib/stores.ts';
    import { errorStore } from '../lib/errorStore';
    import { 
        features, 
        createDefaultOptions, 
        providerGithubUrls, 
        providersRequiringTokens,
        type RomanizationScheme 
    } from '../lib/featureModel';
    import { 
        GetRomanizationStyles, 
        ValidateLanguageTag, 
        CheckMediaLanguageTags,
        NeedsTokenization
    } from '../../wailsjs/go/gui/App';
    
    import FeatureCard from './FeatureCard.svelte';
    import QuickAccessLangSelector from './QuickAccessLangSelector.svelte';

    // Props
    export let selectedFeatures = {
        subs2cards: false,
        dubtitles: false,
        voiceEnhancing: false,
        subtitleRomanization: false,
        selectiveTransliteration: false,
        subtitleTokenization: false
    };
    export let quickAccessLangTag = '';
    export let showLogViewer: boolean;
    export let mediaSource: MediaSource | null = null;

    // State variables
    let visibleFeatures: string[] = [];
    let currentFeatureOptions = createDefaultOptions();
    
    let isValidLanguage: boolean | null = null;
    let tokenizationAllowed = false;
    let isChecking = false;
    let standardTag = '';
    let validationError = '';
    
    let romanizationSchemes: RomanizationScheme[] = [];
    let isRomanizationAvailable = true;
    let isSelectiveTransliterationAvailable = false;
    
    let dockerUnreachable = false;
    let dockerEngine = '';
    let needsDocker = false;
    let needsScraper = false;
    
    let showAudioTrackIndex = false;
    let audioTrackIndex = 0;
    let hasLanguageTags = true;

    // Provider group tracking
    let providerGroups: Record<string, string[]> = {
        subtitle: ['subtitleRomanization', 'selectiveTransliteration', 'subtitleTokenization']
    };
    
    // Output merge group tracking
    let outputMergeGroups: Record<string, string[]> = {
        finalOutput: ['dubtitles', 'voiceEnhancing', 'subtitleRomanization', 'selectiveTransliteration', 'subtitleTokenization']
    };
    
    // Active feature for showing merge options in each merge group
    let activeMergeFeature: Record<string, string | null> = {
        finalOutput: null
    };
    
    // Store for merge option default values
    let mergeOptionValues = {
        mergeOutputFiles: true,
        mergingFormat: 'mp4'
    };

    const dispatch = createEventDispatcher();
    
    // Language validation with debounce
    const validateLanguageTag = debounce(async (code: string, maxOne: boolean) => {
        if (!code) {
            isValidLanguage = null;
            isChecking = false;
            standardTag = '';
            validationError = '';
            await updateRomanizationStyles('');
            return;
        }
        
        isChecking = true;
        try {
            const response = await ValidateLanguageTag(code, maxOne);
            
            isValidLanguage = response.isValid;
            standardTag = response.standardTag || '';
            validationError = response.error || '';
            
            if (response.isValid) {
                await updateRomanizationStyles(standardTag);
            } else {
                await updateRomanizationStyles('');
            }
        } catch (error) {
            console.error('Error checking language code:', error);
            isValidLanguage = null;
            standardTag = '';
            validationError = 'Validation failed';
            await updateRomanizationStyles('');
        }
        isChecking = false;
    }, 300);

    // Update romanization styles based on language
    async function updateRomanizationStyles(tag: string) {
        if (!tag?.trim()) {
            romanizationSchemes = [];
            isRomanizationAvailable = false;
            isSelectiveTransliterationAvailable = false;
            needsDocker = false;
            needsScraper = false;
            if (selectedFeatures.subtitleRomanization) {
                selectedFeatures.subtitleRomanization = false;
            }
            if (selectedFeatures.selectiveTransliteration) {
                selectedFeatures.selectiveTransliteration = false;
            }
            return;
        }

        try {
            const response = await GetRomanizationStyles(tag);
            
            romanizationSchemes = response.schemes || [];
            isRomanizationAvailable = romanizationSchemes.length > 0;
            
            // Check if selective transliteration is available for this language (only Japanese for now)
            isSelectiveTransliterationAvailable = tag === 'jpn';
            
            needsScraper = response.needsScraper || false;
            dockerUnreachable = response.dockerUnreachable || false;
            needsDocker = response.needsDocker || false;
            dockerEngine = response.dockerEngine || 'Docker Desktop';
            
            // Automatically set the first scheme if only one is available
            if (romanizationSchemes.length === 1) {
                currentFeatureOptions.subtitleRomanization.style = romanizationSchemes[0].name;
            }
            
            // Disable subtitle romanization if not available
            if (!isRomanizationAvailable && selectedFeatures.subtitleRomanization) {
                selectedFeatures.subtitleRomanization = false;
                errorStore.addError({
                    id: 'no-romanization',
                    message: 'No transliteration scheme available for selected language',
                    severity: 'warning'
                });
            }
            
            // Disable selective transliteration if not available
            if (!isSelectiveTransliterationAvailable && selectedFeatures.selectiveTransliteration) {
                selectedFeatures.selectiveTransliteration = false;
                errorStore.addError({
                    id: 'no-selective-transliteration',
                    message: 'Kanji to Kana transliteration is only available for Japanese',
                    severity: 'warning'
                });
            }
        } catch (error) {
            console.error('Error fetching romanization styles:', error);
            romanizationSchemes = [];
            isRomanizationAvailable = false;
            isSelectiveTransliterationAvailable = false;
            if (selectedFeatures.subtitleRomanization) {
                selectedFeatures.subtitleRomanization = false;
            }
            if (selectedFeatures.selectiveTransliteration) {
                selectedFeatures.selectiveTransliteration = false;
            }
        }
    }

    // Synchronize merge option values
    function synchronizeMergeOptions() {
        // This now just extracts merge option values from the active feature
        // in each group, if available
        
        for (const groupName of Object.keys(activeMergeFeature)) {
            const activeFeature = activeMergeFeature[groupName];
            
            if (activeFeature && 
                selectedFeatures[activeFeature] && 
                currentFeatureOptions[activeFeature]) {
                
                // Update global merge values from the active feature
                if (currentFeatureOptions[activeFeature].mergeOutputFiles !== undefined) {
                    mergeOptionValues.mergeOutputFiles = currentFeatureOptions[activeFeature].mergeOutputFiles;
                }
                if (currentFeatureOptions[activeFeature].mergingFormat !== undefined) {
                    mergeOptionValues.mergingFormat = currentFeatureOptions[activeFeature].mergingFormat;
                }
            }
        }
    }
    
    // This function controls which feature shows the merge options
    function updateMergeOptionsVisibility() {
        let hasChanges = false;
        
        // For each merge group, only show merge options on one selected feature
        for (const [groupName, groupFeatures] of Object.entries(outputMergeGroups)) {
            // Always use the first enabled feature in the order they appear in the features array
            // This ensures the merge options always appear on the feature that's earliest in the list
            
            // Find the first enabled feature based on the original feature order in features array
            const orderedEnabledFeatures = features
                .filter(f => 
                    f.outputMergeGroup === groupName && 
                    groupFeatures.includes(f.id) && 
                    selectedFeatures[f.id]
                )
                .map(f => f.id);
            
            if (orderedEnabledFeatures.length > 0) {
                // Always use the first enabled feature in the list
                const firstEnabledFeature = orderedEnabledFeatures[0];
                
                // If the active feature has changed, update it
                if (activeMergeFeature[groupName] !== firstEnabledFeature) {
                    activeMergeFeature[groupName] = firstEnabledFeature;
                    hasChanges = true;
                }
                
                // Process each feature
                for (const featureId of groupFeatures) {
                    if (!currentFeatureOptions[featureId]) {
                        currentFeatureOptions[featureId] = {};
                    }
                    
                    // Create a shallow copy of the options object
                    const optionsCopy = {...currentFeatureOptions[featureId]};
                    
                    if (featureId === firstEnabledFeature) {
                        // This is the active feature - ensure it has merge options
                        optionsCopy.mergeOutputFiles = mergeOptionValues.mergeOutputFiles;
                        optionsCopy.mergingFormat = mergeOptionValues.mergingFormat;
                    } else {
                        // Remove merge options from inactive features
                        if (optionsCopy.mergeOutputFiles !== undefined) {
                            delete optionsCopy.mergeOutputFiles;
                            hasChanges = true;
                        }
                        if (optionsCopy.mergingFormat !== undefined) {
                            delete optionsCopy.mergingFormat;
                            hasChanges = true;
                        }
                    }
                    
                    // Only update if changes were made
                    if (JSON.stringify(optionsCopy) !== JSON.stringify(currentFeatureOptions[featureId])) {
                        hasChanges = true;
                        currentFeatureOptions[featureId] = optionsCopy;
                    }
                }
            } else {
                // No enabled features in this group
                if (activeMergeFeature[groupName] !== null) {
                    activeMergeFeature[groupName] = null;
                    hasChanges = true;
                }
            }
        }
        
        // Only dispatch if changes were made
        if (hasChanges) {
            dispatch('optionsChange', currentFeatureOptions);
        }
    }
    
    // API token checking
    function checkProviderApiToken(provider: string): { isValid: boolean; tokenType: string | null } {
        const tokenType = providersRequiringTokens[provider];
        if (!tokenType) return { isValid: true, tokenType: null };
        
        const currentSettings = get(settings);
        const hasToken = currentSettings?.apiKeys?.[tokenType]?.trim().length > 0;
        
        return { 
            isValid: hasToken,
            tokenType: tokenType
        };
    }
    
    function updateProviderWarnings() {
        // Check dubtitles STT provider
        if (selectedFeatures.dubtitles && currentFeatureOptions.dubtitles) {
            const sttProvider = currentFeatureOptions.dubtitles.stt;
            const { isValid, tokenType } = checkProviderApiToken(sttProvider);
            if (!isValid) {
                errorStore.addError({
                    id: 'provider-dubtitles',
                    message: `${tokenType} API token is required for ${sttProvider}`,
                    severity: 'critical'
                });
            } else {
                errorStore.removeError('provider-dubtitles');
            }
        } else {
            errorStore.removeError('provider-dubtitles');
        }

        // Check voice enhancing provider
        if (selectedFeatures.voiceEnhancing && currentFeatureOptions.voiceEnhancing) {
            const sepLib = currentFeatureOptions.voiceEnhancing.sepLib;
            const { isValid, tokenType } = checkProviderApiToken(sepLib);
            if (!isValid) {
                errorStore.addError({
                    id: 'provider-voiceEnhancing',
                    message: `${tokenType} API token is required for ${sepLib}`,
                    severity: 'critical'
                });
            } else {
                errorStore.removeError('provider-voiceEnhancing');
            }
        } else {
            errorStore.removeError('provider-voiceEnhancing');
        }
    }
    
    // Media file checking
    async function checkMediaFiles() {
        if (mediaSource) {
            try {
                const info = await CheckMediaLanguageTags(mediaSource.path);
                hasLanguageTags = info.hasLanguageTags;
                showAudioTrackIndex = !hasLanguageTags;
            } catch (error) {
                console.error('Error checking media files:', error);
            }
        }
    }
    
    // Event handlers
    function handleFeatureEnabledChange(event: CustomEvent) {
        const { id, enabled } = event.detail;
        selectedFeatures[id] = enabled;
        updateProviderWarnings();
        
        // Handle provider group features
        const featureDef = features.find(f => f.id === id);
        if (featureDef?.providerGroup) {
            // If this feature is part of a provider group, check if any feature in the group is enabled
            const groupFeatures = providerGroups[featureDef.providerGroup] || [];
            const isAnyGroupFeatureEnabled = groupFeatures.some(fId => selectedFeatures[fId]);
            
            // Add subtitleProviderSettings to visible features if any subtitle feature is enabled
            if (featureDef.providerGroup === 'subtitle') {
                // Find the subtitleProviderSettings feature for conditions
                if (isAnyGroupFeatureEnabled && !visibleFeatures.includes('subtitleProviderSettings')) {
                    visibleFeatures = [...visibleFeatures, 'subtitleProviderSettings'];
                }
            }
        }
        
        // Handle output merge group features
        if (featureDef?.outputMergeGroup) {
            // When a feature in a merge group is enabled or disabled, 
            // we need to update the merge options visibility
            
            // If this feature was just enabled, it might need to get the merge options
            if (enabled) {
                // Get the group this feature belongs to
                const groupName = featureDef.outputMergeGroup;
                const groupFeatures = outputMergeGroups[groupName] || [];
                
                // Find all enabled features in this group ordered by their appearance in the features array
                const orderedEnabledFeatures = features
                    .filter(f => 
                        f.outputMergeGroup === groupName && 
                        groupFeatures.includes(f.id) && 
                        (f.id === id || selectedFeatures[f.id]) // Include this feature as if it's already enabled
                    )
                    .map(f => f.id);
                
                // If this feature is the first enabled one in the list, it should get the merge options
                if (orderedEnabledFeatures[0] === id) {
                    activeMergeFeature[groupName] = id;
                }
            }
            
            // Update visibility in all cases
            updateMergeOptionsVisibility();
        }
        
        // If a feature was enabled, scroll it into view but only if necessary
        if (enabled) {
            // Use requestAnimationFrame instead of setTimeout for better performance
            requestAnimationFrame(() => {
                const featureCard = document.querySelector(`[data-feature-id="${id}"]`);
                if (!featureCard) return;
                
                // Get the scroll container (mask-fade element)
                const scrollContainer = featureCard.closest('.mask-fade');
                if (!scrollContainer) return;
                
                // Get the position of the feature card
                const containerRect = scrollContainer.getBoundingClientRect();
                const featureRect = featureCard.getBoundingClientRect();
                
                // Check if feature is already fully visible - if so, don't scroll
                const isFullyVisible = (
                    featureRect.top >= containerRect.top + containerRect.height * 0.1 && 
                    featureRect.bottom <= containerRect.bottom - containerRect.height * 0.1
                );
                
                // Only scroll if the feature is not already fully visible
                if (!isFullyVisible) {
                    // Let OptionHeight calculate before scrolling
                    setTimeout(() => {
                        // Check again if the component is still mounted
                        if (!document.body.contains(featureCard)) return;
                        
                        // Use the simpler scrollIntoView API with smooth behavior
                        // This uses the browser's native smooth scrolling which is more optimized
                        featureCard.scrollIntoView({
                            behavior: 'smooth',
                            block: 'center'
                        });
                    }, 200);
                }
            });
        }
    }
    
    function handleOptionChange(event: CustomEvent) {
        const { featureId, optionId, value } = event.detail;
        
        // Check if this is a provider-related option
        const isProviderOption = optionId === 'style' || optionId === 'provider' || 
                optionId === 'dockerRecreate' || optionId === 'browserAccessURL';
        
        // Check if this is a merge-related option
        const isMergeOption = optionId === 'mergeOutputFiles' || optionId === 'mergingFormat';
        
        // Check if this belongs to a provider group
        const feature = features.find(f => f.id === featureId);
        const isInProviderGroup = feature && feature.providerGroup;
        const isInMergeGroup = feature && feature.outputMergeGroup;
        
        // Handle provider group options
        if (isInProviderGroup && isProviderOption) {
            // Get all features in the same provider group
            const groupFeatures = providerGroups[feature.providerGroup] || [];
            
            // Propagate provider-related changes to all features in the group
            groupFeatures.forEach(groupFeatureId => {
                // Create the options object if it doesn't exist
                if (!currentFeatureOptions[groupFeatureId]) {
                    currentFeatureOptions[groupFeatureId] = {};
                }
                
                // Copy the provider-related option
                currentFeatureOptions[groupFeatureId][optionId] = value;
            });
            
            // Special handling for romanization style changes
            if (optionId === 'style') {
                const selectedScheme = romanizationSchemes.find(s => s.name === value);
                if (selectedScheme) {
                    const providerValue = selectedScheme.provider;
                    // Update provider for all features in the group
                    groupFeatures.forEach(groupFeatureId => {
                        currentFeatureOptions[groupFeatureId]['provider'] = providerValue;
                    });
                }
            }
        } 
        // Handle merge group options
        else if (isMergeOption) {
            // Update the global merge option value
            mergeOptionValues[optionId] = value;
            
            // Update the current feature option
            currentFeatureOptions[featureId][optionId] = value;
            
            // Since this is coming from a user interaction on the active feature,
            // we don't need to run the full updateMergeOptionsVisibility
            // Just dispatch the change
            dispatch('optionsChange', currentFeatureOptions);
        } 
        else {
            // For non-special options or features not in groups, just update directly
            currentFeatureOptions[featureId][optionId] = value;
        }

        dispatch('optionsChange', currentFeatureOptions);
    }
    
    function handleLanguageTagChange(event: CustomEvent) {
        quickAccessLangTag = event.detail.languageTag;
        validateLanguageTag(quickAccessLangTag, true);
    }
    
    function handleAudioTrackChange(event: CustomEvent) {
        showAudioTrackIndex = event.detail.showAudioTrackIndex;
        audioTrackIndex = event.detail.audioTrackIndex;
    }
    
    function shouldShowFeature(featureDef: any): boolean {
        if (!featureDef.showCondition) return true;
        
        try {
            // Replace context variables with their values
            const prepared = featureDef.showCondition
                .replace(/context\.([a-zA-Z0-9_]+)/g, (_, prop) => {
                    return JSON.stringify(context[prop]);
                });
            
            // Use Function constructor to evaluate the expression
            return new Function('return ' + prepared)();
        } catch (error) {
            console.error('Error evaluating feature condition:', featureDef.showCondition, error);
            return false;
        }
    }
    
    async function checkTokenization(code: string) {
        try {
            tokenizationAllowed = await NeedsTokenization(code);
            console.log("NeedsTokenization returned:", tokenizationAllowed);
        } catch (err) {
            console.error("NeedsTokenization failed:", err);
            tokenizationAllowed = false;
        }
    }

    // Prepare context for conditions
    let context = {
        standardTag: '',
        needsDocker: false,
        needsScraper: false,
        romanizationSchemes: [],
        selectedFeatures: {}
    };
    
    // Update context when dependencies change
    $: context = {
        standardTag,
        needsDocker,
        needsScraper,
        romanizationSchemes,
        selectedFeatures
    };
    
    // Reactive statements
    $: anyFeatureSelected = Object.values(selectedFeatures).some(v => v);

    $: if (quickAccessLangTag !== undefined) {
        needsDocker = false;
        needsScraper = false;
        validateLanguageTag(quickAccessLangTag, true);
        checkTokenization(quickAccessLangTag)
        
        if (!quickAccessLangTag) {
            isRomanizationAvailable = false;
            selectedFeatures.subtitleRomanization = false;
        }
    }
    
    // Error management
    // Memoize feature selection state and language validation to reduce reactivity overhead
    let prevFeaturesSelected = false;
    let prevLanguageValidState = null;
    let prevLanguageTag = '';
    
    // Optimize error handling with debounced updates
    const updateErrorState = debounce(() => {
        // Check if features selection state changed
        const featuresSelected = Object.values(selectedFeatures).some(v => v);
        if (prevFeaturesSelected !== featuresSelected) {
            if (!featuresSelected) {
                errorStore.addError({
                    id: 'no-features',
                    message: 'Select at least one processing feature',
                    severity: 'critical'
                });
            } else {
                errorStore.removeError('no-features');
            }
            prevFeaturesSelected = featuresSelected;
        }

        // Check if language validation state changed
        if (prevLanguageValidState !== isValidLanguage || prevLanguageTag !== quickAccessLangTag) {
            if (!isValidLanguage && quickAccessLangTag) {
                errorStore.addError({
                    id: 'invalid-language',
                    message: validationError || 'Invalid language code',
                    severity: 'critical'
                });
            } else {
                errorStore.removeError('invalid-language');
            }
            prevLanguageValidState = isValidLanguage;
            prevLanguageTag = quickAccessLangTag;
        }
    }, 100);
    
    // Trigger error state update when relevant state changes
    $: {
        if (selectedFeatures || isValidLanguage || quickAccessLangTag) {
            requestAnimationFrame(updateErrorState);
        }
    }

    // Settings subscription
    settings.subscribe(value => {
        if (value?.targetLanguage && value.targetLanguage !== quickAccessLangTag) {
            quickAccessLangTag = value.targetLanguage;
            validateLanguageTag(value.targetLanguage, true);
        }
    });
    
    settings.subscribe(value => {
        if (value) {
            updateProviderWarnings();
        }
    });
    
    // Remove this section as we've moved selective transliteration to its own feature
    
    // Media source change
    $: if (mediaSource) {
        checkMediaFiles();
    } else {
        showAudioTrackIndex = false;
        hasLanguageTags = true;
        audioTrackIndex = 0;
    }
    
    // Docker errors
    $: {
        if ((selectedFeatures.subtitleRomanization || selectedFeatures.selectiveTransliteration) && 
            needsDocker && dockerUnreachable) {
            errorStore.addError({
                id: 'docker-required',
                message: `${dockerEngine} is required but not reachable`,
                severity: 'critical',
                docsUrl: 'https://docs.docker.com/get-docker/'
            });
        } else {
            errorStore.removeError('docker-required');
        }
    }
    
    // Browser URL errors
    $: {
        if (selectedFeatures.subtitleRomanization && needsScraper && 
            (!currentFeatureOptions.subtitleRomanization.browserAccessURL || 
             !currentFeatureOptions.subtitleRomanization.browserAccessURL.startsWith('ws://'))) {
            errorStore.addError({
                id: 'invalid-browser-url',
                message: 'Valid browser access URL is required for web scraping',
                severity: 'critical'
            });
        } else {
            errorStore.removeError('invalid-browser-url');
        }
    }
    
    // Options change dispatch
    $: {
        dispatch('optionsChange', currentFeatureOptions);
    }
    
    // Feature selection change
    // Use a debounced version of update functions to reduce lag
    const debouncedUpdateMergeOptionsVisibility = debounce(updateMergeOptionsVisibility, 100);
    
    $: if (selectedFeatures) {
        updateProviderWarnings();
        debouncedUpdateMergeOptionsVisibility();
    }

    // Special initialization to clean up inconsistencies in feature options
    function cleanupFeatureOptions() {
        // Initialize all features and remove any stray merge options
        Object.entries(outputMergeGroups).forEach(([groupName, groupFeatures]) => {
            groupFeatures.forEach(featureId => {
                if (!currentFeatureOptions[featureId]) {
                    currentFeatureOptions[featureId] = {};
                }
                
                // Remove merge options from all features initially
                delete currentFeatureOptions[featureId].mergeOutputFiles;
                delete currentFeatureOptions[featureId].mergingFormat;
            });
        });
        
        // Then run the update to add options only to the active feature
        updateMergeOptionsVisibility();
    }
    
    // Component lifecycle
    onMount(async () => {
        const currentSettings = get(settings);
        if (currentSettings?.targetLanguage) {
            quickAccessLangTag = currentSettings.targetLanguage;
            await validateLanguageTag(currentSettings.targetLanguage, true);
            await checkTokenization(quickAccessLangTag);
        }
        updateProviderWarnings();
        cleanupFeatureOptions(); // Clean up feature options on mount
        
        // Optimize staggered animation for features
        // Check if device has reduced motion preference
        const prefersReducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
        
        if (prefersReducedMotion) {
            // Respect user's motion preference - show all features at once
            visibleFeatures = Object.keys(selectedFeatures);
        } else {
            // More efficient batch animation with fewer DOM updates
            const allFeatures = Object.keys(selectedFeatures);
            const batches = 3; // Reduce number of DOM updates by showing features in 3 batches
            const initialWait = showLogViewer ? 100 : 0; // Much shorter initial wait
            
            // Add features in batches for better performance
            for (let b = 0; b < batches; b++) {
                const batchFeatures = allFeatures.filter((_, i) => i % batches === b);
                setTimeout(() => {
                    visibleFeatures = [...visibleFeatures, ...batchFeatures];
                }, initialWait + (b * 120)); // Use fixed delay increments instead of exponential
            }
        }
    });
    
    onDestroy(() => {
        errorStore.removeError('docker-required');
        errorStore.removeError('invalid-browser-url');
        errorStore.removeError('no-features');
        errorStore.removeError('invalid-language');
        errorStore.removeError('provider-dubtitles');
        errorStore.removeError('provider-voiceEnhancing');
        errorStore.removeError('no-romanization');
        errorStore.removeError('no-selective-transliteration');
    });
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between pl-0 pr-0">
        <h2 class="text-xl font-medium text-secondary flex items-center pl-4 gap-2">
            <span class="material-icons text-secondary">tune</span>
            Select Features
        </h2>
        
        <!-- Language selector component -->
        <div class="flex items-center ml-auto item-right gap-2 pr-3">
            <QuickAccessLangSelector 
                languageTag={quickAccessLangTag}
                {isValidLanguage}
                {isChecking}
                {validationError}
                {showAudioTrackIndex}
                {audioTrackIndex}
                on:languageTagChange={handleLanguageTagChange}
                on:audioTrackChange={handleAudioTrackChange}
            />
        </div>
    </div>
    
    <div class="space-y-4">
        {#each features.filter(f => visibleFeatures.includes(f.id) && (!f.showCondition || shouldShowFeature(f))) as feature, i (feature.id)}
            <div 
                in:fly={{ 
                    x: 100, // Reduced distance for better performance
                    duration: Math.min(300, 300 - (i * 15)), // Cap min duration at 150ms
                    easing: cubicOut,
                    opacity: 0
                }}
                style="will-change: transform, opacity; contain: content;"
            >
                <div data-feature-id={feature.id}>
                    <FeatureCard
                        {feature}
                        enabled={selectedFeatures[feature.id]}
                        options={currentFeatureOptions[feature.id]}
                        {anyFeatureSelected}
                        {romanizationSchemes}
                        {isRomanizationAvailable}
                        {tokenizationAllowed}
                        {needsDocker}
                        {dockerUnreachable}
                        {dockerEngine}
                        {needsScraper}
                        {standardTag}
                        {providerGithubUrls}
                        {selectedFeatures}
                        {providerGroups}
                        {outputMergeGroups}
                        on:enabledChange={handleFeatureEnabledChange}
                        on:optionChange={handleOptionChange}
                    />
                </div>
            </div>
        {/each}
        <br>
    </div>
</div>

<style>
    /* Add smooth animation for cards when scrolling */
    div {
        will-change: transform;
        transform: translateZ(0);
    }
    
    /* Glow effects are now in FeatureCard component */
</style>