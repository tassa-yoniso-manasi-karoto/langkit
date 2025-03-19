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
    import BackgroundGradient from './components/BackgroundGradient.svelte';
    import Settings from './components/Settings.svelte';
    import ProcessButton from './components/ProcessButton.svelte';
    import UpdateNotification from './components/UpdateNotification.svelte';
    import ProgressManager from './components/ProgressManager.svelte';
    import LogViewerNotification from './components/LogViewerNotification.svelte';

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
    
    // Deferred loading state for feature selector - wait for main UI to render first
    let showFeatureSelector = false;
    
    // References for LogViewer button positioning
    let logViewerButton: HTMLButtonElement;
    let logViewerButtonPosition = { x: 0, y: 0 };

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
    }
    
    // Update LogViewer button position whenever processing state changes
    // This handles the slide appearance/disappearance of the cancel button
    $: {
        if (isProcessing !== undefined) {
            // Use setTimeout to allow DOM to update first
            setTimeout(() => {
                if (logViewerButton) {
                    const rect = logViewerButton.getBoundingClientRect();
                    logViewerButtonPosition = {
                        x: rect.left + rect.width / 2,
                        y: rect.top
                    };
                }
            }, 2000);
        }
    }
    
    $: {
        if (!Object.values(selectedFeatures).some(v => v)) {
            errorStore.addError({
                id: "no-features",
                message: "Select at least one processing feature",
                severity: "critical"
            });
        } else {
            errorStore.removeError("no-features");
        }
    }
    
    $: {
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

    // Helper to check for actual error logs (not user cancellations)
    function hasErrorLogs(): boolean {
        return $logStore.some(log => 
            (log.behavior === 'abort_task' && log.level.toUpperCase() === 'ERROR') || 
            (log.behavior === 'abort_all' && log.level.toUpperCase() === 'ERROR') ||
            (log.level.toUpperCase() === 'ERROR' && 
             (!log.behavior || log.behavior !== 'user_cancel') &&
             (!log.message || !log.message.toLowerCase().includes('cancel')))
        );
    }
    
    // Tooltip visibility state
    let tooltipDismissed = false;
    let tooltipVisible = false;
    
    // Show tooltip when hovering over the button (if errors exist)
    function handleLogButtonHover() {
        if (hasErrorLogs() && !showLogViewer && tooltipDismissed) {
            tooltipVisible = true;
            
            // Update button position to ensure tooltip is correctly positioned
            if (logViewerButton) {
                const rect = logViewerButton.getBoundingClientRect();
                logViewerButtonPosition = {
                    x: rect.left + rect.width / 2,
                    y: rect.top
                };
            }
        }
    }
    
    function toggleLogViewer() {
        showLogViewer = !showLogViewer;
        
        // Mark tooltip as dismissed once user has seen it
        if (hasErrorLogs()) {
            tooltipDismissed = true;
        }
        
        // Update button position after the transition completes
        setTimeout(() => {
            if (logViewerButton) {
                const rect = logViewerButton.getBoundingClientRect();
                logViewerButtonPosition = {
                    x: rect.left + rect.width / 2,
                    y: rect.top
                };
            }
        }, 2000); // Match transition duration from slide animation
    }

    async function handleProcess() {
        if (!currentFeatureOptions || !mediaSource) return;

        isProcessing = true;
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
            
            // Increment app start count before setting
            const updatedSettings = {
                ...loadedSettings,
                appStartCount: (loadedSettings.appStartCount || 0) + 1
            };
            
            settings.set(updatedSettings);
            showGlow = updatedSettings.enableGlow;
            defaultTargetLanguage = updatedSettings.targetLanguage;
            showLogViewer = updatedSettings.showLogViewerByDefault;
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
            //console.log(`[${timestamp}] Window state check - minimized: ${minimized}, previous state: ${isWindowMinimized}`);
            
            // Only update if state changed to avoid unnecessary re-renders
            if (minimized !== isWindowMinimized) {
                isWindowMinimized = minimized;
                
                // If window is minimized, reduce animations and processing
                if (minimized) {
                    // console.log(`[${timestamp}] 🔴 WINDOW MINIMIZED - reducing UI animations and processing`);
                    // logStore.addLog({
                    //     level: 'INFO',
                    //     message: '🔴 Window minimized - performance optimizations active',
                    //     time: timestamp
                    // });
                    
                    // Hide glow effect when minimized regardless of settings
                    showGlow = false;
                } else {
                    // console.log(`[${timestamp}] 🟢 WINDOW RESTORED - resuming normal operation`);
                    // logStore.addLog({
                    //     level: 'INFO',
                    //     message: '🟢 Window restored - normal performance mode',
                    //     time: timestamp
                    // });
                    
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
        
        // When window is minimized, we still want to track progress state
    // but we can skip visual updates to save resources
    if (isWindowMinimized) {
        // Log skipped updates stats
        skippedUpdateCount += pendingProgressUpdates.length;
        
        // Every 10 skipped updates, log summary
        if (skippedUpdateCount % 10 === 0) {
            // console.log(`[${new Date().toISOString()}] ⏭️ Throttled ${skippedUpdateCount} progress updates while minimized`);
        }
        
        // Process only the most recent update for each unique progress bar ID
        // This ensures state is maintained even when visual updates are skipped
        const latestUpdatesByID = new Map();
        pendingProgressUpdates.forEach(update => {
            latestUpdatesByID.set(update.id, update);
        });
        
        // Apply only the latest update for each bar to maintain state
        Array.from(latestUpdatesByID.values()).forEach(data => {
            updateProgressBar(data);
        });
        
        // Clear the queue after processing the latest updates
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
        
        // Calculate LogViewer button position for notifications
        const updateLogViewerButtonPosition = () => {
            if (logViewerButton) {
                const rect = logViewerButton.getBoundingClientRect();
                logViewerButtonPosition = {
                    x: rect.left + rect.width / 2,
                    y: rect.top
                };
            }
        };
        
        // Update position initially, on resize, and after transitions
        updateLogViewerButtonPosition();
        window.addEventListener('resize', updateLogViewerButtonPosition);
        
        // Also update on transition end events
        const handleTransitionEnd = (e: TransitionEvent) => {
            // Check if the transition is related to layout changes
            if (e.target && (e.target as HTMLElement).classList.contains('will-change-transform')) {
                updateLogViewerButtonPosition();
            }
        };
        document.addEventListener('transitionend', handleTransitionEnd);
        
        // Log initialization of performance monitoring
        console.log(`[${new Date().toISOString()}] 🚀 Initializing window-state based performance optimizations:
        - Window state checked every 2 seconds
        - Visual updates throttled when minimized
        - Progress updates batched for efficiency
        - Animations reduced when window not visible
        - Glow effect disabled when minimized
        - Log updates filtered when minimized
        - Deferred feature selector loading`);
        
        // Add to application logs
        logStore.addLog({
            level: 'INFO',
            message: '🚀 Initialized window-state performance optimizations',
            time: new Date().toISOString()
        });
        
        // Check window state every 2 seconds to optimize resource usage
        windowCheckInterval = window.setInterval(checkWindowState, 2000);
        
        // Defer loading of the Feature Selector component until main UI has rendered
        // This improves perceived performance and creates a nicer sequential reveal effect
        setTimeout(() => {
            console.log(`[${new Date().toISOString()}] 🎬 Showing feature selector component after UI shell render`);
            showFeatureSelector = true;
        }, 300); // 300ms gives UI shell time to render first
        
        // Initialize log listener with passive option for better performance
        EventsOn("log", (rawLog: any) => {
            // Always process logs even when minimized to maintain complete log history
            logStore.addLog(rawLog);
        });
        
        // Get version info on load
        GetVersion()
            .then((result: any) => {
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
            // Always add to pending updates queue to maintain state correctness
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
            window.removeEventListener('resize', updateLogViewerButtonPosition);
            document.removeEventListener('transitionend', handleTransitionEnd);
            
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
    <BackgroundGradient />
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
                        
                        <!-- Deferred loading of the FeatureSelector component -->
                        {#if showFeatureSelector}
                            <!-- Use fade-in animation for the feature selector -->
                            <div in:fade={{ duration: 300 }}>
                                <FeatureSelector
                                    bind:selectedFeatures
                                    bind:quickAccessLangTag
                                    bind:showLogViewer
                                    on:optionsChange={handleOptionsChange}
                                    {mediaSource}
                                    class="feature-selector"
                                />
                            </div>
                        {:else}
                            <!-- Placeholder with same height to prevent layout shift -->
                            <div class="h-20 rounded-lg bg-white/5 border-2 border-dashed border-primary/10 animate-pulse"></div>
                        {/if}
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
                        
                        <!-- Log viewer toggle button with notifications and pulsing effects -->
                        <div class="relative">
                            <button
                                class="h-12 w-12 flex items-center justify-center rounded-lg
                                       transition-all duration-200 will-change-transform
                                       {showLogViewer ? 'bg-primary text-sky-dark' : 'bg-white/10 text-white'}
                                       hover:bg-opacity-80 hover:-translate-y-0.5
                                       hover:shadow-lg
                                       focus:outline-none focus:ring-2
                                       {showLogViewer ? 'focus:ring-primary/50' : 'focus:ring-white/30'}
                                       focus:ring-offset-2 focus:ring-offset-bg
                                       {isProcessing && !showLogViewer && !hasErrorLogs() ? 'log-button-pulse' : ''}
                                       {hasErrorLogs() && !showLogViewer ? 'log-button-error-pulse' : ''}"
                                on:mouseenter={handleLogButtonHover}
                                on:click={toggleLogViewer}
                                aria-label="{showLogViewer ? 'Hide log viewer' : 'Show log viewer'}"
                                bind:this={logViewerButton}
                            >
                                <span class="material-icons">
                                    {showLogViewer ? "chevron_right" : "chevron_left"}
                                </span>
                                
                                <!-- Error indicator badge -->
                                {#if !showLogViewer && hasErrorLogs()}
                                    <span class="absolute -top-1 -right-1 h-4 w-4 bg-error-all rounded-full border border-white flex items-center justify-center text-[10px] text-white font-bold animate-pulse">
                                        !
                                    </span>
                                {/if}
                            </button>
                            
                            <!-- Log Viewer Notification -->
                            {#if ((isProcessing && !tooltipDismissed) || 
                                 hasErrorLogs() || 
                                 tooltipVisible
                                ) && !showLogViewer && logViewerButtonPosition}
                                <LogViewerNotification 
                                    position={logViewerButtonPosition} 
                                    mode={hasErrorLogs() ? 'error' : 'processing'}
                                    onOpenLogViewer={toggleLogViewer}
                                    onDismiss={() => {
                                        tooltipDismissed = true;
                                        tooltipVisible = false;
                                    }}
                                />
                            {/if}
                        </div>
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

    /* Loading animations */
    @keyframes pulse {
        0% { opacity: 0.5; }
        50% { opacity: 0.2; }
        100% { opacity: 0.5; }
    }
    
    .animate-pulse {
        animation: pulse 1.5s ease-in-out infinite;
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
    
    /* Log viewer button animations */
    @keyframes log-button-pulse {
        0% { box-shadow: 0 0 0 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0.4); }
        70% { box-shadow: 0 0 0 10px hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0); }
        100% { box-shadow: 0 0 0 0 hsla(var(--primary-hue), var(--primary-saturation), var(--primary-lightness), 0); }
    }
    
    @keyframes log-button-error-pulse {
        0% { box-shadow: 0 0 0 0 hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.4); }
        70% { box-shadow: 0 0 0 10px hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0); }
        100% { box-shadow: 0 0 0 0 hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0); }
    }
    
    .log-button-pulse {
        animation: log-button-pulse 2s infinite;
    }
    
    .log-button-error-pulse {
        animation: log-button-error-pulse 1.5s infinite;
        border: 1px solid hsla(var(--error-all-hue), var(--error-all-saturation), var(--error-all-lightness), 0.5);
    }
</style>
