<script lang="ts">
    import MemoryTestButton from './MemoryTestButton.svelte';
</script>

<h4>Tests</h4>

<!-- Memory testing section -->
<div class="memory-test-section">
    <h5 class="section-title">Memory Testing</h5>
    <div class="test-controls">
        <MemoryTestButton
            size="medium"
            variant="primary"
        />
        <span class="test-description">Test WASM memory management</span>
    </div>
</div>

<!-- Space for future test additions -->

<style>
    h4 {
        margin: 0 0 12px 0;
        font-size: 13px;
        opacity: 0.9;
    }
    
    .memory-test-section {
        margin-bottom: 16px;
        padding-bottom: 12px;
        border-bottom: 1px solid rgba(255, 255, 255, 0.1);
    }
    
    .section-title {
        font-size: 0.75rem;
        font-weight: 600;
        margin-bottom: 0.5rem;
        opacity: 0.8;
    }
    
    .test-controls {
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }
    
    .test-description {
        font-size: 0.75rem;
        opacity: 0.7;
    }
</style>