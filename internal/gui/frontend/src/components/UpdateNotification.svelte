<script>
    import { OpenURL } from '../../wailsjs/go/gui/App';
    
    export let href = '';
    
    const handleClick = (event) => {
        event.preventDefault();
        OpenURL(href);
    };
</script>

<a 
    href={href}
    on:click={handleClick}
    class="ml-2 font-bold cursor-pointer text-orange-500 underline blinking-update"
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