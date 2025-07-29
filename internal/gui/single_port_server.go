package gui

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/assetserver"
	assetserveroptions "github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
)

// UnifiedServer provides a single-port server for all Langkit services
type UnifiedServer struct {
	echo        *echo.Echo
	listener    net.Listener
	port        int
	logger      zerolog.Logger
	
	// Components
	wsManager   *WebSocketManager
	apiServer   *api.Server
	assetHandler http.Handler
	
	// Configuration
	config      RuntimeConfig
}

// WSMessage represents a generic WebSocket message
type WSMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp,omitempty"`
	ID        string      `json:"id,omitempty"`
}

// WebSocketManager handles WebSocket connections and broadcasting
type WebSocketManager struct {
	upgrader  websocket.Upgrader
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex
	writeMu   sync.Mutex
	logger    zerolog.Logger
	onConnect func()
}

// UnifiedServerConfig holds configuration for the unified server
type UnifiedServerConfig struct {
	RuntimeConfig RuntimeConfig
	AssetOptions  assetserveroptions.Options
	Logger        zerolog.Logger
	APIServer     *api.Server
	OnWSConnect   func()
	BackendOnly   bool // If true, only serve API and WebSocket, not frontend assets
}

// NewUnifiedServer creates a new single-port server for all services
func NewUnifiedServer(config UnifiedServerConfig) (*UnifiedServer, error) {
	// Create listener for dynamic port allocation
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}
	
	port := listener.Addr().(*net.TCPAddr).Port
	config.Logger.Info().Int("port", port).Msg("Unified server listening")
	
	// Update runtime config with the single port
	config.RuntimeConfig.APIPort = port
	config.RuntimeConfig.WSPort = port
	if !config.BackendOnly {
		config.RuntimeConfig.FrontendPort = port
	}
	
	// Create asset handler for frontend files (only if not backend-only)
	var assetHandler http.Handler
	if !config.BackendOnly {
		assetHandler, err = assetserver.NewAssetHandler(config.AssetOptions, &simpleEchoLogger{config.Logger})
		if err != nil {
			listener.Close()
			return nil, fmt.Errorf("failed to create asset handler: %w", err)
		}
	}
	
	// Create WebSocket manager
	wsManager := &WebSocketManager{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Accept connections from same origin and Wails webview
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		clients:   make(map[*websocket.Conn]bool),
		logger:    config.Logger,
		onConnect: config.OnWSConnect,
	}
	
	// Create Echo instance
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	
	// Configure Echo logger
	e.Logger.SetOutput(&echoLogAdapter{logger: config.Logger})
	
	// Setup middleware
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(echoLoggerMiddleware(config.Logger))
	e.Use(middleware.CORS())
	
	server := &UnifiedServer{
		echo:         e,
		listener:     listener,
		port:         port,
		logger:       config.Logger,
		wsManager:    wsManager,
		apiServer:    config.APIServer,
		assetHandler: assetHandler,
		config:       config.RuntimeConfig,
	}
	
	// Setup routes
	server.setupRoutes()
	
	return server, nil
}

// setupRoutes configures all routes for the unified server
func (s *UnifiedServer) setupRoutes() {
	// Health check endpoint
	s.echo.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":       "healthy",
			"port":         s.port,
			"runtime":      s.config.Runtime,
			"backend_only": s.assetHandler == nil,
		})
	})
	
	// WebSocket endpoint
	s.echo.GET("/ws", s.handleWebSocket)
	
	// Mount Chi router for API routes
	// Create a wrapper that strips the /api prefix before passing to Chi
	apiHandler := http.StripPrefix("/api", s.apiServer.Router())
	s.echo.Any("/api/*", echo.WrapHandler(apiHandler))
	
	// Frontend routes (only if not backend-only)
	if s.assetHandler != nil {
		// Frontend routes with config injection
		s.echo.GET("/", s.handleIndexWithConfig)
		s.echo.GET("/index.html", s.handleIndexWithConfig)
		
		// All other static assets
		s.echo.GET("/*", echo.WrapHandler(s.assetHandler))
	}
}

// handleWebSocket handles WebSocket upgrade and connection management
func (s *UnifiedServer) handleWebSocket(c echo.Context) error {
	ws, err := s.wsManager.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return err
	}
	
	// Register client
	s.wsManager.clientsMu.Lock()
	s.wsManager.clients[ws] = true
	s.wsManager.clientsMu.Unlock()
	
	s.logger.Info().Msg("WebSocket client connected")
	
	// Send connection confirmation
	msg := WSMessage{
		Type:      "connected",
		Data:      map[string]interface{}{"message": "WebSocket connection established"},
		Timestamp: time.Now().Unix(),
	}
	
	s.wsManager.writeMu.Lock()
	err = ws.WriteJSON(msg)
	s.wsManager.writeMu.Unlock()
	
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to send connection message")
	}
	
	// Call onConnect callback if set
	if s.wsManager.onConnect != nil {
		s.wsManager.onConnect()
	}
	
	// Handle the connection
	defer func() {
		s.wsManager.clientsMu.Lock()
		delete(s.wsManager.clients, ws)
		s.wsManager.clientsMu.Unlock()
		ws.Close()
		s.logger.Info().Msg("WebSocket client disconnected")
	}()
	
	// Read messages from client
	for {
		var msg map[string]interface{}
		err := ws.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Error().Err(err).Msg("WebSocket read error")
			}
			break
		}
		
		// Handle ping messages
		if msgType, ok := msg["type"].(string); ok && msgType == "ping" {
			pong := WSMessage{
				Type:      "pong",
				Data:      map[string]interface{}{"timestamp": time.Now().Unix()},
				Timestamp: time.Now().Unix(),
			}
			
			s.wsManager.writeMu.Lock()
			err = ws.WriteJSON(pong)
			s.wsManager.writeMu.Unlock()
			
			if err != nil {
				s.logger.Error().Err(err).Msg("Failed to send pong")
				break
			}
		}
	}
	
	return nil
}


// handleIndexWithConfig serves index.html with injected configuration
func (s *UnifiedServer) handleIndexWithConfig(c echo.Context) error {
	// Use the config injection middleware approach
	handler := NewEchoConfigInjectionMiddleware(s.config)(s.assetHandler)
	handler.ServeHTTP(c.Response().Writer, c.Request())
	return nil
}

// Start starts the unified server
func (s *UnifiedServer) Start() error {
	s.logger.Info().
		Int("port", s.port).
		Str("runtime", s.config.Runtime).
		Msg("Starting unified server")
	
	// Start Echo server with existing listener
	s.echo.Listener = s.listener
	go func() {
		if err := s.echo.Start(""); err != nil && err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("Unified server error")
		}
	}()
	
	return nil
}

// Shutdown gracefully stops the unified server
func (s *UnifiedServer) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := s.echo.Shutdown(ctx); err != nil {
		s.logger.Error().Err(err).Msg("Error shutting down unified server")
	}
}

// GetPort returns the port the server is listening on
func (s *UnifiedServer) GetPort() int {
	return s.port
}

// Emit sends a message to all connected WebSocket clients
func (s *UnifiedServer) Emit(msgType string, data interface{}) {
	msg := WSMessage{
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
	
	s.wsManager.clientsMu.RLock()
	clients := make([]*websocket.Conn, 0, len(s.wsManager.clients))
	for client := range s.wsManager.clients {
		clients = append(clients, client)
	}
	s.wsManager.clientsMu.RUnlock()
	
	for _, client := range clients {
		s.wsManager.writeMu.Lock()
		err := client.WriteJSON(msg)
		s.wsManager.writeMu.Unlock()
		
		if err != nil {
			s.logger.Error().Err(err).Msg("Failed to write to WebSocket client")
			// Remove failed client
			s.wsManager.clientsMu.Lock()
			delete(s.wsManager.clients, client)
			s.wsManager.clientsMu.Unlock()
			client.Close()
		}
	}
}

// SetOnConnect sets the callback for new WebSocket connections
func (s *UnifiedServer) SetOnConnect(fn func()) {
	s.wsManager.onConnect = fn
}


// echoLogAdapter adapts zerolog to Echo's logger interface
type echoLogAdapter struct {
	logger zerolog.Logger
}

func (l *echoLogAdapter) Write(p []byte) (n int, err error) {
	l.logger.Debug().Msg(string(p))
	return len(p), nil
}

// simpleEchoLogger adapts zerolog for asset server logging
type simpleEchoLogger struct {
	logger zerolog.Logger
}

func (l *simpleEchoLogger) Debug(message string, args ...interface{}) {
	l.logger.Debug().Msgf(message, args...)
}

func (l *simpleEchoLogger) Error(message string, args ...interface{}) {
	l.logger.Error().Msgf(message, args...)
}

// echoLoggerMiddleware creates Echo logging middleware
func echoLoggerMiddleware(logger zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			
			err := next(c)
			
			req := c.Request()
			res := c.Response()
			
			logger.Info().
				Str("method", req.Method).
				Str("path", req.URL.Path).
				Int("status", res.Status).
				Dur("latency", time.Since(start)).
				Str("ip", c.RealIP()).
				Msg("HTTP request")
			
			return err
		}
	}
}