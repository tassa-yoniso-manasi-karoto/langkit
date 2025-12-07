<script lang="ts">
    import { createEventDispatcher, onMount, onDestroy, afterUpdate, tick } from 'svelte';
    import { fly } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import { get } from 'svelte/store';
    
    import { settings, showSettings, llmStateStore, welcomePopupVisible, type LLMStateChange } from '../lib/stores.ts';
    import { wsClient } from '../ws/client';
    import { updateSTTModels, sttModelsStore } from '../lib/featureModel';
    import { invalidationErrorStore } from '../lib/invalidationErrorStore';
    import { logStore } from '../lib/logStore';
    import { logger } from '../lib/logger';
    import {
        features,
        featuresStore,
        createDefaultOptions,
        providerGithubUrls,
        providersRequiringTokens,
        updateSummaryProviders,
        updateSummaryModels,
        updateFeatureChoices,
        summaryProvidersStore,
        summaryModelsStore,
        type RomanizationScheme
    } from '../lib/featureModel';
    import { 
        featureGroupStore, 
        type FeatureGroup,
        groupHasEnabledFeature,
        romanizationSchemesStore,
        languageRequirementsStore
    } from '../lib/featureGroupStore';
    import { 
        CheckMediaLanguageTags
    } from '../api/services/media';
    import {
        GetRomanizationStyles,
        ValidateLanguageTag,
        NeedsTokenization
    } from '../api';
    import {
        GetAvailableSTTModelsForUI,
        GetAvailableSummaryProviders,
        GetAvailableSummaryModels
    } from '../api/services/models';
    import { debounce } from 'lodash';
    import { getMediumDebounce, getTinyDebounce, getSmallDebounce } from '../lib/debouncePresets';
    
    import FeatureCard from './FeatureCard.svelte';
    import QuickAccessLangSelector from './QuickAccessLangSelector.svelte';

    // Props
    export let selectedFeatures = {
        subs2cards: false,
        dubtitles: false,
        voiceEnhancing: false,
        subtitleRomanization: false,
        selectiveTransliteration: false,
        subtitleTokenization: false,
        condensedAudio: false
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
    let isRomanizationAvailable = false;
    let isSelectiveTransliterationAvailable = false;
    
    let dockerUnreachable = false;
    let dockerEngine = '';
    let needsDocker = false;
    let needsScraper = false;
    
    let showAudioTrackIndex = false;
    let audioTrackIndex = 0;
    let hasLanguageTags = true;

    // New loading state flags
    let isProcessingLanguage = false;
    let isLoadingSchemes = false;

    // Group tracking for reference
    let providerGroups: Record<string, string[]> = {
        subtitle: ['subtitleRomanization', 'selectiveTransliteration', 'subtitleTokenization']
    };
    
    let currentSTTModels = { models: [], names: [], available: false, suggested: "" };
    let sttModelsUnsubscribe: () => void;
    
    // Same for summary providers and models
    let currentSummaryProviders = { providers: [], names: [], available: false, suggested: "" };
    let currentSummaryModels = { models: [], names: [], available: false, suggested: "" };
    let summaryProvidersUnsubscribe: () => void;
    let summaryModelsUnsubscribe: () => void;
    
    // Reactive features array from store
    let reactiveFeatures = features;
    let featuresUnsubscribe: () => void;
    
    // LLM state management
    let llmState: LLMStateChange | null = null;
    let llmStateUnsubscribe: () => void;
    let isLLMInitializing = true;
    
    // WebSocket event handlers for proper cleanup
    let llmStateChangedHandler: (data: any) => void;
    let connectedHandler: () => void;
    
    const dispatch = createEventDispatcher();
    
    /**
     * Main language validation function
     * Separated from UI updates for cleaner architecture
     */
    async function validateLanguage(code: string, maxOne: boolean = true): Promise<void> {
        if (!code) {
            isValidLanguage = null;
            standardTag = '';
            validationError = '';
            return;
        }
        
        isChecking = true;
        try {
            const response = await ValidateLanguageTag(code, maxOne);
            isValidLanguage = response.isValid;
            standardTag = response.standardTag || '';
            validationError = response.error || '';
            logger.debug('FeatureSelector', 'Language validated', { 
            code, 
            standardTag, 
            isValid: isValidLanguage, 
            error: validationError 
        });
        } catch (error) {
            logger.error('featureSelector', 'Error checking language code', { error });
            isValidLanguage = null;
            standardTag = '';
            validationError = 'Validation failed';
        } finally {
            isChecking = false;
        }
    }
    
    /**
     * Fetch available summary providers from the backend
     * NOTE: This is NOT debounced because the UI needs immediate updates
     * for proper dropdown reactivity and auto-selection
     */
    async function fetchSummaryProviders() {
        logger.info('FeatureSelector', 'Fetching available summary providers');
        try {
            const summaryProviders = await GetAvailableSummaryProviders();
            
            // Update feature models and store
            updateSummaryProviders(summaryProviders);
            
            if (!summaryProviders.available) {
                // Handle case where no providers are available
                invalidationErrorStore.addError({
                    id: 'no-summary-providers',
                    message: 'No summary providers available. Check API keys in settings.',
                    severity: 'warning',
                    action: {
                        label: 'Open Settings',
                        handler: () => {
                            showSettings.set(true);
                        }
                    }
                });
            } else {
                // If the error was previously shown and providers are now available, remove it
                invalidationErrorStore.removeError('no-summary-providers');
            }
            
            // Update the local state
            currentSummaryProviders = summaryProviders;
            
            // If there's a suggested provider, fetch its models
            if (summaryProviders.suggested) {
                await fetchSummaryModels(summaryProviders.suggested);
            }
            
            // Force reactivity update to ensure dropdowns are populated
            currentFeatureOptions = {...currentFeatureOptions};
            
            return summaryProviders;
        } catch (error) {
            logger.error('featureSelector', 'Failed to load summary providers', { error });
            return { providers: [], names: [], available: false, suggested: "" };
        }
    }
    
    // Store a mapping of provider -> models to avoid race conditions
    let providerModelsMap: Record<string, string[]> = {};
    
    /**
     * Fetch available models for a specified summary provider
     * NOTE: This is NOT debounced because the UI needs immediate updates
     * for proper dropdown reactivity and auto-selection
     */
    async function fetchSummaryModels(providerName: string) {
        if (!providerName) {
            logger.warn('featureSelector', 'Cannot fetch summary models: No provider specified');
            return { models: [], names: [], available: false, suggested: "" };
        }
        
        logger.debug('featureSelector', `Fetching available summary models for provider: ${providerName}`);
        try {
            // Starting fetch
            const fetchStartTime = Date.now();
            const summaryModels = await GetAvailableSummaryModels(providerName);
            logger.debug('featureSelector', `Received models for ${providerName} after ${Date.now() - fetchStartTime}ms`, {
                modelCount: summaryModels.names.length,
                firstThree: summaryModels.names.slice(0, 3)
            });
            
            // IMPORTANT: Store models in our provider-specific map
            providerModelsMap[providerName] = [...summaryModels.names];
            
            // Only update the UI if this is still the current provider
            // This fixes the race condition where multiple providers' models are being fetched
            const currentProvider = currentFeatureOptions?.condensedAudio?.summaryProvider;
            if (currentProvider === providerName) {
                logger.debug('featureSelector', `Updating UI for current provider: ${providerName}`);
                
                // Update feature models and store with provider context
                updateSummaryModels(summaryModels, providerName);
                
                // Update the selected model if needed
                if (summaryModels.names.length > 0 && 
                    (!currentFeatureOptions.condensedAudio.summaryModel || 
                     !summaryModels.names.includes(currentFeatureOptions.condensedAudio.summaryModel))) {
                    const newModel = summaryModels.suggested || summaryModels.names[0];
                    currentFeatureOptions.condensedAudio.summaryModel = newModel;
                    
                    // Notify parent of the model change
                    dispatch('optionChange', {
                        featureId: 'condensedAudio',
                        optionId: 'summaryModel',
                        value: newModel
                    });
                }
                
                // Force reactivity update
                currentFeatureOptions = {...currentFeatureOptions};
            } else {
                logger.debug('featureSelector', `Ignoring models for ${providerName} because current provider is ${currentProvider}`);
            }
            
            if (!summaryModels.available) {
                // Handle case where no models are available
                invalidationErrorStore.addError({
                    id: 'no-summary-models',
                    message: `No summary models available for ${providerName}. Check API keys in settings.`,
                    severity: 'warning',
                    action: {
                        label: 'Open Settings',
                        handler: () => {
                            showSettings.set(true);
                        }
                    }
                });
            } else {
                // If the error was previously shown and models are now available, remove it
                invalidationErrorStore.removeError('no-summary-models');
            }
            
            // Update the local state
            currentSummaryModels = summaryModels;
            
            return summaryModels;
        } catch (error) {
            logger.error('featureSelector', `Failed to load summary models for provider: ${providerName}`, { error });
            return { models: [], names: [], available: false, suggested: "" };
        }
    }
    
    /**
     * Pure function to load romanization schemes
     * Only performs the API call and updates scheme data
     */
    async function loadRomanizationSchemes(tag: string): Promise<boolean> {
        if (!tag?.trim()) {
            romanizationSchemes = [];
            isRomanizationAvailable = false;
            isSelectiveTransliterationAvailable = false;
            needsDocker = false;
            needsScraper = false;
            
            // Clear the stores
            romanizationSchemesStore.set([]);
            languageRequirementsStore.set({
                needsScraper: false,
                needsDocker: false,
                dockerUnreachable: false,
                dockerEngine: 'Docker'
            });
            
            return false;
        }

        isLoadingSchemes = true;
        try {
            const response = await GetRomanizationStyles(tag);
            
            romanizationSchemes = response.schemes || [];
            isRomanizationAvailable = romanizationSchemes.length > 0;
            
            isSelectiveTransliterationAvailable = tag === 'jpn';
            needsScraper = response.needsScraper || false;
            dockerUnreachable = response.dockerUnreachable || false;
            needsDocker = response.needsDocker || false;
            dockerEngine = response.dockerEngine || 'Docker Desktop';
            
            // Update the reactive stores
            romanizationSchemesStore.set(romanizationSchemes);
            languageRequirementsStore.set({
                needsScraper: response.needsScraper || false,
                needsDocker: response.needsDocker || false,
                dockerUnreachable: response.dockerUnreachable || false,
                dockerEngine: response.dockerEngine || 'Docker Desktop'
            });
            
            logger.debug('FeatureSelector', 'Romanization schemes loaded', { 
                tag, 
                schemeCount: romanizationSchemes.length, 
                needsDocker, 
                needsScraper 
            });
            
            return isRomanizationAvailable;
        } catch (error) {
            logger.error('FeatureSelector', 'Error fetching romanization styles', { error });
            romanizationSchemes = [];
            isRomanizationAvailable = false;
            isSelectiveTransliterationAvailable = false;
            
            // Clear the stores on error
            romanizationSchemesStore.set([]);
            languageRequirementsStore.set({
                needsScraper: false,
                needsDocker: false,
                dockerUnreachable: false,
                dockerEngine: 'Docker'
            });
            
            return false;
        } finally {
            isLoadingSchemes = false;
        }
    }
    
    /**
     * Applies the default romanization style when schemes are available
     * This is a synchronous function that updates UI state
     */
    function applyDefaultRomanizationStyle(): void {
        if (romanizationSchemes.length === 0) {
            logger.debug('FeatureSelector', 'No romanization schemes available to set as default');
            return;
        }
        
        const newStyle = romanizationSchemes[0].name;
        const newProvider = (romanizationSchemes[0].providers || []).join(' → ');
        
        logger.info('FeatureSelector', 'Setting default romanization style', { 
            style: newStyle, 
            provider: newProvider 
        });
        
        // Update group store options
        featureGroupStore.setGroupOption('subtitle', 'style', newStyle);
        featureGroupStore.setGroupOption('subtitle', 'provider', newProvider);
        
        // Update feature options for all subtitle features
        ['subtitleRomanization', 'selectiveTransliteration', 'subtitleTokenization'].forEach(featureId => {
            if (currentFeatureOptions[featureId]) {
                currentFeatureOptions[featureId].style = newStyle;
                currentFeatureOptions[featureId].provider = newProvider;
            }
        });
        
        // Force synchronization to all features
        currentFeatureOptions = featureGroupStore.syncOptionsToFeatures('subtitle', currentFeatureOptions);
        
        // Notify parent of changes
        dispatch('optionsChange', currentFeatureOptions);
    }
    
    /**
     * Checks if tokenization is supported for a language
     */
    async function checkTokenization(code: string): Promise<boolean> {
        try {
            tokenizationAllowed = await NeedsTokenization(code);
            logger.debug('FeatureSelector', 'Tokenization support checked', { 
                languageCode: code, 
                allowed: tokenizationAllowed 
            });
            return tokenizationAllowed;
        } catch (err) {
            logger.error('FeatureSelector', 'Error checking tokenization support', { error: err });
            tokenizationAllowed = false;
            return false;
        }
    }
    
    /**
     * Master function to handle language changes
     * Coordinates all the steps in a clean, sequential way
     */
    async function processLanguageChange(newLanguage: string): Promise<void> {
        if (isProcessingLanguage) {
            logger.trace('FeatureSelector', 'Already processing language change, skipping');
            return;
        }
        
        isProcessingLanguage = true;
        logger.info('FeatureSelector', 'Processing language change', { newLanguage });
        
        try {
            // Step 1: Reset all feature selections for safety
            if (newLanguage) {
                resetAllFeatures();
            }
            
            // Step 2: Validate the language
            await validateLanguage(newLanguage, true);
            
            if (!isValidLanguage && newLanguage) {
                logger.warn('FeatureSelector', 'Invalid language', { language: newLanguage });
                return;
            }
            
            // Step 3: Use the standardized tag if available, otherwise use raw input
            const effectiveTag = standardTag || newLanguage;
            
            // Step 4: Load romanization schemes
            const schemesAvailable = await loadRomanizationSchemes(effectiveTag);
            
            // Step 5: Check tokenization support
            await checkTokenization(effectiveTag);
            
            // Step 6: Apply default style if schemes are available
            if (schemesAvailable) {
                applyDefaultRomanizationStyle();
            }
            
            // Step 7: Update errors based on availability
            debouncedUpdateFeatureAvailabilityErrors();
            
        } catch (error) {
            logger.error('FeatureSelector', 'Error during language change processing', { error });
            logStore.addLog({
                level: 'ERROR',
                message: `Error processing language change: ${error.message}`,
                time: new Date().toISOString()
            });
        } finally {
            isProcessingLanguage = false;
        }
    }
    
    // Declare debounced functions - will be initialized in onMount
    let debouncedProcessLanguageChange: typeof processLanguageChange;
    let debouncedUpdateProviderWarnings: typeof updateProviderWarnings;
    let debouncedUpdateFeatureAvailabilityErrors: typeof updateFeatureAvailabilityErrors;
    
    /**
     * Update error messages based on feature availability
     */
    function updateFeatureAvailabilityErrors(): void {
        // Handle romanization availability
        if (!isRomanizationAvailable && selectedFeatures.subtitleRomanization) {
            selectedFeatures.subtitleRomanization = false;
            invalidationErrorStore.addError({
                id: 'no-romanization',
                message: 'No transliteration scheme available for selected language',
                severity: 'warning'
            });
        } else {
            invalidationErrorStore.removeError('no-romanization');
        }
        
        // Handle selective transliteration availability
        if (!isSelectiveTransliterationAvailable && selectedFeatures.selectiveTransliteration) {
            selectedFeatures.selectiveTransliteration = false;
            invalidationErrorStore.addError({
                id: 'no-selective-transliteration',
                message: 'Kanji to Kana transliteration is only available for Japanese',
                severity: 'warning'
            });
        } else {
            invalidationErrorStore.removeError('no-selective-transliteration');
        }
    }
    
    // Function to reset all feature selections
    function resetAllFeatures() {
        logger.info('FeatureSelector', 'Resetting all feature selections due to language change');
        
        // Disable all features
        Object.keys(selectedFeatures).forEach(featureId => {
            if (selectedFeatures[featureId]) {
                // Set to false
                selectedFeatures[featureId] = false;
                
                // Handle feature groups
                const featureDef = reactiveFeatures.find(f => f.id === featureId);
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

    // Handle feature click for toggling and unavailable features
    function handleFeatureEnabledChange(event: CustomEvent) {
        const { id, enabled } = event.detail;
        logger.info('FeatureSelector', 'Feature toggled', { featureId: id, enabled });
        
        // Update the selected features state
        selectedFeatures[id] = enabled;
        debouncedUpdateProviderWarnings();
        
        // Find the feature definition
        const featureDef = reactiveFeatures.find(f => f.id === id);
        if (!featureDef) {
            logger.error('FeatureSelector', 'Feature not found', { featureId: id });
            return;
        }
        
        if (featureDef.featureGroups?.length) {
            handleFeatureGroupUpdates(featureDef, id, enabled);
            
            // Update display order after feature enable/disable to ensure proper topmost display
            setTimeout(registerFeatureDisplayOrder, 50);
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
        logger.trace('featureSelector', `Feature ${featureId} belongs to groups: ${featureDef.featureGroups.join(', ')}`);
        
        featureDef.featureGroups.forEach(groupId => {
            logger.trace('featureSelector', `Processing group ${groupId} for feature ${featureId}`);
            
            // Update enabled state in the group store
            featureGroupStore.updateFeatureEnabled(groupId, featureId, enabled);
            
            // Get all feature IDs in this group for reference
            const groupFeatureIds = getFeatureIdsInGroup(groupId);
            
            // Get all enabled features in this group
            const enabledFeaturesInGroup = getEnabledFeaturesInGroup(groupId);
            logger.trace('featureSelector', `Group ${groupId} has ${enabledFeaturesInGroup.length} enabled features}`);
            
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
        logger.trace('featureSelector', `Feature ${featureId} enabled - checking if it should be active display for group ${groupId}`);
        
        // Get the features in the order they should be prioritized
        const enabledOrderedFeatures = groupFeatureIds.filter(fId => enabledFeaturesInGroup.includes(fId));
        
        // If this is the highest priority enabled feature, make it the active display feature
        if (enabledOrderedFeatures.length > 0 && enabledOrderedFeatures[0] === featureId) {
            logger.trace('featureSelector', `Making ${featureId} the active display feature for group ${groupId}`);
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
            logger.debug('FeatureSelector', 'Active display feature disabled', { 
                featureId, 
                groupId, 
                remainingEnabledFeatures: enabledFeaturesInGroup.length 
            });
            
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
        logger.debug('FeatureSelector', 'Syncing options for group to features', { groupId });
        currentFeatureOptions = featureGroupStore.syncOptionsToFeatures(
            groupId, currentFeatureOptions
        );
    }

    // Improved provider warning checks
    function updateProviderWarnings() {
        logger.trace('FeatureSelector', 'Running updateProviderWarnings check');
        
        // Check dubtitles STT provider
        if (selectedFeatures.dubtitles && currentFeatureOptions.dubtitles) {
            const sttModel = currentFeatureOptions.dubtitles.stt;
            logger.trace('FeatureSelector', 'Checking provider requirements for STT model', { sttModel });
            
            // Find the model info to get the provider
            const modelInfo = currentSTTModels.models.find(m => m.name === sttModel);
            
            if (modelInfo) {
                const providerName = modelInfo.providerName.toLowerCase(); // e.g., "openai", "replicate"
                logger.trace('FeatureSelector', 'Model provider identified', { providerName });
                
                // Check if this provider requires a token
                const { isValid, tokenType } = checkProviderApiToken(providerName);
                logger.debug('FeatureSelector', 'Provider token check', { 
                    provider: providerName, 
                    isValid, 
                    tokenType 
                });
                
                if (!isValid) {
                    // Use addError to add/update the error message
                    const errorMessage = `${tokenType || providerName} API token is required for ${modelInfo.displayName}`;
                    logger.debug('FeatureSelector', 'Adding provider error', { 
                        errorId: 'provider-dubtitles', 
                        message: errorMessage 
                    });
                    
                    invalidationErrorStore.addError({
                        id: 'provider-dubtitles',
                        message: errorMessage,
                        severity: 'critical'
                    });
                } else {
                    // Remove the error if it exists
                    logger.trace('FeatureSelector', 'Token is valid, removing provider error', { 
                        errorId: 'provider-dubtitles' 
                    });
                    invalidationErrorStore.removeError('provider-dubtitles');
                }
            } else {
                logger.warn('FeatureSelector', 'Could not find model info', { sttModel });
                // Clear any existing error if model not found
                invalidationErrorStore.removeError('provider-dubtitles');
            }
        } else {
            // Remove the error if the feature is disabled
            logger.trace('FeatureSelector', 'Feature not selected, removing provider error', { 
                errorId: 'provider-dubtitles' 
            });
            invalidationErrorStore.removeError('provider-dubtitles');
        }

        // Check voice enhancing provider with similar pattern
        if (selectedFeatures.voiceEnhancing && currentFeatureOptions.voiceEnhancing) {
            const sepLib = currentFeatureOptions.voiceEnhancing.sepLib;
            const { isValid, tokenType } = checkProviderApiToken(sepLib);

            if (!isValid) {
                invalidationErrorStore.addError({
                    id: 'provider-voiceEnhancing',
                    message: `${tokenType || sepLib} API token is required for ${sepLib}`,
                    severity: 'critical'
                });
            } else {
                invalidationErrorStore.removeError('provider-voiceEnhancing');
            }
        } else {
            invalidationErrorStore.removeError('provider-voiceEnhancing');
        }
    }

    // Improved provider check with explicit logging
    function checkProviderApiToken(provider: string): { isValid: boolean; tokenType: string | null } {
        // Map provider names from STT models to their corresponding API key names in settings
        const providerKeyMapping: Record<string, string> = {
            'replicate': 'replicate',
            'openai': 'openAI',
            'elevenlabs': 'elevenLabs'
        };
        
        logger.trace('FeatureSelector', 'Checking API token for provider', { provider });
        
        // Normalize provider name to lowercase for case-insensitive matching
        const normalizedProvider = provider.toLowerCase();
        
        // Get the appropriate token type using the mapping
        let tokenType = providerKeyMapping[normalizedProvider];
        
        // Fallback to original mapping if not found
        if (!tokenType) {
            tokenType = providersRequiringTokens[normalizedProvider];
        }
        
        logger.trace('FeatureSelector', 'Token type determined', { 
            provider, 
            tokenType: tokenType || 'none required' 
        });
        
        // Check if token is needed
        if (!tokenType) {
            return { isValid: true, tokenType: null };
        }
        
        // Check settings for the token
        const currentSettings = get(settings);
        
        // Ensure settings and apiKeys exist
        if (!currentSettings || !currentSettings.apiKeys) {
            return { isValid: false, tokenType };
        }
        
        // Check if token has a value
        const hasToken = Boolean(currentSettings.apiKeys[tokenType]?.trim());
        
        logger.debug('FeatureSelector', 'Token status', { 
            provider, 
            tokenType, 
            hasToken 
        });
        
        return { 
            isValid: hasToken,
            tokenType
        };
    }
    
    // Media file checking
    async function checkMediaFiles() {
        if (mediaSource) {
            try {
                const info = await CheckMediaLanguageTags(mediaSource.path);
                hasLanguageTags = info.hasLanguageTags;
                showAudioTrackIndex = !hasLanguageTags;
            } catch (error) {
                logger.error('FeatureSelector', 'Error checking media files', { error });
            }
        }
    }
    
    
    function handleAudioTrackChange(event: CustomEvent) {
        showAudioTrackIndex = event.detail.showAudioTrackIndex;
        audioTrackIndex = event.detail.audioTrackIndex;
        logger.debug('FeatureSelector', 'Audio track changed', { 
            showAudioTrackIndex, 
            audioTrackIndex 
        });
    }

    let isProcessingSTTChange = false;
    function handleOptionChange(event: CustomEvent) {
        const { featureId, optionId, value, isGroupOption, groupId, isSTTModelChange } = event.detail;
        
        logger.debug('FeatureSelector', 'Option changed', { 
            featureId, 
            optionId, 
            value, 
            isGroupOption, 
            groupId 
        });
        
        // For non-group options, directly update the feature's options (if needed)
        if (!isGroupOption) {
            // Check if the value has already been updated by a previous handler
            if (currentFeatureOptions[featureId][optionId] !== value) {
                currentFeatureOptions[featureId][optionId] = value;
            }
        }
        
       // Special handling for STT model changes
        if (isSTTModelChange && featureId === 'dubtitles' && optionId === 'stt') {
            // Prevent duplicate processing
            if (isProcessingSTTChange) {
                logger.trace('FeatureSelector', 'Ignoring recursive STT model change event', { model: value });
                return;
            }

            // Set flag to prevent processing duplicates
            isProcessingSTTChange = true;

            try {
                logger.info('FeatureSelector', 'Handling STT model change', { newModel: value });

                // Force provider warnings check immediately
                debouncedUpdateProviderWarnings();
            } finally {
                // Always reset flag
                isProcessingSTTChange = false;
            }
        }

        // Special handling for voice separation library changes
        if (featureId === 'voiceEnhancing' && optionId === 'sepLib') {
            logger.info('FeatureSelector', 'Handling sepLib change', { newSepLib: value });

            // Use the new value directly (not from currentFeatureOptions which may not be updated yet)
            if (selectedFeatures.voiceEnhancing) {
                const { isValid, tokenType } = checkProviderApiToken(value);

                if (!isValid) {
                    invalidationErrorStore.addError({
                        id: 'provider-voiceEnhancing',
                        message: `${tokenType || value} API token is required for ${value}`,
                        severity: 'critical'
                    });
                } else {
                    invalidationErrorStore.removeError('provider-voiceEnhancing');
                }
            }
        }
        
        // Handle group option changes
        if (isGroupOption && groupId) {
            logger.debug('FeatureSelector', 'Received group option change', { 
                groupId, 
                optionId, 
                value 
            });
            
            // Special handling for romanization style changes
            if (groupId === 'subtitle' && optionId === 'style' && romanizationSchemes.length > 0) {
                // Update the provider based on the selected style
                const selectedScheme = romanizationSchemes.find(s => s.name === value);
                if (selectedScheme) {
                    const providerValue = (selectedScheme.providers || []).join(' → ');
                    logger.info('FeatureSelector', 'Romanization style changed', { 
                        style: value, 
                        provider: providerValue 
                    });
                    
                    // First set the style
                    featureGroupStore.setGroupOption(groupId, optionId, value);
                    
                    // Use tick() to ensure the style update is processed before updating provider
                    // This prevents race conditions with reactive statements
                    tick().then(() => {
                        // Then set the provider after the style has been fully processed
                        featureGroupStore.setGroupOption(groupId, 'provider', providerValue);
                        
                        // Sync all values from the group store
                        currentFeatureOptions = featureGroupStore.syncOptionsToFeatures(
                            groupId, currentFeatureOptions
                        );
                        
                        // Dispatch changes
                        dispatch('optionsChange', currentFeatureOptions);
                    });
                    
                    return;
                }
            }
            
            // Update the group store - this is the central source of truth for group options
            featureGroupStore.setGroupOption(groupId, optionId, value);
            
            // Sync values from the group store to all features in the group
            currentFeatureOptions = featureGroupStore.syncOptionsToFeatures(
                groupId, currentFeatureOptions
            );

            // Dispatch changes to parent
            dispatch('optionsChange', currentFeatureOptions);
            return;
        }
        
        dispatch('optionsChange', currentFeatureOptions);
        
        // Special case for summary provider changes - fetch models for the new provider
        if (featureId === 'condensedAudio' && optionId === 'summaryProvider') {
            const newProvider = value;
            logger.info('FeatureSelector', 'Summary provider changed', { newProvider });
            
            // Fetch models for the selected provider if it's a non-empty string
            if (newProvider && typeof newProvider === 'string') {
                // Clear the current model selection
                currentFeatureOptions.condensedAudio.summaryModel = '';
                
                // Check if we already have models for this provider in our map
                if (providerModelsMap[newProvider] && providerModelsMap[newProvider].length > 0) {
                    logger.debug('FeatureSelector', 'Using cached models for provider', { 
                        provider: newProvider,
                        modelCount: providerModelsMap[newProvider].length,
                        firstThree: providerModelsMap[newProvider].slice(0, 3)
                    });
                    
                    // Update the choices using the proper function
                    updateFeatureChoices('condensedAudio', 'summaryModel', providerModelsMap[newProvider]);
                    
                    // Select a default model
                    if (providerModelsMap[newProvider].length > 0) {
                        currentFeatureOptions.condensedAudio.summaryModel = providerModelsMap[newProvider][0];
                        
                        // Notify parent of the model change
                        dispatch('optionChange', {
                            featureId: 'condensedAudio',
                            optionId: 'summaryModel',
                            value: currentFeatureOptions.condensedAudio.summaryModel
                        });
                    }
                    
                    // Force current feature options to update
                    currentFeatureOptions = {...currentFeatureOptions};
                } else {
                    // We don't have models for this provider yet - clear the list 
                    updateFeatureChoices('condensedAudio', 'summaryModel', []);
                    
                    // Force current feature options to update
                    currentFeatureOptions = {...currentFeatureOptions};
                }
                
                // Fetch models for this provider (will update the UI when done if it's still the current provider)
                fetchSummaryModels(newProvider);
            }
        }
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
            logger.error('FeatureSelector', 'Error evaluating feature condition', { 
                condition: featureDef.showCondition, 
                error 
            });
            return false;
        }
    }
    
    function ensureValidSTTModel() {
      if (!currentFeatureOptions.dubtitles) return;
      
      // Always use the first available model if the model list exists
      if (currentSTTModels && currentSTTModels.names && currentSTTModels.names.length > 0) {
        const firstModel = currentSTTModels.names[0];
        const currentModel = currentFeatureOptions.dubtitles.stt;
        
        // Update if current model doesn't exist in the list
        if (!currentSTTModels.names.includes(currentModel)) {
          logger.info('FeatureSelector', 'Resetting STT model to first available', { 
              previousModel: currentModel, 
              newModel: firstModel 
          });
          currentFeatureOptions.dubtitles.stt = firstModel;
          dispatch('optionChange', { featureId: 'dubtitles', optionId: 'stt', value: firstModel });
        }
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
        selectedFeatures,
        sttModels: currentSTTModels.models || []
    };
    
    // Reactive statements
    $: anyFeatureSelected = Object.values(selectedFeatures).some(v => v);
    
    // React to ALL changes to quickAccessLangTag (whether from user input or settings)
    let lastProcessedLangTag = '';
    $: if (quickAccessLangTag !== lastProcessedLangTag && isInitialDataLoaded) {
        logger.info('FeatureSelector', 'Language tag changed', { 
            previousTag: lastProcessedLangTag, 
            newTag: quickAccessLangTag 
        });
        
        // Process language change if it's different (case-insensitive)
        if (lastProcessedLangTag.toLowerCase() !== quickAccessLangTag.toLowerCase()) {
            if (debouncedProcessLanguageChange) {
                debouncedProcessLanguageChange(quickAccessLangTag);
            } else {
                // If debounced function not ready yet, call directly
                processLanguageChange(quickAccessLangTag);
            }
        }
        
        // Update the last processed value
        lastProcessedLangTag = quickAccessLangTag;
    }

    // Function to register the display order of features in the UI
    function registerFeatureDisplayOrder() {
        // Get all visible features in their current DOM order
        const featureElements = Array.from(document.querySelectorAll('[data-feature-id]'));
        
        if (featureElements.length === 0) {
            logger.trace('featureSelector', 'No feature elements found in the DOM yet');
            return;
        }
        
        const orderedFeatureIds = featureElements
            .map(el => el.getAttribute('data-feature-id'))
            .filter(Boolean);
        
        logger.trace('featureSelector', 'Current feature display order', { orderedFeatureIds });
        
        // Update the display order for each group
        Object.keys(featureGroupStore.getGroups()).forEach(groupId => {
            featureGroupStore.updateFeatureDisplayOrder(groupId, orderedFeatureIds);
        });
    }
    
    // Error management
    // Memoize feature selection state and language validation to reduce reactivity overhead
    let prevFeaturesSelected = false;
    let prevLanguageValidState = null;
    let prevLanguageTag = '';
    
    // Special initialization to clean up inconsistencies in feature options
    function cleanupFeatureOptions() {
        // Initialize all features with appropriate options
        reactiveFeatures.forEach(feature => {
            if (!currentFeatureOptions[feature.id]) {
                currentFeatureOptions[feature.id] = {};
            }
            
            // Ensure appropriate initialization for each feature
            if (feature.featureGroups?.includes('merge')) {
                // Ensure merge options are properly initialized for this group
                const groupOptions = featureGroupStore.getGroupOptions('merge');
                if (groupOptions) {
                    // For each shared option in the merge group
                    if (feature.groupSharedOptions?.['merge']) {
                        feature.groupSharedOptions['merge'].forEach(optionId => {
                            // Initialize from group store if available
                            if (groupOptions[optionId] !== undefined) {
                                currentFeatureOptions[feature.id][optionId] = groupOptions[optionId];
                            }
                        });
                    }
                }
            }
        });
    }
    
    // Component lifecycle
    // Initialize feature groups
    function initializeFeatureGroups() {
        logger.info('FeatureSelector', 'Initializing feature groups', { 
            currentLanguage: standardTag 
        });
        
        // Initialize the canonical order once during setup
        const canonicalOrder = reactiveFeatures.map(f => f.id);
        featureGroupStore.initializeCanonicalOrder(canonicalOrder);
        
        // First, handle existing features to ensure they're visible
        // This ensures all feature cards are created correctly first
        for (let feature of reactiveFeatures) {
            // Include selective transliteration regardless of its label (which changes based on language)
            const isSubtitleFeature = feature.id === 'subtitleRomanization' || 
                                      feature.id === 'selectiveTransliteration' || 
                                      feature.id === 'subtitleTokenization';
                                      
            if (isSubtitleFeature) {
                logger.debug('FeatureSelector', 'Adding feature to subtitle group', { 
                    featureId: feature.id 
                });
                
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
                
                // Register each option with its group - this is crucial for the new approach
                ['style', 'provider', 'dockerRecreate', 'browserAccessURL'].forEach(optionId => {
                    featureGroupStore.registerOptionToGroup('subtitle', optionId);
                });
                
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
                            currentFeatureOptions[feature.id][optionId] = (romanizationSchemes[0].providers || []).join(' → ');
                        } else {
                            currentFeatureOptions[feature.id][optionId] = '';
                        }
                    }
                });
            }
            
            // Handle merge features
            const isMergeFeature = feature.outputMergeGroup === 'merge';
            if (isMergeFeature) {
                logger.debug('FeatureSelector', 'Adding feature to merge group', { 
                    featureId: feature.id 
                });
                
                // Mark for group membership
                if (!feature.featureGroups) {
                    feature.featureGroups = ['merge'];
                } else if (!feature.featureGroups.includes('merge')) {
                    feature.featureGroups.push('merge');
                }
                
                // Make sure groupSharedOptions are defined
                if (!feature.groupSharedOptions) {
                    feature.groupSharedOptions = {
                        'merge': ['mergeOutputFiles', 'mergingFormat']
                    };
                } else if (!feature.groupSharedOptions['merge']) {
                    feature.groupSharedOptions['merge'] = ['mergeOutputFiles', 'mergingFormat'];
                }
                
                // Register each feature in the group store individually
                featureGroupStore.addFeatureToGroup('merge', feature.id);
                
                // Register merge options with the merge group - crucial for proper handling
                featureGroupStore.registerOptionToGroup('merge', 'mergeOutputFiles');
                featureGroupStore.registerOptionToGroup('merge', 'mergingFormat');
                
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
            validationRules: [/*
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
                }*/
            ]
        };
        
        // Define the merge output group
        const mergeGroup: FeatureGroup = {
            id: 'merge',
            label: 'Output Merging',
            description: 'Features that can be merged into output',
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
        
        // Sync happens later during initialization - no need to do it here
        
        // Update subtitle features with shared options
        const subtitleFeatures = reactiveFeatures.filter(f => 
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
        
        // Initialize merge group options with defaults
        featureGroupStore.setGroupOption('merge', 'mergeOutputFiles', false);
        featureGroupStore.setGroupOption('merge', 'mergingFormat', 'mp4');
        
        // Sync the group options to features in the merge group
        currentFeatureOptions = featureGroupStore.syncOptionsToFeatures('merge', currentFeatureOptions);
    }

    // Update display order when features are fully rendered
    afterUpdate(() => {
        if (isInitialDataLoaded && visibleFeatures.length > 0) {
            // Use tick to ensure the DOM is fully updated
            tick().then(() => {
                registerFeatureDisplayOrder();
            });
        }
    });
    
    // Subscribe to error store changes to verify updates are being applied
    let invalidationErrorStoreUnsubscribe: () => void;
    
    onMount(async () => {
        // Initialize debounced functions
        debouncedProcessLanguageChange = debounce(processLanguageChange, getMediumDebounce());
        debouncedUpdateProviderWarnings = debounce(updateProviderWarnings, getSmallDebounce());
        debouncedUpdateFeatureAvailabilityErrors = debounce(updateFeatureAvailabilityErrors, getSmallDebounce());
        
        // Initialize debounced settings update handler
        debouncedSettingsUpdate = debounce(() => {
            ensureValidSTTModel();
            debouncedUpdateProviderWarnings();
        }, getSmallDebounce());
        
        // Subscribe to STT models store
        sttModelsUnsubscribe = sttModelsStore.subscribe(value => {
            currentSTTModels = value;
        });
        
        // Subscribe to summary providers and models stores
        summaryProvidersUnsubscribe = summaryProvidersStore.subscribe(value => {
            currentSummaryProviders = value;
        });
        
        summaryModelsUnsubscribe = summaryModelsStore.subscribe(value => {
            currentSummaryModels = value;
        });
        
        // Subscribe to features store for reactivity
        featuresUnsubscribe = featuresStore.subscribe(value => {
            reactiveFeatures = value;
        });
        
        // Subscribe to LLM state changes
        llmStateUnsubscribe = llmStateStore.subscribe(state => {
            logger.trace('featureSelector', 'LLM state updated:', state?.globalState);
            llmState = state;
            
            // Update initialization flag (include 'updating' state)
            isLLMInitializing = !state || state.globalState === 'initializing' || state.globalState === 'uninitialized' || state.globalState === 'updating';
            
            // When LLM system becomes ready, fetch providers and prefetch all models
            if (state?.globalState === 'ready' && !currentSummaryProviders.available) {
                logger.debug('featureSelector', 'LLM system ready, fetching providers');
                fetchSummaryProviders().then(async (providers) => {
                    logger.debug('featureSelector', `Providers fetched: ${providers.names.length} available`);
                    
                    // Prefetch models for ALL providers to avoid delays when user selects
                    if (providers.available && providers.names.length > 0) {
                        logger.debug('featureSelector', 'Prefetching models for all providers...');
                        
                        // Fetch models for all providers in parallel
                        const modelFetchPromises = providers.names.map(providerName => {
                            logger.debug('featureSelector', `Prefetching models for provider: ${providerName}`);
                            return fetchSummaryModels(providerName).catch(error => {
                                logger.error('featureSelector', `Failed to prefetch models for ${providerName}:`, error);
                                return null;
                            });
                        });
                        
                        await Promise.all(modelFetchPromises);
                        logger.debug('featureSelector', 'All provider models prefetched');
                    }
                    
                    // Set default provider selection to the suggested one (now prioritizes openrouter-free)
                    if (providers.suggested && currentFeatureOptions?.condensedAudio) {
                        currentFeatureOptions.condensedAudio.summaryProvider = providers.suggested;
                        logger.debug('featureSelector', `Set default provider to: ${providers.suggested}`);
                        
                        // Also set the default model for the suggested provider if available
                        if (providerModelsMap[providers.suggested] && providerModelsMap[providers.suggested].length > 0) {
                            currentFeatureOptions.condensedAudio.summaryModel = providerModelsMap[providers.suggested][0];
                            logger.debug('featureSelector', `Set default model to: ${currentFeatureOptions.condensedAudio.summaryModel}`);
                            
                            // Update the model dropdown choices
                            updateFeatureChoices('condensedAudio', 'summaryModel', providerModelsMap[providers.suggested] || []);
                        }
                    }
                    
                    // Force reactivity update to refresh dropdowns
                    currentFeatureOptions = {...currentFeatureOptions};
                    
                    // Additional force update for the features array itself
                    const condensedAudioFeature = reactiveFeatures.find(f => f.id === 'condensedAudio');
                    if (condensedAudioFeature) {
                        logger.debug('FeatureSelector', 'Current provider choices', { 
                            choices: condensedAudioFeature.options.summaryProvider.choices 
                        });
                    }
                });
            }
        });
        
        // Connect to WebSocket for real-time updates
        logger.info('FeatureSelector', 'Connecting to WebSocket');
        
        // Register handler for LLM state changes
        llmStateChangedHandler = (data) => {
            logger.debug('FeatureSelector', 'LLM state change received', { 
                globalState: data.globalState 
            });
            llmStateStore.set(data);
        };
        wsClient.on('llm.state.changed', llmStateChangedHandler);
        
        // Also handle initial state if server sends it on connection
        connectedHandler = async () => {
            logger.info('FeatureSelector', 'WebSocket connected, requesting initial LLM state');
            // The server will send the current LLM state automatically
        };
        wsClient.on('connected', connectedHandler);
        
        logger.info('FeatureSelector', 'Component mounting - loading data', { 
            llmState: llmState?.globalState 
        });

        try {
            // Load STT models BEFORE any animation starts
            try {
                const sttModels = await GetAvailableSTTModelsForUI();
                
                // Update feature models and store
                updateSTTModels(sttModels);
                
                if (!sttModels.available) {
                    // Handle case where no models are available
                    invalidationErrorStore.addError({
                        id: 'no-stt-models',
                        message: 'No speech-to-text models available. Check API keys in settings.',
                        severity: 'warning',
                        action: {
                            label: 'Open Settings',
                            handler: () => {
                                // Correct way to update a Svelte store
                                showSettings.set(true);
                            }
                        }
                    });
                }
                
                // If dubtitles has a current model selected that's not available, update it
                if (currentFeatureOptions?.dubtitles?.stt && 
                    !sttModels.names.includes(currentFeatureOptions.dubtitles.stt)) {
                    currentFeatureOptions.dubtitles.stt = sttModels.suggested;
                }
                
                // Don't fetch summary providers here - wait for LLM state to be ready
                // The LLM state subscription will trigger fetchSummaryProviders when ready
            } catch (error) {
                logger.error('FeatureSelector', 'Failed to load STT models', { error });
            }
            
            // Only load summary providers if LLM is ready, otherwise wait for state subscription
            if (llmState?.globalState === 'ready') {
                try {
                    const summaryProviders = await fetchSummaryProviders();
                    
                    if (!summaryProviders.available) {
                        // Handle case where no providers are available
                        invalidationErrorStore.addError({
                            id: 'no-summary-providers',
                            message: 'No summary providers available. Check API keys in settings.',
                            severity: 'warning',
                            action: {
                                label: 'Open Settings',
                                handler: () => {
                                    showSettings.set(true);
                                }
                            }
                        });
                    }
                    
                    // If condensedAudio has a provider selected that's not available, update it
                    if (currentFeatureOptions?.condensedAudio?.summaryProvider && 
                        !summaryProviders.names.includes(currentFeatureOptions.condensedAudio.summaryProvider)) {
                        currentFeatureOptions.condensedAudio.summaryProvider = summaryProviders.suggested;
                        
                        // Also fetch models for the suggested provider
                        if (summaryProviders.suggested) {
                            await fetchSummaryModels(summaryProviders.suggested);
                        }
                    }
                } catch (error) {
                    logger.error('FeatureSelector', 'Failed to load summary providers', { error });
                }
            } else {
                logger.debug('FeatureSelector', 'LLM not ready, skipping summary provider fetch');
            }
            
            // Initialize canonical feature order from feature definitions
            const canonicalOrder = reactiveFeatures.map(f => f.id);
            featureGroupStore.initializeCanonicalOrder(canonicalOrder);
            logger.debug('FeatureSelector', 'Initialized canonical feature order', { canonicalOrder });
            
            // Initialize feature groups 
            initializeFeatureGroups();
            
            // Load all necessary data before showing the component
            // This prevents visual glitches during initialization
            const currentSettings = get(settings);
            
            if (currentSettings?.targetLanguage) {
                quickAccessLangTag = currentSettings.targetLanguage;
                
                // Process language change instead of direct validation
                await processLanguageChange(currentSettings.targetLanguage);
            }
            
            // Prepare component by loading config and options
            debouncedUpdateProviderWarnings();
            cleanupFeatureOptions(); // Clean up feature options on mount
            
            // Make sure we're fully loaded before starting animations
            await new Promise(resolve => setTimeout(resolve, 50));
            
            // Mark component as ready BEFORE starting animations
            isInitialDataLoaded = true;
            logger.info('FeatureSelector', 'Initial data loaded successfully');
            
            // Check if welcome popup is visible
            const isWelcomePopupVisible = get(welcomePopupVisible);
            
            if (isWelcomePopupVisible) {
                logger.info('FeatureSelector', 'Welcome popup is visible, keeping features hidden');
                
                // Keep features completely hidden while welcome popup is visible
                // This prevents them from showing through the semi-transparent popup
                visibleFeatures = [];
                
                // Subscribe to welcome popup state and start animation when it closes
                const unsubscribe = welcomePopupVisible.subscribe(isVisible => {
                    if (!isVisible) {
                        logger.info('FeatureSelector', 'Welcome popup closed, preparing feature reveal');
                        unsubscribe();
                        
                        // Add delay to ensure welcome popup closing animation completes
                        // On low-spec machines, the store updates faster than the GUI can render,
                        // so we need to wait for the fade-out animation (200ms) plus some buffer
                        setTimeout(() => {
                            logger.info('FeatureSelector', 'Starting feature reveal animation after delay');
                            startFeatureRevealAnimation();
                        }, 400); // 200ms fade-out + 200ms buffer for low-spec machines
                    }
                });
            } else {
                // Welcome popup not visible, proceed with animation immediately
                startFeatureRevealAnimation();
            }
            
            // Do an initial provider warning check after everything is set up
            setTimeout(() => {
                debouncedUpdateProviderWarnings();
                logger.debug('FeatureSelector', 'Initial provider warnings check completed');
            }, 500);
            
        } catch (error) {
            logger.error('FeatureSelector', 'Error during initialization', { error });
            // Mark as loaded anyway to prevent endless loading state
            isInitialDataLoaded = true;
            
            // In case of error, check if welcome popup is visible before showing features
            const isWelcomePopupVisible = get(welcomePopupVisible);
            if (!isWelcomePopupVisible) {
                // Only show features if welcome popup is not visible
                visibleFeatures = Object.keys(selectedFeatures);
            } else {
                // Keep features hidden and wait for welcome popup to close
                visibleFeatures = [];
                const unsubscribe = welcomePopupVisible.subscribe(isVisible => {
                    if (!isVisible) {
                        unsubscribe();
                        setTimeout(() => {
                            visibleFeatures = Object.keys(selectedFeatures);
                        }, 400);
                    }
                });
            }
            
            // Log the error to help with debugging
            logStore.addLog({
                level: 'ERROR',
                message: `Error initializing feature selector: ${error.message}`,
                time: new Date().toISOString()
            });
        }
    });

    onDestroy(() => {
        logger.info('FeatureSelector', 'Component unmounting, cleaning up');
       
        // Clean up store subscriptions
        if (sttModelsUnsubscribe) {
            sttModelsUnsubscribe();
        }
        
        if (summaryProvidersUnsubscribe) {
            summaryProvidersUnsubscribe();
        }
        
        if (summaryModelsUnsubscribe) {
            summaryModelsUnsubscribe();
        }
        
        if (featuresUnsubscribe) {
            featuresUnsubscribe();
        }
        
        // Clean up error store subscription
        if (invalidationErrorStoreUnsubscribe) {
            invalidationErrorStoreUnsubscribe();
        }
        
        // Clean up LLM state subscription
        if (llmStateUnsubscribe) {
            llmStateUnsubscribe();
        }
        
        // Note: WebSocket disconnection is handled globally in App.svelte
        // We just need to remove our event handlers
        if (llmStateChangedHandler) {
            wsClient.off('llm.state.changed', llmStateChangedHandler);
        }
        if (connectedHandler) {
            wsClient.off('connected', connectedHandler);
        }
        
        // Clean up any error messages we created
        invalidationErrorStore.removeError('no-stt-models');
        invalidationErrorStore.removeError('no-summary-providers');
        invalidationErrorStore.removeError('no-summary-models');
        
        // Clear legacy errors
        invalidationErrorStore.removeError('docker-required');
        invalidationErrorStore.removeError('invalid-browser-url');
        invalidationErrorStore.removeError('no-features');
        invalidationErrorStore.removeError('invalid-language');
        invalidationErrorStore.removeError('provider-dubtitles');
        invalidationErrorStore.removeError('provider-voiceEnhancing');
        invalidationErrorStore.removeError('no-romanization');
        invalidationErrorStore.removeError('no-selective-transliteration');
        
        // Clear feature group errors - be thorough with all possible error IDs
        featureGroupStore.clearGroupErrors('subtitle');
        invalidationErrorStore.removeError('group-subtitle-browser-url');
        invalidationErrorStore.removeError('group-subtitle-browser-url-validation');
    });
    
    function softLanding(t) {
       return 1 - Math.pow(1 - t, 3.5);
    }
    
    /**
     * Start the staggered feature reveal animation
     */
    function startFeatureRevealAnimation() {
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
                    logger.trace('FeatureSelector', 'Revealing feature', { 
                        feature, 
                        delayMs: Math.round(delay) 
                    });
                    visibleFeatures = [...visibleFeatures, feature];
                }, delay);
            });
            
            // Register display order after all animations complete
            const maxDelay = 100 * Math.pow(1.75, orderedFeatures.length / 1.2) + 200;
            setTimeout(registerFeatureDisplayOrder, maxDelay);
        }
    }

    // Call this method whenever the STT models change
    $: if (currentSTTModels) {
      ensureValidSTTModel();
    }

    // Declare debounced settings update handler - will be initialized in onMount
    let debouncedSettingsUpdate: () => void;
    
    // Also call it when settings change
    settings.subscribe(value => {
      if (value && debouncedSettingsUpdate) {
        // Use debounced update for WebView2 compatibility
        debouncedSettingsUpdate();
      }
    });
    
    // React to programmatic changes to quickAccessLangTag (e.g., from settings)
    let previousQuickAccessLangTag = quickAccessLangTag;
    $: {
        if (quickAccessLangTag !== previousQuickAccessLangTag && isInitialDataLoaded) {
            logger.info('FeatureSelector', 'Language tag changed programmatically', { 
                previousTag: previousQuickAccessLangTag, 
                newTag: quickAccessLangTag 
            });
            
            // Process the language change
            if (debouncedProcessLanguageChange) {
                debouncedProcessLanguageChange(quickAccessLangTag);
            } else {
                // If debounced function not ready yet, call directly
                processLanguageChange(quickAccessLangTag);
            }
            
            // Update the previous value
            previousQuickAccessLangTag = quickAccessLangTag;
        }
    }
    
    // update provider warnings when STT model changes
    $: if (currentFeatureOptions?.dubtitles?.stt) {
        // This ensures we update warnings whenever the STT model changes
        logger.debug('FeatureSelector', 'STT model changed reactively', {
            model: currentFeatureOptions.dubtitles.stt
        });
        debouncedUpdateProviderWarnings();
    }

    $: if (currentSTTModels && currentSTTModels.models && currentSTTModels.models.length > 0) {
        logger.debug('FeatureSelector', 'STT models updated, checking default model selection');
        
        // Make sure dubtitles feature options exist
        if (!currentFeatureOptions.dubtitles) {
            currentFeatureOptions.dubtitles = {};
        }
        
        // If no model is selected or the selected model isn't in the list, use the first one
        const currentModel = currentFeatureOptions.dubtitles.stt;
        if (!currentModel || !currentSTTModels.names.includes(currentModel)) {
            const firstModel = currentSTTModels.names[0];
            logger.info('FeatureSelector', 'Setting initial STT model', { 
                model: firstModel 
            });
            
            // Update the feature options directly
            currentFeatureOptions.dubtitles.stt = firstModel;
            
            // And dispatch the change
            dispatch('optionsChange', currentFeatureOptions);
        }
    }
</script>

<div class="space-y-6">
    <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center pl-0 pr-0">
        <!-- Title that will shrink as needed -->
        <h2 class="text-xl font-medium text-tertiary flex items-center pl-4 gap-2 overflow-hidden min-w-0">
            <span class="material-icons text-tertiary flex-shrink-0">tune</span>
            <span class="truncate">Select Features</span>
        </h2>
        
        <!-- Language selector component - won't shrink -->
        <div class="pr-3">
            <QuickAccessLangSelector 
                bind:languageTag={quickAccessLangTag}
                {isValidLanguage}
                {isChecking}
                {validationError}
                {showAudioTrackIndex}
                {audioTrackIndex}
                on:audioTrackChange={handleAudioTrackChange}
            />
        </div>
    </div>
    
    <!-- Feature cards container - only rendered after data is fully loaded -->
    <div class="space-y-4 overflow-visible feature-cards-container">
        {#if isInitialDataLoaded}
            {#each reactiveFeatures.filter(f => visibleFeatures.includes(f.id) && (!f.showCondition || shouldShowFeature(f))) as feature, i (feature.id)}
                <div 
                    in:fly={{ 
                        x: 400,
                        duration: Math.min(550, 550 - (i * 15)),
                        easing: softLanding,
                        opacity: 0
                    }}
                    style="will-change: transform, opacity; overflow: visible;"
                    class="px-2 my-2"
                >
                    <div data-feature-id={feature.id} class="overflow-visible px-2">
                        <FeatureCard
                            {feature}
                            enabled={selectedFeatures[feature.id]}
                            options={currentFeatureOptions[feature.id]}
                            {anyFeatureSelected}
                            {romanizationSchemes}
                            {isRomanizationAvailable}
                            {tokenizationAllowed}
                            {dockerUnreachable}
                            {dockerEngine}
                            {standardTag}
                            {providerGithubUrls}
                            {selectedFeatures}
                            {providerGroups}
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
    @import './featureStyles.css';
    
    /* Add smooth animation for cards when scrolling */
    div {
        will-change: transform;
        transform: translateZ(0);
    }
</style>