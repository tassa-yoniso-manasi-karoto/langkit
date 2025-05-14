<script lang="ts">
    import { fade, scale } from 'svelte/transition';
    import { onMount, onDestroy } from 'svelte';
    import Portal from "svelte-portal/src/Portal.svelte";
    import { wasmLogger, WasmLogLevel } from '../lib/wasm-logger';
    import { getWasmState } from '../lib/wasm-state';
    import { settings } from '../lib/stores';
    import { logger } from '../lib/logger';
    import WasmPerformanceDashboard from './WasmPerformanceDashboard.svelte';
    import MemoryTestButton from './MemoryTestButton.svelte';
    
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
        { id: 'debug', name: 'Debug', icon: 'bug_report' }
    ];
    
    // Store current settings
    let currentSettings;
    const unsubscribeSettings = settings.subscribe(value => {
        currentSettings = value;
    });
    
    // Only show in dev mode - reactively update when version is loaded
    $: showDashboard = !!version && (version === 'dev' || version.includes('dev'));
    
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
        wasmLogger.log(
            WasmLogLevel.DEBUG,
            'devtools',
            `Developer dashboard ${isExpanded ? 'expanded' : 'minimized'}`
        );
    }
    
    function switchTab(id: string) {
        activeTab = id;
    }
    
    // Clean up event listeners on destroy
    onDestroy(() => {
        window.removeEventListener('mousemove', handleMouseMove);
        window.removeEventListener('mouseup', handleMouseUp);
        unsubscribeSettings();
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
            wasmLogger.log(
                WasmLogLevel.INFO,
                'devtools',
                `Developer dashboard initialized (version: ${version})`
            );
        }

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
        };
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
                        <div transition:fade={{duration: 200}}>
                            <WasmPerformanceDashboard />
                        </div>
                    {:else if activeTab === 'state'}
                        <div transition:fade={{duration: 200}}>
                            <h4>Application State</h4>
                            
                            <div class="state-section">
                                <h5 class="text-xs font-semibold mb-2 opacity-80">Counter Values</h5>
                                <table class="state-table">
                                    <tbody>
                                        <tr>
                                            <td class="state-key">countAppStart</td>
                                            <td class="state-value">{currentSettings?.countAppStart || 0}</td>
                                            <td class="state-description">App launch count</td>
                                        </tr>
                                        <tr>
                                            <td class="state-key">countProcessStart</td>
                                            <td class="state-value">{currentSettings?.countProcessStart || 0}</td>
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
                        </div>
                    {:else if activeTab === 'logs'}
                        <div transition:fade={{duration: 200}}>
                            <h4>Log Viewer Controls</h4>

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

                        </div>
                    {:else if activeTab === 'debug'}
                        <div transition:fade={{duration: 200}}>
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

                            <div class="debug-controls">
                                <button class="debug-button">
                                    Reset All Settings
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
</style>