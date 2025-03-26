<script lang="ts">
    import { onMount } from 'svelte';
    import { slide, fade } from 'svelte/transition';
    import { settings, showSettings } from '../lib/stores';
    import { ValidateLanguageTag } from '../../wailsjs/go/gui/App';
    import { ExportDebugReport } from '../../wailsjs/go/gui/App';
    
    import TextInput from './TextInput.svelte';
    import NumericInput from './NumericInput.svelte';

    export let onClose: () => void;

    interface LanguageCheckResponse {
        isValid: boolean;
        error?: string;
    }

    let currentSettings = {
        apiKeys: {
            replicate: '',
            assemblyAI: '',
            elevenLabs: '',
            openAI: ''
        },
        targetLanguage: '',
        nativeLanguages: '',
        enableGlow: true,
        showLogViewerByDefault: false,
        maxAPIRetries: 10,
        maxLogEntries: 10000,
        maxWorkers: 1
    };

    let targetLangValid = false;
    let nativeLangValid = false;
    let targetLangError = '';
    let nativeLangError = '';
    let isValid = true;

    // Local state for debug-export UI
    let isExportingDebug = false;
    let exportSuccess = false;
    let exportError = '';

    /*
      We define a reactive variable "exportGlowClass" that is set to:
        - "glow-success" if export succeeded,
        - "glow-error" if export failed,
        - "glow-default" otherwise.
    */
    $: exportGlowClass = exportSuccess
        ? 'glow-success'
        : exportError
            ? 'glow-error'
            : 'glow-default';

    async function validateLanguages() {
        if (currentSettings.targetLanguage) {
            const targetResponse = await ValidateLanguageTag(currentSettings.targetLanguage, true);
            targetLangValid = targetResponse.isValid;
            targetLangError = targetResponse.error || '';
        } else {
            targetLangValid = false;
            targetLangError = '';
        }

        if (currentSettings.nativeLanguages) {
            const nativeResponse = await ValidateLanguageTag(currentSettings.nativeLanguages, false);
            nativeLangValid = nativeResponse.isValid;
            nativeLangError = nativeResponse.error || '';
        } else {
            nativeLangValid = false;
            nativeLangError = '';
        }

        isValid = (!currentSettings.targetLanguage || targetLangValid) &&
                  (!currentSettings.nativeLanguages || nativeLangValid);
    }

    async function saveSettings() {
        await validateLanguages();
        if (!isValid) return;
        try {
            await window.go.gui.App.SaveSettings(currentSettings);
            
            // Trigger STT model refresh after API key changes
            try {
                // Explicitly request a refresh of STT models with new API keys
                await window.go.gui.App.RefreshSTTModelsAfterSettingsUpdate();
            } catch (error) {
                console.error('Failed to refresh STT models:', error);
            }
            
            settings.set(currentSettings);
            onClose();
            window.dispatchEvent(new CustomEvent('settingsUpdated', {
                detail: currentSettings
            }));
        } catch (error) {
            console.error('Failed to save settings:', error);
        }
    }

    async function exportDebugReport() {
        isExportingDebug = true;
        exportSuccess = false;
        exportError = '';
        try {
            await ExportDebugReport();
            exportSuccess = true;
        } catch (err) {
            console.error('Failed to export debug report:', err);
            exportError = err?.message || 'Unknown error occurred.';
        } finally {
            isExportingDebug = false;
        }
    }

    onMount(async () => {
        try {
            const loadedSettings = await window.go.gui.App.LoadSettings();
            settings.set(loadedSettings);
            currentSettings = {
                ...loadedSettings,
                targetLanguage: loadedSettings.targetLanguage || '',
                nativeLanguages: loadedSettings.nativeLanguages || ''
            };
            await validateLanguages();
        } catch (error) {
            console.error('Failed to load settings:', error);
        }
    });

    $: {
        if (currentSettings.targetLanguage !== undefined ||
            currentSettings.nativeLanguages !== undefined) {
            validateLanguages();
        }
    }

    settings.subscribe(value => {
        if (value) {
            currentSettings = {
                ...value,
                targetLanguage: value.targetLanguage || currentSettings.targetLanguage || '',
                nativeLanguages: value.nativeLanguages || currentSettings.nativeLanguages || ''
            };
        }
    });
</script>

{#if $showSettings}
    <div class="fixed inset-0 bg-bg/40 backdrop-blur-md z-50 overflow-y-auto"
         transition:fade={{ duration: 200 }}>
        <div class="container mx-auto max-w-2xl p-4 min-h-screen flex items-center"
             transition:slide={{ duration: 300 }}
             on:click|stopPropagation>
            <div class="bg-bg-800/20 backdrop-blur-xl rounded-xl shadow-2xl border border-primary/20 w-full 
                        shadow-settings">
                <!-- Header -->
                <div class="p-6 border-b border-primary/20">
                    <div class="flex items-center justify-between">
                        <h2 class="text-xl font-medium text-white flex items-center gap-2">
                            <span class="material-icons text-primary">settings</span>
                            Settings
                        </h2>
                        <button class="w-10 h-10 flex items-center justify-center rounded-full
                                border-0 hover:border-0 bg-transparent text-gray-300 transition-colors duration-200
                                hover:text-red-500 hover:scale-125 hover:font-bold focus:outline-none"
                                on:click={onClose}>
                            <span class="material-icons">close</span>
                        </button>
                    </div>
                </div>
                
                <!-- Content -->
                <div class="p-6 space-y-8 max-h-[calc(100vh-16rem)] overflow-y-auto settings-content">
                    <!-- Language Settings -->
                    <section class="space-y-6 settings-section">
                        <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                            <span class="material-icons text-primary">translate</span>
                            Default Language Settings
                        </h3>
                        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                            <!-- Target Language -->
                            <div class="space-y-2">
                                <label class="text-sm text-gray-200 font-medium">
                                    Target Language
                                </label>
                                <div class="relative">
                                    <TextInput
                                        bind:value={currentSettings.targetLanguage}
                                        maxLength={9}
                                        placeholder="e.g. es, yue or pt-BR"
                                        className="px-3 py-2.5 hover:border-primary/55 hover:shadow-input
                                                  focus:border-primary focus:ring-1 focus:shadow-input-focus
                                                  focus:ring-primary/50 placeholder:text-white/30 pr-10 
                                                  bg-black/20 backdrop-blur-sm border-primary/40"
                                    />
                                    {#if targetLangValid}
                                        <span class="absolute right-3 top-1/2 -translate-y-1/2
                                                     material-icons text-pale-green text-sm">
                                            check_circle
                                        </span>
                                    {:else if targetLangError}
                                        <span class="absolute right-3 top-1/2 -translate-y-1/2
                                                     material-icons text-red-500 text-sm"
                                              title={targetLangError}>
                                            error
                                        </span>
                                    {/if}
                                </div>
                            </div>

                            <!-- Native Languages -->
                            <div class="space-y-2">
                                <label class="text-sm text-gray-200 font-medium">
                                    Native Language(s)
                                </label>
                                <div class="relative">
                                    <TextInput
                                        bind:value={currentSettings.nativeLanguages}
                                        placeholder="e.g. en, fr, es"
                                        className="px-3 py-2.5 hover:border-primary/55 hover:shadow-input
                                                  focus:border-primary focus:ring-1 focus:shadow-input-focus
                                                  focus:ring-primary/50 placeholder:text-white/30 pr-10 
                                                  bg-black/20 backdrop-blur-sm border-primary/40"
                                    />
                                    {#if nativeLangValid}
                                        <span class="absolute right-3 top-1/2 -translate-y-1/2
                                                     material-icons text-pale-green text-sm">
                                            check_circle
                                        </span>
                                    {:else if nativeLangError}
                                        <span class="absolute right-3 top-1/2 -translate-y-1/2
                                                     material-icons text-red-500 text-sm"
                                              title={nativeLangError}>
                                            error
                                        </span>
                                    {/if}
                                </div>
                            </div>
                        </div>
                    </section>

                    <!-- API Settings -->
                    <section class="space-y-6 settings-section">
                        <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                            <span class="material-icons text-primary">vpn_key</span>
                            API Keys
                        </h3>
                        <div class="space-y-4">
                            <div class="relative glass-input-container">
                                <input
                                    type="password"
                                    bind:value={currentSettings.apiKeys.replicate}
                                    class="w-full bg-black/20 backdrop-blur-sm border border-primary/40 rounded-lg pl-[156px] pr-3 py-2
                                           hover:border-primary/55 hover:shadow-input tracking-wider text-lg
                                           focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                           focus:shadow-input-focus transition-all duration-200"
                                />
                                <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                             w-[140px] bg-primary/10 border-r border-primary/30 rounded-l-lg
                                             text-sm text-primary font-medium">
                                    Replicate
                                </span>
                            </div>
                            <div class="relative glass-input-container">
                                <input
                                    type="password"
                                    bind:value={currentSettings.apiKeys.assemblyAI}
                                    class="w-full bg-black/20 backdrop-blur-sm border border-primary/40 rounded-lg pl-[156px] pr-3 py-2
                                           hover:border-primary/55 hover:shadow-input tracking-wider text-lg
                                           focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                           focus:shadow-input-focus transition-all duration-200"
                                />
                                <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                             w-[140px] bg-primary/10 border-r border-primary/30 rounded-l-lg
                                             text-sm text-primary font-medium">
                                    Assembly AI
                                </span>
                            </div>
                            <div class="relative glass-input-container">
                                <input
                                    type="password"
                                    bind:value={currentSettings.apiKeys.elevenLabs}
                                    class="w-full bg-black/20 backdrop-blur-sm border border-primary/40 rounded-lg pl-[156px] pr-3 py-2
                                           hover:border-primary/55 hover:shadow-input tracking-wider text-lg
                                           focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                           focus:shadow-input-focus transition-all duration-200"
                                />
                                <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                             w-[140px] bg-primary/10 border-r border-primary/30 rounded-l-lg
                                             text-sm text-primary font-medium">
                                    Eleven Labs
                                </span>
                            </div>
                             <div class="relative glass-input-container">
                                 <input
                                     type="password"
                                     bind:value={currentSettings.apiKeys.openAI}
                                     class="w-full bg-black/20 backdrop-blur-sm border border-primary/40 rounded-lg pl-[156px] pr-3 py-2
                                            hover:border-primary/55 hover:shadow-input tracking-wider text-lg
                                            focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                            focus:shadow-input-focus transition-all duration-200"
                                 />
                                 <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                              w-[140px] bg-primary/10 border-r border-primary/30 rounded-l-lg
                                              text-sm text-primary font-medium">
                                     OpenAI
                                 </span>
                             </div>
                        </div>
                        <div class="flex items-center gap-4">
                            <label class="text-sm text-gray-200 whitespace-nowrap">
                                Maximum API retries:
                            </label>
                            <NumericInput
                                bind:value={currentSettings.maxAPIRetries}
                                min={1}
                                step={1}
                                className="w-32 px-3 py-2 pl-4 hover:border-primary/55
                                           hover:shadow-input focus:shadow-input-focus
                                           focus:border-primary focus:ring-1
                                           focus:ring-primary/50 transition-all
                                           duration-200 bg-black/20 backdrop-blur-sm border-primary/40"
                            />
                        </div>
                    </section>

                    <!-- Worker Pool Settings -->
                    <section class="space-y-6 settings-section">
                        <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                            <span class="material-icons text-primary">speed</span>
                            Worker Pool Settings
                        </h3>
                        <div class="flex items-center gap-4">
                            <label class="text-sm text-gray-200 whitespace-nowrap">
                                Maximum Workers:
                            </label>
                            <NumericInput
                                bind:value={currentSettings.maxWorkers}
                                min={1}
                                step={1}
                                className="w-32 px-3 py-2 pl-4 hover:border-primary/55
                                           hover:shadow-input focus:shadow-input-focus
                                           focus:border-primary focus:ring-1
                                           focus:ring-primary/50 transition-all
                                           duration-200 bg-black/20 backdrop-blur-sm border-primary/40"
                            />
                        </div>
                    </section>

                    <!-- UI Settings -->
                    <section class="space-y-6 settings-section">
                        <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                            <span class="material-icons text-primary">palette</span>
                            Interface Settings
                        </h3>
                        <div class="space-y-4">
                            <label class="flex items-center gap-3 cursor-pointer group checkbox-container">
                                <input
                                    type="checkbox"
                                    bind:checked={currentSettings.enableGlow}
                                    class="w-4 h-4 accent-primary rounded custom-checkbox"
                                />
                                <span class="text-sm text-gray-200 group-hover:text-white transition-colors">
                                    Enable glow effects (disable if you experience performance issues)
                                </span>
                            </label>
                            <label class="flex items-center gap-3 cursor-pointer group checkbox-container">
                                <input
                                    type="checkbox"
                                    bind:checked={currentSettings.showLogViewerByDefault}
                                    class="w-4 h-4 accent-primary rounded custom-checkbox"
                                />
                                <span class="text-sm text-gray-200 group-hover:text-white transition-colors">
                                    Show log viewer by default
                                </span>
                            </label>
                            <div class="flex items-center gap-4">
                                <label class="text-sm text-gray-200 whitespace-nowrap">
                                    Maximum log entries:
                                </label>
                                <NumericInput
                                    bind:value={currentSettings.maxLogEntries}
                                    min={100}
                                    step={100}
                                    className="w-32 px-3 py-2 pl-4 hover:border-primary/55
                                               hover:shadow-input focus:shadow-input-focus
                                               focus:border-primary focus:ring-1
                                               focus:ring-primary/50 transition-all
                                               duration-200 bg-black/20 backdrop-blur-sm border-primary/40"
                                />
                            </div>
                        </div>
                    </section>

                    <!-- Diagnostic / Debug Export Section -->
                    <section class="space-y-6 settings-section">
                        <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                            <span class="material-icons text-primary">bug_report</span>
                            Diagnostic Tools
                        </h3>
                        <div class="space-y-4">
                            <div class="flex items-center gap-4">
                                <button
                                    class="px-6 py-3 text-white/90 rounded-lg font-semibold bg-bg/70 backdrop-blur-sm 
                                           border-[2.5px] border-primary/80 transition-all duration-200 focus:outline-none 
                                           hover:shadow-input hover:border-primary focus:shadow-input-focus
                                           debug-export-button"
                                    on:click={exportDebugReport}
                                    disabled={isExportingDebug}
                                    class:glow-success={exportGlowClass==='glow-success'}
                                    class:glow-error={exportGlowClass==='glow-error'}
                                >
                                    Export Debug Report
                                </button>
                                {#if isExportingDebug}
                                    <span class="inline-flex items-center gap-1 text-gray-200 text-sm">
                                        <span class="material-icons animate-spin">autorenew</span>
                                        Creating debug report, please wait...
                                    </span>
                                {:else if exportSuccess}
                                    <span class="inline-flex items-center gap-1 text-green-400 text-sm">
                                        <span class="material-icons">check_circle</span>
                                        Debug report successfully saved!
                                    </span>
                                {:else if exportError}
                                    <span class="inline-flex items-center gap-1 text-red-400 text-sm">
                                        <span class="material-icons">error</span>
                                        {exportError}
                                    </span>
                                {/if}
                            </div>
                        </div>
                    </section>
                </div>

                <!-- Footer -->
                <div class="p-6 border-t border-primary/20 flex justify-end gap-3">
                    <button
                        class="px-4 py-2 backdrop-blur-sm bg-black/30 border border-primary/30 text-white/80 
                              hover:text-white hover:bg-red-500/60 hover:border-red-500/80 transition-all duration-200 rounded-lg"
                        on:click={onClose}
                    >
                        Cancel
                    </button>
                    <button
                        class="px-6 py-2 bg-primary/90 backdrop-blur-sm text-white rounded-lg font-medium 
                              transition-all duration-200 hover:bg-primary disabled:opacity-50 
                              disabled:cursor-not-allowed shadow-md shadow-primary/30"
                        on:click={saveSettings}
                        disabled={!isValid}
                    >
                        Save Changes
                    </button>
                </div>
            </div>
        </div>
    </div>
{/if}

<style>
    /* Glassmorphic effect mixins */
    .glass-input-container input:focus {
        backdrop-filter: blur(8px);
    }
    
    /* Smooth scrolling for overflow */
    .overflow-y-auto {
        scrollbar-gutter: stable;
        scroll-behavior: smooth;
    }
    .overflow-y-auto::-webkit-scrollbar {
        width: 6px;
    }
    .overflow-y-auto::-webkit-scrollbar-track {
        background: transparent;
    }
    .overflow-y-auto::-webkit-scrollbar-thumb {
        background-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.5);
        border-radius: 20px;
    }
    .overflow-y-auto::-webkit-scrollbar-thumb:hover {
        background-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.7);
    }

    /* Settings panel drop shadow */
    .shadow-settings {
        box-shadow: 
            0 10px 50px 0 rgba(0, 0, 0, 0.5),
            0 0 20px 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.2);
    }
    
    /* Enhanced hover states for inputs */
    :global(.shadow-input) {
        box-shadow: 
            0 0 8px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.2),
            inset 0 0 3px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.1);
    }
    
    :global(.shadow-input-focus) {
        box-shadow: 
            0 0 12px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4),
            inset 0 0 5px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.2);
    }

    /* Add subtle hover effect to section headers */
    .settings-heading {
        transition: text-shadow 0.2s ease;
        position: relative;
    }

    .settings-heading:hover {
        text-shadow: 0 0 10px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.7);
    }
    
    .settings-heading:hover .material-icons {
        text-shadow: 0 0 15px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.9);
    }
    /* Style checkboxes */
    .checkbox-container:hover .custom-checkbox {
        box-shadow: 0 0 8px 2px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.3);
    }
    
    /* Debug export button with permanent border */
    .debug-export-button {
        box-shadow: 0 0 5px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.2);
    }

    /*
      Glow classes:
      - .glow-success: animated green glow gradient
      - .glow-error: animated red glow gradient
    */

    @keyframes glowSuccess {
        0% {
            box-shadow: 
                0 0 5px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.2);
            border-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4);
        }
        100% {
            border-color: #22c55e;
            box-shadow:
                0 0 8px 2px rgba(34, 197, 94, 0.4),
                0 0 16px 4px rgba(34, 197, 94, 0.2),
                0 0 24px 6px rgba(34, 197, 94, 0.15);
        }
    }

    :global(.glow-success) {
        animation: glowSuccess 1s forwards;
    }

    @keyframes glowError {
        0% {
            box-shadow: 
                0 0 5px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.2);
            border-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4);
        }
        100% {
            border-color: #ef4444;
            box-shadow:
                0 0 8px 2px rgba(239, 68, 68, 0.4),
                0 0 16px 4px rgba(239, 68, 68, 0.2),
                0 0 24px 6px rgba(239, 68, 68, 0.1);
        }
    }

    :global(.glow-error) {
        animation: glowError 1s forwards;
    }
</style>