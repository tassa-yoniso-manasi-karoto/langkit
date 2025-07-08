package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var (
	// global provider manager instance
	globalProviderManager *TranslitProviderManager
	// protect global manager lifecycle
	managerLifecycleLock sync.Mutex
)

// Constants for timeouts
const (
	DefaultProviderCloseTimeout    = 5 * time.Minute
	DefaultPoolShutdownTimeout     = 10 * time.Minute
	DefaultManagerMaintenanceTimeout = 3 * time.Minute
)

// ErrPoolAtCapacity is returned when a provider pool is at capacity
var ErrPoolAtCapacity = fmt.Errorf("provider pool at capacity")

// InitTranslitService initializes or re-initializes the global TranslitProviderManager
func InitTranslitService(logger zerolog.Logger) *TranslitProviderManager {
	managerLifecycleLock.Lock()
	defer managerLifecycleLock.Unlock()

	// If no manager exists, or if the existing one has been shut down, create a new one
	if globalProviderManager == nil || globalProviderManager.IsShutdown() {
		if globalProviderManager != nil && !globalProviderManager.IsShutdown() {
			logger.Warn().Msg("Found an active TranslitProviderManager during Init; shutting it down before creating a new one")
			globalProviderManager.Shutdown()
		}
		
		logger.Info().Msg("Creating a new TranslitProviderManager instance")
		config := DefaultProviderManagerConfig()
		globalProviderManager = NewTranslitProviderManager(config, logger)
		DefaultProviderManager = globalProviderManager
	} else {
		logger.Info().Msg("Reusing existing active TranslitProviderManager instance")
	}
	
	return globalProviderManager
}

// ShutdownTranslitService gracefully shuts down the transliteration service
func ShutdownTranslitService() {
	managerLifecycleLock.Lock()
	defer managerLifecycleLock.Unlock()

	if globalProviderManager != nil && !globalProviderManager.IsShutdown() {
		globalProviderManager.Shutdown()
	}
	
	// Set to nil so the next InitTranslitService call knows to create a new instance
	globalProviderManager = nil
	DefaultProviderManager = nil
}

// ProcessWithManagedProvider handles the lifecycle of using a managed transliteration provider
func ProcessWithManagedProvider(
	ctx context.Context,
	langCode string,
	style string,
	text string,
	handler MessageHandler,
	manager *TranslitProviderManager,
) (StringResult, error) {
	// Use the global manager if none provided
	if manager == nil {
		if globalProviderManager == nil {
			// Initialize with default logger if not done yet
			logger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
			InitTranslitService(logger)
		}
		manager = globalProviderManager
	}

	// Get provider from the pool
	pooledProvider, err := manager.GetProvider(ctx, langCode, style, handler)
	if err != nil {
		return StringResult{}, fmt.Errorf("failed to get provider: %w", err)
	}

	// Make sure we release the provider when done
	defer manager.ReleaseProvider(pooledProvider)

	// Process the text with the provider
	result, err := pooledProvider.Provider.ProcessText(ctx, text, handler)
	if err != nil {
		// Mark provider as unhealthy when errors occur
		pooledProvider.IsHealthy = false
		pooledProvider.LastError = err
		
		// Log the provider health issue with detailed information
		if handler != nil && handler.ZeroLog() != nil {
			handler.ZeroLog().Warn().
				Str("provider_key", pooledProvider.Key.String()).
				Str("provider_name", pooledProvider.Provider.ProviderName()).
				Err(err).
				Msg("Transliteration provider marked as unhealthy due to processing error")
		}
		
		// Also log to the manager's logger if available
		if manager != nil {
			manager.logger.Warn().
				Str("provider_key", pooledProvider.Key.String()).
				Str("provider_name", pooledProvider.Provider.ProviderName()).
				Err(err).
				Msg("Transliteration provider marked as unhealthy due to processing error")
		}
		
		return StringResult{}, fmt.Errorf("failed to process text: %w", err)
	}

	return result, nil
}