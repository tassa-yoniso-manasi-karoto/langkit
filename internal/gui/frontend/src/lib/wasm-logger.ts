// src/lib/wasm-logger.ts - Simplified version
export enum WasmLogLevel {
  TRACE = -1,  // Add TRACE level below DEBUG
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
  // Add throttling mechanism
  private throttleMap: Map<string, {count: number, lastTime: number}> = new Map();
  private throttleInterval: number = 60000; // 1 minute throttle window
 
  private isRoutineOperation(component: string, message: string): boolean {
    const routineKeywords = [
      'threshold', 'decision', 'using', 'memory check',
      'metrics', 'updated', 'loaded', 'initialized',
      'performance', 'operation', 'maintenance'
    ];
 
    // Check component and message against keywords
    const lowerComponent = component.toLowerCase();
    const lowerMessage = message.toLowerCase();
 
    return routineKeywords.some(keyword =>
      lowerComponent.includes(keyword) ||
      lowerMessage.includes(keyword)
    );
  }
 
  log(level: WasmLogLevel, component: string, message: string, metrics?: Record<string, any>, operation?: string) {
    // Force routine operations to TRACE level (Feedback Step 6)
    if (level > WasmLogLevel.TRACE && this.isRoutineOperation(component, message)) {
      // Exception: Don't downgrade WARN/ERROR/CRITICAL if they contain routine keywords
      // This prevents masking actual issues that happen during routine checks.
      if (level < WasmLogLevel.WARN) {
          level = WasmLogLevel.TRACE;
      }
    }
 
    // Skip excessive logging for trace and debug in production mode
    if (level <= WasmLogLevel.DEBUG && !this.isDevMode()) {
      // Only log 1 out of every 50 trace/debug messages in production
      if (Math.random() > 0.02) return;
    }

    // Create a key for throttling similar logs
    const throttleKey = `${level}:${component}:${message.substring(0, 50)}`;

    // Check if this log should be throttled
    const now = Date.now();
    const throttleInfo = this.throttleMap.get(throttleKey);

    if (throttleInfo) {
      // If within throttle window, increment count but don't log
      if (now - throttleInfo.lastTime < this.throttleInterval) {
        throttleInfo.count++;
        this.throttleMap.set(throttleKey, throttleInfo);
        return;
      } else {
        // If outside window, log with summary of throttled messages (if any were throttled)
        if (throttleInfo.count > 1) {
          const originalLevel = level;
          // Use TRACE level for throttled message summaries (unless ERROR/CRITICAL)
          if (level < WasmLogLevel.ERROR) {
            level = WasmLogLevel.TRACE;
          }

          this.emitLog(
            level,
            component,
            `${message} (${throttleInfo.count} similar messages throttled in last minute)`,
            metrics,
            operation
          );
        } else {
          this.emitLog(level, component, message, metrics, operation);
        }
        // Reset throttle info
        this.throttleMap.set(throttleKey, {count: 1, lastTime: now});
      }
    } else {
      // First occurrence, log normally and start tracking
      this.emitLog(level, component, message, metrics, operation);
      this.throttleMap.set(throttleKey, {count: 1, lastTime: now});
    }
  }

  private emitLog(level: WasmLogLevel, component: string, message: string, metrics?: Record<string, any>, operation?: string) {
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
      case WasmLogLevel.TRACE: // Add TRACE case
        // Use console.debug for TRACE to avoid potential browser differences with console.trace
        console.debug(prefix, entry.message, entry.metrics || '');
        break;
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

  private isDevMode(): boolean {
    // Check if in development mode
    // TODO: Use a more robust check if possible, e.g., environment variable set during build
    // For now, using the provided example:
    return (window as any).__LANGKIT_VERSION === 'dev';
  }

  // Get all logs for debug purposes
  getAllLogs(): WasmLogEntry[] {
    return [...this.logs];
  }

  // Clear logs
  clearLogs() {
    this.logs = [];
    this.throttleMap.clear(); // Also clear the throttle map
  }
}

export const wasmLogger = new WasmLogger();