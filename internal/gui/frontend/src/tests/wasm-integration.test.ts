// src/tests/wasm-integration.test.ts

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { 
  isWasmEnabled, 
  enableWasm, 
  getWasmModule, 
  shouldUseWasm 
} from '../lib/wasm';
import { 
  wasmState, 
  getWasmState, 
  resetWasmMetrics, 
  updatePerformanceMetrics 
} from '../lib/wasm-state';
import { wasmLogger } from '../lib/wasm-logger';

// Mock global window.go
vi.mock('../wailsjs/go/gui/App', () => ({
  RecordWasmLog: vi.fn(),
  RecordWasmState: vi.fn()
}));

// Mock the WebAssembly module
const mockMergeInsertLogs = vi.fn();
const mockGetMemoryUsage = vi.fn(() => ({
  total_bytes: 1048576,
  used_bytes: 524288,
  utilization: 0.5,
  peak_bytes: 786432,
  allocation_count: 42
}));
const mockForceGarbageCollection = vi.fn();
const mockEstimateMemoryForLogs = vi.fn(() => ({
  estimated_bytes: 250000,
  available_bytes: 500000,
  would_fit: true,
  utilization_after: 0.75
}));

// Create a mock for the WASM module
const mockWasmModule = {
  merge_insert_logs: mockMergeInsertLogs,
  get_memory_usage: mockGetMemoryUsage,
  force_garbage_collection: mockForceGarbageCollection,
  estimate_memory_for_logs: mockEstimateMemoryForLogs
};

// Mock the getter for the module
vi.mock('../lib/wasm', async () => {
  const actual = await vi.importActual('../lib/wasm');
  return {
    ...actual,
    getWasmModule: vi.fn(() => mockWasmModule),
    isWasmEnabled: vi.fn(() => true)
  };
});

describe('WebAssembly Integration', () => {
  beforeEach(() => {
    // Reset the mock functions
    vi.clearAllMocks();
    
    // Reset wasm state
    resetWasmMetrics();
  });

  it('should track operations correctly', () => {
    const initialOperations = getWasmState().totalOperations;
    
    // Track a few operations
    for (let i = 0; i < 5; i++) {
      wasmState.trackOperation('testOperation');
    }
    
    const finalState = getWasmState();
    
    // Should have 5 more operations than before
    expect(finalState.totalOperations).toBe(initialOperations + 5);
    expect(finalState.operationsPerType.testOperation).toBe(5);
    
    // Should have lastUsed timestamp
    expect(finalState.lastUsed).toBeDefined();
    expect(typeof finalState.lastUsed).toBe('number');
  });

  it('should update performance metrics accurately', () => {
    const initialMetrics = getWasmState().performanceMetrics;
    
    // Update metrics with some test values
    updatePerformanceMetrics(10, 20, 100);
    
    const updatedMetrics = getWasmState().performanceMetrics;
    
    // Check metrics were updated
    expect(updatedMetrics.avgWasmTime).toBe(10);
    expect(updatedMetrics.avgTsTime).toBe(20);
    expect(updatedMetrics.speedupRatio).toBe(2); // 20/10
    expect(updatedMetrics.operationsCount).toBe(1);
    
    // Add another measurement
    updatePerformanceMetrics(20, 50, 200);
    
    const finalMetrics = getWasmState().performanceMetrics;
    
    // Check running average calculation is correct
    expect(finalMetrics.avgWasmTime).toBe(15); // (10 + 20) / 2
    expect(finalMetrics.avgTsTime).toBe(35); // (20 + 50) / 2
    expect(finalMetrics.speedupRatio).toBe(35/15); // ~2.33
    expect(finalMetrics.operationsCount).toBe(2);
  });

  it('should make correct decisions for when to use WebAssembly', () => {
    // Set up test state with performance advantage for WASM
    updatePerformanceMetrics(10, 30, 500); // WASM 3x faster
    updatePerformanceMetrics(12, 36, 600); // Maintain 3x ratio
    updatePerformanceMetrics(11, 33, 550); // Maintain 3x ratio
    updatePerformanceMetrics(10, 30, 500); // Maintain 3x ratio
    updatePerformanceMetrics(12, 36, 600); // Maintain 3x ratio
    
    // Test with different log counts
    expect(shouldUseWasm(100)).toBe(false); // Too small, below default threshold
    expect(shouldUseWasm(500)).toBe(true);  // At default threshold with good speedup
    expect(shouldUseWasm(5000)).toBe(true); // Large dataset, definite use case
    
    // Now make WASM only slightly faster
    resetWasmMetrics();
    updatePerformanceMetrics(10, 11, 500); // Only 1.1x faster
    updatePerformanceMetrics(12, 13, 600);
    updatePerformanceMetrics(11, 12, 550);
    updatePerformanceMetrics(10, 11, 500);
    updatePerformanceMetrics(12, 13, 600);
    
    // Test decisions with minimal speedup
    expect(shouldUseWasm(500)).toBe(false);  // At threshold but not enough speedup
    expect(shouldUseWasm(1000)).toBe(false); // Larger but still not worth it
    
    // Test with memory pressure
    // First restore good performance
    resetWasmMetrics();
    updatePerformanceMetrics(10, 30, 500); // WASM 3x faster again
    updatePerformanceMetrics(12, 36, 600);
    updatePerformanceMetrics(11, 33, 550);
    updatePerformanceMetrics(10, 30, 500);
    updatePerformanceMetrics(12, 36, 600);
    
    // Now simulate high memory pressure
    wasmState.memoryUsage = {
      total: 1048576,
      used: 943718,  // 90% used
      utilization: 0.9, 
      peak_bytes: 943718,
      allocation_count: 100
    };
    
    // Should avoid WASM due to memory pressure
    expect(shouldUseWasm(5000)).toBe(false);
  });

  it('should handle memory management correctly', () => {
    // Mock the memory usage results
    mockGetMemoryUsage.mockReturnValueOnce({
      total_bytes: 1048576,
      used_bytes: 262144,  // 25% used
      utilization: 0.25,
      peak_bytes: 524288,
      allocation_count: 20
    }).mockReturnValueOnce({
      total_bytes: 1048576,
      used_bytes: 786432,  // 75% used
      utilization: 0.75,
      peak_bytes: 786432,
      allocation_count: 50
    }).mockReturnValueOnce({
      total_bytes: 1048576,
      used_bytes: 104858,  // 10% used after GC
      utilization: 0.1,
      peak_bytes: 786432,
      allocation_count: 5
    });
    
    // Check memory usage updates
    const memInfo1 = mockWasmModule.get_memory_usage();
    wasmState.updateMemoryUsage(memInfo1);
    
    expect(wasmState.memoryUsage.utilization).toBe(0.25);
    
    // Simulate high memory usage
    const memInfo2 = mockWasmModule.get_memory_usage();
    wasmState.updateMemoryUsage(memInfo2);
    
    expect(wasmState.memoryUsage.utilization).toBe(0.75);
    
    // Simulate garbage collection
    mockForceGarbageCollection();
    const memInfo3 = mockWasmModule.get_memory_usage();
    wasmState.updateMemoryUsage(memInfo3);
    
    expect(wasmState.memoryUsage.utilization).toBe(0.1);
    
    // Verify calls
    expect(mockGetMemoryUsage).toHaveBeenCalledTimes(3);
    expect(mockForceGarbageCollection).toHaveBeenCalledTimes(1);
  });
});