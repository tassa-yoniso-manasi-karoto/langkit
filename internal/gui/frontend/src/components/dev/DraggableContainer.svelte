<script lang="ts">
    import { createEventDispatcher } from 'svelte';
    
    // Props
    export let position = { x: 20, y: 20 };
    export let isDragging = false;
    export let handleSelector: string = ''; // CSS selector for draggable areas
    export let zIndex: string = ''; // Optional z-index CSS variable
    
    // Dispatch events
    const dispatch = createEventDispatcher();
    
    // Internal dragging state
    let startX = 0;
    let startY = 0;
    let posX = position.x;
    let posY = position.y;
    
    // Handle dragging for both the icon and expanded dashboard
    function handleMouseDown(event: MouseEvent) {
        // If handleSelector is provided, check if the click is on a draggable area
        if (handleSelector) {
            const target = event.target as HTMLElement;
            const draggableArea = target.closest(handleSelector);
            if (!draggableArea) {
                return; // Not clicking on a draggable area
            }
        }
        
        // Make sure the event isn't coming from buttons with stopPropagation
        const target = event.target as HTMLElement;

        // Check if we have an explicit stopPropagation marker
        if (target.hasAttribute('on:mousedown|stopPropagation')) {
            return;
        }

        // Start dragging
        isDragging = true;
        startX = event.clientX;
        startY = event.clientY;

        // Add events to window to track cursor even when outside element
        window.addEventListener('mousemove', handleMouseMove);
        window.addEventListener('mouseup', handleMouseUp);

        // Prevent default behavior
        event.preventDefault();
        event.stopPropagation();
    }
    
    function handleMouseMove(event: MouseEvent) {
        if (!isDragging) return;

        // Calculate movement
        const dx = event.clientX - startX;
        const dy = event.clientY - startY;

        // Update position and reset drag start point for next move
        posX += dx;
        posY += dy;
        startX = event.clientX;
        startY = event.clientY;

        // Keep on screen (simple boundaries)
        if (posX < 0) posX = 0;
        if (posY < 0) posY = 0;
        if (posX > window.innerWidth - 50) posX = window.innerWidth - 50;
        if (posY > window.innerHeight - 50) posY = window.innerHeight - 50;

        // Dispatch position change event
        dispatch('positionChange', { x: posX, y: posY });

        // Prevent defaults
        event.preventDefault();
        event.stopPropagation();
    }
    
    function handleMouseUp(event: MouseEvent | null) {
        isDragging = false;
        window.removeEventListener('mousemove', handleMouseMove);
        window.removeEventListener('mouseup', handleMouseUp);

        // Prevent defaults if we were actually dragging
        if (event) {
            event.preventDefault();
            event.stopPropagation();
        }
        
        // Dispatch drag end event
        dispatch('dragEnd', { isDragging: false });
    }
    
    // Update internal position when prop changes
    $: {
        posX = position.x;
        posY = position.y;
    }
</script>

<div
    on:mousedown={handleMouseDown}
    style="position: fixed; left: {posX}px; top: {posY}px; {zIndex ? `z-index: var(${zIndex});` : ''}"
    class:dragging={isDragging}
>
    <slot />
</div>

<style>
    .dragging {
        user-select: none;
        -webkit-user-select: none;
    }
</style>