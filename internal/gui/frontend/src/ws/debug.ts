// Debug utilities for WebSocket implementation
import { logger } from '../lib/logger';

export function debugWebSocketConnection(port: number) {
    logger.trace('websocket', `WebSocket server should be running on port ${port}`);
    
    // Test basic WebSocket connectivity
    const testSocket = new WebSocket(`ws://localhost:${port}/ws`);
    
    testSocket.onopen = () => {
        logger.trace('websocket', 'Debug connection successful');
        testSocket.close();
    };
    
    testSocket.onerror = (error) => {
        logger.error('websocket', 'Debug connection failed', { error });
        testSocket.close();
    };
    
    testSocket.onclose = () => {
        logger.trace('websocket', 'Debug connection closed');
    };
}

// Generic WebSocket status logging
export function logWebSocketStats(ws: WebSocket | null) {
    if (!ws) {
        logger.trace('websocket', 'WebSocket is null');
        return;
    }
    
    const readyStates = {
        0: 'CONNECTING',
        1: 'OPEN', 
        2: 'CLOSING',
        3: 'CLOSED'
    };
    
    logger.trace('websocket', 'WebSocket status:', {
        readyState: readyStates[ws.readyState] || 'UNKNOWN',
        url: ws.url,
        protocol: ws.protocol,
        extensions: ws.extensions
    });
}

// Log WebSocket message for debugging
export function logMessage(type: string, data: any, direction: 'in' | 'out' = 'in') {
    const maxDataPreview = 200;
    let dataPreview = '';
    
    try {
        const dataStr = JSON.stringify(data);
        dataPreview = dataStr.length > maxDataPreview 
            ? dataStr.substring(0, maxDataPreview) + '...' 
            : dataStr;
    } catch (e) {
        dataPreview = '[Unable to stringify data]';
    }
    
    logger.trace('websocket', `Message ${direction}`, {
        type,
        dataKeys: data && typeof data === 'object' ? Object.keys(data) : [],
        dataPreview,
        timestamp: new Date().toISOString()
    });
}