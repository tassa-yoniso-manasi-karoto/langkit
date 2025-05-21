package llms

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
)

// Error variables for Registry
var (
	ErrRegistryNotReady     = errors.New("llm registry: system not ready")
	ErrRegistryNotInit      = errors.New("llm registry: system not initialized (Start() not called)")
	ErrRegistryShutdown     = errors.New("llm registry: system is shut down or shutting down")
	ErrContextCanceled      = errors.New("llm registry: operation canceled by context")
	ErrProviderInitFailed   = errors.New("llm registry: provider initialization failed")
	ErrProviderModelsFailed = errors.New("llm registry: provider failed to fetch models")
)

// Registry is the central manager for LLM providers
type Registry struct {
	config              config.Settings
	client              *Client
	globalState         GlobalServiceState
	providerStates      map[string]ProviderState
	stateChangeNotifier func(StateChange)
	readySignalChan     chan struct{}
	updateTriggerChan   chan config.Settings
	shutdownChan        chan struct{}
	backgroundWorkerWG  sync.WaitGroup
	subscribers         map[chan StateChange]bool
	subscribersMu       sync.RWMutex
	mu                  sync.RWMutex
	logger              zerolog.Logger
	initialized         bool
	isShutdown          bool
}

// NewRegistry creates a new LLM provider registry
func NewRegistry(initialSettings config.Settings, baseLogger zerolog.Logger, notifierFunc func(StateChange)) *Registry {
	registryLogger := baseLogger.With().Str("component", "llm_registry").Logger()
	registryLogger.Trace().Msg("NewRegistry: Creating new LLM Registry instance.")
	r := &Registry{
		config:              initialSettings,
		client:              NewClient(), // Initialize client here
		globalState:         GSUninitialized,
		providerStates:      make(map[string]ProviderState),
		stateChangeNotifier: notifierFunc,
		readySignalChan:     make(chan struct{}),
		updateTriggerChan:   make(chan config.Settings, 5),
		shutdownChan:        make(chan struct{}),
		subscribers:         make(map[chan StateChange]bool),
		logger:              registryLogger,
		initialized:         false,
		isShutdown:          false,
	}
	r.logger.Trace().Msg("NewRegistry: Instance created.")
	return r
}

// Start initializes the registry and begins the background worker
func (r *Registry) Start() error {
	r.mu.Lock()
	r.logger.Trace().Msg("Start: Acquired mutex.")
	if r.isShutdown {
		r.mu.Unlock()
		r.logger.Warn().Msg("Start: Attempted to start a shutdown registry.")
		return ErrRegistryShutdown
	}
	if r.initialized {
		r.mu.Unlock()
		r.logger.Warn().Msg("Start: Registry already started.")
		return errors.New("llm registry: already started")
	}
	r.initialized = true
	r.mu.Unlock()
	r.logger.Trace().Msg("Start: Released mutex.")

	r.logger.Info().Msg("LLM Registry: Scheduling background worker to start initialization.")
	r.backgroundWorkerWG.Add(1)
	go r.backgroundWorker()

	return nil
}

// backgroundWorker is the main goroutine that handles provider initialization and updates
func (r *Registry) backgroundWorker() {
	defer r.backgroundWorkerWG.Done()
	r.logger.Debug().Msg("backgroundWorker: Goroutine started.")

	// Set initial state and notify from within the worker goroutine
	r.setGlobalStateAndNotify(GSInitializing, "LLM services initializing...", nil)
	r.performFullInitialization(r.config) // Use initial config

	r.logger.Debug().Msg("backgroundWorker: Initial full initialization complete. Entering update/shutdown loop.")
	for {
		select {
		case newSettings, ok := <-r.updateTriggerChan:
			if !ok {
				r.logger.Info().Msg("backgroundWorker: Update trigger channel closed, likely during shutdown.")
				return
			}
			r.logger.Info().Msg("backgroundWorker: Configuration update received via trigger channel.")
			r.config = newSettings // Update internal config
			r.setGlobalStateAndNotify(GSUpdating, "Configuration changed, re-initializing LLM providers...", nil)
			r.performFullInitialization(newSettings) // Re-run initialization with new settings
			r.logger.Info().Msg("backgroundWorker: Re-initialization due to configuration update complete.")
		case <-r.shutdownChan:
			r.logger.Info().Msg("backgroundWorker: Shutdown signal received. Terminating worker.")
			return
		}
	}
}

// performFullInitialization initializes all providers based on configuration
func (r *Registry) performFullInitialization(settings config.Settings) {
	r.logger.Trace().Msg("performFullInitialization: Starting.")
	r.mu.Lock()
	r.logger.Trace().Msg("performFullInitialization: Acquired main mutex.")
	r.client = NewClient() // Create a fresh client for this initialization cycle
	r.providerStates = make(map[string]ProviderState)
	r.mu.Unlock()
	r.logger.Trace().Msg("performFullInitialization: Released main mutex. Client and providerStates reset.")

	// CRITICAL: Refresh the global APIKeys store with the current settings for this cycle.
	r.logger.Trace().Interface("settings_api_keys", settings.APIKeys).Msg("performFullInitialization: Calling LoadAPIKeysFromSettings to refresh global API key store.")
	LoadAPIKeysFromSettings(settings) // This uses the 'settings' parameter passed to this function.

	providersToInit := make(map[string]string)
	// Check global APIKeys store which was just updated
	if APIKeys.Has("openai") {
		providersToInit["openai"] = APIKeys.Get("openai")
	}
	if APIKeys.Has("openrouter") {
		providersToInit["openrouter"] = APIKeys.Get("openrouter")
	}
	if APIKeys.Has("google") {
		providersToInit["google"] = APIKeys.Get("google")
	}
	r.logger.Trace().Interface("providers_to_init", providersToInit).Msg("performFullInitialization: Determined providers to initialize based on refreshed APIKeys.")

	if len(providersToInit) == 0 {
		r.logger.Warn().Msg("performFullInitialization: No LLM providers to initialize (no API keys configured or found in current settings).")
		r.setGlobalStateAndNotify(GSReady, "No LLM providers available for initialization.", nil)
		r.signalReady()
		r.logger.Trace().Msg("performFullInitialization: Finished due to no providers to init.")
		return
	}

	r.logger.Info().Int("count", len(providersToInit)).Msg("performFullInitialization: Starting initialization of LLM providers.")

	r.mu.Lock()
	r.logger.Trace().Msg("performFullInitialization: Acquired main mutex for initial provider state setup.")
	for providerName := range providersToInit {
		r.providerStates[providerName] = ProviderState{
			Status:      "not_attempted",
			LastUpdated: time.Now(),
		}
	}
	r.mu.Unlock()
	r.logger.Trace().Msg("performFullInitialization: Released main mutex after initial provider state setup.")
	r.notifyStateChange("Starting individual provider initializations.", "")

	var wg sync.WaitGroup
	for providerName, apiKey := range providersToInit {
		wg.Add(1)
		pName := providerName // Capture range variables for goroutine
		pApiKey := apiKey
		go r.initializeSingleProvider(pName, pApiKey, &wg)
	}

	r.logger.Trace().Msg("performFullInitialization: All single provider initialization goroutines launched. Waiting for WaitGroup...")
	wg.Wait()
	r.logger.Info().Msg("performFullInitialization: All LLM provider initialization attempts completed.")

	r.mu.Lock()
	r.logger.Trace().Msg("performFullInitialization: Acquired main mutex for final client setup and global state.")
	defer func() {
		r.mu.Unlock()
		r.logger.Trace().Msg("performFullInitialization: Released main mutex after final client setup and global state.")
	}()

	readyProvidersCount := 0
	var actualRegisteredProviders []string

	for providerNameInState, state := range r.providerStates {
		if state.Status == "ready" || state.Status == "models_loaded" {
			// Get the API key that was used for this provider's initialization attempt
			apiKeyForProvider, keyExists := providersToInit[providerNameInState]
			if !keyExists {
				r.logger.Error().Str("provider", providerNameInState).Msg("performFullInitialization: API key not found in providersToInit map for a provider marked ready. This is a bug.")
				continue
			}

			var providerInstance Provider
			switch providerNameInState {
			case "openai":
				providerInstance = NewOpenAIProvider(apiKeyForProvider)
			case "openrouter": // This is the master OpenRouter
				masterOpenRouterProvider := NewOpenRouterProvider(apiKeyForProvider)
				if masterOpenRouterProvider != nil {
					// Create the free provider variant
					freeProvider := NewOpenRouterFreeProvider(masterOpenRouterProvider)
					if freeProvider != nil {
						r.client.RegisterProvider(freeProvider)
						actualRegisteredProviders = append(actualRegisteredProviders, freeProvider.GetName())
						r.logger.Trace().Str("provider_variant", freeProvider.GetName()).Msg("Registered OpenRouter Free variant with client.")
					}
					// Create the paid provider variant
					paidProvider := NewOpenRouterPaidProvider(masterOpenRouterProvider)
					if paidProvider != nil {
						r.client.RegisterProvider(paidProvider) // Name will be "openrouter"
						actualRegisteredProviders = append(actualRegisteredProviders, paidProvider.GetName())
						r.logger.Trace().Str("provider_variant", paidProvider.GetName()).Msg("Registered OpenRouter Paid/Standard variant with client.")
					}
					// If at least one variant was created, count the master as "ready" for readyProvidersCount
					if freeProvider != nil || paidProvider != nil {
						readyProvidersCount++
					}
				}
				continue // Skip direct registration of master, variants are registered
			case "google":
				providerInstance = NewGoogleProvider(apiKeyForProvider)
			default:
				r.logger.Warn().Str("provider_name", providerNameInState).Msg("performFullInitialization: Unknown provider type during client population.")
				continue
			}

			if providerInstance != nil {
				r.client.RegisterProvider(providerInstance)
				actualRegisteredProviders = append(actualRegisteredProviders, providerInstance.GetName())
				readyProvidersCount++ // Increment only if it's not the OpenRouter master
				r.logger.Trace().Str("provider", providerInstance.GetName()).Msg("Registered provider with client.")
			}
		}
	}
	r.logger.Debug().Int("ready_providers_count", readyProvidersCount).Strs("registered_to_client", actualRegisteredProviders).Msg("performFullInitialization: Client populated with ready providers.")

	if readyProvidersCount > 0 {
		defaultSet := false
		preferredOrder := []string{"openrouter-free", "openai", "openrouter", "google"}
		for _, name := range preferredOrder {
			if _, ok := r.client.GetProvider(name); ok {
				r.client.SetDefaultProvider(name)
				r.logger.Info().Str("provider", name).Msg("performFullInitialization: Set as default LLM provider on client.")
				defaultSet = true
				break
			}
		}
		if !defaultSet && len(r.client.ListProviders()) > 0 {
			firstProviderName := r.client.ListProviders()[0].GetName()
			r.client.SetDefaultProvider(firstProviderName)
			r.logger.Info().Str("provider", firstProviderName).Msg("performFullInitialization: Set first available provider as default on client.")
		}
	}

	if readyProvidersCount > 0 {
		r.globalState = GSReady
		r.logger.Info().Int("ready_providers", readyProvidersCount).Msg("performFullInitialization: LLM registry is now Ready.")
	} else if len(providersToInit) > 0 { // If attempts were made but none succeeded
		r.globalState = GSError
		r.logger.Warn().Msg("performFullInitialization: No LLM providers successfully initialized. Registry state is Error.")
	} else { // No providers were configured to attempt
		r.globalState = GSReady // Ready, but empty
		r.logger.Info().Msg("performFullInitialization: No LLM providers configured. Registry initialization phase complete (effectively Ready but empty).")
	}

	r.notifyStateChange("All provider initialization attempts complete.", "")
	r.signalReady()
	r.logger.Trace().Msg("performFullInitialization: Finished.")
}

// initializeSingleProvider initializes a single provider
func (r *Registry) initializeSingleProvider(providerName, apiKey string, wg *sync.WaitGroup) {
	defer wg.Done()
	r.logger.Trace().Str("provider", providerName).Msg("initializeSingleProvider: Starting.")

	r.updateProviderState(providerName, "initializing_models", nil, nil, fmt.Sprintf("Provider %s: Attempting to initialize and fetch models.", providerName))

	var providerInstance Provider
	var errInstantiate error

	switch providerName {
	case "openai":
		providerInstance = NewOpenAIProvider(apiKey)
	case "openrouter": // This is the master OpenRouter provider
		providerInstance = NewOpenRouterProvider(apiKey)
	case "google":
		providerInstance = NewGoogleProvider(apiKey)
	default:
		errInstantiate = fmt.Errorf("unknown provider type: %s", providerName)
	}

	if errInstantiate != nil || providerInstance == nil {
		finalErr := errInstantiate
		if finalErr == nil {
			finalErr = ErrProviderInitFailed
		}
		r.logger.Error().Err(finalErr).Str("provider", providerName).Msg("initializeSingleProvider: Failed to instantiate provider.")
		r.updateProviderState(providerName, "error", finalErr, nil, fmt.Sprintf("Provider %s: Instantiation failed.", providerName))
		return
	}
	r.logger.Trace().Str("provider", providerName).Msg("initializeSingleProvider: Provider instance created.")

	// Create a context with timeout for model fetching for this specific provider
	modelFetchCtx, modelFetchCancel := context.WithTimeout(context.Background(), 60*time.Second) // Increased timeout for model fetching
	defer modelFetchCancel()

	r.logger.Trace().Str("provider", providerName).Msg("initializeSingleProvider: Calling GetAvailableModels...")
	models := providerInstance.GetAvailableModels(modelFetchCtx) // Pass the context

	if modelFetchCtx.Err() != nil {
		// Context error (e.g., timeout)
		errMsg := fmt.Sprintf("Provider %s: Model fetching timed out or was canceled.", providerName)
		r.logger.Error().Err(modelFetchCtx.Err()).Str("provider", providerName).Msg("initializeSingleProvider: Model fetching context error.")
		r.updateProviderState(providerName, "error", fmt.Errorf("%w: %s", ErrContextCanceled, modelFetchCtx.Err().Error()), nil, errMsg)
		return
	}

	// Check if models is nil, which might indicate an error during GetAvailableModels not related to context
	if models == nil {
		errMsg := fmt.Sprintf("Provider %s: Failed to fetch models (returned nil, check provider logs).", providerName)
		r.logger.Warn().Str("provider", providerName).Msg("initializeSingleProvider: GetAvailableModels returned nil, assuming fetch error.")
		// Check if an error was already set by GetAvailableModels (if it updates providerStates directly, which it shouldn't)
		r.mu.RLock()
		existingState := r.providerStates[providerName]
		r.mu.RUnlock()
		if existingState.Status != "error" { // Avoid overwriting a more specific error
			r.updateProviderState(providerName, "error", ErrProviderModelsFailed, nil, errMsg)
		}
		return
	}

	successMsg := fmt.Sprintf("Provider %s: Ready with %d models.", providerName, len(models))
	r.logger.Info().Str("provider", providerName).Int("models_count", len(models)).Msg("initializeSingleProvider: Models fetched successfully.")
	r.updateProviderState(providerName, "ready", nil, models, successMsg) // Or "models_loaded" if there's another step
}

// updateProviderState updates a provider's state and sends notification
func (r *Registry) updateProviderState(providerName, status string, err error, models []ModelInfo, message string) {
	r.mu.Lock()
	r.logger.Trace().Str("provider", providerName).Str("new_status", status).Msg("updateProviderState: Acquired mutex.")

	state := ProviderState{
		Status:      status,
		Error:       err,
		LastUpdated: time.Now(),
		Models:      models,
	}
	// Preserve existing models if new models slice is nil and status isn't error
	if models == nil && status != "error" {
		if existing, ok := r.providerStates[providerName]; ok {
			state.Models = existing.Models
		}
	}

	r.providerStates[providerName] = state
	r.mu.Unlock()
	r.logger.Trace().Str("provider", providerName).Str("new_status", status).Msg("updateProviderState: Released mutex.")

	r.notifyStateChange(message, providerName)
}

// setGlobalStateAndNotify updates the global state and sends notification
func (r *Registry) setGlobalStateAndNotify(state GlobalServiceState, message string, err error) {
	r.mu.Lock()
	r.logger.Trace().Str("new_global_state", state.String()).Msg("setGlobalStateAndNotify: Acquired mutex.")
	r.globalState = state
	if err != nil { // If an error is passed, ensure global state reflects an error if not already.
		if r.globalState != GSError { // Don't override if already GSError
			r.logger.Error().Err(err).Msg("setGlobalStateAndNotify: Global error occurred, setting global state to GSError.")
			r.globalState = GSError
			if message == "" || !strings.Contains(message, "error") { // Append error if message isn't already error-related
				message = fmt.Sprintf("%s (Error: %s)", message, err.Error())
			}
		}
	}
	r.mu.Unlock()
	r.logger.Trace().Str("new_global_state", r.globalState.String()).Msg("setGlobalStateAndNotify: Released mutex.")

	r.notifyStateChange(message, "")
}

// notifyStateChange sends a notification about state changes
func (r *Registry) notifyStateChange(message, updatedProviderName string) {
	r.mu.RLock()
	r.logger.Trace().Str("message", message).Str("updated_provider", updatedProviderName).Msg("notifyStateChange: Acquired RLock for snapshot.")

	statesCopy := make(map[string]ProviderState, len(r.providerStates))
	for name, state := range r.providerStates {
		statesCopy[name] = state
	}

	stateChange := StateChange{
		Timestamp:              time.Now(),
		GlobalState:           r.globalState,
		UpdatedProviderName:   updatedProviderName,
		ProviderStatesSnapshot: statesCopy,
		Message:               message,
	}
	currentGlobalStateStr := r.globalState.String() // Capture for logging outside lock
	r.mu.RUnlock()
	r.logger.Trace().Str("message", message).Str("updated_provider", updatedProviderName).Msg("notifyStateChange: Released RLock for snapshot.")

	r.logger.Debug().
		Str("global_state", currentGlobalStateStr).
		Str("updated_provider", updatedProviderName).
		Str("message", message).
		Int("subscriber_map_len", len(r.subscribers)).
		Msg("Broadcasting state change.")

	if r.stateChangeNotifier != nil {
		r.logger.Trace().Msg("notifyStateChange: Calling stateChangeNotifier (e.g., Wails EventEmit).")
		r.stateChangeNotifier(stateChange) // This is the func passed to NewRegistry
		r.logger.Trace().Msg("notifyStateChange: stateChangeNotifier call returned.")
	}

	r.subscribersMu.RLock()
	r.logger.Trace().Int("go_subscriber_count", len(r.subscribers)).Msg("notifyStateChange: Acquired subscribers RLock.")
	activeSubscribersNotified := 0
	for ch := range r.subscribers { // Iterate over keys (channels)
		// Non-blocking send
		select {
		case ch <- stateChange:
			r.logger.Trace().Msg("notifyStateChange: Sent stateChange to a Go subscriber channel.")
			activeSubscribersNotified++
		default:
			r.logger.Warn().Msg("notifyStateChange: Failed to send state change to a Go subscriber (channel full or closed).")
		}
	}
	r.subscribersMu.RUnlock()
	r.logger.Trace().Int("active_subscribers_notified", activeSubscribersNotified).Msg("notifyStateChange: Released subscribers RLock. Finished notifying Go subscribers.")
}

// signalReady closes the readySignalChan to unblock waiters
func (r *Registry) signalReady() {
	r.mu.Lock()
	r.logger.Trace().Msg("signalReady: Acquired mutex.")
	defer func() {
		r.mu.Unlock()
		r.logger.Trace().Msg("signalReady: Released mutex.")
	}()

	select {
	case <-r.readySignalChan:
		r.logger.Trace().Msg("signalReady: readySignalChan was already closed.")
	default:
		close(r.readySignalChan)
		r.logger.Debug().Msg("signalReady: Closed readySignalChan.")
	}

	r.readySignalChan = make(chan struct{}) // Re-create for next cycle
	r.logger.Trace().Msg("signalReady: Re-created readySignalChan for next cycle.")
}

// Public Methods

// TriggerUpdate sends a configuration update to the background worker
func (r *Registry) TriggerUpdate(newSettings config.Settings) {
	r.logger.Debug().Msg("TriggerUpdate: Received request to update settings.")
	r.mu.RLock()
	isShut := r.isShutdown
	r.mu.RUnlock()
	if isShut {
		r.logger.Warn().Msg("TriggerUpdate: Registry is shut down. Update ignored.")
		return
	}

	select {
	case r.updateTriggerChan <- newSettings:
		r.logger.Info().Msg("TriggerUpdate: New settings sent to updateTriggerChan.")
	default:
		r.logger.Warn().Msg("TriggerUpdate: updateTriggerChan is full. Update might be delayed or skipped if not processed quickly.")
	}
}

// Shutdown stops the background worker and cleans up
func (r *Registry) Shutdown() {
	r.mu.Lock()
	r.logger.Trace().Msg("Shutdown: Acquired main mutex.")
	if r.isShutdown {
		r.mu.Unlock()
		r.logger.Info().Msg("Shutdown: Registry already shut down or in process.")
		return
	}
	r.isShutdown = true
	r.globalState = GSError // Or a new GSShutdown state
	shutdownMessage := "Registry shutting down"
	r.mu.Unlock()
	r.logger.Trace().Msg("Shutdown: Released main mutex. Marked as shutdown.")

	r.logger.Info().Msg("LLM Registry: Initiating shutdown sequence.")
	r.notifyStateChange(shutdownMessage, "")

	// Close shutdownChan to signal worker. Use select for non-blocking close.
	select {
	case <-r.shutdownChan: // Already closed
		r.logger.Trace().Msg("Shutdown: shutdownChan was already closed.")
	default:
		close(r.shutdownChan)
		r.logger.Trace().Msg("Shutdown: Closed shutdownChan to signal worker.")
	}

	r.logger.Trace().Msg("Shutdown: Waiting for background worker to complete...")
	r.backgroundWorkerWG.Wait()
	r.logger.Debug().Msg("Shutdown: Background worker finished.")

	r.subscribersMu.Lock()
	r.logger.Trace().Int("subscriber_count", len(r.subscribers)).Msg("Shutdown: Acquired subscribers mutex for closing channels.")
	for ch := range r.subscribers {
		select {
		case <-ch:
		default:
			close(ch)
			r.logger.Trace().Msg("Shutdown: Closed a subscriber channel.")
		}
	}
	r.subscribers = make(map[chan StateChange]bool) // Clear the map
	r.subscribersMu.Unlock()
	r.logger.Trace().Msg("Shutdown: Released subscribers mutex. All subscriber channels closed and map cleared.")

	r.mu.Lock()
	if r.readySignalChan != nil {
		select {
		case <-r.readySignalChan:
		default:
			close(r.readySignalChan)
			r.logger.Trace().Msg("Shutdown: Closed readySignalChan.")
		}
		r.readySignalChan = nil // Set to nil after closing
	}
	// Close updateTriggerChan as well, as the worker consuming from it is stopping.
	if r.updateTriggerChan != nil {
		// Check if already closed (though select on send is usually enough)
		// For a buffered channel, closing is safe if no more sends will occur.
		close(r.updateTriggerChan)
		r.updateTriggerChan = nil
		r.logger.Trace().Msg("Shutdown: Closed updateTriggerChan.")
	}
	r.mu.Unlock()

	r.logger.Info().Msg("LLM Registry: Shutdown complete.")
}

// SubscribeToStateChanges returns a new channel that will receive state change events.
func (r *Registry) SubscribeToStateChanges() chan StateChange { // Return bi-directional for map key
	ch := make(chan StateChange, 10) // Buffered channel

	r.subscribersMu.Lock()
	r.logger.Trace().Msg("SubscribeToStateChanges: Acquired subscribers mutex.")
	if r.isShutdown { // Don't allow new subscriptions if shutting down/shut down
		r.subscribersMu.Unlock()
		r.logger.Warn().Msg("SubscribeToStateChanges: Attempted to subscribe to a shutdown registry. Returning closed channel.")
		close(ch) // Return an immediately closed channel
		return ch
	}
	r.subscribers[ch] = true
	r.subscribersMu.Unlock()
	r.logger.Trace().Msg("SubscribeToStateChanges: Released subscribers mutex. New subscriber added.")

	// Send the current state immediately in a non-blocking way
	go func() {
		initialState := r.GetCurrentStateSnapshot()
		r.logger.Trace().Str("initial_global_state", initialState.GlobalState.String()).Msg("SubscribeToStateChanges: Sending initial state to new subscriber.")
		select {
		case ch <- initialState:
			r.logger.Trace().Msg("SubscribeToStateChanges: Successfully sent initial state to new subscriber.")
		case <-time.After(1 * time.Second): // Timeout for sending initial state
			r.logger.Warn().Msg("SubscribeToStateChanges: Timeout sending initial state to new subscriber (subscriber not reading).")
			// If timed out, the subscriber might be stuck or slow. Consider unsubscribing it.
			// For now, just log. If this happens, the subscriber might miss the initial state.
		}
	}()

	return ch
}

// UnsubscribeFromStateChanges removes a subscriber channel.
func (r *Registry) UnsubscribeFromStateChanges(ch chan StateChange) { // Param is bi-directional
	r.subscribersMu.Lock()
	r.logger.Trace().Msg("UnsubscribeFromStateChanges: Acquired subscribers mutex.")
	if _, ok := r.subscribers[ch]; ok {
		delete(r.subscribers, ch) // Remove from map first
		// Now safely close the channel if it wasn't already closed by shutdown
		select {
		case <-ch: // Already closed
			r.logger.Trace().Msg("UnsubscribeFromStateChanges: Channel was already closed.")
		default:
			close(ch)
			r.logger.Debug().Msg("UnsubscribeFromStateChanges: Subscriber removed and channel closed.")
		}
	} else {
		r.logger.Warn().Msg("UnsubscribeFromStateChanges: Attempted to unsubscribe a non-existent or already removed channel.")
	}
	r.subscribersMu.Unlock()
	r.logger.Trace().Msg("UnsubscribeFromStateChanges: Released subscribers mutex.")
}

// GetCurrentStateSnapshot returns the current state of the registry
func (r *Registry) GetCurrentStateSnapshot() StateChange {
	r.mu.RLock()
	r.logger.Trace().Msg("GetCurrentStateSnapshot: Acquired RLock.")
	defer func() {
		r.mu.RUnlock()
		r.logger.Trace().Msg("GetCurrentStateSnapshot: Released RLock.")
	}()

	statesCopy := make(map[string]ProviderState, len(r.providerStates))
	for name, state := range r.providerStates {
		statesCopy[name] = state
	}

	return StateChange{
		Timestamp:              time.Now(),
		GlobalState:           r.globalState,
		ProviderStatesSnapshot: statesCopy,
		Message:               fmt.Sprintf("Current global state: %s", r.globalState.String()),
	}
}

// WaitForReady blocks until the registry's initial processing is complete or the context is done
func (r *Registry) WaitForReady(ctx context.Context) error {
	r.logger.Trace().Msg("WaitForReady: Called.")
	r.mu.RLock()
	r.logger.Trace().Msg("WaitForReady: Acquired RLock for initial state check.")
	currentReadyChan := r.readySignalChan
	currentState := r.globalState
	isInit := r.initialized
	isShut := r.isShutdown
	r.mu.RUnlock()
	r.logger.Trace().Str("current_state", currentState.String()).Bool("initialized", isInit).Bool("shutdown", isShut).Msg("WaitForReady: Released RLock. Initial state checked.")

	if isShut {
		r.logger.Warn().Msg("WaitForReady: Registry is shut down or shutting down.")
		return ErrRegistryShutdown
	}
	if !isInit {
		r.logger.Warn().Msg("WaitForReady: Registry Start() has not been called.")
		return ErrRegistryNotInit
	}

	if currentState == GSReady {
		r.logger.Trace().Msg("WaitForReady: GlobalState is already Ready.")
		return nil
	}
	if currentState == GSError {
		r.logger.Warn().Msg("WaitForReady: GlobalState is Error after initialization attempt.")
		return ErrRegistryNotReady
	}

	r.logger.Debug().Msg("WaitForReady: GlobalState is not yet Ready. Waiting on readySignalChan or context.")
	select {
	case <-currentReadyChan: // This channel is closed when an init cycle completes
		r.mu.RLock()
		finalState := r.globalState
		r.mu.RUnlock()
		r.logger.Debug().Str("final_state_after_signal", finalState.String()).Msg("WaitForReady: Received signal from readySignalChan.")
		if finalState == GSReady {
			return nil
		}
		r.logger.Warn().Str("final_state", finalState.String()).Msg("WaitForReady: Signaled but final state is not GSReady.")
		return ErrRegistryNotReady
	case <-ctx.Done():
		r.logger.Warn().Err(ctx.Err()).Msg("WaitForReady: Context canceled or timed out while waiting for registry readiness.")
		return fmt.Errorf("%w: %s", ErrContextCanceled, ctx.Err().Error())
	}
}

// GetClient returns the client instance if the registry is ready
func (r *Registry) GetClient() (*Client, error) {
	r.mu.RLock()
	r.logger.Trace().Msg("GetClient: Acquired RLock.")
	defer func() {
		r.mu.RUnlock()
		r.logger.Trace().Msg("GetClient: Released RLock.")
	}()

	if r.isShutdown {
		r.logger.Warn().Msg("GetClient: Registry is shut down.")
		return nil, ErrRegistryShutdown
	}
	if r.globalState != GSReady {
		r.logger.Warn().Str("global_state", r.globalState.String()).Msg("GetClient: Registry not ready.")
		return nil, ErrRegistryNotReady
	}
	if r.client == nil { // Should not happen if GSReady
		r.logger.Error().Msg("GetClient: Registry is GSReady but client is nil. This indicates an internal bug.")
		return nil, errors.New("llm registry: internal error - client is nil despite ready state")
	}
	return r.client, nil
}
