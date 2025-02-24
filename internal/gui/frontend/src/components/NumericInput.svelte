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

    function handleKeydown(e: KeyboardEvent) {
        // Only allow backspace if there's more than one character
        if (e.key === 'Backspace' && (e.target as HTMLInputElement).value.length <= 1) {
            // If only one character left, don't allow deletion
            // This prevents empty values without surprising the user
            e.preventDefault();
        }
    }
    
    // Base classes that will always be applied
    const baseClasses = "bg-sky-dark/50 focus:outline-none transition-all duration-200";
    
    // Default styling classes that can be overridden
    const defaultClasses = "border-2 border-accent/30 rounded-md h-[42px] px-3 text-sm font-medium focus:border-accent focus:ring-2 focus:ring-accent/30 hover:border-accent/50";
    
    // Conditional classes
    const conditionalClasses = `${center ? 'text-center' : ''} ${fullWidth ? 'w-full' : ''}`;
    
    // Merge classes with tailwind-merge to properly handle class conflicts
    $: inputClasses = twMerge(baseClasses, defaultClasses, conditionalClasses, className);
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