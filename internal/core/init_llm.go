package core

import (
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/summary"
	"github.com/tassa-yoniso-manasi-karoto/langkit/pkg/llms"
)

// InitLLM initializes the LLM subsystem and related components
func InitLLM(handler MessageHandler) {
	// Initialize LLM client system
	llms.Initialize(*handler.ZeroLog())
	
	// Register providers
	llms.RegisterDefaultProviders()
	
	// Initialize summary service (which depends on LLM)
	summary.Initialize(*handler.ZeroLog())
	
	handler.ZeroLog().Info().Msg("LLM and summary services initialized")
}

// ShutdownLLM cleans up LLM resources
func ShutdownLLM(handler MessageHandler) {
	// Clean up summary service
	summary.Shutdown()
	
	handler.ZeroLog().Info().Msg("LLM and summary services shut down")
}