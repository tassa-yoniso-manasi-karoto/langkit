<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { llmStateStore, type LLMStateChange } from '../lib/stores';
    import { logger } from '../lib/logger';

    // Props
    export let showDetails = false;
    export let compact = false;

    // State
    let llmState: LLMStateChange | null = null;
    let unsubscribe: () => void;

    // Reactive computations
    $: stateColor = getStateColor(llmState?.globalState);
    $: stateIcon = getStateIcon(llmState?.globalState);
    $: stateMessage = getStateMessage(llmState);
    $: providerStatuses = getProviderStatuses(llmState?.providerStatesSnapshot);

    onMount(() => {
        unsubscribe = llmStateStore.subscribe(state => {
            llmState = state;
            logger.trace('llm-status', 'LLM state updated in indicator:', state?.globalState);
        });
    });

    onDestroy(() => {
        if (unsubscribe) {
            unsubscribe();
        }
    });

    function getStateColor(globalState?: string): string {
        switch (globalState) {
            case 'ready': return 'text-green-500';
            case 'initializing': return 'text-yellow-500';
            case 'updating': return 'text-blue-500';
            case 'error': return 'text-red-500';
            case 'uninitialized':
            default: return 'text-gray-500';
        }
    }

    function getStateIcon(globalState?: string): string {
        switch (globalState) {
            case 'ready': return 'check_circle';
            case 'initializing': return 'hourglass_empty';
            case 'updating': return 'sync';
            case 'error': return 'error';
            case 'uninitialized':
            default: return 'radio_button_unchecked';
        }
    }

    function getStateMessage(state: LLMStateChange | null): string {
        if (!state) return 'LLM system not initialized';
        
        switch (state.globalState) {
            case 'ready': 
                const readyCount = Object.values(state.providerStatesSnapshot || {})
                    .filter(p => p.status === 'ready').length;
                return `${readyCount} provider${readyCount !== 1 ? 's' : ''} ready`;
            case 'initializing': 
                return 'Initializing providers...';
            case 'updating': 
                return 'Updating configuration...';
            case 'error': 
                return state.message || 'LLM system error';
            case 'uninitialized':
            default: 
                return 'LLM system not started';
        }
    }

    function getProviderStatuses(providerStates: Record<string, any> | undefined) {
        if (!providerStates) return [];
        
        return Object.entries(providerStates).map(([name, state]) => ({
            name,
            status: state.status,
            error: state.error,
            modelCount: state.models?.length || 0,
            displayName: getProviderDisplayName(name)
        }));
    }

    function getProviderDisplayName(providerName: string): string {
        switch (providerName) {
            case 'openai': return 'OpenAI';
            case 'google': return 'Google';
            case 'openrouter': return 'OpenRouter';
            default: return providerName.charAt(0).toUpperCase() + providerName.slice(1);
        }
    }

    function getProviderStatusColor(status: string): string {
        switch (status) {
            case 'ready': return 'text-green-500';
            case 'initializing_models': return 'text-yellow-500';
            case 'error': return 'text-red-500';
            case 'not_attempted':
            default: return 'text-gray-400';
        }
    }

    function getProviderStatusIcon(status: string): string {
        switch (status) {
            case 'ready': return 'check_circle';
            case 'initializing_models': return 'autorenew';
            case 'error': return 'error_outline';
            case 'not_attempted':
            default: return 'radio_button_unchecked';
        }
    }
</script>

<div class="llm-status-indicator" class:compact>
    <!-- Main Status -->
    <div class="flex items-center gap-2">
        <span class="material-icons text-sm {stateColor} transition-colors duration-200">
            {stateIcon}
        </span>
        
        {#if !compact}
            <span class="text-sm {stateColor} font-medium">
                {stateMessage}
            </span>
        {/if}
    </div>

    <!-- Detailed Provider Status -->
    {#if showDetails && !compact && providerStatuses.length > 0}
        <div class="mt-3 space-y-2 text-xs">
            <div class="font-medium text-gray-600 dark:text-gray-300">
                Provider Status:
            </div>
            
            {#each providerStatuses as provider}
                <div class="flex items-center justify-between gap-2 p-2 bg-gray-50 dark:bg-gray-800 rounded">
                    <div class="flex items-center gap-2">
                        <span class="material-icons text-xs {getProviderStatusColor(provider.status)}">
                            {getProviderStatusIcon(provider.status)}
                        </span>
                        <span class="font-medium">{provider.displayName}</span>
                    </div>
                    
                    <div class="flex items-center gap-2 text-gray-500">
                        {#if provider.status === 'ready'}
                            <span>{provider.modelCount} models</span>
                        {:else if provider.status === 'error' && provider.error}
                            <span class="text-red-500 text-xs truncate max-w-32" title={provider.error}>
                                {provider.error}
                            </span>
                        {:else}
                            <span class="capitalize">{provider.status.replace('_', ' ')}</span>
                        {/if}
                    </div>
                </div>
            {/each}
        </div>
    {/if}

    <!-- Timestamp (for debugging) -->
    {#if showDetails && !compact && llmState?.timestamp}
        <div class="mt-2 text-xs text-gray-400">
            Last update: {new Date(llmState.timestamp).toLocaleTimeString()}
        </div>
    {/if}
</div>

<style>
    .llm-status-indicator {
        @apply select-none;
    }
    
    .llm-status-indicator.compact {
        @apply flex items-center;
    }
    
    .llm-status-indicator :global(.material-icons) {
        font-feature-settings: 'liga';
    }
</style>