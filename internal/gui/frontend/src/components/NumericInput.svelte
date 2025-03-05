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
    const baseClasses = "bg-sky-dark/50 focus:outline-none transition-all duration-200";
    
    // Default styling classes that can be overridden
    const defaultClasses = "border-2 border-primary/30 rounded-md h-[42px] px-3 text-sm font-medium focus:border-primary focus:ring-2 focus:ring-primary/30 hover:border-primary/50";
    
    // Conditional classes based on the component's props
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
