import { writable, get, derived } from 'svelte/store';
import { settings, userActivityState } from './stores';
import { logger } from './logger';
import {
  isWasmEnabled,
  getWasmModule,
  getWasmSizeThreshold,
  shouldUseWasm,
  // canProcessSafely, // Removed, logic is in checkMemoryAvailability
  handleWasmError,
  WasmOperationError,
  // New imports based on updated wasm.ts
  isOperationBlacklisted,
  checkMemoryAvailability,
  serializeLogsForWasm,
  deserializeLogsFromWasm,
  clearOperationErrorCount
} from './wasm';
import type { WasmModule } from './wasm'; // Import WasmModule as type

// Keep direct state function imports
import { trackOperation, updatePerformanceMetrics } from './wasm-state'; // setWasmError is handled within handleWasmError now

export interface LogMessage {
    level: string;
    message: string;
    time: string;
    behavior?: string;
    _sequence?: number;        // Monotonically increasing number from backend
    _unix_time?: number;       // Unix timestamp in milliseconds for efficient sorting
    _original_time?: string;   // Original RFC3339 timestamp string
    _visible?: boolean;        // Flag for virtualization (internal use)
    _height?: number;          // Cached height for virtualization (internal use)
    // Allow any additional fields
    [key: string]: any;
}

type LogIndex = Map<number, number>; // Maps sequence number -> array index

function createLogStore() {
    // Main store with all logs
    const logsWritable = writable<LogMessage[]>([]);
    const { subscribe, update, set } = logsWritable; // Keep destructuring for internal use
    
    // Maps for efficient lookups - separate from the store to avoid triggering reactivity
    let sequenceIndex: LogIndex = new Map();
    let isLogsSorted = true;

    // State tracking
    let highestSequence = 0;
    let lastAddTime = 0;
    let pendingBatch: LogMessage[] = [];
    let processingBatch = false;
    let batchProcessTimer: number | null = null; // Use number for browser setTimeout

    /**
     * Formats a log message with proper time format and additional metadata
     */
    function formatLog(rawLog: any): LogMessage | null {
        try {
            // Parse the log if it's a string
            const logData: LogMessage = typeof rawLog === 'string' ? JSON.parse(rawLog) : rawLog;
            
            // Extract the original timestamp
            const originalTime = logData.time || new Date().toISOString();
            
            // Format display time (HH:MM:SS) - no milliseconds
            const displayTime = new Date(originalTime).toLocaleTimeString('en-US', {
                hour12: false,
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit'
            });
            
            // Use unix timestamp if available, otherwise parse ISO string
            let unixTime = logData._unix_time;
            if (unixTime === undefined) {
                // Convert to milliseconds for sorting if not provided by backend
                unixTime = new Date(originalTime).getTime();
            }
            
            // Use sequence from backend or generate
            const sequence = logData._sequence !== undefined 
                ? logData._sequence 
                : highestSequence + 1;
                
            if (sequence > highestSequence) {
                highestSequence = sequence;
            }
            
            // Return normalized log entry with metadata
            return {
                ...logData,
                time: displayTime,
                _original_time: originalTime,
                _unix_time: unixTime,
                _sequence: sequence,
                _visible: false,
                _height: 0
            };
        } catch (error) {
            logger.error('store/logStore', "Error processing log", { error, rawLog });
            return null;
        }
    }

    /**
     * TypeScript implementation of mergeInsertLogs
     * Original function remains as the TypeScript implementation
     */
    function mergeInsertLogsTS(existingLogs: LogMessage[], newLogs: LogMessage[]): LogMessage[] {
        // Short-circuit for empty arrays
        if (newLogs.length === 0) return existingLogs;
        if (existingLogs.length === 0) return newLogs;
        
        // Sort the new logs batch (typically small) by unix time
        newLogs.sort((a, b) => {
            const timeA = a._unix_time || 0;
            const timeB = b._unix_time || 0;
            
            if (timeA !== timeB) {
                return timeA - timeB;
            }
            
            // If times match, use sequence as tie-breaker
            return (a._sequence || 0) - (b._sequence || 0);
        });
        
        // If existing logs aren't sorted, sort them once
        let targetLogs = existingLogs;
        if (!isLogsSorted) {
            targetLogs = [...existingLogs].sort((a, b) => {
                const timeA = a._unix_time || 0;
                const timeB = b._unix_time || 0;
                
                if (timeA !== timeB) {
                    return timeA - timeB;
                }
                
                return (a._sequence || 0) - (b._sequence || 0);
            });
            isLogsSorted = true;
        }
        
        // Merge the two sorted arrays
        const result: LogMessage[] = [];
        let i = 0, j = 0;
        
        while (i < targetLogs.length && j < newLogs.length) {
            const timeA = targetLogs[i]._unix_time || 0;
            const timeB = newLogs[j]._unix_time || 0;
            
            if (timeA <= timeB) {
                result.push(targetLogs[i]);
                i++;
            } else {
                result.push(newLogs[j]);
                j++;
            }
        }
        
        // Add any remaining entries
        while (i < targetLogs.length) {
            result.push(targetLogs[i]);
            i++;
        }
        
        while (j < newLogs.length) {
            result.push(newLogs[j]);
            j++;
        }
        
        return result;
    }

    /**
     * WebAssembly-Enhanced Log Processing
     *
     * This module integrates WebAssembly optimization for performance-critical
     * log processing operations, particularly the `mergeInsertLogs` function
     * which handles the chronological ordering and merging of log entries.
     *
     * Integration Architecture:
     *
     * 1. The original TypeScript implementation remains as `mergeInsertLogsTS`
     * 2. A wrapper function `mergeInsertLogs` decides whether to use WebAssembly:
     *    - Based on log volume (small datasets use TypeScript)
     *    - Based on memory availability (avoids WebAssembly OOM errors)
     *    - Based on performance metrics (adaptive thresholds)
     * 3. Performance metrics are collected to optimize future decisions
     * 4. Error handling ensures graceful fallback to TypeScript
     *
     * This approach maintains 100% compatibility while providing significant
     * performance improvements for large log volumes.
     */

    /**
     * Helper function to ensure objects are serializable for WebAssembly
     * Converts JavaScript Maps to plain objects/arrays
     * This is needed because serde_wasm_bindgen can't deserialize JS Map objects directly
     */
    function ensureSerializable<T>(data: T[]): any[] {
      if (!data) return [];
      if (!Array.isArray(data)) {
        logger.warn('store/logStore', 'Non-array passed to ensureSerializable');
        return [];
      }

      logger.trace('store/logStore', 'Serializing for WASM', {
        count: data.length,
        sampleType: data.length > 0 ? typeof data[0] : 'none'
      });

      const convertMapToObject = (map: any) => {
        const obj: Record<string, any> = {};
        try {
          // Convert Map to plain object
          if (map instanceof Map) {
            map.forEach((value, key) => {
              obj[key] = value;
            });
          } else if (typeof map.forEach === 'function') {
            // Fallback for Map-like objects
            map.forEach((value: any, key: string) => {
              obj[key] = value;
            });
          } else {
            // Last resort: try to access entries method
            try {
              const entries = Array.from((map as any).entries());
              for (const [key, value] of entries) {
                obj[key] = value;
              }
            } catch (entriesErr) {
              logger.warn('store/logStore', 'Failed to convert Map using entries()', { error: String(entriesErr) });
              // Just return the object as-is - let the WASM error show us what's happening
              return obj;
            }
          }

          return obj;
        } catch (err) {
          logger.warn('store/logStore', 'Error converting Map', { error: String(err) });
          return {}; // Return empty object as fallback
        }
      };

      const deepCleanObject = (obj: any): any => {
        if (obj === null || obj === undefined) return obj;

        // Check if it's a primitive (not an object)
        if (typeof obj !== 'object') return obj;

        // Check if it's a Map
        if (Object.prototype.toString.call(obj) === '[object Map]') {
          return convertMapToObject(obj);
        }

        // Handle arrays recursively
        if (Array.isArray(obj)) {
          return obj.map(item => deepCleanObject(item));
        }

        // Handle regular objects by recursively cleaning all properties
        const result: Record<string, any> = {};
        Object.keys(obj).forEach(key => {
          const value = obj[key];
          if (value !== null && typeof value === 'object') {
            if (Object.prototype.toString.call(value) === '[object Map]') {
              result[key] = convertMapToObject(value);
            } else {
              result[key] = deepCleanObject(value);
            }
          } else {
            result[key] = value;
          }
        });

        return result;
      };

      return data.map(item => deepCleanObject(item));
    }
    // Updated mergeInsertLogs from Phase 1.2 and Phase 2.2
    function mergeInsertLogs(existingLogs: LogMessage[], newLogs: LogMessage[]): LogMessage[] {
      // Track operation in WASM state
      trackOperation('mergeInsertLogs');

      const totalLogCount = existingLogs.length + newLogs.length;

      // Early return for empty arrays with proper typing
      if (newLogs.length === 0) return existingLogs;
      if (existingLogs.length === 0) return newLogs;

      // Get current settings - add this line
      const $settings = get(settings);
      const $userActivityStateData = get(userActivityState);

      // Skip WASM if user is AFK to save CPU
      if ($userActivityStateData.state === 'afk') {
        logger.trace('store/logStore', 'Skipping WASM merge - user is AFK', { logCount: totalLogCount });
        return mergeInsertLogsTS(existingLogs, newLogs);
      }

      // IMPROVEMENT #1: Check if operation is forced first
      // This ensures user preferences take precedence over automatic checks
      if ($settings.forceWasmMode === 'enabled' && isWasmEnabled() && !isOperationBlacklisted('mergeInsertLogs')) {
        try {
          const wasmModule = getWasmModule();
          if (!wasmModule) {
            throw new Error("WebAssembly module not available");
          }

          // Use WebAssembly implementation with the serializable helper
          const serialized = serializeLogsForWasm([...existingLogs, ...newLogs]);

          // We need to use our local ensureSerializable function to prevent Maps
          const serializedExisting = ensureSerializable(serialized.data.slice(0, existingLogs.length));
          const serializedNew = ensureSerializable(serialized.data.slice(existingLogs.length));

          // Log what we're sending for debugging
          logger.trace('store/logStore', 'Forced WASM mode - sending data to merge_insert_logs', {
            existingType: Object.prototype.toString.call(serializedExisting),
            newType: Object.prototype.toString.call(serializedNew),
            hasMap: serializedExisting.some(item =>
              item && Object.prototype.toString.call(item) === '[object Map]'
            ) || serializedNew.some(item =>
              item && Object.prototype.toString.call(item) === '[object Map]'
            )
          });

          const wasmStartTime = performance.now();
          const result = wasmModule.merge_insert_logs(
            serializedExisting,
            serializedNew
          );
          const wasmEndTime = performance.now();
          const deserialized = deserializeLogsFromWasm(result);
          
          // Track performance
          updatePerformanceMetrics(
            wasmEndTime - wasmStartTime, 0, totalLogCount, 'mergeInsertLogs',
            serialized.time, deserialized.time
          );
          
          return deserialized.logs as LogMessage[];
        } catch (error: any) {
          // Still handle errors properly
          handleWasmError(error, 'mergeInsertLogs', { logCount: totalLogCount });
          return mergeInsertLogsTS(existingLogs, newLogs);
        }
      }

      // Only check blacklist after force settings
      if (isOperationBlacklisted('mergeInsertLogs')) {
        logger.info('store/logStore', `Using TypeScript fallback for blacklisted operation (mergeInsertLogs)`);
        return mergeInsertLogsTS(existingLogs, newLogs);
      }

      // Original memory and size checks only apply in auto mode
      if ($settings.forceWasmMode !== 'disabled') {
        // Check memory availability with simplified boolean check
        const memoryAvailable = checkMemoryAvailability(totalLogCount);
        if (!memoryAvailable) {
          logger.debug('store/logStore', `Using TypeScript fallback due to memory constraints`, { logCount: totalLogCount });
          return mergeInsertLogsTS(existingLogs, newLogs);
        }

        // Check if we should use WebAssembly based on adaptive threshold and size
        if (shouldUseWasm(totalLogCount, 'mergeInsertLogs')) {
          // <<< INSERTED WASM LOGIC BLOCK >>>
          try {
            const wasmModule = getWasmModule();
            if (!wasmModule || typeof wasmModule.merge_insert_logs !== 'function') {
              throw new WasmOperationError('WebAssembly module not properly initialized', 'mergeInsertLogs', {
                moduleAvailable: !!wasmModule,
                functionAvailable: !!wasmModule && typeof wasmModule.merge_insert_logs === 'function'
              });
            }

            // Optimized serialization (Phase 2.2)
            // Note: The serializeLogsForWasm function currently just prepares the data structure.
            // The actual heavy serialization is done by wasm-bindgen during the call.
            // We measure the JS prep time here.
            const serialized = serializeLogsForWasm([...existingLogs, ...newLogs]); // Pass combined for potential optimization

            // Measure WebAssembly execution time
            const wasmStartTime = performance.now();

            // Use optimized approach based on log volume (Phase 2.2)
            let result;

            // For very large log sets, use the batch merge approach
            if (totalLogCount > 10000) {
              logger.debug('store/logStore', 'Using pre-sort strategy for very large merge', { logCount: totalLogCount });
              // Sort the arrays first in JS to potentially reduce WebAssembly workload if WASM sort is slow
              const sortedExisting = [...existingLogs].sort((a, b) => (a._unix_time || 0) - (b._unix_time || 0));
              const sortedNew = [...newLogs].sort((a, b) => (a._unix_time || 0) - (b._unix_time || 0));

              // Now merge the pre-sorted arrays in WASM - ensure they're serializable
              const serializedExisting = ensureSerializable(sortedExisting);
              const serializedNew = ensureSerializable(sortedNew);
              result = wasmModule.merge_insert_logs(serializedExisting, serializedNew);

              // Log what we're sending to debug the Map issue
              logger.trace('store/logStore', 'Sending data to WASM merge function', {
                existingType: Object.prototype.toString.call(serializedExisting),
                newType: Object.prototype.toString.call(serializedNew)
              });
            }
            // For normal sized log sets, use standard WASM approach
            else {
              // Pass the original arrays but ensure they're serializable
              const serializedExisting = ensureSerializable(serialized.data.slice(0, existingLogs.length));
              const serializedNew = ensureSerializable(serialized.data.slice(existingLogs.length));

              // Log what we're sending to debug the Map issue
              logger.trace('store/logStore', 'Sending data to WASM merge function', {
                existingType: Object.prototype.toString.call(serializedExisting),
                newType: Object.prototype.toString.call(serializedNew)
              });

              result = wasmModule.merge_insert_logs(serializedExisting, serializedNew);
            }

            const wasmEndTime = performance.now();
            const wasmTime = wasmEndTime - wasmStartTime;

            // Optimized deserialization (Phase 2.2)
            // Measures JS-side processing time after receiving data from WASM.
            const deserialized = deserializeLogsFromWasm(result);

            // Add diagnostic logging to examine the deserialized result
            logger.trace('store/logStore', 'WASM merge result analysis', {
              resultLength: deserialized.logs.length,
              hasItems: deserialized.logs.length > 0,
              firstItem: deserialized.logs.length > 0 ? JSON.stringify(deserialized.logs[0]).slice(0, 100) + '...' : 'none',
              // If we have items, check their structure
              hasExpectedFields: deserialized.logs.length > 0 ?
                deserialized.logs[0].hasOwnProperty('level') &&
                deserialized.logs[0].hasOwnProperty('message') &&
                deserialized.logs[0].hasOwnProperty('time') : false,
              // Sample of fields if available
              fields: deserialized.logs.length > 0 ? Object.keys(deserialized.logs[0]).join(', ') : 'none'
            });

            // Record successful operation (Phase 1.2)
            clearOperationErrorCount('mergeInsertLogs');

            // Update metrics with all timing information (Phase 2.2)
            updatePerformanceMetrics(
              wasmTime,
              0, // No TS comparison here by default
              totalLogCount,
              'mergeInsertLogs',
              serialized.time, // JS serialization prep time
              deserialized.time // JS deserialization processing time
            );

            // Occasionally benchmark TypeScript for comparison (5% chance) (Phase 2.2)
            let tsTime = 0;
            if (Math.random() < 0.05) {
              const tsStartTime = performance.now();
              const tsResult = mergeInsertLogsTS(existingLogs, newLogs); // Run TS version
              const tsEndTime = performance.now();
              tsTime = tsEndTime - tsStartTime;

              // Log comparison (CHANGED FROM INFO to TRACE)
              logger.trace('store/logStore', `Merge performance comparison for ${totalLogCount} logs`, {
                wasmTime: wasmTime.toFixed(2) + 'ms',
                tsTime: tsTime.toFixed(2) + 'ms',
                serializationTime: serialized.time.toFixed(2) + 'ms',
                deserializationTime: deserialized.time.toFixed(2) + 'ms',
                totalWasmOverhead: (wasmTime + serialized.time + deserialized.time).toFixed(2) + 'ms',
                speedup: tsTime > 0 && wasmTime > 0 ? (tsTime / wasmTime).toFixed(2) + 'x' : 'N/A',
                netSpeedup: tsTime > 0 && (wasmTime + serialized.time + deserialized.time) > 0 ? (tsTime / (wasmTime + serialized.time + deserialized.time)).toFixed(2) + 'x' : 'N/A',
                serializationOptimization: serialized.optimization
              });

              // Update TypeScript comparison metrics specifically
              updatePerformanceMetrics(
                wasmTime, // Pass wasmTime again
                tsTime,   // Pass the measured tsTime
                totalLogCount,
                'mergeInsertLogs',
                serialized.time,
                deserialized.time
              );
            }

            return deserialized.logs as LogMessage[];
          } catch (error: any) {
            // Use the enhanced error handler (Phase 1.2)
            handleWasmError(error, 'mergeInsertLogs', {
              logCount: totalLogCount,
              existingLogsLength: existingLogs.length,
              newLogsLength: newLogs.length
            });

            // Fall back to TypeScript implementation
            return mergeInsertLogsTS(existingLogs, newLogs);
          }
          //
        } else { // Add an else block for clarity
           //logger.debug('store/logStore', `Using TypeScript for mergeInsertLogs (${totalLogCount} logs) based on adaptive decision.`);
           return mergeInsertLogsTS(existingLogs, newLogs);
        }
      }
      
      // Default fallback to TypeScript
      return mergeInsertLogsTS(existingLogs, newLogs);
    }


    /**
     * Rebuild the sequence index map for O(1) lookups
     */
    function rebuildIndex(logs: LogMessage[]): void {
        sequenceIndex.clear();
        logs.forEach((log, index) => {
            if (log._sequence !== undefined) {
                sequenceIndex.set(log._sequence, index);
            }
        });
    }
    
    /**
     * Add a single log with batching support
     */
    function addLog(rawLog: any) {
        const formattedLog = formatLog(rawLog);
        if (!formattedLog) return;
        
        // Add to pending batch
        pendingBatch.push(formattedLog);
        
        // Process batch immediately if:
        // 1. It's been over 50ms since last add (terminal activity)
        // 2. Batch size is over 10 (moderate activity - short enough batches)
        const now = Date.now();
        if (now - lastAddTime > 50 || pendingBatch.length > 10) {
            processLogBatch();
        } else if (!batchProcessTimer) {
            // Schedule a batch process if none is scheduled (max delay 16ms)
            batchProcessTimer = window.setTimeout(processLogBatch, 16); // Use window.setTimeout
        }
        
        lastAddTime = now;
    }
    
    /**
     * Efficiently add multiple logs at once with proper ordering
     */
    function addLogBatch(logBatch: any[]) {
        if (!logBatch || !logBatch.length) return;
        
        // Format each log and add to pending batch
        const formattedLogs = logBatch
            .map(formatLog)
            .filter((log): log is LogMessage => log !== null);
        
        pendingBatch.push(...formattedLogs);
        
        // Process immediately
        processLogBatch();
    }
    
    /**
     * Process accumulated logs in a batch for efficiency
     * UPDATED: No longer caps logs at maxEntries - keeps all logs
     */
    function processLogBatch() {
        if (processingBatch || pendingBatch.length === 0) return;
        
        processingBatch = true;
        if (batchProcessTimer) {
            clearTimeout(batchProcessTimer);
            batchProcessTimer = null;
        }
        
        // Make a copy of pending batch and clear it
        const batchToProcess = [...pendingBatch];
        pendingBatch = [];
        
        // Update the store - NO longer capping logs
        update(logs => {
            // Use the merged implementation that can leverage WebAssembly
            const mergedLogs = mergeInsertLogs(logs, batchToProcess); // Calls the new wrapper function
            
            // Rebuild index
            rebuildIndex(mergedLogs);
            
            return mergedLogs;
        });
        
        processingBatch = false;
        
        // If more logs accumulated during processing, schedule another process
        if (pendingBatch.length > 0) {
            batchProcessTimer = window.setTimeout(processLogBatch, 0); // Use window.setTimeout
        }
    }

    /**
     * Clear all logs
     */
    function clearLogs() {
        sequenceIndex.clear();
        highestSequence = 0;
        lastAddTime = 0;
        pendingBatch = [];
        
        if (batchProcessTimer) {
            clearTimeout(batchProcessTimer);
            batchProcessTimer = null;
        }
        
        set([]);
    }
    
    /**
     * Get a log by its sequence number
     */
    function getLogBySequence(sequence: number): LogMessage | undefined {
        const currentLogs = get(logsWritable); // Use get() on the store
        const index = sequenceIndex.get(sequence);
        return index !== undefined ? currentLogs[index] : undefined;
    }
    
    /**
     * Get the index of a log by its sequence
     */
    function getLogIndexBySequence(sequence: number): number | undefined {
        return sequenceIndex.get(sequence);
    }
    
    /**
     * Check if any logs are visible in a given viewport range
     * This is used for virtualization optimization
     */
    function hasVisibleLogs(startIndex: number, endIndex: number): boolean {
        const currentLogs = get(logsWritable); // Use get() on the store
        for (let i = startIndex; i <= endIndex && i < currentLogs.length; i++) {
            if (currentLogs[i]._visible) return true;
        }
        return false;
    }
    
    /**
     * Mark logs as visible in viewport
     */
    function setLogsVisible(startIndex: number, endIndex: number, visible: boolean = true) {
        update(logs => {
            // Create a new array to avoid direct mutation if necessary, though Svelte might handle this
            const newLogs = [...logs]; 
            for (let i = startIndex; i <= endIndex && i < newLogs.length; i++) {
                if (newLogs[i]) { // Check if log exists at index
                   newLogs[i] = { ...newLogs[i], _visible: visible };
                }
            }
            return newLogs;
        });
    }
    
    function hasPendingLogs(): boolean {
        return pendingBatch.length > 0;
    }
    
    // Create derived stores for filtered logs by level - pass the store itself
    const errorLogs = derived(logsWritable, ($logs) =>
        $logs.filter(log => log.level?.toUpperCase() === 'ERROR')
    );
    
    const warnLogs = derived(logsWritable, ($logs) =>
        $logs.filter(log => log.level?.toUpperCase() === 'WARN')
    );
    
    const infoLogs = derived(logsWritable, ($logs) =>
        $logs.filter(log => log.level?.toUpperCase() === 'INFO')
    );
    
    const debugLogs = derived(logsWritable, ($logs) =>
        $logs.filter(log => log.level?.toUpperCase() === 'DEBUG')
    );
    
    // NEW: Derived store to check if logs exceed max entries - pass the stores
    const exceededMaxEntries = derived([logsWritable, settings], ([$logs, $settings]) => {
        // Use the dedicated virtualization threshold with fallback to 2000
        const threshold = ($settings?.logViewerVirtualizationThreshold !== undefined)
            ? $settings.logViewerVirtualizationThreshold
            : 2000;
            
        return $logs.length > threshold;
    });

    // Return the public API
    return {
        subscribe,
        addLog,
        addLogBatch,
        clearLogs,
        getLogBySequence,
        getLogIndexBySequence,
        hasVisibleLogs,
        setLogsVisible,
        hasPendingLogs,
        
        // Derived stores for log levels
        errorLogs,
        warnLogs,
        infoLogs,
        debugLogs,
        
        // Derived store to check if logs exceed max entries
        exceededMaxEntries
    };
}

export const logStore = createLogStore();