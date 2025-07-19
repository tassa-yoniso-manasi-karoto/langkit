package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// Server represents the WebRPC API server
type Server struct {
	registry *Registry
	router   chi.Router
	server   *http.Server
	listener net.Listener
	port     int
	logger   zerolog.Logger
	mu       sync.Mutex
}

// Config holds server configuration
type Config struct {
	// Host to bind to (default: localhost)
	Host string
	// Port to bind to (0 for dynamic allocation)
	Port int
	// ReadTimeout for HTTP server
	ReadTimeout time.Duration
	// WriteTimeout for HTTP server
	WriteTimeout time.Duration
	// Enable CORS for local development
	EnableCORS bool
}

// DefaultConfig returns default server configuration
func DefaultConfig() *Config {
	return &Config{
		Host:         "localhost",
		Port:         0, // Dynamic port allocation
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		EnableCORS:   true, // For Wails webview
	}
}

// NewServer creates a new WebRPC server
func NewServer(config *Config, logger zerolog.Logger) (*Server, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Create listener for dynamic port allocation
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	logger.Debug().
		Str("host", config.Host).
		Int("port", port).
		Msg("WebRPC server listening")

	// Create chi router
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(loggerMiddleware(logger))

	// CORS middleware for local development
	if config.EnableCORS {
		r.Use(corsMiddleware())
	}

	// Health check endpoint
	r.Get("/health", healthHandler)

	// Service discovery endpoint
	r.Get("/services", func(w http.ResponseWriter, r *http.Request) {
		// This will be implemented after registry is set
	})

	srv := &Server{
		registry: NewRegistry(logger),
		router:   r,
		listener: listener,
		port:     port,
		logger:   logger,
		server: &http.Server{
			Handler:      r,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
		},
	}

	// Update service discovery handler
	r.Get("/services", srv.servicesHandler)

	return srv, nil
}

// GetPort returns the port the server is listening on
func (s *Server) GetPort() int {
	return s.port
}

// Registry returns the service registry
func (s *Server) Registry() *Registry {
	return s.registry
}

// RegisterService adds a service to the server
func (s *Server) RegisterService(service Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.registry.Register(service); err != nil {
		return err
	}

	// Mount the service on the router
	s.registry.Mount(s.router)

	return nil
}

// Start begins serving requests
func (s *Server) Start() error {
	go func() {
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("WebRPC server error")
		}
	}()

	return nil
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown() error {
	s.logger.Debug().Msg("Shutting down WebRPC server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	return nil
}

// Middleware functions
var logBlacklist = []string{"BackendLogger"}

func loggerMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(wrapped, r)
			
			for _, s := range logBlacklist {
				if strings.HasSuffix(r.URL.Path, s) {
					return
				}
			}
			
			logger.Trace().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapped.Status()).
				Dur("duration", time.Since(start)).
				Str("remote", r.RemoteAddr).
				Msg("HTTP request")
		})
	}
}

func corsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow Wails webview origin
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Webrpc")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Handler functions

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) servicesHandler(w http.ResponseWriter, r *http.Request) {
	services := s.registry.List()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"services": services,
		"count":    len(services),
	})
}