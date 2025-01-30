<script lang="ts">
    import { createEventDispatcher } from 'svelte';
    import { slide } from 'svelte/transition';
    import { debounce } from 'lodash';
    
    import { settings } from '../lib/stores.ts';
    import Dropdown from './Dropdown.svelte';
    
    import { GetRomanizationStyles } from '../../wailsjs/go/gui/App';
    import { CheckLanguageCode } from '../../wailsjs/go/gui/App';
    
    const dispatch = createEventDispatcher();

    $: {
        dispatch('optionsChange', currentFeatureOptions);
    }
    
    export let defaultLanguage = '';
    export let selectedFeatures = {
        subs2cards: false,
        dubtitles: false,
        voiceEnhancing: false,
        subtitleRomanization: false
    };
    
    interface OptionLabels {
        [key: string]: {
            [key: string]: string;
        };
    }

    interface LanguageCheckResponse {
        standardCode: string;
        isValid: boolean;
    }
    
    let romanizationStyles: string[] = ["n/a"];
    
    // Define option choices as a constant
    const optionChoices = {
        dubtitles: {
            stt: ["whisper", "insanely-fast-whisper", "universal-1"]
        },
        voiceEnhancing: {
            sepLib: ["demucs", "demucs_ft", "spleeter"],
            mergingFormat: ["mp4", "mkv"]
        }
    };
    
    let currentFeatureOptions = {
        subs2cards: {
            padTiming: 250,
            screenshotWidth: 1280,
            screenshotHeight: 720,
            condensedAudio: false
        },
        dubtitles: {
            padTiming: 250,
            stt: "whisper",          // Default value
            sttTimeout: 90
        },
        voiceEnhancing: {
            sepLib: "demucs",        // Default value
            voiceBoost: 13,
            originalBoost: -9,
            limiter: 0.9,
            mergingFormat: "mp4"     // Default value
        },
        subtitleRomanization: {
            style: "",
            selectiveTransliteration: 0
        }
    };

    // When a dropdown selection changes, update the corresponding option
    function handleDropdownChange(feature: string, option: string, value: string) {
        currentFeatureOptions[feature][option] = value;
        dispatch('optionsChange', currentFeatureOptions);
    }

    const optionLabels = {
        subs2cards: {
            padTiming: "Padding (ms)",
            screenshotWidth: "Screenshot Width (px)",
            screenshotHeight: "Screenshot Height (px)"
        },
        dubtitles: {
            padTiming: "Padding (ms)",
            stt: "STT",
            sttTimeout: "STT Timeout (sec)"
        },
        voiceEnhancing: {
            sepLib: "Voice separation library to use",
            voiceBoost: "Voice Boost (dB)",
            originalBoost: "Original Audio Boost (dB)",
            limiter: "Limiter (dBFS) (cf. ffmpeg's alimiter)"
        },
        subtitleRomanization: {
            selectiveTransliteration: "Retain Kanjis bellow most frequent",
        }
    };
    
    let languageCode = '';
    let isValidLanguage: boolean | null = null;
    let standardCode = '';
    let isChecking = false;
    
    function formatDisplayText(text: string): string {
        return text
            .replace(/([A-Z])/g, ' $1')
            .split(/(?=[A-Z])/)
            .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
            .join(' ');
    }

    $: anyFeatureSelected = Object.values(selectedFeatures).some(v => v);
    
    function getTextColorClass(enabled: boolean, anyFeatureSelected: boolean): string {
        if (enabled) return 'text-white';
        if (!anyFeatureSelected) return 'text-white';
        return 'text-white/70';
    }

    let isDefaultLoaded = false;

    settings.subscribe(value => {
        if (value.targetLanguage && !isDefaultLoaded) {
            languageCode = value.targetLanguage;
            checkLanguageCode(value.targetLanguage);
            isDefaultLoaded = true;
        }
    });
    const checkLanguageCode = debounce(async (code: string) => {
        if (!code) {
            isValidLanguage = null;
            isChecking = false;
            standardCode = '';
            return;
        }
        
        isChecking = true;
        try {
            const response: LanguageCheckResponse = await CheckLanguageCode(code.toLowerCase());
            isValidLanguage = response.isValid;
            standardCode = response.standardCode;
            console.log('Response:', response, 'Input:', code);
        } catch (error) {
            console.error('Error checking language code:', error);
            isValidLanguage = null;
            standardCode = '';
        }
        isChecking = false;
    }, 500);

    $: if (languageCode !== undefined) {
        checkLanguageCode(languageCode);
    }

    $: isJapanese = standardCode === 'jpn';

    // Update options when Japanese is detected
    $: if (isJapanese && !currentFeatureOptions.subtitleRomanization.hasOwnProperty('selectiveTransliteration')) {
        currentFeatureOptions = {
            ...currentFeatureOptions,
            subtitleRomanization: {
                ...currentFeatureOptions.subtitleRomanization,
                selectiveTransliteration: 100
            }
        };
    } else if (!isJapanese && currentFeatureOptions.subtitleRomanization.hasOwnProperty('selectiveTransliteration')) {
        const { selectiveTransliteration, ...rest } = currentFeatureOptions.subtitleRomanization;
        currentFeatureOptions = {
            ...currentFeatureOptions,
            subtitleRomanization: rest
        };
    }
    
    // Add a store for selected values
    let selectedValues = {};

    // Initialize selected values with first array item for each array option
    $: {
        Object.entries(currentFeatureOptions).forEach(([feature, options]) => {
            Object.entries(options).forEach(([option, value]) => {
                if (Array.isArray(value) && !selectedValues[`${feature}_${option}`]) {
                    selectedValues[`${feature}_${option}`] = value[0];
                }
            });
        });
    }
    // Update romanization styles when language changes
    $: if (standardCode) {
        updateRomanizationStyles(standardCode);
    }
    let isRomanizationAvailable = true;

    $: isRomanizationAvailable = Boolean(standardCode && romanizationStyles.length > 0);

    // Watch for changes in standardCode and romanization availability
    $: {
        if (!standardCode || !isRomanizationAvailable) {
            selectedFeatures.subtitleRomanization = false;
        }
    }
    
    async function updateRomanizationStyles(langCode: string) {
        if (!langCode) {
            romanizationStyles = [];
            isRomanizationAvailable = false;
            selectedFeatures.subtitleRomanization = false;
            return;
        }

        try {
            const styles = await GetRomanizationStyles(langCode);
            romanizationStyles = styles;
            isRomanizationAvailable = styles.length > 0;
            
            if (!isRomanizationAvailable) {
                selectedFeatures.subtitleRomanization = false;
            }
            
            if (isRomanizationAvailable && 
                (!selectedValues['subtitleRomanization_style'] || 
                !styles.includes(selectedValues['subtitleRomanization_style']))) {
                selectedValues['subtitleRomanization_style'] = styles[0];
            }
        } catch (error) {
            console.error('Error fetching romanization styles:', error);
            romanizationStyles = [];
            isRomanizationAvailable = false;
            selectedFeatures.subtitleRomanization = false;
        }
    }
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between pl-8 pr-4">
        <h2 class="text-xl font-medium text-accent/90 flex items-center gap-2">
            <span class="material-icons text-accent/70">tune</span>
            Select Features
        </h2>
        
        <div class="relative flex items-center gap-3 px-4">
            <span class="text-sm text-accent/70 font-medium">Language Code</span>
            <div class="relative">
                <input
                    type="text"
                    bind:value={languageCode}
                    placeholder=""
                    class="w-12 h-7 bg-white/5 border border-accent/30 rounded px-2
                         text-sm font-medium text-center
                         focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/50
                         transition-all duration-200
                         {isValidLanguage === false ? 'border-[#ec5f67] focus:ring-[#ec5f67]/50' : ''}
                         {isValidLanguage === true ? 'border-[#99c794] focus:ring-[#99c794]/50' : ''}"
                />
                {#if isChecking}
                    <span class="absolute left-[calc(100%+12px)] top-1/2 -translate-y-1/2
                                material-icons animate-spin text-accent/70 text-sm">
                        refresh
                    </span>
                {:else if isValidLanguage === false}
                    <span class="absolute left-[calc(100%+12px)] top-1/2 -translate-y-1/2
                                material-icons text-[#ec5f67] text-sm">
                        error
                    </span>
                {:else if isValidLanguage === true}
                    <span class="absolute left-[calc(100%+12px)] top-1/2 -translate-y-1/2
                                material-icons text-[#99c794] text-sm">
                        check
                    </span>
                {/if}
            </div>
        </div>
    </div>
    
    <div class="space-y-4">
        {#each Object.entries(selectedFeatures) as [feature, enabled]}
            <div class="bg-white/5 rounded-lg
                       transition-all duration-300 ease-out transform
                       hover:translate-y-[-2px]
                       {!isRomanizationAvailable && feature === 'subtitleRomanization' ? 'opacity-50' : ''}"
                 class:shadow-glow-strong={enabled && !anyFeatureSelected}
                 class:shadow-glow={enabled}
                 class:hover:shadow-glow-hover={!enabled}
                 class:opacity-30={anyFeatureSelected && !enabled}>
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

                    {#if feature === 'subtitleRomanization'}
                        {#if !standardCode}
                            <div class="mt-2 flex items-left text-xs text-white/80 pl-7">
                                Please select a language to proceed.
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
                            {#each Object.entries(currentFeatureOptions[feature]) as [option, value]}
                                <div class="flex items-center">
                                    <span class="text-gray-300 text-sm">
                                        {optionLabels[feature][option] || formatDisplayText(option)}
                                    </span>
                                </div>
                                <div>
                                    {#if optionChoices[feature]?.[option]}
                                        <Dropdown
                                            options={optionChoices[feature][option]}
                                            value={currentFeatureOptions[feature][option]}
                                            on:change={(e) => handleDropdownChange(feature, option, e.detail)}
                                            label={optionLabels[feature][option] || formatDisplayText(option)}
                                        />
                                    {:else if typeof value === 'boolean'}
                                        <label class="inline-flex items-center cursor-pointer">
                                            <input 
                                                type="checkbox" 
                                                class="w-4 h-4 accent-accent"
                                                bind:checked={currentFeatureOptions[feature][option]}
                                            />
                                        </label>
                                    {:else if typeof value === 'number'}
                                        <input 
                                            type="number" 
                                            step="0.1"
                                            bind:value={currentFeatureOptions[feature][option]}
                                            class="w-full bg-sky-dark/50 border border-accent/30 rounded px-3 py-1
                                                   focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
                                                   transition-colors duration-200 text-sm
                                                   font-medium" 
                                        />
                                    {:else}
                                        <input 
                                            type="text"
                                            bind:value={currentFeatureOptions[feature][option]}
                                            class="w-full bg-sky-dark/50 border border-accent/30 rounded px-3 py-1
                                                   focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
                                                   transition-colors duration-200 text-sm
                                                   font-medium" 
                                        />
                                    {/if}
                                </div>
                            {/each}
                        </div>
                    </div>
                {/if}
            </div>
        {/each}
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
</style>