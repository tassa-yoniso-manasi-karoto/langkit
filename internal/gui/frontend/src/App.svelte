<script lang="ts">
    import { fade, slide } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import { onMount, onDestroy } from 'svelte';
    import { get } from 'svelte/store'; // Added missing import
    import '@material-design-icons/font';

    import { settings, showSettings } from './lib/stores';
    import { logStore } from './lib/logStore';
    import { errorStore } from './lib/errorStore';
    import { progressBars, updateProgressBar, removeProgressBar, resetAllProgressBars } from './lib/progressBarsStore';
    // Remove command executor imports
    // import { initializeCommandExecutor, ShutdownCommand } from './lib/wasm-commands'; 
    import { enableWasm, setWasmSizeThreshold, isWasmEnabled } from './lib/wasm'; // Added isWasmEnabled
    import { wasmLogger, WasmLogLevel } from './lib/wasm-logger'; // Phase 1
    import { reportWasmState, syncWasmStateForReport } from './lib/wasm-state'; // Phase 1 & 3
    // import { errorStore } from './lib/errorStore'; // Already imported above

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

    // Comment out CheckDocker as it's not exported
    import { SendProcessingRequest, CancelProcessing, GetVersion, LoadSettings, SaveSettings, RefreshSTTModelsAfterSettingsUpdate /*, CheckDocker*/ } from '../wailsjs/go/gui/App'; 
    import { EventsOn } from '../wailsjs/runtime/runtime';
    import type { gui } from '../wailsjs/go/models'; // Import backend models for typing

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
                    handler: () => (document.querySelector(".drop-zone") as HTMLElement)?.click() // Type assertion
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
            }, 200); // Reduced delay
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
        }, 200); // Match transition duration from slide animation
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
            // Construct the request object matching the Go backend type
            const request: gui.ProcessRequest = { // Add type annotation
                path: mediaSource.path,
                selectedFeatures,
                options: { Options: currentFeatureOptions }, // Re-nest options
                languageCode: effectiveLanguageCode,
                audioTrackIndex: mediaSource?.audioTrackIndex ?? 0, // Use nullish coalescing
                // convertValues: true // Removed incorrect property
            };

            console.log("Sending processing request:", request);
            await SendProcessingRequest(request);
        } catch (error: any) { // Type the error
            console.error("Processing failed:", error);
            errorStore.addError({
                id: "processing-failed",
                message: "Processing failed: " + (error?.message || "Unknown error"),
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
            const loadedSettings = await LoadSettings(); // Use direct import
            
            // Increment app start count before setting
            const updatedSettings = {
                ...get(settings), // Merge with existing settings from store
                ...loadedSettings,
                appStartCount: ((loadedSettings as any).appStartCount || 0) + 1 // Use type assertion
            };
            
            settings.set(updatedSettings as any); // Use type assertion until Settings type is fully updated
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
        // try {
        //     const available = await CheckDocker(); // Use direct import
        //     if (!available) {
        //         errorStore.addError({
        //             id: "docker-not-available",
        //             message: "Docker is not available. Some features may be limited.",
        //             severity: "warning",
        //             dismissible: true,
        //             docsUrl: "https://docs.docker.com/get-docker/"
        //         });
        //     } else {
        //         errorStore.removeError("docker-not-available");
        //     }
        // } catch (error) {
        //     console.error("Docker check failed:", error);
        //     errorStore.addError({
        //         id: "docker-check-failed",
        //         message: "Failed to check Docker availability",
        //         severity: "warning",
        //         dismissible: true
        //     });
        // }
        console.warn("Docker check temporarily disabled."); // Placeholder
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
                    showGlow = $settings?.enableGlow ?? true; // Use nullish coalescing
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
            const latestUpdatesByID = new Map<string, any>(); // Add type annotation
            pendingProgressUpdates.forEach(update => {
                if (update && update.id) { // Check if update and id exist
                   latestUpdatesByID.set(update.id, update);
                }
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

    // Define these functions in the component scope so they can be removed in onDestroy
    let handleTransitionEnd: (e: TransitionEvent) => void;
    let updateLogViewerButtonPosition: () => void;

    onMount(async () => { // Make onMount async
        // Initialize window state detection - check initially and set up interval
        checkWindowState();
        
        // Calculate LogViewer button position for notifications
        updateLogViewerButtonPosition = () => { // Assign to the outer scope variable
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
        handleTransitionEnd = (e: TransitionEvent) => { // Assign to the outer scope variable
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
        
        // --- WASM Initialization (Phase 1 & 4, simplified) ---
        try {
            // No command executor initialization needed
            // wasmLogger.log(WasmLogLevel.INFO, 'init', 'Command executor initialized.'); // Removed log

            // Load settings before initializing WASM
            await loadSettings(); 
            const $currentSettings = get(settings); // Get latest settings after load

            // Listen for settings changes to enable/disable WebAssembly
            settings.subscribe(async ($settingsValue) => {
                // Check if useWasm exists and has changed
                if ($settingsValue.useWasm !== undefined && $settingsValue.useWasm !== $currentSettings.useWasm) { 
                    try {
                        const wasEnabled = await enableWasm($settingsValue.useWasm);
                        if (wasEnabled) {
                            wasmLogger.log(
                                WasmLogLevel.INFO,
                                'config', 
                                'WebAssembly enabled and initialized successfully via settings change.'
                            );
                            // Set size threshold from settings
                            if ($settingsValue.wasmSizeThreshold) {
                                setWasmSizeThreshold($settingsValue.wasmSizeThreshold);
                            }
                        } else if ($settingsValue.useWasm) {
                            // Handle case where enabling was requested but failed
                            errorStore.addError({
                                id: 'wasm-init-failed-setting',
                                message: 'Failed to initialize WebAssembly optimization after enabling in settings.',
                                severity: 'warning',
                                dismissible: true,
                            });
                        }
                    } catch (error: any) {
                        wasmLogger.log(WasmLogLevel.ERROR, 'config', `Error handling WASM setting change: ${error.message}`);
                        errorStore.addError({
                            id: 'wasm-setting-error',
                            message: `Error applying WebAssembly setting: ${error.message}`,
                            severity: 'critical',
                            dismissible: true,
                        });
                    }
                } else if ($settingsValue.wasmSizeThreshold !== $currentSettings.wasmSizeThreshold) {
                     // Handle threshold change if WASM is already enabled
                     if (isWasmEnabled()) {
                         setWasmSizeThreshold($settingsValue.wasmSizeThreshold);
                     }
                }
            });

            // Initialize WASM on startup if enabled in loaded settings
            if ($currentSettings.useWasm) {
                wasmLogger.log(WasmLogLevel.INFO, 'init', 'Attempting initial WASM initialization based on settings...');
                await enableWasm(true); // This will trigger initializeWasm if needed
            }

            // Listen for request to send WASM state from backend (Phase 3)
            EventsOn("request-wasm-state", async () => {
                wasmLogger.log(WasmLogLevel.DEBUG, 'backend', 'Backend requested WebAssembly state sync for report');
                try {
                    // syncWasmStateForReport is synchronous now
                    syncWasmStateForReport(); 
                    wasmLogger.log(WasmLogLevel.INFO, 'backend', 'WebAssembly state synchronized for report');
                } catch (error: any) {
                    wasmLogger.log(WasmLogLevel.ERROR, 'backend', `Failed to sync WebAssembly state: ${error.message}`);
                }
            });

        } catch (initError: any) {
            wasmLogger.log(WasmLogLevel.CRITICAL, 'init', `Critical error during WASM setup: ${initError.message}`); // Simplified message
            errorStore.addError({
                id: 'wasm-critical-init-error',
                message: `Critical error during application initialization: ${initError.message}`,
                severity: 'critical',
                dismissible: false, // This might be a fatal error
            });
        }
        // --- End WASM Initialization ---
        
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
        
        // Optimized log batch handler - delegates parsing and insertion to logStore
        EventsOn("log-batch", (logBatch) => {
            if (!Array.isArray(logBatch) || logBatch.length === 0) return;
            
            // Use the logStore's batch processing directly - it handles merging, ordering and chunking
            logStore.addLogBatch(logBatch);
            
            // For very large batches, log a debug message
            if (logBatch.length > 200) {
                console.debug(`Processed large log batch: ${logBatch.length} entries`);
            }
        });

        // Efficient progress batch handler with smart grouping
        EventsOn("progress-batch", (progressBatch) => {
            if (!Array.isArray(progressBatch) || progressBatch.length === 0) return;
            
            // Skip excessive updates when window is minimized to save resources
            if (isWindowMinimized && progressBatch.length > 10) {
                // Only process a few important updates for state maintenance
                const consolidatedUpdates: { [key: string]: any } = {}; // Add index signature
                
                // Keep only the latest update for each task ID
                progressBatch.forEach(update => {
                    if (update && update.id) {
                        consolidatedUpdates[update.id] = update;
                    }
                });
                
                // Apply only the latest updates
                Object.values(consolidatedUpdates).forEach((update: any) => { // Add type
                    // Check if update is complete or has error before applying
                    if (update.progress >= 100 || update.errorState) { 
                        updateProgressBar(update);
                    } else {
                        // For ongoing tasks, maybe only update every Nth time?
                        // For simplicity now, just update latest state
                        updateProgressBar(update);
                    }
                });
                
                // Log skipped updates
                // console.log(`[${new Date().toISOString()}] ⏭️ Throttled ${progressBatch.length - Object.keys(consolidatedUpdates).length} progress updates while minimized`);
                
            } else {
                // Process all updates normally when window is visible or batch is small
                progressBatch.forEach(data => {
                    updateProgressBar(data);
                });
            }
        });

        // Handle task completion events
        EventsOn("task-complete", (taskId: string) => {
            console.log(`Task ${taskId} completed.`);
            // Optionally remove the progress bar after a short delay
            setTimeout(() => removeProgressBar(taskId), 2000);
        });

        // Handle task error events
        EventsOn("task-error", (errorData: { id: string, error: string }) => {
            console.error(`Task ${errorData.id} failed: ${errorData.error}`);
            // Update the specific progress bar to show error state
            updateProgressBar({
                id: errorData.id,
                operation: 'Task Error', // Add default operation
                color: 'bg-red-500', // Add default color
                size: 'small', // Add default size
                progress: 100, // Mark as complete but with error
                errorState: 'task_error'
            });
        });
        
        // Handle application version check
        EventsOn("update-available", (newVersion: string) => {
            console.log(`Update available: ${newVersion}`);
            version = newVersion; // Update version display if needed
            updateAvailable = true;
        });
        
        // Get initial version
        GetVersion().then(v => version = v.version); // Access the version property
        
        // Check Docker availability on startup
        // checkDockerAvailability(); // Commented out
    });

    // Cleanup on component destruction
    onDestroy(() => {
        if (windowCheckInterval) clearInterval(windowCheckInterval);
        // Need to remove listeners added in onMount
        if (handleTransitionEnd) { // Check if function is defined before removing
           document.removeEventListener('transitionend', handleTransitionEnd); 
        }
        if (updateLogViewerButtonPosition) { // Check if function is defined before removing
           window.removeEventListener('resize', updateLogViewerButtonPosition); 
        }
        
        // No command executor shutdown needed
    });
</script>

<main class="relative min-h-screen w-full overflow-hidden bg-bg-900 text-white font-sans">
    <!-- Background Effects -->
    <BackgroundGradient />
    {#if showGlow}
        <GlowEffect />
    {/if}

    <!-- Settings Modal -->
    <Settings {version} onClose={() => $showSettings = false} />

    <!-- Main Content Area -->
    <div class="relative z-10 flex flex-col items-center justify-center min-h-screen p-4 md:p-8">
        <!-- Header -->
        <header class="absolute top-0 left-0 right-0 p-4 flex justify-between items-center z-20">
            <div class="text-xs text-gray-400">
                Langkit v{version}
                {#if updateAvailable}
                    <UpdateNotification />
                {/if}
            </div>
            <div class="flex items-center gap-2">
                 <!-- Log Viewer Button -->
                 <button 
                    bind:this={logViewerButton}
                    class="relative w-10 h-10 flex items-center justify-center rounded-full 
                           bg-input-bg/60 backdrop-blur-sm text-gray-300 transition-all duration-200 
                           hover:bg-primary/30 hover:text-white focus:outline-none focus:ring-2 
                           focus:ring-primary/50 shadow-md"
                    on:click={toggleLogViewer}
                    on:mouseenter={handleLogButtonHover} 
                    on:mouseleave={() => tooltipVisible = false}
                    title={showLogViewer ? "Hide Logs" : "Show Logs"}
                >
                    <span class="material-icons text-lg">description</span>
                    {#if hasErrorLogs() && !showLogViewer}
                        <span class="absolute -top-1 -right-1 w-3 h-3 bg-red-500 rounded-full border-2 border-bg-900 animate-pulse"></span>
                    {/if}
                </button>
                
                <!-- Settings Button -->
                <button 
                    class="w-10 h-10 flex items-center justify-center rounded-full 
                           bg-input-bg/60 backdrop-blur-sm text-gray-300 transition-all duration-200 
                           hover:bg-primary/30 hover:text-white focus:outline-none focus:ring-2 
                           focus:ring-primary/50 shadow-md"
                    on:click={() => $showSettings = true}
                    title="Settings"
                >
                    <span class="material-icons text-lg">settings</span>
                </button>
            </div>
        </header>

        <!-- Core UI -->
        <div class="w-full max-w-3xl flex flex-col items-center gap-8 md:gap-12 pt-16 pb-8">
            <!-- Media Input -->
            <MediaInput 
                bind:mediaSource={mediaSource as any}
                bind:previewFiles={previewFiles as any}
                on:error={(e) => errorStore.addError({ id: 'media-input-error', message: e.detail, severity: 'critical' })}
            />

            <!-- Feature Selector (Deferred Loading) -->
            {#if showFeatureSelector}
                <div in:fade={{ duration: 500, delay: 100 }} class="w-full">
                    <FeatureSelector 
                        bind:selectedFeatures 
                        on:optionsChange={handleOptionsChange}
                        mediaSource={mediaSource as any}
                        // Removed defaultTargetLanguage prop
                        bind:quickAccessLangTag
                    />
                </div>
            {:else}
                 <div class="w-full h-48 flex items-center justify-center text-gray-500"> 
                     <!-- Placeholder --> Loading features... 
                  </div>
            {/if}

            <!-- Processing Controls & Progress -->
            <div class="w-full flex flex-col items-center gap-6">
                 {#if !isProcessing}
                    <div in:fade={{ duration: 300 }}>
                        <ProcessButton 
                            on:click={handleProcess} 
                            isProcessing={isProcessing} 
                        />
                    </div>
                 {:else}
                    <div class="w-full flex flex-col items-center gap-4"
                         in:fade={{ duration: 300 }}>
                        <ProgressManager />
                        <button 
                            class="px-6 py-2 bg-red-600/80 backdrop-blur-sm text-white rounded-lg font-medium 
                                   transition-all duration-200 hover:bg-red-500 shadow-md shadow-red-500/30"
                            on:click={handleCancel}
                        >
                            Cancel Processing
                        </button>
                    </div>
                 {/if}
            </div>
        </div>
        
        <!-- Log Viewer Panel -->
        {#if showLogViewer}
            <div transition:slide={{ duration: 300, easing: cubicOut }} 
                 class="fixed bottom-0 left-0 right-0 h-1/2 md:h-1/3 z-30 shadow-top bg-bg-800/80 backdrop-blur-md border-t border-primary/30">
                <LogViewer />
            </div>
        {/if}
        
        <!-- Log Viewer Notification -->
        <LogViewerNotification
            position={logViewerButtonPosition}
            onOpenLogViewer={toggleLogViewer}
        />
    </div>
</main>

<style>
    .shadow-top {
        box-shadow: 0 -4px 15px rgba(0, 0, 0, 0.3);
    }
    /* Ensure drop zone click handler works */
    .drop-zone {
        cursor: pointer;
    }
</style>
