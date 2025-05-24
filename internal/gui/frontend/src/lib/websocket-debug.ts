// Debug utilities for WebSocket implementation
import { logger } from './logger';

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

export function logLLMStateChange(state: any) {
    logger.trace('llm-state', 'State change received:', {
        globalState: state?.globalState,
        timestamp: state?.timestamp,
        providerCount: Object.keys(state?.providerStatesSnapshot || {}).length
    });
    
    // Log provider details in debug mode
    if (state?.providerStatesSnapshot) {
        Object.entries(state.providerStatesSnapshot).forEach(([name, providerState]: [string, any]) => {
            logger.trace('llm-provider', `Provider ${name}:`, {
                status: providerState.status,
                error: providerState.error,
                modelCount: providerState.models?.length || 0
            });
        });
    }
}

// Enhanced debugging utilities
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

export function createConnectionMonitor() {
    let connectionAttempts = 0;
    let successfulConnections = 0;
    let lastConnectionTime: Date | null = null;
    let totalUptime = 0;
    
    return {
        recordAttempt() {
            connectionAttempts++;
            logger.trace('websocket', 'Connection attempt:', { attempt: connectionAttempts });
        },
        
        recordSuccess() {
            successfulConnections++;
            lastConnectionTime = new Date();
            logger.trace('websocket', 'Connection successful:', { 
                successCount: successfulConnections,
                successRate: (successfulConnections / connectionAttempts * 100).toFixed(1) + '%'
            });
        },
        
        recordDisconnect() {
            if (lastConnectionTime) {
                const uptime = Date.now() - lastConnectionTime.getTime();
                totalUptime += uptime;
                logger.trace('websocket', 'Connection lost:', {
                    sessionUptime: uptime + 'ms',
                    totalUptime: totalUptime + 'ms'
                });
            }
        },
        
        getStats() {
            return {
                attempts: connectionAttempts,
                successes: successfulConnections,
                successRate: connectionAttempts > 0 ? (successfulConnections / connectionAttempts) : 0,
                totalUptime,
                isConnected: lastConnectionTime !== null
            };
        }
    };
}

// Test summary generation (for debugging)
export async function testSummaryGeneration() {
    try {
        const testText = "This is a test subtitle content for summary generation testing.";
        const testOptions = {
            provider: "openai",
            model: "gpt-3.5-turbo",
            outputLanguage: "English",
            maxLength: 50,
            temperature: 0.7
        };
        
        logger.trace('summary-test', 'Testing summary generation...');
        
        const result = await window.go.gui.App.GenerateSummary(testText, "English", testOptions);
        
        logger.trace('summary-test', 'Summary generation successful:', {
            inputLength: testText.length,
            outputLength: result.length,
            result: result.substring(0, 100) + (result.length > 100 ? '...' : '')
        });
        
        return result;
    } catch (error) {
        logger.error('summary-test', 'Summary generation failed:', { error });
        throw error;
    }
}