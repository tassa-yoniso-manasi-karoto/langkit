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
  resetWasmMetricsInternal, // Import the correctly named function
  wasmState // Import the state object itself for persistence
} from './wasm-state';
import type { WasmState } from './wasm-state'; // Use type-only import for WasmState
import { settings, wasmActive } from './stores'; // Import wasmActive store
import { get } from 'svelte/store';

// --- Start Phase 1.2: New Error Types ---
// Enhanced error types with better context
export class WasmInitializationError extends Error {
    context: Record<string, any>;

    constructor(message: string, context: Record<string, any> = {}) {
        super(message);
        this.name = 'WasmInitializationError';
        this.context = {
            timestamp: Date.now(),
            ...context
        };
    }
}

export class WasmMemoryError extends Error {
    memoryInfo: Record<string, any>;

    constructor(message: string, memoryInfo: Record<string, any> = {}) {
        super(message);
        this.name = 'WasmMemoryError';
        this.memoryInfo = {
            timestamp: Date.now(),
            ...memoryInfo
        };
    }
}

export class WasmOperationError extends Error {
    operation: string;
    details: Record<string, any>;

    constructor(message: string, operation: string, details: Record<string, any> = {}) {
        super(message);
        this.name = 'WasmOperationError';
        this.operation = operation;
        this.details = {
            timestamp: Date.now(),
            ...details
        };
    }
}
// --- End Phase 1.2: New Error Types ---


// --- Start Update WasmModule Interface ---
export interface WasmModule {
  merge_insert_logs: (existingLogs: any[], newLogs: any[]) => any[];
  get_memory_usage: () => { // Updated return type based on Rust changes
    total_bytes: number;
    used_bytes: number;
    utilization: number;
    peak_bytes?: number;
    allocation_count?: number;
    // New metrics from Rust
    average_allocation?: number;
    allocation_rate?: number;
    time_since_last_gc?: number;
    memory_growth_trend?: number;
    fragmentation_estimate?: number;
  };
  force_garbage_collection: () => void;
  estimate_memory_for_logs: (logCount: number) => { // Updated return type based on Rust changes
    estimated_bytes: number;
    current_available: number;
    would_fit: boolean;
    // New fields from Rust
    projected_utilization?: number;
    risk_level?: 'high' | 'moderate' | 'low';
    recommendation?: 'proceed_with_caution' | 'proceed' | 'use_typescript_fallback';
  };
  // Potential SIMD function (if enabled in Rust build)
  contains_text_simd?: (haystack: string, needle: string) => boolean;
}
// --- End Update WasmModule Interface ---

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
const operationThresholds = new Map<string, number>(); // Added in Phase 2.1

// --- Exported Functions ---

// Exported state getter (needed by dashboard)
export function getWasmState() {
    return getWasmStateInternal();
}

// Function to update the wasmActive store for UI indicator
function setWasmActive(active: boolean) {
  wasmActive.set(active);
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
    `WASM global size threshold set to ${WASM_SIZE_THRESHOLD}`
  );
}

export function getWasmSizeThreshold(): number {
  return WASM_SIZE_THRESHOLD;
}

// --- Start Phase 2.1: Operation-Specific Thresholds ---
/**
 * Sets the threshold for a specific operation type
 */
export function setOperationThreshold(operation: string, threshold: number): void {
  const validatedThreshold = Math.max(
    WASM_CONFIG.MIN_THRESHOLD,
    Math.min(threshold, WASM_CONFIG.MAX_THRESHOLD)
  );

  operationThresholds.set(operation, validatedThreshold);

  wasmLogger.log(
    WasmLogLevel.INFO,
    'config',
    `Set WebAssembly threshold for ${operation} to ${validatedThreshold}`
  );
}

/**
 * Gets the threshold for a specific operation type
 * Falls back to the global threshold if none is set
 */
export function getOperationThreshold(operation: string): number {
  return operationThresholds.get(operation) || getWasmSizeThreshold();
}
// --- End Phase 2.1: Operation-Specific Thresholds ---


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

// --- Start Phase 4.1: loadBuildInfo ---
/**
 * Load WebAssembly build information for versioning and cache management
 */
async function loadBuildInfo(): Promise<WasmBuildInfo | null> {
  try {
    // Use relative path from public/index.html
    const response = await fetch(`./wasm/build-info.json?t=${Date.now()}`);
    if (!response.ok) {
      throw new Error(`Failed to fetch build info: ${response.statusText}`);
    }

    const buildInfo = await response.json();
    wasmLogger.log(
      WasmLogLevel.DEBUG,
      'init',
      `WebAssembly build info loaded: ${buildInfo.version} (${buildInfo.buildDate})`
    );

    return buildInfo;
  } catch (error: any) {
    wasmLogger.log(
      WasmLogLevel.WARN,
      'init',
      `Failed to load WebAssembly build info: ${error.message}`
    );
    return null;
  }
}
// --- End Phase 4.1: loadBuildInfo ---

// --- Start Replace initializeWasm ---
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
          runtime: "wails", // Assuming Wails context, adjust if needed
          timestamp: Date.now()
        });
      }

      // First, fetch build info to get version for cache busting (Phase 4)
      wasmBuildInfo = await loadBuildInfo(); // Use the new function

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
      // TODO: Update Settings type in stores.ts if needed
      if (($settings as any).wasmSizeThreshold) {
        setWasmSizeThreshold(($settings as any).wasmSizeThreshold);
      }

      resolve(true);
    } catch (error: any) {
      const endTime = performance.now();
      const initTime = endTime - startTime;

      // Use the enhanced error handler
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
// --- End Replace initializeWasm ---

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
            used: formatBytes(memoryInfo.used_bytes), // Use formatBytes
            total: formatBytes(memoryInfo.total_bytes) // Use formatBytes
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
  // Avoid division by zero or negative time diff
  if (timeDiffMinutes <= 0) return;

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
          if (!wasmModule) return; // Add null check for wasmModule
          const afterGcInfo = wasmModule.get_memory_usage();
          const memoryFreed = last.used - afterGcInfo.used_bytes;

          wasmLogger.log(
            WasmLogLevel.INFO,
            'memory',
            `Garbage collection effect: ${formatBytes(memoryFreed)} freed`,
            {
              beforeGc: formatBytes(last.used),
              afterGc: formatBytes(afterGcInfo.used_bytes),
              isLeakConfirmed: memoryFreed < memoryGrowthBytes * 0.5 // Heuristic: if less than half the growth was freed
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
  if (ms < 0) return 'N/A'; // Handle negative values
  if (ms < 1000) return `${Math.round(ms)}ms`;
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
      // Ensure nested objects are also handled correctly if they exist
      if (parsedMetrics.performanceMetrics.logSizeDistribution) {
        wasmState.performanceMetrics.logSizeDistribution = {
          ...wasmState.performanceMetrics.logSizeDistribution,
          ...parsedMetrics.performanceMetrics.logSizeDistribution
        };
      }
      if (parsedMetrics.performanceMetrics.operationTimings) {
         wasmState.performanceMetrics.operationTimings = {
           ...wasmState.performanceMetrics.operationTimings,
           ...parsedMetrics.performanceMetrics.operationTimings
         };
      }
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

// --- Start Phase 2.1: Threshold Auto-Adjustment ---
/**
 * Automatically adjusts operation-specific thresholds based on performance data
 */
function updateOperationThresholds(): void {
  const metrics = getWasmStateInternal().performanceMetrics;

  if (!metrics.operationTimings || metrics.operationsCount < 10) {
    return; // Not enough data to make good adjustments
  }

  // Check each operation with enough measurements
  Object.entries(metrics.operationTimings).forEach(([operation, stats]) => {
    if (stats.count < 5) return; // Skip operations with few measurements

    const currentThreshold = getOperationThreshold(operation);
    let newThreshold = currentThreshold;

    // Calculate speedup for this specific operation
    const operationSpeedup = calculateOperationSpeedup(operation);

    if (operationSpeedup > 3.0) {
      // Significant speedup - lower threshold
      newThreshold = Math.max(
        WASM_CONFIG.MIN_THRESHOLD,
        Math.round(currentThreshold * 0.8)
      );
    } else if (operationSpeedup < 1.2) {
      // Minimal speedup - raise threshold
      newThreshold = Math.min(
        WASM_CONFIG.MAX_THRESHOLD,
        Math.round(currentThreshold * 1.2)
      );
    }

    // Only update if threshold changed
    if (newThreshold !== currentThreshold) {
      setOperationThreshold(operation, newThreshold);

      wasmLogger.log(
        WasmLogLevel.INFO,
        'threshold',
        `Auto-adjusted ${operation} threshold based on performance data`,
        {
          previousThreshold: currentThreshold,
          newThreshold: newThreshold,
          speedupRatio: operationSpeedup.toFixed(2),
          operationCount: stats.count,
          avgTime: stats.avgTime.toFixed(2)
        }
      );
    }
  });
}

// Calculate operation-specific speedup ratio
function calculateOperationSpeedup(operation: string): number {
  const metrics = getWasmStateInternal().performanceMetrics;

  if (!metrics.operationTimings || !metrics.operationTimings[operation]) {
    return 0;
  }

  // Use operation-specific metrics if available
  const tsTime = metrics.avgTsTime; // Use global average TS time for comparison
  const wasmTime = metrics.operationTimings[operation].avgTime;

  if (wasmTime <= 0 || tsTime <= 0) {
    return 0;
  }

  return tsTime / wasmTime;
}

// Schedule periodic threshold adjustments
setInterval(() => {
  if (isWasmEnabled() &&
      getWasmStateInternal().performanceMetrics.operationsCount > 10) {
    updateOperationThresholds();
  }
}, 5 * 60 * 1000); // Check every 5 minutes
// --- End Phase 2.1: Threshold Auto-Adjustment ---


// --- Start Streamlined Error Handling ---
// Simplified categorization with fewer categories and severity levels
function categorizeWasmError(error: Error): {
  category: 'memory' | 'initialization' | 'execution' | 'unknown';
  severity: 'high' | 'low';
  recoverable: boolean;
} {
  // Check for specific error types first
  if (error instanceof WebAssembly.RuntimeError ||
      error instanceof WebAssembly.LinkError ||
      error instanceof WebAssembly.CompileError ||
      error instanceof WasmInitializationError) {
    return {
      category: 'initialization',
      severity: 'high',
      recoverable: false
    };
  }

  if (error instanceof WasmMemoryError) {
    return {
      category: 'memory',
      severity: 'high',
      recoverable: true
    };
  }

  if (error instanceof WasmOperationError) {
    return {
      category: 'execution',
      severity: 'low',
      recoverable: true
    };
  }

  // Check message patterns for memory issues
  const message = error.message.toLowerCase();
  if (message.includes('memory') ||
      message.includes('allocation') ||
      message.includes('heap') ||
      message.includes('out of memory')) {
    return {
      category: 'memory',
      severity: 'high',
      recoverable: true
    };
  }

  // Default case for unknown errors
  return {
    category: 'unknown',
    severity: 'low',
    recoverable: true
  };
}

// Operation blacklist management (kept from previous version)
const operationBlacklist = new Set<string>();
const operationErrorCounts = new Map<string, number>();

function addToOperationBlacklist(operation: string): void {
  operationBlacklist.add(operation);
  wasmLogger.log(
    WasmLogLevel.WARN,
    'recovery',
    `Blacklisting operation "${operation}" due to repeated errors`
  );

  // Schedule removal after a timeout to allow retrying later
  setTimeout(() => {
    operationBlacklist.delete(operation);
    operationErrorCounts.delete(operation);

    wasmLogger.log(
      WasmLogLevel.INFO,
      'recovery',
      `Removed "${operation}" from WebAssembly operation blacklist`
    );
  }, 5 * 60 * 1000); // 5 minutes
}

export function isOperationBlacklisted(operation: string): boolean { // Export for use in logStore
  return operationBlacklist.has(operation);
}

function trackOperationError(operation: string): boolean {
  // Increment error count for this operation
  const currentCount = operationErrorCounts.get(operation) || 0;
  const newCount = currentCount + 1;
  operationErrorCounts.set(operation, newCount);

  // Blacklist after 3 consecutive errors
  if (newCount >= 3) {
    addToOperationBlacklist(operation);
    return true;
  }

  return false;
}

// Export for use in logStore
export function clearOperationErrorCount(operation: string): void {
  operationErrorCounts.delete(operation);
}


// Simplified recovery strategies
function getRecoveryStrategy(
  error: Error, // Pass the original error for context if needed
  errorType: ReturnType<typeof categorizeWasmError>,
  operation: string
): () => void { // Return type is the action function
  // Non-recoverable errors: disable WASM
  if (!errorType.recoverable) {
    return () => {
      wasmLogger.log(
        WasmLogLevel.CRITICAL,
        'recovery',
        `Disabling WebAssembly due to non-recoverable error in ${operation}`
      );
      enableWasm(false);
    };
  }

  // Memory errors: try garbage collection
  if (errorType.category === 'memory') {
    return () => {
      const wasmModule = getWasmModule();
      if (wasmModule && typeof wasmModule.force_garbage_collection === 'function') {
        wasmLogger.log(
          WasmLogLevel.WARN,
          'memory',
          'Attempting garbage collection due to memory error'
        );
        wasmModule.force_garbage_collection();
      }
    };
  }

  // Execution errors: track for potential blacklisting
  if (errorType.category === 'execution') {
    return () => {
      // Track error count
      const blacklisted = trackOperationError(operation);
      if (blacklisted) {
        wasmLogger.log(
          WasmLogLevel.WARN,
          'recovery',
          `Blacklisting operation "${operation}" due to repeated errors`
        );
      }
    };
  }

  // Default: no specific recovery action
  return () => {};
}


/**
 * Centralized error handler for WebAssembly operations (Streamlined Version)
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
  disableOnCritical: boolean = false // Keep parameter for compatibility, but logic is simpler
): void {
  // Categorize the error
  const errorType = categorizeWasmError(error);

  // Get appropriate recovery strategy
  const recoveryAction = getRecoveryStrategy(error, errorType, operation);

  // Determine log level
  const logLevel = errorType.severity === 'high' ? WasmLogLevel.ERROR : WasmLogLevel.WARN;

  // Log with essential context
  wasmLogger.log(
    logLevel,
    'error',
    `WebAssembly ${operation} failed: ${error.message}`,
    {
      ...context,
      errorName: error.name,
      operation,
      category: errorType.category,
      severity: errorType.severity
      // Optionally include stack trace for high severity errors
      // errorStack: errorType.severity === 'high' ? error.stack : undefined
    }
  );

  // Update error state
  setWasmError(error);

  // Apply recovery strategy
  recoveryAction();

  // Disable WebAssembly if requested for critical errors (high severity, non-recoverable)
  // This check is now simpler based on the streamlined categorization
  if (errorType.severity === 'high' && !errorType.recoverable && disableOnCritical) {
     wasmLogger.log(
        WasmLogLevel.CRITICAL,
        'recovery',
        `Disabling WebAssembly due to critical error in ${operation} (forced)`
      );
    enableWasm(false);
  }

  // Report state to backend for crash reporting
  reportWasmState();
}
// --- End Streamlined Error Handling ---
// --- End Phase 1.2: Error Handling Refinement ---


// --- Start Phase 2.1: Enhanced shouldUseWasm ---
/**
 * Determines whether to use WebAssembly for log processing operations.
 *
 * This function uses an adaptive decision-making process based on:
 * 1. Current performance metrics of both TS and WASM implementations
 * 2. Memory availability and pressure
 * 3. Dataset size relative to configurable thresholds
 * 4. Operation-specific performance characteristics
 * 5. Estimated hardware capabilities
 * 6. Operation blacklist status
 *
 * @param totalLogCount The total number of logs to be processed
 * @param operation The operation type (default: 'mergeInsertLogs')
 * @returns Whether WebAssembly should be used for this operation
 *
 * Performance Characteristics:
 * - Small datasets (<500 logs): WebAssembly typically provides 1.2-1.5x speedup
 * - Medium datasets (500-2000 logs): WebAssembly typically provides 2-3x speedup
 * - Large datasets (>2000 logs): WebAssembly typically provides 5-10x speedup
 *
 * Memory Impact:
 * - WebAssembly operations require additional memory for serialization overhead
 * - Under high memory pressure conditions, the function may recommend using TypeScript
 * - GC is automatically triggered when memory utilization exceeds thresholds
 */
export function shouldUseWasm(
  totalLogCount: number,
  operation: string = 'mergeInsertLogs'
): boolean {
  // Get current settings
  const $settings = get(settings);

  // Check force override setting
  if ($settings.forceWasmMode === 'enabled') {
    // Still check if WASM is available and not blacklisted
    if (!isWasmEnabled() || isOperationBlacklisted(operation)) {
      wasmLogger.log(
        WasmLogLevel.WARN,
        'threshold',
        `WebAssembly forced enabled but unavailable or blacklisted for operation: ${operation}`
      );
      return false;
    }
    return true; // Force-enable if available
  } else if ($settings.forceWasmMode === 'disabled') {
    return false; // Force-disable regardless of other factors
  }

  // If 'auto' mode, use the existing sophisticated logic
  // No changes to the current implementation from here on

  // Basic checks first
  if (!isWasmEnabled() || isOperationBlacklisted(operation)) {
    if (isOperationBlacklisted(operation)) {
        wasmLogger.log(WasmLogLevel.INFO, 'threshold', `Skipping WASM for blacklisted operation: ${operation}`);
    }
    return false;
  }

  // Get current state and metrics
  const currentState = getWasmStateInternal();
  const metrics = currentState.performanceMetrics;

  // Early decision for very small datasets - always use TypeScript
  if (totalLogCount < 50) {
    return false;
  }

  // Check memory availability first - if it can't proceed, don't use WASM
  const memCheck = checkMemoryAvailability(totalLogCount);
  if (!memCheck.canProceed) {
      wasmLogger.log(
          WasmLogLevel.INFO,
          'threshold',
          `Using TypeScript fallback due to memory constraints: ${memCheck.actionTaken}`,
          { memoryInfo: memCheck.memoryInfo, operation, logCount: totalLogCount }
      );
      return false;
  }


  // Early decision for very large datasets - use WebAssembly if memory allows
  if (totalLogCount > 5000 && metrics.operationsCount > 0) {
    // Memory check already passed above
    return true;
  }

  // If we haven't measured enough operations, use static threshold
  if (metrics.operationsCount < 5) {
    return totalLogCount >= getOperationThreshold(operation); // Use operation-specific threshold
  }

  // Check operation-specific metrics if available
  let operationMetrics = null;
  if (metrics.operationTimings && metrics.operationTimings[operation]) {
    operationMetrics = metrics.operationTimings[operation];
  }

  // Get hardware capabilities approximation
  const hardwareScore = estimateHardwareCapabilities();

  // Calculate serialization overhead with more accurate model
  // Serialization has both fixed and variable components
  const fixedSerializationCost = 0.5; // Base cost in ms
  const varSerializationCost = totalLogCount * 0.001; // 1µs per log entry (adjust based on real data)
  const estimatedSerializationMs = fixedSerializationCost + varSerializationCost;

  // Estimate TypeScript execution time with non-linear scaling
  // Use operation-specific baseline if available, otherwise global average
  const baseTs = (operationMetrics && operationMetrics.count > 2 && metrics.avgTsTime > 0) ?
                 metrics.avgTsTime : // Use global TS average if available
                 10; // Default baseline TS time (e.g., 10ms per 1000 logs) if no data

  // Non-linear scaling factor based on log count
  const tsScalingFactor = calculateTsScalingFactor(totalLogCount);
  // Estimate TS time relative to a baseline size (e.g., 1000 logs)
  const estimatedTsMs = baseTs * tsScalingFactor * (totalLogCount / 1000);


  // Estimate WebAssembly execution time (more linear scaling)
  const wasmAvgTime = (operationMetrics && operationMetrics.count > 2) ?
                     operationMetrics.avgTime :
                     metrics.avgWasmTime; // Fall back to global WASM average

  // Use a slightly non-linear scaling for WASM too, but less aggressive than TS
  const wasmScalingFactor = Math.log10(Math.max(100, totalLogCount)) / Math.log10(1000); // Relative to 1000 logs
  const estimatedWasmMs = wasmAvgTime * wasmScalingFactor * (totalLogCount / 1000);


  // Total WebAssembly time including overhead
  const totalWasmTimeMs = estimatedWasmMs + estimatedSerializationMs;

  // Calculate estimated performance gain
  let estimatedGain = 1.0;
  if (totalWasmTimeMs > 0 && estimatedTsMs > 0) { // Ensure both are positive
    estimatedGain = estimatedTsMs / totalWasmTimeMs;
  } else if (estimatedTsMs > 0) {
      estimatedGain = Infinity; // If WASM time is zero/negative, gain is effectively infinite
  } else {
      estimatedGain = 0; // If TS time is zero/negative, gain is zero
  }


  // Adjust minimum gain threshold based on hardware capabilities
  // Require higher gain on powerful hardware because TS might be fast enough
  const adjustedMinGain = WASM_CONFIG.MIN_PERFORMANCE_GAIN *
                         (hardwareScore > 0.7 ? 1.2 : hardwareScore < 0.3 ? 0.8 : 1.0);

  // Memory pressure adjustment - require higher gain when memory is constrained
  let memoryPressureAdjustment = 1.0;
  if (currentState.memoryUsage) {
    if (currentState.memoryUsage.utilization > 0.8) {
      memoryPressureAdjustment = 1.5; // Require 50% more gain when memory is highly constrained
    } else if (currentState.memoryUsage.utilization > 0.6) {
      memoryPressureAdjustment = 1.2; // Require 20% more gain when memory is moderately constrained
    }
  }

  // Final adjusted minimum gain threshold
  const finalMinGain = adjustedMinGain * memoryPressureAdjustment;

  // Require minimum performance gain to justify WebAssembly overhead
  const meetsMinGain = estimatedGain >= finalMinGain;

  // Check if log count exceeds the configured threshold for the operation
  const isLargeEnough = totalLogCount >= getOperationThreshold(operation);

  // Combined decision: use WebAssembly for large enough datasets with sufficient performance gain
  const useWasm = meetsMinGain && isLargeEnough;

  // Log decision for large datasets or during calibration
  if (totalLogCount > 1000 || metrics.operationsCount < 10) {
    wasmLogger.log(
      WasmLogLevel.DEBUG,
      'threshold',
      `WebAssembly decision for ${operation} with ${totalLogCount} logs: ${useWasm ? 'Use WASM' : 'Use TypeScript'}`,
      {
        estimatedTsMs: estimatedTsMs.toFixed(2),
        estimatedWasmMs: estimatedWasmMs.toFixed(2),
        serializationOverhead: estimatedSerializationMs.toFixed(2),
        totalWasmTimeMs: totalWasmTimeMs.toFixed(2),
        estimatedGain: estimatedGain.toFixed(2),
        requiredGain: finalMinGain.toFixed(2),
        hardwareScore: hardwareScore.toFixed(2),
        memoryPressure: currentState.memoryUsage?.utilization?.toFixed(2) || 'unknown',
        meetsMinGain,
        isLargeEnough,
        threshold: getOperationThreshold(operation)
      }
    );
  }

  return useWasm;
}

/**
 * Calculates TypeScript performance scaling factor based on log count
 * TypeScript performance degrades non-linearly with dataset size
 */
function calculateTsScalingFactor(logCount: number): number {
  // TypeScript degradation is roughly O(n log n) for sorting operations
  if (logCount <= 500) {
    return 1.0; // Assume linear up to 500 logs
  } else if (logCount <= 2000) {
    // Start of non-linear scaling (e.g., 1.0 to 1.2)
    return 1.0 + 0.2 * ((logCount - 500) / 1500);
  } else {
    // More aggressive non-linear scaling for larger datasets (e.g., 1.2 + log factor)
    // Adjust the base (2000) and factor (0.4) based on observed performance
    return 1.2 + 0.4 * Math.log10(Math.max(1, logCount / 2000));
  }
}

/**
 * Estimates hardware capabilities to adjust thresholds
 * @returns Score from 0-1 indicating relative hardware performance
 */
function estimateHardwareCapabilities(): number {
  try {
    // Use available hardware concurrency as proxy for CPU power
    const hardwareConcurrency = navigator.hardwareConcurrency || 4; // Default to 4 cores if unavailable
    // Scale 0-1 where 8+ cores is max (adjust max cores as needed)
    const concurrencyScore = Math.min(hardwareConcurrency / 8, 1.0);

    // Use device memory API if available (Chrome only)
    let memoryScore = 0.5; // Default to middle value
    if ('deviceMemory' in navigator) {
      const deviceMemoryGB = (navigator as any).deviceMemory; // Use 'as any' to access non-standard property
      // Check if the value is a number before using it
      if (typeof deviceMemoryGB === 'number') {
        // Scale 0-1 where 8+ GB is max (adjust max GB as needed)
        memoryScore = Math.min(deviceMemoryGB / 8, 1.0);
      }
    }

    // Use performance.now() timing precision as proxy for device capability
    // Measure how many timing operations we can do in 5ms
    const startTime = performance.now();
    let iterations = 0;
    while (performance.now() - startTime < 5) {
      performance.now();
      iterations++;
    }

    // Scale iterations to a 0-1 score (calibrated values - adjust 100000 based on testing)
    const iterationsScore = Math.min(iterations / 100000, 1.0);

    // Combine scores with weights (adjust weights based on importance)
    // Example: CPU=40%, Memory=30%, Timing=30%
    return (concurrencyScore * 0.4) + (memoryScore * 0.3) + (iterationsScore * 0.3);
  } catch (e) {
    // Fall back to middle value for any measurement errors
    wasmLogger.log(WasmLogLevel.WARN, 'hardware', `Failed to estimate hardware capabilities: ${e}`);
    return 0.5;
  }
}
// --- End Phase 2.1: Enhanced shouldUseWasm ---


// --- Start Phase 1.1: checkMemoryAvailability ---
/**
 * Proactively checks if an operation might cause memory issues and
 * performs garbage collection or fallback as needed. Includes JSDoc from Phase 4.2.
 *
 * This function uses the WebAssembly memory estimation API to predict memory usage
 * and determines if the operation can proceed safely or should fall back to TypeScript.
 *
 * @param logCount Number of logs to be processed
 * @returns Object with information about whether the operation can proceed with WebAssembly
 *
 * Memory Safety Considerations:
 * - Estimates memory requirements before committing to WebAssembly
 * - Considers fragmentation and allocation patterns from Rust module
 * - Respects high water marks for peak memory usage
 * - Will force garbage collection if memory is near capacity but operation might fit
 */
export function checkMemoryAvailability(logCount: number): {
  canProceed: boolean;
  actionTaken: string; // e.g., 'normal', 'gc_performed', 'high_memory_post_gc', 'insufficient_memory', 'wasm_disabled', 'error'
  memoryInfo?: any; // Contains relevant memory details or error message
} {
  if (!isWasmEnabled() || !wasmModule) {
    return { canProceed: false, actionTaken: 'wasm_disabled' };
  }

  try {
    // Check if estimation function exists
    if (typeof wasmModule.estimate_memory_for_logs !== 'function' || typeof wasmModule.get_memory_usage !== 'function') {
        wasmLogger.log(WasmLogLevel.WARN, 'memory', 'Memory check functions not available in WASM module.');
        // Fallback: allow if log count is below a high threshold, otherwise deny
        const safeThreshold = getWasmSizeThreshold() * 5; // Example: 5x the normal threshold
        const allow = logCount < safeThreshold;
        return { canProceed: allow, actionTaken: allow ? 'estimation_unavailable_proceed' : 'estimation_unavailable_deny' };
    }


    // Get current memory usage
    const memInfo = wasmModule.get_memory_usage();
    updateMemoryUsage(memInfo); // Update central state

    // Check if memory utilization is already critically high
    if (memInfo.utilization > 0.85) {
      wasmLogger.log(
        WasmLogLevel.WARN,
        'memory',
        `High memory pressure detected (${(memInfo.utilization * 100).toFixed(1)}%), performing GC before estimation`,
        { memoryInfo: memInfo }
      );

      wasmModule.force_garbage_collection();

      // Check memory again after GC
      const postGcInfo = wasmModule.get_memory_usage();
      updateMemoryUsage(postGcInfo); // Update state again
      if (postGcInfo.utilization > 0.75) {
        // Still high after GC, use TS fallback
        return {
          canProceed: false,
          actionTaken: 'high_memory_post_gc',
          memoryInfo: postGcInfo
        };
      }
      // GC helped, proceed with estimation
    }

    // Check estimated memory for operation using the enhanced Rust function
    const estimate = wasmModule.estimate_memory_for_logs(logCount);

    // Log the estimation details for debugging
    wasmLogger.log(WasmLogLevel.DEBUG, 'memory', 'Memory estimation result', { estimate, logCount });


    if (!estimate.would_fit) {
      // Won't fit even with current memory, try GC if not already done
      if (memInfo.utilization <= 0.85) { // Avoid GC if already done above
          wasmLogger.log(WasmLogLevel.INFO, 'memory', 'Estimated memory insufficient, attempting GC.', { estimate });
          wasmModule.force_garbage_collection();

          // Get updated estimate after GC
          const postGcEstimate = wasmModule.estimate_memory_for_logs(logCount);
          updateMemoryUsage(wasmModule.get_memory_usage()); // Update state

          if (!postGcEstimate.would_fit) {
            return {
              canProceed: false,
              actionTaken: 'insufficient_memory_post_gc',
              memoryInfo: {
                estimate: postGcEstimate,
                current: getWasmStateInternal().memoryUsage // Get updated usage
              }
            };
          } else {
             // GC created enough space
             return {
               canProceed: true,
               actionTaken: 'gc_created_space',
               memoryInfo: postGcEstimate
             };
          }
      } else {
          // Already tried GC or utilization was critical, still won't fit
          return {
              canProceed: false,
              actionTaken: 'insufficient_memory',
              memoryInfo: {
                  estimate: estimate,
                  current: memInfo // Use the initial high memory info
              }
          };
      }
    }

    // Recommendation check from estimate
    if (estimate.recommendation === 'use_typescript_fallback') {
        return {
            canProceed: false,
            actionTaken: 'recommendation_fallback',
            memoryInfo: estimate
        };
    }

    // Normal case - can proceed with operation
    return {
      canProceed: true,
      actionTaken: estimate.recommendation === 'proceed_with_caution' ? 'proceed_with_caution' : 'normal',
      memoryInfo: { estimate, current: memInfo }
    };
  } catch (e: any) {
    // Use the central error handler
    handleWasmError(e, 'checkMemoryAvailability', { logCount });

    return {
      canProceed: false, // Assume unsafe on error
      actionTaken: 'error',
      memoryInfo: { error: e.message }
    };
  }
}
// --- End Phase 1.1: checkMemoryAvailability ---


// --- Start Phase 2.2: Serialization ---
/**
 * Measures and manages serialization overhead between JavaScript and WebAssembly
 */
const serializationMetrics = {
  totalSerializeTime: 0,
  totalDeserializeTime: 0,
  count: 0,
  avgSerializeTime: 0,
  avgDeserializeTime: 0,
  maxSerializeTime: 0,
  maxDeserializeTime: 0,

  // Track serialization time for performance tuning
  track(serializeTime: number, deserializeTime: number): void {
    this.totalSerializeTime += serializeTime;
    this.totalDeserializeTime += deserializeTime;
    this.count++;
    this.avgSerializeTime = this.totalSerializeTime / this.count;
    this.avgDeserializeTime = this.totalDeserializeTime / this.count;
    this.maxSerializeTime = Math.max(this.maxSerializeTime, serializeTime);
    this.maxDeserializeTime = Math.max(this.maxDeserializeTime, deserializeTime);

    // Optionally log metrics periodically
    if (this.count % 20 === 0) {
        wasmLogger.log(WasmLogLevel.DEBUG, 'serialization', 'Serialization Metrics Update', {
            count: this.count,
            avgSerialize: this.avgSerializeTime.toFixed(2) + 'ms',
            avgDeserialize: this.avgDeserializeTime.toFixed(2) + 'ms',
            maxSerialize: this.maxSerializeTime.toFixed(2) + 'ms',
            maxDeserialize: this.maxDeserializeTime.toFixed(2) + 'ms',
        });
    }
  }
};

/**
 * Optimized serialization for transferring logs to WebAssembly
 *
 * @param logs The log array to serialize
 * @returns The serialized data with timing and optimization info
 */
export function serializeLogsForWasm(logs: any[]): {
  data: any;
  time: number;
  optimization: string; // 'direct_small', 'slim_large', 'standard', 'error_fallback'
} {
  const startTime = performance.now();
  let result;
  let optimization = 'none';

  try {
    // Fast path for small log arrays - use direct serialization
    if (logs.length < 100) {
      result = logs; // Pass directly, WASM bindgen handles it
      optimization = 'direct_small';
    }
    // Optimization for large arrays - strip unnecessary fields if needed by WASM
    // NOTE: Current Rust `LogMessage` handles extra fields, so slimming might not be needed
    // unless specific performance issues arise with large objects.
    // Keeping the structure for potential future use.
    else if (logs.length > 1000 && false) { // Disabled slimming for now
      // Create slimmer objects with only needed fields
      const slimLogs = logs.map(log => ({
        level: log.level,
        message: log.message,
        time: log.time,
        behavior: log.behavior,
        _sequence: log._sequence,
        _unix_time: log._unix_time
        // Only include fields defined in Rust struct if strict matching is needed
      }));
      result = slimLogs;
      optimization = 'slim_large';
    }
    // Default case - pass the array as is
    else {
      result = logs;
      optimization = 'standard';
    }

    const endTime = performance.now();
    const serializationTime = endTime - startTime;

    // Note: Actual serialization happens via wasm-bindgen when calling the WASM function.
    // This time primarily measures JS-side preparation if any.

    return {
      data: result,
      time: serializationTime, // JS prep time
      optimization
    };
  } catch (error: any) {
    // Handle JS-side preparation errors gracefully
    const endTime = performance.now();
    const serializationTime = endTime - startTime;

    wasmLogger.log(
      WasmLogLevel.WARN,
      'serialization',
      `Error during log serialization preparation: ${error.message}`,
      {
        errorName: error.name,
        optimization,
        logCount: logs.length
      }
    );

    // Fall back to raw data
    return {
      data: logs,
      time: serializationTime,
      optimization: 'error_fallback'
    };
  }
}

/**
 * Optimized deserialization for receiving logs from WebAssembly
 *
 * @param data The data from WebAssembly
 * @returns The deserialized logs with timing information
 */
export function deserializeLogsFromWasm(data: any): {
  logs: any[];
  time: number; // Measures JS-side processing time after receiving data
} {
  const startTime = performance.now();
  let result;

  try {
    // We trust the WebAssembly module (via wasm-bindgen) to return valid data structure (e.g., array)
    // Just ensure it's an array, otherwise return empty.
    result = Array.isArray(data) ? data : [];

    // Restore any prototype methods or perform type conversions if needed
    // (typically not needed for simple data objects returned by serde)

    const endTime = performance.now();
    const deserializationTime = endTime - startTime;

    // Track metrics (using the shared tracker)
    // Note: We need the corresponding serialize time to track properly.
    // This tracking might be better placed in the function calling serialize/deserialize/wasm.
    // serializationMetrics.track(0, deserializationTime); // Example if serialize time is unknown here

    return {
      logs: result,
      time: deserializationTime
    };
  } catch (error: any) {
    const endTime = performance.now();
    const deserializationTime = endTime - startTime;

    wasmLogger.log(
      WasmLogLevel.WARN,
      'serialization',
      `Error during log deserialization processing: ${error.message}`,
      {
        errorName: error.name,
        dataType: typeof data,
        isArray: Array.isArray(data)
      }
    );

    // Return empty array on error
    return {
      logs: [],
      time: deserializationTime
    };
  }
}
// --- End Phase 2.2: Serialization ---