<script lang="ts">
    import { onMount } from 'svelte';
    import { slide, fade } from 'svelte/transition';
    
    import { settings } from '../lib/stores.ts';
    
    import { CheckLanguageCode } from '../../wailsjs/go/gui/App';
    
    export let isOpen = false;
    export let onClose: () => void;
    
    let currentSettings = {
        apiKeys: {
            replicate: '',
            assemblyAI: '',
            elevenLabs: ''
        },
        targetLanguage: '',
        nativeLanguage: '',
        enableGlow: true
    };
    
    let targetLangError = '';
    let nativeLangError = '';
    let isValid = true;

    // Validate both language codes
    async function validateLanguages() {
        const targetResponse = await CheckLanguageCode(settings.targetLanguage);
        const nativeResponse = await CheckLanguageCode(settings.nativeLanguage);

        targetLangError = !targetResponse.isValid && settings.targetLanguage 
            ? 'Invalid language code' 
            : '';
        nativeLangError = !nativeResponse.isValid && settings.nativeLanguage 
            ? 'Invalid language code' 
            : '';

        // Update validity
        isValid = (!settings.targetLanguage || targetResponse.isValid) && 
                 (!settings.nativeLanguage || nativeResponse.isValid);

        // Update standardized codes if valid
        if (targetResponse.isValid) {
            settings.targetLanguage = targetResponse.standardCode;
        }
        if (nativeResponse.isValid) {
            settings.nativeLanguage = nativeResponse.standardCode;
        }
    }
    
    onMount(async () => {
        try {
            const loadedSettings = await window.go.gui.App.LoadSettings();
            settings.set(loadedSettings);
            currentSettings = loadedSettings;
            await validateLanguages();
        } catch (error) {
            console.error('Failed to load settings:', error);
        }
    });

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
    
    // Subscribe to settings store
    settings.subscribe(value => {
        currentSettings = { ...value };
    });
</script>

{#if isOpen}
    <div class="fixed inset-0 bg-black/50 backdrop-blur-sm z-50 overflow-y-auto"
         transition:fade={{ duration: 200 }}
         on:click={onClose}>
        
        <div class="mx-auto max-w-lg p-4 my-4"
             transition:slide={{ duration: 300 }}
             on:click|stopPropagation>
            
            <div class="bg-[#252525] rounded-xl shadow-2xl border border-white/10 flex flex-col">
                <!-- Header -->
                <div class="p-6 border-b border-white/10">
                    <div class="flex items-center justify-between">
                        <h2 class="text-xl font-medium text-accent/90 flex items-center gap-2">
                            <span class="material-icons text-accent/70">settings</span>
                            Settings
                        </h2>
                        <button 
                            class="flex items-center justify-center w-8 h-8 p-0 hover:bg-white/10 
                                   rounded-full transition-colors aspect-square"
                            on:click={onClose}
                        >
                            <span class="material-icons">close</span>
                        </button>
                    </div>
                </div>
                
                <!-- Scrollable Content -->
                <div class="p-6 space-y-6">
                    <!-- Language Settings -->
                    <div class="space-y-4">
                        <h3 class="text-lg font-medium text-accent/80">Default Language Settings</h3>
                    </div>
                    <div class="grid grid-cols-2 gap-4">
                        <div class="space-y-2">
                            <label class="text-sm text-gray-300">Target Language</label>
                            <div class="relative">
                                <input
                                    type="text"
                                    bind:value={currentSettings.targetLanguage}
                                    on:blur={validateLanguages}
                                    class="w-full bg-sky-dark/50 border border-accent/30 rounded px-3 pr-8 py-2
                                           focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
                                           transition-colors duration-200
                                           {targetLangError ? 'border-red-500' : ''}"
                                    placeholder="e.g., jpn"
                                />
                                {#if targetLangError}
                                    <span class="absolute right-2 top-1/2 -translate-y-1/2
                                                material-icons text-[#ec5f67] text-sm">
                                        error
                                    </span>
                                {:else if currentSettings.targetLanguage}
                                    <span class="absolute right-2 top-1/2 -translate-y-1/2
                                                material-icons text-[#99c794] text-sm">
                                        check
                                    </span>
                                {/if}
                            </div>
                            {#if targetLangError}
                                <p class="text-red-500 text-sm">{targetLangError}</p>
                            {/if}
                        </div>

                        <div class="space-y-2">
                            <label class="text-sm text-gray-300">Native Language</label>
                            <div class="relative">
                                <input
                                    type="text"
                                    bind:value={currentSettings.nativeLanguage}
                                    on:blur={validateLanguages}
                                    class="w-full bg-sky-dark/50 border border-accent/30 rounded px-3 pr-8 py-2
                                           focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
                                           transition-colors duration-200
                                           {nativeLangError ? 'border-red-500' : ''}"
                                    placeholder="e.g., eng"
                                />
                                {#if nativeLangError}
                                    <span class="absolute right-2 top-1/2 -translate-y-1/2
                                                material-icons text-[#ec5f67] text-sm">
                                        error
                                    </span>
                                {:else if currentSettings.nativeLanguage}
                                    <span class="absolute right-2 top-1/2 -translate-y-1/2
                                                material-icons text-[#99c794] text-sm">
                                        check
                                    </span>
                                {/if}
                            </div>
                            {#if nativeLangError}
                                <p class="text-red-500 text-sm">{nativeLangError}</p>
                            {/if}
                        </div>
                    </div>

                    <!-- UI Settings -->
                    <div class="space-y-4">
                        <h3 class="text-lg font-medium text-accent/80">User Interface Settings</h3>
                        
                        <div class="flex items-center space-x-3">
                            <input
                                type="checkbox"
                                id="enableGlow"
                                bind:checked={currentSettings.enableGlow}
                                class="w-4 h-4 rounded border-accent/30 text-accent
                                       focus:ring-accent focus:ring-offset-0 focus:ring-1
                                       bg-sky-dark/50"
                            />
                            <label for="enableGlow" class="text-sm text-gray-300">
                                Enable Glow Effects (disable if UI is sluggish/stuttering)
                            </label>
                        </div>
                    </div>

                    <!-- API Keys -->
                    <div class="space-y-4">
                        <h3 class="text-lg font-medium text-accent/80">API Keys</h3>
                        {#each Object.entries(currentSettings.apiKeys) as [service, key]}
                            <div class="space-y-2">
                                <label class="text-sm text-gray-300 text-left">
                                    {service.charAt(0).toUpperCase() + service.slice(1).replace(/([A-Z])/g, ' $1')}
                                </label>
                                <input
                                    type="password"
                                    bind:value={currentSettings.apiKeys[service]}
                                    class="w-full bg-sky-dark/50 border border-accent/30 rounded px-3 py-2
                                           focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
                                           transition-colors duration-200"
                                    placeholder="Enter API key"
                                />
                            </div>
                        {/each}
                    </div>
                </div>
                
                <!-- Footer -->
                <div class="p-6 border-t border-white/10 flex justify-end gap-3 flex-shrink-0">
                    <button
                        class="px-4 py-2 text-white/70 hover:text-white
                               transition-colors duration-200"
                        on:click={onClose}
                    >
                        Cancel
                    </button>
                    <button
                        class="p-0 px-4 py-2 bg-accent text-sky-dark rounded-lg
                               transition-all duration-200 hover:bg-opacity-80"
                        on:click={saveSettings}
                    >
                        Save Settings
                    </button>
                </div>
            </div>
        </div>
    </div>
{/if}