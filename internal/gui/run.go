package gui

import (
	"context"
	"fmt"
	"time"
	
	"github.com/rs/zerolog"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/services"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/batch"
)

// ServerComponents holds the initialized servers and related components
type ServerComponents struct {
	APIServer *api.Server
	WSServer  *WebSocketServer
	Throttler *batch.AdaptiveEventThrottler
	Handler   *core.GUIHandler
}

// InitializeServers creates and starts the API and WebSocket servers
// This is shared logic used by both Wails and Qt runtimes
func InitializeServers(ctx context.Context, logger zerolog.Logger) (*ServerComponents, error) {
	// Create WebSocket server first (needed by throttler)
	// Note: WebSocket server starts automatically in NewWebSocketServer
	wsServer, err := NewWebSocketServer(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebSocket server: %w", err)
	}
	logger.Info().Int("port", wsServer.GetPort()).Msg("WebSocket server started")
	
	// Create broadcaster function for throttler
	broadcaster := func(msgType string, data interface{}) {
		wsServer.Emit(msgType, data)
	}
	
	// Initialize the throttler with WebSocket broadcaster
	throttler := batch.NewAdaptiveEventThrottler(
		ctx,
		0,                    // minInterval - will be updated from settings
		250*time.Millisecond, // maxInterval - will be updated from settings
		500*time.Millisecond, // rateWindow for measuring event frequency
		true,                 // enabled by default
		&logger,              // Logger for throttler
		broadcaster,          // WebSocket broadcaster
	)
	
	// Initialize handler with throttler and WebSocket server
	handler := core.NewGUIHandler(ctx, throttler, wsServer)
	
	// Create WebRPC API server
	apiServer, err := api.NewServer(api.DefaultConfig(), logger)
	if err != nil {
		wsServer.Shutdown()
		return nil, fmt.Errorf("failed to create API server: %w", err)
	}
	
	// Register all services
	if err := registerServices(apiServer, logger, wsServer, throttler, handler, ctx); err != nil {
		wsServer.Shutdown()
		return nil, fmt.Errorf("failed to register services: %w", err)
	}
	
	// Start API server
	if err := apiServer.Start(); err != nil {
		wsServer.Shutdown()
		return nil, fmt.Errorf("failed to start API server: %w", err)
	}
	logger.Info().Int("port", apiServer.GetPort()).Msg("WebRPC API server started")
	
	return &ServerComponents{
		APIServer: apiServer,
		WSServer:  wsServer,
		Throttler: throttler,
		Handler:   handler,
	}, nil
}

// registerServices registers all WebRPC services with the API server
func registerServices(
	apiServer *api.Server,
	logger zerolog.Logger,
	wsServer *WebSocketServer,
	throttler *batch.AdaptiveEventThrottler,
	handler *core.GUIHandler,
	ctx context.Context,
) error {
	// Register language service
	langSvc := services.NewLanguageService(logger)
	if err := apiServer.RegisterService(langSvc); err != nil {
		return fmt.Errorf("failed to register language service: %w", err)
	}
	
	// Register dependency service
	depsSvc := services.NewDependencyService(logger, wsServer)
	if err := apiServer.RegisterService(depsSvc); err != nil {
		return fmt.Errorf("failed to register dependency service: %w", err)
	}
	
	// Register dry run service (handler implements DryRunProvider)
	dryRunSvc := services.NewDryRunService(logger, handler)
	if err := apiServer.RegisterService(dryRunSvc); err != nil {
		return fmt.Errorf("failed to register dry run service: %w", err)
	}
	
	// Register logging service (handler implements LoggingProvider)
	loggingSvc := services.NewLoggingService(logger, handler, wsServer, throttler, handler, ctx)
	if err := apiServer.RegisterService(loggingSvc); err != nil {
		return fmt.Errorf("failed to register logging service: %w", err)
	}
	
	// Register system service
	systemSvc := services.NewSystemService(logger)
	if err := apiServer.RegisterService(systemSvc); err != nil {
		return fmt.Errorf("failed to register system service: %w", err)
	}
	
	// Register model service (handler implements STTModelProvider and LLMRegistryProvider)
	modelSvc := services.NewModelService(logger, handler, handler)
	if err := apiServer.RegisterService(modelSvc); err != nil {
		return fmt.Errorf("failed to register model service: %w", err)
	}
	
	// Register media service (handler implements MediaProvider)
	mediaSvc := services.NewMediaService(logger, handler)
	if err := apiServer.RegisterService(mediaSvc); err != nil {
		return fmt.Errorf("failed to register media service: %w", err)
	}
	
	// Note: Settings service is not registered here because it needs
	// a reference to the App struct which is Wails-specific
	
	return nil
}