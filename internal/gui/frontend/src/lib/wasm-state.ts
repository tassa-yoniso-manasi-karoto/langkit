// src/lib/wasm-state.ts - Remove command references
import { wasmLogger, WasmLogLevel } from './wasm-logger';

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

export function getWasmState(): WasmState {
  // Return a shallow copy for reading
  return { ...wasmState }; 
}

// Internal function to reset metrics, called by wasm.ts
export function resetWasmMetricsInternal(): void { 
  wasmState.performanceMetrics = {
    avgWasmTime: 0,
    avgTsTime: 0,
    speedupRatio: 0,
    operationsCount: 0
  };
  wasmState.totalOperations = 0;
  wasmState.operationsPerType = {};
  
  wasmLogger.log(
    WasmLogLevel.INFO,
    'metrics',
    'WebAssembly performance metrics reset (internal)'
  );
  
  // Report reset state to backend
  reportWasmState();
}

/**
 * Updates performance metrics with detailed breakdown
 * 
 * @param wasmTime Pure WebAssembly execution time
 * @param tsTime TypeScript execution time
 * @param logCount Number of logs processed
 * @param operation Operation type
 * @param serializationTime Time spent on serialization (optional)
 * @param deserializationTime Time spent on deserialization (optional)
 */
export function updatePerformanceMetrics(
  wasmTime: number,
  tsTime: number,
  logCount: number,
  operation: string = 'mergeInsertLogs',
  serializationTime: number = 0,
  deserializationTime: number = 0
): void {
  const m = wasmState.performanceMetrics;
  
  // Update running averages
  const newCount = m.operationsCount + 1;
  const oldWeight = m.operationsCount / newCount;
  const newWeight = 1 / newCount;
  
  // Use exponential moving average for more stable metrics
  m.avgWasmTime = m.avgWasmTime * oldWeight + wasmTime * newWeight;
  
  // Only update TS time if it's valid
  if (tsTime > 0) {
    m.avgTsTime = m.avgTsTime * oldWeight + tsTime * newWeight;
  }
  
  // Track serialization overhead
  if (serializationTime > 0) {
    m.avgSerializationTime = m.avgSerializationTime * oldWeight + serializationTime * newWeight;
  }
  
  // Track deserialization overhead
  if (deserializationTime > 0) {
    m.avgDeserializationTime = m.avgDeserializationTime * oldWeight + deserializationTime * newWeight;
  }
  
  m.operationsCount = newCount;
  
  // Update speedup ratios
  if (m.avgWasmTime > 0 && m.avgTsTime > 0) {
    // Pure WebAssembly/TypeScript ratio
    m.speedupRatio = m.avgTsTime / m.avgWasmTime;
    
    // Net ratio including serialization overhead
    const totalWasmTime = m.avgWasmTime + m.avgSerializationTime + m.avgDeserializationTime;
    m.netSpeedupRatio = totalWasmTime > 0 ? m.avgTsTime / totalWasmTime : 0;
  }
  
  // Update log size distribution
  if (logCount < 500) {
    m.logSizeDistribution.small++;
  } else if (logCount < 2000) {
    m.logSizeDistribution.medium++;
  } else {
    m.logSizeDistribution.large++;
  }
  
  // Update operation-specific timings
  if (!m.operationTimings[operation]) {
    m.operationTimings[operation] = { avgTime: 0, count: 0 };
  }
  
  const opStats = m.operationTimings[operation];
  const opOldWeight = opStats.count / (opStats.count + 1);
  const opNewWeight = 1 / (opStats.count + 1);
  
  opStats.avgTime = opStats.avgTime * opOldWeight + wasmTime * opNewWeight;
  opStats.count++;
  
  // Log metrics update for significant changes
  if (newCount % 10 === 0 || newCount < 10) {
    wasmLogger.log(
      WasmLogLevel.INFO,
      'metrics',
      `Performance metrics updated (${m.operationsCount} operations)`,
      {
        avgWasmTime: m.avgWasmTime.toFixed(2) + 'ms',
        avgTsTime: m.avgTsTime.toFixed(2) + 'ms',
        serializationOverhead: m.avgSerializationTime.toFixed(2) + 'ms',
        deserializationOverhead: m.avgDeserializationTime.toFixed(2) + 'ms',
        speedupRatio: m.speedupRatio.toFixed(2) + 'x',
        netSpeedupRatio: m.netSpeedupRatio.toFixed(2) + 'x',
        logSizeDistribution: m.logSizeDistribution
      }
    );
  }
}

// Track operation for metrics
export function trackOperation(operation: string): void {
  wasmState.totalOperations++;
  wasmState.operationsPerType[operation] = 
    (wasmState.operationsPerType[operation] || 0) + 1;
  
  wasmState.lastUsed = Date.now();
}

// Update memory usage info
export function updateMemoryUsage(memInfo: any): void {
   // Ensure memInfo is an object before accessing properties
   if (typeof memInfo !== 'object' || memInfo === null) {
       wasmLogger.log(WasmLogLevel.WARN, 'state', 'Received invalid memory info for updateMemoryUsage', { received: memInfo });
       return;
   }
   
   // Update state, including optional Phase 2 fields if they exist
   wasmState.memoryUsage = {
     total: memInfo.total_bytes ?? 0, // Use nullish coalescing for defaults
     used: memInfo.used_bytes ?? 0,
     utilization: memInfo.utilization ?? 0,
     peak_bytes: memInfo.peak_bytes, // Assign directly, will be undefined if not present
     allocation_count: memInfo.allocation_count // Assign directly, will be undefined if not present
   };
}

// Set error state
export function setWasmError(error: Error): void {
  wasmState.lastError = error;
  wasmState.initStatus = WasmInitStatus.FAILED; // Also mark as failed on error
  
  // Report updated state to backend
  reportWasmState();
}

// Report current WASM state to backend for crash reports
export function reportWasmState(): void {
  try {
    // Use type assertion for window.go
    (window as any).go.gui.App.RecordWasmState(JSON.stringify(wasmState)); 
  } catch (e) {
    console.error("Failed to report WASM state to backend:", e);
  }
}

// Sync state on request (Phase 3 addition)
export function syncWasmStateForReport(): void {
  // Simply report the current state. The backend handles history/snapshotting.
  reportWasmState(); 
}