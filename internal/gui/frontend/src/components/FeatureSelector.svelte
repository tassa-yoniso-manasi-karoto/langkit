<script lang="ts">
    import { createEventDispatcher, onMount } from 'svelte';
    import { slide } from 'svelte/transition';
    import { get } from 'svelte/store';
    
    import { debounce } from 'lodash';
    
    import { settings } from '../lib/stores';
    import Dropdown from './Dropdown.svelte';
    import { GetRomanizationStyles, ValidateLanguageTag } from '../../wailsjs/go/gui/App';

    // Initialize event dispatcher for parent communication
    const dispatch = createEventDispatcher();

    // Props
    export let selectedFeatures = {
        subs2cards: false,
        dubtitles: false,
        voiceEnhancing: false,
        subtitleRomanization: false
    };

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
            selectiveTransliteration: number;
        };
    }

    interface LanguageCheckResponse {
        isValid: boolean;
        standardTag?: string;
        error?: string;
    }
    
    // State variables
    let languageCode = '';
    let isValidLanguage: boolean | null = null;
    let isChecking = false;
    let standardTag = '';
    let validationError = '';
    let romanizationStyles: string[] = [];
    let isRomanizationAvailable = true;

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
            sttTimeout: 90
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
            selectiveTransliteration: 0
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
            sttTimeout: "Speech-To-Text Timeout (sec)"
        },
        voiceEnhancing: {
            sepLib: "Voice separation library",
            voiceBoost: "Voice Boost (dB)",
            originalBoost: "Original Audio Boost (dB)",
            limiter: "Limiter (dBFS)"
        },
        subtitleRomanization: {
            style: "Romanization Style",
            selectiveTransliteration: "Retain Kanjis below most frequent"
        }
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
        console.log('Validating language:', code, 'maxOne?:', maxOne); // Debug log
        
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
            console.log('Validation response:', response); // Debug log
            
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

    // Update romanization styles based on language
    async function updateRomanizationStyles(tag: string) {
        console.log('Updating romanization styles for:', tag);
        
        if (!tag?.trim()) {  // Better empty check
            console.log('No valid tag provided, disabling romanization');
            romanizationStyles = [];
            isRomanizationAvailable = false;
            if (selectedFeatures.subtitleRomanization) {
                selectedFeatures.subtitleRomanization = false;
            }
            return;
        }

        try {
            const styles = await GetRomanizationStyles(tag);
            console.log('Received romanization styles:', styles);
            romanizationStyles = styles;
            isRomanizationAvailable = styles && styles.length > 0;
            
            if (!isRomanizationAvailable && selectedFeatures.subtitleRomanization) {
                selectedFeatures.subtitleRomanization = false;
            }
        } catch (error) {
            console.error('Error fetching romanization styles:', error);
            romanizationStyles = [];
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

    // Reactive statements
    $: anyFeatureSelected = Object.values(selectedFeatures).some(v => v);

    $: if (languageCode !== undefined) {
        validateLanguageTag(languageCode, true);
        // Force update of romanization availability
        if (!languageCode) {
            isRomanizationAvailable = false;
            selectedFeatures.subtitleRomanization = false;
        }
    }
    
    $: {
        const hasFeatures = Object.values(selectedFeatures).some(v => v);
        const isLanguageValid = isValidLanguage === true;
        
        // Dispatch an event to notify parent about validity
        dispatch('validityChange', {
            isValid: hasFeatures && isLanguageValid
        });
    }

    // Settings subscription
    settings.subscribe(value => {
        console.log('Settings updated:', value); // Debug log
        if (value?.targetLanguage && value.targetLanguage !== languageCode) {
            console.log('Updating language code from settings:', value.targetLanguage);
            languageCode = value.targetLanguage;
            validateLanguageTag(value.targetLanguage, true);
        }
    });

    onMount(async () => {
        const currentSettings = get(settings);
        if (currentSettings?.targetLanguage) {
            languageCode = currentSettings.targetLanguage;
            await validateLanguageTag(currentSettings.targetLanguage, true);
        }
    });

    // Dispatch options changes
    $: {
        dispatch('optionsChange', currentFeatureOptions);
    }

    // Helper function for text color classes
    function getTextColorClass(enabled: boolean, anyFeatureSelected: boolean): string {
        if (enabled) return 'text-white';
        if (!anyFeatureSelected) return 'text-white';
        return 'text-white/70';
    }
</script>

<div class="space-y-6">
    <div class="flex items-center justify-between pl-8 pr-4">
        <h2 class="text-xl font-medium text-accent/90 flex items-center gap-2">
            <span class="material-icons text-accent/70">tune</span>
            Select Features
        </h2>
        
        <div class="relative flex items-center gap-3">
            <span class="text-sm text-accent/70 font-medium">Target Language</span>
            <div class="relative">
                <input
                    type="text"
                    bind:value={languageCode}
                    maxlength="9"
                    placeholder="e.g. ja, zh-Hans"
                    class="w-32 bg-white/5 border border-accent/30 rounded px-3 py-2
                           text-sm font-medium
                           focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/50
                           transition-all duration-200
                           {isValidLanguage === false ? 'border-red-500' : ''}"
                />
                {#if isChecking}
                    <span class="absolute right-3 top-1/2 -translate-y-1/2
                                material-icons animate-spin text-accent/70 text-sm">
                        refresh
                    </span>
                {:else if isValidLanguage === false}
                    <span class="absolute right-3 top-1/2 -translate-y-1/2
                                material-icons text-red-500 text-sm"
                          title={validationError}>
                        error
                    </span>
                {:else if isValidLanguage === true}
                    <span class="absolute right-3 top-1/2 -translate-y-1/2
                                material-icons text-green-500 text-sm">
                        check_circle
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
                        {#if !languageCode}
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
                                            step={option.includes('Boost') ? '0.1' : '1'}
                                            bind:value={currentFeatureOptions[feature][option]}
                                            class="w-full bg-sky-dark/50 border border-accent/30 rounded px-3 py-1
                                                   focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
                                                   transition-colors duration-200 text-sm
                                                   font-medium" 
                                        />
                                    {:else if feature === 'subtitleRomanization' && option === 'style'}
                                        <Dropdown
                                            options={romanizationStyles}
                                            value={currentFeatureOptions[feature][option]}
                                            on:change={(e) => handleDropdownChange(feature, option, e.detail)}
                                            label="Select style"
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