<script lang="ts">

    import { onMount, onDestroy } from 'svelte';
    import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
    import ProgressBar from './ProgressBar.svelte';
    
    import { settings } from '../lib/stores';

    interface LogMessage {
        level: string;
        message: string;
        time: string;
        behavior?: string;
        // Allow any additional fields for structured logging
        [key: string]: any;
    }


    let logs: LogMessage[] = [];
    export let downloadProgress: any = null;
    
    let scrollContainer: HTMLElement;
    let autoScroll = true;
    let isScrolling = false;
    let scrollTimeout: number;

    // Add log level filter
    const logLevels = ['TRACE', 'DEBUG', 'INFO', 'WARN', 'ERROR', 'FATAL', 'PANIC'];
    let selectedLogLevel = 'INFO';
    
    const logLevelPriority = {
        'trace': 0,
        'debug': 1,
        'info': 2,
        'warn': 3,
        'error': 4,
        'fatal': 5,
        'panic': 6
    };

    // Update getLevelClass to handle case standardization
    const getLevelClass = (level: string) => ({
        'DEBUG': 'debug',
        'INFO': 'info',
        'WARN': 'warn',
        'ERROR': 'error',
        'FATAL': 'fatal',
        'PANIC': 'panic',
        'TRACE': 'trace'
    }[level.toUpperCase()] || 'info');

    // Update filtered logs to use standardized case
    $: filteredLogs = logs.filter(log => 
        logLevelPriority[log.level.toLowerCase()] >= logLevelPriority[selectedLogLevel.toLowerCase()]
    );

    // Add debug logging
    $: {
        console.log("Filtered logs:", filteredLogs);
        console.log("Selected level:", selectedLogLevel);
        console.log("Filtering active:", logs.length !== filteredLogs.length);
    }
    
    
    function addLog(logData: LogMessage) {
        // If we've reached the maximum, remove the oldest entries
        if (logs.length >= $settings.maxLogEntries) {
            // Remove oldest entries to make room for the new one
            logs = logs.slice(-($settings.maxLogEntries - 1));
        }
        
        // Add the new log entry
        logs = [...logs, logData];
        
        if (autoScroll) {
            scrollToBottom();
        }
    }
    
    function handleScroll(e: Event) {
        if (isScrolling) return;
        
        const target = e.currentTarget as HTMLElement;
        const isAtBottom = Math.abs(
            target.scrollHeight - target.clientHeight - target.scrollTop
        ) < 1;
        
        if (!isScrolling) {
            autoScroll = isAtBottom;
        }

        clearTimeout(scrollTimeout);
        
        scrollTimeout = window.setTimeout(() => {
            isScrolling = false;
        }, 150);
    }

    function scrollToBottom() {
        if (!scrollContainer || !autoScroll) return;
        
        isScrolling = true;
        requestAnimationFrame(() => {
            scrollContainer.scrollTop = scrollContainer.scrollHeight;
            setTimeout(() => {
                isScrolling = false;
            }, 50);
        });
    }

    function toggleAutoScroll(value: boolean) {
        autoScroll = value;
        if (autoScroll) {
            scrollToBottom();
        }
    }

    function clearLogs() {
        logs = [];
        downloadProgress = null;
    }

    const behaviorColors = {
        'abort_task': 'text-red-400',
        'abort_all': 'text-red-600',
        'probe': 'text-yellow-400'
    };

    // Helper function to format fields
    function formatFields(fields: Record<string, any> | undefined): string {
        if (!fields) return '';
        return Object.entries(fields)
            .map(([key, value]) => `${key}=${JSON.stringify(value)}`)
            .join(' ');
    }

    // Helper function to format structured fields
    function formatStructuredFields(log: LogMessage): string {
        const excludedKeys = ['level', 'message', 'time', 'behavior'];
        const fields = Object.entries(log)
            .filter(([key]) => !excludedKeys.includes(key))
            .map(([key, value]) => {
                if (typeof value === 'object') {
                    return `${key}=${JSON.stringify(value)}`;
                }
                return `${key}=${value}`;
            })
            .join(' ');
        return fields;
    }

    // Update the mount event handler
    let mounted = false; // Add this flag

    // Add a reactive statement to monitor logs array
    $: {
        console.log("Logs array changed, new length:", logs.length);
        console.log("Current logs:", logs);
    }
    
    onMount(() => {
        mounted = true;
        
        EventsOn("log", (rawLog: any) => {
            try {
                const logData: LogMessage = typeof rawLog === 'string' ? JSON.parse(rawLog) : rawLog;
                
                const timeStr = new Date(logData.time).toLocaleTimeString('en-US', {
                    hour12: false,
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit'
                });
                logData.time = timeStr;
                
                addLog(logData);
                
            } catch (error) {
                console.error("Error processing log:", error);
                console.error("Raw log data:", rawLog);
            }
        });

        EventsOn("progress", (progress) => {
            downloadProgress = progress;
            if (autoScroll) {
                scrollToBottom();
            }
        });

        EventsOn("status", (status: string) => {
            // Handle status updates if needed
        });
    });

    function handleLogBehavior(log: LogMessage) {
        switch (log.behavior) {
            case 'abort_all':
                // Handle complete abort
                break;
            case 'abort_task':
                // Handle single task abort
                break;
            case 'warning':
                // Handle warning
                break;
        }
    }

    onDestroy(() => {
        EventsOff("log");
        EventsOff("download-progress");
        clearTimeout(scrollTimeout);
    });

    $: if (logs.length && autoScroll) {
        scrollToBottom();
    }
</script>

<div class="log-viewer font-dm-mono">
    <div class="controls">
        <div class="flex items-center gap-6">
            <!-- Log level filter -->
            <div class="flex items-center gap-2">
                <span class="text-xs uppercase tracking-wider font-medium text-gray-400">
                    Log Level:
                </span>
                <select
                    bind:value={selectedLogLevel}
                    class="bg-[#333] text-white text-xs font-medium uppercase tracking-wider
                           border-none rounded px-2 py-1.5
                           focus:ring-1 focus:ring-accent outline-none"
                >
                    {#each logLevels as level}
                        <option value={level}>{level}</option>
                    {/each}
                </select>
            </div>

            <!-- Auto-scroll toggle -->
            <button 
                type="button" 
                class="flex items-center gap-2 text-xs uppercase tracking-wider font-medium
                       text-gray-400 hover:text-white transition-colors"
                on:click={() => toggleAutoScroll(!autoScroll)}
            >
                <input 
                    type="checkbox" 
                    checked={autoScroll}
                    on:change={(e) => toggleAutoScroll(e.target.checked)}
                    class="w-3.5 h-3.5 accent-accent"
                />
                Auto-scroll
            </button>
            
            <!-- Clear button -->
            <button 
                on:click={clearLogs}
                class="text-xs uppercase tracking-wider font-medium
                       text-gray-400 hover:text-white transition-colors"
            >
                Clear
            </button>
        </div>
    </div>
    
    <div class="content-wrapper">
        <div 
            class="log-container" 
            bind:this={scrollContainer} 
            on:scroll={handleScroll}
        >
            {#if filteredLogs.length === 0}
                <div class="empty-state">
                    <span>No logs to display</span>
                </div>
            {:else}
                {#each filteredLogs as log}
                    <div class="log-entry {log.behavior ? behaviorColors[log.behavior] : ''}" 
                         transition:fade={{ duration: 150 }}>
                        <span class="time">{log.time}</span>
                        <span class="level {getLevelClass(log.level)}">{log.level}</span>
                        <span class="message">
                            {log.message}
                            {#if formatStructuredFields(log)}
                                <span class="fields">{formatStructuredFields(log)}</span>
                            {/if}
                        </span>
                    </div>
                {/each}
            {/if}
        </div>
        
        {#if downloadProgress}
            <div class="progress-section">
                <ProgressBar 
                    progress={downloadProgress.progress}
                    current={downloadProgress.current}
                    total={downloadProgress.total}
                    description={downloadProgress.description}
                />
            </div>
        {/if}
    </div>
</div>
<!--{#if import.meta.env.DEV}
    <div class="debug-overlay">
        Total logs: {logs.length}<br>
        Filtered logs: {filteredLogs.length}<br>
        Selected level: {selectedLogLevel}<br>
        Log levels present: {[...new Set(logs.map(l => l.level))].join(', ')}
    </div>
{/if}-->

<style>
    .log-viewer {
        display: flex;
        flex-direction: column;
        height: 100%;
        background: #1e1e1e;
        color: #ffffff;
        font-family: 'DM Mono', monospace;
        font-size: 12px;
    }


    .content-wrapper {
        display: flex;
        flex-direction: column;
        flex: 1;
        min-height: 0;
        position: relative;
    }

    .controls {
        padding: 8px 12px;
        border-bottom: 1px solid #333;
        display: flex;
        justify-content: space-between;
        align-items: center;
        background: #252525;
        height: 40px; /* Fixed height for consistency */
    }

    .auto-scroll {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .empty-state {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
        width: 100%;
        position: absolute;
        top: 0;
        left: 0;
    }


    .empty-state {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
        color: #666;
        font-style: italic;
        font-size: 14px;
    }

    .log-container {
        flex: 1;
        overflow-y: auto;
        padding: 0;
        min-height: 0;
    }
    
    .log-entry {
        padding: 4px 12px;
        border-bottom: 1px solid #2a2a2a;
        white-space: pre-wrap;
        word-wrap: break-word;
        line-height: 1.4;
        display: flex;
        align-items: baseline;
        justify-content: flex-start;
        text-align: left;
        width: 100%;
        min-width: fit-content; /* Handle overflow */
    }


    .log-entry:hover {
        background: rgba(255, 255, 255, 0.02);
        width: 100%;
    }

    .time {
        color: #888;
        margin-right: 8px;
        font-size: 11px;
        flex-shrink: 0;
    }

    .level {
        font-weight: bold;
        margin-right: 8px;
        flex-shrink: 0;
        min-width: 40px;
    }

    .message {
        flex-grow: 1;
        color: #d4d4d4;
        text-align: left;
        overflow-x: auto; /* Allow horizontal scroll for very long messages */
    }
    /* Add horizontal scrollbar styling for overflow */
    .message::-webkit-scrollbar {
        height: 6px;
    }

    .message::-webkit-scrollbar-track {
        background: #1e1e1e;
    }

    .message::-webkit-scrollbar-thumb {
        background-color: #444444;
        border-radius: 3px;
    }

    .message::-webkit-scrollbar-thumb:hover {
        background-color: #555555;
    }

    .progress-section {
        padding: 8px;
        background: #252525;
        border-top: 1px solid #333;
    }

    /* Log level colors matching zerolog's ConsoleWriter */
    .debug { color: #3b82f6; }  /* blue */
    .info { color: #10b981; }   /* green */
    .warn { color: #f59e0b; }   /* yellow */
    .error { color: #ef4444; }  /* red */
    .fatal { color: #dc2626; }  /* darker red */
    .panic { color: #b91c1c; }  /* darkest red */
    .trace { color: #c084fc; }  /* purple */

    button {
        padding: 4px 12px;
        background: #333;
        border: none;
        color: #999;
        border-radius: 3px;
        cursor: pointer;
        font-size: 11px;
        text-transform: uppercase;
        transition: all 0.2s ease;
    }

    button:hover {
        background: #444;
        color: #fff;
    }

    /* Style the checkbox to be smaller and match the theme */
    input[type="checkbox"] {
        accent-color: #9f6ef7;
        margin: 0;
    }

    /* Scrollbar styles */
    .log-container::-webkit-scrollbar {
        width: 6px;
        height: 6px;
    }

    .log-container::-webkit-scrollbar-track {
        background: #1e1e1e;
    }

    .log-container::-webkit-scrollbar-thumb {
        background-color: #444444;
        border-radius: 3px;
    }

    .log-container::-webkit-scrollbar-thumb:hover {
        background-color: #555555;
    }

    .log-container::-webkit-scrollbar-corner {
        background: #1e1e1e;
    }
    

    select {
        appearance: none;
        background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 24 24' stroke='white'%3E%3Cpath stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M19 9l-7 7-7-7'%3E%3C/path%3E%3C/svg%3E");
        background-repeat: no-repeat;
        background-position: right 0.5rem center;
        background-size: 1em;
        padding-right: 1.75rem;
    }

    select:focus {
        box-shadow: 0 0 0 2px rgba(159, 110, 247, 0.3);
    }
    
    
    .fields {
        color: #666;
        margin-left: 8px;
        font-size: 11px;
    }

    .behavior {
        color: #666;
        margin-left: 8px;
        font-style: italic;
        font-size: 11px;
    }

    /* Add behavior-specific styles */
    .log-entry.abort_task {
        background: rgba(239, 68, 68, 0.1);
    }

    .log-entry.abort_all {
        background: rgba(239, 68, 68, 0.2);
    }

    .log-entry.warning {
        background: rgba(251, 191, 36, 0.1);
    }
    .fields {
        color: #666;
        margin-left: 8px;
        font-size: 11px;
        font-family: 'DM Mono', monospace;
    }
    .debug-overlay {
        position: fixed;
        top: 0;
        right: 0;
        background: rgba(0,0,0,0.8);
        padding: 8px;
        color: white;
        font-size: 12px;
        z-index: 9999;
        pointer-events: none;
    }
</style>