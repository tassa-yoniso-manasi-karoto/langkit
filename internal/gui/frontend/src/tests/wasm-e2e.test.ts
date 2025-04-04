/**
 * This test utility demonstrates end-to-end testing of the WebAssembly integration.
 * It intentionally avoids using test frameworks to allow running directly in a browser.
 */

import { enableWasm, getWasmModule, shouldUseWasm } from '../lib/wasm';
import { logStore } from '../lib/logStore';
import { getWasmState, resetWasmMetrics } from '../lib/wasm-state';
import { get } from 'svelte/store';

// Configure console output for test results
const testResults = {
  total: 0,
  passed: 0,
  failed: 0
};

function assert(condition: boolean, message: string) {
  testResults.total++;
  
  if (condition) {
    testResults.passed++;
    console.log(`✅ PASS: ${message}`);
  } else {
    testResults.failed++;
    console.error(`❌ FAIL: ${message}`);
  }
}

async function runE2ETests() {
  console.log('Starting WebAssembly E2E tests...');
  
  // Test initialization
  console.log('\n=== Testing initialization ===');
  const wasmEnabled = await enableWasm(true);
  
  assert(wasmEnabled, 'WebAssembly should initialize successfully');
  
  const wasmModule = getWasmModule();
  assert(!!wasmModule, 'WebAssembly module should be available');
  
  if (!wasmEnabled || !wasmModule) {
    console.error('Cannot continue tests without WebAssembly initialized');
    return;
  }
  
  // Test memory management
  console.log('\n=== Testing memory management ===');
  const memInfo = wasmModule.get_memory_usage();
  
  assert(!!memInfo, 'Should get memory info');
  assert(typeof memInfo.total_bytes === 'number', 'Memory info should include total_bytes');
  assert(typeof memInfo.used_bytes === 'number', 'Memory info should include used_bytes');
  assert(typeof memInfo.utilization === 'number', 'Memory info should include utilization');
  
  // Test log merging
  console.log('\n=== Testing log merging ===');
  
  // Reset metrics and clear logs
  resetWasmMetrics();
  logStore.clearLogs();
  
  // Generate test batch
  const generateLogs = (count: number, startTime = Date.now()) => {
    return Array(count).fill(0).map((_, i) => ({
      level: 'INFO',
      message: `Test log ${i}`,
      time: new Date(startTime + i * 1000).toISOString()
    }));
  };
  
  // Add first batch
  const batch1 = generateLogs(100);
  logStore.addLogBatch(batch1);
  
  // Wait for processing
  await new Promise(resolve => setTimeout(resolve, 100));
  
  // Check logs were added
  const logsAfterBatch1 = get(logStore);
  assert(logsAfterBatch1.length === 100, 'Should have 100 logs after first batch');
  
  // Add second batch with overlapping timestamps
  const batch2 = generateLogs(50, Date.now() - 50000); // Start earlier
  logStore.addLogBatch(batch2);
  
  // Wait for processing
  await new Promise(resolve => setTimeout(resolve, 100));
  
  // Check logs were merged correctly
  const logsAfterBatch2 = get(logStore);
  assert(logsAfterBatch2.length === 150, 'Should have 150 logs after second batch');
  
  // Check logs are in chronological order
  let chronologicalOrder = true;
  for (let i = 1; i < logsAfterBatch2.length; i++) {
    const prevTime = logsAfterBatch2[i-1]._unix_time || 0;
    const currTime = logsAfterBatch2[i]._unix_time || 0;
    if (prevTime > currTime) {
      chronologicalOrder = false;
      break;
    }
  }
  
  assert(chronologicalOrder, 'Logs should be in chronological order');
  
  // Test performance metrics
  console.log('\n=== Testing performance metrics ===');
  const metrics = getWasmState().performanceMetrics;
  
  assert(metrics.operationsCount > 0, 'Should have recorded operations');
  assert(typeof metrics.avgWasmTime === 'number', 'Should have measured WASM time');
  
  // Test garbage collection
  console.log('\n=== Testing garbage collection ===');
  const beforeGC = wasmModule.get_memory_usage();
  
  // Add large batches to increase memory usage
  const largeBatch = generateLogs(5000);
  logStore.addLogBatch(largeBatch);
  
  // Wait for processing
  await new Promise(resolve => setTimeout(resolve, 300));
  
  const afterLargeBatch = wasmModule.get_memory_usage();
  assert(afterLargeBatch.used_bytes >= beforeGC.used_bytes, 'Memory usage should increase after large batch');
  
  // Force garbage collection
  wasmModule.force_garbage_collection();
  
  const afterGC = wasmModule.get_memory_usage();
  console.log(`Memory before GC: ${formatBytes(afterLargeBatch.used_bytes)}`);
  console.log(`Memory after GC: ${formatBytes(afterGC.used_bytes)}`);
  
  // Print test summary
  console.log('\n=== Test Summary ===');
  console.log(`Total tests: ${testResults.total}`);
  console.log(`Passed: ${testResults.passed}`);
  console.log(`Failed: ${testResults.failed}`);
  
  if (testResults.failed === 0) {
    console.log('✅ All tests passed!');
  } else {
    console.error(`❌ ${testResults.failed} tests failed.`);
  }
}

// Helper to format bytes
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 Bytes';
  
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Run the tests when this script is loaded
if (typeof window !== 'undefined') {
  // In browser environment
  (window as any).runWasmE2ETests = runE2ETests;
  console.log('WebAssembly E2E tests loaded. Run with window.runWasmE2ETests()');
} else {
  // In Node.js environment
  runE2ETests().catch(console.error);
}