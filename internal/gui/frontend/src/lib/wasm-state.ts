// src/lib/wasm-state.ts - Refined with immutable updates
import { wasmLogger, WasmLogLevel } from './wasm-logger';
// Import error type for instanceof check, use type-only import
import type { WasmInitializationError } from './wasm';

export enum WasmInitStatus {
  NOT_STARTED = "not_started",
  INITIALIZING = "initializing",
  SUCCESS = "success",
  FAILED = "failed"
}

export interface WasmState {
  initStatus: WasmInitStatus;
  initTime?: number;              // Time taken to initialize (ms)
  lastUsed?: number;              // Last time WASM was used
  totalOperations: number;        // Total operations performed
  operationsPerType: Record<string, number>; // Count by operation type
  lastError?: Error;              // Last error that occurred
  memoryUsage?: {
    total: number;                // Total WASM memory (bytes)
    used: number;                 // Used WASM memory (bytes)
    utilization: number;          // Used/Total ratio
    peak_bytes?: number;          // Peak memory usage
    allocation_count?: number;    // Number of allocations tracked
    // New fields from enhanced Rust tracking (Phase 1.1)
    allocation_rate?: number;
    time_since_last_gc?: number;
    memory_growth_trend?: number;
    fragmentation_estimate?: number;
    average_allocation?: number; // Added from Rust struct
  };
  performanceMetrics: {
    avgWasmTime: number;          // Running average of WASM execution time
    avgTsTime: number;            // Running average of TS execution time
    avgSerializationTime: number; // Running average of serialization time
    avgDeserializationTime: number; // Running average of deserialization time
    speedupRatio: number;         // TS time / WASM time
    netSpeedupRatio: number;      // TS time / (WASM time + serialization overhead)
    operationsCount: number;      // Number of operations measured
    logSizeDistribution: {        // Distribution of log sizes processed
      small: number;              // < 500 logs
      medium: number;             // 500-2000 logs
      large: number;              // > 2000 logs
    };
    operationTimings: {           // Timings by operation type
      [operation: string]: {
        avgTime: number;          // Average time for this operation
        count: number;            // Number of operations measured
      };
    };
  };
}

// Initial state
const initialState: WasmState = {
  initStatus: WasmInitStatus.NOT_STARTED,
  totalOperations: 0,
  operationsPerType: {},
  performanceMetrics: {
    avgWasmTime: 0,
    avgTsTime: 0,
    avgSerializationTime: 0,
    avgDeserializationTime: 0,
    speedupRatio: 0,
    netSpeedupRatio: 0,
    operationsCount: 0,
    logSizeDistribution: {
      small: 0,
      medium: 0,
      large: 0
    },
    operationTimings: {}
  }
};

// Export the state directly for wasm.ts to manage persistence
export let wasmState: WasmState = { ...initialState };

// --- Start Phase 1.3: Immutable State Updates ---
/**
 * Create an immutable state update function
 * @param updates Partial state object with changes
 */
function updateState(updates: Partial<WasmState>): void {
  const prevState = { ...wasmState }; // Shallow copy for comparison

  // Validate critical fields before applying
  if (updates.initStatus !== undefined &&
      !Object.values(WasmInitStatus).includes(updates.initStatus)) {
    wasmLogger.log(
      WasmLogLevel.ERROR,
      'state',
      `Invalid initStatus value provided: ${updates.initStatus}`
    );
    return; // Do not apply invalid update
  }

  // Apply updates immutably
  wasmState = {
    ...wasmState, // Spread current state
    ...updates,   // Spread updates over current state
    // Deep merge for nested objects to avoid losing existing keys
    performanceMetrics: updates.performanceMetrics ? {
      ...wasmState.performanceMetrics, // Keep existing metrics
      ...updates.performanceMetrics    // Overwrite with new metrics
    } : wasmState.performanceMetrics, // If no perf metrics update, keep old one
    memoryUsage: updates.memoryUsage ? {
      ...wasmState.memoryUsage,       // Keep existing memory info
      ...updates.memoryUsage          // Overwrite with new memory info
    } : wasmState.memoryUsage,       // If no memory update, keep old one
    operationsPerType: updates.operationsPerType ? {
      ...wasmState.operationsPerType, // Keep existing counts
      ...updates.operationsPerType    // Overwrite with new counts
    } : wasmState.operationsPerType, // If no ops update, keep old one
    // Ensure lastError is handled correctly (don't merge, just replace)
    // Use hasOwnProperty to correctly handle updates where lastError is explicitly set to undefined/null
    lastError: updates.hasOwnProperty('lastError') ? updates.lastError : wasmState.lastError
  };

  // Log significant state changes
  const significantChange = detectSignificantChange(prevState, wasmState);
  if (significantChange) {
    wasmLogger.log(
      WasmLogLevel.INFO,
      'state',
      `WebAssembly state changed: ${significantChange}`,
      {
        changeType: significantChange,
        // Include only changed fields to reduce log size
        changes: extractChanges(prevState, wasmState)
      }
    );

    // Report significant changes to backend
    reportWasmState();
  }
}

/**
 * Detect significant state changes that should be logged or reported
 * @param prevState The state before the update
 * @param newState The state after the update
 * @returns A string describing the change, or null if not significant
 */
function detectSignificantChange(prevState: WasmState, newState: WasmState): string | null {
  // Check for initialization status changes
  if (prevState.initStatus !== newState.initStatus) {
    return `initStatus changed from ${prevState.initStatus} to ${newState.initStatus}`;
  }

  // Check for new error state
  if (!prevState.lastError && newState.lastError) {
    return `error occurred: ${newState.lastError.name}`;
  }
  // Check if error changed (e.g., different error occurred)
  if (prevState.lastError && newState.lastError && prevState.lastError.message !== newState.lastError.message) {
      return `new error occurred: ${newState.lastError.name}`;
  }
  // Check if error was cleared
  if (prevState.lastError && !newState.lastError) {
      return `error cleared`;
  }


  // Check for significant memory changes
  if (prevState.memoryUsage && newState.memoryUsage) {
    const prevUtil = prevState.memoryUsage.utilization;
    const newUtil = newState.memoryUsage.utilization;

    if (newUtil > 0.9 && prevUtil <= 0.9) {
      return 'memory utilization exceeded 90%';
    }

    if (newUtil > 0.75 && prevUtil <= 0.75) {
      return 'memory utilization exceeded 75%';
    }

    // Check for significant decrease after being high
    if (newUtil < 0.5 && prevUtil >= 0.75) {
      return 'memory utilization decreased significantly';
    }
  }

  // Check for performance threshold changes (e.g., speedup ratio crossing a boundary)
  if (prevState.performanceMetrics && newState.performanceMetrics) {
    const prevRatio = prevState.performanceMetrics.speedupRatio;
    const newRatio = newState.performanceMetrics.speedupRatio;

    // Check if ratio changed significantly (e.g., by more than 25%)
    if (newRatio && prevRatio && Math.abs(newRatio - prevRatio) / prevRatio > 0.25) {
      return `performance ratio changed significantly (from ${prevRatio.toFixed(2)}x to ${newRatio.toFixed(2)}x)`;
    }

    // Check if net ratio crossed the 1.0 boundary
    const prevNetRatio = prevState.performanceMetrics.netSpeedupRatio;
    const newNetRatio = newState.performanceMetrics.netSpeedupRatio;
    if (prevNetRatio <= 1.0 && newNetRatio > 1.0) {
        return `net performance gain achieved (ratio: ${newNetRatio.toFixed(2)}x)`;
    }
    if (prevNetRatio > 1.0 && newNetRatio <= 1.0) {
        return `net performance gain lost (ratio: ${newNetRatio.toFixed(2)}x)`;
    }
  }

  return null; // No significant change detected
}

/**
 * Extract only the changed fields between two state objects for efficient logging
 * @param prevState State before update
 * @param newState State after update
 * @returns An object containing only the changed key-value pairs
 */
function extractChanges(prevState: WasmState, newState: WasmState): any {
  const changes: Record<string, any> = {};

  // Check top-level fields (excluding nested objects and lastError)
  Object.keys(newState).forEach(key => {
    const typedKey = key as keyof WasmState;

    // Skip nested objects and lastError here, handle them separately
    if (typedKey === 'performanceMetrics' || typedKey === 'memoryUsage' || typedKey === 'operationsPerType' || typedKey === 'lastError') {
      return;
    }

    if (prevState[typedKey] !== newState[typedKey]) {
      changes[key] = newState[typedKey];
    }
  });

  // Check lastError specifically
  if (prevState.lastError?.message !== newState.lastError?.message) {
      changes.lastError = newState.lastError ? { name: newState.lastError.name, message: newState.lastError.message } : null;
  }


  // Check nested memoryUsage object
  if (newState.memoryUsage) {
      const memChanges: Record<string, any> = {};
      let hasMemChanges = false;
      const prevMem = prevState.memoryUsage || {}; // Handle case where it didn't exist before

      Object.keys(newState.memoryUsage).forEach(key => {
          const typedKey = key as keyof NonNullable<WasmState['memoryUsage']>;
          // Explicitly get previous value, default to undefined if prevState.memoryUsage is null/undefined
          const prevValue = prevState.memoryUsage ? prevState.memoryUsage[typedKey] : undefined;
          const newValue = newState.memoryUsage![typedKey];
          // Compare the previous and new values
          if (prevValue !== newValue) {
              memChanges[key] = newValue;
              hasMemChanges = true;
          }
      });

      if (hasMemChanges) {
          changes.memoryUsage = memChanges;
      }
  } else if (prevState.memoryUsage) {
      // Handle case where memoryUsage was removed
      changes.memoryUsage = null;
  }


  // Check nested performanceMetrics object
  if (newState.performanceMetrics) {
      const perfChanges: Record<string, any> = {};
      let hasPerfChanges = false;
      const prevPerf = prevState.performanceMetrics || {} as WasmState['performanceMetrics']; // Handle case where it didn't exist before

      Object.keys(newState.performanceMetrics).forEach(key => {
          const typedKey = key as keyof WasmState['performanceMetrics'];

          // Simple comparison for primitive types
          if (typeof newState.performanceMetrics[typedKey] !== 'object') {
              if (prevPerf[typedKey] !== newState.performanceMetrics[typedKey]) {
                  perfChanges[key] = newState.performanceMetrics[typedKey];
                  hasPerfChanges = true;
              }
          } else {
              // For nested objects like logSizeDistribution and operationTimings, do a deep compare (JSON stringify is simple way)
              if (JSON.stringify(prevPerf[typedKey]) !== JSON.stringify(newState.performanceMetrics[typedKey])) {
                  perfChanges[key] = newState.performanceMetrics[typedKey]; // Log the entire changed sub-object
                  hasPerfChanges = true;
              }
          }
      });

      if (hasPerfChanges) {
          changes.performanceMetrics = perfChanges;
      }
  } else if (prevState.performanceMetrics) {
      // Handle case where performanceMetrics was removed
      changes.performanceMetrics = null;
  }

  // Check operationsPerType
  if (newState.operationsPerType) {
      const opsChanges: Record<string, any> = {};
      let hasOpsChanges = false;
      const prevOps = prevState.operationsPerType || {};

      // Check for added/changed keys
      Object.keys(newState.operationsPerType).forEach(key => {
          if (prevOps[key] !== newState.operationsPerType[key]) {
              opsChanges[key] = newState.operationsPerType[key];
              hasOpsChanges = true;
          }
      });
      // Check for removed keys
      Object.keys(prevOps).forEach(key => {
          if (!(key in newState.operationsPerType)) {
              opsChanges[key] = undefined; // Indicate removal
              hasOpsChanges = true;
          }
      });

      if (hasOpsChanges) {
          changes.operationsPerType = opsChanges;
      }
  } else if (prevState.operationsPerType) {
      changes.operationsPerType = null;
  }


  return changes;
}
// --- End Phase 1.3: Immutable State Updates ---


export function getWasmState(): WasmState {
  // Return a deep copy to prevent external mutation issues, especially with nested objects
  // Using JSON parse/stringify for a simple deep copy
  try {
      return JSON.parse(JSON.stringify(wasmState));
  } catch (e) {
      wasmLogger.log(WasmLogLevel.ERROR, 'state', `Failed to deep copy wasmState: ${e}`);
      return { ...wasmState }; // Fallback to shallow copy on error
  }
}

// Internal function to reset metrics, called by wasm.ts
// Renamed to avoid conflict if resetWasmMetrics is exported from wasm.ts
export function resetWasmMetricsInternal(): void {
  // Use updateState for immutable update
  updateState({
      performanceMetrics: { ...initialState.performanceMetrics }, // Reset to initial metrics
      totalOperations: 0,
      operationsPerType: {}
  });

  wasmLogger.log(
    WasmLogLevel.INFO,
    'metrics',
    'WebAssembly performance metrics reset (internal)'
  );

  // Report reset state to backend
  reportWasmState();
}

/**
 * Updates performance metrics with detailed breakdown using immutable updates
 *
 * @param wasmTime Pure WebAssembly execution time
 * @param tsTime TypeScript execution time (0 if not measured)
 * @param logCount Number of logs processed
 * @param operation Operation type
 * @param serializationTime Time spent on JS serialization prep (optional)
 * @param deserializationTime Time spent on JS deserialization processing (optional)
 */
export function updatePerformanceMetrics(
  wasmTime: number,
  tsTime: number,
  logCount: number,
  operation: string = 'mergeInsertLogs',
  serializationTime: number = 0,
  deserializationTime: number = 0
): void {
  const m = wasmState.performanceMetrics; // Get current metrics

  // Calculate new running averages
  const newCount = m.operationsCount + 1;
  // Use a smaller weight for new samples after many operations to stabilize averages
  const weightFactor = Math.min(0.1, 1 / newCount); // e.g., weight 0.1 after 10 ops

  // Calculate new metrics immutably
  let newAvgWasmTime = m.avgWasmTime * (1 - weightFactor) + wasmTime * weightFactor;
  let newAvgTsTime = m.avgTsTime;
  let newAvgSerializationTime = m.avgSerializationTime;
  let newAvgDeserializationTime = m.avgDeserializationTime;

  // Only update TS time if it's valid (tsTime > 0)
  if (tsTime > 0) {
    newAvgTsTime = m.avgTsTime * (1 - weightFactor) + tsTime * weightFactor;
  }

  // Update serialization/deserialization times if provided
  if (serializationTime > 0) {
      newAvgSerializationTime = m.avgSerializationTime * (1 - weightFactor) + serializationTime * weightFactor;
  }
  if (deserializationTime > 0) {
      newAvgDeserializationTime = m.avgDeserializationTime * (1 - weightFactor) + deserializationTime * weightFactor;
  }


  // Update speedup ratios based on the new averages
  let newSpeedupRatio = m.speedupRatio;
  let newNetSpeedupRatio = m.netSpeedupRatio;
  if (newAvgWasmTime > 0 && newAvgTsTime > 0) {
    newSpeedupRatio = newAvgTsTime / newAvgWasmTime;
    const totalWasmTime = newAvgWasmTime + newAvgSerializationTime + newAvgDeserializationTime;
    newNetSpeedupRatio = totalWasmTime > 0 ? newAvgTsTime / totalWasmTime : 0;
  }

  // Update log size distribution immutably
  const newDistribution = { ...m.logSizeDistribution };
  if (logCount < 500) {
    newDistribution.small++;
  } else if (logCount < 2000) {
    newDistribution.medium++;
  } else {
    newDistribution.large++;
  }

  // Update operation-specific timings immutably
  const newOperationTimings = { ...m.operationTimings };
  const opStats = newOperationTimings[operation] || { avgTime: 0, count: 0 };
  const opNewCount = opStats.count + 1;
  const opWeightFactor = Math.min(0.1, 1 / opNewCount);

  newOperationTimings[operation] = {
      avgTime: opStats.avgTime * (1 - opWeightFactor) + wasmTime * opWeightFactor,
      count: opNewCount
  };

  // Apply all updates immutably via updateState
  updateState({
    performanceMetrics: {
      // Spread existing metrics first, then overwrite calculated ones
      ...m,
      avgWasmTime: newAvgWasmTime,
      avgTsTime: newAvgTsTime,
      avgSerializationTime: newAvgSerializationTime,
      avgDeserializationTime: newAvgDeserializationTime,
      speedupRatio: newSpeedupRatio,
      netSpeedupRatio: newNetSpeedupRatio,
      operationsCount: newCount,
      logSizeDistribution: newDistribution,
      operationTimings: newOperationTimings
    }
  });

  // Log metrics update periodically or for significant changes (handled by updateState)
  // Optional: Add specific logging here if needed beyond significant change detection
  if (newCount % 10 === 0 || newCount < 5) { // Log early and then periodically
      wasmLogger.log(
          WasmLogLevel.DEBUG, // Use DEBUG level for periodic updates
          'metrics',
          `Perf metrics updated (${newCount} ops)`,
          {
              operation,
              wasmTime: wasmTime.toFixed(2),
              tsTime: tsTime > 0 ? tsTime.toFixed(2) : 'N/A',
              logCount,
              avgWasm: newAvgWasmTime.toFixed(2),
              avgTs: newAvgTsTime.toFixed(2),
              ratio: newSpeedupRatio.toFixed(2),
              netRatio: newNetSpeedupRatio.toFixed(2)
          }
      );
  }
}

// Track operation for metrics using immutable updates
export function trackOperation(operation: string): void {
  // Prepare the update for operationsPerType
  const newOpsPerType = {
      ...wasmState.operationsPerType,
      [operation]: (wasmState.operationsPerType[operation] || 0) + 1
  };

  // Apply updates immutably
  updateState({
    totalOperations: wasmState.totalOperations + 1,
    operationsPerType: newOpsPerType,
    lastUsed: Date.now()
  });
}

// Update memory usage info using immutable updates
export function updateMemoryUsage(memInfo: any): void {
  // Ensure memInfo is an object before accessing properties
  if (typeof memInfo !== 'object' || memInfo === null) {
    wasmLogger.log(WasmLogLevel.WARN, 'state', 'Received invalid memory info for updateMemoryUsage', { received: memInfo });
    return;
  }

  // Prepare the memoryUsage update, including new fields from Rust
  const newMemoryUsage = {
      total: memInfo.total_bytes ?? 0,
      used: memInfo.used_bytes ?? 0,
      utilization: memInfo.utilization ?? 0,
      peak_bytes: memInfo.peak_bytes,
      allocation_count: memInfo.allocation_count,
      // Include additional memory metrics from enhanced tracking
      allocation_rate: memInfo.allocation_rate,
      time_since_last_gc: memInfo.time_since_last_gc,
      memory_growth_trend: memInfo.memory_growth_trend,
      fragmentation_estimate: memInfo.fragmentation_estimate,
      average_allocation: memInfo.average_allocation
  };


  // Apply updates immutably
  updateState({
    memoryUsage: newMemoryUsage
  });
}

// Set error state using immutable updates
export function setWasmError(error: Error): void {
  // Check if the error is an initialization error to potentially update initStatus
  // Need to import WasmInitializationError type for this check
  const isInitError = (error as any).name === 'WasmInitializationError'; // Simple check if type import fails

  updateState({
    lastError: error,
    // Only update initStatus if this is specifically an initialization error
    ...(isInitError ? { initStatus: WasmInitStatus.FAILED } : {})
  });
  // Note: reportWasmState is called within updateState if the change is significant
}
// --- End Phase 1.3: Refactored Functions ---


// Report current WASM state to backend for crash reports
export function reportWasmState(): void {
  try {
    // Use type assertion for window.go
    // Create a serializable copy of the state, excluding potentially non-serializable Error object details
    const stateToReport = {
        ...wasmState,
        lastError: wasmState.lastError ? {
            name: wasmState.lastError.name,
            message: wasmState.lastError.message,
            // Optionally include stack, but it can be large
            // stack: wasmState.lastError.stack
        } : undefined
    };
    (window as any).go.gui.App.RecordWasmState(JSON.stringify(stateToReport));
  } catch (e) {
    // Avoid logging errors here if the logger itself might be causing issues during init/shutdown
    console.error("Failed to report WASM state to backend:", e);
  }
}

// Sync state on request (Phase 3 addition - kept for potential use)
export function syncWasmStateForReport(): void {
  // Simply report the current state. The backend handles history/snapshotting.
  reportWasmState();
}