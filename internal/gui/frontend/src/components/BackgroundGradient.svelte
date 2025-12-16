<script lang="ts">
    // This component creates a seamless transition for the glow effect
    // Uses multiple offset gradients to eliminate banding through interference
</script>

<div class="background-gradient"></div>

<style>
    .background-gradient {
        position: fixed;
        inset: 0;
        width: 100vw;
        height: 100vh;
        /*
         * Anti-banding technique: Layer 3 identical gradients with slight position offsets.
         * The banding patterns from each layer don't align, creating destructive interference
         * that produces a smoother visual result. Each layer is at ~33% opacity to sum to ~100%.
         */
        background:
            /* Layer 1: Original position */
            radial-gradient(
                circle at 10% 90%,
                rgba(120, 81, 169, 0.045) 0%,
                rgba(140, 95, 208, 0.038) 12%,
                rgba(159, 110, 247, 0.033) 25%,
                rgba(207, 110, 223, 0.025) 37%,
                rgba(255, 110, 199, 0.018) 50%,
                rgba(207, 110, 223, 0.012) 62%,
                rgba(159, 110, 247, 0.007) 75%,
                rgba(100, 80, 150, 0.003) 87%,
                transparent 100%
            ),
            /* Layer 2: Offset - blurs out banding in mid-range */
            radial-gradient(
                circle at 12% 88%,
                rgba(120, 81, 169, 0.02) 0%,
                rgba(159, 110, 247, 0.015) 26%,
                rgba(255, 110, 199, 0.008) 51%,
                rgba(159, 110, 247, 0.003) 76%,
                transparent 100%
            ),
            /* Layer 3: Offset opposite - blurs out banding */
            radial-gradient(
                circle at 8% 92%,
                rgba(120, 81, 169, 0.02) 0%,
                rgba(159, 110, 247, 0.015) 24%,
                rgba(255, 110, 199, 0.008) 49%,
                rgba(159, 110, 247, 0.003) 74%,
                transparent 100%
            );
        pointer-events: none;
        z-index: 0;
        overflow: hidden;
        contain: strict;
        will-change: opacity;
    }
</style>