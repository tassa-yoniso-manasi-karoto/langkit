<script lang="ts">
    import { onMount } from 'svelte';
    import { enableTraceLogsStore, enableFrontendLoggingStore, displayFrontendLogsStore, sendFrontendTraceLogsStore, settings } from '../../lib/stores';
    import { isWasmSupported } from '../../lib/wasm';
    import { LoadSettings } from '../../api/services/settings';
    
    // Ensure settings are loaded
    onMount(async () => {
        // Settings should already be loaded from App.svelte, but verify they exist
        // If settings look like defaults (e.g., no apiKeys), reload them
        const currentSettings = $settings;
        if (!currentSettings.apiKeys) {
            try {
                const loadedSettings = await LoadSettings();
                settings.set(loadedSettings);
            } catch (error) {
                console.error('Failed to load settings in LogsDebugDashboard:', error);
            }
        }
    });
</script>

<h4>Log Controls</h4>
<div class="control-section mb-4">
    <div class="flex items-center gap-3">
        <label class="switch">
            <input type="checkbox" bind:checked={$enableTraceLogsStore}>
            <span class="toggle-slider round"></span>
        </label>
        <span class="text-sm text-gray-300">Enable Trace Logs</span>
    </div>
    <p class="text-xs text-gray-500 mt-1">
        Streams verbose trace logs to the GUI log viewer.
    </p>
</div>

<div class="control-section mb-4">
    <div class="flex items-center gap-3">
        <label class="switch">
            <input type="checkbox" bind:checked={$enableFrontendLoggingStore}>
            <span class="toggle-slider round"></span>
        </label>
        <span class="text-sm text-gray-300">Send Frontend Logs to Backend</span>
    </div>
    <p class="text-xs text-gray-500 mt-1">
        Forwards frontend logs to the backend for logging.
    </p>
    
    {#if $enableFrontendLoggingStore}
        <div class="ml-8 mt-3 flex items-center gap-3">
            <label class="switch">
                <input type="checkbox" bind:checked={$sendFrontendTraceLogsStore}>
                <span class="toggle-slider round"></span>
            </label>
            <span class="text-sm text-gray-300">Include Trace Logs</span>
        </div>
    {/if}
</div>

<div class="control-section mb-4">
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

<h4>LogViewer Controls</h4>
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
    <h5 class="text-xs font-semibold mb-2 opacity-80">WebAssembly Settings</h5>
    {#if !isWasmSupported()}
        <p class="text-xs text-gray-500">WebAssembly not supported in this browser</p>
    {:else}
        <div class="flex items-center gap-3 mb-3">
            <label class="switch">
                <input type="checkbox" bind:checked={$settings.useWasm}>
                <span class="toggle-slider round"></span>
            </label>
            <span class="text-sm text-gray-300">Enable WebAssembly</span>
        </div>
        
        {#if $settings.useWasm}
            <div class="mb-3">
                <label class="text-xs text-gray-300 block mb-1">WebAssembly Mode</label>
                <select 
                    bind:value={$settings.forceWasmMode}
                    class="control-select text-xs"
                    style="padding: 4px 8px; background: hsla(215, 20%, 20%, 0.9); border: 1px solid hsla(215, 30%, 40%, 0.4); border-radius: 4px; color: white; min-width: 150px;"
                >
                    <option value="auto">Auto (Based on threshold)</option>
                    <option value="enabled">Always Enabled</option>
                    <option value="disabled">Always Disabled</option>
                </select>
            </div>
            
            {#if $settings.forceWasmMode === 'auto'}
                <div class="mb-2">
                    <label class="text-xs text-gray-300 block mb-1">
                        Size Threshold: {$settings.wasmSizeThreshold} logs
                    </label>
                    <input 
                        type="range"
                        bind:value={$settings.wasmSizeThreshold}
                        min="100"
                        max="5000"
                        step="100"
                        class="w-full h-2 bg-gray-700 rounded-lg appearance-none cursor-pointer"
                        style="background: linear-gradient(to right, hsl(261, 90%, 70%) 0%, hsl(261, 90%, 70%) {($settings.wasmSizeThreshold - 100) / 49}%, #4b5563 {($settings.wasmSizeThreshold - 100) / 49}%, #4b5563 100%);"
                    >
                    <div class="flex justify-between text-xs text-gray-500 mt-1">
                        <span>100</span>
                        <span>5000</span>
                    </div>
                </div>
            {/if}
        {/if}
        
        <p class="text-xs text-gray-500 mt-1">
            Controls WebAssembly usage for log processing performance.
        </p>
    {/if}
</div>

<div class="control-section mb-4">
    <h5 class="text-xs font-semibold mb-2 opacity-80">LogViewer Performance</h5>
    <div class="mb-2">
        <label class="text-xs text-gray-300 block mb-1">
            Virtualization Threshold: {$settings.logViewerVirtualizationThreshold} logs
        </label>
        <input 
            type="range"
            bind:value={$settings.logViewerVirtualizationThreshold}
            min="500"
            max="10000"
            step="500"
            class="w-full h-2 bg-gray-700 rounded-lg appearance-none cursor-pointer"
            style="background: linear-gradient(to right, hsl(261, 90%, 70%) 0%, hsl(261, 90%, 70%) {($settings.logViewerVirtualizationThreshold - 500) / 95}%, #4b5563 {($settings.logViewerVirtualizationThreshold - 500) / 95}%, #4b5563 100%);"
        >
        <div class="flex justify-between text-xs text-gray-500 mt-1">
            <span>500</span>
            <span>10000</span>
        </div>
    </div>
    <p class="text-xs text-gray-500 mt-1">
        Enable virtualization when log count exceeds threshold.
    </p>
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

<style>
    h4 {
        margin: 0 0 12px 0;
        font-size: 13px;
        opacity: 0.9;
    }
    
    /* Control sections layout */
    .control-section {
        margin-bottom: 16px;
        padding-bottom: 8px;
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    }

    .control-section:last-child {
        border-bottom: none;
        margin-bottom: 0;
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
    
    /* Utility classes */
    .mb-4 {
        margin-bottom: 1rem;
    }
    
    .mb-2 {
        margin-bottom: 0.5rem;
    }
    
    .mt-1 {
        margin-top: 0.25rem;
    }
    
    .text-xs {
        font-size: 0.75rem;
    }
    
    .text-sm {
        font-size: 0.875rem;
    }
    
    .text-gray-300 {
        color: rgba(255, 255, 255, 0.7);
    }
    
    .text-gray-500 {
        color: rgba(255, 255, 255, 0.5);
    }
    
    .font-semibold {
        font-weight: 600;
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
    
    .items-center {
        align-items: center;
    }
    
    .gap-2 {
        gap: 0.5rem;
    }
    
    .gap-3 {
        gap: 0.75rem;
    }
    
    .ml-8 {
        margin-left: 2rem;
    }
    
    .mt-3 {
        margin-top: 0.75rem;
    }
</style>