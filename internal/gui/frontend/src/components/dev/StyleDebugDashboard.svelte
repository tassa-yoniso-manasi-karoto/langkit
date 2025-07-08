<script lang="ts">
    import { defaultValues, defaultProgressWaveValues } from '../../lib/dev/styleControlsDefaults';
    
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
    export let resetProgressWaveControls: () => void;
    export let onStyleControlChange: (property: string, value: number) => void;
    export let onProgressWaveControlChange: (property: string, value: number) => void;
    export let onTargetColorChange: (value: string) => void;
    export let onStyleSubTabChange: (tab: string) => void;
    export let onProgressStateChange: (state: typeof currentProgressState) => void;
    
    // Helper function to convert hex to HSL
    function hexToHSL(hex: string) {
        let r = 0, g = 0, b = 0;
        if (hex.length === 4) {
            r = parseInt(hex[1] + hex[1], 16);
            g = parseInt(hex[2] + hex[2], 16);
            b = parseInt(hex[3] + hex[3], 16);
        } else if (hex.length === 7) {
            r = parseInt(hex[1] + hex[2], 16);
            g = parseInt(hex[3] + hex[4], 16);
            b = parseInt(hex[5] + hex[6], 16);
        }
        
        r /= 255;
        g /= 255;
        b /= 255;
        
        const max = Math.max(r, g, b);
        const min = Math.min(r, g, b);
        let h = 0, s = 0, l = (max + min) / 2;
        
        if (max !== min) {
            const d = max - min;
            s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
            switch (max) {
                case r: h = ((g - b) / d + (g < b ? 6 : 0)) / 6; break;
                case g: h = ((b - r) / d + 2) / 6; break;
                case b: h = ((r - g) / d + 4) / 6; break;
            }
        }
        
        return {
            h: Math.round(h * 360),
            s: Math.round(s * 100),
            l: Math.round(l * 100)
        };
    }
    
    function copyExportedValues() {
        const exported = JSON.stringify(styleControls, null, 2);
        navigator.clipboard.writeText(exported);
    }
    
    function copyProgressWaveValues() {
        const exported = JSON.stringify(progressWaveControls, null, 2);
        navigator.clipboard.writeText(exported);
    }
    
    function simulateProgressState(state: typeof currentProgressState) {
        currentProgressState = state;
        onProgressStateChange(state);
        
        // Dispatch event to trigger animation in ProgressManager
        document.dispatchEvent(new CustomEvent('progress-state-demo', { 
            detail: { state } 
        }));
    }
    
    function applyTargetColor() {
        const hsl = hexToHSL(targetColorHex);
        styleControls.bgHue = hsl.h;
        styleControls.bgSaturation = hsl.s;
        styleControls.bgLightness = hsl.l;
        applyStyleControls();
    }
</script>

<h4>Style Controls</h4>
<div class="mb-3">
    <div class="flex items-center gap-2 text-xs text-gray-400">
        <span class="flex-shrink-0">Target color</span>
        <input
            type="text"
            bind:value={targetColorHex}
            on:input={() => onTargetColorChange(targetColorHex)}
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
        on:click={() => onStyleSubTabChange('main')}
    >
        Main Interface
    </button>
    <button
        class="px-3 py-2 text-xs {activeStyleSubTab === 'welcome' ? 'text-white border-b-2 border-primary' : 'text-white/60'}"
        on:click={() => onStyleSubTabChange('welcome')}
    >
        Welcome Popup
    </button>
    <button
        class="px-3 py-2 text-xs {activeStyleSubTab === 'progress' ? 'text-white border-b-2 border-primary' : 'text-white/60'}"
        on:click={() => onStyleSubTabChange('progress')}
    >
        Progress
    </button>
    <button
        class="px-3 py-2 text-xs {activeStyleSubTab === 'coffee' ? 'text-white border-b-2 border-primary' : 'text-white/60'}"
        on:click={() => onStyleSubTabChange('coffee')}
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
                    min="0"
                    max="360"
                    step="1"
                    bind:value={styleControls.bgHue}
                    on:input={() => { onStyleControlChange('bgHue', styleControls.bgHue); applyStyleControls(); }}
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
                    on:input={() => { onStyleControlChange('bgSaturation', styleControls.bgSaturation); applyStyleControls(); }}
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
                    max="20"
                    step="0.05"
                    bind:value={styleControls.bgLightness}
                    on:input={() => { onStyleControlChange('bgLightness', styleControls.bgLightness); applyStyleControls(); }}
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
        <div class="slider-control">
            <label class="slider-label">Opacity: {styleControls.bgOpacity.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={styleControls.bgOpacity}
                    on:input={() => { onStyleControlChange('bgOpacity', styleControls.bgOpacity); applyStyleControls(); }}
                    class="slider"
                />
                <button
                    class="reset-button"
                    on:click={() => resetProperty('bgOpacity')}
                    title="Reset to default"
                >
                    ↺
                </button>
            </div>
        </div>
    </div>
    <div class="mt-2">
        <button
            class="control-button text-xs px-3 py-1"
            on:click={applyTargetColor}
        >
            Apply Target Color to Background
        </button>
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
                    on:input={() => { onStyleControlChange('featureCardSaturation', styleControls.featureCardSaturation); applyStyleControls(); }}
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
                    on:input={() => { onStyleControlChange('featureCardLightness', styleControls.featureCardLightness); applyStyleControls(); }}
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
            <label class="slider-label">Opacity: {styleControls.featureCardOpacity.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={styleControls.featureCardOpacity}
                    on:input={() => { onStyleControlChange('featureCardOpacity', styleControls.featureCardOpacity); applyStyleControls(); }}
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
    </div>
    <div class="slider-grid mt-2">
        <div class="slider-control">
            <label class="slider-label">Gradient Start: {styleControls.featureCardGradientStartOpacity.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={styleControls.featureCardGradientStartOpacity}
                    on:input={() => { onStyleControlChange('featureCardGradientStartOpacity', styleControls.featureCardGradientStartOpacity); applyStyleControls(); }}
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
                    on:input={() => { onStyleControlChange('featureCardGradientEndOpacity', styleControls.featureCardGradientEndOpacity); applyStyleControls(); }}
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
                    on:input={() => { onStyleControlChange('mediaInputSaturation', styleControls.mediaInputSaturation); applyStyleControls(); }}
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
                    on:input={() => { onStyleControlChange('mediaInputLightness', styleControls.mediaInputLightness); applyStyleControls(); }}
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
                    on:input={() => { onStyleControlChange('mediaInputOpacity', styleControls.mediaInputOpacity); applyStyleControls(); }}
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

<!-- Glow Effect Controls -->
<div class="control-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">Glow Effect</h5>
    <div class="slider-grid">
        <div class="slider-control">
            <label class="slider-label">Opacity: {styleControls.glowOpacity.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={styleControls.glowOpacity}
                    on:input={() => { onStyleControlChange('glowOpacity', styleControls.glowOpacity); applyStyleControls(); }}
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

<!-- Background Gradient Controls -->
<div class="control-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">Background Gradient</h5>
    <div class="slider-grid">
        <div class="slider-control">
            <label class="slider-label">Position X: {styleControls.bgGradientPosX}%</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="100"
                    step="1"
                    bind:value={styleControls.bgGradientPosX}
                    on:input={() => { onStyleControlChange('bgGradientPosX', styleControls.bgGradientPosX); applyStyleControls(); }}
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
                    min="0"
                    max="100"
                    step="1"
                    bind:value={styleControls.bgGradientPosY}
                    on:input={() => { onStyleControlChange('bgGradientPosY', styleControls.bgGradientPosY); applyStyleControls(); }}
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
    {#each [1, 2, 3, 4] as stopNum}
    <div class="mb-2">
        <div class="text-xs font-medium mb-1 opacity-70">Gradient Stop {stopNum} ({(stopNum - 1) * 25}%)</div>
        <div class="slider-grid">
            <div class="slider-control">
                <label class="slider-label">Hue: {styleControls[`bgGradientStop${stopNum}Hue`]}</label>
                <div class="slider-row">
                    <input
                        type="range"
                        min="0"
                        max="360"
                        bind:value={styleControls[`bgGradientStop${stopNum}Hue`]}
                        on:input={() => { onStyleControlChange(`bgGradientStop${stopNum}Hue`, styleControls[`bgGradientStop${stopNum}Hue`]); applyStyleControls(); }}
                        class="slider"
                    />
                    <button
                        class="reset-button"
                        on:click={() => resetProperty(`bgGradientStop${stopNum}Hue`)}
                        title="Reset to default"
                    >
                        ↺
                    </button>
                </div>
            </div>
            <div class="slider-control">
                <label class="slider-label">Sat: {styleControls[`bgGradientStop${stopNum}Sat`]}%</label>
                <div class="slider-row">
                    <input
                        type="range"
                        min="0"
                        max="100"
                        bind:value={styleControls[`bgGradientStop${stopNum}Sat`]}
                        on:input={() => { onStyleControlChange(`bgGradientStop${stopNum}Sat`, styleControls[`bgGradientStop${stopNum}Sat`]); applyStyleControls(); }}
                        class="slider"
                    />
                    <button
                        class="reset-button"
                        on:click={() => resetProperty(`bgGradientStop${stopNum}Sat`)}
                        title="Reset to default"
                    >
                        ↺
                    </button>
                </div>
            </div>
            <div class="slider-control">
                <label class="slider-label">Light: {styleControls[`bgGradientStop${stopNum}Light`]}%</label>
                <div class="slider-row">
                    <input
                        type="range"
                        min="0"
                        max="100"
                        bind:value={styleControls[`bgGradientStop${stopNum}Light`]}
                        on:input={() => { onStyleControlChange(`bgGradientStop${stopNum}Light`, styleControls[`bgGradientStop${stopNum}Light`]); applyStyleControls(); }}
                        class="slider"
                    />
                    <button
                        class="reset-button"
                        on:click={() => resetProperty(`bgGradientStop${stopNum}Light`)}
                        title="Reset to default"
                    >
                        ↺
                    </button>
                </div>
            </div>
            <div class="slider-control">
                <label class="slider-label">Alpha: {styleControls[`bgGradientStop${stopNum}Alpha`].toFixed(2)}</label>
                <div class="slider-row">
                    <input
                        type="range"
                        min="0"
                        max="1"
                        step="0.01"
                        bind:value={styleControls[`bgGradientStop${stopNum}Alpha`]}
                        on:input={() => { onStyleControlChange(`bgGradientStop${stopNum}Alpha`, styleControls[`bgGradientStop${stopNum}Alpha`]); applyStyleControls(); }}
                        class="slider"
                    />
                    <button
                        class="reset-button"
                        on:click={() => resetProperty(`bgGradientStop${stopNum}Alpha`)}
                        title="Reset to default"
                    >
                        ↺
                    </button>
                </div>
            </div>
        </div>
    </div>
    {/each}
</div>

<!-- Glow Position & Properties -->
<div class="control-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">Glow Position & Properties</h5>
    <div class="slider-grid">
        <div class="slider-control">
            <label class="slider-label">Position X: {styleControls.glowPositionX}vw</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="-50"
                    max="150"
                    step="1"
                    bind:value={styleControls.glowPositionX}
                    on:input={() => { onStyleControlChange('glowPositionX', styleControls.glowPositionX); applyStyleControls(); }}
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
                    min="-50"
                    max="150"
                    step="1"
                    bind:value={styleControls.glowPositionY}
                    on:input={() => { onStyleControlChange('glowPositionY', styleControls.glowPositionY); applyStyleControls(); }}
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
                    min="10"
                    max="100"
                    step="1"
                    bind:value={styleControls.glowSize}
                    on:input={() => { onStyleControlChange('glowSize', styleControls.glowSize); applyStyleControls(); }}
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
                    min="20"
                    max="200"
                    step="1"
                    bind:value={styleControls.glowBlur}
                    on:input={() => { onStyleControlChange('glowBlur', styleControls.glowBlur); applyStyleControls(); }}
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
            <label class="slider-label">Animation Scale: {styleControls.glowAnimationScale.toFixed(1)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="1"
                    max="3"
                    step="0.1"
                    bind:value={styleControls.glowAnimationScale}
                    on:input={() => { onStyleControlChange('glowAnimationScale', styleControls.glowAnimationScale); applyStyleControls(); }}
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
            <label class="slider-label">Animation Speed: {styleControls.glowAnimationSpeed}s</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="1"
                    max="30"
                    step="1"
                    bind:value={styleControls.glowAnimationSpeed}
                    on:input={() => { onStyleControlChange('glowAnimationSpeed', styleControls.glowAnimationSpeed); applyStyleControls(); }}
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

{:else if activeStyleSubTab === 'welcome'}
<!-- Welcome Overlay & Panel -->
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
                    step="0.01"
                    bind:value={styleControls.welcomeOverlayOpacity}
                    on:input={() => { onStyleControlChange('welcomeOverlayOpacity', styleControls.welcomeOverlayOpacity); applyStyleControls(); }}
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
            <label class="slider-label">Panel BG: {styleControls.welcomePanelBgOpacity.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={styleControls.welcomePanelBgOpacity}
                    on:input={() => { onStyleControlChange('welcomePanelBgOpacity', styleControls.welcomePanelBgOpacity); applyStyleControls(); }}
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
                    step="1"
                    bind:value={styleControls.welcomePanelBlur}
                    on:input={() => { onStyleControlChange('welcomePanelBlur', styleControls.welcomePanelBlur); applyStyleControls(); }}
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
                    max="1"
                    step="0.01"
                    bind:value={styleControls.welcomeBorderOpacity}
                    on:input={() => { onStyleControlChange('welcomeBorderOpacity', styleControls.welcomeBorderOpacity); applyStyleControls(); }}
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

<!-- Welcome Cards & Buttons -->
<div class="control-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">Cards & Buttons</h5>
    <div class="slider-grid">
        <div class="slider-control">
            <label class="slider-label">Card BG: {styleControls.welcomeCardBgOpacity.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={styleControls.welcomeCardBgOpacity}
                    on:input={() => { onStyleControlChange('welcomeCardBgOpacity', styleControls.welcomeCardBgOpacity); applyStyleControls(); }}
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
            <label class="slider-label">Card Hover: {styleControls.welcomeCardHoverOpacity.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={styleControls.welcomeCardHoverOpacity}
                    on:input={() => { onStyleControlChange('welcomeCardHoverOpacity', styleControls.welcomeCardHoverOpacity); applyStyleControls(); }}
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
            <label class="slider-label">Button BG: {styleControls.welcomeButtonBgOpacity.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={styleControls.welcomeButtonBgOpacity}
                    on:input={() => { onStyleControlChange('welcomeButtonBgOpacity', styleControls.welcomeButtonBgOpacity); applyStyleControls(); }}
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
                    step="0.01"
                    bind:value={styleControls.welcomeButtonBorderOpacity}
                    on:input={() => { onStyleControlChange('welcomeButtonBorderOpacity', styleControls.welcomeButtonBorderOpacity); applyStyleControls(); }}
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

<!-- Welcome UI Elements -->
<div class="control-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">UI Elements</h5>
    <div class="slider-grid">
        <div class="slider-control">
            <label class="slider-label">Progress Dots: {styleControls.welcomeProgressDotOpacity.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={styleControls.welcomeProgressDotOpacity}
                    on:input={() => { onStyleControlChange('welcomeProgressDotOpacity', styleControls.welcomeProgressDotOpacity); applyStyleControls(); }}
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
    </div>
</div>

<!-- Welcome Text Opacity -->
<div class="control-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">Text Opacity</h5>
    <div class="slider-grid">
        <div class="slider-control">
            <label class="slider-label">Primary Text: {styleControls.welcomeTextPrimaryOpacity.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={styleControls.welcomeTextPrimaryOpacity}
                    on:input={() => { onStyleControlChange('welcomeTextPrimaryOpacity', styleControls.welcomeTextPrimaryOpacity); applyStyleControls(); }}
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
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={styleControls.welcomeTextSecondaryOpacity}
                    on:input={() => { onStyleControlChange('welcomeTextSecondaryOpacity', styleControls.welcomeTextSecondaryOpacity); applyStyleControls(); }}
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
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={styleControls.welcomeTextTertiaryOpacity}
                    on:input={() => { onStyleControlChange('welcomeTextTertiaryOpacity', styleControls.welcomeTextTertiaryOpacity); applyStyleControls(); }}
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
<!-- Progress Wave Controls -->
<div class="control-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">State Simulation</h5>
    <div class="flex flex-wrap gap-2 mb-3">
        <button 
            class="state-button {currentProgressState === 'normal' ? 'active' : ''}"
            on:click={() => simulateProgressState('normal')}
        >
            Normal
        </button>
        <button 
            class="state-button {currentProgressState === 'error_task' ? 'active' : ''}"
            on:click={() => simulateProgressState('error_task')}
        >
            Error Task
        </button>
        <button 
            class="state-button {currentProgressState === 'error_all' ? 'active' : ''}"
            on:click={() => simulateProgressState('error_all')}
        >
            Error All
        </button>
        <button 
            class="state-button {currentProgressState === 'user_cancel' ? 'active' : ''}"
            on:click={() => simulateProgressState('user_cancel')}
        >
            User Cancel
        </button>
        <button 
            class="state-button {currentProgressState === 'complete' ? 'active' : ''}"
            on:click={() => simulateProgressState('complete')}
        >
            Complete
        </button>
    </div>
</div>

<!-- Normal Wave Colors -->
<div class="control-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">Normal - Wave Colors</h5>
    {#each [1, 2, 3, 4] as waveNum}
    <div class="mb-2">
        <div class="text-xs font-medium mb-1 opacity-70">Wave {waveNum}</div>
        <div class="slider-grid">
            <div class="slider-control">
                <label class="slider-label">Hue: {progressWaveControls[`wave${waveNum}Hue`]}</label>
                <div class="slider-row">
                    <input
                        type="range"
                        min="0"
                        max="360"
                        bind:value={progressWaveControls[`wave${waveNum}Hue`]}
                        on:input={() => { onProgressWaveControlChange(`wave${waveNum}Hue`, progressWaveControls[`wave${waveNum}Hue`]); applyProgressWaveControls(); }}
                        class="slider"
                    />
                    <button
                        class="reset-button"
                        on:click={() => resetProgressWaveProperty(`wave${waveNum}Hue`)}
                        title="Reset to default"
                    >
                        ↺
                    </button>
                </div>
            </div>
            <div class="slider-control">
                <label class="slider-label">Sat: {progressWaveControls[`wave${waveNum}Saturation`]}%</label>
                <div class="slider-row">
                    <input
                        type="range"
                        min="0"
                        max="100"
                        bind:value={progressWaveControls[`wave${waveNum}Saturation`]}
                        on:input={() => { onProgressWaveControlChange(`wave${waveNum}Saturation`, progressWaveControls[`wave${waveNum}Saturation`]); applyProgressWaveControls(); }}
                        class="slider"
                    />
                    <button
                        class="reset-button"
                        on:click={() => resetProgressWaveProperty(`wave${waveNum}Saturation`)}
                        title="Reset to default"
                    >
                        ↺
                    </button>
                </div>
            </div>
            <div class="slider-control">
                <label class="slider-label">Light: {progressWaveControls[`wave${waveNum}Lightness`]}%</label>
                <div class="slider-row">
                    <input
                        type="range"
                        min="0"
                        max="100"
                        bind:value={progressWaveControls[`wave${waveNum}Lightness`]}
                        on:input={() => { onProgressWaveControlChange(`wave${waveNum}Lightness`, progressWaveControls[`wave${waveNum}Lightness`]); applyProgressWaveControls(); }}
                        class="slider"
                    />
                    <button
                        class="reset-button"
                        on:click={() => resetProgressWaveProperty(`wave${waveNum}Lightness`)}
                        title="Reset to default"
                    >
                        ↺
                    </button>
                </div>
            </div>
            <div class="slider-control">
                <label class="slider-label">Alpha: {progressWaveControls[`wave${waveNum}Alpha`].toFixed(2)}</label>
                <div class="slider-row">
                    <input
                        type="range"
                        min="0"
                        max="1"
                        step="0.01"
                        bind:value={progressWaveControls[`wave${waveNum}Alpha`]}
                        on:input={() => { onProgressWaveControlChange(`wave${waveNum}Alpha`, progressWaveControls[`wave${waveNum}Alpha`]); applyProgressWaveControls(); }}
                        class="slider"
                    />
                    <button
                        class="reset-button"
                        on:click={() => resetProgressWaveProperty(`wave${waveNum}Alpha`)}
                        title="Reset to default"
                    >
                        ↺
                    </button>
                </div>
            </div>
        </div>
    </div>
    {/each}
</div>

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
                    on:input={() => { onProgressWaveControlChange('waveIntensity', progressWaveControls.waveIntensity); applyProgressWaveControls(); }}
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
                    on:input={() => { onProgressWaveControlChange('waveFrequency', progressWaveControls.waveFrequency); applyProgressWaveControls(); }}
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
                    on:input={() => { onProgressWaveControlChange('animationSpeed', progressWaveControls.animationSpeed); applyProgressWaveControls(); }}
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
                    on:input={() => { onProgressWaveControlChange('blurAmount', progressWaveControls.blurAmount); applyProgressWaveControls(); }}
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

<!-- Progress Bar Specific -->
<div class="control-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">Progress Bar</h5>
    <div class="slider-grid">
        <div class="slider-control">
            <label class="slider-label">Edge Glow: {progressWaveControls.progressEdgeGlow.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={progressWaveControls.progressEdgeGlow}
                    on:input={() => { onProgressWaveControlChange('progressEdgeGlow', progressWaveControls.progressEdgeGlow); applyProgressWaveControls(); }}
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
                    step="0.01"
                    bind:value={progressWaveControls.progressSweepOpacity}
                    on:input={() => { onProgressWaveControlChange('progressSweepOpacity', progressWaveControls.progressSweepOpacity); applyProgressWaveControls(); }}
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
                    max="10"
                    step="0.1"
                    bind:value={progressWaveControls.progressSweepDuration}
                    on:input={() => { onProgressWaveControlChange('progressSweepDuration', progressWaveControls.progressSweepDuration); applyProgressWaveControls(); }}
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

<!-- Additional Aesthetics -->
<div class="control-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">Additional Aesthetics</h5>
    <div class="slider-grid">
        <div class="slider-control">
            <label class="slider-label">BG Darkness: {progressWaveControls.progressBgDarkness}%</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="-50"
                    max="50"
                    bind:value={progressWaveControls.progressBgDarkness}
                    on:input={() => { onProgressWaveControlChange('progressBgDarkness', progressWaveControls.progressBgDarkness); applyProgressWaveControls(); }}
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
            <label class="slider-label">Wave Opacity: {progressWaveControls.waveOverallOpacity.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.01"
                    bind:value={progressWaveControls.waveOverallOpacity}
                    on:input={() => { onProgressWaveControlChange('waveOverallOpacity', progressWaveControls.waveOverallOpacity); applyProgressWaveControls(); }}
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
            <label class="slider-label">Direction: {progressWaveControls.waveDirection}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="-1"
                    max="1"
                    step="2"
                    bind:value={progressWaveControls.waveDirection}
                    on:input={() => { onProgressWaveControlChange('waveDirection', progressWaveControls.waveDirection); applyProgressWaveControls(); }}
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
            <label class="slider-label">Stacking: {progressWaveControls.waveStackingMultiplier.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0.5"
                    max="1.5"
                    step="0.01"
                    bind:value={progressWaveControls.waveStackingMultiplier}
                    on:input={() => { onProgressWaveControlChange('waveStackingMultiplier', progressWaveControls.waveStackingMultiplier); applyProgressWaveControls(); }}
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
            <label class="slider-label">Offset: {progressWaveControls.waveOffsetMultiplier.toFixed(2)}</label>
            <div class="slider-row">
                <input
                    type="range"
                    min="0.5"
                    max="2"
                    step="0.01"
                    bind:value={progressWaveControls.waveOffsetMultiplier}
                    on:input={() => { onProgressWaveControlChange('waveOffsetMultiplier', progressWaveControls.waveOffsetMultiplier); applyProgressWaveControls(); }}
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

<div class="control-button-group mt-3">
    <button class="control-button" on:click={copyProgressWaveValues}>
        Copy Wave Values
    </button>
    <button class="control-button reset-button" on:click={resetProgressWaveControls}>
        Reset Progress Waves
    </button>
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
                    on:input={() => { onStyleControlChange('coffeeMugHue', styleControls.coffeeMugHue); applyStyleControls(); }}
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
                    on:input={() => { onStyleControlChange('coffeeMugSaturation', styleControls.coffeeMugSaturation); applyStyleControls(); }}
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
                    on:input={() => { onStyleControlChange('coffeeMugLightness', styleControls.coffeeMugLightness); applyStyleControls(); }}
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
                    on:input={() => { onStyleControlChange('coffeeMugOpacity', styleControls.coffeeMugOpacity); applyStyleControls(); }}
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
    
    /* State buttons for progress simulation */
    .state-button {
        padding: 4px 8px;
        background: rgba(255, 255, 255, 0.1);
        border: 1px solid rgba(255, 255, 255, 0.2);
        border-radius: 4px;
        color: rgba(255, 255, 255, 0.7);
        font-size: 11px;
        cursor: pointer;
        transition: all 0.2s;
    }
    
    .state-button:hover {
        background: rgba(255, 255, 255, 0.15);
        border-color: rgba(255, 255, 255, 0.3);
        color: white;
    }
    
    .state-button.active {
        background: var(--primary-color, #9f6ef7);
        border-color: var(--primary-color, #9f6ef7);
        color: white;
    }
    
    /* Utility classes */
    .text-xs {
        font-size: 0.75rem;
    }
    
    .text-white {
        color: white;
    }
    
    .text-white\/60 {
        color: rgba(255, 255, 255, 0.6);
    }
    
    .text-gray-400 {
        color: rgba(255, 255, 255, 0.5);
    }
    
    .font-semibold {
        font-weight: 600;
    }
    
    .font-medium {
        font-weight: 500;
    }
    
    .font-mono {
        font-family: monospace;
    }
    
    .mb-1 {
        margin-bottom: 0.25rem;
    }
    
    .mb-2 {
        margin-bottom: 0.5rem;
    }
    
    .mb-3 {
        margin-bottom: 0.75rem;
    }
    
    .mb-4 {
        margin-bottom: 1rem;
    }
    
    .mt-2 {
        margin-top: 0.5rem;
    }
    
    .mt-3 {
        margin-top: 0.75rem;
    }
    
    .px-1 {
        padding-left: 0.25rem;
        padding-right: 0.25rem;
    }
    
    .px-3 {
        padding-left: 0.75rem;
        padding-right: 0.75rem;
    }
    
    .py-1 {
        padding-left: 0.25rem;
        padding-right: 0.25rem;
    }
    
    .py-2 {
        padding-top: 0.5rem;
        padding-bottom: 0.5rem;
    }
    
    .opacity-70 {
        opacity: 0.7;
    }
    
    .opacity-80 {
        opacity: 0.8;
    }
    
    .flex {
        display: flex;
    }
    
    .flex-wrap {
        flex-wrap: wrap;
    }
    
    .flex-shrink-0 {
        flex-shrink: 0;
    }
    
    .flex-1 {
        flex: 1;
    }
    
    .items-center {
        align-items: center;
    }
    
    .gap-2 {
        gap: 0.5rem;
    }
    
    .h-6 {
        height: 1.5rem;
    }
    
    .w-20 {
        width: 5rem;
    }
    
    .min-w-0 {
        min-width: 0;
    }
    
    .rounded {
        border-radius: 0.25rem;
    }
    
    .border {
        border-width: 1px;
    }
    
    .border-b {
        border-bottom-width: 1px;
    }
    
    .border-b-2 {
        border-bottom-width: 2px;
    }
    
    .border-primary {
        border-color: var(--primary-color, #9f6ef7);
    }
    
    .border-white\/10 {
        border-color: rgba(255, 255, 255, 0.1);
    }
    
    .border-white\/20 {
        border-color: rgba(255, 255, 255, 0.2);
    }
    
    .border-white\/30 {
        border-color: rgba(255, 255, 255, 0.3);
    }
    
    .bg-white\/10 {
        background-color: rgba(255, 255, 255, 0.1);
    }
    
    .text-white {
        color: white;
    }
</style>