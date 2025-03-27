import { writable, get, derived } from 'svelte/store';
import { settings } from './stores';

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
    const { subscribe, update, set } = writable<LogMessage[]>([]);
    
    // Maps for efficient lookups - separate from the store to avoid triggering reactivity
    let sequenceIndex: LogIndex = new Map();
    let isLogsSorted = true;

    // State tracking
    let highestSequence = 0;
    let lastAddTime = 0;
    let pendingBatch: LogMessage[] = [];
    let processingBatch = false;
    let batchProcessTimer: NodeJS.Timeout | null = null;

    /**
     * Formats a log message with proper time format and additional metadata
     */
    function formatLog(rawLog: any): LogMessage | null {
        try {
            // Parse the log if it's a string
            const logData: LogMessage = typeof rawLog === 'string' ? JSON.parse(rawLog) : rawLog;
            
            // Extract the original timestamp
            const originalTime = logData.time || new Date().toISOString();
            
            // Format display time (HH:MM:SS)
            const displayTime = new Date(originalTime).toLocaleTimeString('en-US', {
                hour12: false,
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit',
                fractionalSecondDigits: 3
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
     * Efficiently merge-inserts a batch of logs into the existing sorted array
     * This is much faster than doing a full sort on each update
     * O(n+m) complexity instead of O((n+m)log(n+m))
     */
    function mergeInsertLogs(existingLogs: LogMessage[], newLogs: LogMessage[]): LogMessage[] {
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
            batchProcessTimer = setTimeout(processLogBatch, 16);
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
        
        // Update the store
        update(logs => {
            // Get settings for max entries
            const maxEntries = get(settings).maxLogEntries || 10000;
            
            // Merge-insert the logs
            const mergedLogs = mergeInsertLogs(logs, batchToProcess);
            
            // Cap logs if needed
            const cappedLogs = mergedLogs.length > maxEntries
                ? mergedLogs.slice(-maxEntries)
                : mergedLogs;
            
            // Rebuild index
            rebuildIndex(cappedLogs);
            
            return cappedLogs;
        });
        
        processingBatch = false;
        
        // If more logs accumulated during processing, schedule another process
        if (pendingBatch.length > 0) {
            batchProcessTimer = setTimeout(processLogBatch, 0);
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
        return subscribe(logs => {
            const index = sequenceIndex.get(sequence);
            return index !== undefined ? logs[index] : undefined;
        });
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
        return subscribe(logs => {
            for (let i = startIndex; i <= endIndex && i < logs.length; i++) {
                if (logs[i]._visible) return true;
            }
            return false;
        });
    }
    
    /**
     * Mark logs as visible in viewport
     */
    function setLogsVisible(startIndex: number, endIndex: number, visible: boolean = true) {
        update(logs => {
            for (let i = startIndex; i <= endIndex && i < logs.length; i++) {
                logs[i]._visible = visible;
            }
            return logs;
        });
    }
    
    // Create derived stores for filtered logs by level
    const errorLogs = derived(subscribe, ($logs) => 
        $logs.filter(log => log.level?.toUpperCase() === 'ERROR')
    );
    
    const warnLogs = derived(subscribe, ($logs) => 
        $logs.filter(log => log.level?.toUpperCase() === 'WARN')
    );
    
    const infoLogs = derived(subscribe, ($logs) => 
        $logs.filter(log => log.level?.toUpperCase() === 'INFO')
    );
    
    const debugLogs = derived(subscribe, ($logs) => 
        $logs.filter(log => log.level?.toUpperCase() === 'DEBUG')
    );

    return {
        subscribe,
        addLog,
        addLogBatch,
        clearLogs,
        getLogBySequence,
        getLogIndexBySequence,
        hasVisibleLogs,
        setLogsVisible,
        // Derived stores for each log level
        errorLogs,
        warnLogs,
        infoLogs,
        debugLogs
    };
}

export const logStore = createLogStore();