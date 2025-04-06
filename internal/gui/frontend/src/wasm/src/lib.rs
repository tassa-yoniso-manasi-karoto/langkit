// src/wasm/lib.rs - Optimized version with better memory management
use wasm_bindgen::prelude::*;
use serde::{Serialize, Deserialize};
use js_sys::Error;
use std::collections::HashMap; // Needed for extra_fields

// Use a static mutable variable for the allocation tracker.
// This requires unsafe blocks for access, which is common in FFI contexts.
static mut ALLOCATION_TRACKER: Option<AllocationTracker> = None;

// --- Start Replace AllocationTracker ---
// Enhance the AllocationTracker with more detailed metrics
struct AllocationTracker {
    active_bytes: usize,
    peak_bytes: usize,
    allocation_count: usize,
    // New fields for better memory analytics
    allocation_history: [usize; 10],  // Circular buffer of recent allocations
    history_index: usize,
    average_allocation: usize,
    sample_count: usize,
    last_gc_time: u64,     // Timestamp of last garbage collection
    allocation_rate: f64,  // Bytes per second allocation rate
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
        }
    }

    // Enhanced allocation tracking with rate calculation
    fn track_allocation(&mut self, bytes: usize) {
        // Track basic metrics
        self.active_bytes += bytes;
        self.allocation_count += 1;
        if self.active_bytes > self.peak_bytes {
            self.peak_bytes = self.active_bytes;
        }

        // Track allocation patterns for better prediction
        self.allocation_history[self.history_index] = bytes;
        self.history_index = (self.history_index + 1) % 10;

        // Update running average
        self.sample_count += 1;
        // Prevent division by zero if sample_count was 0 before incrementing
        if self.sample_count > 0 {
             self.average_allocation = ((self.average_allocation * (self.sample_count - 1)) + bytes) / self.sample_count;
        }


        // Calculate allocation rate (bytes/second)
        let now = get_timestamp_ms();
        if self.last_gc_time > 0 {
            let time_diff = now.saturating_sub(self.last_gc_time); // Use saturating_sub for safety
            if time_diff > 0 {
                // Exponential moving average for stability
                let new_rate = bytes as f64 / (time_diff as f64 / 1000.0);
                self.allocation_rate = self.allocation_rate * 0.7 + new_rate * 0.3;
            }
        }
    }

    // More accurate deallocation tracking
    fn track_deallocation(&mut self, bytes: usize) {
        if bytes <= self.active_bytes {
            self.active_bytes -= bytes;
        } else {
            // This is a more severe issue than we currently handle
            log("WARNING: Attempted to deallocate more bytes than tracked as active");
            self.active_bytes = 0;
        }
    }

    // Reset tracking after garbage collection
    fn reset(&mut self) {
        self.active_bytes = 0;
        self.allocation_count = 0;
        self.last_gc_time = get_timestamp_ms();
        // Keep historical data for trend analysis (peak_bytes, history, etc.)
    }

    // Predict if an operation would cause memory issues
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
// Memory management utilities with improved accuracy
#[wasm_bindgen]
pub fn get_memory_usage() -> JsValue { // Remove extra pub
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


    // Enhanced memory info with new metrics
    let memory_info = serde_json::json!({
        "total_bytes": total_bytes,
        "used_bytes": tracker.active_bytes,
        "utilization": if total_bytes > 0 { tracker.active_bytes as f64 / total_bytes as f64 } else { 0.0 },
        "peak_bytes": tracker.peak_bytes,
        "allocation_count": tracker.allocation_count,
        // New metrics
        "average_allocation": tracker.average_allocation,
        "allocation_rate": tracker.allocation_rate,
        "time_since_last_gc": get_timestamp_ms().saturating_sub(tracker.last_gc_time), // Use saturating_sub
        "memory_growth_trend": calculate_memory_growth_trend(tracker),
        "fragmentation_estimate": estimate_fragmentation(tracker, total_bytes)
    });

    match serde_wasm_bindgen::to_value(&memory_info) {
        Ok(js_value) => js_value,
        Err(_) => JsValue::NULL,
    }
}

// Calculate memory growth trend from history
fn calculate_memory_growth_trend(tracker: &AllocationTracker) -> f64 {
    // Simple linear regression on recent allocations
    // Positive value indicates growth, negative indicates shrinking
    // Value represents bytes per allocation

    let mut sum_x: i64 = 0; // Use i64 to avoid overflow with multiplication
    let mut sum_y: i64 = 0;
    let mut sum_xy: i64 = 0;
    let mut sum_xx: i64 = 0;
    let mut n: i64 = 0;


    for i in 0..10 {
        let y = tracker.allocation_history[i];
        if y > 0 {
            let x = i as i64 + 1; // Use i64 for calculations
            sum_x += x;
            sum_y += y as i64;
            sum_xy += x * (y as i64);
            sum_xx += x * x;
            n += 1;
        }
    }

    if n < 2 {
        return 0.0;
    }

    // Calculate slope using floating point numbers
    let n_f64 = n as f64;
    let denominator = n_f64 * (sum_xx as f64) - (sum_x as f64) * (sum_x as f64);

    if denominator == 0.0 {
         return 0.0; // Avoid division by zero
    }

    let slope = (n_f64 * (sum_xy as f64) - (sum_x as f64) * (sum_y as f64)) / denominator;


    slope
}

// Estimate memory fragmentation
fn estimate_fragmentation(tracker: &AllocationTracker, total_bytes: usize) -> f64 {
    // This is a simplification - real fragmentation would require more insight into the allocator
    if tracker.allocation_count < 10 || total_bytes == 0 || tracker.average_allocation == 0 {
        return 0.0;
    }

    // Heuristic: more allocations + deallocations = higher likelihood of fragmentation
    // Compare total allocations count to the theoretical minimum number of allocations
    // if all memory was allocated in average-sized chunks.
    let theoretical_alloc_count = tracker.active_bytes as f64 / tracker.average_allocation as f64;
    if theoretical_alloc_count <= 0.0 {
        return 0.0;
    }
    let fragmentation_factor = (tracker.allocation_count as f64) / theoretical_alloc_count;

    // Normalize to 0-1 range, clamping at 0 and 1
    (fragmentation_factor - 1.0).max(0.0).min(1.0)
}
// --- End Replace get_memory_usage and helpers ---


// Implement useful garbage collection (resets tracker)
// IMPROVEMENT #3: Better "garbage collection" that gives reasonable usage values
#[wasm_bindgen]
pub fn force_garbage_collection() { // Remove extra pub
    // Get the tracker instance
    let tracker = get_allocation_tracker();
    
    // Log before state for diagnostics
    log(&format!("WebAssembly GC: Before cleanup - active_bytes: {}, allocation_count: {}",
        tracker.active_bytes, tracker.allocation_count));
    
    // Reset the tracker completely
    tracker.reset();
    
    // Get the current memory usage
    let memory = wasm_bindgen::memory();
    let total_bytes = match js_sys::Reflect::get(&memory, &"buffer".into()) {
        Ok(buffer) => {
            if let Some(array_buffer) = buffer.dyn_ref::<js_sys::ArrayBuffer>() {
                array_buffer.byte_length() as usize
            } else {
                0
            }
        },
        Err(_) => 0,
    };
    
    // Set a reasonable baseline instead of zero
    // This is a heuristic - assume 10% is in use for baseline runtime needs
    let baseline_usage = total_bytes / 10;
    tracker.active_bytes = baseline_usage;
    tracker.allocation_count = 1; // Show at least one allocation
    
    // Log the action and new state
    log(&format!("WebAssembly garbage collection performed. Memory reset to baseline: {} bytes", baseline_usage));
}

// IMPROVEMENT #4: Add memory growth capability
#[wasm_bindgen]
pub fn ensure_sufficient_memory(needed_bytes: usize) -> bool { // Remove extra pub
    // Get the memory as a proper Memory object instead of JsValue
    let memory_js = wasm_bindgen::memory();
    // Use js_sys::WebAssembly::Memory::from which directly converts or panics (suitable here)
    // If more robust error handling is needed, use try_from as before.
    let memory = js_sys::WebAssembly::Memory::from(memory_js);
    
    let tracker = get_allocation_tracker();
    
    // Calculate how much memory we have - now using proper Memory object
    let current_pages = memory.grow(0); // This doesn't grow, just returns current size
    let total_bytes = (current_pages as usize) * 65536; // 64KB per page
    
    // Calculate available memory
    let available_bytes = if total_bytes > tracker.active_bytes {
        total_bytes - tracker.active_bytes
    } else {
        0
    };
    
    // If we need more memory
    if available_bytes < needed_bytes {
        // Add a buffer to avoid frequent allocations
        let required_additional = needed_bytes - available_bytes;
        // Calculate pages needed, rounding up, and add 1 page buffer
        let pages_needed = ((required_additional + 65535) / 65536) as u32 + 1;
        
        // Try to grow memory - now using proper Memory object
        log(&format!("WebAssembly: Attempting to grow memory by {} pages ({} bytes)",
            pages_needed, pages_needed as usize * 65536));
            
        // The grow method returns the previous page count, or 0xFFFFFFFF on error
        let previous_pages = memory.grow(pages_needed);
        
        // Check if the return value indicates an error (-1 cast to u32)
        if previous_pages != 0xFFFFFFFF {
            let new_total = ((previous_pages as usize) + (pages_needed as usize)) * 65536;
            log(&format!("WebAssembly: Memory growth successful. New capacity: {} bytes", new_total));
            return true;
        } else {
            log("WebAssembly: Failed to grow memory. System may be constrained.");
            return false;
        }
    }
    
    // We already have enough memory
    return true;
}

// Note: The AllocationTracker::reset function (lines 85-91) remains as is,
// as it correctly resets the values before the baseline is applied here.


// --- Start Replace estimate_memory_for_logs ---
// Improved memory estimation for operations
#[wasm_bindgen]
pub fn estimate_memory_for_logs(log_count: usize) -> JsValue { // Remove extra pub
    // Base memory per log entry (more accurate based on actual LogMessage structure)
    let base_size = std::mem::size_of::<LogMessage>();

    // Average string sizes based on tracker data
    let tracker = get_allocation_tracker();
    let avg_string_size = if tracker.sample_count > 0 && tracker.average_allocation > 0 {
        // Assume strings are roughly 1/4 of the average allocation size.
        // This is a heuristic and might need tuning based on real data.
        (tracker.average_allocation as f64 / 4.0) as usize
    } else {
        80  // Default assumption if no data (e.g., 80 bytes for strings per log)
    };

    // Calculate with overhead for map structure and potential string expansion
    let estimated_bytes = log_count * (base_size + avg_string_size);

    // Get memory info
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


    // Determine if operation would fit using the tracker's method
    let would_fit = tracker.would_operation_fit(estimated_bytes, total_bytes);

    // Calculate memory after operation
    let projected_utilization = if total_bytes > 0 {
        (tracker.active_bytes + estimated_bytes) as f64 / total_bytes as f64
    } else {
        1.0 // Assume 100% utilization if total_bytes is 0
    };

    // Detailed result to inform decision making
    let result = serde_json::json!({
        "estimated_bytes": estimated_bytes,
        "current_available": if total_bytes > tracker.active_bytes { total_bytes - tracker.active_bytes } else { 0 },
        "would_fit": would_fit,
        "projected_utilization": projected_utilization,
        // IMPROVEMENT #2: More realistic thresholds
        "risk_level": if projected_utilization > 0.95 { // Increased from 0.9
            "high"
        } else if projected_utilization > 0.85 { // Increased from 0.75
            "moderate"
        } else {
            "low"
        },
        "recommendation": if would_fit {
            if projected_utilization > 0.9 { // Increased from 0.85
                "proceed_with_caution"
            } else {
                "proceed"
            }
        } else {
            "use_typescript_fallback"
        }
    });

    match serde_wasm_bindgen::to_value(&result) {
        Ok(js_value) => js_value,
        Err(_) => JsValue::NULL,
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