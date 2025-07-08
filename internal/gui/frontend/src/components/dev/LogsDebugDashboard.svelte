<script lang="ts">
    import { enableTraceLogsStore, enableFrontendLoggingStore, displayFrontendLogsStore } from '../../lib/stores';
    
    // No props needed - using stores directly
</script>

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
</style>