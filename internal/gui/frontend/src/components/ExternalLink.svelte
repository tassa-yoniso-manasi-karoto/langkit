<script>
    import { OpenURL } from '../../wailsjs/go/gui/App';

    export let href = '';
    export let className = '';
    export let preserveColors = false;

    const handleClick = (event) => {
        event.preventDefault();
        OpenURL(href);
    };
    
    // Only include the color classes if preserveColors is false
    const baseClasses = preserveColors 
        ? 'underline cursor-pointer' 
        : 'text-sky-500 hover:text-sky-600 underline cursor-pointer';
</script>

<a 
    href={href}
    on:click={handleClick}
    class={`${baseClasses} ${className}`}>
    <slot />
</a>