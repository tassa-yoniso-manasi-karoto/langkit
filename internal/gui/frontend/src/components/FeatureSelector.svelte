<script lang="ts">
    import { createEventDispatcher, onMount, onDestroy, afterUpdate } from 'svelte';
    import { fly } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import { get } from 'svelte/store';
    import { debounce } from 'lodash';
    
    import { settings } from '../lib/stores.ts';
    import { errorStore } from '../lib/errorStore';
    import { logStore } from '../lib/logStore';
    import { 
        features, 
        createDefaultOptions, 
        providerGithubUrls, 
        providersRequiringTokens,
        type RomanizationScheme 
    } from '../lib/featureModel';
    import { 
        featureGroupStore, 
        type FeatureGroup,
        groupHasEnabledFeature
    } from '../lib/featureGroupStore';
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

    // Legacy group tracking for backward compatibility during transition
    let providerGroups: Record<string, string[]> = {
        subtitle: ['subtitleRomanization', 'selectiveTransliteration', 'subtitleTokenization']
    };
    
    let outputMergeGroups: Record<string, string[]> = {
        finalOutput: ['dubtitles', 'voiceEnhancing', 'subtitleRomanization', 'selectiveTransliteration', 'subtitleTokenization']
    };
    
    // Active feature for showing merge options in each merge group
    let activeMergeFeature: Record<string, string | null> = {
        finalOutput: null
    };
    
    // Store for merge option default values
    let mergeOptionValues = {
        mergeOutputFiles: false,
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
            
            // Reset all feature selections
            resetAllFeatures();
            
            await updateRomanizationStyles('');
            return;
        }
        
        isChecking = true;
        try {
            const response = await ValidateLanguageTag(code, maxOne);
            
            const previousTag = standardTag;
            isValidLanguage = response.isValid;
            standardTag = response.standardTag || '';
            validationError = response.error || '';
            
            // If language has changed, reset all feature selections
            if (previousTag !== standardTag) {
                resetAllFeatures();
            }
            
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
            
            // Reset all feature selections on error
            resetAllFeatures();
            
            await updateRomanizationStyles('');
        }
        isChecking = false;
    }, 300);
    
    // Function to reset all feature selections
    function resetAllFeatures() {
        console.log("Resetting all feature selections due to language change");
        
        // Disable all features
        Object.keys(selectedFeatures).forEach(featureId => {
            if (selectedFeatures[featureId]) {
                // Set to false
                selectedFeatures[featureId] = false;
                
                // Handle feature groups
                const featureDef = features.find(f => f.id === featureId);
                if (featureDef?.featureGroups?.length) {
                    featureDef.featureGroups.forEach(groupId => {
                        featureGroupStore.updateFeatureEnabled(groupId, featureId, false);
                    });
                }
            }
        });
        
        // Notify parent component of changes
        dispatch('optionsChange', currentFeatureOptions);
    }

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
            
            // Automatically set the first scheme if only one is available or if none is set yet
            if (romanizationSchemes.length > 0) {
                const currentStyle = featureGroupStore.getGroupOption('subtitle', 'style');
                if (!currentStyle || romanizationSchemes.length === 1) {
                    const newStyle = romanizationSchemes[0].name;
                    
                    // Update the style and provider in the group store
                    featureGroupStore.setGroupOption('subtitle', 'style', newStyle);
                    featureGroupStore.setGroupOption('subtitle', 'provider', romanizationSchemes[0].provider);
                    
                    // Also update the individual feature options for backward compatibility
                    currentFeatureOptions.subtitleRomanization.style = newStyle;
                    currentFeatureOptions.subtitleRomanization.provider = romanizationSchemes[0].provider;
                    
                    // Set the same values for selectiveTransliteration when available
                    if (isSelectiveTransliterationAvailable) {
                        currentFeatureOptions.selectiveTransliteration.style = newStyle;
                        currentFeatureOptions.selectiveTransliteration.provider = romanizationSchemes[0].provider;
                    }
                    
                    // Set the same values for subtitleTokenization
                    currentFeatureOptions.subtitleTokenization.style = newStyle;
                    currentFeatureOptions.subtitleTokenization.provider = romanizationSchemes[0].provider;
                    
                    console.log(`Set default romanization style to ${newStyle} with provider ${romanizationSchemes[0].provider}`);
                }
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
        console.log(`Feature toggle: ${id} -> ${enabled}`);
        
        // Update the selected features state
        selectedFeatures[id] = enabled;
        updateProviderWarnings();
        
        // Find the feature definition
        const featureDef = features.find(f => f.id === id);
        if (!featureDef) {
            console.error(`Feature not found: ${id}`);
            return;
        }
        
        // Process feature groups if this feature belongs to any
        if (featureDef.featureGroups?.length) {
            handleFeatureGroupUpdates(featureDef, id, enabled);
            
            // Update display order after feature enable/disable to ensure proper topmost display
            setTimeout(registerFeatureDisplayOrder, 50);
        }

        // Legacy output merge group handling
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

    /**
     * Handles all feature group related updates when a feature's enabled state changes
     */
    function handleFeatureGroupUpdates(featureDef: FeatureDefinition, featureId: string, enabled: boolean) {
        console.log(`Feature ${featureId} belongs to groups: ${featureDef.featureGroups.join(', ')}`);
        
        featureDef.featureGroups.forEach(groupId => {
            console.log(`Processing group ${groupId} for feature ${featureId}`);
            
            // Update enabled state in the group store
            featureGroupStore.updateFeatureEnabled(groupId, featureId, enabled);
            
            // Get all feature IDs in this group for reference
            const groupFeatureIds = getFeatureIdsInGroup(groupId);
            
            // Get all enabled features in this group
            const enabledFeaturesInGroup = getEnabledFeaturesInGroup(groupId);
            console.log(`Group ${groupId} has ${enabledFeaturesInGroup.length} enabled features`);
            
            // Handle active display feature updates based on the state change
            if (enabled) {
                handleFeatureEnabled(groupId, featureId, groupFeatureIds, enabledFeaturesInGroup);
            } else {
                handleFeatureDisabled(groupId, featureId, groupFeatureIds, enabledFeaturesInGroup);
            }
            
            // Ensure options are consistent across all features in the group
            syncFeatureOptions(groupId);
        });
    }

    /**
     * Handle logic when a feature is enabled
     */
    function handleFeatureEnabled(
        groupId: string, 
        featureId: string, 
        groupFeatureIds: string[], 
        enabledFeaturesInGroup: string[]
    ) {
        console.log(`Feature ${featureId} enabled - checking if it should be active display for group ${groupId}`);
        
        // Get the features in the order they should be prioritized
        const enabledOrderedFeatures = groupFeatureIds.filter(fId => enabledFeaturesInGroup.includes(fId));
        
        // If this is the highest priority enabled feature, make it the active display feature
        if (enabledOrderedFeatures.length > 0 && enabledOrderedFeatures[0] === featureId) {
            console.log(`Making ${featureId} the active display feature for group ${groupId}`);
            featureGroupStore.updateActiveDisplayFeature(
                groupId,
                groupFeatureIds,
                enabledFeaturesInGroup
            );
        }
    }

    /**
     * Handle logic when a feature is disabled
     */
    function handleFeatureDisabled(
        groupId: string, 
        featureId: string, 
        groupFeatureIds: string[], 
        enabledFeaturesInGroup: string[]
    ) {
        // Only update the active display feature if the disabled feature was the active one
        if (featureGroupStore.isActiveDisplayFeature(groupId, featureId)) {
            console.log(`Feature ${featureId} was active display for group ${groupId} but is now disabled`);
            console.log(`After disabling, group ${groupId} has ${enabledFeaturesInGroup.length} enabled features`);
            
            // Update the active display feature to the next best available
            featureGroupStore.updateActiveDisplayFeature(
                groupId, 
                groupFeatureIds,
                enabledFeaturesInGroup
            );
        }
    }

    /**
     * Get all feature IDs that belong to a specific group
     */
    function getFeatureIdsInGroup(groupId: string): string[] {
        return features
            .filter(f => f.featureGroups?.includes(groupId))
            .map(f => f.id);
    }

    /**
     * Get all enabled features in a specific group
     */
    function getEnabledFeaturesInGroup(groupId: string): string[] {
        // First get all feature IDs in this group in their defined order
        const groupFeatureIds = getFeatureIdsInGroup(groupId);
        
        // Then filter for only the enabled ones, maintaining their original order
        return groupFeatureIds.filter(fId => selectedFeatures[fId]);
    }
    /**
     * Sync options from the group store to all features in the group
     */
    function syncFeatureOptions(groupId: string) {
        console.log(`Syncing options for group ${groupId} to features`);
        currentFeatureOptions = featureGroupStore.syncOptionsToFeatures(
            groupId, currentFeatureOptions
        );
    }

    /**
     * Update subtitle provider settings visibility
     */
    function updateSubtitleProviderVisibility(featureDef: FeatureDefinition) {
    }

    function handleOptionChange(event: CustomEvent) {
        const { featureId, optionId, value, isGroupOption, groupId, isUserInput } = event.detail;
        
        // Handle group option changes (new implementation)
        if (isGroupOption && groupId) {
            console.log(`FeatureSelector received group option change - ${groupId}.${optionId}: '${value}'`, 
                isUserInput ? '(user input)' : '');
            
            // Always trust user input as authoritative
            if (isUserInput) {
                console.log(`Setting authoritative user value for ${groupId}.${optionId}: '${value}'`);
            }
            
            // Special handling for romanization style changes
            if (groupId === 'subtitle' && optionId === 'style' && romanizationSchemes.length > 0) {
                // Update the provider based on the selected style
                const selectedScheme = romanizationSchemes.find(s => s.name === value);
                if (selectedScheme) {
                    console.log(`Style changed to ${value}, updating provider to ${selectedScheme.provider}`);
                    
                    // First set the style
                    featureGroupStore.setGroupOption(groupId, optionId, value);
                    
                    // Then set the provider
                    featureGroupStore.setGroupOption(groupId, 'provider', selectedScheme.provider);
                    
                    // Sync all values from the group store
                    currentFeatureOptions = featureGroupStore.syncOptionsToFeatures(
                        groupId, currentFeatureOptions
                    );
                    
                    // Dispatch changes
                    dispatch('optionsChange', currentFeatureOptions);
                    return;
                }
            }
            
            // Update the group store - this is the central source of truth for group options
            featureGroupStore.setGroupOption(groupId, optionId, value);
            
            // Sync values from the group store to all features in the group
            currentFeatureOptions = featureGroupStore.syncOptionsToFeatures(
                groupId, currentFeatureOptions
            );
            
            // Dispatch changes
            dispatch('optionsChange', currentFeatureOptions);
            return;
        }
        
        // Check if this is a merge-related option (migrating to group handling)
        const isMergeOption = optionId === 'mergeOutputFiles' || optionId === 'mergingFormat';
        
        if (isMergeOption) {
            // Update the global merge option value for backward compatibility
            mergeOptionValues[optionId] = value;
            
            // Handle it as a group option by setting it in the feature group store
            featureGroupStore.setGroupOption('finalOutput', optionId, value);
            
            // Sync values from the group store to all features in the group
            currentFeatureOptions = featureGroupStore.syncOptionsToFeatures(
                'finalOutput', currentFeatureOptions
            );
            
            // Dispatch changes
            dispatch('optionsChange', currentFeatureOptions);
        } 
        else {
            // For non-special options or features not in groups, just update directly
            currentFeatureOptions[featureId][optionId] = value;
            dispatch('optionsChange', currentFeatureOptions);
        }
    }
    
    function handleLanguageTagChange(event: CustomEvent) {
        const previousTag = quickAccessLangTag;
        const newTag = event.detail.languageTag;
        
        quickAccessLangTag = newTag;
        
        // If tag has changed significantly (not just case or formatting), reset features
        if (previousTag.toLowerCase() !== newTag.toLowerCase()) {
            console.log(`Language changed from ${previousTag} to ${newTag}`);
        }
        
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
    
    // New loading state to prevent flickering during initial data loading
    let isInitialDataLoaded = false;
    
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
            const previousTag = quickAccessLangTag;
            quickAccessLangTag = value.targetLanguage;
            
            // If changing to a completely different language, reset features
            if (previousTag && previousTag.toLowerCase() !== value.targetLanguage.toLowerCase()) {
                console.log(`Language changed from settings: ${previousTag} to ${value.targetLanguage}`);
                // The resetAllFeatures call will be triggered by validateLanguageTag
            }
            
            validateLanguageTag(value.targetLanguage, true);
        }
    });
    
    settings.subscribe(value => {
        if (value) {
            updateProviderWarnings();
        }
    });
    
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
    
    // Remove all browser URL validation errors when the component is updated
    $: {
        // Clear any legacy browser URL errors
        errorStore.removeError('invalid-browser-url');
        
        // Also clear any group browser URL errors if no subtitle features are enabled
        const anySubtitleFeatureEnabled = features.some(f => 
            f.featureGroups?.includes('subtitle') && selectedFeatures[f.id]);
            
        if (!anySubtitleFeatureEnabled) {
            errorStore.removeError('group-subtitle-browser-url');
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
    // Initialize feature groups
    function initializeFeatureGroups() {
        console.log(`Initializing feature groups - current language: ${standardTag}`);
        
        // First, handle existing features to ensure they're visible
        // This ensures all feature cards are created correctly first
        for (let feature of features) {
            // Include selective transliteration regardless of its label (which changes based on language)
            const isSubtitleFeature = feature.id === 'subtitleRomanization' || 
                                      feature.id === 'selectiveTransliteration' || 
                                      feature.id === 'subtitleTokenization';
                                      
            if (isSubtitleFeature) {
                console.log(`Adding ${feature.id} to subtitle group`);
                
                // Mark for group membership but don't initialize fully yet
                if (!feature.featureGroups) {
                    feature.featureGroups = ['subtitle'];
                } else if (!feature.featureGroups.includes('subtitle')) {
                    feature.featureGroups.push('subtitle');
                }
                
                // Make sure groupSharedOptions are defined
                if (!feature.groupSharedOptions) {
                    feature.groupSharedOptions = {
                        'subtitle': ['style', 'provider', 'dockerRecreate', 'browserAccessURL']
                    };
                } else if (!feature.groupSharedOptions['subtitle']) {
                    feature.groupSharedOptions['subtitle'] = ['style', 'provider', 'dockerRecreate', 'browserAccessURL'];
                }
                
                // Register each feature in the group store individually
                featureGroupStore.addFeatureToGroup('subtitle', feature.id);
                
                // Initialize feature options if not already set
                if (!currentFeatureOptions[feature.id]) {
                    currentFeatureOptions[feature.id] = {};
                }
                
                // Make sure shared options exist
                ['style', 'provider', 'dockerRecreate', 'browserAccessURL'].forEach(optionId => {
                    if (currentFeatureOptions[feature.id][optionId] === undefined) {
                        // Initialize with default or empty value
                        if (optionId === 'dockerRecreate') {
                            currentFeatureOptions[feature.id][optionId] = false;
                        } else if (optionId === 'style' && romanizationSchemes.length > 0) {
                            currentFeatureOptions[feature.id][optionId] = romanizationSchemes[0].name;
                        } else if (optionId === 'provider' && romanizationSchemes.length > 0) {
                            currentFeatureOptions[feature.id][optionId] = romanizationSchemes[0].provider;
                        } else {
                            currentFeatureOptions[feature.id][optionId] = '';
                        }
                    }
                });
            }
            
            // Handle merge features
            const isMergeFeature = feature.outputMergeGroup === 'finalOutput';
            if (isMergeFeature) {
                console.log(`Adding ${feature.id} to finalOutput merge group`);
                
                // Mark for group membership
                if (!feature.featureGroups) {
                    feature.featureGroups = ['finalOutput'];
                } else if (!feature.featureGroups.includes('finalOutput')) {
                    feature.featureGroups.push('finalOutput');
                }
                
                // Make sure groupSharedOptions are defined
                if (!feature.groupSharedOptions) {
                    feature.groupSharedOptions = {
                        'finalOutput': ['mergeOutputFiles', 'mergingFormat']
                    };
                } else if (!feature.groupSharedOptions['finalOutput']) {
                    feature.groupSharedOptions['finalOutput'] = ['mergeOutputFiles', 'mergingFormat'];
                }
                
                // Register each feature in the group store individually
                featureGroupStore.addFeatureToGroup('finalOutput', feature.id);
                
                // Initialize feature options if not already set
                if (!currentFeatureOptions[feature.id]) {
                    currentFeatureOptions[feature.id] = {};
                }
            }
        }
        
        // Define the subtitle group
        const subtitleGroup: FeatureGroup = {
            id: 'subtitle',
            label: 'Subtitle Processing',
            description: 'Features related to subtitle processing',
            featureIds: ['subtitleRomanization', 'selectiveTransliteration', 'subtitleTokenization'],
            sharedOptions: ['style', 'provider', 'dockerRecreate', 'browserAccessURL'],
            validationRules: [
                {
                    id: 'browser-url-validation',
                    optionId: 'browserAccessURL',
                    // Fixed validation that runs only when needed
                    validator: (url) => {
                        // If scraper isn't needed, don't validate
                        if (!needsScraper) return true;
                        
                        // Check for a valid WebSocket URL
                        return Boolean(url && url.startsWith('ws://'));
                    },
                    errorMessage: 'Valid browser access URL is required for web scraping',
                    severity: 'critical'
                }
            ]
        };
        
        // Define the merge output group
        const mergeGroup: FeatureGroup = {
            id: 'finalOutput',
            label: 'Output Merging',
            description: 'Features that can be merged into final output',
            featureIds: [
                'dubtitles', 
                'voiceEnhancing', 
                'subtitleRomanization', 
                'selectiveTransliteration', 
                'subtitleTokenization'
            ],
            sharedOptions: ['mergeOutputFiles', 'mergingFormat']
        };
        
        // Register feature groups in the store
        featureGroupStore.registerGroup(subtitleGroup);
        featureGroupStore.registerGroup(mergeGroup);
        
        // Update subtitle features with shared options
        const subtitleFeatures = features.filter(f => 
            ['subtitleRomanization', 'selectiveTransliteration', 'subtitleTokenization'].includes(f.id));
            
        subtitleFeatures.forEach(feature => {
            // Make sure group membership is set
            if (!feature.featureGroups) {
                feature.featureGroups = ['subtitle'];
            } else if (!feature.featureGroups.includes('subtitle')) {
                feature.featureGroups.push('subtitle');
            }
            
            // Add group shared options
            if (!feature.groupSharedOptions) {
                feature.groupSharedOptions = {};
            }
            
            // Define which options are shared in the subtitle group
            feature.groupSharedOptions['subtitle'] = ['style', 'provider', 'dockerRecreate', 'browserAccessURL'];
            
            // Make sure the feature has the required options defined
            if (!currentFeatureOptions[feature.id]) {
                currentFeatureOptions[feature.id] = {};
            }
        });
        
        // Initialize shared options from existing feature options - use the first available value
        const initialGroupOptions: Record<string, any> = {
            'style': 'paiboon', // Default to paiboon if nothing set
            'provider': '',
            'browserAccessURL': '',
            'dockerRecreate': false
        };
        
        // Scan all subtitle features for options to use as initial values
        subtitleFeatures.forEach(feature => {
            if (currentFeatureOptions[feature.id]) {
                const options = currentFeatureOptions[feature.id];
                
                ['style', 'provider', 'browserAccessURL', 'dockerRecreate'].forEach(optionId => {
                    // Only set if the option has a non-empty value
                    if (options[optionId] !== undefined && options[optionId] !== '' && 
                        (optionId !== 'style' || options[optionId] !== 'paiboon')) {
                        initialGroupOptions[optionId] = options[optionId];
                    }
                });
            }
        });
        
        // Apply all collected initial values to the group
        Object.entries(initialGroupOptions).forEach(([optionId, value]) => {
            featureGroupStore.setGroupOption('subtitle', optionId, value);
        });
        
        // Apply group options to all features to ensure consistency
        subtitleFeatures.forEach(feature => {
            ['style', 'provider', 'browserAccessURL', 'dockerRecreate'].forEach(optionId => {
                currentFeatureOptions[feature.id][optionId] = initialGroupOptions[optionId];
            });
        });
        
        // Initialize merge group options
        featureGroupStore.setGroupOption('finalOutput', 'mergeOutputFiles', mergeOptionValues.mergeOutputFiles);
        featureGroupStore.setGroupOption('finalOutput', 'mergingFormat', mergeOptionValues.mergingFormat);
    }

    // Function to register the display order of features in the UI
    function registerFeatureDisplayOrder() {
        // Get all visible features in their current DOM order
        const featureElements = Array.from(document.querySelectorAll('[data-feature-id]'));
        
        if (featureElements.length === 0) {
            console.log('No feature elements found in the DOM yet');
            return;
        }
        
        const orderedFeatureIds = featureElements
            .map(el => el.getAttribute('data-feature-id'))
            .filter(Boolean);
        
        console.log('Current feature display order:', orderedFeatureIds);
        
        // Update the display order for each group
        Object.keys(featureGroupStore.getGroups()).forEach(groupId => {
            featureGroupStore.updateFeatureDisplayOrder(groupId, orderedFeatureIds);
        });
    }
    
    // Update display order when features are fully rendered
    afterUpdate(() => {
        if (isInitialDataLoaded && visibleFeatures.length > 0) {
            // Use a slight delay to ensure the DOM is fully updated
            setTimeout(registerFeatureDisplayOrder, 100);
        }
    });
    
    onMount(async () => {
        console.log("FeatureSelector mounting - loading data...");
        
        try {
            // Initialize canonical feature order from feature definitions
            const canonicalOrder = features.map(f => f.id);
            featureGroupStore.initializeCanonicalOrder(canonicalOrder);
            console.log('Initialized canonical feature order:', canonicalOrder);
            
            // Initialize feature groups 
            initializeFeatureGroups();
            
            // Load all necessary data before showing the component
            // This prevents visual glitches during initialization
            const currentSettings = get(settings);
            
            if (currentSettings?.targetLanguage) {
                quickAccessLangTag = currentSettings.targetLanguage;
                
                // Execute these operations sequentially to ensure consistent state
                await validateLanguageTag(currentSettings.targetLanguage, true);
                await checkTokenization(quickAccessLangTag);
            }
            
            // Prepare component by loading config and options
            updateProviderWarnings();
            cleanupFeatureOptions(); // Clean up feature options on mount
            
            // Make sure we're fully loaded before starting animations
            await new Promise(resolve => setTimeout(resolve, 50));
            
            // Mark component as ready BEFORE starting animations
            isInitialDataLoaded = true;
            console.log("FeatureSelector initial data loaded successfully");
            
            // Restore the progressive reveal animation
            // Check if device has reduced motion preference
            const prefersReducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
            
            if (prefersReducedMotion) {
                // Respect user's motion preference - show all features at once
                visibleFeatures = Object.keys(selectedFeatures);
                
                // Register display order immediately if not using animations
                setTimeout(registerFeatureDisplayOrder, 50);
            } else {
                // Use proper staggered animation with timeouts
                const allFeatures = Object.keys(selectedFeatures);
                
                // Clear visibleFeatures to ensure animation starts fresh
                visibleFeatures = [];
                
                // Use a more dynamic animation approach based on the original features order
                // This ensures we respect the intended UX design
                
                // First, ensure we're working with the original feature order from the features array
                const orderedFeatures = features
                    .map(f => f.id)
                    .filter(id => allFeatures.includes(id));
                
                // Add features one by one with staggered delays
                orderedFeatures.forEach((feature, index) => {
                    // Create an exponential delay curve for a more natural feel
                    // First items appear quickly, later items have increasing delays
                    const baseDelay = 100;
                    const incrementFactor = 1.75;
                    const delay = baseDelay * Math.pow(incrementFactor, index / 1.2);
                    
                    setTimeout(() => {
                        console.log(`Revealing feature ${feature} at ${Math.round(delay)}ms`);
                        visibleFeatures = [...visibleFeatures, feature];
                    }, delay);
                });
                
                // Register display order after all animations complete
                const maxDelay = 100 * Math.pow(1.75, orderedFeatures.length / 1.2) + 200;
                setTimeout(registerFeatureDisplayOrder, maxDelay);
            }
        } catch (error) {
            console.error("Error during FeatureSelector initialization:", error);
            // Mark as loaded anyway to prevent endless loading state
            isInitialDataLoaded = true;
            
            // In case of error, show all features at once
            visibleFeatures = Object.keys(selectedFeatures);
            
            // Log the error to help with debugging
            logStore.addLog({
                level: 'ERROR',
                message: `Error initializing feature selector: ${error.message}`,
                time: new Date().toISOString()
            });
        }
    });
    
    function softLanding(t) {
       return 1 - Math.pow(1 - t, 3.5);
    }

    onDestroy(() => {
        console.log('FeatureSelector unmounting, cleaning up errors');
        
        // Clear legacy errors
        errorStore.removeError('docker-required');
        errorStore.removeError('invalid-browser-url');
        errorStore.removeError('no-features');
        errorStore.removeError('invalid-language');
        errorStore.removeError('provider-dubtitles');
        errorStore.removeError('provider-voiceEnhancing');
        errorStore.removeError('no-romanization');
        errorStore.removeError('no-selective-transliteration');
        
        // Clear feature group errors - be thorough with all possible error IDs
        featureGroupStore.clearGroupErrors('subtitle');
        errorStore.removeError('group-subtitle-browser-url');
        errorStore.removeError('group-subtitle-browser-url-validation');
    });
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between pl-0 pr-0">
        <h2 class="text-xl font-medium text-tertiary flex items-center pl-4 gap-2">
            <span class="material-icons text-tertiary">tune</span>
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
    
    <!-- Feature cards container - only rendered after data is fully loaded -->
    <div class="space-y-4 overflow-visible">
        {#if isInitialDataLoaded}
            {#each features.filter(f => visibleFeatures.includes(f.id) && (!f.showCondition || shouldShowFeature(f))) as feature, i (feature.id)}
                <div 
                    in:fly={{ 
                        x: 400,
                        duration: Math.min(550, 550 - (i * 15)),
                        easing: softLanding,
                        opacity: 0
                    }}
                    style="will-change: transform, opacity; overflow: visible;"
                    class="px-0 my-2"
                >
                    <div data-feature-id={feature.id} class="overflow-visible">
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
                            {mergeOptionValues}
                            on:enabledChange={handleFeatureEnabledChange}
                            on:optionChange={handleOptionChange}
                        />
                    </div>
                </div>
            {/each}
        {/if}
        <br>
    </div>
</div>

<style>
    /* Add smooth animation for cards when scrolling */
    div {
        will-change: transform;
        transform: translateZ(0);
    }
</style>