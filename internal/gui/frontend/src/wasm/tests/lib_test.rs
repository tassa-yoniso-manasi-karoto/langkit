#[cfg(test)]
mod tests {
    // Need to import from parent crate, which is exposed by wasm_bindgen
    use wasm_bindgen_test::*;
    use wasm_bindgen::JsValue;

    // Import the functions from the parent crate that are exposed with wasm_bindgen
    use wasm_bindgen::prelude::*;
    
    // Import the crate functions directly
    use log_engine::{merge_insert_logs, get_memory_usage, force_garbage_collection};

    wasm_bindgen_test_configure!(run_in_browser);

    #[wasm_bindgen_test]
    fn test_empty_arrays() {
        // Test empty arrays handling
        let empty_array1 = js_sys::Array::new();
        let empty_array2 = js_sys::Array::new();
        let empty_array3 = js_sys::Array::new();
        let some_logs1 = create_test_logs(5);
        let some_logs2 = create_test_logs(5);
        
        // Empty new logs should return existing logs unchanged
        let result = merge_insert_logs(some_logs1.into(), empty_array1.into()).unwrap();
        assert_eq!(js_sys::Array::from(&result).length(), 5);
        
        // Empty existing logs should return new logs unchanged
        let result = merge_insert_logs(empty_array2.into(), some_logs2.into()).unwrap();
        assert_eq!(js_sys::Array::from(&result).length(), 5);
        
        // Both empty should return empty
        let empty_array3_clone = empty_array3.clone();
        let result = merge_insert_logs(empty_array3_clone.into(), empty_array3.into()).unwrap();
        assert_eq!(js_sys::Array::from(&result).length(), 0);
    }

    #[wasm_bindgen_test]
    fn test_merge_sorted_arrays() {
        // Create two sorted arrays
        let logs1 = create_sorted_logs(1, 5); // 5 logs starting at time 1
        let logs2 = create_sorted_logs(6, 5); // 5 logs starting at time 6
        
        // Merge them
        let result = merge_insert_logs(logs1.into(), logs2.into()).unwrap();
        let result_array = js_sys::Array::from(&result);
        
        // Check length and order
        assert_eq!(result_array.length(), 10);
        
        // Verify order is maintained
        for i in 0..9 {
            let time1 = get_unix_time_from_log(&result_array.get(i as u32));
            let time2 = get_unix_time_from_log(&result_array.get((i+1) as u32));
            assert!(time1 <= time2, "Logs not in chronological order");
        }
    }

    #[wasm_bindgen_test]
    fn test_merge_with_duplicates() {
        // Create logs with same timestamps
        let logs1 = create_logs_with_timestamps(&[1.0, 2.0, 3.0, 4.0, 5.0]);
        let logs2 = create_logs_with_timestamps(&[2.0, 3.0, 6.0, 7.0]);
        
        // Merge them
        let result = merge_insert_logs(logs1.into(), logs2.into()).unwrap();
        let result_array = js_sys::Array::from(&result);
        
        // Check total length
        assert_eq!(result_array.length(), 9);
        
        // Verify order is maintained
        for i in 0..8 {
            let time1 = get_unix_time_from_log(&result_array.get(i as u32));
            let time2 = get_unix_time_from_log(&result_array.get((i+1) as u32));
            assert!(time1 <= time2, "Logs not in chronological order");
        }
    }

    #[wasm_bindgen_test]
    fn test_sequence_tie_breaker() {
        // Create logs with same timestamps but different sequences
        let log1 = create_log_with_sequence(1.0, 1);
        let log2 = create_log_with_sequence(1.0, 2);
        
        let logs1 = js_sys::Array::new();
        logs1.push(&log1);
        
        let logs2 = js_sys::Array::new();
        logs2.push(&log2);
        
        // Merge them
        let result = merge_insert_logs(logs1.into(), logs2.into()).unwrap();
        let result_array = js_sys::Array::from(&result);
        
        // Check order (sequence 1 should come before sequence 2)
        assert_eq!(result_array.length(), 2);
        let seq1 = get_sequence_from_log(&result_array.get(0));
        let seq2 = get_sequence_from_log(&result_array.get(1));
        assert!(seq1 < seq2, "Sequence tie-breaker not working");
    }

    #[wasm_bindgen_test]
    fn test_memory_tracking() {
        // Test memory tracking 
        let before = get_memory_usage();
        let before_obj = js_sys::Object::from(before.clone());
        let before_bytes = js_sys::Reflect::get(&before_obj, &"used_bytes".into()).unwrap();
        let _before_used = before_bytes.as_f64().unwrap() as usize;
        
        // Create large arrays to force memory allocation
        let large_logs1 = create_test_logs(1000);
        let large_logs2 = create_test_logs(1000);
        
        // Process and discard result to keep reference
        let _ = merge_insert_logs(large_logs1.into(), large_logs2.into()).unwrap();
        
        // Check memory increased
        let after = get_memory_usage();
        let after_obj = js_sys::Object::from(after.clone());
        let after_used = js_sys::Reflect::get(&after_obj, &"used_bytes".into()).unwrap();
        let after_used = after_used.as_f64().unwrap() as usize;
        
        // Memory should have increased (though this depends on when GC runs)
        // We mainly verify it's tracking something
        assert!(after_used > 0, "Memory tracking not working");
        
        // Test force GC
        force_garbage_collection();
        
        // Memory usage after GC
        let after_gc = get_memory_usage();
        let after_gc_obj = js_sys::Object::from(after_gc.clone());
        let after_gc_used = js_sys::Reflect::get(&after_gc_obj, &"used_bytes".into()).unwrap();
        let after_gc_used = after_gc_used.as_f64().unwrap() as usize;
        
        // Memory should have decreased after GC
        assert!(after_gc_used < after_used, "Garbage collection not working");
    }

    // Helper functions
    fn create_test_logs(count: u32) -> js_sys::Array {
        let array = js_sys::Array::new();
        for i in 0..count {
            let time = (1000.0 + i as f64) * 1000.0; // Unique timestamps
            let log = create_log_with_timestamp(time);
            array.push(&log);
        }
        array
    }
    
    fn create_sorted_logs(start_time: u32, count: u32) -> js_sys::Array {
        let array = js_sys::Array::new();
        for i in 0..count {
            let time = (start_time as f64 + i as f64) * 1000.0;
            let log = create_log_with_timestamp(time);
            array.push(&log);
        }
        array
    }
    
    fn create_logs_with_timestamps(times: &[f64]) -> js_sys::Array {
        let array = js_sys::Array::new();
        for (_i, &time) in times.iter().enumerate() { // Prefix unused 'i' with '_'
            let log = create_log_with_timestamp(time * 1000.0);
            array.push(&log);
        }
        array
    }
    
    fn create_log_with_timestamp(time: f64) -> js_sys::Object {
        let log = js_sys::Object::new();
        js_sys::Reflect::set(&log, &"level".into(), &"INFO".into()).unwrap();
        js_sys::Reflect::set(&log, &"message".into(), &"Test message".into()).unwrap();
        js_sys::Reflect::set(&log, &"time".into(), &"12:34:56".into()).unwrap();
        js_sys::Reflect::set(&log, &"_unix_time".into(), &time.into()).unwrap();
        log
    }
    
    fn create_log_with_sequence(time: f64, sequence: u32) -> js_sys::Object {
        let log = create_log_with_timestamp(time * 1000.0);
        js_sys::Reflect::set(&log, &"_sequence".into(), &sequence.into()).unwrap();
        log
    }
    
    fn get_unix_time_from_log(log_value: &JsValue) -> f64 {
        let log_obj = js_sys::Object::from(log_value.clone());
        let time = js_sys::Reflect::get(&log_obj, &"_unix_time".into()).unwrap();
        time.as_f64().unwrap()
    }
    
    fn get_sequence_from_log(log_value: &JsValue) -> u32 {
        let log_obj = js_sys::Object::from(log_value.clone());
        let seq = js_sys::Reflect::get(&log_obj, &"_sequence".into()).unwrap();
        seq.as_f64().unwrap() as u32
    }
}