<script lang="ts">
    // Prototype status (would be connected to actual processing later)
    let isProcessing = false;
    let progress = 0;
    let status = "";
</script>

{#if isProcessing}
    <div class="status-container" transition:fade>
        <div class="progress-bar">
            <div class="progress" style="width: {progress}%"></div>
        </div>
        <p class="status-text">{status}</p>
    </div>
{/if}

<style>
    .status-container {
        margin-top: 2rem;
        padding: 1rem;
        background-color: #232342;
        border-radius: 8px;
    }

    .progress-bar {
        width: 100%;
        height: 8px;
        background-color: #1a1a2e;
        border-radius: 4px;
        overflow: hidden;
    }

    .progress {
        height: 100%;
        background-color: #64dfdf;
        transition: width 0.3s ease;
    }

    .status-text {
        text-align: center;
        margin: 0.5rem 0 0;
        color: #64dfdf;
    }
</style>