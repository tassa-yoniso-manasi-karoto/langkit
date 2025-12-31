# internal/gui

This package provides the GUI runtime infrastructure for Langkit. It supports two modes:
- **Wails mode**: Standalone desktop app with native WebView
- **Server mode**: Headless server for browser/Qt WebEngine access

## File Overview

| File | Purpose |
|------|---------|
| `wails_run.go` | Entry point for Wails mode (`gui.Run()`) |
| `wails_app.go` | Wails `App` struct with lifecycle callbacks (`startup`, `domReady`, `beforeClose`, `shutdown`) |
| `server_run.go` | Entry point for server/headless mode |
| `run.go` | Shared server initialization used by both modes |
| `single_port_server.go` | Unified HTTP server serving API, WebSocket, and frontend assets on one port |
| `middleware.go` | HTTP middleware for runtime config injection into `index.html` |
| `assets.go` | Embedded frontend assets (`//go:embed`) |
| `throttling.go` | Event throttling configuration helpers |
| `logs.go` | Logger accessor helper |
| `err.go` | Crash handling and error dialogs |

## Architecture

All business logic is exposed via **WebRPC services** in `internal/api/services/`. The files in this package only handle:
- Application lifecycle
- Server infrastructure
- Runtime-specific concerns (dialogs, crash reports)

The frontend communicates exclusively through WebRPC over HTTP, not through Wails method bindings.
