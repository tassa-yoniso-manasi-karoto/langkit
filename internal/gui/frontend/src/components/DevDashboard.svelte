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
    import DraggableContainer from './dev/DraggableContainer.svelte';
    import LogsDebugDashboard from './dev/LogsDebugDashboard.svelte';
    import StateDebugDashboard from './dev/StateDebugDashboard.svelte';
    import DebugControlsDashboard from './dev/DebugControlsDashboard.svelte';
    import StyleDebugDashboard from './dev/StyleDebugDashboard.svelte';
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
    
    // State variables
    let isExpanded = false;
    let position = { x: 20, y: 20 }; // Initial position
    let isDragging = false; // For DraggableContainer
    
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
    
    // Handle position update from DraggableContainer
    function handlePositionChange(event: CustomEvent) {
        position = event.detail;
    }
    
    // Handle drag end
    function handleDragEnd(event: CustomEvent) {
        isDragging = event.detail.isDragging;
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
            
            if (position.x + rect.width > window.innerWidth) {
                position.x = window.innerWidth - rect.width;
            }
            if (position.y + rect.height > window.innerHeight) {
                position.y = window.innerHeight - rect.height;
            }
            // Trigger reactivity
            position = position;
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
            <DraggableContainer
                {position}
                {isDragging}
                on:positionChange={handlePositionChange}
                on:dragEnd={handleDragEnd}
                zIndex="--z-index-dev-dashboard"
            >
                <div
                    bind:this={iconBubble}
                    class="dev-dashboard-icon"
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
            </DraggableContainer>
        </Portal>
    {:else}
        <!-- Expanded dashboard panel -->
        <Portal target="body">
            <DraggableContainer
                {position}
                {isDragging}
                on:positionChange={handlePositionChange}
                on:dragEnd={handleDragEnd}
                zIndex="--z-index-dev-dashboard"
                handleSelector=".dashboard-header"
            >
                <div
                    bind:this={dashboardPanel}
                    class="dev-dashboard-panel"
                    transition:scale={{duration: 300}}
                >
                    <!-- Header (draggable area) -->
                    <div
                        class="dashboard-header"
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
                            {#if tab.id === 'performance'}
                                <svg class="tab-icon" xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512" fill="currentColor">
                                    <path fill="currentColor" d="m394.252 320.607l16.984 66.557h-46.052l14.796-66.557zM314.612 0H512v512H0V0h197.388c0 81.883 117.224 81.883 117.224 0M152.667 275.934h-33.92l39.059 181.26h34.348l27.094-123.361l25.191 123.361h33.683l43.198-181.26h-33.255l-26.69 124.931l-25.356-124.931h-31.733l-28.188 123.409zm313.756 181.26l-53.331-181.26H359.57l-43.864 181.26h34.111l8.897-40.344h60.943l11.56 40.344z"/>
                                </svg>
                            {:else}
                                <span class="material-icons">{tab.icon}</span>
                            {/if}
                            <span>{tab.name}</span>
                        </button>
                    {/each}
                </div>

                <!-- Content area -->
                <div class="dashboard-content">
                    {#if activeTab === 'performance'}
                        <WASMDashboard />
                    {:else if activeTab === 'state'}
                        <StateDebugDashboard 
                            {currentStatistics}
                            {currentSettings}
                            {currentUserActivityState}
                            {isForced}
                        />
                    {:else if activeTab === 'logs'}
                        <LogsDebugDashboard />
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

                            <DebugControlsDashboard
                                currentLLMState={currentLLMState}
                                currentUserActivityState={currentUserActivityState}
                                isForced={isForced}
                                currentDockerStatus={currentDockerStatus}
                                dockerForced={dockerForced}
                                currentInternetStatus={currentInternetStatus}
                                internetForced={internetForced}
                                currentFFmpegStatus={currentFFmpegStatus}
                                ffmpegForced={ffmpegForced}
                                currentMediaInfoStatus={currentMediaInfoStatus}
                                mediainfoForced={mediainfoForced}
                            />

                    {:else if activeTab === 'style'}
                        <StyleDebugDashboard
                            styleControls={styleControls}
                            progressWaveControls={progressWaveControls}
                            targetColorHex={targetColorHex}
                            currentProgressState={currentProgressState}
                            activeStyleSubTab={activeStyleSubTab}
                            applyStyleControls={applyStyleControls}
                            resetProperty={resetProperty}
                            resetStyleControls={resetStyleControls}
                            applyProgressWaveControls={applyProgressWaveControls}
                            resetProgressWaveProperty={resetProgressWaveProperty}
                            onStyleControlChange={(property, value) => { styleControls[property] = value; }}
                            onProgressWaveControlChange={(property, value) => { progressWaveControls[property] = value; }}
                            onTargetColorChange={(value) => { targetColorHex = value; }}
                            onStyleSubTabChange={(tab) => { activeStyleSubTab = tab; }}
                            onProgressStateChange={(state) => { currentProgressState = state; }}
                        />
                    {/if}
                </div>
            </div>
            </DraggableContainer>
        </Portal>
    {/if}
{/if}

<style>
    /* Base styles for the draggable icon */
    .dev-dashboard-icon {
        /* z-index moved to app.css */
        user-select: none;
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
    
    .tab-button .tab-icon {
        width: 16px;
        height: 16px;
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
    
    
</style>