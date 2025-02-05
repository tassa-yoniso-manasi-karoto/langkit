<script lang="ts">
    import { fade, slide } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import { onMount } from 'svelte';
    import '@material-design-icons/font';
    
    import { settings, showSettings } from './lib/stores.ts';
    import { logStore } from './lib/logStore';
    
    import MediaInput from './components/MediaInput.svelte';
    import FeatureSelector from './components/FeatureSelector.svelte';
    import LogViewer from './components/LogViewer.svelte';
    import GlowEffect from './components/GlowEffect.svelte';
    import Settings from './components/Settings.svelte';
    
    import { ProcessFiles } from '../wailsjs/go/gui/App';
    import { EventsOn } from '../wailsjs/runtime/runtime';

    // Define interfaces
    interface VideoInfo {
        name: string;
        path: string;
        size: number;
    }

    interface FeatureOptions {
        [key: string]: any;
    }

    // Component state
    let selectedFiles: VideoInfo[] = [];
    let selectedPath: string = '';
    let selectedFeatures = {
        subs2cards: false,
        dubtitles: false,
        voiceEnhancing: false,
        subtitleRomanization: false
    };
    let currentFeatureOptions: FeatureOptions | undefined;
    let isProcessing = false;
    let showLogViewer = false;
    let progress = 0;
    let showGlow = true;
    let defaultTargetLanguage = '';
    let featureValid = false;
    
    function handleValidityChange(event: CustomEvent) {
        featureValid = event.detail.isValid;
    }

    // Compute overall form validity
    $: isFormValid = selectedFiles.length > 0 && featureValid;

    // Event handlers
    function handleOptionsChange(event: CustomEvent<FeatureOptions>) {
        currentFeatureOptions = event.detail;
    }

    function toggleLogViewer() {
        showLogViewer = !showLogViewer;
    }

    async function handleProcess() {
        if (!currentFeatureOptions) return;
        
        isProcessing = true;
        showLogViewer = true;
        progress = 0;

        try {
            const request = {
                files: selectedFiles.map(f => f.path),
                selectedFeatures,
                options: currentFeatureOptions
            };

            await ProcessFiles(request);
        } catch (error) {
            console.error('Processing failed:', error);
        } finally {
            isProcessing = false;
            progress = 0;
        }
    }

    // Settings management
    async function loadSettings() {
        try {
            const loadedSettings = await window.go.gui.App.LoadSettings();
            settings.set(loadedSettings);
            showGlow = loadedSettings.enableGlow;
            defaultTargetLanguage = loadedSettings.targetLanguage;
        } catch (error) {
            console.error('Failed to load settings:', error);
        }
    }

    // Initialization
    onMount(() => {
        // Initialize log listener regardless of log viewer visibility
        EventsOn("log", (rawLog: any) => {
            logStore.addLog(rawLog);
        });
        
        // Listen for settings updates
        window.runtime.EventsOn("settings-loaded", (loadedSettings) => {
            settings.set(loadedSettings);
            showGlow = loadedSettings.enableGlow;
            defaultTargetLanguage = loadedSettings.targetLanguage;
            showLogViewer = loadedSettings.showLogViewerByDefault;
        });

        loadSettings();
    });

    $: if ($settings) {
        showLogViewer = $settings.showLogViewerByDefault;
    }

    // Settings update listener
    window.addEventListener('settingsUpdated', ((event: CustomEvent) => {
        settings.set(event.detail);
        showGlow = event.detail.enableGlow;
    }) as EventListener);
</script>

<div class="min-h-screen min-w-screen bg-bg text-gray-100 font-dm-sans fixed inset-0">
    {#if showGlow}
        <GlowEffect {isProcessing} />
    {/if}
    <div class="flex h-full p-8 gap-8 relative z-10">
        <div class="absolute top-4 right-4 z-20">
            <button 
                class="w-10 h-10 flex items-center justify-center rounded-lg bg-white/10 text-white/70
                       transition-all duration-200 hover:bg-white/15 hover:text-white
                       hover:-translate-y-0.5"
                on:click={() => $showSettings = true}
            >
                <span class="material-icons text-[20px]">settings</span>
            </button>
        </div>
        <!-- Main content area -->
        <div class="flex-1 relative {showLogViewer ? 'w-[55%]' : 'w-full'} transition-all duration-300">
            <div class="h-full flex flex-col">
                <!-- Scrollable content -->
                <div class="flex-1 overflow-y-auto pr-4 mask-fade">
                    <div class="max-w-2xl mx-auto space-y-6">
                        <MediaInput bind:selectedFiles />
                        <FeatureSelector 
                            bind:selectedFeatures 
                            on:optionsChange={handleOptionsChange}
                            on:validityChange={handleValidityChange}
                            {selectedFiles}
                            selectedPath={selectedPath}
                        />
                    </div>
                </div>

                <!-- Fixed bottom button area -->
    <div class="pt-6 pb-2 bg-gradient-to-t from-sky-dark via-sky-dark">
        <div class="max-w-2xl mx-auto flex justify-center items-center gap-4">
            <button 
                class="px-8 py-3 bg-accent text-sky-dark rounded-lg font-medium
                       transition-all duration-200 ease-in-out
                       disabled:opacity-50 disabled:cursor-not-allowed
                       hover:bg-opacity-80 hover:-translate-y-0.5
                       shadow-lg"
                disabled={!isFormValid || isProcessing} 
                on:click={handleProcess}
            >
                {#if isProcessing}
                    <div class="flex items-center gap-2">
                        <span class="material-icons animate-spin">refresh</span>
                        Processing...
                    </div>
                {:else}
                    Process Files
                {/if}
            </button>
            
            <button 
                class="p-2 rounded-lg transition-all duration-200
                       {showLogViewer ? 'bg-accent text-sky-dark' : 'bg-white/10 text-white'}
                       hover:bg-opacity-80"
                on:click={toggleLogViewer}
            >
                <span class="material-icons">
                    {showLogViewer ? 'chevron_right' : 'chevron_left'}
                </span>
            </button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Log viewer panel -->
        {#if showLogViewer}
            <div class="w-[45%] rounded-lg overflow-hidden transition-all duration-500 ease-out
                        shadow-[4px_4px_0_0_rgba(159,110,247,0.4),8px_8px_16px_-2px_rgba(159,110,247,0.35)]
                        hover:shadow-[4px_4px_0_0_rgba(159,110,247,0.5),8px_8px_20px_-2px_rgba(159,110,247,0.4)]"
                 in:slide={{ duration: 400, delay: 100, axis: 'x', easing: cubicOut }}
                 out:slide={{ duration: 400, axis: 'x', easing: cubicOut }}>
                <LogViewer />
            </div>
        {/if}
    </div>
</div>
<Settings 
    onClose={() => $showSettings = false}
/>

<style>
    /* Smooth fade mask for scrollable content */
    .mask-fade {
        mask-image: linear-gradient(
            to bottom,
            transparent,
            black 2%,
            black 98%,
            transparent
        );
        -webkit-mask-image: linear-gradient(
            to bottom,
            transparent,
            black 2%,
            black 98%,
            transparent
        );
        scrollbar-gutter: stable;
    }

    /* Smooth scrolling with inertia */
    .mask-fade {
        scroll-behavior: smooth;
        -webkit-overflow-scrolling: touch;
        overscroll-behavior: contain;
    }

    /* Hide scrollbar but keep functionality */
    .mask-fade::-webkit-scrollbar {
        width: 8px;
    }

    .mask-fade::-webkit-scrollbar-track {
        background: transparent;
    }

    .mask-fade::-webkit-scrollbar-thumb {
        background-color: rgba(255, 255, 255, 0.1);
        border-radius: 20px;
        border: 3px solid transparent;
        background-clip: content-box;
    }

    .mask-fade::-webkit-scrollbar-thumb:hover {
        background-color: rgba(255, 255, 255, 0.2);
    }
</style>