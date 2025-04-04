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
    speedupRatio: number;         // TS time / WASM time
    operationsCount: number;      // Number of operations measured
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
    speedupRatio: 0,
    operationsCount: 0
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

// Update performance metrics after each operation
export function updatePerformanceMetrics(wasmTime: number, tsTime: number, logCount: number): void {
  const m = wasmState.performanceMetrics;
  
  // Update running averages
  const newCount = m.operationsCount + 1;
  m.avgWasmTime = ((m.avgWasmTime * m.operationsCount) + wasmTime) / newCount;
  // Ensure tsTime is valid before calculating average
  if (tsTime > 0) {
      m.avgTsTime = ((m.avgTsTime * m.operationsCount) + tsTime) / newCount;
  }
  m.operationsCount = newCount;
  
  // Calculate speedup ratio, handle division by zero
  m.speedupRatio = (m.avgWasmTime > 0 && m.avgTsTime > 0) ? m.avgTsTime / m.avgWasmTime : 0;
  
  wasmLogger.log(
    WasmLogLevel.DEBUG,
    'metrics',
    `Updated performance metrics`,
    {
      avgWasmTime: m.avgWasmTime.toFixed(2),
      avgTsTime: m.avgTsTime.toFixed(2),
      speedupRatio: m.speedupRatio.toFixed(2),
      operationsCount: m.operationsCount,
      logCount
    }
  );
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