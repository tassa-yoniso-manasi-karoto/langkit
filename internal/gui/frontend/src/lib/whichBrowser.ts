import { logger } from './logger';
import { writable } from 'svelte/store';

/**
 * Detect browser engine type
 * Returns 'webkit' for Safari/WebKit browsers, 'chromium' for Chrome/Edge/etc, or 'unknown'
 */
export function detectBrowserEngine(): 'webkit' | 'chromium' | 'unknown' {
  try {
    // Check for Chromium/Chrome/Edge
    if (window.chrome && window.chrome.runtime) {
      return 'chromium';
    }
    
    // Check for WebKit/Safari using feature detection
    // Safari has window.webkit, while CSS.supports is more reliable
    if (window.webkit || 
        (CSS && CSS.supports && CSS.supports('-webkit-appearance', 'none') && !window.chrome)) {
      return 'webkit';
    }
    
    // Additional WebKit check using user agent as fallback
    const ua = navigator.userAgent.toLowerCase();
    if (ua.includes('webkit') && !ua.includes('chrome') && !ua.includes('chromium')) {
      return 'webkit';
    }
    
    // If we detect chrome/chromium in user agent but didn't catch it above
    if (ua.includes('chrome') || ua.includes('chromium')) {
      return 'chromium';
    }
    
    return 'unknown';
  } catch (error) {
    logger.error('utils', 'Failed to detect browser engine', { error });
    return 'unknown';
  }
}

// Create a store for browser engine detection
export const browserEngine = writable<'webkit' | 'chromium' | 'unknown'>('unknown');

// Initialize browser engine detection
if (typeof window !== 'undefined') {
  browserEngine.set(detectBrowserEngine());
}