import { writable } from 'svelte/store';
import { settings } from './stores';
import { get } from 'svelte/store';

export interface LogMessage {
    level: string;
    message: string;
    time: string;
    behavior?: string;
    // Allow any additional fields for structured logging
    [key: string]: any;
}

function createLogStore() {
    const { subscribe, set, update } = writable<LogMessage[]>([]);

    function addLog(rawLog: any) {
        try {
            const logData: LogMessage = typeof rawLog === 'string' ? JSON.parse(rawLog) : rawLog;
            
            // Format the time
            const timeStr = new Date(logData.time).toLocaleTimeString('en-US', {
                hour12: false,
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit'
            });
            logData.time = timeStr;

            update(logs => {
                const maxEntries = get(settings).maxLogEntries;
                
                // If we've reached the maximum, remove the oldest entries
                if (logs.length >= maxEntries) {
                    logs = logs.slice(-(maxEntries - 1));
                }
                
                // Add the new log
                return [...logs, logData];
            });
        } catch (error) {
            console.error("Error processing log:", error);
            console.error("Raw log data:", rawLog);
        }
    }

    function clearLogs() {
        set([]);
    }

    return {
        subscribe,
        addLog,
        clearLogs
    };
}

export const logStore = createLogStore();