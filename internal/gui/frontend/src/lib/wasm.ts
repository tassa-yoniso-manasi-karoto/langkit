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
  standardizeMemoryInfo, // Import standardizeMemoryInfo function
  wasmState, // Import the state object itself for persistence
  updateState, // Import the updateState function
  // Import missing functions
  trackOperation,
  updatePerformanceMetrics
} from './wasm-state';
import type { WasmState } from './wasm-state'; // Use type-only import for WasmState
import { settings, wasmActive } from './stores'; // Import wasmActive store and settings
import { get } from 'svelte/store'; // Import get from svelte/store

// --- Update rate limiters for more aggressive throttling ---
// Increase intervals by 5x to reduce frequency
let lastOperationDecisionLog = 0;
let lastMemoryCheckLog = 0;
let lastPerformanceLog = 0;
let lastMaintenanceLog = 0;
let lastStateChangeLog = 0;

// Operation counters with much more aggressive reduction
let operationCounter = 0;

// Only log 0.01% of operations in production (was 0.2%)
function shouldLogOperation(): boolean {
  operationCounter++;

  // In development mode, still log more but much less
  const isDevMode = (window as any).__LANGKIT_VERSION === 'dev';

  if (isDevMode) {
    return operationCounter % 2000 === 0; // 0.05% of operations in dev mode (was 2%)
  } else {
    return operationCounter % 10000 === 0; // 0.01% in production (was 0.2%)
  }
}

// Increase interval for time-based throttling (5 minutes instead of 1)
function shouldLogByInterval(lastLogTime: number, minInterval: number = 300000): [boolean, number] {
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
  get_memory_usage: () => { // Updated return type based on Rust changes (matches get_enhanced_stats)
    total_bytes: number;
    used_bytes: number;
    utilization: number;
    peak_bytes?: number;
    allocation_count?: number;
    // New metrics from Rust (enhanced stats)
    average_allocation?: number;
    allocation_rate?: number;
    time_since_last_gc?: number;
    growth_events?: number;
    growth_failures?: number;
    time_since_last_growth?: number;
    gc_events?: number;
    allocations_since_last_gc?: number;
    total_allocated_bytes?: number;
    reused_bytes?: number;
    memory_efficiency?: number;
    current_pages?: number;
    max_pages?: number; // Theoretical max
    available_pages?: number;
    // Deprecated/Removed metrics from previous version?
    // memory_growth_trend?: number; // This was calculated in TS before, now in Rust? Check Rust code.
    // fragmentation_estimate?: number; // This was calculated in TS before, now in Rust? Check Rust code.
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
  // Add missing function definitions from Rust
  find_log_at_scroll_position?: (
    logs_array: any[],
    log_positions_map: Record<number, number>,
    log_heights_map: Record<number, number>,
    scroll_top: number,
    avg_log_height: number,
    position_buffer: number,
    start_offset?: number // Optional offset added in Rust
  ) => number; // Returns the index
  recalculate_positions?: (
    logs_array: any[],
    log_heights_map: Record<number, number>,
    avg_log_height: number,
    position_buffer: number
  ) => { positions: Record<string, number>, totalHeight: number }; // Returns object with positions and height
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
// Removed duplicate: let lastMemoryCheckLog = 0; (already declared above)


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
// Removed module-level WASM_SIZE_THRESHOLD, will read from settings store
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

/**
 * Request a complete WebAssembly module reset to free memory
 * This is a more effective form of "garbage collection" as it
 * allows the browser to reclaim all WebAssembly memory.
 * 
 * @returns Promise<boolean> indicating success of the operation
 */
export async function requestMemoryReset(): Promise<boolean> {
  // Skip if WebAssembly isn't active
  if (!isWasmEnabled() || !wasmModule) {
    wasmLogger.log(WasmLogLevel.WARN, 'memory', 'Memory reset requested when WebAssembly is not active');
    return false;
  }
  
  wasmLogger.log(WasmLogLevel.INFO, 'memory', 'Performing WebAssembly module reset to free memory');
  
  // Capture current settings
  const currentSettings = get(settings);
  const wasForceEnabled = currentSettings.forceWasmMode === 'enabled';
  
  // Release module reference to allow garbage collection
  wasmModule = null;
  wasmInitialized = false;
  wasmEnabled = false;
  
  // Update the wasmActive indicator
  setWasmActive(false);
  
  // Allow browser garbage collection to run
  await new Promise(resolve => setTimeout(resolve, 50));
  
  // Restore WebAssembly if it was force-enabled
  if (wasForceEnabled) {
    wasmLogger.log(WasmLogLevel.INFO, 'memory', 'Restoring WebAssembly module after memory reset');
    return initializeWasm();
  }
  
  return true;
}

// --- Internal Functions ---

// Removed setWasmSizeThreshold function. Threshold is now managed via the settings store.

// Updated to always pull from settings with fallback
export function getWasmSizeThreshold(): number {
    const $settings = get(settings);

    // If available in settings, use that value (with validation)
    if ($settings?.wasmSizeThreshold !== undefined) {
        return Math.max(
            WASM_CONFIG.MIN_THRESHOLD,
            Math.min($settings.wasmSizeThreshold, WASM_CONFIG.MAX_THRESHOLD)
        );
    }

    // Fallback to default config value if setting is not available
    return WASM_CONFIG.DEFAULT_SIZE_THRESHOLD;
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

      // NEW: Pre-allocate memory for better performance
      if (wasmModule && typeof wasmModule.ensure_sufficient_memory === 'function') {
        try {
          // Pre-allocate 32MB (or appropriate size for your application)
          const initialMemory = 32 * 1024 * 1024; // 32MB in bytes
          wasmLogger.log(
            WasmLogLevel.INFO,
            'init',
            `Pre-allocating ${formatBytes(initialMemory)} of WebAssembly memory`
          );

          const success = wasmModule.ensure_sufficient_memory(initialMemory);
          if (success) {
            const memInfo = wasmModule.get_memory_usage();
            wasmLogger.log(
              WasmLogLevel.INFO,
              'init',
              `Pre-allocation successful: ${formatBytes(memInfo.total_bytes)} total WebAssembly memory`,
              {
                // Add null checks for potentially missing properties from Rust
                pages: memInfo.current_pages ?? 'N/A',
                utilization: (memInfo.utilization * 100).toFixed(1) + '%'
              }
            );
          } else {
            wasmLogger.log(
              WasmLogLevel.WARN,
              'init',
              `Pre-allocation failed: could not reserve ${formatBytes(initialMemory)}`
            );
          }
        } catch (preAllocError) {
          wasmLogger.log(
            WasmLogLevel.WARN,
            'init',
            `Memory pre-allocation error: ${preAllocError}`
          );
          // Continue initialization even if pre-allocation fails
        }
      }

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

      // Threshold is now read directly via getWasmSizeThreshold(), no need to set it here.

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
      }); // Removed extra 'true' argument

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
 * Pre-warms the WebAssembly module with robust memory validation
 */
function preWarmWebAssembly(): void {
  if (!isWasmEnabled() || !wasmModule) {
    wasmLogger.log(WasmLogLevel.INFO, 'init', 'WebAssembly pre-warming skipped: module not available');
    return;
  }
  
  wasmLogger.log(WasmLogLevel.INFO, 'init', 'Starting WebAssembly pre-warming');
  
  try {
    // Get initial memory state using standardized approach
    const initialMemInfo = getStandardizedMemoryInfo();
    wasmLogger.log(
      WasmLogLevel.INFO,
      'memory',
      `Initial memory: ${memoryFormatter.formatBytes(initialMemInfo.total_bytes)} total, ${memoryFormatter.formatBytes(initialMemInfo.used_bytes)} used`,
      { utilization: memoryFormatter.formatUtilization(initialMemInfo.utilization) }
    );
    
    // Attempt a single memory pre-allocation with size validation
    if (typeof wasmModule.ensure_sufficient_memory === 'function') {
      // Try a single 16MB allocation
      const targetSizeMB = 16;
      const bytes = targetSizeMB * 1024 * 1024;
      
      wasmLogger.log(
        WasmLogLevel.INFO,
        'memory',
        `Pre-allocating ${targetSizeMB}MB of WebAssembly memory`
      );
      
      const success = wasmModule.ensure_sufficient_memory(bytes);
      
      // Verify allocation with explicit memory check
      if (success) {
        const afterMemInfo = getStandardizedMemoryInfo();
        
        // Compare memory sizes
        const initialBytes = initialMemInfo.total_bytes;
        const afterBytes = afterMemInfo.total_bytes;
        
        if (afterBytes > initialBytes) {
          const increaseMB = (afterBytes - initialBytes) / (1024 * 1024);
          
          wasmLogger.log(
            WasmLogLevel.INFO,
            'memory',
            `Pre-allocation successful: +${increaseMB.toFixed(1)}MB memory`,
            {
              before: memoryFormatter.formatBytes(initialBytes),
              after: memoryFormatter.formatBytes(afterBytes),
              utilization: memoryFormatter.formatUtilization(afterMemInfo.utilization)
            }
          );
        } else {
          wasmLogger.log(
            WasmLogLevel.INFO,
            'memory',
            `Pre-allocation reported success but memory did not increase. This is expected in some environments.`
          );
        }
      } else {
        wasmLogger.log(
          WasmLogLevel.INFO,
          'memory',
          `Pre-allocation not performed. This is normal in memory-constrained environments.`
        );
      }
    }
    
    // Execute warmup operations with sample data
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
    
    // Execute operations to warm up function paths
    const startTime = performance.now();
    wasmModule.merge_insert_logs(smallArrayA, smallArrayB);
    wasmModule.merge_insert_logs([], smallArrayA);
    wasmModule.merge_insert_logs(smallArrayB, []);
    const warmupTime = performance.now() - startTime;
    
    wasmLogger.log(
      WasmLogLevel.INFO,
      'init',
      `WebAssembly module pre-warmed successfully in ${warmupTime.toFixed(2)}ms`
    );
    
    // Setup maintenance interval
    setupMaintenanceInterval();
  } catch (error) {
    wasmLogger.log(
      WasmLogLevel.ERROR,
      'init',
      `WebAssembly pre-warming failed: ${error instanceof Error ? error.message : String(error)}`
    );
    
    // Use central error handler
    handleWasmError(
      error instanceof Error ? error : new Error(String(error)),
      'preWarmWebAssembly'
    );
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
    // Update settings store directly for persistence
    settings.update($settings => ({
        ...$settings,
        wasmSizeThreshold: newThreshold
    }));
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
    // Use standardized memory info for reliable values
    const memoryInfo = getStandardizedMemoryInfo();

    // Define warning thresholds
    if (memoryInfo.utilization > 0.85) {
      wasmLogger.log(
        WasmLogLevel.WARN, // KEEP AS WARN - serious issue
        'memory',
        `Critical memory pressure: ${(memoryInfo.utilization * 100).toFixed(1)}% used`,
        { utilization: memoryInfo.utilization }
      );
      // Force garbage collection at critical levels
      wasmModule.force_garbage_collection();
    } else if (memoryInfo.utilization > 0.7) {
      wasmLogger.log(
        WasmLogLevel.TRACE, // CHANGED FROM WARN to TRACE - High but not critical is routine
        'memory',
        `High memory pressure: ${(memoryInfo.utilization * 100).toFixed(1)}% used`
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
        // Get memory info using standardized function
        const memoryInfo = getStandardizedMemoryInfo();

        // Update memory usage
        updateMemoryUsage(memoryInfo);

        // Check memory thresholds
        checkMemoryThresholds();

        // Log memory info periodically (CHANGED FROM DEBUG to TRACE)
        const now = Date.now();
        if (now - lastMemoryCheckLog > 300000) { // Every 5 minutes
          lastMemoryCheckLog = now;
          wasmLogger.log(
            WasmLogLevel.TRACE,
            'memory',
            `Memory usage check wasm_total="${(memoryInfo.total_bytes / (1024 * 1024)).toFixed(2)} MB" wasm_used="${(memoryInfo.used_bytes / (1024 * 1024)).toFixed(2)} MB" wasm_utilization=${(memoryInfo.utilization * 100).toFixed(1)}%`
          );
        }
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
      // Get memory info using standardized function for reliable values
      const memoryInfo = getStandardizedMemoryInfo();

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
        // Log GC operations but minimize details to reduce risk of invalid values
        wasmLogger.log(
          WasmLogLevel.INFO,
          'memory',
          `Automatic garbage collection triggered (${(memoryInfo.utilization * 100).toFixed(1)}% memory used)`
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
  Object.entries(metrics.operationTimings).forEach(([operation, stats]: [string, { avgTime: number; count: number }]) => { // Add type annotation for stats
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


// --- Start Simplified Error Handling ---
/**
 * Simplified categorization for WebAssembly errors
 * Focused on recovery actions rather than detailed categorization
 */
function categorizeWasmError(error: Error): {
  category: 'memory' | 'initialization' | 'execution' | 'unknown';
  severity: 'high' | 'low';
  recoverable: boolean;
  recoveryAction: 'disable' | 'reset' | 'blacklist' | 'none';
} {
  // Check for critical initialization errors - disable WebAssembly
  if (error instanceof WebAssembly.RuntimeError ||
      error instanceof WebAssembly.LinkError ||
      error instanceof WebAssembly.CompileError ||
      error instanceof WasmInitializationError) {
    return {
      category: 'initialization',
      severity: 'high',
      recoverable: false,
      recoveryAction: 'disable'
    };
  }

  // Check for memory issues - try a module reset
  if (error instanceof WasmMemoryError ||
      error.message.toLowerCase().includes('memory') ||
      error.message.toLowerCase().includes('allocation')) {
    return {
      category: 'memory',
      severity: 'high',
      recoverable: true,
      recoveryAction: 'reset'
    };
  }

  // Check for operation-specific errors - use blacklist
  if (error instanceof WasmOperationError) {
    return {
      category: 'execution',
      severity: 'low',
      recoverable: true,
      recoveryAction: 'blacklist'
    };
  }

  // Default case - blacklist the operation
  return {
    category: 'unknown',
    severity: 'low',
    recoverable: true,
    recoveryAction: 'blacklist'
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


/**
 * Implements the recovery strategy based on the error classification
 * Uses the direct recoveryAction field to determine what to do
 */
function getRecoveryStrategy(
  error: Error,
  errorType: ReturnType<typeof categorizeWasmError>,
  operation: string
): () => void {
  // Use the direct recoveryAction field to determine what to do
  switch(errorType.recoveryAction) {
    case 'disable':
      return () => {
        wasmLogger.log(
          WasmLogLevel.CRITICAL,
          'recovery',
          `Disabling WebAssembly due to critical error in ${operation}`
        );
        enableWasm(false);
      };
      
    case 'reset':
      return () => {
        wasmLogger.log(
          WasmLogLevel.WARN,
          'memory',
          `Attempting WebAssembly module reset due to memory error in ${operation}`
        );
        // Use the new module reset function for better memory cleanup
        requestMemoryReset().catch(resetError => {
          wasmLogger.log(
            WasmLogLevel.ERROR,
            'memory',
            `Module reset failed: ${resetError.message}`
          );
        });
      };
      
    case 'blacklist':
      return () => {
        addToOperationBlacklist(operation, error);
      };
      
    case 'none':
    default:
      return () => {}; // No action
  }
}


/**
 * Centralized error handler for WebAssembly operations
 * Simplified implementation with direct recovery actions
 *
 * @param error The error that occurred
 * @param operation The operation that failed
 * @param context Additional context information
 */
export function handleWasmError(
  error: unknown,
  operation: string,
  context: Record<string, any> = {}
): void {
  // Ensure error is an instance of Error for consistent handling
  const errorInstance = error instanceof Error ? error : new Error(String(error));
  
  // Categorize the error
  const errorType = categorizeWasmError(errorInstance);

  // Determine log level based on severity
  const logLevel = errorType.severity === 'high' ? WasmLogLevel.ERROR : WasmLogLevel.WARN;

  // Log error with minimal context
  wasmLogger.log(
    logLevel,
    'error',
    `WebAssembly ${operation} failed: ${errorInstance.message}`,
    {
      errorName: errorInstance.name,
      operation,
      category: errorType.category,
      recoveryAction: errorType.recoveryAction
    }
  );

  // Update state (only for high severity errors)
  if (errorType.severity === 'high') {
    setWasmError(errorInstance);
  }

  // Get and apply recovery strategy
  const recovery = getRecoveryStrategy(errorInstance, errorType, operation);
  recovery();
  
  // Report state for significant errors
  if (errorType.severity === 'high') {
    reportWasmState();
  }
}
// --- End Streamlined Error Handling ---
// --- End Phase 1.2: Error Handling Refinement ---


// --- Start Phase 2.1: Enhanced shouldUseWasm ---
/**
 * Determines whether to use WebAssembly for log processing operations.
 * 
 * Simplified implementation with straightforward decision rules:
 * 1. Check if WebAssembly is enabled and not blacklisted
 * 2. Honor forced mode settings
 * 3. Use simple threshold comparison
 *
 * @param totalLogCount The total number of logs to be processed
 * @param operation The operation type (default: 'mergeInsertLogs')
 * @returns Whether WebAssembly should be used for this operation
 */
export function shouldUseWasm(
  totalLogCount: number,
  operation: string = 'mergeInsertLogs'
): boolean {
  // Basic checks first
  if (!isWasmEnabled() || !wasmModule) return false;
  if (isOperationBlacklisted(operation)) return false;
  
  // Get current settings
  const currentSettings = get(settings);
  
  // Honor forced mode settings
  if (currentSettings.forceWasmMode === 'enabled') return true;
  if (currentSettings.forceWasmMode === 'disabled') return false;
  
  // For auto mode, use simple threshold
  const threshold = getOperationThreshold(operation);
  
  // Check memory availability before using WebAssembly
  if (totalLogCount > threshold) {
    const memoryInfo = getStandardizedMemoryInfo();
    if (!memoryInfo.available) return false;
    
    // For very large logs, make sure utilization is reasonable
    if (totalLogCount > 5000 && memoryInfo.utilization > 0.8) {
      return false;
    }
    
    return true;
  }
  
  // Default to TypeScript for small logs
  return false;
}

// Unused functions removed
// --- End Phase 2.1: Enhanced shouldUseWasm ---


// Module-level variables for tracking
let lastMemoryCheckLogTime = 0;

// --- Start Memory Formatting Utilities ---
/**
 * Formats memory values consistently for logging and display
 */
const memoryFormatter = {
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
// --- End Memory Formatting Utilities ---

// --- Start Phase 1.1: checkMemoryAvailability ---
// Default memory info object with safe values
const DEFAULT_MEMORY_INFO = {
  total_bytes: 16 * 1024 * 1024, // 16MB
  used_bytes: 1 * 1024 * 1024,   // 1MB
  utilization: 0.0625,           // 6.25%
  current_pages: 256,            // 16MB / 64KB
  page_size_bytes: 65536,        // 64KB per page
  peak_bytes: 1 * 1024 * 1024,   // Peak usage
  allocation_count: 1,           // Minimal allocation count
  is_valid: true,                // Explicitly mark as valid
  available: true                // Mark memory as available by default
};

/**
 * Gets standardized memory information from WebAssembly
 * This is the ONLY function that should directly access WebAssembly memory metrics
 */
export function getStandardizedMemoryInfo(): any {
  try {
    // Check if WebAssembly is available
    if (!isWasmEnabled() || !wasmModule) {
      return DEFAULT_MEMORY_INFO;
    }
    
    try {
      // Get raw memory info
      const rawMemInfo = wasmModule.get_memory_usage();
      
      // Validate we have a real object before standardization
      if (!rawMemInfo || typeof rawMemInfo !== 'object') {
        wasmLogger.log(
          WasmLogLevel.WARN,
          'memory',
          'WebAssembly returned invalid memory object, using defaults',
          { receivedValue: typeof rawMemInfo }
        );
        return DEFAULT_MEMORY_INFO;
      }
      
      // Standardize the valid object
      return standardizeMemoryInfo(rawMemInfo);
    } catch (innerError) {
      // Log module-specific errors
      wasmLogger.log(
        WasmLogLevel.ERROR,
        'memory',
        `WebAssembly module error: ${innerError instanceof Error ? innerError.message : String(innerError)}`
      );
      return DEFAULT_MEMORY_INFO;
    }
  } catch (error) {
    // Log outer errors (infrequently)
    if (Math.random() < 0.1) {
      wasmLogger.log(
        WasmLogLevel.ERROR,
        'memory',
        `Failed to get memory information: ${error instanceof Error ? error.message : String(error)}`
      );
    }
    
    return DEFAULT_MEMORY_INFO;
  }
}

/**
 * Checks if memory is available for an operation
 * Uses a simplified decision model with reliable criteria
 */
export function checkMemoryAvailability(logCount: number): boolean {
  // For small operations, always proceed
  if (logCount < 500) {
    return true;
  }
  
  // Ensure WebAssembly is available
  if (!isWasmEnabled() || !wasmModule) {
    return false;
  }
  
  try {
    // Get memory info directly
    const memInfo = getStandardizedMemoryInfo();
    
    // Check if memory is available at all
    if (!memInfo.available) {
      return false;
    }
    
    // For very large log sets, ensure plenty of headroom
    if (logCount > 5000 && memInfo.utilization > 0.7) {
      // Try memory reset if utilization is very high
      if (memInfo.utilization > 0.9) {
        // Request a full memory reset as a last resort
        requestMemoryReset().catch(() => {
          // Ignore errors from reset, already logged in the function
        });
        return false;
      }
      return false;
    }
    
    // For medium sized logs, ensure reasonable memory
    if (logCount > 1000 && memInfo.utilization > 0.8) {
      return false;
    }
    
    // Basic estimate: ~250 bytes per log entry
    const estimatedBytes = logCount * 250;
    const totalBytes = memInfo.total_bytes || 0;
    const usedBytes = memInfo.used_bytes || 0;
    
    // Check if we have enough free memory (with 20% buffer)
    const freeBytes = Math.max(0, totalBytes - usedBytes);
    const neededWithBuffer = estimatedBytes * 1.2;
    
    return freeBytes >= neededWithBuffer;
  } catch (error) {
    // Log error and return false
    wasmLogger.log(
      WasmLogLevel.ERROR,
      'memory',
      `Memory check failed: ${error instanceof Error ? error.message : String(error)}`
    );
    return false;
  }
}
// --- End Phase 1.1: checkMemoryAvailability ---


// --- Start Phase 2.2: Serialization ---

/**
 * Optimized serialization for transferring logs to WebAssembly
 *
 * @param logs The log array to serialize
 * @returns The serialized data with timing and optimization info
 */
export function serializeLogsForWasm(logs: any[]): {
  data: any;
  time: number;
  optimization: string;
} {
  const startTime = performance.now();
  let optimization = 'standard';

  try {
    // SIMPLIFIED APPROACH: Always use direct pass-through
    // This avoids potential serialization issues and keeps memory usage predicable
    const result = logs;
    
    const endTime = performance.now();
    const serializationTime = endTime - startTime;

    return {
      data: result,
      time: serializationTime,
      optimization: 'standard' // Always use standard to simplify behavior
    };
  } catch (error: any) {
    const endTime = performance.now();
    const serializationTime = endTime - startTime;

    // Only log serialization errors rarely
    if (Math.random() < 0.1) {
      wasmLogger.log(
        WasmLogLevel.WARN,
        'serialization',
        `Error during log serialization: ${error.message}`
      );
    }

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

    // Time measurement complete

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


// --- Start findLogAtScrollPositionWasm ---
/**
 * Find the log entry at a given scroll position using WebAssembly optimization
 *
 * This function is optimized for frequent scrolling operations with:
 * - Range-limited serialization to reduce overhead
 * - Memory availability checks specialized for scrolling
 * - Intelligent log subset selection based on viewport position
 * 
 * @param logs Array of log entries
 * @param logPositions Map of sequence numbers to Y positions
 * @param logHeights Map of sequence numbers to heights
 * @param scrollTop Current scroll position
 * @param avgLogHeight Average log entry height
 * @param positionBuffer Buffer between log entries
 * @param scrollMetrics Optional metrics about scroll behavior
 * @returns Index of the log entry at the given scroll position
 */
export function findLogAtScrollPositionWasm(
    logs: any[],
    logPositions: Map<number, number>,
    logHeights: Map<number, number>,
    scrollTop: number,
    avgLogHeight: number,
    positionBuffer: number,
    scrollMetrics?: {
        frequency?: number,  // Calls per second for throttling decisions
        visibleLogs?: number // Approximate number of visible logs
    }
): number {
    // Track operation for metrics
    trackOperation('findLogAtScrollPosition');

    // Get WASM module
    const wasmModule = getWasmModule();
    if (!wasmModule || typeof wasmModule.find_log_at_scroll_position !== 'function') {
        throw new WasmOperationError('WebAssembly module not properly initialized', 'findLogAtScrollPosition', {
            moduleAvailable: !!wasmModule,
            functionAvailable: !!wasmModule && typeof wasmModule.find_log_at_scroll_position === 'function'
        });
    }

    // OPTIMIZATION: Only serialize data for visible range + buffer
    // This reduces overhead for large logs arrays
    const estimatedVisibleRange = scrollMetrics?.visibleLogs || 50;
    const buffer = 100; // Safety buffer

    // Calculate the range of logs to consider based on scroll position
    const estimatedIndex = Math.floor(scrollTop / (avgLogHeight + positionBuffer));
    const start = Math.max(0, estimatedIndex - buffer);
    const end = Math.min(logs.length, estimatedIndex + estimatedVisibleRange + buffer);
    
    // Subset of logs to process (only include what's potentially visible)
    const relevantLogs = logs.slice(start, end);

    // Prepare compact data - only convert necessary entries
    const positionsObj: Record<number, number> = {};
    const heightsObj: Record<number, number> = {};
    
    // Only include positions and heights for the relevant range
    for (let i = start; i < end; i++) {
        if (i < logs.length) {
            const log = logs[i];
            const sequence = log._sequence !== undefined ? log._sequence : i;
            
            // Only lookup actual values if they exist, otherwise use defaults
            if (logPositions.has(sequence)) {
                positionsObj[sequence] = logPositions.get(sequence)!;
            }
            
            if (logHeights.has(sequence)) {
                heightsObj[sequence] = logHeights.get(sequence)!;
            }
        }
    }

    // Measure WebAssembly execution time
    const tsStartTime = performance.now();
    
    // Occasionally measure TypeScript performance for comparison (1% of operations)
    let tsTime = 0;
    if (Math.random() < 0.01) {
        // Compare TypeScript implementation performance (but don't use the result)
        let low = 0;
        let high = relevantLogs.length - 1;
        
        while (low <= high) {
            const mid = Math.floor((low + high) / 2);
            const sequence = relevantLogs[mid]._sequence || 0;
            const pos = logPositions.get(sequence) || mid * (avgLogHeight + positionBuffer);
            const height = logHeights.get(sequence) || avgLogHeight + positionBuffer;
            
            if (scrollTop >= pos && scrollTop < (pos + height)) {
                // Found exact log (but don't use result, just measuring)
                break;
            }
            
            if (scrollTop < pos) {
                high = mid - 1;
            } else {
                low = mid + 1;
            }
        }
        
        tsTime = performance.now() - tsStartTime;
    }
    
    const wasmStartTime = performance.now();
    
    // Call WebAssembly function with explicit bounds information
    const result = wasmModule.find_log_at_scroll_position(
        relevantLogs,
        positionsObj,
        heightsObj,
        scrollTop,
        avgLogHeight,
        positionBuffer,
        start // Pass start offset so Rust can adjust result
    );
    
    const wasmEndTime = performance.now();
    const wasmTime = wasmEndTime - wasmStartTime;
    
    // Clear operation from blacklist on success
    clearOperationErrorCount('findLogAtScrollPosition');
    
    // Update performance metrics - especially important for scroll performance
    updatePerformanceMetrics(
        wasmTime,
        tsTime, // Include TS comparison if available
        end - start, // Only count processed logs, not entire array
        'findLogAtScrollPosition',
        0, // Simplified serialization, not tracking overhead separately
        0  // No deserialization overhead for this operation
    );

    // Periodically log scroll performance metrics (very infrequently to avoid spam)
    if (scrollMetrics?.frequency && Math.random() < 0.001) {
        const now = Date.now();
        const lastScrollMetricsLog = getWasmState().lastScrollMetricsLog || 0;
        
        if (now - lastScrollMetricsLog > 5000) { // No more than once per 5 seconds
            wasmLogger.log(
                WasmLogLevel.TRACE,
                'virtualization',
                `Scroll performance: ${wasmTime.toFixed(2)}ms for ${relevantLogs.length} logs`,
                {
                    callFrequency: `${scrollMetrics.frequency.toFixed(1)}/s`,
                    visibleRange: scrollMetrics.visibleLogs,
                    speedup: tsTime > 0 ? (tsTime / wasmTime).toFixed(2) + 'x' : 'N/A',
                    totalLogs: logs.length,
                    processedRange: `${start}-${end}`,
                    viewport: scrollTop.toFixed(0)
                }
            );
            
            // Update the last log timestamp
            updateState({ lastScrollMetricsLog: now });
        }
    }
    
    return result as number;
}
// --- End findLogAtScrollPositionWasm ---


// --- Start recalculatePositionsWasm ---
/**
 * Recalculate log entry positions using WebAssembly optimization
 *
 * This function optimizes the position calculation for log entries with:
 * - Memory availability checks before processing
 * - Performance comparison with TypeScript implementation
 * - Efficient serialization of height data
 * - Adaptive behavior based on log volume
 * 
 * @param logs Array of log entries
 * @param logHeights Map of sequence numbers to heights
 * @param avgLogHeight Average log entry height
 * @param positionBuffer Buffer between log entries
 * @param tsComparisonTime Optional TypeScript time for performance comparison
 * @returns Object containing the calculated positions and total height
 */
export function recalculatePositionsWasm(
    logs: any[],
    logHeights: Map<number, number>,
    avgLogHeight: number,
    positionBuffer: number,
    tsComparisonTime: number = 0
): { positions: Map<number, number>, totalHeight: number } {
    // Track operation for metrics
    trackOperation('recalculatePositions');

    // Get WASM module
    const wasmModule = getWasmModule();
    if (!wasmModule || typeof wasmModule.recalculate_positions !== 'function') {
        throw new WasmOperationError('WebAssembly module not properly initialized', 'recalculatePositions', {
            moduleAvailable: !!wasmModule,
            functionAvailable: !!wasmModule && typeof wasmModule.recalculate_positions === 'function'
        });
    }

    // CRITICAL: Check memory availability for this large calculation
    // This operation works on all logs, so it needs sufficient memory
    const memoryAvailable = checkMemoryAvailability(logs.length);
    if (!memoryAvailable) {
        throw new WasmMemoryError('Insufficient memory for position calculation', {
            logCount: logs.length
        });
    }

    // Convert Map to object efficiently - for large collections, use a more
    // efficient approach that avoids unnecessary lookups
    const heightsObj: Record<number, number> = {};
    
    // Use a lower-level optimization for large height maps
    if (logHeights.size > 1000) {
        // Process in chunks to avoid blocking the main thread
        const entries = Array.from(logHeights.entries());
        const chunkSize = 500;
        
        for (let i = 0; i < entries.length; i += chunkSize) {
            const chunk = entries.slice(i, i + chunkSize);
            for (const [key, value] of chunk) {
                heightsObj[key] = value;
            }
        }
    } else {
        // For smaller collections, use the direct forEach approach
        logHeights.forEach((value, key) => {
            heightsObj[key] = value;
        });
    }

    // Measure performance of TypeScript implementation if needed
    let tsTime = tsComparisonTime;
    if (tsTime <= 0 && Math.random() < 0.05) { // 5% of operations for comparison
        const tsStartTime = performance.now();
        
        // Run TypeScript implementation for comparison, but don't use the result
        // This just simulates the work to measure performance
        let currentPosition = 0;
        let totalHeightTs = 0;
        
        for (const log of logs) {
            const sequence = log._sequence || 0;
            // Don't store positions - just calculate
            const height = logHeights.get(sequence) || avgLogHeight + positionBuffer;
            currentPosition += height;
            totalHeightTs += height;
        }
        
        const tsEndTime = performance.now();
        tsTime = tsEndTime - tsStartTime;
    }

    // Measure WebAssembly execution time
    const wasmStartTime = performance.now();

    // Call WebAssembly function with memory checks
    try {
        const result = wasmModule.recalculate_positions(
            logs,
            heightsObj,
            avgLogHeight,
            positionBuffer
        );

        const wasmEndTime = performance.now();
        const wasmTime = wasmEndTime - wasmStartTime;

        // Clear operation from blacklist on success
        clearOperationErrorCount('recalculatePositions');

        // Update performance metrics with proper comparisons
        updatePerformanceMetrics(
            wasmTime,
            tsTime, // Include TypeScript comparison if available
            logs.length,
            'recalculatePositions',
            0, // Serialization overhead tracked separately if needed
            0  // Deserialization overhead tracked separately if needed
        );

        // Periodically log performance metrics (very infrequently)
        if (Math.random() < 0.02 && logs.length > 1000) { // Only 2% of large recalculations
            const now = Date.now();
            const lastRecalcMetricsLog = getWasmState().lastRecalcMetricsLog || 0;
            
            if (now - lastRecalcMetricsLog > 60000) { // No more than once per minute
                const state = getWasmState();
                const memoryInfo = wasmModule.get_memory_usage();
                
                wasmLogger.log(
                    WasmLogLevel.INFO, // Use INFO for this less frequent operation
                    'virtualization',
                    `Position calculation: ${wasmTime.toFixed(2)}ms for ${logs.length} logs`,
                    {
                        speedup: tsTime > 0 ? (tsTime / wasmTime).toFixed(2) + 'x' : 'N/A',
                        memoryUtilization: memoryInfo ? `${(memoryInfo.utilization * 100).toFixed(1)}%` : 'unknown',
                        averageAllocation: memoryInfo?.average_allocation ? formatBytes(memoryInfo.average_allocation) : 'unknown',
                        peakMemory: memoryInfo?.peak_bytes ? formatBytes(memoryInfo.peak_bytes) : 'unknown'
                    }
                );
                
                // Update the last log timestamp
                updateState({ lastRecalcMetricsLog: now });
            }
        }

        // Convert positions object back to Map efficiently
        const positionsMap = new Map<number, number>();
        const positionsObj = result.positions as Record<string, number>;
        
        // For large collections, use a chunked approach to avoid blocking
        if (logs.length > 1000) {
            const keys = Object.keys(positionsObj);
            const chunkSize = 500;
            
            for (let i = 0; i < keys.length; i += chunkSize) {
                const chunk = keys.slice(i, i + chunkSize);
                for (const key of chunk) {
                    positionsMap.set(parseInt(key, 10), positionsObj[key]);
                }
            }
        } else {
            // Direct conversion for smaller collections
            Object.keys(positionsObj).forEach(key => {
                positionsMap.set(parseInt(key, 10), positionsObj[key]);
            });
        }

        return {
            positions: positionsMap,
            totalHeight: result.totalHeight as number
        };
    } catch (error) {
        // Track memory usage after failed operation
        if (typeof wasmModule.get_memory_usage === 'function') {
            const memInfo = wasmModule.get_memory_usage();
            updateMemoryUsage(memInfo);
        }
        
        // Handle specific error types and rethrow
        if (error instanceof Error) {
            throw error; // Already a proper error object, let handleWasmError handle it
        } else {
            throw new WasmOperationError(
                `Position calculation failed: ${String(error)}`,
                'recalculatePositions',
                { logCount: logs.length }
            );
        }
    }
}
