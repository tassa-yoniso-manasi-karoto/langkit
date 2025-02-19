<script lang="ts">
    import { createEventDispatcher, onMount, onDestroy } from 'svelte';
    import { slide } from 'svelte/transition';
    import { get } from 'svelte/store';
    
    import { debounce } from 'lodash';
    
    import { settings, showSettings } from '../lib/stores.ts';
    import { errorStore } from '../lib/errorStore';
    import Dropdown from './Dropdown.svelte';
    import Hovertip from './Hovertip.svelte';
    import { GetRomanizationStyles, ValidateLanguageTag, CheckMediaLanguageTags } from '../../wailsjs/go/gui/App';

    // Props
    export let selectedFeatures = {
        subs2cards: false,
        dubtitles: false,
        voiceEnhancing: false,
        subtitleRomanization: false
    };
    export let quickAccessLangTag = '';

    // Interfaces
    interface FeatureOptions {
        subs2cards: {
            padTiming: number;
            screenshotWidth: number;
            screenshotHeight: number;
            condensedAudio: boolean;
        };
        dubtitles: {
            padTiming: number;
            stt: string;
            sttTimeout: number;
            initialPrompt: string;
        };
        voiceEnhancing: {
            sepLib: string;
            voiceBoost: number;
            originalBoost: number;
            limiter: number;
            mergingFormat: string;
        };
        subtitleRomanization: {
            style: string;
            provider: string;
            selectiveTransliteration: number;
            dockerRecreate: boolean;
            browserAccessURL: string;
        };
    }

    interface LanguageCheckResponse {
        isValid: boolean;
        standardTag?: string;
        error?: string;
    }
    const dispatch = createEventDispatcher();
    
    // State variables
    let isValidLanguage: boolean | null = null;
    let isChecking = false;
    let standardTag = '';
    let validationError = '';
    
    let romanizationStyles: string[] = [];
    let isRomanizationAvailable = true;
    
    let dockerUnreachable = false;
    let dockerEngine = '';
    let needsDocker = false;
    let needsScraper = false;
    
    let showAudioTrackIndex = false;
    let audioTrackIndex = 0;
    let hasLanguageTags = true;
    
    let providerWarnings: Record<string, string> = {};
    
    const providersRequiringTokens = {
        'whisper': 'replicate',
        'insanely-fast-whisper': 'replicate',
        'demucs': 'replicate',
        'spleeter': 'replicate',
        'universal-1': 'assemblyAI'
    };

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
    
    export let mediaSource: MediaSource | null = null;

    // Update the checkMediaFiles function
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
    
    // Feature options configuration
    const optionChoices = {
        dubtitles: {
            stt: ["whisper", "insanely-fast-whisper", "universal-1"]
        },
        voiceEnhancing: {
            sepLib: ["demucs", "demucs_ft", "spleeter"],
            mergingFormat: ["mp4", "mkv"]
        }
    };

    const optionOrder = {
        subtitleRomanization: [
            'style',
            'provider',
            'selectiveTransliteration',
            'dockerRecreate',
            'browserAccessURL'
        ],
    };

    // Initialize feature options with default values
    let currentFeatureOptions: FeatureOptions = {
        subs2cards: {
            padTiming: 250,
            screenshotWidth: 1280,
            screenshotHeight: 720,
            condensedAudio: false
        },
        dubtitles: {
            padTiming: 250,
            stt: "whisper",
            sttTimeout: 90,
            initialPrompt: ""
        },
        voiceEnhancing: {
            sepLib: "demucs",
            voiceBoost: 13,
            originalBoost: -9,
            limiter: 0.9,
            mergingFormat: "mp4"
        },
        subtitleRomanization: {
            style: "",
            selectiveTransliteration: 0,
            dockerRecreate: false,
            browserAccessURL: ""
        }
    };

    // Option labels for UI display
    const optionLabels = {
        subs2cards: {
            padTiming: "Padding (ms)",
            screenshotWidth: "Screenshot Width (px)",
            screenshotHeight: "Screenshot Height (px)",
            condensedAudio: "Condensed Audio"
        },
        dubtitles: {
            padTiming: "Padding (ms)",
            stt: "Speech-To-Text",
            sttTimeout: "Speech-To-Text Timeout (sec)",
            initialPrompt: "Initial prompt for Whisper"      
        },
        voiceEnhancing: {
            sepLib: "Voice separation library",
            voiceBoost: "Voice Boost (dB)",
            originalBoost: "Original Audio Boost (dB)",
            limiter: "Limiter (dBFS)"
        },
        subtitleRomanization: {
            style: "Romanization Style",
            selectiveTransliteration: "Retain Kanjis below most frequent",
            dockerRecreate: "Recreate Docker containers",
            browserAccessURL: "Browser access URL"
        }
    };
    
    const optionHovertips = {
        trackOverride: "In case the audiotracks of your media files don't have proper languages tags, set the number/index of the audio track to use as basis for processing here. It is still a good idea to set the language tag for STT, romanization... etc.",
        dubtitles: {
            initialPrompt: "Whisper works best when provided with an initial prompt containing exact names and terms from your audio. List character names with correct spellings (e.g.,'Eren Yeager','Mikasa Ackerman'), unique terminology (e.g.,'ODM gear') and any words the model might struggle with. Limit your prompt to 30-50 key terms for optimal results. Prioritize words that appear frequently in your audio and those with unusual pronunciations or spellings. Use comma separation rather than complete sentences. Avoid adding plot information or dialogue patterns - stick to names and terminology only. If transcribing series content like podcasts or shows, add location names and recurring concepts that define the content's universe. Maximum length is 224 tokens (approx. 850 characters)."    
        },
        subtitleRomanization: {
            browserAccessURL: "URL to programmatically control a Chromium-based browser through Devtools. Required for providers that need web scraping capabilities",
            selectiveTransliteration: "Set a threshold value so that high-frequency Kanji in subtitles are preserved while less common or irregular Kanjis are transliterated to hiragana",
            dockerRecreate: "Use this if the previous run failed or if you're experiencing issues."
        }
    };
    
    const providerGithubUrls = {
        'ichiran': 'https://github.com/tshatrov/ichiran',
        'aksharamukha': 'https://github.com/virtualvinodh/aksharamukha',
        'iuliia': 'https://github.com/mehanizm/iuliia-go',
    };
    // Helper function to format display text
    function formatDisplayText(text: string): string {
        return text
            .replace(/([A-Z])/g, ' $1')
            .split(/(?=[A-Z])/)
            .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
            .join(' ');
    }

    // Language validation with debounce
    const validateLanguageTag = debounce(async (code: string, maxOne: boolean) => {
        //console.log('Validating language:', code, 'maxOne?:', maxOne);
        
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
            //console.log('Validation response:', response);
            
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
    }, 300); // Reduced debounce time for better responsiveness

   interface RomanizationScheme {
        name: string;
        description: string;
        provider: string;
    }

    // Update state variables
    let romanizationSchemes: RomanizationScheme[] = [];
    
    async function updateRomanizationStyles(tag: string) {
        //console.log('Updating romanization styles for:', tag);
        
        if (!tag?.trim()) {
            //console.log('No valid tag provided, disabling romanization');
            romanizationSchemes = [];
            isRomanizationAvailable = false;
            needsDocker = false;
            needsScraper = false;
            if (selectedFeatures.subtitleRomanization) {
                selectedFeatures.subtitleRomanization = false;
            }
            return;
        }

        try {
            const response = await GetRomanizationStyles(tag);
            //console.log('Received romanization response:', response);
            
            romanizationSchemes = response.schemes || [];
            isRomanizationAvailable = romanizationSchemes.length > 0;
            needsScraper = response.needsScraper || false;
            dockerUnreachable = response.dockerUnreachable || false;
            needsDocker = response.needsDocker || false;
            dockerEngine = response.dockerEngine || 'Docker Desktop';
            
            // Automatically set the first scheme if only one is available
            if (romanizationSchemes.length === 1) {
                currentFeatureOptions.subtitleRomanization.style = romanizationSchemes[0].name;
            }
            
            if (!isRomanizationAvailable && selectedFeatures.subtitleRomanization) {
                selectedFeatures.subtitleRomanization = false;
                errorStore.addError({
                    id: 'no-romanization',
                    message: 'No transliteration scheme available for selected language',
                    severity: 'warning'
                });
            }
        } catch (error) {
            console.error('Error fetching romanization styles:', error);
            romanizationSchemes = [];
            isRomanizationAvailable = false;
            if (selectedFeatures.subtitleRomanization) {
                selectedFeatures.subtitleRomanization = false;
            }
        }
    }

    // Handle dropdown changes
    function handleDropdownChange(feature: string, option: string, value: string) {
        currentFeatureOptions[feature][option] = value;
        dispatch('optionsChange', currentFeatureOptions);
    }
    
    // Helper function for text color classes
    function getTextColorClass(enabled: boolean, anyFeatureSelected: boolean): string {
        if (enabled) return 'text-white';
        if (!anyFeatureSelected) return 'text-white';
        return 'text-white/70';
    }

    // Reactive statements
    $: anyFeatureSelected = Object.values(selectedFeatures).some(v => v);

    $: if (quickAccessLangTag !== undefined) {
        needsDocker = false;
        needsScraper = false;
        validateLanguageTag(quickAccessLangTag, true);
        // Force update of romanization availability
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

        // Provider API errors
        Object.entries(providerWarnings).forEach(([feature, warning]) => {
            errorStore.addError({
                id: `provider-${feature}`,
                message: warning,
                severity: 'critical'
            });
        });
    }

    // Settings subscription
    settings.subscribe(value => {
        console.log('Settings updated:', value);
        if (value?.targetLanguage && value.targetLanguage !== quickAccessLangTag) {
            console.log('Updating language code from settings:', value.targetLanguage);
            quickAccessLangTag = value.targetLanguage;
            validateLanguageTag(value.targetLanguage, true);
        }
    });
    settings.subscribe(value => {
        if (value) {
            updateProviderWarnings();
        }
    });
    
    $: if (selectedFeatures) {
    updateProviderWarnings();
    }

    $: if (currentFeatureOptions) {
    updateProviderWarnings();
    }

    onMount(async () => {
        const currentSettings = get(settings);
        if (currentSettings?.targetLanguage) {
            quickAccessLangTag = currentSettings.targetLanguage;
            await validateLanguageTag(currentSettings.targetLanguage, true);
        }
         updateProviderWarnings();
    });

    $: {
        dispatch('optionsChange', currentFeatureOptions);
    }
    
    // Add the Japanese-specific option to the feature options when applicable
    $: if (standardTag === 'jpn' && !currentFeatureOptions.subtitleRomanization.hasOwnProperty('selectiveTransliteration')) {
        currentFeatureOptions = {
            ...currentFeatureOptions,
            subtitleRomanization: {
                ...currentFeatureOptions.subtitleRomanization,
                selectiveTransliteration: 100
            }
        };
    } else if (standardTag != 'jpn' && currentFeatureOptions.subtitleRomanization.hasOwnProperty('selectiveTransliteration')) {
        const { selectiveTransliteration, ...rest } = currentFeatureOptions.subtitleRomanization;
        currentFeatureOptions = {
            ...currentFeatureOptions,
            subtitleRomanization: rest
        };
    }
    // Update warnings when options or features change
    $: {
        if (currentFeatureOptions) {
            updateProviderWarnings();
        }
    }
    
    $: if (mediaSource) {
        checkMediaFiles();
    } else {
        // Reset audio track related states when no path is selected
        showAudioTrackIndex = false;
        hasLanguageTags = true;
        audioTrackIndex = 0;
    }
    
    $: {
        if (selectedFeatures.subtitleRomanization && needsDocker && dockerUnreachable) {
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
    
    // Compute the ordering for dubtitles options so that the “initialPrompt” is only shown for whisper.
    $: dubtitlesOptionOrder = currentFeatureOptions.dubtitles.stt === "whisper"
        ? ['padTiming', 'stt', 'sttTimeout', 'initialPrompt']
        : ['padTiming', 'stt', 'sttTimeout'];
        
    onDestroy(() => {
        errorStore.removeError('docker-required');
        errorStore.removeError('invalid-browser-url');
        errorStore.removeError('no-features');
        errorStore.removeError('invalid-language');
    });
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between pl-0 pr-0">
        <h2 class="text-xl font-medium text-accent/90 flex items-center pl-4 gap-2">
            <span class="material-icons text-accent/70">tune</span>
            Select Features
        </h2>
        
        <!-- Target Language input -->
        <div class="flex items-center ml-auto item-right gap-2 pr-3">
            <span class="text-accent text-sm whitespace-nowrap">
                Target Language
            </span>
            <input
                type="text"
                bind:value={quickAccessLangTag}
                maxlength="9"
                placeholder="e.g. ja, zh-Hans"
                class="w-24 bg-sky-dark/50 border border-accent/30 rounded px-2 py-2
                       focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
                       transition-colors duration-200 text-xs font-medium"
            />
            {#if isChecking}
                <span class="absolute right-4 top-1/2 -translate-y-1/2
                            material-icons animate-spin text-accent/70 text-sm">
                    refresh
                </span>
            {:else if isValidLanguage === false}
                <span class="absolute right-4 top-1/2 -translate-y-1/2
                            material-icons text-red-500 text-sm"
                      title={validationError}>
                    error
                </span>
            {:else if isValidLanguage === true}
                <span class="absolute right-4 top-1/2 -translate-y-1/2
                            material-icons text-green-300 text-sm">
                    check_circle
                </span>
            {/if}
        </div>

        <!-- Audio track selection with slide animation -->
        <div class="flex">
          <!-- Disclosure triangle button with connected border and matching background -->
          <button
            class="flex items-center justify-center p-4 w-6 h-6
                   border border-accent/30 
                   hover:border-accent/60 hover:bg-accent/10 
                   transition-all duration-500 focus:outline-none
                   {showAudioTrackIndex 
                      ? 'bg-accent/5 rounded-tl rounded-bl rounded-tr-none rounded-br-none'
                      : 'rounded'}"
            on:click={() => {
              showAudioTrackIndex = !showAudioTrackIndex;
              if (!showAudioTrackIndex) {
                  audioTrackIndex = 0; // Reset to 0 when hiding
              } else {
                  audioTrackIndex = audioTrackIndex === 0 ? 1 : audioTrackIndex;
              }
            }}
            title="Toggle audio track selection"
          >
            <span class="transform transition-transform duration-1000 text-accent/70
                         hover:text-accent text-2xl leading-none"
                  class:rotate-180={showAudioTrackIndex}>
              ◀
            </span>
          </button>

          <!-- Audio track input with slide animation -->
          {#if showAudioTrackIndex || !hasLanguageTags}
            <!-- Use negative left margin to overlap the shared border -->
            <div class="-ml-px flex items-center"
                 transition:slide={{ duration: 200, axis: 'x' }}>
              <!-- Panel: use matching background and borders; remove left rounding -->
              <div class="flex items-center bg-accent/5 
                         border border-accent/30 border-l-0
                         rounded-r px-2 p-4 h-6">
                <span class="text-accent text-sm whitespace-nowrap">
                  Track Override
                </span>
	            <Hovertip message="{optionHovertips.trackOverride}">
		            <!-- The trigger slot: the element that activates the hovertip -->
		            <span slot="trigger" class="material-icons text-accent/70 cursor-help pr-1 leading-none material-icon-adjust">
			            help_outline
		            </span>
	            </Hovertip>
                <!-- The input field: reduced horizontal padding and fixed height -->
                <input
                  type="number"
                  bind:value={audioTrackIndex}
                  min="1"
                  max="99"
                  class="w-10 h-6 bg-sky-dark/50 border border-accent/30 rounded
                         px-1 text-xs font-medium text-center
                         focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
                         transition-colors duration-200"
                />
              </div>
            </div>
          {/if}
        </div>


    </div>
    
    <div class="space-y-4">
        {#each Object.entries(selectedFeatures) as [feature, enabled]}
            <div class="bg-white/5 rounded-lg
                     transition-all duration-300 ease-out transform
                     {!isRomanizationAvailable && feature === 'subtitleRomanization' 
                        ? 'opacity-50 cursor-not-allowed' 
                        : 'hover:translate-y-[-2px]'}"
                 class:shadow-glow-strong={enabled && !anyFeatureSelected}
                 class:shadow-glow={enabled}
                 class:hover:shadow-glow-hover={!enabled && isRomanizationAvailable}
                 class:opacity-30={anyFeatureSelected && !enabled}
                 on:click={() => {
                    if (!isRomanizationAvailable && feature === 'subtitleRomanization') {
                        const element = event.currentTarget;
                        element.classList.remove('shake-animation');
                        void element.offsetWidth;
                        element.classList.add('shake-animation');
                    }
                 }}>
                <div class="p-4 border-b border-white/10">
                    <label class="flex items-center gap-3 cursor-pointer group
                                {!isRomanizationAvailable && feature === 'subtitleRomanization' ? 'cursor-not-allowed' : ''}">
                        <input
                            type="checkbox"
                            class="w-4 h-4 accent-accent"
                            bind:checked={selectedFeatures[feature]}
                            disabled={!isRomanizationAvailable && feature === 'subtitleRomanization'}
                        />
                        <span class="text-lg transition-all duration-300 {getTextColorClass(enabled, anyFeatureSelected)}
                                   group-hover:text-accent/90"
                              class:font-semibold={enabled || !anyFeatureSelected}>
                            {formatDisplayText(feature)}
                        </span>
                    </label>
                    {#if enabled && get(errorStore).some(e => e.id === `provider-${feature}`)}
                        <div class="mt-2 flex items-center gap-2 text-red-400 text-xs pl-7">
                            <span class="material-icons text-[14px]">warning</span>
                            <span>
                                {get(errorStore).find(e => e.id === `provider-${feature}`)?.message}
                                <button 
                                    class="ml-1 text-accent hover:text-accent/80 transition-colors"
                                    on:click={() => $showSettings = true}
                                >
                                    Configure API Keys
                                </button>
                            </span>
                        </div>
                    {/if}
                    
                    {#if feature === 'subtitleRomanization'}
                        {#if selectedFeatures.subtitleRomanization && needsDocker && !dockerUnreachable}
                            <div class="mt-2 flex items-left text-xs font-bold text-green-300 pl-7">
                                🟢 {dockerEngine} is running and reachable.	&nbsp;<span class="relative top-[-3px]"> 🐳</span>
                            </div>
                        {/if}
                        {#if needsDocker && dockerUnreachable}
                            <div class="mt-2 flex items-left text-xs font-bold text-red-500 pl-7">
                                🔴 {dockerEngine} is required but not reachable. Please make sure it is installed and running.
                            </div>
                        {:else if !quickAccessLangTag}
                            <div class="mt-2 flex items-left text-xs text-white/80 pl-7">
                                Please select a language to proceed.
                            </div>
                        {:else if !isValidLanguage}
                            <div class="mt-2 flex items-left text-xs text-white/80 pl-7">
                                Please enter a valid language code.
                            </div>
                        {:else if !isRomanizationAvailable}
                            <div class="mt-2 flex items-left text-xs text-white/80 pl-7">
                                Sorry, no transliteration scheme has been implemented for this language yet! 
                            </div>
                            <div class="mt-2 flex items-left text-xs text-white/80 pl-7">
                                <a href="https://github.com/tassa-yoniso-manasi-karoto/translitkit" 
                                   target="_blank" 
                                   class="hover:text-white/60 transition-colors duration-200">
                                    Pull requests and feedback are welcome.
                                </a>
                            </div>
                        {/if}
                    {/if}
                </div>
                
                {#if enabled}
                    <div class="p-4" transition:slide={{ duration: 300 }}>
                        <div class="grid grid-cols-[1fr,1.5fr] gap-x-6 gap-y-3 transition-opacity duration-300">
                            {#each (optionOrder[feature] || Object.keys(currentFeatureOptions[feature])) as option}
                                {@const value = currentFeatureOptions[feature][option]}
                                <div class="flex items-center">
                                    <!-- instead of manually patching in the options that are meant to appear only in a certain configuration we use
                                    the same looping logic as any other options to declare them and write empty if statement for the configurations
                                    where they are not meant to appear (there is probably a better way to achieve the same result but this is the
                                    simplest I can think of) -->
                                    {#if option === 'selectiveTransliteration' && !(standardTag === 'jpn')}
                                    {:else if option === 'initialPrompt' && currentFeatureOptions.dubtitles.stt !== 'whisper'}
                                    {:else if option === 'dockerRecreate' && !needsDocker}
                                    {:else if option === 'browserAccessURL' && !needsScraper}
                                    {:else if option === 'provider'}
                                        <span class="text-gray-300 text-sm text-left">Provider</span>
                                    {:else}
                                        <span class="text-gray-300 text-sm text-left flex items-center gap-2">
                                            {optionLabels[feature][option] || formatDisplayText(option)}
                                            {#if optionHovertips[feature]?.[option]}
	                                            <Hovertip message="{optionHovertips[feature][option]}">
		                                            <!-- The trigger slot: the element that activates the hovertip -->
		                                            <span slot="trigger" class="material-icons text-accent/70 cursor-help pr-1 leading-none material-icon-adjust">
			                                            help_outline
		                                            </span>
	                                            </Hovertip>
                                            {/if}
                                        </span>
                                    {/if}
                                </div>
                                <div>
                                    {#if (option === 'selectiveTransliteration')}
                                        {#if (standardTag === 'jpn')}
                                            <input 
                                                type="number" 
                                                bind:value={currentFeatureOptions[feature][option]}
                                                min="1"
                                                max="3000"
                                                class="w-full h-[42px] bg-sky-dark/50 border-2 border-accent/30 rounded-md px-3
                                                       focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/30
                                                       hover:border-accent/50
                                                       transition-all duration-200 text-sm font-medium text-center"
                                                placeholder="Enter threshold (e.g., 100)"
                                            />
                                         {/if}
                                    {:else if option === 'initialPrompt'}
                                        {#if currentFeatureOptions.dubtitles.stt === 'whisper'}
                                     		<textarea
			                                    bind:value={currentFeatureOptions.dubtitles.initialPrompt}
			                                    class="w-full bg-sky-dark/50 border border-accent/30 rounded px-3 py-2 text-sm font-medium 
			                                           focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
			                                           transition-colors duration-200 placeholder:text-gray-500"
			                                    rows="3"
			                                    maxlength="850"
			                                    placeholder="e.g. Attack on Titan: Eren Yeager, Mikasa Ackerman, Armin Arlert, Titans, Colossal Titan, Armored Titan, Survey Corps, Wall Maria, Wall Rose, Wall Sina, ODM gear, Omni-directional mobility gear, Captain Levi, Commander Erwin Smith, Cadet Corps, Garrison Regiment, Military Police, Trost District, Shiganshina District, 3D Maneuver Gear, Sasha Blouse, Jean Kirstein, Connie Springer, Reiner Braun, Bertholdt Hoover, Annie Leonhart, Hange Zoë, Grisha Yeager, Carla Yeager, Cannons, blades, survey mission, beyond the walls, Scout Regiment, titan attack, breach, trainees, The 104th Cadet Corps"
		                                    />
                                        {/if}
                                    {:else if optionChoices[feature]?.[option]}
                                        <Dropdown
                                            options={optionChoices[feature][option]}
                                            value={currentFeatureOptions[feature][option]}
                                            on:change={(e) => handleDropdownChange(feature, option, e.detail)}
                                            label={optionLabels[feature][option] || formatDisplayText(option)}
                                        />
                                    {:else if typeof value === 'boolean'}
                                        {#if option === 'dockerRecreate' && !needsDocker}
                                        {:else}
                                        <label class="inline-flex items-center cursor-pointer">
                                            <input 
                                                type="checkbox" 
                                                class="w-5 h-5 accent-accent rounded border-2 border-accent/50 
                                                       checked:bg-accent checked:border-accent
                                                       focus:ring-2 focus:ring-accent/30
                                                       transition-all duration-200
                                                       cursor-pointer"
                                                bind:checked={currentFeatureOptions[feature][option]}
                                            />
                                        </label>
                                        {/if}
                                    {:else if typeof value === 'number'}
                                        <input 
                                            type="number" 
                                            step={option.includes('Boost') ? '0.1' : '1'}
                                            bind:value={currentFeatureOptions[feature][option]}
                                            class="w-full h-[42px] bg-sky-dark/50 border-2 border-accent/30 rounded-md px-3
                                                   focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/30
                                                   hover:border-accent/50
                                                   transition-all duration-200 text-sm font-medium text-center"
                                        />
                                    {:else if feature === 'subtitleRomanization' && option === 'style'}
                                        <Dropdown
                                            options={romanizationSchemes}
                                            optionKey="name"
                                            optionLabel="description"
                                            value={currentFeatureOptions[feature][option]}
                                            on:change={(e) => {
                                                handleDropdownChange(feature, option, e.detail);
                                                const selectedScheme = romanizationSchemes.find(s => s.name === e.detail);
                                                if (selectedScheme) {
                                                    currentFeatureOptions[feature]['provider'] = selectedScheme.provider;
                                                }
                                            }}
                                            label="Select style"
                                        />
                                    {:else if feature === 'subtitleRomanization' && option === 'provider'}
                                        <div class="w-full px-3 py-1 text-sm inline-flex font-bold text-white/90 items-center justify-center gap-2">
                                            {#if currentFeatureOptions[feature]['style']}
                                                {@const provider = romanizationSchemes.find(s => s.name === currentFeatureOptions[feature]['style'])?.provider || ''}
                                                {provider}
                                                {#if providerGithubUrls[provider]}
                                                    <a href={providerGithubUrls[provider]}
                                                       target="_blank"
                                                       rel="noopener noreferrer"
                                                       class="text-accent/70 hover:text-accent transition-colors duration-200"
                                                       title="View provider repository">
                                                        <svg viewBox="0 0 16 16" class="w-5 h-5" fill="currentColor">
                                                            <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                                                        </svg>
                                                    </a>
                                                {/if}
                                            {/if}
                                        </div>
                                    {:else if option === 'browserAccessURL' && !needsScraper}
                                        <!-- Skip rendering -->
                                    {:else if option === 'browserAccessURL' && needsScraper}
                                        <input 
                                            type="text"
                                            bind:value={currentFeatureOptions[feature][option]}
                                            class="w-full bg-sky-dark/50 border border-accent/30 rounded px-3 py-1
                                                   focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
                                                   transition-colors duration-200 text-sm font-medium placeholder:text-gray-500"
                                            placeholder="e.g. ws://127.0.0.1:9222/devtools/browser/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
                                        />
                                    {:else}
                                        <input 
                                            type="text"
                                            bind:value={currentFeatureOptions[feature][option]}
                                            class="w-full bg-sky-dark/50 border border-accent/30 rounded px-3 py-1
                                                   focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
                                                   transition-colors duration-200 text-sm font-medium"
                                        />
                                    {/if}
                                </div>
                            {/each}
                        </div>
                    </div>
                {/if}
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
    /* Glow effects for different states */
    :global(.shadow-glow) {
        box-shadow: 2px 2px 0 0 rgba(159, 110, 247, 0.3),
                   4px 4px 8px -2px rgba(159, 110, 247, 0.2);
    }

    :global(.shadow-glow-strong) {
        box-shadow: 2px 2px 0 0 rgba(159, 110, 247, 0.4),
                   4px 4px 12px -2px rgba(159, 110, 247, 0.25);
    }

    :global(.shadow-glow-hover) {
        box-shadow: 2px 2px 0 0 rgba(159, 110, 247, 0.35),
                   4px 4px 16px -2px rgba(159, 110, 247, 0.3);
    }
    @keyframes shake {
        0%, 100% { transform: translateX(0) translateY(0); }
        25% { transform: translateX(-2px) translateY(0); }
        75% { transform: translateX(2px) translateY(0); }
    }
    
    .shake-animation {
        animation: shake 0.4s ease-in-out;
        position: relative;
    }
    .group:hover div[class*="group-hover"] {
        transition: opacity 150ms ease-in-out;
        opacity: 1;
    }

    div[class*="group-hover"] {
        opacity: 0;
        pointer-events: none;
    }

    .group:hover div[class*="group-hover"] {
        pointer-events: auto;
    }
    .material-icon-adjust {
	    position: relative;
	    top: 5px; /* Adjust this value as needed */
    }
</style>