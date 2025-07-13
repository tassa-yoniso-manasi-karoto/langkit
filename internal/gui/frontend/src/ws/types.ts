// WebSocket message types and interfaces

export interface WSMessage {
    type: string;
    data: any;
    timestamp?: number;
    id?: string;
}

export type MessageHandler = (data: any) => void;

export interface ConnectionStats {
    attempts: number;
    successes: number;
    successRate: number;
    totalUptime: number;
    isConnected: boolean;
    lastConnectionTime?: Date;
}

export interface WebSocketClientConfig {
    maxReconnectDelay?: number;
    initialReconnectDelay?: number;
    enableDebug?: boolean;
}