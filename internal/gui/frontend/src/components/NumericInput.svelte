<script lang="ts">
    import { twMerge } from 'tailwind-merge';
    
    export let value: number;
    export let min: number | undefined = undefined;
    export let max: number | undefined = undefined;
    export let placeholder: string = "";
    export let step: number | string = 1;
    export let fullWidth: boolean = true;
    export let center: boolean = true;
    export let className: string = "";
    export let invalid: boolean = false;
    export let errorMessage: string = '';
    
    function handleKeydown(e: KeyboardEvent) {
        const input = e.target as HTMLInputElement;
        // Remove minus signs from the value before checking its length.
        const digits = input.value.replace(/-/g, '');
        // For both Backspace and Delete: if there's only one digit left 
        // (and the value isn't just a lone minus sign), prevent deletion.
        if ((e.key === 'Backspace' || e.key === 'Delete') && digits.length <= 1 && input.value !== '-') {
            e.preventDefault();
        }
    }
    
    // Base classes that will always be applied
    const baseClasses = "form-input focus:outline-none transition-all duration-200";
    
    // Default styling classes that can be overridden
    const defaultClasses = "rounded-md h-[42px] px-3 text-sm font-medium";
    
    // Conditional classes based on the component's props
    const conditionalClasses = `${center ? 'text-center' : ''} ${fullWidth ? 'w-full' : ''}`;
    
    // Merge classes with tailwind-merge to properly handle class conflicts
    // Add invalid class conditionally
    $: inputClasses = twMerge(
        baseClasses,
        defaultClasses,
        conditionalClasses,
        className,
        invalid ? 'border-error-task' : 'border-white/10' // Apply error border if invalid
    );
</script>

<input 
    type="number" 
    bind:value={value}
    min={min}
    max={max}
    placeholder={placeholder}
    step={step}
    on:keydown={handleKeydown}
    class={inputClasses}
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
    
    /* Style number input arrows */
    input::-webkit-inner-spin-button,
    input::-webkit-outer-spin-button {
        opacity: 1;
        height: 24px;
        margin-right: 2px;
    }
</style>