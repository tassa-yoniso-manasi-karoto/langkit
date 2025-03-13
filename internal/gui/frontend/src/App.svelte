<script lang="ts">
    import { fade, slide } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import { onMount, onDestroy } from 'svelte';
    import '@material-design-icons/font';

    import { settings, showSettings } from './lib/stores';
    import { logStore } from './lib/logStore';
    import { errorStore } from './lib/errorStore';
    import { progressBars, updateProgressBar, removeProgressBar, resetAllProgressBars } from './lib/progressBarsStore';
    
    // Import window API from Wails
    import { WindowIsMinimised, WindowIsMaximised } from '../wailsjs/runtime/runtime';

    import MediaInput from './components/MediaInput.svelte';
    import FeatureSelector from './components/FeatureSelector.svelte';
    import LogViewer from './components/LogViewer.svelte';
    import GlowEffect from './components/GlowEffect.svelte';
    import Settings from './components/Settings.svelte';
    import ProcessButton from './components/ProcessButton.svelte';
    import UpdateNotification from './components/UpdateNotification.svelte';
    import ProgressManager from './components/ProgressManager.svelte';

    import { SendProcessingRequest, CancelProcessing, GetVersion } from '../wailsjs/go/gui/App';
    import { EventsOn } from '../wailsjs/runtime/runtime';

    // Define interfaces
    interface VideoInfo {
        name: string;
        path: string;
        size: number;
    }

    interface FeatureOptions {
        [featureId: string]: {
            [optionId: string]: any;
        };
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
        subtitleRomanization: false,
        selectiveTransliteration: false,
        subtitleTokenization: false
    };
    let currentFeatureOptions: FeatureOptions | undefined;
    let isProcessing = false;
    let showLogViewer = false;
    let progress = 0;
    let showGlow = true;
    let defaultTargetLanguage = "";
    let quickAccessLangTag = "";
    
    // Window state tracking
    let isWindowMinimized = false;
    let isWindowMaximized = false;

    // Optimized reactive error management - track previous values to only update when needed
    let prevMediaSource = null;
    let prevFeaturesSelected = false;
    let prevNativeLanguages = null;
    
    // Throttle error updates to prevent too many updates in succession
    function throttledErrorUpdate() {
        // Check media source changes
        if (prevMediaSource !== mediaSource) {
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
            prevMediaSource = mediaSource;
        }

        // Check feature selection changes
        const featuresSelected = Object.values(selectedFeatures).some(v => v);
        if (prevFeaturesSelected !== featuresSelected) {
            if (!featuresSelected) {
                errorStore.addError({
                    id: "no-features",
                    message: "Select at least one processing feature",
                    severity: "critical"
                });
            } else {
                errorStore.removeError("no-features");
            }
            prevFeaturesSelected = featuresSelected;
        }

        // Check native languages changes
        if (prevNativeLanguages !== $settings.nativeLanguages) {
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
            prevNativeLanguages = $settings.nativeLanguages;
        }
    }
    
    // Handle all error updates in a single reactive statement with RAF for performance
    $: {
        if (mediaSource !== prevMediaSource || 
            Object.values(selectedFeatures).some(v => v) !== prevFeaturesSelected ||
            $settings.nativeLanguages !== prevNativeLanguages) {
            
            // Use requestAnimationFrame to batch updates to the next frame
            requestAnimationFrame(throttledErrorUpdate);
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
        
        // Completely clear all progress bars when starting a new process
        // This ensures we don't have lingering error states from previous runs
        progressBars.set([]);

        // Use the quick access language tag if it differs from the default
        const effectiveLanguageCode = quickAccessLangTag && quickAccessLangTag !== defaultTargetLanguage
            ? quickAccessLangTag
            : defaultTargetLanguage;

        try {
            const request = {
                path: mediaSource.path,
                selectedFeatures,
                options: { Options: currentFeatureOptions },
                languageCode: effectiveLanguageCode,
                audioTrackIndex: mediaSource?.audioTrackIndex || 0
            };

            console.log("Sending processing request:", request);
            await SendProcessingRequest(request);
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
            
            // Mark all current progress bars as cancelled
            progressBars.update(bars => {
                return bars.map(bar => ({
                    ...bar,
                    errorState: 'user_cancel'
                }));
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

    // Use a more efficient approach to handle events, with debouncing for frequent events
    let progressUpdateDebounceTimer: number | null = null;
    let pendingProgressUpdates: any[] = [];
    let windowCheckInterval: number | null = null;
    
    // Performance optimization based on window state
    async function checkWindowState() {
        try {
            // Check window minimized state
            const minimized = await WindowIsMinimised();
            const timestamp = new Date().toISOString();
            
            // Log every state check for verification
            console.log(`[${timestamp}] Window state check - minimized: ${minimized}, previous state: ${isWindowMinimized}`);
            
            // Only update if state changed to avoid unnecessary re-renders
            if (minimized !== isWindowMinimized) {
                isWindowMinimized = minimized;
                
                // If window is minimized, reduce animations and processing
                if (minimized) {
                    console.log(`[${timestamp}] 🔴 WINDOW MINIMIZED - reducing UI animations and processing`);
                    logStore.addLog({
                        level: 'INFO',
                        message: '🔴 Window minimized - performance optimizations active',
                        time: timestamp
                    });
                    
                    // Hide glow effect when minimized regardless of settings
                    showGlow = false;
                } else {
                    console.log(`[${timestamp}] 🟢 WINDOW RESTORED - resuming normal operation`);
                    logStore.addLog({
                        level: 'INFO',
                        message: '🟢 Window restored - normal performance mode',
                        time: timestamp
                    });
                    
                    // Restore glow effect based on user settings
                    showGlow = $settings?.enableGlow || true;
                }
                
                // Log specific optimization changes
                console.log(`[${timestamp}] Optimizations applied:
                - Glow effect: ${showGlow ? 'ENABLED' : 'DISABLED'}
                - Progress updates: ${minimized ? 'THROTTLED (10fps)' : 'NORMAL (60fps)'}
                - Log updates: ${minimized ? 'MINIMAL' : 'NORMAL'}
                - Animations: ${minimized ? 'REDUCED' : 'NORMAL'}`);
            }
            
            // Check maximized state too (could be used for enhancing UI on large screens)
            const maximized = await WindowIsMaximised();
            
            // Only log if maximized state changes
            if (maximized !== isWindowMaximized) {
                console.log(`[${timestamp}] Window maximized state changed: ${maximized}`);
                isWindowMaximized = maximized;
            }
            
        } catch (error) {
            console.error('Failed to check window state:', error);
        }
    }
    
    // Track performance metrics for verification
    let progressUpdateCount = 0;
    let lastProgressTimestamp = Date.now();
    let skippedUpdateCount = 0;
    
    // Function to process progress updates in batches
    function processProgressUpdates() {
        const now = Date.now();
        const timeSinceLastUpdate = now - lastProgressTimestamp;
        lastProgressTimestamp = now;
        
        // Skip visual updates if window is minimized to save resources
        if (isWindowMinimized) {
            // Log skipped updates stats
            skippedUpdateCount += pendingProgressUpdates.length;
            
            // Every 10 skipped updates, log summary
            if (skippedUpdateCount % 10 === 0) {
                console.log(`[${new Date().toISOString()}] ⏭️ Skipped ${skippedUpdateCount} progress updates while minimized`);
            }
            
            // Clear updates without processing them
            pendingProgressUpdates = [];
            progressUpdateDebounceTimer = null;
            return;
        }
        
        // Process all pending progress updates at once
        const updateCount = pendingProgressUpdates.length;
        progressUpdateCount += updateCount;
        
        // Log performance stats every 20 updates
        if (progressUpdateCount % 20 === 0) {
            console.log(`[${new Date().toISOString()}] ⚡ Progress update stats:
            - Batch size: ${updateCount} updates
            - Time since last update: ${timeSinceLastUpdate}ms
            - Update interval: ${isWindowMinimized ? 'throttled (100ms)' : 'normal (16ms)'}
            - Total updates: ${progressUpdateCount}
            - Window state: ${isWindowMinimized ? 'minimized' : 'normal'}`);
        }
        
        // Apply updates
        pendingProgressUpdates.forEach(data => {
            updateProgressBar(data);
        });
        
        pendingProgressUpdates = [];
        progressUpdateDebounceTimer = null;
    }

    onMount(() => {
        // Initialize window state detection - check initially and set up interval
        checkWindowState();
        
        // Log initialization of performance monitoring
        console.log(`[${new Date().toISOString()}] 🚀 Initializing window-state based performance optimizations:
        - Window state checked every 2 seconds
        - Visual updates throttled when minimized
        - Progress updates batched for efficiency
        - Animations reduced when window not visible
        - Glow effect disabled when minimized
        - Log updates filtered when minimized`);
        
        // Add to application logs
        logStore.addLog({
            level: 'INFO',
            message: '🚀 Initialized window-state performance optimizations',
            time: new Date().toISOString()
        });
        
        // Check window state every 2 seconds to optimize resource usage
        windowCheckInterval = window.setInterval(checkWindowState, 2000);
        
        // Initialize log listener with passive option for better performance
        EventsOn("log", (rawLog: any) => {
            // Skip log UI updates if window is minimized
            if (isWindowMinimized) return;
            
            // Log events won't trigger reflow, so we can add them directly
            logStore.addLog(rawLog);
        });
        
        // Get version info on load
        GetVersion()
            .then((result: any) => {
                // Avoid unnecessary console.log in production
                version = result.version;
                updateAvailable = result.newerVersionAvailable;
            })
            .catch(err => {
                console.error("Failed to get version info:", err);
            });

        // Listen for settings updates
        EventsOn("settings-loaded", (loadedSettings) => {
            // Batch updates to reduce reflows
            requestAnimationFrame(() => {
                settings.set(loadedSettings);
                showGlow = loadedSettings.enableGlow;
                defaultTargetLanguage = loadedSettings.targetLanguage;
                showLogViewer = loadedSettings.showLogViewerByDefault;
            });
        });

        // Load settings
        loadSettings();
        
        // Batch progress updates for better performance
        EventsOn("progress", (data) => {
            // Skip visual updates entirely if window is minimized
            if (isWindowMinimized && !data.critical) {
                return;
            }
            
            // Add to pending updates queue
            pendingProgressUpdates.push(data);
            
            // Adjust frame rate based on window state
            // Use slower updates when minimized or not processing
            const updateInterval = isWindowMinimized ? 100 : 16; // 10fps when minimized vs 60fps
            
            // Debounce updates to process multiple progress updates in a single frame
            if (!progressUpdateDebounceTimer) {
                progressUpdateDebounceTimer = window.setTimeout(processProgressUpdates, updateInterval);
            }
        });

        // These events are less frequent, so we can process them immediately
        EventsOn("progress-remove", (barID: string) => {
            removeProgressBar(barID);
        });
        
        EventsOn("progress-reset", () => {
            resetAllProgressBars();
        });
        
        // Check Docker availability
        checkDockerAvailability();
        
        // Add settings update listener using more efficient approach
        const handleSettingsUpdated = ((event: CustomEvent) => {
            settings.set(event.detail);
            showGlow = event.detail.enableGlow;
        }) as EventListener;
        
        window.addEventListener("settingsUpdated", handleSettingsUpdated, { passive: true });
        
        // Clean up all event listeners and timers on component destruction
        return () => {
            window.removeEventListener("settingsUpdated", handleSettingsUpdated);
            
            // Clear progress update timer
            if (progressUpdateDebounceTimer) {
                clearTimeout(progressUpdateDebounceTimer);
                processProgressUpdates(); // Process any remaining updates
            }
            
            // Clear window check interval
            if (windowCheckInterval) {
                clearInterval(windowCheckInterval);
                windowCheckInterval = null;
            }
            
            errorStore.clearErrors();
        };
    });
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
    {#if showGlow && !isWindowMinimized}
        <GlowEffect {isProcessing} />
    {/if}

    <!-- Settings button container -->
    <div class="absolute top-4 right-4 z-20 flex items-center gap-4">
        <button
            class="w-10 h-10 flex items-center justify-center rounded-lg bg-white/10 text-white/70
                   transition-all duration-200 hover:bg-white/15 hover:text-white
                   hover:-translate-y-0.5 hover:shadow-lg hover:shadow-white/5
                   focus:outline-none focus:ring-2 focus:ring-primary/50"
            on:click={() => $showSettings = true}
            aria-label="Open settings"
        >
            <span class="material-icons text-[20px]">settings</span>
        </button>
    </div>

    <div class="flex h-full p-8 gap-8 relative z-10">
        <!-- Main content area with width optimization to prevent layout thrashing -->
        <div class="flex-1 relative will-change-transform" 
             style="width: {showLogViewer ? '55%' : '100%'}; transition: width 300ms ease-out;">
            <div class="h-full flex flex-col">
                <!-- Scrollable content with optimizations -->
                <div class="flex-1 no-scrollbar overflow-y-auto pr-4 mask-fade">
                    <div class="max-w-2xl mx-auto space-y-6 will-change-transform contain-layout">
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

                <!-- Fixed bottom area -->
                <div class="pt-4 pb-1 bg-gradient-to-t from-sky-dark via-sky-dark">
                    <!-- Progress Manager with minimal spacing -->
                    <div class="mb-2">
                        <ProgressManager {isProcessing}/>
                    </div>
                    
                    <!-- Process Button Row with hardware acceleration -->
                    <div class="max-w-2xl mx-auto flex justify-center items-center gap-4 pb-0 will-change-transform">
                        <ProcessButton
                            {isProcessing}
                            on:process={handleProcess}
                        />
                        
                        <!-- Cancel button with optimized transitions -->
                        {#if isProcessing}
                            <div class="h-12 w-12" style="contain: strict;">
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
                            </div>
                        {/if}
                        
                        <!-- Log viewer toggle button with optimized transitions -->
                        <button
                            class="h-12 w-12 flex items-center justify-center rounded-lg
                                   transition-all duration-200 will-change-transform
                                   {showLogViewer ? 'bg-primary text-sky-dark' : 'bg-white/10 text-white'}
                                   hover:bg-opacity-80 hover:-translate-y-0.5
                                   hover:shadow-lg
                                   focus:outline-none focus:ring-2
                                   {showLogViewer ? 'focus:ring-primary/50' : 'focus:ring-white/30'}
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

        <!-- Log viewer panel with optimized rendering -->
        {#if showLogViewer}
            <div class="w-[45%] rounded-lg overflow-hidden will-change-transform
                        shadow-[4px_4px_0_0_rgba(159,110,247,0.4),8px_8px_16px_-2px_rgba(159,110,247,0.35)]
                        hover:shadow-[4px_4px_0_0_rgba(159,110,247,0.5),8px_8px_20px_-2px_rgba(159,110,247,0.4)]"
                 style="transform: translateZ(0); contain: content;"
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
