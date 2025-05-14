<script lang="ts">
    import { onMount, onDestroy } from 'svelte'; // Added onDestroy
    import { slide, fade } from 'svelte/transition';
    import { settings, showSettings } from '../lib/stores';
    import { ValidateLanguageTag } from '../../wailsjs/go/gui/App';
    import { ExportDebugReport } from '../../wailsjs/go/gui/App';
    import { logger } from '../lib/logger';
    
    import TextInput from './TextInput.svelte';
    import NumericInput from './NumericInput.svelte';
    import SelectInput from './SelectInput.svelte';
    import Hovertip from './Hovertip.svelte'; // Import hovertip component
    import { isWasmSupported, getWasmState, resetWasmMetrics } from '../lib/wasm'; // Import WASM utils
    import { WasmInitStatus } from '../lib/wasm-state'; // Import enum

    // Track if we're currently resetting the animation state
    let isResetting = false;
    
    export let onClose: () => void = () => { /* FIXME this reset doesn't work but claude keeps outputing the same code... whatever honestly.*/ 
        // Set flag to prevent reactive updates during reset
        isResetting = true;
        
        // First directly manipulate the DOM to forcefully remove animation classes
        const button = document.querySelector('.debug-export-button');
        if (button) {
            // Force remove all animation classes
            button.classList.remove('glow-success', 'glow-error', 'glow-reset');
            // button.classList.add('force-reset'); // Removed this problematic class manipulation
        }
        
        // Reset state variables after DOM manipulation
        exportSuccess = false;
        exportError = '';
        
        // Use a timeout to allow the DOM to update before clearing reset state
        setTimeout(() => {
            // if (button) { // No need to remove force-reset anymore
            //     button.classList.remove('force-reset');
            // }
            isResetting = false;
        }, 150);
    };
    
    export let version: string = "";

    interface LanguageCheckResponse {
        isValid: boolean;
        error?: string;
    }

    // Initialize with default values to ensure structure is complete
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
        convertValues: false, // Add missing property, default to false
        useWasm: true,
        wasmSizeThreshold: 500,
        forceWasmMode: 'auto',
        logViewerVirtualizationThreshold: 2000, // Add new field with default
        // Add new WASM memory settings with defaults
        wasmMemoryPreallocation: 32, // 32MB default
        wasmMemoryMargin: 'medium',  // Medium safety margin
        wasmMemoryGrowthStrategy: 'balanced', // Balanced growth
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
    
    // WASM state for UI binding
    let wasmState = getWasmState();
    // Update wasmState periodically for the dashboard
    let wasmStateUpdateInterval: number | null = null;
    
    let cancelButton: HTMLButtonElement | null = null; // Add type

    function handleMouseEnter(event: MouseEvent) { // Add type
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
        fills.forEach((fill: Element) => cancelButton?.removeChild(fill)); // Add type and optional chaining
      }
    }

    // Moved definition earlier

    // Check if we should show dev-only features
    $: isDevVersion = version === "dev";

    // Modified reactive declaration to handle reset state
    $: exportGlowClass = isResetting
        ? '' // No special class needed if force-reset is removed
        : exportSuccess
            ? 'glow-success'
            : exportError
                ? 'glow-error'
                : '';

    // Define the resetExportState function before it's used
    function resetExportState() {
        exportSuccess = false;
        exportError = '';
    }

    async function validateLanguages() {
        if (currentSettings.targetLanguage) {
            const targetResponse = await ValidateLanguageTag(currentSettings.targetLanguage, true);
            targetLangValid = targetResponse.isValid;
            targetLangError = targetResponse.error || '';
        } else {
            targetLangValid = true; // Allow empty target language
            targetLangError = '';
        }

        if (currentSettings.nativeLanguages) {
            const nativeResponse = await ValidateLanguageTag(currentSettings.nativeLanguages, false);
            nativeLangValid = nativeResponse.isValid;
            nativeLangError = nativeResponse.error || '';
        } else {
            nativeLangValid = true; // Allow empty native languages
            nativeLangError = '';
        }

        isValid = targetLangValid && nativeLangValid; // Both must be valid if provided
    }

    async function saveSettings() {
        await validateLanguages();
        if (!isValid) return;
        try {
            // Save to backend
            await (window as any).go.gui.App.SaveSettings(currentSettings);
            // Update store with our current values
            // Create a new object with the correct type for forceWasmMode before setting
            const settingsToSave = {
                ...currentSettings,
                forceWasmMode: currentSettings.forceWasmMode as "auto" | "enabled" | "disabled"
            };
            settings.set(settingsToSave);
            
            // Trigger STT model refresh after API key changes
            try {
                await (window as any).go.gui.App.RefreshSTTModelsAfterSettingsUpdate();
            } catch (error) {
                logger.error('settings', 'Failed to refresh STT models', { error });
            }
            
            // Update local wasmState
            wasmState = getWasmState(); 
            
            // Close settings modal on save
            onClose();
            
            // Notify other components about settings update
            window.dispatchEvent(new CustomEvent('settingsUpdated', {
                detail: currentSettings
            }));
        } catch (error) {
            logger.error('settings', 'Failed to save settings', { error });
            // Show error in the UI - ensure error is treated as Error instance
            const errorMsg = error instanceof Error ? error.message : String(error);
            exportError = 'Failed to save settings: ' + (errorMsg || 'Unknown error');
            setTimeout(resetExportState, 3000);
        }
    }
    
    // Handle individual setting updates (for immediate updates like checkboxes and WebAssembly settings)
    async function updateSettings() {
        await validateLanguages();
        if (!isValid) return;
        try {
            // Only update specific WebAssembly settings that should apply immediately
            // Include logViewerVirtualizationThreshold in the check for immediate updates
            if (currentSettings.useWasm !== undefined ||
                currentSettings.wasmSizeThreshold !== undefined ||
                currentSettings.forceWasmMode !== undefined ||
                currentSettings.logViewerVirtualizationThreshold !== undefined) {
                    
                await (window as any).go.gui.App.SaveSettings(currentSettings);
                // Create a new object with the correct type for forceWasmMode before setting
                const settingsToUpdate = {
                    ...currentSettings,
                    forceWasmMode: currentSettings.forceWasmMode as "auto" | "enabled" | "disabled"
                };
                settings.set(settingsToUpdate);
                wasmState = getWasmState();
            }
        } catch (error) {
            logger.error('settings', 'Failed to update settings', { error });
        }
    }

    async function exportDebugReport() {
        isExportingDebug = true;
        exportSuccess = false;
        exportError = '';
        try {
            await ExportDebugReport();
            exportSuccess = true;
        } catch (err: any) { // Type the error
            logger.error('settings', 'Failed to export debug report', { error: err });
            exportError = err?.message || 'Unknown error occurred.';
        } finally {
            isExportingDebug = false;
        }
    }

    onMount(async () => {
        try {
            // Load settings from backend
            const loadedSettings = await (window as any).go.gui.App.LoadSettings();
            settings.set(loadedSettings); // Update store with backend data
            
            // Merge loaded settings with defaults to ensure all fields exist
            currentSettings = {
                ...currentSettings, // Keep defaults as fallback
                ...loadedSettings,
                targetLanguage: loadedSettings.targetLanguage || '',
                nativeLanguages: loadedSettings.nativeLanguages || '',
                // Ensure WASM fields exist
                useWasm: loadedSettings.useWasm !== undefined ? loadedSettings.useWasm : true,
                wasmSizeThreshold: loadedSettings.wasmSizeThreshold || 500,
                forceWasmMode: (loadedSettings.forceWasmMode || 'auto') as "auto" | "enabled" | "disabled", // Add type assertion
                // Ensure event throttling exists
                eventThrottling: loadedSettings.eventThrottling || {
                    enabled: true,
                    minInterval: 0,
                    maxInterval: 250
                }
            };
            await validateLanguages();
        } catch (error) {
            logger.error('settings', 'Failed to load settings', { error });
        }

        // Update local wasmState on mount as well
        wasmState = getWasmState();
        // Start interval to keep dashboard updated
        wasmStateUpdateInterval = window.setInterval(() => {
            wasmState = getWasmState();
        }, 1000); // Update every second
    });

    // Re-validate whenever relevant parts of currentSettings change
    $: {
        if (currentSettings.targetLanguage !== undefined ||
            currentSettings.nativeLanguages !== undefined) {
            validateLanguages();
        }
    }

    // Keep currentSettings synced with the store if it changes elsewhere
    // This is useful if other components update settings
    settings.subscribe(value => {
        if (value && Object.keys(value).length > 0) {
            // Don't overwrite local changes during editing
            if (!showSettings) {
                currentSettings = {
                    ...currentSettings, // Keep defaults as fallback
                    ...value,
                    targetLanguage: value.targetLanguage || '',
                    nativeLanguages: value.nativeLanguages || '',
                    forceWasmMode: (value.forceWasmMode || 'auto') as "auto" | "enabled" | "disabled", // Add type assertion
                    // Add type check for eventThrottling
                    eventThrottling: typeof value.eventThrottling === 'object' && value.eventThrottling !== null
                                        ? value.eventThrottling
                                        : currentSettings.eventThrottling
                };
                validateLanguages();
            }
        }
    });
    
    // Clear interval on component destroy
    onDestroy(() => {
        if (wasmStateUpdateInterval) {
            clearInterval(wasmStateUpdateInterval);
        }
    });
</script>

{#if $showSettings}
    <div class="settings-modal">
        <!-- Improved backdrop with more blur and less transparency -->
        <div class="fixed inset-0 backdrop-blur-lg overflow-y-auto"
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
                                            minLength={1}
                                            maxLength={9}
                                            placeholder="e.g. es, yue or pt-BR"
                                            className="px-3 py-2.5 hover:border-primary/55 hover:shadow-input
                                                      focus:border-primary focus:ring-1 focus:shadow-input-focus
                                                      focus:ring-primary/50 placeholder:text-white/40 pr-10
                                                      bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                            
                                        />
                                        {#if targetLangValid && currentSettings.targetLanguage}
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
                                            minLength={1}
                                            maxLength={100}
                                            placeholder="e.g. en, fr, es"
                                            className="px-3 py-2.5 hover:border-primary/55 hover:shadow-input
                                                      focus:border-primary focus:ring-1 focus:shadow-input-focus
                                                      focus:ring-primary/50 placeholder:text-white/40 pr-10
                                                      bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                            
                                        />
                                        {#if nativeLangValid && currentSettings.nativeLanguages}
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

                        <!-- API and Timeout Settings with improved input styling -->
                        <section class="space-y-6">
                            <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                                <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32" class="text-primary" stroke-width="0.8" stroke="currentColor">
                                    <path fill="currentColor" d="M28 26c-.178 0-.347.03-.511.074l-1.056-1.055c.352-.595.567-1.28.567-2.019s-.215-1.424-.567-2.019l1.055-1.055c.165.043.334.074.512.074a2 2 0 1 0-2-2c0 .178.03.347.074.512l-1.055 1.055C24.424 19.215 23.739 19 23 19s-1.424.215-2.019.567l-1.055-1.055c.043-.165.074-.334.074-.512a2 2 0 1 0-2 2c.178 0 .347-.03.512-.074l1.055 1.055C19.215 21.576 19 22.261 19 23s.215 1.424.567 2.019l-1.055 1.055A2 2 0 0 0 18 26a2 2 0 1 0 2 2c0-.178-.03-.347-.074-.512l1.055-1.055c.595.352 1.28.567 2.019.567s1.424-.215 2.019-.567l1.055 1.055A2 2 0 0 0 26 28a2 2 0 1 0 2-2m-7-3c0-1.102.897-2 2-2s2 .898 2 2s-.897 2-2 2s-2-.897-2-2"/>
                                    <circle cx="22" cy="10" r="2" fill="currentColor"/>
                                    <path fill="currentColor" d="M21 2c-4.963 0-9 4.037-9 9c0 .779.099 1.547.294 2.291L2 23.586V30h6.414l7-7l-2.707-2.707l-1.414 1.414L12.586 23l-1.59 1.59l-1.287-1.295l-1.418 1.41l1.29 1.299L7.587 28H4v-3.586l9.712-9.712l.856-.867l-.199-.585A7.008 7.008 0 0 1 21 4c3.86 0 7.001 3.14 7.001 7h2c0-4.963-4.037-9-9-9Z"/>
                                </svg>
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
                        </section>
                            
                        <!-- API TIMEOUTS SECTION -->
                        <section class="space-y-6">
                            <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                                <span class="material-icons text-primary">timer</span>
                                Timeouts & Retries
                            </h3>
                            <!-- Maximum API retries -->
                            <div class="setting-row">
                                <div class="setting-label">
                                    <span>Maximum API retries</span>
                                    <span class="setting-description">Number of retry attempts for failed API calls</span>
                                </div>
                                <div class="setting-control">
                                    <NumericInput
                                        bind:value={currentSettings.maxAPIRetries}
                                        min={1}
                                        step={1}
                                        className="w-48 px-3 py-2 hover:border-primary/55
                                                hover:shadow-input focus:shadow-input-focus
                                                focus:border-primary focus:ring-1
                                                focus:ring-primary/50 bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                    />
                                </div>
                            </div>
                            
                            <!-- Voice separation timeout -->
                            <div class="setting-row">
                                <div class="setting-label">
                                    <span>Voice separation timeout</span>
                                    <span class="setting-description">Timeout in seconds for voice separation operations (Demucs, Spleeter) - Default: 2100</span>
                                </div>
                                <div class="setting-control">
                                    <NumericInput
                                        bind:value={currentSettings.timeoutSep}
                                        min={60}
                                        max={7200}
                                        step={60}
                                        className="w-48 px-3 py-2 hover:border-primary/55
                                                hover:shadow-input focus:shadow-input-focus
                                                focus:border-primary focus:ring-1
                                                focus:ring-primary/50 bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                    />
                                </div>
                            </div>
                            
                            <!-- Speech-to-text timeout -->
                            <div class="setting-row">
                                <div class="setting-label">
                                    <span>Speech-to-text timeout</span>
                                    <span class="setting-description">Timeout in seconds for speech-to-text operations per audio segment - Default: 90</span>
                                </div>
                                <div class="setting-control">
                                    <NumericInput
                                        bind:value={currentSettings.timeoutSTT}
                                        min={10}
                                        max={600}
                                        step={10}
                                        className="w-48 px-3 py-2 hover:border-primary/55
                                                hover:shadow-input focus:shadow-input-focus
                                                focus:border-primary focus:ring-1
                                                focus:ring-primary/50 bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                    />
                                </div>
                            </div>
                            
                            <!-- Download timeout -->
                            <div class="setting-row">
                                <div class="setting-label">
                                    <span>Download timeout</span>
                                    <span class="setting-description">Timeout in seconds for download operations - Default: 600</span>
                                </div>
                                <div class="setting-control">
                                    <NumericInput
                                        bind:value={currentSettings.timeoutDL}
                                        min={30}
                                        max={3600}
                                        step={30}
                                        className="w-48 px-3 py-2 hover:border-primary/55
                                                hover:shadow-input focus:shadow-input-focus
                                                focus:border-primary focus:ring-1
                                                focus:ring-primary/50 bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                    />
                                </div>
                            </div>
                        </section>

                        <!-- Worker Pool Settings with improved styling -->
                        <section class="space-y-6">
                            <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                                <span class="material-icons text-primary">speed</span>
                                Worker Pool Settings
                            </h3>
                            <div class="setting-row">
                                <div class="setting-label">
                                    <span>Maximum Workers</span>
                                    <span class="setting-description">Number of concurrent worker processes</span>
                                </div>
                                <div class="setting-control">
                                    <NumericInput
                                        bind:value={currentSettings.maxWorkers}
                                        min={1}
                                        step={1}
                                        className="w-48 px-3 py-2 hover:border-primary/55
                                                hover:shadow-input focus:shadow-input-focus
                                                focus:border-primary focus:ring-1
                                                focus:ring-primary/50 bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                    />
                                </div>
                            </div>
                        </section>
                        
                        <!-- Intermediary File Handling Settings -->
                        <section class="space-y-6">
                            <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                                <span class="material-icons text-primary">storage</span>
                                Intermediary Files
                            </h3>
                            
                            <div class="setting-row">
                                <div class="setting-label">
                                    <div class="flex items-center justify-center gap-1">
                                        <span>Intermediary File Mode</span>
                                        <Hovertip position="right">
                                            <span slot="trigger" class="material-icons text-xs text-primary/80 cursor-help">help_outline</span>
                                            <div class="max-w-xs">
                                                <p class="mb-2">Intermediary files can be useful for reprocessing with different settings, but may consume substantial disk space:</p>
                                                <ul class="list-disc ml-4 space-y-1">
                                                    <li><strong class="text-primary/90">Keep:</strong> Preserves files at original quality for maximum reusability</li>
                                                    <li><strong class="text-primary/90">Recompress:</strong> Compresses files to balance space and reusability</li>
                                                    <li><strong class="text-primary/90">Delete:</strong> Removes intermediary files immediately after processing</li>
                                                </ul>
                                            </div>
                                        </Hovertip>
                                    </div>
                                    <span class="setting-description">How to handle intermediary files produced during processing</span>
                                </div>
                                
                                <div class="setting-control">
                                    <SelectInput
                                        bind:value={currentSettings.intermediaryFileMode}
                                        className="px-3 py-2 rounded-lg"
                                        on:change={updateSettings}
                                    >
                                        <option value="keep">Keep Files (Original Quality)</option>
                                        <option value="recompress">Recompress Files (Save Space)</option>
                                        <option value="delete">Delete Files (Save Maximum Space)</option>
                                    </SelectInput>
                                </div>
                            </div>
                        </section>
                        
                        <!-- Performance Settings -->
                        <section class="space-y-6">
                            <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                                <span class="material-icons text-primary">speed</span> <!-- Changed icon -->
                                Performance Settings
                            </h3>

                            <!-- WebAssembly Settings -->
                            <div class="mb-5 pb-4">
                                <h4 class="text-base font-medium mb-2 text-gray-200">WebAssembly Optimization</h4>

                                {#if !isWasmSupported()}
                                    <div class="p-3 bg-warning-all/10 border border-warning-all/30 rounded-lg">
                                        <span class="text-warning-all text-sm">WebAssembly is not supported in this browser. Optimization is unavailable.</span>
                                    </div>
                                {:else}
                                    <!-- Enable WebAssembly -->
                                    <div class="setting-row">
                                        <div class="setting-label">
                                            <span>Enable WebAssembly</span>
                                            <span class="setting-description">Use WebAssembly for improved performance in log processing</span>
                                        </div>
                                        <div class="setting-control">
                                            <label class="toggle-switch">
                                                <input
                                                    type="checkbox"
                                                    bind:checked={currentSettings.useWasm}
                                                    on:change={updateSettings}
                                                />
                                                <span class="slider round"></span>
                                            </label>
                                        </div>
                                    </div>

                                    <!-- WebAssembly Mode -->
                                    <div class="setting-row" class:disabled={!currentSettings.useWasm}>
                                        <div class="setting-label">
                                            <span>WebAssembly Mode</span>
                                            <span class="setting-description">Control when WebAssembly optimization is used</span>
                                        </div>
                                        <div class="setting-control">
                                            <SelectInput
                                                bind:value={currentSettings.forceWasmMode}
                                                on:change={updateSettings}
                                                disabled={!currentSettings.useWasm}
                                                className="px-3 py-2 rounded-lg disabled:opacity-50"
                                            >
                                                <option value="auto">Auto (Based on threshold)</option>
                                                <option value="enabled">Always Enabled</option>
                                                <option value="disabled">Always Disabled</option>
                                            </SelectInput>
                                        </div>
                                    </div>

                                    <!-- WebAssembly Threshold -->
                                    <div class="setting-row" class:disabled={!currentSettings.useWasm || currentSettings.forceWasmMode !== 'auto'}>
                                        <div class="setting-label">
                                            <span>WebAssembly Size Threshold</span>
                                            <span class="setting-description">Use WebAssembly for operations with more than {currentSettings.wasmSizeThreshold} logs (Range: 100-5000)</span>
                                        </div>
                                        <div class="setting-control">
                                            <NumericInput
                                                bind:value={currentSettings.wasmSizeThreshold}
                                                min={100}
                                                max={5000}
                                                step={100}
                                                disabled={!currentSettings.useWasm || currentSettings.forceWasmMode !== 'auto'}
                                                className="w-48 px-3 py-2 hover:border-primary/55
                                                        hover:shadow-input focus:shadow-input-focus
                                                        focus:border-primary focus:ring-1
                                                        focus:ring-primary/50 bg-black/40 backdrop-blur-sm border-primary/40 text-white disabled:opacity-50"
                                            />
                                        </div>
                                    </div>
                                {/if}
                            </div>
                            <!-- LogViewer Settings -->
                            <div class="mb-5 mt-6 pt-6 border-t border-primary/20"> <!-- Added top margin/border -->
                                <h4 class="text-base font-medium mb-2 text-gray-200">LogViewer Performance</h4>

                                <!-- Virtualization Threshold -->
                                <div class="setting-row">
                                    <div class="setting-label">
                                        <span>Virtualization Threshold</span>
                                        <span class="setting-description">Enable virtualization when log count exceeds this threshold (Range: 500-10000)</span>
                                    </div>
                                    <div class="setting-control">
                                        <NumericInput
                                            bind:value={currentSettings.logViewerVirtualizationThreshold}
                                            min={500}
                                            max={10000}
                                            step={500}
                                            className="w-48 px-3 py-2 hover:border-primary/55
                                                    hover:shadow-input focus:shadow-input-focus
                                                    focus:border-primary focus:ring-1
                                                    focus:ring-primary/50 bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                        />
                                    </div>
                                </div>

                                <!-- Information about virtualization -->
                                <div class="text-xs text-unobtrusive mt-2 bg-primary/5 p-2 rounded">
                                    <p>Virtualization improves performance with large log volumes by only rendering visible logs. Lower thresholds improve performance but may cause visual glitches in some browsers.</p>
                                </div>
                            </div>
                        </section>

                        <!-- UI Settings with improved styling -->
                        <section class="space-y-6">
                            <h3 class="text-lg font-medium text-primary flex items-center gap-2 settings-heading">
                                <span class="material-icons text-primary">palette</span>
                                Interface Settings
                            </h3>
                            <!-- Enable glow effects -->
                            <div class="setting-row">
                                <div class="setting-label">
                                    <span>Enable glow effects</span>
                                    <span class="setting-description">Adds visual glow effects to UI elements</span>
                                </div>
                                <div class="setting-control">
                                    <label class="toggle-switch">
                                        <input
                                            type="checkbox"
                                            bind:checked={currentSettings.enableGlow}
                                            on:change={updateSettings}
                                        />
                                        <span class="slider round"></span>
                                    </label>
                                </div>
                            </div>
                            
                            <!-- Show log viewer by default -->
                            <div class="setting-row">
                                <div class="setting-label">
                                    <span>Show log viewer by default</span>
                                    <span class="setting-description">Automatically open log viewer when starting a process</span>
                                </div>
                                <div class="setting-control">
                                    <label class="toggle-switch">
                                        <input
                                            type="checkbox"
                                            bind:checked={currentSettings.showLogViewerByDefault}
                                            on:change={updateSettings}
                                        />
                                        <span class="slider round"></span>
                                    </label>
                                </div>
                            </div>
                            
                            <!-- Maximum log entries -->
                            <div class="setting-row">
                                <div class="setting-label">
                                    <span>Maximum log entries</span>
                                    <span class="setting-description">Limit the number of log entries to improve performance</span>
                                </div>
                                <div class="setting-control">
                                    <NumericInput
                                        bind:value={currentSettings.maxLogEntries}
                                        min={100}
                                        step={100}
                                        className="w-48 px-3 py-2 hover:border-primary/55
                                                hover:shadow-input focus:shadow-input-focus
                                                focus:border-primary focus:ring-1
                                                focus:ring-primary/50 bg-black/40 backdrop-blur-sm border-primary/40 text-white"
                                        on:change={updateSettings}
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
                                        class="debug-export-button px-6 py-3 text-white rounded-lg font-semibold
                                               bg-input-bg/50 backdrop-blur-sm focus:outline-none"
                                        on:click={exportDebugReport}
                                        disabled={isExportingDebug}
                                        class:glow-success={exportGlowClass === 'glow-success'}
                                        class:glow-error={exportGlowClass === 'glow-error'}
                                        class:force-reset={exportGlowClass === 'force-reset'}
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
                            class="px-4 py-2 text-white/90 border border-primary/30 transition-all duration-300 rounded-lg 
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
      border-color: hsl(var(--error-hard-hue), calc(var(--error-hard-saturation) * 2), calc(var(--error-hard-lightness) / 2.5)) !important;
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
    
    /* Enhanced debug export button with ambient glow */
    .debug-export-button {
        /* Base state - always have an ambient glow */
        border: 2px solid hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.8);
        box-shadow: 
            0 0 8px 2px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.35),
            inset 0 0 3px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.2);
        
        /* Define CSS variables for various states */
        --ambient-glow-opacity: 0.35;
        --hover-glow-opacity: 0.7; 
        --focus-glow-opacity: 0.5;
        --active-glow-opacity: 0.4;
        
        --ambient-glow-spread: 2px;
        --hover-glow-spread: 4px;
        --focus-glow-spread: 3px;
        --active-glow-spread: 2px;
        
        --ambient-glow-blur: 8px;
        --hover-glow-blur: 15px;
        --focus-glow-blur: 10px;
        --active-glow-blur: 10px;
        
        --success-color-light: hsla(145, 63%, 49%, 1);
        --success-color-dark: hsla(145, 63%, 30%, 1);
        --error-color-light: hsla(0, 84%, 60%, 1);
        --error-color-dark: hsla(0, 84%, 45%, 1);
        
        /* Transition for all properties */
        transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    }
    
    /* Hover state with stronger glow */
    .debug-export-button:hover {
        transform: translateY(-1px);
        border-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 1);
        box-shadow:
            0 0 var(--hover-glow-blur) var(--hover-glow-spread) 
                hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), var(--hover-glow-opacity)),
            0 0 calc(var(--hover-glow-blur) * 1.5) calc(var(--hover-glow-spread) * 0.5) 
                hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), calc(var(--hover-glow-opacity) * 0.5)),
            inset 0 0 3px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.25);
    }
    
    /* Focus state with mild glow */
    .debug-export-button:focus {
        outline: none;
        border-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.9);
        box-shadow:
            0 0 var(--focus-glow-blur) var(--focus-glow-spread) 
                hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), var(--focus-glow-opacity)),
            inset 0 0 3px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.2);
    }
    
    /* Keep mild glow when active */
    .debug-export-button:active {
        transform: translateY(0);
        box-shadow:
            0 0 var(--active-glow-blur) var(--active-glow-spread) 
                hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), var(--active-glow-opacity)),
            inset 0 0 4px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.25);
    }
    
    /* Define CSS variables for focus state that will be used by animations */
    .debug-export-button {
        --focus-glow-shadow: 
            0 0 var(--focus-glow-blur) var(--focus-glow-spread) 
                hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), var(--focus-glow-opacity)),
            inset 0 0 3px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.2);
        --focus-border-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.9);
    }
    
    /* Success animation starting from focus state */
    @keyframes glowSuccess {
        0% {
            box-shadow: var(--focus-glow-shadow);
            border-color: var(--focus-border-color);
        }
        100% {
            border-color: var(--success-color-light);
            box-shadow:
                0 0 8px 2px var(--success-color-light),
                0 0 16px 4px hsla(145, 63%, 49%, 0.6),
                0 0 24px 6px hsla(145, 63%, 49%, 0.4),
                inset 0 0 3px hsla(145, 63%, 60%, 0.3);
        }
    }
    
    /* Error animation starting from focus state */
    @keyframes glowError {
        0% {
            box-shadow: var(--focus-glow-shadow);
            border-color: var(--focus-border-color);
        }
        100% {
            border-color: var(--error-color-light);
            box-shadow:
                0 0 8px 2px var(--error-color-light),
                0 0 16px 4px hsla(0, 84%, 60%, 0.6),
                0 0 24px 6px hsla(0, 84%, 60%, 0.4),
                inset 0 0 3px hsla(0, 84%, 70%, 0.3);
        }
    }
    
    /* Apply success animation */
    :global(.glow-success) {
        animation: glowSuccess 1s cubic-bezier(0.2, 0, 0.3, 1) forwards !important;
    }
    
    /* Apply error animation */
    :global(.glow-error) {
        animation: glowError 1s cubic-bezier(0.2, 0, 0.3, 1) forwards !important;
    }
    
    /* Removed potentially corrupted :global(.force-reset) rule */

    /* Enhanced throttling control styles */
    .disabled {
        opacity: 0.5;
        pointer-events: none;
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

    /* Add text shadows to make white text more legible on translucent backgrounds */
    input, select {
        text-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
    }
    
    /* Toggle switch styles for WebAssembly settings */
    .toggle-switch {
        position: relative;
        display: inline-block;
        width: 64px;  /* 1.5x from original 46px */
        height: 34px; /* 1.5x from original 24px */
    }
    
    .toggle-switch input {
        opacity: 0;
        width: 0;
        height: 0;
    }
    
    .slider {
        position: absolute;
        cursor: pointer;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(60, 60, 80, 0.6);
        transition: .4s;
        border-radius: 17px; /* Half of height */
        border: 2px solid hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.3);
    }
    
    .slider:before {
        position: absolute;
        content: "";
        height: 22px; /* Appropriately sized */
        width: 22px;  /* Appropriately sized */
        left: 6px;    /* Adjusted for new size */
        bottom: 4px;  /* Adjusted for new size */
        background-color: white;
        transition: .4s;
        border-radius: 50%;
    }
    
    input:checked + .slider {
        background-color: hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.8);
    }
    
    input:focus + .slider {
        box-shadow: 0 0 6px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.7);
    }
    
    input:checked + .slider:before {
        transform: translateX(30px); /* Adjusted for new width */
    }
    
    /* Setting row styles */
    .setting-row {
        display: grid;
        grid-template-columns: 1fr 1fr;
        align-items: center;
        gap: 1rem;
        padding: 0.75rem 0;
        border-bottom: 1px solid hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.1);
    }
    
    .setting-label {
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
        align-items: center;
        text-align: center;
    }
    
    .setting-description {
        font-size: 0.75rem;
        color: rgba(255, 255, 255, 0.6);
    }
    
    .setting-control {
        min-width: 120px;
        width: 100%;
        display: flex;
        justify-content: center;
        align-items: center;
    }
</style>