<script lang="ts">
    import { onMount } from 'svelte';
    import { slide, fade } from 'svelte/transition';
    import { settings, showSettings } from '../lib/stores.ts';
    import { ValidateLanguageTag } from '../../wailsjs/go/gui/App';

    export let onClose: () => void;

    interface LanguageCheckResponse {
        isValid: boolean;
        error?: string;
    }

    let currentSettings = {
        apiKeys: {
            replicate: '',
            assemblyAI: '',
            elevenLabs: ''
        },
        targetLanguage: '',
        nativeLanguages: '',
        enableGlow: true
    };

    let targetLangValid = false;
    let nativeLangValid = false;
    let targetLangError = '';
    let nativeLangError = '';
    let isValid = true;

    async function validateLanguages() {
        // Validate target language
        if (currentSettings.targetLanguage) {
            const targetResponse = await ValidateLanguageTag(currentSettings.targetLanguage, true);
            targetLangValid = targetResponse.isValid;
            targetLangError = targetResponse.error || '';
        } else {
            targetLangValid = false;
            targetLangError = '';
        }

        // Validate native languages
        if (currentSettings.nativeLanguages) {
            const nativeResponse = await ValidateLanguageTag(currentSettings.nativeLanguages, false);
            nativeLangValid = nativeResponse.isValid;
            nativeLangError = nativeResponse.error || '';
        } else {
            nativeLangValid = false;
            nativeLangError = '';
        }

        // Update overall validity
        isValid = (!currentSettings.targetLanguage || targetLangValid) &&
                 (!currentSettings.nativeLanguages || nativeLangValid);
    }

    async function saveSettings() {
        await validateLanguages();
        if (!isValid) return;
        
        try {
            await window.go.gui.App.SaveSettings(currentSettings);
            settings.set(currentSettings);
            onClose();
            window.dispatchEvent(new CustomEvent('settingsUpdated', { 
                detail: currentSettings 
            }));
        } catch (error) {
            console.error('Failed to save settings:', error);
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

    // Watch for changes in language inputs
    $: {
        if (currentSettings.targetLanguage !== undefined || 
            currentSettings.nativeLanguages !== undefined) {
            validateLanguages();
        }
    }

    // Subscribe to settings store
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
    <div class="fixed inset-0 bg-black/50 backdrop-blur-sm z-50 overflow-y-auto"
         transition:fade={{ duration: 200 }}
    >
        
        <div class="container mx-auto max-w-2xl p-4 min-h-screen flex items-center"
             transition:slide={{ duration: 300 }}
             on:click|stopPropagation>
            
            <div class="bg-[#252525] rounded-xl shadow-2xl border border-white/10 w-full">
                <!-- Header -->
                <div class="p-6 border-b border-white/10">
                    <div class="flex items-center justify-between">
                        <h2 class="text-xl font-medium text-accent/90 flex items-center gap-2">
                            <span class="material-icons text-accent/70">settings</span>
                            Settings
                        </h2>
                    <button 
                        class="w-10 h-10 flex items-center justify-center rounded-full
                               transition-colors duration-200 hover:bg-red-500/90"
                        on:click={onClose}
                    >
                        <span class="material-icons">close</span>
                    </button>
                    </div>
                </div>
                
                <!-- Content -->
                <div class="p-6 space-y-8 max-h-[calc(100vh-16rem)] overflow-y-auto">
                    <!-- Language Settings -->
                    <section class="space-y-6">
                        <h3 class="text-lg font-medium text-accent/80 flex items-center gap-2">
                            <span class="material-icons text-accent/70">translate</span>
                            Default Language Settings
                        </h3>
                        
                        <div class="grid grid-cols-2 gap-6">
                            <!-- Target Languages -->
                            <div class="space-y-2">
                                <label class="text-sm text-gray-300 font-medium">
                                    Target Language
                                </label>
                                <div class="relative">
                                   <input
                                        type="text"
                                        bind:value={currentSettings.targetLanguage}
                                        maxlength="9"
                                        class="w-full bg-sky-dark/50 border border-accent/30 rounded-lg px-3 py-2.5
                                               focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/50
                                               transition-all duration-200 placeholder:text-white/30"
                                        placeholder="e.g. es, yue or pt-BR"
                                    />
                                    {#if targetLangValid}
                                        <span class="absolute right-3 top-1/2 -translate-y-1/2
                                                   material-icons text-green-300 text-sm">
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
                                <label class="text-sm text-gray-300 font-medium">
                                    Native Language(s)
                                </label>
                                <div class="relative">
                                    <input
                                        type="text"
                                        bind:value={currentSettings.nativeLanguages}
                                        class="w-full bg-sky-dark/50 border border-accent/30 rounded-lg px-3 py-2.5
                                               focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/50
                                               transition-all duration-200 placeholder:text-white/30"
                                        placeholder="e.g. en, fr, es"
                                    />
                                    {#if nativeLangValid}
                                        <span class="absolute right-3 top-1/2 -translate-y-1/2
                                                   material-icons text-green-300 text-sm">
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
                    <section class="space-y-6">
                        <h3 class="text-lg font-medium text-accent/80 flex items-center gap-2">
                            <span class="material-icons text-accent/70">vpn_key</span>
                            API Keys
                        </h3>
                        <div class="space-y-4">
                            <div class="relative">
                                <input
                                    type="password"
                                    bind:value={currentSettings.apiKeys.replicate}
                                    class="w-full bg-sky-dark/50 border border-accent/30 rounded-lg pl-[156px] pr-3 py-2
                                           focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/50
                                           transition-all duration-200"
                                />
                                <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                             w-[140px] bg-accent/10 border-r border-accent/30 rounded-l-lg
                                             text-sm text-accent/90 font-medium">
                                    Replicate
                                </span>
                            </div>
                            <div class="relative">
                                <input
                                    type="password"
                                    bind:value={currentSettings.apiKeys.assemblyAI}
                                    class="w-full bg-sky-dark/50 border border-accent/30 rounded-lg pl-[156px] pr-3 py-2
                                           focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/50
                                           transition-all duration-200"
                                />
                                <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                             w-[140px] bg-accent/10 border-r border-accent/30 rounded-l-lg
                                             text-sm text-accent/90 font-medium">
                                    Assembly AI
                                </span>
                            </div>
                            <div class="relative">
                                <input
                                    type="password"
                                    bind:value={currentSettings.apiKeys.elevenLabs}
                                    class="w-full bg-sky-dark/50 border border-accent/30 rounded-lg pl-[156px] pr-3 py-2
                                           focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/50
                                           transition-all duration-200"
                                />
                                <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                             w-[140px] bg-accent/10 border-r border-accent/30 rounded-l-lg
                                             text-sm text-accent/90 font-medium">
                                    Eleven Labs
                                </span>
                            </div>
                        </div>
                    </section>

                    <!-- UI Settings -->
                    <section class="space-y-6">
                        <h3 class="text-lg font-medium text-accent/80 flex items-center gap-2">
                            <span class="material-icons text-accent/70">palette</span>
                            Interface Settings
                        </h3>
                        <div class="space-y-4">
                            <label class="flex items-center gap-3 cursor-pointer group">
                                <input
                                    type="checkbox"
                                    bind:checked={currentSettings.enableGlow}
                                    class="w-4 h-4 accent-accent rounded"
                                />
                                <span class="text-sm text-gray-300 group-hover:text-white transition-colors">
                                    Enable glow effects (disable if you experience performance issues)
                                </span>
                            </label>
                            
                            <label class="flex items-center gap-3 cursor-pointer group">
                                <input
                                    type="checkbox"
                                    bind:checked={currentSettings.showLogViewerByDefault}
                                    class="w-4 h-4 accent-accent rounded"
                                />
                                <span class="text-sm text-gray-300 group-hover:text-white transition-colors">
                                    Show log viewer by default
                                </span>
                            </label>

                            <div class="space-y-2">
                                <label class="text-sm text-left block text-gray-300">Maximum log entries:</label>
                                <input
                                    type="number"
                                    bind:value={currentSettings.maxLogEntries}
                                    min="100"
                                    max="10000"
                                    step="100"
                                    class="w-full bg-sky-dark/50 border border-accent/30 rounded-lg px-3 py-2
                                           focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent/50
                                           transition-all duration-200"
                                />
                            </div>
                        </div>
                    </section>
                </div>
                
                <!-- Footer -->
                <div class="p-6 border-t border-white/10 flex justify-end gap-3">
                    <button
                        class="px-4 py-2 text-white/70 hover:text-white hover:bg-red-500/90
                               transition-colors duration-200 rounded-lg transition-all duration-200"
                        on:click={onClose}
                    >
                        Cancel
                    </button>
                    <button
                        class="px-6 py-2 bg-accent text-sky-dark rounded-lg font-medium
                               transition-all duration-200 hover:bg-opacity-80
                               disabled:opacity-50 disabled:cursor-not-allowed"
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
    /* Add smooth scrolling */
    .overflow-y-auto {
        scrollbar-gutter: stable;
        scroll-behavior: smooth;
    }

    /* Customize scrollbar */
    .overflow-y-auto::-webkit-scrollbar {
        width: 8px;
    }

    .overflow-y-auto::-webkit-scrollbar-track {
        background: transparent;
    }

    .overflow-y-auto::-webkit-scrollbar-thumb {
        background-color: rgba(255, 255, 255, 0.1);
        border-radius: 20px;
        border: 3px solid transparent;
        background-clip: content-box;
    }

    .overflow-y-auto::-webkit-scrollbar-thumb:hover {
        background-color: rgba(255, 255, 255, 0.2);
    }
</style>