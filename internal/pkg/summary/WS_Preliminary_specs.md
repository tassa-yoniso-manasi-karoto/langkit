# Summary Package Specification - WebSocket Architecture

## Overview

The `summary` package provides a flexible framework for generating summaries of content using Large Language Models (LLMs). It serves as an abstraction layer over the underlying LLM providers, offering a consistent interface for summarization operations while hiding the complexity of provider-specific implementations.

The package's primary purpose is to support the "Condensed Audio" feature, which extracts audio segments from videos based on subtitle timecodes and merges them into a single file, with optional summaries embedded as metadata.

This specification has been revised to use WebSocket communication instead of Wails events for cross-runtime reactive state management, addressing reliability issues encountered with the Wails event bridge.

## Implementation Details

The implementation spans multiple packages and files, creating a cohesive system for asynchronous LLM management and summary generation.

## WebSocket Architecture

### Overview

The WebSocket architecture replaces the unreliable Wails event system with a robust, standard WebSocket connection for state propagation between the backend Go application and the frontend JavaScript/Svelte components.

### Key Benefits

1. **Reliability**: WebSocket provides guaranteed message delivery with built-in reconnection
2. **Debuggability**: WebSocket traffic is visible in browser DevTools
3. **Standards-based**: Uses web standards instead of framework-specific solutions
4. **Bidirectional**: Allows future expansion for frontend→backend communication

### WebSocket Server Design

```go
// internal/gui/websocket_server.go
type WebSocketServer struct {
    upgrader  websocket.Upgrader
    clients   map[*websocket.Conn]bool
    clientsMu sync.RWMutex
    port      int
    logger    zerolog.Logger
}

// Message types
type WSMessage struct {
    Type    string      `json:"type"`
    Payload interface{} `json:"payload"`
}
```

The WebSocket server:
- Listens on a dynamically allocated port (OS chooses available port)
- Maintains a registry of connected clients
- Broadcasts state changes to all connected clients
- Handles client disconnections gracefully
- Provides JSON-encoded messages for easy frontend parsing

### Key Files and Their Roles

#### Core LLM Infrastructure (`pkg/llms/`)

1. **state_types.go**
   - Defines the core state enum `GlobalServiceState` with values like `GSUninitialized`, `GSInitializing`, `GSReady`
   - Contains `ProviderState` struct tracking individual provider status and models
   - Implements `StateChange` event payload for state notifications
   - These types are JSON-serialized for WebSocket transmission

2. **registry_async.go**
   - Houses the `Registry` struct - the centerpiece of the asynchronous architecture
   - Contains critical methods like `Start()`, `backgroundWorker()`, `performFullInitialization()` 
   - **Modified**: `stateChangeNotifier` now sends to WebSocket instead of Wails events
   - Manages provider lifecycle with methods like `initializeSingleProvider()`
   - Continues to support internal Go subscribers via channels

3. **interfaces.go**
   - Defines the `Provider` interface with the crucial `GetAvailableModels(ctx context.Context)` method
   - Sets the contract that all LLM providers must implement

4. **registry.go**
   - Acts as a compatibility layer between old synchronous and new asynchronous systems
   - Contains `GetDefaultClient()` that redirects to the registry's managed client
   - Handles API key loading through `LoadAPIKeysFromSettings()`

5. **client.go**
   - Implements the `Client` struct that holds the ready LLM providers
   - Contains registry-populated method `ListProviders()` used by summary service

#### Summary Package (`internal/pkg/summary/`)

1. **manager.go**
   - Orchestrates integration with the registry through `Initialize()` and `listenToLLMRegistryChanges()`
   - Consumer side of event system, processing `StateChange` events
   - Contains `handleStateChange()` which updates providers when registry is ready
   - Implements the singleton pattern via `GetDefaultService()`

2. **service.go**
   - Provides thread-safe provider management with `RegisterProvider()`, `ClearProviders()`
   - Exposes the primary API method `GenerateSummary()` called by application code
   - Contains query methods like `GetModelsForProvider()` used by the GUI

3. **provider.go**
   - Defines the `Provider` interface for summary generation
   - Implements `BaseProvider` and `DefaultSummaryProvider` which wrap LLM providers
   - Contains `GetSupportedModels()` which uses the updated context-aware LLM interface

4. **options.go**
   - Defines the `Options` struct controlling summarization parameters
   - Used to configure the summary requests sent to providers

#### Application Integration (`internal/`)

1. **core/init_llm.go**
   - Entry point for LLM system initialization via `InitLLM()`
   - **Modified**: Sets up WebSocket server and passes it to registry for state notifications
   - Creates and starts the registry with `registry.Start()`
   - Links summary system to registry with `summary.Initialize()`
   - Passes WebSocket server reference to registry's notifier function

2. **gui/app.go**
   - GUI-side consumer of the registry state via `llmRegistry` field
   - **New**: Creates and manages WebSocket server in `startup()`
   - **New**: Exposes `GetWebSocketPort()` method for frontend to discover connection
   - Implements Wails-exposed methods like `GetAvailableSummaryProviders()`
   - Handles settings updates with `SaveSettings()` which triggers registry updates
   - Manages LLM resource lifecycle and WebSocket server shutdown in `beforeClose()`

3. **gui/websocket_server.go** (New File)
   - Implements the WebSocket server using gorilla/websocket
   - Handles WebSocket upgrade, client management, and message broadcasting
   - Provides `BroadcastStateChange()` method called by registry notifier
   - Manages automatic reconnection and error recovery

4. **gui/frontend/src/components/FeatureSelector.svelte**
   - **Modified**: Connects to WebSocket server instead of Wails events
   - Establishes WebSocket connection on mount using port from `GetWebSocketPort()`
   - Implements automatic reconnection logic
   - Parses WebSocket messages and updates UI based on state changes
   - Shows loading indicators during initialization

## Architecture

### Core Components

```
                     ┌───────────────────┐
┌─────────────┐      │                   │      ┌─────────────────┐
│             │      │  Summary Service  │      │                 │
│    Client   │◄─────┤                   ├─────►│    Providers    │
│  (Consumer) │      │                   │      │                 │
└─────────────┘      └────────┬──────────┘      └─────────────────┘
                              │                         ▲
                              │                         │
                     ┌────────▼──────────┐             │
                     │                   │             │
                     │  Registry Client  │─────────────┘
                     │                   │
                     └───────┬───────────┘
                             │
                             │
                     ┌───────▼───────────┐
                     │                   │
                     │   LLM Registry    │
                     │                   │
                     └───────────────────┘
```

### Key Components

1. **Service**: Central orchestrator that manages providers and handles summary generation requests
   - Thread-safe provider registry
   - Bridge between client requests and provider implementations
   - Supports provider selection, model selection, and summary options

2. **Provider Interface**: Abstraction for different summarization implementations
   - Common methods for all providers: Generate, GetName, GetSupportedModels
   - BaseProvider for shared functionality
   - DefaultSummaryProvider for wrapping LLM providers

3. **Options**: Configuration parameters for summary generation
   - Controls summary length, language, temperature, etc.
   - Supports custom prompts for fine-grained control

4. **Manager**: Component responsible for lifecycle management
   - Registry integration
   - Provider initialization and updates
   - State tracking and event handling

## Asynchronous Registry Integration

The summary package features an innovative asynchronous registry-based initialization system that addresses several key challenges:

### Non-Blocking Initialization

The system uses an event-driven architecture to initialize LLM providers without blocking application startup. This is critical for maintaining GUI responsiveness.

- **Registry Start Process**:
  1. Application starts LLM Registry in background
  2. Registry begins provider initialization process
  3. Application continues startup without waiting
  4. Summary service receives state updates as they occur

### State Management

The system maintains a clear state machine for tracking provider status:

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│                  │     │                  │     │                  │
│  Uninitialized   │────►│   Initializing   │────►│      Ready       │
│                  │     │                  │     │                  │
└──────────────────┘     └──────────────┬───┘     └──────────────────┘
                                        │                  ▲
                                        │                  │
                                        ▼                  │
                                ┌──────────────────┐      │
                                │                  │      │
                                │      Error       │      │
                                │                  │      │
                                └──────────────────┘      │
                                        ▲                 │
                                        │                 │
                                        ▼                 │
                                ┌──────────────────┐      │
                                │                  │      │
                                │     Updating     │──────┘
                                │                  │
                                └──────────────────┘
```

- **State Propagation**:
  - Registry emits state change events
  - Summary service subscribes to these events
  - Service monitors provider status and availability
  - Client interfaces gracefully handle transitional states

### Provider Management

The system dynamically adapts to provider availability:

1. **Initial Setup**: Service starts with empty or basic providers
2. **Dynamic Updates**: As providers become ready, service updates its internal registry
3. **Client Awareness**: Client applications can query provider status
4. **UI Responsiveness**: Interface elements can reflect provider availability

### Event-Driven Approach

The integration uses a subscription model for state changes:

1. **Event Subscription**: Summary manager subscribes to registry events
2. **Event Processing**: Manager processes events based on state transitions
3. **Provider Updates**: Summary providers are updated when registry signals readiness
4. **Client Updates**: Service maintains provider consistency with the registry

## Implementation Details

### Internal Mechanisms

#### Registry Initialization Process

The registry initialization flow implements a sophisticated state machine:

1. **Registry Creation**: The registry is instantiated in `core.InitLLM()` with configuration and a notification callback
2. **Start Sequence**: The `registry.Start()` method transitions state to `GSInitializing` and launches the background worker
3. **Provider Discovery**: The worker examines available API keys in `performFullInitialization()`
4. **Concurrent Initialization**: For each eligible provider, a goroutine is started in `initializeSingleProvider()`
5. **Model Fetching**: Each provider makes API calls to load available models with proper error handling
6. **Client Population**: Only successfully initialized providers are registered with the client
7. **State Transition**: The registry moves to `GSReady` state and emits a notification
8. **Summary Integration**: The summary service responds to this state change by rebuilding its providers

Key synchronization points:
- `backgroundWorkerWG`: Ensures clean shutdown by tracking background goroutines
- `mu` mutex: Protects access to shared state within the registry
- `readySignalChan`: Allows synchronous waiting for registry readiness
- `notifyStateChange()`: Thread-safe event emission with state snapshots

#### Thread Safety Implementation

The implementation uses multiple defensive mechanisms:

1. **Double-checked locking** in the registry:
   ```go
   r.mu.RLock() // Read lock first for efficiency
   if len(r.models) > 0 {
       r.mu.RUnlock()
       return r.models
   }
   r.mu.RUnlock()
   
   r.mu.Lock() // Full lock only when necessary
   defer r.mu.Unlock()
   // Check again after acquiring full lock
   if len(r.models) > 0 {
       return r.models
   }
   ```

2. **Race-free event dispatch** in the subscription system:
   - Snapshot state while holding read lock
   - Release lock before potentially slow delivery to subscribers
   - Use select with default case for non-blocking channel sends

3. **Atomic updates** in the summary service:
   - Lock during provider map updates
   - Clear and rebuild provider set in single operation
   - Update client reference atomically

### State Handling

- The summary service handles different registry states, including:
  - Uninitialized: Registry not yet started
  - Initializing: Providers are being initialized
  - Ready: Providers are available and operational
  - Error: Critical failure in the registry
  - Updating: Configuration changed, re-initialization in progress

### Thread Safety

- Multiple synchronization mechanisms ensure safe concurrent access:
  - Registry-level locks protect global state
  - Service-level locks protect provider management
  - Provider map with controlled access via mutex
  - State snapshot pattern for safe state reporting

### Error Handling

- The system employs a graceful degradation approach:
  - Returns empty provider lists rather than errors when initializing
  - Provides clear status information for UI error handling
  - Automatically attempts recovery when registry state changes
  - Logs detailed error information for diagnostics

## Integration Points

### GUI Integration

The system integrates with the GUI through WebSocket communication:

#### WebSocket Protocol

1. **Connection Establishment**
   - Frontend calls `GetWebSocketPort()` to discover the server port
   - Connects to `ws://localhost:{port}/ws`
   - Server upgrades HTTP connection to WebSocket

2. **Message Format**
   ```typescript
   interface WSMessage {
       type: string;      // Message type (e.g., "statechange", "error")
       payload: any;      // Type-specific payload
   }
   ```

3. **State Change Messages**
   ```typescript
   interface StateChangeMessage {
       type: "statechange";
       payload: {
           timestamp: string;
           globalState: string;
           updatedProviderName?: string;
           providerStatesSnapshot: Record<string, ProviderState>;
           message?: string;
       };
   }
   ```

4. **Connection Management**
   - Automatic reconnection with exponential backoff
   - Heartbeat/ping-pong for connection health
   - Graceful degradation when WebSocket unavailable

The WebSocket integration provides:
- Real-time state updates without polling
- Reliable message delivery with acknowledgments
- Network-level debugging through browser DevTools
- Support for multiple concurrent clients
- Extensibility for future bidirectional communication

#### Frontend State Management

The frontend implements a reactive state management system:

1. **LLM State Store (`llmStateStore.ts`)**
   - Svelte store for tracking LLM system state and provider status
   - Provides reactive updates to all subscribed components
   - Defines interfaces for LLM state types (`LLMStateChange`, `LLMProviderState`)
   - Offers helper functions like `isProviderReady()` for component use

2. **Visual Feedback System**
   - Status badges indicating global LLM state (initializing, ready, error)
   - Provider-specific loading indicators during model retrieval
   - Conditional display of feature options based on provider readiness
   - Error messages with provider-specific context
   - Disabling of interactive elements until providers are ready

#### Data Flow Between Components

1. **Initialization Flow**
   - `app.startup()` creates WebSocket server and gets dynamic port
   - `core.InitLLM()` creates the registry with WebSocket notifier function
   - The registry starts background initialization in `backgroundWorker()`
   - Svelte components mount and call `window.go.gui.App.GetWebSocketPort()`
   - Frontend establishes WebSocket connection to `ws://localhost:{port}/ws`
   - App calls `GetInitialLLMState()` for immediate state if needed
   - WebSocket messages update `llmStateStore` which triggers reactive UI updates
   - Registry state changes are broadcast to all connected WebSocket clients

2. **Provider Discovery**
   - When a GUI component needs provider information, it calls `GetAvailableSummaryProviders()`
   - Before calling, component checks current state via `llmStateStore`
   - The backend method checks the registry state using `llmRegistry.GetCurrentStateSnapshot()`
   - If the registry isn't ready, it returns a response with `status` indicating initialization
   - If ready, it queries `summary.GetDefaultService().ListProviders()` for available providers
   - UI elements update to reflect provider status (ready, loading, or error)

3. **Configuration Updates**
   - When settings change, `app.SaveSettings()` is called with new settings
   - This triggers `llmRegistry.TriggerUpdate(settings)` to start re-initialization
   - A signal is sent through the `updateTriggerChan` to the background worker
   - Background worker calls `performFullInitialization()` with new settings
   - New state is propagated through events to update the GUI

4. **Cleanup Process**
   - When the application closes, `app.beforeClose()` is called
   - This calls `core.ShutdownLLM()` which signals registry shutdown
   - Registry stops background worker and cleans up resources
   - Summary service clears providers and resets for potential future initialization

### API Interface

Client code interacts with the summary system through these main entry points:

- `GetAvailableSummaryProviders()`: Lists available providers
- `GetAvailableSummaryModels(provider)`: Lists models for a provider
- `GenerateSummary(text, language, options)`: Creates a summary with selected options
- `GetWebSocketPort()`: Returns the WebSocket server port for frontend connection

## WebSocket Implementation Guide

### Backend Implementation

1. **WebSocket Server Setup (gui/websocket_server.go)**
   ```go
   func NewWebSocketServer(logger zerolog.Logger) (*WebSocketServer, error) {
       listener, err := net.Listen("tcp", "localhost:0")
       if err != nil {
           return nil, err
       }
       
       port := listener.Addr().(*net.TCPAddr).Port
       
       ws := &WebSocketServer{
           upgrader: websocket.Upgrader{
               CheckOrigin: func(r *http.Request) bool {
                   return true // Allow connections from Wails webview
               },
           },
           clients: make(map[*websocket.Conn]bool),
           port:    port,
           logger:  logger,
       }
       
       http.HandleFunc("/ws", ws.handleWebSocket)
       go http.Serve(listener, nil)
       
       return ws, nil
   }
   ```

2. **Registry Integration**
   ```go
   // In core/init_llm.go
   func InitLLM(handler MessageHandler, wsServer *WebSocketServer) *llms.Registry {
       notifierFunc := func(change llms.StateChange) {
           wsServer.BroadcastStateChange(change)
       }
       
       registry := llms.NewRegistry(settings, logger, notifierFunc)
       registry.Start()
       return registry
   }
   ```

### Frontend Implementation

1. **WebSocket Connection (lib/websocket.ts)**
   ```typescript
   export class LLMWebSocket {
       private ws: WebSocket | null = null;
       private reconnectTimer: number | null = null;
       private reconnectDelay = 1000;
       
       async connect() {
           const port = await window.go.gui.App.GetWebSocketPort();
           this.ws = new WebSocket(`ws://localhost:${port}/ws`);
           
           this.ws.onmessage = (event) => {
               const message = JSON.parse(event.data);
               if (message.type === 'statechange') {
                   llmStateStore.set(message.payload);
               }
           };
           
           this.ws.onclose = () => {
               this.scheduleReconnect();
           };
       }
       
       private scheduleReconnect() {
           this.reconnectTimer = setTimeout(() => {
               this.connect();
           }, this.reconnectDelay);
           this.reconnectDelay = Math.min(this.reconnectDelay * 2, 30000);
       }
   }
   ```

2. **Component Integration**
   ```typescript
   // In FeatureSelector.svelte
   onMount(() => {
       const ws = new LLMWebSocket();
       ws.connect();
       
       onDestroy(() => {
           ws.disconnect();
       });
   });
   ```

## Best Practices

### Configuration Updates

When API keys or other configuration changes:

1. App saves settings via `app.go` method `SaveSettings(settings config.Settings)`
2. Registry receives update trigger via `llmRegistry.TriggerUpdate(settings)`
3. Registry signals reconfiguration with state `GSUpdating` 
4. Registry reinitializes providers through the same background worker
5. Summary service receives state change via its subscription
6. Service rebuilds providers by calling `handleStateChange(registry, stateChange)`
7. UI components observe state change via Wails event subscription
8. UI reflects updated availability by refreshing element states

The registry provides a sophisticated implementation for reinitialization that:

- Uses a buffered `updateTriggerChan` to queue configuration changes
- Ensures clean shutdown of previous providers before initialization
- Preserves proper state transitions (`GSUpdating` → processing → `GSReady`)
- Provides comprehensive logging of the reconfiguration process
- Detects and reports API key changes for different providers
- Maintains consistent client interface during transitions

### Error States

The system supports several types of failure modes:

- Initialization failures (API key issues)
- Network failures (API connectivity)
- Authentication failures (key validity)
- Temporary service outages (retry mechanisms)

### Extension

The architecture supports future expansion through:

- Provider interface for new summarization methods
- Registry pattern for new LLM providers
- Options structure for additional parameters
- Event system for state propagation

## Performance and Resource Considerations

### Resource Management

The implementation carefully manages resources to avoid leaks and optimize performance:

1. **Channel Management**
   - All subscriber channels are properly closed during shutdown
   - The `updateTriggerChan` is buffered to handle concurrent updates
   - Non-blocking sends prevent deadlocks in event distribution

2. **Memory Efficiency**
   - Provider states only store necessary model information
   - Events use snapshots to prevent long-term references
   - Providers are cleaned up when no longer needed

3. **Concurrency Control**
   - Background workers are properly tracked with WaitGroups
   - Read-write locks optimize for read-heavy access patterns
   - Context cancellation propagates to long-running operations

### Application Impact

The asynchronous architecture dramatically improves the application experience:

1. **Startup Performance**
   - GUI becomes responsive in milliseconds instead of seconds
   - Progress indicators replace frozen interfaces
   - Users can interact with other features while providers initialize

2. **Resilience**
   - Network issues with one provider don't affect others
   - Temporary API failures don't break the entire application
   - Configuration errors are isolated to specific providers

3. **Flexibility**
   - New LLM providers can be added without changing core architecture
   - Configuration changes take effect dynamically
   - Different provider combinations adapt to various operating conditions

## Migration from Wails Events

### Key Changes

1. **Event Emission**
   - **Before**: `runtime.EventsEmit(wailsContext, "llm:statechange", change)`
   - **After**: `wsServer.BroadcastStateChange(change)`

2. **Frontend Event Handling**
   - **Before**: `EventsOn("llm:statechange", handler)`
   - **After**: WebSocket `onmessage` handler

3. **Initial State**
   - **Before**: Component calls Wails method then subscribes to events
   - **After**: WebSocket connection sends current state on connect

4. **Error Handling**
   - **Before**: Silent event loss with no recovery
   - **After**: Automatic reconnection with exponential backoff

### Migration Steps

1. Add gorilla/websocket dependency: `go get github.com/gorilla/websocket`
2. Create `gui/websocket_server.go` with server implementation
3. Update `app.startup()` to create WebSocket server
4. Modify `core.InitLLM()` to accept WebSocket server
5. Update registry notifier to use WebSocket broadcast
6. Create frontend WebSocket client class
7. Replace `EventsOn` with WebSocket message handlers
8. Test reconnection and error scenarios

## Troubleshooting WebSocket Issues

### Common Problems and Solutions

1. **Port Already in Use**
   - Solution: Use `localhost:0` to let OS assign available port
   - Always close server properly in `app.beforeClose()`

2. **Connection Refused**
   - Check WebSocket server is running before frontend connects
   - Verify port number matches between backend and frontend
   - Check firewall/antivirus not blocking localhost connections

3. **Messages Not Received**
   - Verify JSON marshaling of state types
   - Check browser console for WebSocket errors
   - Use browser DevTools Network tab to inspect WebSocket frames

4. **Memory Leaks**
   - Ensure clients are removed from map on disconnect
   - Close all connections on server shutdown
   - Clear reconnection timers on component unmount

## Conclusion

The WebSocket-based architecture provides a significant improvement over the Wails event system, offering:

1. **Reliability**: No more lost events - WebSocket provides guaranteed delivery
2. **Debuggability**: Full visibility into communication via browser DevTools
3. **Standards Compliance**: Uses web standards instead of framework-specific APIs
4. **Performance**: Direct TCP connection with minimal overhead
5. **Flexibility**: Easy to extend for bidirectional communication
6. **Resilience**: Built-in reconnection and error recovery

The asynchronous registry architecture combined with WebSocket communication creates a robust foundation for LLM-based summarization. This implementation ensures the application remains responsive while providing reliable, real-time state updates to the frontend.

This architecture represents a mature solution that addresses the limitations discovered in the Wails event system, providing a solid foundation for current and future development needs.