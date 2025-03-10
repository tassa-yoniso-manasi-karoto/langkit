// Feature configuration model to centralize all feature-related data

export interface FeatureOption {
    type: 'number' | 'boolean' | 'string' | 'dropdown' | 'romanizationDropdown' | 'provider';
    label: string;
    default: any;
    min?: number;
    max?: number;
    step?: string;
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
    showCondition?: string; // Expression to determine if this feature should be shown
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
    'spleeter': 'replicate',
    'universal-1': 'assemblyAI'
};

export const providerGithubUrls = {
    'ichiran': 'https://github.com/tshatrov/ichiran',
    'aksharamukha': 'https://github.com/virtualvinodh/aksharamukha',
    'iuliia': 'https://github.com/mehanizm/iuliia-go',
};

// Define the features with their options
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
            },
            condensedAudio: {
                type: 'boolean',
                label: 'Condensed Audio',
                default: false
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
                default: 'whisper',
                choices: ['whisper', 'insanely-fast-whisper', 'universal-1']
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
                hovertip: "Whisper works best when provided with an initial prompt containing exact names and terms from your audio.\n\n 🡆 List character names with correct spellings (e.g.,'Eren Yeager','Mikasa Ackerman'), unique terminology (e.g.,'ODM gear'), location names, recurring concepts that define the content's universe and any words the model might struggle with.\n 🡆 Limit your prompt to 30-50 key terms for optimal results. Prioritize words that appear frequently in your audio and those with unusual pronunciations or spellings.\n 🡆 Use comma separation rather than complete sentences. Avoid adding plot information or dialogue patterns - stick to names and terminology only.\n\n Maximum length is 224 tokens (approx. 850 characters).",
                placeholder: "e.g. Attack on Titan: Eren Yeager, Mikasa Ackerman, Armin Arlert, Titans, Colossal Titan, Armored Titan, Survey Corps, Wall Maria, Wall Rose, Wall Sina, ODM gear, Omni-directional mobility gear, Captain Levi, Commander Erwin Smith, Cadet Corps, Garrison Regiment, Military Police, Trost District, Shiganshina District, 3D Maneuver Gear, Sasha Blouse, Jean Kirstein, Connie Springer, Reiner Braun, Bertholdt Hoover, Annie Leonhart, Hange Zoë, Grisha Yeager, Carla Yeager, Cannons, blades, survey mission, beyond the walls, Scout Regiment, titan attack, breach, trainees, The 104th Cadet Corps",
                showCondition: "feature.dubtitles.stt === 'whisper'"
            }
        },
        requiresToken: ['whisper', 'insanely-fast-whisper', 'universal-1']
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
                type: 'number',
                label: 'Limiter (dBFS)',
                default: 0.9,
                min: 0.0625,
                max: 1,
                step: '0.0125'
            },
            mergingFormat: {
                type: 'dropdown',
                label: 'Merging Format',
                default: 'mp4',
                choices: ['mp4', 'mkv']
            }
        },
        requiresToken: ['demucs', 'spleeter']
    },
        {
        id: 'subtitleRomanization',
        label: 'Subtitle Romanization',
        optionOrder: ['style', 'provider', 'dockerRecreate', 'browserAccessURL'],
        options: {
            style: {
                type: 'romanizationDropdown',
                label: 'Romanization Style',
                default: ''
            },
            provider: {
                type: 'provider',
                label: 'Provider',
                default: '',
                showCondition: "context.romanizationSchemes.length > 0"
            },
            dockerRecreate: {
                type: 'boolean',
                label: 'Recreate Docker containers',
                default: false,
                hovertip: "Use this if the previous run failed or if you're experiencing issues.",
                showCondition: "context.needsDocker"
            },
            browserAccessURL: {
                type: 'string',
                label: 'Browser access URL',
                default: '',
                hovertip: "URL to programmatically control a Chromium-based browser through Devtools.\nYou can get the URL from running Chromium from a terminal with --remote-debugging-port=9222 flag.\n\n 𝗥𝗲𝗾𝘂𝗶𝗿𝗲𝗱 𝗳𝗼𝗿 𝗽𝗿𝗼𝘃𝗶𝗱𝗲𝗿𝘀 𝘁𝗵𝗮𝘁 𝗻𝗲𝗲𝗱 𝘄𝗲𝗯 𝘀𝗰𝗿𝗮𝗽𝗶𝗻𝗴 𝗰𝗮𝗽𝗮𝗯𝗶𝗹𝗶𝘁𝗶𝗲𝘀.",
                placeholder: "e.g. ws://127.0.0.1:9222/devtools/browser/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
                showCondition: "context.needsScraper"
            }
        },
        requiresLanguage: true,
        requiresDocker: true,
        requiresScraper: true,
        providerGroup: 'subtitle'
    },
    {
        id: 'selectiveTransliteration',
        label: 'Selective Transliteration',
        options: {
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
            }
        },
        requiresLanguage: true,
        availableLanguages: ['jpn'],
        providerGroup: 'subtitle'
    },
    {
        id: 'subtitleTokenization',
        label: 'Subtitle Tokenization',
        options: {
            provider: {
                type: 'provider',
                label: 'Provider',
                default: '',
                showCondition: "context.romanizationSchemes.length > 0 && !context.selectedFeatures.subtitleRomanization"
            },
            dockerRecreate: {
                type: 'boolean',
                label: 'Recreate Docker containers',
                default: false,
                hovertip: "Use this if the previous run failed or if you're experiencing issues.",
                showCondition: "context.needsDocker && !context.selectedFeatures.subtitleRomanization"
            },
            browserAccessURL: {
                type: 'string',
                label: 'Browser access URL',
                default: '',
                hovertip: "URL to programmatically control a Chromium-based browser through Devtools.\nYou can get the URL from running Chromium from a terminal with --remote-debugging-port=9222 flag.\n\n 𝗥𝗲𝗾𝘂𝗶𝗿𝗲𝗱 𝗳𝗼𝗿 𝗽𝗿𝗼𝘃𝗶𝗱𝗲𝗿𝘀 𝘁𝗵𝗮𝘁 𝗻𝗲𝗲𝗱 𝘄𝗲𝗯 𝘀𝗰𝗿𝗮𝗽𝗶𝗻𝗴 𝗰𝗮𝗽𝗮𝗯𝗶𝗹𝗶𝘁𝗶𝗲𝘀.",
                placeholder: "e.g. ws://127.0.0.1:9222/devtools/browser/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
                showCondition: "context.needsScraper && !context.selectedFeatures.subtitleRomanization"
            }
        },
        requiresLanguage: true,
        requiresDocker: true,
        requiresScraper: true,
        providerGroup: 'subtitle'
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

// Helper to format display text (camelCase to Title Case)
export function formatDisplayText(text: string): string {
    return text
        .replace(/([A-Z])/g, ' $1')
        .split(/(?=[A-Z])/)
        .map(word => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
        .join(' ');
}