# Dual UI Runtime Migration Plan

## Overview

This document outlines the design and migration plan for supporting two UI runtimes in a single Langkit binary:

1. **Wails mode** (default): Traditional GUI with WebView2/WebKit
2. **Headless server mode**: HTTP server for Qt WebEngine (Anki integration)

## Understanding Wails Internals

Based on investigation of the Wails codebase:

- **No traditional HTTP server**: Wails intercepts WebView requests at a low level using platform-specific APIs
- **Asset embedding**: Uses standard `//go:embed all:frontend/dist` directive
- **Runtime injection**: Wails injects runtime scripts (`/wails/runtime.js`, `/wails/ipc.js`) into served HTML
- **API calls**: Not HTTP - uses IPC bridge with `window.go.App.Method()` calls

Since Langkit has migrated to WebRPC for API calls, we can bypass Wails' complex asset serving and use standard Go HTTP serving in headless mode.

## Architecture Design

### Single Binary, Two Modes

```go
//go:embed all:frontend/dist
var assets embed.FS

func main() {
    if len(os.Args) > 1 && os.Args[1] == "--server" {
        runHeadlessServer() // For Qt/Anki - no Wails imports executed
    } else {
        gui.Run() // Normal Wails GUI
    }
}
```

### Headless Server Architecture

The headless server completely bypasses Wails initialization:

- **Frontend assets**: Served directly from embedded FS via Chi router
- **WebRPC API**: Existing API endpoints (already migrated from Wails)
- **WebSocket**: Existing real-time communication
- **Native dialogs**: Zenity for cross-platform file/progress dialogs
- **No WebView2**: No platform-specific UI components loaded

```
[Anki Python Add-on] 
         |
         v
[Qt WebEngine Tab] <--HTTP--> [Langkit Headless Server]
                                 - Static files (Chi)
                                 - WebRPC API (existing)
                                 - WebSocket (existing)
                                 - Zenity dialogs (native)
```

## DOM Injection for Configuration

Since the frontend needs to know API/WebSocket ports, we'll inject configuration into `index.html`:

```go
func serveIndex(w http.ResponseWriter, r *http.Request) {
    indexHTML, _ := fs.ReadFile(assets, "frontend/dist/index.html")

    config := fmt.Sprintf(`
        <script>
            window.__LANGKIT_CONFIG__ = {
                apiPort: %d,
                wsPort: %d,
                mode: "qt",
                runtime: "anki"
            };
        </script>
    `, apiPort, wsPort)

    html := strings.Replace(string(indexHTML), "</head>", config + "</head>", 1)
    w.Write([]byte(html))
}
```

Frontend will check for this config before falling back to Wails IPC methods.

## Anki Add-on Architecture

### Core Components

The Python add-on serves as a minimal wrapper with focused functionality:

1. **Process Management**
   
   - Start/stop/restart Langkit server
   - Single instance enforcement
   - Graceful shutdown on Anki exit
   - Crash detection and restart

2. **Binary Management**
   
   ```python
   class BinaryManager:
       def download_with_progress(self, progress_callback)
       def verify_checksum(self, binary_path) -> bool
       def get_installed_version() -> Optional[str]
       def check_for_updates() -> Optional[NewVersion]
   ```

3. **UI Integration**
   
   - Add Langkit tab to Anki's main interface
   - Host Qt WebEngine view pointing to Langkit server
   - Handle tab switching and lifecycle

4. **First-Run Setup**
   
   - Download binary with progress bar
   - Verify checksum
   - Test server startup
   - Configure settings

### Dialog Architecture

The existing UI abstraction layer (`internal/ui`) allows runtime-specific dialogs:

- **Wails mode**: Uses native Wails dialogs (integrated with app window)
- **Qt/headless mode**: Uses Zenity dialogs (cross-platform native dialogs)

No dialog bridge needed - the Go binary shows its own dialogs based on runtime:

```go
// Wails mode
ui.Initialize(dialogs.NewWailsFileDialog(ctx))

// Qt mode
ui.Initialize(dialogs.NewZenityFileDialog())
```

Benefits of this approach:

- Simplified architecture - no IPC for dialogs
- Native look and feel on all platforms
- Already cross-platform (Windows, macOS, Linux)
- Supports all needed dialog types (file, progress, notifications)
- Less code to maintain

## Anki UI Integration

### First-Class Tab Integration

Langkit appears as a main tab in Anki's interface:

```
[Decks] [Add] [Browse] [Stats] [Langkit] [Sync]
```

When clicked:

- Complete window takeover (no side-by-side mode needed)
- Full screen real estate for feature cards and log viewer
- Maintains Anki's visual hierarchy

### Exit Strategies

1. **Toolbar Persistence**: Keep Anki's main toolbar visible, click other tabs to exit
2. **Runtime-Specific UI**: Add subtle "← Back to Anki" in Qt mode only
3. **Keyboard**: ESC key to return to main Anki view

## Migration Requirements

### 1. Remaining Wails Dependencies

#### Startup/Shutdown Logic

Since Wails takes over program execution with `wails.Run()`, we can't use interfaces. Instead, extract common startup/shutdown logic:

```go
// Shared initialization logic
func commonStartup(ctx context.Context, apiServer *api.Server, wsServer *WebSocketServer) error {
    // Initialize LLM system
    // Start API server
    // Load settings
    // Initialize logging
    return nil
}

// Wails mode - called by Wails
func (a *App) startup(ctx context.Context) {
    commonStartup(ctx, a.apiServer, a.wsServer)
    // Additional Wails-specific initialization
}

// Headless mode - called directly
func runHeadlessServer() {
    apiServer := api.NewServer(...)
    wsServer := NewWebSocketServer(...)

    commonStartup(context.Background(), apiServer, wsServer)
    // Start Chi HTTP server
}
```

This respects how Wails actually works while maximizing code reuse.

#### GetWebSocketPort() Wails Method

Will be replaced by DOM injection - frontend reads from `window.__LANGKIT_CONFIG__`.

### 2. File Server Implementation

Using Wails' AssetHandler with Chi router:

```go
func runHeadlessServer() {
    // Create Wails asset handler (reuses all SPA routing logic)
    assetOptions := &assetserver.Options{
        Assets: assets, // Your embedded frontend
    }
    assetHandler, err := assetserver.NewAssetHandler(assetOptions, logger)
    if err != nil {
        panic(err)
    }

    // Create Chi router
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // Apply config injection middleware to index paths
    r.Get("/", configInjectionMiddleware(assetHandler))
    r.Get("/index.html", configInjectionMiddleware(assetHandler))

    // All other assets served directly by AssetHandler
    r.Handle("/*", assetHandler)

    // Note: WebRPC and WebSocket run on their own ports (existing)

    log.Println("Frontend: http://localhost:8080")
    http.ListenAndServe(":8080", r)
}
```

This approach reuses Wails' battle-tested asset serving logic while bypassing all GUI-specific code.

## Implementation Tasks

### Phase 1: Core Server Mode

- [ ] Create `runHeadlessServer()` function with Chi router
- [ ] Integrate Wails AssetHandler for asset serving
- [ ] Implement config injection middleware (ports) using httptest.ResponseRecorder
- [ ] Test embedded assets are accessible without Wails runtime

### Phase 2: Frontend Compatibility

- [ ] Update frontend API client to check `window.__LANGKIT_CONFIG__`
- [ ] Add fallback logic: DOM config → default ports
- [ ] Update WebSocket client to use injected configuration
- [ ] Add runtime-specific UI adjustments (exit button for Qt mode)

### Phase 3: Anki Add-on Development

- [ ] Create minimal Python wrapper with process management
- [ ] Implement binary downloader with progress UI
- [ ] Add checksum verification
- [ ] Implement ZenityFileDialog for Qt/headless mode
- [ ] Implement main tab integration in Anki UI

### Phase 4: Integration & Polish

- [ ] Test Windows binary with `--server` flag (no console output expected)
- [ ] Verify file dialogs work with Zenity
- [ ] Test complete user flow from Anki

### Phase 5: Distribution

- [ ] Package add-on for AnkiWeb
- [ ] Create installation guide
- [ ] Document server mode for developers

## Detailed Implementation: Reusing Wails AssetServer

### Understanding Wails Asset Architecture

Based on analysis of the Wails codebase, the asset serving is cleanly separated into layers all contained within package `asserserver` or `asserserver/webview`:

1. **`assetHandler`** - Pure HTTP handler for serving from embed.FS
2. **`AssetServer`** - Wrapper that adds Wails runtime injection
3. **Platform interceptors** - WebView-specific request handling

For headless mode, we only need the first layer.

### Key Components to Use

#### 1. AssetHandler (Core Logic)

```go
import "github.com/wailsapp/wails/v2/pkg/assetserver"
import "github.com/wailsapp/wails/v2/pkg/options/assetserver"

// Create handler that serves embedded assets
assetOptions := &assetserver.Options{
    Assets: assets,  // Your //go:embed all:frontend/dist
}
handler, err := assetserver.NewAssetHandler(assetOptions, logger)
```

This automatically provides:

- **SPA routing**: Serves index.html for non-existent paths
- **MIME types**: Correct Content-Type headers
- **Path resolution**: Finds frontend/dist within embed.FS
- **Error handling**: Proper 404 responses

#### 2. Configuration Injection Middleware

Since we're not using Wails' script injection, we need our own middleware:

```go
func configInjectionMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Use httptest.ResponseRecorder to capture response
        recorder := httptest.NewRecorder()
        next.ServeHTTP(recorder, r)

        // Only process successful HTML responses
        if recorder.Code != http.StatusOK || 
           !strings.Contains(recorder.Header().Get("Content-Type"), "text/html") {
            // Pass through unchanged
            for k, v := range recorder.Header() {
                w.Header()[k] = v
            }
            w.WriteHeader(recorder.Code)
            recorder.Body.WriteTo(w)
            return
        }

        // Inject configuration
        body := recorder.Body.String()
        config := fmt.Sprintf(`<script>
            window.__LANGKIT_CONFIG__ = {
                apiPort: %d,
                wsPort: %d,
                mode: "qt",
                runtime: "anki"
            };
        </script>`, apiPort, wsPort)

        // Insert before </head>
        newBody := strings.Replace(body, "</head>", config + "</head>", 1)

        // Write modified response
        w.Header().Set("Content-Type", recorder.Header().Get("Content-Type"))
        w.Header().Set("Content-Length", fmt.Sprint(len(newBody)))
        w.WriteHeader(recorder.Code)
        w.Write([]byte(newBody))
    })
}
```

### Implementation Steps

1. **Import Required Packages**
   
   ```go
   import (
       "github.com/wailsapp/wails/v2/pkg/assetserver"
       assetserveroptions "github.com/wailsapp/wails/v2/pkg/options/assetserver"
   )
   ```

2. **Create Asset Handler**
   
   - Use `NewAssetHandler` not `NewAssetServer`
   - Pass your embedded assets
   - No Wails runtime injection occurs

3. **Setup Chi Router**
   
   - Apply injection middleware to index routes
   - Serve other assets directly
   - Maintain clean separation of concerns

4. **Handle Edge Cases**
   
   - SPA routes automatically work
   - 404s handled correctly
   - Binary files served without modification

### Files to Reference in Wails

For deeper understanding, these Wails files are most relevant:

- `v2/pkg/assetserver/assethandler.go` - Core serving logic
- `v2/pkg/assetserver/fs.go` - Path resolution in embed.FS
- `v2/pkg/assetserver/mimecache.go` - MIME type handling
- `v2/pkg/assetserver/body_recorder.go` - Response modification pattern

### Benefits Over Custom Implementation

1. **Production-tested**: Wails' asset serving is battle-tested across thousands of applications
2. **SPA-aware**: Correctly handles client-side routing without configuration
3. **Performance**: Optimized MIME type detection and caching
4. **Maintainability**: Leverages existing, well-documented code
5. **Compatibility**: Ensures frontend works identically in both modes

## Technical Considerations

### Windows Console Behavior

- GUI binary compiled with `-H windowsgui` can receive args but not output to console
- Server mode will write to log file for debugging
- Status endpoint optional: `http://localhost:8080/status`

### Port Management

- Frontend: 8080 (fixed for simplicity [dev note: why?])
- WebRPC API: OS-assigned dynamic port
- WebSocket: OS-assigned dynamic port
- All ports injected via DOM, very low conflict risk

### Binary Distribution

- Host releases on GitHub
- Add-on downloads appropriate binary for platform
- Checksum verification for security
- Version checking using existing version.go logic

### Process Architecture

```
Anki Process
  └── Python Add-on
       ├── Binary Manager (download/update)
       ├── UI Manager (Qt WebEngine tab)
       └── Process Manager
            └── Langkit Server (subprocess)
                 ├── Chi HTTP Server (frontend)
                 ├── WebRPC API Server
                 ├── WebSocket Server
                 └── Zenity Dialogs (native)
```
