<script lang="ts">
    import { onMount } from 'svelte';
    
    export let isProcessing = false;
    let blob: HTMLDivElement;
</script>

<div class="glow-container" class:paused={isProcessing}>
    <div bind:this={blob} class="glow-blob" class:paused={isProcessing}></div>
    <div class="glow-blur"></div>
</div>


<style>
    .glow-container {
        position: fixed;
        /* Extend beyond viewport in all directions */
        top: -5vh;
        left: -5vw;
        right: -5vw;
        bottom: -5vh;
        /* Make it larger than viewport to cover animation shifts */
        width: 110vw;
        height: 110vh;
        pointer-events: none;
        z-index: 0;
        animation: float 6s ease-in-out infinite;
        will-change: transform;
        transform: translate3d(0, 0, 0);
        overflow: hidden;
        contain: content;
    }

    .glow-blob {
        position: absolute;
        width: 26vmax;
        height: 26vmax;
        /* Adjust position to account for container offset */
        left: calc(73vw + 5vw);
        bottom: calc(-15vw + 5vh);
        border-radius: 50%;
        background: linear-gradient(
            45deg,
            rgba(159, 110, 247, 0.4),
            rgba(190, 120, 255, 0.7),
            rgba(255, 100, 255, 0.6),
            rgba(255, 30, 255, 0.5)
        );
        opacity: 0,6;
        filter: blur(100px);
        animation: pulse 10s ease-in-out infinite;
        transform-origin: center center;
        will-change: transform;
        contain: strict;
    }

    .glow-blur {
        height: 100%;
        width: 100%;
        position: fixed;
        inset: 0;
        z-index: 1;
        backdrop-filter: blur(150px);
        -webkit-backdrop-filter: blur(150px);
        pointer-events: none;
        contain: strict;
    }

    @keyframes float {
        0%, 100% {
            transform: translateY(2vw);
        }
        50% {
            transform: translateY(-2.5vw);
        }
    }

    @keyframes pulse {
        0%, 100% {
            transform: scale(1);
        }
        50% {
            transform: scale(1.2);
        }
    }
    .paused {
        animation-play-state: paused !important;
    }

</style>