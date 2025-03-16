<script lang="ts">
    import { twMerge } from 'tailwind-merge';
    import { createEventDispatcher, onMount } from 'svelte';

    export let value: string = "";
    export let placeholder: string = "";
    export let minLength: number | undefined;
    export let maxLength: number | undefined;
    export let fullWidth: boolean = true;
    export let center: boolean = false;
    export let className: string = "";

    // Create dispatcher for input events
    const dispatch = createEventDispatcher();

    // Always-applied classes
    const baseClasses = "bg-sky-dark/50 focus:outline-none transition-all duration-200";

    // Same default hover/focus as NumericInput
    const defaultClasses = `
      border-2 border-primary/30 rounded-md h-[42px] px-3 text-sm font-medium
      focus:border-primary focus:ring-2 focus:ring-primary/30
      hover:border-primary/50
    `;

    const conditionalClasses = `${center ? 'text-center' : ''} ${fullWidth ? 'w-full' : ''}`;

    $: inputClasses = twMerge(baseClasses, defaultClasses, conditionalClasses, className);

    // Handle input changes with immediate effect
    function handleInput(event) {
        // Update value and dispatch event
        value = event.target.value;
        dispatch('input', event);
    }
</script>

<input
    type="text"
    bind:value={value}
    placeholder={placeholder}
    minlength={minLength}
    maxlength={maxLength}
    class={inputClasses}
    on:input={handleInput}
    on:change
    on:focus
    on:blur
/>

<style>
    input:active {
        transform: translateY(0) !important;
        transition-duration: 50ms;
    }
</style>
