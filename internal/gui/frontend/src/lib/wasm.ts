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
  wasmState, // Import the state object itself for persistence
  updateState // Import the updateState function
} from './wasm-state';
import type { WasmState } from './wasm-state'; // Use type-only import for WasmState
import { settings, wasmActive } from './stores'; // Import wasmActive store
import { get } from 'svelte/store';

// --- Start Rate Limiters (Feedback Step 2) ---
let lastOperationDecisionLog = 0;  // Timestamp of last operation decision log
// lastMemoryCheckLog is already declared below (line ~178), removing duplicate
let lastPerformanceLog = 0;        // Timestamp of last performance log
let lastMaintenanceLog = 0;        // Timestamp of last maintenance log
let lastStateChangeLog = 0;        // Timestamp of last state change log

// Operation counters (for logging only specific percentages)
let operationCounter = 0;

// Only log 1 out of every X operations
function shouldLogOperation(): boolean {
  operationCounter++;

  // In development mode, log more frequently
  const isDevMode = (window as any).__LANGKIT_VERSION === 'dev';

  if (isDevMode) {
    return operationCounter % 50 === 0; // Log 2% of operations in dev mode
  } else {
    return operationCounter % 500 === 0; // Log 0.2% of operations in production
  }
}

// Only log operations after time interval
function shouldLogByInterval(lastLogTime: number, minInterval: number = 60000): [boolean, number] {
  const now = Date.now();
  const shouldLog = now - lastLogTime > minInterval;
  return [shouldLog, shouldLog ? now : lastLogTime];
}
// --- End Rate Limiters ---

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
  // Added in Improvement #4/5
  ensure_sufficient_memory?: (needed_bytes: number) => boolean;
}
// --- End Update WasmModule Interface ---

// --- State ---
let wasmModule: WasmModule | null = null;
let wasmInitialized = false;
let wasmEnabled = false;
let initializePromise: Promise<boolean> | null = null;
let wasmBuildInfo: WasmBuildInfo | null = null; // Added in Phase 4

// State tracking for reduced logging (Claude's suggestions)
let lastForceModeLog = 0;
let lastThresholdLog = 0;
let lastMemoryCheckLog = 0;

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
    WasmLogLevel.TRACE, // CHANGED FROM INFO
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
    WasmLogLevel.TRACE, // CHANGED FROM INFO
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
    WasmLogLevel.TRACE, // CHANGED FROM INFO
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

// Environment-aware path resolution for WebAssembly loading
export function getEnvironmentOptimizedPaths(
  basePath: string,
  cacheBuster: string,
  version: string = 'unknown'
): string[] {
  // Determine if we're in development mode
  const isDev = version === 'dev';
  
  if (isDev) {
    // Development environment - prioritize Wails protocol
    return [
      // Wails protocol path (most likely to succeed in development)
      `wails://wails.localhost:34115/wasm/${basePath}${cacheBuster}`,
      
      // Fallbacks for development
      `/wasm/${basePath}${cacheBuster}`,
      `${window.location.pathname}${basePath.startsWith('/') ? basePath.substring(1) : basePath}${cacheBuster}`,
      `${window.location.origin}/wasm/${basePath}${cacheBuster}`
    ];
  } else {
    // Production environment - prioritize standard web paths
    return [
      // Standard web paths (most likely to succeed in production)
      `/wasm/${basePath}${cacheBuster}`,
      `${window.location.origin}/wasm/${basePath}${cacheBuster}`,
      
      // Fallback paths
      `${window.location.pathname}${basePath.startsWith('/') ? basePath.substring(1) : basePath}${cacheBuster}`,
      `/public/wasm/${basePath}${cacheBuster}`
    ];
  }
}

// --- Start Phase 4.1: loadBuildInfo ---
/**
 * Load WebAssembly build information for versioning and cache management
 */
async function loadBuildInfo(version: string = 'unknown'): Promise<WasmBuildInfo | null> {
  try {
    const buildInfoPaths = getEnvironmentOptimizedPaths('build-info.json', `?t=${Date.now()}`, version);
    
    wasmLogger.log(
      WasmLogLevel.INFO,
      'init',
      `Attempting to load WebAssembly build info with paths`,
      { paths: buildInfoPaths, environment: version }
    );
    
    // Try each path until successful
    let response = null;
    let loadError = null;
    
    for (const path of buildInfoPaths) {
      try {
        wasmLogger.log(
          WasmLogLevel.DEBUG, // Use DEBUG level to reduce log spam
          'init',
          `Trying to load build info from: ${path}`
        );
        
        const fetchResponse = await fetch(path);
        if (fetchResponse.ok) {
          response = fetchResponse;
          wasmLogger.log(
            WasmLogLevel.INFO,
            'init',
            `Successfully loaded build info from: ${path}`
          );
          break; // Success, exit loop
        }
      } catch (err) {
        loadError = err;
      }
    }
    
    if (!response) {
      const errorMessage = loadError instanceof Error ? loadError.message : String(loadError);
      throw new Error(`Failed to fetch build info from any path: ${errorMessage}`);
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
      `Failed to load WebAssembly build info: ${error.message}`,
      {
        errorStack: error.stack,
        errorType: error.name,
        networkStatus: navigator.onLine ? 'online' : 'offline'
      }
    );
    return null;
  }
}
// --- End Phase 4.1: loadBuildInfo ---

// --- Start initializeWasm (Restored Version) ---
export async function initializeWasm(): Promise<boolean> {
  if (initializePromise) return initializePromise;

  let modulePaths: string[] = []; // Declare modulePaths here

  let currentWasmState = getWasmStateInternal();
  if (currentWasmState.initStatus === WasmInitStatus.SUCCESS) {
    return true; // Already initialized
  }

  // Update state to initializing
  wasmState.initStatus = WasmInitStatus.INITIALIZING;
  reportWasmState();

  initializePromise = new Promise<boolean>(async (resolve) => {
    // Define startTime here so it's accessible in catch/finally
    const startTime = performance.now();
    let initTime = 0; // Initialize initTime

    if (!wasmEnabled) {
      wasmState.initStatus = WasmInitStatus.NOT_STARTED;
      reportWasmState();
      resolve(false);
      return;
    }

    wasmLogger.log(WasmLogLevel.TRACE, 'init', 'Starting WebAssembly subsystem initialization'); // CHANGED FROM INFO

    try {
      if (!isWasmSupported()) {
        throw new WasmInitializationError("WebAssembly not supported in this browser", {
          runtime: "wails",
          timestamp: Date.now()
        });
      }

      // Get version from window if available
      const version = (window as any).__LANGKIT_VERSION || 'unknown';
      wasmLogger.log(
        WasmLogLevel.INFO,
        'init',
        `Initializing WebAssembly with environment: ${version}`
      );

      // Load build info first using environment-aware paths
      wasmBuildInfo = await loadBuildInfo(version);

      // Create cache buster
      const cacheBuster = wasmBuildInfo
        ? `?v=${wasmBuildInfo.version}&t=${wasmBuildInfo.timestamp}`
        : `?t=${Date.now()}`;

      // Assign to the outer scope variable
      modulePaths = getEnvironmentOptimizedPaths('log_engine.js', cacheBuster, version);

      // Enhanced logging
      wasmLogger.log(
        WasmLogLevel.INFO,
        'init',
        `Attempting to load WebAssembly module with environment: ${version}`,
        {
          paths: modulePaths,
          buildInfo: wasmBuildInfo || 'unavailable',
          documentBasePath: document.baseURI,
          locationHref: window.location.href
        }
      );

      // Try loading from each path until success
      let module = null;
      let loadError = null;

      for (const path of modulePaths) {
        try {
          wasmLogger.log(
            WasmLogLevel.TRACE, // CHANGED FROM INFO
            'init',
            `Trying to load WASM module from: ${path}`
          );

          module = await import(/* @vite-ignore */ path);

          wasmLogger.log(
            WasmLogLevel.TRACE, // CHANGED FROM INFO
            'init',
            `Successfully loaded WASM module from: ${path}`
          );

          break; // Success, exit loop
        } catch (err) {
          const errorMessage = err instanceof Error ? err.message : String(err);
          wasmLogger.log(
            WasmLogLevel.TRACE, // CHANGED FROM WARN
            'init',
            `Failed to load WASM module from: ${path}`,
            { error: errorMessage }
          );

          loadError = err;
          // Continue to next path
        }
      }

      if (!module) {
        const errorMessage = loadError instanceof Error ? loadError.message : String(loadError);
        throw new Error(`Failed to load WebAssembly module from any path: ${errorMessage}`);
      }

      // Initialize the module
      await module.default();

      // Set references and update state
      wasmModule = module;
      wasmInitialized = true;
      wasmState.initStatus = WasmInitStatus.SUCCESS;

      const endTime = performance.now();
      initTime = endTime - startTime; // Calculate initTime here
      wasmState.initTime = initTime;

      // Log success (KEEP AS INFO - significant state change)
      wasmLogger.log(
        WasmLogLevel.INFO,
        'init',
        'WebAssembly module initialized successfully',
        {
          initTime,
          wasmSize: getWasmSize(),
          version: wasmBuildInfo?.version || 'unknown',
          buildDate: wasmBuildInfo?.buildDate || 'unknown',
          environment: version
        }
      );

      // Perform initial setup
      scheduleMemoryCheck(); // Sets up GC and monitoring intervals
      loadSavedMetrics();

      // Initial threshold from settings
      const $settings = get(settings);
      if (($settings as any).wasmSizeThreshold) {
        setWasmSizeThreshold(($settings as any).wasmSizeThreshold);
      }

      // After successful initialization, pre-warm the module
      if (wasmInitialized) {
        // Run pre-warming after a short delay to allow UI rendering to complete
        setTimeout(() => {
          preWarmWebAssembly();
        }, 500);
      }

      resolve(true); // Resolve promise on success

    } catch (error: unknown) { // Catch as unknown for type safety
      const endTime = performance.now();
      // Calculate initTime even on error if startTime was set
      initTime = startTime ? endTime - startTime : 0;

      // Ensure error is an Error instance
      const errorInstance = error instanceof Error ? error : new Error(String(error));

      // Enhanced error logging for better debugging (KEEP AS ERROR)
      wasmLogger.log(
        WasmLogLevel.ERROR,
        'init',
        `WebAssembly module load error: ${errorInstance.message}`,
        {
          attemptedPaths: modulePaths, // Log all attempted paths
          errorStack: errorInstance.stack,
          errorType: errorInstance.name,
          networkStatus: navigator.onLine ? 'online' : 'offline',
          buildInfo: wasmBuildInfo || 'unavailable'
        }
      );

      // Use the enhanced error handler, passing attemptedPaths
      handleWasmError(errorInstance, 'initialization', {
        initTime,
        attemptedPaths: modulePaths, // Pass attempted paths
        buildInfo: wasmBuildInfo || 'unavailable'
      }, true); // Disable on critical init errors

      wasmInitialized = false;
      wasmState.initStatus = WasmInitStatus.FAILED;
      wasmState.initTime = initTime; // Record init time even on failure

      resolve(false); // Resolve promise on failure

    } finally {
      // Report final initialization status to backend for crash reports
      reportWasmState();
      initializePromise = null; // Reset promise state
    }
  });

  return initializePromise;
}
// --- End initializeWasm (Restored Version) ---




// --- Start Automatic Pre-Warming and Persistence ---

/**
 * Pre-warms the WebAssembly module to reduce runtime latency
 *
 * This function:
 * 1. Pre-allocates memory to avoid runtime memory growth
 * 2. Executes sample operations to JIT-compile critical functions
 * 3. Sets up scheduled maintenance to keep the module "warm"
 */
export function preWarmWebAssembly(): void {
  if (!isWasmEnabled() || !wasmModule) {
    wasmLogger.log(WasmLogLevel.INFO, 'init', 'WebAssembly pre-warming skipped: module not available');
    return;
  }

  wasmLogger.log(WasmLogLevel.TRACE, 'init', 'Starting WebAssembly pre-warming'); // CHANGED FROM INFO

  try {
    // Use local constant for type safety
    const currentWasmModule = wasmModule;

    // Step 1: Pre-allocate memory if supported
    if (typeof currentWasmModule.ensure_sufficient_memory === 'function') {
      // Start with a reasonable 8MB allocation
      // This prevents allocations during critical operations
      const success = currentWasmModule.ensure_sufficient_memory(8 * 1024 * 1024);
      wasmLogger.log(
        WasmLogLevel.INFO,
        'memory',
        `WebAssembly pre-allocation ${success ? 'successful' : 'failed'}`,
        { requestedBytes: 8 * 1024 * 1024 }
      );
    }

    // Step 2: Create sample data for warm-up operations
    // These operations trigger JIT compilation of critical code paths

    // Small array warm-up (sorted arrays, common case)
    const smallArrayA = Array(10).fill(0).map((_, i) => ({
      level: 'INFO',
      message: `Pre-warm ${i}`,
      time: new Date().toISOString(),
      _unix_time: Date.now() + i,
      _sequence: i
    }));

    const smallArrayB = Array(10).fill(0).map((_, i) => ({
      level: 'INFO',
      message: `Pre-warm ${i+10}`,
      time: new Date().toISOString(),
      _unix_time: Date.now() + i + 10,
      _sequence: i + 10
    }));

    // Run basic operations to warm up function paths
    const startTime = performance.now();

    // Operation 1: Standard insert
    currentWasmModule.merge_insert_logs(smallArrayA, smallArrayB);

    // Operation 2: Overlap case (tests sorting logic)
    const overlap1 = smallArrayA.slice(0, 5);
    const overlap2 = smallArrayB.slice(0, 5);
    currentWasmModule.merge_insert_logs(overlap1, overlap2);

    // Operation 3: Empty array handling
    currentWasmModule.merge_insert_logs([], smallArrayA);
    currentWasmModule.merge_insert_logs(smallArrayB, []);

    const warmupTime = performance.now() - startTime;

    wasmLogger.log(
      WasmLogLevel.TRACE, // CHANGED FROM INFO
      'init',
      `WebAssembly module pre-warmed successfully in ${warmupTime.toFixed(2)}ms`,
      {
        warmupTime,
        // Ensure get_memory_usage exists before calling
        memoryUsage: typeof currentWasmModule.get_memory_usage === 'function'
          ? currentWasmModule.get_memory_usage()
          : 'unavailable'
      }
    );

    // Step 3: Set up scheduled maintenance to keep the module warm
    setupMaintenanceInterval();

  } catch (error: any) {
    wasmLogger.log(
      WasmLogLevel.ERROR,
      'init',
      `WebAssembly pre-warming failed: ${error.message}`,
      { errorStack: error.stack }
    );
    // Use handleWasmError for consistency
    handleWasmError(error, 'preWarmWebAssembly');
  }
}

/**
 * Sets up periodic maintenance to keep the WebAssembly module "warm"
 *
 * This prevents the JavaScript engine from de-optimizing rarely used code paths
 * and helps maintain consistent performance.
 */
function setupMaintenanceInterval(): void {
  // Clear existing interval if any
  if (wasmState.maintenanceIntervalId) {
    clearInterval(wasmState.maintenanceIntervalId);
    // wasmState.maintenanceIntervalId = undefined; // Use updateState below
  }

  // Create a maintenance interval (every 5 minutes)
  const MAINTENANCE_INTERVAL = 5 * 60 * 1000; // 5 minutes

  const intervalId = setInterval(() => {
    // Use local constant for type safety
    const currentWasmModule = wasmModule;

    if (!isWasmEnabled() || !currentWasmModule) {
      // Clean up if module becomes unavailable
      if (wasmState.maintenanceIntervalId) {
        clearInterval(wasmState.maintenanceIntervalId);
        updateState({ maintenanceIntervalId: undefined });
      }
      return;
    }

    try {
      // Check if module was recently used (within last 10 minutes)
      const state = getWasmStateInternal(); // Use internal getter
      const now = Date.now();
      const lastUsed = state.lastUsed || 0;

      // Only run maintenance if the module hasn't been used recently
      if (now - lastUsed > 10 * 60 * 1000) {
        wasmLogger.log(WasmLogLevel.TRACE, 'maintenance', 'Performing WebAssembly maintenance'); // CHANGED FROM DEBUG

        // Run a simple operation to keep code paths warm
        const testArray = Array(5).fill(0).map((_, i) => ({
          level: 'INFO',
          message: `Maintenance ${i}`,
          time: new Date().toISOString(),
          _unix_time: now + i,
          _sequence: i
        }));

        // Ensure functions exist before calling
        if (typeof currentWasmModule.merge_insert_logs === 'function') {
            currentWasmModule.merge_insert_logs([], testArray);
        }

        // Check memory and perform garbage collection if utilization is high
        if (typeof currentWasmModule.get_memory_usage === 'function' &&
            typeof currentWasmModule.force_garbage_collection === 'function') {
            const memInfo = currentWasmModule.get_memory_usage();
            if (memInfo.utilization > 0.7) {
              wasmLogger.log(WasmLogLevel.INFO, 'maintenance', 'Performing maintenance garbage collection');
              currentWasmModule.force_garbage_collection();
            }
        }
      }

      // Add threshold adjustment during maintenance
      try {
        // Adjust thresholds based on performance data
        adjustSizeThresholds();
      } catch (error: any) {
        wasmLogger.log(
          WasmLogLevel.WARN,
          'adaptive',
          `Threshold adjustment failed: ${error.message}`
        );
      }

    } catch (error: any) {
      wasmLogger.log(
        WasmLogLevel.WARN,
        'maintenance',
        `WebAssembly maintenance failed: ${error.message}`
      );
      // Don't stop the interval on maintenance error, just log it
    }
  }, MAINTENANCE_INTERVAL);

  // Store interval ID using updateState
  updateState({ maintenanceIntervalId: intervalId as unknown as number }); // Store interval ID

  // Ensure cleanup on page unload
  window.addEventListener('beforeunload', () => {
    if (wasmState.maintenanceIntervalId) {
      clearInterval(wasmState.maintenanceIntervalId);
      // No need to update state here as the page is unloading
    }
  });
}

// --- End Automatic Pre-Warming and Persistence ---


// --- Start Adaptive Size Threshold Learning ---

/**
 * Adaptive learning system for WebAssembly size thresholds
 *
 * This system analyzes performance data to determine the optimal
 * threshold for switching between TypeScript and WebAssembly based
 * on actual measured performance in the current environment.
 */

// Configuration for adaptive learning
const ADAPTIVE_LEARNING_CONFIG = {
  MIN_OPERATIONS_REQUIRED: 20,     // Minimum operations before adapting
  ADJUSTMENT_INTERVAL: 10 * 60000, // 10 minutes between adjustments
  MAX_ADJUSTMENT_FACTOR: 0.2,      // Max 20% change per adjustment
  PERFORMANCE_MARGIN: 1.1,         // Require 10% margin over threshold
};

// Track when we last adjusted thresholds
let lastThresholdAdjustment = 0;

/**
 * Analyzes performance metrics and adjusts size thresholds accordingly
 *
 * This function:
 * 1. Examines performance data across different log sizes
 * 2. Determines if thresholds should be adjusted
 * 3. Makes incremental adjustments to optimize performance
 *
 * @returns {boolean} True if an adjustment was made
 */
export function adjustSizeThresholds(): boolean {
  const now = Date.now();

  // Don't adjust too frequently
  if (now - lastThresholdAdjustment < ADAPTIVE_LEARNING_CONFIG.ADJUSTMENT_INTERVAL) {
    return false;
  }

  // Get current state and metrics
  const state = getWasmStateInternal(); // Use internal getter

  // Need enough data for meaningful analysis
  if (state.performanceMetrics.operationsCount < ADAPTIVE_LEARNING_CONFIG.MIN_OPERATIONS_REQUIRED) {
    wasmLogger.log(
      WasmLogLevel.DEBUG,
      'adaptive',
      `Not enough operations for threshold learning (${state.performanceMetrics.operationsCount}/${ADAPTIVE_LEARNING_CONFIG.MIN_OPERATIONS_REQUIRED})`
    );
    return false;
  }

  // Extract metrics by log size
  const metrics = state.performanceMetrics;
  const currentThreshold = getWasmSizeThreshold(); // Use global threshold for now
  let newThreshold = currentThreshold;
  let adjustmentReason = '';

  // Initialize threshold analysis stats
  const analysis = {
    smallLogsCount: metrics.logSizeDistribution.small || 0,
    mediumLogsCount: metrics.logSizeDistribution.medium || 0,
    largeLogsCount: metrics.logSizeDistribution.large || 0,
    avgWasmTime: metrics.avgWasmTime,
    avgTsTime: metrics.avgTsTime,
    speedupRatio: metrics.speedupRatio,
    netSpeedupRatio: metrics.netSpeedupRatio
  };

  // Decision logic for threshold adjustment

  // Case 1: WebAssembly performs well on small datasets - lower threshold
  if (analysis.smallLogsCount >= 5 &&
      analysis.netSpeedupRatio > WASM_CONFIG.MIN_PERFORMANCE_GAIN * ADAPTIVE_LEARNING_CONFIG.PERFORMANCE_MARGIN) {

    // WebAssembly is very beneficial for small logs, lower threshold
    const adjustmentFactor = Math.min(
      ADAPTIVE_LEARNING_CONFIG.MAX_ADJUSTMENT_FACTOR,
      (analysis.netSpeedupRatio - WASM_CONFIG.MIN_PERFORMANCE_GAIN) / 10 // Smaller steps down
    );

    const proposedThreshold = Math.max(
      WASM_CONFIG.MIN_THRESHOLD,
      Math.floor(currentThreshold * (1 - adjustmentFactor))
    );

    if (proposedThreshold < currentThreshold) {
      newThreshold = proposedThreshold;
      adjustmentReason = 'small_logs_high_performance';
    }
  }
  // Case 2: WebAssembly doesn't perform well enough on medium-sized logs - raise threshold
  else if (analysis.mediumLogsCount >= 5 &&
           analysis.netSpeedupRatio < WASM_CONFIG.MIN_PERFORMANCE_GAIN) {

    // WebAssembly isn't beneficial enough, raise threshold
    const adjustmentFactor = Math.min(
      ADAPTIVE_LEARNING_CONFIG.MAX_ADJUSTMENT_FACTOR,
      (WASM_CONFIG.MIN_PERFORMANCE_GAIN - analysis.netSpeedupRatio) / 2 // Larger steps up if perf is bad
    );

    const proposedThreshold = Math.min(
      WASM_CONFIG.MAX_THRESHOLD,
      Math.ceil(currentThreshold * (1 + adjustmentFactor))
    );

    if (proposedThreshold > currentThreshold) {
      newThreshold = proposedThreshold;
      adjustmentReason = 'medium_logs_low_performance';
    }
  }
  // Case 3: Mixed data suggests an optimum around current threshold +/- small adjustment
  else if (analysis.smallLogsCount >= 5 && analysis.mediumLogsCount >= 5) {
    // If we have both small and medium logs, find the sweet spot

    // Get operation-specific timing data if available (assuming 'mergeInsertLogs' is primary)
    const opTimings = metrics.operationTimings?.['mergeInsertLogs'];

    if (opTimings && opTimings.count > 5) {
      // Fine-tune based on operation-specific metrics
      if (opTimings.avgTime < metrics.avgWasmTime * 0.9) { // If this op is >10% faster than avg
        // This operation is faster than average, maybe lower threshold slightly
        newThreshold = Math.max(
          WASM_CONFIG.MIN_THRESHOLD,
          Math.floor(currentThreshold * 0.95) // 5% decrease
        );
        adjustmentReason = 'operation_better_than_average';
      } else if (opTimings.avgTime > metrics.avgWasmTime * 1.1) { // If this op is >10% slower than avg
        // This operation is slower than average, maybe raise threshold slightly
        newThreshold = Math.min(
          WASM_CONFIG.MAX_THRESHOLD,
          Math.ceil(currentThreshold * 1.05) // 5% increase
        );
        adjustmentReason = 'operation_worse_than_average';
      }
    }
  }

  // Apply the threshold change if needed
  if (newThreshold !== currentThreshold) {
    setWasmSizeThreshold(newThreshold); // Apply the new global threshold
    lastThresholdAdjustment = now;

    // Track adjustment for diagnostic purposes using updateState
    const currentAdjustments = wasmState.thresholdAdjustments || [];
    const newAdjustment = {
        timestamp: now,
        previousThreshold: currentThreshold,
        newThreshold: newThreshold,
        reason: adjustmentReason,
        metrics: { // Capture metrics at time of adjustment
          smallLogs: analysis.smallLogsCount,
          mediumLogs: analysis.mediumLogsCount,
          largeLogs: analysis.largeLogsCount,
          speedupRatio: analysis.speedupRatio,
          netSpeedupRatio: analysis.netSpeedupRatio
        }
      };

    // Ensure thresholdAdjustments exists on wasmState before pushing
    const updatedAdjustments = currentAdjustments.length >= 10
        ? [...currentAdjustments.slice(1), newAdjustment]
        : [...currentAdjustments, newAdjustment];

    updateState({ thresholdAdjustments: updatedAdjustments });

    wasmLogger.log(
      WasmLogLevel.TRACE, // CHANGED FROM INFO
      'adaptive',
      `Adjusted WebAssembly threshold from ${currentThreshold} to ${newThreshold}`,
      {
        reason: adjustmentReason,
        analysis,
        changePercent: Math.round((newThreshold - currentThreshold) / currentThreshold * 100)
      }
    );

    // reportWasmState() is called within updateState if significant
    return true;
  }

  // No adjustment needed
  wasmLogger.log(
    WasmLogLevel.TRACE, // CHANGED FROM DEBUG
    'adaptive',
    `No threshold adjustment needed (current: ${currentThreshold})`,
    { analysis }
  );

  return false;
}

// --- End Adaptive Size Threshold Learning ---

// Check memory thresholds and issue warnings when needed
function checkMemoryThresholds(): void {
  if (!isWasmEnabled() || !wasmModule) return;

  try {
    const memoryInfo = wasmModule.get_memory_usage();

    // Define warning thresholds
    if (memoryInfo.utilization > 0.85) {
      wasmLogger.log(
        WasmLogLevel.WARN, // KEEP AS WARN - serious issue
        'memory',
        `Critical memory pressure: ${(memoryInfo.utilization * 100).toFixed(1)}% used`,
        { memoryInfo }
      );
      // Force garbage collection at critical levels
      wasmModule.force_garbage_collection();
    } else if (memoryInfo.utilization > 0.7) {
      wasmLogger.log(
        WasmLogLevel.TRACE, // CHANGED FROM WARN to TRACE - High but not critical is routine
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

        // Log memory info periodically (CHANGED FROM DEBUG to TRACE)
        wasmLogger.log(
          WasmLogLevel.TRACE,
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

// --- Start Smart Blacklist Recovery ---

// Enhanced blacklist interface
interface BlacklistEntry {
  operation: string;        // Name of the blacklisted operation
  timestamp: number;        // When the operation was blacklisted
  retryCount: number;       // Number of consecutive failures
  nextRetryTime: number;    // When to try again
  lastError?: string;       // Last error message
  backoffMs: number;        // Current backoff duration
}

// Replace the simple Set with a Map for richer data
const operationBlacklist: Map<string, BlacklistEntry> = new Map();
// operationErrorCounts is no longer needed as retryCount is tracked in BlacklistEntry

/**
 * Checks if an operation is currently blacklisted
 *
 * This function also handles retry logic with exponential backoff:
 * - If it's time to retry, removes the operation from blacklist temporarily
 * - Otherwise, returns the blacklist status
 */
export function isOperationBlacklisted(operation: string): boolean {
  const entry = operationBlacklist.get(operation);
  if (!entry) return false;

  const now = Date.now();

  // Check if it's time to retry the operation
  if (now > entry.nextRetryTime) {
    // Temporarily remove from blacklist to allow one operation attempt
    // It will be re-added by handleWasmError if it fails again
    operationBlacklist.delete(operation);
    updateBlacklistState(); // Update state for dashboard

    wasmLogger.log(
      WasmLogLevel.TRACE, // CHANGED FROM INFO
      'recovery',
      `Attempting retry for blacklisted operation "${operation}" after ${entry.retryCount} failures`,
      {
        retryCount: entry.retryCount,
        blacklistedForMs: now - entry.timestamp,
        lastError: entry.lastError
      }
    );

    // Allow the operation to proceed for this attempt
    return false;
  }

  // Still blacklisted - REMOVE or make extremely infrequent (Feedback Step 4)
  // wasmLogger.log(
  //   WasmLogLevel.DEBUG,
  //   'blacklist',
  //   `Operation "${operation}" is blacklisted for ${Math.round((entry.nextRetryTime - now) / 1000)}s more`,
  //   {
  //     retryCount: entry.retryCount,
  //     timeRemainingMs: entry.nextRetryTime - now,
  //     blacklistedSince: entry.timestamp
  //   }
  // );

  return true;
}

/**
 * Adds or updates an operation in the blacklist with exponential backoff
 *
 * The backoff time increases with each consecutive failure:
 * - 1st failure: 5 seconds
 * - 2nd failure: 15 seconds
 * - 3rd failure: 45 seconds
 * - etc. (capped at 30 minutes)
 */
export function addToOperationBlacklist(operation: string, error?: Error): void {
  const now = Date.now();
  // Get retry count from existing entry or start at 1
  const existing = operationBlacklist.get(operation);
  const retryCount = existing ? existing.retryCount + 1 : 1;

  // Calculate backoff with exponential increase (5s * 3^(retryCount-1))
  // Cap at 30 minutes to prevent excessive delays
  const baseBackoff = 5000; // 5 seconds base
  const maxBackoff = 30 * 60 * 1000; // 30 minutes maximum

  const backoffMs = Math.min(
    baseBackoff * Math.pow(3, retryCount - 1),
    maxBackoff
  );

  // Create or update the blacklist entry
  const entry: BlacklistEntry = {
    operation,
    timestamp: now, // Record the time of this specific failure/blacklisting event
    retryCount,
    nextRetryTime: now + backoffMs,
    lastError: error?.message,
    backoffMs
  };

  operationBlacklist.set(operation, entry);

  // Log the blacklisting with appropriate level based on retry count (KEEP AS IS - WARN/ERROR)
  const logLevel = retryCount >= 3 ? WasmLogLevel.ERROR : WasmLogLevel.WARN;

  wasmLogger.log(
    logLevel,
    'recovery',
    `Blacklisting operation "${operation}" for ${Math.round(backoffMs/1000)}s (failure #${retryCount})`,
    {
      retryCount,
      backoffDurationMs: backoffMs,
      nextRetryTime: entry.nextRetryTime,
      error: error?.message || 'Unknown error'
    }
  );

  // Update WASM state to track blacklist entries for dashboard
  updateBlacklistState();
}

/**
 * Clears the blacklist entry for a successful operation.
 * Called when an operation that might have been blacklisted succeeds.
 */
export function clearOperationErrorCount(operation: string): void {
  if (operationBlacklist.has(operation)) {
    wasmLogger.log(
      WasmLogLevel.TRACE, // CHANGED FROM INFO
      'recovery',
      `Operation "${operation}" succeeded, removing from blacklist.`
    );
    operationBlacklist.delete(operation);
    updateBlacklistState();
  }
  // No need to manage operationErrorCounts map anymore
}

/**
 * Updates the WASM state with current blacklist information.
 * This makes blacklisted operations visible in the dashboard.
 */
function updateBlacklistState(): void {
  // Convert Map values to array for state storage
  const blacklistedArray = Array.from(operationBlacklist.values());

  // Use updateState for immutable update (assuming updateState exists and handles nested objects)
  // This requires 'blacklistedOperations' to be added to WasmState interface
  updateState({ blacklistedOperations: blacklistedArray });

  // reportWasmState() is called within updateState if significant change detected
}

// Helper function to check if an error is related to initialization (used in handleWasmError)
function isWasmInitializationError(error: Error): boolean {
    return error instanceof WasmInitializationError ||
           error instanceof WebAssembly.RuntimeError ||
           error instanceof WebAssembly.LinkError ||
           error instanceof WebAssembly.CompileError;
}

// --- End Smart Blacklist Recovery ---


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

  // Execution errors: Add to blacklist (which handles counting and backoff)
  if (errorType.category === 'execution' || errorType.category === 'unknown') {
      return () => {
          addToOperationBlacklist(operation, error);
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
  error: unknown, // Change type to unknown for better type safety
  operation: string,
  context: Record<string, any> = {},
  disableOnCritical: boolean = false
): void {
  // Ensure error is an instance of Error for consistent handling
  const errorInstance = error instanceof Error ? error : new Error(String(error));
  // Categorize the error using the guaranteed Error instance
  const errorType = categorizeWasmError(errorInstance);

  // Get appropriate recovery strategy using the guaranteed Error instance
  const recoveryAction = getRecoveryStrategy(errorInstance, errorType, operation);

  // Determine log level
  const logLevel = errorType.severity === 'high' ? WasmLogLevel.ERROR : WasmLogLevel.WARN;

  // Log with essential context
  wasmLogger.log(
    logLevel,
    'error',
    `WebAssembly ${operation} failed: ${errorInstance.message}`,
    {
      ...context, // Spread original context first
      errorName: errorInstance.name,
      operation,
      category: errorType.category,
      severity: errorType.severity,
      // Optionally include stack trace for high severity errors
      errorStack: errorType.severity === 'high' ? errorInstance.stack : undefined,
      // Include attemptedPaths if available in context
      attemptedPaths: context.attemptedPaths
    }
  );

  // Update error state using the guaranteed Error instance
  setWasmError(errorInstance);

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
    // Only log force mode once per minute, not per operation
    const now = Date.now();
    if (now - lastForceModeLog > 60000) {
      wasmLogger.log(
        WasmLogLevel.TRACE, // Changed from INFO to TRACE
        'threshold',
        `WebAssembly force enabled in settings, will use for all operations when available`
      );
      lastForceModeLog = now;
    }

    // Check if WASM is available and not blacklisted
    if (!isWasmEnabled() || isOperationBlacklisted(operation)) {
      // Don't log this routine check that happens constantly (Feedback Step 3)
      // wasmLogger.log(
      //   WasmLogLevel.TRACE,
      //   'threshold',
      //   `WebAssembly forced enabled but unavailable or blacklisted for operation: ${operation}`
      // );
      // REMOVED STRAY PARENTHESIS -> );
      return false;
    }
    return true; // Force-enable if available
  } else if ($settings.forceWasmMode === 'disabled') {
    // Log force disable only once per minute
    const now = Date.now();
     if (now - lastForceModeLog > 60000) {
        wasmLogger.log(
            WasmLogLevel.TRACE, // Use TRACE
            'threshold',
            `WebAssembly force disabled in settings, using TypeScript.`
        );
        lastForceModeLog = now;
     }
    return false; // Force-disable regardless of other factors
  }

  // If 'auto' mode, use the existing sophisticated logic
  // No changes to the current implementation from here on

  // Basic checks first
  if (!isWasmEnabled() || isOperationBlacklisted(operation)) {
    if (isOperationBlacklisted(operation)) {
        // Log blacklist skips only infrequently (using interval limiter)
        let shouldLogBlacklist;
        [shouldLogBlacklist, lastOperationDecisionLog] = shouldLogByInterval(lastOperationDecisionLog);
        if (shouldLogBlacklist) {
            wasmLogger.log(WasmLogLevel.TRACE, 'threshold', `Skipping WASM for blacklisted operation: ${operation}`); // CHANGED FROM INFO to TRACE
        }
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
      // Log memory constraint fallbacks only infrequently (using interval limiter)
      let shouldLogMemConstraint;
      [shouldLogMemConstraint, lastOperationDecisionLog] = shouldLogByInterval(lastOperationDecisionLog);
      if (shouldLogMemConstraint) {
          wasmLogger.log(
              WasmLogLevel.TRACE, // CHANGED FROM DEBUG to TRACE
              'threshold',
              `Using TypeScript fallback due to memory constraints: ${memCheck.actionTaken}`,
              { memoryInfo: memCheck.memoryInfo, operation, logCount: totalLogCount }
          );
      }
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

  // Only log decision for 0.2% of operations or once per minute for large datasets (Feedback Step 3)
  let shouldLogDecision = false;

  if (totalLogCount > 5000) { // Check for large datasets first
    [shouldLogDecision, lastOperationDecisionLog] = shouldLogByInterval(lastOperationDecisionLog);
  } else { // Otherwise use percentage-based logging
    shouldLogDecision = shouldLogOperation();
  }

  if (shouldLogDecision) {
    wasmLogger.log(
      WasmLogLevel.TRACE, // ALWAYS TRACE for routine decisions
      'threshold',
      `WebAssembly decision for ${operation} with ${totalLogCount} logs: ${useWasm ? 'Use WASM' : 'Use TypeScript'}`,
      // include metrics...
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
// Add explicit return type to satisfy compiler, even though all paths should return
export function checkMemoryAvailability(logCount: number): {
  canProceed: boolean;
  actionTaken: string;
  memoryInfo?: any;
} { // Ensure function always returns this type
  if (!isWasmEnabled() || !wasmModule) {
    return { canProceed: false, actionTaken: 'wasm_disabled' };
  }

  // Define checkRecord structure for type safety
  type MemoryCheckRecord = {
      timestamp: number;
      logCount: number;
      initialUtilization: number;
      actions: string[];
      outcome: string;
      finalUtilization: number;
      error?: string;
  };
  let checkRecordInstance: MemoryCheckRecord | null = null;

  try {
    // Use a local constant for wasmModule after the initial check for type safety within the block
    const currentWasmModule = wasmModule;

    // Check if necessary functions exist on the confirmed module instance
    if (typeof currentWasmModule.estimate_memory_for_logs !== 'function' ||
        typeof currentWasmModule.get_memory_usage !== 'function') {
      wasmLogger.log(WasmLogLevel.WARN, 'memory', 'Memory check functions not available in WASM module.');
      const safeThreshold = getWasmSizeThreshold() * 5;
      const allow = logCount < safeThreshold;
      return { canProceed: allow, actionTaken: allow ? 'estimation_unavailable_proceed' : 'estimation_unavailable_deny' };
    }

    // Get current memory usage
    const memInfo = currentWasmModule.get_memory_usage();
    updateMemoryUsage(memInfo);

    // --- Start Memory Check Record Tracking ---
    const state = getWasmStateInternal();
    if (!state.memoryChecks) {
      state.memoryChecks = []; // Initialize if it doesn't exist (will be typed correctly in Change #6)
    }
    if (state.memoryChecks.length >= 20) {
      state.memoryChecks.shift();
    }
    checkRecordInstance = {
      timestamp: Date.now(),
      logCount,
      initialUtilization: memInfo.utilization,
      actions: [],
      outcome: 'pending',
      finalUtilization: memInfo.utilization,
    };
    // Push the instance, assuming state.memoryChecks is now an array
    (state.memoryChecks as MemoryCheckRecord[]).push(checkRecordInstance);

    // Only log memory usage on interval and only for large operations (Feedback Step 3)
    let shouldLogMemory = false;
    if (logCount > 1000) {
      [shouldLogMemory, lastMemoryCheckLog] = shouldLogByInterval(lastMemoryCheckLog);
    }

    if (shouldLogMemory) {
      wasmLogger.log(
        WasmLogLevel.TRACE, // Always TRACE for routine checks
        'memory',
        `Memory check for ${logCount} logs: ${Math.round(memInfo.utilization * 100)}% utilized`,
        { memoryInfo: memInfo } // Use memInfo directly
      );
    }
    // --- End Memory Check Record Tracking ---

    // --- High Memory Check + GC ---
    // Use level based on urgency (Feedback Step 3)
    if (memInfo.utilization > 0.9) {
      checkRecordInstance.actions.push('gc_attempted');
      wasmLogger.log(WasmLogLevel.WARN, 'memory', `Critical memory pressure (${(memInfo.utilization * 100).toFixed(1)}%), performing GC`, { memoryInfo: memInfo }); // High memory pressure is WARN
      currentWasmModule.force_garbage_collection();
      const postGcInfo = currentWasmModule.get_memory_usage();
      updateMemoryUsage(postGcInfo);
      checkRecordInstance.finalUtilization = postGcInfo.utilization; // Update final utilization early

      if (postGcInfo.utilization > 0.9) { // Still critical after GC
        // --- Memory Growth Attempt ---
        if (typeof currentWasmModule.ensure_sufficient_memory === 'function') {
          checkRecordInstance.actions.push('memory_growth_attempted');
          const estimatedBytesNeeded = logCount * 400; // Example estimation
          wasmLogger.log(WasmLogLevel.INFO, 'memory', `Attempting memory growth for ${logCount} logs`, { estimatedBytesNeeded }); // Keep INFO for growth attempt
          const growthSuccess = currentWasmModule.ensure_sufficient_memory(estimatedBytesNeeded);

          if (growthSuccess) {
            const postGrowthInfo = currentWasmModule.get_memory_usage();
            updateMemoryUsage(postGrowthInfo);
            checkRecordInstance.outcome = 'growth_succeeded';
            checkRecordInstance.finalUtilization = postGrowthInfo.utilization;
            wasmLogger.log(WasmLogLevel.INFO, 'memory', `Memory growth successful`, { memoryInfo: postGrowthInfo }); // Keep INFO for success
            return { canProceed: true, actionTaken: 'memory_growth_successful', memoryInfo: postGrowthInfo };
          } else {
            checkRecordInstance.outcome = 'growth_failed';
            wasmLogger.log(WasmLogLevel.WARN, 'memory', `Memory growth failed, using TS fallback`, { logCount }); // Keep WARN for failure
            return { canProceed: false, actionTaken: 'memory_growth_failed', memoryInfo: postGcInfo };
          }
        } else {
          checkRecordInstance.outcome = 'high_memory_no_growth_support';
          return { canProceed: false, actionTaken: 'high_memory_post_gc_no_growth_support', memoryInfo: postGcInfo };
        }
        // --- End Memory Growth Attempt ---
      } else { // GC brought utilization below critical
        checkRecordInstance.outcome = 'gc_succeeded_high_pressure'; // GC helped below critical
        // Log elevated pressure if still high and logging is enabled
        if (postGcInfo.utilization > 0.75 && shouldLogMemory) {
           wasmLogger.log(WasmLogLevel.TRACE, 'memory', `Elevated memory pressure after GC (${(postGcInfo.utilization * 100).toFixed(1)}%)`, { memoryInfo: postGcInfo });
        }
        // Proceed to estimation
      }
    }
    // --- End High Memory Check + GC ---

    // --- Memory Estimation Check ---
    const estimate = currentWasmModule.estimate_memory_for_logs(logCount);
    // Log memory estimation result (CHANGED FROM DEBUG to TRACE)
    wasmLogger.log(WasmLogLevel.TRACE, 'memory', 'Memory estimation result', { estimate, logCount });

    if (!estimate.would_fit) {
      // Try GC only if not already done due to high pressure
      if (!checkRecordInstance.actions.includes('gc_attempted')) {
        checkRecordInstance.actions.push('gc_attempted_on_estimate_fail');
        wasmLogger.log(WasmLogLevel.WARN, 'memory', 'Estimated memory insufficient, attempting GC.', { estimate }); // KEEP AS WARN - potential issue
        currentWasmModule.force_garbage_collection();
        const postGcEstimate = currentWasmModule.estimate_memory_for_logs(logCount);
        const postGcInfo = currentWasmModule.get_memory_usage();
        updateMemoryUsage(postGcInfo);
        checkRecordInstance.finalUtilization = postGcInfo.utilization; // Update final utilization

        if (!postGcEstimate.would_fit) {
          checkRecordInstance.outcome = 'insufficient_memory_post_gc';
          return { canProceed: false, actionTaken: 'insufficient_memory_post_gc', memoryInfo: { estimate: postGcEstimate, current: postGcInfo } };
        } else {
          checkRecordInstance.outcome = 'gc_created_space';
          return { canProceed: true, actionTaken: 'gc_created_space', memoryInfo: postGcEstimate };
        }
      } else {
        // Already tried GC, still won't fit
        checkRecordInstance.outcome = 'insufficient_memory_after_high_pressure_gc';
        // Use the latest known utilization before this check
        checkRecordInstance.finalUtilization = state.memoryUsage?.utilization || memInfo.utilization;
        return { canProceed: false, actionTaken: 'insufficient_memory', memoryInfo: { estimate: estimate, current: state.memoryUsage || memInfo } };
      }
    }
    // --- End Memory Estimation Check ---

    // --- Recommendation Check ---
    if (estimate.recommendation === 'use_typescript_fallback') {
      checkRecordInstance.outcome = 'recommendation_fallback';
      // Use the latest known utilization before this check
      checkRecordInstance.finalUtilization = state.memoryUsage?.utilization || memInfo.utilization;
      return { canProceed: false, actionTaken: 'recommendation_fallback', memoryInfo: estimate };
    }
    // --- End Recommendation Check ---

    // --- Normal Proceed ---
    checkRecordInstance.outcome = 'normal_operation';
    // Use the latest known utilization before this check
    checkRecordInstance.finalUtilization = state.memoryUsage?.utilization || memInfo.utilization;
    return {
      canProceed: true,
      actionTaken: estimate.recommendation === 'proceed_with_caution' ? 'proceed_with_caution' : 'normal',
      memoryInfo: { estimate, current: memInfo }
    };
    // --- End Normal Proceed ---

  } catch (e: unknown) {
    const errorInstance = e instanceof Error ? e : new Error(String(e));
    // Update the check record if it was created
    if (checkRecordInstance) {
        checkRecordInstance.outcome = 'error';
        checkRecordInstance.error = errorInstance.message;
        // Try to get final utilization, default to initial if error occurs
        try {
            // Use the local constant which is guaranteed non-null here if initial check passed
            if (wasmModule && typeof wasmModule.get_memory_usage === 'function') {
                 checkRecordInstance.finalUtilization = wasmModule.get_memory_usage().utilization;
            } else {
                 checkRecordInstance.finalUtilization = checkRecordInstance.initialUtilization;
            }
        } catch (_) {
             checkRecordInstance.finalUtilization = checkRecordInstance.initialUtilization;
        }
    }

    // Use the central error handler (logCount is in scope)
    handleWasmError(errorInstance, 'checkMemoryAvailability', { logCount });

    // Ensure catch block returns the declared type
    return {
      canProceed: false,
      actionTaken: 'error',
      memoryInfo: { error: errorInstance.message }
    };
  }
} // End of checkMemoryAvailability function
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

    // REMOVE or make extremely infrequent (Feedback Step 4)
    // if (this.count % 20 === 0) {
    //     wasmLogger.log(WasmLogLevel.DEBUG, 'serialization', 'Serialization Metrics Update', {
    //         count: this.count,
    //         avgSerialize: this.avgSerializeTime.toFixed(2) + 'ms',
    //         avgDeserialize: this.avgDeserializeTime.toFixed(2) + 'ms',
    //         maxSerialize: this.maxSerializeTime.toFixed(2) + 'ms',
    //         maxDeserialize: this.maxDeserializeTime.toFixed(2) + 'ms',
    //     });
    // }
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