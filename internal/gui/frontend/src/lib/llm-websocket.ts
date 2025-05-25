import { llmStateStore } from './stores';
import { logLLMStateChange, createConnectionMonitor, logWebSocketStats } from './websocket-debug';
import { logger } from './logger';

export interface WSMessage {
    type: 'statechange' | 'initial_state' | 'ping' | 'pong';
    payload: any;
}

export class LLMWebSocket {
    private ws: WebSocket | null = null;
    private reconnectTimer: number | null = null;
    private reconnectDelay = 1000;
    private maxReconnectDelay = 30000;
    private port: number | null = null;
    private isDestroyed = false;
    private connectionMonitor = createConnectionMonitor();

    async connect(): Promise<void> {
        if (this.isDestroyed) return;

        this.connectionMonitor.recordAttempt();

        try {
            // Get port from backend
            if (!this.port) {
                this.port = await window.go.gui.App.GetWebSocketPort();
            }

            const url = `ws://localhost:${this.port}/ws`;
            logger.info('llm-websocket', 'Connecting to WebSocket', { url });
            
            this.ws = new WebSocket(url);

            this.ws.onopen = () => {
                logger.info('llm-websocket', 'WebSocket connected');
                this.reconnectDelay = 1000; // Reset backoff
            };

            this.ws.onmessage = (event) => {
                try {
                    const message: WSMessage = JSON.parse(event.data);
                    logger.debug('llm-websocket', 'Message received', { 
                        type: message.type, 
                        payload: message.payload 
                    });
                    this.handleMessage(message);
                } catch (err) {
                    logger.error('llm-websocket', 'Failed to parse message', { 
                        error: err, 
                        rawData: event.data 
                    });
                }
            };

            this.ws.onerror = (error) => {
                logger.error('llm-websocket', 'WebSocket error', { error });
            };

            this.ws.onclose = () => {
                logger.info('llm-websocket', 'WebSocket disconnected');
                this.ws = null;
                this.scheduleReconnect();
            };

        } catch (err) {
            logger.error('llm-websocket', 'Failed to connect', { error: err });
            this.scheduleReconnect();
        }
    }

    private handleMessage(message: WSMessage): void {
        switch (message.type) {
            case 'statechange':
            case 'initial_state':
                logger.debug('llm-websocket', 'Updating LLM state', { 
                    globalState: message.payload.globalState 
                });
                logLLMStateChange(message.payload);
                llmStateStore.set(message.payload);
                
                // Log detailed provider states
                if (message.payload.providerStatesSnapshot) {
                    Object.entries(message.payload.providerStatesSnapshot).forEach(([provider, state]: [string, any]) => {
                        logger.trace('llm-websocket', 'Provider state', { 
                            provider, 
                            status: state.status 
                        });
                    });
                }
                break;

            case 'ping':
                this.sendPong();
                break;
        }
    }

    private sendPong(): void {
        if (this.ws?.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({ type: 'pong' }));
        }
    }

    private scheduleReconnect(): void {
        if (this.isDestroyed || this.reconnectTimer) return;

        logger.debug('llm-websocket', 'Scheduling reconnect', { delayMs: this.reconnectDelay });
        
        this.reconnectTimer = window.setTimeout(() => {
            this.reconnectTimer = null;
            this.connect();
        }, this.reconnectDelay);

        // Exponential backoff
        this.reconnectDelay = Math.min(
            this.reconnectDelay * 2,
            this.maxReconnectDelay
        );
    }

    disconnect(): void {
        logger.info('llm-websocket', 'Disconnecting WebSocket');
        this.isDestroyed = true;
        
        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
        }
        
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
    }

    isConnected(): boolean {
        return this.ws?.readyState === WebSocket.OPEN;
    }

    getConnectionStats() {
        return this.connectionMonitor.getStats();
    }

    logConnectionStats() {
        const stats = this.getConnectionStats();
        logger.info('llm-websocket', 'Connection statistics', { stats });
        logWebSocketStats(this.ws);
        return stats;
    }
}