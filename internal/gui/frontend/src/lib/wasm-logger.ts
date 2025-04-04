// src/lib/wasm-logger.ts - Simplified version
export enum WasmLogLevel {
  DEBUG = 0,
  INFO = 1,
  WARN = 2,
  ERROR = 3,
  CRITICAL = 4
}

export interface WasmLogEntry {
  level: WasmLogLevel;
  component: string;  // 'init', 'memory', 'process', etc.
  message: string;
  timestamp: number;
  metrics?: Record<string, any>;  // Optional performance metrics
  operation?: string;  // Operation being performed
}

class WasmLogger {
  private logs: WasmLogEntry[] = [];
  private bufferSize: number = 1000;
  
  log(level: WasmLogLevel, component: string, message: string, metrics?: Record<string, any>, operation?: string) {
    const entry: WasmLogEntry = {
      level,
      component,
      message,
      timestamp: Date.now(),
      metrics,
      operation
    };
    
    // Add to internal buffer with size limit
    this.logs.push(entry);
    if (this.logs.length > this.bufferSize) {
      this.logs.shift(); // Remove oldest entry
    }
    
    // Local console output with appropriate level
    this.consoleOutput(entry);
    
    // Send to backend via Wails
    this.relayToCrashReporter(entry);
  }
  
  private consoleOutput(entry: WasmLogEntry) {
    const prefix = `[WASM:${entry.component}]`;
    switch (entry.level) {
      case WasmLogLevel.DEBUG:
        console.debug(prefix, entry.message, entry.metrics || '');
        break;
      case WasmLogLevel.INFO:
        console.info(prefix, entry.message, entry.metrics || '');
        break;
      case WasmLogLevel.WARN:
        console.warn(prefix, entry.message, entry.metrics || '');
        break;
      case WasmLogLevel.ERROR:
      case WasmLogLevel.CRITICAL:
        console.error(prefix, entry.message, entry.metrics || '');
        break;
    }
  }
  
  private relayToCrashReporter(entry: WasmLogEntry) {
    try {
      // Call backend method to store log in crash reporter
      // Use type assertion for window.go
      (window as any).go.gui.App.RecordWasmLog(JSON.stringify(entry)); 
    } catch (e) {
      console.error("Failed to relay log to crash reporter:", e);
    }
  }
  
  // Get all logs for debug purposes
  getAllLogs(): WasmLogEntry[] {
    return [...this.logs];
  }
  
  // Clear logs
  clearLogs() {
    this.logs = [];
  }
}

export const wasmLogger = new WasmLogger();