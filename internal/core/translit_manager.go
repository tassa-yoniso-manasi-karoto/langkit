package core

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ProviderKey uniquely identifies a provider type by language and style
type ProviderKey struct {
	LangCode string
	Style    string
}

// String returns a string representation of the provider key
func (pk ProviderKey) String() string {
	return fmt.Sprintf("%s:%s", pk.LangCode, pk.Style)
}

// PooledProvider wraps a TranslitProvider with lifecycle metadata
type PooledProvider struct {
	Provider     TranslitProvider
	InUse        bool
	LastUsed     time.Time
	UsageCount   int64
	CreatedAt    time.Time
	InitDuration time.Duration
	IsHealthy    bool
	LastError    error
	Key          ProviderKey
}

// ProviderPool manages a collection of providers for a specific language and style
type ProviderPool struct {
	Key       ProviderKey
	Providers []*PooledProvider
	mu        sync.Mutex
	LastUsed  time.Time
	logger    zerolog.Logger
	config    ProviderManagerConfig
}

// NewProviderPool creates a new provider pool for the specified language and style
func NewProviderPool(key ProviderKey, config ProviderManagerConfig, logger zerolog.Logger) *ProviderPool {
	return &ProviderPool{
		Key:       key,
		Providers: make([]*PooledProvider, 0, config.InitialPoolCapacity),
		LastUsed:  time.Now(),
		logger:    logger.With().Str("component", "provider_pool").Str("pool", key.String()).Logger(),
		config:    config,
	}
}

// AcquireProvider gets an available provider from the pool or creates a new one
func (p *ProviderPool) AcquireProvider(ctx context.Context) (*PooledProvider, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.LastUsed = time.Now()

	// Try to find an available provider
	for _, provider := range p.Providers {
		if !provider.InUse {
			if !provider.IsHealthy {
				// Log detailed information about the unhealthy provider being skipped
				p.logger.Warn().
					Str("provider_id", fmt.Sprintf("%p", provider.Provider)).
					Str("provider_key", provider.Key.String()).
					Str("provider_name", provider.Provider.ProviderName()).
					Err(provider.LastError).
					Msg("Skipping unhealthy provider in pool")
				continue
			}
			
			// Provider is available and healthy
			provider.InUse = true
			provider.UsageCount++
			p.logger.Debug().
				Str("provider_id", fmt.Sprintf("%p", provider.Provider)).
				Int64("usage_count", provider.UsageCount).
				Msg("Acquired existing provider from pool")
			return provider, nil
		}
	}

	// If we're at max capacity, wait for a provider to become available
	if len(p.Providers) >= p.config.MaxProvidersPerLang {
		p.logger.Warn().
			Int("current_size", len(p.Providers)).
			Int("max_size", p.config.MaxProvidersPerLang).
			Msg("Provider pool at capacity, cannot create new provider")
		
		// Return a typed error rather than waiting to avoid deadlocks
		return nil, fmt.Errorf("%w for %s (%d providers), try again later", 
			ErrPoolAtCapacity, p.Key, p.config.MaxProvidersPerLang)
	}

	// Create a new provider
	p.logger.Info().
		Msg("Creating new provider for pool")
	
	provider, err := p.createNewProvider(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create new provider: %w", err)
	}

	return provider, nil
}

// ReleaseProvider returns a provider to the pool
func (p *ProviderPool) ReleaseProvider(provider *PooledProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, existingProvider := range p.Providers {
		if existingProvider == provider {
			p.Providers[i].InUse = false
			p.Providers[i].LastUsed = time.Now()
			p.logger.Debug().
				Str("provider_id", fmt.Sprintf("%p", provider.Provider)).
				Msg("Released provider back to pool")
			return
		}
	}

	p.logger.Warn().
		Str("provider_id", fmt.Sprintf("%p", provider.Provider)).
		Msg("Attempted to release provider not found in pool")
}

// createNewProvider instantiates and initializes a new provider
func (p *ProviderPool) createNewProvider(ctx context.Context) (*PooledProvider, error) {
	startTime := time.Now()

	// Create a new provider instance
	rawProvider, err := GetTranslitProvider(p.Key.LangCode, p.Key.Style)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider for %s: %w", p.Key, err)
	}

	// Create a dummy task for initialization with DockerRecreate=false to reuse containers
	dummyTask := &Task{
		DockerRecreate: false,
		Handler: &silentMessageHandler{
			logger: p.logger,
			ctx:    ctx,
		},
		RomanizationStyle: p.Key.Style,
	}

	// Initialize the provider
	initStartTime := time.Now()
	err = rawProvider.Initialize(ctx, dummyTask)
	initDuration := time.Since(initStartTime)

	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider for %s: %w", p.Key, err)
	}

	// Create the pooled provider
	pooledProvider := &PooledProvider{
		Provider:     rawProvider,
		InUse:        true,
		LastUsed:     time.Now(),
		UsageCount:   1,
		CreatedAt:    startTime,
		InitDuration: initDuration,
		IsHealthy:    true,
		Key:          p.Key,
	}

	// Add to pool
	p.Providers = append(p.Providers, pooledProvider)

	p.logger.Info().
		Str("provider_id", fmt.Sprintf("%p", rawProvider)).
		Dur("init_duration", initDuration).
		Msg("Created and initialized new provider")

	return pooledProvider, nil
}

// CleanupIdleProviders removes providers that haven't been used for a while
func (p *ProviderPool) CleanupIdleProviders(ctx context.Context, maxIdleTime time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	remaining := make([]*PooledProvider, 0, len(p.Providers))
	
	for _, provider := range p.Providers {
		// Skip providers that are in use
		if provider.InUse {
			remaining = append(remaining, provider)
			continue
		}

		// Check if the provider has been idle too long
		idleTime := now.Sub(provider.LastUsed)
		if idleTime > maxIdleTime {
			p.logger.Info().
				Str("provider_id", fmt.Sprintf("%p", provider.Provider)).
				Dur("idle_time", idleTime).
				Msg("Closing idle provider")
			
			// Close the provider
			closeCtx, cancel := context.WithTimeout(ctx, DefaultProviderCloseTimeout)
			err := provider.Provider.Close(closeCtx, p.Key.LangCode, p.Key.Style)
			cancel()
			
			if err != nil {
				p.logger.Warn().
					Err(err).
					Str("provider_id", fmt.Sprintf("%p", provider.Provider)).
					Msg("Failed to close idle provider")
			}
		} else {
			// Keep this provider
			remaining = append(remaining, provider)
		}
	}

	// Update the pool with remaining providers
	p.Providers = remaining
}

// Shutdown closes all providers in the pool
func (p *ProviderPool) Shutdown(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Info().
		Int("provider_count", len(p.Providers)).
		Msg("Shutting down provider pool")

	for _, provider := range p.Providers {
		closeCtx, cancel := context.WithTimeout(ctx, DefaultProviderCloseTimeout)
		err := provider.Provider.Close(closeCtx, p.Key.LangCode, p.Key.Style)
		cancel()
		
		if err != nil {
			p.logger.Warn().
				Err(err).
				Str("provider_id", fmt.Sprintf("%p", provider.Provider)).
				Msg("Failed to close provider during shutdown")
		}
	}

	// Clear the providers list
	p.Providers = nil
}

// ProviderManagerConfig holds configuration for the provider manager
type ProviderManagerConfig struct {
	MaxProvidersPerLang int           // Maximum providers per language
	InitialPoolCapacity int           // Initial capacity of each provider pool
	IdleTimeout         time.Duration // How long to keep unused providers
}

// DefaultProviderManagerConfig returns a default configuration
func DefaultProviderManagerConfig() ProviderManagerConfig {
	return ProviderManagerConfig{
		MaxProvidersPerLang: 3,
		InitialPoolCapacity: 3,
		IdleTimeout:         30 * time.Minute,
	}
}

// TranslitProviderManager manages provider pools for different languages
type TranslitProviderManager struct {
	pools       map[string]*ProviderPool
	mu          sync.RWMutex
	config      ProviderManagerConfig
	logger      zerolog.Logger
	shutdown    chan struct{}
	shutdownWg  sync.WaitGroup
	shutdownMu  sync.Mutex
	isShutdown  bool
}

// NewTranslitProviderManager creates a new provider manager
func NewTranslitProviderManager(config ProviderManagerConfig, logger zerolog.Logger) *TranslitProviderManager {
	manager := &TranslitProviderManager{
		pools:      make(map[string]*ProviderPool),
		config:     config,
		logger:     logger.With().Str("component", "provider_manager").Logger(),
		shutdown:   make(chan struct{}),
		isShutdown: false, // Explicitly initialize as not shut down
	}

	// Start background maintenance
	manager.startMaintenance()

	return manager
}

// GetProvider acquires a provider for the specified language and style
func (m *TranslitProviderManager) GetProvider(ctx context.Context, langCode, style string) (*PooledProvider, error) {
	key := ProviderKey{LangCode: langCode, Style: style}
	poolKey := key.String()

	// First try with read lock to see if pool exists
	m.mu.RLock()
	pool, exists := m.pools[poolKey]
	m.mu.RUnlock()

	if !exists {
		// Need to create the pool, use write lock
		m.mu.Lock()
		// Double-check in case another goroutine created it
		pool, exists = m.pools[poolKey]
		if !exists {
			// Create new pool
			pool = NewProviderPool(key, m.config, m.logger)
			m.pools[poolKey] = pool
			m.logger.Info().
				Str("pool_key", poolKey).
				Msg("Created new provider pool")
		}
		m.mu.Unlock()
	}

	// Acquire provider from the pool
	return pool.AcquireProvider(ctx)
}

// ReleaseProvider returns a provider to its pool
func (m *TranslitProviderManager) ReleaseProvider(provider *PooledProvider) {
	poolKey := provider.Key.String()

	m.mu.RLock()
	pool, exists := m.pools[poolKey]
	m.mu.RUnlock()

	if exists {
		pool.ReleaseProvider(provider)
	} else {
		m.logger.Warn().
			Str("pool_key", poolKey).
			Str("provider_id", fmt.Sprintf("%p", provider.Provider)).
			Msg("Attempted to release provider to non-existent pool")
	}
}

// startMaintenance starts a background goroutine to perform maintenance tasks
func (m *TranslitProviderManager) startMaintenance() {
	m.shutdownWg.Add(1)
	go func() {
		defer m.shutdownWg.Done()

		ticker := time.NewTicker(m.config.IdleTimeout / 4)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.performMaintenance()
			case <-m.shutdown:
				return
			}
		}
	}()
}

// performMaintenance cleans up idle providers and pools
func (m *TranslitProviderManager) performMaintenance() {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultManagerMaintenanceTimeout)
	defer cancel()

	// Get all pools under read lock
	m.mu.RLock()
	poolKeys := make([]string, 0, len(m.pools))
	for key := range m.pools {
		poolKeys = append(poolKeys, key)
	}
	m.mu.RUnlock()

	// Clean up idle providers in each pool
	for _, key := range poolKeys {
		m.mu.RLock()
		pool, exists := m.pools[key]
		m.mu.RUnlock()

		if exists {
			pool.CleanupIdleProviders(ctx, m.config.IdleTimeout)
		}
	}

	// Remove empty pools that haven't been used for a long time
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, pool := range m.pools {
		if len(pool.Providers) == 0 && now.Sub(pool.LastUsed) > m.config.IdleTimeout*2 {
			delete(m.pools, key)
			m.logger.Info().
				Str("pool_key", key).
				Msg("Removed unused empty provider pool")
		}
	}
}

// Shutdown closes all provider pools
func (m *TranslitProviderManager) Shutdown() {
	// Check if already shut down
	m.shutdownMu.Lock()
	if m.isShutdown {
		m.shutdownMu.Unlock()
		m.logger.Info().Msg("TranslitProviderManager already shut down")
		return
	}
	m.isShutdown = true
	m.shutdownMu.Unlock()

	m.logger.Info().Msg("Shutting down TranslitProviderManager")
	
	// Signal maintenance goroutine to stop
	// Check if channel is already closed to prevent panic
	select {
	case <-m.shutdown:
		// Already closed or being closed
	default:
		close(m.shutdown)
	}
	
	// Wait for maintenance goroutine to finish
	m.shutdownWg.Wait()
	
	// Shutdown all pools
	ctx, cancel := context.WithTimeout(context.Background(), DefaultPoolShutdownTimeout)
	defer cancel()
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for key, pool := range m.pools {
		m.logger.Info().Str("pool_key", key).Msg("Shutting down provider pool")
		pool.Shutdown(ctx)
	}
	
	// Clear all pools
	m.pools = make(map[string]*ProviderPool)
}

// IsShutdown returns whether the manager has been shut down
func (m *TranslitProviderManager) IsShutdown() bool {
	m.shutdownMu.Lock()
	defer m.shutdownMu.Unlock()
	return m.isShutdown
}

// GetProvidersStats returns statistics about all provider pools
func (m *TranslitProviderManager) GetProvidersStats() map[string]interface{} {
	stats := make(map[string]interface{})
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	poolStats := make(map[string]interface{})
	totalProviders := 0
	activeProviders := 0
	
	for key, pool := range m.pools {
		pool.mu.Lock()
		
		poolStat := map[string]interface{}{
			"total_providers":   len(pool.Providers),
			"active_providers":  0,
			"idle_providers":    0,
			"last_used":         pool.LastUsed.Format(time.RFC3339),
		}
		
		for _, provider := range pool.Providers {
			if provider.InUse {
				poolStat["active_providers"] = poolStat["active_providers"].(int) + 1
				activeProviders++
			} else {
				poolStat["idle_providers"] = poolStat["idle_providers"].(int) + 1
			}
		}
		
		totalProviders += len(pool.Providers)
		poolStats[key] = poolStat
		
		pool.mu.Unlock()
	}
	
	stats["pools"] = poolStats
	stats["total_pools"] = len(m.pools)
	stats["total_providers"] = totalProviders
	stats["active_providers"] = activeProviders
	stats["idle_providers"] = totalProviders - activeProviders
	
	return stats
}

// silentMessageHandler is a minimal implementation of MessageHandler for initialization
type silentMessageHandler struct {
	logger zerolog.Logger
	ctx    context.Context
}

func (h *silentMessageHandler) ZeroLog() *zerolog.Logger {
	return &h.logger
}

func (h *silentMessageHandler) GetContext() context.Context {
	return h.ctx
}

func (h *silentMessageHandler) LogErr(err error, behavior string, msg string) *ProcessingError {
	h.logger.Error().Err(err).Str("behavior", behavior).Msg(msg)
	return &ProcessingError{Err: err, Behavior: behavior}
}

func (h *silentMessageHandler) LogErrWithLevel(level int8, err error, behavior string, msg string) *ProcessingError {
	h.logger.Error().Err(err).Str("behavior", behavior).Int8("level", level).Msg(msg)
	return &ProcessingError{Err: err, Behavior: behavior}
}

func (h *silentMessageHandler) IncrementProgress(id string, inc, total, weight int, title, subtitle, height string) {
	// Do nothing - silent handler
}

func (h *silentMessageHandler) IncrementProgressAdvanced(id string, inc, total, weight int, title, subtitle, height string) {
	// Do nothing - silent handler
}

func (h *silentMessageHandler) ResetProgress() {
	// Do nothing - silent handler
}

func (h *silentMessageHandler) RemoveProgressBar(id string) {
	// Do nothing - silent handler
}

func (h *silentMessageHandler) SetHighLoadMode(durations ...time.Duration) {
	// Do nothing - silent handler
}

func (h *silentMessageHandler) Log(level int8, behavior string, msg string) *ProcessingError {
	h.logger.WithLevel(zerolog.Level(level)).Str("behavior", behavior).Msg(msg)
	return &ProcessingError{Err: nil, Behavior: behavior}
}

func (h *silentMessageHandler) LogFields(level int8, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	event := h.logger.WithLevel(zerolog.Level(level)).Str("behavior", behavior)
	for k, v := range fields {
		event = event.Interface(k, v)
	}
	event.Msg(msg)
	return &ProcessingError{Err: nil, Behavior: behavior}
}

func (h *silentMessageHandler) LogErrFields(err error, behavior string, msg string, fields map[string]interface{}) *ProcessingError {
	event := h.logger.Error().Err(err).Str("behavior", behavior)
	for k, v := range fields {
		event = event.Interface(k, v)
	}
	event.Msg(msg)
	return &ProcessingError{Err: err, Behavior: behavior}
}

func (h *silentMessageHandler) GetLogBuffer() bytes.Buffer {
	return bytes.Buffer{} // Empty buffer for silent handler
}

func (h *silentMessageHandler) HandleStatus(status string) {
	// Do nothing - silent handler
}

func (h *silentMessageHandler) IsCLI() bool {
	return true
}

// Global instance of the provider manager
var DefaultProviderManager *TranslitProviderManager