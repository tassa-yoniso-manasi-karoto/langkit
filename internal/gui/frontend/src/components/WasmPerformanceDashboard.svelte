<!-- src/components/WasmPerformanceDashboard.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  // Corrected import path for wasm state/build info functions
  import { getWasmState, resetWasmMetrics, getWasmBuildInfo, getWasmModule } from '../lib/wasm';
  import { WasmInitStatus } from '../lib/wasm-state'; // Import the enum
  import { wasmLogger, WasmLogLevel } from '../lib/wasm-logger'; // Import logger
  import { formatBytes, formatTime, formatTimestamp } from '../lib/utils'; // Import utils

  // Component state
  let wasmState = getWasmState();
  let buildInfo = getWasmBuildInfo();
  let updateInterval: number | null = null;
  let chartData: { label: string, wasm: number, ts: number }[] = [];
  let maxChartPoints = 20;

  // Chart state
  let showChart = false;
  
  // Update metrics periodically
  onMount(() => {
    updateWasmState();
    
    // Add initial data point if available
    if (wasmState.performanceMetrics.operationsCount > 0) {
        addDataPoint();
    }
    
    // Update every second
    updateInterval = window.setInterval(() => {
      updateWasmState();
      
      // Only add data points when operations have occurred and state is valid
      if (wasmState && wasmState.performanceMetrics && wasmState.performanceMetrics.operationsCount > 0) {
        addDataPoint();
      }
    }, 1000);
  });
  
  onDestroy(() => {
    if (updateInterval) {
      clearInterval(updateInterval);
    }
  });
  
  function updateWasmState() {
    wasmState = getWasmState();
    buildInfo = getWasmBuildInfo();
  }
  
  function addDataPoint() {
    // Add new data point (only if we have performance data)
    if (wasmState && wasmState.performanceMetrics && wasmState.performanceMetrics.operationsCount > 0) {
      chartData = [
        ...chartData, 
        {
          label: new Date().toLocaleTimeString(),
          // Ensure values are numbers, default to 0 if undefined/null
          wasm: wasmState.performanceMetrics.avgWasmTime || 0, 
          ts: wasmState.performanceMetrics.avgTsTime || 0
        }
      ];
      
      // Keep only the latest points
      if (chartData.length > maxChartPoints) {
        chartData = chartData.slice(chartData.length - maxChartPoints);
      }
    }
  }
  
  function handleResetMetrics() {
    resetWasmMetrics(); // This function is now imported from wasm.ts
    chartData = [];
    updateWasmState(); // Refresh local state after reset
  }
  
  // Get color based on ratio
  function getSpeedupColor(ratio: number | undefined): string {
    if (ratio === null || ratio === undefined || isNaN(ratio)) return "text-gray-400"; // Handle undefined/NaN
    if (ratio <= 1) return "text-red-500";
    if (ratio < 1.5) return "text-yellow-500";
    if (ratio < 2.5) return "text-green-400";
    return "text-green-300";
  }
  
  // Toggle chart visibility
  function toggleChart() {
    showChart = !showChart;
  }

  // Add function to force garbage collection (from Phase 2 refinement)
  function forceGarbageCollection() {
    if (wasmState?.initStatus === WasmInitStatus.SUCCESS) { // Use enum member
      try {
        const wasmModule = getWasmModule();
        if (wasmModule && wasmModule.force_garbage_collection) {
          wasmModule.force_garbage_collection();
          wasmLogger.log(
            WasmLogLevel.INFO,
            'memory',
            'Manual garbage collection performed',
            { source: 'user_action' }
          );
          // Refresh state after GC
          setTimeout(updateWasmState, 100); 
        } else {
           wasmLogger.log(WasmLogLevel.WARN, 'memory', 'force_garbage_collection function not found in WASM module.');
        }
      } catch (error: any) {
        wasmLogger.log(
          WasmLogLevel.ERROR,
          'memory',
          `Failed to perform garbage collection: ${error.message}`
        );
      }
    } else {
        wasmLogger.log(WasmLogLevel.WARN, 'memory', 'Cannot force GC: WASM not initialized successfully.');
    }
  }

</script>

<div class="bg-gray-800/60 backdrop-blur-sm rounded-lg p-4 shadow-md text-white">
  <div class="flex items-center justify-between mb-3">
    <h3 class="text-lg font-medium">WebAssembly Performance</h3>
    
    <div class="flex gap-2">
      <button
        class="px-2 py-1 bg-primary/20 hover:bg-primary/30 text-white text-xs rounded transition-colors"
        on:click={toggleChart}
        title={showChart ? 'Hide performance chart' : 'Show performance chart'}
      >
        {showChart ? 'Hide' : 'Show'} Chart
      </button>
      
      <button
        class="px-2 py-1 bg-gray-700 hover:bg-gray-600 text-white text-xs rounded transition-colors"
        on:click={handleResetMetrics}
        title="Reset performance metrics and clear chart"
      >
        Reset
      </button>
       <button
        class="px-2 py-1 bg-gray-600 hover:bg-gray-500 text-white text-xs rounded transition-colors"
        on:click={forceGarbageCollection}
        title="Request WebAssembly memory cleanup (resets allocation tracker)"
        disabled={wasmState?.initStatus !== WasmInitStatus.SUCCESS}
      >
        Clean Memory
      </button>
    </div>
  </div>
  
  {#if buildInfo}
    <div class="text-xs text-gray-400 mb-3">
      Version {buildInfo.version} <span class="text-gray-500">|</span> Built: {formatTimestamp(buildInfo.timestamp * 1000)}
    </div>
  {:else}
     <div class="text-xs text-gray-500 mb-3 italic">Build info not available...</div>
  {/if}
  
  <!-- Performance metrics summary -->
  <div class="grid grid-cols-2 gap-3 mb-4">
    <div class="bg-gray-700/70 rounded p-3">
      <div class="text-sm text-gray-400">Avg. Speedup</div>
      <div class="text-xl font-bold {getSpeedupColor(wasmState?.performanceMetrics?.speedupRatio)}">
        {wasmState?.performanceMetrics?.speedupRatio?.toFixed(2) ?? 'N/A'}x
      </div>
      <div class="text-xs text-gray-500 mt-1">
        Based on {wasmState?.performanceMetrics?.operationsCount ?? 0} operations
      </div>
    </div>
    
    <div class="bg-gray-700/70 rounded p-3">
      <div class="text-sm text-gray-400">Average Times</div>
      <div class="flex flex-col">
        <div class="text-sm">
          <span class="text-green-400">WASM:</span> 
          <span class="font-bold">{formatTime(wasmState?.performanceMetrics?.avgWasmTime)}</span>
        </div>
        <div class="text-sm">
          <span class="text-blue-400">TS:</span> 
          <span class="font-bold">{formatTime(wasmState?.performanceMetrics?.avgTsTime)}</span>
        </div>
      </div>
    </div>
    
    <div class="bg-gray-700/70 rounded p-3">
      <div class="text-sm text-gray-400">Memory Usage</div>
      <div class="flex flex-col">
        <div class="text-sm">
          <span class="text-gray-400">Used:</span> 
          <span class="font-bold">
            {wasmState?.memoryUsage ? formatBytes(wasmState.memoryUsage.used) : 'N/A'}
          </span>
        </div>
        <div class="text-sm">
          <span class="text-gray-400">Total:</span> 
          <span class="font-bold">
            {wasmState?.memoryUsage ? formatBytes(wasmState.memoryUsage.total) : 'N/A'}
          </span>
        </div>
      </div>
    </div>
    
    <div class="bg-gray-700/70 rounded p-3">
      <div class="text-sm text-gray-400">Operations</div>
      <div class="text-xl font-bold text-white">{wasmState?.totalOperations ?? 0}</div>
      <div class="text-xs text-gray-500 mt-1">
        Total WASM operations
      </div>
    </div>
  </div>
  
  <!-- Memory utilization bar -->
  {#if wasmState?.memoryUsage}
    <div class="mb-4">
      <div class="flex justify-between text-xs text-gray-400 mb-1">
        <span>Memory Utilization</span>
        <span>{Math.round((wasmState.memoryUsage.utilization || 0) * 100)}%</span>
      </div>
      <div class="w-full bg-gray-700 rounded-full h-2">
        <div 
          class="h-2 rounded-full transition-all duration-500" 
          class:bg-green-500={wasmState.memoryUsage.utilization < 0.7}
          class:bg-yellow-500={wasmState.memoryUsage.utilization >= 0.7 && wasmState.memoryUsage.utilization < 0.85}
          class:bg-red-500={wasmState.memoryUsage.utilization >= 0.85}
          style="width: {(wasmState.memoryUsage.utilization || 0) * 100}%"
        ></div>
      </div>
       <div class="text-xs text-gray-500 mt-1">
         Peak: {formatBytes(wasmState.memoryUsage.peak_bytes ?? 0)} | Allocs: {wasmState.memoryUsage.allocation_count ?? 0}
       </div>
    </div>
  {/if}
  
  <!-- Performance comparison chart -->
  {#if showChart && chartData.length > 1}
    {@const maxValue = Math.max(0.01, ...chartData.map(d => Math.max(d.wasm, d.ts))) * 1.1} <!-- Ensure maxValue is not 0 -->
    {@const height = 160} <!-- chart height in pixels -->
    <!-- Now the div containing the chart -->
    <div class="mt-4">
      <div class="text-sm text-gray-400 mb-2">Performance Trend (Avg. Time per Op)</div>
      <div class="relative h-40 bg-gray-800/80 rounded p-2">
        <!-- Chart grid lines -->
        <div class="absolute inset-0 border border-gray-700 grid grid-rows-4 pointer-events-none">
          {#each {length: 3} as _} <div class="border-b border-gray-700/50"></div> {/each}
        </div>
        
        <!-- WebAssembly line (green) -->
        <svg class="absolute inset-0 w-full h-full overflow-visible" viewBox="0 0 100 {height}">
          <polyline
            points={chartData.map((point, i) =>
              `${i * (100 / (chartData.length - 1))} ${height - (point.wasm / maxValue) * height}`
            ).join(' ')}
            stroke="rgba(52, 211, 153, 0.8)"
            stroke-width="2"
            fill="none"
            vector-effect="non-scaling-stroke"
          />
        </svg>
        
        <!-- TypeScript line (blue) -->
        <svg class="absolute inset-0 w-full h-full overflow-visible" viewBox="0 0 100 {height}">
          <polyline
            points={chartData.map((point, i) =>
              `${i * (100 / (chartData.length - 1))} ${height - (point.ts / maxValue) * height}`
            ).join(' ')}
            stroke="rgba(59, 130, 246, 0.8)"
            stroke-width="2"
            fill="none"
            vector-effect="non-scaling-stroke"
          />
        </svg>
        
        <!-- Legend -->
        <div class="absolute bottom-0 left-0 right-0 flex justify-center gap-4 text-xs py-1 bg-gray-800/60">
          <div class="flex items-center">
            <div class="w-3 h-3 bg-green-500 rounded-full mr-1"></div>
            <span class="text-green-400">WebAssembly</span>
          </div>
          <div class="flex items-center">
            <div class="w-3 h-3 bg-blue-500 rounded-full mr-1"></div>
            <span class="text-blue-400">TypeScript</span>
          </div>
        </div>
      </div>
    </div>
  {/if}
  
  <!-- Operation type breakdown -->
  {#if wasmState?.operationsPerType && Object.keys(wasmState.operationsPerType).length > 0}
    <div class="mt-4">
      <div class="text-sm text-gray-400 mb-2">Operation Types</div>
      <div class="grid grid-cols-2 gap-2">
        {#each Object.entries(wasmState.operationsPerType) as [opType, count]}
          <div class="flex justify-between items-center bg-gray-700/50 p-2 rounded text-xs">
            <span class="text-gray-300">{opType}</span>
            <span class="font-mono">{count}</span>
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>

<style>
  /* Add styles for chart animations if needed */
  svg polyline {
    transition: points 0.3s ease-out;
  }
</style>