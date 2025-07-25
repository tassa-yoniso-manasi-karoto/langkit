package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/assetserver"
	assetserveroptions "github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/interfaces"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/services"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/batch"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/ui"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/ui/browser"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/ui/dialogs"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

// RunServerMode runs Langkit in headless server mode for Qt/Anki integration
func RunServerMode() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Server mode panic: %v\n", r)
			os.Exit(1)
		}
	}()

	// Setup logger
	writer := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.TimeOnly,
	}
	logger := zerolog.New(writer).With().Timestamp().Str("module", "server").Logger()
	logger.Info().Msg("Starting Langkit in server mode...")

	// Check for optional Anki addon config path
	var ankiConfigPath string
	if len(os.Args) > 2 {
		ankiConfigPath = os.Args[2]
		logger.Info().Str("config_path", ankiConfigPath).Msg("Anki addon config path provided")
	}

	// Initialize UI manager with Zenity dialogs for native file operations and URL opening
	ui.Initialize(dialogs.NewZenityFileDialog(), browser.NewZenityURLOpener())
	logger.Info().Msg("UI manager initialized with Zenity dialogs and URL opener")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize servers (WebSocket and API)
	servers, err := InitializeServers(ctx, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize servers")
	}

	// Create frontend HTTP server with dynamic port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create frontend listener")
	}
	frontendPort := listener.Addr().(*net.TCPAddr).Port

	// Create runtime config for DOM injection
	config := RuntimeConfig{
		APIPort:      servers.APIServer.GetPort(),
		WSPort:       servers.WSServer.GetPort(),
		FrontendPort: frontendPort,
		Mode:         "qt",
		Runtime:      "anki",
	}

	// Create Wails AssetHandler for serving embedded frontend
	assetOptions := assetserveroptions.Options{
		Assets: assets,
	}
	assetHandler, err := assetserver.NewAssetHandler(assetOptions, &simpleLogger{logger})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create asset handler")
	}

	// Setup Chi router with middleware
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Apply config injection middleware to index routes
	configMiddleware := NewConfigInjectionMiddleware(config)
	r.Get("/", configMiddleware(assetHandler).ServeHTTP)
	r.Get("/index.html", configMiddleware(assetHandler).ServeHTTP)
	
	// All other assets served directly by AssetHandler
	r.Handle("/*", assetHandler)

	// Initialize additional components (similar to domReady in wails_app.go)
	initializeServerComponents(ctx, servers, &logger)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Update Anki config if path was provided
	if ankiConfigPath != "" {
		if err := updateAnkiConfig(ankiConfigPath, frontendPort, config.APIPort, config.WSPort, &logger); err != nil {
			logger.Error().Err(err).Msg("Failed to update Anki config")
			// Continue anyway - the addon can still discover ports through logs
		}
	}

	// Start frontend server in goroutine
	go func() {
		logger.Info().
			Int("frontend_port", frontendPort).
			Int("api_port", config.APIPort).
			Int("ws_port", config.WSPort).
			Msg("Server mode ready")
		
		logger.Warn().Msgf("\n\n\t\t\t🡆 🡆 🡆 FRONTEND: http://localhost:%d 🡄 🡄 🡄\n\n", frontendPort)
		
		if err := http.Serve(listener, r); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Frontend server failed")
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	logger.Info().Msg("Shutting down server mode...")

	// Cleanup
	if servers.Throttler != nil {
		servers.Throttler.SyncFlush()
	}
	if servers.APIServer != nil {
		// API server doesn't have a Stop method, but it will be cleaned up on process exit
	}
	if servers.WSServer != nil {
		servers.WSServer.Shutdown()
	}

	logger.Info().Msg("Server mode shutdown complete")
}

// initializeServerComponents initializes LLM, settings, and other components
func initializeServerComponents(ctx context.Context, servers *ServerComponents, logger *zerolog.Logger) {
	handler := servers.Handler

	// Load settings
	settings, err := config.LoadSettings()
	if err != nil {
		// Use empty settings as default if loading fails
		settings = config.Settings{}
		logger.Warn().Err(err).Msg("Failed to load settings, using defaults")
	}

	// Update throttler settings from config
	UpdateThrottlerWithSettings(servers.Throttler, settings, *logger)

	// Emit settings to frontend
	if servers.WSServer != nil {
		servers.WSServer.Emit("settings.loaded", settings)
	}

	// Initialize LLM system with async registry and WebSocket server
	llmRegistry := core.InitLLM(handler, ctx, servers.WSServer)
	logger.Info().Msg("LLM registry initialized")

	// Set the LLM registry in the handler so it can be accessed by services
	handler.SetLLMRegistry(llmRegistry)

	// Register settings service (needs throttler and llm registry references)
	settingsProvider := &serverSettingsProvider{
		throttler:   servers.Throttler,
		llmRegistry: llmRegistry,
		logger:      *logger,
	}
	settingsSvc := services.NewSettingsService(*logger, settingsProvider)
	if err := servers.APIServer.RegisterService(settingsSvc); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register settings service")
	}
	logger.Info().Msg("Settings service registered")

	// Set up WebSocket connection callback to send initial LLM state
	servers.WSServer.SetOnConnect(func() {
		if llmRegistry != nil {
			stateSnapshot := llmRegistry.GetCurrentStateSnapshot()
			servers.WSServer.Emit("llm.state.changed", stateSnapshot)
			logger.Debug().Msg("Sent initial LLM state to new WebSocket client")
		}
	})

	logger.Info().Msg("Server component initialization complete")
}

var _ interfaces.SettingsProvider = (*serverSettingsProvider)(nil)

// serverSettingsProvider implements interfaces.SettingsProvider for server mode
type serverSettingsProvider struct {
	throttler   *batch.AdaptiveEventThrottler
	llmRegistry *llms.Registry
	logger      zerolog.Logger
}

// UpdateThrottlerSettings implements interfaces.SettingsProvider
func (s *serverSettingsProvider) UpdateThrottlerSettings(settings interface{}) {
	// Type assert to config.Settings
	if configSettings, ok := settings.(config.Settings); ok {
		UpdateThrottlerWithSettings(s.throttler, configSettings, s.logger)
	} else {
		s.logger.Error().Msg("UpdateThrottlerSettings: invalid settings type")
	}
}

// TriggerLLMRegistryUpdate implements interfaces.SettingsProvider
func (s *serverSettingsProvider) TriggerLLMRegistryUpdate(settings interface{}) {
	// Type assert to config.Settings
	if configSettings, ok := settings.(config.Settings); ok {
		if s.llmRegistry != nil {
			s.logger.Info().Msg("Triggering LLM registry update with new settings")
			s.llmRegistry.TriggerUpdate(configSettings)
		}
	} else {
		s.logger.Error().Msg("TriggerLLMRegistryUpdate: invalid settings type")
	}
}

// simpleLogger adapts zerolog.Logger to assetserver.Logger interface
type simpleLogger struct {
	logger zerolog.Logger
}

func (l *simpleLogger) Debug(message string, args ...interface{}) {
	l.logger.Debug().Msgf(message, args...)
}

func (l *simpleLogger) Error(message string, args ...interface{}) {
	l.logger.Error().Msgf(message, args...)
}

// updateAnkiConfig updates the Anki addon's config.json with server port information
func updateAnkiConfig(configPath string, frontendPort, apiPort, wsPort int, logger *zerolog.Logger) error {
	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON into a map to preserve existing keys
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Add langkit server information
	config["langkit_server"] = map[string]interface{}{
		"frontend_port": frontendPort,
		"api_port":      apiPort,
		"ws_port":       wsPort,
		"updated_at":    time.Now().Unix(),
	}

	// Marshal back to JSON with indentation
	updatedData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config JSON: %w", err)
	}

	// Write back to file
	if err := os.WriteFile(configPath, updatedData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	logger.Info().
		Str("path", configPath).
		Int("frontend_port", frontendPort).
		Int("api_port", apiPort).
		Int("ws_port", wsPort).
		Msg("Updated Anki addon config with server ports")

	return nil
}