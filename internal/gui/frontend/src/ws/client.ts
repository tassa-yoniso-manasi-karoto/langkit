import { logger } from '../lib/logger';
import type { WSMessage, MessageHandler, ConnectionStats, WebSocketClientConfig } from './types';

export class WebSocketClient {
    private ws: WebSocket | null = null;
    private reconnectTimer: number | null = null;
    private reconnectDelay = 1000;
    private maxReconnectDelay = 30000;
    private port: number | null = null;
    private isDestroyed = false;
    
    // Event handlers map
    private handlers: Map<string, Set<MessageHandler>> = new Map();
    
    // Connection stats
    private connectionAttempts = 0;
    private successfulConnections = 0;
    private lastConnectionTime: Date | null = null;
    private totalUptime = 0;
    
    // Configuration
    private config: WebSocketClientConfig;
    
    constructor(config: WebSocketClientConfig = {}) {
        this.config = {
            maxReconnectDelay: config.maxReconnectDelay || 30000,
            initialReconnectDelay: config.initialReconnectDelay || 1000,
            enableDebug: config.enableDebug || false
        };
        this.reconnectDelay = this.config.initialReconnectDelay!;
    }
    
    /**
     * Truncate data for logging to prevent excessively long log entries
     * @param data Any data to truncate
     * @param maxLength Maximum string length (default: 150 chars)
     * @returns Truncated string representation
     */
    private truncateData(data: any, maxLength: number = 150): string {
        let str: string;
        
        if (typeof data === 'string') {
            str = data;
        } else if (data === null || data === undefined) {
            return String(data);
        } else {
            try {
                str = JSON.stringify(data);
            } catch {
                str = String(data);
            }
        }
        
        if (str.length <= maxLength) {
            return str;
        }
        
        // Show beginning and end with ellipsis in middle
        const startLength = Math.floor((maxLength - 5) / 2);
        const endLength = maxLength - startLength - 5;
        return str.substring(0, startLength) + ' ... ' + str.substring(str.length - endLength);
    }
    
    async connect(port?: number): Promise<void> {
        if (this.isDestroyed) return;
        
        this.connectionAttempts++;
        
        try {
            // Get port from backend if not provided
            if (!port && !this.port) {
                this.port = await window.go.gui.App.GetWebSocketPort();
            } else if (port) {
                this.port = port;
            }
            
            const url = `ws://localhost:${this.port}/ws`;
            logger.info('websocket', 'Connecting to WebSocket', { url });
            
            this.ws = new WebSocket(url);
            
            this.ws.onopen = () => {
                logger.info('websocket', 'WebSocket connected');
                this.successfulConnections++;
                this.lastConnectionTime = new Date();
                this.reconnectDelay = this.config.initialReconnectDelay!;
                
                // Emit connected event
                this.emit('connected', { timestamp: Date.now() });
            };
            
            this.ws.onmessage = (event) => {
                try {
                    const message: WSMessage = JSON.parse(event.data);
                    
                    if (this.config.enableDebug) {
                        logger.debug('websocket', 'Message received', { 
                            type: message.type, 
                            dataPreview: this.truncateData(message.data)
                        });
                    }
                    
                    // Emit message to handlers
                    this.emit(message.type, message.data);
                    
                } catch (err) {
                    logger.error('websocket', 'Failed to parse message', { 
                        error: err, 
                        rawData: this.truncateData(event.data) 
                    });
                }
            };
            
            this.ws.onerror = (error) => {
                logger.error('websocket', 'WebSocket error', { error });
            };
            
            this.ws.onclose = () => {
                logger.info('websocket', 'WebSocket disconnected');
                
                // Update connection stats
                if (this.lastConnectionTime) {
                    const uptime = Date.now() - this.lastConnectionTime.getTime();
                    this.totalUptime += uptime;
                    this.lastConnectionTime = null;
                }
                
                // Emit disconnected event
                this.emit('disconnected', { timestamp: Date.now() });
                
                this.ws = null;
                this.scheduleReconnect();
            };
            
        } catch (err) {
            logger.error('websocket', 'Failed to connect', { error: err });
            this.scheduleReconnect();
        }
    }
    
    private scheduleReconnect(): void {
        if (this.isDestroyed || this.reconnectTimer) return;
        
        logger.debug('websocket', 'Scheduling reconnect', { delayMs: this.reconnectDelay });
        
        this.reconnectTimer = window.setTimeout(() => {
            this.reconnectTimer = null;
            this.connect();
        }, this.reconnectDelay);
        
        // Exponential backoff
        this.reconnectDelay = Math.min(
            this.reconnectDelay * 2,
            this.config.maxReconnectDelay!
        );
    }
    
    disconnect(): void {
        logger.info('websocket', 'Disconnecting WebSocket');
        this.isDestroyed = true;
        
        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
        }
        
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        
        // Clear all handlers
        this.handlers.clear();
    }
    
    // Event handling methods
    on(eventType: string, handler: MessageHandler): void {
        if (!this.handlers.has(eventType)) {
            this.handlers.set(eventType, new Set());
        }
        this.handlers.get(eventType)!.add(handler);
        
        if (this.config.enableDebug) {
            logger.trace('websocket', 'Handler registered', { 
                eventType, 
                handlerCount: this.handlers.get(eventType)!.size 
            });
        }
    }
    
    off(eventType: string, handler: MessageHandler): void {
        const handlers = this.handlers.get(eventType);
        if (handlers) {
            handlers.delete(handler);
            if (handlers.size === 0) {
                this.handlers.delete(eventType);
            }
        }
    }
    
    private emit(eventType: string, data: any): void {
        const handlers = this.handlers.get(eventType);
        if (handlers) {
            handlers.forEach(handler => {
                try {
                    handler(data);
                } catch (err) {
                    logger.error('websocket', 'Handler error', { 
                        eventType, 
                        error: err 
                    });
                }
            });
        }
    }
    
    // Connection status methods
    isConnected(): boolean {
        return this.ws?.readyState === WebSocket.OPEN;
    }
    
    getConnectionStats(): ConnectionStats {
        return {
            attempts: this.connectionAttempts,
            successes: this.successfulConnections,
            successRate: this.connectionAttempts > 0 
                ? this.successfulConnections / this.connectionAttempts 
                : 0,
            totalUptime: this.totalUptime,
            isConnected: this.isConnected(),
            lastConnectionTime: this.lastConnectionTime || undefined
        };
    }
    
    // Send message (for future bidirectional communication)
    send(message: any): void {
        if (this.ws?.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(message));
        } else {
            logger.warn('websocket', 'Cannot send message, WebSocket not connected');
        }
    }
}

// Export a singleton instance for convenience
export const wsClient = new WebSocketClient();