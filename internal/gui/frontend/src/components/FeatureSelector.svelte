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
        CheckMediaLanguageTags 
    } from '../../wailsjs/go/gui/App';
    
    import FeatureCard from './FeatureCard.svelte';
    import QuickAccessLangSelector from './QuickAccessLangSelector.svelte';

    // Props
    export let selectedFeatures = {
        subs2cards: false,
        dubtitles: false,
        voiceEnhancing: false,
        subtitleRomanization: false,
        selectiveTransliteration: false
    };
    export let quickAccessLangTag = '';
    export let showLogViewer: boolean;
    export let mediaSource: MediaSource | null = null;

    // State variables
    let visibleFeatures: string[] = [];
    let currentFeatureOptions = createDefaultOptions();
    
    let isValidLanguage: boolean | null = null;
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
        
        // If a feature was enabled, scroll it into view after a small delay
        // to allow UI to update and expand the feature card options
        if (enabled) {
            // First timeout to let the feature expand
            setTimeout(() => {
                const featureCard = document.querySelector(`[data-feature-id="${id}"]`);
                if (featureCard) {
                    // Get the scroll container (mask-fade element)
                    const scrollContainer = featureCard.closest('.mask-fade');
                    if (!scrollContainer) return;
                    
                    // Get the position of the feature card within the scroll container
                    const containerRect = scrollContainer.getBoundingClientRect();
                    const featureRect = featureCard.getBoundingClientRect();
                    
                    // Calculate the vertical padding needed to avoid mask-fade edges
                    const verticalPadding = containerRect.height * 0.15; // 15% of container height
                    
                    // Calculate target scroll position to center the feature
                    let targetScrollTop = scrollContainer.scrollTop + 
                                        (featureRect.top - containerRect.top) - 
                                        (containerRect.height / 2) + 
                                        (featureRect.height / 2);
                    
                    // Ensure the feature won't be hidden by top or bottom mask-fade
                    const minScrollTop = featureRect.top - containerRect.top - verticalPadding;
                    const maxScrollTop = featureRect.bottom - containerRect.bottom + verticalPadding;
                    
                    // Apply smoothly with animation
                    scrollContainer.scrollTo({
                        top: targetScrollTop,
                        behavior: 'smooth'
                    });
                }
            }, 250);
        }
    }
    
    function handleOptionChange(event: CustomEvent) {
        const { featureId, optionId, value } = event.detail;
        currentFeatureOptions[featureId][optionId] = value;
        updateProviderWarnings();
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

    // Reactive statements
    $: anyFeatureSelected = Object.values(selectedFeatures).some(v => v);

    $: if (quickAccessLangTag !== undefined) {
        needsDocker = false;
        needsScraper = false;
        validateLanguageTag(quickAccessLangTag, true);
        
        if (!quickAccessLangTag) {
            isRomanizationAvailable = false;
            selectedFeatures.subtitleRomanization = false;
        }
    }
    
    // Error management
    $: {
        // Feature selection errors
        if (!Object.values(selectedFeatures).some(v => v)) {
            errorStore.addError({
                id: 'no-features',
                message: 'Select at least one processing feature',
                severity: 'critical'
            });
        } else {
            errorStore.removeError('no-features');
        }

        // Language validation errors
        if (!isValidLanguage && quickAccessLangTag) {
            errorStore.addError({
                id: 'invalid-language',
                message: validationError || 'Invalid language code',
                severity: 'critical'
            });
        } else {
            errorStore.removeError('invalid-language');
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
    $: if (selectedFeatures) {
        updateProviderWarnings();
    }

    // Component lifecycle
    onMount(async () => {
        const currentSettings = get(settings);
        if (currentSettings?.targetLanguage) {
            quickAccessLangTag = currentSettings.targetLanguage;
            await validateLanguageTag(currentSettings.targetLanguage, true);
        }
        updateProviderWarnings();
        
        let initialWait = 0;
        if (showLogViewer) {
            initialWait = 2500;
        }
        
        // Staggered animation for features
        const allFeatures = Object.keys(selectedFeatures);
        allFeatures.forEach((feature, index) => {
            const baseDelay = 50; // Base delay in ms
            const exponentialFactor = 2.2; // Exponential factor
            const delay = baseDelay * Math.pow(exponentialFactor, index / 1.2);
            setTimeout(() => {
                visibleFeatures = [...visibleFeatures, feature];
            }, delay + initialWait);
        });
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
        {#each features.filter(f => visibleFeatures.includes(f.id)) as feature, i (feature.id)}
            <div in:fly={{ 
                x: 300, 
                duration: 400 - (i * 20),
                easing: cubicOut,
                opacity: 0
            }}>
                <div data-feature-id={feature.id}>
                    <FeatureCard
                        {feature}
                        enabled={selectedFeatures[feature.id]}
                        options={currentFeatureOptions[feature.id]}
                        {anyFeatureSelected}
                        {romanizationSchemes}
                        {isRomanizationAvailable}
                        {needsDocker}
                        {dockerUnreachable}
                        {dockerEngine}
                        {needsScraper}
                        {standardTag}
                        {providerGithubUrls}
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