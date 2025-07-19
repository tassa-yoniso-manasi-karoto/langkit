package core

import (
	"context"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/summary"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

// StateChangeNotifier is an interface for broadcasting state changes via WebSocket
type StateChangeNotifier interface {
	Emit(msgType string, data interface{})
}

// InitLLM initializes the LLM subsystem and related components
// It sets up the Registry but doesn't block on initialization
func InitLLM(handler MessageHandler, wailsContext context.Context, notifier StateChangeNotifier) *llms.Registry {
	llms.AppName = "Langkit"
	
	// Load settings
	settings, err := config.LoadSettings()
	if err != nil {
		handler.ZeroLog().Error().Err(err).Msg("Failed to load settings for LLM initialization")
		settings = config.Settings{} // Use empty settings as fallback
	}
	
	// Create and start the registry
	notifierFunc := func(change llms.StateChange) {
		// Broadcast via WebSocket
		if notifier != nil {
			notifier.Emit("llm.state.changed", change)
		}
		
		handler.ZeroLog().Debug().
			Str("global_state", change.GlobalState.String()).
			Str("message", change.Message).
			Msg("LLM state change emitted")
	}
	
	registry := llms.NewRegistry(settings, *handler.ZeroLog(), notifierFunc)
	llms.DefaultRegistry = registry
	
	registry.Start()
	
	// Initialize summary service (which depends on LLM)
	summary.Initialize(*handler.ZeroLog(), registry)
	
	handler.ZeroLog().Debug().Msg("LLM and summary services initialized")
	
	return registry
}

// ShutdownLLM cleans up LLM resources
func ShutdownLLM(handler MessageHandler) {
	// Shutdown registry
	if llms.DefaultRegistry != nil {
		llms.DefaultRegistry.Shutdown()
	}
	
	// Clean up summary service
	summary.Shutdown()
	
	handler.ZeroLog().Debug().Msg("LLM and summary services shut down")
}