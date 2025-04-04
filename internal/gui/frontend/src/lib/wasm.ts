// src/lib/wasm.ts - Simplified direct implementation without command pattern
import { wasmLogger, WasmLogLevel } from './wasm-logger';
import { 
  WasmInitStatus, 
  getWasmState as getWasmStateInternal, // Rename internal getter
  reportWasmState, 
  updateMemoryUsage, 
  setWasmError,
  resetWasmMetrics as resetWasmMetricsInternal, // Rename internal reset
  wasmState // Import the state object itself for persistence
} from './wasm-state';
import { settings } from './stores';
import { get } from 'svelte/store';

// --- State ---
let wasmModule: any = null;
let wasmInitialized = false;
let wasmEnabled = false;
let initializePromise: Promise<boolean> | null = null;
let wasmBuildInfo: WasmBuildInfo | null = null; // Added in Phase 4

// --- Interfaces ---
interface WasmBuildInfo { // Added in Phase 4
  version: string;
  timestamp: number;
  buildDate: string;
}

// --- Configuration ---
export const WASM_CONFIG = {
  DEFAULT_SIZE_THRESHOLD: 500,
  MIN_THRESHOLD: 100,
  MAX_THRESHOLD: 5000,
  MIN_PERFORMANCE_GAIN: 1.2
};
let WASM_SIZE_THRESHOLD = WASM_CONFIG.DEFAULT_SIZE_THRESHOLD;

// --- Exported Functions ---

// Exported state getter (needed by dashboard)
export function getWasmState() {
    return getWasmStateInternal();
}

// Exported metrics reset function (needed by dashboard/settings)
export function resetWasmMetrics(): void {
  resetWasmMetricsInternal(); // Call the internal reset function from wasm-state
  // Clear saved metrics from localStorage (Phase 4)
  try {
    localStorage.removeItem('wasm-metrics');
    wasmLogger.log(WasmLogLevel.INFO, 'metrics', 'Cleared saved metrics from localStorage.');
  } catch (e: any) {
    wasmLogger.log(WasmLogLevel.WARN, 'metrics', `Failed to clear saved metrics: ${e.message}`);
  }
}

// Exported build info getter (needed by dashboard)
export function getWasmBuildInfo(): WasmBuildInfo | null {
  return wasmBuildInfo;
}

// --- Internal Functions ---

export function setWasmSizeThreshold(threshold: number): void {
  WASM_SIZE_THRESHOLD = Math.max(
    WASM_CONFIG.MIN_THRESHOLD,
    Math.min(threshold, WASM_CONFIG.MAX_THRESHOLD)
  );
  
  wasmLogger.log(
    WasmLogLevel.INFO,
    'config',
    `WASM size threshold set to ${WASM_SIZE_THRESHOLD}`
  );
}

export function getWasmSizeThreshold(): number {
  return WASM_SIZE_THRESHOLD;
}

export function enableWasm(enabled: boolean): Promise<boolean> {
  const previouslyEnabled = wasmEnabled;
  wasmEnabled = enabled;
  
  wasmLogger.log(
    WasmLogLevel.INFO,
    'config',
    `WebAssembly ${enabled ? 'enabled' : 'disabled'}`
  );
  
  // Only initialize if enabling and not already initialized or initializing
  if (enabled && !wasmInitialized && !initializePromise) {
    return initializeWasm();
  }
  
  // If disabling, ensure state is reported
  if (previouslyEnabled && !enabled) {
      wasmState.initStatus = WasmInitStatus.NOT_STARTED; // Reflect disabled state
      reportWasmState();
  } else if (enabled && wasmInitialized) {
      // If already enabled and initialized, just report current state
      reportWasmState();
  }
  
  return Promise.resolve(wasmInitialized);
}

export function isWasmEnabled(): boolean {
  return wasmEnabled && wasmInitialized;
}

export function isWasmSupported(): boolean {
  return typeof WebAssembly === 'object' && 
         typeof WebAssembly.instantiate === 'function';
}

export function getWasmModule(): any {
  return wasmModule;
}

export async function initializeWasm(): Promise<boolean> {
  if (initializePromise) return initializePromise;
  
  let currentWasmState = getWasmStateInternal(); // Use internal getter
  if (currentWasmState.initStatus === WasmInitStatus.SUCCESS) {
    return true; // Already initialized
  }
  
  // Update state to initializing (directly, as this happens before command executor might be ready)
  wasmState.initStatus = WasmInitStatus.INITIALIZING;
  reportWasmState(); // Report initializing state

  initializePromise = new Promise<boolean>(async (resolve) => {
    if (!wasmEnabled) {
      wasmState.initStatus = WasmInitStatus.NOT_STARTED; // Reset if disabled before finishing
      reportWasmState();
      resolve(false);
      return;
    }
    
    const startTime = performance.now();
    let modulePath = ''; // Declare modulePath outside the try block
    wasmLogger.log(WasmLogLevel.INFO, 'init', 'Initializing WebAssembly module');
    
    try {
      if (!isWasmSupported()) {
        throw new Error("WebAssembly not supported in this browser");
      }
      
      // First, fetch build info to get version for cache busting (Phase 4)
      try {
        // Use relative path from public/index.html where this JS will run
        const buildInfoResponse = await fetch(`./wasm/build-info.json?t=${Date.now()}`); 
        if (buildInfoResponse.ok) {
          wasmBuildInfo = await buildInfoResponse.json();
          wasmLogger.log(
            WasmLogLevel.INFO,
            'init',
            `WebAssembly build info loaded - version ${wasmBuildInfo?.version}`
          );
        } else {
           throw new Error(`Failed to fetch build info: ${buildInfoResponse.statusText}`);
        }
      } catch (buildInfoError: any) {
        wasmLogger.log(
          WasmLogLevel.WARN,
          'init',
          `Failed to load WebAssembly build info: ${buildInfoError.message}. Using timestamp cache buster.`
        );
        // Continue anyway - we'll use a timestamp for cache busting
      }
      
      // Build cache buster based on build info or current time (Phase 4)
      const cacheBuster = wasmBuildInfo 
        ? `?v=${wasmBuildInfo.version}&t=${wasmBuildInfo.timestamp}` 
        : `?t=${Date.now()}`;
      
      // Dynamic import of WebAssembly module with cache busting (Phase 4)
      // Use relative path from public/index.html
      modulePath = `./wasm/log_engine.js${cacheBuster}`; // Assign to the outer variable
      wasmLogger.log(WasmLogLevel.DEBUG, 'init', `Loading module from: ${modulePath}`);
      
      // @ts-ignore - This file is generated by the build process
      const module = await import(/* @vite-ignore */ modulePath); 
      await module.default(); // Initialize the WASM module
      
      wasmModule = module;
      wasmInitialized = true;
      wasmState.initStatus = WasmInitStatus.SUCCESS; // Update state directly
      
      const endTime = performance.now();
      const initTime = endTime - startTime;
      wasmState.initTime = initTime; // Record init time
      
      // Log successful initialization with metrics
      wasmLogger.log(
        WasmLogLevel.INFO, 
        'init', 
        'WebAssembly module initialized successfully', 
        {
          initTime,
          wasmSize: getWasmSize(), // Calculate size after load
          version: wasmBuildInfo?.version || 'unknown', // Phase 4
          buildDate: wasmBuildInfo?.buildDate || 'unknown', // Phase 4
        }
      );
      
      // Schedule memory usage check
      scheduleMemoryCheck();
      
      // Load saved metrics from localStorage (Phase 4)
      loadSavedMetrics(); 
      
      // Initial threshold from settings
      const $settings = get(settings);
      // TODO: Update Settings type in stores.ts
      if (($settings as any).wasmSizeThreshold) {
        setWasmSizeThreshold(($settings as any).wasmSizeThreshold);
      }
      
      resolve(true);
    } catch (error: any) {
      const endTime = performance.now();
      const initTime = endTime - startTime;
      
      wasmInitialized = false;
      wasmState.initStatus = WasmInitStatus.FAILED; // Update state directly
      wasmState.initTime = initTime; // Record init time even on failure
      
      // Update error state directly
      setWasmError(error); // This also calls reportWasmState
      
      wasmLogger.log(
        WasmLogLevel.ERROR, 
        'init', 
        `WebAssembly initialization failed: ${error.message}`,
        { 
          initTime,
          modulePath: error.modulePath || modulePath || 'unknown', // Include path in error
          errorType: error.name,
        }
      );
      
      resolve(false);
    } finally {
      // Report final initialization status to backend for crash reports
      reportWasmState();
      initializePromise = null; // Reset promise state
    }
  });
  
  return initializePromise;
}

// Schedule regular memory checks when WASM is in use
function scheduleMemoryCheck() {
  if (!wasmInitialized || !wasmModule) return;
  
  // Check memory usage every 30 seconds while module is initialized
  setInterval(() => {
    const currentState = getWasmStateInternal(); // Get current state for check
    // Only check if used recently (e.g., within the last 5 minutes)
    if (wasmModule && currentState.lastUsed && Date.now() - currentState.lastUsed < 300000) { 
      try {
        const memoryInfo = wasmModule.get_memory_usage();
        
        // Update memory usage directly
        updateMemoryUsage(memoryInfo);
        
        // Log memory info periodically
        wasmLogger.log(
          WasmLogLevel.DEBUG, 
          'memory', 
          'Memory usage check', 
          {
            utilization: (memoryInfo.utilization * 100).toFixed(1) + '%',
            used: (memoryInfo.used_bytes / 1024 / 1024).toFixed(2) + 'MB',
            total: (memoryInfo.total_bytes / 1024 / 1024).toFixed(2) + 'MB'
          }
        );
        
      } catch (e: any) {
        wasmLogger.log(WasmLogLevel.ERROR, 'memory', `Memory check failed: ${e.message}`);
      }
    }
  }, 30000);
}

// Get WASM file size (best effort)
function getWasmSize(): number {
  try {
    // This relies on browser performance API and might not always work
    const resources = performance.getEntriesByType('resource');
    // Adjust the name based on the actual built file name
    // Use relative path from public/index.html
    const wasmResource = resources.find(r => r.name.endsWith('/wasm/log_engine_bg.wasm')) as PerformanceResourceTiming | undefined; 
    return wasmResource?.encodedBodySize || 0;
  } catch (e) {
    wasmLogger.log(WasmLogLevel.WARN, 'init', 'Could not determine WASM file size via Performance API.');
    return 0;
  }
}

// --- Phase 4: Persistent Metrics Storage ---

function loadSavedMetrics(): void {
  try {
    const savedMetrics = localStorage.getItem('wasm-metrics');
    if (!savedMetrics) return;
    
    const parsedMetrics = JSON.parse(savedMetrics);
    
    // Carefully merge saved metrics into the current state
    if (parsedMetrics.performanceMetrics) {
      wasmState.performanceMetrics = {
        ...wasmState.performanceMetrics, // Keep existing defaults/structure
        ...parsedMetrics.performanceMetrics // Overwrite with saved values
      };
    }
    
    if (typeof parsedMetrics.totalOperations === 'number') {
      wasmState.totalOperations = parsedMetrics.totalOperations;
    }
    
    if (parsedMetrics.operationsPerType) {
      wasmState.operationsPerType = {
        ...parsedMetrics.operationsPerType // Simple overwrite for operation types
      };
    }
    
    wasmLogger.log(
      WasmLogLevel.INFO,
      'metrics',
      'Loaded saved WebAssembly metrics from localStorage',
      {
        operationsCount: wasmState.performanceMetrics.operationsCount,
        speedupRatio: wasmState.performanceMetrics.speedupRatio?.toFixed(2),
      }
    );
  } catch (error: any) {
    wasmLogger.log(
      WasmLogLevel.WARN,
      'metrics',
      `Failed to load saved metrics: ${error.message}`
    );
    // Clear potentially corrupted data
    localStorage.removeItem('wasm-metrics');
  }
}

// Save metrics to localStorage
function saveMetrics(): void { // Keep internal
  if (!wasmInitialized) return; // Don't save if WASM isn't even initialized

  try {
    const metricsToSave = {
      performanceMetrics: wasmState.performanceMetrics,
      totalOperations: wasmState.totalOperations,
      operationsPerType: wasmState.operationsPerType,
      savedAt: new Date().toISOString(),
    };
    
    localStorage.setItem('wasm-metrics', JSON.stringify(metricsToSave));
    
    wasmLogger.log(
      WasmLogLevel.DEBUG,
      'metrics',
      'Saved WebAssembly metrics to localStorage'
    );
  } catch (error: any) {
    wasmLogger.log(
      WasmLogLevel.WARN,
      'metrics',
      `Failed to save metrics: ${error.message}`
    );
  }
}

// Schedule periodic saving of metrics
function setupMetricsPersistence(): void {
  // Load saved metrics during initialization (called within initializeWasm)
  
  // Save metrics every 5 minutes
  setInterval(() => {
    if (wasmInitialized) { // Only save if initialized
      saveMetrics();
    }
  }, 5 * 60 * 1000);
  
  // Save on page unload
  window.addEventListener('beforeunload', () => {
    if (wasmInitialized) { // Only save if initialized
      saveMetrics();
    }
  });
}

// Initialize metric persistence during module load
setupMetricsPersistence();

// --- Phase 2: Adaptive Threshold Logic ---
// (Added based on Phase 2 refinement plan)
export function shouldUseWasm(totalLogCount: number): boolean {
    // If WASM is not enabled or initialized, don't use it
    if (!isWasmEnabled()) {
        return false;
    }
    
    const currentWasmState = getWasmStateInternal(); // Use internal getter
    const metrics = currentWasmState.performanceMetrics;
    
    // If we haven't measured enough operations, use static threshold
    if (metrics.operationsCount < 5) {
        return totalLogCount >= getWasmSizeThreshold();
    }
    
    // Calculate serialization overhead based on log count
    // This is an estimation that increases logarithmically with size
    // Adjust the factor (0.5) based on real-world testing if needed
    const estimatedSerializationMs = 0.5 * Math.log10(totalLogCount) * totalLogCount / 100;
    
    // Estimate TypeScript execution time based on historical data
    // Assume linear scaling for simplicity (adjust if needed)
    const estimatedTsMsLinear = metrics.avgTsTime * totalLogCount; 

    // Estimate WebAssembly execution time based on historical data
    // Include serialization overhead in the estimation
    const estimatedWasmMsLinear = (metrics.avgWasmTime * totalLogCount) + estimatedSerializationMs;
    
    // Check if estimated WASM performance meets our minimum gain threshold
    const estimatedGain = (estimatedTsMsLinear > 0 && estimatedWasmMsLinear > 0) ? estimatedTsMsLinear / estimatedWasmMsLinear : 0;
    const meetsMinGain = estimatedGain >= WASM_CONFIG.MIN_PERFORMANCE_GAIN;
    
    // Check if the log count is large enough to justify the overhead (use static threshold as a baseline)
    const isLargeEnough = totalLogCount >= getWasmSizeThreshold();
    
    // Make decision based on both factors
    const useWasm = meetsMinGain && isLargeEnough;
    
    // Log the decision for debugging if it's a large dataset
    if (totalLogCount > 1000) {
        wasmLogger.log(
            WasmLogLevel.DEBUG,
            'decision',
            `WASM decision for ${totalLogCount} logs: ${useWasm ? 'Use WASM' : 'Use TypeScript'}`,
            {
                estimatedTsMs: estimatedTsMsLinear.toFixed(2),
                estimatedWasmMs: estimatedWasmMsLinear.toFixed(2),
                estimatedGain: estimatedGain.toFixed(2),
                meetsMinGain,
                isLargeEnough,
                sizeThreshold: getWasmSizeThreshold(),
                minGainRequired: WASM_CONFIG.MIN_PERFORMANCE_GAIN
            }
        );
    }
    
    return useWasm;
}

// --- Phase 2: Memory Estimation Check ---
// (Added based on Phase 2 refinement plan)
export function canProcessLogCount(logCount: number): boolean {
    if (!wasmInitialized || !wasmModule) return false; // Cannot process if not ready
    
    try {
        const estimate = wasmModule.estimate_memory_for_logs(logCount);
        if (estimate && typeof estimate.would_fit === 'boolean') {
            return estimate.would_fit;
        } else {
            wasmLogger.log(WasmLogLevel.WARN, 'memory', 'Invalid memory estimation result received from WASM.');
            return true; // Default to true if estimation fails, rely on runtime errors
        }
    } catch (e: any) {
        wasmLogger.log(
            WasmLogLevel.ERROR,
            'memory',
            `Failed to estimate memory for ${logCount} logs: ${e.message}`
        );
        return true; // Default to true on error to avoid blocking processing unnecessarily
    }
}