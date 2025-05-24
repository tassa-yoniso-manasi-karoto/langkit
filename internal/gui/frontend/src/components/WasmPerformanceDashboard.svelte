<!-- WasmPerformanceDashboard.svelte -->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { getWasmState, getWasmModule } from '../lib/wasm';
  import { formatBytes, formatTime } from '../lib/utils';
  import { settings } from '../lib/stores';
  import { get } from 'svelte/store';
  import { logger } from '../lib/logger';

  // Component state
  let wasmState = getWasmState();
  let updateInterval: number | null = null;
  let memoryUtilizationHistory: {timestamp: number, utilization: number}[] = [];

  // Simplified tab structure
  let selectedTab = 'performance'; // 'performance', 'memory'

  // Update metrics periodically
  onMount(() => {
    logger.info('dashboard', 'Initializing WebAssembly performance dashboard');

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
        logger.error('dashboard', `Error refreshing memory info: ${e.message}`);
        return false;
      }
    };

    // Initial update
    updateWasmState();

    // Force an immediate memory refresh
    if (refreshMemoryInfo()) {
      logger.info('dashboard', 'Initial memory info updated');
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

  onDestroy(() => {
    if (updateInterval) {
      clearInterval(updateInterval);
    }
  });

  // Helper to safely get memory utilization
  function getMemoryUtilization(memoryUsage: any): number {
    // Debug output to help diagnose memory issues
    logger.trace('dashboard', 'Calculating memory utilization');
    
    // Exit early if memory usage is null or undefined
    if (!memoryUsage) {
      logger.warn('dashboard', 'Memory usage is null or undefined');
      return 0;
    }
    
    // First check if memoryUsage is a Map
    const isMap = Object.prototype.toString.call(memoryUsage) === '[object Map]';
    logger.trace('dashboard', `Memory usage is a Map? ${isMap}`);

    if (isMap) {
      // Access using Map.get() method
      try {
        // First try the utilization_estimate field (new field from Rust)
        const utilization_estimate = (memoryUsage as Map<string, any>).get('utilization_estimate');
        if (typeof utilization_estimate === 'number') {
          logger.trace('dashboard', `Using utilization_estimate: ${utilization_estimate}`);
          return utilization_estimate;
        }

        // Then try the utilization field (field from wasm-state.ts)
        const utilization = (memoryUsage as Map<string, any>).get('utilization');
        if (typeof utilization === 'number') {
          logger.trace('dashboard', `Using utilization: ${utilization}`);
          return utilization;
        }

        // Calculate utilization from tracked and total bytes if available
        const tracked_bytes = (memoryUsage as Map<string, any>).get('tracked_bytes');
        const total_bytes = (memoryUsage as Map<string, any>).get('total_bytes');
        if (typeof tracked_bytes === 'number' &&
            typeof total_bytes === 'number' &&
            total_bytes > 0) {
          const calculated = tracked_bytes / total_bytes;
          logger.trace('dashboard', `Calculated from tracked/total: ${calculated.toFixed(4)}`);
          return calculated;
        }
        
        // Try used bytes if tracked not available
        const used_bytes = (memoryUsage as Map<string, any>).get('used_bytes');
        if (typeof used_bytes === 'number' &&
            typeof total_bytes === 'number' &&
            total_bytes > 0) {
          const calculated = used_bytes / total_bytes;
          logger.trace('dashboard', `Calculated from used/total: ${calculated.toFixed(4)}`);
          return calculated;
        }
      } catch (e) {
        logger.error('dashboard', `Error accessing Map properties: ${e.message}`);
      }
    } else {
      // Regular object property access
      try {
        // First try the utilization_estimate field (new field from Rust)
        if (typeof memoryUsage.utilization_estimate === 'number') {
          logger.trace('dashboard', `Using utilization_estimate: ${memoryUsage.utilization_estimate}`);
          return memoryUsage.utilization_estimate;
        }

        // Then try the utilization field (field from wasm-state.ts)
        if (typeof memoryUsage.utilization === 'number') {
          logger.trace('dashboard', `Using utilization: ${memoryUsage.utilization}`);
          return memoryUsage.utilization;
        }

        // Calculate utilization from tracked and total bytes if available
        if (typeof memoryUsage.tracked_bytes === 'number' &&
            typeof memoryUsage.total_bytes === 'number' &&
            memoryUsage.total_bytes > 0) {
          const calculated = memoryUsage.tracked_bytes / memoryUsage.total_bytes;
          logger.trace('dashboard', `Calculated from tracked/total: ${calculated.toFixed(4)}`);
          return calculated;
        }

        // Calculate utilization from used and total bytes if available
        if (typeof memoryUsage.used_bytes === 'number' &&
            typeof memoryUsage.total_bytes === 'number' &&
            memoryUsage.total_bytes > 0) {
          const calculated = memoryUsage.used_bytes / memoryUsage.total_bytes;
          logger.trace('dashboard', `Calculated from used/total: ${calculated.toFixed(4)}`);
          return calculated;
        }

        // Calculate utilization from used and total fields (older format)
        if (typeof memoryUsage.used === 'number' &&
            typeof memoryUsage.total === 'number' &&
            memoryUsage.total > 0) {
          const calculated = memoryUsage.used / memoryUsage.total;
          logger.trace('dashboard', `Calculated from used/total (legacy): ${calculated.toFixed(4)}`);
          return calculated;
        }
      } catch (e) {
        logger.error('dashboard', `Error accessing object properties: ${e.message}`);
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
      
      logger.debug('dashboard', 'Available memory properties', properties);
    } catch (e) {
      logger.error('dashboard', `Error logging properties: ${e.message}`);
    }

    // Default fallback to avoid NaN
    logger.warn('dashboard', 'Using default utilization fallback: 0');
    return 0;
  }

  function updateWasmState() {
    // Get latest WASM state
    wasmState = getWasmState();
    logger.trace('dashboard', 'Updated WASM state');

    // Get fresh memory information directly from the WASM module
    try {
      const wasmModule = getWasmModule();
      if (wasmModule && wasmModule.get_memory_usage) {
        // Get fresh memory info
        const freshMemoryInfo = wasmModule.get_memory_usage();
        logger.trace('dashboard', 'Retrieved fresh memory info from module');

        // Override the memoryUsage in wasmState with fresh data
        if (freshMemoryInfo) {
          wasmState = {
            ...wasmState,
            memoryUsage: freshMemoryInfo
          };
          logger.trace('dashboard', 'Updated wasmState with fresh memory info');
        }
      }
    } catch (e) {
      logger.error('dashboard', `Error getting fresh memory info: ${e.message}`);
    }

    // Add point to memory history (limited to 60 points = 1 minute)
    if (wasmState?.memoryUsage) {
      const utilization = getMemoryUtilization(wasmState.memoryUsage);
      logger.trace('dashboard', `Current memory utilization: ${utilization.toFixed(4)}`);

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
      // Only log if WASM is actually enabled
      const currentSettings = get(settings);
      if (currentSettings.forceWasmMode === 'enabled' || 
          (currentSettings.forceWasmMode === 'auto' && wasmState?.enabled)) {
        logger.warn('dashboard', 'No memoryUsage available in wasmState');
      }
    }
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
        logger.error('dashboard', "WebAssembly function reset_internal_allocation_stats not available");
        return;
      }

      logger.info('dashboard', "Cleaning up WASM memory");
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
          logger.error('dashboard', `Error refreshing memory info after reset: ${e.message}`);
        }
      }, 50);
    } catch (e) {
      logger.error('dashboard', `Failed to reset memory tracking: ${e.message}`);
    }
  }

  // Helper to get memory values
  function getMemoryValue(memoryUsage: any, key: string): number {
    if (!memoryUsage) return 0;
    const isMap = Object.prototype.toString.call(memoryUsage) === '[object Map]';
    if (isMap) {
      return (memoryUsage as Map<string, any>).get(key) || 0;
    }
    return memoryUsage[key] || 0;
  }

  // Ultra compact formatters
  function formatCompactBytes(bytes: number): string {
    if (bytes < 1024) return bytes + 'B';
    if (bytes < 1048576) return (bytes / 1024).toFixed(1) + 'K';
    if (bytes < 1073741824) return (bytes / 1048576).toFixed(1) + 'M';
    return (bytes / 1073741824).toFixed(1) + 'G';
  }

  function formatCompactTime(ms: number): string {
    if (ms < 1) return ms.toFixed(2) + 'ms';
    if (ms < 1000) return ms.toFixed(0) + 'ms';
    return (ms / 1000).toFixed(2) + 's';
  }

</script>

<div class="bg-gray-800/60 text-white text-xs p-2 rounded-sm border border-gray-700/50">
  <!-- Modern compact header -->
  <div class="flex items-center gap-2 mb-2">
    <div class="flex gap-1">
      <button
        class="px-2 py-0.5 rounded-sm transition-colors duration-150 {selectedTab === 'performance' ? 'bg-blue-600 text-white' : 'bg-gray-700/50 text-gray-400 hover:bg-gray-700'}"
        on:click={() => selectedTab = 'performance'}
      >
        Performance
      </button>
      <button
        class="px-2 py-0.5 rounded-sm transition-colors duration-150 {selectedTab === 'memory' ? 'bg-blue-600 text-white' : 'bg-gray-700/50 text-gray-400 hover:bg-gray-700'}"
        on:click={() => selectedTab = 'memory'}
      >
        Memory
      </button>
    </div>
    <span class="text-gray-500 text-[10px] ml-auto">WASM Monitor</span>
  </div>

  <!-- Performance Tab -->
  {#if selectedTab === 'performance'}
    <div class="space-y-2">
      <!-- Function calls table -->
      <table class="w-full text-[11px]">
        <thead>
          <tr class="text-gray-400 border-b border-gray-700/50">
            <th class="text-left pb-1 font-medium">Function</th>
            <th class="text-right pb-1 font-medium">Calls</th>
            <th class="text-right pb-1 font-medium">Avg Time</th>
          </tr>
        </thead>
        <tbody>
          {#if wasmState?.operationsPerType && Object.keys(wasmState.operationsPerType).length > 0}
            {#each Object.entries(wasmState.operationsPerType) as [operation, count]}
              <tr class="hover:bg-gray-700/30 transition-colors duration-150">
                <td class="py-0.5 text-gray-300">{operation}</td>
                <td class="py-0.5 text-right text-gray-200">{count}</td>
                <td class="py-0.5 text-right text-gray-200">
                  {wasmState.performanceMetrics?.operationTimings?.[operation]?.avgTime
                    ? formatCompactTime(wasmState.performanceMetrics.operationTimings[operation].avgTime)
                    : '—'}
                </td>
              </tr>
            {/each}
          {:else}
            <tr>
              <td colspan="3" class="py-2 text-center text-gray-500">No operations recorded</td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>
  {/if}

  <!-- Memory Tab -->
  {#if selectedTab === 'memory'}
    <div class="space-y-2">
      {#if wasmState?.memoryUsage}
        {@const utilization = getMemoryUtilization(wasmState.memoryUsage)}
        {@const used = getMemoryValue(wasmState.memoryUsage, 'tracked_bytes') || getMemoryValue(wasmState.memoryUsage, 'used_bytes')}
        {@const total = getMemoryValue(wasmState.memoryUsage, 'total_bytes')}
        {@const pages = getMemoryValue(wasmState.memoryUsage, 'current_pages')}
        {@const allocs = getMemoryValue(wasmState.memoryUsage, 'allocation_count')}
        
        <!-- Memory stats grid -->
        <div class="grid grid-cols-2 gap-2 text-[11px]">
          <div class="bg-gray-700/30 rounded-sm p-1.5">
            <div class="text-gray-400 text-[10px]">Used Memory</div>
            <div class="text-white font-medium">{formatCompactBytes(used)}</div>
          </div>
          <div class="bg-gray-700/30 rounded-sm p-1.5">
            <div class="text-gray-400 text-[10px]">Total Memory</div>
            <div class="text-white font-medium">{formatCompactBytes(total)}</div>
          </div>
          <div class="bg-gray-700/30 rounded-sm p-1.5">
            <div class="text-gray-400 text-[10px]">Pages</div>
            <div class="text-white font-medium">{pages}</div>
          </div>
          <div class="bg-gray-700/30 rounded-sm p-1.5">
            <div class="text-gray-400 text-[10px]">Allocations</div>
            <div class="text-white font-medium">{allocs}</div>
          </div>
        </div>

        <!-- Modern utilization bar -->
        <div class="mt-2">
          <div class="flex items-center gap-2 text-[11px]">
            <span class="text-gray-400">Utilization</span>
            <div class="flex-1 bg-gray-700/50 rounded-sm h-2 overflow-hidden">
              <div
                class="h-2 transition-all duration-300"
                class:bg-emerald-500={utilization < 0.7}
                class:bg-amber-500={utilization >= 0.7 && utilization < 0.85}
                class:bg-red-500={utilization >= 0.85}
                style="width: {utilization * 100}%"
              ></div>
            </div>
            <span class="text-white font-medium w-10 text-right">{Math.round(utilization * 100)}%</span>
          </div>
        </div>
      {:else}
        <div class="text-center py-4 text-gray-500">No memory data available</div>
      {/if}
    </div>
  {/if}
</div>