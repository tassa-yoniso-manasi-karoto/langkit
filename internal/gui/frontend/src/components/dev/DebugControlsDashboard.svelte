<script lang="ts">
    import { 
        forceLLMState, resetLLMState,
        forceUserActivityState, resetUserActivityState,
        forceDockerStatus, resetDockerStatus,
        forceInternetStatus, resetInternetStatus,
        forceFFmpegStatus, resetFFmpegStatus,
        forceMediaInfoStatus, resetMediaInfoStatus
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
</script>

<h4>Debug Controls</h4>

<div class="debug-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">LLM State</h5>
    <div class="status-row">
        <span class="text-gray-400 text-xs">Current state:</span>
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

<div class="debug-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">User Activity</h5>
    <div class="status-row">
        <span class="text-gray-400 text-xs">Current:</span>
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

<div class="debug-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">Docker Status</h5>
    <div class="status-row">
        <span class="text-gray-400 text-xs">Status:</span>
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

<div class="debug-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">Internet Status</h5>
    <div class="status-row">
        <span class="text-gray-400 text-xs">Status:</span>
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

<div class="debug-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">FFmpeg Status</h5>
    <div class="status-row">
        <span class="text-gray-400 text-xs">Status:</span>
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

<div class="debug-section">
    <h5 class="text-xs font-semibold mb-2 opacity-80">MediaInfo Status</h5>
    <div class="status-row">
        <span class="text-gray-400 text-xs">Status:</span>
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
</style>