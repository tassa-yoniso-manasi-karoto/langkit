<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { slide, fade } from 'svelte/transition';
    
    export let isOpen = false;
    export let onClose: () => void;
    
    let apiKeys = {
        replicate: '',
        assemblyAI: '',
        elevenLabs: ''
    };
    
    // Load saved API keys on mount
    onMount(async () => {
        const savedKeys = await window.localStorage.getItem('apiKeys');
        if (savedKeys) {
            apiKeys = JSON.parse(savedKeys);
        }
    });
    
    function saveSettings() {
        window.localStorage.setItem('apiKeys', JSON.stringify(apiKeys));
        onClose();
    }
</script>

{#if isOpen}
    <div class="fixed inset-0 bg-black/50 backdrop-blur-sm z-50"
         transition:fade={{ duration: 200 }}
         on:click={onClose}>
        
        <div class="fixed inset-x-0 top-[10%] mx-auto max-w-lg p-6"
             transition:slide={{ duration: 300 }}
             on:click|stopPropagation>
            
            <div class="bg-[#252525] rounded-xl shadow-2xl border border-white/10">
                <!-- Header -->
                <div class="p-6 border-b border-white/10">
                    <div class="flex items-center justify-between">
                        <h2 class="text-xl font-medium text-accent/90 flex items-center gap-2">
                            <span class="material-icons text-accent/70">settings</span>
                            Settings
                        </h2>
                        <button 
                            class="p-1 hover:bg-white/10 rounded-full transition-colors"
                            on:click={onClose}
                        >
                            <span class="material-icons">close</span>
                        </button>
                    </div>
                </div>
                
                <!-- Content -->
                <div class="p-6 space-y-6">
                    <div class="space-y-4">
                        {#each Object.entries(apiKeys) as [service, key]}
                            <div class="space-y-2">
                                <label class="text-sm text-gray-300">
                                    {service.replace(/([A-Z])/g, ' $1').trim()} API Key
                                </label>
                                <input
                                    type="password"
                                    bind:value={apiKeys[service]}
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
                <div class="p-6 border-t border-white/10 flex justify-end gap-3">
                    <button
                        class="px-4 py-2 text-white/70 hover:text-white
                               transition-colors duration-200"
                        on:click={onClose}
                    >
                        Cancel
                    </button>
                    <button
                        class="px-4 py-2 bg-accent text-sky-dark rounded-lg
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