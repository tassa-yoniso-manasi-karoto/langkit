<!-- src/components/WasmPerformanceDashboard.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { getWasmState, resetWasmMetrics, getWasmModule } from '../lib/wasm'; 
  import { WasmInitStatus } from '../lib/wasm-state';
  import type { WasmState } from '../lib/wasm-state';
  import { formatBytes, formatTime } from '../lib/utils';
  import { wasmLogger, WasmLogLevel } from '../lib/wasm-logger';
  import { settings } from '../lib/stores';
  import { get } from 'svelte/store';

  // Component state
  let wasmState: WasmState = getWasmState();
  let updateInterval: number | null = null;
  let memoryUtilizationHistory: {timestamp: number, utilization: number}[] = [];

  // Simplified tab structure
  let selectedTab = 'overview'; // 'overview', 'memory', 'operations', 'diagnostics', 'adaptive'

  // Import the adjustSizeThresholds function
  import { adjustSizeThresholds } from '../lib/wasm';
  // Need lastThresholdAdjustment to force update if needed
  // This is a bit hacky, ideally the state management would handle this better
  let lastThresholdAdjustment = 0;

  // Update metrics periodically
  onMount(() => {
    updateWasmState();
    updateInterval = window.setInterval(updateWasmState, 1000); // Update every second
    
    // Start tracking memory utilization for the chart
    if (wasmState?.memoryUsage?.utilization) {
      memoryUtilizationHistory.push({
        timestamp: Date.now(),
        utilization: wasmState.memoryUsage.utilization
      });
    }
  });

  onDestroy(() => {
    if (updateInterval) {
      clearInterval(updateInterval);
    }
  });

  function updateWasmState() {
    wasmState = getWasmState(); // Fetch latest state
    
    // Add point to memory history (limited to 60 points = 1 minute)
    if (wasmState?.memoryUsage?.utilization) {
      if (memoryUtilizationHistory.length >= 60) {
        memoryUtilizationHistory.shift(); // Remove oldest
      }
      
      memoryUtilizationHistory.push({
        timestamp: Date.now(),
        utilization: wasmState.memoryUsage.utilization
      });
    }
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
    if (value === undefined || value === null) return 'N/A';
    return value.toFixed(precision);
  }

  // Get color class based on memory utilization
  function getMemoryUtilizationClass(utilization: number): string {
    if (utilization >= 0.9) return 'bg-red-500';
    if (utilization >= 0.75) return 'bg-yellow-500';
    return 'bg-green-500';
  }

  // Force garbage collection
  function forceGarbageCollection() {
    const currentState = getWasmState();

    if (currentState?.initStatus !== WasmInitStatus.SUCCESS) {
      wasmLogger.log(
        WasmLogLevel.WARN,
        'memory',
        'Cannot force GC: WebAssembly not initialized successfully'
      );
      return;
    }

    try {
      const wasmModule = getWasmModule();
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

  // Request memory growth
  function requestMemoryGrowth() {
    const currentState = getWasmState();

    if (currentState?.initStatus !== WasmInitStatus.SUCCESS) {
      return;
    }

    try {
      const wasmModule = getWasmModule();
      if (!wasmModule || typeof wasmModule.ensure_sufficient_memory !== 'function') {
        wasmLogger.log(
          WasmLogLevel.WARN,
          'memory',
          'ensure_sufficient_memory function not found in WASM module'
        );
        return;
      }

      // Request an additional 4MB of memory (conservative)
      const success = wasmModule.ensure_sufficient_memory(4 * 1024 * 1024); 
      
      if (success) {
        wasmLogger.log(
          WasmLogLevel.INFO,
          'memory',
          'Manual memory growth performed successfully',
          { source: 'user_action' }
        );
      } else {
        wasmLogger.log(
          WasmLogLevel.WARN,
          'memory',
          'Manual memory growth request failed',
          { source: 'user_action' }
        );
      }

      // Refresh state after growth attempt to show updated memory usage
      setTimeout(updateWasmState, 100);
    } catch (e: any) {
      wasmLogger.log(
        WasmLogLevel.ERROR,
        'memory',
        `Failed to perform memory growth: ${e.message}`
      );
    }
  }
  
  // Get status badge class
  function getStatusBadgeClass(status: string): string {
    switch(status) {
      case 'growth_succeeded': return 'bg-green-500 text-white';
      case 'growth_failed': return 'bg-red-500 text-white';
      case 'gc_succeeded': return 'bg-blue-500 text-white';
      case 'error': return 'bg-red-600 text-white';
      case 'normal_operation': return 'bg-gray-500 text-white';
      default: return 'bg-gray-700 text-white';
    }
  }
  
  // Get simple date format for timestamps
  function formatDate(timestamp: number): string {
    return new Date(timestamp).toLocaleTimeString();
  }
  
  // Settings store ($settings) is accessed directly in the template via auto-subscription
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
        class="px-2 py-1 bg-primary/20 hover:bg-primary/30 text-white text-xs rounded transition-colors"
        class:bg-primary={selectedTab === 'diagnostics'}
        on:click={() => selectedTab = 'diagnostics'}
      >
        Diagnostics
      </button>

      <button
        class="px-2 py-1 bg-primary/20 hover:bg-primary/30 text-white text-xs rounded transition-colors"
        class:bg-primary={selectedTab === 'adaptive'}
        on:click={() => selectedTab = 'adaptive'}
      >
        Adaptive
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

  <!-- Overview Tab -->
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
        <div class="flex justify-between items-center">
          <div class="text-sm text-gray-400 mb-2">Memory Usage</div>
          <div class="text-xs text-gray-500">
            <span class="font-medium">Mode:</span> 
            <span class="ml-1 text-white">
              {($settings as any)?.forceWasmMode === 'enabled' ? 'Forced' : 
                ($settings as any)?.forceWasmMode === 'disabled' ? 'Disabled' : 'Auto'}
            </span>
          </div>
        </div>
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

            <div class="flex gap-2 mt-3">
              <button
                class="px-3 py-1 bg-blue-600/50 hover:bg-blue-500/60 text-white text-xs rounded flex-1"
                on:click={forceGarbageCollection}
                disabled={wasmState?.initStatus !== WasmInitStatus.SUCCESS}
              >
                Clean Up Memory
              </button>
              
              <button
                class="px-3 py-1 bg-purple-600/50 hover:bg-purple-500/60 text-white text-xs rounded flex-1"
                on:click={requestMemoryGrowth}
                disabled={wasmState?.initStatus !== WasmInitStatus.SUCCESS}
              >
                Request More Memory
              </button>
            </div>
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

  <!-- Memory Tab -->
  {#if selectedTab === 'memory'}
    <div class="grid grid-cols-1 gap-4">
      <!-- Memory Utilization Chart -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Memory Utilization History</div>
        <div class="h-40 w-full">
          <!-- Simple memory chart visualization -->
          <div class="relative w-full h-full bg-gray-800/80 rounded border border-gray-600">
            {#if memoryUtilizationHistory.length > 0}
              <!-- Draw axis lines -->
              <div class="absolute left-0 bottom-0 w-full h-px bg-gray-600"></div>
              <div class="absolute left-0 bottom-[33%] w-full h-px bg-gray-700/50"></div>
              <div class="absolute left-0 bottom-[67%] w-full h-px bg-gray-700/50"></div>
              
              <!-- Draw axis labels -->
              <div class="absolute left-1 bottom-[calc(100%-18px)] text-xs text-gray-500">100%</div>
              <div class="absolute left-1 bottom-[calc(67%-9px)] text-xs text-gray-500">67%</div>
              <div class="absolute left-1 bottom-[calc(33%-9px)] text-xs text-gray-500">33%</div>
              <div class="absolute left-1 bottom-[-8px] text-xs text-gray-500">0%</div>
              
              <!-- Draw utilization line -->
              <svg class="absolute inset-0 w-full h-full overflow-visible" preserveAspectRatio="none">
                <polyline
                  points="{memoryUtilizationHistory.map((point, index) => 
                    `${(index / (memoryUtilizationHistory.length - 1)) * 100}%,${100 - (point.utilization * 100)}%`
                  ).join(' ')}"
                  fill="none"
                  stroke="rgba(147, 51, 234, 0.7)"
                  stroke-width="2"
                  vector-effect="non-scaling-stroke"
                />
              </svg>
              
              <!-- Draw danger threshold line at 90% -->
              <div class="absolute left-0 right-0 bottom-[90%] h-px bg-red-500/70"></div>
              <div class="absolute right-1 bottom-[calc(90%-9px)] text-xs text-red-500/70">90%</div>

              <!-- Utilization markers -->
              {#each memoryUtilizationHistory as point, i}
                {#if i % 5 === 0 || i === memoryUtilizationHistory.length - 1}
                  <div
                    class="absolute w-2 h-2 rounded-full"
                    class:bg-green-500={point.utilization < 0.7}
                    class:bg-yellow-500={point.utilization >= 0.7 && point.utilization < 0.9}
                    class:bg-red-500={point.utilization >= 0.9}
                    style="bottom: {point.utilization * 100}%; left: {(i / (memoryUtilizationHistory.length - 1)) * 100}%; 
                           transform: translate(-50%, 50%);"
                  ></div>
                {/if}
              {/each}
            {:else}
              <div class="flex items-center justify-center h-full text-gray-500">No memory data available</div>
            {/if}
          </div>
        </div>
      </div>
      
      <!-- Memory Details -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Memory Details</div>
        <div class="grid grid-cols-2 md:grid-cols-3 gap-4">
          <div>
            <div class="text-xs text-gray-500">Total Memory</div>
            <div class="text-sm font-bold text-white">{formatBytes(wasmState?.memoryUsage?.total || 0)}</div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Used Memory</div>
            <div class="text-sm font-bold text-white">{formatBytes(wasmState?.memoryUsage?.used || 0)}</div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Peak Usage</div>
            <div class="text-sm font-bold text-white">{formatBytes(wasmState?.memoryUsage?.peak_bytes || 0)}</div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Allocations</div>
            <div class="text-sm font-bold text-white">{wasmState?.memoryUsage?.allocation_count || 0}</div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Avg. Allocation</div>
            <div class="text-sm font-bold text-white">{formatBytes(wasmState?.memoryUsage?.average_allocation || 0)}</div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Memory Growth</div>
            <div class="text-sm font-bold text-white">{wasmState?.memoryGrowthEvents?.length || 0} events</div>
          </div>
        </div>
      </div>
      
      <!-- Memory Growth Events -->
      {#if wasmState?.memoryGrowthEvents && wasmState.memoryGrowthEvents.length > 0}
        <div class="bg-gray-700/70 rounded p-4">
          <div class="text-sm text-gray-400 mb-2">Memory Growth Events</div>
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="text-left text-xs text-gray-500">
                  <th class="pb-2">Time</th>
                  <th class="pb-2">Requested</th>
                  <th class="pb-2">Before</th>
                  <th class="pb-2">After</th>
                  <th class="pb-2">Status</th>
                  <th class="pb-2">Reason</th>
                </tr>
              </thead>
              <tbody>
                {#each wasmState.memoryGrowthEvents as event}
                  <tr class="border-t border-gray-600/30">
                    <td class="py-2 text-xs">{formatDate(event.timestamp)}</td>
                    <td class="py-2">{formatBytes(event.requestedBytes)}</td>
                    <td class="py-2">{event.beforePages} pages</td>
                    <td class="py-2">{event.afterPages || '—'} pages</td>
                    <td class="py-2">
                      <span class="px-2 py-0.5 rounded text-xs {event.success ? 'bg-green-500/70' : 'bg-red-500/70'}">
                        {event.success ? 'Success' : 'Failed'}
                      </span>
                    </td>
                    <td class="py-2 text-xs text-gray-300">{event.reason}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      {/if}
      
      <!-- Memory Thresholds -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Memory Thresholds</div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <div class="text-xs text-gray-500">Low Risk (0-85%)</div>
            <div class="h-2 w-full bg-gray-600 rounded-full mt-1">
              <div class="h-2 rounded-full bg-green-500" style="width: 85%"></div>
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Moderate Risk (85-95%)</div>
            <div class="h-2 w-full bg-gray-600 rounded-full mt-1">
              <div class="h-2 rounded-full bg-yellow-500" style="width: 10%"></div>
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500">High Risk (95-100%)</div>
            <div class="h-2 w-full bg-gray-600 rounded-full mt-1">
              <div class="h-2 rounded-full bg-red-500" style="width: 5%"></div>
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Current</div>
            <div class="h-2 w-full bg-gray-600 rounded-full mt-1">
              <div class="h-2 rounded-full {getMemoryUtilizationClass(wasmState?.memoryUsage?.utilization || 0)}" 
                   style="width: {(wasmState?.memoryUsage?.utilization || 0) * 100}%"></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- Operations Tab -->
  {#if selectedTab === 'operations'}
    <div class="grid grid-cols-1 gap-4">
      <!-- Operation Statistics -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Operation Statistics</div>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="text-left text-xs text-gray-500">
                <th class="pb-2">Operation</th>
                <th class="pb-2">Count</th>
                <th class="pb-2">Avg. Time</th>
                <th class="pb-2">WebAssembly vs. TS</th>
                <th class="pb-2">Success Rate</th>
              </tr>
            </thead>
            <tbody>
              {#if wasmState?.operationsPerType && Object.keys(wasmState.operationsPerType).length > 0}
                {#each Object.entries(wasmState.operationsPerType) as [operation, count]}
                  <tr class="border-t border-gray-600/30">
                    <td class="py-2">{operation}</td>
                    <td class="py-2">{count}</td>
                    <td class="py-2">
                      {wasmState.performanceMetrics?.operationTimings?.[operation]?.avgTime 
                        ? formatTime(wasmState.performanceMetrics.operationTimings[operation].avgTime)
                        : '—'}
                    </td>
                    <td class="py-2">
                      {#if wasmState.performanceMetrics?.operationTimings?.[operation] && wasmState.performanceMetrics?.avgTsTime > 0}
                        <span class="{getPerformanceClass(wasmState.performanceMetrics.avgTsTime / wasmState.performanceMetrics.operationTimings[operation].avgTime)}">
                          {formatNumber(wasmState.performanceMetrics.avgTsTime / wasmState.performanceMetrics.operationTimings[operation].avgTime)}×
                        </span>
                      {:else}
                        —
                      {/if}
                    </td>
                    <td class="py-2">
                      {#if wasmState.fallbackReasons && Object.keys(wasmState.fallbackReasons).length > 0}
                        {#if wasmState.fallbackReasons[operation] > 0}
                          <span class="text-yellow-400">
                            {Math.round((1 - wasmState.fallbackReasons[operation] / count) * 100)}%
                          </span>
                        {:else}
                          <span class="text-green-400">100%</span>
                        {/if}
                      {:else}
                        <span class="text-green-400">100%</span>
                      {/if}
                    </td>
                  </tr>
                {/each}
              {:else}
                <tr>
                  <td colspan="5" class="py-4 text-center text-gray-500">No operations tracked yet</td>
                </tr>
              {/if}
            </tbody>
          </table>
        </div>
      </div>
      
      <!-- Log Size Distribution -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Log Size Distribution</div>
        <div class="grid grid-cols-3 gap-4">
          <div>
            <div class="text-xs text-gray-500">{'Small (<500)'}</div>
            <div class="text-lg font-bold text-white">{wasmState?.performanceMetrics?.logSizeDistribution?.small || 0}</div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Medium (500-2000)</div>
            <div class="text-lg font-bold text-white">{wasmState?.performanceMetrics?.logSizeDistribution?.medium || 0}</div>
          </div>
          <div>
            <div class="text-xs text-gray-500">{'Large (>2000)'}</div>
            <div class="text-lg font-bold text-white">{wasmState?.performanceMetrics?.logSizeDistribution?.large || 0}</div>
          </div>
        </div>
        
        <!-- Size distribution visualization -->
        <div class="mt-4">
          <div class="w-full h-6 bg-gray-600 rounded-full overflow-hidden flex">
            {#if wasmState?.performanceMetrics?.logSizeDistribution}
              {@const total = (wasmState.performanceMetrics.logSizeDistribution.small || 0) + 
                             (wasmState.performanceMetrics.logSizeDistribution.medium || 0) + 
                             (wasmState.performanceMetrics.logSizeDistribution.large || 0)}
              {#if total > 0}
                <div class="h-full bg-blue-400" style="width: {(wasmState.performanceMetrics.logSizeDistribution.small / total) * 100}%"></div>
                <div class="h-full bg-purple-400" style="width: {(wasmState.performanceMetrics.logSizeDistribution.medium / total) * 100}%"></div>
                <div class="h-full bg-green-400" style="width: {(wasmState.performanceMetrics.logSizeDistribution.large / total) * 100}%"></div>
              {/if}
            {/if}
          </div>
          <div class="flex justify-between text-xs text-gray-500 mt-1">
            <span class="text-blue-400">Small</span>
            <span class="text-purple-400">Medium</span>
            <span class="text-green-400">Large</span>
          </div>
        </div>
      </div>
      
      <!-- Performance Metrics -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Detailed Performance Metrics</div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <div class="text-xs text-gray-500">Avg. WebAssembly Time</div>
            <div class="text-sm font-bold text-white">
              {formatTime(wasmState?.performanceMetrics?.avgWasmTime || 0)}
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Avg. TypeScript Time</div>
            <div class="text-sm font-bold text-white">
              {formatTime(wasmState?.performanceMetrics?.avgTsTime || 0)}
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Avg. Serialization Time</div>
            <div class="text-sm font-bold text-white">
              {formatTime(wasmState?.performanceMetrics?.avgSerializationTime || 0)}
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Avg. Deserialization Time</div>
            <div class="text-sm font-bold text-white">
              {formatTime(wasmState?.performanceMetrics?.avgDeserializationTime || 0)}
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Raw Speedup Ratio</div>
            <div class="text-sm font-bold {getPerformanceClass(wasmState?.performanceMetrics?.speedupRatio || 0)}">
              {formatNumber(wasmState?.performanceMetrics?.speedupRatio || 0)}×
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Net Speedup Ratio</div>
            <div class="text-sm font-bold {getPerformanceClass(wasmState?.performanceMetrics?.netSpeedupRatio || 0)}">
              {formatNumber(wasmState?.performanceMetrics?.netSpeedupRatio || 0)}×
            </div>
          </div>
        </div>
      </div>
    </div>
  {/if}
  
  <!-- Diagnostics Tab -->
  {#if selectedTab === 'diagnostics'}
    <div class="grid grid-cols-1 gap-4">
      <!-- Memory Checks -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Recent Memory Checks</div>
        {#if wasmState?.memoryChecks && wasmState.memoryChecks.length > 0}
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="text-left text-xs text-gray-500">
                  <th class="pb-2">Time</th>
                  <th class="pb-2">Log Count</th>
                  <th class="pb-2">Initial %</th>
                  <th class="pb-2">Actions</th>
                  <th class="pb-2">Final %</th>
                  <th class="pb-2">Outcome</th>
                </tr>
              </thead>
              <tbody>
                {#each wasmState.memoryChecks as check}
                  <tr class="border-t border-gray-600/30">
                    <td class="py-2 text-xs">{formatDate(check.timestamp)}</td>
                    <td class="py-2">{check.logCount}</td>
                    <td class="py-2">{Math.round(check.initialUtilization * 100)}%</td>
                    <td class="py-2 text-xs">
                      {#if check.actions && check.actions.length > 0}
                        {check.actions.map(a => a.replace(/_/g, ' ')).join(', ')}
                      {:else}
                        None
                      {/if}
                    </td>
                    <td class="py-2">{Math.round(check.finalUtilization * 100)}%</td>
                    <td class="py-2">
                      <span class="px-2 py-0.5 rounded text-xs {getStatusBadgeClass(check.outcome)}">
                        {check.outcome.replace(/_/g, ' ')}
                      </span>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {:else}
          <div class="text-gray-500 text-center py-4">No memory checks recorded yet</div>
        {/if}
      </div>
      
      <!-- Fallback Reasons -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Fallback Reasons</div>
        {#if wasmState?.fallbackReasons && Object.keys(wasmState.fallbackReasons).length > 0}
          <div class="overflow-hidden">
            <table class="w-full text-sm">
              <thead>
                <tr class="text-left text-xs text-gray-500">
                  <th class="pb-2">Reason</th>
                  <th class="pb-2">Count</th>
                  <th class="pb-2">Distribution</th>
                </tr>
              </thead>
              <tbody>
                {#each Object.entries(wasmState.fallbackReasons) as [reason, count]}
                  {@const total = Object.values(wasmState.fallbackReasons).reduce((sum, c) => sum + (c as number), 0)}
                  <tr class="border-t border-gray-600/30">
                    <td class="py-2">{reason.replace(/_/g, ' ')}</td>
                    <td class="py-2">{count}</td>
                    <td class="py-2 relative">
                      <div class="w-full h-2 bg-gray-600 rounded-full">
                        <div class="h-2 bg-yellow-500 rounded-full" style="width: {(count as number) / total * 100}%"></div>
                      </div>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {:else}
          <div class="text-gray-500 text-center py-4">No fallbacks recorded yet</div>
        {/if}
      </div>
      
      <!-- Configuration Status -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Configuration</div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <div class="text-xs text-gray-500">WebAssembly Mode</div>
            <div class="text-sm font-bold text-white">
              {($settings as any)?.forceWasmMode === 'enabled' ? 'Forced Enabled' : 
               ($settings as any)?.forceWasmMode === 'disabled' ? 'Forced Disabled' : 'Automatic'}
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Size Threshold</div>
            <div class="text-sm font-bold text-white">
              {($settings as any)?.wasmSizeThreshold || 'Default'} logs
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Initialization Time</div>
            <div class="text-sm font-bold text-white">
              {wasmState?.initTime ? formatTime(wasmState.initTime) : 'N/A'}
            </div>
          </div>
          <div>
            <div class="text-xs text-gray-500">Last Operation</div>
            <div class="text-sm font-bold text-white">
              {wasmState?.lastUsed ? formatDate(wasmState.lastUsed) : 'Never'}
            </div>
          </div>
        </div>
      </div>
      
      <!-- Error Information -->
      {#if wasmState?.lastError}
        <div class="bg-red-800/40 rounded p-4 border border-red-500/30">
          <div class="text-sm text-red-300 mb-2">Last Error</div>
          <div class="text-white font-bold mb-1">{wasmState.lastError.name}</div>
          <div class="text-sm text-white/80">{wasmState.lastError.message}</div>
        </div>
      {/if}
      
      <!-- Blacklisted Operations -->
      {#if wasmState?.blacklistedOperations && wasmState.blacklistedOperations.length > 0}
        <div class="bg-red-800/30 rounded p-4 border border-red-500/30">
          <div class="text-sm text-red-300 mb-2">Blacklisted Operations</div>
          <div class="overflow-x-auto">
            <table class="w-full text-sm">
              <thead>
                <tr class="text-left text-xs text-gray-500">
                  <th class="pb-2">Operation</th>
                  <th class="pb-2">Failures</th>
                  <th class="pb-2">Blacklisted For</th>
                  <th class="pb-2">Retry In</th>
                  <th class="pb-2">Error</th>
                </tr>
              </thead>
              <tbody>
                {#each wasmState.blacklistedOperations as entry}
                  {@const now = Date.now()}
                  <tr class="border-t border-gray-600/30">
                    <td class="py-2">{entry.operation}</td>
                    <td class="py-2">{entry.retryCount}x</td>
                    <td class="py-2">{formatTime(now - entry.timestamp)}</td>
                    <td class="py-2">{formatTime(Math.max(0, entry.nextRetryTime - now))}</td>
                    <td class="py-2 text-xs">{entry.lastError || 'Unknown error'}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      {/if}

      <!-- Adaptive Learning Section -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Adaptive Learning</div>
        <div class="flex flex-col">
          <div class="text-sm mb-2">
            <span class="text-gray-400">Current Threshold:</span>
            <span class="font-bold ml-2">{($settings as any)?.wasmSizeThreshold || 'Default'} logs</span>
          </div>
          <div class="flex gap-2">
            <button
              class="px-3 py-1 bg-purple-600/50 hover:bg-purple-500/60 text-white text-xs rounded"
              on:click={() => {
                // Manual trigger for threshold adjustment
                try {
                  // Temporarily reset last adjustment time to force an update check
                  // This is a workaround; ideally, the adjust function would return status
                  lastThresholdAdjustment = 0;
                  const adjusted = adjustSizeThresholds();
                  if (!adjusted) {
                    wasmLogger.log(WasmLogLevel.INFO, 'adaptive', 'Manual threshold adjustment check: No adjustment needed based on current metrics.');
                    // Optionally provide user feedback here (e.g., via a notification)
                  } else {
                     wasmLogger.log(WasmLogLevel.INFO, 'adaptive', 'Manual threshold adjustment check: Adjustment applied.');
                  }
                  // Refresh state to show potential changes
                  updateWasmState();
                } catch (e: any) { // Catch potential errors during adjustment
                   wasmLogger.log(WasmLogLevel.ERROR, 'adaptive', `Manual threshold adjustment failed: ${e.message}`);
                   console.error("Failed to adjust thresholds:", e);
                }
              }}
              disabled={wasmState?.initStatus !== WasmInitStatus.SUCCESS}
            >
              Analyze & Adjust Thresholds Now
            </button>
          </div>
        </div>
      </div>

    </div>
  {/if}

  <!-- Adaptive Tab -->
  {#if selectedTab === 'adaptive'}
    <div class="grid grid-cols-1 gap-4">
       <!-- Threshold Adjustments -->
       <div class="bg-gray-700/70 rounded p-4">
         <div class="text-sm text-gray-400 mb-2">Size Threshold Adjustments</div>
         {#if wasmState?.thresholdAdjustments && wasmState.thresholdAdjustments.length > 0}
           <div class="overflow-x-auto">
             <table class="w-full text-sm">
               <thead>
                 <tr class="text-left text-xs text-gray-500">
                   <th class="pb-2">Time</th>
                   <th class="pb-2">Previous</th>
                   <th class="pb-2">New</th>
                   <th class="pb-2">Change</th>
                   <th class="pb-2">Reason</th>
                 </tr>
               </thead>
               <tbody>
                 {#each wasmState.thresholdAdjustments as adjustment}
                   <tr class="border-t border-gray-600/30">
                     <td class="py-2">{formatDate(adjustment.timestamp)}</td>
                     <td class="py-2">{adjustment.previousThreshold} logs</td>
                     <td class="py-2">{adjustment.newThreshold} logs</td>
                     <td class="py-2">
                       <span class="{adjustment.newThreshold > adjustment.previousThreshold
                         ? 'text-red-400' : 'text-green-400'}">
                         {Math.round((adjustment.newThreshold - adjustment.previousThreshold) /
                           adjustment.previousThreshold * 100)}%
                       </span>
                     </td>
                     <td class="py-2">{adjustment.reason.replace(/_/g, ' ')}</td>
                   </tr>
                 {/each}
               </tbody>
             </table>
           </div>
         {:else}
           <div class="text-gray-500 text-center py-4">No threshold adjustments recorded yet</div>
         {/if}
       </div>
    </div>
  {/if}
</div>