<script lang="ts">
    import { onMount } from 'svelte';
    import { slide, fade } from 'svelte/transition';
    import { settings, showSettings } from '../lib/stores';
    import { ValidateLanguageTag } from '../../wailsjs/go/gui/App';
    import { ExportDebugReport } from '../../wailsjs/go/gui/App';

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
        enableGlow: true,
        showLogViewerByDefault: false,
        maxAPIRetries: 10,
        maxLogEntries: 10000
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
    <div class="fixed inset-0 bg-black/50 backdrop-blur-sm z-50 overflow-y-auto"
         transition:fade={{ duration: 200 }}>
        <div class="container mx-auto max-w-2xl p-4 min-h-screen flex items-center"
             transition:slide={{ duration: 300 }}
             on:click|stopPropagation>
            <div class="bg-[#252525] rounded-xl shadow-2xl border border-white/10 w-full">
                <!-- Header -->
                <div class="p-6 border-b border-white/10">
                    <div class="flex items-center justify-between">
                        <h2 class="text-xl font-medium text-primary/90 flex items-center gap-2">
                            <span class="material-icons text-primary/70">settings</span>
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
                <div class="p-6 space-y-8 max-h-[calc(100vh-16rem)] overflow-y-auto">
                    <!-- Language Settings -->
                    <section class="space-y-6">
                        <h3 class="text-lg font-medium text-primary/80 flex items-center gap-2">
                            <span class="material-icons text-primary/70">translate</span>
                            Default Language Settings
                        </h3>
                        <div class="grid grid-cols-2 gap-6">
                            <!-- Target Language -->
                            <div class="space-y-2">
                                <label class="text-sm text-gray-300 font-medium">
                                    Target Language
                                </label>
                                <div class="relative">
                                    <input
                                        type="text"
                                        bind:value={currentSettings.targetLanguage}
                                        maxlength="9"
                                        class="w-full bg-sky-dark/50 border border-primary/30 rounded-lg px-3 py-2.5
                                               hover:border-primary/55 hover:shadow-sm hover:shadow-primary/30
                                               focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                               transition-all duration-200 placeholder:text-white/30"
                                        placeholder="e.g. es, yue or pt-BR"
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
                                <label class="text-sm text-gray-300 font-medium">
                                    Native Language(s)
                                </label>
                                <div class="relative">
                                    <input
                                        type="text"
                                        bind:value={currentSettings.nativeLanguages}
                                        class="w-full bg-sky-dark/50 border border-primary/30 rounded-lg px-3 py-2.5
                                               hover:border-primary/55 hover:shadow-sm hover:shadow-primary/30
                                               focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                               transition-all duration-200 placeholder:text-white/30"
                                        placeholder="e.g. en, fr, es"
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
                    <section class="space-y-6">
                        <h3 class="text-lg font-medium text-primary/80 flex items-center gap-2">
                            <span class="material-icons text-primary/70">vpn_key</span>
                            API Keys
                        </h3>
                        <div class="space-y-4">
                            <div class="relative">
                                <input
                                    type="password"
                                    bind:value={currentSettings.apiKeys.replicate}
                                    class="w-full bg-sky-dark/50 border border-primary/30 rounded-lg pl-[156px] pr-3 py-2
                                           hover:border-primary/55 hover:shadow-sm hover:shadow-primary/30 tracking-wider text-lg
                                           focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                           transition-all duration-200"
                                />
                                <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                             w-[140px] bg-primary/10 border-r border-primary/30 rounded-l-lg
                                             text-sm text-primary/90 font-medium">
                                    Replicate
                                </span>
                            </div>
                            <div class="relative">
                                <input
                                    type="password"
                                    bind:value={currentSettings.apiKeys.assemblyAI}
                                    class="w-full bg-sky-dark/50 border border-primary/30 rounded-lg pl-[156px] pr-3 py-2
                                           hover:border-primary/55 hover:shadow-sm hover:shadow-primary/30 tracking-wider text-lg
                                           focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                           transition-all duration-200"
                                />
                                <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                             w-[140px] bg-primary/10 border-r border-primary/30 rounded-l-lg
                                             text-sm text-primary/90 font-medium">
                                    Assembly AI
                                </span>
                            </div>
                            <div class="relative">
                                <input
                                    type="password"
                                    bind:value={currentSettings.apiKeys.elevenLabs}
                                    class="w-full bg-sky-dark/50 border border-primary/30 rounded-lg pl-[156px] pr-3 py-2
                                           hover:border-primary/55 hover:shadow-sm hover:shadow-primary/30 tracking-wider text-lg
                                           focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                           transition-all duration-200"
                                />
                                <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                             w-[140px] bg-primary/10 border-r border-primary/30 rounded-l-lg
                                             text-sm text-primary/90 font-medium">
                                    Eleven Labs
                                </span>
                            </div>
                        </div>
                        <div class="flex items-center gap-4">
                            <label class="text-sm text-gray-300 whitespace-nowrap">
                                Maximum API retries:
                            </label>
                            <input
                                type="number"
                                bind:value={currentSettings.maxAPIRetries}
                                min="1"
                                class="w-32 bg-sky-dark/50 border border-primary/30 rounded-lg px-3 py-2 pl-4
                                       hover:border-primary/55 hover:shadow-sm hover:shadow-primary/30
                                       focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                       transition-all duration-200"
                            />
                        </div>
                    </section>

                    <!-- UI Settings -->
                    <section class="space-y-6">
                        <h3 class="text-lg font-medium text-primary/80 flex items-center gap-2">
                            <span class="material-icons text-primary/70">palette</span>
                            Interface Settings
                        </h3>
                        <div class="space-y-4">
                            <label class="flex items-center gap-3 cursor-pointer group">
                                <input
                                    type="checkbox"
                                    bind:checked={currentSettings.enableGlow}
                                    class="w-4 h-4 accent-primary rounded"
                                />
                                <span class="text-sm text-gray-300 group-hover:text-white transition-colors">
                                    Enable glow effects (disable if you experience performance issues)
                                </span>
                            </label>
                            <label class="flex items-center gap-3 cursor-pointer group">
                                <input
                                    type="checkbox"
                                    bind:checked={currentSettings.showLogViewerByDefault}
                                    class="w-4 h-4 accent-primary rounded"
                                />
                                <span class="text-sm text-gray-300 group-hover:text-white transition-colors">
                                    Show log viewer by default
                                </span>
                            </label>
                            <div class="flex items-center gap-4">
                                <label class="text-sm text-gray-300 whitespace-nowrap">
                                    Maximum log entries:
                                </label>
                                <input
                                    type="number"
                                    bind:value={currentSettings.maxLogEntries}
                                    min="100"
                                    max="10000"
                                    step="100"
                                    class="w-32 bg-sky-dark/50 border border-primary/30 rounded-lg px-3 py-2 pl-4
                                           hover:border-primary/55 hover:shadow-sm hover:shadow-primary/30
                                           focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                           transition-all duration-200"
                                />
                            </div>
                        </div>
                    </section>

                    <!-- Diagnostic / Debug Export Section -->
                    <section class="space-y-6">
                        <h3 class="text-lg font-medium text-primary/80 flex items-center gap-2">
                            <span class="material-icons text-primary/70">bug_report</span>
                            Diagnostic Tools
                        </h3>
                        <div class="space-y-4">
                            <div class="flex items-center gap-4">
                                <button
                                    class="px-6 py-3 text-gray-300 rounded-lg font-medium bg-sky-dark/50 transition-all duration-200 focus:outline-none"
                                    on:click={exportDebugReport}
                                    disabled={isExportingDebug}
                                    class:glow-success={exportGlowClass==='glow-success'}
                                    class:glow-error={exportGlowClass==='glow-error'}
                                    class:glow-default={exportGlowClass==='glow-default'}
                                >
                                    Export Debug Report
                                </button>
                                {#if isExportingDebug}
                                    <span class="inline-flex items-center gap-1 text-gray-300 text-sm">
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
                <div class="p-6 border-t border-white/10 flex justify-end gap-3">
                    <button
                        class="px-4 py-2 text-white/70 hover:text-white hover:bg-red-500/90 transition-colors duration-200 rounded-lg"
                        on:click={onClose}
                    >
                        Cancel
                    </button>
                    <button
                        class="px-6 py-2 bg-primary text-sky-dark rounded-lg font-medium transition-all duration-200 hover:bg-opacity-80 disabled:opacity-50 disabled:cursor-not-allowed"
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
    /* Smooth scrolling for overflow */
    .overflow-y-auto {
        scrollbar-gutter: stable;
        scroll-behavior: smooth;
    }
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

    /*
      Glow classes:
      - .glow-default: no glow, violet border
      - .glow-success: animated green glow gradient
      - .glow-error: animated red glow gradient
    */

    :global(.glow-default) {
        border: 1px solid #9f6ef7;
        transition: box-shadow 0.6s ease-in-out, border-color 0.6s ease-in-out;
    }

    @keyframes glowSuccess {
        0% {
            box-shadow: none;
            border-color: #9f6ef7;
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
            box-shadow: none;
            border-color: #9f6ef7;
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
