# Projet summary
The project is in alpha release.

Langkit is an all-in-one command-line or GUI tool designed to **facilitate language learning from native media content** using a collection of diverse features to transform movies, TV shows, etc., into **easily ‘digestible’ material**. It was made with scalability, fault-tolerance in mind and supports automatic subtitle detection, bulk/recursive directory processing, seamless resumption of previously interrupted processing runs and multiple native (reference) language fallback.
Key Features:
    1. Subs2cards: Creates Anki flashcards (audio, image, text) from subtitle timings, similar to the classic subs2srs. Supports modern codecs (Opus/AVIF) and multi-threading.
    2. Making Dubtitles: Generates accurate subtitle files that match dubbed audio tracks using Speech-To-Text (STT) models (like Whisper). This addresses mismatches between standard subtitles and dubs.
    3. Voice Enhancing: Isolates and amplifies dialogue in audio tracks while reducing background music/effects, making speech clearer. Uses models like Demucs.
    4. Subtitle Romanization: Converts subtitles into roman characters based on pronunciation for various languages.
    5. Subtitle Tokenization: Adds spaces between words for languages that don't typically use them.
    6. Selective (Kanji) Transliteration: For Japanese, converts less frequent or irregularly read Kanji to Hiragana based on user-defined frequency thresholds, aiding incremental learning.
    7. Condensed Audio: Facilitates passive language immersion by generating an abridged audio file containing only the dialogue from media, guided by subtitle timings.

The backend is Golang code.
The GUI is supported by the wails GUI framework (v2) and is powered by Svelte 4 / Typescript code with TailwindCSS.
A refactor has completely discarded Wails App methods, and my frontend now functions with standard web technologies such as WebSocket and HTTP via WebRPC (the old Wails method remain in the source code in at internal/gui/frontend/*go but are unused).
(FIY WebRPC is a schema-driven RPC framework that generates type-safe client and server code from a simple interface definition language called RIDL. Built on standard HTTP and JSON, it offers a simpler, more accessible alternative to gRPC by automating the creation of networking code and fully typed client libraries for web and microservice communication.)
This makes it possible to run my front-end inside a regular browser, or even inside Anki's QtWebEngine, so I have created an add-on that acts as a wrapper for my actual app so that I can run Langkit inside Anki & integrate it inside the Anki ecosystem.
This migration was roughly guided by the document docs/Dual_UI_Runtime_Architecture.md.

Anki's source is accessible for consultation in anki/ directory just in case.
Inspecting the available webviews via DevTools shows that Anki creates three specific web views: "bottom toolbar" (which has its own widget/toolbar), "main webview", and "top toolbar" (also with its own widget/toolbar). These Anki web views are hidden when Langkit's web view is opened, and then shown again when Langkit is closed.

# User-defined requirements

## Plan mode

Investigate YOURSELF. The default instruction of plan mode say you should research the issue using the Task tool with Plan subagent, but it is IMO a bad approach so my instructions are: DO NOT USE PLAN SUBAGENT IN PLAN MODE.

## Build Commands
- Build frontend only (deprecated): `cd internal/gui/frontend && npm run build`
- Build GO: `go build -o langkit-cli ./cmd/cli`
- Build Wails v2: `wails build`

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
  - pkg/: Shared utilities (public API!!)
  - config/: Configuration
- **Patterns**: Context propagation, interface-based design, dependency injection
- **UI Guidelines**:
  - Follow Material Design principles
  - All interactive elements should have visual feedback
  - Use Tailwind utility classes for styling
  - Stores for state management (errorStore, logStore, progressBarsStore)

## Code management
- When I say "suggest a commit message" this mean you provide a draft of a message in your answer, it does NOT mean that I want you to git commit.
- When writing commits focus on the WHY, not on the WHAT (the what is self obvious in the diff of the commit)
- IMPORTANT: when writing the commit message each line shouldn't go beyond 80 characters
- Do NOT write commit messages in the "conventional commit" style i.e. do not prefix with "feat: ", "fix: " or whatnot
- When writing detailed list of changed in the git descriptions, use one line per change and preceed it by a bullet point "∙"

