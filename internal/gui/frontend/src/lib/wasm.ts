import initWasmBindgen, * as wasmGeneratedExports from '../wasm-generated/pkg/log_engine.js';
import { logger } from './logger';
import {
  WasmInitStatus,
  getWasmState as getWasmStateInternal,
  reportWasmState,
  updateMemoryUsage,
  setWasmError,
  resetWasmMetricsInternal,
  standardizeMemoryInfo,
  wasmState,
  updateState,
  trackOperation,
  updatePerformanceMetrics
} from './wasm-state';
import type { WasmState } from './wasm-state';
import { settings, wasmActive } from './stores';
import { get } from 'svelte/store';

let lastMemoryCheckLog = 0;

function shouldLogVerbose(): boolean {
  return (window as any).__LANGKIT_VERSION === 'dev';
}

export class WasmInitializationError extends Error {
    context: Record<string, any>;
    constructor(message: string, context: Record<string, any> = {}) {
        super(message);
        this.name = 'WasmInitializationError';
        this.context = { timestamp: Date.now(), ...context };
    }
}

export class WasmMemoryError extends Error {
    memoryInfo: Record<string, any>;
    constructor(message: string, memoryInfo: Record<string, any> = {}) {
        super(message);
        this.name = 'WasmMemoryError';
        this.memoryInfo = { timestamp: Date.now(), ...memoryInfo };
    }
}

export class WasmOperationError extends Error {
    operation: string;
    details: Record<string, any>;
    constructor(message: string, operation: string, details: Record<string, any> = {}) {
        super(message);
        this.name = 'WasmOperationError';
        this.operation = operation;
        this.details = { timestamp: Date.now(), ...details };
    }
}

export interface WasmModule {
  merge_insert_logs: (existingLogs: any[], newLogs: any[]) => any[];
  get_memory_usage: () => any;
  reset_internal_allocation_stats: () => void;
  estimate_memory_for_logs: (logCount: number) => any;
  contains_text_simd?: (haystack: string, needle: string) => boolean;
  ensure_sufficient_memory?: (needed_bytes: number) => boolean;
  // findLogAtScrollPosition and recalculatePositions removed due to performance issues
}

let wasmModule: WasmModule | null = null;
let wasmInitialized = false;
let wasmEnabled = false;
let initializePromise: Promise<boolean> | null = null;
let wasmBuildInfo: WasmBuildInfo | null = null;

interface WasmBuildInfo {
  version: string;
  timestamp: number;
  buildDate: string;
  wasmSizeBytes?: number;
}

export const WASM_CONFIG = {
  DEFAULT_SIZE_THRESHOLD: 500,
  MIN_THRESHOLD: 100,
  MAX_THRESHOLD: 5000,
  MIN_PERFORMANCE_GAIN: 1.2
};
const operationThresholds = new Map<string, number>();

export function getWasmState() { return getWasmStateInternal(); }

function setWasmActive(active: boolean) { wasmActive.set(active); }

export function resetWasmMetrics(): void {
  resetWasmMetricsInternal();
  try {
    localStorage.removeItem('wasm-metrics');
    if (shouldLogVerbose()) logger.info('store/wasm', 'Cleared saved metrics.');
  } catch (e: any) {
    logger.warn('store/wasm', `Failed clear saved metrics: ${e.message}`);
  }
}

export function getWasmBuildInfo(): WasmBuildInfo | null { return wasmBuildInfo; }

export async function requestMemoryReset(): Promise<boolean> {
  if (!isWasmEnabled() || !wasmModule) {
    logger.warn('store/wasm', 'Mem reset: WASM not active.');
    return false;
  }
  logger.info('store/wasm', 'Performing WASM module reset.');
  const currentSettings = get(settings);
  const wasForceEnabled = currentSettings.forceWasmMode === 'enabled';
  wasmModule = null; wasmInitialized = false; wasmEnabled = false;
  setWasmActive(false);
  await new Promise(resolve => setTimeout(resolve, 50));
  if (wasForceEnabled) {
    logger.info('store/wasm', 'Restoring WASM after reset.');
    return initializeWasm();
  }
  return true;
}

export function getWasmSizeThreshold(): number {
    const $settings = get(settings);
    if ($settings?.wasmSizeThreshold !== undefined) {
        return Math.max(WASM_CONFIG.MIN_THRESHOLD, Math.min($settings.wasmSizeThreshold, WASM_CONFIG.MAX_THRESHOLD));
    }
    return WASM_CONFIG.DEFAULT_SIZE_THRESHOLD;
}

export function setOperationThreshold(operation: string, threshold: number): void {
  const validatedThreshold = Math.max(WASM_CONFIG.MIN_THRESHOLD, Math.min(threshold, WASM_CONFIG.MAX_THRESHOLD));
  operationThresholds.set(operation, validatedThreshold);
  if (shouldLogVerbose()) logger.trace('store/wasm', `Set threshold ${operation}: ${validatedThreshold}`);
}

export function getOperationThreshold(operation: string): number {
  return operationThresholds.get(operation) || getWasmSizeThreshold();
}

export function enableWasm(enabled: boolean): Promise<boolean> {
  const previouslyEnabled = wasmEnabled;
  wasmEnabled = enabled;
  if (shouldLogVerbose()) logger.trace('store/wasm', `WASM ${enabled ? 'enabled' : 'disabled'}`);
  if (enabled && !wasmInitialized && !initializePromise) {
    return initializeWasm();
  }
  if (previouslyEnabled && !enabled) {
      wasmState.initStatus = WasmInitStatus.NOT_STARTED;
      reportWasmState();
  } else if (enabled && wasmInitialized) {
      reportWasmState();
  }
  return Promise.resolve(wasmInitialized);
}

export function isWasmEnabled(): boolean { return wasmEnabled && wasmInitialized; }

export function isWasmSupported(): boolean {
  return typeof WebAssembly === 'object' && typeof WebAssembly.instantiate === 'function';
}

export function getWasmModule(): WasmModule | null { return wasmModule; }

async function loadBuildInfo(version: string = 'unknown'): Promise<WasmBuildInfo | null> {
  try {
    const buildInfoPath = `/wasm/build-info.json?t=${Date.now()}`;
    if (shouldLogVerbose()) logger.debug('store/wasm', `Loading build info: ${buildInfoPath}`);
    const response = await fetch(buildInfoPath);
    if (!response.ok) throw new Error(`Fetch failed: ${response.status}`);
    const buildInfoData = await response.json();
    if (shouldLogVerbose()) logger.debug('store/wasm', `Build info loaded: v${buildInfoData.version}`);
    return buildInfoData;
  } catch (error: any) {
    logger.warn('store/wasm', `Failed to load build-info.json: ${error.message}`);
    return null;
  }
}

export async function initializeWasm(): Promise<boolean> {
  if (initializePromise) return initializePromise;
  if (wasmState.initStatus === WasmInitStatus.SUCCESS) return true;

  wasmState.initStatus = WasmInitStatus.INITIALIZING;
  reportWasmState();
  initializePromise = new Promise<boolean>(async (resolve) => {
    const startTime = performance.now();
    let initTime = 0;

    if (!wasmEnabled) {
      wasmState.initStatus = WasmInitStatus.NOT_STARTED;
      reportWasmState();
      resolve(false);
      return;
    }
    if (shouldLogVerbose()) logger.trace('store/wasm', 'Starting WASM init');

    try {
      if (!isWasmSupported()) throw new WasmInitializationError("WASM not supported");
      const version = (window as any).__LANGKIT_VERSION || 'unknown';
      wasmBuildInfo = await loadBuildInfo(version); // Loads from /public/wasm/build-info.json
      if (shouldLogVerbose()) logger.info('store/wasm', `Env: ${version}, Build: ${wasmBuildInfo?.version || 'N/A'}`);
      
      logger.info('store/wasm', 'Using statically imported, inlined WASM module.');

      // Use the async initializeWithInlinedBinary function which calls __wbg_init
      // This ensures the global overrides for WebAssembly.instantiate are triggered
      logger.info('store/wasm', 'Initializing WASM with inlined binary.');
      const instantiatedModuleExports = await wasmGeneratedExports.initializeWithInlinedBinary();
      logger.info('store/wasm', 'WASM binary initialization completed');

      // Assign values to our module from the result of initialization
      const moduleInstance = {
        merge_insert_logs: wasmGeneratedExports.merge_insert_logs,
        get_memory_usage: wasmGeneratedExports.get_memory_usage,
        reset_internal_allocation_stats: wasmGeneratedExports.reset_internal_allocation_stats,
        ensure_sufficient_memory: wasmGeneratedExports.ensure_sufficient_memory,
        estimate_memory_for_logs: wasmGeneratedExports.estimate_memory_for_logs,
        contains_text_simd: wasmGeneratedExports.contains_text_simd
        // findLogAtScrollPosition and recalculatePositions removed due to performance issues
      } as WasmModule;
      
      let browserApisOk = false;
      let memInfoFromRust;

      // FIRST CHECK: Use get_memory_api_access_status from our inlined JS
      if (typeof wasmGeneratedExports.get_memory_api_access_status === 'function') {
        const apiAccessStatus = wasmGeneratedExports.get_memory_api_access_status();
        if (apiAccessStatus && apiAccessStatus.success === true &&
            apiAccessStatus.has_browser_api_access === true &&
            apiAccessStatus.total_bytes > 0) {
          browserApisOk = true;
          logger.info('store/wasm', 'POST-INIT SUCCESS: WebAssembly browser APIs verified via inliner status check.', { statusResult: apiAccessStatus });
          // Update memory usage with the data from the status check
          updateMemoryUsage({
            total_bytes: apiAccessStatus.total_bytes,
            used_bytes: 0, // Will be updated by Rust tracker
            utilization: 0,
            has_browser_api_access: true,
            available: true
          });
        } else {
          logger.critical('store/wasm', 'POST-INIT WARNING: WebAssembly memory access check failed in inliner.', { statusResult: apiAccessStatus });
          // Continue to second check - don't fail yet
        }
      } else {
        logger.warn('store/wasm', 'get_memory_api_access_status function not found in WASM module.');
        // Continue to second check
      }

      // SECOND CHECK: Verify through the Rust get_memory_usage function
      if (!browserApisOk && moduleInstance && typeof moduleInstance.get_memory_usage === 'function') {
        try {
          memInfoFromRust = moduleInstance.get_memory_usage();
          if (memInfoFromRust && memInfoFromRust.has_browser_api_access === true && memInfoFromRust.total_bytes > 0) {
            browserApisOk = true;
            logger.info('store/wasm', 'POST-INIT SUCCESS: WASM APIs verified through Rust function.', { total_bytes: memInfoFromRust.total_bytes });
            updateMemoryUsage(memInfoFromRust);
          } else {
            logger.critical('store/wasm', 'POST-INIT FAILURE: Rust reports WebAssembly APIs inaccessible.', { memInfoFromRust });
          }
        } catch (e) {
          logger.critical('store/wasm', 'POST-INIT ERROR calling get_memory_usage()', { e });
        }
      } else if (!browserApisOk) {
        logger.critical('store/wasm', 'POST-INIT FAILURE: WebAssembly browser APIs inaccessible via memory info');
      }

      if (!browserApisOk) {
        throw new WasmInitializationError('WebAssembly browser APIs are not accessible after initialization', { memInfoFromRust });
      }

      wasmModule = moduleInstance;
      wasmInitialized = true;
      wasmState.initStatus = WasmInitStatus.SUCCESS;
      initTime = performance.now() - startTime;
      wasmState.initTime = initTime;

      if (wasmModule.ensure_sufficient_memory) {
        try {
          // Get initial memory using standardizeMemoryInfo helper to handle Map
          const stdMemInfo = memInfoFromRust ? standardizeMemoryInfo(memInfoFromRust) : null;
          const initialBytes = stdMemInfo?.total_bytes || 0;
          const preallocTarget = 32 * 1024 * 1024;
          if (shouldLogVerbose()) logger.info('store/wasm', `Pre-allocating ${formatBytes(preallocTarget)}.`);
          if (wasmModule.ensure_sufficient_memory(preallocTarget)) {
            const afterMem = wasmModule.get_memory_usage();
            // Log the detailed memory object for diagnostics - always use standardizeMemoryInfo
            if (shouldLogVerbose()) {
              // Use the standardizeMemoryInfo helper which handles Maps correctly
              const stdAfterMem = standardizeMemoryInfo(afterMem);
              logger.info('store/wasm', `Pre-alloc success. After: ${formatBytes(stdAfterMem?.total_bytes)}`);

              // Log extra info in dev mode
              if ((window as any).__LANGKIT_VERSION === 'dev') {
                // Check if afterMem is a Map
                const isMap = Object.prototype.toString.call(afterMem) === '[object Map]';

                if (isMap) {
                  // Log Map keys for diagnostics
                  const mapKeys = [];
                  try {
                    (afterMem as Map<string, any>).forEach((v, k) => mapKeys.push(k));
                    logger.debug('store/wasm', `Memory is a Map with keys: ${mapKeys.join(', ')}`);
                  } catch (e) {
                    logger.debug('store/wasm', `Error getting Map keys: ${e.message}`);
                  }
                } else {
                  // Log regular object keys
                  const objKeys = Object.keys(afterMem || {});
                  logger.debug('store/wasm', `Memory is a regular object with keys: ${objKeys.join(', ')}`);
                }
              }
            }
          } else { logger.warn('store/wasm', `Pre-alloc failed.`); }
        } catch (e) { logger.warn('store/wasm', `Pre-alloc error: ${e}`); }
      }
      
      logger.info('store/wasm', 'WASM module initialized successfully.', { initTime: initTime.toFixed(0), wasmSize: getWasmSize() });
      scheduleMemoryCheck();
      loadSavedMetrics();
      if (wasmInitialized) setTimeout(() => preWarmWebAssembly(), 500);
      resolve(true);
    } catch (error: unknown) {
      initTime = performance.now() - startTime;
      const err = error instanceof Error ? error : new Error(String(error));
      logger.error('store/wasm', `WASM Init Error: ${err.message}`, { errName: err.name });
      handleWasmError(err, 'initialization', { initTime, buildInfo });
      wasmInitialized = false; wasmModule = null;
      wasmState.initStatus = WasmInitStatus.FAILED; wasmState.initTime = initTime;
      resolve(false);
    } finally {
      reportWasmState();
      initializePromise = null;
    }
  });
  return initializePromise;
}

function preWarmWebAssembly(): void {
  if (!isWasmEnabled() || !wasmModule) return;
  if (shouldLogVerbose()) logger.info('store/wasm', 'Pre-warming WASM.');
  try {
    // Create simple objects for pre-warming that match the expected structure
    // IMPORTANT: We need to create plain objects, not Maps
    const createPlainObject = (index: number) => {
      // Create a plain JavaScript object with all the properties
      return {
        level: 'INFO',
        message: 'Test message ' + index,
        time: new Date().toISOString(),
        _sequence: index,
        _unix_time: Date.now() / 1000
      };
    };

    // Create arrays of plain objects
    const sa = [createPlainObject(1), createPlainObject(2)];
    const sb = [createPlainObject(3), createPlainObject(4)];

    // Convert them if needed with our serialization helper
    const serializedA = ensureWasmSerializable(sa);
    const serializedB = ensureWasmSerializable(sb);

    logger.trace('store/wasm', 'Pre-warming with serialized test objects', {
      objectType: Object.prototype.toString.call(serializedA[0]),
      isMap: Object.prototype.toString.call(serializedA[0]) === '[object Map]'
    });

    // Debug the serialized objects
    logger.trace('store/wasm', 'First serialized object', {
      keys: Object.keys(serializedA[0]).join(', '),
      level: serializedA[0].level,
      message: serializedA[0].message
    });

    wasmModule.merge_insert_logs(serializedA, serializedB);

    if (shouldLogVerbose()) logger.info('store/wasm', `Pre-warm complete.`);
    setupMaintenanceInterval();
  } catch (error) {
    logger.error('store/wasm', 'Pre-warm failed', {
      error: error instanceof Error ? error.message : String(error)
    });
    handleWasmError(error instanceof Error ? error : new Error(String(error)), 'preWarm');
  }
}

function setupMaintenanceInterval(): void {
  if (wasmState.maintenanceIntervalId) clearInterval(wasmState.maintenanceIntervalId);
  const intervalId = setInterval(() => {
    if (!isWasmEnabled() || !wasmModule || !wasmState.lastUsed || (Date.now() - wasmState.lastUsed < 600000)) return;
    if (shouldLogVerbose()) logger.trace('store/wasm', 'Performing WASM maintenance.');
    try {
      // Example: if (wasmModule.get_memory_usage()?.utilization_estimate > 0.7 && wasmModule.reset_internal_allocation_stats) wasmModule.reset_internal_allocation_stats();
      adjustSizeThresholds();
    } catch (e) { if (shouldLogVerbose()) logger.warn('store/wasm', `Maint. error: ${e}`);}
  }, 300000);
  updateState({ maintenanceIntervalId: intervalId as unknown as number });
  window.addEventListener('beforeunload', () => { if (wasmState.maintenanceIntervalId) clearInterval(wasmState.maintenanceIntervalId); });
}

const ADAPTIVE_LEARNING_CONFIG = { MIN_OPERATIONS_REQUIRED: 20, ADJUSTMENT_INTERVAL: 600000, MAX_ADJUSTMENT_FACTOR: 0.2, PERFORMANCE_MARGIN: 1.1 };
let lastThresholdAdjustmentTime = 0; // For adjustSizeThresholds

export function adjustSizeThresholds(): boolean {
  const now = Date.now();
  if (now - lastThresholdAdjustmentTime < ADAPTIVE_LEARNING_CONFIG.ADJUSTMENT_INTERVAL) return false;
  const state = getWasmStateInternal();
  if (state.performanceMetrics.operationsCount < ADAPTIVE_LEARNING_CONFIG.MIN_OPERATIONS_REQUIRED) return false;
  
  const metrics = state.performanceMetrics;
  const currentThreshold = getWasmSizeThreshold();
  let newThreshold = currentThreshold;
  let adjustmentReason = "";

  if ((metrics.logSizeDistribution.small||0) >=5 && metrics.netSpeedupRatio > WASM_CONFIG.MIN_PERFORMANCE_GAIN * ADAPTIVE_LEARNING_CONFIG.PERFORMANCE_MARGIN) {
      newThreshold = Math.max(WASM_CONFIG.MIN_THRESHOLD, Math.floor(currentThreshold * (1 - ADAPTIVE_LEARNING_CONFIG.MAX_ADJUSTMENT_FACTOR * 0.5)));
      if (newThreshold < currentThreshold) adjustmentReason = 'small_high_perf';
  } else if ((metrics.logSizeDistribution.medium||0) >=5 && metrics.netSpeedupRatio < WASM_CONFIG.MIN_PERFORMANCE_GAIN) {
      newThreshold = Math.min(WASM_CONFIG.MAX_THRESHOLD, Math.ceil(currentThreshold * (1 + ADAPTIVE_LEARNING_CONFIG.MAX_ADJUSTMENT_FACTOR * 0.5)));
      if (newThreshold > currentThreshold) adjustmentReason = 'medium_low_perf';
  }

  if (newThreshold !== currentThreshold) {
    settings.update($s => ({ ...$s, wasmSizeThreshold: newThreshold }));
    lastThresholdAdjustmentTime = now;
    // updateState({ thresholdAdjustments: ... }); // Optional: track adjustments in wasmState
    if (shouldLogVerbose()) logger.trace('store/wasm', `Threshold: ${currentThreshold} -> ${newThreshold} (${adjustmentReason})`);
    return true;
  }
  return false;
}

function checkMemoryThresholds(): void {
  if (!isWasmEnabled() || !wasmModule || !wasmModule.get_memory_usage) return;
  try {
    const memInfo = standardizeMemoryInfo(wasmModule.get_memory_usage());
    if (memInfo.utilization > 0.85) {
      logger.warn('store/wasm', `Critical memory: ${(memInfo.utilization*100).toFixed(0)}%`);
      if (wasmModule.reset_internal_allocation_stats) wasmModule.reset_internal_allocation_stats();
    } else if (memInfo.utilization > 0.7 && shouldLogVerbose()) {
      logger.trace('store/wasm', `High memory: ${(memInfo.utilization*100).toFixed(0)}%`);
    }
  } catch (e) { logger.error('store/wasm', `Mem check fail: ${e}`); }
}

function scheduleMemoryCheck() {
  if (!wasmInitialized || !wasmModule) return;
  // setupAutomaticMemoryReset(); // Simplified: reset only on critical threshold
  // setupMemoryMonitoring(); // Simplified: can be re-added if leak detection is critical
  setInterval(() => {
    if (wasmModule && wasmState.lastUsed && (Date.now() - wasmState.lastUsed < 300000)) {
      try {
        const memInfo = getStandardizedMemoryInfo();
        updateMemoryUsage(memInfo);
        checkMemoryThresholds(); // This will call reset_internal_allocation_stats if needed
        const now = Date.now();
        if (shouldLogVerbose() && (now - lastMemoryCheckLog > 300000)) {
          lastMemoryCheckLog = now;
          logger.trace('store/wasm', `Mem: ${formatBytes(memInfo.used_bytes)}/${formatBytes(memInfo.total_bytes)} (${(memInfo.utilization*100).toFixed(0)}%)`);
        }
      } catch (e) { logger.error('store/wasm', `Periodic mem check fail: ${e}`); }
    }
  }, 30000);
}

// Removed setupAutomaticMemoryReset and setupMemoryMonitoring as separate complex functions to save tokens.
// Their core ideas (resetting stats on high usage) are partly integrated into checkMemoryThresholds or can be.
// analyzeMemoryTrend is also removed for brevity.

function formatBytes(bytes: number): string {
  if (typeof bytes !== 'number' || Number.isNaN(bytes) || bytes < 0) return 'N/A';
  if (bytes === 0) return '0 B';
  const u = ['B', 'KB', 'MB', 'GB']; const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${u[i]}`;
}
function formatTime(ms: number): string { 
  if (ms < 0) return 'N/A'; if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60000) return `${(ms/1000).toFixed(1)}s`; return `${(ms/60000).toFixed(1)}m`;
}

function getWasmSize(): number {
  try {
    if (wasmBuildInfo && typeof (wasmBuildInfo as any).wasmSizeBytes === 'number') {
      return (wasmBuildInfo as any).wasmSizeBytes;
    }
    if (shouldLogVerbose()) logger.debug('store/wasm', 'getWasmSize: wasmSizeBytes missing.');
    return 0;
  } catch (e) { logger.warn('store/wasm', `WASM size error: ${e}`); return 0; }
}

function loadSavedMetrics(): void {
  try {
    const saved = localStorage.getItem('wasm-metrics'); if (!saved) return;
    const p = JSON.parse(saved);
    if (p.performanceMetrics) wasmState.performanceMetrics = { ...wasmState.performanceMetrics, ...p.performanceMetrics };
    if (typeof p.totalOperations === 'number') wasmState.totalOperations = p.totalOperations;
    if (p.operationsPerType) wasmState.operationsPerType = { ...p.operationsPerType };
    if (shouldLogVerbose()) logger.info('store/wasm', 'Loaded saved metrics.');
  } catch (e) { logger.warn('store/wasm', `Load metrics fail: ${e}`); localStorage.removeItem('wasm-metrics'); }
}
function saveMetrics(): void {
  if (!wasmInitialized) return;
  try {
    localStorage.setItem('wasm-metrics', JSON.stringify({ performanceMetrics: wasmState.performanceMetrics, totalOperations: wasmState.totalOperations, operationsPerType: wasmState.operationsPerType, savedAt: new Date().toISOString() }));
    if (shouldLogVerbose()) logger.debug('store/wasm', 'Saved metrics.');
  } catch (e) { logger.warn('store/wasm', `Save metrics fail: ${e}`); }
}
function setupMetricsPersistence(): void {
  setInterval(() => { if (wasmInitialized) saveMetrics(); }, 300000);
  window.addEventListener('beforeunload', () => { if (wasmInitialized) saveMetrics(); });
}
setupMetricsPersistence();

function categorizeWasmError(error: Error): any { 
  if (error instanceof WebAssembly.RuntimeError || error instanceof WebAssembly.LinkError || error instanceof WebAssembly.CompileError || error instanceof WasmInitializationError) return { category: 'initialization', severity: 'high', recoverable: false, recoveryAction: 'disable' };
  if (error instanceof WasmMemoryError || error.message.toLowerCase().includes('memory') || error.message.toLowerCase().includes('allocation')) return { category: 'memory', severity: 'high', recoverable: true, recoveryAction: 'reset' };
  if (error instanceof WasmOperationError) return { category: 'execution', severity: 'low', recoverable: true, recoveryAction: 'blacklist' };
  return { category: 'unknown', severity: 'low', recoverable: true, recoveryAction: 'blacklist' };
}

const operationBlacklist: Map<string, any> = new Map();
export function isOperationBlacklisted(operation: string): boolean { 
  const entry = operationBlacklist.get(operation); if (!entry) return false;
  if (Date.now() > entry.nextRetryTime) { operationBlacklist.delete(operation); updateBlacklistState(); if (shouldLogVerbose()) logger.trace('store/wasm', `Retry ${operation}`); return false; }
  return true;
}
export function addToOperationBlacklist(operation: string, error?: Error): void { 
  const now = Date.now(), existing = operationBlacklist.get(operation), retryCount = existing ? existing.retryCount + 1 : 1;
  const backoffMs = Math.min(5000 * Math.pow(3, retryCount - 1), 1800000);
  const entry = { operation, timestamp: now, retryCount, nextRetryTime: now + backoffMs, lastError: error?.message, backoffMs };
  operationBlacklist.set(operation, entry);
  logger.warn('store/wasm', `Blacklisting ${operation} for ${Math.round(backoffMs/1000)}s (#${retryCount})`, { err: error?.message });
  updateBlacklistState();
}
export function clearOperationErrorCount(operation: string): void {
  if (operationBlacklist.has(operation)) { if (shouldLogVerbose()) logger.trace('store/wasm', `${operation} succeeded, removed from blacklist.`); operationBlacklist.delete(operation); updateBlacklistState(); }
}
function updateBlacklistState(): void {
  updateState({ blacklistedOperations: Array.from(operationBlacklist.values()) });
}
function isWasmInitializationError(error: Error): boolean {
    return error instanceof WasmInitializationError || error instanceof WebAssembly.RuntimeError || error instanceof WebAssembly.LinkError || error instanceof WebAssembly.CompileError;
}

function getRecoveryStrategy(error: Error, errorType: any, operation: string): () => void { 
  switch(errorType.recoveryAction) {
    case 'disable': return () => { logger.critical('store/wasm', `Disabling WASM: critical error in ${operation}`); enableWasm(false); };
    case 'reset': return () => { logger.warn('store/wasm', `Attempting WASM reset: memory error in ${operation}`); requestMemoryReset().catch(e => logger.error('store/wasm', `Module reset fail: ${e}`)); };
    case 'blacklist': return () => addToOperationBlacklist(operation, error);
    default: return () => {};
  }
}
export function handleWasmError(error: unknown, operation: string, context: Record<string, any> = {}): void { 
  const err = error instanceof Error ? error : new Error(String(error));
  const errorType = categorizeWasmError(err);
  logger.warn('store/wasm', `WASM ${operation} fail: ${err.message}`, { name: err.name, op: operation, cat: errorType.category, rec: errorType.recoveryAction });
  if (errorType.severity === 'high') setWasmError(err);
  getRecoveryStrategy(err, errorType, operation)();
  if (errorType.severity === 'high') reportWasmState();
}

export function shouldUseWasm(totalLogCount: number, operation: string = 'mergeInsertLogs'): boolean {
  if (!isWasmEnabled() || !wasmModule || isOperationBlacklisted(operation)) return false;
  const settingsNow = get(settings);
  if (settingsNow.forceWasmMode === 'enabled') return true;
  if (settingsNow.forceWasmMode === 'disabled') return false;
  if (totalLogCount > getOperationThreshold(operation)) {
    const memInfo = getStandardizedMemoryInfo();
    if (!memInfo.available) return false;
    if (totalLogCount > 5000 && memInfo.utilization > 0.8) return false;
    return true;
  }
  return false;
}

const memoryFormatter = { 
  formatBytes(bytes: number): string { if (typeof bytes !== 'number' || Number.isNaN(bytes) || bytes < 0) return 'N/A'; if (bytes === 0) return '0 B'; const u = ['B', 'KB', 'MB', 'GB']; const i = Math.floor(Math.log(bytes) / Math.log(1024)); return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${u[i]}`; },
  formatUtilization(utilization: number): string { if (typeof utilization !== 'number' || Number.isNaN(utilization)) return 'N/A'; return `${(utilization * 100).toFixed(1)}%`; },
  formatMemoryInfo(memInfo: any): Record<string, string> { return { total: this.formatBytes(memInfo.total_bytes), used: this.formatBytes(memInfo.used_bytes), utilization: this.formatUtilization(memInfo.utilization), pages: String(memInfo.current_pages || 'N/A') }; }
};

const DEFAULT_MEMORY_INFO = { 
  total_bytes: 16777216, used_bytes: 1048576, utilization: 0.0625, current_pages: 256, page_size_bytes: 65536, peak_bytes: 1048576, allocation_count: 1, is_valid: true, available: true
};
export function getStandardizedMemoryInfo(): any {
  try {
    if (!isWasmEnabled() || !wasmModule || !wasmModule.get_memory_usage) return DEFAULT_MEMORY_INFO;
    const rawMemInfo = wasmModule.get_memory_usage();
    if (!rawMemInfo || typeof rawMemInfo !== 'object') { if (shouldLogVerbose()) logger.warn('store/wasm', 'Invalid mem obj from WASM'); return DEFAULT_MEMORY_INFO; }
    return standardizeMemoryInfo(rawMemInfo);
  } catch (e) { if (shouldLogVerbose()) logger.error('store/wasm', `getStdMemInfo err: ${e}`); return DEFAULT_MEMORY_INFO; }
}
export function checkMemoryAvailability(logCount: number): boolean {
  if (logCount < 500) return true;
  if (!isWasmEnabled() || !wasmModule) return false;
  try {
    const memInfo = getStandardizedMemoryInfo();
    if (!memInfo.available) return false;
    if (logCount > 5000 && memInfo.utilization > 0.7) { if (memInfo.utilization > 0.9) requestMemoryReset(); return false; }
    if (logCount > 1000 && memInfo.utilization > 0.8) return false;
    const estBytes = logCount * 250, total = memInfo.total_bytes||0, used = memInfo.used_bytes||0;
    return (Math.max(0, total - used)) >= (estBytes * 1.2);
  } catch (e) { logger.error('store/wasm', `checkMemAvail err: ${e}`); return false; }
}

/**
 * Helper function to ensure objects are serializable for WebAssembly
 * Converts JavaScript Maps to plain objects/arrays
 * This is needed because serde_wasm_bindgen can't deserialize JS Map objects directly
 */
function ensureWasmSerializable<T>(data: T[]): any[] {
  return data.map(item => {
    if (item === null || item === undefined) {
      return item;
    }

    // Check if the item is a Map
    if (Object.prototype.toString.call(item) === '[object Map]') {
      // Convert Map to plain object
      const obj: Record<string, any> = {};
      try {
        (item as unknown as Map<string, any>).forEach((value, key) => {
          obj[key] = value;
        });
      } catch (e) {
        logger.error('store/wasm', "Error converting Map to object", { error: e });
      }
      return obj;
    }

    // For regular objects, deeply convert any Map properties
    if (typeof item === 'object') {
      const newObj: Record<string, any> = {};
      Object.keys(item).forEach(key => {
        const value = (item as Record<string, any>)[key];

        // Recursively convert Maps in nested objects
        if (value && typeof value === 'object' &&
            Object.prototype.toString.call(value) === '[object Map]') {
          const subObj: Record<string, any> = {};
          try {
            (value as Map<string, any>).forEach((v, k) => {
              subObj[k] = v;
            });
            newObj[key] = subObj;
          } catch (e) {
            newObj[key] = value; // Keep original if conversion fails
          }
        } else {
          newObj[key] = value;
        }
      });
      return newObj;
    }

    // Primitive values pass through unchanged
    return item;
  });
}

export function serializeLogsForWasm(logs: any[]): { data: any; time: number; optimization: string; } {
  const s = performance.now();

  // Validate input
  if (!logs) {
    logger.warn('store/wasm', 'Null or undefined logs array passed to serialization');
    return { data: [], time: 0, optimization: 'empty_array_fallback' };
  }

  // Handle non-arrays
  if (!Array.isArray(logs)) {
    logger.warn('store/wasm', 'Non-array value passed to serializeLogsForWasm', {
      type: typeof logs,
      toString: Object.prototype.toString.call(logs)
    });
    return { data: [], time: 0, optimization: 'empty_array_fallback' };
  }

  // Skip conversion for empty arrays - but return safely
  if (logs.length === 0) {
    return { data: [], time: 0, optimization: 'empty_array' };
  }

  try {
    // Log for diagnostics
    if (logs.length > 0) {
      const firstItem = logs[0];
      logger.trace('store/wasm', 'Serializing logs array for WASM', {
        count: logs.length,
        firstItemType: typeof firstItem,
        firstItemToString: firstItem ? Object.prototype.toString.call(firstItem) : 'null',
        hasMap: logs.some(item => item && Object.prototype.toString.call(item) === '[object Map]')
      });
    }

    // Ensure all objects are serializable by converting Maps to plain objects
    const serializedLogs = ensureWasmSerializable(logs);

    return { data: serializedLogs, time: performance.now() - s, optimization: 'map_conversion' };
  }
  catch (e) {
    // Always log serialization errors - they're critical
    logger.warn('store/wasm', `Serialization error: ${e instanceof Error ? e.message : String(e)}`, {
      logCount: logs.length
    });

    // Try to create a safe version of the logs
    const safeData = logs.map(item => {
      if (item === null || item === undefined) return item;
      if (typeof item !== 'object') return item;
      
      // Create a new plain object with basic properties only
      return { 
        level: item.level || 'info',
        message: item.message || '',
        time: item.time || new Date().toISOString(),
        _sequence: item._sequence || 0,
        _unix_time: item._unix_time || (Date.now() / 1000)
      };
    });

    return { data: safeData, time: performance.now() - s, optimization: 'error_fallback'};
  }
}

export function deserializeLogsFromWasm(data: any): { logs: any[]; time: number; } {
  const s = performance.now();

  // Validate input
  if (!data) {
    logger.warn('store/wasm', 'Null or undefined data returned from WASM');
    return { logs: [], time: 0 };
  }

  try {
    // Log result type for diagnostics (changed from DEBUG to TRACE to reduce log spam)
    logger.trace('store/wasm', 'Deserializing data from WASM', {
      type: typeof data,
      toString: Object.prototype.toString.call(data),
      isArray: Array.isArray(data),
      length: Array.isArray(data) ? data.length : 'n/a',
      sample: Array.isArray(data) && data.length > 0 ?
        JSON.stringify(data[0]).substring(0, 100) + '...' : 'none'
    });

    // Convert to array if it's not already
    let resultArray: any[] = [];

    if (Array.isArray(data)) {
      resultArray = data;
      // Log the array length to help diagnose empty results (changed from DEBUG to TRACE to reduce log spam)
      //logger.trace('store/wasm', `Deserialized array length: ${resultArray.length}`);

      // If array is empty when we expected data, log a warning
      if (resultArray.length === 0) {
        logger.warn('store/wasm', 'WASM returned empty array when non-empty was expected');
      }

      // Log a sample item to see if it has the right structure (changed from DEBUG to TRACE to reduce log spam)
      if (resultArray.length > 0) {
        const sample = resultArray[0];
        logger.trace('store/wasm', 'Sample deserialized log entry', {
          keys: Object.keys(sample).join(', '),
          hasLevel: sample.hasOwnProperty('level'),
          hasMessage: sample.hasOwnProperty('message'),
          hasTime: sample.hasOwnProperty('time'),
          sample: JSON.stringify(sample).substring(0, 100) + '...'
        });
      }
    } else if (typeof data === 'object' && data !== null) {
      // Try to extract values if it's a Map-like object
      if (Object.prototype.toString.call(data) === '[object Map]') {
        try {
          resultArray = Array.from((data as Map<any, any>).values());
          logger.debug('store/wasm', `Converted Map to array of length: ${resultArray.length}`);
        } catch (mapErr) {
          logger.warn('store/wasm', `Failed to extract Map values: ${mapErr}`);
        }
      } else {
        // Try to convert object to array if it has numeric keys
        const keys = Object.keys(data).filter(k => !isNaN(Number(k)));
        if (keys.length > 0) {
          resultArray = keys.map(k => data[k]).filter(Boolean);
          logger.debug('store/wasm', `Converted object with numeric keys to array of length: ${resultArray.length}`);
        } else {
          // Last attempt - check if there's a data or entries field
          if (data.data && Array.isArray(data.data)) {
            resultArray = data.data;
            logger.debug('store/wasm', `Used data field from object, length: ${resultArray.length}`);
          } else if (data.entries && Array.isArray(data.entries)) {
            resultArray = data.entries;
            logger.debug('store/wasm', `Used entries field from object, length: ${resultArray.length}`);
          } else {
            // Try to wrap the single object in an array if it looks like a log entry
            if (data.level && data.message) {
              resultArray = [data];
              logger.debug('store/wasm', `Wrapped single log entry in array`);
            } else {
              logger.warn('store/wasm', `Couldn't convert object to array`, {
                keys: Object.keys(data).join(', ')
              });
            }
          }
        }
      }
    }

    // Ensure any objects in the array have the required properties
    // This is critical to prevent "undefined is not an object" errors
    resultArray = resultArray.map(item => {
      if (!item) return {
        level: 'info',
        message: 'Empty log entry',
        time: new Date().toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' }),
        _unix_time: Date.now() / 1000,
        _sequence: 0
      };

      // Ensure level is always a string
      if (!item.level) item.level = 'info';
      else if (typeof item.level !== 'string') item.level = String(item.level);

      // Ensure message is always a string
      if (!item.message) item.message = '';
      else if (typeof item.message !== 'string') item.message = String(item.message);

      // Format time as HH:MM:SS - get time from multiple potential sources
      // Priority:
      // 1. Use existing HH:MM:SS formatted string if available
      // 2. Use _unix_time if available to create fresh formatted string
      // 3. Use original time string with parsing
      // 4. Use current time as fallback
      let formattedTime;

      // First check if already in HH:MM:SS format
      if (typeof item.time === 'string' && /^\d{2}:\d{2}:\d{2}$/.test(item.time)) {
        // Already correctly formatted, use as-is
        formattedTime = item.time;
      }
      // Next try to format from unix timestamp if available
      else if (typeof item._unix_time === 'number' && item._unix_time > 0) {
        try {
          const date = new Date(item._unix_time * 1000); // Convert seconds to milliseconds
          if (!isNaN(date.getTime())) {
            formattedTime = date.toLocaleTimeString('en-US', {
              hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit'
            });
          }
        } catch (e) {
          // Fall through to next option if this fails
        }
      }

      // If we still don't have a formatted time, try to use the time string
      if (!formattedTime && typeof item.time === 'string') {
        try {
          const date = new Date(item.time);
          if (!isNaN(date.getTime())) {
            formattedTime = date.toLocaleTimeString('en-US', {
              hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit'
            });
          } else {
            // Invalid date, use original string if it has some content
            formattedTime = item.time || null;
          }
        } catch (e) {
          // If parsing fails, use the original string if it has content
          formattedTime = item.time || null;
        }
      }

      // Final fallback - use current time
      if (!formattedTime) {
        formattedTime = new Date().toLocaleTimeString('en-US', {
          hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit'
        });
      }

      // Return the processed item with properly formatted time
      return {
        ...item,
        level: item.level,
        message: item.message,
        time: formattedTime,
        _unix_time: item._unix_time || Date.now() / 1000,
        _sequence: item._sequence || 0
      };
    });

    // Log the final return value (changed from DEBUG to TRACE to reduce log spam)
    //logger.trace('store/wasm', `Returning array of length: ${resultArray.length}`);

    return { logs: resultArray, time: performance.now() - s };
  } catch (e) {
    logger.warn('store/wasm', `Deserialization error: ${e instanceof Error ? e.message : String(e)}`);
    return { logs: [], time: performance.now() - s };
  }
}

