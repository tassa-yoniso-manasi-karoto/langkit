<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { EventsOn } from '../../wailsjs/runtime/runtime';

    export let taskId: string;

    let progress = 0;
    let description = 'Starting download...';
    let visible = true;

    let unlisten: () => void;

    onMount(() => {
        unlisten = EventsOn(`${taskId}-progress`, (data: any) => {
            progress = data.progress;
            description = data.description;
            if (progress >= 100) {
                setTimeout(() => {
                    visible = false;
                }, 1000);
            }
        });
    });

    onDestroy(() => {
        if (unlisten) {
            unlisten();
        }
    });
</script>

{#if visible}
    <div class="w-full bg-gray-700 rounded-full h-2.5 my-2">
        <div class="bg-primary h-2.5 rounded-full" style="width: {progress}%"></div>
    </div>
    <div class="text-xs text-gray-400 text-center">{description}</div>
{/if}
