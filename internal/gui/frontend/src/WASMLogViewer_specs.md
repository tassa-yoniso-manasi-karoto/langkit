# Langkit WebAssembly Integration - Implementation Specification

## Key Files

### Frontend (TypeScript/Svelte)
- `/internal/gui/frontend/src/lib/wasm.ts` - Core initialization and WebAssembly management
- `/internal/gui/frontend/src/lib/wasm-state.ts` - State tracking and performance metrics
- `/internal/gui/frontend/src/lib/wasm-logger.ts` - Dedicated logging for WebAssembly operations
- `/internal/gui/frontend/src/lib/logStore.ts` - Integration with log processing
- `/internal/gui/frontend/src/components/WasmPerformanceDashboard.svelte` - UI for monitoring performance
- `/internal/gui/frontend/src/components/Settings.svelte` - Settings integration
- `/internal/gui/frontend/src/App.svelte` - Application initialization and event handling

### Rust/WebAssembly
- `/internal/gui/frontend/src/wasm/Cargo.toml` - Rust project definition
- `/internal/gui/frontend/src/wasm/src/lib.rs` - WebAssembly implementation
- `/internal/gui/frontend/src/wasm/tests/lib_test.rs` - Rust tests

### Build & Testing
- `/scripts/build-wasm.sh` - WebAssembly build script
- `/internal/gui/frontend/src/tests/logstore-wasm.test.ts` - WebAssembly unit tests
- `/internal/gui/frontend/src/tests/wasm-e2e.test.ts` - End-to-end tests
- `/internal/gui/frontend/src/benchmarks/wasm-performance.bench.ts` - Performance benchmarking

### Backend (Go)
- `/internal/gui/app.go` - Backend WebAssembly state handling
- `/internal/pkg/crash/reporter.go` - Crash reporting integration
- `/internal/pkg/crash/writer.go` - Debug report generation with WebAssembly state

## 1. Overview

This document describes the current WebAssembly implementation in Langkit, which optimizes critical performance bottlenecks in log processing. Following a pragmatic approach of achieving "90% of the benefits with 10% of the effort," WebAssembly is used to enhance specific high-impact functions while maintaining compatibility with environments where WebAssembly is unavailable.

## 2. Implementation Structure

The implementation follows a modular architecture with clear separation of concerns:

```
Frontend (Svelte/TypeScript)                  Backend (Go)
┌─────────────────────────────┐              ┌─────────────────────────────┐
│                             │              │                             │
│  ┌─────────┐    ┌─────────┐ │              │  ┌─────────┐                │
│  │LogStore │───▶│TS Impl. │ │              │  │  CORE / │                │
│  └─────────┘    └─────────┘ │              │  │ Log Srcs│                │
│       │                     │              │  └────┬────┘                │
│       ▼                     │              │       │                     │
│  ┌─────────┐    ┌─────────┐ │ RecordWasm   │  ┌────▼────┐   ┌─────────┐  │
│  │Wasm     │───▶│WASM Impl│ │ Log/State    │  │ GUI     │   │ Crash   │  │
│  │Logger   │    └─────────┘ │──────────▶   │  │ Handler │──▶│ Report  │  │
│  └─────────┘        ▲       │              │  │         │   │ System  │  │
│       │             │       │              │  └────┬────┘   └─────────┘  │
│       │             │       │              │       │                     │
│  ┌─────────┐    ┌─────────┐ │              │                             │
│  │Wasm     │───▶│WASM     │ │              │                             │
│  │State    │    │Module   │ │              │                             │
│  └─────────┘    └─────────┘ │              │                             │
│                             │              │                             │
└─────────────────────────────┘              └─────────────────────────────┘
```

### 2.1 Core Components

1. **WebAssembly Module** (`src/wasm/src/lib.rs`): 
   - Rust implementation of `merge_insert_logs` for efficient log merging
   - Memory management utilities
   - Error handling with proper JavaScript integration

2. **State Management** (`lib/wasm-state.ts`):
   - Tracks WebAssembly initialization status
   - Records performance metrics
   - Manages memory usage statistics
   - Provides diagnostic information

3. **Logger** (`lib/wasm-logger.ts`):
   - Dedicated logging system for WebAssembly operations
   - Integration with backend crash reporting
   - Structured log entries with component and metrics information

4. **Main Integration** (`lib/wasm.ts`):
   - Initialization and configuration
   - Adaptive threshold logic for optimal WebAssembly utilization
   - Memory usage monitoring
   - Settings integration

5. **LogStore Integration** (`lib/logStore.ts`):
   - Smart delegation between TypeScript and WebAssembly implementations
   - Performance measurements for benchmarking
   - Error handling and fallback logic

6. **UI Components** (`components/WasmPerformanceDashboard.svelte`):
   - Real-time performance metrics display
   - Memory usage visualization
   - Settings controls

7. **Backend Integration**:
   - Crash reporting with WebAssembly state inclusion
   - Event handling for diagnostics
   - Memory usage monitoring

## 3. Optimized Functions

Currently optimized functions:

| Function | File | Description | Acceleration |
|----------|------|-------------|-------------|
| `mergeInsertLogs` | logStore.ts | Merges and chronologically sorts log entries | 2-10x |

The implementation uses smart decision-making to only use WebAssembly when beneficial:
- Uses TypeScript for small datasets (< 500 logs by default)
- Adaptively determines threshold based on measured performance
- Falls back to TypeScript when WebAssembly is unavailable

## 4. Build System Integration

### 4.1 Development Workflow

The WebAssembly module is integrated into the standard build process:

1. **WebAssembly Build Script** (`scripts/build-wasm.sh`):
   - Compiles Rust to WebAssembly
   - Optimizes for size and performance
   - Generates build metadata for cache busting
   - Copies output to correct public directory

2. **NPM Scripts**:
   ```
   "build": "npm run build:wasm && vite build"
   "build:wasm": "bash ../../../../../scripts/build-wasm.sh"
   "dev:wasm": "nodemon --watch src/wasm -e rs --exec 'npm run build:wasm'"
   ```

3. **Wails Integration**:
   - `wails build` automatically triggers WebAssembly compilation
   - Proper error handling ensures build proceeds even if WebAssembly compilation fails

### 4.2 Build Requirements

The WebAssembly build requires:
- Rust toolchain (rustc, cargo)
- wasm-pack (`cargo install wasm-pack`)
- wasm32-unknown-unknown target (`rustup target add wasm32-unknown-unknown`)

### 4.3 Output Files

The build process generates:
- `public/wasm/log_engine_bg.wasm` - The compiled WebAssembly binary
- `public/wasm/log_engine.js` - JavaScript glue code
- `public/wasm/build-info.json` - Build metadata for versioning and cache busting

## 5. Testing & Performance Analysis

### 5.1 Testing Infrastructure

Testing is built into the development workflow:

```
"test": "vitest run"
"test:watch": "vitest"
"test:wasm": "vitest run logstore-wasm"
"benchmark": "tsx src/benchmarks/wasm-performance.bench.ts"
```

### 5.2 Test Files

- `tests/logstore-wasm.test.ts` - Unit tests for WebAssembly functionality
- `tests/wasm-e2e.test.ts` - End-to-end tests for WebAssembly integration
- `benchmarks/wasm-performance.bench.ts` - Performance benchmarking

### 5.3 Benchmark Results

Typical performance improvements measured across different log volumes:

| Dataset Size | TypeScript | WebAssembly | Speedup |
|--------------|------------|-------------|---------|
| Small (550)  | 0.5-1ms    | 0.4-0.8ms   | 1.2-1.5x |
| Medium (2200)| 3-5ms      | 1-2ms       | 2-3x    |
| Large (5500) | 25-40ms    | 4-8ms       | 5-7x    |
| XL (11000)   | 80-120ms   | 10-15ms     | 8-10x   |

*Note: Actual performance varies by browser and hardware*

## 6. User Settings

WebAssembly functionality is configurable through the settings UI:

1. **Enable/Disable** - Toggle WebAssembly optimization
2. **Size Threshold** - Configure minimum log count for WebAssembly use (50-5000)
3. **Performance Dashboard** - View real-time metrics

Settings are persisted and applied on application restart.

## 7. Diagnostic Integration

WebAssembly state is included in crash reports and debug exports:

```
WEBASSEMBLY STATUS
==================
Status: success
Operations: 152
Speed Ratio: 4.23x
Memory Usage: 23.5%
```

## 8. Production Considerations

1. **Browser Support**:
   - Works in all modern browsers that support WebAssembly
   - Gracefully degrades to TypeScript implementation in unsupported environments

2. **Memory Usage**:
   - Typically consumes 1-4MB of WebAssembly memory
   - Automatic garbage collection
   - Memory availability checking prevents crashes

3. **Error Handling**:
   - Comprehensive error trapping
   - Clean fallbacks to TypeScript implementation
   - Detailed diagnostic logging

## 9. Future Expansion

The current implementation focuses on the highest value function (`mergeInsertLogs`). Future candidates for optimization include:

1. `findLogAtScrollPosition` - For improved scrolling performance with large log volumes
2. Log filtering operations - For faster filter application with complex conditions
3. Search operations - For near-instant search across large log volumes

## 10. Versioning and Cache Management

WebAssembly builds include version information to enable proper cache invalidation:

```json
{
  "version": "0.1.0",
  "timestamp": 1712208335,
  "buildDate": "2025-04-04T12:32:15Z"
}
```

The cache system:
- Automatically refreshes when new versions are detected
- Uses timestamp fallbacks if version info is unavailable
- Diagnostic information includes build metadata