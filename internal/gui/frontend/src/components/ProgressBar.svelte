<script lang="ts">
    import { Progressbar } from 'flowbite-svelte';
    import { sineOut } from 'svelte/easing';
    
    export let progress = 0;
    export let current = 0;
    export let total = 0;
    export let speed = '';
    export let currentFile = '';
    export let operation = 'Downloading';
    
    $: percentage = Math.min(Math.round(progress), 100).toString();

    function formatFileName(path: string): string {
        const parts = path.split('/');
        return parts[parts.length - 1];
    }
</script>

<div class="bg-[#252525] rounded-lg p-3 shadow-lg">
    <div class="space-y-3">
        <div class="flex justify-between items-start text-sm">
            <div class="space-y-1 min-w-0 flex-1 mr-4">
                <div class="flex items-center gap-2">
                    <span class="text-accent font-medium">
                        {operation}
                    </span>
                    <span class="text-[#7cafc2] px-2 py-0.5 rounded bg-[#7cafc2]/10 text-xs font-mono">
                        {current}/{total}
                    </span>
                </div>
                <div class="text-[#d4d4d4] truncate" title={currentFile}>
                    {formatFileName(currentFile)}
                </div>
            </div>
            <div class="flex items-center gap-3 shrink-0">
                <span class="text-[#99c794] px-2 py-0.5 rounded bg-[#99c794]/10 font-mono">
                    {percentage}%
                </span>
                {#if speed}
                    <span class="text-[#7cafc2] px-2 py-0.5 rounded bg-[#7cafc2]/10 font-mono">
                        {speed}
                    </span>
                {/if}
            </div>
        </div>
        
        <Progressbar
            progress={percentage}
            size="h-1.5"
            color="green"
            class="custom-progress"
        />
    </div>
</div>

<style>
    :global(.custom-progress) {
        @apply bg-[#1e1e1e];
        border-radius: 4px;
        overflow: hidden;
    }
    
    :global(.custom-progress div) {
        @apply bg-[#99c794];
        box-shadow: 0 0 10px rgba(153, 199, 148, 0.3);
        transition: width 300ms ease-out;
    }
</style>