<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { logger } from '../../lib/logger';
    import { SetDryRunConfig, InjectDryRunError, GetDryRunStatus } from '../../api/services/dryrun';
    
    // Props
    export let isProcessing: boolean = false;
    
    // Test configuration
    let processingDelay = 1000; // ms
    let errorSchedule: Array<{index: number, type: string}> = [];
    let isTestRunning = false;
    
    // Store previous configuration for seamless reconfiguration
    let previousConfig: any = null;
    
    // New error input fields
    let newErrorIndex = 0;
    let newErrorType = "error";
    
    // Convert between display names and internal names
    const displayToInternal: Record<string, string> = {
        'Error': 'error',
        'AbortTask': 'abort_task',
        'AbortAll': 'abort_all'
    };
    
    const internalToDisplay: Record<string, string> = {
        'error': 'Error',
        'abort_task': 'AbortTask',
        'abort_all': 'AbortAll'
    };
    
    // Predefined scenarios (using internal names)
    const scenarios = {
        clean: { 
            name: "Clean", 
            errors: [] 
        },
        earlyAbort: { 
            name: "Early Abort", 
            errors: [{index: 2, type: "abort_all"}] 
        },
        scattered: { 
            name: "Scattered", 
            errors: [
                {index: 1, type: "abort_task"},
                {index: 4, type: "error"},
                {index: 7, type: "abort_task"}
            ]
        },
        progressive: { 
            name: "Progressive", 
            errors: [
                {index: 5, type: "error"},
                {index: 8, type: "abort_task"},
                {index: 10, type: "abort_all"}
            ]
        },
        mixed: {
            name: "Mixed Errors",
            errors: [
                {index: 2, type: "error"},
                {index: 5, type: "error"},
                {index: 9, type: "abort_task"}
            ]
        }
    };
    
    let selectedScenario = "";
    
    // Track previous processing state
    let wasProcessing = false;
    
    // Auto-reset when processing transitions from true to false
    $: {
        if (wasProcessing && !isProcessing && isTestRunning) {
            // Processing just finished, reset dry run
            SetDryRunConfig({
                enabled: false,
                delayMs: 1000,
                errorPoints: {},
                nextErrorIndex: -1,
                nextErrorType: "",
                processedCount: 0
            }).then(() => {
                isTestRunning = false;
                logger.info('dev/DryRunProgressTest', 'Dry run auto-reset after processing finished');
            }).catch(error => {
                logger.error('dev/DryRunProgressTest', 'Failed to auto-reset dry run', { error });
            });
        }
        wasProcessing = isProcessing;
    }
    
    async function configureDryRun() {
        // Convert error schedule to map format
        const errorMap: Record<number, string> = {};
        errorSchedule.forEach(e => {
            errorMap[e.index] = e.type;
        });
        
        // Create configuration
        const config = {
            enabled: true,
            delayMs: processingDelay,
            errorPoints: errorMap,
            nextErrorIndex: -1,
            nextErrorType: "",
            processedCount: 0
        };
        
        // Store for potential reconfiguration
        previousConfig = config;
        
        // Set the dry run configuration
        try {
            await SetDryRunConfig(config);
            isTestRunning = true;
            logger.info('dev/DryRunProgressTest', 'Dry run configured', { 
                errorCount: errorSchedule.length,
                delay: processingDelay 
            });
        } catch (error) {
            logger.error('dev/DryRunProgressTest', 'Failed to set dry run config', { error });
            alert("Failed to configure dry run: " + error);
            return;
        }
    }
    
    
    async function injectError(type: string) {
        try {
            await InjectDryRunError(type);
            logger.debug('dev/DryRunProgressTest', 'Injected error', { type });
        } catch (error) {
            logger.error('dev/DryRunProgressTest', 'Failed to inject error', { error });
            alert("Failed to inject error: " + error);
        }
    }
    
    function loadScenario(key: string) {
        const scenario = scenarios[key as keyof typeof scenarios];
        if (scenario) {
            errorSchedule = [...scenario.errors];
            selectedScenario = key;
        }
    }
    
    function addError() {
        if (newErrorIndex >= 0) {
            errorSchedule = [...errorSchedule, {
                index: newErrorIndex,
                type: newErrorType
            }];
            // Sort by index
            errorSchedule.sort((a, b) => a.index - b.index);
        }
    }
    
    function removeError(index: number) {
        errorSchedule = errorSchedule.filter((_, i) => i !== index);
    }
    
    function clearErrors() {
        errorSchedule = [];
        selectedScenario = "";
    }
</script>

<div class="dry-run-test">
    <h5 class="section-title">Progress & Error Testing</h5>
    
    <!-- Compact delay control -->
    <div class="delay-control">
        <span class="delay-label">Delay:</span>
        <input 
            type="range" 
            min="100" 
            max="5000" 
            step="100" 
            bind:value={processingDelay} 
            disabled={isTestRunning}
            class="delay-slider"
        >
        <span class="delay-value">{processingDelay}ms</span>
    </div>
    
    <!-- Compact scenario selection -->
    <div class="scenarios-line">
        <span class="scenarios-label">Scenarios:</span>
        <div class="scenario-buttons">
            {#each Object.entries(scenarios) as [key, scenario]}
                <button 
                    class="scenario-btn"
                    class:selected={selectedScenario === key}
                    on:click={() => loadScenario(key)} 
                    disabled={isTestRunning}
                >
                    {scenario.name}
                </button>
            {/each}
        </div>
    </div>
    
    <!-- Error Schedule -->
    <div class="error-schedule-section">
        <div class="schedule-header">
            <span class="schedule-title">Error Schedule:</span>
            <button 
                class="clear-btn"
                on:click={clearErrors}
                disabled={isTestRunning || errorSchedule.length === 0}
            >
                Clear
            </button>
        </div>
        
        {#if errorSchedule.length > 0}
            <div class="error-list">
                {#each errorSchedule as error, i}
                    <div class="error-item">
                        <span class="error-info">
                            File #{error.index + 1}: 
                            <span class="error-type" 
                                  class:abort-all={error.type === 'abort_all'}
                                  class:error={error.type === 'error'}>
                                {internalToDisplay[error.type] || error.type}
                            </span>
                        </span>
                        <button 
                            class="remove-btn"
                            on:click={() => removeError(i)}
                            disabled={isTestRunning}
                        >
                            ×
                        </button>
                    </div>
                {/each}
            </div>
        {:else}
            <p class="no-errors">No errors scheduled</p>
        {/if}
        
        <!-- Add new error -->
        <div class="add-error-section">
            <input 
                type="number" 
                placeholder="#" 
                min="1"
                bind:value={newErrorIndex}
                disabled={isTestRunning}
                class="index-input"
            >
            <select 
                bind:value={newErrorType}
                disabled={isTestRunning}
                class="type-select"
            >
                <option value="error">Error</option>
                <option value="abort_task">AbortTask</option>
                <option value="abort_all">AbortAll</option>
            </select>
            <button 
                class="add-btn"
                on:click={addError}
                disabled={isTestRunning}
            >
                Add
            </button>
        </div>
    </div>
    
    <!-- Runtime Controls -->
    <div class="runtime-controls">
        <button 
            class="control-btn primary"
            on:click={configureDryRun} 
            disabled={isProcessing || isTestRunning}
        >
            Configure & Enable Dry Run
        </button>
    </div>
    
    <!-- Manual Error Injection -->
    <div class="injection-controls">
        <span class="injection-label">Manual Injection:</span>
        <button 
            class="inject-btn error"
            on:click={() => injectError('error')}
            disabled={!isTestRunning || !isProcessing}
        >
            Inject Error
        </button>
        
        <button 
            class="inject-btn task"
            on:click={() => injectError('abort_task')}
            disabled={!isTestRunning || !isProcessing}
        >
            Inject AbortTask
        </button>
        
        <button 
            class="inject-btn critical"
            on:click={() => injectError('abort_all')}
            disabled={!isTestRunning || !isProcessing}
        >
            Inject AbortAll
        </button>
    </div>
</div>

<style>
    .dry-run-test {
        padding: 0.5rem;
        background: rgba(255, 255, 255, 0.03);
        border-radius: 0.375rem;
        margin-bottom: 0.75rem;
    }
    
    .section-title {
        font-size: 0.75rem;
        font-weight: 600;
        margin-bottom: 0.75rem;
        color: rgba(255, 255, 255, 0.9);
    }
    
    /* Compact delay control */
    .delay-control {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        margin-bottom: 0.5rem;
        font-size: 0.625rem;
    }
    
    .delay-label {
        font-weight: 600;
        opacity: 0.8;
    }
    
    .delay-slider {
        flex: 1;
        height: 6px;
        cursor: pointer;
        -webkit-appearance: none;
        appearance: none;
        background: rgba(255, 255, 255, 0.2);
        border-radius: 3px;
        outline: none;
    }
    
    .delay-slider::-webkit-slider-thumb {
        -webkit-appearance: none;
        appearance: none;
        width: 14px;
        height: 14px;
        background: var(--primary-color);
        border-radius: 50%;
        cursor: pointer;
    }
    
    .delay-slider::-moz-range-thumb {
        width: 14px;
        height: 14px;
        background: var(--primary-color);
        border-radius: 50%;
        cursor: pointer;
        border: none;
    }
    
    .delay-value {
        min-width: 45px;
        text-align: right;
        opacity: 0.8;
    }
    
    /* Compact scenarios */
    .scenarios-line {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        margin-bottom: 0.5rem;
        font-size: 0.625rem;
    }
    
    .scenarios-label {
        font-weight: 600;
        opacity: 0.8;
    }
    
    .scenario-buttons {
        display: flex;
        gap: 0.25rem;
    }
    
    .scenario-btn {
        padding: 0.125rem 0.375rem;
        font-size: 0.625rem;
        background: rgba(255, 255, 255, 0.1);
        border: 1px solid rgba(255, 255, 255, 0.2);
        border-radius: 0.25rem;
        color: rgba(255, 255, 255, 0.8);
        cursor: pointer;
        transition: all 0.2s;
    }
    
    .scenario-btn:hover:not(:disabled) {
        background: rgba(255, 255, 255, 0.15);
        color: white;
    }
    
    .scenario-btn.selected {
        background: var(--primary-color-muted);
        border-color: var(--primary-color);
        color: white;
    }
    
    .scenario-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
    
    /* Error schedule section */
    .error-schedule-section {
        margin: 0.5rem 0;
        padding: 0.5rem;
        background: rgba(0, 0, 0, 0.2);
        border-radius: 0.25rem;
    }
    
    .schedule-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 0.375rem;
    }
    
    .schedule-title {
        font-size: 0.625rem;
        font-weight: 600;
        opacity: 0.8;
    }
    
    .clear-btn {
        padding: 0.125rem 0.375rem;
        font-size: 0.625rem;
        background: rgba(255, 0, 0, 0.2);
        border: 1px solid rgba(255, 0, 0, 0.3);
        border-radius: 0.25rem;
        color: rgba(255, 255, 255, 0.8);
        cursor: pointer;
        transition: all 0.2s;
    }
    
    .clear-btn:hover:not(:disabled) {
        background: rgba(255, 0, 0, 0.3);
    }
    
    .clear-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
    
    .error-list {
        display: flex;
        flex-direction: column;
        gap: 0.125rem;
        margin-bottom: 0.5rem;
        max-height: 100px;
        overflow-y: auto;
    }
    
    .error-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.25rem 0.375rem;
        background: rgba(255, 255, 255, 0.05);
        border-radius: 0.25rem;
        font-size: 0.625rem;
    }
    
    .error-info {
        display: flex;
        align-items: center;
        gap: 0.375rem;
    }
    
    .error-type {
        font-weight: 600;
        font-family: monospace;
        color: hsl(45, 100%, 60%);
    }
    
    .error-type.abort-all {
        color: hsl(0, 100%, 60%);
    }
    
    .error-type.error {
        color: hsl(200, 100%, 60%);
    }
    
    .remove-btn {
        width: 18px;
        height: 18px;
        padding: 0;
        background: rgba(255, 0, 0, 0.2);
        border: 1px solid rgba(255, 0, 0, 0.3);
        border-radius: 0.25rem;
        color: rgba(255, 255, 255, 0.8);
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 0.875rem;
        line-height: 1;
        transition: all 0.2s;
    }
    
    .remove-btn:hover:not(:disabled) {
        background: rgba(255, 0, 0, 0.4);
    }
    
    .no-errors {
        font-size: 0.625rem;
        opacity: 0.5;
        font-style: italic;
        margin: 0.25rem 0;
    }
    
    .add-error-section {
        display: flex;
        gap: 0.25rem;
        align-items: center;
    }
    
    .index-input {
        width: 50px;
        padding: 0.125rem 0.25rem;
        font-size: 0.625rem;
        background: rgba(255, 255, 255, 0.1);
        border: 1px solid rgba(255, 255, 255, 0.2);
        border-radius: 0.25rem;
        color: white;
    }
    
    .type-select {
        flex: 1;
        padding: 0.125rem 0.25rem;
        font-size: 0.625rem;
        background: rgba(255, 255, 255, 0.1);
        border: 1px solid rgba(255, 255, 255, 0.2);
        border-radius: 0.25rem;
        color: white;
    }
    
    .type-select option {
        background: #1a1a1a;
    }
    
    .add-btn {
        padding: 0.125rem 0.5rem;
        font-size: 0.625rem;
        background: var(--primary-color-muted);
        border: 1px solid var(--primary-color);
        border-radius: 0.25rem;
        color: white;
        cursor: pointer;
        transition: all 0.2s;
    }
    
    .add-btn:hover:not(:disabled) {
        background: var(--primary-color);
    }
    
    .add-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
    
    /* Runtime controls */
    .runtime-controls {
        display: flex;
        gap: 0.375rem;
        margin: 0.75rem 0 0.5rem;
    }
    
    .control-btn {
        flex: 1;
        padding: 0.375rem 0.75rem;
        font-size: 0.625rem;
        font-weight: 600;
        border-radius: 0.25rem;
        border: 1px solid;
        cursor: pointer;
        transition: all 0.2s;
    }
    
    .control-btn.primary {
        background: var(--primary-color-muted);
        border-color: var(--primary-color);
        color: white;
        position: relative;
        overflow: hidden;
    }
    
    .control-btn.primary:hover:not(:disabled) {
        background: var(--primary-color);
    }
    
    .control-btn.primary:active:not(:disabled) {
        background: var(--primary-color-muted);
        transform: scale(0.98);
    }
    
    .control-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
    
    /* Manual injection controls */
    .injection-controls {
        display: flex;
        align-items: center;
        gap: 0.375rem;
        padding: 0.375rem 0.5rem;
        background: rgba(255, 165, 0, 0.05);
        border: 1px solid rgba(255, 165, 0, 0.2);
        border-radius: 0.25rem;
    }
    
    .injection-label {
        font-size: 0.625rem;
        font-weight: 600;
        color: rgba(255, 165, 0, 0.9);
    }
    
    .inject-btn {
        padding: 0.25rem 0.5rem;
        font-size: 0.625rem;
        font-weight: 600;
        border-radius: 0.25rem;
        border: 1px solid;
        cursor: pointer;
        transition: all 0.2s;
    }
    
    .inject-btn.error {
        background: rgba(100, 150, 255, 0.2);
        border-color: rgba(100, 150, 255, 0.4);
        color: white;
    }
    
    .inject-btn.error:hover:not(:disabled) {
        background: rgba(100, 150, 255, 0.3);
    }
    
    .inject-btn.task {
        background: rgba(255, 165, 0, 0.2);
        border-color: rgba(255, 165, 0, 0.4);
        color: white;
    }
    
    .inject-btn.task:hover:not(:disabled) {
        background: rgba(255, 165, 0, 0.3);
    }
    
    .inject-btn.critical {
        background: rgba(255, 0, 0, 0.2);
        border-color: rgba(255, 0, 0, 0.4);
        color: white;
    }
    
    .inject-btn.critical:hover:not(:disabled) {
        background: rgba(255, 0, 0, 0.3);
    }
    
    .inject-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
</style>