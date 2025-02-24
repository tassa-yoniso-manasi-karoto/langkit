# CLAUDE.md - Langkit Development Guide

## Build Commands
- Build entire application: `wails build --clean`
- Build for Windows: `wails build --clean --platform windows/amd64`
- Use scripts in `/scripts/` directory for platform-specific builds
- Install Wails CLI (required): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

## Code Style Guidelines
- **Imports**: Standard library first, third-party next, internal packages last
- **Naming**: 
  - Variables/Functions: camelCase
  - Types/Exported: PascalCase
  - Constants: ALL_CAPS (enum-like) or camelCase (others)
- **Error Handling**: Return errors, use custom ProcessingError type, early returns
- **Formatting**: Standard Go formatting (gofmt)
- **Comments**: Document behavior rather than implementation
- **Package Organization**:
  - core/: Business logic
  - cli/: Command-line interface
  - pkg/: Shared utilities
  - config/: Configuration
- **Patterns**: Context propagation, interface-based design, dependency injection
- Always use tabs to indent Golang code and use 4 spaces to indent CSS, Javascript or Svelte code.
- The GUI should follow the Material Design philosophy and principles.
- In the GUI any actions on interactable elements should trigger an effect to provide the user visual feedback.