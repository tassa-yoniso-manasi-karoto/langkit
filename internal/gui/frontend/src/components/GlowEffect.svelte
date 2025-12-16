<script lang="ts">
    import { onMount } from 'svelte';
    import { liteModeStore } from '../lib/stores';

    export let isProcessing = false;
    let blob: HTMLDivElement;

    // Track lite mode for Qt+Windows compatibility
    $: liteMode = $liteModeStore.enabled;
</script>

<div class="glow-container" class:paused={isProcessing}>
    <div bind:this={blob} class="glow-blob" class:paused={isProcessing}></div>
    <!-- Disable backdrop-filter in lite mode to avoid Qt WebEngine flickering on Windows -->
    <div class="glow-blur" class:lite={liteMode}></div>
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
        width: var(--style-glow-size, 26vmax);
        height: var(--style-glow-size, 26vmax);
        /* Adjust position to account for container offset */
        left: var(--style-glow-position-x, 78vw);
        bottom: var(--style-glow-position-y, -10vw);
        border-radius: 50%;
        background: var(--style-glow-gradient, linear-gradient(
            45deg,
            rgba(159, 110, 247, 0.4),
            rgba(190, 120, 255, 0.7),
            rgba(255, 100, 255, 0.6),
            rgba(255, 30, 255, 0.5)
        ));
        opacity: var(--style-glow-opacity, 0.6);
        filter: blur(var(--style-glow-blur, 100px));
        animation: pulse var(--style-glow-animation-speed, 10s) ease-in-out infinite;
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

    /* Qt WebEngine on Windows: disable backdrop-filter to prevent flickering */
    .glow-blur.lite {
        backdrop-filter: none;
        -webkit-backdrop-filter: none;
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
            transform: scale(var(--style-glow-animation-scale, 1.2));
        }
    }
    .paused {
        animation-play-state: paused !important;
    }

</style>