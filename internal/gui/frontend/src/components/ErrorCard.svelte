<script lang="ts">
    import { slide } from 'svelte/transition';
    import { cubicOut } from 'svelte/easing';
    
    export let message: string;
    export let show: boolean = false;
</script>

{#if show}
    <div class="overflow-hidden">
        <div class="mx-6 px-4 py-3 rounded-b-xl bg-red-500/10 border border-t-0 border-red-500/20"
             style="margin-top: -1px"
             in:slide={{ duration: 400, easing: cubicOut }}
             out:slide={{ duration: 300, easing: cubicOut }}>
            <p class="text-sm text-red-200/80">
                {@html message}
            </p>
        </div>
    </div>
{/if}