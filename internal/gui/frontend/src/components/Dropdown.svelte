<!-- Dropdown.svelte -->
<script lang="ts">
    export let options: Array<any> = [];
    export let value: string = '';
    export let label: string = '';
    export let optionKey: string = '';  // Field to use as value
    export let optionLabel: string = ''; // Field to use as label
    import { createEventDispatcher, onMount } from 'svelte';
    const dispatch = createEventDispatcher();

    // If optionKey/optionLabel are provided, use them to get values/labels from objects
    // Otherwise, treat options as simple strings
    function getValue(option: any): string {
        if (optionKey && typeof option === 'object') {
            return option[optionKey];
        }
        return option;
    }

    function getLabel(option: any): string {
        if (optionLabel && typeof option === 'object') {
            return option[optionLabel] || option[optionKey] || option;
        }
        return option;
    }

    function handleSelect(event: Event) {
        const target = event.target as HTMLSelectElement;
        value = target.value;
        dispatch('change', target.value);
    }

    // Reactive statement to ensure default selection
    $: if (options.length > 0 && (!value || !options.some(opt => getValue(opt) === value))) {
        const defaultValue = getValue(options[0]);
        if (defaultValue !== value) {
            value = defaultValue;
            dispatch('change', defaultValue);
        }
    }
</script>

<select
    bind:value
    on:change={handleSelect}
    class="w-full bg-sky-dark/50 border border-accent/30 rounded px-1 py-1
           focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
           transition-colors duration-200 text-sm font-medium"
>
    {#each options as option}
        <option value={getValue(option)}>
            {getLabel(option)}
        </option>
    {/each}
</select>