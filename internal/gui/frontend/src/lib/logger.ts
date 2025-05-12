/* IMPORTANT: make sure to specify a component whenever you use the logger
component inform from which part of the frontend was a given log emitted from
*/


export enum LogLevel {
  TRACE = -1,
  DEBUG = 0,
  INFO = 1,
  WARN = 2,
  ERROR = 3,
  CRITICAL = 4
}

export interface LogEntry {
  level: LogLevel;
  component: string;
  message: string;
  timestamp: number;
  context?: Record<string, any>;
  operation?: string;
  sessionId?: string;
  tags?: string[];
  stackTrace?: string;
}

export interface ThrottleConfig {
  enabled: boolean;
  interval: number;
  maxSimilarLogs: number;
  byComponent: Record<string, { interval: number; maxLogs: number }>;
  sampleInterval: number;
}

export interface BatchConfig {
  enabled: boolean;
  maxSize: number;
  maxWaitMs: number;
  retryCount: number;
  retryDelayMs: number;
}

export interface LoggerConfig {
  minLevel: LogLevel;
  bufferSize: number;
  throttling: ThrottleConfig;
  batching: BatchConfig;
  consoleOutput: boolean;
  captureStack: boolean;
  autoLogErrors: boolean;
  developerMode: boolean;
  highVolumeCategories: Set<string>;
  sampleRate: number;
  criticalPatterns: RegExp[];
  operationTimeout: number;
}

/**
 * Circular buffer implementation for efficient log storage
 */
class CircularBuffer<T> {
  private buffer: Array<T | null>;
  private head = 0;
  private tail = 0;
  private count = 0;
  
  constructor(private capacity: number) {
    this.buffer = new Array(capacity).fill(null);
  }
  
  push(item: T): void {
    this.buffer[this.tail] = item;
    this.tail = (this.tail + 1) % this.capacity;
    
    if (this.count === this.capacity) {
      // Buffer is full, move head
      this.head = (this.head + 1) % this.capacity;
    } else {
      this.count++;
    }
  }
  
  getAll(): T[] {
    const result: T[] = [];
    if (this.count === 0) return result;
    
    let current = this.head;
    for (let i = 0; i < this.count; i++) {
      const item = this.buffer[current];
      if (item !== null) result.push(item);
      current = (current + 1) % this.capacity;
    }
    
    return result;
  }
  
  clear(): void {
    this.buffer.fill(null);
    this.head = 0;
    this.tail = 0;
    this.count = 0;
  }
  
  get size(): number {
    return this.count;
  }
}

export class Logger {
  private logBuffer: CircularBuffer<LogEntry>;
  private throttleMap: Map<string, {
    count: number;
    lastTime: number;
    samples: string[];
  }> = new Map();
  
  private globalContext: Record<string, any> = {};
  private operationContexts: Map<string, {
    context: Record<string, any>;
    startTime: number;
    timeoutId?: number;
  }> = new Map();
  
  private activeOperation?: string;
  private sessionId: string;
  private batchMode = false;
  private batchedLogs: LogEntry[] = [];
  private batchTimer?: number;
  private timers: Map<string, number> = new Map();
  private retryQueue: Array<{entry: LogEntry, retries: number}> = [];
  private isProcessingRetryQueue = false;
  private eventListeners: Array<() => void> = [];
  
  private config: LoggerConfig = {
    minLevel: LogLevel.INFO,
    bufferSize: 500, // Reduced from 1000
    throttling: {
      enabled: true,
      interval: 60000,
      maxSimilarLogs: 5,
      byComponent: {
        ui: { interval: 30000, maxLogs: 10 },
        api: { interval: 10000, maxLogs: 3 },
        media: { interval: 60000, maxLogs: 3 }
      },
      sampleInterval: 10 // Sample every 10th message for throttle summary
    },
    batching: {
      enabled: true,
      maxSize: 20,
      maxWaitMs: 2000,
      retryCount: 3,
      retryDelayMs: 1000
    },
    consoleOutput: true,
    captureStack: true,
    autoLogErrors: true,
    developerMode: false, // Will be detected automatically
    highVolumeCategories: new Set([
      'ui', 'api', 'media', 'network', 'performance'
    ]),
    sampleRate: 0.01, // 1% sample rate for high volume logs in production
    criticalPatterns: [
      /error/i, /fail/i, /exception/i, /crash/i
    ],
    operationTimeout: 5 * 60 * 1000 // 5 minute default timeout for operations
  };
  
  /**
   * Creates a new Logger instance
   */
  constructor(config?: Partial<LoggerConfig>) {
    // Apply custom config
    if (config) {
      this.config = this.mergeConfig(this.config, config);
    }
    
    // Auto-detect developer mode if not explicitly set
    if (config?.developerMode === undefined) {
      this.config.developerMode = this.detectDeveloperMode();
    }
    
    // Initialize circular buffer
    this.logBuffer = new CircularBuffer<LogEntry>(this.config.bufferSize);
    
    // Generate session ID
    this.sessionId = this.generateSessionId();
    
    // Capture environment info for global context
    this.globalContext = {
      userAgent: navigator.userAgent,
      viewport: `${window.innerWidth}x${window.innerHeight}`,
      timestamp: Date.now(),
      sessionId: this.sessionId
    };
    
    // Add version if available
    try {
      const version = this.getAppVersion();
      if (version) {
        this.globalContext.appVersion = version;
      }
    } catch (e) {
      // Silently ignore if version detection fails
    }
    
    // Set up error listener if configured
    if (this.config.autoLogErrors) {
      this.setupErrorListener();
    }
    
    // Set up retry processing
    this.startRetryProcessing();
    
    // Set up beforeunload to flush batched logs and save pending logs
    const unloadHandler = this.handleBeforeUnload.bind(this);
    window.addEventListener('beforeunload', unloadHandler);
    this.eventListeners.push(() => {
      window.removeEventListener('beforeunload', unloadHandler);
    });
    
    this.info('logger', 'Logger initialized', { 
      developerMode: this.config.developerMode,
      minLevel: LogLevel.INFO
    });
  }
  
  /**
   * Main log method with component and level
   */
  log(level: LogLevel, component: string, message: string, context?: Record<string, any>, operation?: string): void {
    // Skip logs below minimum level
    if (level < this.config.minLevel) return;
    
    // Apply sampling for high-volume categories in production mode
    if (level <= LogLevel.DEBUG && 
        this.config.highVolumeCategories.has(component) &&
        !this.config.developerMode) {
      if (Math.random() > this.config.sampleRate) return;
    }
    
    // Create throttle key with improved signature generation
    const throttleKey = this.generateThrottleKey(level, component, message);
    
    // Check for throttling
    if (this.shouldThrottle(level, component, throttleKey, message)) {
      return;
    }
    
    // Prepare the log entry
    const entry: LogEntry = {
      level,
      component,
      message,
      timestamp: Date.now(),
      context: this.buildContext(context),
      operation: operation || this.activeOperation,
      sessionId: this.sessionId,
      tags: this.deriveTags(level, component, message)
    };
    
    // Add stack trace for errors if configured
    if (this.config.captureStack && level >= LogLevel.ERROR) {
      entry.stackTrace = this.captureStack();
    }
    
    // Handle the log entry
    if (this.batchMode && this.config.batching.enabled) {
      this.batchedLogs.push(entry);
      
      // Set up batch timer if not already running
      if (!this.batchTimer && this.config.batching.maxWaitMs > 0) {
        this.batchTimer = window.setTimeout(() => {
          this.flushBatch();
          this.batchTimer = undefined;
        }, this.config.batching.maxWaitMs);
      }
      
      // Flush if we've reached the batch size
      if (this.batchedLogs.length >= this.config.batching.maxSize) {
        this.flushBatch();
      }
    } else {
      this.processLogEntry(entry);
    }
  }
  
  /**
   * Convenience methods for different log levels
   */
  trace(component: string, message: string, context?: Record<string, any>, operation?: string): void {
    this.log(LogLevel.TRACE, component, message, context, operation);
  }
  
  debug(component: string, message: string, context?: Record<string, any>, operation?: string): void {
    this.log(LogLevel.DEBUG, component, message, context, operation);
  }
  
  info(component: string, message: string, context?: Record<string, any>, operation?: string): void {
    this.log(LogLevel.INFO, component, message, context, operation);
  }
  
  warn(component: string, message: string, context?: Record<string, any>, operation?: string): void {
    this.log(LogLevel.WARN, component, message, context, operation);
  }
  
  error(component: string, message: string, context?: Record<string, any>, operation?: string): void {
    this.log(LogLevel.ERROR, component, message, context, operation);
  }
  
  critical(component: string, message: string, context?: Record<string, any>, operation?: string): void {
    this.log(LogLevel.CRITICAL, component, message, context, operation);
  }
  
  /**
   * Log an Error object with stack trace
   */
  logError(err: Error, component: string, message?: string, context?: Record<string, any>): void {
    const msg = message || `Error: ${err.message}`;
    const ctx = { 
      ...context,
      errorType: err.name,
      errorMessage: err.message,
      stack: err.stack
    };
    
    this.log(LogLevel.ERROR, component, msg, ctx);
  }
  
  /**
   * Set global context that will be included with all logs
   */
  setGlobalContext(context: Record<string, any>): void {
    this.globalContext = { ...this.globalContext, ...context };
  }
  
  /**
   * Start a new operation context
   */
  startOperation(name: string, context?: Record<string, any>, timeout?: number): void {
    // End any existing operation first
    if (this.activeOperation) {
      this.endOperation({ status: 'interrupted', reason: 'New operation started' });
    }
    
    this.activeOperation = name;
    
    // Clear any existing operation timeout
    const existing = this.operationContexts.get(name);
    if (existing?.timeoutId) {
      window.clearTimeout(existing.timeoutId);
    }
    
    // Set timeout for operation (if enabled)
    const actualTimeout = timeout ?? this.config.operationTimeout;
    let timeoutId: number | undefined;
    
    if (actualTimeout > 0) {
      timeoutId = window.setTimeout(() => {
        if (this.activeOperation === name) {
          this.warn('operations', `Operation timed out: ${name}`, { 
            timeoutMs: actualTimeout 
          });
          this.endOperation({ status: 'timeout', timeoutMs: actualTimeout });
        } else {
          // Just remove from tracking if no longer active
          this.operationContexts.delete(name);
        }
      }, actualTimeout);
    }
    
    // Store operation context
    this.operationContexts.set(name, {
      context: context || {},
      startTime: Date.now(),
      timeoutId
    });
    
    this.info('operations', `Operation started: ${name}`, context);
  }
  
  /**
   * End the current operation
   */
  endOperation(result?: string | Record<string, any>): void {
    if (!this.activeOperation) return;
    
    const name = this.activeOperation;
    const operationData = this.operationContexts.get(name);
    
    // Clear timeout if exists
    if (operationData?.timeoutId) {
      window.clearTimeout(operationData.timeoutId);
    }
    
    // Calculate duration
    const duration = operationData 
      ? Date.now() - operationData.startTime 
      : undefined;
    
    const context = typeof result === 'string' 
      ? { result, durationMs: duration } 
      : { ...(result || {}), durationMs: duration };
      
    this.info('operations', `Operation completed: ${name}`, context);
    
    // Clean up
    this.operationContexts.delete(name);
    this.activeOperation = undefined;
  }
  
  /**
   * Start timer for performance measurement
   */
  startTimer(name: string, component?: string): void {
    const start = performance.now();
    this.timers.set(name, start);
    
    if (component) {
      this.trace(component, `Timer started: ${name}`);
    }
  }
  
  /**
   * End timer and return duration
   */
  endTimer(name: string, component?: string, logLevel: LogLevel = LogLevel.DEBUG): number {
    const start = this.timers.get(name);
    if (start === undefined) {
      this.warn('performance', `Timer "${name}" was never started`);
      return 0;
    }
    
    const end = performance.now();
    const duration = end - start;
    this.timers.delete(name);
    
    if (component) {
      this.log(logLevel, component, `Timer ${name}: ${duration.toFixed(2)}ms`, { 
        duration,
        timerName: name 
      });
    }
    
    return duration;
  }
  
  /**
   * Track user interactions
   */
  trackUserAction(action: string, details?: Record<string, any>): void {
    this.info('user', `User action: ${action}`, details);
  }
  
  /**
   * Begin batching logs (for high-volume operations)
   */
  beginBatch(): void {
    // Flush any existing batch first
    if (this.batchedLogs.length > 0) {
      this.flushBatch();
    }
    
    this.batchMode = true;
    this.batchedLogs = [];
  }
  
  /**
   * End batching and optionally flush logs
   */
  endBatch(flush: boolean = true): void {
    this.batchMode = false;
    if (flush && this.batchedLogs.length > 0) {
      this.flushBatch();
    }
    
    // Clear batch timer if exists
    if (this.batchTimer) {
      window.clearTimeout(this.batchTimer);
      this.batchTimer = undefined;
    }
  }
  
  /**
   * Flush batched logs to backend in a single request
   */
  flushBatch(): void {
    if (this.batchedLogs.length === 0) return;
    
    // Clone the batch to allow for new logs to come in
    const batch = [...this.batchedLogs];
    this.batchedLogs = [];
    
    // Clear batch timer if exists
    if (this.batchTimer) {
      window.clearTimeout(this.batchTimer);
      this.batchTimer = undefined;
    }
    
    // First, add all logs to the buffer
    for (const entry of batch) {
      // Add to internal circular buffer
      this.logBuffer.push(entry);
      
      // Output to console if enabled
      if (this.config.consoleOutput) {
        this.consoleOutput(entry);
      }
    }
    
    // Then send as a batch to backend
    this.relayBatchToBackend(batch);
  }
  
  /**
   * Clear all logs
   */
  clearLogs(): void {
    this.logBuffer.clear();
    this.throttleMap.clear();
    this.info('logger', 'Logs cleared');
  }
  
  /**
   * Get all logs for debug purposes
   */
  getAllLogs(): LogEntry[] {
    return this.logBuffer.getAll();
  }
  
  /**
   * Set minimum log level
   */
  setMinLogLevel(level: LogLevel): void {
    this.config.minLevel = level;
    this.info('logger', `Log level set to: ${this.getLogLevelName(level)}`);
  }
  
  /**
   * Get log level name as string
   */
  getLogLevelName(level: LogLevel): string {
    switch(level) {
      case LogLevel.TRACE: return 'TRACE';
      case LogLevel.DEBUG: return 'DEBUG';
      case LogLevel.INFO: return 'INFO';
      case LogLevel.WARN: return 'WARN';
      case LogLevel.ERROR: return 'ERROR';
      case LogLevel.CRITICAL: return 'CRITICAL';
      default: return 'UNKNOWN';
    }
  }
  
  /**
   * Clean up resources used by the logger
   */
  destroy(): void {
    // Clean up any active operations
    for (const [name, data] of this.operationContexts.entries()) {
      if (data.timeoutId) {
        window.clearTimeout(data.timeoutId);
      }
    }
    
    // Clear any batch timer
    if (this.batchTimer) {
      window.clearTimeout(this.batchTimer);
    }
    
    // Flush any pending logs
    if (this.batchedLogs.length > 0) {
      this.flushBatch();
    }
    
    // Remove any event listeners
    for (const cleanup of this.eventListeners) {
      cleanup();
    }
    
    this.info('logger', 'Logger destroyed');
  }
  
  /**
   * Private: Process a single log entry
   */
  private processLogEntry(entry: LogEntry): void {
    // Add to internal circular buffer
    this.logBuffer.push(entry);
    
    // Output to console if enabled
    if (this.config.consoleOutput) {
      this.consoleOutput(entry);
    }
    
    // Send to backend
    this.relayToBackend(entry);
  }
  
  /**
   * Private: Build context combining global and operation contexts
   */
  private buildContext(localContext?: Record<string, any>): Record<string, any> {
    // Start with global context
    const context = { ...this.globalContext };
    
    // Add operation context if there's an active operation
    if (this.activeOperation) {
      const opData = this.operationContexts.get(this.activeOperation);
      if (opData?.context) {
        Object.assign(context, opData.context);
        
        // Also add operation duration
        context.operationElapsedMs = Date.now() - opData.startTime;
      }
    }
    
    // Add local context if provided
    if (localContext) {
      Object.assign(context, localContext);
    }
    
    return context;
  }
  
  /**
   * Private: Output to browser console
   */
  private consoleOutput(entry: LogEntry): void {
    // Skip console output for some levels based on mode
    if (!this.config.developerMode && entry.level <= LogLevel.DEBUG) {
      return;
    }
    
    const prefix = `[${entry.component}]`;
    const context = entry.context ? entry.context : '';
    
    // Add styling based on level
    let method = 'log';
    let style = '';
    
    switch (entry.level) {
      case LogLevel.TRACE:
        method = 'debug';
        style = 'color: #8c84e8; font-weight: normal;';
        break;
      case LogLevel.DEBUG:
        method = 'debug';
        style = 'color: #84a9e8; font-weight: normal;';
        break;
      case LogLevel.INFO:
        method = 'info';
        style = 'color: #4caf50; font-weight: normal;';
        break;
      case LogLevel.WARN:
        method = 'warn';
        style = 'color: #ff9800; font-weight: bold;';
        break;
      case LogLevel.ERROR:
        method = 'error';
        style = 'color: #f44336; font-weight: bold;';
        break;
      case LogLevel.CRITICAL:
        method = 'error';
        style = 'color: #b71c1c; font-weight: bold; font-size: 1.1em;';
        break;
    }
    
    // Use styled console output
    console[method](`%c${prefix}`, style, entry.message, context);
    
    // Show stack trace separately for errors
    if (entry.stackTrace && entry.level >= LogLevel.ERROR) {
      console.groupCollapsed('Stack trace');
      console.error(entry.stackTrace);
      console.groupEnd();
    }
  }
  
  /**
   * Private: Send log to backend
   */
  private relayToBackend(entry: LogEntry): void {
    try {
      // Clone entry to avoid mutation
      const entryCopy = { ...entry };
      
      // Ensure context is serializable and limit size
      if (entryCopy.context) {
        entryCopy.context = this.sanitizeContext(entryCopy.context);
      }
      
      // Send to backend via Wails bridge
      (window as any).go.gui.App.BackendLogger(
        entry.component, 
        JSON.stringify(entryCopy)
      );
    } catch (e) {
      console.error("Failed to relay log to backend:", e);
      
      // Add to retry queue for critical logs
      if (entry.level >= LogLevel.ERROR) {
        this.retryQueue.push({ entry, retries: 0 });
      }
      
      // For non-critical logs, use sendBeacon as last resort
      else if (navigator.sendBeacon) {
        try {
          const beacon = new Blob(
            [JSON.stringify({ component: entry.component, log: entry })], 
            { type: 'application/json' }
          );
          navigator.sendBeacon('/api/logs', beacon);
        } catch (beaconErr) {
          // Last resort failed, log to console only
          console.error("Failed to send log via beacon:", beaconErr);
        }
      }
    }
  }
  
  /**
   * Private: Send batch of logs to backend in single request
   */
  private relayBatchToBackend(entries: LogEntry[]): void {
    if (entries.length === 0) return;
    
    try {
      // Use component from first entry
      const component = entries[0].component;
      
      // Clone entries to avoid mutation
      const sanitizedEntries = entries.map(entry => {
        const copy = { ...entry };
        if (copy.context) {
          copy.context = this.sanitizeContext(copy.context);
        }
        return copy;
      });
      
      // Send batch to backend
      (window as any).go.gui.App.BackendLoggerBatch(
        component,
        JSON.stringify(sanitizedEntries)
      );
    } catch (e) {
      console.error("Failed to relay batch to backend:", e);
      
      // Add critical logs to retry queue
      for (const entry of entries) {
        if (entry.level >= LogLevel.ERROR) {
          this.retryQueue.push({ entry, retries: 0 });
        }
      }
      
      // For non-critical logs, use sendBeacon as last resort
      if (navigator.sendBeacon) {
        try {
          const beacon = new Blob(
            [JSON.stringify({ batch: entries })], 
            { type: 'application/json' }
          );
          navigator.sendBeacon('/api/logs/batch', beacon);
        } catch (beaconErr) {
          // Last resort failed, already logged to console
        }
      }
    }
  }
  
  /**
   * Private: Process retry queue
   */
  private startRetryProcessing(): void {
    const processRetryQueue = async () => {
      if (this.isProcessingRetryQueue || this.retryQueue.length === 0) return;
      
      this.isProcessingRetryQueue = true;
      
      try {
        // Process retries one by one
        const item = this.retryQueue.shift();
        if (!item) {
          this.isProcessingRetryQueue = false;
          return;
        }
        
        const { entry, retries } = item;
        
        if (retries < this.config.batching.retryCount) {
          try {
            // Try to send again
            (window as any).go.gui.App.BackendLogger(
              entry.component,
              JSON.stringify(entry)
            );
          } catch (e) {
            // Put back in queue with incremented retry count
            this.retryQueue.push({ entry, retries: retries + 1 });
            
            // Wait before next retry
            await new Promise(resolve => 
              setTimeout(resolve, this.config.batching.retryDelayMs)
            );
          }
        } else {
          // Max retries reached, use sendBeacon as last resort
          if (navigator.sendBeacon) {
            try {
              const beacon = new Blob(
                [JSON.stringify({ component: entry.component, log: entry })], 
                { type: 'application/json' }
              );
              navigator.sendBeacon('/api/logs', beacon);
            } catch (beaconErr) {
              // Last resort failed, give up
            }
          }
        }
      } finally {
        this.isProcessingRetryQueue = false;
        
        // Continue processing if more items in queue
        if (this.retryQueue.length > 0) {
          setTimeout(processRetryQueue, 10);
        }
      }
    };
    
    // Start processing retry queue periodically
    setInterval(processRetryQueue, 5000);
  }
  
  /**
   * Private: Check if in development mode
   */
  private detectDeveloperMode(): boolean {
    // Check for common development indicators
    if (
      (window as any).__LANGKIT_VERSION === 'dev' ||
      window.location.hostname === 'localhost' ||
      window.location.hostname === '127.0.0.1' ||
      window.location.port === '3000' || // Common dev ports
      window.location.port === '8080' ||
      window.location.port === '5173' ||
      // For Wails development
      window.location.href.includes('wails.localhost') ||
      // Check for query param
      new URLSearchParams(window.location.search).has('dev')
    ) {
      return true;
    }
    return false;
  }
  
  /**
   * Private: Get application version
   */
  private getAppVersion(): string | null {
    // Try different ways to get version
    return (window as any).__LANGKIT_VERSION || 
           (window as any).appVersion || 
           document.querySelector('meta[name="app-version"]')?.getAttribute('content') ||
           null;
  }
  
  /**
   * Private: Generate a unique session ID
   */
  private generateSessionId(): string {
    return Date.now().toString(36) + Math.random().toString(36).substring(2, 9);
  }
  
  /**
   * Private: Generate throttle key with improved algorithm
   */
  private generateThrottleKey(level: LogLevel, component: string, message: string): string {
    // For better throttling, normalize the message:
    // 1. Remove specific IDs, numbers, timestamps, etc.
    const normalized = message
      // Replace UUIDs, hexadecimal hashes
      .replace(/[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}/gi, '[UUID]')
      // Replace numeric sequences (keep at most 2 digits)
      .replace(/\b\d{3,}\b/g, '[NUM]')
      // Replace timestamps in various formats
      .replace(/\d{1,2}:\d{2}(:\d{2})?(\.\d+)?/g, '[TIME]')
      .replace(/\d{4}-\d{2}-\d{2}/g, '[DATE]')
      // Replace URLs, file paths
      .replace(/(https?:\/\/[^\s]+)/g, '[URL]')
      .replace(/([\\\/][\w\-. ]+)+/g, '[PATH]')
      .trim();
    
    // Use a more stable algorithm that captures the message essence
    // Taking first 40 chars gives a good signature while handling longer messages better
    const signature = normalized.length > 40 
      ? normalized.substring(0, 40) 
      : normalized;
    
    return `${level}:${component}:${signature}`;
  }
  
  /**
   * Private: Check if log should be throttled
   */
  private shouldThrottle(level: LogLevel, component: string, throttleKey: string, message: string): boolean {
    // Skip throttling for high-level logs
    if (level >= LogLevel.WARN) return false;
    
    // Check for critical patterns that should never be throttled
    if (this.config.criticalPatterns.some(pattern => pattern.test(message))) {
      return false;
    }
    
    if (!this.config.throttling.enabled) return false;
    
    const now = Date.now();
    const throttleInfo = this.throttleMap.get(throttleKey);
    
    // Get component-specific throttle settings or defaults
    const componentSettings = this.config.throttling.byComponent[component];
    const throttleInterval = componentSettings?.interval || this.config.throttling.interval;
    const maxLogs = componentSettings?.maxLogs || this.config.throttling.maxSimilarLogs;
    
    // Higher volume reduction for trace logs
    const effectiveMaxLogs = level === LogLevel.TRACE ? Math.max(1, Math.floor(maxLogs / 3)) : maxLogs;
    
    if (throttleInfo) {
      // If within throttle window, increment count but don't log
      if (now - throttleInfo.lastTime < throttleInterval) {
        // Store sample message occasionally for richer summary
        if (throttleInfo.count % this.config.throttling.sampleInterval === 0 && 
            throttleInfo.samples.length < 3) {
          throttleInfo.samples.push(message);
        }
        
        throttleInfo.count++;
        this.throttleMap.set(throttleKey, throttleInfo);
        
        // Only log if we haven't exceeded the max
        return throttleInfo.count > effectiveMaxLogs;
      } else {
        // Report throttled messages with summary
        if (throttleInfo.count > effectiveMaxLogs) {
          const samplesText = throttleInfo.samples.length > 0 
            ? ` Examples: ${throttleInfo.samples.join(" | ")}` 
            : '';
            
          this.processLogEntry({
            level,
            component,
            message: `${message} (${throttleInfo.count} similar messages throttled in last ${Math.round(throttleInterval/1000)}s)${samplesText}`,
            timestamp: now,
            context: { throttled: true, count: throttleInfo.count },
            sessionId: this.sessionId,
            tags: ['throttled']
          });
        }
        
        // Reset throttle info
        this.throttleMap.set(throttleKey, {
          count: 1, 
          lastTime: now,
          samples: []
        });
        
        return false;
      }
    } else {
      // First occurrence, no throttling
      this.throttleMap.set(throttleKey, { 
        count: 1, 
        lastTime: now,
        samples: []
      });
      
      return false;
    }
  }
  
  /**
   * Private: Capture stack trace
   */
  private captureStack(): string {
    const err = new Error();
    return err.stack || '';
  }
  
  /**
   * Private: Derive tags from log content
   */
  private deriveTags(level: LogLevel, component: string, message: string): string[] {
    const tags: string[] = [component];
    
    // Add level as tag
    switch (level) {
      case LogLevel.TRACE: tags.push('trace'); break;
      case LogLevel.DEBUG: tags.push('debug'); break;
      case LogLevel.INFO: tags.push('info'); break;
      case LogLevel.WARN: tags.push('warning'); break;
      case LogLevel.ERROR: tags.push('error'); break;
      case LogLevel.CRITICAL: tags.push('critical'); break;
    }
    
    // Add operation tag if present
    if (this.activeOperation) {
      tags.push(`op:${this.activeOperation}`);
    }
    
    // Add error tag for errors
    if (level >= LogLevel.ERROR) {
      tags.push('error');
    }
    
    // Add performance tag for performance-related messages
    if (message.toLowerCase().includes('performance') || 
        message.toLowerCase().includes('timer') ||
        component === 'performance') {
      tags.push('performance');
    }
    
    return tags;
  }
  
  /**
   * Private: Set up global error listener
   */
  private setupErrorListener(): void {
    const errorHandler = (event: ErrorEvent) => {
      this.logError(
        event.error || new Error(event.message),
        'window',
        'Unhandled error',
        {
          source: event.filename,
          line: event.lineno,
          column: event.colno
        }
      );
    };
    
    const rejectionHandler = (event: PromiseRejectionEvent) => {
      const error = event.reason instanceof Error 
        ? event.reason 
        : new Error(String(event.reason));
      
      this.logError(
        error,
        'promise',
        'Unhandled promise rejection',
        {
          reason: String(event.reason)
        }
      );
    };
    
    window.addEventListener('error', errorHandler);
    window.addEventListener('unhandledrejection', rejectionHandler);
    
    // Store cleanup function
    this.eventListeners.push(() => {
      window.removeEventListener('error', errorHandler);
      window.removeEventListener('unhandledrejection', rejectionHandler);
    });
  }
  
  /**
   * Private: Handle beforeunload event
   */
  private handleBeforeUnload(e: BeforeUnloadEvent): void {
    // Flush any batched logs
    if (this.batchedLogs.length > 0) {
      this.flushBatch();
    }
    
    // Use sendBeacon for any critical logs in retry queue
    if (this.retryQueue.length > 0 && navigator.sendBeacon) {
      try {
        const criticalRetries = this.retryQueue
          .filter(item => item.entry.level >= LogLevel.ERROR)
          .map(item => item.entry);
          
        if (criticalRetries.length > 0) {
          const beacon = new Blob(
            [JSON.stringify({ batch: criticalRetries })], 
            { type: 'application/json' }
          );
          navigator.sendBeacon('/api/logs/batch', beacon);
        }
      } catch (e) {
        // Ignore errors during unload
      }
    }
  }
  
  /**
   * Private: Sanitize context to ensure it's serializable
   */
  private sanitizeContext(context: Record<string, any>): Record<string, any> {
    const result: Record<string, any> = {};
    const serializeSeen = new WeakMap();
    
    const sanitizeValue = (value: any, depth: number = 0): any => {
      // Max depth check
      if (depth > 5) return '[MAX_DEPTH]';
      
      // Handle null and primitive types
      if (value === null || value === undefined) return value;
      if (typeof value !== 'object' && typeof value !== 'function') return value;
      
      // Handle functions
      if (typeof value === 'function') return '[FUNCTION]';
      
      // Handle circular references
      if (value instanceof Object) {
        if (serializeSeen.has(value)) return '[CIRCULAR]';
        serializeSeen.set(value, true);
      }
      
      // Handle arrays
      if (Array.isArray(value)) {
        return value.map(item => sanitizeValue(item, depth + 1));
      }
      
      // Handle DOM nodes
      if (value instanceof Node) return value.nodeName || '[DOM_NODE]';
      
      // Handle objects
      try {
        const obj: Record<string, any> = {};
        
        // Limit to reasonable number of properties
        const entries = Object.entries(value).slice(0, 20);
        
        for (const [key, val] of entries) {
          // Skip functions, symbols, and non-standard properties
          if (typeof val === 'function' || typeof key === 'symbol') continue;
          
          // Sanitize value recursively
          obj[key] = sanitizeValue(val, depth + 1);
        }
        
        // Add indication if properties were truncated
        if (Object.keys(value).length > 20) {
          obj['...'] = `[${Object.keys(value).length - 20} more properties]`;
        }
        
        return obj;
      } catch (e) {
        // If anything goes wrong, return a placeholder
        return '[UNSERIALIZABLE]';
      }
    };
    
    // Process each top-level field
    for (const [key, value] of Object.entries(context)) {
      try {
        result[key] = sanitizeValue(value);
      } catch (e) {
        result[key] = '[ERROR_SERIALIZING]';
      }
    }
    
    return result;
  }
  
  /**
   * Private: Merge configuration objects
   */
  private mergeConfig(defaultConfig: LoggerConfig, customConfig: Partial<LoggerConfig>): LoggerConfig {
    const result = { ...defaultConfig };
    
    // Merge top-level properties
    for (const key in customConfig) {
      if (key === 'throttling' && customConfig.throttling) {
        // Deep merge throttling config
        result.throttling = {
          ...result.throttling,
          ...customConfig.throttling,
          byComponent: {
            ...result.throttling.byComponent,
            ...(customConfig.throttling.byComponent || {})
          }
        };
      } else if (key === 'batching' && customConfig.batching) {
        // Deep merge batching config
        result.batching = {
          ...result.batching,
          ...customConfig.batching
        };
      } else if (key === 'highVolumeCategories' && customConfig.highVolumeCategories) {
        // Convert to Set
        result.highVolumeCategories = new Set([
          ...result.highVolumeCategories,
          ...customConfig.highVolumeCategories
        ]);
      } else if (key === 'criticalPatterns' && Array.isArray(customConfig.criticalPatterns)) {
        // Replace patterns
        result.criticalPatterns = [...customConfig.criticalPatterns];
      } else {
        // Simple property assignment
        (result as any)[key] = (customConfig as any)[key];
      }
    }
    
    return result;
  }
}

// Export singleton instance
export const logger = new Logger();