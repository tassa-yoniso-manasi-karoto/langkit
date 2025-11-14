package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	assetserveroptions "github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
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

	// Check for optional Anki addon config path and determine runtime
	var ankiConfigPath string
	var runtime string
	var dialogPort int
	if len(os.Args) > 2 && os.Args[2] != "" {
		ankiConfigPath = os.Args[2]
		runtime = "anki"
		logger.Info().Str("config_path", ankiConfigPath).Msg("Anki addon config path provided - running in Anki mode")

		// Try to read dialog port from config file
		if port, err := readDialogPortFromConfig(ankiConfigPath); err == nil && port > 0 {
			dialogPort = port
			logger.Info().Int("dialog_port", dialogPort).Msg("Dialog server port read from config")
		} else {
			logger.Warn().Err(err).Msg("Failed to read dialog port from config, Qt dialogs will be unavailable")
		}
	} else {
		runtime = "browser"
		logger.Info().Msg("No config path provided - running in browser mode")
	}

	// Initialize UI manager based on runtime
	if runtime == "anki" && dialogPort > 0 {
		// Use Qt dialogs via IPC for Anki mode
		ui.Initialize(dialogs.NewQtFileDialog(dialogPort), browser.NewSystemURLOpener())
		logger.Info().Msg("UI manager initialized with Qt dialogs (via IPC) and URL opener")
	} else {
		// Use Zenity dialogs for browser/standalone mode
		ui.Initialize(dialogs.NewZenityFileDialog(), browser.NewSystemURLOpener())
		logger.Info().Msg("UI manager initialized with Zenity dialogs and URL opener")
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create WebRPC API server without listener (router only)
	apiServer := api.NewServerWithoutListener(api.DefaultConfig(), logger)

	// Create runtime config
	runtimeConfig := RuntimeConfig{
		Runtime: runtime,
	}

	// Create asset options for frontend
	assetOptions := assetserveroptions.Options{
		Assets: assets,
	}

	// Create unified server configuration
	unifiedConfig := UnifiedServerConfig{
		RuntimeConfig: runtimeConfig,
		AssetOptions:  assetOptions,
		Logger:        logger,
		APIServer:     apiServer,
		BackendOnly:   false, // Serve frontend, API, and WebSocket
		OnWSConnect:   nil,   // Will be set after creating components
	}

	// Create unified server
	unifiedServer, err := NewUnifiedServer(unifiedConfig)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create unified server")
	}

	// Create broadcaster function for throttler
	broadcaster := func(msgType string, data interface{}) {
		unifiedServer.Emit(msgType, data)
	}

	// Initialize the throttler
	throttler := batch.NewAdaptiveEventThrottler(
		ctx,
		0,                    // minInterval - will be updated from settings
		250*time.Millisecond, // maxInterval - will be updated from settings
		500*time.Millisecond, // rateWindow for measuring event frequency
		true,                 // enabled by default
		&logger,              // Logger for throttler
		broadcaster,          // WebSocket broadcaster
	)

	// Initialize handler
	handler := core.NewGUIHandler(ctx, throttler, unifiedServer)

	// Register all services with API server
	if err := registerServices(apiServer, logger, unifiedServer, throttler, handler, ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register services")
	}

	// Create server components for compatibility
	servers := &ServerComponents{
		APIServer: apiServer,
		WSServer:  unifiedServer,
		Throttler: throttler,
		Handler:   handler,
	}

	// Initialize additional components (LLM, settings, etc.)
	initializeServerComponents(ctx, servers, &logger)

	// Start the unified server
	if err := unifiedServer.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start unified server")
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Update Anki config if path was provided
	if ankiConfigPath != "" {
		port := unifiedServer.GetPort()
		if err := updateAnkiConfigSinglePort(ankiConfigPath, port, &logger); err != nil {
			logger.Error().Err(err).Msg("Failed to update Anki config")
			// Continue anyway - the addon can still discover ports through logs
		}
	}

	// Log server information
	port := unifiedServer.GetPort()
	logger.Info().
		Int("port", port).
		Msg("Server mode ready (single port)")
	
	logger.Warn().Msgf("\n\n\t\t\t🡆 🡆 🡆 FRONTEND: http://localhost:%d 🡄 🡄 🡄\n\n", port)

	// Wait for shutdown signal
	<-sigChan
	logger.Info().Msg("Shutting down server mode...")

	// Cleanup
	if servers.Throttler != nil {
		servers.Throttler.SyncFlush()
	}
	unifiedServer.Shutdown()

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


// updateAnkiConfigSinglePort updates the Anki addon's config.json with single port information
// readDialogPortFromConfig reads the dialog server port from the Anki config file
func readDialogPortFromConfig(configPath string) (int, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return 0, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Debug log the config content
	fmt.Printf("[readDialogPortFromConfig] Config keys: %v\n", getMapKeys(config))

	// Look for dialog_port in the config
	if dialogPort, ok := config["dialog_port"].(float64); ok {
		fmt.Printf("[readDialogPortFromConfig] Found dialog_port: %d\n", int(dialogPort))
		return int(dialogPort), nil
	}

	// Debug log what we actually found
	if dp, exists := config["dialog_port"]; exists {
		fmt.Printf("[readDialogPortFromConfig] dialog_port exists but wrong type: %T\n", dp)
	}

	return 0, fmt.Errorf("dialog_port not found in config")
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func updateAnkiConfigSinglePort(configPath string, port int, logger *zerolog.Logger) error {
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

	// IMPORTANT: Preserve the dialog_port if it exists
	var dialogPort interface{}
	if dp, exists := config["dialog_port"]; exists {
		dialogPort = dp
		logger.Debug().Interface("dialog_port", dialogPort).Msg("Preserving dialog_port in config")
	}

	// Add langkit server information with single port
	config["langkit_server"] = map[string]interface{}{
		"port":          port,
		"frontend_port": port, // Keep for backward compatibility
		"api_port":      port, // Keep for backward compatibility
		"ws_port":       port, // Keep for backward compatibility
		"single_port":   true, // Flag to indicate single-port mode
		"updated_at":    time.Now().Unix(),
	}

	// Restore dialog_port if it existed
	if dialogPort != nil {
		config["dialog_port"] = dialogPort
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
		Int("port", port).
		Msg("Updated Anki addon config with single server port")

	return nil
}