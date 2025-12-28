<script lang="ts">
    import { fade, slide } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import { onMount, onDestroy } from 'svelte';
    import { get } from 'svelte/store';
    import '@material-design-icons/font';

    import { settings, showSettings, wasmActive, statisticsStore, welcomePopupVisible, userActivityState as userActivityStateStore, dockerStatusStore, internetStatusStore, ffmpegStatusStore, mediainfoStatusStore, systemInfoStore, llmStateStore, liteModeStore } from './lib/stores';
    import { nvidiaGPUStore } from './lib/nvidiaGPUStore';
    import { logStore } from './lib/logStore';
    import { invalidationErrorStore } from './lib/invalidationErrorStore';
    import { currentSchemeNeedsDockerStore } from './lib/featureGroupStore';
    import { logger } from './lib/logger';
    import { wsClient } from './ws/client';
    import { progressBars, updateProgressBar, removeProgressBar, resetAllProgressBars } from './lib/progressBarsStore';
    import { enableWasm, isWasmEnabled, getWasmModule } from './lib/wasm'; // Removed setWasmSizeThreshold
    import { reportWasmState, syncWasmStateForReport, getWasmState } from './lib/wasm-state';

    // Import runtime utilities
    import {
        safeWindowIsMinimised,
        safeWindowIsMaximised,
        initializeDragDrop,
        cleanupDragDrop,
        initializeRuntimeStores,
        isAnkiMode
    } from './lib/runtime/stores';

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
    import DevDashboard from './components/DevDashboard.svelte';
    import CoffeeSupport from './components/CoffeeSupport.svelte';
    import WelcomePopup from './components/WelcomePopup.svelte';
    import ReturnToAnkiButton from './components/ReturnToAnkiButton.svelte';
    import ChangelogPopup from './components/ChangelogPopup.svelte';
    
    import { GetVersion, GetSystemInfo, CheckForUpdate, ShowWarning } from './api/services/system';
    import { getGraphicsInfo } from './lib/graphicsInfo';
    import { SendProcessingRequest, CancelProcessing, GetProcessingStatus } from './api/services/processing';
    import {
        CheckDockerAvailability,
        CheckInternetConnectivity,
        CheckFFmpegAvailability,
        CheckMediaInfoAvailability
    } from './api/services/deps';
    import {
        LoadSettings,
        SaveSettings,
        LoadStatistics,
        UpdateStatistics,
        IncrementStatistic
    } from './api/services/settings';
    import { RefreshSTTModelsAfterSettingsUpdate } from './api/services/models';
    import type { gui } from '../wailsjs/go/models';

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

    let mediaSource: MediaSource | null = null;
    let previewFiles: MediaSource[] = [];
    let selectedFeatures = {
        subs2cards: false,
        dubtitles: false,
        voiceEnhancing: false,
        subtitleRomanization: false,
        selectiveTransliteration: false,
        subtitleTokenization: false,
        condensedAudio: false
    };
    let currentFeatureOptions: FeatureOptions | undefined;
    let isProcessing = false;
    let processingStartTime: number = 0;
    let showLogViewer = false;
    let progress = 0;
    let showGlow = true;
    let defaultTargetLanguage = "";
    let quickAccessLangTag = "";
    
    
    // Window state tracking
    let isWindowMinimized = false;
    let isWindowMaximized = false;
    
    // User activity states
    const UserActivityState = {
        ACTIVE: 'active',           // User is currently active
        IDLE: 'idle',              // No activity for short period (5s - 5min)
        AFK: 'afk'                 // Away from keyboard for long period (>5min)
    } as const;
    type UserActivityStateType = typeof UserActivityState[keyof typeof UserActivityState];
    
    // User activity tracking
    let userActivityState: UserActivityStateType = UserActivityState.ACTIVE;
    let userActivityTimer: ReturnType<typeof setTimeout> | null = null;
    let afkTimer: ReturnType<typeof setTimeout> | null = null;
    let lastActivityTime = Date.now();
    let isActivityStateForced = false;
    
    // Subscribe to the store to get forced state changes
    const unsubscribeActivityState = userActivityStateStore.subscribe(value => {
        if (value.isForced) {
            userActivityState = value.state;
            isActivityStateForced = true;
            logger.trace('app', 'User activity state forced', { state: value.state });
        } else if (isActivityStateForced && !value.isForced) {
            // Reset forced flag when store is reset
            isActivityStateForced = false;
        }
    });
    
    // Timeouts for state transitions
    const IDLE_TIMEOUT = 5000;        // 5 seconds to idle
    const AFK_TIMEOUT = 300000;       // 5 minutes to AFK
    
    // Deferred loading state for feature selector - wait for main UI to render first
    let showFeatureSelector = false;
    
    // References for LogViewer button positioning
    let logViewerButton: HTMLButtonElement;
    let logViewerButtonPosition = { x: 0, y: 0 };
    
    // Cancel button reference for fill effect
    let cancelButtonRef: HTMLButtonElement | null = null;

    // Track lite mode for conditional backdrop-filter (Qt+Windows flickering workaround)
    $: liteMode = $liteModeStore.enabled;

    // Reactive tracking of language settings
    // A reactive statement to sync settings target language with quick access language
    $: {
        if ($settings && $settings.targetLanguage !== undefined) {
            // When the settings target language changes, update both the default and quick access
            if ($settings.targetLanguage !== defaultTargetLanguage) {
                logger.trace('settings', `Target language changed: ${defaultTargetLanguage} → ${$settings.targetLanguage}`);
                defaultTargetLanguage = $settings.targetLanguage;
                quickAccessLangTag = $settings.targetLanguage;
            }
        }
    }
    
    
    // State for welcome popup
    let showWelcomePopup = false;

    // State for changelog popup - triggers check after welcome popup closes
    let triggerChangelogCheck = false;

    // Sync welcome popup state with store
    $: welcomePopupVisible.set(showWelcomePopup);
    
    // Global drag and drop state
    let globalDragOver = false;
    let droppedFilePath: string | null = null;
    
    // Combined activity state: window minimized = immediate AFK (unless forced)
    $: {
        if (isWindowMinimized && userActivityState !== UserActivityState.AFK && !isActivityStateForced) {
            userActivityState = UserActivityState.AFK;
            userActivityStateStore.set(UserActivityState.AFK, false);
            logger.trace('app', 'User marked as AFK due to window minimization');
        }
    }
    
    // Helper functions for checking activity states
    function isUserActive(): boolean {
        return userActivityState === UserActivityState.ACTIVE && !isWindowMinimized;
    }
    
    function isUserIdle(): boolean {
        return userActivityState === UserActivityState.IDLE && !isWindowMinimized;
    }
    
    function isUserAFK(): boolean {
        return userActivityState === UserActivityState.AFK || isWindowMinimized;
    }
    
    // Robust user activity detection with 3-state system
    function handleUserActivity() {
        // Don't override forced states
        if (isActivityStateForced) {
            return;
        }
        
        const now = Date.now();
        const previousState = userActivityState;
        
        lastActivityTime = now;
        
        // Clear existing timers
        if (userActivityTimer) {
            clearTimeout(userActivityTimer);
            userActivityTimer = null;
        }
        if (afkTimer) {
            clearTimeout(afkTimer);
            afkTimer = null;
        }
        
        // Don't change state if window is minimized
        if (isWindowMinimized) {
            return;
        }
        
        // Set state to active
        userActivityState = UserActivityState.ACTIVE;
        userActivityStateStore.set(UserActivityState.ACTIVE, false);
        
        // Log state transition
        if (previousState !== UserActivityState.ACTIVE) {
            logger.trace('app', 'User activity state changed', { 
                from: previousState, 
                to: userActivityState 
            });
        }
        
        // Set timer for transition to IDLE
        userActivityTimer = setTimeout(() => {
            if (!isActivityStateForced) {
                userActivityState = UserActivityState.IDLE;
                userActivityStateStore.set(UserActivityState.IDLE, false);
                logger.trace('app', 'User marked as IDLE due to inactivity');
                
                // Set timer for transition to AFK
                afkTimer = setTimeout(() => {
                    if (!isActivityStateForced) {
                        userActivityState = UserActivityState.AFK;
                        userActivityStateStore.set(UserActivityState.AFK, false);
                        logger.trace('app', 'User marked as AFK due to extended inactivity');
                    }
                }, AFK_TIMEOUT - IDLE_TIMEOUT);
            }
        }, IDLE_TIMEOUT);
    }

    // Reactive error management
    $: {
        if (!mediaSource) {
            logger.trace('app', 'No media source selected');
            invalidationErrorStore.addError({
                id: "no-media",
                message: "No media file selected",
                severity: "critical",
                action: {
                    label: "Select Media",
                    handler: () => (document.querySelector(".drop-zone") as HTMLElement)?.click() // Type assertion
                }
            });
        } else {
            logger.info('app', 'Media source selected', { 
                path: mediaSource.path, 
                name: mediaSource.name,
                size: mediaSource.size,
                audioTrackIndex: mediaSource.audioTrackIndex 
            });
            invalidationErrorStore.removeError("no-media");
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

    // Communicate processing state to Anki addon via document title
    // This allows the addon to warn users when closing Anki during processing
    $: if ($isAnkiMode) {
        document.title = isProcessing ? '__LANGKIT_STATE:processing' : '__LANGKIT_STATE:idle';
    }

    // Expose global function for backend to trigger title update after command processing
    $: if ($isAnkiMode) {
        (window as any).__langkitUpdateTitle = () => {
            document.title = isProcessing ? '__LANGKIT_STATE:processing' : '__LANGKIT_STATE:idle';
        };
    }
    
    $: {
        if (!Object.values(selectedFeatures).some(v => v)) {
            logger.trace('app', 'No features selected');
            invalidationErrorStore.addError({
                id: "no-features",
                message: "Select at least one processing feature",
                severity: "critical"
            });
        } else {
            const activeFeatures = Object.entries(selectedFeatures)
                .filter(([_, enabled]) => enabled)
                .map(([feature, _]) => feature);
            logger.debug('app', 'Features selected', { activeFeatures });
            invalidationErrorStore.removeError("no-features");
        }
    }
    
    $: {
        if (!$settings.nativeLanguages) {
            invalidationErrorStore.addError({
                id: "no-native-lang",
                message: "Configure native languages in settings",
                severity: "warning",
                action: {
                    label: "Open Settings",
                    handler: () => $showSettings = true
                }
            });
        } else {
            invalidationErrorStore.removeError("no-native-lang");
        }
    }
    
    // Check Docker requirements for romanization/transliteration features
    $: {
        const dockerChecked = $dockerStatusStore.checked;
        const dockerAvailable = $dockerStatusStore.available;

        // Check if subtitle linguistic processing features are selected
        const linguisticFeaturesSelected = selectedFeatures.subtitleRomanization ||
                                         selectedFeatures.selectiveTransliteration ||
                                         selectedFeatures.subtitleTokenization;

        // Check if current scheme requires Docker
        const requiresDocker = linguisticFeaturesSelected && $currentSchemeNeedsDockerStore;

        // Only create error if Docker is not available AND the current scheme actually requires Docker
        if (dockerChecked && !dockerAvailable && requiresDocker) {
            let message = "The selected romanization provider requires Docker to function.";
            if (window.navigator.platform.includes("Win")) {
                message += " On Windows Home, Docker requires WSL. See the 'A Note on Specific Languages' page for more info.";
            }
            invalidationErrorStore.addError({
                id: "docker-required",
                message: message,
                severity: "critical",
                dismissible: false,
                docsUrl: "https://docs.docker.com/get-docker/"
            });
        } else {
            invalidationErrorStore.removeError("docker-required");
        }
    }

    // Check Docker requirements for voice enhancing (docker-based demucs providers)
    $: {
        const dockerChecked = $dockerStatusStore.checked;
        const dockerAvailable = $dockerStatusStore.available;

        // Check if voice enhancing is selected with a docker-based provider
        const sepLib = currentFeatureOptions?.voiceEnhancing?.sepLib as string | undefined;
        const voiceEnhancingNeedsDocker = selectedFeatures.voiceEnhancing &&
                                          sepLib &&
                                          sepLib.startsWith('docker-');

        if (dockerChecked && !dockerAvailable && voiceEnhancingNeedsDocker) {
            let message = "The selected voice separation provider requires Docker to function.";
            if (window.navigator.platform.includes("Win")) {
                message += " On Windows Home, Docker requires WSL.";
            }
            invalidationErrorStore.addError({
                id: "docker-required-voice",
                message: message,
                severity: "critical",
                dismissible: false,
                docsUrl: "https://docs.docker.com/get-docker/"
            });
        } else {
            invalidationErrorStore.removeError("docker-required-voice");
        }
    }

    // Check Internet requirements for selected features
    $: {
        const internetChecked = $internetStatusStore.checked;
        const internetOnline = $internetStatusStore.online;
        
        // Check if subtitle linguistic processing features are selected
        const linguisticFeaturesSelected = selectedFeatures.subtitleRomanization || 
                                         selectedFeatures.selectiveTransliteration || 
                                         selectedFeatures.subtitleTokenization;
        
        // Check if AI-powered features are selected
        const aiPoweredFeaturesSelected = selectedFeatures.dubtitles || 
                                        selectedFeatures.voiceEnhancing ||
                                        (selectedFeatures.condensedAudio && currentFeatureOptions?.condensedAudio?.enableSummary);
        
        // Need internet if any processing features are selected (all require internet)
        const needsInternet = aiPoweredFeaturesSelected || linguisticFeaturesSelected;
        
        if (internetChecked && !internetOnline && needsInternet) {
            let message = "An internet connection is required for ";
            if (aiPoweredFeaturesSelected && linguisticFeaturesSelected) {
                message += "AI-powered features and linguistic processing.";
            } else if (aiPoweredFeaturesSelected) {
                message += "AI-powered features (dubtitles, voice enhancing).";
            } else {
                message += "linguistic processing.";
            }
            
            invalidationErrorStore.addError({
                id: "internet-required",
                message: message,
                severity: "critical",
                dismissible: false
            });
        } else {
            invalidationErrorStore.removeError("internet-required");
        }
    }
    
    // Check FFmpeg availability - required for all media processing
    $: {
        const ffmpegChecked = $ffmpegStatusStore.checked;
        const ffmpegAvailable = $ffmpegStatusStore.available;
        
        if (ffmpegChecked && !ffmpegAvailable) {
            invalidationErrorStore.addError({
                id: "ffmpeg-required",
                message: "FFmpeg is required for media processing. Please install FFmpeg to use Langkit.",
                severity: "critical",
                dismissible: false,
                docsUrl: "https://ffmpeg.org/download.html"
            });
        } else {
            invalidationErrorStore.removeError("ffmpeg-required");
        }
    }
    
    // Check MediaInfo availability - required for media analysis
    $: {
        const mediainfoChecked = $mediainfoStatusStore.checked;
        const mediainfoAvailable = $mediainfoStatusStore.available;
        
        if (mediainfoChecked && !mediainfoAvailable) {
            invalidationErrorStore.addError({
                id: "mediainfo-required",
                message: "MediaInfo is required for media analysis. Please install MediaInfo to use Langkit.",
                severity: "critical",
                dismissible: false,
                docsUrl: "https://mediaarea.net/en/MediaInfo/Download"
            });
        } else {
            invalidationErrorStore.removeError("mediainfo-required");
        }
    }
    
    function handleOptionsChange(event: CustomEvent<FeatureOptions>) {
        currentFeatureOptions = event.detail;
        logger.debug('app', 'Feature options changed', { options: event.detail });
    }

    // Helper to check for actual error logs (not user cancellations)
    function hasErrorLogs(): boolean {
        return $logStore.some(log => {
            const logTime = log._unix_time || 0;
            
            // If processing, only show errors from current run
            if (isProcessing && processingStartTime > 0) {
                if (logTime < processingStartTime) return false;
            } else {
                // When not processing, only show recent errors (last 5 minutes)
                const fiveMinutesAgo = Date.now() - (5 * 60 * 1000);
                if (logTime < fiveMinutesAgo) return false;
            }
            
            return (log.behavior === 'abort_task' && log.level.toUpperCase() === 'ERROR') || 
                   (log.behavior === 'abort_all' && log.level.toUpperCase() === 'ERROR') ||
                   (log.level.toUpperCase() === 'ERROR' && 
                    (!log.behavior || log.behavior !== 'user_cancel') &&
                    (!log.message || !log.message.toLowerCase().includes('cancel')));
        });
    }
    
    // Tooltip visibility state
    let tooltipDismissed = false;
    let errorTooltipDismissed = false; // Track error notification dismissal separately
    let tooltipVisible = false;
    
    // Show tooltip when hovering over the button (if errors exist)
    function handleLogButtonHover() {
        if (hasErrorLogs() && !showLogViewer && !tooltipDismissed) {
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
        logger.info('app', 'Log viewer toggled', { visible: showLogViewer });
        
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

    // Fill effect handlers for cancel button
    function handleCancelMouseEnter(event: MouseEvent) {
        if (!cancelButtonRef) return;
        
        // Get exact coordinates relative to the button
        const rect = cancelButtonRef.getBoundingClientRect();
        const x = event.clientX - rect.left;
        const y = event.clientY - rect.top;
        
        // Get the fill color from CSS variable
        const computedStyle = getComputedStyle(cancelButtonRef);
        const fillColor = computedStyle.getPropertyValue('--fill-color') || 'hsla(var(--fill-red-hue), var(--fill-red-saturation), var(--fill-red-lightness), var(--fill-red-alpha))';
        
        // Create and style a new element for the fill effect
        const fill = document.createElement('div');
        fill.style.position = 'absolute';
        fill.style.left = x + 'px';
        fill.style.top = y + 'px';
        fill.style.width = '0';
        fill.style.height = '0';
        fill.style.borderRadius = '50%';
        fill.style.backgroundColor = fillColor;
        fill.style.transform = 'translate(-50%, -50%)';
        fill.style.transition = 'width 0.5s ease-out, height 0.5s ease-out';
        fill.style.zIndex = '-1';
        
        // Append to button
        cancelButtonRef.appendChild(fill);
        
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

    function handleCancelMouseLeave() {
        // Remove all fill elements when mouse leaves
        if (cancelButtonRef) {
            const fills = cancelButtonRef.querySelectorAll('div');
            fills.forEach((fill: Element) => cancelButtonRef?.removeChild(fill));
        }
    }

    async function handleProcess() {
        if (!currentFeatureOptions || !mediaSource) return;
	
        // Increment process start count
        try {
            const newCount = await IncrementStatistic('countProcessStart');
            logger.trace('app', `Process start count incremented to: ${newCount}`);
            
            // Update the local store with the new value
            statisticsStore.updatePartial({ countProcessStart: newCount });
        } catch (error) {
            logger.error('app', 'Failed to increment process start statistic', { error });
        }

        processingStartTime = Date.now(); // Returns milliseconds since Unix epoch
        logger.trace('app', `Starting new processing run at timestamp: ${processingStartTime} (${new Date(processingStartTime).toISOString()})`);
        isProcessing = true;
        progress = 0;
        errorTooltipDismissed = false; // Reset error notification dismissal for new run
        
        // Apply "Show log viewer by default" setting when starting a process
        const currentSettings = get(settings);
        if (currentSettings && currentSettings.showLogViewerByDefault) {
            showLogViewer = true;
            logger.trace('app', 'Showing log viewer based on default setting');
        }
        
        // Completely clear all progress bars when starting a new process
        // This ensures we don't have lingering error states from previous runs
        progressBars.set([]);

        // Use the quick access language tag if it differs from the default
        const effectiveLanguageCode = quickAccessLangTag && quickAccessLangTag !== defaultTargetLanguage
            ? quickAccessLangTag
            : defaultTargetLanguage;

        try {
            // Transform voice enhancing options: translate sepLib + useNvidiaGPU to actual provider name
            const transformedOptions = { ...currentFeatureOptions };
            if (transformedOptions.voiceEnhancing) {
                const voiceOpts = { ...transformedOptions.voiceEnhancing };
                const sepLib = voiceOpts.sepLib as string;
                const useNvidiaGPU = voiceOpts.useNvidiaGPU as boolean;

                // If GPU is enabled and it's a docker provider, transform to nvidia variant
                if (useNvidiaGPU && sepLib && sepLib.startsWith('docker-') && !sepLib.includes('nvidia')) {
                    voiceOpts.sepLib = sepLib.replace('docker-', 'docker-nvidia-');
                    logger.debug('app', 'Transformed sepLib for GPU', { from: sepLib, to: voiceOpts.sepLib });
                }

                // Remove the useNvidiaGPU option as backend doesn't need it (it's UI-only)
                delete voiceOpts.useNvidiaGPU;
                transformedOptions.voiceEnhancing = voiceOpts;
            }

            // Construct the request object matching the Go backend type
            const request = { // ProcessRequest type
                path: mediaSource.path,
                selectedFeatures,
                options: { Options: transformedOptions }, // LLMs: DO NOT CHANGE THIS LINE. As is to match the backend Go type FeatureOptions.
                languageCode: effectiveLanguageCode,
                audioTrackIndex: mediaSource?.audioTrackIndex ?? 0, // Use nullish coalescing
            };

            logger.trace('app', "Sending processing request", { request });
            await SendProcessingRequest(request);
        } catch (error: any) { // Type the error
            logger.error('app', "Processing failed", { error });
            invalidationErrorStore.addError({
                id: "processing-failed",
                message: "Processing failed: " + (error?.message || "Unknown error"),
                severity: "critical",
                dismissible: true
            });
            // Reset isProcessing on error since no WebSocket events will be emitted if request fails
            isProcessing = false;
        } finally {
            // Progress is always reset regardless of success/failure
            progress = 0;
        }
    }

    async function handleCancel() {
        try {
            await CancelProcessing();
            isProcessing = false;
            invalidationErrorStore.addError({
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
            logger.error('app', "Failed to cancel processing", { error });
            // Use warning instead of critical - don't block the Process button
            // isProcessing stays true so cancel button remains visible for retry
            invalidationErrorStore.addError({
                id: "cancel-failed",
                message: "Failed to cancel processing",
                severity: "warning",
                dismissible: true
            });
        }
    }

    async function loadSettings() {
        try {
            const loadedSettings = await LoadSettings(); // Use direct import
            
            settings.set(loadedSettings as any); // Use type assertion until Settings type is fully updated
            
            // Initialize showGlow based on liteMode setting (if not minimized)
            // liteMode = true means reduced effects, so showGlow = !liteMode
            if (!isWindowMinimized) {
                showGlow = !loadedSettings.liteMode;
                // Sync with liteModeStore store
                liteModeStore.setUserPreference(loadedSettings.liteMode);
            }
            defaultTargetLanguage = loadedSettings.targetLanguage;
            showLogViewer = loadedSettings.showLogViewerByDefault;
        } catch (error) {
            logger.error('app', "Failed to load settings", { error });/*
            invalidationErrorStore.addError({
                id: "settings-load-failed",
                message: "Failed to load settings",
                severity: "critical",
                dismissible: true,
                action: {
                    label: "Retry",
                    handler: () => loadSettings()
                }
            });*/
        }
    }

    async function loadStatisticsData() {
        try {
            // Load statistics from backend
            const stats = await LoadStatistics();
            statisticsStore.set(stats);
            
            // Check if this is the first app start or if deps are missing
            if (stats.countAppStart === 0 ||
                version == "dev" ||
                ($ffmpegStatusStore.checked && !$ffmpegStatusStore.available) ||
                ($mediainfoStatusStore.checked && !$mediainfoStatusStore.available)) {
                logger.info('app', 'First app start or missing dependencies, showing welcome popup');
                showWelcomePopup = true;
            } else {
                // Welcome popup won't show, trigger changelog check after feature cards animate
                // FeatureSelector starts immediately when welcomePopupVisible is false
                // Give time for feature cards staggered animation (~1500ms) plus buffer
                logger.debug('app', 'Welcome popup skipped, scheduling changelog check');
                setTimeout(() => {
                    logger.debug('app', 'Triggering changelog upgrade check (no welcome popup)');
                    triggerChangelogCheck = true;
                }, 2000);
            }

            // Increment app start count
            const newCount = await IncrementStatistic('countAppStart');
            logger.trace('app', `App start count incremented to: ${newCount}`);
            
            // Update the local store with the new value
            statisticsStore.updatePartial({ countAppStart: newCount });
        } catch (error) {
            logger.error('app', "Failed to load statistics", { error });
        }
    }

    async function checkDockerAvailability() {
        try {
            const dockerStatus = await CheckDockerAvailability();
            dockerStatusStore.set({
                available: dockerStatus.available || false,
                version: dockerStatus.version,
                engine: dockerStatus.engine,
                error: dockerStatus.error,
                checked: true
            });
            
            logger.debug('app', 'Docker check completed', dockerStatus);
        } catch (error) {
            logger.error('app', 'Docker check failed', { error });
            dockerStatusStore.set({
                available: false,
                error: 'Check failed',
                checked: true
            });
        }
    }
    
    async function checkInternetConnectivity() {
        try {
            const internetStatus = await CheckInternetConnectivity();
            internetStatusStore.set({
                online: internetStatus.online || false,
                latency: internetStatus.latency,
                error: internetStatus.error,
                checked: true
            });
            
            logger.debug('app', 'Internet check completed', internetStatus);
        } catch (error) {
            logger.error('app', 'Internet check failed', { error });
            internetStatusStore.set({
                online: false,
                error: 'Check failed',
                checked: true
            });
        }
    }
    
    async function checkFFmpegAvailability() {
        try {
            const ffmpegStatus = await CheckFFmpegAvailability();
            ffmpegStatusStore.set({
                available: ffmpegStatus.available || false,
                version: ffmpegStatus.version,
                path: ffmpegStatus.path,
                error: ffmpegStatus.error,
                checked: true
            });
            
            logger.debug('app', 'FFmpeg check completed', ffmpegStatus);
        } catch (error) {
            logger.error('app', 'FFmpeg check failed', { error });
            ffmpegStatusStore.set({
                available: false,
                error: 'Check failed',
                checked: true
            });
        }
    }
    
    async function checkMediaInfoAvailability() {
        try {
            const mediainfoStatus = await CheckMediaInfoAvailability();
            mediainfoStatusStore.set({
                available: mediainfoStatus.available || false,
                version: mediainfoStatus.version,
                path: mediainfoStatus.path,
                error: mediainfoStatus.error,
                checked: true
            });
            
            logger.debug('app', 'MediaInfo check completed', mediainfoStatus);
        } catch (error) {
            logger.error('app', 'MediaInfo check failed', { error });
            mediainfoStatusStore.set({
                available: false,
                error: 'Check failed',
                checked: true
            });
        }
    }

    async function checkNvidiaGPUAvailability() {
        try {
            await nvidiaGPUStore.refresh();
            logger.debug('app', 'NVIDIA GPU check completed', get(nvidiaGPUStore));
        } catch (error) {
            logger.error('app', 'NVIDIA GPU check failed', { error });
        }
    }

    // Set up periodic Docker checks when unavailable
    $: {
        if ($dockerStatusStore.checked && !$dockerStatusStore.available && $dockerStatusStore.error !== 'Debug: Forced state') {
            // Start checking every 5 seconds if not already checking
            if (!dockerCheckInterval) {
                logger.debug('app', 'Starting periodic Docker availability checks');
                dockerCheckInterval = setInterval(checkDockerAvailability, 5000);
            }
        } else {
            // Stop checking when available or forced
            if (dockerCheckInterval) {
                logger.debug('app', 'Stopping periodic Docker availability checks');
                clearInterval(dockerCheckInterval);
                dockerCheckInterval = null;
            }
        }
    }
    
    // Set up periodic Internet checks when unavailable
    $: {
        if ($internetStatusStore.checked && !$internetStatusStore.online && $internetStatusStore.error !== 'Debug: Forced state') {
            // Start checking every 5 seconds if not already checking
            if (!internetCheckInterval) {
                logger.debug('app', 'Starting periodic Internet connectivity checks');
                internetCheckInterval = setInterval(checkInternetConnectivity, 5000);
            }
        } else {
            // Stop checking when online or forced
            if (internetCheckInterval) {
                logger.debug('app', 'Stopping periodic Internet connectivity checks');
                clearInterval(internetCheckInterval);
                internetCheckInterval = null;
            }
        }
    }
    
    // Set up periodic FFmpeg checks when unavailable
    $: {
        if ($ffmpegStatusStore.checked && !$ffmpegStatusStore.available && $ffmpegStatusStore.error !== 'Debug: Forced state') {
            // Start checking every 5 seconds if not already checking
            if (!ffmpegCheckInterval) {
                logger.debug('app', 'Starting periodic FFmpeg availability checks');
                ffmpegCheckInterval = setInterval(checkFFmpegAvailability, 5000);
            }
        } else {
            // Stop checking when available or forced
            if (ffmpegCheckInterval) {
                logger.debug('app', 'Stopping periodic FFmpeg availability checks');
                clearInterval(ffmpegCheckInterval);
                ffmpegCheckInterval = null;
            }
        }
    }
    
    // Set up periodic MediaInfo checks when unavailable
    $: {
        if ($mediainfoStatusStore.checked && !$mediainfoStatusStore.available && $mediainfoStatusStore.error !== 'Debug: Forced state') {
            // Start checking every 5 seconds if not already checking
            if (!mediainfoCheckInterval) {
                logger.debug('app', 'Starting periodic MediaInfo availability checks');
                mediainfoCheckInterval = setInterval(checkMediaInfoAvailability, 5000);
            }
        } else {
            // Stop checking when available or forced
            if (mediainfoCheckInterval) {
                logger.debug('app', 'Stopping periodic MediaInfo availability checks');
                clearInterval(mediainfoCheckInterval);
                mediainfoCheckInterval = null;
            }
        }
    }

    // Use a more efficient approach to handle events, with debouncing for frequent events
    let progressUpdateDebounceTimer: number | null = null;
    let pendingProgressUpdates: any[] = [];
    let windowCheckInterval: number | null = null;
    
    // Periodic status check intervals
    let dockerCheckInterval: ReturnType<typeof setInterval> | null = null;
    let internetCheckInterval: ReturnType<typeof setInterval> | null = null;
    let ffmpegCheckInterval: ReturnType<typeof setInterval> | null = null;
    let mediainfoCheckInterval: ReturnType<typeof setInterval> | null = null;
    
    // Performance optimization based on window state
    async function checkWindowState() {
        try {
            // Check window minimized state
            const minimized = await safeWindowIsMinimised();
            const timestamp = new Date().toISOString();
            
            // Only update if state changed to avoid unnecessary re-renders
            if (minimized !== isWindowMinimized) {
                isWindowMinimized = minimized;
                
                // If window is minimized, reduce animations and processing
                if (minimized) {
                    showGlow = false;
                } else {
                    // liteMode = true means no glow, so showGlow = !liteMode
                    showGlow = !($settings?.liteMode ?? false);
                }
                
                // Log specific optimization changes
                logger.trace('app', `Optimizations applied:
                - Glow effect: ${showGlow ? 'ENABLED' : 'DISABLED'}
                - Progress updates: ${minimized ? 'THROTTLED (10fps)' : 'NORMAL (60fps)'}
                - Log updates: ${minimized ? 'MINIMAL' : 'NORMAL'}
                - Animations: ${minimized ? 'REDUCED' : 'NORMAL'}`);
            }
            
            // Check maximized state too (could be used for enhancing UI on large screens)
            const maximized = await safeWindowIsMaximised();
            
            // Only log if maximized state changes
            if (maximized !== isWindowMaximized) {
                logger.trace('app', `Window maximized state changed: ${maximized}`);
                isWindowMaximized = maximized;
            }
            
        } catch (error) {
            logger.error('app', 'Failed to check window state', { error });
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
            logger.trace('app', `⚡ Progress update stats:
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

    // Define these functions and subscriptions in the component scope so they can be removed in onDestroy
    let handleTransitionEnd: (e: TransitionEvent) => void;
    let updateLogViewerButtonPosition: () => void;
    let uiSettingsSubscription: () => void;
    let liteModeSubscription: () => void;
    
    // Global drag and drop handler
    async function handleGlobalFileDrop(x: number, y: number, paths: string[]) {
        logger.debug('app', 'Global file drop detected', { 
            x, 
            y, 
            pathCount: paths.length,
            paths 
        });
        
        // Reset visual feedback
        globalDragOver = false;
        
        if (paths.length === 1) {
            droppedFilePath = paths[0];
            // The MediaInput component will handle the file processing
        } else {
            logger.trace('app', 'Multiple files dropped, ignoring', { fileCount: paths.length });
        }
    }

    onMount(async () => { // Make onMount async
        // Initialize runtime stores
        await initializeRuntimeStores();
        
        // Add delay for Chrome DevTools debugging
        // const debugDelay = 10000; // 10 seconds
        // logger.warn('app', `DEBUG MODE: Waiting ${debugDelay/1000} seconds for Chrome DevTools connection...`);
        // console.log(`DEBUG MODE: Waiting ${debugDelay/1000} seconds for Chrome DevTools connection...`);
        // await new Promise(resolve => setTimeout(resolve, debugDelay));
        // logger.warn('app', 'DEBUG MODE: Starting initialization after delay');
        // console.log('DEBUG MODE: Starting initialization after delay');
        
        // Register callback to send frontend logs to LogViewer
        logger.registerLogViewerCallback((logMessage) => {
            logStore.addLog(logMessage);
        });
        
        // Initialize WebSocket connection early
        logger.info('app', 'Initializing WebSocket connection');
        
        // Set up WebSocket event handlers
        wsClient.on('connected', async (data) => {
            logger.info('app', 'WebSocket connected/reconnected', { timestamp: data.timestamp });
            
            // Sync processing state on reconnection
            try {
                const processingStatus = await GetProcessingStatus();
                isProcessing = processingStatus.isProcessing;
                logger.debug('app', 'Processing state resynced after reconnection', { isProcessing });
            } catch (error) {
                logger.error('app', 'Failed to resync processing status on reconnection', { error });
            }
        });
        
        wsClient.on('llm.state.changed', (data) => {
            logger.debug('app', 'LLM state change received via WebSocket', { 
                globalState: data.globalState 
            });
            llmStateStore.set(data);
        });
        
        // Set up log handlers
        wsClient.on('log.entry', (rawLog: any) => {
            // Always process logs even when minimized to maintain complete log history
            logStore.addLog(rawLog);
        });
        
        wsClient.on('log.batch', (logBatch) => {
            if (!Array.isArray(logBatch) || logBatch.length === 0) return;
            
            // Use the logStore's batch processing directly - it handles merging, ordering and chunking
            logStore.addLogBatch(logBatch);
            
            // For very large batches, log a debug message
            if (logBatch.length > 200) {
                logger.debug('app', 'Processed large log batch', { batchSize: logBatch.length });
            }
        });
        
        // Set up progress handlers
        wsClient.on('progress.remove', (taskId: string) => {
            logger.trace('app', `Explicitly removing progress bar: ${taskId}`);
            removeProgressBar(taskId);
        });
        
        wsClient.on('progress.reset', (data: boolean) => {
            logger.trace('app', 'Resetting all progress bars');
            resetAllProgressBars();
        });
        
        // Handle individual progress events (sent in direct-pass mode)
        wsClient.on('progress.updated', (data) => {
            updateProgressBar(data);
        });
        
        wsClient.on('progress.batch', (progressBatch) => {
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
                
            } else {
                // Process all updates normally when window is visible or batch is small
                progressBatch.forEach(data => {
                    updateProgressBar(data);
                });
            }
        });
        
        // Set up processing event handlers
        wsClient.on('processing.started', (data) => {
            logger.info('app', 'Processing started event received', { timestamp: data.timestamp });
            isProcessing = true;
        });
        
        wsClient.on('processing.completed', (data) => {
            logger.info('app', 'Processing completed event received', { 
                status: data.status, 
                error: data.error,
                timestamp: data.timestamp 
            });
            isProcessing = false;
            
            // If processing failed, add error to invalidation store
            if (data.status === 'error' && data.error) {
                invalidationErrorStore.addError({
                    id: "processing-failed-backend",
                    message: `Processing failed: ${data.error}`,
                    severity: "critical",
                    dismissible: true
                });
            }
        });
        
        // Set up WASM state request handler
        wsClient.on('wasm.state.request', () => {
            logger.debug('app', 'Backend requested WebAssembly state');
            
            // Update memory info if WebAssembly is active
            try {
                const module = getWasmModule();
                if (module && module.get_memory_usage) {
                    const memInfo = module.get_memory_usage();
                    // Direct update via imported function - no command pattern
                    import('./lib/wasm-state').then(m => m.updateMemoryUsage(memInfo));
                }
            } catch (e: any) {
                logger.error('app', `Failed to get memory info: ${e.message}`);
            }
            
            // Send current state to backend
            syncWasmStateForReport();
        });
        
        // Set up settings loaded handler
        wsClient.on('settings.loaded', (loadedSettings) => {
            logger.debug('app', 'Settings loaded via WebSocket', { settings: loadedSettings });
            // Settings are already handled by the settings store, this is just for logging
        });
        
        // Start WebSocket connection
        // CRITICAL: wsClient.connect SHOULD NEVER BE CALLED ELSEWHERE
        try {
            await wsClient.connect();
            logger.info('app', 'WebSocket connected successfully');
        } catch (error) {
            logger.error('app', 'Failed to connect WebSocket', { error });
            // WebSocket will auto-reconnect, so we continue app initialization
        }
        
        // Initialize WebRPC API client
        try {
            const { getAPIBaseUrl } = await import('./api');
            const apiUrl = await getAPIBaseUrl();
            logger.info('app', `WebRPC API initialized at ${apiUrl}`);
        } catch (error) {
            logger.error('app', 'Failed to initialize WebRPC API', { error });
            // Continue app initialization even if API fails
            // The API will retry on first use
        }
        
        // Get initial version and pass it to WebAssembly for environment-aware loading
        try {
            const versionInfo = await GetVersion();
            version = versionInfo.version; // Access the version property 
            // Add version to window for WebAssembly to access
            (window as any).__LANGKIT_VERSION = version;
            
            logger.info('app', `Application version detected: ${version}`, 
                { isDevMode: version === 'dev' }
            );
            
            // Check for updates asynchronously (don't await)
            CheckForUpdate().then(result => {
                updateAvailable = result.updateAvailable;
                if (updateAvailable) {
                    logger.info('app', 'Update available detected');
                }
            }).catch(error => {
                logger.error('app', 'Failed to check for update', { error });
            });
        } catch (error) {
            logger.error('app', 'Failed to get version info', { error });
            // Keep default version value
        }
        
        
        // Initialize system info store for OS-dependent functionality
        try {
            const systemInfo = await GetSystemInfo();
            systemInfoStore.set(systemInfo);
            logger.debug('app', 'System info initialized', { os: systemInfo.os, arch: systemInfo.arch });

            // Initialize reduced effects mode based on runtime and OS
            // This disables backdrop-filter blur effects on Qt+Windows to avoid flickering
            const ankiMode = get(isAnkiMode);
            liteModeStore.setAuto(ankiMode, systemInfo.os);
            // Debug override can be toggled in the Developer Dashboard > Style tab
        } catch (error) {
            logger.error('app', 'Failed to get system info', { error });
        }

        // Check hardware acceleration and warn if not available
        try {
            const graphicsInfo = await getGraphicsInfo();
            logger.debug('app', 'Graphics info detected', {
                renderer: graphicsInfo.renderer,
                hardwareAccelerated: graphicsInfo.hardwareAccelerated,
                webgpuAvailable: graphicsInfo.webgpuAvailable
            });

            if (!graphicsInfo.hardwareAccelerated) {
                const softwareRenderer = graphicsInfo.softwareRenderer || 'software renderer';
                logger.warn('app', 'Hardware acceleration not available', { softwareRenderer });

                // Build warning message based on runtime mode
                let warningMessage = 'Hardware graphics acceleration is not available in this WebView. ' +
                    'The application is using "' + softwareRenderer + '" which will result in significantly degraded performance.\n\n';

                if (get(isAnkiMode)) {
                    // Anki-specific instructions with OS-appropriate video driver recommendation
                    const sysInfo = get(systemInfoStore);
                    let recommendedDriver = 'OpenGL';
                    if (sysInfo && sysInfo.os === 'windows') {
                        recommendedDriver = 'Direct3D';
                    } else if (sysInfo && sysInfo.os === 'darwin') {
                        recommendedDriver = 'OpenGL'; // Metal is default and should work, OpenGL as fallback
                    }
                    // Linux: OpenGL is the standard choice

                    warningMessage += 'To fix this in Anki, go to:\n' +
                        'Tools > Preferences > Appearance > General section\n' +
                        'and ensure "Video driver" is set to "' + recommendedDriver + '" (not "Software").\n\n' +
                        'Note: You will need to restart Anki for the change to take effect.';
                } else {
                    // Generic instructions for standalone/browser mode
                    warningMessage += 'For optimal performance, please ensure your graphics drivers are up to date ' +
                        'and hardware acceleration is enabled in your system settings.';
                }

                // CLI alternative applies to all modes
                warningMessage += '\n\nAlternatively, you can use Langkit\'s command-line interface (langkit-cli) ' +
                    'which does not require graphics acceleration.';

                ShowWarning('Hardware Acceleration Unavailable', warningMessage).catch(err => {
                    logger.error('app', 'Failed to show hardware acceleration warning', { error: err });
                });

                // Also add to invalidation store for persistent UI indicator
                invalidationErrorStore.addError({
                    id: 'no-hardware-acceleration',
                    message: 'Hardware acceleration unavailable. Performance may be degraded.',
                    severity: 'warning',
                    dismissible: true
                });

                // Force lite mode to reduce GPU load when no hardware acceleration
                liteModeStore.setNoHardwareAcceleration();
                showGlow = false;
            }
        } catch (error) {
            logger.error('app', 'Failed to check hardware acceleration', { error });
        }

        // Sync processing state on mount (handles HMR and initial load)
        try {
            const processingStatus = await GetProcessingStatus();
            isProcessing = processingStatus.isProcessing;
            logger.debug('app', 'Processing state synchronized', { isProcessing });
        } catch (error) {
            logger.error('app', 'Failed to get processing status', { error });
            // Assume not processing if we can't get status
            isProcessing = false;
        }
        
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
        
        // Set up global drag and drop handling
        initializeDragDrop(handleGlobalFileDrop);
        logger.trace('app', 'Global drag and drop handler set up');
        document.addEventListener('transitionend', handleTransitionEnd);
        
        // Check window state every 2 seconds to optimize resource usage
        windowCheckInterval = window.setInterval(checkWindowState, 2000);
        
        try {
            // First, log the WebAssembly startup
            logger.info('app', 'Starting WebAssembly subsystem initialization');
            
            // Load settings before initializing WebAssembly
            await loadSettings(); 
            const $currentSettings = get(settings);
            
            // Load statistics data
            await loadStatisticsData();
            
            // Check if WebAssembly is supported by the browser
            if (!await import('./lib/wasm').then(m => m.isWasmSupported())) {
                logger.warn('app', 'WebAssembly is not supported by this browser');
                invalidationErrorStore.addError({
                    id: 'wasm-not-supported',
                    message: 'WebAssembly is not supported by your browser. Some optimizations will be disabled.',
                    severity: 'warning',
                    dismissible: true,
                });
                
                // Update settings to disable WebAssembly if it was enabled
                if ($currentSettings.useWasm) {
                    const updatedSettings = {
                        ...$currentSettings,
                        useWasm: false
                    };
                    settings.set(updatedSettings);
                    await SaveSettings(updatedSettings);
                }
                
                // Skip remaining WebAssembly initialization
                return;
            }
            
            // Setup the request-wasm-state event handler has been moved to WebSocket initialization section
            
            // Subscribe to UI settings changes
            uiSettingsSubscription = settings.subscribe(($newSettings) => {
                // Update showGlow when liteMode setting changes
                // liteMode = true means no glow, so showGlow = !liteMode
                if ($newSettings && $newSettings.liteMode !== undefined) {
                    // Only update showGlow if not minimized
                    if (!isWindowMinimized) {
                        showGlow = !$newSettings.liteMode;
                        // Sync with liteModeStore store
                        liteModeStore.setUserPreference($newSettings.liteMode);
                    }
                }

                // Update showLogViewer when showLogViewerByDefault setting changes
                if ($newSettings && $newSettings.showLogViewerByDefault !== undefined) {
                    // If the user hasn't explicitly set showLogViewer yet, or if we're turning it on
                    if (!isProcessing || $newSettings.showLogViewerByDefault) {
                        showLogViewer = $newSettings.showLogViewerByDefault;
                        logger.trace('app', `Updated log viewer visibility based on setting: ${showLogViewer ? 'visible' : 'hidden'}`);
                    }
                }
            });

            // Subscribe to liteModeStore for Dev Dashboard toggle (has priority over saved settings)
            liteModeSubscription = liteModeStore.subscribe(($liteMode) => {
                // Update showGlow when liteModeStore changes (e.g., debug override from Dev Dashboard)
                if (!isWindowMinimized) {
                    showGlow = !$liteMode.enabled;
                    logger.trace('app', 'showGlow updated from liteModeStore', {
                        enabled: $liteMode.enabled,
                        reason: $liteMode.reason,
                        showGlow
                    });
                }
            });

            // Listen for settings changes to enable/disable WebAssembly
            settings.subscribe(async ($newSettings) => {
                // Only process WebAssembly settings if they've changed
                if ($newSettings.useWasm !== undefined && $newSettings.useWasm !== $currentSettings.useWasm) {
                    logger.info('app', `WebAssembly setting changed to: ${$newSettings.useWasm ? 'enabled' : 'disabled'}`);
                    
                    try {
                        const wasEnabled = await enableWasm($newSettings.useWasm);
                        
                        if ($newSettings.useWasm) {
                            if (wasEnabled) {
                                logger.info('app', 'WebAssembly successfully enabled via settings');
                            } else {
                                // Handle case where enabling failed
                                invalidationErrorStore.addError({
                                    id: 'wasm-init-failed',
                                    message: 'Failed to initialize WebAssembly optimization.',
                                    severity: 'warning',
                                    dismissible: true,
                                });
                            }
                        } else {
                            logger.info('app', 'WebAssembly disabled via settings');
                        }
                    } catch (error: any) {
                        logger.error('app', `Error applying WebAssembly setting: ${error.message}`);
                        
                        invalidationErrorStore.addError({
                            id: 'wasm-setting-error',
                            message: `Error applying WebAssembly setting: ${error.message}`,
                            severity: 'warning',
                            dismissible: true,
                        });
                    }
                }
            });

            // Initialize WebAssembly on startup if enabled in settings
            if ($currentSettings.useWasm) {
                logger.info('app', 'Initializing WebAssembly based on saved settings');
                
                const wasEnabled = await enableWasm(true);
                
                if (wasEnabled) {
                    logger.info('app', 'WebAssembly initialized successfully on application startup');
                } else {
                    logger.warn('app', 'WebAssembly initialization failed on startup, check browser console for details');
                }
            } else {
                logger.info('app', 'WebAssembly optimization is disabled in settings');
            }

        } catch (initError: any) {
            logger.critical('app', `Critical error during WebAssembly setup: ${initError.message}`);
            
            invalidationErrorStore.addError({
                id: 'wasm-critical-init-error',
                message: `Error during application initialization: ${initError.message}`,
                severity: 'warning',
                dismissible: true,
            });
        }
        
        // Defer loading of the Feature Selector component until main UI has rendered
        // This improves perceived performance and creates a nicer sequential reveal effect
        setTimeout(() => {
            logger.trace('app', `🎬 Showing feature selector component after UI shell render`);
            showFeatureSelector = true;
        }, 300); // 300ms gives UI shell time to render first
        
        // Set up user activity event listeners
        const activityEvents = ['mousemove', 'keydown', 'mousedown', 'touchstart', 'wheel', 'click'];
        activityEvents.forEach(event => {
            window.addEventListener(event, handleUserActivity, { passive: true });
        });
        
        // Trigger initial activity detection
        handleUserActivity();
        
        // Check Docker, Internet, FFmpeg, MediaInfo and NVIDIA GPU availability on startup
        checkDockerAvailability();
        checkInternetConnectivity();
        checkFFmpegAvailability();
        checkMediaInfoAvailability();
        checkNvidiaGPUAvailability();
    });

    // Cleanup on component destruction
    onDestroy(() => {
        if (windowCheckInterval) clearInterval(windowCheckInterval);
        
        // Clear periodic status check intervals
        if (dockerCheckInterval) {
            clearInterval(dockerCheckInterval);
            dockerCheckInterval = null;
        }
        if (internetCheckInterval) {
            clearInterval(internetCheckInterval);
            internetCheckInterval = null;
        }
        if (ffmpegCheckInterval) {
            clearInterval(ffmpegCheckInterval);
            ffmpegCheckInterval = null;
        }
        if (mediainfoCheckInterval) {
            clearInterval(mediainfoCheckInterval);
            mediainfoCheckInterval = null;
        }
        
        // Remove listeners added in onMount
        if (handleTransitionEnd) {
           document.removeEventListener('transitionend', handleTransitionEnd); 
        }
        if (updateLogViewerButtonPosition) {
           window.removeEventListener('resize', updateLogViewerButtonPosition); 
        }
        
        // Clean up global drag and drop handler
        cleanupDragDrop();
        logger.trace('app', 'Global drag and drop handler cleaned up');
        
        // Clean up user activity event listeners
        const activityEvents = ['mousemove', 'keydown', 'mousedown', 'touchstart', 'wheel', 'click'];
        activityEvents.forEach(event => {
            window.removeEventListener(event, handleUserActivity);
        });
        
        // Clear activity timers
        if (userActivityTimer) {
            clearTimeout(userActivityTimer);
        }
        if (afkTimer) {
            clearTimeout(afkTimer);
        }
        
        // Unsubscribe from UI settings subscription
        if (uiSettingsSubscription) {
            uiSettingsSubscription();
        }

        // Unsubscribe from liteModeStore subscription
        if (liteModeSubscription) {
            liteModeSubscription();
        }

        // Unsubscribe from activity state
        unsubscribeActivityState();
        
        // Log application shutdown
        logger.info('app', 'Application shutting down, performing cleanup');
        
        // Force garbage collection if WebAssembly is active
        try {
            const module = getWasmModule();
            if (module && module.force_garbage_collection) {
                module.force_garbage_collection();
                logger.info('app', 'Performed final garbage collection during shutdown');
            }
        } catch (e: any) {
            logger.warn('app', `Failed to perform final cleanup: ${e.message}`);
        }
        
        // Report final state for crash reporting
        syncWasmStateForReport();
    });
</script>

<!-- Version display -->
<div class="fixed top-[0.2rem] right-[3.9rem] z-50 p-0 text-[0.6rem] text-gray-500 text-xs font-dm-mono flex items-center">
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
        <CoffeeSupport {version} />
    {/if}
</div>

<!-- Main container -->
<div class="w-screen h-screen bg-bg text-gray-100 font-dm-sans fixed inset-0
     {globalDragOver ? 'ring-2 ring-primary ring-opacity-50' : ''}"
     style="--wails-drop-target: drop;"
     on:dragenter={(e) => { e.preventDefault(); globalDragOver = true; }}
     on:dragleave={(e) => { e.preventDefault(); if (e.currentTarget === e.target) globalDragOver = false; }}
     on:dragover={(e) => { e.preventDefault(); }}
     on:drop={(e) => { e.preventDefault(); }}>
    <BackgroundGradient />
    <!-- GlowEffect now has conditional backdrop-filter via liteModeStore store -->
    {#if showGlow && !isWindowMinimized && userActivityState !== UserActivityState.AFK}
        <GlowEffect {isProcessing} />
    {/if}

    <!-- Return to Anki button (Qt mode only) -->
    <ReturnToAnkiButton />

    <!-- Settings button  -->
    <!-- Conditionally disable backdrop-blur in lite mode to prevent Qt WebEngine flickering -->
    <div class="absolute top-4 right-4 z-20 flex items-center gap-4">
        <button
            class="w-10 h-10 flex items-center justify-center rounded-xl
                   {liteMode ? 'bg-white/15' : 'bg-white/5 backdrop-blur-md'} border border-white/10
                   text-white/70 transition-all duration-300
                   hover:bg-white/10 hover:border-primary/30 hover:text-primary
                   hover:-translate-y-0.5 hover:shadow-lg hover:shadow-primary/20
                   focus:outline-none focus:ring-2 focus:ring-primary/50"
            on:click={() => {
                logger.info('app', 'Settings opened');
                $showSettings = true;
            }}
            aria-label="Open settings"
        >
            <span class="material-icons text-[22px]">settings</span>
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
                            bind:droppedFilePath
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
                        <ProgressManager {isProcessing} {isWindowMinimized} {userActivityState} />
                    </div>
                    
                    <!-- Process Button Row with hardware acceleration -->
                    <div class="max-w-2xl mx-auto flex justify-center items-center gap-4 pb-0 will-change-transform">
                        <ProcessButton
                            {isProcessing}
                            on:process={handleProcess}
                        />
                        
                        <!-- Cancel button with optimized transitions -->
                        {#if isProcessing}
                            <div class="h-14 w-12 flex items-center">
                                <button
                                    class="h-12 w-12 flex items-center justify-center rounded-lg
                                           bg-red-600/40 text-white transition-all duration-200
                                           border border-red-500/50
                                           hover:-translate-y-0.5
                                           hover:shadow-lg hover:shadow-red-500/20
                                           hover:border-red-400
                                           focus:outline-none focus:ring-2 focus:ring-red-500/50
                                           focus:ring-offset-2 focus:ring-offset-bg
                                           fill-effect fill-effect-red"
                                    on:click={handleCancel}
                                    on:mouseenter={handleCancelMouseEnter}
                                    on:mouseleave={handleCancelMouseLeave}
                                    in:slide={{ duration: 200, axis: "x" }}
                                    out:slide={{ duration: 200, axis: "x" }}
                                    aria-label="Cancel processing"
                                    bind:this={cancelButtonRef}
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
                                    <span class="absolute -top-1 -right-1 h-4 w-4 bg-error-hard rounded-full border border-white flex items-center justify-center text-[10px] text-white font-bold animate-pulse">
                                        !
                                    </span>
                                {/if}

                                <!-- Removed duplicate button -->
                            </button>
                            
                            <!-- Log Viewer Notification -->
                            <LogViewerNotification 
                                processingStartTime={processingStartTime}
                                position={logViewerButtonPosition} 
                                isProcessing={isProcessing}
                                hasErrorsFromParent={hasErrorLogs()}
                                isVisible={((isProcessing && !tooltipDismissed) || 
                                            (hasErrorLogs() && (!errorTooltipDismissed || tooltipVisible))) && 
                                            !showLogViewer && 
                                            !!logViewerButtonPosition}
                                onOpenLogViewer={toggleLogViewer}
                                onDismiss={() => {
                                    tooltipDismissed = true;
                                    tooltipVisible = false;
                                    // Also dismiss error notifications
                                    if (hasErrorLogs()) {
                                        errorTooltipDismissed = true;
                                    }
                                }}
                            />
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
                <LogViewer version={version} isProcessing={isProcessing} userActivityState={userActivityState} />
            </div>
        {/if}
    </div>
</div>

<Settings
    version={version}
    onClose={() => {
        logger.info('app', 'Settings closed');
        $showSettings = false;
    }}
/>

<!-- Developer dashboard (only appears in dev mode) -->
{#if version}
    <DevDashboard {version} {isProcessing} />
{/if}

<!-- Welcome popup for first-time users -->
{#if showWelcomePopup}
    <WelcomePopup
        {version}
        onClose={() => {
            logger.info('app', 'Welcome popup closed');
            showWelcomePopup = false;
            // Trigger changelog check after feature cards animation completes
            // FeatureSelector waits 400ms then animates cards with staggered delays
            // Total animation time is roughly 1500-2000ms, so we wait 2000ms
            setTimeout(() => {
                logger.debug('app', 'Triggering changelog upgrade check');
                triggerChangelogCheck = true;
            }, 2000);
        }}
        recheckFFmpeg={checkFFmpegAvailability}
        recheckMediaInfo={checkMediaInfoAvailability}
    />
{/if}

<!-- Changelog popup - checks for upgrade when triggerCheck becomes true -->
<ChangelogPopup
    triggerCheck={triggerChangelogCheck}
    on:dismissed={() => {
        logger.info('app', 'Changelog popup dismissed');
        triggerChangelogCheck = false;
    }}
    on:checked={(e) => {
        if (!e.detail.shouldShow) {
            triggerChangelogCheck = false;
        }
    }}
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
        0% { box-shadow: 0 0 0 0 hsla(var(--error-hard-hue), var(--error-hard-saturation), var(--error-hard-lightness), 0.4); }
        70% { box-shadow: 0 0 0 10px hsla(var(--error-hard-hue), var(--error-hard-saturation), var(--error-hard-lightness), 0); }
        100% { box-shadow: 0 0 0 0 hsla(var(--error-hard-hue), var(--error-hard-saturation), var(--error-hard-lightness), 0); }
    }
    
    .log-button-pulse {
        animation: log-button-pulse 2s infinite;
    }
    
    .log-button-error-pulse {
        animation: log-button-error-pulse 1.5s infinite;
        border: 1px solid hsla(var(--error-hard-hue), var(--error-hard-saturation), var(--error-hard-lightness), 0.5);
    }
    
    /* Ensure drop zone click handler works */

    /* WASM status indicator */
    .wasm-status-indicator {
        position: relative;
        transition: background-color 0.3s ease, color 0.3s ease;
    }

    .wasm-status-indicator.active {
        background-color: rgba(var(--primary-rgb), 0.25);
        animation: wasm-pulse 2s infinite;
    }

    @keyframes wasm-pulse {
        0% {
            box-shadow: 0 0 0 0 rgba(var(--primary-rgb), 0.4);
        }
        70% {
            box-shadow: 0 0 0 6px rgba(var(--primary-rgb), 0);
        }
        100% {
            box-shadow: 0 0 0 0 rgba(var(--primary-rgb), 0);
        }
    }
    .drop-zone {
        cursor: pointer;
    }
</style>
