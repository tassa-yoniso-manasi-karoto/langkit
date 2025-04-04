import { writable, get, derived } from 'svelte/store';
import { settings } from './stores';
import { 
  isWasmEnabled, 
  getWasmModule, 
  getWasmSizeThreshold,
  shouldUseWasm,
  canProcessSafely,
  handleWasmError,
  WasmOperationError,
  WasmModule
} from './wasm';
import { wasmLogger, WasmLogLevel } from './wasm-logger';
// Remove command pattern imports
// import { getCommandExecutor, TrackOperationCommand, UpdateMetricsCommand, SetErrorCommand } from './wasm-commands';
// Keep direct state function imports
import { trackOperation, updatePerformanceMetrics, setWasmError } from './wasm-state'; 

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
            console.error("Error processing log:", error, rawLog);
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
    function mergeInsertLogs(existingLogs: LogMessage[], newLogs: LogMessage[]): LogMessage[] {
        // Track operation in WASM state
        trackOperation('mergeInsertLogs');
        
        const totalLogCount = existingLogs.length + newLogs.length;
        
        // First check if we have enough memory to process with WebAssembly
        if (!canProcessSafely(totalLogCount)) {
            wasmLogger.log(
                WasmLogLevel.INFO, 
                'memory', 
                `Using TypeScript due to memory constraints for ${totalLogCount} logs`
            );
            return mergeInsertLogsTS(existingLogs, newLogs);
        }
        
        // For small datasets, use TypeScript implementation
        if (totalLogCount < getWasmSizeThreshold()) {
            wasmLogger.log(
                WasmLogLevel.DEBUG, 
                'threshold', 
                `Using TypeScript for small dataset (${totalLogCount} logs)`
            );
            return mergeInsertLogsTS(existingLogs, newLogs);
        }
        
        // Then check if we should use WebAssembly based on adaptive threshold
        if (shouldUseWasm(totalLogCount)) {
            try {
                const wasmModule = getWasmModule();
                if (!wasmModule || typeof wasmModule.merge_insert_logs !== 'function') {
                    throw new WasmOperationError('WebAssembly module not properly initialized', 'mergeInsertLogs', {
                      moduleAvailable: !!wasmModule,
                      functionAvailable: !!wasmModule && typeof wasmModule.merge_insert_logs === 'function'
                    });
                }
                
                // Measure serialization time (approximated)
                const serializeStartTime = performance.now();
                // For measurement purposes, we'd need custom code here to isolate serialization
                // This is a placeholder for the concept
                const serializeEndTime = performance.now();
                const serializationTime = serializeEndTime - serializeStartTime;
                
                // Measure WebAssembly execution time
                const wasmStartTime = performance.now();
                // Ensure logs are properly typed to avoid serialization issues
                const result = wasmModule.merge_insert_logs(
                  existingLogs as LogMessage[], 
                  newLogs as LogMessage[]
                );
                const wasmEndTime = performance.now();
                const wasmTime = wasmEndTime - wasmStartTime;
                
                // Measure deserialization time (approximated)
                const deserializeStartTime = performance.now();
                // For measurement purposes, we'd need custom code here to isolate deserialization
                const deserializeEndTime = performance.now();
                const deserializationTime = deserializeEndTime - deserializeStartTime;
                
                // Occasionally benchmark TypeScript for comparison
                let tsTime = 0;
                if (Math.random() < 0.1) {
                    const tsStartTime = performance.now();
                    mergeInsertLogsTS(existingLogs, newLogs);
                    const tsEndTime = performance.now();
                    tsTime = tsEndTime - tsStartTime;
                    
                    // Update metrics with all timing information
                    updatePerformanceMetrics(
                        wasmTime,
                        tsTime,
                        totalLogCount
                    );
                    
                    // Log detailed metrics for large operations
                    if (totalLogCount > 1000) {
                        wasmLogger.log(
                            WasmLogLevel.INFO, 
                            'performance', 
                            `Large merge operation completed`, 
                            {
                                wasmTime: wasmTime.toFixed(2),
                                tsTime: tsTime.toFixed(2),
                                speedup: tsTime > 0 ? (tsTime / wasmTime).toFixed(2) : 'N/A',
                                logCount: totalLogCount,
                                serializationTime: serializationTime.toFixed(2),
                                deserializationTime: deserializationTime.toFixed(2)
                            },
                            'mergeInsertLogs'
                        );
                    }
                }
                
                return result as LogMessage[];
            } catch (error: any) {
                // Use the centralized error handler
                handleWasmError(error, 'mergeInsertLogs', {
                    logCount: totalLogCount,
                    existingLogsLength: existingLogs.length,
                    newLogsLength: newLogs.length
                });
                
                // Fall back to TypeScript implementation
                return mergeInsertLogsTS(existingLogs, newLogs);
            }
        } else {
            // Not using WebAssembly based on adaptive decision
            return mergeInsertLogsTS(existingLogs, newLogs);
        }
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
        // TODO: Update Settings type in stores.ts
        const maxEntries = ($settings as any)?.maxLogEntries || 5000; 
        return $logs.length > maxEntries;
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