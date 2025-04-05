<!-- src/components/WasmPerformanceDashboard.svelte - Enhanced version from Phase 3.2 -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  // Corrected import path for wasm state/build info functions
  // Import getWasmModule for GC button
  import { getWasmState, resetWasmMetrics, getWasmBuildInfo, getWasmModule } from '../lib/wasm';
  import { WasmInitStatus } from '../lib/wasm-state'; // Import the enum
  import type { WasmState } from '../lib/wasm-state'; // Import the type
  import { wasmLogger, WasmLogLevel } from '../lib/wasm-logger'; // Import logger
  import { formatBytes, formatTime, formatTimestamp } from '../lib/utils'; // Import utils

  // Component state
  let wasmState: WasmState = getWasmState(); // Use type
  let buildInfo = getWasmBuildInfo();
  let updateInterval: number | null = null;
  // Updated chart data structure
  let chartData: { label: string, wasm: number, ts: number, net: number }[] = [];
  let maxChartPoints = 20;
  let selectedTab = 'overview'; // 'overview', 'memory', 'operations', 'diagnostics'

  // Chart state
  let showChart = false;

  // Enhanced memory stats
  let memoryHistory: {timestamp: number, used: number, utilization: number}[] = [];

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

      // Track memory usage history
      if (wasmState.memoryUsage) {
        trackMemoryHistory();
      }
    }, 1000);
  });

  onDestroy(() => {
    if (updateInterval) {
      clearInterval(updateInterval);
    }
  });

  function updateWasmState() {
    wasmState = getWasmState(); // Fetch latest state
    buildInfo = getWasmBuildInfo(); // Fetch latest build info
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
          ts: wasmState.performanceMetrics.avgTsTime || 0,
          // Calculate net time including serialization/deserialization
          net: (wasmState.performanceMetrics.avgWasmTime || 0) +
               (wasmState.performanceMetrics.avgSerializationTime || 0) +
               (wasmState.performanceMetrics.avgDeserializationTime || 0)
        }
      ];

      // Keep only the latest points
      if (chartData.length > maxChartPoints) {
        chartData = chartData.slice(chartData.length - maxChartPoints);
      }
    }
  }

  function trackMemoryHistory() {
    if (wasmState.memoryUsage) {
      memoryHistory = [
        ...memoryHistory,
        {
          timestamp: Date.now(),
          used: wasmState.memoryUsage.used,
          utilization: wasmState.memoryUsage.utilization
        }
      ];

      // Keep only the latest points for memory history
      if (memoryHistory.length > 60) { // Keep last 60 seconds
        memoryHistory = memoryHistory.slice(memoryHistory.length - 60);
      }
    }
  }

  function handleResetMetrics() {
    resetWasmMetrics(); // This function is now imported from wasm.ts
    chartData = [];
    memoryHistory = []; // Clear memory history too
    updateWasmState(); // Refresh local state after reset
  }

  // Get color based on ratio
  function getSpeedupColor(ratio: number | undefined): string {
    if (ratio === undefined || ratio === null || isNaN(ratio)) return "text-gray-400";
    if (ratio <= 1) return "text-red-500";
    if (ratio < 1.5) return "text-yellow-500";
    if (ratio < 2.5) return "text-green-400";
    return "text-green-300"; // Brighter green for higher speedup
  }

  // Toggle chart visibility
  function toggleChart() {
    showChart = !showChart;
  }

  // Force garbage collection with improved null safety
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

  // Get memory trend indicator
  function getMemoryTrend(): { trend: 'stable' | 'rising' | 'falling', rate: number } {
    if (memoryHistory.length < 5) { // Need a few points for trend
      return { trend: 'stable', rate: 0 };
    }

    // Calculate trend over last few points (e.g., last 5 seconds)
    const recentPoints = memoryHistory.slice(-5);
    const firstPoint = recentPoints[0];
    const lastPoint = recentPoints[recentPoints.length - 1];

    const timeDiff = (lastPoint.timestamp - firstPoint.timestamp) / 1000; // seconds
    if (timeDiff <= 0) {
      return { trend: 'stable', rate: 0 }; // Avoid division by zero
    }

    const memoryDiff = lastPoint.used - firstPoint.used;
    const ratePerSec = memoryDiff / timeDiff;

    // Define thresholds for stable/rising/falling
    const stableThreshold = 1024; // +/- 1KB/sec is considered stable

    if (Math.abs(ratePerSec) < stableThreshold) {
      return { trend: 'stable', rate: ratePerSec };
    } else if (ratePerSec > 0) {
      return { trend: 'rising', rate: ratePerSec };
    } else {
      return { trend: 'falling', rate: ratePerSec };
    }
  }

  // Calculate memory trend reactively
  $: memoryTrend = getMemoryTrend();

</script>

<div class="bg-gray-800/60 backdrop-blur-sm rounded-lg p-4 shadow-md text-white">
  <div class="flex items-center justify-between mb-3">
    <h3 class="text-lg font-medium">WebAssembly Performance</h3>

    <!-- Tab Buttons -->
    <div class="flex gap-2">
      <button
        class="px-2 py-1 bg-primary/20 hover:bg-primary/30 text-white text-xs rounded transition-colors"
        class:bg-primary={selectedTab === 'overview'}
        on:click={() => selectedTab = 'overview'}
        title="Show performance overview"
      >
        Overview
      </button>

      <button
        class="px-2 py-1 bg-primary/20 hover:bg-primary/30 text-white text-xs rounded transition-colors"
        class:bg-primary={selectedTab === 'memory'}
        on:click={() => selectedTab = 'memory'}
        title="Show memory metrics"
      >
        Memory
      </button>

      <button
        class="px-2 py-1 bg-primary/20 hover:bg-primary/30 text-white text-xs rounded transition-colors"
        class:bg-primary={selectedTab === 'operations'}
        on:click={() => selectedTab = 'operations'}
        title="Show operation details"
      >
        Operations
      </button>

      <button
        class="px-2 py-1 bg-primary/20 hover:bg-primary/30 text-white text-xs rounded transition-colors"
        class:bg-primary={selectedTab === 'diagnostics'}
        on:click={() => selectedTab = 'diagnostics'}
        title="Show diagnostics"
      >
        Diagnostics
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

  {#if buildInfo}
    <div class="text-xs text-gray-400 mb-3">
      Version {buildInfo.version} <span class="text-gray-500">|</span> Built: {formatTimestamp(buildInfo.timestamp * 1000)}
    </div>
  {:else}
     <div class="text-xs text-gray-500 mb-3 italic">Build info not available...</div>
  {/if}

  <!-- Tab content -->
  {#if selectedTab === 'overview'}
    <!-- Performance metrics summary -->
    <div class="grid grid-cols-2 gap-3 mb-4">
      <div class="bg-gray-700/70 rounded p-3">
        <div class="text-sm text-gray-400">Speedup Metrics</div>
        <div class="text-xl font-bold {getSpeedupColor(wasmState?.performanceMetrics?.speedupRatio)}">
          {wasmState?.performanceMetrics?.speedupRatio?.toFixed(2) ?? 'N/A'}x
        </div>
        <div class="text-xs text-gray-400 mt-1">
          Net: {wasmState?.performanceMetrics?.netSpeedupRatio?.toFixed(2) ?? 'N/A'}x
        </div>
        <div class="text-xs text-gray-500 mt-1">
          Based on {wasmState?.performanceMetrics?.operationsCount ?? 0} operations
        </div>
      </div>

      <div class="bg-gray-700/70 rounded p-3">
        <div class="text-sm text-gray-400">Processing Times</div>
        <div class="flex flex-col">
          <div class="text-sm">
            <span class="text-green-400">WASM:</span>
            <span class="font-bold">{formatTime(wasmState?.performanceMetrics?.avgWasmTime)}</span>
          </div>
          <div class="text-sm">
            <span class="text-blue-400">TS:</span>
            <span class="font-bold">{formatTime(wasmState?.performanceMetrics?.avgTsTime)}</span>
          </div>
          <div class="text-xs text-gray-400">
            <span>Overhead:</span>
            <span>{formatTime((wasmState?.performanceMetrics?.avgSerializationTime || 0) + (wasmState?.performanceMetrics?.avgDeserializationTime || 0))}</span>
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
        <div class="text-sm text-gray-400">Total Operations</div>
        <div class="text-xl font-bold text-white">{wasmState?.totalOperations ?? 0}</div>
        <div class="text-xs text-gray-500 mt-1">
          <span>S: {wasmState?.performanceMetrics?.logSizeDistribution?.small ?? 0}</span> /
          <span>M: {wasmState?.performanceMetrics?.logSizeDistribution?.medium ?? 0}</span> /
          <span>L: {wasmState?.performanceMetrics?.logSizeDistribution?.large ?? 0}</span>
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
    <div class="mt-4">
      <div class="flex justify-between text-sm text-gray-400 mb-2">
        <span>Performance Trend (Avg Time / Op)</span>
        <button
          class="text-xs text-gray-400 hover:text-white"
          on:click={toggleChart}
        >
          {showChart ? 'Hide Chart' : 'Show Chart'}
        </button>
      </div>

      {#if showChart && chartData.length > 1}
        {@const maxValue = Math.max(0.01, ...chartData.map(d => Math.max(d.wasm, d.ts, d.net))) * 1.1}
        {@const height = 160}
        <div class="relative h-40 bg-gray-800/80 rounded p-2">
          <!-- Chart grid lines -->
          <div class="absolute inset-0 border border-gray-700 grid grid-rows-4 pointer-events-none">
            {#each {length: 3} as _} <div class="border-b border-gray-700/50"></div> {/each}
          </div>

          <!-- Net WebAssembly time line (yellow) -->
          <svg class="absolute inset-0 w-full h-full overflow-visible" viewBox="0 0 100 {height}">
            <polyline
              points={chartData.map((point, i) =>
                `${i * (100 / (chartData.length - 1))} ${height - (point.net / maxValue) * height}`
              ).join(' ')}
              stroke="rgba(250, 204, 21, 0.6)"
              stroke-width="2"
              fill="none"
              vector-effect="non-scaling-stroke"
            />
          </svg>

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
              <span class="text-green-400">WASM</span>
            </div>
            <div class="flex items-center">
              <div class="w-3 h-3 bg-yellow-500 rounded-full mr-1"></div>
              <span class="text-yellow-400">WASM+Overhead</span>
            </div>
            <div class="flex items-center">
              <div class="w-3 h-3 bg-blue-500 rounded-full mr-1"></div>
              <span class="text-blue-400">TypeScript</span>
            </div>
          </div>
        </div>
      {:else}
        <div class="h-10 flex items-center justify-center text-gray-500 text-sm">
          {chartData.length > 1 ? (showChart ? '' : 'Chart hidden') : 'Not enough data for chart'}
        </div>
      {/if}
    </div>

  {:else if selectedTab === 'memory'}
    <!-- Memory tab content -->
    <div class="space-y-4">
      <!-- Memory stats -->
      <div class="grid grid-cols-2 gap-3">
        <div class="bg-gray-700/70 rounded p-3">
          <div class="text-sm text-gray-400">Current Usage</div>
          <div class="text-xl font-bold">
            {wasmState?.memoryUsage ? formatBytes(wasmState.memoryUsage.used) : 'N/A'}
          </div>
          <div class="text-xs text-gray-400 mt-1">
            Total: {wasmState?.memoryUsage ? formatBytes(wasmState.memoryUsage.total) : 'N/A'}
          </div>
        </div>

        <div class="bg-gray-700/70 rounded p-3">
          <div class="text-sm text-gray-400">Memory Trend</div>
          <div class="text-xl font-bold flex items-center">
            {#if memoryTrend?.trend === 'stable'}
              <span class="text-gray-300">Stable</span>
            {:else if memoryTrend?.trend === 'rising'}
              <span class="text-red-400">Rising</span>
            {:else if memoryTrend?.trend === 'falling'}
              <span class="text-green-400">Falling</span>
            {/if}

            {#if memoryTrend?.trend !== 'stable'}
              <span class="text-sm ml-2">
                ({formatBytes(Math.abs(memoryTrend.rate))}/s)
              </span>
            {/if}
          </div>
          <div class="text-xs text-gray-400 mt-1">
            Based on last {memoryHistory.length} data points
          </div>
        </div>

        <div class="bg-gray-700/70 rounded p-3">
          <div class="text-sm text-gray-400">Peak Memory</div>
          <div class="text-xl font-bold">
            {wasmState?.memoryUsage ? formatBytes(wasmState.memoryUsage.peak_bytes || 0) : 'N/A'}
          </div>
          <div class="text-xs text-gray-400 mt-1">
            Since initialization
          </div>
        </div>

        <div class="bg-gray-700/70 rounded p-3">
          <div class="text-sm text-gray-400">Allocations</div>
          <div class="text-xl font-bold">
            {wasmState?.memoryUsage?.allocation_count ?? 'N/A'}
          </div>
          <div class="text-xs text-gray-400 mt-1">
            Since last GC
          </div>
        </div>
      </div>

      <!-- Memory trend graph -->
      {#if memoryHistory.length > 1}
        {@const maxMemory = Math.max(1024, ...memoryHistory.map(p => p.used)) * 1.1} <!-- Ensure maxMemory is at least 1KB -->
        {@const height = 160}
        <div class="mt-4">
          <div class="text-sm text-gray-400 mb-2">Memory History (Last 60s)</div>
          <div class="relative h-40 bg-gray-800/80 rounded p-2">
            <!-- Chart grid lines -->
            <div class="absolute inset-0 border border-gray-700 grid grid-rows-4 pointer-events-none">
              {#each {length: 3} as _} <div class="border-b border-gray-700/50"></div> {/each}
            </div>

            <!-- Memory usage line -->
            <svg class="absolute inset-0 w-full h-full overflow-visible" viewBox="0 0 100 {height}">
              <polyline
                points={memoryHistory.map((point, i) =>
                  `${i * (100 / (memoryHistory.length - 1))} ${height - (point.used / maxMemory) * height}`
                ).join(' ')}
                stroke="rgba(52, 211, 153, 0.8)"
                stroke-width="2"
                fill="none"
                vector-effect="non-scaling-stroke"
              />
            </svg>

            <!-- Label -->
            <div class="absolute bottom-1 right-2 text-xs text-gray-400">
              Max: {formatBytes(maxMemory / 1.1)} <!-- Show actual max, not scaled -->
            </div>
          </div>
        </div>
      {/if}

      <!-- Advanced memory metrics -->
      {#if wasmState?.memoryUsage}
        <div class="space-y-3 mt-4">
          <div class="text-sm text-gray-400">Advanced Memory Metrics</div>

          <!-- Memory utilization bar -->
          <div>
            <div class="flex justify-between text-xs text-gray-400 mb-1">
              <span>Utilization</span>
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
          </div>

          <!-- Fragmentation -->
          {#if wasmState.memoryUsage.fragmentation_estimate !== undefined}
            <div>
              <div class="flex justify-between text-xs text-gray-400 mb-1">
                <span>Fragmentation Estimate</span>
                <span>{Math.round((wasmState.memoryUsage.fragmentation_estimate || 0) * 100)}%</span>
              </div>
              <div class="w-full bg-gray-700 rounded-full h-2">
                <div
                  class="h-2 rounded-full transition-all duration-500"
                  class:bg-green-500={wasmState.memoryUsage.fragmentation_estimate < 0.3}
                  class:bg-yellow-500={wasmState.memoryUsage.fragmentation_estimate >= 0.3 && wasmState.memoryUsage.fragmentation_estimate < 0.6}
                  class:bg-red-500={wasmState.memoryUsage.fragmentation_estimate >= 0.6}
                  style="width: {(wasmState.memoryUsage.fragmentation_estimate || 0) * 100}%"
                ></div>
              </div>
            </div>
          {/if}

          <!-- Memory metrics table -->
          <table class="w-full text-xs mt-2">
            <tbody>
              {#if wasmState.memoryUsage.allocation_rate !== undefined}
                <tr>
                  <td class="text-gray-400 pr-2 py-0.5">Allocation Rate:</td>
                  <td class="py-0.5">{formatBytes(wasmState.memoryUsage.allocation_rate)}/s</td>
                </tr>
              {/if}

              {#if wasmState.memoryUsage.average_allocation !== undefined}
                <tr>
                  <td class="text-gray-400 pr-2 py-0.5">Avg. Allocation:</td>
                  <td class="py-0.5">{formatBytes(wasmState.memoryUsage.average_allocation)}</td>
                </tr>
              {/if}

              {#if wasmState.memoryUsage.time_since_last_gc !== undefined}
                <tr>
                  <td class="text-gray-400 pr-2 py-0.5">Time Since Last GC:</td>
                  <td class="py-0.5">{formatTime(wasmState.memoryUsage.time_since_last_gc)}</td>
                </tr>
              {/if}

              {#if wasmState.memoryUsage.memory_growth_trend !== undefined}
                <tr>
                  <td class="text-gray-400 pr-2 py-0.5">Growth Trend:</td>
                  <td class="py-0.5">{wasmState.memoryUsage.memory_growth_trend > 0 ? 'Growing' : 'Shrinking'} ({wasmState.memoryUsage.memory_growth_trend.toFixed(2)})</td>
                </tr>
              {/if}
            </tbody>
          </table>
        </div>
      {/if}

      <!-- Memory Management Actions -->
      <div class="flex justify-end mt-4 gap-2">
        <button
          class="px-3 py-1 bg-blue-600/50 hover:bg-blue-500/60 text-white text-xs rounded transition-colors"
          on:click={forceGarbageCollection}
          disabled={wasmState?.initStatus !== WasmInitStatus.SUCCESS}
          title="Force WebAssembly garbage collection"
        >
          Force Garbage Collection
        </button>
      </div>
    </div>

  {:else if selectedTab === 'operations'}
    <!-- Operations tab content -->
    <div class="space-y-4">
      <!-- Operations summary -->
      <div class="grid grid-cols-2 gap-3">
        <div class="bg-gray-700/70 rounded p-3">
          <div class="text-sm text-gray-400">Total Operations</div>
          <div class="text-xl font-bold">{wasmState?.totalOperations || 0}</div>
          <div class="text-xs text-gray-400 mt-1">
            Since initialization
          </div>
        </div>

        <div class="bg-gray-700/70 rounded p-3">
          <div class="text-sm text-gray-400">Avg Operation Timing</div>
          <div class="flex flex-col">
            <div class="text-sm">
              <span class="text-green-400">WASM:</span>
              <span class="font-bold">{formatTime(wasmState?.performanceMetrics?.avgWasmTime || 0)}</span>
            </div>
            <div class="text-sm">
              <span class="text-gray-400">Overhead:</span>
              <span class="font-bold">{formatTime((wasmState?.performanceMetrics?.avgSerializationTime || 0) +
                                               (wasmState?.performanceMetrics?.avgDeserializationTime || 0))}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Operation type breakdown -->
      {#if wasmState?.operationsPerType && Object.keys(wasmState.operationsPerType).length > 0}
        <div class="mt-4">
          <div class="text-sm text-gray-400 mb-2">Operation Types</div>
          <div class="grid grid-cols-1 gap-2">
            {#each Object.entries(wasmState.operationsPerType) as [opType, count]}
              <!-- Enhanced operation card -->
              <div class="bg-gray-700/50 p-3 rounded">
                <div class="flex justify-between items-center">
                  <span class="text-gray-300 font-medium">{opType}</span>
                  <span class="font-mono">{count}</span>
                </div>

                <!-- Add operation-specific metrics if available -->
                {#if wasmState.performanceMetrics.operationTimings && wasmState.performanceMetrics.operationTimings[opType]}
                  {@const opTiming = wasmState.performanceMetrics.operationTimings[opType]}
                  <div class="mt-2 text-xs grid grid-cols-2 gap-x-4">
                    <div>
                      <span class="text-gray-400">Avg. Time:</span>
                      <span class="text-white font-medium">{formatTime(opTiming.avgTime)}</span>
                    </div>
                    <div>
                      <span class="text-gray-400">Count:</span>
                      <span class="text-white font-medium">{opTiming.count}</span>
                    </div>
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <!-- Log Size Distribution -->
      {#if wasmState?.performanceMetrics?.logSizeDistribution}
        <!-- Move const declarations directly inside the #if block -->
        {@const total = (wasmState.performanceMetrics.logSizeDistribution.small || 0) + (wasmState.performanceMetrics.logSizeDistribution.medium || 0) + (wasmState.performanceMetrics.logSizeDistribution.large || 0)}
        {@const smallPct = total > 0 ? (wasmState.performanceMetrics.logSizeDistribution.small || 0) / total * 100 : 0}
        {@const medPct = total > 0 ? (wasmState.performanceMetrics.logSizeDistribution.medium || 0) / total * 100 : 0}
        {@const largePct = total > 0 ? (wasmState.performanceMetrics.logSizeDistribution.large || 0) / total * 100 : 0}
        <div class="mt-4">
          <div class="text-sm text-gray-400 mb-2">Log Size Distribution</div>

          <!-- Remove const declarations from here -->



          <div class="w-full rounded-lg overflow-hidden bg-gray-700 h-6 flex text-white">
            <div
              class="bg-blue-500 h-full flex items-center justify-center text-xs"
              style="width: {smallPct}%;"
              title="Small (<500): {wasmState.performanceMetrics.logSizeDistribution.small || 0} ({smallPct.toFixed(1)}%)"
            >
              {smallPct >= 10 ? 'S' : ''}
            </div>
            <div
              class="bg-green-500 h-full flex items-center justify-center text-xs"
              style="width: {medPct}%;"
              title="Medium (500-2000): {wasmState.performanceMetrics.logSizeDistribution.medium || 0} ({medPct.toFixed(1)}%)"
            >
              {medPct >= 10 ? 'M' : ''}
            </div>
            <div
              class="bg-yellow-500 h-full flex items-center justify-center text-xs"
              style="width: {largePct}%;"
              title="Large (>2000): {wasmState.performanceMetrics.logSizeDistribution.large || 0} ({largePct.toFixed(1)}%)"
            >
              {largePct >= 10 ? 'L' : ''}
            </div>
          </div>

          <div class="flex justify-between text-xs text-gray-400 mt-1">
            <div>S: {wasmState.performanceMetrics.logSizeDistribution.small || 0}</div>
            <div>M: {wasmState.performanceMetrics.logSizeDistribution.medium || 0}</div>
            <div>L: {wasmState.performanceMetrics.logSizeDistribution.large || 0}</div>
          </div>
        </div>
      {/if}
    </div>

  {:else if selectedTab === 'diagnostics'}
    <!-- Diagnostics tab content -->
    <div class="space-y-4">
      <!-- Status summary -->
      <div class="grid grid-cols-2 gap-3">
        <div class="bg-gray-700/70 rounded p-3">
          <div class="text-sm text-gray-400">Initialization Status</div>
          <div class="text-lg font-bold">
            {#if wasmState.initStatus === WasmInitStatus.SUCCESS}
              <span class="text-green-400">Success</span>
            {:else if wasmState.initStatus === WasmInitStatus.INITIALIZING}
              <span class="text-yellow-400">Initializing...</span>
            {:else if wasmState.initStatus === WasmInitStatus.FAILED}
              <span class="text-red-400">Failed</span>
            {:else}
              <span class="text-gray-400">Not Started</span>
            {/if}
          </div>
          {#if wasmState.initTime !== undefined}
            <div class="text-xs text-gray-400 mt-1">
              Init Time: {formatTime(wasmState.initTime)}
            </div>
          {/if}
        </div>

        <div class="bg-gray-700/70 rounded p-3">
          <div class="text-sm text-gray-400">Last Used</div>
          <div class="text-lg">
            {wasmState.lastUsed
              ? new Date(wasmState.lastUsed).toLocaleTimeString()
              : 'Never'}
          </div>
          {#if wasmState.lastUsed}
            <div class="text-xs text-gray-400 mt-1">
              {Math.floor((Date.now() - wasmState.lastUsed) / 1000)}s ago
            </div>
          {/if}
        </div>

        <!-- Error status -->
        <div class="bg-gray-700/70 rounded p-3 col-span-2">
          <div class="text-sm text-gray-400">Last Error</div>
          {#if wasmState.lastError}
            <div class="text-red-400 font-medium mt-1 text-xs break-words">
              {wasmState.lastError.name}: {wasmState.lastError.message}
            </div>
            {#if (wasmState.lastError as any).operation}
              <div class="text-xs text-gray-400 mt-1">
                Operation: {(wasmState.lastError as any).operation}
              </div>
            {/if}
             {#if (wasmState.lastError as any).context}
              <details class="text-xs text-gray-500 mt-1">
                  <summary class="cursor-pointer">Context</summary>
                  <pre class="whitespace-pre-wrap break-all bg-gray-800 p-1 rounded mt-1">{JSON.stringify((wasmState.lastError as any).context, null, 2)}</pre>
              </details>
            {/if}
             {#if (wasmState.lastError as any).memoryInfo}
              <details class="text-xs text-gray-500 mt-1">
                  <summary class="cursor-pointer">Memory Info</summary>
                  <pre class="whitespace-pre-wrap break-all bg-gray-800 p-1 rounded mt-1">{JSON.stringify((wasmState.lastError as any).memoryInfo, null, 2)}</pre>
              </details>
            {/if}
          {:else}
            <div class="text-gray-300 mt-1">No errors reported</div>
          {/if}
        </div>
      </div>

      <!-- Build information -->
      {#if buildInfo}
        <div class="mt-4">
          <div class="text-sm text-gray-400 mb-2">Build Information</div>
          <table class="w-full text-xs border-collapse">
            <tbody>
              <tr class="border-b border-gray-700">
                <td class="py-1 text-gray-400 w-1/3">Version:</td>
                <td class="py-1 text-white">{buildInfo.version}</td>
              </tr>
              <tr class="border-b border-gray-700">
                <td class="py-1 text-gray-400">Build Date:</td>
                <td class="py-1 text-white">{buildInfo.buildDate}</td>
              </tr>
              <tr>
                <td class="py-1 text-gray-400">Timestamp:</td>
                <td class="py-1 text-white">{formatTimestamp(buildInfo.timestamp * 1000)}</td>
              </tr>
            </tbody>
          </table>
        </div>
      {/if}

      <!-- Browser compatibility -->
      <div class="mt-4">
        <div class="text-sm text-gray-400 mb-2">Browser Compatibility</div>
        <div class="bg-gray-700/50 p-3 rounded">
          <div class="flex justify-between items-center mb-2">
            <span class="text-gray-300">WebAssembly Support</span>
            {#if typeof WebAssembly === 'object' && typeof WebAssembly.instantiate === 'function'}
              <span class="text-green-400">Supported</span>
            {:else}
              <span class="text-red-400">Not Supported</span>
            {/if}
          </div>

          <table class="w-full text-xs">
            <tbody>
              <tr>
                <td class="text-gray-400 pr-2 py-0.5 align-top">User Agent:</td>
                <td class="break-words py-0.5">{navigator.userAgent}</td>
              </tr>
              <tr>
                <td class="text-gray-400 pr-2 py-0.5">Platform:</td>
                <td class="py-0.5">{navigator.platform}</td>
              </tr>
              <tr>
                <td class="text-gray-400 pr-2 py-0.5">Hardware Concurrency:</td>
                <td class="py-0.5">{navigator.hardwareConcurrency || 'Unknown'}</td>
              </tr>
              {#if 'deviceMemory' in navigator}
                <tr>
                  <td class="text-gray-400 pr-2 py-0.5">Device Memory (est.):</td>
                  <!-- @ts-ignore - deviceMemory not in standard Navigator type -->
                  <td class="py-0.5">~{navigator.deviceMemory}GB</td>
                </tr>
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  /* Add styles for chart animations if needed */
  svg polyline {
    transition: points 0.3s ease-out;
  }
  /* Style for details summary */
  details > summary {
      list-style: none; /* Remove default marker */
  }
  details > summary::-webkit-details-marker {
      display: none; /* Remove default marker for Chrome */
  }
  details > summary::before {
      content: '▶ '; /* Add custom marker */
      font-size: 0.8em;
  }
  details[open] > summary::before {
      content: '▼ '; /* Change marker when open */
  }
</style>