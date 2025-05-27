// Feature configuration model to centralize all feature-related data

export interface FeatureOption {
    type: 'number' | 'boolean' | 'string' | 'dropdown' | 'romanizationDropdown' | 'provider' | 'slider';
    label: string;
    default: any;
    min?: number;
    max?: number;
    step?: number | string;
    choices?: string[];
    hovertip?: string;
    placeholder?: string;
    showCondition?: string; // Expression to determine if this option should be shown
}

export interface FeatureDefinition {
    id: string;
    label: string;
    options: Record<string, FeatureOption>;
    optionOrder?: string[];
    requiresToken?: string[];
    requiresDocker?: boolean;
    requiresScraper?: boolean;
    requiresLanguage?: boolean;
    availableLanguages?: string[]; // Languages where this feature is available
    providerGroup?: string; // Used to group features sharing the same provider
    outputMergeGroup?: string; // Used to group features that contribute to the merged output
    showMergeBanner?: boolean; // Whether to show the merge banner for this feature
    dependentFeature?: string; // ID of the feature this feature depends on (e.g., dubtitles for subtitle processing)
    dependencyMessage?: string; // Message to display when a feature depends on another
    showCondition?: string; // Expression to determine if this feature should be shown
    
    featureGroups?: string[]; // IDs of feature groups this feature belongs to
    groupSharedOptions?: Record<string, string[]>; // Group ID -> Option IDs
}

export interface RomanizationScheme {
    name: string;
    description: string;
    provider: string;
}

export const providersRequiringTokens = {
    'whisper': 'replicate',
    'insanely-fast-whisper': 'replicate',
    'demucs': 'replicate',
    'spleeter': 'replicate'
};

export const providerGithubUrls = {
    'ichiran': 'https://github.com/tshatrov/ichiran',
    'aksharamukha': 'https://github.com/virtualvinodh/aksharamukha',
    'iuliia': 'https://github.com/mehanizm/iuliia-go',
};

// Define the features with their options
// Define common merge options for reference
const commonMergeOptions = {
    mergeOutputFiles: {
        type: 'boolean',
        label: 'Merge all processed outputs',
        default: false,
        hovertip: "When enabled, all processed outputs (dubtitles, enhanced audio, romanized subtitles, etc.) will be merged into a single video file."
    },
    mergingFormat: {
        type: 'dropdown',
        label: 'Merging Format',
        default: 'mp4',
        choices: ['mp4', 'mkv']
    }
};

export const features: FeatureDefinition[] = [
    {
        id: 'subs2cards',
        label: 'Subs2cards',
        options: {
            padTiming: {
                type: 'number',
                label: 'Padding (ms)',
                default: 250
            },
            screenshotWidth: {
                type: 'number',
                label: 'Screenshot Width (px)',
                default: 1280
            },
            screenshotHeight: {
                type: 'number',
                label: 'Screenshot Height (px)',
                default: 720
            }
        },
        dependentFeature: 'dubtitles',
        dependencyMessage: "Dubtitles will be used as a source for subs2cards when both features are enabled"
    },
    {
        id: 'condensedAudio',
        label: 'Make Condensed Audio',
        optionOrder: ['audioFormat', 'enableSummary', 'summaryProvider', 'summaryModel', 'summaryMaxLength', 'summaryTemperature', 'summaryCustomPrompt'],
        options: {
            audioFormat: {
                type: 'dropdown',
                label: 'Audio Format',
                default: 'MP3',
                choices: ['MP3', 'AAC', 'Opus']
            },
            enableSummary: {
                type: 'boolean',
                label: 'Generate Summary with LLM',
                default: false,
                hovertip: "Generate a summary of the media content and embed it in the audio metadata as lyrics.",
                showCondition: "feature.condensedAudio.audioFormat !== 'Opus'"
            },
            summaryProvider: {
                type: 'dropdown',
                label: 'Summary Provider',
                default: '',
                choices: [],  // Dynamically populated
                showCondition: "feature.condensedAudio.enableSummary === true && context.isLLMReady === true"
            },
            summaryModel: {
                type: 'dropdown',
                label: 'Summary Model',
                default: '',
                choices: [],  // Dynamically populated
                showCondition: "feature.condensedAudio.enableSummary === true && context.isLLMReady === true"
            },
            summaryMaxLength: {
                type: 'slider',
                label: 'Max Summary Length',
                default: 0,
                min: 0,
                max: 1000,
                step: 50,
                showCondition: "feature.condensedAudio.enableSummary === true && context.isLLMReady === true",
                hovertip: "Set to Auto (0) to let the LLM decide the length, or specify a maximum word count."
            },
            summaryTemperature: {
                type: 'slider',
                label: 'Summary Temperature',
                default: 0.7,
                min: 0.0,
                max: 2.0,
                step: 0.1,
                showCondition: "feature.condensedAudio.enableSummary === true && context.isLLMReady === true",
                hovertip: "Controls creativity: 0 = focused, 2 = creative"
            },
            summaryCustomPrompt: {
                type: 'string',
                label: 'Override(!!) Default Prompt',
                default: '',
                showCondition: "feature.condensedAudio.enableSummary === true && context.isLLMReady === true",
                hovertip: "IF PROVIDED, your custom prompt will be used directly with the subtitle content appended after your custom prompt.\n\nLangkit will not automatically add instructions for output language or input language hints if a custom prompt is used; include these in your custom prompt if needed.\n\nUse this option at your own risk!",
            }
        }
    },
    {
        id: 'dubtitles',
        label: 'Dubtitles',
        options: {
            padTiming: {
                type: 'number',
                label: 'Padding (ms)',
                default: 250
            },
            stt: {
                type: 'dropdown',
                label: 'Speech-To-Text',
                default: '', // 1st of slice returned by backend
                choices: []  // now dynamically provided by backend
            },
            sttTimeout: {
                type: 'number',
                label: 'Speech-To-Text Timeout (sec)',
                default: 90
            },
            initialPrompt: {
                type: 'string',
                label: 'Initial prompt for Whisper',
                default: '',
                hovertip: "Whisper works best when provided with an initial prompt containing exact names and terms from your audio.\n\n 🡆 List character names with correct spellings (e.g.,'Eren Yeager','Mikasa Ackerman'), unique terminology (e.g.,'ODM gear'), location names, recurring concepts that define the content's universe and any words the model might struggle with.\n 🡆 Limit your prompt to 30-50 key terms for optimal results. Prioritize words that appear frequently in your audio and those with unusual pronunciations or spellings.\n 🡆 Use comma separation rather than complete sentences. Avoid adding plot information or dialogue patterns - stick to names and terminology only.\n\nThe prompt should match the audio language.\nMaximum length is 224 tokens (approx. 850 characters).",
                placeholder: "e.g. Attack on Titan: Eren Yeager, Mikasa Ackerman, Armin Arlert, Titans, Colossal Titan, Armored Titan, Survey Corps, Wall Maria, Wall Rose, Wall Sina, ODM gear, Omni-directional mobility gear, Captain Levi, Commander Erwin Smith, Cadet Corps, Garrison Regiment, Military Police, Trost District, Shiganshina District, 3D Maneuver Gear, Sasha Blouse, Jean Kirstein, Connie Springer, Reiner Braun, Bertholdt Hoover, Annie Leonhart, Hange Zoë, Grisha Yeager, Carla Yeager, Cannons, blades, survey mission, beyond the walls, Scout Regiment, titan attack, breach, trainees, The 104th Cadet Corps",
                showCondition: "feature.dubtitles.stt === 'whisper'"
            },
            mergeOutputFiles: {
                type: 'boolean',
                label: 'Merge all processed outputs',
                default: false,
                hovertip: "When enabled, all processed outputs (dubtitles, enhanced audio, romanized subtitles, etc.) will be merged into a single video file.",
                showCondition: "context.isTopmostForOption"
            },
            mergingFormat: {
                type: 'dropdown',
                label: 'Merging Format',
                default: 'mp4',
                choices: ['mp4', 'mkv'],
                showCondition: "context.isTopmostForOption && featureGroupStore.getGroupOption('merge', 'mergeOutputFiles') === true"
            }
        },
        requiresToken: ['whisper', 'insanely-fast-whisper', 'universal-1'],
        outputMergeGroup: 'merge',
        showMergeBanner: true,
        featureGroups: ['merge'],
        groupSharedOptions: {
            'merge': ['mergeOutputFiles', 'mergingFormat']
        }
    },
    {
        id: 'voiceEnhancing',
        label: 'Voice Enhancing',
        options: {
            sepLib: {
                type: 'dropdown',
                label: 'Voice separation library',
                default: 'demucs',
                choices: ['demucs', 'demucs_ft', 'spleeter']
            },
            voiceBoost: {
                type: 'number',
                label: 'Voice Boost (dB)',
                default: 13,
                step: '0.1'
            },
            originalBoost: {
                type: 'number',
                label: 'Original Audio Boost (dB)',
                default: -9,
                step: '0.1'
            },
            limiter: {
                type: 'slider',
                label: 'Limiter (dBFS)',
                default: 0.9,
                min: 0.0625,
                max: 1,
                step: 0.0125
            },
            mergeOutputFiles: {
                type: 'boolean',
                label: 'Merge all processed outputs',
                default: false,
                hovertip: "When enabled, all processed outputs (dubtitles, enhanced audio, romanized subtitles, etc.) will be merged into a single video file.",
                showCondition: "context.isTopmostForOption"
            },
            mergingFormat: {
                type: 'dropdown',
                label: 'Merging Format',
                default: 'mp4',
                choices: ['mp4', 'mkv'],
                showCondition: "context.isTopmostForOption && featureGroupStore.getGroupOption('merge', 'mergeOutputFiles') === true"
            }
        },
        requiresToken: ['demucs', 'spleeter'],
        outputMergeGroup: 'merge',
        showMergeBanner: true,
        featureGroups: ['merge'],
        groupSharedOptions: {
            'merge': ['mergeOutputFiles', 'mergingFormat']
        }
    },
    {
        id: 'subtitleRomanization',
        label: 'Subtitle Romanization',
        optionOrder: ['style', 'provider', 'dockerRecreate', 'browserAccessURL', 'mergeOutputFiles', 'mergingFormat'],
        options: {
            style: {
                type: 'romanizationDropdown',
                label: 'Romanization Style',
                default: '',
                showCondition: "context.isTopmostForOption"
            },
            provider: {
                type: 'provider',
                label: 'Provider',
                default: '',
                showCondition: "context.isTopmostForOption && context.romanizationSchemes.length > 0"
            },
            dockerRecreate: {
                type: 'boolean',
                label: 'Recreate Docker containers',
                default: false,
                hovertip: "Use this if the previous run failed or if you're experiencing issues.",
                showCondition: "context.isTopmostForOption && context.needsDocker"
            },
            browserAccessURL: {
                type: 'string',
                label: 'Browser access URL',
                default: '',
                hovertip: "Optional URL to a Chromium-based browser's DevTools interface.\nIf not provided or invalid, a browser will be automatically downloaded and managed.\n\nTo get a URL: Run Chrome/Chromium with:\n --remote-debugging-port=9222 flag\nand use the WebSocket URL displayed in the terminal or in:\nchrome://inspect/#devices",
                placeholder: "e.g. ws://127.0.0.1:9222/devtools/browser/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
                showCondition: "context.isTopmostForOption && context.needsScraper"
            },
            mergeOutputFiles: {
                type: 'boolean',
                label: 'Merge all processed outputs',
                default: false,
                hovertip: "When enabled, all processed outputs (dubtitles, enhanced audio, romanized subtitles, etc.) will be merged into a single video file.",
                showCondition: "context.isTopmostForOption"
            },
            mergingFormat: {
                type: 'dropdown',
                label: 'Merging Format',
                default: 'mp4',
                choices: ['mp4', 'mkv'],
                showCondition: "context.isTopmostForOption && featureGroupStore.getGroupOption('merge', 'mergeOutputFiles') === true"
            }
        },
        requiresLanguage: true,
        requiresDocker: true,
        requiresScraper: true,
        providerGroup: 'subtitle',
        outputMergeGroup: 'merge',
        showMergeBanner: true,
        dependentFeature: 'dubtitles',
        dependencyMessage: "Dubtitles will be used as a source for romanization when both features are enabled",
        featureGroups: ['subtitle', 'merge'],
        groupSharedOptions: {
            'subtitle': ['style', 'provider', 'dockerRecreate', 'browserAccessURL'],
            'merge': ['mergeOutputFiles', 'mergingFormat']
        }
    },
    {
        id: 'selectiveTransliteration',
        label: 'Selective Transliteration',
        options: {
            provider: {
                type: 'provider',
                label: 'Provider',
                default: '',
                showCondition: "context.isTopmostForOption && context.romanizationSchemes.length > 0"
            },
            dockerRecreate: {
                type: 'boolean',
                label: 'Recreate Docker containers',
                default: false,
                hovertip: "Use this if the previous run failed or if you're experiencing issues.",
                showCondition: "context.isTopmostForOption && context.needsDocker"
            },
            browserAccessURL: {
                type: 'string',
                label: 'Browser access URL',
                default: '',
                hovertip: "URL to programmatically control a Chromium-based browser through Devtools.\nYou can get the URL from running Chromium from a terminal with --remote-debugging-port=9222 flag.\n\n 𝗥𝗲𝗾𝘂𝗶𝗿𝗲𝗱 𝗳𝗼𝗿 𝗽𝗿𝗼𝘃𝗶𝗱𝗲𝗿𝘀 𝘁𝗵𝗮𝘁 𝗻𝗲𝗲𝗱 𝘄𝗲𝗯 𝘀𝗰𝗿𝗮𝗽𝗶𝗻𝗴 𝗰𝗮𝗽𝗮𝗯𝗶𝗹𝗶𝘁𝗶𝗲𝘀.",
                placeholder: "e.g. ws://127.0.0.1:9222/devtools/browser/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
                showCondition: "context.isTopmostForOption && context.needsScraper"
            },
            tokenizeOutput: {
                type: 'boolean',
                label: 'Tokenize words',
                default: true,
                hovertip: "If enabled, the transliterated text will have spaces between words for easier reading."
            },
            kanjiFrequencyThreshold: {
                type: 'number',
                label: 'Retain Kanjis below most frequent',
                default: 100,
                min: 1,
                max: 3000,
                hovertip: "Set a threshold value so that high-frequency Kanji in subtitles are preserved while less common or irregular Kanjis are transliterated to hiragana.",
                placeholder: "Enter threshold (e.g., 100)",
                showCondition: "context.standardTag === 'jpn'"
            },
            mergeOutputFiles: {
                type: 'boolean',
                label: 'Merge all processed outputs',
                default: false,
                hovertip: "When enabled, all processed outputs (dubtitles, enhanced audio, romanized subtitles, etc.) will be merged into a single video file.",
                showCondition: "context.isTopmostForOption"
            },
            mergingFormat: {
                type: 'dropdown',
                label: 'Merging Format',
                default: 'mp4',
                choices: ['mp4', 'mkv'],
                showCondition: "context.isTopmostForOption && featureGroupStore.getGroupOption('merge', 'mergeOutputFiles') === true"
            }
        },
        optionOrder: ['style', 'provider', 'dockerRecreate', 'browserAccessURL', 'tokenizeOutput', 'kanjiFrequencyThreshold', 'mergeOutputFiles', 'mergingFormat'],
        requiresLanguage: true,
        availableLanguages: ['jpn'],
        providerGroup: 'subtitle',
        outputMergeGroup: 'merge',
        showMergeBanner: true,
        dependentFeature: 'dubtitles',
        dependencyMessage: "Dubtitles will be used as a source for selective transliteration when both features are enabled",
        featureGroups: ['subtitle', 'merge'],
        groupSharedOptions: {
            'subtitle': ['style', 'provider', 'dockerRecreate', 'browserAccessURL'],
            'merge': ['mergeOutputFiles', 'mergingFormat']
        },
        requiresDocker: true,
        requiresScraper: true
    },
    {
        id: 'subtitleTokenization',
        label: 'Subtitle Tokenization',
        options: {
            provider: {
                type: 'provider',
                label: 'Provider',
                default: '',
                showCondition: "context.isTopmostForOption && context.romanizationSchemes.length > 0"
            },
            dockerRecreate: {
                type: 'boolean',
                label: 'Recreate Docker containers',
                default: false,
                hovertip: "Use this if the previous run failed or if you're experiencing issues.",
                showCondition: "context.isTopmostForOption && context.needsDocker"
            },
            browserAccessURL: {
                type: 'string',
                label: 'Browser access URL',
                default: '',
                hovertip: "URL to programmatically control a Chromium-based browser through Devtools.\nYou can get the URL from running Chromium from a terminal with --remote-debugging-port=9222 flag.\n\n 𝗥𝗲𝗾𝘂𝗶𝗿𝗲𝗱 𝗳𝗼𝗿 𝗽𝗿𝗼𝘃𝗶𝗱𝗲𝗿𝘀 𝘁𝗵𝗮𝘁 𝗻𝗲𝗲𝗱 𝘄𝗲𝗯 𝘀𝗰𝗿𝗮𝗽𝗶𝗻𝗴 𝗰𝗮𝗽𝗮𝗯𝗶𝗹𝗶𝘁𝗶𝗲𝘀.",
                placeholder: "e.g. ws://127.0.0.1:9222/devtools/browser/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
                showCondition: "context.isTopmostForOption && context.needsScraper"
            },
            mergeOutputFiles: {
                type: 'boolean',
                label: 'Merge all processed outputs',
                default: false,
                hovertip: "When enabled, all processed outputs (dubtitles, enhanced audio, romanized subtitles, etc.) will be merged into a single video file.",
                showCondition: "context.isTopmostForOption"
            },
            mergingFormat: {
                type: 'dropdown',
                label: 'Merging Format',
                default: 'mp4',
                choices: ['mp4', 'mkv'],
                showCondition: "context.isTopmostForOption && featureGroupStore.getGroupOption('merge', 'mergeOutputFiles') === true"
            }
        },
        optionOrder: ['style', 'provider', 'dockerRecreate', 'browserAccessURL', 'mergeOutputFiles', 'mergingFormat'],
        requiresLanguage: true,
        requiresDocker: true,
        requiresScraper: true,
        providerGroup: 'subtitle',
        outputMergeGroup: 'merge',
        showMergeBanner: true,
        dependentFeature: 'dubtitles',
        dependencyMessage: "Dubtitles will be used as a source for tokenization when both features are enabled",
        featureGroups: ['subtitle', 'merge'],
        groupSharedOptions: {
            'subtitle': ['style', 'provider', 'dockerRecreate', 'browserAccessURL'],
            'merge': ['mergeOutputFiles', 'mergingFormat']
        }
    }
];

// Create default options object based on feature definitions
export function createDefaultOptions() {
    const options: Record<string, any> = {};
    
    features.forEach(feature => {
        options[feature.id] = {};
        
        Object.entries(feature.options).forEach(([optionId, option]) => {
            options[feature.id][optionId] = option.default;
        });
    });
    
    return options;
}

// STT Model interface matching the backend response
export interface STTModelInfo {
  name: string;
  displayName: string;
  description: string;
  providerName: string;
  isDepreciated: boolean;
  isRecommended: boolean;
  takesInitialPrompt: boolean;
  isAvailable: boolean;
}

export interface STTModelsResponse {
  models: STTModelInfo[];
  names: string[];
  available: boolean;
  suggested: string;
}

// Summary Provider and Model interfaces
export interface SummaryProviderInfo {
  name: string;
  displayName: string; 
  description: string;
}

export interface SummaryModelInfo {
  id: string;
  name: string;
  description: string;
  providerName: string;
}

export interface SummaryProvidersResponse {
  providers: SummaryProviderInfo[];
  names: string[];
  available: boolean;
  suggested: string;
}

export interface SummaryModelsResponse {
  models: SummaryModelInfo[];
  names: string[];
  available: boolean;
  suggested: string;
}

// Create stores for model information
import { writable } from 'svelte/store';
export const sttModelsStore = writable<STTModelsResponse>({
  models: [],
  names: [],
  available: false,
  suggested: ""
});

// Create stores for summary provider and model information
export const summaryProvidersStore = writable<SummaryProvidersResponse>({
  providers: [],
  names: [],
  available: false,
  suggested: ""
});

export const summaryModelsStore = writable<SummaryModelsResponse>({
  models: [],
  names: [],
  available: false,
  suggested: ""
});

// Create a combined store for provider+models to ensure reactivity
export const summarySelectionStore = writable({
  provider: "",
  model: "",
  providers: [] as string[],
  models: [] as string[],
  modelsMap: {} as Record<string, string[]> // Provider -> models mapping
});

// Function to update choices for a specific feature option
export function updateFeatureChoices(featureId: string, optionId: string, choices: string[], suggested?: string): void {
  const feature = features.find(f => f.id === featureId);
  if (feature && feature.options[optionId]) {
    feature.options[optionId].choices = choices;
    
    // Update default value if suggested is provided and the current default is not in choices
    if (suggested && !choices.includes(feature.options[optionId].default)) {
      feature.options[optionId].default = suggested;
    }
  }
}

// Function to update feature model with available STT models
export function updateSTTModels(sttModels: STTModelsResponse): void {
  // Always update the store with all models
  sttModelsStore.set(sttModels);
  
  // Always update choices for the STT dropdown with all models
  updateFeatureChoices('dubtitles', 'stt', sttModels.names, sttModels.suggested);
  
  // Also update the default value to always be the first model in the list
  const dubtitlesFeature = features.find(f => f.id === 'dubtitles');
  if (dubtitlesFeature && dubtitlesFeature.options.stt && sttModels.names.length > 0) {
    dubtitlesFeature.options.stt.default = sttModels.names[0];
  }
  
  // Update the initialPrompt condition as before
  if (dubtitlesFeature && dubtitlesFeature.options.initialPrompt) {
    dubtitlesFeature.options.initialPrompt.showCondition = 
      `(function() {
         const sttModel = feature.dubtitles.stt;
         const sttModels = ${JSON.stringify(sttModels.models)};
         const modelInfo = sttModels.find(m => m.name === sttModel);
         return modelInfo && modelInfo.takesInitialPrompt;
       })()`;
  }
}

// Update feature model with available summary providers
export function updateSummaryProviders(providers: SummaryProvidersResponse): void {
  // Always update the store with all providers
  summaryProvidersStore.set(providers);
  
  // Update choices for the summaryProvider dropdown
  updateFeatureChoices('condensedAudio', 'summaryProvider', providers.names, providers.suggested);
  
  // Update the default value to be the suggested provider or the first in the list
  const condensedAudioFeature = features.find(f => f.id === 'condensedAudio');
  if (condensedAudioFeature && condensedAudioFeature.options.summaryProvider && providers.names.length > 0) {
    condensedAudioFeature.options.summaryProvider.default = providers.suggested || providers.names[0];
    
    // Create a new reference for the choices array to ensure reactivity
    condensedAudioFeature.options.summaryProvider.choices = [...providers.names];
    
    // Force the options object to be seen as a new reference to trigger reactivity
    condensedAudioFeature.options = {...condensedAudioFeature.options};
  }
}

// Update feature model with available summary models for a provider
export function updateSummaryModels(models: SummaryModelsResponse, forProvider?: string): void {
  // Always update the store with all models
  summaryModelsStore.set(models);
  
  // Since we now handle the dropdown options update directly in FeatureSelector.svelte
  // based on the providerModelsMap, this function mainly just updates the store.
  
  // If we have a specific provider, we can log it for debugging
  if (forProvider) {
    import('./logger').then(({ logger }) => {
      logger.debug('featureModel', 'Updated models store for provider', { 
        provider: forProvider, 
        modelCount: models.names.length 
      });
    });
  }
  
  // Note: We no longer update condensedAudioFeature.options.summaryModel.choices here
  // because that's now handled in the FeatureSelector component with provider awareness
}

// Helper to format display text (camelCase to Title Case)
export function formatDisplayText(text: string): string {
    return text
        .replace(/([A-Z])/g, ' $1')
        .split(/(?=[A-Z])/)
        .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
        .join(' ');
}