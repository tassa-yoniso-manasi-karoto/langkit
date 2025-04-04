// src/wasm/lib.rs - Optimized version with better memory management
use wasm_bindgen::prelude::*;
use serde::{Serialize, Deserialize};
use js_sys::Error;
use std::collections::HashMap; // Needed for extra_fields

// Use a static mutable variable for the allocation tracker.
// This requires unsafe blocks for access, which is common in FFI contexts.
static mut ALLOCATION_TRACKER: Option<AllocationTracker> = None;

struct AllocationTracker {
    active_bytes: usize,
    peak_bytes: usize,
    allocation_count: usize,
}

impl AllocationTracker {
    fn new() -> Self {
        Self {
            active_bytes: 0,
            peak_bytes: 0,
            allocation_count: 0,
        }
    }

    // Helper to track allocations (approximate size)
    fn track_allocation(&mut self, bytes: usize) {
        self.active_bytes += bytes;
        self.allocation_count += 1;
        if self.active_bytes > self.peak_bytes {
            self.peak_bytes = self.active_bytes;
        }
    }

    // Helper to track deallocations (approximate size)
    // Note: Accurate deallocation tracking is complex without a custom allocator.
    // This is a placeholder and might not be perfectly accurate.
    fn track_deallocation(&mut self, bytes: usize) {
        if bytes <= self.active_bytes {
            self.active_bytes -= bytes;
        } else {
            // This shouldn't happen in normal operation, but guard against underflow
            self.active_bytes = 0;
        }
    }

    fn reset(&mut self) {
        self.active_bytes = 0;
        self.allocation_count = 0;
        // Keep peak_bytes for historical tracking unless explicitly reset
    }
}

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


#[wasm_bindgen]
pub fn merge_insert_logs(existing_logs_js: JsValue, new_logs_js: JsValue) -> Result<JsValue, JsValue> {
    // Reset allocation tracking for this specific operation
    get_allocation_tracker().reset();
    
    // Handle empty arrays as special cases for efficiency
    if js_sys::Array::is_array(&new_logs_js) && js_sys::Array::from(&new_logs_js).length() == 0 {
        return Ok(existing_logs_js);
    }
    
    if js_sys::Array::is_array(&existing_logs_js) && js_sys::Array::from(&existing_logs_js).length() == 0 {
        return Ok(new_logs_js);
    }
    
    // Deserialize logs with error handling
    let existing_logs: Vec<LogMessage> = match serde_wasm_bindgen::from_value(existing_logs_js) {
        Ok(logs) => {
            // Track this allocation approximately
            let estimated_size: usize = logs.iter().map(estimate_log_message_size).sum();
            get_allocation_tracker().track_allocation(estimated_size);
            logs
        },
        Err(e) => return Err(Error::new(&format!("Failed to deserialize existing logs: {:?}", e)).into()),
    };
    
    let mut new_logs: Vec<LogMessage> = match serde_wasm_bindgen::from_value(new_logs_js) {
        Ok(logs) => {
            // Track this allocation too
            let estimated_size: usize = logs.iter().map(estimate_log_message_size).sum();
            get_allocation_tracker().track_allocation(estimated_size);
            logs
        },
        Err(e) => return Err(Error::new(&format!("Failed to deserialize new logs: {:?}", e)).into()),
    };
    
    // Sort new logs efficiently in place to avoid cloning
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
    
    // Pre-allocate the result vector to avoid reallocations
    let total_capacity = existing_logs.len() + new_logs.len();
    let mut result = Vec::with_capacity(total_capacity);
    
    // Track this allocation (vector capacity itself)
    // Note: This doesn't account for the size of the elements yet, as they are moved/cloned below.
    get_allocation_tracker().track_allocation(total_capacity * std::mem::size_of::<LogMessage>());
    
    // Use efficient in-place merging algorithm
    let mut i = 0;
    let mut j = 0;
    
    while i < existing_logs.len() && j < new_logs.len() {
        let time_a = existing_logs[i].unix_time.unwrap_or(0.0);
        let time_b = new_logs[j].unix_time.unwrap_or(0.0);
        
        // Compare timestamps with safe handling for NaN values
        match time_a.partial_cmp(&time_b) {
            Some(std::cmp::Ordering::Less) | Some(std::cmp::Ordering::Equal) => {
                // Clone from existing_logs (moving is complex with Vec ownership here)
                result.push(existing_logs[i].clone());
                i += 1;
            },
            Some(std::cmp::Ordering::Greater) => {
                // Clone from new_logs
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
    
    // Add any remaining entries by cloning
    while i < existing_logs.len() {
        result.push(existing_logs[i].clone());
        i += 1;
    }
    
    while j < new_logs.len() {
        result.push(new_logs[j].clone());
        j += 1;
    }
    
    // Estimate the size of the final result vector for tracking
    let final_result_size: usize = result.iter().map(estimate_log_message_size).sum();
    // We allocated space earlier, now adjust based on final content size
    // This is still approximate. A custom allocator would be needed for precision.
    // Let's assume the initial capacity allocation was roughly correct for now.

    // Serialize back to JsValue with error handling
    match serde_wasm_bindgen::to_value(&result) {
        Ok(js_array) => Ok(js_array),
        Err(e) => Err(Error::new(&format!("Failed to serialize result: {:?}", e)).into()),
    }
}


// Memory management utilities with improved accuracy
#[wasm_bindgen]
pub fn get_memory_usage() -> JsValue {
    let tracker = get_allocation_tracker();
    
    let memory = wasm_bindgen::memory();
    let total_bytes = match memory.grow(0) { // grow(0) returns current page count
        Ok(pages) => pages * 65536, // WebAssembly page size is 64KiB
        Err(_) => 0, // Failed to get memory info
    };
    
    // Use our tracked allocations for more accurate reporting
    let used_bytes = tracker.active_bytes;
    
    let memory_info = MemoryInfo {
        total_bytes,
        used_bytes,
        utilization: if total_bytes > 0 { used_bytes as f64 / total_bytes as f64 } else { 0.0 },
        peak_bytes: tracker.peak_bytes,
        allocation_count: tracker.allocation_count,
    };
    
    match serde_wasm_bindgen::to_value(&memory_info) {
        Ok(js_value) => js_value,
        Err(_) => JsValue::NULL, // Return null if serialization fails
    }
}


// Implement useful garbage collection (resets tracker)
#[wasm_bindgen]
pub fn force_garbage_collection() {
    // Reset our allocation tracking
    get_allocation_tracker().reset();
    
    // Log the action
    log("WebAssembly garbage collection: reset allocation tracking");
    
    // In a real implementation with actual caches, we would clear them here
    // For now, this at least provides accurate memory tracking reset
}


// Add a function to estimate memory for a given log count
#[wasm_bindgen]
pub fn estimate_memory_for_logs(log_count: usize) -> JsValue {
    // Approximate size of a LogMessage (use a constant average for estimation)
    const AVG_LOG_MESSAGE_SIZE: usize = 250; // Average size including string fields, adjust as needed
    
    // Estimate memory needed for the logs themselves
    let estimated_bytes = log_count * AVG_LOG_MESSAGE_SIZE;
    
    // Get current memory info
    let memory = wasm_bindgen::memory();
     let total_bytes = match memory.grow(0) { // grow(0) returns current page count
        Ok(pages) => pages * 65536, // WebAssembly page size is 64KiB
        Err(_) => 0, // Failed to get memory info
    };
    let current_used = get_allocation_tracker().active_bytes;
    let current_available = if total_bytes >= current_used { total_bytes - current_used } else { 0 };

    // Create result object using serde_json
    let result = serde_json::json!({
        "estimated_bytes": estimated_bytes,
        "current_available": current_available,
        "would_fit": current_available >= estimated_bytes,
    });
    
    match serde_wasm_bindgen::to_value(&result) {
        Ok(js_value) => js_value,
        Err(_) => JsValue::NULL,
    }
}