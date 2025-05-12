// memory-test.ts - Utility to definitively test WebAssembly memory access
import * as wasmModule from '../../wasm-generated/pkg/log_engine.js';
import { wasmLogger, WasmLogLevel } from '../../lib/wasm-logger';

const MEMORY_TEST_COMPONENT = 'wasm-memory-test';

/**
 * This function performs a rigorous test of WebAssembly memory access
 * by checking memory access from both JS and Rust sides
 */
export async function testWasmMemoryAccess(): Promise<void> {
  wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "=== WebAssembly Memory Access Test Started ===");
  
  try {
    // Initialize the WASM module using our inlined binary
    wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "Initializing WASM module...");
    const exports = await wasmModule.initializeWithInlinedBinary();
    wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "WASM initialization complete");
    
    // First check: verification through inliner status
    if (typeof wasmModule.get_memory_api_access_status === 'function') {
      const status = wasmModule.get_memory_api_access_status();
      wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "Inliner memory status check", status);
      
      if (status && status.success === true && status.has_browser_api_access === true) {
        wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "✅ Inliner reports memory API access is available", {
          total_bytes: formatBytes(status.total_bytes)
        });
      } else {
        wasmLogger.log(WasmLogLevel.ERROR, MEMORY_TEST_COMPONENT, "❌ Inliner reports memory API access is NOT available", status);
      }
    } else {
      wasmLogger.log(WasmLogLevel.ERROR, MEMORY_TEST_COMPONENT, "Inliner status check function not available");
    }
    
    // Second check: verification through Rust memory usage function
    if (typeof exports.get_memory_usage === 'function') {
      const memUsage = exports.get_memory_usage();
      
      // First, check if memUsage is a Map
      const isMap = Object.prototype.toString.call(memUsage) === '[object Map]';
      wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, `Memory usage object type: ${isMap ? 'Map' : 'regular object'}`);
      
      // Log keys for Map or Object
      let memKeys: string[] = [];
      if (isMap) {
        (memUsage as Map<string, any>).forEach((_, key) => memKeys.push(key));
      } else {
        memKeys = memUsage ? Object.keys(memUsage).sort() : [];
      }
      
      wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "Memory usage keys", {
        keys: memKeys.join(', '),
        keyCount: memKeys.length,
        objectType: typeof memUsage,
        isNull: memUsage === null,
        isUndefined: memUsage === undefined,
        isMap: isMap
      });
      
      // Then log all properties and values for complete diagnostics
      const allProps = {};
      if (memUsage) {
        if (isMap) {
          for (const key of memKeys) {
            const value = (memUsage as Map<string, any>).get(key);
            allProps[key] = {
              value: value,
              type: typeof value,
              formatted: typeof value === 'number' ? formatBytes(value) : String(value)
            };
          }
        } else {
          for (const key of memKeys) {
            allProps[key] = {
              value: memUsage[key],
              type: typeof memUsage[key],
              formatted: typeof memUsage[key] === 'number' ? formatBytes(memUsage[key]) : String(memUsage[key])
            };
          }
        }
      }
      
      wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "Memory usage values (all properties)", allProps);
      
      // Convert Map to object for easier handling if needed
      const memoryData = {};
      if (isMap) {
        (memUsage as Map<string, any>).forEach((value, key) => {
          memoryData[key] = value;
        });
        
        wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "Memory usage converted from Map to object", {
          objKeys: Object.keys(memoryData).join(', '),
          totalBytes: memoryData['total_bytes']
        });
      }
      
      // Extract critical values based on object type
      let totalBytes, trackedBytes, hasApiAccess;
      
      if (isMap) {
        totalBytes = (memUsage as Map<string, any>).get('total_bytes');
        trackedBytes = (memUsage as Map<string, any>).get('tracked_bytes');
        hasApiAccess = (memUsage as Map<string, any>).get('has_browser_api_access');
      } else {
        totalBytes = memUsage?.total_bytes;
        trackedBytes = memUsage?.tracked_bytes;
        hasApiAccess = memUsage?.has_browser_api_access;
      }
      
      // Finally log the standard expected properties
      wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "Memory usage summary", {
        formatted: {
          total_bytes: formatBytes(totalBytes),
          tracked_bytes: formatBytes(trackedBytes)
        },
        raw: {
          total_bytes: totalBytes,
          tracked_bytes: trackedBytes,
          has_api_access: hasApiAccess,
          isMap: isMap
        }
      });
      
      // Check if the memory usage reports browser API access
      if (hasApiAccess === true) {
        wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "✅ Rust reports memory API access is available");
      } else {
        wasmLogger.log(WasmLogLevel.ERROR, MEMORY_TEST_COMPONENT, "❌ Rust reports memory API access is NOT available", {
          hasApiAccess,
          valueType: typeof hasApiAccess,
          directAccess: isMap ? null : memUsage?.has_browser_api_access,
          isMap: isMap
        });
      }
    } else {
      wasmLogger.log(WasmLogLevel.ERROR, MEMORY_TEST_COMPONENT, "get_memory_usage function not available");
    }
    
    // Third check: test memory growth
    if (typeof exports.ensure_sufficient_memory === 'function') {
      // Test with 10MB, 20MB, 30MB
      const sizes = [10, 20, 30];
      
      for (const size of sizes) {
        const sizeBytes = size * 1024 * 1024;
        wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, `Testing memory growth to ${size}MB...`);
        
        // Before growth
        const beforeMem = exports.get_memory_usage();
        
        // Check if beforeMem is a Map
        const beforeIsMap = Object.prototype.toString.call(beforeMem) === '[object Map]';
        
        // Get before memory values
        let beforeTotalBytes, beforeTrackedBytes;
        if (beforeIsMap) {
          beforeTotalBytes = (beforeMem as Map<string, any>).get('total_bytes');
          beforeTrackedBytes = (beforeMem as Map<string, any>).get('tracked_bytes');
        } else {
          beforeTotalBytes = beforeMem?.total_bytes;
          beforeTrackedBytes = beforeMem?.tracked_bytes;
        }
        
        wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "Memory before growth (summary)", { 
          total_bytes: formatBytes(beforeTotalBytes),
          used_bytes: formatBytes(beforeTrackedBytes),
          isMap: beforeIsMap
        });
        
        // Attempt growth
        wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, `Calling ensure_sufficient_memory(${formatBytes(sizeBytes)})`);
        const success = exports.ensure_sufficient_memory(sizeBytes);
        wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, `ensure_sufficient_memory returned: ${success}`);
        
        // After growth
        const afterMem = exports.get_memory_usage();
        
        // Check if afterMem is a Map
        const afterIsMap = Object.prototype.toString.call(afterMem) === '[object Map]';
        
        // Get after memory values
        let afterTotalBytes, afterTrackedBytes;
        if (afterIsMap) {
          afterTotalBytes = (afterMem as Map<string, any>).get('total_bytes');
          afterTrackedBytes = (afterMem as Map<string, any>).get('tracked_bytes');
        } else {
          afterTotalBytes = afterMem?.total_bytes;
          afterTrackedBytes = afterMem?.tracked_bytes;
        }
        
        // Log results
        if (success) {
          wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, `Memory growth SUCCESS to ${size}MB`, {
            before: formatBytes(beforeTotalBytes),
            after: formatBytes(afterTotalBytes),
            growth: formatBytes((afterTotalBytes || 0) - (beforeTotalBytes || 0))
          });
        } else {
          wasmLogger.log(WasmLogLevel.ERROR, MEMORY_TEST_COMPONENT, `Memory growth FAILED for ${size}MB`, {
            before: formatBytes(beforeTotalBytes),
            after: formatBytes(afterTotalBytes)
          });
        }
        
        // Verify growth actually worked by comparing values
        if (typeof afterTotalBytes === 'number' && typeof beforeTotalBytes === 'number') {
          const bytesGrew = afterTotalBytes > beforeTotalBytes;
          if (bytesGrew) {
            wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "✅ Memory size actually increased", {
              before: formatBytes(beforeTotalBytes),
              after: formatBytes(afterTotalBytes),
              growth: formatBytes(afterTotalBytes - beforeTotalBytes)
            });
          } else {
            wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "ℹ️ Memory size unchanged (sufficient memory already available)");
          }
        } else {
          wasmLogger.log(WasmLogLevel.ERROR, MEMORY_TEST_COMPONENT, "❌ Cannot compare memory sizes - invalid values", {
            beforeTotalBytes,
            afterTotalBytes,
            beforeType: typeof beforeTotalBytes,
            afterType: typeof afterTotalBytes,
            beforeIsMap,
            afterIsMap
          });
        }
      }
    } else {
      wasmLogger.log(WasmLogLevel.ERROR, MEMORY_TEST_COMPONENT, "ensure_sufficient_memory function not available");
    }
    
    // Fourth check: test merge operation to verify memory is truly accessible
    if (typeof exports.merge_insert_logs === 'function') {
      // Create test data - ensure they're plain objects, not Maps
      // IMPORTANT: Create serializable objects to avoid Map conversion
      function createSerializableLogObject(index: number, offset: number = 0) {
        return {
          level: 'INFO',
          message: `Test message ${index + offset}`,
          time: new Date().toISOString(),
          behavior: 'test',  // Add this to match the struct
          // Metadata with underscore prefix
          _sequence: index + offset,
          _unix_time: (Date.now() / 1000) + (offset ? index/100 : 0),
          _original_time: new Date().toISOString(),
          _visible: true,
          _height: 20
        };
      }

      // Create arrays using a plain function that returns objects
      const testArrayA: any[] = [];
      const testArrayB: any[] = [];

      // Fill arrays with objects
      for (let i = 0; i < 100; i++) {
        testArrayA.push(createSerializableLogObject(i));
        testArrayB.push(createSerializableLogObject(i, 100));
      }

      // Verify array items are serializable
      const firstA = testArrayA[0];
      wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "Test array structure check", {
        itemType: typeof firstA,
        itemToString: Object.prototype.toString.call(firstA),
        keys: Object.keys(firstA).join(', '),
        isMap: Object.prototype.toString.call(firstA) === '[object Map]'
      });
      
      wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "Testing merge_insert_logs function with 200 log entries");
      
      try {
        // Get memory before operation
        const beforeMem = exports.get_memory_usage();
        
        // Check if beforeMem is a Map
        const beforeIsMap = Object.prototype.toString.call(beforeMem) === '[object Map]';
        
        // Helper function to ensure objects are plain, not Maps
        function ensureArrayHasNoMaps(arr: any[]): any[] {
          return arr.map(item => {
            if (Object.prototype.toString.call(item) === '[object Map]') {
              // Convert Map to object
              const obj: Record<string, any> = {};
              try {
                (item as any as Map<string, any>).forEach((value, key) => {
                  obj[key] = value;
                });
              } catch (e) {
                wasmLogger.log(WasmLogLevel.ERROR, MEMORY_TEST_COMPONENT, `Error converting Map: ${e}`);
              }
              return obj;
            }
            return item;
          });
        }

        // Ensure test data doesn't contain Maps
        const serializedA = ensureArrayHasNoMaps(testArrayA);
        const serializedB = ensureArrayHasNoMaps(testArrayB);

        // Perform merge with serialized data
        wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "Calling merge_insert_logs with serialized arrays");
        const resultArray = exports.merge_insert_logs(serializedA, serializedB);
        
        // Get memory after operation
        const afterMem = exports.get_memory_usage();
        
        // Check if afterMem is a Map
        const afterIsMap = Object.prototype.toString.call(afterMem) === '[object Map]';
        
        // Get tracked bytes properly based on object type
        let beforeTrackedBytes, afterTrackedBytes;
        
        if (beforeIsMap) {
          beforeTrackedBytes = (beforeMem as Map<string, any>).get('tracked_bytes');
        } else {
          beforeTrackedBytes = beforeMem?.tracked_bytes;
        }
        
        if (afterIsMap) {
          afterTrackedBytes = (afterMem as Map<string, any>).get('tracked_bytes');
        } else {
          afterTrackedBytes = afterMem?.tracked_bytes;
        }
        
        // Log results
        const expectedLength = testArrayA.length + testArrayB.length;
        const actualLength = resultArray?.length || 0;
        
        wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "Merge operation results", {
          expectedLength,
          actualLength,
          success: actualLength === expectedLength,
          memoryBefore: formatBytes(beforeTrackedBytes),
          memoryAfter: formatBytes(afterTrackedBytes),
          memoryDelta: formatBytes((afterTrackedBytes || 0) - (beforeTrackedBytes || 0)),
          beforeIsMap,
          afterIsMap,
          resultType: typeof resultArray
        });
        
        if (actualLength === expectedLength) {
          wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "✅ Merge operation succeeded with correct array length");
        } else {
          wasmLogger.log(WasmLogLevel.ERROR, MEMORY_TEST_COMPONENT, "❌ Merge operation failed with incorrect array length");
        }
        
        // Check if memory usage increased, which would indicate active tracking
        if (typeof afterTrackedBytes === 'number' && 
            typeof beforeTrackedBytes === 'number') {
          const memoryIncreased = afterTrackedBytes > beforeTrackedBytes;
          if (memoryIncreased) {
            wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "✅ Memory usage tracking is working correctly");
          } else {
            wasmLogger.log(WasmLogLevel.WARN, MEMORY_TEST_COMPONENT, "⚠️ Memory usage did not increase after operation");
          }
        }
      } catch (error) {
        wasmLogger.log(WasmLogLevel.ERROR, MEMORY_TEST_COMPONENT, "❌ Merge operation failed with exception", {
          error: error instanceof Error ? error.message : String(error)
        });
      }
    }
    
    wasmLogger.log(WasmLogLevel.INFO, MEMORY_TEST_COMPONENT, "=== Memory Test Complete ===");
    
  } catch (error) {
    wasmLogger.log(WasmLogLevel.ERROR, MEMORY_TEST_COMPONENT, "Memory test failed with exception", {
      error: error instanceof Error ? error.message : String(error)
    });
  }
}

function formatBytes(bytes: number | undefined): string {
  if (bytes === undefined || typeof bytes !== 'number' || isNaN(bytes) || bytes < 0) return 'N/A';
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

// Script entry point - if this file is imported directly
if (import.meta.url === document.currentScript?.getAttribute('src')) {
  testWasmMemoryAccess().catch(err => {
    wasmLogger.log(WasmLogLevel.ERROR, MEMORY_TEST_COMPONENT, "Unhandled error in memory test", { 
      error: err instanceof Error ? err.message : String(err)
    });
  });
}