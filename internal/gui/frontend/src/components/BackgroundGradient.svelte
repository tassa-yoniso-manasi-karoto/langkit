<script lang="ts">
    // This component creates a seamless transition for the glow effect
</script>

<div class="background-gradient"></div>

<style>
    .background-gradient {
        position: fixed;
        inset: 0;
        width: 100vw;
        height: 100vh;
        background: var(--style-background-gradient, radial-gradient(
            circle at 10% 90%,
            rgba(120, 81, 169, 0.2) 0%, 
            rgba(159, 110, 247, 0.15) 25%,
            rgba(255, 110, 199, 0.08) 50%,
            rgba(159, 110, 247, 0.03) 75%,
            rgba(36, 36, 36, 0) 100%
        ));
        pointer-events: none;
        z-index: 0;
        overflow: hidden;
        contain: strict;
        will-change: opacity;
    }
</style>