# Dual UI Runtime Architecture

## Overview

Langkit supports three UI runtime modes from a single binary:

1. **Wails mode** (default): Standalone GUI with native WebView
2. **Browser mode**: Web server accessed via browser (`--server`)
3. **Anki mode**: Qt WebEngine integration (`--server /path/to/config.json`)

## Architecture

### Runtime Detection

The server detects runtime mode based on command-line arguments:
- No args → Wails mode
- `--server` → Browser mode  
- `--server /path/to/config.json` → Anki mode

### Single-Port Architecture

All services run on a single dynamically-assigned port:
- Frontend assets (served via Echo/Chi)
- WebRPC API endpoints
- WebSocket connections

Configuration is injected into `index.html` via middleware:
```javascript
window.__LANGKIT_CONFIG__ = {
    apiPort: 12345,
    wsPort: 12345,
    runtime: "anki"  // or "wails" or "browser"
}
```

### Frontend Runtime Handling

The frontend uses Svelte stores for reactive runtime detection:
- `$isWailsMode`, `$isBrowserMode`, `$isAnkiMode`
- Stores initialized at app startup
- Components adapt UI based on runtime (e.g., drag-drop support, return button)

## Anki Add-on Integration

### UI Integration

The add-on uses a "push" approach:
1. Hides Anki's webviews (toolbar, main, bottom)
2. Adds Langkit webview to the main layout
3. Provides ESC key and button to return to Anki

This avoids Qt widget lifecycle issues while providing full-screen experience.

### Core Components

1. **Process Management**: Start/stop/restart server with pipe handling
2. **Binary Management**: Auto-download with checksum verification
3. **WebView Integration**: Custom Qt WebEngine view with drag-drop support

### Drag-Drop Implementation

- **Wails**: Native drag-drop via Wails runtime
- **Anki**: Qt drag events bridged to JavaScript  
- **Browser**: File picker only (no drag-drop)

## Technical Details

### UI Runtime Abstraction

#### Backend (`ui` Package)

The `ui` package provides a unified interface for runtime-specific operations:
```go
ui.Initialize(fileDialog, urlOpener)
```

This abstraction allows:
- **Wails mode**: Native file dialogs via Wails API
- **Server modes**: Zenity dialogs for cross-platform compatibility
- Seamless switching between implementations based on runtime

#### Frontend Runtime Module

The frontend's `lib/runtime` module provides:
- **Runtime detection**: Reactive Svelte stores that components can subscribe to
- **Safe wrappers**: Functions like `safeWindowIsMinimised()` that gracefully handle missing APIs
- **Hybrid handlers**: Unified interfaces that adapt to the current runtime

For example, the drag-drop handler:
- Detects runtime mode via stores
- Wails mode: Registers native drag-drop handlers
- Anki mode: Sets up Qt bridge functions (`window.handleFileDrop`)
- Browser mode: Gracefully disables drag-drop
- All modes use the same API (`initializeDragDrop()`)

This abstraction enables components to work seamlessly across all runtimes without conditional logic scattered throughout the codebase.

### Dialog Handling

Runtime-specific dialogs:
- **Wails mode**: Native Wails dialogs
- **Server modes**: Zenity for cross-platform file dialogs

### Process Architecture

```
Anki Process
  └── Python Add-on
       ├── Binary Manager
       ├── WebView Tab (Qt WebEngine)
       └── Process Manager
            └── Langkit Server
                 └── Single Port (Echo server)
                      ├── Frontend assets
                      ├── WebRPC API
                      └── WebSocket
```

### Asset Serving

Server mode uses Wails' AssetHandler for SPA routing and MIME types, with custom middleware for config injection.

## Implementation Status

### Completed
- [x] Single-port unified server
- [x] Runtime detection and config injection
- [x] UI runtime abstraction (backend and frontend)
- [x] Svelte stores for runtime mode
- [x] Anki addon with push integration
- [x] Cross-platform drag-drop support
- [x] Binary auto-download and verification
- [x] Subprocess pipe handling fix

### Pending
- [ ] Windows console output handling
- [ ] AnkiWeb distribution
- [ ] Installation documentation