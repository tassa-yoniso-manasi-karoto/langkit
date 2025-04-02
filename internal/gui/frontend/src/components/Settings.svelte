<script lang="ts">
    import { onMount } from 'svelte';
    import { slide, fade } from 'svelte/transition';
    import { settings, showSettings } from '../lib/stores';
    import { ValidateLanguageTag } from '../../wailsjs/go/gui/App';
    import { ExportDebugReport } from '../../wailsjs/go/gui/App';
    
    import TextInput from './TextInput.svelte';
    import NumericInput from './NumericInput.svelte';

    export let onClose: () => void;
    export let version: string = "";

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
        maxWorkers: 1,
        eventThrottling: {
            enabled: true,
            minInterval: 0,
            maxInterval: 250
        }
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
    
    let cancelButton;

    function handleMouseEnter(event) {
      if (!cancelButton) return;
      
      // Get exact coordinates relative to the button
      const rect = cancelButton.getBoundingClientRect();
      const x = event.clientX - rect.left;
      const y = event.clientY - rect.top;
      
      // Create and style a new element for the fill effect
      const fill = document.createElement('div');
      fill.style.position = 'absolute';
      fill.style.left = x + 'px';
      fill.style.top = y + 'px';
      fill.style.width = '0';
      fill.style.height = '0';
      fill.style.borderRadius = '50%';
      fill.style.backgroundColor = 'hsla(0, 85%, 45%, 0.8)';
      fill.style.transform = 'translate(-50%, -50%)';
      fill.style.transition = 'width 0.5s ease-out, height 0.5s ease-out';
      fill.style.zIndex = '-1';
      
      // Append to button
      cancelButton.style.position = 'relative';
      cancelButton.style.overflow = 'hidden';
      cancelButton.appendChild(fill);
      
      // Force reflow
      fill.offsetWidth;
      
      // Calculate max dimension needed to fill button
      const maxDim = Math.max(
        rect.width * 2,
        rect.height * 2,
        Math.sqrt(Math.pow(x, 2) + Math.pow(y, 2)) * 2,
        Math.sqrt(Math.pow(rect.width - x, 2) + Math.pow(y, 2)) * 2,
        Math.sqrt(Math.pow(x, 2) + Math.pow(rect.height - y, 2)) * 2,
        Math.sqrt(Math.pow(rect.width - x, 2) + Math.pow(rect.height - y, 2)) * 2
      );
      
      // Expand the fill
      fill.style.width = maxDim + 'px';
      fill.style.height = maxDim + 'px';
    }

    function handleMouseLeave() {
      // Remove all fill elements when mouse leaves
      if (cancelButton) {
        const fills = cancelButton.querySelectorAll('div');
        fills.forEach(fill => cancelButton.removeChild(fill));
      }
    }

    // Check if we should show dev-only features
    $: isDevVersion = version === "dev";

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
                nativeLanguages: loadedSettings.nativeLanguages || '',
                eventThrottling: loadedSettings.eventThrottling || {
                    enabled: true,
                    minInterval: 0,
                    maxInterval: 250
                }
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
                nativeLanguages: value.nativeLanguages || currentSettings.nativeLanguages || '',
                eventThrottling: value.eventThrottling || currentSettings.eventThrottling || {
                    enabled: true,
                    minInterval: 0,
                    maxInterval: 250
                }
            };
        }
    });
</script>

{#if $showSettings}
    <div class="settings-modal">
        <!-- Improved backdrop with more blur and less transparency -->
        <div class="fixed inset-0 backdrop-blur-lg z-50 overflow-y-auto"
             transition:fade={{ duration: 200 }}>
            <div class="container mx-auto max-w-2xl p-4 min-h-screen flex items-center"
                 transition:slide={{ duration: 300 }}
                 on:click|stopPropagation>
                <!-- Improved panel background with less transparency -->
                <div class="backdrop-blur-3xl rounded-xl shadow-2xl border border-primary/30 w-full 
                            shadow-settings">
                    <!-- Header with improved contrast -->
                    <div class="p-6 border-b border-primary/30 bg-bg-800/50">
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
                    
                    <!-- Content with improved readability -->
                    <div class="p-6 space-y-8 max-h-[calc(100vh-16rem)] overflow-y-auto settings-content">
                        <!-- Language Settings -->
                        <section class="space-y-6">
                            <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                                <span class="material-icons text-primary">translate</span>
                                Default Language Settings
                            </h3>
                            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                                <!-- Target Language with improved input styling -->
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
                                                      focus:ring-primary/50 placeholder:text-white/40 pr-10 
                                                      bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                            customBackground="rgba(0, 0, 0, 0.4)"
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

                                <!-- Native Languages with improved input styling -->
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
                                                      focus:ring-primary/50 placeholder:text-white/40 pr-10 
                                                      bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                            customBackground="rgba(0, 0, 0, 0.4)"
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

                        <!-- API Settings with improved input styling -->
                        <section class="space-y-6">
                            <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                                <span class="material-icons text-primary">vpn_key</span>
                                API Keys
                            </h3>
                            <div class="space-y-4">
                                <div class="relative glass-input-container">
                                    <input
                                        type="password"
                                        bind:value={currentSettings.apiKeys.replicate}
                                        class="w-full bg-black backdrop-blur-sm border border-primary/40 rounded-lg pl-[156px] pr-3 py-2
                                               hover:border-primary/55 hover:shadow-input tracking-wider text-lg text-white
                                               focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                               focus:shadow-input-focus transition-all duration-200"
                                    />
                                    <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                                 w-[140px] bg-primary/20 border-r border-primary/30 rounded-l-lg
                                                 text-sm text-primary font-medium">
                                        Replicate
                                    </span>
                                </div>
                                <div class="relative glass-input-container">
                                    <input
                                        type="password"
                                        bind:value={currentSettings.apiKeys.assemblyAI}
                                        class="w-full bg-black/40 backdrop-blur-sm border border-primary/40 rounded-lg pl-[156px] pr-3 py-2
                                               hover:border-primary/55 hover:shadow-input tracking-wider text-lg text-white
                                               focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                               focus:shadow-input-focus transition-all duration-200"
                                    />
                                    <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                                 w-[140px] bg-primary/20 border-r border-primary/30 rounded-l-lg
                                                 text-sm text-primary font-medium">
                                        AssemblyAI
                                    </span>
                                </div>
                                <div class="relative glass-input-container">
                                    <input
                                        type="password"
                                        bind:value={currentSettings.apiKeys.elevenLabs}
                                        class="w-full bg-black/40 backdrop-blur-sm border border-primary/40 rounded-lg pl-[156px] pr-3 py-2
                                               hover:border-primary/55 hover:shadow-input tracking-wider text-lg text-white
                                               focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                               focus:shadow-input-focus transition-all duration-200"
                                    />
                                    <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                                 w-[140px] bg-primary/20 border-r border-primary/30 rounded-l-lg
                                                 text-sm text-primary font-medium">
                                        ElevenLabs
                                    </span>
                                </div>
                                 <div class="relative glass-input-container">
                                     <input
                                         type="password"
                                         bind:value={currentSettings.apiKeys.openAI}
                                         class="w-full bg-black/40 backdrop-blur-sm border border-primary/40 rounded-lg pl-[156px] pr-3 py-2
                                                hover:border-primary/55 hover:shadow-input tracking-wider text-lg text-white
                                                focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/50
                                                focus:shadow-input-focus transition-all duration-200"
                                     />
                                     <span class="absolute left-0 top-0 bottom-0 flex items-center justify-center
                                                  w-[140px] bg-primary/20 border-r border-primary/30 rounded-l-lg
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
                                               duration-200 bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                />
                            </div>
                        </section>

                        <!-- Worker Pool Settings with improved styling -->
                        <section class="space-y-6">
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
                                               duration-200 bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                />
                            </div>
                        </section>

                        <!-- UI Settings with improved styling -->
                        <section class="space-y-6">
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
                                                   duration-200 bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                    />
                                </div>
                            </div>
                        </section>

                        <!-- Diagnostic / Debug Export Section with improved styling -->
                        <section class="space-y-6">
                            <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                                <span class="material-icons text-primary">bug_report</span>
                                Diagnostic Tools
                            </h3>
                            <div class="space-y-4">
                                <div class="flex items-center gap-4">
                                    <button
                                        class="px-6 py-3 text-white rounded-lg font-semibold bg-bg-800/70 backdrop-blur-sm 
                                               border-primary/80 transition-all duration-200 focus:outline-none 
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

                    <!-- Footer with improved styling -->
                    <div class="p-6 border-t border-primary/30 flex justify-end gap-3 bg-bg-800/50">
                        <button
                            bind:this={cancelButton}
                            class="px-4 py-2 text-white/90 border border-primary/30 transition-all duration-200 rounded-lg 
                                  hover:text-white hover:border-red-500/80 cancel-button"
                            on:click={onClose}
                            on:mouseenter={handleMouseEnter}
                            on:mouseleave={handleMouseLeave}
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
    </div>
{/if}

<style>
    :global(.settings-modal input) {
        background-color: hsla(var(--input-bg), 0.75) !important;
        backdrop-filter: blur(10px) !important; 
        -webkit-backdrop-filter: blur(10px) !important;
    }

    /*:global(.settings-modal input:hover) {
        background-color: hsla(var(--input-bg-hover), 0.8) !important;
        backdrop-filter: blur(8px) !important;
        -webkit-backdrop-filter: blur(8px) !important;
    }

    :global(.settings-modal input:focus) {
        background-color: hsla(var(--input-bg-focus), 0.9) !important;
        backdrop-filter: blur(10px) !important;
        -webkit-backdrop-filter: blur(10px) !important;
    }*/
    
    /* Animated cancel button with propagating hover effect from entry point */
    :global(.settings-modal .cancel-button) {
      position: relative;
      overflow: hidden;
      background-color: hsla(var(--input-bg), 0.85) !important;
      backdrop-filter: blur(8px) !important;
      -webkit-backdrop-filter: blur(8px) !important;
      border: 3px solid var(--input-border) !important;
      transition: color 0.3s ease-in, border-color 0.3s ease-in !important;
      z-index: 1;
    }

    :global(.settings-modal .cancel-button::before) {
      content: "";
      position: absolute;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background: radial-gradient(circle at var(--x, center) var(--y, center), 
                  hsla(0, 85%, 45%, 0.9) 0%, 
                  hsla(0, 85%, 45%, 0.85) 60%, 
                  hsla(0, 85%, 45%, 0.8) 100%);
      opacity: 0;
      transform: scale(0);
      transition: opacity 0.5s ease-in, transform 0.5s ease-in !important;
      z-index: -1;
    }

    :global(.settings-modal .cancel-button:hover) {
      color: white !important;
      border-color: hsl(var(--error-all-hue), calc(var(--error-all-saturation) * 2), calc(var(--error-all-lightness) / 2.5)) !important;
    }

    :global(.settings-modal .cancel-button:hover::before) {
      opacity: 1;
      transform: scale(3);
    }
    
    
    

    /* Enhanced glassmorphic effect */
    .glass-input-container input:focus {
        backdrop-filter: blur(12px);
    }
    
    /* Improved scrollbar for better visibility */
    .overflow-y-auto {
        scrollbar-gutter: stable;
        scroll-behavior: smooth;
    }
    .overflow-y-auto::-webkit-scrollbar {
        width: 6px;
    }
    .overflow-y-auto::-webkit-scrollbar-track {
        background: rgba(0, 0, 0, 0.1);
    }
    .overflow-y-auto::-webkit-scrollbar-thumb {
        background-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.6);
        border-radius: 20px;
    }
    .overflow-y-auto::-webkit-scrollbar-thumb:hover {
        background-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.8);
    }

    /* Enhanced settings panel drop shadow */
    .shadow-settings {
        box-shadow: 
            0 10px 50px 0 rgba(0, 0, 0, 0.7),
            0 0 20px 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.3);
    }
    
    /* Enhanced hover states for inputs */
    :global(.shadow-input) {
        box-shadow: 
            0 0 8px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.3),
            inset 0 0 3px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.2);
    }
    
    :global(.shadow-input-focus) {
        box-shadow: 
            0 0 12px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.5),
            inset 0 0 5px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.3);
    }

    /* Enhanced hover effect for section headers */
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
    
    /* Enhanced checkbox styles */
    .checkbox-container:hover .custom-checkbox {
        box-shadow: 0 0 8px 2px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4);
    }
    
    /* Enhanced debug export button */
    .debug-export-button {
        box-shadow: 0 0 5px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.3);
    }
    
    /* Enhanced throttling control styles */
    .disabled {
        opacity: 0.5;
        pointer-events: none;
    }

    @keyframes glowSuccess {
        0% {
            box-shadow: 
                0 0 5px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.3);
            border-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.5);
        }
        100% {
            border-color: #22c55e;
            box-shadow:
                0 0 8px 2px rgba(34, 197, 94, 0.5),
                0 0 16px 4px rgba(34, 197, 94, 0.3),
                0 0 24px 6px rgba(34, 197, 94, 0.2);
        }
    }

    :global(.glow-success) {
        animation: glowSuccess 1s forwards;
    }

    @keyframes glowError {
        0% {
            box-shadow: 
                0 0 5px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.3);
            border-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.5);
        }
        100% {
            border-color: #ef4444;
            box-shadow:
                0 0 8px 2px rgba(239, 68, 68, 0.5),
                0 0 16px 4px rgba(239, 68, 68, 0.3),
                0 0 24px 6px rgba(239, 68, 68, 0.15);
        }
    }

    :global(.glow-error) {
        animation: glowError 1s forwards;
    }
    
    /* Add text shadows to make white text more legible on translucent backgrounds */
    input, select {
        text-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
    }
</style>