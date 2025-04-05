<!-- src/components/WasmPerformanceDashboard.svelte -->
<script lang="ts">
  // Keep existing imports
  import { onMount, onDestroy } from 'svelte';
  import { getWasmState, resetWasmMetrics, getWasmModule } from '../lib/wasm'; // Import necessary functions
  import { WasmInitStatus } from '../lib/wasm-state'; // Import enum
  import type { WasmState } from '../lib/wasm-state'; // Import the type
  import { formatBytes, formatTime } from '../lib/utils'; // Import utils
  import { wasmLogger, WasmLogLevel } from '../lib/wasm-logger'; // Import logger

  // Component state
  let wasmState: WasmState = getWasmState(); // Use type
  let updateInterval: number | null = null;

  // Simplified tab structure
  let selectedTab = 'overview'; // 'overview', 'memory', 'operations'

  // Update metrics periodically
  onMount(() => {
    updateWasmState();
    updateInterval = window.setInterval(updateWasmState, 1000); // Update every second
  });

  onDestroy(() => {
    if (updateInterval) {
      clearInterval(updateInterval);
    }
  });

  function updateWasmState() {
    wasmState = getWasmState(); // Fetch latest state
  }

  function handleResetMetrics() {
    resetWasmMetrics();
    updateWasmState(); // Refresh local state after reset
  }

  // Helper functions for formatting
  function getPerformanceClass(ratio: number): string {
    if (ratio >= 5) return 'text-green-300';
    if (ratio >= 3) return 'text-green-400';
    if (ratio >= 1.5) return 'text-green-500';
    if (ratio >= 1.2) return 'text-yellow-400';
    return 'text-red-400';
  }

  function getPerformanceLabel(ratio: number): string {
    if (ratio >= 5) return 'Excellent';
    if (ratio >= 3) return 'Great';
    if (ratio >= 1.5) return 'Good';
    if (ratio >= 1.2) return 'Moderate';
    return 'Minimal';
  }

  // Format numbers with specified precision
  function formatNumber(value: number, precision: number = 2): string {
    return value.toFixed(precision);
  }

  // Force garbage collection
  function forceGarbageCollection() {
    const currentState = getWasmState(); // Get current state

    if (currentState?.initStatus !== WasmInitStatus.SUCCESS) {
      wasmLogger.log(
        WasmLogLevel.WARN,
        'memory',
        'Cannot force GC: WebAssembly not initialized successfully'
      );
      return;
    }

    try {
      const wasmModule = getWasmModule(); // Get module instance
      if (!wasmModule || typeof wasmModule.force_garbage_collection !== 'function') {
        wasmLogger.log(
          WasmLogLevel.WARN,
          'memory',
          'force_garbage_collection function not found in WASM module'
        );
        return;
      }

      wasmModule.force_garbage_collection();
      wasmLogger.log(
        WasmLogLevel.INFO,
        'memory',
        'Manual garbage collection performed',
        { source: 'user_action' }
      );

      // Refresh state after GC to show updated memory usage
      setTimeout(updateWasmState, 100); // Short delay to allow state update
    } catch (e: any) {
      wasmLogger.log(
        WasmLogLevel.ERROR,
        'memory',
        `Failed to perform garbage collection: ${e.message}`
      );
    }
  }
</script>

<div class="bg-gray-800/60 backdrop-blur-sm rounded-lg p-4 shadow-md text-white">
  <div class="flex items-center justify-between mb-3">
    <h3 class="text-lg font-medium">WebAssembly Performance</h3>

    <!-- Simplified Tab Buttons -->
    <div class="flex gap-2">
      <button
        class="px-2 py-1 bg-primary/20 hover:bg-primary/30 text-white text-xs rounded transition-colors"
        class:bg-primary={selectedTab === 'overview'}
        on:click={() => selectedTab = 'overview'}
      >
        Overview
      </button>

      <button
        class="px-2 py-1 bg-primary/20 hover:bg-primary/30 text-white text-xs rounded transition-colors"
        class:bg-primary={selectedTab === 'memory'}
        on:click={() => selectedTab = 'memory'}
      >
        Memory
      </button>

      <button
        class="px-2 py-1 bg-primary/20 hover:bg-primary/30 text-white text-xs rounded transition-colors"
        class:bg-primary={selectedTab === 'operations'}
        on:click={() => selectedTab = 'operations'}
      >
        Operations
      </button>

      <button
        class="px-2 py-1 bg-gray-700 hover:bg-gray-600 text-white text-xs rounded transition-colors"
        on:click={handleResetMetrics}
        title="Reset performance metrics and clear chart"
      >
        Reset
      </button>
    </div>
  </div>

  <!-- Overview Tab: Simplified and More Focused -->
  {#if selectedTab === 'overview'}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <!-- Performance Card -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Performance Impact</div>
        <div class="flex items-end gap-2">
          <div class="text-2xl font-bold {getPerformanceClass(wasmState?.performanceMetrics?.speedupRatio)}">
            {formatNumber(wasmState?.performanceMetrics?.speedupRatio)}×
          </div>
          <div class="text-sm text-gray-300">
            {getPerformanceLabel(wasmState?.performanceMetrics?.speedupRatio)}
          </div>
        </div>
        <div class="text-xs text-gray-400 mt-2">
          Based on {wasmState?.performanceMetrics?.operationsCount || 0} operations
        </div>

        <!-- Performance impact visualization -->
        <div class="mt-3">
          <div class="flex justify-between text-xs text-gray-400">
            <span>TypeScript</span>
            <span>WebAssembly</span>
          </div>
          <div class="relative h-4 bg-gray-600 rounded mt-1 overflow-hidden">
            {#if wasmState?.performanceMetrics?.speedupRatio > 0}
              <div class="absolute top-0 left-0 h-full bg-blue-500 rounded"
                   style="width: {Math.min(100, 100 / (wasmState.performanceMetrics.speedupRatio || 1))}%">
              </div>
            {/if}
          </div>
          <div class="flex justify-between text-xs text-gray-500 mt-1">
            <span>{formatTime(wasmState?.performanceMetrics?.avgTsTime)}</span>
            <span>{formatTime(wasmState?.performanceMetrics?.avgWasmTime)}</span>
          </div>
        </div>
      </div>

      <!-- Memory Usage Card -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Memory Usage</div>
        {#if wasmState?.memoryUsage}
          <div class="flex flex-col">
            <div class="text-md">
              <span class="text-gray-400">Used:</span>
              <span class="font-bold">
                {formatBytes(wasmState.memoryUsage.used)}
              </span>
              <span class="text-gray-500">
                of {formatBytes(wasmState.memoryUsage.total)}
              </span>
            </div>

            <!-- Memory usage bar -->
            <div class="mt-2">
              <div class="flex justify-between text-xs text-gray-400 mb-1">
                <span>Utilization</span>
                <span>{Math.round((wasmState.memoryUsage.utilization || 0) * 100)}%</span>
              </div>
              <div class="w-full bg-gray-600 rounded-full h-2">
                <div
                  class="h-2 rounded-full transition-all duration-500"
                  class:bg-green-500={wasmState.memoryUsage.utilization < 0.7}
                  class:bg-yellow-500={wasmState.memoryUsage.utilization >= 0.7 && wasmState.memoryUsage.utilization < 0.85}
                  class:bg-red-500={wasmState.memoryUsage.utilization >= 0.85}
                  style="width: {(wasmState.memoryUsage.utilization || 0) * 100}%"
                ></div>
              </div>
            </div>

            <button
              class="mt-3 px-3 py-1 bg-blue-600/50 hover:bg-blue-500/60 text-white text-xs rounded self-end"
              on:click={forceGarbageCollection}
              disabled={wasmState?.initStatus !== WasmInitStatus.SUCCESS}
            >
              Clean Up Memory
            </button>
          </div>
        {:else}
          <div class="text-gray-400">Memory information not available</div>
        {/if}
      </div>

      <!-- Activity Summary -->
      <div class="bg-gray-700/70 rounded p-4 col-span-1 md:col-span-2">
        <div class="text-sm text-gray-400 mb-2">Activity Summary</div>
        <div class="grid grid-cols-3 gap-4">
          <div>
            <div class="text-xs text-gray-400">Total Operations</div>
            <div class="text-xl font-bold text-white">{wasmState?.totalOperations || 0}</div>
          </div>
          <div>
            <div class="text-xs text-gray-400">Net Speedup</div>
            <div class="text-xl font-bold {getPerformanceClass(wasmState?.performanceMetrics?.netSpeedupRatio)}">
              {formatNumber(wasmState?.performanceMetrics?.netSpeedupRatio)}×
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-400">Status</div>
            <div class="text-xl font-bold">
              {#if wasmState?.initStatus === WasmInitStatus.SUCCESS}
                <span class="text-green-400">Active</span>
              {:else if wasmState?.initStatus === WasmInitStatus.INITIALIZING}
                <span class="text-yellow-400">Starting</span>
              {:else if wasmState?.initStatus === WasmInitStatus.FAILED}
                <span class="text-red-400">Failed</span>
              {:else}
                <span class="text-gray-400">Inactive</span>
              {/if}
            </div>
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- Keep Memory and Operations tabs similar to original but with improved clarity -->
  <!-- [Memory tab and Operations tab code - keep with minor clarity improvements] -->
  <!-- TODO: Implement Memory and Operations tabs based on the previous version or specs -->
  {#if selectedTab === 'memory'}
      <div class="text-gray-400 p-4 text-center">Memory details view not yet implemented in this version.</div>
  {/if}
  {#if selectedTab === 'operations'}
      <div class="text-gray-400 p-4 text-center">Operations details view not yet implemented in this version.</div>
  {/if}
</div>