package gui

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// WebSocketServer manages WebSocket connections for real-time event broadcasting
type WebSocketServer struct {
	upgrader  websocket.Upgrader
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex
	writeMu   sync.Mutex // Protects WebSocket writes
	port      int
	listener  net.Listener
	logger    zerolog.Logger
	onConnect func() // Callback when a client connects
}

// WSMessage represents a generic WebSocket message
type WSMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp,omitempty"`
	ID        string      `json:"id,omitempty"`
}

// NewWebSocketServer creates a new WebSocket server on a dynamic port
func NewWebSocketServer(logger zerolog.Logger) (*WebSocketServer, error) {
	// Use "localhost:0" for OS to assign available port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	port := listener.Addr().(*net.TCPAddr).Port
	logger.Info().Int("port", port).Msg("WebSocket server listening")

	ws := &WebSocketServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Accept connections from Wails webview
				// In production, you might want to be more restrictive
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		clients:  make(map[*websocket.Conn]bool),
		port:     port,
		listener: listener,
		logger:   logger,
	}

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.handleWebSocket)

	// Start server in background
	go func() {
		if err := http.Serve(listener, mux); err != nil {
			logger.Error().Err(err).Msg("WebSocket server error")
		}
	}()

	return ws, nil
}

// GetPort returns the port the server is listening on
func (ws *WebSocketServer) GetPort() int {
	return ws.port
}

// handleWebSocket handles incoming WebSocket connections
func (ws *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.logger.Error().Err(err).Msg("Failed to upgrade connection")
		return
	}

	// Register client
	ws.clientsMu.Lock()
	ws.clients[conn] = true
	ws.clientsMu.Unlock()

	ws.logger.Info().Msg("WebSocket client connected")

	// Send connection confirmation
	msg := WSMessage{
		Type:      "connected",
		Data:      map[string]interface{}{"message": "WebSocket connection established"},
		Timestamp: time.Now().Unix(),
	}

	ws.writeMu.Lock()
	err = conn.WriteJSON(msg)
	ws.writeMu.Unlock()
	
	if err != nil {
		ws.logger.Error().Err(err).Msg("Failed to send connection message")
	}
	
	// Call connection callback if set
	if ws.onConnect != nil {
		ws.onConnect()
	}

	// Cleanup on disconnect
	defer func() {
		ws.clientsMu.Lock()
		delete(ws.clients, conn)
		ws.clientsMu.Unlock()
		conn.Close()
		ws.logger.Info().Msg("WebSocket client disconnected")
	}()

	// Read pump to detect disconnection
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				ws.logger.Error().Err(err).Msg("WebSocket error")
			}
			break
		}
		// We don't process incoming messages for now
		// Could add ping/pong handling here if needed
	}
}

// Broadcast sends a message of the specified type to all connected clients
func (ws *WebSocketServer) Broadcast(msgType string, data interface{}) {
	msg := WSMessage{
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	ws.clientsMu.RLock()
	clients := make([]*websocket.Conn, 0, len(ws.clients))
	for client := range ws.clients {
		clients = append(clients, client)
	}
	ws.clientsMu.RUnlock()

	// Send without holding lock to avoid blocking
	for _, client := range clients {
		// Protect WebSocket writes with mutex
		ws.writeMu.Lock()
		err := client.WriteJSON(msg)
		ws.writeMu.Unlock()
		
		if err != nil {
			ws.logger.Error().Err(err).Str("msgType", msgType).Msg("Failed to send to client")
			// Client will be removed when read pump detects disconnect
		}
	}
}

// SetOnConnect sets a callback to be called when a client connects
func (ws *WebSocketServer) SetOnConnect(callback func()) {
	ws.onConnect = callback
}

// Shutdown gracefully shuts down the WebSocket server
func (ws *WebSocketServer) Shutdown() error {
	ws.logger.Info().Msg("Shutting down WebSocket server")
	
	// Close all client connections
	ws.clientsMu.Lock()
	for client := range ws.clients {
		client.Close()
	}
	ws.clients = make(map[*websocket.Conn]bool)
	ws.clientsMu.Unlock()

	// Close the listener
	if ws.listener != nil {
		return ws.listener.Close()
	}
	return nil
}