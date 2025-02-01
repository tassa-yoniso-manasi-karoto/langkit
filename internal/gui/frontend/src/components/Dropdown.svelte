<script lang="ts">
    export let options: string[];
    export let defaultOption: string;
    export let value: string;
    export let label: string;
    
    defaultOption = options[0]
</script>

<select
    bind:value={defaultOption}
    class="w-full bg-sky-dark/50 border border-accent/30 rounded px-3 py-1.5
           focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent
           transition-colors duration-200 text-sm font-medium
           appearance-none cursor-pointer
           bg-[url('data:image/svg+xml;charset=US-ASCII,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%2224%22%20height%3D%2224%22%20viewBox%3D%220%200%2024%2024%22%20fill%3D%22none%22%20stroke%3D%22%239f6ef7%22%20stroke-width%3D%222%22%20stroke-linecap%3D%22round%22%20stroke-linejoin%3D%22round%22%3E%3Cpolyline%20points%3D%226%209%2012%2015%2018%209%22%3E%3C%2Fpolyline%3E%3C%2Fsvg%3E')]
           bg-[length:1.25rem]
           bg-[right_0.5rem_center]
           bg-no-repeat"
>
    {#each options as option}
        <option value={option}>{option}</option>
    {/each}
</select>