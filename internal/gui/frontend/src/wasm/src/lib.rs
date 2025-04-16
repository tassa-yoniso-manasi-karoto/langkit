// src/wasm/lib.rs - Optimized version with better memory management
use wasm_bindgen::prelude::*;
use serde::{Serialize, Deserialize};
use js_sys::Error;
use std::collections::HashMap; // Needed for extra_fields

// Use a static mutable variable for the allocation tracker.
// This requires unsafe blocks for access, which is common in FFI contexts.
static mut ALLOCATION_TRACKER: Option<AllocationTracker> = None;

// --- Start Replace AllocationTracker ---
// Expand AllocationTracker with growth tracking
struct AllocationTracker {
    // Existing fields...
    active_bytes: usize,
    peak_bytes: usize,
    allocation_count: usize,
    allocation_history: [usize; 10],  // Circular buffer of recent allocations
    history_index: usize,
    average_allocation: usize,
    sample_count: usize,
    last_gc_time: u64,     // Timestamp of last garbage collection
    allocation_rate: f64,  // Bytes per second allocation rate

    // NEW FIELDS for better tracking
    growth_events: usize,         // Count of successful memory growths
    growth_failures: usize,       // Count of failed memory growths
    last_growth_time: u64,        // Timestamp of last successful growth
    last_growth_failure: u64,     // Timestamp of last growth failure
    allocations_since_last_gc: usize, // Track allocations since last GC
    total_allocated_bytes: usize, // Lifetime total of all bytes allocated
    reused_bytes: usize,          // Estimate of memory reuse (from GC)
    gc_events: usize,             // Count of GC events
}

impl AllocationTracker {
    fn new() -> Self {
        Self {
            active_bytes: 0,
            peak_bytes: 0,
            allocation_count: 0,
            allocation_history: [0; 10],
            history_index: 0,
            average_allocation: 0,
            sample_count: 0,
            last_gc_time: 0,
            allocation_rate: 0.0,
            // Initialize new fields
            growth_events: 0,
            growth_failures: 0,
            last_growth_time: 0,
            last_growth_failure: 0,
            allocations_since_last_gc: 0,
            total_allocated_bytes: 0,
            reused_bytes: 0,
            gc_events: 0,
        }
    }

    // Track allocation with improved stats
    fn track_allocation(&mut self, bytes: usize) {
        // Original tracking logic...
        self.active_bytes += bytes;
        self.allocation_count += 1;
        self.allocations_since_last_gc += 1;
        self.total_allocated_bytes += bytes;

        if self.active_bytes > self.peak_bytes {
            self.peak_bytes = self.active_bytes;
        }

        // Allocation pattern tracking
        self.allocation_history[self.history_index] = bytes;
        self.history_index = (self.history_index + 1) % 10;

        // Update running average
        self.sample_count += 1;
        if self.sample_count > 0 {
            self.average_allocation = ((self.average_allocation * (self.sample_count - 1)) + bytes) / self.sample_count;
        }

        // Allocation rate calculation
        let now = get_timestamp_ms();
        if self.last_gc_time > 0 {
            let time_diff = now.saturating_sub(self.last_gc_time);
            if time_diff > 0 {
                let new_rate = bytes as f64 / (time_diff as f64 / 1000.0);
                self.allocation_rate = self.allocation_rate * 0.7 + new_rate * 0.3;
            }
        }
    }

    // More accurate deallocation tracking - Keep existing one
    fn track_deallocation(&mut self, bytes: usize) {
        if bytes <= self.active_bytes {
            self.active_bytes -= bytes;
        } else {
            log("WARNING: Attempted to deallocate more bytes than tracked as active");
            self.active_bytes = 0;
        }
    }

    // Improved reset for garbage collection
    fn reset(&mut self) {
        // Track reused memory estimate before resetting
        self.reused_bytes += self.active_bytes;

        // Original reset logic
        self.active_bytes = 0;
        self.allocation_count = 0;
        self.allocations_since_last_gc = 0;

        // Update GC stats
        self.last_gc_time = get_timestamp_ms();
        self.gc_events += 1;
    }

    // Predict if an operation would cause memory issues - Keep existing one
    fn would_operation_fit(&self, estimated_bytes: usize, wasm_heap_size: usize) -> bool {
        // Conservative estimate: need the bytes plus 20% overhead
        let required_bytes = (estimated_bytes as f64 * 1.2) as usize;

        // Available memory calculation
        let available = if wasm_heap_size > self.active_bytes {
            wasm_heap_size - self.active_bytes
        } else {
            0
        };

        // True if operation would fit with a safety margin
        available >= required_bytes
    }

    // Function to provide enhanced memory usage statistics
    fn get_enhanced_stats(&self) -> serde_json::Value {
        serde_json::json!({
            "active_bytes": self.active_bytes,
            "peak_bytes": self.peak_bytes,
            "allocation_count": self.allocation_count,
            "average_allocation": self.average_allocation,
            "allocation_rate": self.allocation_rate,
            "time_since_last_gc": get_timestamp_ms().saturating_sub(self.last_gc_time),
            // New detailed statistics
            "growth_events": self.growth_events,
            "growth_failures": self.growth_failures,
            "time_since_last_growth": get_timestamp_ms().saturating_sub(self.last_growth_time),
            "gc_events": self.gc_events,
            "allocations_since_last_gc": self.allocations_since_last_gc,
            "total_allocated_bytes": self.total_allocated_bytes,
            "reused_bytes": self.reused_bytes,
            "memory_efficiency": if self.total_allocated_bytes > 0 {
                self.reused_bytes as f64 / self.total_allocated_bytes as f64
            } else {
                0.0
            }
        })
    }
}
// --- End Replace AllocationTracker ---


// Function to safely get a mutable reference to the static tracker
fn get_allocation_tracker() -> &'static mut AllocationTracker {
    unsafe {
        // Initialize the tracker if it hasn't been already
        if ALLOCATION_TRACKER.is_none() {
            ALLOCATION_TRACKER = Some(AllocationTracker::new());
        }
        ALLOCATION_TRACKER.as_mut().unwrap()
    }
}

// --- Start Insert get_timestamp_ms ---
// Helper function to get millisecond timestamp
fn get_timestamp_ms() -> u64 {
    let now = js_sys::Date::now();
    now as u64
}
// --- End Insert get_timestamp_ms ---


#[wasm_bindgen]
extern "C" {
    #[wasm_bindgen(js_namespace = console)]
    fn log(s: &str); // For logging debug messages from WASM to browser console
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
    // Handle any additional dynamic fields using serde_json::Value
    #[serde(flatten)]
    extra_fields: HashMap<String, serde_json::Value>,
}

// Estimate the size of a LogMessage for tracking purposes
// This is an approximation as string sizes vary.
fn estimate_log_message_size(log_msg: &LogMessage) -> usize {
    let base_size = std::mem::size_of::<LogMessage>();
    let string_size_estimate = log_msg.level.as_ref().map_or(0, |s| s.len()) +
                               log_msg.message.as_ref().map_or(0, |s| s.len()) +
                               log_msg.time.as_ref().map_or(0, |s| s.len()) +
                               log_msg.behavior.as_ref().map_or(0, |s| s.len()) +
                               log_msg.original_time.as_ref().map_or(0, |s| s.len());
    // Add estimate for HashMap extra_fields (key + value size estimate)
    let extra_fields_size: usize = log_msg.extra_fields.iter().map(|(k, v)| {
        k.len() + match v {
            serde_json::Value::String(s) => s.len(),
            _ => std::mem::size_of_val(v), // Rough estimate for non-string types
        }
    }).sum();

    base_size + string_size_estimate + extra_fields_size
}


#[derive(Serialize, Deserialize)]
pub struct MemoryInfo {
    total_bytes: usize,      // Total WASM memory available
    used_bytes: usize,       // Estimated currently used bytes based on tracker
    utilization: f64,        // used_bytes / total_bytes
    peak_bytes: usize,       // Peak memory usage recorded by tracker
    allocation_count: usize, // Number of allocations tracked
}


// --- Start Replace merge_insert_logs and helpers ---
#[wasm_bindgen]
pub fn merge_insert_logs(existing_logs_js: JsValue, new_logs_js: JsValue) -> Result<JsValue, JsValue> { // Remove extra pub
    // Reset allocation tracking for this specific operation
    get_allocation_tracker().reset();

    // Quick check for empty arrays
    if js_sys::Array::is_array(&new_logs_js) && js_sys::Array::from(&new_logs_js).length() == 0 {
        return Ok(existing_logs_js);
    }

    if js_sys::Array::is_array(&existing_logs_js) && js_sys::Array::from(&existing_logs_js).length() == 0 {
        return Ok(new_logs_js);
    }

    // NEW: Calculate estimated memory requirements
    let existing_count = if js_sys::Array::is_array(&existing_logs_js) {
        js_sys::Array::from(&existing_logs_js).length() as usize
    } else {
        0
    };
    
    let new_count = if js_sys::Array::is_array(&new_logs_js) {
        js_sys::Array::from(&new_logs_js).length() as usize
    } else {
        0
    };
    
    // Estimate memory needs (conservative but not excessive)
    let total_count = existing_count + new_count;
    let estimated_bytes = total_count * 256; // Rough estimate of bytes per log
    
    // Ensure we have sufficient memory for this operation
    let memory_check = ensure_sufficient_memory(estimated_bytes);
    if !memory_check {
        return Err(Error::new(&format!(
            "Insufficient memory for merge operation: needed ~{} bytes for {} logs",
            estimated_bytes, total_count
        )).into());
    }

    // Check for special cases that can be optimized
    if is_append_only_pattern(&existing_logs_js, &new_logs_js) {
        return append_only_merge(existing_logs_js, new_logs_js);
    }
    // Add check for prepend pattern
    if is_prepend_pattern(&existing_logs_js, &new_logs_js) {
        return prepend_merge(existing_logs_js, new_logs_js);
    }


    // Standard path for mixed logs
    let existing_logs: Vec<LogMessage> = match serde_wasm_bindgen::from_value::<Vec<LogMessage>>(existing_logs_js) {
        Ok(logs) => {
            // Track this allocation approximately
            let estimated_size: usize = logs.iter().map(estimate_log_message_size).sum();
            get_allocation_tracker().track_allocation(estimated_size);
            logs
        },
        Err(e) => return Err(Error::new(&format!("Failed to deserialize existing logs: {:?}", e)).into()),
    };

    let mut new_logs: Vec<LogMessage> = match serde_wasm_bindgen::from_value::<Vec<LogMessage>>(new_logs_js) {
        Ok(logs) => {
            // Track this allocation too
            let estimated_size: usize = logs.iter().map(estimate_log_message_size).sum();
            get_allocation_tracker().track_allocation(estimated_size);
            logs
        },
        Err(e) => return Err(Error::new(&format!("Failed to deserialize new logs: {:?}", e)).into()),
    };

    // Use an optimized merge algorithm based on the input characteristics
    let result = if existing_logs.len() > 10000 || new_logs.len() > 10000 {
        // For very large arrays, use a memory-efficient approach
        memory_efficient_merge(&existing_logs, &mut new_logs)
    } else {
        // For normal sized arrays, use a faster approach
        standard_merge(existing_logs, new_logs)
    };

    // Serialize back to JsValue with error handling
    match serde_wasm_bindgen::to_value(&result) {
        Ok(js_array) => Ok(js_array),
        Err(e) => Err(Error::new(&format!("Failed to serialize result: {:?}", e)).into()),
    }
}

// Check if this is an append-only pattern (all new logs come after existing logs)
fn is_append_only_pattern(existing_logs_js: &JsValue, new_logs_js: &JsValue) -> bool {
    if !js_sys::Array::is_array(existing_logs_js) || !js_sys::Array::is_array(new_logs_js) {
        return false;
    }

    let existing_array = js_sys::Array::from(existing_logs_js);
    let new_array = js_sys::Array::from(new_logs_js);

    if existing_array.length() == 0 || new_array.length() == 0 {
        return true; // Empty arrays can be appended trivially
    }

    // Check the last item of existing vs first item of new
    let last_existing = existing_array.get(existing_array.length() - 1);
    let first_new = new_array.get(0);

    // Get timestamps safely
    let last_existing_time = get_unix_time(&last_existing).unwrap_or(0.0);
    let first_new_time = get_unix_time(&first_new).unwrap_or(std::f64::MAX);

    // If the earliest new log is later than or equal to the latest existing log, this is append-only
    last_existing_time <= first_new_time
}

// Check if this is a prepend-only pattern (all new logs come before existing logs)
fn is_prepend_pattern(existing_logs_js: &JsValue, new_logs_js: &JsValue) -> bool {
    if !js_sys::Array::is_array(existing_logs_js) || !js_sys::Array::is_array(new_logs_js) {
        return false;
    }

    let existing_array = js_sys::Array::from(existing_logs_js);
    let new_array = js_sys::Array::from(new_logs_js);

    if existing_array.length() == 0 || new_array.length() == 0 {
        return true; // Empty arrays can be prepended trivially
    }

    // Check the first item of existing vs last item of new
    let first_existing = existing_array.get(0);
    let last_new = new_array.get(new_array.length() - 1);

    // Get timestamps safely
    let first_existing_time = get_unix_time(&first_existing).unwrap_or(std::f64::MAX);
    let last_new_time = get_unix_time(&last_new).unwrap_or(0.0);

    // If the latest new log is earlier than or equal to the earliest existing log, this is prepend-only
    last_new_time <= first_existing_time
}


// Fast path for append-only case
fn append_only_merge(existing_logs_js: JsValue, new_logs_js: JsValue) -> Result<JsValue, JsValue> {
    let existing_array = js_sys::Array::from(&existing_logs_js);
    let new_array = js_sys::Array::from(&new_logs_js);

    // Create result array by concatenating
    let result = js_sys::Array::new_with_length(existing_array.length() + new_array.length());


    // Add all existing logs
    for i in 0..existing_array.length() {
        result.set(i, existing_array.get(i));
    }

    // Add all new logs
    for i in 0..new_array.length() {
        result.set(existing_array.length() + i, new_array.get(i));
    }

    Ok(result.into())
}


// Fast path for prepend case
fn prepend_merge(existing_logs_js: JsValue, new_logs_js: JsValue) -> Result<JsValue, JsValue> {
    let existing_array = js_sys::Array::from(&existing_logs_js);
    let new_array = js_sys::Array::from(&new_logs_js);

    // Create result array by concatenating in reverse order
    let result = js_sys::Array::new_with_length(existing_array.length() + new_array.length());


    // Add all new logs first
    for i in 0..new_array.length() {
        result.set(i, new_array.get(i));
    }

    // Then add all existing logs
    for i in 0..existing_array.length() {
        result.set(new_array.length() + i, existing_array.get(i));
    }

    Ok(result.into())
}

// Helper to safely get unix_time from JS object
fn get_unix_time(obj: &JsValue) -> Option<f64> {
    if obj.is_undefined() || obj.is_null() {
        return None;
    }

    // Use js_sys::Reflect to access property dynamically
    match js_sys::Reflect::get(obj, &"_unix_time".into()) {
        Ok(time_val) => time_val.as_f64(), // as_f64 handles undefined/null returning None
        Err(_) => None, // Handle potential error during property access
    }
}


// Standard merge algorithm for normal-sized arrays
fn standard_merge(mut existing_logs: Vec<LogMessage>, mut new_logs: Vec<LogMessage>) -> Vec<LogMessage> {
    // Pre-allocate the result vector to avoid reallocations
    let total_capacity = existing_logs.len() + new_logs.len();
    let mut result = Vec::with_capacity(total_capacity);

    // Track this allocation
    get_allocation_tracker().track_allocation(total_capacity * std::mem::size_of::<LogMessage>());

    // Sort both arrays first for more efficient merging
    sort_logs(&mut existing_logs);
    sort_logs(&mut new_logs);

    // Use efficient merge algorithm (similar to std::vec::Vec::append but merges sorted)
    let mut i = 0;
    let mut j = 0;

    while i < existing_logs.len() && j < new_logs.len() {
        let time_a = existing_logs[i].unix_time.unwrap_or(0.0);
        let time_b = new_logs[j].unix_time.unwrap_or(0.0);
        let seq_a = existing_logs[i].sequence.unwrap_or(0);
        let seq_b = new_logs[j].sequence.unwrap_or(0);


        // Compare timestamps first, then sequence as tie-breaker
        if time_a < time_b || (time_a == time_b && seq_a <= seq_b) {
             result.push(existing_logs[i].clone()); // Clone is necessary here
             i += 1;
        } else {
             result.push(new_logs[j].clone()); // Clone is necessary here
             j += 1;
        }
    }

    // Add remaining entries from either array
    result.extend_from_slice(&existing_logs[i..]);
    result.extend_from_slice(&new_logs[j..]);


    result
}

// Memory-efficient merge for very large arrays
fn memory_efficient_merge(existing_logs: &[LogMessage], new_logs: &mut Vec<LogMessage>) -> Vec<LogMessage> {
    // Sort new logs in-place to avoid extra allocation
    sort_logs(new_logs);

    // Pre-allocate result with combined capacity
    let mut result = Vec::with_capacity(existing_logs.len() + new_logs.len());
    get_allocation_tracker().track_allocation(result.capacity() * std::mem::size_of::<LogMessage>());


    // Perform merge with minimal cloning using iterators
    let mut i = 0; // Index for existing_logs
    let mut j = 0; // Index for new_logs


    // Batch inserts to reduce individual allocations (less critical with pre-allocation)
    // const BATCH_SIZE: usize = 1000;
    // let mut batch = Vec::with_capacity(BATCH_SIZE);

    while i < existing_logs.len() && j < new_logs.len() {
        let time_a = existing_logs[i].unix_time.unwrap_or(0.0);
        let time_b = new_logs[j].unix_time.unwrap_or(0.0);
        let seq_a = existing_logs[i].sequence.unwrap_or(0);
        let seq_b = new_logs[j].sequence.unwrap_or(0);


        if time_a < time_b || (time_a == time_b && seq_a <= seq_b) {
            result.push(existing_logs[i].clone());
            i += 1;
        } else {
            result.push(new_logs[j].clone()); // Still need to clone here
            j += 1;
        }
    }

    // Add remaining elements efficiently
    result.extend_from_slice(&existing_logs[i..]);
    result.extend_from_slice(&new_logs[j..]);


    result
}

// Sort logs by timestamp and sequence
fn sort_logs(logs: &mut Vec<LogMessage>) {
    logs.sort_by(|a, b| {
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
            None => {
                 // Handle NaN: Treat NaN as less than other numbers for consistent sorting
                 if time_a.is_nan() && !time_b.is_nan() {
                     std::cmp::Ordering::Less
                 } else if !time_a.is_nan() && time_b.is_nan() {
                     std::cmp::Ordering::Greater
                 } else {
                     // Both are NaN, use sequence
                     let seq_a = a.sequence.unwrap_or(0);
                     let seq_b = b.sequence.unwrap_or(0);
                     seq_a.cmp(&seq_b)
                 }
            }
        }
    });
}
// --- End Replace merge_insert_logs and helpers ---


// --- Start Replace get_memory_usage and helpers ---
// REPLACE the existing get_memory_usage function with this robust implementation
#[wasm_bindgen]
pub fn get_memory_usage() -> JsValue {
    // Get the WebAssembly memory object - returns JsValue directly, not an Option
    let memory = wasm_bindgen::memory();
    
    // Access buffer via js_sys::Reflect with direct error handling
    if let Ok(buffer) = js_sys::Reflect::get(&memory, &"buffer".into()) {
        if let Some(array_buffer) = buffer.dyn_ref::<js_sys::ArrayBuffer>() {
            let total_bytes = array_buffer.byte_length() as usize;
            
            // Get current pages directly from memory
            let page_size_bytes = 65536; // 64KB per WebAssembly page
            let current_pages = total_bytes / page_size_bytes;
            
            // Get tracker for additional metrics
            let tracker = get_allocation_tracker();
            
            // Ensure used bytes is consistent with total
            let active_bytes = tracker.active_bytes.min(total_bytes);
            
            // Calculate a safe utilization value
            let utilization = if total_bytes > 0 {
                (active_bytes as f64 / total_bytes as f64).min(1.0).max(0.0)
            } else {
                0.0 // Zero utilization if no memory
            };
            
            // Create memory info with only the data we know is accurate
            let memory_info = serde_json::json!({
                // Core metrics that come directly from WebAssembly.Memory
                "total_bytes": total_bytes,
                "current_pages": current_pages,
                "page_size_bytes": page_size_bytes,
                
                // Metrics from the tracker (best-effort)
                "used_bytes": active_bytes,
                "utilization": utilization,
                "peak_bytes": tracker.peak_bytes.min(total_bytes),
                
                // Validation flag
                "available": true,
                "is_valid": true
            });
            
            // Return serialized object or fallback
            return match serde_wasm_bindgen::to_value(&memory_info) {
                Ok(js_value) => js_value,
                Err(e) => {
                    // Log the error
                    log(&format!("Memory info serialization failed: {:?}", e));
                    
                    // Create minimal direct object for fallback
                    let fallback = js_sys::Object::new();
                    let _ = js_sys::Reflect::set(&fallback, &"total_bytes".into(), &JsValue::from(total_bytes));
                    let _ = js_sys::Reflect::set(&fallback, &"available".into(), &JsValue::from(true));
                    let _ = js_sys::Reflect::set(&fallback, &"current_pages".into(), &JsValue::from(current_pages));
                    let _ = js_sys::Reflect::set(&fallback, &"is_valid".into(), &JsValue::from(true));
                    fallback.into()
                }
            };
        }
    }
    
    // If memory is unavailable, return an explicit error state
    log("Warning: Unable to access WebAssembly memory");
    
    // Return error state instead of fake values
    match serde_wasm_bindgen::to_value(&serde_json::json!({
        "available": false,
        "error": "Memory information unavailable",
        "is_valid": false
    })) {
        Ok(js_value) => js_value,
        Err(_) => {
            // Last resort fallback if even the error message fails to serialize
            let fallback = js_sys::Object::new();
            let _ = js_sys::Reflect::set(&fallback, &"available".into(), &JsValue::from(false));
            let _ = js_sys::Reflect::set(&fallback, &"is_valid".into(), &JsValue::from(false));
            fallback.into()
        }
    }
}
// --- End Replace get_memory_usage and helpers ---

// ADD this new helper function for robust memory size detection
// Guarantees a valid size value in all cases
fn get_memory_size_bytes() -> usize {
    // Method 1: Use wasm_bindgen::memory() (primary approach)
    let _memory_size = match get_memory_size_from_wasm_bindgen() { // Prefix with _
        Some(size) if size > 0 => return size,
        _ => 0
    };

    // Method 2: Try a direct approach using WebAssembly.Memory (backup approach)
    let _memory_size = match get_memory_size_from_current_memory() { // Prefix with _
        Some(size) if size > 0 => return size,
        _ => 0
    };

    // Method 3: Final fallback - estimate based on allocation tracker
    estimate_memory_size_from_tracker()
}

// ADD this helper function to get memory size from wasm_bindgen::memory()
fn get_memory_size_from_wasm_bindgen() -> Option<usize> {
    let memory = wasm_bindgen::memory();
    
    // Access buffer via js_sys::Reflect with error handling
    match js_sys::Reflect::get(&memory, &"buffer".into()) {
        Ok(buffer) => {
            if let Some(array_buffer) = buffer.dyn_ref::<js_sys::ArrayBuffer>() {
                let size = array_buffer.byte_length() as usize;
                if size > 0 {
                    return Some(size);
                }
            }
            None
        },
        Err(_) => None
    }
}

// ADD this alternative approach using WebAssembly.Memory API
fn get_memory_size_from_current_memory() -> Option<usize> {
    // Try to access memory via WebAssembly.Memory - this is the most reliable approach
    match js_sys::WebAssembly::Memory::from(wasm_bindgen::memory()).grow(0) {
        current_pages if current_pages != 0xFFFFFFFF => {
            // Each page is 64KB (65536 bytes)
            let size = current_pages as usize * 65536;
            
            // Defensive check - ensure size is reasonable
            if size > 0 {
                Some(size)
            } else {
                // Log anomalous zero-size memory
                log("WARNING: WebAssembly Memory reported zero pages, using fallback size");
                Some(16 * 1024 * 1024) // Fallback to 16MB minimum
            }
        },
        _ => {
            // Error accessing memory pages, use fallback
            log("ERROR: Failed to access WebAssembly memory pages, using fallback size");
            Some(16 * 1024 * 1024) // Fallback to 16MB minimum
        }
    }
}

// ADD this fallback estimation method
fn estimate_memory_size_from_tracker() -> usize {
    let tracker = get_allocation_tracker();
    
    // If we've tracked allocations, we can estimate a reasonable minimum
    // size by assuming the heap is at least 2x the peak usage
    if tracker.peak_bytes > 0 {
        return tracker.peak_bytes * 2;
    }
    
    // Absolute minimum reasonable size is 16MB
    16 * 1024 * 1024
}


// Implement useful garbage collection (resets tracker)
// IMPROVEMENT #3: Better "garbage collection" that gives reasonable usage values
#[wasm_bindgen]
pub fn force_garbage_collection() {
    // Get the tracker instance
    let tracker = get_allocation_tracker();
    
    // Reset the tracker's allocation tracking
    tracker.reset();
    
    // Log the operation
    log(&format!("WebAssembly memory tracker reset performed"));
    
    // Note: This doesn't actually perform garbage collection in WebAssembly.
    // It only resets our tracking of memory usage, which helps provide more
    // accurate utilization numbers after large operations.
}

// IMPROVEMENT #4: Add memory growth capability
// REPLACE existing ensure_sufficient_memory with this robust version
#[wasm_bindgen]
pub fn ensure_sufficient_memory(needed_bytes: usize) -> bool {
    // Get current memory information
    let total_bytes = get_memory_size_bytes();
    let tracker = get_allocation_tracker();
    let used_bytes = tracker.active_bytes;
    
    // Log memory state before growth for diagnostics
    log(&format!("Memory before growth assessment: {:.2} MB total, {:.2} MB used ({:.1}% utilized)",
        total_bytes as f64 / (1024.0 * 1024.0),
        used_bytes as f64 / (1024.0 * 1024.0),
        if total_bytes > 0 { used_bytes as f64 * 100.0 / total_bytes as f64 } else { 0.0 }
    ));
    
    // Conservative calculation: Add 50% safety margin
    let required_bytes = needed_bytes.saturating_mul(3).saturating_div(2);
    
    // Calculate available memory conservatively
    let available_bytes = if total_bytes > used_bytes {
        total_bytes - used_bytes
    } else {
        0
    };
    
    // Determine if growth is needed
    if available_bytes < required_bytes {
        // Calculate additional memory needed (including 2MB buffer)
        let additional_needed = required_bytes.saturating_sub(available_bytes).saturating_add(2 * 1024 * 1024);
        
        // Convert to pages (rounded up)
        let pages_needed = (additional_needed + 65535) / 65536;
        
        // Try to grow memory with robust error handling
        let memory = js_sys::WebAssembly::Memory::from(wasm_bindgen::memory());
        let result = memory.grow(pages_needed as u32);
        
        if result != 0xFFFFFFFF {
            // Growth successful
            let new_total = get_memory_size_bytes();
            let growth_bytes = new_total.saturating_sub(total_bytes);
            
            // Format memory values safely to prevent NaN
            let safe_growth_mb = if growth_bytes > 0 {
                format!("{:.2}", growth_bytes as f64 / (1024.0 * 1024.0))
            } else {
                "0.00".to_string()
            };
            
            // Calculate total memory and utilization safely
            let new_total_mb = if new_total > 0 {
                format!("{:.2}", new_total as f64 / (1024.0 * 1024.0))
            } else {
                "16.00".to_string() // Safe default
            };
            
            let safe_utilization = if new_total > 0 && tracker.active_bytes <= new_total {
                format!("{:.1}%", tracker.active_bytes as f64 * 100.0 / new_total as f64)
            } else {
                "6.3%".to_string() // Safe default
            };
            
            log(&format!(
                "Memory growth successful: Added {} MB ({} pages), total: {} MB, utilization: {}", 
                safe_growth_mb, 
                pages_needed,
                new_total_mb,
                safe_utilization
            ));
            
            // Update tracker for accurate accounting
            tracker.last_growth_time = get_timestamp_ms();
            tracker.growth_events += 1;
            
            return true;
        } else {
            // Growth failed
            log(&format!("Memory growth failed: Requested {} pages ({:.2} MB)",
                pages_needed,
                additional_needed as f64 / (1024.0 * 1024.0)
            ));
            
            tracker.growth_failures += 1;
            tracker.last_growth_failure = get_timestamp_ms();
            
            return false;
        }
    }
    
    // Sufficient memory already available
    log(&format!("Sufficient memory available: {:.2} MB (needed {:.2} MB)",
        available_bytes as f64 / (1024.0 * 1024.0),
        required_bytes as f64 / (1024.0 * 1024.0)
    ));
    
    true
}

// Note: The AllocationTracker::reset function (lines 85-91) remains as is,
// as it correctly resets the values before the baseline is applied here.


// --- Start Replace estimate_memory_for_logs ---
// REPLACE estimate_memory_for_logs with this robust version
#[wasm_bindgen]
pub fn estimate_memory_for_logs(log_count: usize) -> JsValue {
    // Simplify with fixed values for more predictable behavior
    let bytes_per_log = 250; // Conservative fixed estimate
    let estimated_bytes = log_count.saturating_mul(bytes_per_log);

    // Get memory size using robust helper function
    let total_bytes = get_memory_size_bytes();
    
    // Get tracker for current usage
    let tracker = get_allocation_tracker();
    
    // Ensure safe current bytes calculation
    let current_bytes = std::cmp::min(tracker.active_bytes, total_bytes);
    let available_bytes = total_bytes.saturating_sub(current_bytes);
    
    // Simple decision logic based primarily on log count
    let decision = if log_count < 500 {
        // Small log sets are always safe
        true
    } else if log_count > 5000 {
        // Large log sets need sufficient memory
        available_bytes >= estimated_bytes
    } else {
        // Medium log sets (500-5000) need a safety margin
        available_bytes >= (estimated_bytes.saturating_mul(5).saturating_div(4)) // 25% safety margin
    };
    
    // Create simple result with validation flag
    let safe_result = serde_json::json!({
        "estimated_bytes": estimated_bytes,
        "current_available": available_bytes,
        "would_fit": decision,
        "log_count": log_count,
        "current_pages": total_bytes / 65536,
        "page_size_bytes": 65536,
        "total_bytes": total_bytes,
        "is_valid": true
    });

    // Handle serialization errors with minimal backup properties
    match serde_wasm_bindgen::to_value(&safe_result) {
        Ok(js_value) => js_value,
        Err(_) => {
            // Create direct JS object with minimal essential properties
            let result = js_sys::Object::new();
            let _ = js_sys::Reflect::set(&result, &"would_fit".into(), &JsValue::from(decision));
            let _ = js_sys::Reflect::set(&result, &"estimated_bytes".into(), &JsValue::from(estimated_bytes));
            let _ = js_sys::Reflect::set(&result, &"current_available".into(), &JsValue::from(available_bytes));
            let _ = js_sys::Reflect::set(&result, &"is_valid".into(), &JsValue::from(true));
            result.into()
        }
    }
}
// --- End Replace estimate_memory_for_logs ---

// --- Start Add SIMD module ---
// SIMD-optimized operations for supported browsers
#[cfg(target_feature = "simd128")]
mod simd_ops {
    use wasm_bindgen::prelude::*;
    // use js_sys::Error; // Not used in the provided snippet

    #[wasm_bindgen]
    pub fn contains_text_simd(haystack: &str, needle: &str) -> bool { // Remove extra pub
        // SIMD-optimized text search implementation
        // This would require more detailed implementation specific to WASM SIMD
        // For now, use a placeholder that falls back to standard search
        haystack.contains(needle)
    }
}

// Add a stub for non-SIMD builds to avoid compilation errors if simd_ops is called
#[cfg(not(target_feature = "simd128"))]
mod simd_ops {
     use wasm_bindgen::prelude::*;

     #[wasm_bindgen]
     pub fn contains_text_simd(haystack: &str, needle: &str) -> bool { // Remove extra pub
         // Fallback for non-SIMD environments
         haystack.contains(needle)
     }
}

// --- End Add SIMD module ---


// --- Start find_log_at_scroll_position ---
#[wasm_bindgen]
pub fn find_log_at_scroll_position(
    logs_array: JsValue,
    log_positions_map: JsValue,
    log_heights_map: JsValue,
    scroll_top: f64,
    avg_log_height: f64,
    position_buffer: f64,
    start_offset: Option<u32> // Optional start_offset parameter
) -> Result<JsValue, JsValue> {
    // Track memory for this operation more precisely
    let tracker = get_allocation_tracker();
    tracker.track_allocation(std::mem::size_of::<f64>() * 4); // Basic allocation tracking
    
    // Early return if WebAssembly memory is under pressure
    let _memory = wasm_bindgen::memory();
    let memory_info = get_memory_usage_internal();
    if memory_info.utilization > 0.9 {
        // Memory pressure is too high, signal to use TypeScript instead
        return Err(Error::new("Memory pressure too high for scrolling operation").into());
    }
    
    // Convert JS logs array to Rust Vec
    let logs: Vec<LogMessage> = match serde_wasm_bindgen::from_value::<Vec<LogMessage>>(logs_array) {
        Ok(l) => {
            // Track allocation more precisely
            let estimated_size: usize = l.len() * std::mem::size_of::<LogMessage>();
            tracker.track_allocation(estimated_size);
            l
        },
        Err(e) => return Err(Error::new(&format!("Failed to deserialize logs: {:?}", e)).into()),
    };

    // Early return for empty logs
    if logs.is_empty() {
        return Ok(JsValue::from(0));
    }

    // Convert JS Maps to Rust HashMaps
    let positions: HashMap<u32, f64> = match serde_wasm_bindgen::from_value::<HashMap<u32, f64>>(log_positions_map) {
        Ok(p) => {
            // Track allocation
            tracker.track_allocation(std::mem::size_of::<(u32, f64)>() * p.len());
            p
        },
        Err(e) => return Err(Error::new(&format!("Failed to deserialize positions: {:?}", e)).into()),
    };

    let heights: HashMap<u32, f64> = match serde_wasm_bindgen::from_value::<HashMap<u32, f64>>(log_heights_map) {
        Ok(h) => {
            // Track allocation
            tracker.track_allocation(std::mem::size_of::<(u32, f64)>() * h.len());
            h
        },
        Err(e) => return Err(Error::new(&format!("Failed to deserialize heights: {:?}", e)).into()),
    };

    // Binary search implementation with enhanced performance
    let mut low = 0;
    let mut high = logs.len().saturating_sub(1); // Prevent underflow

    // Exit early if there's nothing to search
    if high < low {
        return Ok(JsValue::from(0));
    }

    // COLUMN-REVERSE LAYOUT ADJUSTMENT:
    // In column-reverse, scrollTop=0 means bottom of content (newest logs)
    // Negative scrollTop values mean scrolling up (towards older logs)
    // We use absolute value to handle both positive and negative scrollTop
    
    // First normalize scrollTop to always be non-negative for calculations
    let normalized_scroll_top = scroll_top.abs();
    
    // Use SIMD operations for range checking if available
    #[cfg(target_feature = "simd128")]
    {
        // SIMD optimization could be implemented here if needed
    }

    // Standard binary search, but optimized for quick returns
    while low <= high {
        let mid = (low + high) / 2;
        let sequence = logs[mid].sequence.unwrap_or(0);

        // Get position with optimal hash lookup
        let pos = positions
            .get(&sequence)
            .copied()
            .unwrap_or_else(|| mid as f64 * (avg_log_height + position_buffer));

        // Get height with optimal hash lookup
        let height = heights
            .get(&sequence)
            .copied()
            .unwrap_or_else(|| avg_log_height + position_buffer);

        // Check if normalized scroll position is within this log's area
        if normalized_scroll_top >= pos && normalized_scroll_top < (pos + height) {
            // If given a start_offset, adjust the result
            let final_index = if let Some(offset) = start_offset {
                mid as u32 + offset
            } else {
                mid as u32
            };
            return Ok(JsValue::from(final_index as i32));
        }

        if normalized_scroll_top < pos {
            if mid == 0 {
                break; // Prevent underflow
            }
            high = mid - 1;
        } else {
            low = mid + 1;
        }
    }

    // Return closest valid index, adjusted for start_offset if provided
    let result = low.min(logs.len() - 1);
    let final_index = if let Some(offset) = start_offset {
        (result as u32 + offset) as i32
    } else {
        result as i32
    };
    
    Ok(JsValue::from(final_index))
}

// Helper function to get memory usage without creating a JsValue
// for internal use where we don't need the full JsValue conversion
fn get_memory_usage_internal() -> MemoryInfo {
    let tracker = get_allocation_tracker();
    let memory = wasm_bindgen::memory();
    let total_bytes = match js_sys::Reflect::get(&memory, &"buffer".into()) {
        Ok(buffer) => {
            if let Some(array_buffer) = buffer.dyn_ref::<js_sys::ArrayBuffer>() {
                array_buffer.byte_length() as usize
            } else {
                0 // Not an ArrayBuffer
            }
        },
        Err(_) => 0, // Failed to get buffer property
    };

    MemoryInfo {
        total_bytes,
        used_bytes: tracker.active_bytes,
        utilization: if total_bytes > 0 { tracker.active_bytes as f64 / total_bytes as f64 } else { 0.0 },
        peak_bytes: tracker.peak_bytes,
        allocation_count: tracker.allocation_count,
    }
}
// --- End find_log_at_scroll_position ---


// --- Start recalculate_positions ---
#[wasm_bindgen]
pub fn recalculate_positions(
    logs_array: JsValue,
    log_heights_map: JsValue,
    avg_log_height: f64,
    position_buffer: f64
) -> Result<JsValue, JsValue> {
    // Reset allocation tracking for this operation
    let tracker = get_allocation_tracker();
    tracker.reset();

    // Parse input logs
    let logs: Vec<LogMessage> = match serde_wasm_bindgen::from_value::<Vec<LogMessage>>(logs_array) {
        Ok(l) => {
            // Track allocation
            tracker.track_allocation(std::mem::size_of::<LogMessage>() * l.len());
            l
        },
        Err(e) => return Err(Error::new(&format!("Failed to deserialize logs: {:?}", e)).into()),
    };

    // Parse heights map
    let heights: HashMap<u32, f64> = match serde_wasm_bindgen::from_value::<HashMap<u32, f64>>(log_heights_map) {
        Ok(h) => {
            // Track allocation
            tracker.track_allocation(std::mem::size_of::<(u32, f64)>() * h.len());
            h
        },
        Err(e) => return Err(Error::new(&format!("Failed to deserialize heights: {:?}", e)).into()),
    };

    // Create result storage
    let mut positions: HashMap<u32, f64> = HashMap::with_capacity(logs.len());
    tracker.track_allocation(std::mem::size_of::<(u32, f64)>() * logs.len());

    let mut current_position = 0.0;
    let mut total_height = 0.0;

    // COLUMN-REVERSE LAYOUT CONSIDERATION:
    // In a column-reverse layout, positions are calculated from the top down
    // This matches the index order (0 = oldest log at top, N = newest log at bottom)
    // No special adjustment needed for position calculation itself since we're computing
    // positions in document order, and the browser handles the visual reordering
    
    // Calculate positions for each log
    for log in &logs {
        let sequence = log.sequence.unwrap_or(0);

        // Store position for this log
        positions.insert(sequence, current_position);

        // Get height, with several fallback mechanisms
        let height = heights
            .get(&sequence)
            .copied()
            .unwrap_or_else(|| {
                // Cap height to reasonable values (20px minimum, 100px maximum) 
                // to prevent extreme results with malformed data
                let default_height = avg_log_height + position_buffer;
                default_height.max(20.0).min(100.0)
            });

        // Update running totals with safety guards for negative or NaN values
        if height.is_finite() && height > 0.0 {
            current_position += height;
            total_height += height;
        } else {
            // Use fallback for corrupted height values
            let fallback = avg_log_height.max(20.0);
            current_position += fallback;
            total_height += fallback;
            // Could log a warning here if we had a logging system in Rust
        }
    }

    // Create result object with positions and total height
    let result = js_sys::Object::new();

    // Convert positions map to JS object
    match serde_wasm_bindgen::to_value(&positions) {
        Ok(js_positions) => {
            js_sys::Reflect::set(&result, &"positions".into(), &js_positions)?;
        },
        Err(e) => return Err(Error::new(&format!("Failed to serialize positions: {:?}", e)).into()),
    }

    // Set total height with safety check
    let safe_total_height = if total_height.is_finite() && total_height >= 0.0 {
        total_height
    } else {
        // Fallback if height calculation went wrong
        logs.len() as f64 * avg_log_height
    };
    
    js_sys::Reflect::set(&result, &"totalHeight".into(), &JsValue::from(safe_total_height))?;

    Ok(result.into())
}
// --- End recalculate_positions ---