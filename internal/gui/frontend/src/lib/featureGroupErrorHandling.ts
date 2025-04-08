// src/lib/featureGroupErrorHandling.ts
import { logStore } from './logStore';
import { errorStore } from './errorStore';

/**
 * Safe wrapper for group option operations with error handling
 * @param operation Function to execute safely
 * @param context Context info for error reporting
 * @param fallback Fallback value if operation fails
 * @returns Result of operation or fallback
 */
export function safeGroupOperation<T>(
  operation: () => T,
  context: { feature?: string; group?: string; option?: string; action: string },
  fallback: T
): T {
  try {
    return operation();
  } catch (error) {
    // Log the error
    logStore.addLog({
      level: 'ERROR',
      message: `Group system error during ${context.action}: ${(error as Error).message}`, // Cast error
      time: new Date().toISOString()
    });
    
    // Create a user-friendly error ID and message
    const errorId = `group-system-${context.action.replace(/\s+/g, '-')}-${Date.now()}`;
    const featureInfo = context.feature ? ` for feature ${context.feature}` : '';
    const groupInfo = context.group ? ` in group ${context.group}` : '';
    const optionInfo = context.option ? ` (option ${context.option})` : '';
    
    const errorMessage = `Error ${context.action}${featureInfo}${groupInfo}${optionInfo}`;
    
    // Add to error store
    errorStore.addError({
      id: errorId,
      message: errorMessage,
      severity: 'warning',
      dismissible: true
    });
    
    // Return fallback value
    return fallback;
  }
}

/**
 * Performance monitoring for group operations
 */
export class GroupPerformanceMonitor {
  private operations: Map<string, { count: number; totalTime: number; maxTime: number }> = new Map();
  
  /**
   * Measure the execution time of an operation
   * @param operationName Name of the operation
   * @param operation Function to measure
   * @returns Result of the operation
   */
  measure<T>(operationName: string, operation: () => T): T {
    const start = performance.now();
    
    try {
      return operation();
    } finally {
      const time = performance.now() - start;
      
      // Record metrics
      if (!this.operations.has(operationName)) {
        this.operations.set(operationName, { count: 0, totalTime: 0, maxTime: 0 });
      }
      
      const stats = this.operations.get(operationName)!;
      stats.count++;
      stats.totalTime += time;
      stats.maxTime = Math.max(stats.maxTime, time);
    }
  }
  
  /**
   * Get performance metrics for all operations
   * @returns Performance metrics
   */
  getMetrics() {
    const result: Record<string, { count: number; avgTime: number; maxTime: number }> = {};
    
    this.operations.forEach((stats, operation) => {
      result[operation] = {
        count: stats.count,
        avgTime: stats.totalTime / stats.count,
        maxTime: stats.maxTime
      };
    });
    
    return result;
  }
  
  /**
   * Reset all metrics
   */
  reset() {
    this.operations.clear();
  }
}

/**
 * Global performance monitor instance
 */
export const groupPerformance = new GroupPerformanceMonitor();

// For development, expose to window
if (import.meta.env.DEV) {
  (window as any).__groupPerformance = groupPerformance;
}