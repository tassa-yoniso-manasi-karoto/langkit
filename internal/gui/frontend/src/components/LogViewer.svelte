<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
    import ProgressBar from './ProgressBar.svelte';

    let logs: Array<{time: string, level: string, message: string}> = [];
    export let downloadProgress: any = null;
    let scrollContainer: HTMLElement;
    let autoScroll = true;
    let isScrolling = false;
    let scrollTimeout: number;
    
    // Add log level filter
    const logLevels = ['TRACE', 'DEBUG', 'INFO', 'WARN', 'ERROR', 'FATAL', 'PANIC'];
    let selectedLogLevel = 'INFO';
    
    const logLevelPriority = {
        'TRACE': 0,
        'DEBUG': 1,
        'INFO': 2,
        'WARN': 3,
        'ERROR': 4,
        'FATAL': 5,
        'PANIC': 6
    };

    $: filteredLogs = logs.filter(log => 
        logLevelPriority[log.level] >= logLevelPriority[selectedLogLevel]
    );
    
    const getLevelClass = (level: string) => ({
        'DEBUG': 'debug',
        'INFO': 'info',
        'WARN': 'warn',
        'ERROR': 'error',
        'FATAL': 'fatal',
        'PANIC': 'panic',
        'TRACE': 'trace'
    }[level] || 'info');
    
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

    onMount(() => {
        EventsOn("log", (log) => {
            logs = [...logs, log];
            if (autoScroll) {
                scrollToBottom();
            }
        });

        EventsOn("download-progress", (progress) => {
            downloadProgress = progress;
            if (autoScroll) {
                scrollToBottom();
            }
        });
    });

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
            {#each filteredLogs as log}
                <div class="log-entry" transition:fade={{ duration: 150 }}>
                    <span class="time">{log.time}</span>
                    <span class="level {getLevelClass(log.level)}">{log.level}</span>
                    <span class="message">{log.message}</span>
                </div>
            {/each}
        </div>
        
        {#if downloadProgress}
            <div class="progress-section">
                <ProgressBar 
                    progress={downloadProgress.progress}
                    current={downloadProgress.current}
                    total={downloadProgress.total}
                    speed={downloadProgress.speed}
                    currentFile={downloadProgress.currentFile}
                    operation={downloadProgress.operation}
                />
            </div>
        {/if}
    </div>
</div>

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
    }

    .log-entry:hover {
        background: rgba(255, 255, 255, 0.02);
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
    }

    .progress-section {
        padding: 8px;
        background: #252525;
        border-top: 1px solid #333;
    }

    /* Log level colors */
    .debug { color: #7cafc2; }
    .info { color: #99c794; }
    .warn { color: #fac863; }
    .error { color: #ec5f67; }
    .fatal { color: #ff8080; }
    .panic { color: #ff6b6b; }
    .trace { color: #c792ea; }

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
</style>