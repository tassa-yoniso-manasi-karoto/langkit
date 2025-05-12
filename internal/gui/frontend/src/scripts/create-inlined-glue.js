#!/usr/bin/env node
/**
 * create-inlined-glue.js
 * A Node.js script that inlines a WASM binary into its JavaScript glue code
 * This replaces the previous wasm-inline.sh bash script with a more precise approach
 */

import fs from 'fs';
import path from 'path';

// Check command line arguments
if (process.argv.length < 5) {
  console.error('Usage: node create-inlined-glue.js <wasm-file> <js-glue-file> <output-file>');
  process.exit(1);
}

// Get the input and output file paths
const wasmPath = process.argv[2];
const jsGluePath = process.argv[3];
const outputPath = process.argv[4];

console.log(`WASM binary: ${wasmPath}`);
console.log(`JS glue file: ${jsGluePath}`);
console.log(`Output file: ${outputPath}`);

// Read the WASM file as a binary buffer
let wasmBinary;
try {
  wasmBinary = fs.readFileSync(wasmPath);
  console.log(`Read WASM binary: ${wasmBinary.length} bytes`);
} catch (error) {
  console.error(`Error reading WASM file: ${error.message}`);
  process.exit(1);
}

// Convert the WASM binary to a base64 string
const wasmBase64 = wasmBinary.toString('base64');
console.log(`Converted WASM to base64: ${wasmBase64.length} characters`);

// Read the JS glue file
let jsGlue;
try {
  jsGlue = fs.readFileSync(jsGluePath, 'utf8');
  console.log(`Read JS glue file: ${jsGlue.length} characters`);
} catch (error) {
  console.error(`Error reading JS glue file: ${error.message}`);
  process.exit(1);
}

// Create the prefix with the WASM binary and global overrides
const prefix = `/**
 * Inlined WebAssembly module
 * Generated: ${new Date().toISOString()}
 * Original module: ${path.basename(wasmPath)} (${wasmBinary.length} bytes)
 */

// The WebAssembly binary inlined as Base64
const WASM_BINARY_BASE64 = "${wasmBase64}";

// Store the original WASM size for metrics
const WASM_ORIGINAL_SIZE_BYTES = ${wasmBinary.length};

// Function to convert Base64 to ArrayBuffer
function base64ToArrayBuffer(base64) {
  const binaryString = atob(base64);
  const bytes = new Uint8Array(binaryString.length);
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = binaryString.charCodeAt(i);
  }
  return bytes.buffer;
}

// Convert the Base64 WASM binary to an ArrayBuffer once at load time
const WASM_BINARY_BUFFER = base64ToArrayBuffer(WASM_BINARY_BASE64);

// Flag to track WebAssembly memory API access for diagnostics
let __wasm_memory_api_access_status = null;

// Internal function to check WebAssembly memory API access
function __check_memory_api_access(instance) {
  try {
    if (instance && instance.exports && instance.exports.memory && 
        instance.exports.memory instanceof WebAssembly.Memory) {
      __wasm_memory_api_access_status = {
        success: true,
        has_browser_api_access: true,
        total_bytes: instance.exports.memory.buffer.byteLength,
        timestamp: Date.now()
      };
      return __wasm_memory_api_access_status;
    }
    __wasm_memory_api_access_status = {
      success: false,
      has_browser_api_access: false,
      error: "Memory object not accessible or not WebAssembly.Memory",
      timestamp: Date.now()
    };
    return __wasm_memory_api_access_status;
  } catch (e) {
    __wasm_memory_api_access_status = {
      success: false,
      has_browser_api_access: false,
      error: e instanceof Error ? e.message : String(e),
      timestamp: Date.now()
    };
    return __wasm_memory_api_access_status;
  }
}

// WebAssembly API wrapper that intercepts all fetch attempts
(function() {
  // Store original WebAssembly functions
  const originalInstantiate = WebAssembly.instantiate;
  const originalInstantiateStreaming = WebAssembly.instantiateStreaming;
  
  // Override instantiate to intercept URL/Request inputs with proper memory checking
  WebAssembly.instantiate = function(bufferOrModule, importObject) {
    // Check if this is a URL, Request, or string path - which needs to be intercepted
    const needsIntercept = typeof bufferOrModule === 'string' || 
                          bufferOrModule instanceof Request ||
                          (typeof URL !== 'undefined' && bufferOrModule instanceof URL);
  
    // If we need to intercept a URL/Request, use our inlined buffer
    if (needsIntercept) {
      console.log("[wasm-inline] WebAssembly.instantiate intercepted URL/Request - using inlined binary");
      
      // Use our inlined buffer and immediately check API access
      return originalInstantiate(WASM_BINARY_BUFFER, importObject).then(result => {
        // CRITICAL: Immediately check memory API access after successful instantiation
        const apiStatus = __check_memory_api_access(result.instance);
        
        // These lines ensure proper WASM-bindgen setup by mimicking its behavior
        if (typeof __wbg_set_wasm === 'function' && result.instance && result.instance.exports) {
          // Make sure wasm-bindgen's internal setup functions work with this instance
          __wbg_set_wasm(result.instance.exports);
        }
        
        // Log API access status for debugging
        if (apiStatus && apiStatus.success) {
          console.log("[wasm-inline] WebAssembly memory API access VERIFIED", apiStatus);
        } else {
          console.error("[wasm-inline] WebAssembly memory API access FAILED", apiStatus);
        }
        
        return result;
      }).catch(error => {
        console.error("[wasm-inline] Failed to instantiate WebAssembly with inlined binary:", error);
        throw error;
      });
    }
    
    // For normal ArrayBuffer instantiation, check API access after completion
    return originalInstantiate(bufferOrModule, importObject).then(result => {
      // CRITICAL: Check API access for all instantiations
      const apiStatus = __check_memory_api_access(result.instance);
      
      // Process API access status for normal instantiation
      if (apiStatus && apiStatus.success) {
        console.log("[wasm-inline] WebAssembly memory API access verified for normal instantiation", apiStatus);
      } else {
        console.warn("[wasm-inline] WebAssembly memory API access check failed for normal instantiation", apiStatus);
      }
      
      return result;
    });
  };
  
  // Override instantiateStreaming to always use our inlined buffer with proper memory checking
  WebAssembly.instantiateStreaming = function(source, importObject) {
    console.log("[wasm-inline] WebAssembly.instantiateStreaming intercepted - using inlined binary");
    
    // Use direct instantiation with our inlined buffer 
    return originalInstantiate(WASM_BINARY_BUFFER, importObject).then(result => {
      // CRITICAL: Immediately check memory API access after successful instantiation
      const apiStatus = __check_memory_api_access(result.instance);
      
      // These lines ensure proper WASM-bindgen setup by mimicking its behavior
      if (typeof __wbg_set_wasm === 'function' && result.instance && result.instance.exports) {
        // Make sure wasm-bindgen's internal setup functions work with this instance
        __wbg_set_wasm(result.instance.exports);
      }
      
      // Log API access status for debugging
      if (apiStatus && apiStatus.success) {
        console.log("[wasm-inline] WebAssembly memory API access VERIFIED for streaming instantiation", apiStatus);
      } else {
        console.error("[wasm-inline] WebAssembly memory API access FAILED for streaming instantiation", apiStatus);
      }
      
      return result;
    }).catch(error => {
      console.error("[wasm-inline] Failed to instantiate WebAssembly with streaming API:", error);
      throw error;
    });
  };
  
  // Override fetch to intercept .wasm requests
  const originalFetch = window.fetch;
  window.fetch = function(input, init) {
    // Check if this is a .wasm file request
    const url = input instanceof Request ? input.url : String(input);
    if (url.endsWith('.wasm') || url.includes('log_engine_bg.wasm')) {
      console.log("[wasm-inline] fetch intercepted WASM request:", url);
      console.log("[wasm-inline] Providing inlined WASM binary instead of network fetch");
      
      // Create a mock Response with our binary
      return Promise.resolve(new Response(
        WASM_BINARY_BUFFER,
        {
          status: 200,
          headers: new Headers({
            'Content-Type': 'application/wasm'
          })
        }
      ));
    }
    
    // Otherwise, pass through to original fetch
    return originalFetch.apply(this, arguments);
  };
})();

// ORIGINAL WASM-BINDGEN CODE BELOW
// (except for fetch/instantiate operations which are intercepted)
`;

// Create the suffix with additional helper functions and exports
const suffix = `

// Function to check WebAssembly memory API access status (added by inliner)
export function get_memory_api_access_status() {
  return __wasm_memory_api_access_status;
}

// Add a specialized init function that uses the inlined binary
export async function initializeWithInlinedBinary() {
  console.log("[wasm-inline] Using inlined WebAssembly binary with async init");
  // Use the default export (which is __wbg_init) to ensure the async initialization path
  // This will trigger our global overrides for WebAssembly.instantiate 
  // which use WASM_BINARY_BUFFER and check memory API access
  return __wbg_init(undefined);
}
`;

// Combine the prefix, JS glue, and suffix
const output = `${prefix}${jsGlue}${suffix}`;

// Write the output file
try {
  fs.writeFileSync(outputPath, output, 'utf8');
  console.log(`Successfully wrote output file: ${outputPath}`);
} catch (error) {
  console.error(`Error writing output file: ${error.message}`);
  process.exit(1);
}

console.log('WASM inlining complete!');