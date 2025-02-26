<script lang="ts">
    import { fade, slide } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import { onMount, onDestroy } from 'svelte';
    import '@material-design-icons/font';

    import { settings, showSettings } from './lib/stores';
    import { logStore } from './lib/logStore';
    import { errorStore } from './lib/errorStore';

    import MediaInput from './components/MediaInput.svelte';
    import FeatureSelector from './components/FeatureSelector.svelte';
    import LogViewer from './components/LogViewer.svelte';
    import GlowEffect from './components/GlowEffect.svelte';
    import Settings from './components/Settings.svelte';
    import ProcessButton from './components/ProcessButton.svelte';
    import UpdateNotification from './components/UpdateNotification.svelte';

    import { ProcessFiles, CancelProcessing, GetVersion } from '../wailsjs/go/gui/App';
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

    interface MediaSource {
        name: string;
        path: string;
        size?: number;
        audioTrackIndex?: number;
    }

    // Version info state variables
    let version: string = "";
    let updateAvailable: boolean = false;

    // Other state variables
    let mediaSource: MediaSource | null = null;
    let previewFiles: MediaSource[] = [];
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
    let defaultTargetLanguage = "";
    let quickAccessLangTag = "";

    // Reactive error management
    $: {
        if (!mediaSource) {
            errorStore.addError({
                id: "no-media",
                message: "No media file selected",
                severity: "critical",
                action: {
                    label: "Select Media",
                    handler: () => document.querySelector(".drop-zone")?.click()
                }
            });
        } else {
            errorStore.removeError("no-media");
        }

        if (!Object.values(selectedFeatures).some(v => v)) {
            errorStore.addError({
                id: "no-features",
                message: "Select at least one processing feature",
                severity: "critical"
            });
        } else {
            errorStore.removeError("no-features");
        }

        if (!$settings.nativeLanguages) {
            errorStore.addError({
                id: "no-native-lang",
                message: "Configure native languages in settings",
                severity: "warning",
                action: {
                    label: "Open Settings",
                    handler: () => $showSettings = true
                }
            });
        } else {
            errorStore.removeError("no-native-lang");
        }
    }

    function handleOptionsChange(event: CustomEvent<FeatureOptions>) {
        currentFeatureOptions = event.detail;
    }

    function toggleLogViewer() {
        showLogViewer = !showLogViewer;
    }

    async function handleProcess() {
        if (!currentFeatureOptions || !mediaSource) return;

        isProcessing = true;
        showLogViewer = true;
        progress = 0;

        // Use the quick access language tag if it differs from the default
        const effectiveLanguageCode = quickAccessLangTag && quickAccessLangTag !== defaultTargetLanguage
            ? quickAccessLangTag
            : defaultTargetLanguage;

        try {
            const request = {
                path: mediaSource.path,
                selectedFeatures,
                options: currentFeatureOptions,
                languageCode: effectiveLanguageCode,
                audioTrackIndex: mediaSource?.audioTrackIndex || 0
            };

            console.log("Sending processing request:", request);
            await ProcessFiles(request);
        } catch (error) {
            console.error("Processing failed:", error);
            errorStore.addError({
                id: "processing-failed",
                message: "Processing failed: " + (error.message || "Unknown error"),
                severity: "critical",
                dismissible: true
            });
        } finally {
            isProcessing = false;
            progress = 0;
        }
    }

    async function handleCancel() {
        try {
            await CancelProcessing();
            isProcessing = false;
            errorStore.addError({
                id: "processing-cancelled",
                message: "Processing cancelled by user",
                severity: "info",
                dismissible: true
            });
        } catch (error) {
            console.error("Failed to cancel processing:", error);
            errorStore.addError({
                id: "cancel-failed",
                message: "Failed to cancel processing",
                severity: "critical",
                dismissible: true
            });
        }
    }

    async function loadSettings() {
        try {
            const loadedSettings = await window.go.gui.App.LoadSettings();
            settings.set(loadedSettings);
            showGlow = loadedSettings.enableGlow;
            defaultTargetLanguage = loadedSettings.targetLanguage;
            showLogViewer = loadedSettings.showLogViewerByDefault;
        } catch (error) {
            console.error("Failed to load settings:", error);
            errorStore.addError({
                id: "settings-load-failed",
                message: "Failed to load settings",
                severity: "critical",
                dismissible: true,
                action: {
                    label: "Retry",
                    handler: () => loadSettings()
                }
            });
        }
    }

    async function checkDockerAvailability() {
        try {
            const available = await window.go.gui.App.CheckDocker();
            if (!available) {
                errorStore.addError({
                    id: "docker-not-available",
                    message: "Docker is not available. Some features may be limited.",
                    severity: "warning",
                    dismissible: true,
                    docsUrl: "https://docs.docker.com/get-docker/"
                });
            } else {
                errorStore.removeError("docker-not-available");
            }
        } catch (error) {
            console.error("Docker check failed:", error);
            errorStore.addError({
                id: "docker-check-failed",
                message: "Failed to check Docker availability",
                severity: "warning",
                dismissible: true
            });
        }
    }

    onMount(() => {
        // Initialize log listener
        EventsOn("log", (rawLog: any) => {
            logStore.addLog(rawLog);
        });

        // Listen for settings updates
        EventsOn("settings-loaded", (loadedSettings) => {
            settings.set(loadedSettings);
            showGlow = loadedSettings.enableGlow;
            defaultTargetLanguage = loadedSettings.targetLanguage;
            showLogViewer = loadedSettings.showLogViewerByDefault;
        });

        GetVersion()
            .then((result: any) => {
                console.log("GetVersion result:", result);
                version = result.version;
                updateAvailable = result.newerVersionAvailable;
            })
            .catch(err => {
                console.error("Failed to get version info:", err);
            });

        loadSettings();
        checkDockerAvailability();
    });

    onDestroy(() => {
        clearErrors();
    });

    window.addEventListener("settingsUpdated", ((event: CustomEvent) => {
        settings.set(event.detail);
        showGlow = event.detail.enableGlow;
    }) as EventListener);
</script>

<!-- Version display (fixed, using Tailwind and DM Mono) -->
<div class="fixed top-[0.5rem] right-[3.9rem] z-50 p-0 text-[0.6rem] text-gray-500 text-xs font-dm-mono">
    {#if version}
        {#if version === "dev"}
            {version}
        {:else}
            v{version}
        {/if}
        {#if updateAvailable}
            <UpdateNotification href="https://github.com/tassa-yoniso-manasi-karoto/langkit/releases">
                an update is available
            </UpdateNotification>
        {/if}
    {/if}
</div>

<!-- Main container now spans full viewport -->
<div class="w-screen h-screen bg-bg text-gray-100 font-dm-sans fixed inset-0">
    {#if showGlow}
        <GlowEffect {isProcessing} />
    {/if}

    <!-- Settings button container -->
    <div class="absolute top-4 right-4 z-20 flex items-center gap-4">
        <button
            class="w-10 h-10 flex items-center justify-center rounded-lg bg-white/10 text-white/70
                   transition-all duration-200 hover:bg-white/15 hover:text-white
                   hover:-translate-y-0.5 hover:shadow-lg hover:shadow-white/5
                   focus:outline-none focus:ring-2 focus:ring-accent/50"
            on:click={() => $showSettings = true}
            aria-label="Open settings"
        >
            <span class="material-icons text-[20px]">settings</span>
        </button>
    </div>

    <div class="flex h-full p-8 gap-8 relative z-10">
        <!-- Main content area -->
        <div class="flex-1 relative {showLogViewer ? 'w-[55%]' : 'w-full'} transition-all duration-300">
            <div class="h-full flex flex-col">
                <!-- Scrollable content -->
                <div class="flex-1 no-scrollbar overflow-y-auto pr-4 mask-fade">
                    <div class="max-w-2xl mx-auto space-y-6">
                        <MediaInput
                            bind:mediaSource
                            bind:previewFiles
                            class="drop-zone"
                        />
                        <FeatureSelector
                            bind:selectedFeatures
                            bind:quickAccessLangTag
                            bind:showLogViewer
                            on:optionsChange={handleOptionsChange}
                            {mediaSource}
                            class="feature-selector"
                        />
                    </div>
                </div>

                <!-- Fixed bottom button area -->
                <div class="pt-6 pb-2 bg-gradient-to-t from-sky-dark via-sky-dark">
                    <div class="max-w-2xl mx-auto flex justify-center items-center gap-4">
                        <ProcessButton
                            {isProcessing}
                            on:process={handleProcess}
                        />
                        {#if isProcessing}
                            <button
                                class="h-12 w-12 flex items-center justify-center rounded-lg
                                       bg-red-500/30 text-white transition-all duration-200
                                       hover:bg-red-500/90 hover:-translate-y-0.5
                                       hover:shadow-lg hover:shadow-red-500/20
                                       focus:outline-none focus:ring-2 focus:ring-red-500/50
                                       focus:ring-offset-2 focus:ring-offset-bg"
                                on:click={handleCancel}
                                in:slide={{ duration: 200, axis: "x" }}
                                out:slide={{ duration: 200, axis: "x" }}
                                aria-label="Cancel processing"
                            >
                                <span class="material-icons">close</span>
                            </button>
                        {/if}
                        <button
                            class="h-12 w-12 flex items-center justify-center rounded-lg
                                   transition-all duration-200
                                   {showLogViewer ? 'bg-accent text-sky-dark' : 'bg-white/10 text-white'}
                                   hover:bg-opacity-80 hover:-translate-y-0.5
                                   hover:shadow-lg
                                   focus:outline-none focus:ring-2
                                   {showLogViewer ? 'focus:ring-accent/50' : 'focus:ring-white/30'}
                                   focus:ring-offset-2 focus:ring-offset-bg"
                            on:click={toggleLogViewer}
                            aria-label="{showLogViewer ? 'Hide log viewer' : 'Show log viewer'}"
                        >
                            <span class="material-icons">
                                {showLogViewer ? "chevron_right" : "chevron_left"}
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
                 in:slide={{ duration: 400, delay: 100, axis: "x", easing: cubicOut }}
                 out:slide={{ duration: 400, axis: "x", easing: cubicOut }}
                 role="region"
                 aria-live="polite"
            >
                <LogViewer version={version} />
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
        black 7%,
        black 93%,
        transparent
    );
    -webkit-mask-image: linear-gradient(
        to bottom,
        transparent,
        black 7%,
        black 93%,
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

    :global(.settings-modal) {
        transition: opacity 0.3s ease-out, transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    }

    :global(.settings-modal.opened) {
        opacity: 1;
        transform: translateY(0);
    }
    
    .no-scrollbar {
        -ms-overflow-style: none;  /* IE and Edge */
        scrollbar-width: none;     /* Firefox */
    }
    .no-scrollbar::-webkit-scrollbar {
        display: none;             /* Chrome, Safari, Opera */
    }
</style>
