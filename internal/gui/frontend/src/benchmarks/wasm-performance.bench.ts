import { performance } from 'perf_hooks';
import { 
  enableWasm, 
  getWasmModule,
  shouldUseWasm,
  isWasmEnabled
} from '../lib/wasm';
import { wasmLogger, WasmLogLevel } from '../lib/wasm-logger';

interface LogMessage {
  level: string;
  message: string;
  time: string;
  behavior?: string;
  _sequence?: number;
  _unix_time?: number;
  _original_time?: string;
  _visible?: boolean;
  _height?: number;
  [key: string]: any;
}

/**
 * TypeScript implementation of mergeInsertLogs for benchmarking
 */
function mergeInsertLogsTS(existingLogs: LogMessage[], newLogs: LogMessage[]): LogMessage[] {
  // Short-circuit for empty arrays
  if (newLogs.length === 0) return existingLogs;
  if (existingLogs.length === 0) return newLogs;
  
  // Sort the new logs batch by unix time
  newLogs.sort((a, b) => {
    const timeA = a._unix_time || 0;
    const timeB = b._unix_time || 0;
    
    if (timeA !== timeB) {
      return timeA - timeB;
    }
    
    // If times match, use sequence as tie-breaker
    return (a._sequence || 0) - (b._sequence || 0);
  });
  
  // Ensure existing logs are sorted (assume they are for benchmark)
  const targetLogs = existingLogs;
  
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
 * Generate logs for testing
 */
function generateLogs(count: number, startTime: number = Date.now(), startSeq: number = 0): LogMessage[] {
  const logs: LogMessage[] = [];
  
  for (let i = 0; i < count; i++) {
    logs.push({
      level: 'INFO',
      message: `Test message ${i}`,
      time: new Date(startTime + i * 100).toTimeString().split(' ')[0],
      _sequence: startSeq + i,
      _unix_time: startTime + i * 100,
      _original_time: new Date(startTime + i * 100).toISOString()
    });
  }
  
  // Shuffle array to simulate unsorted logs
  return logs.sort(() => Math.random() - 0.5);
}

/**
 * Run benchmark comparison
 */
async function runBenchmark() {
  console.log('Starting WebAssembly performance benchmark...');
  
  // Ensure WebAssembly is initialized
  console.log('Initializing WebAssembly...');
  const wasmEnabled = await enableWasm(true);
  
  if (!wasmEnabled) {
    console.error('WebAssembly initialization failed. Cannot run benchmark.');
    return;
  }
  
  // Verify wasm module is available
  const wasmModule = getWasmModule();
  if (!wasmModule) {
    console.error('WebAssembly module not available. Cannot run benchmark.');
    return;
  }
  
  console.log('WebAssembly initialized successfully.');
  
  // Test dataset sizes
  const datasets = [
    { name: 'Tiny', existingSize: 100, newSize: 10 },
    { name: 'Small', existingSize: 500, newSize: 50 },
    { name: 'Medium', existingSize: 2000, newSize: 200 },
    { name: 'Large', existingSize: 5000, newSize: 500 },
    { name: 'X-Large', existingSize: 10000, newSize: 1000 }
  ];
  
  // Results table
  console.log('\n-----------------------------------------------------');
  console.log('| Dataset  | Size      | TS Time | WASM Time | Ratio |');
  console.log('|----------|-----------|---------|-----------|-------|');
  
  // Run benchmark for each dataset
  for (const dataset of datasets) {
    // Generate test data
    const existingLogs = generateLogs(dataset.existingSize);
    const newLogs = generateLogs(dataset.newSize, Date.now() + 3600000); // Start 1 hour later
    
    const totalSize = dataset.existingSize + dataset.newSize;
    
    // Measure TypeScript implementation
    const tsStartTime = performance.now();
    const tsResult = mergeInsertLogsTS(existingLogs, newLogs);
    const tsEndTime = performance.now();
    const tsTime = tsEndTime - tsStartTime;
    
    // Measure WebAssembly implementation
    const wasmStartTime = performance.now();
    const wasmResult = wasmModule.merge_insert_logs(existingLogs, newLogs);
    const wasmEndTime = performance.now();
    const wasmTime = wasmEndTime - wasmStartTime;
    
    // Calculate speedup
    const speedup = tsTime / wasmTime;
    
    // Verify results length is correct
    const expectedLength = dataset.existingSize + dataset.newSize;
    const tsLength = tsResult.length;
    const wasmLength = Array.isArray(wasmResult) ? wasmResult.length : -1;
    
    // Log results
    console.log(`| ${dataset.name.padEnd(8)} | ${totalSize.toString().padEnd(9)} | ${tsTime.toFixed(2).padEnd(7)} | ${wasmTime.toFixed(2).padEnd(9)} | ${speedup.toFixed(2)} |`);
    
    // Verify results match
    if (tsLength !== expectedLength || wasmLength !== expectedLength) {
      console.error(`Result length mismatch for ${dataset.name}: TS=${tsLength}, WASM=${wasmLength}, Expected=${expectedLength}`);
    }
    
    // Give time for garbage collection between runs
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  
  console.log('-----------------------------------------------------');
  
  // Check memory usage
  const memInfo = wasmModule.get_memory_usage();
  console.log('\nWebAssembly Memory Usage:');
  console.log(`- Total: ${formatBytes(memInfo.total_bytes)}`);
  console.log(`- Used: ${formatBytes(memInfo.used_bytes)} (${(memInfo.utilization * 100).toFixed(1)}%)`);
  console.log(`- Peak: ${formatBytes(memInfo.peak_bytes || 0)}`);
  console.log(`- Allocations: ${memInfo.allocation_count || 'N/A'}`);
  
  // Force garbage collection
  console.log('\nForcing garbage collection...');
  wasmModule.force_garbage_collection();
  
  // Check memory after GC
  const memInfoAfterGC = wasmModule.get_memory_usage();
  console.log('\nWebAssembly Memory After GC:');
  console.log(`- Used: ${formatBytes(memInfoAfterGC.used_bytes)} (${(memInfoAfterGC.utilization * 100).toFixed(1)}%)`);
  console.log(`- Allocations: ${memInfoAfterGC.allocation_count || 'N/A'}`);
}

// Helper for formatting bytes
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 Bytes';
  
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Run the benchmark
runBenchmark().catch(console.error);