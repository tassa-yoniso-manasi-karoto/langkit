/**
 * WebAssembly module interface for Langkit log processing optimization
 * 
 * This module provides WebAssembly-powered performance improvements for
 * critical log processing operations while maintaining full compatibility
 * with environments where WebAssembly is unavailable.
 * 
 * The implementation follows a pragmatic approach focusing on:
 * 1. Getting "90% of the benefits with 10% of the effort"
 * 2. Maintaining compatibility with all environments
 * 3. Providing transparent fallbacks when WebAssembly is unavailable
 * 4. Collecting detailed performance metrics for optimization
 * 
 * WebAssembly Integration Architecture
 * 
 * This implementation consists of several cooperating modules:
 * 
 * 1. wasm.ts (this file)
 *    - Core initialization and configuration
 *    - Feature detection and compatibility checks
 *    - Memory management and performance thresholds
 * 
 * 2. wasm-state.ts
 *    - State tracking for WebAssembly operations
 *    - Performance metrics collection and analysis
 *    - Diagnostic data for crash reports
 * 
 * 3. wasm-logger.ts
 *    - Specialized logging for WebAssembly operations
 *    - Integration with backend crash reporting
 * 
 * 4. Rust implementation (lib.rs)
 *    - Core optimized algorithms
 *    - Memory management and safety checks
 *    - Error handling and diagnostics
 * 
 * The integration is designed to gracefully degrade when WebAssembly
 * is unavailable, ensuring the application remains functional in all
 * environments.
 * 
 * @module wasm
 */
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

// Add this interface for proper type safety
// Add these error classes for better error handling
export class WasmInitializationError extends Error {
  constructor(message: string, public context?: any) {
    super(message);
    this.name = 'WasmInitializationError';
  }
}

export class WasmMemoryError extends Error {
  constructor(message: string, public memoryInfo?: any) {
    super(message);
    this.name = 'WasmMemoryError';
  }
}

export class WasmOperationError extends Error {
  constructor(message: string, public operation: string, public details?: any) {
    super(message);
    this.name = 'WasmOperationError';
  }
}

export interface WasmModule {
  merge_insert_logs: (existingLogs: any[], newLogs: any[]) => any[];
  get_memory_usage: () => {
    total_bytes: number;
    used_bytes: number;
    utilization: number;
    peak_bytes?: number;
    allocation_count?: number;
  };
  force_garbage_collection: () => void;
  estimate_memory_for_logs: (logCount: number) => {
    estimated_bytes: number;
    current_available: number;
    would_fit: boolean;
  };
}

// --- State ---
let wasmModule: WasmModule | null = null;
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

/**
 * Gets the WebAssembly module with proper type checking
 * @returns The WebAssembly module or null if not initialized
 */
export function getWasmModule(): WasmModule | null {
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
        throw new WasmInitializationError("WebAssembly not supported in this browser", {
          runtime: "wails",
          timestamp: Date.now()
        });
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
           throw new WasmInitializationError(`Failed to fetch build info: ${buildInfoResponse.statusText}`, {
             status: buildInfoResponse.status,
             url: buildInfoResponse.url
           });
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
      
      // Add initialization context to error
      handleWasmError(error, 'initialization', {
        initTime,
        modulePath: modulePath || 'unknown',
        buildInfo: wasmBuildInfo || 'unavailable'
      }, true); // Disable on critical init errors
      
      wasmInitialized = false;
      wasmState.initStatus = WasmInitStatus.FAILED;
      wasmState.initTime = initTime;
      
      resolve(false);
    } finally {
      // Report final initialization status to backend for crash reports
      reportWasmState();
      initializePromise = null; // Reset promise state
    }
  });
  
  return initializePromise;
}

// Check memory thresholds and issue warnings when needed
function checkMemoryThresholds(): void {
  if (!isWasmEnabled() || !wasmModule) return;
  
  try {
    const memoryInfo = wasmModule.get_memory_usage();
    
    // Define warning thresholds
    if (memoryInfo.utilization > 0.85) {
      wasmLogger.log(
        WasmLogLevel.WARN,
        'memory',
        `Critical memory pressure: ${(memoryInfo.utilization * 100).toFixed(1)}% used`,
        { memoryInfo }
      );
      // Force garbage collection at critical levels
      wasmModule.force_garbage_collection();
    } else if (memoryInfo.utilization > 0.7) {
      wasmLogger.log(
        WasmLogLevel.WARN,
        'memory',
        `High memory pressure: ${(memoryInfo.utilization * 100).toFixed(1)}% used`,
        { memoryInfo }
      );
    }
  } catch (e: any) {
    wasmLogger.log(
      WasmLogLevel.ERROR, 
      'memory', 
      `Memory check failed: ${e.message}`
    );
  }
}

// Schedule regular memory checks when WASM is in use
function scheduleMemoryCheck() {
  if (!wasmInitialized || !wasmModule) return;
  
  // Setup automatic garbage collection
  setupAutomaticGarbageCollection();
  
  // Also set up monitoring for excessive memory growth
  setupMemoryMonitoring();
  
  // Check memory usage every 30 seconds while module is initialized
  setInterval(() => {
    const currentState = getWasmStateInternal(); // Get current state for check
    // Only check if used recently (e.g., within the last 5 minutes)
    if (wasmModule && currentState.lastUsed && Date.now() - currentState.lastUsed < 300000) { 
      try {
        const memoryInfo = wasmModule.get_memory_usage();
        
        // Update memory usage directly
        updateMemoryUsage(memoryInfo);
        
        // Check memory thresholds
        checkMemoryThresholds();
        
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
  
  // Run more frequent memory threshold checks (every 10 seconds)
  setInterval(() => {
    if (wasmModule && wasmState.lastUsed && Date.now() - wasmState.lastUsed < 300000) {
      checkMemoryThresholds();
    }
  }, 10000);
}

/**
 * Automatic garbage collection scheduler to prevent memory leaks
 * in long-running sessions with adaptive intervals based on memory pressure
 */
function setupAutomaticGarbageCollection(): void {
  let lastGcTime = Date.now();
  let consecutiveHighMemory = 0;
  let adaptiveGcInterval = 60000; // Start with 1 minute
  
  // Adaptive interval adjustment based on memory pressure
  const checkAndAdjustInterval = () => {
    if (!isWasmEnabled() || !wasmModule) return;
    
    try {
      // Get memory info
      const memoryInfo = wasmModule.get_memory_usage();
      
      // Adjust interval based on utilization
      if (memoryInfo.utilization > 0.8) {
        adaptiveGcInterval = 15000; // Every 15 seconds under high pressure
        consecutiveHighMemory++;
      } else if (memoryInfo.utilization > 0.6) {
        adaptiveGcInterval = 30000; // Every 30 seconds under moderate pressure
        consecutiveHighMemory = Math.max(0, consecutiveHighMemory - 1);
      } else {
        adaptiveGcInterval = 60000; // Every minute under low pressure
        consecutiveHighMemory = 0;
      }
      
      // Determine if GC is needed
      const needsGc = (
        Date.now() - lastGcTime > adaptiveGcInterval ||
        memoryInfo.utilization > 0.7 ||
        consecutiveHighMemory >= 3
      );
      
      if (needsGc) {
        wasmLogger.log(
          WasmLogLevel.INFO,
          'memory',
          `Automatic garbage collection triggered`,
          {
            utilization: (memoryInfo.utilization * 100).toFixed(1) + '%',
            timeSinceLastGc: formatTime(Date.now() - lastGcTime),
            adaptiveGcInterval: formatTime(adaptiveGcInterval),
            consecutiveHighMemory
          }
        );
        
        // Perform garbage collection
        wasmModule.force_garbage_collection();
        lastGcTime = Date.now();
        consecutiveHighMemory = 0;
      }
    } catch (e: any) {
      wasmLogger.log(
        WasmLogLevel.ERROR,
        'memory',
        `Automatic garbage collection failed: ${e.message}`
      );
    }
    
    // Schedule next check using the adaptive interval
    setTimeout(checkAndAdjustInterval, adaptiveGcInterval);
  };
  
  // Start the adaptive check cycle
  checkAndAdjustInterval();
}

/**
 * Monitor memory growth to detect potential memory leaks
 */
function setupMemoryMonitoring(): void {
  const memorySnapshots: Array<{timestamp: number, used: number}> = [];
  const snapshotLimit = 10; // Keep last 10 snapshots
  
  // Check memory growth every minute
  setInterval(() => {
    if (!isWasmEnabled() || !wasmModule) return;
    
    try {
      const memoryInfo = wasmModule.get_memory_usage();
      
      // Add snapshot
      memorySnapshots.push({
        timestamp: Date.now(),
        used: memoryInfo.used_bytes
      });
      
      // Keep only last N snapshots
      if (memorySnapshots.length > snapshotLimit) {
        memorySnapshots.shift();
      }
      
      // Need at least 3 snapshots to analyze trend
      if (memorySnapshots.length >= 3) {
        analyzeMemoryTrend(memorySnapshots);
      }
    } catch (e: any) {
      wasmLogger.log(
        WasmLogLevel.ERROR,
        'memory',
        `Memory monitoring failed: ${e.message}`
      );
    }
  }, 60000);
}

/**
 * Analyze memory usage trend to detect leaks
 */
function analyzeMemoryTrend(snapshots: Array<{timestamp: number, used: number}>): void {
  // Calculate growth rate
  const first = snapshots[0];
  const last = snapshots[snapshots.length - 1];
  
  const timeDiffMinutes = (last.timestamp - first.timestamp) / (1000 * 60);
  const memoryGrowthBytes = last.used - first.used;
  const growthRatePerMinute = memoryGrowthBytes / timeDiffMinutes;
  
  // Check if growth rate is concerning
  if (growthRatePerMinute > 1024 * 1024) { // More than 1MB/minute
    wasmLogger.log(
      WasmLogLevel.WARN,
      'memory',
      `Possible memory leak detected: ${formatBytes(growthRatePerMinute)}/minute growth`,
      {
        firstSnapshot: formatBytes(first.used),
        lastSnapshot: formatBytes(last.used),
        timePeriod: `${timeDiffMinutes.toFixed(1)} minutes`,
        totalGrowth: formatBytes(memoryGrowthBytes)
      }
    );
    
    // Force garbage collection to see if it helps
    if (wasmModule && typeof wasmModule.force_garbage_collection === 'function') {
      wasmModule.force_garbage_collection();
      
      // Check effect of garbage collection
      setTimeout(() => {
        try {
          const afterGcInfo = wasmModule.get_memory_usage();
          const memoryFreed = last.used - afterGcInfo.used_bytes;
          
          wasmLogger.log(
            WasmLogLevel.INFO,
            'memory',
            `Garbage collection effect: ${formatBytes(memoryFreed)} freed`,
            {
              beforeGc: formatBytes(last.used),
              afterGc: formatBytes(afterGcInfo.used_bytes),
              isLeakConfirmed: memoryFreed < memoryGrowthBytes * 0.5
            }
          );
        } catch (e: any) {
          wasmLogger.log(WasmLogLevel.ERROR, 'memory', `Failed to check GC effect: ${e.message}`);
        }
      }, 1000);
    }
  }
}

// Helper functions for memory monitoring
function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatTime(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60 * 1000) return `${(ms / 1000).toFixed(1)}s`;
  return `${(ms / (60 * 1000)).toFixed(1)}m`;
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

/**
 * Updates the optimal threshold for WebAssembly usage based on measured performance
 * This function adjusts the threshold automatically to maximize performance
 */
function updateOptimalThreshold(): void {
  if (!wasmInitialized || wasmState.performanceMetrics.operationsCount < 10) {
    return; // Not enough data to make good decisions
  }
  
  const metrics = wasmState.performanceMetrics;
  
  // Only adjust if we have enough data points with a clear performance difference
  if (metrics.avgWasmTime > 0 && metrics.avgTsTime > 0) {
    const currentSpeedup = metrics.speedupRatio;
    const currentThreshold = getWasmSizeThreshold();
    
    // Adjust threshold based on measured performance
    if (currentSpeedup > 3.0) {
      // Significant speedup - consider lowering threshold
      const newThreshold = Math.max(
        WASM_CONFIG.MIN_THRESHOLD,
        Math.min(currentThreshold, Math.round(currentThreshold * 0.8))
      );
      
      if (newThreshold !== currentThreshold) {
        wasmLogger.log(
          WasmLogLevel.INFO,
          'threshold',
          `Auto-adjusting threshold based on performance metrics`,
          {
            previousThreshold: currentThreshold,
            newThreshold: newThreshold,
            speedupRatio: currentSpeedup,
            reason: 'high performance gain'
          }
        );
        
        setWasmSizeThreshold(newThreshold);
      }
    } else if (currentSpeedup < 1.2) {
      // Minimal speedup - consider raising threshold
      const newThreshold = Math.min(
        WASM_CONFIG.MAX_THRESHOLD,
        Math.max(currentThreshold, Math.round(currentThreshold * 1.2))
      );
      
      if (newThreshold !== currentThreshold) {
        wasmLogger.log(
          WasmLogLevel.INFO,
          'threshold',
          `Auto-adjusting threshold based on performance metrics`,
          {
            previousThreshold: currentThreshold,
            newThreshold: newThreshold,
            speedupRatio: currentSpeedup,
            reason: 'low performance gain'
          }
        );
        
        setWasmSizeThreshold(newThreshold);
      }
    }
  }
}

// Check for optimal threshold adjustments periodically
setInterval(() => {
  if (wasmInitialized && wasmState.performanceMetrics.operationsCount > 10) {
    updateOptimalThreshold();
  }
}, 10 * 60 * 1000); // Check every 10 minutes

/**
 * Centralized error handler for WebAssembly operations
 * Logs errors, updates state, and performs recovery actions
 * 
 * @param error The error that occurred
 * @param operation The operation that failed
 * @param context Additional context information
 * @param disableOnCritical Whether to disable WebAssembly on critical errors
 */
export function handleWasmError(
  error: Error,
  operation: string,
  context: Record<string, any> = {},
  disableOnCritical: boolean = false
): void {
  // Determine error severity
  const isCritical = isCriticalWasmError(error);
  const logLevel = isCritical ? WasmLogLevel.CRITICAL : WasmLogLevel.ERROR;
  
  // Add stack trace and browser information
  const enhancedContext = {
    ...context,
    errorName: error.name,
    errorStack: error.stack,
    operation,
    browserInfo: {
      userAgent: navigator.userAgent,
      platform: navigator.platform,
      language: navigator.language,
      timestamp: new Date().toISOString()
    },
    wasmState: {
      initStatus: wasmState.initStatus,
      totalOperations: wasmState.totalOperations,
      memoryUtilization: wasmState.memoryUsage?.utilization
    }
  };
  
  // Log the error with enhanced context
  wasmLogger.log(
    logLevel,
    'error',
    `WebAssembly ${operation} failed: ${error.message}`,
    enhancedContext
  );
  
  // Update error state
  setWasmError(error);
  
  // Report to backend immediately for crash reporting
  reportWasmState();
  
  // Disable WebAssembly for critical errors if requested
  if (isCritical && disableOnCritical) {
    wasmLogger.log(
      WasmLogLevel.CRITICAL,
      'system',
      `Disabling WebAssembly due to critical error in ${operation}`
    );
    enableWasm(false);
  }
}

/**
 * Determines if a WebAssembly error is critical
 * Critical errors indicate fundamental problems with WebAssembly execution
 */
function isCriticalWasmError(error: Error): boolean {
  // Check error types that indicate critical failures
  if (error instanceof WebAssembly.RuntimeError) return true;
  if (error instanceof WebAssembly.LinkError) return true;
  if (error instanceof WebAssembly.CompileError) return true;
  
  // Check for memory-related errors
  const errorMsg = error.message.toLowerCase();
  if (errorMsg.includes('memory') && 
      (errorMsg.includes('out of') || errorMsg.includes('allocation'))) {
    return true;
  }
  
  // Check for initialization errors
  if (errorMsg.includes('initialize') || errorMsg.includes('not ready')) {
    return true;
  }
  
  return false;
}

// --- Phase 2: Adaptive Threshold Logic ---
// (Added based on Phase 2 refinement plan)
/**
 * Determines whether to use WebAssembly based on log count and performance metrics
 * This implements adaptive thresholds that learn from actual performance measurements
 * 
 * @param totalLogCount The total number of logs to be processed
 * @returns boolean indicating whether WebAssembly should be used
 */
export function shouldUseWasm(totalLogCount: number): boolean {
  // If WASM is not enabled or initialized, don't use it
  if (!isWasmEnabled()) {
    return false;
  }
  
  // Get current state and metrics
  const currentState = getWasmStateInternal();
  const metrics = currentState.performanceMetrics;
  
  // If we haven't measured enough operations, use static threshold
  if (metrics.operationsCount < 5) {
    return totalLogCount >= getWasmSizeThreshold();
  }
  
  // Calculate serialization overhead based on log count
  // Serialization increases with log size, approximated with logarithmic scaling
  const estimatedSerializationMs = 0.3 * Math.log10(totalLogCount) * totalLogCount / 100;
  
  // Estimate TypeScript execution time based on historical data
  const estimatedTsMs = metrics.avgTsTime * (totalLogCount / 1000);
  
  // Estimate WebAssembly execution time with serialization overhead
  const estimatedWasmMs = (metrics.avgWasmTime * (totalLogCount / 1000)) + estimatedSerializationMs;
  
  // Calculate estimated performance gain
  let estimatedGain = 1.0;
  if (estimatedWasmMs > 0) {
    estimatedGain = estimatedTsMs / estimatedWasmMs;
  }
  
  // Require minimum performance gain to justify WebAssembly overhead
  const meetsMinGain = estimatedGain >= WASM_CONFIG.MIN_PERFORMANCE_GAIN;
  
  // Check if log count exceeds the configured threshold
  const isLargeEnough = totalLogCount >= getWasmSizeThreshold();
  
  // Combined decision: use WebAssembly for large datasets with performance gain
  const useWasm = meetsMinGain && isLargeEnough;
  
  // Log decision for large datasets to help with performance tuning
  if (totalLogCount > 1000 || metrics.operationsCount < 10) {
    wasmLogger.log(
      WasmLogLevel.DEBUG,
      'threshold',
      `WebAssembly decision for ${totalLogCount} logs: ${useWasm ? 'Use WASM' : 'Use TypeScript'}`,
      {
        estimatedTsMs: estimatedTsMs.toFixed(2),
        estimatedWasmMs: estimatedWasmMs.toFixed(2),
        estimatedGain: estimatedGain.toFixed(2),
        serialization: estimatedSerializationMs.toFixed(2),
        meetsMinGain,
        isLargeEnough,
        threshold: getWasmSizeThreshold(),
        minGainRequired: WASM_CONFIG.MIN_PERFORMANCE_GAIN
      }
    );
  }
  
  return useWasm;
}

// --- Phase 2: Memory Estimation Check ---
// (Added based on Phase 2 refinement plan)
/**
 * Determines whether the WebAssembly module can safely process the given 
 * number of logs without memory issues
 * 
 * @param logCount The total number of logs to be processed
 * @returns true if it's safe to process with WebAssembly, false otherwise
 */
export function canProcessSafely(logCount: number): boolean {
  // Skip check if WebAssembly is not enabled
  if (!isWasmEnabled() || !wasmModule) return true;
  
  try {
    // Check if module has the estimation function
    if (typeof wasmModule.estimate_memory_for_logs !== 'function') {
      wasmLogger.log(
        WasmLogLevel.WARN, 
        'memory', 
        'Memory estimation function not available'
      );
      return logCount < getWasmSizeThreshold() * 2;
    }
    
    // Call the Rust function to estimate memory requirements
    const estimate = wasmModule.estimate_memory_for_logs(logCount);
    
    // Log detailed estimation for large operations
    if (logCount > 1000) {
      wasmLogger.log(
        WasmLogLevel.DEBUG, 
        'memory', 
        `Memory estimation for ${logCount} logs`,
        {
          estimated_bytes: estimate.estimated_bytes,
          available_bytes: estimate.current_available,
          would_fit: estimate.would_fit
        }
      );
    }
    
    return estimate.would_fit;
  } catch (e: any) {
    wasmLogger.log(
      WasmLogLevel.WARN, 
      'memory', 
      `Memory estimation failed: ${e.message}`,
      { logCount }
    );
    
    // Fall back to a conservative size-based approach
    return logCount < getWasmSizeThreshold() * 2;
  }
}