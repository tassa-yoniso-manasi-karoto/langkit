<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { wsClient } from '../ws/client';

    export let taskId: string;

    let progress = 0;
    let description = 'Starting download...';
    let visible = true;

    let progressHandler: ((data: any) => void) | null = null;

    onMount(() => {
        progressHandler = (data: any) => {
        	progress = data.progress;
            description = data.description;
            if (progress >= 100) {
                setTimeout(() => {
                    visible = false;
                }, 1000);
            }
        };
        
        // Convert taskId to WebSocket event pattern
        // If taskId contains "-download-progress", convert to new pattern
        const eventName = taskId.includes('-download-progress') 
            ? `download.${taskId.replace('-download-progress', '')}.progress`
            : taskId;
            
        wsClient.on(eventName, progressHandler);
    });

    onDestroy(() => {
        if (progressHandler) {
            const eventName = taskId.includes('-download-progress') 
                ? `download.${taskId.replace('-download-progress', '')}.progress`
                : taskId;
            wsClient.off(eventName, progressHandler);
        }
    });
</script>

{#if visible}
    <div class="w-full bg-gray-700 rounded-full h-2.5 my-2">
        <div class="bg-primary h-2.5 rounded-full" style="width: {progress}%"></div>
    </div>
    <div class="text-xs text-gray-400 text-center">{description}</div>
{/if}
