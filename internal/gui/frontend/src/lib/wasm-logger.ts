/*
    ⚠️ DEPRECATED, USE logger.ts INSTEAD ⚠️

*/
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
  
  // Add aggressive throttling for common log categories
  private highVolumeCategories = new Set(['threshold', 'memory', 'performance', 'adaptive']);
 
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
    // Skip high volume logs in production by default
    if (level <= WasmLogLevel.TRACE &&
        this.highVolumeCategories.has(component) &&
        !this.isDevMode()) {
      // Only log 0.1% of trace logs in production for high-volume categories
      if (Math.random() > 0.001) return;
    }
    
    // For DEBUG level, also reduce volume in production
    if (level === WasmLogLevel.DEBUG &&
        this.highVolumeCategories.has(component) &&
        !this.isDevMode()) {
      // Only log 1% of debug logs in production for high-volume categories
      if (Math.random() > 0.01) return;
    }

    // Create a key for throttling similar logs - with more specific key to reduce collisions
    const msgSignature = message.split(' ').slice(0, 5).join(' '); // Use first 5 words
    const throttleKey = `${level}:${component}:${msgSignature}`;

    // Increase throttle interval for high volume categories
    const throttleInterval = this.highVolumeCategories.has(component) ?
      this.throttleInterval * 5 : this.throttleInterval;

    // Rest of the existing throttling logic...
    const now = Date.now();
    const throttleInfo = this.throttleMap.get(throttleKey);

    if (throttleInfo) {
      // If within throttle window, increment count but don't log
      if (now - throttleInfo.lastTime < throttleInterval) {
        throttleInfo.count++;
        this.throttleMap.set(throttleKey, throttleInfo);
        return;
      } else {
        // Only summarize if many messages were throttled
        if (throttleInfo.count > 10) {
          // Use lower level for throttled summaries
          const summaryLevel = level < WasmLogLevel.ERROR ?
            WasmLogLevel.TRACE : level;
          
          this.emitLog(
            summaryLevel,
            component,
            `${message} (${throttleInfo.count} similar messages throttled in last ${Math.round(throttleInterval/1000)}s)`
          );
        } else {
          this.emitLog(level, component, message, metrics, operation);
        }
        // Reset throttle info with 5x longer interval for high volume categories
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