<script lang="ts">
    import { 
        forceLLMState, resetLLMState,
        forceUserActivityState, resetUserActivityState,
        forceDockerStatus, resetDockerStatus,
        forceInternetStatus, resetInternetStatus,
        forceFFmpegStatus, resetFFmpegStatus,
        forceMediaInfoStatus, resetMediaInfoStatus,
        setDebounceOverride, resetDebounceOverride, resetAllDebounceOverrides, getDebounceState
    } from '../../lib/dev/debugStateControls';
    
    // Props
    export let currentLLMState: any;
    export let currentUserActivityState: string;
    export let isForced: boolean;
    export let currentDockerStatus: any;
    export let dockerForced: boolean;
    export let currentInternetStatus: any;
    export let internetForced: boolean;
    export let currentFFmpegStatus: any;
    export let ffmpegForced: boolean;
    export let currentMediaInfoStatus: any;
    export let mediainfoForced: boolean;
    
    // Sub-tab state
    let activeSubTab = 'dependencies';
    
    // Debounce state
    let debounceState = getDebounceState();
    let windowsDebounce = debounceState.windowsOverride || debounceState.windowsDefault;
    let otherDebounce = debounceState.otherOverride || debounceState.otherDefault;
    let windowsOverridden = false;
    let otherOverridden = false;
    
    
    // Update debounce when slider changes
    function updateWindowsDebounce() {
        setDebounceOverride('windows', windowsDebounce);
        windowsOverridden = true;
    }
    
    function updateOtherDebounce() {
        setDebounceOverride('other', otherDebounce);
        otherOverridden = true;
    }
    
    // Reset functions
    function resetWindowsDebounce() {
        resetDebounceOverride('windows');
        windowsDebounce = 200;
        windowsOverridden = false;
    }
    
    function resetOtherDebounce() {
        resetDebounceOverride('other');
        otherDebounce = 10;
        otherOverridden = false;
    }
    
    function resetAllDebounce() {
        resetAllDebounceOverrides();
        windowsDebounce = 200;
        otherDebounce = 10;
        windowsOverridden = false;
        otherOverridden = false;
    }
</script>

<!-- Sub-tab navigation -->
<div class="sub-tab-navigation">
    <button 
        class="sub-tab-button {activeSubTab === 'dependencies' ? 'active' : ''}"
        on:click={() => activeSubTab = 'dependencies'}
    >
        Dependencies
    </button>
    <button 
        class="sub-tab-button {activeSubTab === 'debounce' ? 'active' : ''}"
        on:click={() => activeSubTab = 'debounce'}
    >
        Debounce
    </button>
</div>

{#if activeSubTab === 'dependencies'}

<!-- FFmpeg Status (first) -->
<div class="debug-section">
    <div class="status-row">
        <span class="text-gray-400 text-xs font-semibold">FFmpeg Status:</span>
        <span class="status-value" class:text-green-400={currentFFmpegStatus?.available}
              class:text-red-400={!currentFFmpegStatus?.available}>
            {currentFFmpegStatus?.available ? 'Available' : 'Unavailable'}
        </span>
        {#if ffmpegForced}
            <span class="text-purple-400 text-xs ml-2">(forced)</span>
        {:else if currentFFmpegStatus?.checked}
            <span class="text-green-400 text-xs ml-2">(real)</span>
        {:else}
            <span class="text-yellow-400 text-xs ml-2">(checking...)</span>
        {/if}
        {#if currentFFmpegStatus?.version}
            <span class="text-gray-500 text-xs ml-2">v{currentFFmpegStatus.version}</span>
        {/if}
    </div>
    <div class="debug-controls">
        <button class="debug-button" on:click={() => forceFFmpegStatus(true)}>
            Force Available
        </button>
        <button class="debug-button" on:click={() => forceFFmpegStatus(false)}>
            Force Unavailable
        </button>
        <button class="debug-button reset" on:click={resetFFmpegStatus}>
            Reset to Real
        </button>
    </div>
    <div class="text-xs text-gray-500 mt-2">
        Controls FFmpeg availability checks - required for all media processing
    </div>
</div>

<!-- MediaInfo Status (second) -->
<div class="debug-section">
    <div class="status-row">
        <span class="text-gray-400 text-xs font-semibold">MediaInfo Status:</span>
        <span class="status-value" class:text-green-400={currentMediaInfoStatus?.available}
              class:text-red-400={!currentMediaInfoStatus?.available}>
            {currentMediaInfoStatus?.available ? 'Available' : 'Unavailable'}
        </span>
        {#if mediainfoForced}
            <span class="text-purple-400 text-xs ml-2">(forced)</span>
        {:else if currentMediaInfoStatus?.checked}
            <span class="text-green-400 text-xs ml-2">(real)</span>
        {:else}
            <span class="text-yellow-400 text-xs ml-2">(checking...)</span>
        {/if}
        {#if currentMediaInfoStatus?.version}
            <span class="text-gray-500 text-xs ml-2">v{currentMediaInfoStatus.version}</span>
        {/if}
    </div>
    <div class="debug-controls">
        <button class="debug-button" on:click={() => forceMediaInfoStatus(true)}>
            Force Available
        </button>
        <button class="debug-button" on:click={() => forceMediaInfoStatus(false)}>
            Force Unavailable
        </button>
        <button class="debug-button reset" on:click={resetMediaInfoStatus}>
            Reset to Real
        </button>
    </div>
    <div class="text-xs text-gray-500 mt-2">
        Controls MediaInfo availability checks - required for media analysis
    </div>
</div>

<!-- Docker Status (third) -->
<div class="debug-section">
    <div class="status-row">
        <span class="text-gray-400 text-xs font-semibold">Docker Status:</span>
        <span class="status-value" class:text-green-400={currentDockerStatus?.available}
              class:text-red-400={!currentDockerStatus?.available}>
            {currentDockerStatus?.available ? 'Available' : 'Unavailable'}
        </span>
        {#if dockerForced}
            <span class="text-purple-400 text-xs ml-2">(forced)</span>
        {:else if currentDockerStatus?.checked}
            <span class="text-green-400 text-xs ml-2">(real)</span>
        {:else}
            <span class="text-yellow-400 text-xs ml-2">(checking...)</span>
        {/if}
        {#if currentDockerStatus?.version}
            <span class="text-gray-500 text-xs ml-2">v{currentDockerStatus.version}</span>
        {/if}
    </div>
    <div class="debug-controls">
        <button class="debug-button" on:click={() => forceDockerStatus(true)}>
            Force Available
        </button>
        <button class="debug-button" on:click={() => forceDockerStatus(false)}>
            Force Unavailable
        </button>
        <button class="debug-button reset" on:click={resetDockerStatus}>
            Reset to Real
        </button>
    </div>
    <div class="text-xs text-gray-500 mt-2">
        Controls Docker availability checks for features requiring Docker
    </div>
</div>

<!-- Internet Status (fourth) -->
<div class="debug-section">
    <div class="status-row">
        <span class="text-gray-400 text-xs font-semibold">Internet Status:</span>
        <span class="status-value" class:text-green-400={currentInternetStatus?.online}
              class:text-red-400={!currentInternetStatus?.online}>
            {currentInternetStatus?.online ? 'Online' : 'Offline'}
        </span>
        {#if internetForced}
            <span class="text-purple-400 text-xs ml-2">(forced)</span>
        {:else if currentInternetStatus?.checked}
            <span class="text-green-400 text-xs ml-2">(real)</span>
        {:else}
            <span class="text-yellow-400 text-xs ml-2">(checking...)</span>
        {/if}
        {#if currentInternetStatus?.latency}
            <span class="text-gray-500 text-xs ml-2">{currentInternetStatus.latency}ms</span>
        {/if}
    </div>
    <div class="debug-controls">
        <button class="debug-button" on:click={() => forceInternetStatus(true)}>
            Force Online
        </button>
        <button class="debug-button" on:click={() => forceInternetStatus(false)}>
            Force Offline
        </button>
        <button class="debug-button reset" on:click={resetInternetStatus}>
            Reset to Real
        </button>
    </div>
    <div class="text-xs text-gray-500 mt-2">
        Controls Internet connectivity checks for AI-powered features
    </div>
</div>

<!-- LLM State (fifth) -->
<div class="debug-section">
    <div class="status-row">
        <span class="text-gray-400 text-xs font-semibold">LLM State:</span>
        <span class="status-value" class:text-green-400={currentLLMState?.globalState === 'ready'}
              class:text-yellow-400={currentLLMState?.globalState === 'initializing' || currentLLMState?.globalState === 'updating'}
              class:text-red-400={currentLLMState?.globalState === 'error'}>
            {currentLLMState?.globalState || 'unknown'}
        </span>
        {#if currentLLMState?.message?.startsWith('Debug: Forced')}
            <span class="text-purple-400 text-xs ml-2">(debug mode)</span>
        {:else if currentLLMState}
            <span class="text-green-400 text-xs ml-2">(real state)</span>
        {/if}
    </div>
    <div class="debug-controls">
        <button class="debug-button" on:click={() => forceLLMState('initializing')}>
            Force Initializing
        </button>
        <button class="debug-button" on:click={() => forceLLMState('updating')}>
            Force Updating
        </button>
        <button class="debug-button" on:click={() => forceLLMState('ready')}>
            Force Ready
        </button>
        <button class="debug-button" on:click={() => forceLLMState('error')}>
            Force Error
        </button>
        <button class="debug-button reset" on:click={resetLLMState}>
            Reset to Real State
        </button>
    </div>
    <div class="text-xs text-gray-500 mt-2">
        Note: These are client-side only for UI testing
    </div>
</div>

<!-- User Activity (last) -->
<div class="debug-section">
    <div class="status-row">
        <span class="text-gray-400 text-xs font-semibold">User Activity:</span>
        <span class="status-value" class:text-green-400={currentUserActivityState === 'active'}
              class:text-yellow-400={currentUserActivityState === 'idle'}
              class:text-red-400={currentUserActivityState === 'afk'}>
            {currentUserActivityState}
            {#if isForced}
                <span class="text-purple-400 text-xs">(forced)</span>
            {/if}
        </span>
    </div>
    <div class="debug-controls">
        <button class="debug-button" on:click={() => forceUserActivityState('active')}>
            Force Active
        </button>
        <button class="debug-button" on:click={() => forceUserActivityState('idle')}>
            Force Idle
        </button>
        <button class="debug-button" on:click={() => forceUserActivityState('afk')}>
            Force AFK
        </button>
        <button class="debug-button reset" on:click={resetUserActivityState}>
            Reset to Auto
        </button>
    </div>
    <div class="text-xs text-gray-500 mt-2">
        Active: User is interacting | Idle: 5s-5min inactivity | AFK: >5min away
    </div>
</div>

{:else if activeSubTab === 'debounce'}

<!-- Debounce Controls -->
<div class="debug-section">
    <div class="debounce-info">
        <p class="text-xs text-gray-400">
            Control delays for WebView2/WebKit bridge calls.
        </p>
        <p class="text-xs text-gray-400">
            Override: <span class="{windowsOverridden || otherOverridden ? 'text-purple-400' : 'text-gray-500'}">{windowsOverridden || otherOverridden ? 'Yes' : 'No'}</span>
        </p>
    </div>
    
    <!-- Windows Slider -->
    <div class="debounce-row">
        <div class="slider-label">
            <span class="text-xs">Windows</span>
        </div>
        <div class="slider-group">
            <span class="text-xs text-gray-500">10</span>
            <input 
                type="range" 
                min="10" 
                max="800" 
                step="10"
                bind:value={windowsDebounce}
                on:input={updateWindowsDebounce}
                class="debounce-slider compact"
            />
            <span class="text-xs text-gray-500">800</span>
            <span class="slider-value-compact">{windowsDebounce}ms</span>
            <button class="compact-reset" on:click={resetWindowsDebounce} title="Reset to 200ms">
                ↺
            </button>
        </div>
    </div>
    
    <!-- Other OS Slider -->
    <div class="debounce-row">
        <div class="slider-label">
            <span class="text-xs">Linux/Mac</span>
        </div>
        <div class="slider-group">
            <span class="text-xs text-gray-500">10</span>
            <input 
                type="range" 
                min="10" 
                max="800" 
                step="10"
                bind:value={otherDebounce}
                on:input={updateOtherDebounce}
                class="debounce-slider compact"
            />
            <span class="text-xs text-gray-500">800</span>
            <span class="slider-value-compact">{otherDebounce}ms</span>
            <button class="compact-reset" on:click={resetOtherDebounce} title="Reset to 10ms">
                ↺
            </button>
        </div>
    </div>
    
    <div class="debounce-footer">
        <button class="debug-button reset small" on:click={resetAllDebounce}>
            Reset All
        </button>
        <span class="text-xs text-gray-500">Changes apply immediately</span>
    </div>
</div>

{/if}

<style>
    h4 {
        margin: 0 0 12px 0;
        font-size: 13px;
        opacity: 0.9;
    }
    
    /* Debug tab styles */
    .debug-section {
        margin-bottom: 16px;
        padding-bottom: 12px;
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    }
    
    .debug-section:last-child {
        border-bottom: none;
        margin-bottom: 0;
    }
    
    .status-row {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 8px;
        font-size: 12px;
    }
    
    .status-value {
        font-weight: 600;
    }
    
    .debug-controls {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
        margin-top: 8px;
    }
    
    .debug-button {
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
    
    .debug-button:hover {
        background: hsla(215, 20%, 30%, 0.9);
        border-color: hsla(215, 30%, 50%, 0.4);
    }
    
    .debug-button.reset {
        background: hsla(0, 0%, 20%, 0.9);
        border-color: hsla(0, 0%, 40%, 0.4);
    }
    
    .debug-button.reset:hover {
        background: hsla(0, 0%, 30%, 0.9);
        border-color: hsla(0, 0%, 50%, 0.4);
    }
    
    /* Status colors */
    .text-green-400 {
        color: #68e796;
    }
    
    .text-yellow-400 {
        color: #fbbf24;
    }
    
    .text-red-400 {
        color: #f87171;
    }
    
    .text-purple-400 {
        color: #a78bfa;
    }
    
    .text-gray-400 {
        color: rgba(255, 255, 255, 0.5);
    }
    
    /* Utility classes */
    .text-xs {
        font-size: 0.75rem;
    }
    
    .font-semibold {
        font-weight: 600;
    }
    
    .mb-2 {
        margin-bottom: 0.5rem;
    }
    
    .mt-2 {
        margin-top: 0.5rem;
    }
    
    .ml-2 {
        margin-left: 0.5rem;
    }
    
    .opacity-80 {
        opacity: 0.8;
    }
    
    /* Sub-tab navigation */
    .sub-tab-navigation {
        display: flex;
        gap: 4px;
        margin-bottom: 16px;
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    }
    
    .sub-tab-button {
        padding: 8px 16px;
        background: transparent;
        border: none;
        color: rgba(255, 255, 255, 0.6);
        font-size: 12px;
        cursor: pointer;
        transition: all 0.2s;
        border-bottom: 2px solid transparent;
    }
    
    .sub-tab-button:hover {
        color: rgba(255, 255, 255, 0.9);
        background: rgba(255, 255, 255, 0.05);
    }
    
    .sub-tab-button.active {
        color: white;
        border-bottom-color: var(--primary-color);
    }
    
    /* Debounce controls */
    .debounce-info {
        margin-bottom: 8px;
    }
    
    .debounce-slider {
        width: 100%;
        height: 4px;
        background: rgba(255, 255, 255, 0.1);
        border-radius: 2px;
        outline: none;
        -webkit-appearance: none;
        cursor: pointer;
    }
    
    .debounce-slider::-webkit-slider-thumb {
        -webkit-appearance: none;
        width: 16px;
        height: 16px;
        background: var(--primary-color);
        border-radius: 50%;
        cursor: pointer;
        transition: transform 0.2s;
    }
    
    .debounce-slider::-webkit-slider-thumb:hover {
        transform: scale(1.2);
    }
    
    .debounce-slider::-moz-range-thumb {
        width: 16px;
        height: 16px;
        background: var(--primary-color);
        border-radius: 50%;
        cursor: pointer;
        border: none;
        transition: transform 0.2s;
    }
    
    .debounce-slider::-moz-range-thumb:hover {
        transform: scale(1.2);
    }
    
    .text-primary {
        color: var(--primary-color);
    }
    
    /* Compact debounce layout */
    .debounce-row {
        display: flex;
        align-items: center;
        gap: 12px;
        margin: 8px 0;
    }
    
    .slider-label {
        min-width: 70px;
        font-size: 12px;
        color: rgba(255, 255, 255, 0.7);
    }
    
    .slider-group {
        display: flex;
        align-items: center;
        gap: 8px;
        flex: 1;
    }
    
    .debounce-slider.compact {
        flex: 1;
        height: 3px;
        margin: 0;
    }
    
    .slider-value-compact {
        min-width: 45px;
        font-size: 11px;
        font-weight: 600;
        color: var(--primary-color);
        text-align: right;
    }
    
    .compact-reset {
        width: 20px;
        height: 20px;
        padding: 0;
        background: rgba(255, 255, 255, 0.1);
        border: 1px solid rgba(255, 255, 255, 0.2);
        border-radius: 4px;
        color: rgba(255, 255, 255, 0.6);
        font-size: 12px;
        cursor: pointer;
        transition: all 0.2s;
        display: flex;
        align-items: center;
        justify-content: center;
    }
    
    .compact-reset:hover {
        background: rgba(255, 255, 255, 0.2);
        color: white;
    }
    
    .debounce-footer {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-top: 12px;
        padding-top: 8px;
        border-top: 1px solid rgba(255, 255, 255, 0.1);
    }
    
    .debug-button.small {
        padding: 4px 12px;
        font-size: 11px;
        min-width: auto;
    }
</style>