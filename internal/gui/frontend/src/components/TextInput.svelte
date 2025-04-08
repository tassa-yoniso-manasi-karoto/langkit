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
    export let invalid: boolean = false;
    export let errorMessage: string = '';

    // Create dispatcher for input events
    const dispatch = createEventDispatcher();

    // Always-applied classes
    const baseClasses = "form-input focus:outline-none transition-all duration-200";

    // Core styling classes with Tailwind
    const defaultClasses = "rounded-md h-[42px] px-3 text-sm font-medium";

    // Conditional classes based on the component's props
    const conditionalClasses = `${center ? 'text-center' : ''} ${fullWidth ? 'w-full' : ''}`;

    $: inputClasses = twMerge(
        baseClasses,
        defaultClasses,
        conditionalClasses,
        className,
        invalid ? 'border-error-task' : 'border-white/10' // Apply error border if invalid
    );

    // Handle input changes with immediate effect
    function handleInput(event: Event) { // Add type to event
        // Update value and dispatch event
        const target = event.target as HTMLInputElement; // Cast target
        value = target.value;
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
{#if invalid && errorMessage}
    <p class="text-xs text-error-task mt-1">{errorMessage}</p>
{/if}

<style>
    input {
        width: 100%;
        border: 2px solid var(--input-border);
        background-color: hsla(var(--input-bg), 0.4);
        box-shadow: var(--input-shadow);
    }

    input:hover {
        background-color: hsla(var(--input-bg-hover), 0.45);
        border-color: var(--input-border-hover);
    }

    input:focus {
        background-color: hsla(var(--input-bg-focus), 0.5);
        /* Don't override border color on focus if invalid */
        border-color: var(--input-border-focus);
        box-shadow: var(--input-shadow-focus);
    }
    
    /* Ensure invalid border takes precedence on focus */
    input.border-error-task:focus {
        border-color: var(--error-task-color);
    }
    
    input:active {
        transform: translateY(0) !important;
        transition-duration: 50ms;
    }
</style>