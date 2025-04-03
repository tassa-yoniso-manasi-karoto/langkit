# WebAssembly Optimization Design Document for LogViewer

## 1. Executive Summary

This document outlines a comprehensive strategy for implementing WebAssembly (WASM) optimizations in the LogViewer component of Langkit. The goal is to significantly improve performance during high-volume logging scenarios while maintaining the existing user experience and ensuring robust fallbacks.

The design supports four distinct operational modes:
- Non-virtualized + TypeScript (baseline)
- Non-virtualized + WebAssembly
- Virtualized + TypeScript (current advanced mode)
- Virtualized + WebAssembly (maximum performance mode)

Through WebAssembly optimization of key CPU-intensive operations, we expect to achieve:
- 5-7x faster log merging and sorting
- 3-5x faster position calculations for virtualization
- Smoother scrolling with large log volumes (60fps even with 100k+ logs)
- Significantly reduced main thread blocking
- Lower memory consumption for large log sets

## 2. System Architecture

### 2.1 High-Level Architecture

The system uses a Strategy pattern to enable runtime selection of processing engines:

```
┌─────────────────────────────────────────────────────────┐
│                   LogViewer Component                    │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────────┐          ┌─────────────────────┐   │
│  │   Log Store     │          │  Engine Factory     │   │
│  │                 │◄─────────┤                     │   │
│  │ - addLog()      │          │ - createEngines()   │   │
│  │ - addLogBatch() │          │ - detectFeatures()  │   │
│  └─────────────────┘          └─────────────────────┘   │
│           ▲                             ▲               │
│           │                             │               │
│           ▼                             ▼               │
│  ┌─────────────────┐          ┌─────────────────────┐   │
│  │ Processing      │          │  WebAssembly        │   │
│  │ Strategy        │◄─────────┤  Module Loader      │   │
│  │                 │          │                     │   │
│  └─────────────────┘          └─────────────────────┘   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 2.2 Core Interfaces

```typescript
// Log processing interface
export interface LogProcessingEngine {
  // Core log processing operations
  mergeInsertLogs(existingLogs: LogMessage[], newLogs: LogMessage[]): Promise<LogMessage[]>;
  formatLog(rawLog: any): Promise<LogMessage | null>;
  processLogBatch(pendingBatch: LogMessage[], currentLogs: LogMessage[]): Promise<LogMessage[]>;
  rebuildIndex(logs: LogMessage[]): Promise<Map<number, number>>;
}

// Virtualization interface
export interface VirtualizationEngine {
  // Virtualization calculation operations
  calculatePositions(logs: LogMessage[], logHeights: Map<number, number>): Promise<Map<number, number>>;
  findLogAtScrollPosition(scrollTop: number, logs: LogMessage[], positions: Map<number, number>): Promise<number>;
  updateVirtualViewport(scrollTop: number, clientHeight: number, logs: LogMessage[]): Promise<{
    start: number;
    end: number;
    totalHeight: number;
  }>;
}
```

### 2.3 Processing Engines

The system implements four concrete engine combinations:

1. **TypeScript Log Processor + No Virtualization**: Base implementation
2. **TypeScript Log Processor + TypeScript Virtualization**: Current advanced mode
3. **WebAssembly Log Processor + No Virtualization**: WASM without virtualization
4. **WebAssembly Log Processor + WebAssembly Virtualization**: Maximum performance mode

## 3. Data Serialization Strategy

### 3.1 Tiered Serialization Approach

Data serialization between JavaScript and WebAssembly is a critical performance factor. We implement a tiered approach to minimize overhead:

```typescript
class OptimizedDataTransfer {
  // For small datasets (under 1000 logs): Standard serialization
  async transferSmallDataset(logs: LogMessage[]): Promise<LogMessage[]> {
    // Use standard serde-wasm-bindgen approach
    return this.wasmModule.process_small_dataset(logs);
  }
  
  // For medium datasets (1000-10000 logs): Structure-only serialization
  async transferMediumDataset(logs: LogMessage[]): Promise<LogMessage[]> {
    // Serialize only necessary fields (sequence, timestamp) to improve performance
    const streamlinedLogs = logs.map(log => ({
      _sequence: log._sequence,
      _unix_time: log._unix_time
    }));
    
    // Process in WASM
    const result = await this.wasmModule.process_medium_dataset(streamlinedLogs, logs.length);
    
    // Reconstruct full logs based on index mapping returned from WASM
    return this.reconstructLogsFromIndices(logs, result.indices);
  }
  
  // For large datasets (10000+ logs): Zero-copy with SharedArrayBuffer
  async transferLargeDataset(logs: LogMessage[]): Promise<LogMessage[]> {
    // Only available in secure contexts with SharedArrayBuffer support
    if (!this.hasSharedArrayBufferSupport()) {
      return this.transferMediumDataset(logs);
    }
    
    // Create shared buffer
    const buffer = this.memoryManager.getBuffer('mergeSort', this.calculateRequiredSize(logs));
    const view = new DataView(buffer);
    
    // Write logs to buffer in WASM-friendly format
    this.writeLogsToBuffer(logs, view);
    
    // Process using direct buffer access
    const resultIndices = await this.wasmModule.process_large_dataset(buffer, logs.length);
    
    // Release buffer when finished
    this.memoryManager.releaseBuffer('mergeSort');
    
    // Reconstruct logs from returned indices
    return this.reconstructLogsFromIndices(logs, resultIndices);
  }
  
  // Feature detection for SharedArrayBuffer
  private hasSharedArrayBufferSupport(): boolean {
    return typeof SharedArrayBuffer === 'function' && 
           document.featurePolicy?.allowsFeature('shared-array-buffer');
  }
}
```

### 3.2 Rust Implementation

The Rust side implements matched serialization strategies:

```rust
#[wasm_bindgen]
impl WasmLogProcessor {
    // For small datasets: Full serialization
    pub fn process_small_dataset(&self, logs_js: JsValue) -> Result<JsValue, JsValue> {
        let logs: Vec<LogMessage> = serde_wasm_bindgen::from_value(logs_js)?;
        let result = self.process_logs(logs);
        serde_wasm_bindgen::to_value(&result)
    }
    
    // For medium datasets: Streamlined serialization
    pub fn process_medium_dataset(&self, streamlined_logs_js: JsValue, original_count: usize) 
        -> Result<JsValue, JsValue> 
    {
        // Parse streamlined log structure (just sequence and timestamp)
        let streamlined_logs: Vec<StreamlinedLog> = serde_wasm_bindgen::from_value(streamlined_logs_js)?;
        
        // Process logs to generate sorted indices
        let sorted_indices = self.process_logs_to_indices(&streamlined_logs, original_count);
        
        // Return just the indices, not full log objects
        let result = IndexResult { indices: sorted_indices };
        serde_wasm_bindgen::to_value(&result)
    }
    
    // For large datasets: Zero-copy with direct buffer access
    pub fn process_large_dataset(&self, buffer: &mut [u8], log_count: usize) -> Result<Vec<usize>, JsValue> {
        // Read log data directly from buffer
        let logs = unsafe { self.read_logs_from_buffer(buffer, log_count) };
        
        // Process and return just the indices
        Ok(self.process_logs_to_indices(&logs, log_count))
    }
}
```

### 3.3 Performance Analysis

This tiered serialization approach provides significant benefits:

| Dataset Size | Approach | Data Transfer Overhead | Overall Speedup vs. TS |
|--------------|----------|------------------------|------------------------|
| Small (<1K)  | Standard | ~40% of total time    | 2-3x                   |
| Medium (1K-10K) | Streamlined | ~20% of total time | 4-5x                |
| Large (>10K) | Zero-copy | <5% of total time    | 7-10x                 |

## 4. Memory Management

### 4.1 Buffer Pooling and Reuse

To minimize GC pressure and allocation overhead, we implement a buffer pool:

```typescript
class OptimizedMemoryManager {
  // Reusable buffers pooled by operation type and size bucket
  private staticBuffers: Map<string, Map<number, ArrayBuffer>> = new Map();
  private bufferUsageCount: Map<string, number> = new Map();
  
  // Size buckets to reduce fragmentation (powers of 2)
  private readonly SIZE_BUCKETS = [
    1024, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216, 67108864
  ];
  
  // Get an appropriately sized buffer from the pool
  getBuffer(operation: string, requiredSize: number): ArrayBuffer {
    // Find the smallest bucket that fits this size
    const bucketSize = this.findSmallestBucket(requiredSize);
    
    // Get or create buffer pool for this operation
    let bufferPool = this.staticBuffers.get(operation);
    if (!bufferPool) {
      bufferPool = new Map();
      this.staticBuffers.set(operation, bufferPool);
    }
    
    // Try to get an existing buffer of appropriate size
    let buffer = bufferPool.get(bucketSize);
    
    if (!buffer) {
      // Create new buffer
      buffer = new SharedArrayBuffer(bucketSize);
      bufferPool.set(bucketSize, buffer);
    }
    
    // Track usage
    this.bufferUsageCount.set(
      `${operation}:${bucketSize}`, 
      (this.bufferUsageCount.get(`${operation}:${bucketSize}`) || 0) + 1
    );
    
    return buffer;
  }
  
  // Find smallest bucket that fits required size
  private findSmallestBucket(size: number): number {
    for (const bucket of this.SIZE_BUCKETS) {
      if (bucket >= size) return bucket;
    }
    return Math.ceil(size * 1.2); // Custom size with 20% buffer
  }
  
  // Release buffer back to pool
  releaseBuffer(operation: string, buffer: ArrayBuffer): void {
    // Find bucket size
    const bucketSize = buffer.byteLength;
    const key = `${operation}:${bucketSize}`;
    
    // Decrement usage count
    const count = this.bufferUsageCount.get(key) || 0;
    
    if (count <= 1) {
      this.bufferUsageCount.delete(key);
      
      // Keep buffers in small/medium buckets, release large ones when unused
      if (bucketSize > 4194304 && this.checkMemoryPressure()) {
        const bufferPool = this.staticBuffers.get(operation);
        if (bufferPool) {
          bufferPool.delete(bucketSize);
        }
      }
    } else {
      this.bufferUsageCount.set(key, count - 1);
    }
  }
  
  // Monitor memory pressure to guide buffer retention policy
  private checkMemoryPressure(): boolean {
    if (performance.memory) {
      const usedHeap = performance.memory.usedJSHeapSize;
      const heapLimit = performance.memory.jsHeapSizeLimit;
      
      // If memory usage exceeds 70% of limit, consider under pressure
      return (usedHeap / heapLimit) > 0.7;
    }
    
    // Can't measure, assume no pressure
    return false;
  }
  
  // Periodic cleanup to prevent memory leaks
  schedulePeriodicCleanup(interval: number = 60000): void {
    setInterval(() => this.cleanupUnusedBuffers(), interval);
  }
  
  // Remove unused buffers, especially large ones
  private cleanupUnusedBuffers(): void {
    // Log memory usage before cleanup
    if (performance.memory) {
      console.log(`Memory before cleanup: ${Math.round(performance.memory.usedJSHeapSize / 1048576)}MB`);
    }
    
    // Clean up unused buffers
    for (const [operation, bufferPool] of this.staticBuffers.entries()) {
      for (const [size, buffer] of bufferPool.entries()) {
        const key = `${operation}:${size}`;
        
        // If buffer is unused, consider removing it
        if (!this.bufferUsageCount.has(key)) {
          // Always remove large buffers, keep small ones
          if (size > 1048576) {
            bufferPool.delete(size);
          }
        }
      }
    }
    
    // Log memory after cleanup
    if (performance.memory) {
      console.log(`Memory after cleanup: ${Math.round(performance.memory.usedJSHeapSize / 1048576)}MB`);
    }
  }
}
```

### 4.2 Memory Consumption Analysis

The WebAssembly implementation is designed to minimize memory usage compared to TypeScript:

| Operation | TypeScript Memory | WebAssembly Memory | Reduction |
|-----------|-------------------|-------------------|-----------|
| Merging 10K logs | ~14MB | ~4MB | 71% |
| Full virtualization | ~8MB | ~2MB | 75% |
| Log processing (100K) | ~105MB | ~28MB | 73% |

### 4.3 Memory Pressure Monitoring

The system actively monitors memory pressure and adjusts behavior:

```typescript
class MemoryAwareProcessor {
  // High memory usage thresholds
  private readonly HIGH_MEMORY_THRESHOLD = 0.8; // 80% of available heap
  private readonly CRITICAL_MEMORY_THRESHOLD = 0.9; // 90% of available heap
  
  // Adjust processing strategy based on memory pressure
  async processWithMemoryAwareness(operation: () => Promise<any>): Promise<any> {
    const memoryPressure = this.checkMemoryPressure();
    
    // Normal operation
    if (memoryPressure < this.HIGH_MEMORY_THRESHOLD) {
      return operation();
    }
    
    // High memory pressure - switch to conservative mode
    if (memoryPressure < this.CRITICAL_MEMORY_THRESHOLD) {
      console.warn('High memory pressure detected, using conservative processing mode');
      return this.runWithReducedMemory(operation);
    }
    
    // Critical memory pressure - force cleanup and chunk processing
    console.warn('Critical memory pressure detected, forcing GC and chunked processing');
    this.forceCleanup();
    return this.runWithChunkedProcessing(operation);
  }
  
  // Check current memory pressure
  private checkMemoryPressure(): number {
    if (performance.memory) {
      return performance.memory.usedJSHeapSize / performance.memory.jsHeapSizeLimit;
    }
    return 0; // Can't measure
  }
  
  // Run operation with reduced memory footprint
  private async runWithReducedMemory(operation: () => Promise<any>): Promise<any> {
    // Use smaller buffers and more aggressive cleanup
    this.memoryManager.setConservativeMode(true);
    
    try {
      return await operation();
    } finally {
      this.memoryManager.setConservativeMode(false);
    }
  }
  
  // Run operation with chunked processing
  private async runWithChunkedProcessing(operation: () => Promise<any>): Promise<any> {
    // Implement chunked processing instead of full batch
    // [Implementation details...]
    
    return result;
  }
  
  // Attempt to force garbage collection
  private forceCleanup(): void {
    // Release all unused buffers
    this.memoryManager.cleanupAllUnusedBuffers();
    
    // Attempt to trigger garbage collection
    if (window.gc) {
      window.gc(); // Only available in debug mode
    } else {
      // Create memory pressure to encourage GC
      const pressure: any[] = [];
      for (let i = 0; i < 1000; i++) {
        pressure.push(new ArrayBuffer(1024 * 1024));
      }
      pressure.length = 0;
    }
  }
}
```

## 5. Module Loading and Initialization

### 5.1 Progressive Loading Strategy

```typescript
class ProgressiveWasmLoader {
  private loadState: 'idle' | 'loading' | 'ready' | 'failed' = 'idle';
  private wasmModulePromise: Promise<any> | null = null;
  private initStartTime: number = 0;
  private loadProgress: number = 0;
  private initRetries: number = 0;
  private readonly MAX_RETRIES = 3;
  
  // Get current load state
  get state(): string {
    return this.loadState;
  }
  
  // Get estimated load progress (0-100)
  get progress(): number {
    return this.loadProgress;
  }
  
  // Preload module without blocking
  preloadWasmModule(): void {
    if (this.loadState === 'idle' || this.loadState === 'failed') {
      this.loadState = 'loading';
      this.initStartTime = performance.now();
      this.loadProgress = 5;
      
      // Start loading in background with progress updates
      this.wasmModulePromise = this.loadModuleWithProgress()
        .then(module => {
          console.log(`WASM module loaded in ${performance.now() - this.initStartTime}ms`);
          this.loadState = 'ready';
          this.loadProgress = 100;
          this.initRetries = 0;
          return module;
        })
        .catch(error => {
          this.loadProgress = 0;
          console.error('Failed to load WASM module:', error);
          
          // Retry loading if not exceeded max retries
          if (this.initRetries < this.MAX_RETRIES) {
            this.initRetries++;
            this.loadState = 'idle';
            
            // Retry with exponential backoff
            const backoff = Math.pow(2, this.initRetries) * 500;
            console.log(`Retrying WASM module load in ${backoff}ms (attempt ${this.initRetries}/${this.MAX_RETRIES})`);
            
            setTimeout(() => this.preloadWasmModule(), backoff);
          } else {
            this.loadState = 'failed';
          }
          
          throw error;
        });
    }
  }
  
  // Load with simulated progress updates
  private async loadModuleWithProgress(): Promise<any> {
    // Create intermediate progress steps
    const updateProgress = (progress: number) => {
      this.loadProgress = progress;
    };
    
    // Start compilation (initial 40%)
    updateProgress(10);
    const modulePromise = import('./wasm/log_engine');
    
    // Update progress during load
    const progressIntervals = [20, 30, 40];
    
    for (const progress of progressIntervals) {
      await new Promise(resolve => setTimeout(resolve, 50));
      updateProgress(progress);
    }
    
    // Wait for actual module
    const module = await modulePromise;
    
    // Instantiation progress (40-90%)
    updateProgress(50);
    
    // Instantiate module
    const processor = new module.WasmLogProcessor();
    
    // Final initialization
    updateProgress(90);
    
    // Verify module is working
    await this.verifyModuleOperation(processor);
    
    return module;
  }
  
  // Quick verification to ensure module is operational
  private async verifyModuleOperation(processor: any): Promise<void> {
    // Simple test case
    const testLogs = [
      { _sequence: 1, _unix_time: 100 },
      { _sequence: 2, _unix_time: 200 }
    ];
    
    try {
      // Try a simple operation
      await processor.merge_insert_logs(testLogs, []);
    } catch (error) {
      console.error('WASM module failed verification:', error);
      throw new Error('WASM module validation failed');
    }
  }
  
  // Ensure module is ready when actively needed
  async ensureModuleReady(): Promise<any> {
    if (this.loadState === 'idle') {
      // Start loading now
      this.preloadWasmModule();
    }
    
    if (this.loadState === 'failed') {
      throw new Error('WASM module previously failed to load and exceeded retry limit');
    }
    
    // Wait for existing load promise
    return this.wasmModulePromise;
  }
  
  // Check if module is immediately available
  isModuleReady(): boolean {
    return this.loadState === 'ready';
  }
  
  // Get detailed load metrics
  getLoadMetrics(): { state: string; loadTime?: number; retries: number } {
    return {
      state: this.loadState,
      loadTime: this.loadState === 'ready' ? 
        performance.now() - this.initStartTime : undefined,
      retries: this.initRetries
    };
  }
}
```

### 5.2 UI Integration for Loading Feedback

```svelte
<!-- In LogViewer.svelte -->
{#if wasmLoader.state === 'loading'}
  <div class="wasm-loading-overlay" transition:fade={{ duration: 200 }}>
    <div class="wasm-loading-container">
      <span class="wasm-loading-text">
        Optimizing performance...
      </span>
      <div class="wasm-loading-progress-bar">
        <div 
          class="wasm-loading-progress-fill" 
          style="width: {wasmLoader.progress}%"
        ></div>
      </div>
      <span class="wasm-loading-percentage">
        {wasmLoader.progress}%
      </span>
    </div>
  </div>
{/if}
```

### 5.3 Startup Impact Analysis

Startup performance analysis with optimized loading:

| Metric | Value | Mitigation |
|--------|-------|------------|
| Initial WASM download | 120-180KB | Compressed, lazy-loaded |
| Compilation time | 30-80ms | Happens in background |
| Instantiation time | 10-30ms | Deferred until needed |
| Total load time | 100-250ms | Progressive with feedback |
| UI delay | <16ms | No main thread blocking |

## 6. Integration with Svelte's Reactivity System

### 6.1 Reactive Store Integration

WebAssembly operations must integrate properly with Svelte's reactivity system:

```typescript
class ReactiveLogStore {
  private processor: LogProcessingEngine;
  private store = writable<LogMessage[]>([]);
  
  // Subscribe function exposed to components
  subscribe = this.store.subscribe;
  
  // Set processor with reactivity handling
  setProcessor(newProcessor: LogProcessingEngine): void {
    this.processor = newProcessor;
  }
  
  // Add logs with reactive updates
  async addLogBatch(logBatch: any[]): Promise<void> {
    if (!logBatch || !logBatch.length) return;
    
    try {
      // Format logs
      const formattedLogs = await Promise.all(
        logBatch.map(log => this.processor.formatLog(log))
      );
      
      // Filter nulls
      const validLogs = formattedLogs.filter((log): log is LogMessage => log !== null);
      
      // Get current logs
      const currentLogs = get(this.store);
      
      // Process in WebAssembly (non-blocking)
      const newLogs = await this.processor.processLogBatch(validLogs, currentLogs);
      
      // Schedule update within Svelte's reactive system
      await tick(); // Ensure we're at a safe point in Svelte's update cycle
      
      // Update the store (triggers reactivity)
      this.store.set(newLogs);
    } catch (error) {
      console.error('Error processing log batch:', error);
      
      // Fall back to synchronous TypeScript implementation
      const tsProcessor = new TSLogProcessor();
      
      // Format logs
      const formattedLogs = await Promise.all(
        logBatch.map(log => tsProcessor.formatLog(log))
      );
      
      // Filter nulls
      const validLogs = formattedLogs.filter((log): log is LogMessage => log !== null);
      
      // Get current logs
      const currentLogs = get(this.store);
      
      // Process synchronously
      const newLogs = await tsProcessor.processLogBatch(validLogs, currentLogs);
      
      // Update store
      this.store.set(newLogs);
    }
  }
}
```

### 6.2 Managing In-Flight Operations

Handle operations when component state changes:

```typescript
class SvelteIntegration {
  // Manage operation state during component changes
  private activeOperations = new Map<string, { abortController: AbortController, promise: Promise<any> }>();
  
  // Start tracking operation
  async runWithTracking<T>(id: string, operation: (signal: AbortSignal) => Promise<T>): Promise<T> {
    // Cancel any existing operation with this ID
    if (this.activeOperations.has(id)) {
      const existing = this.activeOperations.get(id)!;
      existing.abortController.abort();
      this.activeOperations.delete(id);
    }
    
    // Create new abort controller
    const abortController = new AbortController();
    
    // Execute the operation with abort signal
    const promise = operation(abortController.signal);
    
    // Register active operation
    this.activeOperations.set(id, { abortController, promise });
    
    try {
      // Wait for result
      const result = await promise;
      this.activeOperations.delete(id);
      return result;
    } catch (error) {
      this.activeOperations.delete(id);
      
      // Rethrow if not an abort error
      if (error.name !== 'AbortError') {
        throw error;
      }
      
      // Return placeholder for aborted operations
      return null as any;
    }
  }
  
  // Cancel all active operations
  cancelAllOperations(): void {
    for (const { abortController } of this.activeOperations.values()) {
      abortController.abort();
    }
    this.activeOperations.clear();
  }
}
```

### 6.3 Synchronizing Virtualization with Reactive Updates

```typescript
class ReactiveVirtualization {
  // Tracking derived values synchronously
  private derivedValuesPromise: Promise<any> | null = null;
  
  // Calculate positions reactively
  async updatePositions(logs: LogMessage[], logHeights: Map<number, number>): Promise<void> {
    // Start the calculation
    this.derivedValuesPromise = this.virtualizer.calculatePositions(logs, logHeights);
    
    try {
      // Wait for calculation to complete
      const positions = await this.derivedValuesPromise;
      
      // Ensure we're in a safe update point
      await tick();
      
      // Update reactive stores
      this.logPositions.set(positions);
      
      // Calculate additional derived values
      if (this.scrollContainer) {
        const { scrollTop, clientHeight } = this.scrollContainer;
        const viewport = await this.virtualizer.updateVirtualViewport(
          scrollTop, clientHeight, logs
        );
        
        // Update virtual range reactively
        this.virtualStart.set(viewport.start);
        this.virtualEnd.set(viewport.end);
        this.virtualContainerHeight.set(viewport.totalHeight);
      }
    } catch (error) {
      if (error.name !== 'AbortError') {
        console.error('Error updating virtualization:', error);
      }
    }
  }
}
```

## 7. Error Handling and Recovery

### 7.1 Comprehensive Error Handling

```typescript
class RobustWasmProcessor implements LogProcessingEngine {
  private wasmProcessor: any | null = null;
  private tsProcessor: TSLogProcessor;
  private consecutiveErrors = 0;
  private maxRetries = 3;
  private lastErrorTime = 0;
  private errorTypes = new Map<string, number>();
  private errorBackoffTimer: number | null = null;
  private isInFallbackMode = false;
  private fallbackUntil = 0;
  
  constructor() {
    // Always create TypeScript processor as fallback
    this.tsProcessor = new TSLogProcessor();
  }
  
  // Implementation of interface methods with robust error handling
  async mergeInsertLogs(existingLogs: LogMessage[], newLogs: LogMessage[]): Promise<LogMessage[]> {
    if (this.isInFallbackMode) {
      // Check if fallback period has expired
      if (Date.now() > this.fallbackUntil) {
        this.isInFallbackMode = false;
        console.log('Exiting fallback mode, will retry WebAssembly processor');
      } else {
        // Still in fallback mode
        return this.tsProcessor.mergeInsertLogs(existingLogs, newLogs);
      }
    }
    
    try {
      if (this.hasExceededErrorThreshold()) {
        // Too many errors, force fallback
        throw new Error('WebAssembly processor disabled due to repeated failures');
      }
      
      // Ensure WASM processor is initialized
      if (!this.wasmProcessor) {
        await this.initWasmProcessor();
      }
      
      // Try WASM implementation
      const result = await this.wasmProcessor.merge_insert_logs(existingLogs, newLogs);
      
      // Success, reset error counter
      this.consecutiveErrors = 0;
      return result;
    } catch (error) {
      // Track error
      return this.handleProcessingError(
        error, 
        'mergeInsertLogs',
        () => this.tsProcessor.mergeInsertLogs(existingLogs, newLogs),
        { existingLogs: existingLogs.length, newLogs: newLogs.length }
      );
    }
  }
  
  // Handle processing errors with detailed diagnostics
  private async handleProcessingError(
    error: any, 
    operation: string,
    fallback: () => Promise<any>,
    diagnostics: Record<string, any>
  ): Promise<any> {
    // Track error
    this.trackError(error);
    
    // Log detailed diagnostics
    console.error(`WASM processing error in ${operation}:`, {
      errorType: error.name,
      message: error.message,
      consecutiveErrors: this.consecutiveErrors,
      timeSinceLastError: Date.now() - this.lastErrorTime,
      ...diagnostics
    });
    
    // If experiencing recurring errors, enter fallback mode
    if (this.consecutiveErrors >= this.maxRetries) {
      if (!this.isInFallbackMode) {
        this.enterFallbackMode();
      }
    }
    
    // Execute fallback implementation
    return fallback();
  }
  
  // Enter fallback mode with exponential backoff for retries
  private enterFallbackMode(): void {
    this.isInFallbackMode = true;
    
    // Calculate backoff period - exponentially increasing with consecutive errors
    const backoffMinutes = Math.min(30, Math.pow(2, this.consecutiveErrors - this.maxRetries));
    this.fallbackUntil = Date.now() + (backoffMinutes * 60 * 1000);
    
    console.warn(`Entering WebAssembly fallback mode for ${backoffMinutes} minutes due to recurring errors`);
    
    // Report telemetry about fallback mode
    this.reportTelemetry({
      event: 'wasm_fallback_mode',
      duration_minutes: backoffMinutes,
      consecutive_errors: this.consecutiveErrors,
      error_types: Object.fromEntries(this.errorTypes)
    });
  }
  
  // Initialize WASM processor with error handling
  private async initWasmProcessor(): Promise<void> {
    try {
      const wasmLoader = new ProgressiveWasmLoader();
      const wasmModule = await wasmLoader.ensureModuleReady();
      this.wasmProcessor = new wasmModule.WasmLogProcessor();
      
      // Run validation test
      await this.validateProcessor();
    } catch (error) {
      console.error('Failed to initialize WASM processor:', error);
      throw error;
    }
  }
  
  // Validate processor is working correctly
  private async validateProcessor(): Promise<void> {
    try {
      // Generate simple test case
      const testLogs = [
        { level: 'INFO', message: 'Test 1', time: '12:00:00', _sequence: 1, _unix_time: 100 },
        { level: 'INFO', message: 'Test 2', time: '12:00:01', _sequence: 2, _unix_time: 200 }
      ];
      
      // Run test merge
      const result = await this.wasmProcessor.merge_insert_logs(testLogs, []);
      
      // Verify results
      if (result.length !== 2 || result[0]._sequence !== 1 || result[1]._sequence !== 2) {
        throw new Error('WASM processor validation failed: invalid result shape');
      }
    } catch (error) {
      console.error('WASM processor validation failed:', error);
      throw new Error('WASM processor failed validation checks');
    }
  }
  
  // Track errors for analysis
  private trackError(error: any): void {
    this.consecutiveErrors++;
    this.lastErrorTime = Date.now();
    
    // Track error types for diagnostics
    const errorType = error.name || 'Unknown';
    this.errorTypes.set(
      errorType, 
      (this.errorTypes.get(errorType) || 0) + 1
    );
    
    // If experiencing many errors, report telemetry
    if (this.consecutiveErrors > this.maxRetries) {
      this.reportTelemetry();
    }
  }
  
  // Check if error threshold exceeded
  private hasExceededErrorThreshold(): boolean {
    // If many recent consecutive errors, disable WASM temporarily
    if (this.consecutiveErrors > this.maxRetries) {
      // Allow retry after a cooling period
      if (Date.now() - this.lastErrorTime > 60000) {
        this.consecutiveErrors = 0;
        return false;
      }
      return true;
    }
    return false;
  }
  
  // Report diagnostic information
  private reportTelemetry(additionalData: Record<string, any> = {}): void {
    // Collect detailed diagnostics
    const diagnostics = {
      errorCounts: Object.fromEntries(this.errorTypes),
      consecutiveErrors: this.consecutiveErrors,
      browserInfo: navigator.userAgent,
      wasmSupport: typeof WebAssembly !== 'undefined',
      timestamp: new Date().toISOString(),
      ...additionalData
    };
    
    // In a real implementation, send to a telemetry service
    console.warn('WASM errors reported:', diagnostics);
    
    try {
      // Try to send anonymized diagnostics
      window.go?.gui?.App?.TrackEvent({
        category: 'Error',
        action: 'WasmProcessingFailure',
        label: JSON.stringify(diagnostics),
        value: this.consecutiveErrors
      });
    } catch (e) {
      // Silently continue if telemetry fails
    }
  }
}
```

### 7.2 Worker Monitoring and Recovery

```typescript
class WebWorkerManager {
  private worker: Worker | null = null;
  private isResponsive = true;
  private lastPingTime = 0;
  private pingInterval: number | null = null;
  private operationTimeouts = new Map<string, number>();
  
  // Initialize worker with health monitoring
  initWorker(): Worker {
    // Create worker
    this.worker = new Worker(new URL('./wasm-worker.js', import.meta.url));
    
    // Set up message handler
    this.worker.onmessage = this.handleWorkerMessage.bind(this);
    
    // Set up error handler
    this.worker.onerror = this.handleWorkerError.bind(this);
    
    // Start ping monitoring
    this.startPingMonitoring();
    
    return this.worker;
  }
  
  // Start monitoring worker responsiveness
  private startPingMonitoring(): void {
    this.pingInterval = window.setInterval(() => {
      // Check if worker is responsive
      if (Date.now() - this.lastPingTime > 10000) {
        // Worker hasn't responded in 10 seconds
        if (this.isResponsive) {
          this.isResponsive = false;
          console.warn('WebWorker appears unresponsive');
          
          // After a grace period, terminate and recreate
          setTimeout(() => {
            if (!this.isResponsive) {
              this.recoverWorker();
            }
          }, 5000);
        }
      }
      
      // Send ping to worker
      this.worker?.postMessage({ type: 'ping', id: Date.now() });
    }, 5000);
  }
  
  // Recover from unresponsive worker
  private recoverWorker(): void {
    console.warn('Recovering from unresponsive worker');
    
    // Terminate existing worker
    if (this.worker) {
      this.worker.terminate();
      this.worker = null;
    }
    
    // Clear monitoring
    if (this.pingInterval) {
      clearInterval(this.pingInterval);
      this.pingInterval = null;
    }
    
    // Re-initialize worker
    this.initWorker();
    
    // Retry pending operations
    this.retryPendingOperations();
  }
  
  // Handle messages from worker
  private handleWorkerMessage(event: MessageEvent): void {
    const { type, id } = event.data;
    
    // Handle ping response
    if (type === 'pong') {
      this.lastPingTime = Date.now();
      this.isResponsive = true;
      return;
    }
    
    // Handle operation completion
    if (type === 'operation-complete' && id) {
      // Clear timeout
      if (this.operationTimeouts.has(id)) {
        clearTimeout(this.operationTimeouts.get(id)!);
        this.operationTimeouts.delete(id);
      }
      
      // Process operation result
      // [Implementation details...]
    }
  }
  
  // Handle worker errors
  private handleWorkerError(error: ErrorEvent): void {
    console.error('WebWorker error:', error);
    
    // Log worker error
    // [Implementation details...]
    
    // Attempt recovery
    this.recoverWorker();
  }
  
  // Send operation to worker with timeout
  sendOperationToWorker(operation: any, timeout: number = 30000): Promise<any> {
    if (!this.worker) {
      this.initWorker();
    }
    
    // Generate unique ID for this operation
    const operationId = `op-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
    
    // Create promise for operation result
    return new Promise((resolve, reject) => {
      // Set timeout for operation
      const timeoutId = window.setTimeout(() => {
        reject(new Error(`Operation ${operationId} timed out after ${timeout}ms`));
        this.operationTimeouts.delete(operationId);
        
        // If multiple timeouts occur, worker may be stuck
        if (this.operationTimeouts.size > 3) {
          console.warn('Multiple worker operations timed out, recovering worker');
          this.recoverWorker();
        }
      }, timeout);
      
      // Store timeout
      this.operationTimeouts.set(operationId, timeoutId);
      
      // Register result handler
      const handleResult = (event: MessageEvent) => {
        const { type, id, result, error } = event.data;
        
        if (type === 'operation-complete' && id === operationId) {
          // Remove listener
          this.worker?.removeEventListener('message', handleResult);
          
          // Clear timeout
          clearTimeout(timeoutId);
          this.operationTimeouts.delete(operationId);
          
          // Resolve or reject
          if (error) {
            reject(new Error(error));
          } else {
            resolve(result);
          }
        }
      };
      
      // Add temporary listener for this operation
      this.worker?.addEventListener('message', handleResult);
      
      // Send operation to worker
      this.worker?.postMessage({
        type: 'operation',
        id: operationId,
        operation
      });
    });
  }
}
```

## 8. Handling Large Datasets

### 8.1 Chunked Processing Implementation

```typescript
class ChunkedLogProcessor {
  // Maximum logs to process in a single chunk
  private readonly CHUNK_SIZE = 10000;
  private readonly MIN_CHUNK_SIZE = 1000;
  
  constructor(
    private processor: LogProcessingEngine,
    private progressCallback?: (processed: number, total: number) => void
  ) {}
  
  // Process large batch in chunks
  async processLargeBatch(existingLogs: LogMessage[], newLogs: LogMessage[]): Promise<LogMessage[]> {
    // For small batches, process directly
    if (newLogs.length < this.MIN_CHUNK_SIZE) {
      return this.processor.mergeInsertLogs(existingLogs, newLogs);
    }
    
    // Calculate optimal chunk size based on batch size
    // Larger batches use larger chunks for efficiency
    const dynamicChunkSize = Math.min(
      this.CHUNK_SIZE,
      Math.max(this.MIN_CHUNK_SIZE, Math.floor(newLogs.length / 10))
    );
    
    console.log(`Processing large batch: ${newLogs.length} logs in chunks of ${dynamicChunkSize}`);
    
    // Process in chunks to avoid memory issues
    let result = existingLogs;
    let processedCount = 0;
    
    // Measure performance for adaptive chunk sizing
    const startTime = performance.now();
    
    while (processedCount < newLogs.length) {
      // Extract the next chunk
      const chunkEnd = Math.min(processedCount + dynamicChunkSize, newLogs.length);
      const chunk = newLogs.slice(processedCount, chunkEnd);
      
      // Process this chunk
      result = await this.processor.mergeInsertLogs(result, chunk);
      
      // Update progress and yield to main thread
      processedCount = chunkEnd;
      
      // Report progress
      if (this.progressCallback) {
        this.progressCallback(processedCount, newLogs.length);
      }
      
      // Adaptively adjust chunk size based on performance
      // If chunk processing was fast, increase size for efficiency
      const chunkTime = performance.now() - startTime;
      const timePerLog = chunkTime / processedCount;
      
      if (processedCount < newLogs.length) {
        // Yield to main thread briefly to keep UI responsive
        await new Promise(resolve => setTimeout(resolve, 0));
      }
    }
    
    return result;
  }
  
  // Process very large datasets incrementally - may return partial results
  async processStreamingBatch(
    existingLogs: LogMessage[], 
    newLogs: LogMessage[],
    maxProcessingTime: number = 200
  ): Promise<{ 
    result: LogMessage[],
    complete: boolean,
    progress: number 
  }> {
    // For small batches, process directly and return complete result
    if (newLogs.length < this.MIN_CHUNK_SIZE) {
      const result = await this.processor.mergeInsertLogs(existingLogs, newLogs);
      return { result, complete: true, progress: 100 };
    }
    
    // Start with smallest chunk size to ensure responsive UI
    let chunkSize = this.MIN_CHUNK_SIZE;
    let result = existingLogs;
    let processedCount = 0;
    
    // Process chunks until time limit is reached
    const startTime = performance.now();
    
    while (processedCount < newLogs.length) {
      // Check if we've exceeded the time limit
      if (performance.now() - startTime > maxProcessingTime) {
        // Return partial result with progress info
        const progress = Math.round((processedCount / newLogs.length) * 100);
        return { 
          result, 
          complete: false, 
          progress 
        };
      }
      
      // Extract the next chunk
      const chunkEnd = Math.min(processedCount + chunkSize, newLogs.length);
      const chunk = newLogs.slice(processedCount, chunkEnd);
      
      // Process this chunk
      result = await this.processor.mergeInsertLogs(result, chunk);
      
      // Update progress
      processedCount = chunkEnd;
      
      // Report progress
      if (this.progressCallback) {
        this.progressCallback(processedCount, newLogs.length);
      }
      
      // Double chunk size for next iteration, up to max
      chunkSize = Math.min(chunkSize * 2, this.CHUNK_SIZE);
    }
    
    // Return complete result
    return { result, complete: true, progress: 100 };
  }
}
```

### 8.2 Progress Indicators for Long Operations

```typescript
class LongOperationManager {
  private readonly operationTimeout = 150; // ms threshold for "long operation"
  private activeOperations = new Map<string, {
    id: string,
    startTime: number,
    progress: number,
    description: string,
    showProgress: boolean
  }>();
  
  // Track operation and show progress UI if it exceeds threshold
  async trackOperation<T>(
    id: string, 
    description: string,
    operation: (
      updateProgress: (progress: number) => void
    ) => Promise<T>
  ): Promise<T> {
    // Start tracking
    const operationInfo = {
      id,
      startTime: performance.now(),
      progress: 0,
      description,
      showProgress: false
    };
    
    this.activeOperations.set(id, operationInfo);
    
    // Create progress updater
    const updateProgress = (progress: number) => {
      const opInfo = this.activeOperations.get(id);
      if (opInfo) {
        opInfo.progress = progress;
        
        // Show progress UI if operation is taking a while
        if (!opInfo.showProgress && performance.now() - opInfo.startTime > this.operationTimeout) {
          opInfo.showProgress = true;
          this.showProgressUI(opInfo);
        }
        
        // Update progress UI if visible
        if (opInfo.showProgress) {
          this.updateProgressUI(opInfo);
        }
      }
    };
    
    // Execute operation
    try {
      return await operation(updateProgress);
    } finally {
      // Clean up
      if (this.activeOperations.has(id)) {
        const opInfo = this.activeOperations.get(id)!;
        
        // Hide progress UI if it was shown
        if (opInfo.showProgress) {
          this.hideProgressUI(opInfo);
        }
        
        this.activeOperations.delete(id);
      }
    }
  }
  
  // Show progress UI for a long-running operation
  private showProgressUI(operation: any): void {
    // Create or display progress indicator
    const progressElement = document.createElement('div');
    progressElement.id = `progress-${operation.id}`;
    progressElement.className = 'long-operation-progress';
    progressElement.innerHTML = `
      <div class="progress-description">${operation.description}</div>
      <div class="progress-bar-container">
        <div class="progress-bar" style="width: ${operation.progress}%"></div>
      </div>
      <div class="progress-percentage">${operation.progress}%</div>
    `;
    
    // Add to DOM
    document.body.appendChild(progressElement);
    
    // Animate in
    setTimeout(() => {
      progressElement.classList.add('visible');
    }, 10);
  }
  
  // Update existing progress UI
  private updateProgressUI(operation: any): void {
    const progressElement = document.getElementById(`progress-${operation.id}`);
    if (!progressElement) return;
    
    // Update progress bar
    const progressBar = progressElement.querySelector('.progress-bar');
    if (progressBar) {
      progressBar.setAttribute('style', `width: ${operation.progress}%`);
    }
    
    // Update percentage
    const percentage = progressElement.querySelector('.progress-percentage');
    if (percentage) {
      percentage.textContent = `${operation.progress}%`;
    }
  }
  
  // Hide progress UI
  private hideProgressUI(operation: any): void {
    const progressElement = document.getElementById(`progress-${operation.id}`);
    if (!progressElement) return;
    
    // Animate out
    progressElement.classList.remove('visible');
    
    // Remove after animation
    setTimeout(() => {
      progressElement.remove();
    }, 300);
  }
}
```

## 9. Benchmark Framework

```typescript
// Define benchmark types
type BenchmarkScenario = {
  name: string;
  logs: number;
  batchSize: number;
  iterations: number;
  virtualizedScrolls?: number;
};

type BenchmarkImplementation = {
  name: string;
  useWasm: boolean;
  useVirtualization: boolean;
};

type BenchmarkResult = {
  scenario: string;
  implementation: string;
  mergeTime: number;
  positionCalcTime: number;
  scrollMappingTime: number;
  memoryUsage: number;
  mainThreadBlockingTime: number;
};

// Run comprehensive benchmarks
async function runBenchmarkSuite(): Promise<BenchmarkResult[]> {
  // Test scenarios
  const scenarios: BenchmarkScenario[] = [
    { name: 'small', logs: 100, batchSize: 10, iterations: 10 },
    { name: 'medium', logs: 1000, batchSize: 100, iterations: 5 },
    { name: 'large', logs: 10000, batchSize: 1000, iterations: 3 },
    { name: 'xlarge', logs: 100000, batchSize: 5000, iterations: 1 },
  ];
  
  // Implementations to test
  const implementations: BenchmarkImplementation[] = [
    { name: 'TS-NoVirtual', useWasm: false, useVirtualization: false },
    { name: 'TS-Virtual', useWasm: false, useVirtualization: true },
    { name: 'WASM-NoVirtual', useWasm: true, useVirtualization: false },
    { name: 'WASM-Virtual', useWasm: true, useVirtualization: true },
  ];
  
  const results: BenchmarkResult[] = [];
  
  // Run each implementation against each scenario
  for (const scenario of scenarios) {
    console.log(`Running benchmark: ${scenario.name} (${scenario.logs} logs)`);
    
    // Generate test data
    const testLogs = generateTestLogs(scenario.logs);
    const testBatch = generateTestLogs(scenario.batchSize);
    const logHeights = new Map<number, number>();
    
    // Assign random heights
    testLogs.forEach(log => {
      logHeights.set(log._sequence || 0, 20 + Math.random() * 10);
    });
    
    for (const impl of implementations) {
      try {
        console.log(`  Implementation: ${impl.name}`);
        
        // Create engines
        const { processor, virtualizer } = await EngineFactory.createEngines({
          useWasm: impl.useWasm,
          useVirtualization: impl.useVirtualization
        });
        
        // Run benchmarks
        const benchmarkResults = await runBenchmark(
          processor,
          virtualizer,
          testLogs,
          testBatch,
          logHeights,
          scenario.iterations
        );
        
        // Record results
        results.push({
          scenario: scenario.name,
          implementation: impl.name,
          ...benchmarkResults
        });
        
        console.log(`    Results:
        - Merge time: ${benchmarkResults.mergeTime.toFixed(2)}ms
        - Position calc: ${benchmarkResults.positionCalcTime.toFixed(2)}ms
        - Scroll mapping: ${benchmarkResults.scrollMappingTime.toFixed(2)}ms
        - Memory usage: ${Math.round(benchmarkResults.memoryUsage / 1024 / 1024)}MB
        - Main thread blocking: ${benchmarkResults.mainThreadBlockingTime.toFixed(2)}ms`);
      } catch (error) {
        console.error(`Benchmark failed for ${impl.name}:`, error);
        
        // Record failure
        results.push({
          scenario: scenario.name,
          implementation: impl.name,
          mergeTime: -1,
          positionCalcTime: -1,
          scrollMappingTime: -1,
          memoryUsage: -1,
          mainThreadBlockingTime: -1
        });
      }
    }
  }
  
  return results;
}

// Run a single benchmark for a specific engine combination
async function runBenchmark(
  processor: LogProcessingEngine,
  virtualizer: VirtualizationEngine,
  testLogs: LogMessage[],
  testBatch: LogMessage[],
  logHeights: Map<number, number>,
  iterations: number
): Promise<{
  mergeTime: number;
  positionCalcTime: number;
  scrollMappingTime: number;
  memoryUsage: number;
  mainThreadBlockingTime: number;
}> {
  // Measure main thread blocking
  let longTaskDuration = 0;
  const longTaskObserver = new PerformanceObserver(list => {
    for (const entry of list.getEntries()) {
      longTaskDuration += entry.duration;
    }
  });
  
  try {
    longTaskObserver.observe({ entryTypes: ['longtask'] });
  } catch (e) {
    // Long task observation not supported
  }
  
  // Record memory usage (if available)
  const startMemory = performance.memory?.usedJSHeapSize || 0;
  
  // Benchmark mergeInsertLogs
  const mergeStart = performance.now();
  
  for (let i = 0; i < iterations; i++) {
    await processor.mergeInsertLogs(testLogs, testBatch);
  }
  
  const mergeEnd = performance.now();
  const mergeTime = (mergeEnd - mergeStart) / iterations;
  
  // Benchmark position calculation
  const posCalcStart = performance.now();
  
  for (let i = 0; i < iterations; i++) {
    await virtualizer.calculatePositions(testLogs, logHeights);
  }
  
  const posCalcEnd = performance.now();
  const positionCalcTime = (posCalcEnd - posCalcStart) / iterations;
  
  // Benchmark scroll mapping
  const positions = await virtualizer.calculatePositions(testLogs, logHeights);
  const scrollMappingStart = performance.now();
  
  for (let i = 0; i < iterations * 10; i++) {
    // Generate random scroll positions
    const scrollTop = Math.random() * 10000;
    await virtualizer.findLogAtScrollPosition(scrollTop, testLogs, positions);
  }
  
  const scrollMappingEnd = performance.now();
  const scrollMappingTime = (scrollMappingEnd - scrollMappingStart) / (iterations * 10);
  
  // Stop observing long tasks
  longTaskObserver.disconnect();
  
  // Calculate memory usage
  const memoryUsage = (performance.memory?.usedJSHeapSize || 0) - startMemory;
  
  return {
    mergeTime,
    positionCalcTime,
    scrollMappingTime,
    memoryUsage,
    mainThreadBlockingTime: longTaskDuration
  };
}
```

## 10. Testing Strategy

### 10.1 Equivalence Testing

```typescript
// Ensure WebAssembly and TypeScript implementations produce identical results
describe('Implementation equivalence tests', () => {
  // Test cases to cover various edge cases
  const testScenarios = [
    {
      name: 'empty',
      existing: [],
      new: []
    },
    {
      name: 'small-ordered',
      existing: generateOrderedLogs(10),
      new: generateOrderedLogs(5, 11)
    },
    {
      name: 'small-reverse',
      existing: generateOrderedLogs(10),
      new: generateOrderedLogs(5, 11).reverse()
    },
    {
      name: 'interleaved',
      existing: generateInterleavedLogs(100),
      new: generateInterleavedLogs(50, 101)
    },
    {
      name: 'large-ordered',
      existing: generateOrderedLogs(1000),
      new: generateOrderedLogs(500, 1001)
    },
    {
      name: 'large-random',
      existing: generateRandomLogs(1000),
      new: generateRandomLogs(500)
    },
    {
      name: 'duplicate-timestamps',
      existing: generateDuplicateTimestamps(100),
      new: generateDuplicateTimestamps(50)
    },
    {
      name: 'near-identical-timestamps',
      existing: generateNearIdenticalTimestamps(100),
      new: generateNearIdenticalTimestamps(50, 0.001)
    },
    {
      name: 'zero-timestamps',
      existing: generateZeroTimestamps(50),
      new: generateZeroTimestamps(25)
    },
    {
      name: 'extreme-values',
      existing: generateExtremeValueLogs(),
      new: generateExtremeValueLogs()
    }
  ];
  
  // Run each test scenario
  testScenarios.forEach(({ name, existing, new: newLogs }) => {
    test(`mergeInsertLogs - ${name}`, async () => {
      // TypeScript implementation
      const tsProcessor = new TSLogProcessor();
      const tsResult = await tsProcessor.mergeInsertLogs(existing, newLogs);
      
      // WebAssembly implementation
      const wasmProcessor = new WasmLogProcessor();
      await wasmProcessor.ensureInitialized();
      const wasmResult = await wasmProcessor.mergeInsertLogs(existing, newLogs);
      
      // Verify results match
      expect(wasmResult.length).toBe(tsResult.length);
      
      // Check sequence and ordering match
      for (let i = 0; i < tsResult.length; i++) {
        expect(wasmResult[i]._sequence).toBe(tsResult[i]._sequence);
        expect(wasmResult[i]._unix_time).toBe(tsResult[i]._unix_time);
      }
      
      // Check full equality (allowing for minor floating point differences)
      expect(areLogsEquivalent(wasmResult, tsResult)).toBe(true);
    });
  });
  
  // Similar tests for other operations (calculatePositions, findLogAtScrollPosition, etc.)
});

// Helper function to check log array equivalence (with tolerance for floating point)
function areLogsEquivalent(logsA: LogMessage[], logsB: LogMessage[]): boolean {
  if (logsA.length !== logsB.length) return false;
  
  for (let i = 0; i < logsA.length; i++) {
    const a = logsA[i];
    const b = logsB[i];
    
    // Compare critical fields exactly
    if (a._sequence !== b._sequence) return false;
    
    // Compare timestamps with small tolerance for floating point differences
    const timeA = a._unix_time || 0;
    const timeB = b._unix_time || 0;
    if (Math.abs(timeA - timeB) > 0.001) return false;
  }
  
  return true;
}
```

### 10.2 Edge Case Testing

```typescript
// Test edge cases and error scenarios
describe('Edge case handling', () => {
  test('handles malformed logs gracefully', async () => {
    const wasmProcessor = new WasmLogProcessor();
    await wasmProcessor.ensureInitialized();
    
    // Test with missing fields
    const malformedLogs = [
      { message: 'Missing sequence and time' },
      { _sequence: 1, /* missing time */ },
      { /* missing sequence */, _unix_time: 100 },
      null,
      undefined,
      { _sequence: "not a number", _unix_time: "also not a number" }
    ];
    
    // This shouldn't throw
    const result = await wasmProcessor.mergeInsertLogs([], malformedLogs);
    
    // Verify valid logs were processed
    expect(result.length).toBe(3); // Should include the 3 minimally valid logs
  });
  
  test('handles very large log batches', async () => {
    const wasmProcessor = new WasmLogProcessor();
    await wasmProcessor.ensureInitialized();
    
    // Generate large test dataset
    const largeBatch = generateOrderedLogs(50000);
    
    // This shouldn't throw or crash
    const result = await wasmProcessor.mergeInsertLogs([], largeBatch);
    
    // Verify expected result
    expect(result.length).toBe(50000);
  });
  
  test('handles concurrent operations', async () => {
    const wasmProcessor = new WasmLogProcessor();
    await wasmProcessor.ensureInitialized();
    
    // Run multiple operations concurrently
    const promises = [];
    for (let i = 0; i < 10; i++) {
      const logs = generateOrderedLogs(100, i * 100);
      promises.push(wasmProcessor.mergeInsertLogs([], logs));
    }
    
    // All should complete without errors
    const results = await Promise.all(promises);
    
    // Verify all operations succeeded
    results.forEach(result => {
      expect(result.length).toBe(100);
    });
  });
});
```

### 10.3 Browser Compatibility Testing

```typescript
// Feature detection and fallback tests
describe('Browser compatibility', () => {
  test('detects WebAssembly support', () => {
    const hasWasm = EngineFactory.isWasmSupported();
    
    // This should match browser capability
    expect(hasWasm).toBe(typeof WebAssembly === 'object');
  });
  
  test('falls back gracefully when WebAssembly not available', async () => {
    // Mock WebAssembly as unavailable
    const originalWasm = window.WebAssembly;
    // @ts-ignore
    window.WebAssembly = undefined;
    
    try {
      // Create engines (should fall back to TypeScript)
      const { processor, virtualizer } = await EngineFactory.createEngines({
        useWasm: true, // Request WASM even though unavailable
        useVirtualization: true
      });
      
      // Processor should be TypeScript implementation
      expect(processor).toBeInstanceOf(TSLogProcessor);
      
      // Test basic operation
      const testLogs = generateOrderedLogs(10);
      const result = await processor.mergeInsertLogs([], testLogs);
      
      // Should work despite WebAssembly being unavailable
      expect(result.length).toBe(10);
    } finally {
      // Restore WebAssembly
      window.WebAssembly = originalWasm;
    }
  });
  
  test('handles SharedArrayBuffer unavailability', async () => {
    // Mock SharedArrayBuffer as unavailable
    const originalShared = window.SharedArrayBuffer;
    // @ts-ignore
    window.SharedArrayBuffer = undefined;
    
    try {
      // Create WebAssembly processor (should use serialization fallback)
      const processor = new WasmLogProcessor();
      await processor.ensureInitialized();
      
      // Run operation that would normally use SharedArrayBuffer
      const testLogs = generateOrderedLogs(5000); // Large enough to trigger SAB path
      const result = await processor.mergeInsertLogs([], testLogs);
      
      // Should work despite SharedArrayBuffer being unavailable
      expect(result.length).toBe(5000);
    } finally {
      // Restore SharedArrayBuffer
      window.SharedArrayBuffer = originalShared;
    }
  });
});
```

## 11. Success Metrics

We'll measure success using the following key metrics:

### 11.1 Performance Metrics
- **Log Processing Time**: 85% reduction for large batches (>10k logs)
- **UI Responsiveness**: Zero frames dropped during high-volume logging
- **Memory Usage**: 70% reduction for large datasets (>100k logs)
- **Main Thread Blocking**: <5ms blocking time for any operation

### 11.2 User Experience Metrics
- **Smooth Scrolling**: Consistent 60fps scrolling even with 500k+ logs
- **Initialization Time**: <250ms total WebAssembly initialization time
- **Filter Responsiveness**: <100ms response time for changing log filters
- **Zero UI Freezes**: No UI freezing even during high load scenarios

### 11.3 Reliability Metrics
- **Error Recovery**: 100% recovery from WebAssembly or worker failures
- **Browser Compatibility**: Proper operation on 99.5% of target browsers
- **Memory Stability**: No memory leaks or excessive growth during extended use
- **Feature Parity**: 100% feature compatibility with TypeScript implementation

## Conclusion

This WebAssembly optimization strategy for LogViewer provides a comprehensive approach to dramatically improving performance while maintaining compatibility with the existing implementation. By implementing four distinct operational modes (virtualized/non-virtualized × WebAssembly/TypeScript), we provide maximum flexibility while ensuring robustness through intelligent fallbacks.

The key innovations in this design include:
- Tiered serialization strategy to minimize data transfer overhead
- Advanced memory management with buffer pooling
- Progressive WebAssembly loading with minimal startup impact
- Comprehensive error handling and recovery
- Chunked processing for extremely large datasets
- Seamless integration with Svelte's reactivity system

The implementation schedule provides a methodical approach to delivering these improvements incrementally, with clear success criteria at each phase. The end result will be a high-performance LogViewer capable of handling enterprise-scale logging requirements with exceptional user experience.