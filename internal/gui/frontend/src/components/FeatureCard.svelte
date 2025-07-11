<script lang="ts">
    import { get } from 'svelte/store';
    import { createEventDispatcher, onMount, onDestroy, tick, afterUpdate } from 'svelte';
    import { fade } from 'svelte/transition';
    
    import { formatDisplayText, sttModelsStore, type FeatureDefinition } from '../lib/featureModel';
    import { invalidationErrorStore } from '../lib/invalidationErrorStore';
    import { showSettings, llmStateStore, settings, type LLMStateChange, dockerStatusStore } from '../lib/stores';
    import { featureGroupStore } from '../lib/featureGroupStore';
    import { logger } from '../lib/logger';
    import { ValidateLanguageTag } from '../../wailsjs/go/gui/App';
    import { debounce } from 'lodash';
    import { getOSDebounceDelay } from '../lib/osUtils';
    
    // Class for message items to keep styling consistent
    const messageItemClass = "flex items-center gap-2 py-2 px-3 first:pt-2 last:pb-2 hover:bg-white/5 transition-colors duration-200 group";
    
    import Dropdown from './Dropdown.svelte';
    import Hovertip from './Hovertip.svelte';
    import NumericInput from './NumericInput.svelte';
    import TextInput from './TextInput.svelte';
    import Slider from './Slider.svelte';
    import GroupIcon from './icons/GroupIcon.svelte';
    import DockerIcon from './icons/DockerIcon.svelte';
    import DockerUnavailableIcon from './icons/DockerUnavailableIcon.svelte';
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
    let groupStoreUnsubscribe: () => void;
    
    // References to animated border elements
    let animatedBorderRight: HTMLElement;
    let animatedBorderBottom: HTMLElement;
    
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
        //logger.trace('featureCard', `Feature ${feature.id} topmost status check: ${isTopmostFeatureForAnyGroup}`);
    }

    // Create a local variable to track store changes
    let currentSTTModels = { models: [], names: [], available: false, suggested: "" };
    let sttModelsUnsubscribe: () => void;
    
    // LLM state tracking for summary options
    let llmState: LLMStateChange | null = null;
    let llmStateUnsubscribe: () => void;
    
    // Docker status tracking for debug override
    let dockerStatus: { available?: boolean; error?: string } | null = null;
    let dockerStatusUnsubscribe: () => void;
    
    // Debug override state
    let debugLLMState: string | null = null;
    
    // Check for debug override in LLM state message
    $: {
        if (!llmState) {
            // If state is null/undefined, no debug override
            debugLLMState = null;
        } else if (llmState.message?.startsWith('Debug: Forced')) {
            debugLLMState = llmState.globalState;
        } else {
            debugLLMState = null;
        }
    }
    
    // Native language check
    let isNativeLanguageEnglish = false;
    let settingsUnsubscribe: () => void;
    
    // Function to check if native language is English
    async function checkNativeLanguageIsEnglish() {
        const currentSettings = get(settings);
        if (!currentSettings?.nativeLanguages) {
            isNativeLanguageEnglish = false;
            return;
        }
        
        try {
            // Extract the first language from the comma-separated list
            const languages = currentSettings.nativeLanguages.split(',').map(lang => lang.trim());
            const firstLanguage = languages[0];
            
            if (!firstLanguage) {
                isNativeLanguageEnglish = false;
                return;
            }
            
            // Validate only the first language
            const response = await ValidateLanguageTag(firstLanguage, true);
            isNativeLanguageEnglish = response.isValid && response.standardTag === 'eng';
            
            logger.trace('featureCard', 'Native language English check', {
                nativeLanguages: currentSettings.nativeLanguages,
                firstLanguage: firstLanguage,
                standardTag: response.standardTag,
                isEnglish: isNativeLanguageEnglish
            });
        } catch (error) {
            logger.error('featureCard', 'Error checking native language', { error });
            isNativeLanguageEnglish = false;
        }
    }
    
    // Debounced version to prevent rapid validation calls
    const debouncedCheckNativeLanguageIsEnglish = debounce(checkNativeLanguageIsEnglish, getOSDebounceDelay());
    
    // Reactive computations for LLM state (respecting debug override)
    $: isLLMReady = debugLLMState ? debugLLMState === 'ready' : llmState?.globalState === 'ready';
    $: isLLMInitializing = debugLLMState 
        ? (debugLLMState === 'initializing' || debugLLMState === 'uninitialized')
        : (llmState?.globalState === 'initializing' || llmState?.globalState === 'uninitialized' || llmState?.globalState === 'updating');
    $: isLLMError = debugLLMState ? debugLLMState === 'error' : llmState?.globalState === 'error';
    $: llmErrorMessage = isLLMError ? (llmState?.message || 'LLM system error') : null;
    
    // Reactive computation for Docker state (respecting debug override)
    $: isDockerStatusForced = dockerStatus?.error === 'Debug: Forced state';
    $: isDockerUnavailable = dockerStatus?.checked ? !dockerStatus?.available : dockerUnreachable;
    
    // Reactive computation for missing LLM providers
    $: missingProviders = (() => {
        if (feature.id !== 'condensedAudio' || !isLLMReady) return [];
        
        // Get available providers from the summaryProvider option
        const availableProviders = feature.options.summaryProvider.choices || [];
        
        // Map display names to expected provider keys
        const providerKeyMap: Record<string, string> = {
            'OpenAI': 'openai',
            'Google': 'google',
            'OpenRouter': 'openrouter',
            'OpenRouter Free': 'openrouter-free'
        };
        
        const availableKeys = availableProviders.map(p => providerKeyMap[p] || p.toLowerCase());
        
        // Check which expected providers are missing
        const missing = [];
        if (!availableKeys.includes('openai')) missing.push('OpenAI');
        if (!availableKeys.includes('google')) missing.push('Google');
        // OpenRouter key covers both openrouter and openrouter-free
        if (!availableKeys.includes('openrouter') && !availableKeys.includes('openrouter-free')) missing.push('OpenRouter');
        
        return missing;
    })();
    
    onMount(() => {
        sttModelsUnsubscribe = sttModelsStore.subscribe(value => {
            currentSTTModels = value;
        });
        
        // Subscribe to LLM state for summary options
        dockerStatusUnsubscribe = dockerStatusStore.subscribe(value => {
            dockerStatus = value;
        });
        
        llmStateUnsubscribe = llmStateStore.subscribe(state => {
            llmState = state;
            logger.trace('featureCard', `LLM state update in FeatureCard ${feature.id}:`, state?.globalState);
            
            // Add error to error store when LLM fails (only for condensedAudio feature)
            if (feature.id === 'condensedAudio' && state?.globalState === 'error') {
                invalidationErrorStore.addError({
                    id: 'llm-initialization-failed',
                    message: state.message || 'LLM system failed to initialize. Check your API keys in settings.',
                    severity: 'error',
                    action: {
                        label: 'Open Settings',
                        handler: () => {
                            showSettings.set(true);
                        }
                    }
                });
            } else if (feature.id === 'condensedAudio' && state?.globalState === 'ready') {
                // Remove error when LLM becomes ready
                invalidationErrorStore.removeError('llm-initialization-failed');
            }
        });
        
        // Subscribe to group store changes to re-evaluate topmost status
        groupStoreUnsubscribe = featureGroupStore.subscribe((state) => {
            if (feature.featureGroups?.length) {
                if (enabled) {
                    // Check if the topmost status actually changed for this feature
                    const wasTopmost = isTopmostFeatureForAnyGroup;
                    checkTopmostFeatureStatus();
                    
                    // If topmost status changed, invalidate the option visibility cache
                    if (wasTopmost !== isTopmostFeatureForAnyGroup) {
                        logger.trace('featureCard', `Topmost status changed for ${feature.id}: ${wasTopmost} -> ${isTopmostFeatureForAnyGroup}`);
                        visibleOptionsDirty = true;
                    }
                } else if (isTopmostFeatureForAnyGroup) {
                    // If this feature is disabled but was showing group options, invalidate cache
                    logger.trace('featureCard', `Feature ${feature.id} disabled but was topmost, invalidating cache`);
                    visibleOptionsDirty = true;
                }
            }
        });
        
        // Subscribe to settings for native language check
        settingsUnsubscribe = settings.subscribe(() => {
            debouncedCheckNativeLanguageIsEnglish();
        });
        
        // Initial check
        checkNativeLanguageIsEnglish();
        
        // Initial measurement of the options height if enabled
        if (enabled && optionsWrapper) {
            optionsHeight = optionsWrapper.offsetHeight;
        }
    });
    
    onDestroy(() => {
        if (sttModelsUnsubscribe) {
            sttModelsUnsubscribe();
        }
        if (llmStateUnsubscribe) {
            llmStateUnsubscribe();
        }
        if (dockerStatusUnsubscribe) {
            dockerStatusUnsubscribe();
        }
        if (settingsUnsubscribe) {
            settingsUnsubscribe();
        }
        if (groupStoreUnsubscribe) {
            groupStoreUnsubscribe();
        }
        
        // Clean up any LLM errors we may have created
        if (feature.id === 'condensedAudio') {
            invalidationErrorStore.removeError('llm-initialization-failed');
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
            if (optionsWrapper) {
                optionsHeight = optionsWrapper.offsetHeight;
            }
        }, 50);
    }
    
    // Helper function for text color classes
    function getTextColorClass(enabled: boolean, anyFeatureSelected: boolean): string {
        if (enabled) return 'text-white';
        if (!anyFeatureSelected) return 'text-white';
        return 'text-white/70';
    }
    
    function formatProviderName(providerName: string): string {
        const providerFormatMap: Record<string, string> = {
            'openai': 'OpenAI',
            'replicate': 'Replicate',
            'elevenlabs': 'ElevenLabs'
        };
        
        // Return formatted name or original if not in mapping
        return providerFormatMap[providerName.toLowerCase()] || providerName;
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
    
    // Hover animation handlers
    function handleHoverStart() {
        if (isFeatureDisabled) return;
        
        // Remove any inline styles that might interfere with CSS animations
        if (animatedBorderRight && animatedBorderBottom) {
            animatedBorderRight.style.transition = '';
            animatedBorderBottom.style.transition = '';
            animatedBorderRight.style.opacity = '';
            animatedBorderBottom.style.opacity = '';
        }
    }
    
    function handleHoverEnd() {
        if (!animatedBorderRight || !animatedBorderBottom) return;
        
        // Stop the CSS animations first
        animatedBorderRight.style.animation = 'none';
        animatedBorderBottom.style.animation = 'none';
        
        // Set opacity to current value (should be 1 from animation)
        animatedBorderRight.style.opacity = '1';
        animatedBorderBottom.style.opacity = '1';
        
        // Force reflow to ensure animation is stopped and opacity is set
        void animatedBorderRight.offsetWidth;
        void animatedBorderBottom.offsetWidth;
        
        // Apply fade-out transition
        animatedBorderRight.style.transition = 'opacity 0.7s ease-out';
        animatedBorderBottom.style.transition = 'opacity 0.7s ease-out';
        
        // Trigger fade-out
        requestAnimationFrame(() => {
            animatedBorderRight.style.opacity = '0';
            animatedBorderBottom.style.opacity = '0';
        });
        
        // Clean up after transition
        setTimeout(() => {
            if (animatedBorderRight && animatedBorderBottom) {
                animatedBorderRight.style.animation = '';
                animatedBorderRight.style.transition = '';
                animatedBorderRight.style.opacity = '';
                animatedBorderBottom.style.animation = '';
                animatedBorderBottom.style.transition = '';
                animatedBorderBottom.style.opacity = '';
            }
        }, 700);
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
    const optionVisibilityCache: Map<string, boolean> = new Map();
    let lastContextHash = '';
    
    // Check if option should be shown based on conditions
    function shouldShowOption(optionId: string, optionDef: any): boolean {
        if (!optionDef.showCondition) return true;
        
        // Special handling for initialPrompt condition
        if (feature.id === 'dubtitles' && optionId === 'initialPrompt') {
            const sttModel = options.stt;
            const modelInfo = currentSTTModels.models.find(m => m.name === sttModel);
            return modelInfo?.takesInitialPrompt || false;
        }
        
        // Find which group this option belongs to (if any)
        let optionGroup = null;
        if (feature.groupSharedOptions) {
            for (const [groupId, options] of Object.entries(feature.groupSharedOptions)) {
                if (options.includes(optionId)) {
                    optionGroup = groupId;
                    
                    // Register this option with the group store if not already registered
                    // This ensures the store knows which option belongs to which group
                    featureGroupStore.registerOptionToGroup(groupId, optionId);
                    break;
                }
            }
        }
        
        // Use the feature group store's isTopmostForOption function for precise option-based checks
        let isTopmostForThisOption = false;
        if (enabled) {
            isTopmostForThisOption = featureGroupStore.isTopmostForOption(feature.id, optionId);
            logger.trace('featureCard', `Option ${optionId} isTopmostForOption check: ${isTopmostForThisOption}`);
        }
        
        // For backwards compatibility, maintain generic isTopmostInGroup checks
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
            isTopmostInGroup: isTopmostInAnyGroup,
            isTopmostForOption: isTopmostForThisOption,
            isLLMReady,
            isLLMInitializing,
            isLLMError,
            isNativeLanguageEnglish
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
            isTopmostInGroup: isTopmostInAnyGroup,
            isTopmostForOption: isTopmostForThisOption,
            featureGroupStore, // Add store to context
            isLLMReady, // Add LLM state for condition evaluation
            isLLMInitializing,
            isLLMError,
            isNativeLanguageEnglish
        };
        
        // Feature options reference for conditions 
        const featureData = {
            [feature.id]: options,
            id: feature.id // Include the feature id directly for easier checking
        };
        
        // Simple expression evaluator
        try {
            // Replace context variables and group store references with their values
            const prepared = optionDef.showCondition
                .replace(/context\.([a-zA-Z0-9_]+)/g, (_, prop) => {
                    // Handle featureGroupStore specifically if needed, otherwise stringify
                    if (prop === 'featureGroupStore') {
                        // This property won't be directly replaced here, handled below
                        return 'featureGroupStore';
                    }
                    return JSON.stringify(context[prop]);
                })
                 // Handle featureGroupStore.getGroupOption calls
                .replace(/featureGroupStore\.getGroupOption\(['"]([^'"]+)['"]\s*,\s*['"]([^'"]+)['"]\)/g,
                    (_, groupId, optId) => {
                        // Directly call the store method and stringify the result
                        return JSON.stringify(featureGroupStore.getGroupOption(groupId, optId));
                    })
                // Handle feature property access like feature.dubtitles.mergeOutputFiles
                .replace(/feature\.([a-zA-Z0-9_]+)\.([a-zA-Z0-9_]+)/g, (_, featureId, propId) => {
                    // Access the value from the featureData object
                    return JSON.stringify(featureData[featureId]?.[propId]);
                })
                 // Handle feature.id access
                .replace(/feature\.id/g, () => {
                    return JSON.stringify(feature.id);
                });
            
            // Use Function constructor to evaluate the expression
            const result = new Function('return ' + prepared)();
            
            // Debug logging for condensedAudio feature
            if (feature.id === 'condensedAudio' && optionId !== 'enableSummary') {
                logger.trace('featureCard', `Option ${optionId} condition: ${optionDef.showCondition}, result: ${result}, context.isLLMReady: ${context.isLLMReady}`);
            }
            
            // Cache the result
            optionVisibilityCache.set(cacheKey, result);
            return result;
        } catch (error) {
            logger.error('featureCard', 'Error evaluating condition', { condition: optionDef.showCondition, error });
            optionVisibilityCache.set(cacheKey, false);
            return false;
        }
    }
    
    function handleDropdownChange(optionId: string, value: string) {
        // Get current value before updating for proper change detection
        const oldValue = options[optionId];
        
        // Safety check to prevent duplicate processing
        if (oldValue === value) {
            logger.trace('featureCard', `Ignoring redundant update: ${optionId} remains ${value}`);
            return;
        }
        
        // Log the change for debugging
        logger.trace('featureCard', `FeatureCard handleDropdownChange: ${optionId} from ${oldValue} to ${value}`);
        
        // Update local state
        options[optionId] = value;
        
        // Special flag for STT model changes
        const isSTTModelChange = feature.id === 'dubtitles' && optionId === 'stt';
        
        // Dispatch update to parent
        dispatch('optionChange', { 
            featureId: feature.id, 
            optionId, 
            value,
            isSTTModelChange
        });
    }
    
    // Cached visible options
    let visibleOptionsCache: string[] = [];
    let visibleOptionsDirty = true;
    
    // Mark cache as dirty when dependencies change
    $: {
        if (feature || options || standardTag || selectedFeatures || isLLMReady || isLLMInitializing || isLLMError || isNativeLanguageEnglish || enabled || needsDocker || isDockerUnavailable) {
            visibleOptionsDirty = true;
        }
    }
    
    // Reactive variable for visible options (force recalculation when LLM state changes)
    $: visibleOptions = (() => {
        // Dependencies to trigger recalculation
        const deps = [feature, options, standardTag, selectedFeatures, isLLMReady, isLLMInitializing, isLLMError, isNativeLanguageEnglish];
        const result = getVisibleOptions();
        if (feature.id === 'condensedAudio') {
            logger.trace('featureCard', `Visible options for condensedAudio: ${result.length} options, isLLMReady: ${isLLMReady}, isNativeLanguageEnglish: ${isNativeLanguageEnglish}, debugState: ${debugLLMState}`);
        }
        return result;
    })();
    
    // Reactive variable that tracks if feature has visible options
    $: hasAnyVisibleOptions = visibleOptions.length > 0;
    
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
            if (hasAnyVisibleOptions) {
                animating = true;
                
                if (enabled) {
                    // Opening animation
                    // First measure height of the content
                    if (optionsWrapper) {
                        setTimeout(() => {
                            if (optionsWrapper) {
                                optionsHeight = optionsWrapper.offsetHeight;
                            } else {
                                optionsHeight = 0; // Default height when element doesn't exist
                            }
                            if (optionsContainer) {
                                optionsContainer.style.height = optionsHeight + 'px';
                            }
                            
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
                    if (optionsContainer) {
                        optionsContainer.style.height = '0px';
                    }
                    
                    // Reset topmost status when disabled
                    isTopmostFeatureForAnyGroup = false;
                    
                    // Animation complete
                    setTimeout(() => {
                        animating = false;
                    }, 350);
                }
            } else {
                // No options to show, keep container closed
                if (optionsContainer) {
                    optionsContainer.style.height = '0px';
                }
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
        if (enabled && $invalidationErrorStore.some(e => e.id === `provider-${feature.id}`)) {
            return true;
        }
        
        // Feature-specific messages
        if (feature.id === 'subtitleRomanization') {
            if ((enabled && hasVisibleOptions() && needsDocker && !isDockerUnavailable) ||
                    (needsDocker && isDockerUnavailable) ||
                (!standardTag) ||
                (!isRomanizationAvailable)) {
                return true;
            }
        } else if (feature.id === 'selectiveTransliteration') {
            if ((enabled && hasVisibleOptions() && getVisibleOptions().includes('provider') && needsDocker && !isDockerUnavailable) ||
                    (needsDocker && isDockerUnavailable) ||
                (!standardTag) ||
                (standardTag !== 'jpn' && showNonJpnMessage)) {
                return true;
            }
        } else if (feature.id === 'subtitleTokenization') {
            if ((enabled && hasVisibleOptions() && getVisibleOptions().includes('provider') && needsDocker && !isDockerUnavailable) ||
                    (needsDocker && isDockerUnavailable) ||
                showNotAvailableMessage) {
                return true;
            }
        } else if (feature.id === 'condensedAudio') {
            // Show message if enableSummary is true and LLM is ready
            if (enabled && options.enableSummary && isLLMReady) {
                // Show message if no providers at all OR some providers are missing
                return feature.options.summaryProvider.choices.length === 0 || missingProviders.length > 0;
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
           relative {enabled ? '' : 'overflow-hidden'} 
           ripple
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
     on:mouseenter={handleHoverStart}
     on:mouseleave={handleHoverEnd}
>
    <div class="p-4 pr-1 border-b border-white/10
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
                    {#if enabled && $invalidationErrorStore.some(e => e.id === `provider-${feature.id}`)}
                        <div class={messageItemClass}>
                            <span class="material-icons text-[14px] text-log-warn mt-0.5 group-hover:animate-subtlePulse">
                                warning
                            </span>
                            <div class="flex-1 text-xs text-white/90">
                                <span>{$invalidationErrorStore.find(e => e.id === `provider-${feature.id}`)?.message || ''}</span>
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
                        {#if enabled && hasVisibleOptions() && needsDocker && !isDockerUnavailable}
                            <div class={messageItemClass}>
                                <DockerIcon size="1.5em" className="text-blue-400" />
                                <div class="flex-1 text-xs text-white/90">
                                    <span>{dockerEngine} is running and reachable.</span>
                                </div>
                            </div>
                        {/if}
                        
                        {#if needsDocker && isDockerUnavailable}
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
                        {#if enabled && hasVisibleOptions() && getVisibleOptions().includes('provider') && needsDocker && !isDockerUnavailable}
                            <div class={messageItemClass}>
                                <DockerIcon size="1.5em" className="text-blue-400" />
                                <div class="flex-1 text-xs text-white/90">
                                    <span>{dockerEngine} is running and reachable.</span>
                                </div>
                            </div>
                        {/if}
                        
                        {#if needsDocker && isDockerUnavailable}
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
                                <span class="material-icons text-[14px] text-error-soft mt-0.5 group-hover:animate-subtlePulse">
                                    warning
                                </span>
                                <div class="flex-1 text-xs text-white/90">
                                    <span>Sorry, selective transliteration is currently only available for Japanese Kanji transliteration!</span>
                                </div>
                            </div>
                        {/if}
                    
                    {:else if feature.id === 'subtitleTokenization'}
                        <!-- Docker status banners -->
                        {#if enabled && hasVisibleOptions() && getVisibleOptions().includes('provider') && needsDocker && !isDockerUnavailable}
                            <div class={messageItemClass}>
                                <DockerIcon size="1.5em" className="text-blue-400" />
                                <div class="flex-1 text-xs text-white/90">
                                    <span>{dockerEngine} is running and reachable.</span>
                                </div>
                            </div>
                        {/if}
                        
                        {#if needsDocker && isDockerUnavailable}
                            <div class={messageItemClass}>
                                <DockerUnavailableIcon size="1.5em" className="text-blue-400" />
                                <div class="flex-1 text-xs text-[#ff0000] font-bold">
                                    <span>{dockerEngine} is required but not reachable. Please make sure it is installed and running.</span>
                                </div>
                            </div>
                        {/if}
                        
                        {#if showNotAvailableMessage}
                            <div class={messageItemClass}>
                                <span class="material-icons text-[14px] text-error-soft mt-0.5 group-hover:animate-subtlePulse">
                                    warning
                                </span>
                                <div class="flex-1 text-xs text-white/90">
                                    <span>Sorry, no tokenizer is implemented for this language at this time!</span>
                                </div>
                            </div>
                        {/if}
                    
                    {:else if feature.id === 'condensedAudio'}
                        <!-- LLM Provider availability message -->
                        {#if enabled && options.enableSummary && isLLMReady}
                            {#if feature.options.summaryProvider.choices.length === 0}
                                <!-- No providers at all - more prominent warning -->
                                <div class={messageItemClass}>
                                    <span class="material-icons text-[14px] text-log-warn mt-0.5">
                                        warning
                                    </span>
                                    <div class="flex-1 text-xs text-white/90">
                                        <span>No LLM providers available. Configure API keys in Settings to use this feature.</span>
                                    </div>
                                </div>
                            {:else if missingProviders.length > 0}
                                <!-- Some providers missing - subtle info -->
                                <div class={messageItemClass}>
                                    <span class="material-icons text-[14px] text-primary/60 mt-0.5">
                                        info_outline
                                    </span>
                                    <div class="flex-1 text-xs text-white/70">
                                        <span>
                                            Configure 
                                            {#if missingProviders.length === 1}
                                                {missingProviders[0]}
                                            {:else if missingProviders.length === 2}
                                                {missingProviders[0]} and {missingProviders[1]}
                                            {:else}
                                                {missingProviders.slice(0, -1).join(', ')} and {missingProviders[missingProviders.length - 1]}
                                            {/if}
                                            API {missingProviders.length === 1 ? 'key' : 'keys'} to access more models.
                                        </span>
                                    </div>
                                </div>
                            {/if}
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
    {#if hasAnyVisibleOptions}
    {@const _logVisible = logger.trace('featureCard', 'hasVisibleOptions() returned true')}
    <div
    bind:this={optionsContainer} 
    class="overflow-hidden" 
    style="height: {optionsHeight}px; transition: height 350ms cubic-bezier(0.25, 1, 0.5, 1)"
    hidden={!enabled}
    >
        <div bind:this={optionsWrapper} class="p-4">
            <div class="options-grid">
                <!-- Conditionally key the entire options list for condensedAudio feature -->
                {#key feature.id === 'condensedAudio' ? llmState?.globalState : null}
                {#each visibleOptions as optionId}
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
                    
                    {#if isGroupOption && groupId && featureGroupStore.isTopmostInGroup(groupId, feature.id) && !(feature.id !== 'subtitleRomanization' && optionDef.type === 'romanizationDropdown')}
                        <!-- Using canonical ordering from the feature store -->
                        <div class="mb-4 w-full">
                            <GroupOption 
                                {groupId}
                                featureId={feature.id}
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

                                    // Force reactivity update on dependent options if mergeOutputFiles changed
                                    if (groupId === 'merge' && optionId === 'mergeOutputFiles') {
                                        // Add small delay to ensure the group store has updated and sync is complete
                                        setTimeout(() => {
                                            // Force reevaluation of dependent options by marking cache dirty
                                            visibleOptionsDirty = true;
                                            // Trigger a UI update by forcing Svelte to re-render
                                            // Re-assigning options triggers reactivity for calculations depending on it
                                            options = { ...options };
                                        }, 10);
                                    }
                                }}
                            />
                        </div>
                    {:else}
                        <!-- Regular option with label and input -->
                        <div class="option-row">
                            <div class="option-label">
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
                            </div>
                            <div class="option-input">
                                <div class="input-wrapper">
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
                                        <!-- Boolean input (shifted left to align with other inputs) -->
                                        <label class="inline-flex items-center cursor-pointer -ml-5">
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
                                    {:else if optionDef.type === 'dropdown' && optionId === 'stt'}
                                        <Dropdown
                                            options={optionDef.choices || []}
                                            value={options[optionId]}
                                            labelFunction={(option) => {
                                                // Find the model in the models list
                                                const model = currentSTTModels.models.find(m => m.name === option);
                                                if (model) {
                                                    let label = `${model.displayName} @${formatProviderName(model.providerName)}`;
                                                    if (model.isDepreciated) label += ' (DEPRECATED)';
                                                    return label;
                                                }
                                                return option;
                                            }}
                                            tooltipFunction={(option) => {
                                                // Find the model to get its description
                                                const model = currentSTTModels.models.find(m => m.name === option);
                                                if (model) {
                                                    return model.description;
                                                }
                                                return '';
                                            }}
                                            on:change={(e) => handleDropdownChange(optionId, e.detail)}
                                            label={optionDef.label}
                                        />
                                    {:else if optionDef.type === 'dropdown' && optionId === 'summaryProvider' && feature.id === 'condensedAudio'}
                                        <!-- Special handling for summary provider dropdown -->
                                        {#key [optionDef.choices, isLLMReady]}
                                        <Dropdown
                                            options={optionDef.choices || []}
                                            value={options[optionId]}
                                            on:change={(e) => handleDropdownChange(optionId, e.detail)}
                                            label={optionDef.label}
                                            disabled={!isLLMReady}
                                        />
                                        {/key}
                                    {:else if optionDef.type === 'dropdown' && optionId === 'summaryModel' && feature.id === 'condensedAudio'}
                                        <!-- Special handling for summary model dropdown with double-keyed reactivity -->
                                        {#key [optionDef.choices, options.summaryProvider]}
                                        <Dropdown
                                            options={optionDef.choices || []}
                                            value={options[optionId]}
                                            on:change={(e) => handleDropdownChange(optionId, e.detail)}
                                            label={optionDef.label}
                                            disabled={!isLLMReady}
                                        />
                                        {/key}
                                    {:else if optionDef.type === 'dropdown'}
                                        <!-- Standard dropdown for other options -->
                                        {#key optionDef.choices}
                                        <Dropdown
                                            options={optionDef.choices || []}
                                            value={options[optionId]}
                                            on:change={(e) => handleDropdownChange(optionId, e.detail)}
                                            label={optionDef.label}
                                            disabled={feature.id === 'condensedAudio' && (optionId === 'summaryProvider' || optionId === 'summaryModel') && !isLLMReady}
                                        />
                                        {/key}
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
                                                on:input={() =>
                                                    dispatch('optionChange', {
                                                        featureId: feature.id,
                                                        optionId,
                                                        value: options[optionId]
                                                    })
                                                }
                                            />
                                        {:else if optionId === 'summaryCustomPrompt'}
                                            <textarea
                                                bind:value={options[optionId]}
                                                class="w-full bg-sky-dark/50 border-2 border-primary/30 rounded-md px-3 py-2 text-sm font-medium
                                                    focus:border-primary focus:ring-2 focus:ring-primary/30 hover:border-primary/50 focus:outline-none
                                                    transition-all duration-200 placeholder:text-gray-500"
                                                rows="3"
                                                placeholder={optionDef.placeholder}
                                                on:input={() =>
                                                    dispatch('optionChange', {
                                                        featureId: feature.id,
                                                        optionId,
                                                        value: options[optionId]
                                                    })
                                                }
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
                                    {:else if optionDef.type === 'slider'}
                                        <Slider
                                            bind:value={options[optionId]}
                                            min={optionDef.min || 0}
                                            max={optionDef.max || 100}
                                            step={typeof optionDef.step === 'number' ? optionDef.step : parseFloat(optionDef.step || '1')}
                                            disabled={feature.id === 'condensedAudio' && (optionId === 'summaryMaxLength' || optionId === 'summaryTemperature') && !isLLMReady}
                                            showValue={true}
                                            formatValue={(value) => {
                                                // Special formatting for summaryMaxLength
                                                if (optionId === 'summaryMaxLength') {
                                                    if (value === 0) {
                                                        return 'Auto';
                                                    }
                                                    return `${value} words`;
                                                }
                                                // Format temperature with 1 decimal place
                                                if (optionId === 'summaryTemperature') {
                                                    if (value === 0) return '0.0 (deterministic)';
                                                    if (value === 2) return '2.0 (very creative)';
                                                    return value.toFixed(1);
                                                }
                                                // Format limiter (dBFS) - remove trailing zeros
                                                if (optionId === 'limiter') {
                                                    return parseFloat(value.toFixed(4)).toString();
                                                }
                                                return value.toString();
                                            }}
                                            on:change={() => dispatch('optionChange', { featureId: feature.id, optionId, value: options[optionId] })}
                                        />
                                    {/if}
                                </div>
                            </div>
                        </div>
                    {/if}
                {/each}
                {/key}
                
                <!-- LLM Loading indicator card for condensedAudio feature -->
                {#if feature.id === 'condensedAudio' && options.enableSummary && (isLLMInitializing || isLLMError)}
                    <div class="mt-3 w-full" transition:fade={{ duration: 300 }}>
                        {#if debugLLMState}
                            <div class="text-xs text-purple-400 mb-2 flex items-center gap-1" transition:fade={{ duration: 200 }}>
                                <span class="material-icons text-xs">bug_report</span>
                                Debug mode: Forced {debugLLMState} state
                            </div>
                        {/if}
                        {#key llmState?.globalState}
                        {#if isLLMInitializing}
                            <div class="bg-primary/10 border border-primary/30 rounded-lg p-4 flex items-center gap-4" transition:fade={{ duration: 300 }}>
                                <!-- Custom spinner SVG (2x bigger) -->
                                <div class="flex-shrink-0">
                                    <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" class="text-primary">
                                        <g fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2">
                                            <path stroke-dasharray="16" stroke-dashoffset="16" d="M12 3c4.97 0 9 4.03 9 9">
                                                <animate fill="freeze" attributeName="stroke-dashoffset" dur="0.3s" values="16;0"/>
                                                <animateTransform attributeName="transform" dur="1.5s" repeatCount="indefinite" type="rotate" values="0 12 12;360 12 12"/>
                                            </path>
                                            <path stroke-dasharray="64" stroke-dashoffset="64" stroke-opacity="0.3" d="M12 3c4.97 0 9 4.03 9 9c0 4.97 -4.03 9 -9 9c-4.97 0 -9 -4.03 -9 -9c0 -4.97 4.03 -9 9 -9Z">
                                                <animate fill="freeze" attributeName="stroke-dashoffset" dur="1.2s" values="64;0"/>
                                            </path>
                                        </g>
                                    </svg>
                                </div>
                                <div class="flex-1">
                                    <div class="text-sm font-medium text-primary dark:text-primary">
                                        Initializing LLM Providers
                                    </div>
                                    <div class="text-xs text-primary/70 dark:text-primary/70 mt-1">
                                        Summary options will appear once the language models are ready...
                                    </div>
                                </div>
                            </div>
                        {:else if isLLMError}
                            <div class="bg-red-900/20 border border-red-600/30 rounded-lg p-4 flex items-center gap-3" transition:fade={{ duration: 300 }}>
                                <div class="flex-shrink-0">
                                    <span class="material-icons text-red-500">error_outline</span>
                                </div>
                                <div class="flex-1">
                                    <div class="text-sm font-medium text-red-600 dark:text-red-400">
                                        LLM Initialization Failed
                                    </div>
                                    <div class="text-xs text-red-700 dark:text-red-500 mt-1">
                                        {llmErrorMessage || 'Unable to initialize language models. Check your API keys in settings.'}
                                    </div>
                                </div>
                            </div>
                        {/if}
                        {/key}
                    </div>
                {:else if feature.id === 'condensedAudio' && options.enableSummary && debugLLMState === 'ready'}
                    <!-- Show debug indicator when forcing ready state -->
                    <div class="mt-3 w-full" transition:fade={{ duration: 300 }}>
                        <div class="text-xs text-purple-400 flex items-center gap-1" transition:fade={{ duration: 200 }}>
                            <span class="material-icons text-xs">bug_report</span>
                            Debug mode: Forced ready state (options shown)
                        </div>
                    </div>
                {/if}
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
    <!-- Animated border elements for hover effect -->
    {#if !isFeatureDisabled}
        <div class="animated-border-right" aria-hidden="true" bind:this={animatedBorderRight}></div>
        <div class="animated-border-bottom" aria-hidden="true" bind:this={animatedBorderBottom}></div>
    {/if}
</div>

<style>
    @import './featureStyles.css';
    
    /* Animated border elements that respect rounded corners */
    .animated-border-right,
    .animated-border-bottom {
        position: absolute;
        opacity: 0;
        pointer-events: none;
    }
    
    .animated-border-right {
        width: 3.5px;
        top: 0;
        bottom: 0;
        right: 0;
        background: repeating-linear-gradient(
            to bottom,
            hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.6),
            hsla(var(--secondary-hue), var(--secondary-saturation), var(--secondary-lightness), 0.65),
            hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.6),
            hsla(var(--secondary-hue), var(--secondary-saturation), var(--secondary-lightness), 0.65),
            hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.6)
        );
        background-size: 100% 200%;
        border-radius: 0 0.5rem 0.5rem 0; /* Match parent's border radius on right side */
    }
    
    .animated-border-bottom {
        height: 3px;
        left: 0;
        right: 0;
        bottom: 0;
        background: repeating-linear-gradient(
            to right,
            hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.6),
            hsla(var(--secondary-hue), var(--secondary-saturation), var(--secondary-lightness), 0.65),
            hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.6),
            hsla(var(--secondary-hue), var(--secondary-saturation), var(--secondary-lightness), 0.65),
            hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.6)
        );
        background-size: 200% 100%;
        border-radius: 0 0 0.5rem 0.5rem; /* Match parent's border radius on bottom */
    }
    
    /* Simple fade animation for borders */
    @keyframes borderFadeIn {
        from { opacity: 0; }
        to { opacity: 1; }
    }
    
    /* Show and animate borders on hover */
    .feature-card:hover:not(.disabled) .animated-border-right {
        animation: borderFadeIn 0.8s ease-out forwards,
                  smoothFlowToTop 3s 0.8s infinite linear;
    }
    
    .feature-card:hover:not(.disabled) .animated-border-bottom {
        animation: borderFadeIn 0.8s ease-out forwards,
                  smoothFlowToLeft 3s 0.8s infinite linear;
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