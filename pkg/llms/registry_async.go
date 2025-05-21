package llms

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
)

// Error variables for Registry
var (
	ErrRegistryNotReady = errors.New("registry not ready")
	ErrContextCanceled  = errors.New("context canceled or timed out")
)

// Registry is the central manager for LLM providers
type Registry struct {
	config              config.Settings // Configuration containing API keys
	client              *Client         // Client containing successfully initialized providers
	globalState         GlobalServiceState
	providerStates      map[string]ProviderState
	stateChangeNotifier func(StateChange)
	readySignalChan     chan struct{}
	updateTriggerChan   chan config.Settings
	shutdownChan        chan struct{}
	backgroundWorkerWG  sync.WaitGroup
	subscribers         []chan StateChange // Channels to receive state change events
	subscribersMu       sync.RWMutex       // Mutex for subscribers slice
	mu                  sync.RWMutex
	logger              zerolog.Logger
	initialized         bool // Flag to prevent multiple starts
}

// NewRegistry creates a new LLM provider registry
func NewRegistry(initialSettings config.Settings, logger zerolog.Logger, notifierFunc func(StateChange)) *Registry {
	r := &Registry{
		config:              initialSettings,
		globalState:         GSUninitialized,
		providerStates:      make(map[string]ProviderState),
		stateChangeNotifier: notifierFunc,
		readySignalChan:     make(chan struct{}),
		updateTriggerChan:   make(chan config.Settings, 5), // Buffer for multiple quick updates
		shutdownChan:        make(chan struct{}),
		subscribers:         make([]chan StateChange, 0),
		logger:              logger.With().Str("component", "llm_registry").Logger(),
		initialized:         false,
	}
	
	return r
}

// Start initializes the registry and begins the background worker
func (r *Registry) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.initialized {
		return errors.New("registry already started")
	}
	
	r.logger.Info().Msg("Starting LLM registry")
	r.initialized = true
	r.setGlobalStateAndNotify(GSInitializing, "Initializing LLM providers...")
	
	// Start background worker
	r.backgroundWorkerWG.Add(1)
	go r.backgroundWorker()
	
	return nil
}

// backgroundWorker is the main goroutine that handles provider initialization
func (r *Registry) backgroundWorker() {
	defer r.backgroundWorkerWG.Done()
	
	r.logger.Debug().Msg("LLM registry background worker started")
	r.performFullInitialization(r.config)
	
	for {
		select {
		case newSettings := <-r.updateTriggerChan:
			r.logger.Info().Msg("LLM registry: Configuration update received")
			r.config = newSettings
			r.setGlobalStateAndNotify(GSUpdating, "Configuration changed, re-initializing LLM providers...")
			r.performFullInitialization(newSettings)
			
		case <-r.shutdownChan:
			r.logger.Info().Msg("LLM registry: Shutdown signal received by background worker")
			// Cleanup happens in Shutdown() after this goroutine exits
			return
		}
	}
}

// performFullInitialization initializes all providers based on configuration
func (r *Registry) performFullInitialization(settings config.Settings) {
	r.mu.Lock()
	// Create a fresh client
	r.client = NewClient()
	// Reset provider states
	r.providerStates = make(map[string]ProviderState)
	r.mu.Unlock()
	
	// Load API keys from settings
	LoadAPIKeysFromSettings(settings)
	
	// Determine which providers should be initialized based on available API keys
	providersToInit := make(map[string]string)
	
	// Check for OpenAI API key
	if APIKeys.Has("openai") {
		providersToInit["openai"] = APIKeys.Get("openai")
	}
	
	// Check for OpenRouter API key
	if APIKeys.Has("openrouter") {
		providersToInit["openrouter"] = APIKeys.Get("openrouter")
	}
	
	// Check for Google API key
	if APIKeys.Has("google") {
		providersToInit["google"] = APIKeys.Get("google")
	}
	
	if len(providersToInit) == 0 {
		r.logger.Warn().Msg("No LLM providers to initialize (no API keys configured)")
		r.setGlobalStateAndNotify(GSReady, "No LLM providers available")
		r.signalReady()
		return
	}
	
	r.logger.Info().Int("count", len(providersToInit)).Msg("Initializing LLM providers")
	
	// Initialize provider states map with "not_attempted" status
	r.mu.Lock()
	for providerName := range providersToInit {
		r.providerStates[providerName] = ProviderState{
			Status:      "not_attempted",
			LastUpdated: time.Now(),
		}
	}
	r.mu.Unlock()
	
	// Initialize all providers concurrently
	var wg sync.WaitGroup
	for providerName, apiKey := range providersToInit {
		wg.Add(1)
		go r.initializeSingleProvider(providerName, apiKey, &wg)
	}
	
	// Wait for all providers to finish initialization
	wg.Wait()
	r.logger.Info().Msg("All LLM providers initialization attempts completed")
	
	// Determine final global state and set up client
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Register only successful providers with the client
	readyProviders := 0
	for providerName, state := range r.providerStates {
		if state.Status == "ready" || state.Status == "models_loaded" {
			switch providerName {
			case "openai":
				openAIProvider := NewOpenAIProvider(providersToInit[providerName])
				if openAIProvider != nil {
					r.client.RegisterProvider(openAIProvider)
					readyProviders++
				}
				
			case "openrouter":
				// Get the master OpenRouter provider
				masterOpenRouterProvider := NewOpenRouterProvider(providersToInit[providerName])
				if masterOpenRouterProvider != nil {
					// Create the free provider variant
					freeProvider := NewOpenRouterFreeProvider(masterOpenRouterProvider)
					if freeProvider != nil {
						r.client.RegisterProvider(freeProvider)
					}
					
					// Create the paid provider variant
					paidProvider := NewOpenRouterPaidProvider(masterOpenRouterProvider)
					if paidProvider != nil {
						r.client.RegisterProvider(paidProvider)
					}
					
					readyProviders++
				}
				
			case "google":
				googleProvider := NewGoogleProvider(providersToInit[providerName])
				if googleProvider != nil {
					r.client.RegisterProvider(googleProvider)
					readyProviders++
				}
			}
		}
	}
	
	// Set default provider (prioritize openrouter-free, then openai, etc.)
	if readyProviders > 0 {
		for _, name := range []string{"openrouter-free", "openai", "openrouter", "google"} {
			if _, ok := r.client.GetProvider(name); ok {
				r.client.SetDefaultProvider(name)
				r.logger.Info().Str("provider", name).Msg("Set as default LLM provider")
				break
			}
		}
		
		r.globalState = GSReady
		r.logger.Info().Int("ready_providers", readyProviders).Msg("LLM registry ready")
	} else {
		r.globalState = GSError
		r.logger.Warn().Msg("No LLM providers successfully initialized")
	}
	
	// Notify of state change
	r.notifyStateChange("All initialization complete", "")
	
	// Signal ready
	r.signalReady()
}

// initializeSingleProvider initializes a single provider (runs in its own goroutine)
func (r *Registry) initializeSingleProvider(providerName, apiKey string, wg *sync.WaitGroup) {
	defer wg.Done()
	
	r.logger.Debug().Str("provider", providerName).Msg("Initializing provider")
	
	// Update status to initializing
	r.updateProviderState(providerName, "initializing_models", nil, nil)
	
	// Create a temporary provider for model fetching
	var provider Provider
	var err error
	
	// Instantiate appropriate provider
	switch providerName {
	case "openai":
		provider = NewOpenAIProvider(apiKey)
	case "openrouter":
		provider = NewOpenRouterProvider(apiKey)
	case "google":
		provider = NewGoogleProvider(apiKey)
	default:
		err = fmt.Errorf("unknown provider type: %s", providerName)
	}
	
	if err != nil || provider == nil {
		if err == nil {
			err = errors.New("failed to create provider instance")
		}
		r.updateProviderState(providerName, "error", err, nil)
		return
	}
	
	// Create a context with timeout for model fetching
	modelFetchCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Fetch available models using context
	// This makes an API call to fetch models in a real implementation
	models := provider.GetAvailableModels(modelFetchCtx)
	
	// Update provider state
	if len(models) > 0 {
		r.updateProviderState(providerName, "ready", nil, models)
	} else {
		// If no models were returned but no error occurred, mark as ready but log a warning
		r.updateProviderState(providerName, "ready", nil, []ModelInfo{})
		r.logger.Warn().Str("provider", providerName).Msg("Provider initialized with empty models list")
	}
	
	r.logger.Info().
		Str("provider", providerName).
		Int("models", len(models)).
		Msg("Provider ready")
}

// updateProviderState updates a provider's state and sends notification
func (r *Registry) updateProviderState(providerName, status string, err error, models []ModelInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	state := ProviderState{
		Status:      status,
		Error:       err,
		LastUpdated: time.Now(),
	}
	
	if models != nil {
		state.Models = models
	} else if existing, ok := r.providerStates[providerName]; ok {
		state.Models = existing.Models
	}
	
	r.providerStates[providerName] = state
	
	// Notify outside of mutex lock
	go r.notifyStateChange("Provider state changed", providerName)
}

// setGlobalStateAndNotify updates the global state and sends notification
func (r *Registry) setGlobalStateAndNotify(state GlobalServiceState, message string) {
	r.mu.Lock()
	r.globalState = state
	r.mu.Unlock()
	
	r.notifyStateChange(message, "")
}

// notifyStateChange sends a notification about state changes
func (r *Registry) notifyStateChange(message, providerName string) {
	r.mu.RLock()
	
	// Create a copy of provider states
	statesCopy := make(map[string]ProviderState, len(r.providerStates))
	for name, state := range r.providerStates {
		statesCopy[name] = state
	}
	
	// Create state change event
	stateChange := StateChange{
		Timestamp:             time.Now(),
		GlobalState:           r.globalState,
		UpdatedProviderName:   providerName,
		ProviderStatesSnapshot: statesCopy,
		Message:               message,
	}
	r.mu.RUnlock()
	
	// Call notifier function if set
	if r.stateChangeNotifier != nil {
		r.stateChangeNotifier(stateChange)
	}
	
	// Notify all subscribers
	r.subscribersMu.RLock()
	for _, ch := range r.subscribers {
		// Non-blocking send to avoid deadlocks if a subscriber is not reading
		select {
		case ch <- stateChange:
			// Successfully sent
		default:
			// Channel full or closed, log and continue
			r.logger.Warn().Msg("Failed to send state change to a subscriber (channel full or closed)")
		}
	}
	r.subscribersMu.RUnlock()
}

// signalReady closes the readySignalChan to unblock waiters
func (r *Registry) signalReady() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Close the channel to unblock any waiters
	close(r.readySignalChan)
	
	// Create a new channel for next update cycle
	r.readySignalChan = make(chan struct{})
}

// Public Methods

// TriggerUpdate sends a configuration update to the background worker
func (r *Registry) TriggerUpdate(newSettings config.Settings) {
	select {
	case r.updateTriggerChan <- newSettings:
		r.logger.Debug().Msg("Update triggered in registry")
	default:
		r.logger.Warn().Msg("Update channel full, update skipped")
	}
}

// Shutdown stops the background worker and cleans up
func (r *Registry) Shutdown() {
	r.logger.Info().Msg("Shutting down LLM registry")
	
	// Send shutdown signal to background worker
	close(r.shutdownChan)
	
	// Wait for background worker to finish
	r.backgroundWorkerWG.Wait()
	
	// Close all subscriber channels
	r.subscribersMu.Lock()
	for _, ch := range r.subscribers {
		close(ch)
	}
	r.subscribers = nil
	r.subscribersMu.Unlock()
	
	r.logger.Debug().Msg("LLM registry shutdown complete")
}

// SubscribeToStateChanges returns a channel that will receive state change events
func (r *Registry) SubscribeToStateChanges() <-chan StateChange {
	ch := make(chan StateChange, 10) // Buffer to avoid blocking
	
	r.subscribersMu.Lock()
	r.subscribers = append(r.subscribers, ch)
	r.subscribersMu.Unlock()
	
	// Send the current state immediately
	currentState := r.GetCurrentStateSnapshot()
	select {
	case ch <- currentState:
		// Successfully sent initial state
	default:
		r.logger.Warn().Msg("Failed to send initial state to new subscriber (channel full)")
	}
	
	return ch
}

// UnsubscribeFromStateChanges removes a subscriber channel
func (r *Registry) UnsubscribeFromStateChanges(ch <-chan StateChange) {
	r.subscribersMu.Lock()
	defer r.subscribersMu.Unlock()
	
	for i, subCh := range r.subscribers {
		if subCh == ch {
			// Remove this channel from the slice
			r.subscribers = append(r.subscribers[:i], r.subscribers[i+1:]...)
			close(subCh)
			break
		}
	}
}

// GetCurrentStateSnapshot returns the current state of the registry
func (r *Registry) GetCurrentStateSnapshot() StateChange {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Create a copy of provider states
	statesCopy := make(map[string]ProviderState, len(r.providerStates))
	for name, state := range r.providerStates {
		statesCopy[name] = state
	}
	
	return StateChange{
		Timestamp:             time.Now(),
		GlobalState:           r.globalState,
		ProviderStatesSnapshot: statesCopy,
	}
}

// WaitForReady blocks until the registry is ready or the context is done
func (r *Registry) WaitForReady(ctx context.Context) error {
	r.mu.RLock()
	readyChan := r.readySignalChan
	currentState := r.globalState
	r.mu.RUnlock()
	
	// If already ready or in error state, return immediately
	if currentState == GSReady {
		return nil
	} else if currentState == GSError {
		return ErrRegistryNotReady
	}
	
	// Wait for ready signal or context cancellation
	select {
	case <-readyChan:
		// Check final state after ready signal
		r.mu.RLock()
		finalState := r.globalState
		r.mu.RUnlock()
		
		if finalState == GSReady {
			return nil
		} else {
			return ErrRegistryNotReady
		}
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
	}
}

// GetClient returns the client instance if the registry is ready
func (r *Registry) GetClient() (*Client, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if r.globalState != GSReady {
		return nil, ErrRegistryNotReady
	}
	
	return r.client, nil
}