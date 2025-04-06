import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { logStore } from '../lib/logStore';
import { 
  enableWasm, 
  isWasmEnabled,
  getWasmModule,
  shouldUseWasm
} from '../lib/wasm';
import { getWasmState, resetWasmMetrics } from '../lib/wasm-state';
import { get } from 'svelte/store';

// Mock implementations
vi.mock('../lib/wasm', () => ({
  enableWasm: vi.fn().mockResolvedValue(true),
  isWasmEnabled: vi.fn().mockReturnValue(true),
  getWasmModule: vi.fn().mockReturnValue({
    merge_insert_logs: vi.fn(function(existing, newLogs) {
      // Simplified mock that adds arrays together
      return [...existing, ...newLogs].sort((a, b) => {
        const timeA = a._unix_time || 0;
        const timeB = b._unix_time || 0;
        return timeA - timeB;
      });
    }),
    get_memory_usage: vi.fn().mockReturnValue({
      total_bytes: 1048576,
      used_bytes: 262144,
      utilization: 0.25
    })
  }),
  shouldUseWasm: vi.fn().mockReturnValue(true)
}));

vi.mock('../wailsjs/go/gui/App', () => ({
  RecordWasmLog: vi.fn(),
  RecordWasmState: vi.fn()
}));

describe('LogStore WebAssembly Integration', () => {
  beforeEach(() => {
    // Reset the log store
    logStore.clearLogs();
    resetWasmMetrics();
    vi.clearAllMocks();
  });

  it('should properly add and merge logs', async () => {
    // Enable WebAssembly
    await enableWasm(true);
    
    // Generate test logs
    const testLogs = Array(10).fill(0).map((_, i) => ({
      level: 'INFO',
      message: `Test message ${i}`,
      time: new Date(Date.now() + i * 1000).toISOString()
    }));
    
    // Add logs one by one
    for (const log of testLogs) {
      logStore.addLog(log);
    }
    
    // Allow processing to complete (async batching)
    await new Promise(resolve => setTimeout(resolve, 50));
    
    // Get current logs
    const currentLogs = get(logStore);
    
    // Should have 10 logs
    expect(currentLogs.length).toBe(10);
    
    // Logs should be sorted by time
    for (let i = 1; i < currentLogs.length; i++) {
      const prevTime = currentLogs[i-1]._unix_time || 0;
      const currTime = currentLogs[i]._unix_time || 0;
      expect(prevTime).toBeLessThanOrEqual(currTime);
    }
    
    // WebAssembly should have been used
    expect(shouldUseWasm).toHaveBeenCalled();
    expect(getWasmModule().merge_insert_logs).toHaveBeenCalled();
  });

  it('should correctly handle adding log batches', async () => {
    // Enable WebAssembly
    await enableWasm(true);
    
    // Generate two batches of logs
    const batch1 = Array(5).fill(0).map((_, i) => ({
      level: 'INFO',
      message: `Batch 1 - ${i}`,
      time: new Date(Date.now() + i * 1000).toISOString()
    }));
    
    const batch2 = Array(5).fill(0).map((_, i) => ({
      level: 'INFO',
      message: `Batch 2 - ${i}`,
      time: new Date(Date.now() + (i + 10) * 1000).toISOString() // Later times
    }));
    
    // Add first batch
    logStore.addLogBatch(batch1);
    
    // Allow processing to complete
    await new Promise(resolve => setTimeout(resolve, 50));
    
    // Verify first batch is added
    let currentLogs = get(logStore);
    expect(currentLogs.length).toBe(5);
    
    // Add second batch
    logStore.addLogBatch(batch2);
    
    // Allow processing to complete
    await new Promise(resolve => setTimeout(resolve, 50));
    
    // Verify both batches are now merged
    currentLogs = get(logStore);
    expect(currentLogs.length).toBe(10);
    
    // Verify order - batch 1 messages should come before batch 2
    const batch1Indices = currentLogs
      .map((log, index) => log.message.startsWith('Batch 1') ? index : -1)
      .filter(index => index !== -1);
      
    const batch2Indices = currentLogs
      .map((log, index) => log.message.startsWith('Batch 2') ? index : -1)
      .filter(index => index !== -1);
    
    // All batch 1 indices should be less than all batch 2 indices
    const maxBatch1Index = Math.max(...batch1Indices);
    const minBatch2Index = Math.min(...batch2Indices);
    
    expect(maxBatch1Index).toBeLessThan(minBatch2Index);
  });

  it('should track WebAssembly performance metrics', async () => {
    // Enable WebAssembly
    await enableWasm(true);
    
    // Reset metrics
    resetWasmMetrics();
    
    // Initialize logs state
    const initialMetrics = getWasmState().performanceMetrics;
    expect(initialMetrics.operationsCount).toBe(0);
    
    // Generate and add a large batch of logs
    const largeBatch = Array(1000).fill(0).map((_, i) => ({
      level: 'INFO',
      message: `Log ${i}`,
      time: new Date(Date.now() + i * 100).toISOString()
    }));
    
    logStore.addLogBatch(largeBatch);
    
    // Allow processing to complete
    await new Promise(resolve => setTimeout(resolve, 100));
    
    // Verify operation was tracked
    const metrics = getWasmState().performanceMetrics;
    expect(metrics.operationsCount).toBeGreaterThan(0);
    
    // Verify total operations count increased
    const state = getWasmState();
    expect(state.totalOperations).toBeGreaterThan(0);
    expect(state.operationsPerType.mergeInsertLogs).toBeGreaterThan(0);
  });

  it('should handle WebAssembly fallback correctly', async () => {
    // Mock WebAssembly to be disabled
    vi.mocked(isWasmEnabled).mockReturnValue(false);
    vi.mocked(shouldUseWasm).mockReturnValue(false);
    
    // Generate test logs
    const testLogs = Array(10).fill(0).map((_, i) => ({
      level: 'INFO',
      message: `Test message ${i}`,
      time: new Date(Date.now() + i * 1000).toISOString()
    }));
    
    // Add logs
    logStore.addLogBatch(testLogs);
    
    // Allow processing to complete
    await new Promise(resolve => setTimeout(resolve, 50));
    
    // Verify logs were added (using TypeScript fallback)
    const currentLogs = get(logStore);
    expect(currentLogs.length).toBe(10);
    
    // WebAssembly module should not have been called
    expect(getWasmModule().merge_insert_logs).not.toHaveBeenCalled();
  });
});