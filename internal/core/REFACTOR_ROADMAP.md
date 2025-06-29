# Progressive Refactoring Roadmap: A High-Level Guide

This document outlines a revised, high-level strategy for progressively refactoring the `core` package. The primary goals are to improve testability, maintainability, and clarity by introducing dependency injection and separating concerns. This guide focuses on the "why" and "what" at a strategic level, leaving the specific "how" (e.g., exact function signatures) to be defined during implementation.

## Core Principle: Isolate the "Outside World"

The fundamental principle of this refactor is to isolate the core application logic from external dependencies and side effects. Any part of the code that communicates with the filesystem, external processes, or network APIs is a candidate for abstraction.

The `Task` struct will be the central point for dependency injection. Instead of performing I/O directly, it will delegate these actions to services that are provided to it. In production, these will be real services; in tests, they will be mocks.

---

### Phase 1: Abstracting System Dependencies (The Foundation)

**Goal**: Remove all direct dependencies on the filesystem and external command-line tools from the primary logic flow (`cards.go`, `item.go`, etc.).

**Why**: These are the most significant barriers to writing fast, reliable unit tests. Mocking these dependencies will allow us to test the core logic without touching the disk or running slow external processes.

**Key Areas for Abstraction:**

1.  **Filesystem Operations**:
    *   **What**: Any call to `os.Stat`, `os.ReadDir`, `os.OpenFile`, `os.MkdirAll`, `os.WriteFile`, and `filepath.Glob`.
    *   **Suggested Action**: Create a `Filesystem` service interface that provides methods for these operations. The `Task` struct will hold an instance of this service.

2.  **External Process Execution**:
    *   **What**: All direct calls to `executil.Command` for `ffmpeg` and `mediainfo`.
    *   **Suggested Action**: Create a `MediaToolProvider` or similar service. This service will be responsible for building and executing commands for `ffmpeg` (`media.FFmpeg`) and `mediainfo` (`core.Mediainfo`). This centralizes command execution and makes it easy to mock in tests.

3.  **Path Management**:
    *   **What**: The various path-building helper methods on the `Task` struct (e.g., `outputFile`, `mediaOutputDir`).
    *   **Suggested Action**: Consolidate this logic into a dedicated `PathService`. This service will take the `Task`'s state as input to construct necessary paths, making the logic pure and easily testable.

---

### Phase 2: Separating Core Business Logic

**Goal**: Extract distinct, cohesive units of business logic from the monolithic `cards.go` and `item.go` files into their own services. This will make the main `Execute` function a high-level coordinator, improving readability.

**Key Areas for Abstraction:**

1.  **Audio Track Selection**:
    *   **What**: The logic currently in `track_selector.go` (`getIdealTrack`, `getAnyTargLangMatch`, etc.).
    *   **Suggested Action**: Create a `TrackSelector` service. Crucially, its `ChooseAudio` method should not depend on the entire `Task` struct. Instead, it should accept the necessary data (e.g., the list of audio tracks, target language, channel preferences) as arguments. This makes its dependencies explicit and its behavior easier to test.

2.  **Subtitle File Handling**:
    *   **What**: All interactions with the `subs` package, such as `subs.OpenFile` and `subs.Write`.
    *   **Suggested Action**: Introduce a `SubtitleProvider` service to wrap these file I/O operations. This will allow tests to provide mock subtitle data without reading from the disk.

3.  **Language Detection**:
    *   **What**: The `GuessLangFromFilename` logic.
    *   **Suggested Action**: Move this into a `LanguageDetector` service. This is a simple but effective way to start practicing dependency injection for pure logic components.

---

### Phase 3: Isolating Complex and Asynchronous Services

**Goal**: Abstract the most complex parts of the application, particularly those involving concurrency and external network APIs.

**Why**: These systems are difficult and slow to test. Abstracting them allows us to test the main application flow without running the actual heavyweight operations.

**Key Areas for Abstraction:**

1.  **Concurrency (`Supervisor`)**:
    *   **What**: The entire worker pool and processing logic within `concurrency.go`.
    *   **Suggested Action**: Create a `WorkerPool` or `TaskProcessor` service that encapsulates the `Supervisor`'s responsibilities. The `Execute` method would simply call `taskProcessor.Run()`.

2.  **Transliteration Service**:
    *   **What**: The complex logic involving the provider manager, browser management, and caching in `translit_*.go`.
    *   **Suggested Action**: Create a high-level `TransliterationService`. The `Task` would simply call `transliterator.Process(subtitles)`, and the service would handle all the underlying complexity of managing providers.

3.  **Voice and STT APIs**:
    *   **What**: The API calls within `enhance.go` and `item.go` to services like `voice.SeparateVoice` and `voice.TranscribeAudioWithModel`.
    *   **Suggested Action**: Abstract these behind a `VoiceService` that handles the interactions with different providers.

### Putting It All Together

By the end of this progressive refactor, the `NewTask` function will be responsible for creating and injecting the *real* implementations of these services. In our tests, we will create a `Task` and inject *mock* implementations, giving us full control over the test environment. This will allow us to test the behavior of `Execute` and other core functions thoroughly and efficiently.
