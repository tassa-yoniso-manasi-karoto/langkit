<script lang="ts">
  import { testWasmMemoryAccess } from '../../tests/wasm/memory-test';
  import { logger } from '../../lib/logger';

  export let variant: 'primary' | 'secondary' | 'debug' = 'debug';
  export let size: 'small' | 'medium' | 'large' = 'medium';

  let isRunning = false;

  async function runTest() {
    if (isRunning) return;

    isRunning = true;
    logger.info('memory-test', 'Starting WebAssembly memory test');

    try {
      // Force WASM module initialization if needed
      logger.info('memory-test', 'Importing WASM module directly');

      // Import the module directly to run test
      await testWasmMemoryAccess();
      logger.info('memory-test', 'Memory test completed successfully');
    } catch (error) {
      logger.error('memory-test', 'Memory test failed', {
        error: error instanceof Error ? error.message : String(error)
      });
    } finally {
      isRunning = false;
    }
  }
</script>

<button
  class="flex items-center justify-center gap-1 rounded-lg transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-bg"
  class:primary={variant === 'primary'}
  class:secondary={variant === 'secondary'}
  class:debug={variant === 'debug'}
  class:small={size === 'small'}
  class:medium={size === 'medium'}
  class:large={size === 'large'}
  class:running={isRunning}
  on:click={runTest}
  disabled={isRunning}
  title={isRunning ? 'Running WebAssembly memory test...' : 'Test WebAssembly memory'}
>
  <span class="material-icons text-sm">memory</span>
  {#if isRunning}
    <div class="spinner"></div>
  {/if}
</button>

<style>
  button {
    font-family: 'DM Sans', sans-serif;
    font-weight: 500;
    white-space: nowrap;
  }
  
  .primary {
    background-color: rgb(220, 38, 38); /* Bright red */
    color: white;
    border: 2px solid white;
    animation: pulse 2s infinite;
    font-weight: bold;
    text-transform: uppercase;
  }

  .primary:hover:not(:disabled) {
    background-color: rgb(185, 28, 28);
    transform: scale(1.05);
  }

  .primary:focus {
    box-shadow: 0 0 0 4px rgba(220, 38, 38, 0.5);
  }

  @keyframes pulse {
    0% { box-shadow: 0 0 0 0 rgba(255, 255, 255, 0.7); }
    70% { box-shadow: 0 0 0 10px rgba(255, 255, 255, 0); }
    100% { box-shadow: 0 0 0 0 rgba(255, 255, 255, 0); }
  }
  
  .secondary {
    background-color: rgb(255 255 255 / 0.1);
    color: white;
    border: 1px solid rgb(255 255 255 / 0.2);
  }
  
  .secondary:hover:not(:disabled) {
    background-color: rgb(255 255 255 / 0.15);
  }
  
  .secondary:focus {
    box-shadow: 0 0 0 2px rgb(255 255 255 / 0.2);
  }
  
  .debug {
    background-color: rgb(236 72 153 / 0.1);
    color: rgb(236 72 153);
    border: 1px solid rgb(236 72 153 / 0.3);
  }
  
  .debug:hover:not(:disabled) {
    background-color: rgb(236 72 153 / 0.15);
  }
  
  .debug:focus {
    box-shadow: 0 0 0 2px rgb(236 72 153 / 0.2);
  }
  
  .small {
    padding: 0.25rem;
    font-size: 0.7rem;
    min-width: 2rem;
    min-height: 2rem;
  }

  .medium {
    padding: 0.35rem 0.5rem;
    font-size: 0.8rem;
  }

  .large {
    padding: 0.5rem 0.75rem;
    font-size: 0.875rem;
  }
  
  button:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
  
  .running {
    cursor: wait;
  }
  
  .spinner {
    width: 1rem;
    height: 1rem;
    border: 2px solid currentColor;
    border-bottom-color: transparent;
    border-radius: 50%;
    display: inline-block;
    animation: rotation 1s linear infinite;
  }

  @keyframes rotation {
    0% {
      transform: rotate(0deg);
    }
    100% {
      transform: rotate(360deg);
    }
  }
</style>