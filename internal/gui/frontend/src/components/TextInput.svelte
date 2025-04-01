<script lang="ts">
    import { twMerge } from 'tailwind-merge';
    import { createEventDispatcher } from 'svelte';

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
    const baseClasses = "form-input focus:outline-none transition-all duration-200";

    // Core styling classes with Tailwind
    const defaultClasses = "rounded-md h-[42px] px-3 text-sm font-medium";

    // Conditional classes based on the component's props
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
    input {
        width: 100%;
        border: 2px solid var(--input-border);
        background-color: var(--input-bg);
        box-shadow: var(--input-shadow);
    }
    
    input:hover {
        background-color: var(--input-bg-hover);
        border-color: var(--input-border-hover);
    }
    
    input:focus {
        background-color: var(--input-bg-focus);
        border-color: var(--input-border-focus);
        box-shadow: var(--input-shadow-focus);
    }
    
    input:active {
        transform: translateY(0) !important;
        transition-duration: 50ms;
    }
</style>