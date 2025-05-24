import { llmStateStore } from './stores';
import { logLLMStateChange, createConnectionMonitor, logWebSocketStats } from './websocket-debug';

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
            console.log(`[LLMWebSocket] Connecting to ${url}`);
            
            this.ws = new WebSocket(url);

            this.ws.onopen = () => {
                console.log('[LLMWebSocket] Connected');
                this.reconnectDelay = 1000; // Reset backoff
            };

            this.ws.onmessage = (event) => {
                try {
                    const message: WSMessage = JSON.parse(event.data);
                    console.log('[LLMWebSocket] Message received:', message.type);
                    console.log('[LLMWebSocket] Full message:', message);
                    this.handleMessage(message);
                } catch (err) {
                    console.error('[LLMWebSocket] Failed to parse message:', err);
                    console.error('[LLMWebSocket] Raw data:', event.data);
                }
            };

            this.ws.onerror = (error) => {
                console.error('[LLMWebSocket] Error:', error);
            };

            this.ws.onclose = () => {
                console.log('[LLMWebSocket] Disconnected');
                this.ws = null;
                this.scheduleReconnect();
            };

        } catch (err) {
            console.error('[LLMWebSocket] Failed to connect:', err);
            this.scheduleReconnect();
        }
    }

    private handleMessage(message: WSMessage): void {
        switch (message.type) {
            case 'statechange':
            case 'initial_state':
                console.log('[LLMWebSocket] Updating LLM state:', message.payload.globalState);
                logLLMStateChange(message.payload);
                llmStateStore.set(message.payload);
                
                // Log detailed provider states
                if (message.payload.providerStatesSnapshot) {
                    Object.entries(message.payload.providerStatesSnapshot).forEach(([provider, state]: [string, any]) => {
                        console.log(`[LLMWebSocket] Provider ${provider}:`, state.status);
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

        console.log(`[LLMWebSocket] Scheduling reconnect in ${this.reconnectDelay}ms`);
        
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
        console.log('[LLMWebSocket] Disconnecting');
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
        console.log('[LLMWebSocket] Connection Statistics:', stats);
        logWebSocketStats(this.ws);
        return stats;
    }
}