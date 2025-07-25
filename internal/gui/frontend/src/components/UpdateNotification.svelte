<script>
    import { OpenURL } from '../api/services/system';
    
    export let href = '';
    
    const handleClick = (event) => {
        event.preventDefault();
        OpenURL(href);
    };
</script>

<a 
    href={href}
    on:click={handleClick}
    class="ml-2 font-semibold cursor-pointer text-orange-500 tracking-tighter blinking-update"
    target="_blank"
    rel="noopener noreferrer"
    title="Click to view latest release">
    <slot />
</a>

<style>
    @keyframes fadeBlink {
        0% {
            color: #f97316; /* orange-500 */
        }
        50% {
            color: #dc2626; /* red-600 */
        }
        100% {
            color: #f97316;
        }
    }
    .blinking-update {
        animation: fadeBlink 1.5s linear infinite;
    }
</style>