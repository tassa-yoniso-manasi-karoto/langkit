# CLAUDE.md - Langkit Development Guide

## Build Commands
- Build entire application: `wails build --clean`
- Build for Windows: `wails build --clean --platform windows/amd64`
- Build frontend only: `cd internal/gui/frontend && npm run build`
- Dev frontend: `cd internal/gui/frontend && npm run dev`
- Type checking: `cd internal/gui/frontend && npm run check`
- Install Wails CLI (required): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

## Code Style Guidelines
- **Imports**: Standard library first, third-party next, internal packages last
- **Naming**: 
  - Variables/Functions: camelCase
  - Types/Exported: PascalCase
  - Constants: ALL_CAPS (enum-like) or camelCase (others)
- **Error Handling**: Return errors, use custom ProcessingError type, early returns
- **Formatting**: Standard Go formatting (gofmt), 4 spaces for Svelte/TS/CSS
- **Comments**: Document behavior rather than implementation
- **Package Organization**:
  - core/: Business logic
  - cli/: Command-line interface
  - pkg/: Shared utilities
  - config/: Configuration
- **Patterns**: Context propagation, interface-based design, dependency injection
- **UI Guidelines**:
  - Follow Material Design principles
  - All interactive elements should have visual feedback
  - Use Tailwind utility classes for styling
  - Stores for state management (errorStore, logStore, progressBarsStore)

## Code management
- Do not git add or revert go.mod or go.sum
- Do not git diff or git pull
- Do not write commit messages in the "conventional commit" style
- Include all noteworthy changes in the main commit message, separated by semicolons
