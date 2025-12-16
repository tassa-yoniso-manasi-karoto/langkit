<script lang="ts">
    import { defaultValues, defaultProgressWaveValues } from '../../lib/dev/styleControlsDefaults';
    import { liteModeStore } from '../../lib/stores';

    // Track lite mode for Qt+Windows compatibility testing
    $: liteMode = $liteModeStore.enabled;
    $: liteModeReason = $liteModeStore.reason;

    function toggleLiteMode() {
        // Toggle via debug override
        liteModeStore.setDebugOverride(!liteMode);
    }

    // Props
    export let styleControls: typeof defaultValues;
    export let progressWaveControls: typeof defaultProgressWaveValues;
    export let targetColorHex: string;
    export let currentProgressState: 'normal' | 'error_task' | 'error_all' | 'user_cancel' | 'complete';
    export let activeStyleSubTab: string;
    
    // Functions passed as props
    export let applyStyleControls: () => void;
    export let resetProperty: (propertyName: string) => void;
    export let resetStyleControls: () => void;
    export let applyProgressWaveControls: () => void;
    export let resetProgressWaveProperty: (property: string) => void;
    export let onStyleControlChange: (property: string, value: number) => void;
    export let onProgressWaveControlChange: (property: string, value: number) => void;
    export let onTargetColorChange: (value: string) => void;
    export let onStyleSubTabChange: (tab: string) => void;
    export let onProgressStateChange: (state: typeof currentProgressState) => void;
    
    // Configuration objects for data-driven UI
    const mainInterfaceConfig = {
        background: {
            title: 'Background',
            controls: [
                { key: 'bgHue', label: 'Hue', min: 0, max: 360 },
                { key: 'bgSaturation', label: 'Saturation', min: 0, max: 100, suffix: '%' },
                { key: 'bgLightness', label: 'Lightness', min: 0, max: 20, step: 0.05, decimals: 2, suffix: '%' },
                { key: 'bgOpacity', label: 'Opacity', min: 0, max: 1, step: 0.01, decimals: 2 }
            ]
        },
        featureCards: {
            title: 'Feature Cards',
            controls: [
                { key: 'featureCardSaturation', label: 'Saturation', min: 0, max: 100, suffix: '%' },
                { key: 'featureCardLightness', label: 'Lightness', min: 0, max: 100, suffix: '%' },
                { key: 'featureCardOpacity', label: 'Opacity', min: 0, max: 1, step: 0.01, decimals: 2 },
                { key: 'featureCardGradientStartOpacity', label: 'Gradient Start', min: 0, max: 1, step: 0.01, decimals: 2 },
                { key: 'featureCardGradientEndOpacity', label: 'Gradient End', min: 0, max: 1, step: 0.01, decimals: 2 }
            ]
        },
        mediaInput: {
            title: 'Media Input',
            controls: [
                { key: 'mediaInputSaturation', label: 'Saturation', min: 0, max: 100, suffix: '%' },
                { key: 'mediaInputLightness', label: 'Lightness', min: 0, max: 100, suffix: '%' },
                { key: 'mediaInputOpacity', label: 'Opacity', min: 0, max: 1, step: 0.01, decimals: 2 }
            ]
        },
        glowEffect: {
            title: 'Glow Effect',
            controls: [
                { key: 'glowOpacity', label: 'Opacity', min: 0, max: 1, step: 0.01, decimals: 2 }
            ]
        },
        backgroundGradient: {
            title: 'Background Gradient',
            controls: [
                { key: 'bgGradientPosX', label: 'Position X', min: 0, max: 100, suffix: '%' },
                { key: 'bgGradientPosY', label: 'Position Y', min: 0, max: 100, suffix: '%' }
            ]
        },
        glowPosition: {
            title: 'Glow Position & Properties',
            controls: [
                { key: 'glowPositionX', label: 'Position X', min: -50, max: 150, suffix: 'vw' },
                { key: 'glowPositionY', label: 'Position Y', min: -50, max: 150, suffix: 'vw' },
                { key: 'glowSize', label: 'Size', min: 10, max: 100, suffix: 'vmax' },
                { key: 'glowBlur', label: 'Blur', min: 20, max: 200, suffix: 'px' }
            ]
        },
        glowAnimation: {
            title: 'Glow Animation',
            controls: [
                { key: 'glowAnimationScale', label: 'Animation Scale', min: 1, max: 3, step: 0.1, decimals: 1 },
                { key: 'glowAnimationSpeed', label: 'Animation Speed', min: 1, max: 30, suffix: 's' }
            ]
        }
    };
    
    const welcomeConfig = {
        overlayPanel: {
            title: 'Overlay & Panel',
            controls: [
                { key: 'welcomeOverlayOpacity', label: 'Overlay Opacity', min: 0, max: 1, step: 0.01, decimals: 2 },
                { key: 'welcomePanelBgOpacity', label: 'Panel BG', min: 0, max: 1, step: 0.01, decimals: 2 },
                { key: 'welcomePanelBlur', label: 'Panel Blur', min: 0, max: 48, suffix: 'px' },
                { key: 'welcomeBorderOpacity', label: 'Border Opacity', min: 0, max: 1, step: 0.01, decimals: 2 }
            ]
        },
        cardsButtons: {
            title: 'Cards & Buttons',
            controls: [
                { key: 'welcomeCardBgOpacity', label: 'Card BG', min: 0, max: 1, step: 0.01, decimals: 2 },
                { key: 'welcomeCardHoverOpacity', label: 'Card Hover', min: 0, max: 1, step: 0.01, decimals: 2 },
                { key: 'welcomeButtonBgOpacity', label: 'Button BG', min: 0, max: 1, step: 0.01, decimals: 2 },
                { key: 'welcomeButtonBorderOpacity', label: 'Button Border', min: 0, max: 1, step: 0.01, decimals: 2 }
            ]
        },
        uiElements: {
            title: 'UI Elements',
            controls: [
                { key: 'welcomeProgressDotOpacity', label: 'Progress Dots', min: 0, max: 1, step: 0.01, decimals: 2 }
            ]
        },
        textOpacity: {
            title: 'Text Opacity',
            controls: [
                { key: 'welcomeTextPrimaryOpacity', label: 'Primary Text', min: 0, max: 1, step: 0.01, decimals: 2 },
                { key: 'welcomeTextSecondaryOpacity', label: 'Secondary Text', min: 0, max: 1, step: 0.01, decimals: 2 },
                { key: 'welcomeTextTertiaryOpacity', label: 'Tertiary Text', min: 0, max: 1, step: 0.01, decimals: 2 }
            ]
        }
    };
    
    const progressConfig = {
        wavePhysics: {
            title: 'Wave Physics',
            controls: [
                { key: 'waveIntensity', label: 'Wave Intensity', min: 1, max: 10, step: 0.5, suffix: 'px' },
                { key: 'waveFrequency', label: 'Wave Frequency', min: 0.5, max: 3, step: 0.1, decimals: 2 }
            ]
        },
        animationEffects: {
            title: 'Animation & Effects',
            controls: [
                { key: 'animationSpeed', label: 'Animation Speed', min: 0.1, max: 3, step: 0.1, decimals: 2, suffix: 'x' },
                { key: 'blurAmount', label: 'Blur Amount', min: 0, max: 5, step: 0.1, decimals: 1, suffix: 'px' }
            ]
        },
        progressBar: {
            title: 'Progress Bar',
            controls: [
                { key: 'progressEdgeGlow', label: 'Edge Glow', min: 0, max: 1, step: 0.01, decimals: 2 },
                { key: 'progressSweepOpacity', label: 'Sweep Opacity', min: 0, max: 1, step: 0.01, decimals: 2 },
                { key: 'progressSweepDuration', label: 'Sweep Duration', min: 0.5, max: 10, step: 0.1, decimals: 1, suffix: 's' }
            ]
        },
        aesthetics: {
            title: 'Additional Aesthetics',
            controls: [
                { key: 'progressBgDarkness', label: 'BG Darkness', min: -50, max: 50, suffix: '%' },
                { key: 'waveOverallOpacity', label: 'Wave Opacity', min: 0, max: 1, step: 0.01, decimals: 2 },
                { key: 'waveDirection', label: 'Direction', min: -1, max: 1, step: 2 },
                { key: 'waveStackingMultiplier', label: 'Stacking', min: 0.5, max: 1.5, step: 0.01, decimals: 2 },
                { key: 'waveOffsetMultiplier', label: 'Offset', min: 0.5, max: 2, step: 0.01, decimals: 2 }
            ]
        }
    };
    
    const coffeeConfig = {
        coffeeMug: {
            title: 'Coffee Mug Color',
            controls: [
                { key: 'coffeeMugHue', label: 'Hue', min: 0, max: 360 },
                { key: 'coffeeMugSaturation', label: 'Saturation', min: 0, max: 100, suffix: '%' },
                { key: 'coffeeMugLightness', label: 'Lightness', min: 0, max: 100, suffix: '%' },
                { key: 'coffeeMugOpacity', label: 'Opacity', min: 0, max: 1, step: 0.01, decimals: 2 }
            ]
        }
    };
    
    const gradientStops = [1, 2, 3, 4];
    const waveNumbers = [1, 2, 3, 4];
    
    // Unified handlers
    function handleStyleChange(property: string, value: number) {
        onStyleControlChange(property, value);
        applyStyleControls();
    }
    
    function handleProgressWaveChange(property: string, value: number) {
        onProgressWaveControlChange(property, value);
        applyProgressWaveControls();
    }
    
    function copyExportedValues() {
        const exported = JSON.stringify(styleControls, null, 2);
        navigator.clipboard.writeText(exported);
    }
    
    // Note: Progress state is derived from actual processing conditions
    // and cannot be manually simulated as it depends on multiple factors
    
    // Format value for display
    function formatValue(value: number, config: any) {
        if (config.decimals !== undefined) {
            return value.toFixed(config.decimals);
        }
        return value.toString();
    }
</script>

<!-- Reusable Slider Component
Why I kept it inline:

1. Single-use context - These sliders are only used in StyleDebugDashboard, nowhere else in the codebase. Creating a separate component for single-use adds
complexity without benefit.
2. Shared styling scope - All sliders share the exact same CSS. With a separate component, you'd either:
 - Duplicate styles in both files
 - Import shared styles (another file)
 - Pass style props (prop drilling)
 - Use global styles (loses scoping)
3. Performance - Each component boundary has overhead. With 100+ sliders, that's 100+ component instances vs direct DOM elements.
4. Maintainability paradox - While separate components seem cleaner, for UI-heavy components like this, seeing all the logic in one place can actually be easier to maintain. You don't have to jump between files to understand the flow.
-->
{#if false}
<!-- This is a template for how SliderControl would look if it were a separate component -->
<script>
    export let label;
    export let value;
    export let min = 0;
    export let max = 100;
    export let step = 1;
    export let suffix = '';
    export let decimals = 0;
    export let onChange;
    export let onReset;
</script>
{/if}

<!-- Style Sub-tabs -->
<div class="flex gap-2 mb-4 border-b border-white/10">
    {#each ['main', 'welcome', 'progress', 'coffee'] as tab}
        <button
            class="px-3 py-2 text-xs {activeStyleSubTab === tab ? 'text-white border-b-2 border-primary' : 'text-white/60'}"
            on:click={() => onStyleSubTabChange(tab)}
        >
            {tab === 'main' ? 'Main Interface' : tab.charAt(0).toUpperCase() + tab.slice(1) + (tab === 'welcome' ? ' Popup' : '')}
        </button>
    {/each}
</div>

{#if activeStyleSubTab === 'main'}
    <!-- Qt Windows Compatibility -->
    <div class="control-section">
        <h5 class="text-xs font-semibold mb-2 opacity-80">Qt Windows Compatibility</h5>
        <div class="flex items-center gap-3 mb-2">
            <button
                class="toggle-button {liteMode ? 'active' : ''}"
                on:click={toggleLiteMode}
                title="Toggle lite mode for Qt WebEngine on Windows"
            >
                <span class="toggle-slider"></span>
            </button>
            <span class="text-xs text-white/80">
                Lite Mode
                {#if liteMode}
                    <span class="text-primary ml-1">(ON)</span>
                {:else}
                    <span class="text-white/40 ml-1">(OFF)</span>
                {/if}
            </span>
        </div>
        <div class="text-xs text-white/50 mb-2">
            Disables backdrop-filter blur effects to prevent flickering on Qt WebEngine + Windows.
            {#if liteModeReason === 'auto'}
                <span class="text-yellow-400"> Auto-enabled (Anki + Windows detected)</span>
            {:else if liteModeReason === 'debug-override'}
                <span class="text-primary"> Debug override active</span>
            {/if}
        </div>
    </div>

    <!-- Main Interface Controls -->
    {#each Object.entries(mainInterfaceConfig) as [sectionKey, section]}
        <div class="control-section">
            <h5 class="text-xs font-semibold mb-2 opacity-80">{section.title}</h5>
            <div class="slider-grid">
                {#each section.controls as control}
                    <div class="slider-control">
                        <label class="slider-label">
                            {control.label}: {formatValue(styleControls[control.key], control)}{control.suffix || ''}
                        </label>
                        <div class="slider-row">
                            <input
                                type="range"
                                min={control.min}
                                max={control.max}
                                step={control.step || 1}
                                bind:value={styleControls[control.key]}
                                on:input={() => handleStyleChange(control.key, styleControls[control.key])}
                                class="slider"
                            />
                            <button
                                class="reset-button"
                                on:click={() => resetProperty(control.key)}
                                title="Reset to default"
                            >
                                ↺
                            </button>
                        </div>
                    </div>
                {/each}
            </div>
        </div>
    {/each}
    
    <!-- Background Gradient Stops -->
    <div class="control-section">
        <h5 class="text-xs font-semibold mb-2 opacity-80">Background Gradient</h5>
        <div class="slider-grid">
            {#each mainInterfaceConfig.backgroundGradient.controls as control}
                <div class="slider-control">
                    <label class="slider-label">
                        {control.label}: {styleControls[control.key]}{control.suffix || ''}
                    </label>
                    <div class="slider-row">
                        <input
                            type="range"
                            min={control.min}
                            max={control.max}
                            bind:value={styleControls[control.key]}
                            on:input={() => handleStyleChange(control.key, styleControls[control.key])}
                            class="slider"
                        />
                        <button
                            class="reset-button"
                            on:click={() => resetProperty(control.key)}
                            title="Reset to default"
                        >
                            ↺
                        </button>
                    </div>
                </div>
            {/each}
        </div>
        
        {#each gradientStops as stopNum}
            <div class="mb-2">
                <div class="text-xs font-medium mb-1 opacity-70">Gradient Stop {stopNum} ({(stopNum - 1) * 25}%)</div>
                <div class="slider-grid">
                    {#each ['Hue', 'Sat', 'Light', 'Alpha'] as prop, i}
                        {@const key = `bgGradientStop${stopNum}${prop}`}
                        {@const config = {
                            Hue: { min: 0, max: 360 },
                            Sat: { min: 0, max: 100, suffix: '%' },
                            Light: { min: 0, max: 100, suffix: '%' },
                            Alpha: { min: 0, max: 1, step: 0.01, decimals: 2 }
                        }[prop]}
                        <div class="slider-control">
                            <label class="slider-label">
                                {prop}: {formatValue(styleControls[key], config)}{config.suffix || ''}
                            </label>
                            <div class="slider-row">
                                <input
                                    type="range"
                                    min={config.min}
                                    max={config.max}
                                    step={config.step || 1}
                                    bind:value={styleControls[key]}
                                    on:input={() => handleStyleChange(key, styleControls[key])}
                                    class="slider"
                                />
                                <button
                                    class="reset-button"
                                    on:click={() => resetProperty(key)}
                                    title="Reset to default"
                                >
                                    ↺
                                </button>
                            </div>
                        </div>
                    {/each}
                </div>
            </div>
        {/each}
    </div>

{:else if activeStyleSubTab === 'welcome'}
    <!-- Welcome Popup Controls -->
    {#each Object.entries(welcomeConfig) as [sectionKey, section]}
        <div class="control-section">
            <h5 class="text-xs font-semibold mb-2 opacity-80">{section.title}</h5>
            <div class="slider-grid">
                {#each section.controls as control}
                    <div class="slider-control">
                        <label class="slider-label">
                            {control.label}: {formatValue(styleControls[control.key], control)}{control.suffix || ''}
                        </label>
                        <div class="slider-row">
                            <input
                                type="range"
                                min={control.min}
                                max={control.max}
                                step={control.step || 1}
                                bind:value={styleControls[control.key]}
                                on:input={() => handleStyleChange(control.key, styleControls[control.key])}
                                class="slider"
                            />
                            <button
                                class="reset-button"
                                on:click={() => resetProperty(control.key)}
                                title="Reset to default"
                            >
                                ↺
                            </button>
                        </div>
                    </div>
                {/each}
            </div>
        </div>
    {/each}

{:else if activeStyleSubTab === 'progress'}
    <!-- Progress Wave Controls -->
    <div class="control-section">
        <h5 class="text-xs font-semibold mb-2 opacity-80">Current State</h5>
        <div class="text-xs text-gray-400 mb-3">
            <span class="text-white">
                {currentProgressState.split('_').map(s => s.charAt(0).toUpperCase() + s.slice(1)).join(' ')}
            </span>
            <span class="ml-2 opacity-60">(derived from processing status)</span>
        </div>
    </div>
    
    <!-- Wave Colors for Current State -->
    <div class="control-section">
        <h5 class="text-xs font-semibold mb-2 opacity-80">
            {#if currentProgressState === 'error_task'}
                Error Task - Wave Colors
            {:else if currentProgressState === 'error_all'}
                Error All - Wave Colors
            {:else if currentProgressState === 'complete'}
                Complete State - Wave Colors (uses completion color)
            {:else if currentProgressState === 'user_cancel'}
                User Cancel - Wave Colors (uses gray theme)
            {:else}
                Normal - Wave Colors
            {/if}
        </h5>
        
        {#if currentProgressState === 'complete'}
            <div class="text-xs text-gray-400 mb-3">
                The complete state uses the completion color (green) defined in the main theme.
                Wave colors are automatically derived from the completion hue/saturation/lightness.
            </div>
        {:else if currentProgressState === 'user_cancel'}
            <div class="text-xs text-gray-400 mb-3">
                The user cancel state uses a gray theme with no wave animation.
            </div>
        {:else}
            {@const statePrefix = currentProgressState === 'error_task' ? 'errorTask' : 
                                 currentProgressState === 'error_all' ? 'errorAll' : ''}
            {#key currentProgressState}
            {#each waveNumbers as waveNum}
                <div class="mb-2">
                    <div class="text-xs font-medium mb-1 opacity-70">Wave {waveNum}</div>
                    <div class="slider-grid">
                        {#each ['Hue', 'Saturation', 'Lightness', 'Alpha'] as prop}
                            {@const key = statePrefix ? `${statePrefix}Wave${waveNum}${prop}` : `wave${waveNum}${prop}`}
                            {@const config = {
                                Hue: { min: 0, max: 360 },
                                Saturation: { min: 0, max: 100, suffix: '%', label: 'Sat' },
                                Lightness: { min: 0, max: 100, suffix: '%', label: 'Light' },
                                Alpha: { min: 0, max: 1, step: 0.01, decimals: 2 }
                            }[prop]}
                            <div class="slider-control">
                                <label class="slider-label">
                                    {config.label || prop}: {formatValue(progressWaveControls[key] ?? 0, config)}{config.suffix || ''}
                                </label>
                                <div class="slider-row">
                                    <input
                                        type="range"
                                        min={config.min}
                                        max={config.max}
                                        step={config.step || 1}
                                        bind:value={progressWaveControls[key]}
                                        on:input={() => handleProgressWaveChange(key, progressWaveControls[key])}
                                        class="slider"
                                    />
                                    <button
                                        class="reset-button"
                                        on:click={() => resetProgressWaveProperty(key)}
                                        title="Reset to default"
                                    >
                                        ↺
                                    </button>
                                </div>
                            </div>
                        {/each}
                    </div>
                </div>
            {/each}
            {/key}
        {/if}
    </div>
    
    <!-- Other Progress Controls -->
    {#each Object.entries(progressConfig) as [sectionKey, section]}
        <div class="control-section">
            <h5 class="text-xs font-semibold mb-2 opacity-80">{section.title}</h5>
            <div class="slider-grid">
                {#each section.controls as control}
                    <div class="slider-control">
                        <label class="slider-label">
                            {control.label}: {formatValue(progressWaveControls[control.key], control)}{control.suffix || ''}
                        </label>
                        <div class="slider-row">
                            <input
                                type="range"
                                min={control.min}
                                max={control.max}
                                step={control.step || 1}
                                bind:value={progressWaveControls[control.key]}
                                on:input={() => handleProgressWaveChange(control.key, progressWaveControls[control.key])}
                                class="slider"
                            />
                            <button
                                class="reset-button"
                                on:click={() => resetProgressWaveProperty(control.key)}
                                title="Reset to default"
                            >
                                ↺
                            </button>
                        </div>
                    </div>
                {/each}
            </div>
        </div>
    {/each}

{:else if activeStyleSubTab === 'coffee'}
    <!-- Coffee Mug Controls -->
    {#each Object.entries(coffeeConfig) as [sectionKey, section]}
        <div class="control-section">
            <h5 class="text-xs font-semibold mb-2 opacity-80">{section.title}</h5>
            <div class="slider-grid">
                {#each section.controls as control}
                    <div class="slider-control">
                        <label class="slider-label">
                            {control.label}: {formatValue(styleControls[control.key], control)}{control.suffix || ''}
                        </label>
                        <div class="slider-row">
                            <input
                                type="range"
                                min={control.min}
                                max={control.max}
                                step={control.step || 1}
                                bind:value={styleControls[control.key]}
                                on:input={() => handleStyleChange(control.key, styleControls[control.key])}
                                class="slider"
                            />
                            <button
                                class="reset-button"
                                on:click={() => resetProperty(control.key)}
                                title="Reset to default"
                            >
                                ↺
                            </button>
                        </div>
                    </div>
                {/each}
            </div>
        </div>
    {/each}
{/if}

<!-- Reset Controls -->
<div class="control-section">
    <div class="flex gap-2">
        <button
            class="control-button reset-button"
            on:click={resetStyleControls}
        >
            Reset All
        </button>
        <button
            class="control-button"
            on:click={copyExportedValues}
        >
            Copy Values
        </button>
    </div>
</div>

<style>
    h4 {
        margin: 0 0 12px 0;
        font-size: 13px;
        opacity: 0.9;
    }
    
    /* Control sections layout */
    .control-section {
        margin-bottom: 16px;
        padding-bottom: 12px;
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    }
    
    .control-section:last-child {
        border-bottom: none;
        margin-bottom: 0;
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
    
    .control-button-group {
        display: flex;
        gap: 8px;
        justify-content: center;
    }
    
    
    /* Minimal utility classes - removed 90% of redundant ones */
    .text-xs { font-size: 0.75rem; }
    .text-white { color: white; }
    .text-white\/60 { color: rgba(255, 255, 255, 0.6); }
    .text-gray-400 { color: rgba(255, 255, 255, 0.5); }
    .font-semibold { font-weight: 600; }
    .font-medium { font-weight: 500; }
    .font-mono { font-family: monospace; }
    .mb-1 { margin-bottom: 0.25rem; }
    .mb-2 { margin-bottom: 0.5rem; }
    .mb-3 { margin-bottom: 0.75rem; }
    .mb-4 { margin-bottom: 1rem; }
    .mt-2 { margin-top: 0.5rem; }
    .mt-3 { margin-top: 0.75rem; }
    .ml-2 { margin-left: 0.5rem; }
    .px-1 { padding-left: 0.25rem; padding-right: 0.25rem; }
    .px-3 { padding-left: 0.75rem; padding-right: 0.75rem; }
    .py-1 { padding-left: 0.25rem; padding-right: 0.25rem; }
    .py-2 { padding-top: 0.5rem; padding-bottom: 0.5rem; }
    .opacity-60 { opacity: 0.6; }
    .opacity-70 { opacity: 0.7; }
    .opacity-80 { opacity: 0.8; }
    .flex { display: flex; }
    .flex-wrap { flex-wrap: wrap; }
    .flex-shrink-0 { flex-shrink: 0; }
    .flex-1 { flex: 1; }
    .items-center { align-items: center; }
    .gap-2 { gap: 0.5rem; }
    .h-6 { height: 1.5rem; }
    .w-20 { width: 5rem; }
    .min-w-0 { min-width: 0; }
    .rounded { border-radius: 0.25rem; }
    .border { border-width: 1px; }
    .border-b { border-bottom-width: 1px; }
    .border-b-2 { border-bottom-width: 2px; }
    .border-primary { border-color: var(--primary-color, #9f6ef7); }
    .border-white\/10 { border-color: rgba(255, 255, 255, 0.1); }
    .border-white\/20 { border-color: rgba(255, 255, 255, 0.2); }
    .border-white\/30 { border-color: rgba(255, 255, 255, 0.3); }
    .bg-white\/10 { background-color: rgba(255, 255, 255, 0.1); }

    /* Toggle switch for reduced effects mode */
    .toggle-button {
        position: relative;
        width: 44px;
        height: 24px;
        background: rgba(255, 255, 255, 0.15);
        border: 1px solid rgba(255, 255, 255, 0.2);
        border-radius: 12px;
        cursor: pointer;
        transition: all 0.3s ease;
        padding: 0;
    }

    .toggle-button:hover {
        background: rgba(255, 255, 255, 0.2);
        border-color: rgba(255, 255, 255, 0.3);
    }

    .toggle-button.active {
        background: var(--primary-color, #9f6ef7);
        border-color: var(--primary-color, #9f6ef7);
    }

    .toggle-slider {
        position: absolute;
        top: 2px;
        left: 2px;
        width: 18px;
        height: 18px;
        background: white;
        border-radius: 50%;
        transition: transform 0.3s ease;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
    }

    .toggle-button.active .toggle-slider {
        transform: translateX(20px);
    }

    .text-primary { color: var(--primary-color, #9f6ef7); }
    .text-yellow-400 { color: #facc15; }
    .gap-3 { gap: 0.75rem; }
</style>