# Langkit WebAssembly Integration for Optimization Preliminary Design Specification

## 1. Overview

This document provides comprehensive specifications for implementing WebAssembly optimization for high-performance log processing in Langkit. The implementation follows a pragmatic approach that focuses on maximizing benefits while minimizing risk, aiming to achieve "90% of the benefits with 10% of the effort."

### 1.1 Design Goals

- **Performance Enhancement**: Use WebAssembly to optimize critical performance bottlenecks
- **Risk Mitigation**: Maintain functionality if WebAssembly isn't available
- **Diagnostic Integration**: Full integration with existing crash reporting infrastructure
- **Opt-In Enhancement**: Implementation as an optional feature that can be toggled
- **Progressive Performance**: Adaptive thresholds that adjust based on real-world performance data

### 1.2 Target Functions

Primary optimization target:
- `mergeInsertLogs` in logStore.ts - Critical function for chronological ordering of logs

Secondary target (future consideration):
- `findLogAtScrollPosition` in LogViewer.svelte - Used during scrolling operations

## 2. Architecture Overview

```
Frontend (Svelte/TypeScript)                  Backend (Go)
┌─────────────────────────────┐              ┌─────────────────────────────┐
│                             │              │                             │
│  ┌─────────┐    ┌─────────┐ │    Events    │  ┌─────────┐   ┌─────────┐  │
│  │LogStore │───▶│TS Impl. │◀┼──────┐       │  │         │   │ Crash   │  │
│  └─────────┘    └─────────┘ │      │       │  │         │   │ Report  │  │
│       │                     │      │       │  │ GUI     │   │ System  │  │
│       ▼                     │      └───────┼─▶│ Handler │──▶│         │  │
│  ┌─────────┐    ┌─────────┐ │   RecordWasm │  │         │   │         │  │
│  │Wasm     │───▶│WASM Impl│ │      Log/    │  │         │   │         │  │
│  │Logger   │    └─────────┘ │      State   │  │         │   │         │  │
│  └─────────┘        ▲      │              │  └─────────┘   └─────────┘  │
│       │             │      │              │                             │
│  ┌─────────┐    ┌─────────┐ │              └─────────────────────────────┘
│  │Wasm     │───▶│WASM     │ │
│  │State    │    │Module   │ │
│  └─────────┘    └─────────┘ │
│                             │
└─────────────────────────────┘
```

### 2.1 Component Overview

1. **WebAssembly Module**: Rust implementation of target functions
2. **WebAssembly State Tracker**: Records metrics, errors, and performance data
3. **WebAssembly Logger**: Dedicated logging system for WebAssembly operations
4. **Wrapper Functions**: TypeScript functions that delegate to WebAssembly or TypeScript implementations based on conditions
5. **Backend Integration**: Methods to collect WebAssembly logs and state in crash reports

## 3. WebAssembly Module Implementation

### 3.1 Rust Implementation of `mergeInsertLogs`

```rust
// lib.rs
use wasm_bindgen::prelude::*;
use serde::{Serialize, Deserialize};
use js_sys::Error;

#[wasm_bindgen]
extern "C" {
    #[wasm_bindgen(js_namespace = console)]
    fn log(s: &str);
}

#[derive(Serialize, Deserialize, Clone)]
pub struct LogMessage {
    level: Option<String>,
    message: Option<String>,
    time: Option<String>,
    behavior: Option<String>,
    #[serde(rename = "_sequence")]
    sequence: Option<u32>,
    #[serde(rename = "_unix_time")]
    unix_time: Option<f64>,
    // Additional fields with serialization control
    #[serde(rename = "_original_time", skip_serializing_if = "Option::is_none")]
    original_time: Option<String>,
    #[serde(rename = "_visible", skip_serializing_if = "Option::is_none")]
    visible: Option<bool>,
    #[serde(rename = "_height", skip_serializing_if = "Option::is_none")]
    height: Option<f64>,
}

#[derive(Serialize, Deserialize)]
pub struct MemoryInfo {
    total_bytes: usize,
    used_bytes: usize,
    utilization: f64,
}

#[wasm_bindgen]
pub fn merge_insert_logs(existing_logs_js: JsValue, new_logs_js: JsValue) -> Result<JsValue, JsValue> {
    // Handle empty arrays as special cases
    if js_sys::Array::is_array(&new_logs_js) && js_sys::Array::from(&new_logs_js).length() == 0 {
        return Ok(existing_logs_js);
    }
    
    if js_sys::Array::is_array(&existing_logs_js) && js_sys::Array::from(&existing_logs_js).length() == 0 {
        return Ok(new_logs_js);
    }
    
    // Deserialize logs with error handling
    let existing_logs: Vec<LogMessage> = match serde_wasm_bindgen::from_value(existing_logs_js) {
        Ok(logs) => logs,
        Err(e) => return Err(Error::new(&format!("Failed to deserialize existing logs: {:?}", e)).into()),
    };
    
    let mut new_logs: Vec<LogMessage> = match serde_wasm_bindgen::from_value(new_logs_js) {
        Ok(logs) => logs,
        Err(e) => return Err(Error::new(&format!("Failed to deserialize new logs: {:?}", e)).into()),
    };
    
    // Sort new logs efficiently
    new_logs.sort_by(|a, b| {
        let time_a = a.unix_time.unwrap_or(0.0);
        let time_b = b.unix_time.unwrap_or(0.0);
        
        // Compare timestamps first
        match time_a.partial_cmp(&time_b) {
            Some(std::cmp::Ordering::Equal) => {
                // If timestamps are equal, use sequence as tie-breaker
                let seq_a = a.sequence.unwrap_or(0);
                let seq_b = b.sequence.unwrap_or(0);
                seq_a.cmp(&seq_b)
            },
            Some(ordering) => ordering,
            None => std::cmp::Ordering::Equal, // Handle NaN values
        }
    });
    
    // Efficiently merge the two sorted arrays
    let mut result = Vec::with_capacity(existing_logs.len() + new_logs.len());
    let mut i = 0;
    let mut j = 0;
    
    while i < existing_logs.len() && j < new_logs.len() {
        let time_a = existing_logs[i].unix_time.unwrap_or(0.0);
        let time_b = new_logs[j].unix_time.unwrap_or(0.0);
        
        // Compare timestamps with safe handling for NaN values
        match time_a.partial_cmp(&time_b) {
            Some(std::cmp::Ordering::Less) | Some(std::cmp::Ordering::Equal) => {
                result.push(existing_logs[i].clone());
                i += 1;
            },
            Some(std::cmp::Ordering::Greater) => {
                result.push(new_logs[j].clone());
                j += 1;
            },
            None => {
                // Handle NaN values by preferring existing logs
                result.push(existing_logs[i].clone());
                i += 1;
            }
        }
    }
    
    // Add any remaining entries
    while i < existing_logs.len() {
        result.push(existing_logs[i].clone());
        i += 1;
    }
    
    while j < new_logs.len() {
        result.push(new_logs[j].clone());
        j += 1;
    }
    
    // Serialize back to JsValue with error handling
    match serde_wasm_bindgen::to_value(&result) {
        Ok(js_array) => Ok(js_array),
        Err(e) => Err(Error::new(&format!("Failed to serialize result: {:?}", e)).into()),
    }
}

// Memory management utilities
#[wasm_bindgen]
pub fn get_memory_usage() -> JsValue {
    let memory = wasm_bindgen::memory();
    let total_pages = memory.pages();
    let total_bytes = total_pages * 65536; // WebAssembly page size
    
    // Simple estimate of used memory - in a real impl we'd track allocations
    let used_bytes = total_bytes / 2; // Conservative estimate, could be improved
    
    let memory_info = MemoryInfo {
        total_bytes,
        used_bytes,
        utilization: used_bytes as f64 / total_bytes as f64,
    };
    
    match serde_wasm_bindgen::to_value(&memory_info) {
        Ok(js_value) => js_value,
        Err(_) => JsValue::NULL,
    }
}

// Force garbage collection (placeholder for memory management)
#[wasm_bindgen]
pub fn force_garbage_collection() {
    log("WebAssembly garbage collection requested");
    // In a more complex implementation, we'd drop caches here
}
```

### 3.2 Cargo.toml Configuration

```toml
[package]
name = "log-engine"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["cdylib"]

[dependencies]
wasm-bindgen = "0.2.84"
js-sys = "0.3.61"
serde = { version = "1.0", features = ["derive"] }
serde-wasm-bindgen = "0.4"
wasm-bindgen-futures = "0.4.34"
web-sys = { version = "0.3.61", features = ["console"] }
```

### 3.3 Build Script

```bash
#!/bin/bash
# build-wasm.sh - Build script for WebAssembly module

# Check that wasm-pack is installed
if ! command -v wasm-pack &> /dev/null; then
    echo "Error: wasm-pack is not installed. Please install it with 'cargo install wasm-pack'"
    exit 1
fi

# Build the WebAssembly module optimized for size
cd src/wasm
echo "Building WebAssembly module..."
wasm-pack build --target web -- -Z build-std=panic_abort,std -Z build-std-features=panic_immediate_abort

# Copy the built files to the right location
echo "Copying WebAssembly files to public directory..."
mkdir -p ../../public/wasm
cp pkg/log_engine_bg.wasm ../../public/wasm/
cp pkg/log_engine.js ../../public/wasm/

# Log the size of the WebAssembly file
wasm_size=$(du -h pkg/log_engine_bg.wasm | cut -f1)
echo "WebAssembly module built successfully (size: $wasm_size)"
```

## 4. Frontend Integration Components

### 4.1 WebAssembly Logger

```typescript
// src/lib/wasm-logger.ts
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
    
    // Also send to backend via Wails
    this.relayToCrashReporter(entry);
    
    // Local console output with appropriate level
    this.consoleOutput(entry);
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
      window.go.gui.App.RecordWasmLog(JSON.stringify(entry));
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
```

### 4.2 WebAssembly State Manager

```typescript
// src/lib/wasm-state.ts
import { wasmLogger, WasmLogLevel } from './wasm-logger';

export enum WasmInitStatus {
  NOT_STARTED = "not_started",
  INITIALIZING = "initializing",
  SUCCESS = "success",
  FAILED = "failed"
}

export interface WasmState {
  initStatus: WasmInitStatus;
  initTime?: number;              // Time taken to initialize (ms)
  lastUsed?: number;              // Last time WASM was used
  totalOperations: number;        // Total operations performed
  operationsPerType: Record<string, number>; // Count by operation type
  lastError?: Error;              // Last error that occurred
  memoryUsage?: {
    total: number;                // Total WASM memory (bytes)
    used: number;                 // Used WASM memory (bytes)
    utilization: number;          // Used/Total ratio
  };
  performanceMetrics: {
    avgWasmTime: number;          // Running average of WASM execution time
    avgTsTime: number;            // Running average of TS execution time
    speedupRatio: number;         // TS time / WASM time
    operationsCount: number;      // Number of operations measured
  };
}

// Initial state
const initialState: WasmState = {
  initStatus: WasmInitStatus.NOT_STARTED,
  totalOperations: 0,
  operationsPerType: {},
  performanceMetrics: {
    avgWasmTime: 0,
    avgTsTime: 0,
    speedupRatio: 0,
    operationsCount: 0
  }
};

let wasmState: WasmState = { ...initialState };

export function getWasmState(): WasmState {
  return { ...wasmState };
}

export function resetWasmMetrics(): void {
  wasmState.performanceMetrics = {
    avgWasmTime: 0,
    avgTsTime: 0,
    speedupRatio: 0,
    operationsCount: 0
  };
  wasmState.totalOperations = 0;
  wasmState.operationsPerType = {};
  
  wasmLogger.log(
    WasmLogLevel.INFO,
    'metrics',
    'WebAssembly performance metrics reset'
  );
  
  // Report reset state to backend
  reportWasmState();
}

// Update performance metrics after each operation
export function updatePerformanceMetrics(wasmTime: number, tsTime: number, logCount: number): void {
  const m = wasmState.performanceMetrics;
  
  // Update running averages
  const newCount = m.operationsCount + 1;
  m.avgWasmTime = ((m.avgWasmTime * m.operationsCount) + wasmTime) / newCount;
  m.avgTsTime = ((m.avgTsTime * m.operationsCount) + tsTime) / newCount;
  m.operationsCount = newCount;
  
  // Calculate speedup ratio
  m.speedupRatio = m.avgTsTime / m.avgWasmTime;
  
  wasmLogger.log(
    WasmLogLevel.DEBUG,
    'metrics',
    `Updated performance metrics`,
    {
      avgWasmTime: m.avgWasmTime.toFixed(2),
      avgTsTime: m.avgTsTime.toFixed(2),
      speedupRatio: m.speedupRatio.toFixed(2),
      operationsCount: m.operationsCount,
      logCount
    }
  );
}

// Track operation for metrics
export function trackOperation(operation: string): void {
  wasmState.totalOperations++;
  wasmState.operationsPerType[operation] = 
    (wasmState.operationsPerType[operation] || 0) + 1;
  
  wasmState.lastUsed = Date.now();
}

// Update memory usage info
export function updateMemoryUsage(memInfo: any): void {
  wasmState.memoryUsage = {
    total: memInfo.total_bytes,
    used: memInfo.used_bytes,
    utilization: memInfo.utilization
  };
}

// Set error state
export function setWasmError(error: Error): void {
  wasmState.lastError = error;
  
  // Report updated state to backend
  reportWasmState();
}

// Report current WASM state to backend for crash reports
export function reportWasmState(): void {
  try {
    window.go.gui.App.RecordWasmState(JSON.stringify(wasmState));
  } catch (e) {
    console.error("Failed to report WASM state to backend:", e);
  }
}
```

### 4.3 WebAssembly Module Manager

```typescript
// src/lib/wasm.ts
import { wasmLogger, WasmLogLevel } from './wasm-logger';
import { 
  WasmInitStatus, 
  getWasmState, 
  reportWasmState, 
  updateMemoryUsage,
  setWasmError
} from './wasm-state';
import { settings } from './stores';
import { get } from 'svelte/store';

// Module state
let wasmModule: any = null;
let wasmInitialized = false;
let wasmEnabled = false;
let initializePromise: Promise<boolean> | null = null;

// Size threshold configuration
export const WASM_CONFIG = {
  DEFAULT_SIZE_THRESHOLD: 500,
  MIN_THRESHOLD: 100,
  MAX_THRESHOLD: 5000,
  MIN_PERFORMANCE_GAIN: 1.2
};

let WASM_SIZE_THRESHOLD = WASM_CONFIG.DEFAULT_SIZE_THRESHOLD;

export function setWasmSizeThreshold(threshold: number): void {
  WASM_SIZE_THRESHOLD = Math.max(
    WASM_CONFIG.MIN_THRESHOLD,
    Math.min(threshold, WASM_CONFIG.MAX_THRESHOLD)
  );
  
  wasmLogger.log(
    WasmLogLevel.INFO,
    'config',
    `WASM size threshold set to ${WASM_SIZE_THRESHOLD}`
  );
}

export function getWasmSizeThreshold(): number {
  return WASM_SIZE_THRESHOLD;
}

export function enableWasm(enabled: boolean): Promise<boolean> {
  wasmEnabled = enabled;
  
  wasmLogger.log(
    WasmLogLevel.INFO,
    'config',
    `WebAssembly ${enabled ? 'enabled' : 'disabled'}`
  );
  
  if (enabled && !wasmInitialized && !initializePromise) {
    return initializeWasm();
  }
  
  reportWasmState();
  return Promise.resolve(wasmInitialized);
}

export function isWasmEnabled(): boolean {
  return wasmEnabled && wasmInitialized;
}

export function isWasmSupported(): boolean {
  return typeof WebAssembly === 'object' && 
         typeof WebAssembly.instantiate === 'function';
}

export function getWasmModule(): any {
  return wasmModule;
}

export async function initializeWasm(): Promise<boolean> {
  if (initializePromise) return initializePromise;
  
  // Update state to initializing
  const wasmState = getWasmState();
  if (wasmState.initStatus === WasmInitStatus.SUCCESS) {
    return true;
  }
  
  initializePromise = new Promise<boolean>(async (resolve) => {
    if (!wasmEnabled) {
      resolve(false);
      return;
    }
    
    const startTime = performance.now();
    wasmLogger.log(WasmLogLevel.INFO, 'init', 'Initializing WebAssembly module');
    
    try {
      if (!isWasmSupported()) {
        throw new Error("WebAssembly not supported in this browser");
      }
      
      // Dynamic import of WebAssembly module
      const module = await import('./wasm/log_engine.js');
      await module.default();
      
      wasmModule = module;
      wasmInitialized = true;
      
      const endTime = performance.now();
      const initTime = endTime - startTime;
      
      // Log successful initialization with metrics
      wasmLogger.log(
        WasmLogLevel.INFO, 
        'init', 
        'WebAssembly module initialized successfully', 
        {
          initTime,
          wasmSize: getWasmSize()
        }
      );
      
      // Schedule memory usage check
      scheduleMemoryCheck();
      
      // Initial threshold from settings
      const $settings = get(settings);
      if ($settings.wasmSizeThreshold) {
        setWasmSizeThreshold($settings.wasmSizeThreshold);
      }
      
      resolve(true);
    } catch (error) {
      const endTime = performance.now();
      const initTime = endTime - startTime;
      
      wasmInitialized = false;
      setWasmError(error);
      
      wasmLogger.log(
        WasmLogLevel.ERROR, 
        'init', 
        `WebAssembly initialization failed: ${error.message}`,
        { initTime }
      );
      
      resolve(false);
    } finally {
      // Report initialization status to backend for crash reports
      reportWasmState();
    }
  });
  
  return initializePromise;
}

// Schedule regular memory checks when WASM is in use
function scheduleMemoryCheck() {
  if (!wasmInitialized || !wasmModule) return;
  
  // Check memory usage every 30 seconds while module is initialized
  setInterval(() => {
    const wasmState = getWasmState();
    if (wasmModule && wasmState.lastUsed && Date.now() - wasmState.lastUsed < 300000) {
      try {
        const memoryInfo = wasmModule.get_memory_usage();
        updateMemoryUsage(memoryInfo);
        
        // Log memory info if utilization is high
        if (memoryInfo.utilization > 0.7) {
          wasmLogger.log(
            WasmLogLevel.WARN, 
            'memory', 
            `High WASM memory utilization: ${Math.round(memoryInfo.utilization * 100)}%`,
            memoryInfo
          );
        }
      } catch (e) {
        wasmLogger.log(WasmLogLevel.ERROR, 'memory', `Memory check failed: ${e.message}`);
      }
    }
  }, 30000);
}

// Get WASM file size
function getWasmSize(): number {
  try {
    const resources = performance.getEntriesByType('resource');
    const wasmResource = resources.find(r => r.name.includes('log_engine_bg.wasm'));
    return wasmResource?.encodedBodySize || 0;
  } catch (e) {
    return 0;
  }
}
```

### 4.4 LogStore Integration with WebAssembly

```typescript
// src/lib/logStore.ts - Modified version with WebAssembly integration
import { writable, get, derived } from 'svelte/store';
import { settings } from './stores';
import { 
  isWasmEnabled, 
  getWasmModule, 
  getWasmSizeThreshold 
} from './wasm';
import { 
  trackOperation, 
  updatePerformanceMetrics, 
  setWasmError 
} from './wasm-state';
import { wasmLogger, WasmLogLevel } from './wasm-logger';

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
     * WebAssembly-enhanced mergeInsertLogs function
     * This is the function that will be called by other parts of the code
     */
    function mergeInsertLogs(existingLogs: LogMessage[], newLogs: LogMessage[]): LogMessage[] {
        // Track operation in WASM state
        trackOperation('mergeInsertLogs');
        
        // For small datasets, use TypeScript implementation
        const totalLogCount = existingLogs.length + newLogs.length;
        if (totalLogCount < getWasmSizeThreshold()) {
            wasmLogger.log(
                WasmLogLevel.DEBUG, 
                'threshold', 
                `Using TypeScript for small dataset (${totalLogCount} logs)`
            );
            return mergeInsertLogsTS(existingLogs, newLogs);
        }
        
        // For larger datasets, use WebAssembly if available
        if (isWasmEnabled()) {
            try {
                const wasmModule = getWasmModule();
                if (!wasmModule) {
                    throw new Error('WebAssembly module not initialized');
                }
                
                // Measure both implementations for metrics
                const wasmStartTime = performance.now();
                const result = wasmModule.merge_insert_logs(existingLogs, newLogs);
                const wasmTime = performance.now() - wasmStartTime;
                
                // Benchmark TS implementation for comparison if needed
                let tsTime = 0;
                if (Math.random() < 0.1) { // Only measure 10% of the time to reduce overhead
                    const tsStartTime = performance.now();
                    mergeInsertLogsTS(existingLogs, newLogs); // Don't use result to avoid memory overhead
                    tsTime = performance.now() - tsStartTime;
                    
                    // Update metrics with both times
                    updatePerformanceMetrics(wasmTime, tsTime, totalLogCount);
                    
                    // Log detailed metrics for large operations
                    if (totalLogCount > 1000) {
                        wasmLogger.log(
                            WasmLogLevel.INFO, 
                            'performance', 
                            `Large merge operation completed`, 
                            {
                                wasmTime: wasmTime.toFixed(2),
                                tsTime: tsTime.toFixed(2),
                                speedup: (tsTime / wasmTime).toFixed(2),
                                logCount: totalLogCount
                            },
                            'mergeInsertLogs'
                        );
                    }
                }
                
                return result;
            } catch (error) {
                // Log detailed error information
                wasmLogger.log(
                    WasmLogLevel.ERROR, 
                    'process', 
                    `WebAssembly mergeInsertLogs failed: ${error.message}`, 
                    {
                        logCount: totalLogCount,
                        errorName: error.name,
                        errorStack: error.stack
                    },
                    'mergeInsertLogs'
                );
                
                // Update error state
                setWasmError(error);
                
                // Fall back to TypeScript implementation
                return mergeInsertLogsTS(existingLogs, newLogs);
            }
        }
        
        // Default to TypeScript implementation
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
     * No longer caps logs at maxEntries - keeps all logs
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
            const mergedLogs = mergeInsertLogs(logs, batchToProcess);
            
            // Rebuild index
            rebuildIndex(mergedLogs);
            
            return mergedLogs;
        });
        
        processingBatch = false;
        
        // If more logs accumulated during processing, schedule another process
        if (pendingBatch.length > 0) {
            batchProcessTimer = setTimeout(processLogBatch, 0);
        }
    }

    // Additional logStore methods remain unchanged
    
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
    
    // ... other methods stay the same ...
    
    // Return the public API
    return {
        subscribe,
        addLog,
        addLogBatch,
        clearLogs,
        
        // Expose other methods as needed
        // ...
        
        // Derived stores for log levels
        errorLogs: derived(subscribe, ($logs) => 
            $logs.filter(log => log.level?.toUpperCase() === 'ERROR')
        ),
        warnLogs: derived(subscribe, ($logs) => 
            $logs.filter(log => log.level?.toUpperCase() === 'WARN')
        ),
        infoLogs: derived(subscribe, ($logs) => 
            $logs.filter(log => log.level?.toUpperCase() === 'INFO')
        ),
        debugLogs: derived(subscribe, ($logs) => 
            $logs.filter(log => log.level?.toUpperCase() === 'DEBUG')
        ),
        
        // Derived store to check if logs exceed max entries
        exceededMaxEntries: derived([subscribe, settings], ([$logs, $settings]) => {
            const maxEntries = $settings?.maxLogEntries || 5000;
            return $logs.length > maxEntries;
        })
    };
}

export const logStore = createLogStore();
```

## 5. Backend Integration

### 5.1 Add Methods to the App Struct in Go

```go
// internal/gui/app.go - Add methods to the App struct

// RecordWasmLog stores frontend WebAssembly logs in a dedicated buffer
func (a *App) RecordWasmLog(logJson string) {
    var logEntry map[string]interface{}
    
    if err := json.Unmarshal([]byte(logJson), &logEntry); err != nil {
        a.logger.Error().Err(err).Msg("Failed to parse WebAssembly log entry")
        return
    }
    
    // Format as zerolog entry for consistent formatting
    level := int8(zerolog.InfoLevel)
    if levelVal, ok := logEntry["level"].(float64); ok {
        // Map WebAssembly log levels to zerolog levels
        switch int(levelVal) {
        case 0: // DEBUG
            level = int8(zerolog.DebugLevel)
        case 1: // INFO
            level = int8(zerolog.InfoLevel)
        case 2: // WARN
            level = int8(zerolog.WarnLevel)
        case 3, 4: // ERROR, CRITICAL
            level = int8(zerolog.ErrorLevel)
        }
    }
    
    // Add component and operation info to structured fields
    fields := map[string]interface{}{
        "source": "wasm-frontend",
    }
    
    if component, ok := logEntry["component"].(string); ok {
        fields["component"] = component
    }
    
    if operation, ok := logEntry["operation"].(string); ok {
        fields["operation"] = operation
    }
    
    if metrics, ok := logEntry["metrics"].(map[string]interface{}); ok {
        for k, v := range metrics {
            fields["wasm_"+k] = v
        }
    }
    
    // Get the message from the log entry
    message := "WebAssembly log"
    if msg, ok := logEntry["message"].(string); ok {
        message = msg
    }
    
    // Log using the handler which will capture it for crash reports
    handler.LogFields(level, "wasm_frontend", message, fields)
}

// RecordWasmState captures WebAssembly state for crash reports
func (a *App) RecordWasmState(stateJson string) {
    // Parse JSON first for validation
    var state map[string]interface{}
    if err := json.Unmarshal([]byte(stateJson), &state); err != nil {
        a.logger.Error().Err(err).Msg("Failed to parse WebAssembly state")
        return
    }
    
    // Record the state in crash reporter if available
    if crash.Reporter != nil {
        crash.Reporter.SaveSnapshot("wasm_state", stateJson)
        
        // If the crash reporter has a dedicated method, update it
        if updater, ok := crash.Reporter.(interface{ UpdateWasmState(string) }); ok {
            updater.UpdateWasmState(stateJson)
        }
    }
    
    // Log important state changes
    if initStatus, ok := state["initStatus"].(string); ok {
        a.logger.Info().Str("status", initStatus).Msg("WebAssembly initialization status updated")
    }
    
    // Track performance metrics in app metrics
    if metrics, ok := state["performanceMetrics"].(map[string]interface{}); ok {
        if speedup, ok := metrics["speedupRatio"].(float64); ok {
            a.logger.Debug().Float64("wasm_speedup", speedup).Msg("WebAssembly performance metric updated")
        }
    }
}

// Modify ExportDebugReport to request latest WASM state
func (a *App) ExportDebugReport() error {
    a.logger.Info().Msg("Exporting debug report")
    
    // Request latest WASM state from frontend before generating report
    runtime.EventsEmit(a.ctx, "request-wasm-state")
    
    // Short delay to allow frontend to respond
    time.Sleep(100 * time.Millisecond)
    
    // Flush any pending events before generating report
    if a.throttler != nil {
        a.logger.Debug().Msg("Flushing throttler before generating debug report")
        a.throttler.SyncFlush()
    }
    
    // Existing code for debug report generation
    // ...
}
```

### 5.2 Update Crash Reporter to Include WebAssembly Scope

```go
// internal/pkg/crash/reporter.go - Add WebAssembly state tracking

// Update ExecutionScope struct to include WebAssembly fields
type ExecutionScope struct {
    // Existing fields...
    
    // WebAssembly state tracking
    WasmEnabled         bool
    WasmInitStatus      string
    WasmInitTime        time.Duration
    WasmOperations      int
    WasmMemoryUtil      float64
    WasmSpeedupRatio    float64
    WasmLastError       string
}

// Add dedicated method to parse and update WASM state
func (r *ReporterInstance) UpdateWasmState(stateJson string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    var state map[string]interface{}
    if err := json.Unmarshal([]byte(stateJson), &state); err != nil {
        log.Error().Err(err).Msg("Failed to parse WebAssembly state for reporter")
        return
    }
    
    // Update execution scope with WASM information
    r.executionScope.WasmEnabled = true
    
    if status, ok := state["initStatus"].(string); ok {
        r.executionScope.WasmInitStatus = status
    }
    
    if initTime, ok := state["initTime"].(float64); ok {
        r.executionScope.WasmInitTime = time.Duration(initTime) * time.Millisecond
    }
    
    if totalOps, ok := state["totalOperations"].(float64); ok {
        r.executionScope.WasmOperations = int(totalOps)
    }
    
    if memUsage, ok := state["memoryUsage"].(map[string]interface{}); ok {
        if util, ok := memUsage["utilization"].(float64); ok {
            r.executionScope.WasmMemoryUtil = util
        }
    }
    
    if metrics, ok := state["performanceMetrics"].(map[string]interface{}); ok {
        if speedup, ok := metrics["speedupRatio"].(float64); ok {
            r.executionScope.WasmSpeedupRatio = speedup
        }
    }
    
    if lastErr, ok := state["lastError"].(map[string]interface{}); ok {
        if msg, ok := lastErr["message"].(string); ok {
            r.executionScope.WasmLastError = msg
        }
    }
}
```

### 5.3 Add WebAssembly Section to Crash Reports

```go
// internal/pkg/crash/writer.go - Add WebAssembly section to reports

// Add to writeReportContent function
func writeReportContent(
    mode ReportMode,
    w io.Writer,
    mainErr error,      // might be nil for debug
    settings config.Settings,
    logBuffer bytes.Buffer,
    isCLI bool,
) error {
    // Existing sections...
    
    // Add WebAssembly section right after runtime information
    fmt.Fprintln(w, "WEBASSEMBLY STATUS")
    fmt.Fprintln(w, "==================")

    if Reporter != nil {
        _, execScope := Reporter.GetScopes()
        
        if execScope.WasmEnabled {
            fmt.Fprintf(w, "Enabled: Yes\n")
            fmt.Fprintf(w, "Init Status: %s\n", execScope.WasmInitStatus)
            fmt.Fprintf(w, "Init Time: %s\n", execScope.WasmInitTime)
            fmt.Fprintf(w, "Total Operations: %d\n", execScope.WasmOperations)
            fmt.Fprintf(w, "Memory Utilization: %.1f%%\n", execScope.WasmMemoryUtil * 100)
            fmt.Fprintf(w, "Speed Improvement: %.2fx\n", execScope.WasmSpeedupRatio)
            
            if execScope.WasmLastError != "" {
                fmt.Fprintf(w, "Last Error: %s\n", execScope.WasmLastError)
            }
        } else {
            fmt.Fprintln(w, "WebAssembly: Not Enabled")
        }
    } else {
        fmt.Fprintln(w, "WebAssembly: Status Unknown (Reporter not available)")
    }
    fmt.Fprintln(w, "")
    
    // Continue with existing sections...
}
```

## 6. Settings Integration

### 6.1 Update Settings Model

```typescript
// src/lib/stores.ts - Add WebAssembly settings
import { writable } from 'svelte/store';

type Settings = {
    // Existing settings...
    
    // WebAssembly settings
    useWasm: boolean;
    wasmSizeThreshold: number;
};

/* these values are irrelevant, only the default values of the backend matter */
const initSettings: Settings = {
    // Existing settings...
    
    // Default values for WebAssembly settings
    useWasm: false,
    wasmSizeThreshold: 500,
};

export const settings = writable<Settings>(initSettings);
```

### 6.2 Add Settings UI Component

```svelte
<!-- Settings.svelte - Add WebAssembly settings section -->
<div class="setting-group">
  <h3>Performance</h3>
  
  <div class="setting-item">
    <label class="toggle-switch">
      <input
        type="checkbox"
        bind:checked={$settings.useWasm}
        on:change={() => updateSettings()}
      />
      <span class="slider"></span>
    </label>
    <div class="setting-label">
      <span>Use WebAssembly optimization</span>
      <span class="setting-description">
        Improves performance for large log processing using WebAssembly.
        {#if wasmState?.initStatus === 'SUCCESS'}
          <br>
          <span class="text-success">WebAssembly active - {Math.round(wasmState.performanceMetrics.speedupRatio * 100) / 100}x speedup</span>
        {:else if wasmState?.initStatus === 'FAILED'}
          <br>
          <span class="text-warning">WebAssembly initialization failed: {wasmState.lastError?.message}</span>
        {:else if !isWasmSupported()}
          <br>
          <span class="text-warning">WebAssembly is not supported in your browser.</span>
        {/if}
      </span>
    </div>
  </div>
  
  {#if $settings.useWasm && wasmState?.initStatus === 'SUCCESS'}
    <div class="setting-item">
      <div class="w-full">
        <label class="block mb-1 text-sm">WebAssembly Size Threshold</label>
        <input
          type="range"
          min="50"
          max="2000"
          step="50"
          bind:value={$settings.wasmSizeThreshold}
          on:change={() => updateSettings()}
          class="w-full"
        />
        <div class="flex justify-between text-xs">
          <span>50 logs</span>
          <span>{$settings.wasmSizeThreshold} logs</span>
          <span>2000 logs</span>
        </div>
        <span class="text-xs text-gray-400">
          Only use WebAssembly for processing more than {$settings.wasmSizeThreshold} logs at once
        </span>
      </div>
    </div>
    
    <div class="setting-item">
      <button
        class="px-3 py-1 bg-blue-500 text-white rounded text-sm"
        on:click={() => resetWasmMetrics()}
      >
        Reset WebAssembly Performance Metrics
      </button>
      <div class="setting-label">
        <span class="text-xs">
          Reset tracked performance data to start fresh measurements
        </span>
      </div>
    </div>
  {/if}
</div>
```

### 6.3 Initialize WebAssembly in App.svelte

```typescript
// Add to App.svelte - Initialize WebAssembly when enabled in settings
import { enableWasm, getWasmState, resetWasmMetrics } from './lib/wasm-state';
import { wasmLogger, WasmLogLevel } from './lib/wasm-logger';

// In onMount or appropriate initialization function
async function initializeApplication() {
    // Existing initialization code...
    
    // Listen for settings changes to enable/disable WebAssembly
    settings.subscribe(async ($settings) => {
        if ($settings.useWasm !== undefined) {
            const wasEnabled = await enableWasm($settings.useWasm);
            
            if (wasEnabled) {
                wasmLogger.log(
                    WasmLogLevel.INFO,
                    'config', 
                    'WebAssembly enabled and initialized successfully'
                );
                
                // Set size threshold from settings
                if ($settings.wasmSizeThreshold) {
                    setWasmSizeThreshold($settings.wasmSizeThreshold);
                }
            }
        }
    });
    
    // Listen for request to send WASM state
    EventsOn("request-wasm-state", () => {
        reportWasmState();
    });
}
```

## 7. Testing Strategy

### 7.1 Unit Testing

Create a dedicated test suite for WebAssembly functionality:

```typescript
// tests/wasm.test.ts
import { mergeInsertLogs, mergeInsertLogsTS } from '../src/lib/logStore';
import { initializeWasm, isWasmEnabled } from '../src/lib/wasm';
import { getWasmState, resetWasmMetrics } from '../src/lib/wasm-state';

describe('WebAssembly Functionality', () => {
    beforeAll(async () => {
        // Initialize WebAssembly before tests
        await initializeWasm();
    });
    
    it('should initialize WebAssembly module', () => {
        expect(isWasmEnabled()).toBe(true);
        const state = getWasmState();
        expect(state.initStatus).toBe('SUCCESS');
    });
    
    it('should produce identical results between WebAssembly and TypeScript for small datasets', () => {
        // Create test dataset
        const existingLogs = generateTestLogs(10);
        const newLogs = generateTestLogs(5);
        
        // Run both implementations
        const tsResult = mergeInsertLogsTS(existingLogs, newLogs);
        const wasmResult = mergeInsertLogs(existingLogs, newLogs);
        
        // Verify results are identical
        expect(wasmResult.length).toBe(tsResult.length);
        for (let i = 0; i < wasmResult.length; i++) {
            expect(wasmResult[i]._sequence).toBe(tsResult[i]._sequence);
            expect(wasmResult[i]._unix_time).toBe(tsResult[i]._unix_time);
        }
    });
    
    it('should produce identical results for large datasets', () => {
        // Create larger test dataset
        const existingLogs = generateTestLogs(1000);
        const newLogs = generateTestLogs(500);
        
        // Run both implementations
        const tsResult = mergeInsertLogsTS(existingLogs, newLogs);
        const wasmResult = mergeInsertLogs(existingLogs, newLogs);
        
        // Verify results are identical
        expect(wasmResult.length).toBe(tsResult.length);
        for (let i = 0; i < wasmResult.length; i++) {
            expect(wasmResult[i]._sequence).toBe(tsResult[i]._sequence);
            expect(wasmResult[i]._unix_time).toBe(tsResult[i]._unix_time);
        }
    });
    
    it('should handle edge cases gracefully', () => {
        // Test with empty arrays
        expect(mergeInsertLogs([], [])).toEqual([]);
        
        // Test with one empty array
        const logs = generateTestLogs(5);
        expect(mergeInsertLogs(logs, [])).toEqual(logs);
        expect(mergeInsertLogs([], logs)).toEqual(logs);
        
        // Test with malformed logs
        const malformedLogs = [{ message: 'test' }];
        expect(() => mergeInsertLogs(malformedLogs, [])).not.toThrow();
    });
    
    it('should track performance metrics', () => {
        // Reset metrics before test
        resetWasmMetrics();
        
        // Run a large merge operation to generate metrics
        const existingLogs = generateTestLogs(2000);
        const newLogs = generateTestLogs(1000);
        mergeInsertLogs(existingLogs, newLogs);
        
        // Check that metrics were updated
        const state = getWasmState();
        expect(state.totalOperations).toBeGreaterThan(0);
        expect(state.performanceMetrics.operationsCount).toBeGreaterThan(0);
    });
    
    // Helper function to generate test logs
    function generateTestLogs(count: number): any[] {
        const logs = [];
        const baseTime = Date.now() - count * 100;
        
        for (let i = 0; i < count; i++) {
            logs.push({
                level: 'INFO',
                message: `Test message ${i}`,
                time: new Date(baseTime + i * 100).toISOString(),
                _sequence: i,
                _unix_time: baseTime + i * 100
            });
        }
        
        // Randomize the order
        return logs.sort(() => Math.random() - 0.5);
    }
});
```

### 7.2 Integration Testing

Test the WebAssembly integration with the full application:

```typescript
// tests/integration.test.ts
import { logStore } from '../src/lib/logStore';
import { enableWasm, isWasmEnabled } from '../src/lib/wasm';
import { settings } from '../src/lib/stores';
import { get } from 'svelte/store';

describe('WebAssembly Integration', () => {
    beforeEach(() => {
        // Reset state before each test
        logStore.clearLogs();
        settings.set({ ...get(settings), useWasm: false });
    });
    
    it('should respect settings toggle', async () => {
        // Disable WebAssembly
        await enableWasm(false);
        expect(isWasmEnabled()).toBe(false);
        
        // Enable WebAssembly
        await enableWasm(true);
        expect(isWasmEnabled()).toBe(true);
    });
    
    it('should correctly process log batches with WebAssembly enabled', async () => {
        // Enable WebAssembly
        await enableWasm(true);
        
        // Add a batch of logs
        const logs = [];
        for (let i = 0; i < 1000; i++) {
            logs.push({
                level: 'INFO',
                message: `Test message ${i}`,
                time: new Date(Date.now() + i * 100).toISOString()
            });
        }
        
        logStore.addLogBatch(logs);
        
        // Wait for batch processing to complete
        await new Promise(resolve => setTimeout(resolve, 100));
        
        // Verify logs were processed correctly
        const storedLogs = get(logStore);
        expect(storedLogs.length).toBe(1000);
        
        // Verify logs are in chronological order
        for (let i = 1; i < storedLogs.length; i++) {
            expect(storedLogs[i]._unix_time).toBeGreaterThanOrEqual(storedLogs[i-1]._unix_time);
        }
    });
});
```

### 7.3 Performance Benchmarking

Create a benchmark suite to measure WebAssembly performance:

```typescript
// benchmark/wasm-benchmark.ts
import { mergeInsertLogs, mergeInsertLogsTS } from '../src/lib/logStore';
import { enableWasm } from '../src/lib/wasm';

async function runBenchmarks() {
    console.log("Starting WebAssembly benchmarks...");
    
    // Generate test datasets of various sizes
    const datasets = [
        { name: "tiny", size: 100, newSize: 10 },
        { name: "small", size: 500, newSize: 50 },
        { name: "medium", size: 2000, newSize: 200 },
        { name: "large", size: 5000, newSize: 500 },
        { name: "xlarge", size: 10000, newSize: 1000 }
    ];
    
    // Generate logs
    const testData = datasets.map(dataset => ({
        name: dataset.name,
        existing: generateTestLogs(dataset.size),
        new: generateTestLogs(dataset.newSize)
    }));
    
    // First benchmark with WebAssembly disabled
    await enableWasm(false);
    console.log("\nBenchmarking with WebAssembly disabled:");
    console.log("--------------------------------------");
    
    for (const data of testData) {
        console.log(`\nDataset: ${data.name} (${data.existing.length} + ${data.new.length} logs)`);
        const tsTime = await benchmarkOperation(() => {
            mergeInsertLogs(data.existing, data.new);
        }, 5);
        console.log(`TypeScript: ${tsTime.toFixed(2)}ms (avg of 5 runs)`);
    }
    
    // Then benchmark with WebAssembly enabled
    await enableWasm(true);
    console.log("\nBenchmarking with WebAssembly enabled:");
    console.log("------------------------------------");
    
    for (const data of testData) {
        console.log(`\nDataset: ${data.name} (${data.existing.length} + ${data.new.length} logs)`);
        
        // Warm up WebAssembly
        mergeInsertLogs(data.existing, data.new);
        
        // Benchmark WebAssembly implementation
        const wasmTime = await benchmarkOperation(() => {
            mergeInsertLogs(data.existing, data.new);
        }, 5);
        
        // Benchmark TypeScript implementation for comparison
        const tsTime = await benchmarkOperation(() => {
            mergeInsertLogsTS(data.existing, data.new);
        }, 5);
        
        console.log(`WebAssembly: ${wasmTime.toFixed(2)}ms (avg of 5 runs)`);
        console.log(`TypeScript: ${tsTime.toFixed(2)}ms (avg of 5 runs)`);
        console.log(`Speedup: ${(tsTime / wasmTime).toFixed(2)}x`);
    }
}

// Helper function to benchmark an operation
async function benchmarkOperation(operation: () => void, iterations: number): Promise<number> {
    const times: number[] = [];
    
    for (let i = 0; i < iterations; i++) {
        const start = performance.now();
        operation();
        const end = performance.now();
        times.push(end - start);
    }
    
    // Return average time
    return times.reduce((sum, time) => sum + time, 0) / times.length;
}

// Helper function to generate test logs
function generateTestLogs(count: number): any[] {
    const logs = [];
    const baseTime = Date.now() - count * 100;
    
    for (let i = 0; i < count; i++) {
        logs.push({
            level: 'INFO',
            message: `Test message ${i}`,
            time: new Date(baseTime + i * 100).toISOString(),
            _sequence: i,
            _unix_time: baseTime + i * 100
        });
    }
    
    // Randomize the order
    return logs.sort(() => Math.random() - 0.5);
}

// Run the benchmarks
runBenchmarks().catch(console.error);
```

## 8. Implementation Considerations

### 8.1 Potential Challenges and Mitigations

1. **Serialization Overhead for Small Datasets**
   - **Challenge**: WebAssembly requires serialization/deserialization of data which can offset performance gains for small datasets
   - **Mitigation**: Implement adaptive thresholds that only use WebAssembly for datasets above a certain size
   - **Implementation**: The `WASM_SIZE_THRESHOLD` determines when to use WebAssembly vs. TypeScript

2. **Browser Compatibility**
   - **Challenge**: Not all browsers support WebAssembly
   - **Mitigation**: Always maintain the TypeScript implementation as a fallback
   - **Implementation**: `isWasmSupported()` check before initialization

3. **Memory Management**
   - **Challenge**: WebAssembly has limited memory management capabilities
   - **Mitigation**: Implement memory usage monitoring and garbage collection triggers
   - **Implementation**: Regular memory checks and the `force_garbage_collection()` function

4. **Debugging Difficulties**
   - **Challenge**: WebAssembly code is harder to debug than TypeScript
   - **Mitigation**: Comprehensive logging and crash reporting integration
   - **Implementation**: Dedicated `wasmLogger` and crash reporter integration

5. **Build Integration**
   - **Challenge**: Incorporating WebAssembly builds into the application pipeline
   - **Mitigation**: Streamlined build script with proper error handling
   - **Implementation**: `build-wasm.sh` script with dependency checks

### 8.2 Performance Optimization Priorities

1. **Minimize Serialization Overhead**
   - Simplify log objects before sending to WebAssembly when possible
   - Only send fields needed for the operation (e.g., `_sequence` and `_unix_time` for sorting)

2. **Optimize Rust Implementation**
   - Use efficient sorting algorithms
   - Preallocate vectors with appropriate capacity
   - Avoid unnecessary cloning when possible

3. **Intelligent Thresholds**
   - Start with conservative thresholds
   - Adjust based on measured performance
   - Allow user customization via settings

4. **Memory Optimization**
   - Monitor WebAssembly memory usage
   - Implement manual garbage collection when needed
   - Log high memory utilization for debugging

### 8.3 Incremental Implementation Plan

1. **Phase 1: Initial Implementation**
   - Implement WebAssembly module with `merge_insert_logs` function
   - Create basic wrapper in TypeScript
   - Implement logging and crash reporting integration
   - Add settings toggle

2. **Phase 2: Performance Monitoring and Tuning**
   - Add detailed performance metrics tracking
   - Implement adaptive thresholds
   - Refine memory management
   - Enhance error handling

3. **Phase 3: Additional Functions (if successful)**
   - Implement `find_log_at_scroll_position` in WebAssembly
   - Consider other performance-critical functions
   - Further optimize memory usage and transfer

## 9. Summary

This WebAssembly implementation follows a pragmatic approach that focuses on optimizing critical performance bottlenecks while maintaining full compatibility with the existing codebase. It integrates deeply with Langkit's logging and crash reporting systems to ensure robust error handling and diagnostics.

Key benefits include:
1. Improved performance for large log processing operations
2. Comprehensive diagnostics and crash reporting
3. User-configurable settings for fine-tuning
4. Graceful fallbacks for unsupported environments

The implementation prioritizes "90% of benefits for 10% of effort" by focusing on the `mergeInsertLogs` function, which is the most critical performance bottleneck. The TypeScript implementation is always maintained as a fallback, ensuring robustness and compatibility.

With adaptive thresholds and detailed performance metrics, the system automatically adjusts to provide optimal performance across different usage scenarios and hardware capabilities.