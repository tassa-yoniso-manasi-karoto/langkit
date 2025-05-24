# CLAUDE.md - Langkit Development Guide

## Build Commands
- Build frontend only: `cd internal/gui/frontend && npm run build`
- Dev frontend: `cd internal/gui/frontend && npm run dev`

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
- Do NOT think or write tests without my EXPLICIT COMMAND.
- IMPORTANT: when writing the commit message each line shouldn't go beyond 80 characters
- Unless I specifically request it: Do NOT git add / diff / commit and do not git pull
- Do not write commit messages in the "conventional commit" style i.e. do not prefix with "feat: ", "fix: " or whatnot
- When writing detailed list of changed in the git descriptions, use one line per change and preceed it by a bullet point "∙"
