// src/lib/utils.ts
/**
 * Format bytes to human-readable format
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0 || !bytes) return '0 Bytes'; // Handle null/undefined/zero
  
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']; // Added TB
  // Ensure bytes is a positive number for log calculation
  const i = Math.max(0, Math.floor(Math.log(Math.abs(bytes)) / Math.log(k))); 
  
  // Ensure index does not exceed sizes array bounds
  const index = Math.min(i, sizes.length - 1); 

  return parseFloat((bytes / Math.pow(k, index)).toFixed(2)) + ' ' + sizes[index];
}

/**
 * Format milliseconds to appropriate time unit
 */
export function formatTime(ms: number): string {
  if (ms === null || ms === undefined) return 'N/A';
  if (ms < 0) return 'N/A'; // Handle negative times if they occur
  if (ms < 1) return `${(ms * 1000).toFixed(1)}μs`; // More precision for sub-ms
  if (ms < 1000) return `${ms.toFixed(1)}ms`; // More precision for ms
  return `${(ms / 1000).toFixed(2)}s`;
}

/**
 * Format timestamp to locale string
 */
export function formatTimestamp(timestamp: number): string {
  if (!timestamp) return 'Unknown';
  try {
    return new Date(timestamp).toLocaleString();
  } catch (e) {
    console.error("Failed to format timestamp:", timestamp, e);
    return 'Invalid Date';
  }
}

/**
 * Generate a simple unique ID (not cryptographically secure)
 */
export function generateId(): string {
  return Math.random().toString(36).substring(2, 9) + 
         Date.now().toString(36); // Add timestamp for better uniqueness
}