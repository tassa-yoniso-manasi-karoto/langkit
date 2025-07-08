<script lang="ts">
    import { fade, scale } from 'svelte/transition';
    import { onMount, onDestroy } from 'svelte';
    import Portal from "svelte-portal/src/Portal.svelte";
    import { getWasmState } from '../lib/wasm-state';
    import { settings, llmStateStore, statisticsStore, userActivityState as userActivityStateStore, dockerStatusStore, internetStatusStore, ffmpegStatusStore, mediainfoStatusStore, enableTraceLogsStore, enableFrontendLoggingStore, displayFrontendLogsStore } from '../lib/stores';
    import { isDeveloperMode } from '../lib/developerMode';
    import { logger } from '../lib/logger';
    import WASMDashboard from './dev/WASMDashboard.svelte';
    import MemoryTestButton from './MemoryTestButton.svelte';
    import { SetTraceLogs, GetTraceLogs } from '../../wailsjs/go/gui/App';
    import { 
        forceLLMState, resetLLMState,
        forceUserActivityState, resetUserActivityState,
        forceDockerStatus, resetDockerStatus,
        forceInternetStatus, resetInternetStatus,
        forceFFmpegStatus, resetFFmpegStatus,
        forceMediaInfoStatus, resetMediaInfoStatus
    } from '../lib/dev/debugStateControls';
    import { defaultValues, defaultProgressWaveValues as importedDefaultProgressWaveValues } from '../lib/dev/styleControlsDefaults';
    
    // Props
    export let version: string = '';
    
    // State variables for draggable functionality
    let isDragging = false;
    let isExpanded = false;
    let startX = 0;
    let startY = 0;
    let offsetX = 0;
    let offsetY = 0;
    let posX = 20; // Initial position
    let posY = 20; // Initial position
    
    // Component references
    let iconBubble: HTMLDivElement;
    let dashboardPanel: HTMLDivElement;
    
    // Dashboard tabs
    let activeTab = 'performance';
    const tabs = [
        { id: 'performance', name: 'WASM', icon: 'speed' },
        { id: 'state', name: 'State', icon: 'data_object' },
        { id: 'logs', name: 'Logs', icon: 'subject' },
        { id: 'debug', name: 'Debug', icon: 'bug_report' },
        { id: 'style', name: 'Style', icon: 'palette' }
    ];
    
    // Store current settings
    let currentSettings;
    const unsubscribeSettings = settings.subscribe(value => {
        currentSettings = value;
    });
    
    // Store current LLM state
    let currentLLMState;
    const unsubscribeLLMState = llmStateStore.subscribe(value => {
        currentLLMState = value;
    });
    
    // Store current statistics
    let currentStatistics;
    const unsubscribeStatistics = statisticsStore.subscribe(value => {
        currentStatistics = value;
    });
    
    // Store current user activity state
    let currentUserActivityState = 'active';
    let isForced = false;
    const unsubscribeUserActivity = userActivityStateStore.subscribe(value => {
        currentUserActivityState = value.state;
        isForced = value.isForced;
    });
    
    // Store current Docker status
    let currentDockerStatus;
    let dockerForced = false;
    const unsubscribeDocker = dockerStatusStore.subscribe(value => {
        currentDockerStatus = value;
        // Check if it's forced by looking for special markers
        dockerForced = value.error === 'Debug: Forced state';
    });
    
    // Store current Internet status
    let currentInternetStatus;
    let internetForced = false;
    const unsubscribeInternet = internetStatusStore.subscribe(value => {
        currentInternetStatus = value;
        // Check if it's forced by looking for special markers
        internetForced = value.error === 'Debug: Forced state';
    });
    
    // Store current FFmpeg status
    let currentFFmpegStatus;
    let ffmpegForced = false;
    const unsubscribeFFmpeg = ffmpegStatusStore.subscribe(value => {
        currentFFmpegStatus = value;
        ffmpegForced = value.error === 'Debug: Forced state';
    });
    
    // Store current MediaInfo status
    let currentMediaInfoStatus;
    let mediainfoForced = false;
    const unsubscribeMediaInfo = mediainfoStatusStore.subscribe(value => {
        currentMediaInfoStatus = value;
        mediainfoForced = value.error === 'Debug: Forced state';
    });
    
    // Show when in dev mode or developer mode is enabled
    $: showDashboard = (!!version && (version === 'dev' || version.includes('dev'))) || $isDeveloperMode;
   
    // When the trace logs store changes, call the backend
    $: {
    	if ($enableTraceLogsStore !== undefined) {
    		SetTraceLogs($enableTraceLogsStore);
    	}
    }
    
    // Handle dragging for both the icon and expanded dashboard
    function handleMouseDown(event: MouseEvent) {
        // Make sure the event isn't coming from buttons with stopPropagation
        const target = event.target as HTMLElement;

        // Check if we have an explicit stopPropagation marker
        if (target.hasAttribute('on:mousedown|stopPropagation')) {
            return;
        }

        // Start dragging
        isDragging = true;
        startX = event.clientX;
        startY = event.clientY;

        // Add events to window to track cursor even when outside element
        window.addEventListener('mousemove', handleMouseMove);
        window.addEventListener('mouseup', handleMouseUp);

        // Prevent default behavior
        event.preventDefault();
        event.stopPropagation();
    }
    
    function handleMouseMove(event: MouseEvent) {
        if (!isDragging) return;

        // Calculate movement
        const dx = event.clientX - startX;
        const dy = event.clientY - startY;

        // Update position and reset drag start point for next move
        posX += dx;
        posY += dy;
        startX = event.clientX;
        startY = event.clientY;

        // Keep on screen (simple boundaries)
        if (posX < 0) posX = 0;
        if (posY < 0) posY = 0;
        if (posX > window.innerWidth - 50) posX = window.innerWidth - 50;
        if (posY > window.innerHeight - 50) posY = window.innerHeight - 50;

        // Prevent defaults
        event.preventDefault();
        event.stopPropagation();
    }
    
    function handleMouseUp(event) {
        isDragging = false;
        window.removeEventListener('mousemove', handleMouseMove);
        window.removeEventListener('mouseup', handleMouseUp);

        // Prevent defaults if we were actually dragging
        if (event) {
            event.preventDefault();
            event.stopPropagation();
        }
    }
    
    function toggleDashboard(event) {
        // Prevent event from propagating to parent elements
        if (event) event.stopPropagation();

        logger.trace('devDashboard', `Toggling dashboard: ${isExpanded} → ${!isExpanded}`);
        isExpanded = !isExpanded;

        // Log dashboard toggle
        logger.debug('devDashboard', `Developer dashboard ${isExpanded ? 'expanded' : 'minimized'}`);
    }
    
    function switchTab(id: string) {
        activeTab = id;
    }
    
    // Style controls state
    // 
    // ⚠️  IMPORTANT NOTES FOR FUTURE SLIDER ADDITIONS ⚠️
    // 
    // When adding/removing sliders, be extremely careful about:
    // 
    // 1. NEVER remove existing sliders unless explicitly requested
    //    - User may say "add more sliders" but mean "add additional ones", not replace existing ones
    //    - Always preserve original functionality (e.g., glow opacity slider was original, keep it)
    // 
    // 2. When user provides exported values from dashboard, those values are ONLY for sliders that exist
    //    - Don't hard-code values for sliders you removed 
    //    - If slider is removed, let CSS variable return to its original default in app.css
    //    - Only apply user's exported values to sliders that actually exist in the UI
    // 
    // 3. Distinguish between different effect types:
    //    - "Background gradient" = the radial gradient behind everything (BackgroundGradient.svelte)  
    //    - "Glow effect" = the animated blob effect (GlowEffect.svelte)
    //    - User likely wants background gradient controls, not detailed glow controls
    // 
    // 4. CSS specificity issues:
    //    - Main buttons use both .control-button and .reset-button classes
    //    - Individual slider buttons use only .reset-button in .slider-row context
    //    - Use specific selectors like .control-button.reset-button for main buttons
    // 
    // 5. Variable initialization order:
    //    - Don't reference defaultValues before it's defined
    //    - Define styleControls with actual values, not { ...defaultValues }
    // 
    // 6. Border opacity was REMOVED and should stay removed
    //    - It returns to original CSS default, no longer controllable
    //    - Don't re-add unless explicitly requested
    //
    // 7. When adding sub-tabs or new sections:
    //    - ALL sliders must be included in styleControls object
    //    - ALL sliders must be included in defaultValues object  
    //    - This ensures reset/copy functionality works across all tabs
    //    - Sub-tab sliders are still part of the main styleControls object
    //
    let styleControls = {
        bgHue: 280,
        bgSaturation: 0,
        bgLightness: 2.15,
        bgOpacity: 1,
        featureCardSaturation: 8,
        featureCardLightness: 21,
        featureCardOpacity: 1,
        featureCardGradientStartOpacity: 0.82,
        featureCardGradientEndOpacity: 0,
        mediaInputSaturation: 10,
        mediaInputLightness: 20,
        mediaInputOpacity: 0.06,
        glowOpacity: 0.26,
        glowPositionX: 78,
        glowPositionY: -10,
        glowSize: 26,
        glowBlur: 100,
        glowAnimationScale: 1.8,
        glowAnimationSpeed: 10,
        bgGradientPosX: 19,
        bgGradientPosY: 90,
        bgGradientStop1Hue: 280,
        bgGradientStop1Sat: 15,
        bgGradientStop1Light: 26,
        bgGradientStop1Alpha: 0.11,
        bgGradientStop2Hue: 237,
        bgGradientStop2Sat: 20,
        bgGradientStop2Light: 35,
        bgGradientStop2Alpha: 0.19,
        bgGradientStop3Hue: 320,
        bgGradientStop3Sat: 25,
        bgGradientStop3Light: 45,
        bgGradientStop3Alpha: 0.05,
        bgGradientStop4Hue: 300,
        bgGradientStop4Sat: 20,
        bgGradientStop4Light: 35,
        bgGradientStop4Alpha: 0.18,
        welcomeOverlayOpacity: 0.4,
        welcomePanelBgOpacity: 0.3,
        welcomePanelBlur: 24,
        welcomeBorderOpacity: 0.1,
        welcomeCardBgOpacity: 0.1,
        welcomeCardHoverOpacity: 0.15,
        welcomeButtonBgOpacity: 0.7,
        welcomeButtonBorderOpacity: 0.5,
        welcomeProgressDotOpacity: 0.3,
        welcomeTextPrimaryOpacity: 1,
        welcomeTextSecondaryOpacity: 0.7,
        welcomeTextTertiaryOpacity: 0.6,
        coffeeMugHue: 220,
        coffeeMugSaturation: 9,
        coffeeMugLightness: 43,
        coffeeMugOpacity: 0.67
    };
    
    // Target color input
    let targetColorHex = '#141215';
    
    // Style sub-tabs
    let activeStyleSubTab = 'main';
    let currentProgressState: 'normal' | 'error_task' | 'error_all' | 'user_cancel' | 'complete' = 'normal';
    
    // Progress Manager wave controls
    let progressWaveControls = {
        // Normal state wave colors (HSLA)
        wave1Hue: 261,
        wave1Saturation: 90,
        wave1Lightness: 70,
        wave1Alpha: 0.5,
        wave2Hue: 261,
        wave2Saturation: 90,
        wave2Lightness: 70,
        wave2Alpha: 0.7,
        wave3Hue: 323,
        wave3Saturation: 100,
        wave3Lightness: 72,
        wave3Alpha: 0.8,
        wave4Hue: 323,
        wave4Saturation: 100,
        wave4Lightness: 72,
        wave4Alpha: 0.9,
        // Error task state wave colors
        errorTaskWave1Hue: 50,
        errorTaskWave1Saturation: 90,
        errorTaskWave1Lightness: 75,
        errorTaskWave1Alpha: 0.5,
        errorTaskWave2Hue: 50,
        errorTaskWave2Saturation: 90,
        errorTaskWave2Lightness: 75,
        errorTaskWave2Alpha: 0.7,
        errorTaskWave3Hue: 50,
        errorTaskWave3Saturation: 90,
        errorTaskWave3Lightness: 75,
        errorTaskWave3Alpha: 0.8,
        errorTaskWave4Hue: 50,
        errorTaskWave4Saturation: 90,
        errorTaskWave4Lightness: 75,
        errorTaskWave4Alpha: 0.9,
        // Error all state wave colors
        errorAllWave1Hue: 0,
        errorAllWave1Saturation: 85,
        errorAllWave1Lightness: 60,
        errorAllWave1Alpha: 0.5,
        errorAllWave2Hue: 0,
        errorAllWave2Saturation: 85,
        errorAllWave2Lightness: 60,
        errorAllWave2Alpha: 0.7,
        errorAllWave3Hue: 0,
        errorAllWave3Saturation: 85,
        errorAllWave3Lightness: 60,
        errorAllWave3Alpha: 0.8,
        errorAllWave4Hue: 0,
        errorAllWave4Saturation: 85,
        errorAllWave4Lightness: 60,
        errorAllWave4Alpha: 0.9,
        // Wave physics
        waveIntensity: 4,  // Height of waves
        waveFrequency: 1,  // Density of wave crests
        // Animation
        animationSpeed: 1,  // Speed multiplier
        blurAmount: 1.7,   // Blur filter strength
        // Progress bar specific
        progressEdgeGlow: 0.4,
        progressSweepOpacity: 0.85,
        progressSweepDuration: 2.5,
        // Additional aesthetics
        progressBgDarkness: -10,  // How much darker the background is
        waveOverallOpacity: 1,    // Overall wave transparency
        waveDirection: 1,         // 1 for normal, -1 for reverse
        waveStackingMultiplier: 1.0,  // Opacity gradient between waves (1.0 = no effect)
        waveOffsetMultiplier: 1.0  // Vertical offset spacing between waves
    };
    
    const defaultProgressWaveValues = importedDefaultProgressWaveValues;
    
    // Apply progress wave controls
    function applyProgressWaveControls() {
        const root = document.documentElement;
        
        // Wave colors with stacking multiplier applied
        // Only apply stacking multiplier if it's not at default value (1.0)
        const applyStacking = Math.abs(progressWaveControls.waveStackingMultiplier - 1.0) > 0.01;
        const opacity1 = progressWaveControls.wave1Alpha * progressWaveControls.waveOverallOpacity;
        const opacity2 = progressWaveControls.wave2Alpha * progressWaveControls.waveOverallOpacity * (applyStacking ? Math.pow(progressWaveControls.waveStackingMultiplier, 1) : 1);
        const opacity3 = progressWaveControls.wave3Alpha * progressWaveControls.waveOverallOpacity * (applyStacking ? Math.pow(progressWaveControls.waveStackingMultiplier, 2) : 1);
        const opacity4 = progressWaveControls.wave4Alpha * progressWaveControls.waveOverallOpacity * (applyStacking ? Math.pow(progressWaveControls.waveStackingMultiplier, 3) : 1);
        
        // Normal state wave colors
        root.style.setProperty('--progress-wave-1-fill', `hsla(${progressWaveControls.wave1Hue}, ${progressWaveControls.wave1Saturation}%, ${progressWaveControls.wave1Lightness}%, ${opacity1})`);
        root.style.setProperty('--progress-wave-2-fill', `hsla(${progressWaveControls.wave2Hue}, ${progressWaveControls.wave2Saturation}%, ${progressWaveControls.wave2Lightness}%, ${opacity2})`);
        root.style.setProperty('--progress-wave-3-fill', `hsla(${progressWaveControls.wave3Hue}, ${progressWaveControls.wave3Saturation}%, ${progressWaveControls.wave3Lightness}%, ${opacity3})`);
        root.style.setProperty('--progress-wave-4-fill', `hsla(${progressWaveControls.wave4Hue}, ${progressWaveControls.wave4Saturation}%, ${progressWaveControls.wave4Lightness}%, ${opacity4})`);
        
        // Error task state wave colors
        const errorTaskOpacity1 = progressWaveControls.errorTaskWave1Alpha * progressWaveControls.waveOverallOpacity;
        const errorTaskOpacity2 = progressWaveControls.errorTaskWave2Alpha * progressWaveControls.waveOverallOpacity * (applyStacking ? Math.pow(progressWaveControls.waveStackingMultiplier, 1) : 1);
        const errorTaskOpacity3 = progressWaveControls.errorTaskWave3Alpha * progressWaveControls.waveOverallOpacity * (applyStacking ? Math.pow(progressWaveControls.waveStackingMultiplier, 2) : 1);
        const errorTaskOpacity4 = progressWaveControls.errorTaskWave4Alpha * progressWaveControls.waveOverallOpacity * (applyStacking ? Math.pow(progressWaveControls.waveStackingMultiplier, 3) : 1);
        
        root.style.setProperty('--error-task-wave-1-fill', `hsla(${progressWaveControls.errorTaskWave1Hue}, ${progressWaveControls.errorTaskWave1Saturation}%, ${progressWaveControls.errorTaskWave1Lightness}%, ${errorTaskOpacity1})`);
        root.style.setProperty('--error-task-wave-2-fill', `hsla(${progressWaveControls.errorTaskWave2Hue}, ${progressWaveControls.errorTaskWave2Saturation}%, ${progressWaveControls.errorTaskWave2Lightness}%, ${errorTaskOpacity2})`);
        root.style.setProperty('--error-task-wave-3-fill', `hsla(${progressWaveControls.errorTaskWave3Hue}, ${progressWaveControls.errorTaskWave3Saturation}%, ${progressWaveControls.errorTaskWave3Lightness}%, ${errorTaskOpacity3})`);
        root.style.setProperty('--error-task-wave-4-fill', `hsla(${progressWaveControls.errorTaskWave4Hue}, ${progressWaveControls.errorTaskWave4Saturation}%, ${progressWaveControls.errorTaskWave4Lightness}%, ${errorTaskOpacity4})`);
        
        // Error all state wave colors
        const errorAllOpacity1 = progressWaveControls.errorAllWave1Alpha * progressWaveControls.waveOverallOpacity;
        const errorAllOpacity2 = progressWaveControls.errorAllWave2Alpha * progressWaveControls.waveOverallOpacity * (applyStacking ? Math.pow(progressWaveControls.waveStackingMultiplier, 1) : 1);
        const errorAllOpacity3 = progressWaveControls.errorAllWave3Alpha * progressWaveControls.waveOverallOpacity * (applyStacking ? Math.pow(progressWaveControls.waveStackingMultiplier, 2) : 1);
        const errorAllOpacity4 = progressWaveControls.errorAllWave4Alpha * progressWaveControls.waveOverallOpacity * (applyStacking ? Math.pow(progressWaveControls.waveStackingMultiplier, 3) : 1);
        
        root.style.setProperty('--error-all-wave-1-fill', `hsla(${progressWaveControls.errorAllWave1Hue}, ${progressWaveControls.errorAllWave1Saturation}%, ${progressWaveControls.errorAllWave1Lightness}%, ${errorAllOpacity1})`);
        root.style.setProperty('--error-all-wave-2-fill', `hsla(${progressWaveControls.errorAllWave2Hue}, ${progressWaveControls.errorAllWave2Saturation}%, ${progressWaveControls.errorAllWave2Lightness}%, ${errorAllOpacity2})`);
        root.style.setProperty('--error-all-wave-3-fill', `hsla(${progressWaveControls.errorAllWave3Hue}, ${progressWaveControls.errorAllWave3Saturation}%, ${progressWaveControls.errorAllWave3Lightness}%, ${errorAllOpacity3})`);
        root.style.setProperty('--error-all-wave-4-fill', `hsla(${progressWaveControls.errorAllWave4Hue}, ${progressWaveControls.errorAllWave4Saturation}%, ${progressWaveControls.errorAllWave4Lightness}%, ${errorAllOpacity4})`);
        
        // Progress bar specific
        root.style.setProperty('--progress-edge-opacity', progressWaveControls.progressEdgeGlow);
        root.style.setProperty('--sweep-opacity', progressWaveControls.progressSweepOpacity);
        root.style.setProperty('--sweep-duration', progressWaveControls.progressSweepDuration + 's');
        
        // Animation speed (modify existing animation durations)
        const baseSpeed = 1 / progressWaveControls.animationSpeed;
        root.style.setProperty('--progress-wave-speed-1', (7 * baseSpeed) + 's');
        root.style.setProperty('--progress-wave-speed-2', (10 * baseSpeed) + 's');
        root.style.setProperty('--progress-wave-speed-3', (13 * baseSpeed) + 's');
        root.style.setProperty('--progress-wave-speed-4', (20 * baseSpeed) + 's');
        
        // Blur amount
        root.style.setProperty('--progress-wave-blur', progressWaveControls.blurAmount + 'px');
        
        // Wave physics (requires SVG path modification)
        root.style.setProperty('--progress-wave-intensity', progressWaveControls.waveIntensity);
        root.style.setProperty('--progress-wave-frequency', progressWaveControls.waveFrequency);
        
        // Dispatch event to notify ProgressManager of wave physics changes
        document.dispatchEvent(new CustomEvent('progress-wave-update'));
        
        // Additional aesthetics
        root.style.setProperty('--progress-bg-darkness', progressWaveControls.progressBgDarkness + '%');
        root.style.setProperty('--wave-overall-opacity', progressWaveControls.waveOverallOpacity);
        root.style.setProperty('--wave-direction', progressWaveControls.waveDirection);
        root.style.setProperty('--wave-stacking-multiplier', progressWaveControls.waveStackingMultiplier);
        
        // Wave offset multipliers (exponential progression)
        root.style.setProperty('--wave-offset-base', '1');
        root.style.setProperty('--wave-offset-multiplier', progressWaveControls.waveOffsetMultiplier);
        root.style.setProperty('--wave-offset-multiplier-2', Math.pow(progressWaveControls.waveOffsetMultiplier, 1.5));
        root.style.setProperty('--wave-offset-multiplier-3', Math.pow(progressWaveControls.waveOffsetMultiplier, 2));
    }
    
    // Reset individual progress wave property
    function resetProgressWaveProperty(property: string) {
        progressWaveControls[property] = defaultProgressWaveValues[property];
        applyProgressWaveControls();
    }
    
    // Reset all progress wave controls
    function resetProgressWaveControls() {
        progressWaveControls = { ...defaultProgressWaveValues };
        applyProgressWaveControls();
    }
    
    // Apply style changes to CSS custom properties
    function applyStyleControls() {
        const root = document.documentElement;
        
        // Background color
        root.style.setProperty('--style-bg-color', `hsla(${styleControls.bgHue}, ${styleControls.bgSaturation}%, ${styleControls.bgLightness}%, ${styleControls.bgOpacity})`);
        
        // Feature card styles
        root.style.setProperty('--style-feature-card-bg', `hsla(0, 0%, 100%, ${styleControls.featureCardOpacity})`);
        root.style.setProperty('--style-feature-card-gradient', 
            `linear-gradient(135deg, ` +
            `hsla(${styleControls.bgHue}, ${styleControls.featureCardSaturation}%, ${styleControls.featureCardLightness}%, ${styleControls.featureCardGradientStartOpacity}) 0%, ` +
            `hsla(${styleControls.bgHue}, ${styleControls.featureCardSaturation}%, ${styleControls.featureCardLightness + 5}%, ${styleControls.featureCardGradientEndOpacity}) 100%)`
        );
        
        // Media input styles
        root.style.setProperty('--style-media-input-bg', `hsla(0, 0%, 100%, ${styleControls.mediaInputOpacity})`);
        
        // Effect styles
        root.style.setProperty('--style-glow-opacity', styleControls.glowOpacity.toString());
        
        // Glow positioning and effects
        root.style.setProperty('--style-glow-position-x', `${styleControls.glowPositionX}vw`);
        root.style.setProperty('--style-glow-position-y', `${styleControls.glowPositionY}vw`);
        root.style.setProperty('--style-glow-size', `${styleControls.glowSize}vmax`);
        root.style.setProperty('--style-glow-blur', `${styleControls.glowBlur}px`);
        root.style.setProperty('--style-glow-animation-scale', styleControls.glowAnimationScale.toString());
        root.style.setProperty('--style-glow-animation-speed', `${styleControls.glowAnimationSpeed}s`);
        
        // Background gradient with custom controls
        root.style.setProperty('--style-background-gradient', 
            `radial-gradient(` +
            `circle at ${styleControls.bgGradientPosX}% ${styleControls.bgGradientPosY}%, ` +
            `hsla(${styleControls.bgGradientStop1Hue}, ${styleControls.bgGradientStop1Sat}%, ${styleControls.bgGradientStop1Light}%, ${styleControls.bgGradientStop1Alpha}) 0%, ` +
            `hsla(${styleControls.bgGradientStop2Hue}, ${styleControls.bgGradientStop2Sat}%, ${styleControls.bgGradientStop2Light}%, ${styleControls.bgGradientStop2Alpha}) 25%, ` +
            `hsla(${styleControls.bgGradientStop3Hue}, ${styleControls.bgGradientStop3Sat}%, ${styleControls.bgGradientStop3Light}%, ${styleControls.bgGradientStop3Alpha}) 50%, ` +
            `hsla(${styleControls.bgGradientStop4Hue}, ${styleControls.bgGradientStop4Sat}%, ${styleControls.bgGradientStop4Light}%, ${styleControls.bgGradientStop4Alpha}) 75%, ` +
            `rgba(36, 36, 36, 0) 100%)`
        );
        
        // Welcome Popup styles
        root.style.setProperty('--style-welcome-overlay-opacity', styleControls.welcomeOverlayOpacity.toString());
        root.style.setProperty('--style-welcome-panel-bg-opacity', styleControls.welcomePanelBgOpacity.toString());
        root.style.setProperty('--style-welcome-panel-blur', `${styleControls.welcomePanelBlur}px`);
        root.style.setProperty('--style-welcome-border-opacity', styleControls.welcomeBorderOpacity.toString());
        root.style.setProperty('--style-welcome-card-bg-opacity', styleControls.welcomeCardBgOpacity.toString());
        root.style.setProperty('--style-welcome-card-hover-opacity', styleControls.welcomeCardHoverOpacity.toString());
        root.style.setProperty('--style-welcome-button-bg-opacity', styleControls.welcomeButtonBgOpacity.toString());
        root.style.setProperty('--style-welcome-button-border-opacity', styleControls.welcomeButtonBorderOpacity.toString());
        root.style.setProperty('--style-welcome-progress-dot-opacity', styleControls.welcomeProgressDotOpacity.toString());
        root.style.setProperty('--style-welcome-text-primary-opacity', styleControls.welcomeTextPrimaryOpacity.toString());
        root.style.setProperty('--style-welcome-text-secondary-opacity', styleControls.welcomeTextSecondaryOpacity.toString());
        root.style.setProperty('--style-welcome-text-tertiary-opacity', styleControls.welcomeTextTertiaryOpacity.toString());
        
        // Coffee mug styles
        root.style.setProperty('--style-coffee-mug-color', `hsla(${styleControls.coffeeMugHue}, ${styleControls.coffeeMugSaturation}%, ${styleControls.coffeeMugLightness}%, ${styleControls.coffeeMugOpacity})`);
        
        logger.debug('devDashboard', 'Applied style controls', styleControls);
    }
    

    // Reset individual property to default
    function resetProperty(propertyName: string) {
        if (propertyName in defaultValues) {
            styleControls[propertyName] = defaultValues[propertyName];
            applyStyleControls();
        }
    }

    // Reset all style controls to defaults
    function resetStyleControls() {
        styleControls = { ...defaultValues };
        applyStyleControls();
    }
    
    // Clean up event listeners on destroy
    onDestroy(() => {
        window.removeEventListener('mousemove', handleMouseMove);
        window.removeEventListener('mouseup', handleMouseUp);
        unsubscribeSettings();
        unsubscribeLLMState();
        unsubscribeStatistics();
        unsubscribeUserActivity();
        unsubscribeDocker();
        unsubscribeInternet();
        unsubscribeFFmpeg();
        unsubscribeMediaInfo();
    });
    
    // Keep dashboard in viewport when window is resized
    function handleResize() {
        if (!iconBubble && !dashboardPanel) return;
        
        const element = isExpanded ? dashboardPanel : iconBubble;
        if (element) {
            const rect = element.getBoundingClientRect();
            
            if (posX + rect.width > window.innerWidth) {
                posX = window.innerWidth - rect.width;
            }
            if (posY + rect.height > window.innerHeight) {
                posY = window.innerHeight - rect.height;
            }
        }
    }
    
    onMount(() => {
        // Log dashboard initialization only if shown
        if (showDashboard) {
            logger.info('devDashboard', `Developer dashboard initialized (version: ${version})`);
        }

        // Apply initial style controls
        applyStyleControls();
        applyProgressWaveControls();
        
        // Listen for progress state changes
        const handleStateChange = (event: CustomEvent) => {
            currentProgressState = event.detail.state;
        };
        window.addEventListener('progress-state-change', handleStateChange);

        // Watch for version changes after component is mounted
        const checkVersion = setInterval(() => {
            if (window && (window as any).__LANGKIT_VERSION === 'dev') {
                clearInterval(checkVersion);
                if (!showDashboard) {
                    // Update the component's version from global
                    version = 'dev';
                }
            }
        }, 1000);

        return () => {
            clearInterval(checkVersion);
            window.removeEventListener('progress-state-change', handleStateChange);
        };
       });
      
       onMount(async () => {
        // Sync the trace log state from the backend when the component mounts
        const traceLogsEnabled = await GetTraceLogs();
        enableTraceLogsStore.set(traceLogsEnabled);
       });
      </script>

{#if showDashboard}
    <!-- Floating bubble icon (minimized state) -->
    {#if !isExpanded}
        <Portal target="body">
            <div
                bind:this={iconBubble}
                class="dev-dashboard-icon"
                style="top: {posY}px; left: {posX}px;"
                on:mousedown={handleMouseDown}
                transition:scale={{duration: 300}}
                role="button"
                tabindex="0"
                aria-label="Open developer dashboard"
            >
                <!-- The button is now wrapped in a draggable container -->
                <div class="drag-handle">
                    <button
                        class="icon-button"
                        on:click|stopPropagation={toggleDashboard}
                        on:mousedown|stopPropagation
                        aria-label="Expand dashboard"
                    >
                        <span class="material-icons">developer_mode</span>
                    </button>
                </div>
            </div>
        </Portal>
    {:else}
        <!-- Expanded dashboard panel -->
        <Portal target="body">
            <div
                bind:this={dashboardPanel}
                class="dev-dashboard-panel"
                style="top: {posY}px; left: {posX}px;"
                transition:scale={{duration: 300}}
            >
                <!-- Header (draggable area) -->
                <div
                    class="dashboard-header"
                    on:mousedown={handleMouseDown}
                >
                    <h3>Developer Dashboard</h3>
                    <button
                        class="minimize-button"
                        on:click|stopPropagation={toggleDashboard}
                        on:mousedown|stopPropagation
                        aria-label="Minimize dashboard"
                    >
                        <span class="material-icons">remove</span>
                    </button>
                </div>

                <!-- Tab navigation -->
                <div class="tab-navigation">
                    {#each tabs as tab}
                        <button
                            class="tab-button {activeTab === tab.id ? 'active' : ''}"
                            on:click={() => switchTab(tab.id)}
                            aria-selected={activeTab === tab.id}
                        >
                            <span class="material-icons">{tab.icon}</span>
                            <span>{tab.name}</span>
                        </button>
                    {/each}
                </div>

                <!-- Content area -->
                <div class="dashboard-content">
                    {#if activeTab === 'performance'}
                        <WASMDashboard />
                    {:else if activeTab === 'state'}
                        <h4>Application State</h4>
                            
                            <div class="state-section">
                                <h5 class="text-xs font-semibold mb-2 opacity-80">Counter Values</h5>
                                <table class="state-table">
                                    <tbody>
                                        <tr>
                                            <td class="state-key">countAppStart</td>
                                            <td class="state-value">{currentStatistics?.countAppStart || 0}</td>
                                            <td class="state-description">App launch count</td>
                                        </tr>
                                        <tr>
                                            <td class="state-key">countProcessStart</td>
                                            <td class="state-value">{currentStatistics?.countProcessStart || 0}</td>
                                            <td class="state-description">Processing run count</td>
                                        </tr>
                                    </tbody>
                                </table>
                            </div>
                            
                            <div class="state-section">
                                <h5 class="text-xs font-semibold mb-2 opacity-80">File Settings</h5>
                                <table class="state-table">
                                    <tbody>
                                        <tr>
                                            <td class="state-key">intermediaryFileMode</td>
                                            <td class="state-value">{currentSettings?.intermediaryFileMode || 'keep'}</td>
                                            <td class="state-description">Intermediary file handling</td>
                                        </tr>
                                    </tbody>
                                </table>
                            </div>
                            
                            <div class="state-section">
                                <h5 class="text-xs font-semibold mb-2 opacity-80">User Activity</h5>
                                <table class="state-table">
                                    <tbody>
                                        <tr>
                                            <td class="state-key">userActivityState</td>
                                            <td class="state-value">
                                                <span class:text-green-400={currentUserActivityState === 'active'}
                                                      class:text-yellow-400={currentUserActivityState === 'idle'}
                                                      class:text-red-400={currentUserActivityState === 'afk'}>
                                                    {currentUserActivityState}
                                                    {#if isForced}
                                                        <span class="text-purple-400 text-xs">(forced)</span>
                                                    {/if}
                                                </span>
                                            </td>
                                            <td class="state-description">
                                                {#if currentUserActivityState === 'active'}
                                                    User is actively interacting
                                                {:else if currentUserActivityState === 'idle'}
                                                    No activity for 5s-5min
                                                {:else}
                                                    Away from keyboard >5min
                                                {/if}
                                            </td>
                                        </tr>
                                    </tbody>
                                </table>
                            </div>
                    {:else if activeTab === 'logs'}
                        <h4>Log Viewer Controls</h4>
                        <div class="control-section mb-4">
                        	<h5 class="text-xs font-semibold mb-2 opacity-80">Trace Logs</h5>
                        	<div class="flex items-center gap-3">
                        		<label class="switch">
                        			<input type="checkbox" bind:checked={$enableTraceLogsStore}>
                        			<span class="toggle-slider round"></span>
                        		</label>
                        		<span class="text-sm text-gray-300">Enable Trace Logs</span>
                        	</div>
                        	<p class="text-xs text-gray-500 mt-1">
                        		Streams verbose trace logs to the GUI log viewer. Impacts performance.
                        	</p>
                        </div>
                        
                        <div class="control-section mb-4">
                        	<h5 class="text-xs font-semibold mb-2 opacity-80">Frontend Logging</h5>
                        	<div class="flex items-center gap-3">
                        		<label class="switch">
                        			<input type="checkbox" bind:checked={$enableFrontendLoggingStore}>
                        			<span class="toggle-slider round"></span>
                        		</label>
                        		<span class="text-sm text-gray-300">Send Frontend Logs to Backend</span>
                        	</div>
                        	<p class="text-xs text-gray-500 mt-1">
                        		Forwards frontend logs to the backend for logging through zerolog.
                        	</p>
                        </div>
                        
                        <div class="control-section mb-4">
                        	<h5 class="text-xs font-semibold mb-2 opacity-80">Frontend Log Display</h5>
                        	<div class="flex items-center gap-3">
                        		<label class="switch">
                        			<input type="checkbox" bind:checked={$displayFrontendLogsStore}>
                        			<span class="toggle-slider round"></span>
                        		</label>
                        		<span class="text-sm text-gray-300">Display Frontend Logs in LogViewer</span>
                        	</div>
                        	<p class="text-xs text-gray-500 mt-1">
                        		Shows frontend logs directly in the LogViewer.
                        	</p>
                        </div>
                  
                        <div class="control-section mb-4">
                        	<h5 class="text-xs font-semibold mb-2 opacity-80">Virtualization</h5>
                        	<div class="flex flex-wrap gap-2">
                        		<button
                        			class="control-button"
                        			on:click={() => {
                        				// Toggle virtualization via document event
                        				const evt = new CustomEvent('dev:toggle-virtualization');
                        				document.dispatchEvent(evt);
                        			}}
                        		>
                        			Toggle Virtualization
                        		</button>
                  
                        	</div>
                        </div>
                  
                        <div class="control-section mb-4">
                        	<h5 class="text-xs font-semibold mb-2 opacity-80">Debug Tools</h5>
                        	<div class="flex flex-wrap gap-2">
                        		<button
                        			class="control-button"
                        			on:click={() => {
                        				// Toggle debug overlay via document event
                        				const evt = new CustomEvent('dev:toggle-debug-scroll');
                        				document.dispatchEvent(evt);
                        			}}
                        		>
                        			Debug Scroll Overlay
                        		</button>
                  
                        		<button
                        			class="control-button"
                        			on:click={() => {
                        				// Force scroll to bottom
                        				const evt = new CustomEvent('dev:force-scroll-bottom');
                        				document.dispatchEvent(evt);
                        			}}
                        		>
                        			Force Scroll to Bottom
                        		</button>
                        	</div>
                        </div>
                    {:else if activeTab === 'debug'}
                        <h4>Debug Controls</h4>

                            <!-- Memory testing section -->
                            <div class="memory-test-section mb-4">
                                <h5 class="text-xs font-semibold mb-2 opacity-80">Memory Testing</h5>
                                <div class="flex items-center gap-2">
                                    <MemoryTestButton
                                        size="medium"
                                        variant="primary"
                                    />
                                    <span class="text-xs opacity-70">Test WASM memory management</span>
                                </div>
                            </div>

                            <!-- LLM State Control section -->
                            <div class="control-section mb-4">
                                <h5 class="text-xs font-semibold mb-2 opacity-80">LLM State Control</h5>
                                <div class="text-xs text-gray-400 mb-2">
                                    Current state: <span class="font-mono text-primary">{currentLLMState?.globalState || 'unknown'}</span>
                                    {#if currentLLMState?.message?.startsWith('Debug: Forced')}
                                        <span class="text-purple-400 ml-2">(debug mode)</span>
                                    {:else if currentLLMState}
                                        <span class="text-green-400 ml-2">(real state)</span>
                                    {/if}
                                </div>
                                <div class="flex flex-wrap gap-2">
                                    <button
                                        class="control-button"
                                        on:click={() => forceLLMState('initializing')}
                                    >
                                        Force Initializing
                                    </button>
                                    <button
                                        class="control-button"
                                        on:click={() => forceLLMState('updating')}
                                    >
                                        Force Updating
                                    </button>
                                    <button
                                        class="control-button"
                                        on:click={() => forceLLMState('ready')}
                                    >
                                        Force Ready
                                    </button>
                                    <button
                                        class="control-button"
                                        on:click={() => forceLLMState('error')}
                                    >
                                        Force Error
                                    </button>
                                    <button
                                        class="control-button reset-button"
                                        on:click={() => resetLLMState()}
                                    >
                                        Reset to Real State
                                    </button>
                                </div>
                                <div class="text-xs text-gray-500 mt-2">
                                    Note: These are client-side only for UI testing
                                </div>
                            </div>

                            <!-- User Activity State Control section -->
                            <div class="control-section mb-4">
                                <h5 class="text-xs font-semibold mb-2 opacity-80">User Activity State Control</h5>
                                <div class="text-xs text-gray-400 mb-2">
                                    Current state: <span class="font-mono {currentUserActivityState === 'active' ? 'text-green-400' : currentUserActivityState === 'idle' ? 'text-yellow-400' : 'text-red-400'}">{currentUserActivityState}</span>
                                    {#if isForced}
                                        <span class="text-purple-400 ml-2">(forced)</span>
                                    {:else}
                                        <span class="text-green-400 ml-2">(auto)</span>
                                    {/if}
                                </div>
                                <div class="flex flex-wrap gap-2">
                                    <button
                                        class="control-button"
                                        on:click={() => forceUserActivityState('active')}
                                    >
                                        Force Active
                                    </button>
                                    <button
                                        class="control-button"
                                        on:click={() => forceUserActivityState('idle')}
                                    >
                                        Force Idle
                                    </button>
                                    <button
                                        class="control-button"
                                        on:click={() => forceUserActivityState('afk')}
                                    >
                                        Force AFK
                                    </button>
                                    <button
                                        class="control-button reset-button"
                                        on:click={() => resetUserActivityState()}
                                    >
                                        Reset to Auto
                                    </button>
                                </div>
                                <div class="text-xs text-gray-500 mt-2">
                                    Active: User is interacting | Idle: 5s-5min inactivity | AFK: >5min away
                                </div>
                            </div>

                            <!-- Docker Status Control section -->
                            <div class="control-section mb-4">
                                <h5 class="text-xs font-semibold mb-2 opacity-80">Docker Status Control</h5>
                                <div class="text-xs text-gray-400 mb-2">
                                    Status: <span class="font-mono {currentDockerStatus?.available ? 'text-green-400' : 'text-red-400'}">{currentDockerStatus?.available ? 'Available' : 'Unavailable'}</span>
                                    {#if dockerForced}
                                        <span class="text-purple-400 ml-2">(forced)</span>
                                    {:else if currentDockerStatus?.checked}
                                        <span class="text-green-400 ml-2">(real)</span>
                                    {:else}
                                        <span class="text-yellow-400 ml-2">(checking...)</span>
                                    {/if}
                                    {#if currentDockerStatus?.version}
                                        <span class="text-gray-500 ml-2">v{currentDockerStatus.version}</span>
                                    {/if}
                                </div>
                                <div class="flex flex-wrap gap-2">
                                    <button
                                        class="control-button"
                                        on:click={() => forceDockerStatus(true)}
                                    >
                                        Force Available
                                    </button>
                                    <button
                                        class="control-button"
                                        on:click={() => forceDockerStatus(false)}
                                    >
                                        Force Unavailable
                                    </button>
                                    <button
                                        class="control-button reset-button"
                                        on:click={() => resetDockerStatus()}
                                    >
                                        Reset to Real
                                    </button>
                                </div>
                                <div class="text-xs text-gray-500 mt-2">
                                    Controls Docker availability checks for features requiring Docker
                                </div>
                            </div>

                            <!-- Internet Status Control section -->
                            <div class="control-section mb-4">
                                <h5 class="text-xs font-semibold mb-2 opacity-80">Internet Status Control</h5>
                                <div class="text-xs text-gray-400 mb-2">
                                    Status: <span class="font-mono {currentInternetStatus?.online ? 'text-green-400' : 'text-red-400'}">{currentInternetStatus?.online ? 'Online' : 'Offline'}</span>
                                    {#if internetForced}
                                        <span class="text-purple-400 ml-2">(forced)</span>
                                    {:else if currentInternetStatus?.checked}
                                        <span class="text-green-400 ml-2">(real)</span>
                                    {:else}
                                        <span class="text-yellow-400 ml-2">(checking...)</span>
                                    {/if}
                                    {#if currentInternetStatus?.latency}
                                        <span class="text-gray-500 ml-2">{currentInternetStatus.latency}ms</span>
                                    {/if}
                                </div>
                                <div class="flex flex-wrap gap-2">
                                    <button
                                        class="control-button"
                                        on:click={() => forceInternetStatus(true)}
                                    >
                                        Force Online
                                    </button>
                                    <button
                                        class="control-button"
                                        on:click={() => forceInternetStatus(false)}
                                    >
                                        Force Offline
                                    </button>
                                    <button
                                        class="control-button reset-button"
                                        on:click={() => resetInternetStatus()}
                                    >
                                        Reset to Real
                                    </button>
                                </div>
                                <div class="text-xs text-gray-500 mt-2">
                                    Controls Internet connectivity checks for AI-powered features
                                </div>
                            </div>

                            <!-- FFmpeg Status Control section -->
                            <div class="control-section mb-4">
                                <h5 class="text-xs font-semibold mb-2 opacity-80">FFmpeg Status Control</h5>
                                <div class="text-xs text-gray-400 mb-2">
                                    Status: <span class="font-mono {currentFFmpegStatus?.available ? 'text-green-400' : 'text-red-400'}">{currentFFmpegStatus?.available ? 'Available' : 'Unavailable'}</span>
                                    {#if ffmpegForced}
                                        <span class="text-purple-400 ml-2">(forced)</span>
                                    {:else if currentFFmpegStatus?.checked}
                                        <span class="text-green-400 ml-2">(real)</span>
                                    {:else}
                                        <span class="text-yellow-400 ml-2">(checking...)</span>
                                    {/if}
                                    {#if currentFFmpegStatus?.version}
                                        <span class="text-gray-500 ml-2">v{currentFFmpegStatus.version}</span>
                                    {/if}
                                </div>
                                <div class="flex flex-wrap gap-2">
                                    <button
                                        class="control-button"
                                        on:click={() => forceFFmpegStatus(true)}
                                    >
                                        Force Available
                                    </button>
                                    <button
                                        class="control-button"
                                        on:click={() => forceFFmpegStatus(false)}
                                    >
                                        Force Unavailable
                                    </button>
                                    <button
                                        class="control-button reset-button"
                                        on:click={() => resetFFmpegStatus()}
                                    >
                                        Reset to Real
                                    </button>
                                </div>
                                <div class="text-xs text-gray-500 mt-2">
                                    Controls FFmpeg availability checks - required for all media processing
                                </div>
                            </div>

                            <!-- MediaInfo Status Control section -->
                            <div class="control-section mb-4">
                                <h5 class="text-xs font-semibold mb-2 opacity-80">MediaInfo Status Control</h5>
                                <div class="text-xs text-gray-400 mb-2">
                                    Status: <span class="font-mono {currentMediaInfoStatus?.available ? 'text-green-400' : 'text-red-400'}">{currentMediaInfoStatus?.available ? 'Available' : 'Unavailable'}</span>
                                    {#if mediainfoForced}
                                        <span class="text-purple-400 ml-2">(forced)</span>
                                    {:else if currentMediaInfoStatus?.checked}
                                        <span class="text-green-400 ml-2">(real)</span>
                                    {:else}
                                        <span class="text-yellow-400 ml-2">(checking...)</span>
                                    {/if}
                                    {#if currentMediaInfoStatus?.version}
                                        <span class="text-gray-500 ml-2">v{currentMediaInfoStatus.version}</span>
                                    {/if}
                                </div>
                                <div class="flex flex-wrap gap-2">
                                    <button
                                        class="control-button"
                                        on:click={() => forceMediaInfoStatus(true)}
                                    >
                                        Force Available
                                    </button>
                                    <button
                                        class="control-button"
                                        on:click={() => forceMediaInfoStatus(false)}
                                    >
                                        Force Unavailable
                                    </button>
                                    <button
                                        class="control-button reset-button"
                                        on:click={() => resetMediaInfoStatus()}
                                    >
                                        Reset to Real
                                    </button>
                                </div>
                                <div class="text-xs text-gray-500 mt-2">
                                    Controls MediaInfo availability checks - required for media analysis
                                </div>
                            </div>

                    {:else if activeTab === 'style'}
                        <h4>Style Controls</h4>
                        <div class="mb-3">
                            <div class="flex items-center gap-2 text-xs text-gray-400">
                                <span class="flex-shrink-0">Target color</span>
                                <input
                                    type="text"
                                    bind:value={targetColorHex}
                                    placeholder="#141215"
                                    class="w-20 px-1 py-1 text-xs bg-white/10 border border-white/20 rounded text-white flex-shrink-0 font-mono"
                                    style="max-width: 80px;"
                                />
                                <div
                                    class="h-6 rounded border border-white/30 flex-1 min-w-0"
                                    style="background-color: {targetColorHex}; min-height: 24px;"
                                ></div>
                            </div>
                        </div>

                        <!-- Style Sub-tabs -->
                        <div class="flex gap-2 mb-4 border-b border-white/10">
                            <button
                                class="px-3 py-2 text-xs {activeStyleSubTab === 'main' ? 'text-white border-b-2 border-primary' : 'text-white/60'}"
                                on:click={() => activeStyleSubTab = 'main'}
                            >
                                Main Interface
                            </button>
                            <button
                                class="px-3 py-2 text-xs {activeStyleSubTab === 'welcome' ? 'text-white border-b-2 border-primary' : 'text-white/60'}"
                                on:click={() => activeStyleSubTab = 'welcome'}
                            >
                                Welcome Popup
                            </button>
                            <button
                                class="px-3 py-2 text-xs {activeStyleSubTab === 'progress' ? 'text-white border-b-2 border-primary' : 'text-white/60'}"
                                on:click={() => activeStyleSubTab = 'progress'}
                            >
                                Progress
                            </button>
                            <button
                                class="px-3 py-2 text-xs {activeStyleSubTab === 'coffee' ? 'text-white border-b-2 border-primary' : 'text-white/60'}"
                                on:click={() => activeStyleSubTab = 'coffee'}
                            >
                                Coffee
                            </button>
                        </div>

                        {#if activeStyleSubTab === 'main'}
                        <!-- Background Controls -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Background</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {styleControls.bgHue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="270"
                                            max="290"
                                            step="1"
                                            bind:value={styleControls.bgHue}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgHue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {styleControls.bgSaturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.bgSaturation}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgSaturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {styleControls.bgLightness.toFixed(2)}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="7"
                                            step="0.01"
                                            bind:value={styleControls.bgLightness}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgLightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Feature Card Controls -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Feature Cards</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {styleControls.featureCardSaturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.featureCardSaturation}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('featureCardSaturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {styleControls.featureCardLightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.featureCardLightness}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('featureCardLightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Base Opacity: {styleControls.featureCardOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={styleControls.featureCardOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('featureCardOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Gradient Start: {styleControls.featureCardGradientStartOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={styleControls.featureCardGradientStartOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('featureCardGradientStartOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Gradient End: {styleControls.featureCardGradientEndOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={styleControls.featureCardGradientEndOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('featureCardGradientEndOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Media Input Controls -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Media Input</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {styleControls.mediaInputSaturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.mediaInputSaturation}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('mediaInputSaturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {styleControls.mediaInputLightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.mediaInputLightness}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('mediaInputLightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Opacity: {styleControls.mediaInputOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={styleControls.mediaInputOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('mediaInputOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Effect Controls -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Basic Effects</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Glow Opacity: {styleControls.glowOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={styleControls.glowOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('glowOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Glow Position & Size -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Glow Position & Size</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Position X: {styleControls.glowPositionX}vw</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="-100"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.glowPositionX}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('glowPositionX')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Position Y: {styleControls.glowPositionY}vw</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="-100"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.glowPositionY}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('glowPositionY')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Size: {styleControls.glowSize}vmax</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="5"
                                            max="80"
                                            step="1"
                                            bind:value={styleControls.glowSize}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('glowSize')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Blur: {styleControls.glowBlur}px</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="300"
                                            step="5"
                                            bind:value={styleControls.glowBlur}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('glowBlur')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Glow Animation -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Glow Animation</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Pulse Scale: {styleControls.glowAnimationScale.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="1"
                                            max="3"
                                            step="0.1"
                                            bind:value={styleControls.glowAnimationScale}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('glowAnimationScale')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Speed: {styleControls.glowAnimationSpeed}s</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="1"
                                            max="30"
                                            step="0.5"
                                            bind:value={styleControls.glowAnimationSpeed}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('glowAnimationSpeed')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Background Gradient Position -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Background Gradient Position</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Position X: {styleControls.bgGradientPosX}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="-100"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.bgGradientPosX}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientPosX')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Position Y: {styleControls.bgGradientPosY}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="-100"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.bgGradientPosY}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientPosY')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Background Gradient Stop 1 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Background Gradient Stop 1</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {styleControls.bgGradientStop1Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={styleControls.bgGradientStop1Hue}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop1Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {styleControls.bgGradientStop1Sat}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.bgGradientStop1Sat}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop1Sat')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {styleControls.bgGradientStop1Light}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.bgGradientStop1Light}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop1Light')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {styleControls.bgGradientStop1Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={styleControls.bgGradientStop1Alpha}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop1Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Background Gradient Stop 2 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Background Gradient Stop 2</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {styleControls.bgGradientStop2Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={styleControls.bgGradientStop2Hue}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop2Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {styleControls.bgGradientStop2Sat}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.bgGradientStop2Sat}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop2Sat')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {styleControls.bgGradientStop2Light}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.bgGradientStop2Light}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop2Light')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {styleControls.bgGradientStop2Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={styleControls.bgGradientStop2Alpha}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop2Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Background Gradient Stop 3 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Background Gradient Stop 3</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {styleControls.bgGradientStop3Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={styleControls.bgGradientStop3Hue}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop3Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {styleControls.bgGradientStop3Sat}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.bgGradientStop3Sat}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop3Sat')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {styleControls.bgGradientStop3Light}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.bgGradientStop3Light}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop3Light')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {styleControls.bgGradientStop3Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={styleControls.bgGradientStop3Alpha}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop3Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Background Gradient Stop 4 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Background Gradient Stop 4</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {styleControls.bgGradientStop4Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={styleControls.bgGradientStop4Hue}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop4Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {styleControls.bgGradientStop4Sat}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.bgGradientStop4Sat}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop4Sat')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {styleControls.bgGradientStop4Light}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.bgGradientStop4Light}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop4Light')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {styleControls.bgGradientStop4Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={styleControls.bgGradientStop4Alpha}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('bgGradientStop4Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                        {:else if activeStyleSubTab === 'welcome'}
                        <!-- Welcome Popup Controls -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Overlay & Panel</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Overlay Opacity: {styleControls.welcomeOverlayOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.05"
                                            bind:value={styleControls.welcomeOverlayOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('welcomeOverlayOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Panel BG Opacity: {styleControls.welcomePanelBgOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.05"
                                            bind:value={styleControls.welcomePanelBgOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('welcomePanelBgOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Panel Blur: {styleControls.welcomePanelBlur}px</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="48"
                                            step="2"
                                            bind:value={styleControls.welcomePanelBlur}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('welcomePanelBlur')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Border Opacity: {styleControls.welcomeBorderOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="0.5"
                                            step="0.05"
                                            bind:value={styleControls.welcomeBorderOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('welcomeBorderOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Cards & Buttons</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Card BG Opacity: {styleControls.welcomeCardBgOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="0.5"
                                            step="0.05"
                                            bind:value={styleControls.welcomeCardBgOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('welcomeCardBgOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Card Hover Opacity: {styleControls.welcomeCardHoverOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="0.5"
                                            step="0.05"
                                            bind:value={styleControls.welcomeCardHoverOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('welcomeCardHoverOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Button BG Opacity: {styleControls.welcomeButtonBgOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.05"
                                            bind:value={styleControls.welcomeButtonBgOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('welcomeButtonBgOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Button Border: {styleControls.welcomeButtonBorderOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.05"
                                            bind:value={styleControls.welcomeButtonBorderOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('welcomeButtonBorderOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Text & UI Elements</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Progress Dots: {styleControls.welcomeProgressDotOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.05"
                                            bind:value={styleControls.welcomeProgressDotOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('welcomeProgressDotOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Primary Text: {styleControls.welcomeTextPrimaryOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0.5"
                                            max="1"
                                            step="0.05"
                                            bind:value={styleControls.welcomeTextPrimaryOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('welcomeTextPrimaryOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Secondary Text: {styleControls.welcomeTextSecondaryOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0.3"
                                            max="1"
                                            step="0.05"
                                            bind:value={styleControls.welcomeTextSecondaryOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('welcomeTextSecondaryOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Tertiary Text: {styleControls.welcomeTextTertiaryOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0.3"
                                            max="1"
                                            step="0.05"
                                            bind:value={styleControls.welcomeTextTertiaryOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('welcomeTextTertiaryOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                        {:else if activeStyleSubTab === 'progress'}
                        <!-- Dynamic Wave Controls Based on Current State -->
                        <div class="mb-4">
                            <div class="flex items-center gap-2 mb-2">
                                <span class="text-xs text-gray-400">Current State:</span>
                                <span class="text-xs font-semibold {currentProgressState === 'normal' ? 'text-primary' : currentProgressState === 'error_task' ? 'text-yellow-400' : currentProgressState === 'error_all' ? 'text-red-400' : currentProgressState === 'user_cancel' ? 'text-gray-400' : 'text-green-400'}">
                                    {currentProgressState.replace('_', ' ').toUpperCase()}
                                </span>
                            </div>
                        </div>
                        
                        {#if currentProgressState === 'normal' || currentProgressState === 'complete'}
                        <!-- Normal State Wave Controls -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Wave 1</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {progressWaveControls.wave1Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={progressWaveControls.wave1Hue}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave1Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {progressWaveControls.wave1Saturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.wave1Saturation}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave1Saturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {progressWaveControls.wave1Lightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.wave1Lightness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave1Lightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {progressWaveControls.wave1Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={progressWaveControls.wave1Alpha}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave1Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Wave 2 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Wave 2</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {progressWaveControls.wave2Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={progressWaveControls.wave2Hue}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave2Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {progressWaveControls.wave2Saturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.wave2Saturation}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave2Saturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {progressWaveControls.wave2Lightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.wave2Lightness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave2Lightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {progressWaveControls.wave2Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={progressWaveControls.wave2Alpha}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave2Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Wave 3 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Wave 3</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {progressWaveControls.wave3Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={progressWaveControls.wave3Hue}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave3Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {progressWaveControls.wave3Saturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.wave3Saturation}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave3Saturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {progressWaveControls.wave3Lightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.wave3Lightness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave3Lightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {progressWaveControls.wave3Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={progressWaveControls.wave3Alpha}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave3Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Wave 4 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Wave 4</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {progressWaveControls.wave4Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={progressWaveControls.wave4Hue}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave4Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {progressWaveControls.wave4Saturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.wave4Saturation}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave4Saturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {progressWaveControls.wave4Lightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.wave4Lightness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave4Lightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {progressWaveControls.wave4Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={progressWaveControls.wave4Alpha}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('wave4Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        {:else if currentProgressState === 'error_task'}
                        <!-- Error Task Wave Controls -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80 text-yellow-400">Error Task - Wave 1</h5>
                            <div class="slider-grid">
                                <!-- Only showing Wave 1 and 4 for error states to keep it simple -->
                                <div class="slider-control">
                                    <label class="slider-label">Wave 1 Hue: {progressWaveControls.errorTaskWave1Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={progressWaveControls.errorTaskWave1Hue}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave1Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {progressWaveControls.errorTaskWave1Saturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorTaskWave1Saturation}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave1Saturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {progressWaveControls.errorTaskWave1Lightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorTaskWave1Lightness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave1Lightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {progressWaveControls.errorTaskWave1Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={progressWaveControls.errorTaskWave1Alpha}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave1Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Error Task Wave 2 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80 text-yellow-400">Error Task - Wave 2</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {progressWaveControls.errorTaskWave2Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={progressWaveControls.errorTaskWave2Hue}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave2Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {progressWaveControls.errorTaskWave2Saturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorTaskWave2Saturation}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave2Saturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {progressWaveControls.errorTaskWave2Lightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorTaskWave2Lightness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave2Lightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {progressWaveControls.errorTaskWave2Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={progressWaveControls.errorTaskWave2Alpha}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave2Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Error Task Wave 3 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80 text-yellow-400">Error Task - Wave 3</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {progressWaveControls.errorTaskWave3Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={progressWaveControls.errorTaskWave3Hue}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave3Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {progressWaveControls.errorTaskWave3Saturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorTaskWave3Saturation}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave3Saturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {progressWaveControls.errorTaskWave3Lightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorTaskWave3Lightness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave3Lightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {progressWaveControls.errorTaskWave3Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={progressWaveControls.errorTaskWave3Alpha}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave3Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Error Task Wave 4 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80 text-yellow-400">Error Task - Wave 4</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {progressWaveControls.errorTaskWave4Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={progressWaveControls.errorTaskWave4Hue}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave4Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {progressWaveControls.errorTaskWave4Saturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorTaskWave4Saturation}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave4Saturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {progressWaveControls.errorTaskWave4Lightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorTaskWave4Lightness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave4Lightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {progressWaveControls.errorTaskWave4Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={progressWaveControls.errorTaskWave4Alpha}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorTaskWave4Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        {:else if currentProgressState === 'error_all'}
                        <!-- Error All Wave Controls -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80 text-red-400">Error All - Wave 1</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Wave 1 Hue: {progressWaveControls.errorAllWave1Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={progressWaveControls.errorAllWave1Hue}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave1Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {progressWaveControls.errorAllWave1Saturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorAllWave1Saturation}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave1Saturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {progressWaveControls.errorAllWave1Lightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorAllWave1Lightness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave1Lightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {progressWaveControls.errorAllWave1Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={progressWaveControls.errorAllWave1Alpha}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave1Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Error All Wave 2 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80 text-red-400">Error All - Wave 2</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {progressWaveControls.errorAllWave2Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={progressWaveControls.errorAllWave2Hue}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave2Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {progressWaveControls.errorAllWave2Saturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorAllWave2Saturation}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave2Saturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {progressWaveControls.errorAllWave2Lightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorAllWave2Lightness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave2Lightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {progressWaveControls.errorAllWave2Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={progressWaveControls.errorAllWave2Alpha}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave2Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Error All Wave 3 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80 text-red-400">Error All - Wave 3</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {progressWaveControls.errorAllWave3Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={progressWaveControls.errorAllWave3Hue}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave3Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {progressWaveControls.errorAllWave3Saturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorAllWave3Saturation}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave3Saturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {progressWaveControls.errorAllWave3Lightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorAllWave3Lightness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave3Lightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {progressWaveControls.errorAllWave3Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={progressWaveControls.errorAllWave3Alpha}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave3Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                        

                        <!-- Error All Wave 4 -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80 text-red-400">Error All - Wave 4</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {progressWaveControls.errorAllWave4Hue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={progressWaveControls.errorAllWave4Hue}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave4Hue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {progressWaveControls.errorAllWave4Saturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorAllWave4Saturation}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave4Saturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {progressWaveControls.errorAllWave4Lightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={progressWaveControls.errorAllWave4Lightness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave4Lightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Alpha: {progressWaveControls.errorAllWave4Alpha.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={progressWaveControls.errorAllWave4Alpha}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('errorAllWave4Alpha')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                        

                        {:else}
                        <!-- User Cancel State -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80 text-gray-400">User Cancelled - Static Gradient Only</h5>
                            <p class="text-xs text-gray-500">Wave animations are disabled in cancel state. The interface shows a static gray gradient instead.</p>
                        </div>
                        {/if}

                        <!-- Wave Physics -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Wave Physics</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Wave Intensity: {progressWaveControls.waveIntensity}px</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="1"
                                            max="10"
                                            step="0.5"
                                            bind:value={progressWaveControls.waveIntensity}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('waveIntensity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Wave Frequency: {progressWaveControls.waveFrequency.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0.5"
                                            max="3"
                                            step="0.1"
                                            bind:value={progressWaveControls.waveFrequency}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('waveFrequency')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Animation & Effects -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Animation & Effects</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Animation Speed: {progressWaveControls.animationSpeed.toFixed(2)}x</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0.1"
                                            max="3"
                                            step="0.1"
                                            bind:value={progressWaveControls.animationSpeed}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('animationSpeed')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Blur Amount: {progressWaveControls.blurAmount.toFixed(1)}px</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="5"
                                            step="0.1"
                                            bind:value={progressWaveControls.blurAmount}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('blurAmount')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Additional Aesthetics -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Additional Aesthetics</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Background Darkness: {progressWaveControls.progressBgDarkness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="-50"
                                            max="0"
                                            step="1"
                                            bind:value={progressWaveControls.progressBgDarkness}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('progressBgDarkness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Overall Wave Opacity: {progressWaveControls.waveOverallOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.05"
                                            bind:value={progressWaveControls.waveOverallOpacity}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('waveOverallOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Wave Direction: {progressWaveControls.waveDirection === 1 ? 'Forward' : 'Reverse'}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="-1"
                                            max="1"
                                            step="2"
                                            bind:value={progressWaveControls.waveDirection}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('waveDirection')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Wave Stacking: {progressWaveControls.waveStackingMultiplier.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0.3"
                                            max="1.2"
                                            step="0.05"
                                            bind:value={progressWaveControls.waveStackingMultiplier}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('waveStackingMultiplier')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Wave Offset: {progressWaveControls.waveOffsetMultiplier.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0.3"
                                            max="2"
                                            step="0.05"
                                            bind:value={progressWaveControls.waveOffsetMultiplier}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('waveOffsetMultiplier')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Progress Bar Specific -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Progress Bar Effects</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Edge Glow: {progressWaveControls.progressEdgeGlow.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.05"
                                            bind:value={progressWaveControls.progressEdgeGlow}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('progressEdgeGlow')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Sweep Opacity: {progressWaveControls.progressSweepOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.05"
                                            bind:value={progressWaveControls.progressSweepOpacity}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('progressSweepOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Sweep Duration: {progressWaveControls.progressSweepDuration.toFixed(1)}s</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0.5"
                                            max="5"
                                            step="0.1"
                                            bind:value={progressWaveControls.progressSweepDuration}
                                            on:input={applyProgressWaveControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProgressWaveProperty('progressSweepDuration')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Progress Reset Controls -->
                        <div class="control-section">
                            <div class="flex gap-2">
                                <button
                                    class="control-button reset-button"
                                    on:click={resetProgressWaveControls}
                                >
                                    Reset Progress to Defaults
                                </button>
                                <button
                                    class="control-button"
                                    on:click={() => {
                                        logger.info('devDashboard', 'Current progress wave controls', progressWaveControls);
                                        navigator.clipboard.writeText(JSON.stringify(progressWaveControls, null, 2));
                                    }}
                                >
                                    Copy Progress Values
                                </button>
                            </div>
                        </div>
                        {:else if activeStyleSubTab === 'coffee'}
                        <!-- Coffee Mug Controls -->
                        <div class="control-section">
                            <h5 class="text-xs font-semibold mb-2 opacity-80">Coffee Mug Color</h5>
                            <div class="slider-grid">
                                <div class="slider-control">
                                    <label class="slider-label">Hue: {styleControls.coffeeMugHue}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="360"
                                            step="1"
                                            bind:value={styleControls.coffeeMugHue}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('coffeeMugHue')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Saturation: {styleControls.coffeeMugSaturation}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.coffeeMugSaturation}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('coffeeMugSaturation')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Lightness: {styleControls.coffeeMugLightness}%</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="100"
                                            step="1"
                                            bind:value={styleControls.coffeeMugLightness}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('coffeeMugLightness')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                                <div class="slider-control">
                                    <label class="slider-label">Opacity: {styleControls.coffeeMugOpacity.toFixed(2)}</label>
                                    <div class="slider-row">
                                        <input
                                            type="range"
                                            min="0"
                                            max="1"
                                            step="0.01"
                                            bind:value={styleControls.coffeeMugOpacity}
                                            on:input={applyStyleControls}
                                            class="slider"
                                        />
                                        <button
                                            class="reset-button"
                                            on:click={() => resetProperty('coffeeMugOpacity')}
                                            title="Reset to default"
                                        >
                                            ↺
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                        {/if}

                        <!-- Reset Controls -->
                        <div class="control-section">
                            <div class="flex gap-2">
                                <button
                                    class="control-button reset-button"
                                    on:click={resetStyleControls}
                                >
                                    Reset to Defaults
                                </button>
                                <button
                                    class="control-button"
                                    on:click={() => {
                                        logger.info('devDashboard', 'Current style controls', styleControls);
                                        navigator.clipboard.writeText(JSON.stringify(styleControls, null, 2));
                                    }}
                                >
                                    Copy Values
                                </button>
                            </div>
                        </div>

                    {/if}
                </div>
            </div>
        </Portal>
    {/if}
{/if}

<style>
    /* Base styles for the draggable icon */
    .dev-dashboard-icon {
        position: fixed !important;
        /* z-index moved to app.css */
        user-select: none;
        cursor: move;
        filter: drop-shadow(0 2px 5px rgba(0, 0, 0, 0.2));
        background: transparent;
    }

    .drag-handle {
        width: 64px;
        height: 64px;
        cursor: move;
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 32px;
        background: hsla(215, 15%, 22%, 0.9); /* Match icon button background */
        border: 1px solid hsla(215, 20%, 30%, 0.5);
    }

    .icon-button {
        width: 40px;
        height: 40px;
        border-radius: 20px;
        background: hsla(215, 15%, 22%, 0.9); /* More subtle, sober color */
        border: 1px solid hsla(215, 20%, 30%, 0.5);
        color: hsla(0, 0%, 90%, 0.9);
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
        padding: 0;
        transition: transform 0.2s ease-out, box-shadow 0.2s ease-out;
        box-shadow: 0 3px 10px rgba(0, 0, 0, 0.25);
    }
    
    .icon-button:hover {
        transform: scale(1.05);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
    }
    
    .icon-button:active {
        transform: scale(0.95);
    }
    
    /* Expanded dashboard panel styles */
    .dev-dashboard-panel {
        position: fixed !important;
        /* z-index moved to app.css */
        user-select: none;
        width: 480px; /* Increased width for better content display */
        background: hsla(215, 15%, 15%, 0.9);
        border-radius: 12px;
        box-shadow: 0 8px 20px rgba(0, 0, 0, 0.3),
                    0 0 0 1px hsla(215, 25%, 35%, 0.3);
        backdrop-filter: blur(10px);
        overflow: hidden;
        color: white;
        cursor: move;
    }
    
    .dashboard-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 8px 12px;
        background: hsla(215, 15%, 20%, 0.9); /* More sober, subtle header */
        border-bottom: 1px solid hsla(215, 20%, 25%, 0.5);
        cursor: move;
    }
    
    .dashboard-header h3 {
        margin: 0;
        font-size: 14px;
        font-weight: 600;
    }
    
    .minimize-button {
        width: 24px;
        height: 24px;
        border-radius: 12px;
        background: rgba(255, 255, 255, 0.1);
        border: none;
        color: white;
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
        padding: 0;
        transition: background 0.2s;
    }
    
    .minimize-button:hover {
        background: rgba(255, 255, 255, 0.2);
    }
    
    .minimize-button .material-icons {
        font-size: 18px;
    }
    
    /* Tab navigation */
    .tab-navigation {
        display: flex;
        background: rgba(0, 0, 0, 0.2);
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    }
    
    .tab-button {
        flex: 1;
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 6px;
        background: transparent;
        border: none;
        color: rgba(255, 255, 255, 0.6);
        cursor: pointer;
        font-size: 11px;
        transition: all 0.2s;
    }
    
    .tab-button .material-icons {
        font-size: 16px;
        margin-bottom: 2px;
    }
    
    .tab-button:hover {
        color: rgba(255, 255, 255, 0.9);
        background: rgba(255, 255, 255, 0.05);
    }
    
    .tab-button.active {
        color: white;
        background: rgba(var(--primary-rgb), 0.2);
        box-shadow: inset 0 -2px 0 var(--primary-color);
    }
    
    /* Content area */
    .dashboard-content {
        padding: 12px;
        max-height: 400px;
        overflow-y: auto;
    }
    
    .dashboard-content h4 {
        margin: 0 0 12px 0;
        font-size: 13px;
        opacity: 0.9;
    }

    /* Control sections layout */
    .control-section {
        margin-bottom: 16px;
    }

    .control-button {
        padding: 6px 10px;
        background: hsla(215, 20%, 20%, 0.9);
        border: 1px solid hsla(215, 30%, 40%, 0.4);
        border-radius: 4px;
        color: white;
        font-size: 12px;
        cursor: pointer;
        transition: all 0.2s;
        min-width: 135px;
        text-align: center;
    }

    .control-button:hover {
        background: hsla(215, 20%, 30%, 0.9);
        border-color: hsla(215, 30%, 50%, 0.4);
        box-shadow: 0 0 4px rgba(159, 110, 247, 0.3);
    }
    
    .control-button.reset-button {
        background: hsla(0, 85%, 60%, 0.7) !important;
        border-color: hsla(0, 85%, 60%, 0.5) !important;
    }
    
    .control-button.reset-button:hover {
        background: hsla(0, 85%, 60%, 0.9) !important;
        border-color: hsla(0, 85%, 60%, 0.7) !important;
        box-shadow: 0 0 4px rgba(239, 68, 68, 0.4) !important;
    }

    .control-section {
        margin-bottom: 12px;
        padding-bottom: 8px;
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    }

    .control-section:last-child {
        border-bottom: none;
        margin-bottom: 0;
    }
    
    /* Filter controls */
    .filter-controls {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 8px;
    }
    
    .checkbox-label {
        display: flex;
        align-items: center;
        font-size: 12px;
        cursor: pointer;
    }
    
    .checkbox-label input {
        margin-right: 6px;
    }
    
    /* Debug controls */
    .debug-controls {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }
    
    .debug-button, .control-button {
        padding: 6px 10px;
        background: rgba(255, 255, 255, 0.1);
        border: 1px solid rgba(255, 255, 255, 0.2);
        border-radius: 4px;
        color: white;
        font-size: 12px;
        cursor: pointer;
        transition: all 0.2s;
    }

    .debug-button:hover, .control-button:hover {
        background: rgba(255, 255, 255, 0.15);
        border-color: rgba(255, 255, 255, 0.3);
    }
    
    /* State tab styles */
    .state-section {
        margin-bottom: 16px;
        padding-bottom: 12px;
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    }
    
    .state-section:last-child {
        border-bottom: none;
        margin-bottom: 0;
    }
    
    .state-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 12px;
    }
    
    .state-table tr {
        border-bottom: 1px solid rgba(255, 255, 255, 0.05);
    }
    
    .state-table tr:last-child {
        border-bottom: none;
    }
    
    .state-key {
        width: 40%;
        padding: 6px 4px;
        color: var(--primary-color, #9f6ef7);
        font-family: monospace;
    }
    
    .state-value {
        width: 20%;
        padding: 6px 4px;
        color: rgba(255, 255, 255, 0.9);
        font-weight: 600;
    }
    
    .state-description {
        width: 40%;
        padding: 6px 4px;
        color: rgba(255, 255, 255, 0.6);
        font-style: italic;
    }
    
    /* Activity state colors */
    .text-green-400 {
        color: #68e796;
    }
    
    .text-yellow-400 {
        color: #fbbf24;
    }
    
    .text-red-400 {
        color: #f87171;
    }
    
    /* Style controls specific styles */
    .slider-grid {
        display: grid;
        grid-template-columns: 1fr 1fr;
        gap: 8px;
    }
    
    .slider-control {
        display: flex;
        flex-direction: column;
        gap: 4px;
    }
    
    .slider-row {
        display: flex;
        align-items: center;
        gap: 4px;
    }
    
    .slider-row input {
        flex: 1;
    }
    
    .slider-row .reset-button {
        padding: 2px 6px;
        background: hsla(0, 85%, 60%, 0.7);
        border: 1px solid hsla(0, 85%, 60%, 0.5);
        border-radius: 3px;
        color: white;
        font-size: 10px;
        cursor: pointer;
        transition: all 0.2s;
        min-width: 28px;
        height: 20px;
        display: flex;
        align-items: center;
        justify-content: center;
    }
    
    .slider-row .reset-button:hover {
        background: hsla(0, 85%, 60%, 0.9);
        border-color: hsla(0, 85%, 60%, 0.7);
        transform: scale(1.05);
    }
    
    .slider-label {
        font-size: 11px;
        color: rgba(255, 255, 255, 0.8);
        font-weight: 500;
    }
    
    .slider {
        -webkit-appearance: none;
        appearance: none;
        height: 4px;
        background: rgba(255, 255, 255, 0.1);
        border-radius: 2px;
        outline: none;
        cursor: pointer;
    }
    
    .slider::-webkit-slider-thumb {
        -webkit-appearance: none;
        appearance: none;
        width: 16px;
        height: 16px;
        background: var(--primary-color, #9f6ef7);
        border-radius: 50%;
        cursor: pointer;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
        transition: all 0.2s;
    }
    
    .slider::-webkit-slider-thumb:hover {
        transform: scale(1.1);
        box-shadow: 0 0 8px rgba(159, 110, 247, 0.5);
    }
    
    .slider::-moz-range-thumb {
        width: 16px;
        height: 16px;
        background: var(--primary-color, #9f6ef7);
        border-radius: 50%;
        cursor: pointer;
        border: none;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
        transition: all 0.2s;
    }
    
    .slider::-moz-range-thumb:hover {
    	transform: scale(1.1);
    	box-shadow: 0 0 8px rgba(159, 110, 247, 0.5);
    }
   
    /* Toggle Switch Styles */
    .switch {
    	position: relative;
    	display: inline-block;
    	width: 48px;
    	height: 26px;
    }
   
    .switch input {
    	opacity: 0;
    	width: 0;
    	height: 0;
    }
   
    .switch .toggle-slider {
    	position: absolute;
    	cursor: pointer;
    	top: 0;
    	left: 0;
    	right: 0;
    	bottom: 0;
    	background-color: #4b5563; /* gray-600 */
    	transition: .4s;
    }
   
    .switch .toggle-slider:before {
    	position: absolute;
    	content: "";
    	height: 18px;
    	width: 18px;
    	left: 4px;
    	bottom: 4px;
    	background-color: white;
    	transition: .4s;
    }
   
    .switch input:checked + .toggle-slider {
    	background-color: hsl(261, 90%, 70%); /* primary violet */
    	box-shadow: 0 0 8px hsla(261, 90%, 70%, 0.4);
    }
   
    .switch input:focus + .toggle-slider {
    	box-shadow: 0 0 1px hsl(261, 90%, 70%);
    }
   
    .switch input:checked + .toggle-slider:before {
    	transform: translateX(22px);
    }
   
    .switch .toggle-slider.round {
    	border-radius: 26px;
    }
   
    .switch .toggle-slider.round:before {
    	border-radius: 50%;
    }
   </style>