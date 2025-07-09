<script lang="ts">
    import { fade, slide } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    import { onMount, onDestroy } from 'svelte';
    import { get } from 'svelte/store';
    import '@material-design-icons/font';

    import { settings, showSettings, wasmActive, statisticsStore, welcomePopupVisible, userActivityState as userActivityStateStore, dockerStatusStore, internetStatusStore, ffmpegStatusStore, mediainfoStatusStore } from './lib/stores'; 
    import { logStore } from './lib/logStore';
    import { invalidationErrorStore } from './lib/invalidationErrorStore';
    import { logger } from './lib/logger';
    import { progressBars, updateProgressBar, removeProgressBar, resetAllProgressBars } from './lib/progressBarsStore';
    import { enableWasm, isWasmEnabled, getWasmModule } from './lib/wasm'; // Removed setWasmSizeThreshold
    import { reportWasmState, syncWasmStateForReport, getWasmState } from './lib/wasm-state';

    // Import window API from Wails
    import { WindowIsMinimised, WindowIsMaximised, OnFileDrop, OnFileDropOff } from '../wailsjs/runtime/runtime';

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
    
    import { 
        SendProcessingRequest, 
        CancelProcessing, 
        GetVersion, 
        LoadSettings, 
        SaveSettings, 
        GetCurrentTimestamp,
        RefreshSTTModelsAfterSettingsUpdate,
        LoadStatistics,
        UpdateStatistics,
        IncrementStatistic,
        CheckDockerAvailability,
        CheckInternetConnectivity,
        GetLanguageRequirements,
        CheckFFmpegAvailability,
        CheckMediaInfoAvailability
    } from '../wailsjs/go/gui/App';
    import { EventsOn } from '../wailsjs/runtime/runtime';
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
    
    // Track current language requirements
    let currentLanguageRequirements: gui.LanguageRequirements | null = null;
    
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
    
    // Check language requirements when it changes
    $: {
        if (quickAccessLangTag) {
            GetLanguageRequirements(quickAccessLangTag).then(requirements => {
                currentLanguageRequirements = requirements;
                logger.debug('app', 'Language requirements updated', requirements);
            }).catch(error => {
                logger.error('app', 'Failed to get language requirements', { error });
                currentLanguageRequirements = null;
            });
        } else {
            currentLanguageRequirements = null;
        }
    }
    
    // State for welcome popup
    let showWelcomePopup = false;
    
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
    
    // Check Docker requirements for selected features
    $: {
        const dockerChecked = $dockerStatusStore.checked;
        const dockerAvailable = $dockerStatusStore.available;
        
        // Check if subtitle linguistic processing features are selected
        const linguisticFeaturesSelected = selectedFeatures.subtitleRomanization || 
                                         selectedFeatures.selectiveTransliteration || 
                                         selectedFeatures.subtitleTokenization;
        
        // Check if current language requires Docker (from backend requirements)
        const languageRequiresDocker = currentLanguageRequirements?.requiresDocker || false;
        
        // Only create error if:
        // 1. Docker is not available AND
        // 2. Linguistic features are selected AND
        // 3. The current language actually requires Docker
        if (dockerChecked && !dockerAvailable && linguisticFeaturesSelected && languageRequiresDocker) {
            let message = "Without Docker, linguistic processing for Japanese & Indic languages will not be available.";
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
        
        // Check if current language requires Internet (from backend requirements)
        const languageRequiresInternet = currentLanguageRequirements?.requiresInternet || false;
        
        // Need internet if:
        // 1. AI-powered features are selected OR
        // 2. Linguistic features are selected AND language requires internet
        const needsInternet = aiPoweredFeaturesSelected || 
                            (linguisticFeaturesSelected && languageRequiresInternet);
        
        if (internetChecked && !internetOnline && needsInternet) {
            let message = "An internet connection is required for ";
            if (aiPoweredFeaturesSelected && linguisticFeaturesSelected && languageRequiresInternet) {
                message += "AI-powered features and linguistic processing for Thai, Japanese & Indic languages.";
            } else if (aiPoweredFeaturesSelected) {
                message += "AI-powered features (dubtitles, voice enhancing).";
            } else {
                message += "linguistic processing for Thai, Japanese & Indic languages.";
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
            // Only consider logs from current run
            const logTime = log._unix_time || 0;
            const isCurrentRun = logTime >= processingStartTime;
            
            // Only show errors from current run
            if (!isCurrentRun) return false;
            
            return (log.behavior === 'abort_task' && log.level.toUpperCase() === 'ERROR') || 
                   (log.behavior === 'abort_all' && log.level.toUpperCase() === 'ERROR') ||
                   (log.level.toUpperCase() === 'ERROR' && 
                    (!log.behavior || log.behavior !== 'user_cancel') &&
                    (!log.message || !log.message.toLowerCase().includes('cancel')));
        });
    }
    
    // Tooltip visibility state
    let tooltipDismissed = false;
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

        processingStartTime = await GetCurrentTimestamp(); // FIXME it seems that Date.now() is UNIX epoch and can provide unix timestamp : Math.floor(Date.now() / 1000)
        logger.trace('app', `Starting new processing run at timestamp: ${processingStartTime} (${new Date(processingStartTime).toISOString()})`);
        isProcessing = true;
        progress = 0;
        
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
            // Construct the request object matching the Go backend type
            const request: gui.ProcessRequest = { // Add type annotation
                path: mediaSource.path,
                selectedFeatures,
                options: { Options: currentFeatureOptions }, // LLMs: DO NOT CHANGE THIS LINE. As is to match the backend Go type FeatureOptions.
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
        } finally {
            isProcessing = false;
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
            invalidationErrorStore.addError({
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
            
            settings.set(loadedSettings as any); // Use type assertion until Settings type is fully updated
            
            // Initialize showGlow based on enableGlow setting (if not minimized)
            if (!isWindowMinimized) {
                showGlow = loadedSettings.enableGlow;
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
            if (stats.countAppStart === 0 || version == "dev" || !$ffmpegStatusStore.available || !$mediainfoStatusStore.available) {
                logger.info('app', 'First app start or missing dependencies, showing welcome popup');
                showWelcomePopup = true;
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
                logger.trace('app', `Optimizations applied:
                - Glow effect: ${showGlow ? 'ENABLED' : 'DISABLED'}
                - Progress updates: ${minimized ? 'THROTTLED (10fps)' : 'NORMAL (60fps)'}
                - Log updates: ${minimized ? 'MINIMAL' : 'NORMAL'}
                - Animations: ${minimized ? 'REDUCED' : 'NORMAL'}`);
            }
            
            // Check maximized state too (could be used for enhancing UI on large screens)
            const maximized = await WindowIsMaximised();
            
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
        // Register callback to send frontend logs to LogViewer
        logger.registerLogViewerCallback((logMessage) => {
            logStore.addLog(logMessage);
        });
        
        // Get initial version and pass it to WebAssembly for environment-aware loading
        GetVersion().then(v => {
            version = v.version; // Access the version property 
            // Add version to window for WebAssembly to access
            (window as any).__LANGKIT_VERSION = version;
            
            logger.info('app', `Application version detected: ${version}`, 
                { isDevMode: version === 'dev' }
            );
        });
        
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
        OnFileDrop(handleGlobalFileDrop, true);
        logger.trace('app', 'Global drag and drop handler set up');
        document.addEventListener('transitionend', handleTransitionEnd);
        
        // Log initialization of performance monitoring
        logger.trace('app', `🚀 Initializing window-state based performance optimizations:
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
        
        // --- WebAssembly Initialization ---
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
            
            // Setup the request-wasm-state event handler for crash reporting
            EventsOn("request-wasm-state", () => {
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
            
            // Subscribe to UI settings changes
            uiSettingsSubscription = settings.subscribe(($newSettings) => {
                // Update showGlow when enableGlow setting changes
                if ($newSettings && $newSettings.enableGlow !== undefined) {
                    // Only update showGlow if not minimized
                    if (!isWindowMinimized) {
                        showGlow = $newSettings.enableGlow;
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
                                
                                // Apply threshold from settings
                                // Threshold is now read directly via getWasmSizeThreshold(), no need to set it here.
                                // if ($newSettings.wasmSizeThreshold) {
                                //     setWasmSizeThreshold($newSettings.wasmSizeThreshold);
                                //     logger.info('app', 
                                //         WasmLogLevel.INFO,
                                //         'config',
                                //         `Set WebAssembly size threshold to ${$newSettings.wasmSizeThreshold} logs`
                                //     );
                                // }
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
                // Handle threshold changes separately
                else if ($newSettings.wasmSizeThreshold !== undefined && 
                         $newSettings.wasmSizeThreshold !== $currentSettings.wasmSizeThreshold) {
                    if (isWasmEnabled()) {
                        // Threshold is now read directly via getWasmSizeThreshold(), no need to set it here.
                        // setWasmSizeThreshold($newSettings.wasmSizeThreshold);
                        // logger.info('app', 
                        //     WasmLogLevel.INFO,
                        //     'config',
                        //     `Updated WebAssembly size threshold to ${$newSettings.wasmSizeThreshold} logs`
                        // );
                    }
                }
            });

            // Initialize WebAssembly on startup if enabled in settings
            if ($currentSettings.useWasm) {
                logger.info('app', 'Initializing WebAssembly based on saved settings');
                
                const wasEnabled = await enableWasm(true);
                
                if (wasEnabled) {
                    logger.info('app', 'WebAssembly initialized successfully on application startup');
                    
                    // Apply threshold from settings
                    // Threshold is now read directly via getWasmSizeThreshold(), no need to set it here.
                    // if ($currentSettings.wasmSizeThreshold) {
                    //     setWasmSizeThreshold($currentSettings.wasmSizeThreshold);
                    // }
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
        // --- End WebAssembly Initialization ---
        
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
                logger.debug('app', 'Processed large log batch', { batchSize: logBatch.length });
            }
        });

        EventsOn("progress-remove", (taskId: string) => {
            logger.trace('app', `Explicitly removing progress bar: ${taskId}`);
            removeProgressBar(taskId);
        });

        // Efficient progress batch handler with smart grouping
        // Handle individual progress events (sent in direct-pass mode)
        EventsOn("progress", (data) => {
            updateProgressBar(data);
        });

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
            logger.trace('app', `Task ${taskId} completed.`);
            // Optionally remove the progress bar after a short delay
            setTimeout(() => removeProgressBar(taskId), 2000);
        });

        // Handle task error events
        EventsOn("task-error", (errorData: { id: string, error: string }) => {
            logger.error('app', `Task ${errorData.id} failed: ${errorData.error}`);
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
            logger.info('app', `Update available: ${newVersion}`);
            version = newVersion; // Update version display if needed
            updateAvailable = true;
        });
        
        // Check Docker, Internet, FFmpeg and MediaInfo availability on startup
        checkDockerAvailability();
        checkInternetConnectivity();
        checkFFmpegAvailability();
        checkMediaInfoAvailability();
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
        OnFileDropOff();
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

<!-- Version display (fixed, using Tailwind and DM Mono) -->
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

<!-- Main container now spans full viewport -->
<div class="w-screen h-screen bg-bg text-gray-100 font-dm-sans fixed inset-0
     {globalDragOver ? 'ring-2 ring-primary ring-opacity-50' : ''}"
     style="--wails-drop-target: drop;"
     on:dragenter={(e) => { e.preventDefault(); globalDragOver = true; }}
     on:dragleave={(e) => { e.preventDefault(); if (e.currentTarget === e.target) globalDragOver = false; }}
     on:dragover={(e) => { e.preventDefault(); }}>
    <BackgroundGradient />
    {#if showGlow && !isWindowMinimized && userActivityState !== UserActivityState.AFK}
        <GlowEffect {isProcessing} />
    {/if}

    <!-- Settings button container -->
    <div class="absolute top-4 right-4 z-20 flex items-center gap-4">
        <button
            class="w-10 h-10 flex items-center justify-center rounded-xl
                   bg-white/5 backdrop-blur-md border border-white/10
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

    <!-- Removed central button -->

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
                                isVisible={((isProcessing && !tooltipDismissed) || 
                                            hasErrorLogs() || 
                                            tooltipVisible) && 
                                            !showLogViewer && 
                                            !!logViewerButtonPosition}
                                onOpenLogViewer={toggleLogViewer}
                                onDismiss={() => {
                                    tooltipDismissed = true;
                                    tooltipVisible = false;
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
    <DevDashboard {version} />
{/if}

<!-- Welcome popup for first-time users -->
{#if showWelcomePopup}
    <WelcomePopup
        onClose={() => {
            logger.info('app', 'Welcome popup closed');
            showWelcomePopup = false;
        }}
        recheckFFmpeg={checkFFmpegAvailability}
        recheckMediaInfo={checkMediaInfoAvailability}
    />
{/if}

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
