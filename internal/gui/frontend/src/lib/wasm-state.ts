// src/lib/wasm-state.ts - Refined with immutable updates
// Import error type for instanceof check, use type-only import
import type { WasmInitializationError } from './wasm';
import { logger } from './logger';
import { RecordWasmState } from '../api/services/logging';

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
  
  // Dashboard metrics for memory growth and performance
  memoryGrowthEvents?: Array<{
    timestamp: number;
    requestedBytes: number;
    beforePages: number;
    afterPages?: number;
    success: boolean;
    reason: string;
  }>;
  
  memoryChecks?: Array<{
    timestamp: number;
    logCount: number;
    initialUtilization: number;
    actions: string[];
    outcome: string;
    finalUtilization: number;
    error?: string;
  }>;
  
  fallbackReasons?: Record<string, number>; // Counts of different fallback reasons
  
  // Memory trend data points for visualization
  memoryTrend?: Array<{
    timestamp: number;
    usedBytes: number;
    totalBytes: number;
    utilization: number;
  }>;
  
  // Add blacklist tracking for dashboard visibility (from Smart Blacklist Recovery)
  blacklistedOperations?: Array<{
    operation: string;
    timestamp: number;
    retryCount: number;
    nextRetryTime: number;
    lastError?: string;
    backoffMs: number;
  }>;
  
  maintenanceIntervalId?: number; // For module maintenance interval (from Pre-Warming)

  // Add threshold adjustment tracking for dashboard (from Adaptive Threshold Learning)
  thresholdAdjustments?: Array<{
    timestamp: number;
    previousThreshold: number;
    newThreshold: number;
    reason: string;
    metrics: {
      smallLogs: number;
      mediumLogs: number;
      largeLogs: number;
      speedupRatio: number;
      netSpeedupRatio: number;
    };
  }>;
  
  // Add missing properties for logging timestamps
  lastScrollMetricsLog?: number;
  lastRecalcMetricsLog?: number;
  lastMemoryLogTime?: number; // Added to track infrequent memory logging
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

// Memory information validation and state management

/**
 * Standardizes memory information with robust error handling and validation
 * @param memInfo Raw memory information from WebAssembly
 * @returns Standardized memory information with validated fields
 */
export function standardizeMemoryInfo(memInfo: any): any {
  // Handle the case where memInfo is null or undefined
  if (!memInfo) {
    return getDefaultMemoryInfo();
  }
  
  // CRITICAL FIX: Handle Map objects (which is what we're receiving from WebAssembly)
  // Check both instanceof Map and toString representation for maximum compatibility
  const isMap =
    memInfo instanceof Map ||
    Object.prototype.toString.call(memInfo) === '[object Map]';

  // Helper function to safely get values from either Map or regular object
  const getValue = (key: string) => {
    if (isMap) {
      return (memInfo as Map<string, any>).get(key);
    } else {
      return (memInfo as any)[key];
    }
  };

  // Log detailed info for debugging if needed in development mode
  if (typeof (window as any).__LANGKIT_VERSION === 'string' &&
      (window as any).__LANGKIT_VERSION === 'dev') {
    // logger.debug('store/wasm-state', 'Standardizing memory info', {
    //   infoType: typeof memInfo,
    //   isMap,
    //   toStringResult: Object.prototype.toString.call(memInfo),
    //   keys: isMap
    //     ? Array.from((memInfo as Map<string, any>).keys())
    //     : (memInfo ? Object.keys(memInfo) : [])
    // });
  }
  
  // Check if the data is pre-validated
  const isPreValidated = getValue('is_valid') === true;
  
  // Convert Map to regular object if needed
  let memoryData: Record<string, any> = {};
  if (isMap) {
    // Convert Map entries to regular object
    memInfo.forEach((value, key) => {
      memoryData[key] = value;
    });
  } else if (typeof memInfo === 'object') {
    // If it's already an object, use it directly
    memoryData = { ...memInfo };
  } else {
    // Not an object or Map, return defaults
    return getDefaultMemoryInfo();
  }
  
  // Get critical values with proper null handling
  const totalBytes = getValue('total_bytes');
  const usedBytes = getValue('used_bytes');
  const utilization = getValue('utilization');
  const currentPages = getValue('current_pages');
  const pageSize = getValue('page_size_bytes') || 65536; // Default to 64KB
  
  // Handle case where we only have current_pages but not total_bytes
  if (typeof currentPages === 'number' && currentPages > 0 && 
      (typeof totalBytes !== 'number' || totalBytes <= 0)) {
    // Calculate total bytes from pages
    memoryData.total_bytes = currentPages * pageSize;
  }
  
  // Handle case where we have used_bytes of 0 (valid case for new module)
  if (usedBytes === 0 && typeof totalBytes === 'number' && totalBytes > 0) {
    // This is valid! Set utilization to 0
    memoryData.utilization = 0;
  }
  
  // Calculate utilization if missing but we have both total and used bytes
  if ((typeof utilization !== 'number' || Number.isNaN(utilization)) &&
      typeof totalBytes === 'number' && totalBytes > 0 &&
      typeof usedBytes === 'number' && usedBytes >= 0) {
    memoryData.utilization = usedBytes / totalBytes;
  }
  
  // More careful validation that knows how to handle Maps
  const validationIssues = {
    missingObject: false,
    invalidTotalBytes: typeof totalBytes !== 'number' || 
                       Number.isNaN(totalBytes) || 
                       (totalBytes <= 0 && (typeof currentPages !== 'number' || currentPages <= 0)),
    invalidUsedBytes: typeof usedBytes !== 'number' || 
                     Number.isNaN(usedBytes) || 
                     usedBytes < 0,
    invalidUtilization: (typeof utilization !== 'number' && 
                       !(usedBytes === 0 && typeof totalBytes === 'number' && totalBytes > 0)) || 
                      Number.isNaN(utilization) || 
                      utilization < 0 || 
                      utilization > 1
  };
  
  // Skip validation issues if the data is pre-validated
  const hasValidationIssues = !isPreValidated && (
    validationIssues.invalidTotalBytes || 
    validationIssues.invalidUsedBytes || 
    validationIssues.invalidUtilization
  );
  
  // Log validation issues occasionally
  if (hasValidationIssues && Math.random() < 0.05) {
    logger.warn('store/wasm-state', 'Received invalid memory information from WebAssembly, using defaults', {
      receivedInfo: memInfo,
      validationErrors: validationIssues
    });
    
    // Return safe defaults for invalid data
    return getDefaultMemoryInfo();
  }
  
  // For valid data, return a standardized object with safe defaults if needed
  return {
    ...memoryData,
    total_bytes: typeof totalBytes === 'number' ? Math.max(1, totalBytes) : 16 * 1024 * 1024,
    used_bytes: typeof usedBytes === 'number' ? Math.max(0, usedBytes) : 1 * 1024 * 1024,
    utilization: typeof utilization === 'number' ? 
                Math.max(0, Math.min(utilization, 1)) : 
                (usedBytes === 0 && typeof totalBytes === 'number' && totalBytes > 0) ? 0 : 0.0625,
    is_valid: true,
    available: true // Ensure available flag is always set
  };
}

/**
 * Helper function to get default memory info
 */
function getDefaultMemoryInfo(): Record<string, any> {
  return {
    total_bytes: 16 * 1024 * 1024, // 16MB
    used_bytes: 1 * 1024 * 1024,   // 1MB
    utilization: 0.0625,           // 1/16 = 6.25%
    peak_bytes: 1 * 1024 * 1024,
    allocation_count: 1,
    current_pages: 256,            // 16MB / 64KB = 256 pages
    page_size_bytes: 65536,
    is_valid: true,
    available: true
  };
}
/**
 * Create an immutable state update function
 * @param updates Partial state object with changes
 */
export function updateState(updates: Partial<WasmState>): void { // Add export keyword
  const prevState = { ...wasmState }; // Shallow copy for comparison

  // Validate critical fields before applying
  if (updates.initStatus !== undefined &&
      !Object.values(WasmInitStatus).includes(updates.initStatus)) {
    logger.error('store/wasm-state', `Invalid initStatus value provided: ${updates.initStatus}`);
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
    logger.trace('store/wasm-state', `WebAssembly state changed: ${significantChange}`, {
      changeType: significantChange,
      // Include only changed fields to reduce log size
      changes: extractChanges(prevState, wasmState)
    });

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
  // Only track truly significant status changes
  if (prevState.initStatus !== newState.initStatus) {
    if (newState.initStatus === WasmInitStatus.SUCCESS) {
      return `initialization completed successfully`;
    } else if (newState.initStatus === WasmInitStatus.FAILED) {
      return `initialization failed`;
    }
    return null; // No longer log other status changes
  }

  // Only log critical errors (memory and initialization)
  if (!prevState.lastError && newState.lastError) {
    if (newState.lastError.name === 'WasmInitializationError' ||
        newState.lastError.name === 'WasmMemoryError') {
      return `critical error: ${newState.lastError.name}`;
    }
    return null; // Don't log routine errors
  }

  // Only log extreme memory pressure (>95%)
  if (prevState.memoryUsage && newState.memoryUsage) {
    const prevUtil = prevState.memoryUsage.utilization;
    const newUtil = newState.memoryUsage.utilization;

    // Higher threshold (95% instead of 90%)
    if (newUtil > 0.95 && prevUtil <= 0.95) {
      return 'critical memory pressure (>95%)';
    }

    // Only log major relief (from >90% to <50%)
    if (newUtil < 0.5 && prevUtil >= 0.9) {
      return 'significant memory pressure relief';
    }
  }

  // Higher threshold for reporting performance changes (5x instead of 2x)
  if (prevState.performanceMetrics && newState.performanceMetrics) {
    const prevRatio = prevState.performanceMetrics.speedupRatio;
    const newRatio = newState.performanceMetrics.speedupRatio;

    // Only log if ratio improved by 400% AND is high (>5x)
    if (newRatio && prevRatio && newRatio > 5 &&
        (newRatio / prevRatio) > 4.0) {
      return `extraordinary performance improvement (${newRatio.toFixed(2)}x)`;
    }
  }

  return null;
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


  // Sanitize the changes object to remove Maps/Sets that can't be logged
  return sanitizeForLogging(changes);
}

/**
 * Sanitize an object for logging by converting Maps/Sets to serializable forms
 */
function sanitizeForLogging(obj: any): any {
  if (obj === null || obj === undefined) return obj;
  if (typeof obj !== 'object') return obj;
  
  // Handle Map objects
  if (obj instanceof Map) {
    const result: Record<string, any> = {};
    obj.forEach((value, key) => {
      if (typeof key !== 'symbol') {
        result[String(key)] = sanitizeForLogging(value);
      }
    });
    return result;
  }
  
  // Handle Set objects  
  if (obj instanceof Set) {
    return Array.from(obj).map(item => sanitizeForLogging(item));
  }
  
  // Handle arrays
  if (Array.isArray(obj)) {
    return obj.map(item => sanitizeForLogging(item));
  }
  
  // Handle regular objects
  const result: Record<string, any> = {};
  for (const [key, value] of Object.entries(obj)) {
    if (typeof key !== 'symbol' && typeof value !== 'function') {
      result[key] = sanitizeForLogging(value);
    }
  }
  return result;
}

// --- End Phase 1.3: Immutable State Updates ---


export function getWasmState(): WasmState {
  // Create a deep copy with proper handling of Error objects
  try {
      // Create a new state object for the deep copy
      const stateCopy = { ...wasmState };
      
      // Special handling for lastError (Error objects don't serialize properly with JSON)
      if (wasmState.lastError instanceof Error) {
          // Convert Error object to serializable form
          stateCopy.lastError = {
              name: wasmState.lastError.name,
              message: wasmState.lastError.message,
              stack: wasmState.lastError.stack,
              // For custom error types with additional properties
              ...(wasmState.lastError as any).context,
              ...(wasmState.lastError as any).memoryInfo,
              ...(wasmState.lastError as any).details
          };
      } else if (wasmState.lastError) {
          // If it's not an Error object but still exists, make a simple copy
          stateCopy.lastError = { ...wasmState.lastError };
      }
      
      // Handle other nested objects that need deep copying
      if (wasmState.memoryUsage) {
          stateCopy.memoryUsage = { ...wasmState.memoryUsage };
      }
      
      if (wasmState.performanceMetrics) {
          stateCopy.performanceMetrics = { 
              ...wasmState.performanceMetrics,
              // Deep copy for nested objects within performanceMetrics
              logSizeDistribution: wasmState.performanceMetrics.logSizeDistribution 
                  ? { ...wasmState.performanceMetrics.logSizeDistribution }
                  : undefined,
              operationTimings: wasmState.performanceMetrics.operationTimings
                  ? { ...wasmState.performanceMetrics.operationTimings }
                  : {}
          };
      }
      
      // Deep copy arrays
      if (wasmState.memoryGrowthEvents) {
          stateCopy.memoryGrowthEvents = [...wasmState.memoryGrowthEvents];
      }
      
      if (wasmState.memoryChecks) {
          stateCopy.memoryChecks = [...wasmState.memoryChecks];
      }
      
      if (wasmState.memoryTrend) {
          stateCopy.memoryTrend = [...wasmState.memoryTrend];
      }
      
      if (wasmState.thresholdAdjustments) {
          stateCopy.thresholdAdjustments = [...wasmState.thresholdAdjustments];
      }
      
      if (wasmState.blacklistedOperations) {
          stateCopy.blacklistedOperations = [...wasmState.blacklistedOperations];
      }
      
      return stateCopy;
  } catch (e) {
      logger.error('store/wasm-state', `Failed to create deep copy of wasmState: ${e instanceof Error ? e.message : String(e)}`);
      // Fallback to simple shallow copy
      return { ...wasmState };
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

  logger.trace('store/wasm-state', 'WebAssembly performance metrics reset (internal)');

  // Report reset state to backend
  reportWasmState();
}

// Export updatePerformanceMetrics
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
      // REMOVE or make extremely infrequent (Feedback Step 4)
      // logger.trace('store/wasm-state', 
      //     WasmLogLevel.TRACE, // CHANGED FROM DEBUG
      //     'metrics',
      //     `Perf metrics updated (${newCount} ops)`,
      //     {
      //         operation,
      //         wasmTime: wasmTime.toFixed(2),
      //         tsTime: tsTime > 0 ? tsTime.toFixed(2) : 'N/A',
      //         logCount,
      //         avgWasm: newAvgWasmTime.toFixed(2),
      //         avgTs: newAvgTsTime.toFixed(2),
      //         ratio: newSpeedupRatio.toFixed(2),
      //         netRatio: newNetSpeedupRatio.toFixed(2)
      //     }
      // );
  }
}

// Export trackOperation
export function trackOperation(operation: string): void {
  // Skip tracking removed operations to avoid confusion in the dashboard
  if (operation === 'findLogAtScrollPosition' || operation === 'recalculatePositions') {
    return;
  }

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

/**
 * Formats memory values consistently for logging and display
 */
export const memoryFormatter = {
  /**
   * Format bytes to human-readable string with unit
   */
  formatBytes(bytes: number): string {
    if (typeof bytes !== 'number' || Number.isNaN(bytes) || bytes < 0) {
      return 'N/A';
    }
    
    if (bytes === 0) return '0 B';
    
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    const value = bytes / Math.pow(1024, i);
    
    return `${value.toFixed(2)} ${units[i]}`;
  },
  
  /**
   * Format utilization as percentage string
   */
  formatUtilization(utilization: number): string {
    if (typeof utilization !== 'number' || Number.isNaN(utilization)) {
      return 'N/A';
    }
    
    return `${(utilization * 100).toFixed(1)}%`;
  },
  
  /**
   * Format memory info object for logging
   */
  formatMemoryInfo(memInfo: any): Record<string, string> {
    return {
      total: this.formatBytes(memInfo.total_bytes),
      used: this.formatBytes(memInfo.used_bytes),
      utilization: this.formatUtilization(memInfo.utilization),
      pages: String(memInfo.current_pages || 'N/A')
    };
  }
};

/**
 * Updates memory usage with improved formatting and logging
 * @param memInfo Raw memory information from WebAssembly
 */
export function updateMemoryUsage(memInfo: any): void {
  // Standardize memory info before updating state
  const safeMemInfo = standardizeMemoryInfo(memInfo);
  
  // Apply updates immutably using existing updateState function
  updateState({
    memoryUsage: safeMemInfo
  });
  
  // Update memory trend data for dashboard
  updateMemoryTrend(safeMemInfo);
  
  // Log memory updates very infrequently
  const now = Date.now();
  const lastMemoryLogTime = wasmState.lastMemoryLogTime || 0;
  
  if (now - lastMemoryLogTime > 60000 && safeMemInfo.utilization > 0.7) { // Once per minute if high
    logger.info('store/wasm-state', `Memory update: ${memoryFormatter.formatUtilization(safeMemInfo.utilization)} used`, memoryFormatter.formatMemoryInfo(safeMemInfo));
    
    // Track last log time
    updateState({ lastMemoryLogTime: now });
  }
}

// Export setWasmError
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


// Simplify the throttling approach for state reporting
let lastReportedStateTime = 0;
const STATE_REPORT_INTERVAL = 60000; // Only report state every minute maximum

// Export reportWasmState
export function reportWasmState(): void {
  try {
    // Only report state at most once per minute
    const now = Date.now();
    if (now - lastReportedStateTime < STATE_REPORT_INTERVAL) {
      return;
    }
    
    lastReportedStateTime = now;
    
    // Create a serializable copy of the state
    const stateToReport = {
        ...wasmState,
        lastError: wasmState.lastError ? {
            name: wasmState.lastError.name,
            message: wasmState.lastError.message,
        } : undefined
    };
    
    // Report to backend
    RecordWasmState(JSON.stringify(stateToReport));
  } catch (e) {
    // Avoid logging errors here to prevent cascading issues
    logger.error('store/wasm-state', "Failed to report WASM state to backend", { error: e });
  }
}

// Sync state on request (Phase 3 addition - kept for potential use)
export function syncWasmStateForReport(): void {
  // Simply report the current state. The backend handles history/snapshotting.
  reportWasmState();
}

// Tracking functions for dashboard metrics and performance analysis
export function trackMemoryGrowth(
  requestedBytes: number,
  beforePages: number,
  afterPages: number | undefined,
  success: boolean,
  reason: string
): void {
  // Ensure the array exists using updateState for immutability
  const currentEvents = wasmState.memoryGrowthEvents || [];
  
  // Limit array size (keep last 20)
  const updatedEvents = currentEvents.length >= 20
    ? [...currentEvents.slice(1), { timestamp: Date.now(), requestedBytes, beforePages, afterPages, success, reason }]
    : [...currentEvents, { timestamp: Date.now(), requestedBytes, beforePages, afterPages, success, reason }];

  updateState({ memoryGrowthEvents: updatedEvents });
  // reportWasmState() is called within updateState if significant
}

// Add function to track fallback reasons
export function trackFallbackReason(reason: string): void {
  // Ensure the record exists and update immutably
  const currentReasons = wasmState.fallbackReasons || {};
  const updatedReasons = {
    ...currentReasons,
    [reason]: (currentReasons[reason] || 0) + 1
  };
  
  updateState({ fallbackReasons: updatedReasons });
  // reportWasmState() is called within updateState if significant
}

// Add function to update memory trend data
export function updateMemoryTrend(memoryInfo: any): void {
  // Ensure the array exists
  const currentTrend = wasmState.memoryTrend || [];
  
  // Only add points at reasonable intervals to avoid too many data points
  const now = Date.now();
  const lastPoint = currentTrend[currentTrend.length - 1];
  
  if (!lastPoint || now - lastPoint.timestamp > 5000) { // At least 5 seconds between points
    const newPoint = {
      timestamp: now,
      usedBytes: memoryInfo.used_bytes || 0,
      totalBytes: memoryInfo.total_bytes || 0,
      utilization: memoryInfo.utilization || 0
    };

    // Limit array size (keep last 50 points = ~4 minutes of history)
    const updatedTrend = currentTrend.length >= 50
      ? [...currentTrend.slice(1), newPoint]
      : [...currentTrend, newPoint];
      
    updateState({ memoryTrend: updatedTrend });
    // reportWasmState() is called within updateState if significant
  }
}

// End of file