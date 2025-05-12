<!-- WasmPerformanceDashboard.svelte (Simplified Version) -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { getWasmState, resetWasmMetrics, getWasmModule } from '../lib/wasm';
  import { formatBytes, formatTime } from '../lib/utils';
  import { settings } from '../lib/stores';
  import { get } from 'svelte/store';
  import { wasmLogger, WasmLogLevel } from '../lib/wasm-logger';

  // Component state
  let wasmState = getWasmState();
  let updateInterval: number | null = null;
  let memoryUtilizationHistory: {timestamp: number, utilization: number}[] = [];

  // Simplified tab structure
  let selectedTab = 'performance'; // 'performance', 'memory', 'settings'

  // Update metrics periodically
  onMount(() => {
    wasmLogger.log(WasmLogLevel.INFO, 'dashboard', 'Initializing WebAssembly performance dashboard');

    // Function to do a complete memory refresh from the WASM module
    const refreshMemoryInfo = () => {
      try {
        const wasmModule = getWasmModule();
        if (!wasmModule || !wasmModule.get_memory_usage) {
          return false;
        }

        // Get fresh memory info directly from WASM
        const memInfo = wasmModule.get_memory_usage();
        if (!memInfo) {
          return false;
        }

        // Apply memory info directly to wasmState
        wasmState = {
          ...wasmState,
          memoryUsage: memInfo
        };

        return true;
      } catch (e) {
        wasmLogger.log(WasmLogLevel.ERROR, 'dashboard', `Error refreshing memory info: ${e.message}`);
        return false;
      }
    };

    // Initial update
    updateWasmState();

    // Force an immediate memory refresh
    if (refreshMemoryInfo()) {
      wasmLogger.log(WasmLogLevel.INFO, 'dashboard', 'Initial memory info updated');
    }

    // Set up faster update interval (250ms)
    updateInterval = window.setInterval(() => {
      // First update from the internal state cache
      updateWasmState();

      // Then get a fresh direct reading from WASM
      refreshMemoryInfo();

    }, 250); // Higher frequency for more responsive updates

    // Start tracking memory utilization for the chart
    if (wasmState?.memoryUsage) {
      const utilization = getMemoryUtilization(wasmState.memoryUsage);
      memoryUtilizationHistory.push({
        timestamp: Date.now(),
        utilization
      });
    }
  });

  // Helper to safely get memory utilization
  function getMemoryUtilization(memoryUsage: any): number {
    // Debug output to help diagnose memory issues
    wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', 'Calculating memory utilization');
    
    // Exit early if memory usage is null or undefined
    if (!memoryUsage) {
      wasmLogger.log(WasmLogLevel.WARN, 'dashboard', 'Memory usage is null or undefined');
      return 0;
    }
    
    // First check if memoryUsage is a Map
    const isMap = Object.prototype.toString.call(memoryUsage) === '[object Map]';
    wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', `Memory usage is a Map? ${isMap}`);

    if (isMap) {
      // Access using Map.get() method
      try {
        // First try the utilization_estimate field (new field from Rust)
        const utilization_estimate = (memoryUsage as Map<string, any>).get('utilization_estimate');
        if (typeof utilization_estimate === 'number') {
          wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', `Using utilization_estimate: ${utilization_estimate}`);
          return utilization_estimate;
        }

        // Then try the utilization field (field from wasm-state.ts)
        const utilization = (memoryUsage as Map<string, any>).get('utilization');
        if (typeof utilization === 'number') {
          wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', `Using utilization: ${utilization}`);
          return utilization;
        }

        // Calculate utilization from tracked and total bytes if available
        const tracked_bytes = (memoryUsage as Map<string, any>).get('tracked_bytes');
        const total_bytes = (memoryUsage as Map<string, any>).get('total_bytes');
        if (typeof tracked_bytes === 'number' &&
            typeof total_bytes === 'number' &&
            total_bytes > 0) {
          const calculated = tracked_bytes / total_bytes;
          wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', `Calculated from tracked/total: ${calculated.toFixed(4)}`);
          return calculated;
        }
        
        // Try used bytes if tracked not available
        const used_bytes = (memoryUsage as Map<string, any>).get('used_bytes');
        if (typeof used_bytes === 'number' &&
            typeof total_bytes === 'number' &&
            total_bytes > 0) {
          const calculated = used_bytes / total_bytes;
          wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', `Calculated from used/total: ${calculated.toFixed(4)}`);
          return calculated;
        }
      } catch (e) {
        wasmLogger.log(WasmLogLevel.ERROR, 'dashboard', `Error accessing Map properties: ${e.message}`);
      }
    } else {
      // Regular object property access
      try {
        // First try the utilization_estimate field (new field from Rust)
        if (typeof memoryUsage.utilization_estimate === 'number') {
          wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', `Using utilization_estimate: ${memoryUsage.utilization_estimate}`);
          return memoryUsage.utilization_estimate;
        }

        // Then try the utilization field (field from wasm-state.ts)
        if (typeof memoryUsage.utilization === 'number') {
          wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', `Using utilization: ${memoryUsage.utilization}`);
          return memoryUsage.utilization;
        }

        // Calculate utilization from tracked and total bytes if available
        if (typeof memoryUsage.tracked_bytes === 'number' &&
            typeof memoryUsage.total_bytes === 'number' &&
            memoryUsage.total_bytes > 0) {
          const calculated = memoryUsage.tracked_bytes / memoryUsage.total_bytes;
          wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', `Calculated from tracked/total: ${calculated.toFixed(4)}`);
          return calculated;
        }

        // Calculate utilization from used and total bytes if available
        if (typeof memoryUsage.used_bytes === 'number' &&
            typeof memoryUsage.total_bytes === 'number' &&
            memoryUsage.total_bytes > 0) {
          const calculated = memoryUsage.used_bytes / memoryUsage.total_bytes;
          wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', `Calculated from used/total: ${calculated.toFixed(4)}`);
          return calculated;
        }

        // Calculate utilization from used and total fields (older format)
        if (typeof memoryUsage.used === 'number' &&
            typeof memoryUsage.total === 'number' &&
            memoryUsage.total > 0) {
          const calculated = memoryUsage.used / memoryUsage.total;
          wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', `Calculated from used/total (legacy): ${calculated.toFixed(4)}`);
          return calculated;
        }
      } catch (e) {
        wasmLogger.log(WasmLogLevel.ERROR, 'dashboard', `Error accessing object properties: ${e.message}`);
      }
    }

    // Log all available properties for debugging
    try {
      const properties: Record<string, string> = {};
      
      if (isMap) {
        (memoryUsage as Map<string, any>).forEach((value, key) => {
          properties[key] = `${value} (${typeof value})`;
        });
      } else {
        Object.keys(memoryUsage).forEach(key => {
          properties[key] = `${memoryUsage[key]} (${typeof memoryUsage[key]})`;
        });
      }
      
      wasmLogger.log(WasmLogLevel.DEBUG, 'dashboard', 'Available memory properties', properties);
    } catch (e) {
      wasmLogger.log(WasmLogLevel.ERROR, 'dashboard', `Error logging properties: ${e.message}`);
    }

    // Default fallback to avoid NaN
    wasmLogger.log(WasmLogLevel.WARN, 'dashboard', 'Using default utilization fallback: 0');
    return 0;
  }

  function updateWasmState() {
    // Get latest WASM state
    wasmState = getWasmState();
    wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', 'Updated WASM state');

    // Get fresh memory information directly from the WASM module
    try {
      const wasmModule = getWasmModule();
      if (wasmModule && wasmModule.get_memory_usage) {
        // Get fresh memory info
        const freshMemoryInfo = wasmModule.get_memory_usage();
        wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', 'Retrieved fresh memory info from module');

        // Override the memoryUsage in wasmState with fresh data
        if (freshMemoryInfo) {
          wasmState = {
            ...wasmState,
            memoryUsage: freshMemoryInfo
          };
          wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', 'Updated wasmState with fresh memory info');
        }
      }
    } catch (e) {
      wasmLogger.log(WasmLogLevel.ERROR, 'dashboard', `Error getting fresh memory info: ${e.message}`);
    }

    // Add point to memory history (limited to 60 points = 1 minute)
    if (wasmState?.memoryUsage) {
      const utilization = getMemoryUtilization(wasmState.memoryUsage);
      wasmLogger.log(WasmLogLevel.TRACE, 'dashboard', `Current memory utilization: ${utilization.toFixed(4)}`);

      if (memoryUtilizationHistory.length >= 60) {
        memoryUtilizationHistory.shift();
      }

      memoryUtilizationHistory.push({
        timestamp: Date.now(),
        utilization
      });

      // Force Svelte to update the array reference
      memoryUtilizationHistory = [...memoryUtilizationHistory];
    } else {
      wasmLogger.log(WasmLogLevel.WARN, 'dashboard', 'No memoryUsage available in wasmState');
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

  // Reset internal memory tracking statistics
  function resetMemoryTracking() {
    try {
      const wasmModule = getWasmModule();
      if (!wasmModule || typeof wasmModule.reset_internal_allocation_stats !== 'function') {
        wasmLogger.log(WasmLogLevel.ERROR, 'dashboard', "WebAssembly function reset_internal_allocation_stats not available");
        return;
      }

      wasmLogger.log(WasmLogLevel.INFO, 'dashboard', "Cleaning up WASM memory");
      wasmModule.reset_internal_allocation_stats();

      // Refresh state immediately after reset
      setTimeout(() => {
        updateWasmState();
        // Also get a direct memory update
        try {
          const memInfo = wasmModule.get_memory_usage();
          if (memInfo) {
            wasmState = {
              ...wasmState,
              memoryUsage: memInfo
            };
          }
        } catch (e) {
          wasmLogger.log(WasmLogLevel.ERROR, 'dashboard', `Error refreshing memory info after reset: ${e.message}`);
        }
      }, 50);
    } catch (e) {
      wasmLogger.log(WasmLogLevel.ERROR, 'dashboard', `Failed to reset memory tracking: ${e.message}`);
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
    <div class="grid grid-cols-1 gap-4">
      <!-- WebAssembly Functions Overview -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-4 text-center">WebAssembly Functions</div>

        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          <!-- mergeInsertLogs -->
          <div class="bg-gray-800/70 rounded p-3">
            <div class="text-xs text-blue-400 mb-1 font-medium">Log Processing</div>
            <div class="text-white font-bold mb-0.5">mergeInsertLogs</div>
            <div class="text-xs text-gray-400">
              Merges and sorts log arrays with timestamp-based ordering for the log viewer.
            </div>
            <div class="mt-2 text-xs flex justify-between">
              <span class="text-gray-500">Calls: {wasmState?.operationsPerType?.mergeInsertLogs || 0}</span>
              <span class="text-gray-500">
                {wasmState.performanceMetrics?.operationTimings?.mergeInsertLogs?.avgTime
                  ? formatTime(wasmState.performanceMetrics?.operationTimings?.mergeInsertLogs?.avgTime)
                  : '—'}
              </span>
            </div>
          </div>

          <!-- findLogAtScrollPosition -->
          <div class="bg-gray-800/70 rounded p-3">
            <div class="text-xs text-green-400 mb-1 font-medium">Virtualization</div>
            <div class="text-white font-bold mb-0.5">findLogAtScrollPosition</div>
            <div class="text-xs text-gray-400">
              Efficiently calculates which log entries should be visible at a given scroll position.
            </div>
            <div class="mt-2 text-xs flex justify-between">
              <span class="text-gray-500">Calls: {wasmState?.operationsPerType?.findLogAtScrollPosition || 0}</span>
              <span class="text-gray-500">
                {wasmState.performanceMetrics?.operationTimings?.findLogAtScrollPosition?.avgTime
                  ? formatTime(wasmState.performanceMetrics?.operationTimings?.findLogAtScrollPosition?.avgTime)
                  : '—'}
              </span>
            </div>
          </div>

          <!-- recalculatePositions -->
          <div class="bg-gray-800/70 rounded p-3">
            <div class="text-xs text-purple-400 mb-1 font-medium">Virtualization</div>
            <div class="text-white font-bold mb-0.5">recalculatePositions</div>
            <div class="text-xs text-gray-400">
              Efficiently calculates positions for all log entries in the virtual scroll container.
            </div>
            <div class="mt-2 text-xs flex justify-between">
              <span class="text-gray-500">Calls: {wasmState?.operationsPerType?.recalculatePositions || 0}</span>
              <span class="text-gray-500">
                {wasmState.performanceMetrics?.operationTimings?.recalculatePositions?.avgTime
                  ? formatTime(wasmState.performanceMetrics?.operationTimings?.recalculatePositions?.avgTime)
                  : '—'}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Operation Stats -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-3 text-center">Function Performance</div>

        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="text-left text-xs text-gray-500">
                <th class="pb-3">Function</th>
                <th class="pb-3 text-center">Calls</th>
                <th class="pb-3 text-center">Avg. Time</th>
              </tr>
            </thead>
            <tbody>
              {#if wasmState?.operationsPerType && Object.keys(wasmState.operationsPerType).length > 0}
                {#each Object.entries(wasmState.operationsPerType) as [operation, count]}
                  <tr class="border-t border-gray-600/30">
                    <td class="py-2.5 font-medium">{operation}</td>
                    <td class="py-2.5 text-center">{count}</td>
                    <td class="py-2.5 text-center">
                      {wasmState.performanceMetrics?.operationTimings?.[operation]?.avgTime
                        ? formatTime(wasmState.performanceMetrics.operationTimings[operation].avgTime)
                        : '—'}
                    </td>
                  </tr>
                {/each}
              {:else}
                <tr>
                  <td colspan="3" class="py-6 text-center text-gray-500">No operations tracked yet</td>
                </tr>
              {/if}
            </tbody>
          </table>
        </div>
      </div>

      <!-- WebAssembly Info -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">About WebAssembly Performance</div>
        <p class="text-xs text-gray-300 mb-3">
          WebAssembly functions are used to accelerate performance-critical operations in the application:
        </p>
        <ul class="text-xs text-gray-300 list-disc pl-5 space-y-1">
          <li><span class="text-white font-medium">Log Processing</span>: Merging, sorting, and filtering large log arrays</li>
          <li><span class="text-white font-medium">Virtualization</span>: Efficiently calculating scroll positions for virtual lists</li>
          <li><span class="text-white font-medium">SIMD Operations</span>: Using dedicated CPU vector instructions for text searching</li>
        </ul>
        <p class="text-xs text-gray-300 mt-3">
          Performance metrics are collected during normal application usage. Use the Reset button
          to clear collected performance data.
        </p>
      </div>
    </div>
  {/if}

  <!-- Memory Tab -->
  {#if selectedTab === 'memory'}
    <div class="grid grid-cols-1 gap-4">
      <!-- Simplified Memory Usage Card -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-4">WebAssembly Memory</div>

        {#if wasmState?.memoryUsage}
          <!-- Memory Usage Information -->
          <div class="flex flex-col">
            <!-- Primary Memory Stats -->
            <div class="grid grid-cols-2 gap-8 mb-4">
              <div>
                <div class="text-xs text-gray-500 mb-1">Used Memory</div>
                <div class="text-2xl font-bold text-white">
                  {#if Object.prototype.toString.call(wasmState.memoryUsage) === '[object Map]'}
                    {formatBytes((wasmState.memoryUsage as Map<string, any>).get('tracked_bytes') || (wasmState.memoryUsage as Map<string, any>).get('used_bytes') || 0)}
                  {:else}
                    {formatBytes(wasmState.memoryUsage.tracked_bytes || wasmState.memoryUsage.used_bytes || wasmState.memoryUsage.used || 0)}
                  {/if}
                </div>
              </div>
              <div>
                <div class="text-xs text-gray-500 mb-1">Total Memory</div>
                <div class="text-2xl font-bold text-white">
                  {#if Object.prototype.toString.call(wasmState.memoryUsage) === '[object Map]'}
                    {formatBytes((wasmState.memoryUsage as Map<string, any>).get('total_bytes') || 0)}
                  {:else}
                    {formatBytes(wasmState.memoryUsage.total_bytes || wasmState.memoryUsage.total || 0)}
                  {/if}
                </div>
              </div>
            </div>

            <!-- Memory usage bar -->
            <div class="mb-6">
              {#if wasmState.memoryUsage}
                {@const utilization = getMemoryUtilization(wasmState.memoryUsage)}
                <div class="flex justify-between text-xs text-gray-400 mb-1">
                  <span>Memory Utilization</span>
                  <span>{Math.round(utilization * 100)}%</span>
                </div>
                <div class="w-full bg-gray-600 rounded-full h-3">
                  <div
                    class="h-3 rounded-full transition-all duration-500"
                    class:bg-green-500={utilization < 0.7}
                    class:bg-yellow-500={utilization >= 0.7 && utilization < 0.85}
                    class:bg-red-500={utilization >= 0.85}
                    style="width: {utilization * 100}%"
                  ></div>
                </div>
              {/if}
            </div>

            <!-- Additional Info -->
            <div class="grid grid-cols-2 gap-4">
              <div>
                <div class="text-xs text-gray-500 mb-1">WebAssembly Pages</div>
                <div class="text-sm font-medium text-white">
                  {#if Object.prototype.toString.call(wasmState.memoryUsage) === '[object Map]'}
                    {(wasmState.memoryUsage as Map<string, any>).get('current_pages') || 0}
                  {:else}
                    {wasmState.memoryUsage.current_pages || 0}
                  {/if}
                  <span class="text-xs text-gray-400">
                    @ {formatBytes(65536)}/page
                  </span>
                </div>
              </div>

              <div>
                <div class="text-xs text-gray-500 mb-1">Allocation Count</div>
                <div class="text-sm font-medium text-white">
                  {#if Object.prototype.toString.call(wasmState.memoryUsage) === '[object Map]'}
                    {(wasmState.memoryUsage as Map<string, any>).get('allocation_count') || 0}
                  {:else}
                    {wasmState.memoryUsage.allocation_count || 0}
                  {/if}
                </div>
              </div>
            </div>
          </div>
        {:else}
          <div class="flex items-center justify-center h-32 text-gray-400">
            WebAssembly memory information not available.<br>
            Try enabling WebAssembly in settings.
          </div>
        {/if}
      </div>

      <!-- Memory Explainer Card -->
      <div class="bg-gray-700/70 rounded p-4">
        <div class="text-sm text-gray-400 mb-2">About WebAssembly Memory</div>
        <p class="text-xs text-gray-300 mb-3">
          WebAssembly modules have a dedicated memory space that grows as needed. The memory usage shown
          above reflects the current state of the WASM module's memory:
        </p>
        <ul class="text-xs text-gray-300 list-disc pl-5 space-y-1 mb-3">
          <li><span class="text-white font-medium">Used Memory</span>: Memory currently tracked by the allocator</li>
          <li><span class="text-white font-medium">Total Memory</span>: Total memory allocated to the WASM module</li>
          <li><span class="text-white font-medium">Pages</span>: WebAssembly memory is allocated in 64KB pages</li>
        </ul>
        <p class="text-xs text-gray-300">
          WebAssembly memory is automatically managed. The system will grow memory as needed during operations
          and automatically clean up resources when they're no longer needed.
        </p>
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