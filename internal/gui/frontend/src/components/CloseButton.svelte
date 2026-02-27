<script lang="ts">
    import { createEventDispatcher } from 'svelte';

    export let size: 'xs' | 'sm' | 'md' = 'md';
    export let title: string = '';

    var dispatch = createEventDispatcher<{ click: void }>();
</script>

<button
    class="flex items-center justify-center rounded-full
           bg-transparent text-white/40 focus:outline-none close-btn
           {size === 'xs' ? 'w-6 h-6' : size === 'sm' ? 'w-8 h-8' : 'w-10 h-10'}"
    {title}
    on:click={() => dispatch('click')}
>
    <span class="material-icons {size === 'xs' ? 'text-[16px]' : size === 'sm' ? 'text-base' : ''}">close</span>
</button>

<style>
    .close-btn {
        transition: color 200ms ease,
                    transform 300ms cubic-bezier(0.7, 0, 0.3, 1);
    }
    .close-btn:hover {
        color: hsl(0, 72%, 64%);
        transform: scale(1.2);
    }
</style>
