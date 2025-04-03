# CLAUDE.md - Langkit Development Guide

## Build Commands
- Build entire application: `wails build --clean`
- Build for Windows: `wails build --clean --platform windows/amd64`
- Build frontend only: `cd internal/gui/frontend && npm run build`
- Dev frontend: `cd internal/gui/frontend && npm run dev`
- Type checking: `cd internal/gui/frontend && npm run check`
- Install Wails CLI (required): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

## Code Style Guidelines
- In Svelte / Typescript, the template literals with interpolation (${variable}) break syntax highlighting in my IDE, don't use them and prefer simple string concatenations.
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
- Unless I specifically request it: Do not git add / diff / commit and do not git pull
- Do not write commit messages in the "conventional commit" style
- Do not put "𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇" at the end of the commits but do put a mention of co-authorship as you are supposed to with: "
🤖 Generated with Claude Code
Co-Authored-By: Claude <noreply@anthropic.com>"
- When writing detailed list of changed in the git descriptions, use one line per change and preceed it by a bullet point "∙"
- Include all noteworthy changes in the main commit message, separated by semicolons
