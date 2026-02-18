package gui

import (
	"context"
	"fmt"
	"time"
	
	"github.com/rs/zerolog"
	assetserveroptions "github.com/wailsapp/wails/v2/pkg/options/assetserver"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/services"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/batch"
)

// WebSocketEmitter interface for WebSocket broadcasting
type WebSocketEmitter interface {
	Emit(msgType string, data interface{})
	SetOnConnect(fn func())
	GetPort() int
	Shutdown()
}

// ServerComponents holds the initialized servers and related components
type ServerComponents struct {
	APIServer *api.Server
	WSServer  WebSocketEmitter
	Throttler *batch.AdaptiveEventThrottler
	Handler   *core.GUIHandler
}

// InitializeServers creates and starts the API and WebSocket servers
// This is shared logic used by both Wails and Qt runtimes
func InitializeServers(ctx context.Context, logger zerolog.Logger) (*ServerComponents, error) {
	// Create WebRPC API server without listener (router only)
	apiServer := api.NewServerWithoutListener(api.DefaultConfig(), logger)
	
	// Create runtime config for unified server
	// Both Wails and server modes will use backend-only mode here
	// Frontend is handled separately in each mode
	runtimeConfig := RuntimeConfig{
		Runtime: "wails", // Default to wails, will be overridden in server mode
	}
	
	// Create unified server configuration
	unifiedConfig := UnifiedServerConfig{
		RuntimeConfig: runtimeConfig,
		AssetOptions:  assetserveroptions.Options{}, // Empty - backend only
		Logger:        logger,
		APIServer:     apiServer,
		BackendOnly:   true, // Only serve API and WebSocket
		OnWSConnect:   nil,  // Will be set later if needed
	}
	
	// Create unified server
	unifiedServer, err := NewUnifiedServer(unifiedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create unified server: %w", err)
	}
	
	// Start unified server
	if err := unifiedServer.Start(); err != nil {
		return nil, fmt.Errorf("failed to start unified server: %w", err)
	}
	logger.Info().Int("port", unifiedServer.GetPort()).Msg("Unified server started (API + WebSocket)")
	
	// Create broadcaster function for throttler
	broadcaster := func(msgType string, data interface{}) {
		unifiedServer.Emit(msgType, data)
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
	
	// Initialize handler with throttler and unified server
	handler := core.NewGUIHandler(ctx, throttler, unifiedServer)
	
	// Register all services with API server
	if err := registerServices(apiServer, logger, unifiedServer, throttler, handler, ctx); err != nil {
		unifiedServer.Shutdown()
		return nil, fmt.Errorf("failed to register services: %w", err)
	}
	
	return &ServerComponents{
		APIServer: apiServer,
		WSServer:  unifiedServer,
		Throttler: throttler,
		Handler:   handler,
	}, nil
}

// registerServices registers all WebRPC services with the API server
func registerServices(
	apiServer *api.Server,
	logger zerolog.Logger,
	wsServer WebSocketEmitter,
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
	
	// Register processing service (handler implements ProcessingProvider)
	processingSvc := services.NewProcessingService(logger, handler, wsServer)
	if err := apiServer.RegisterService(processingSvc); err != nil {
		return fmt.Errorf("failed to register processing service: %w", err)
	}
	
	// Register expectation service
	expectationSvc := services.NewExpectationService(logger)
	if err := apiServer.RegisterService(expectationSvc); err != nil {
		return fmt.Errorf("failed to register expectation service: %w", err)
	}

	// Note: Settings service is not registered here because it needs
	// a reference to the App struct which is Wails-specific

	return nil
}