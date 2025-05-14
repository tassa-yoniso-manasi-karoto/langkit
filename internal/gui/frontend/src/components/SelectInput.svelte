<script lang="ts">
    export let value: any;
    export let disabled: boolean = false;
    export let className: string = "";
</script>

<select
    {disabled}
    class={className}
    bind:value
>
    <slot></slot>
</select>

<style>
    /* Most styling now comes from app.css global rules */
    /* We just need to ensure the select behaves properly */
    select {
        /* Ensure appearance is correctly set for dropdown arrows */
        -webkit-appearance: menulist;
        -moz-appearance: menulist;
        appearance: menulist;
    }
</style>