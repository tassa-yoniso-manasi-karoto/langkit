<!-- WasmPerformanceDashboard.svelte (Simplified Version) -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { getWasmState, resetWasmMetrics, getWasmModule } from '../lib/wasm';
  import { formatBytes, formatTime } from '../lib/utils';
  import { settings } from '../lib/stores';
  import { get } from 'svelte/store';

  // Component state
  let wasmState = getWasmState();
  let updateInterval: number | null = null;
  let memoryUtilizationHistory: {timestamp: number, utilization: number}[] = [];

  // Simplified tab structure
  let selectedTab = 'performance'; // 'performance', 'memory', 'settings'

  // Update metrics periodically
  onMount(() => {
    updateWasmState();
    updateInterval = window.setInterval(updateWasmState, 1000);

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
    wasmState = getWasmState();

    // Add point to memory history (limited to 60 points = 1 minute)
    if (wasmState?.memoryUsage?.utilization) {
      if (memoryUtilizationHistory.length >= 60) {
        memoryUtilizationHistory.shift();
      }

      memoryUtilizationHistory.push({
        timestamp: Date.now(),
        utilization: wasmState.memoryUsage.utilization
      });
    }
  }

  function handleResetMetrics() {
    resetWasmMetrics();
    updateWasmState();
  }

  // Helper functions for formatting
  function getPerformanceClass(ratio: number): string {
    if (ratio >= 5) return 'text-green-400';
    if (ratio >= 3) return 'text-green-500';
    if (ratio >= 1.5) return 'text-yellow-400';
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

  // Force garbage collection
  function forceGarbageCollection() {
    try {
      const wasmModule = getWasmModule();
      if (!wasmModule || typeof wasmModule.force_garbage_collection !== 'function') {
        return;
      }

      wasmModule.force_garbage_collection();

      // Refresh state after GC
      setTimeout(updateWasmState, 100);
    } catch (e) {
      console.error("Failed to perform garbage collection:", e);
    }
  }

  // Update settings
  function updateWasmThreshold(event: Event) {
    const input = event.target as HTMLInputElement;
    const newValue = parseInt(input.value, 10);

    if (!isNaN(newValue)) {
      settings.update($settings => ({
        ...$settings,
        wasmSizeThreshold: newValue
      }));
    }
  }
</script>

<div class="bg-gray-800/60 backdrop-blur-sm rounded-lg p-4 shadow-md text-white">
  <div class="flex items-center justify-between mb-3">
    <h3 class="text-lg font-medium">WebAssembly Performance</h3>

    <!-- Tab Buttons -->
    <div class="flex gap-2">
      <button
        class="px-2 py-1 bg-primary/20 hover:bg-primary/30 text-white text-xs rounded transition-colors"
        class:bg-primary={selectedTab === 'performance'}
        on:click={() => selectedTab = 'performance'}
      >
        Performance
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
        class:bg-primary={selectedTab === 'settings'}
        on:click={() => selectedTab = 'settings'}
      >
        Settings
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

  <!-- Performance Tab -->
  {#if selectedTab === 'performance'}
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

      <!-- Operation Stats -->
      <div class="bg-gray-700/70 rounded p-4 col-span-1 md:col-span-2">
        <div class="text-sm text-gray-400 mb-2">Operation Statistics</div>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="text-left text-xs text-gray-500">
                <th class="pb-2">Operation</th>
                <th class="pb-2">Count</th>
                <th class="pb-2">Avg. Time</th>
                <th class="pb-2">Speedup</th>
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
                  </tr>
                {/each}
              {:else}
                <tr>
                  <td colspan="4" class="py-4 text-center text-gray-500">No operations tracked yet</td>
                </tr>
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  {/if}

  <!-- Memory Tab -->
  {#if selectedTab === 'memory'}
    <div class="grid grid-cols-1 gap-4">
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

            <div class="flex gap-2 mt-3">
              <button
                class="px-3 py-1 bg-blue-600/50 hover:bg-blue-500/60 text-white text-xs rounded flex-1"
                on:click={forceGarbageCollection}
              >
                Clean Up Memory
              </button>
            </div>
          </div>
        {:else}
          <div class="text-gray-400">Memory information not available</div>
        {/if}
      </div>

      <!-- Memory Utilization Chart -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Memory History</div>
        <div class="h-40 w-full">
          <!-- Memory chart visualization -->
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
    </div>
  {/if}

  <!-- Settings Tab -->
  {#if selectedTab === 'settings'}
    <div class="grid grid-cols-1 gap-4">
      <!-- WebAssembly Settings -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">Performance Thresholds</div>

        <!-- WASM Mode -->
        <div class="mb-4">
          <label class="text-xs text-gray-400 block mb-1">WebAssembly Mode</label>
          <select
            class="w-full p-2 bg-gray-800 text-white text-sm rounded border border-primary/20"
            bind:value={$settings.forceWasmMode}
          >
            <option value="auto">Auto (Based on threshold)</option>
            <option value="enabled">Always Enabled</option>
            <option value="disabled">Always Disabled</option>
          </select>
          <div class="text-xs text-gray-500 mt-1">
            Controls when WebAssembly optimization is used
          </div>
        </div>

        <!-- WASM Threshold -->
        <div class="mb-4" class:opacity-50={$settings.forceWasmMode !== 'auto'}>
          <label class="text-xs text-gray-400 block mb-1">
            WebAssembly Size Threshold:
            <span class="text-primary ml-1">{$settings.wasmSizeThreshold} logs</span>
          </label>
          <div class="flex items-center gap-2">
            <span class="text-xs text-gray-500">100</span>
            <input
              type="range"
              min="100"
              max="5000"
              step="100"
              value={$settings.wasmSizeThreshold}
              on:change={updateWasmThreshold}
              disabled={$settings.forceWasmMode !== 'auto'}
              class="flex-1"
            />
            <span class="text-xs text-gray-500">5000</span>
          </div>
          <div class="text-xs text-gray-500 mt-1">
            Use WebAssembly for operations with more than {$settings.wasmSizeThreshold} logs
          </div>
        </div>

        <!-- LogViewer Virtualization Threshold -->
        <div class="mb-4">
          <label class="text-xs text-gray-400 block mb-1">
            LogViewer Virtualization Threshold:
            <span class="text-primary ml-1">{$settings.logViewerVirtualizationThreshold} logs</span>
          </label>
          <div class="flex items-center gap-2">
            <span class="text-xs text-gray-500">500</span>
            <input
              type="range"
              min="500"
              max="10000"
              step="500"
              bind:value={$settings.logViewerVirtualizationThreshold}
              class="flex-1"
            />
            <span class="text-xs text-gray-500">10000</span>
          </div>
          <div class="text-xs text-gray-500 mt-1">
            Enable virtualization when log count exceeds this threshold
          </div>
        </div>
      </div>

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
                    <td class="py-2">{formatTime(Math.max(0, entry.nextRetryTime - now))}</td>
                    <td class="py-2 text-xs">{entry.lastError || 'Unknown error'}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      {/if}

      <!-- Informational Card -->
      <div class="bg-primary/10 rounded p-4">
        <div class="text-sm text-primary-300 mb-2">About WebAssembly Optimization</div>
        <p class="text-xs text-gray-300 mb-2">
          WebAssembly acceleration provides significant performance improvements for operations
          with large numbers of logs. The performance benefit increases with log volume:
        </p>
        <ul class="text-xs text-gray-300 list-disc pl-5 space-y-1">
          <li>Small datasets (≤500 logs): 1.2-1.5× faster</li>
          <li>Medium datasets (500-2,000 logs): 2-3× faster</li>
          <li>Large datasets (2,000-5,000 logs): 5-7× faster</li>
          <li>Extra large datasets (>5,000 logs): 8-10× faster</li>
        </ul>
      </div>
    </div>
  {/if}
</div>