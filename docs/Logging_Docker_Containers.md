# Progress Tracking for Docker Containers

This document describes three approaches for tracking progress of Docker-based operations in Langkit.

## Overview

There are three fundamentally different scenarios:

1. **Docker infrastructure operations** (e.g., pulling images) - Docker SDK provides structured progress data directly
2. **Long-running container services** (e.g., database initialization) - Requires parsing container logs via milestone patterns
3. **One-shot exec commands** (e.g., ML model inference) - Parse real-time TTY output from docker exec

---

## Approach 1: Docker SDK Direct API

Used for: Image pulls, container lifecycle operations

### Flow

```
Docker SDK ImagePull()
    ↓ (returns io.ReadCloser with JSON stream)
json.Decoder reads JSONMessage {
    Status: "Downloading"
    ID: "abc123" (layer ID)
    Progress: { Current: 15000000, Total: 142000000 }
}
    ↓ (parsed directly in Go)
pullImageWithProgress() calculates total across layers
    ↓
handler.IncrementDownloadProgress()
    ↓
WebSocket → frontend
```

### Key Points

- Docker SDK gives **structured data directly** via `jsonmessage.JSONMessage`
- No log parsing needed - just decode JSON and extract `Current`/`Total` bytes
- Progress is exact (bytes downloaded)
- Implementation: `internal/pkg/voice/demucs_manager.go` → `pullImageWithProgress()`

### Relevant Types (Docker SDK)

```go
// github.com/docker/docker/pkg/jsonmessage
type JSONMessage struct {
    Status   string        `json:"status,omitempty"`
    ID       string        `json:"id,omitempty"`
    Progress *JSONProgress `json:"progressDetail,omitempty"`
}

type JSONProgress struct {
    Current int64 `json:"current,omitempty"`
    Total   int64 `json:"total,omitempty"`
}
```

---

## Approach 2: Container Log Pattern Matching

Used for: Monitoring progress of **long-running container services** (background processes)

When software inside a container (database, etc.) runs as a background service and outputs progress to stdout/stderr, we need to parse those logs to extract progress information. The `dockerutil` library provides infrastructure for this.

**Note:** This approach is for services started via `docker-compose up` that run continuously. The logs are captured via Docker's logging infrastructure. This does NOT work with `docker exec` commands - for those, see Approach 3 (TTY Exec Output Parsing).

### Example: Ichiran PostgreSQL Database Initialization

Ichiran is a Japanese morphological analyzer that requires a PostgreSQL database. First-time initialization downloads a 200MB dump and restores it to a 4.4GB database (~10-20 minutes). PostgreSQL outputs checkpoint messages to stderr that we can parse to estimate progress.

**Reference commits for this example:**
- **dockerutil** `8cbb8df29d51ec65f7c65709b974cbe4641e4785` - adds `ProgressMilestone`, `ProgressHandler`, regex matching in `ContainerLogConsumer`
- **go-ichiran** `5188b18644fa1be61e2f7e0c747bdbb420e05aec` - defines `IchiranProgressMilestones[]`, adds `WithProgressHandler()` option
- **langkit** `b0a6ab9793c07b6c2841930eb6d9dc523b79fb2a` - wraps callback in `JapaneseProvider.Initialize()` to call `handler.IncrementProgress()`

### Flow

```
Software inside container outputs to stdout/stderr:
    "checkpoint starting: wal"
    "checkpoint complete: wrote 1234 buffers"
    ↓
dockerutil.ContainerLogConsumer receives log lines
    ↓
Checks against registered Milestones[] via regex:
    {Pattern: "checkpoint starting", Progress: 5, Description: "..."}
    {Pattern: "checkpoint complete.*wrote \\d+ buffers", Progress: -1, Description: "..."}
    ↓
When pattern matches → calls ProgressHandler callback
    ↓
Library's progressHandler (set via WithProgressHandler option)
    ↓
Langkit wraps this callback to call handler.IncrementProgress()
    ↓
WebSocket → frontend
```

### Key Points

- Requires **log parsing** because container software doesn't expose structured progress
- `dockerutil.ContainerLogConsumer` watches logs and matches regex patterns
- Progress values are **estimated** based on predefined milestones
- Each containerized service needs its own milestone definitions

### Relevant Types (dockerutil)

```go
// github.com/tassa-yoniso-manasi-karoto/dockerutil

// ProgressMilestone represents a log pattern that indicates progress
type ProgressMilestone struct {
    Pattern     string  // Regex pattern to match
    Progress    float64 // Progress percentage (0-100), or -1 for dynamic
    Description string  // User-friendly description
}

// ProgressHandler is called when milestones are reached
type ProgressHandler func(progress float64, description string, logMessage string)

// ContainerLogConsumer fields for progress tracking
type ContainerLogConsumer struct {
    ProgressHandler ProgressHandler
    Milestones      []ProgressMilestone
    // ... other fields
}
```

### Implementing Progress Tracking for New Container Software

1. **Define milestones** - Identify log patterns that indicate progress stages
2. **Create milestone array** - Define regex patterns with progress percentages
3. **Pass handler via option** - Use functional options pattern (e.g., `WithProgressHandler()`)
4. **Wrap callback in langkit** - Convert to `handler.IncrementProgress()` format

Example milestone definition:
```go
var MyServiceProgressMilestones = []dockerutil.ProgressMilestone{
    {Pattern: "Starting initialization", Progress: 0, Description: "Starting..."},
    {Pattern: "Loading model", Progress: 25, Description: "Loading model..."},
    {Pattern: "Processing complete", Progress: 100, Description: "Done!"},
}
```

---

## Approach 3: TTY Exec Output Parsing

Used for: Monitoring progress of **one-shot commands** run via `docker exec`

When running a command inside an already-running container (e.g., ML inference, audio processing), we use `docker exec` which doesn't go through Docker's logging infrastructure. Instead, we attach directly to the exec's output stream.

### Why TTY Mode?

Modern CLI tools like Python's Rich library output progress bars using ANSI escape codes that update in-place. However, their behavior differs based on whether stdout is a TTY:

- **Without TTY (`Tty: false`)**: Rich detects non-interactive mode and may output nothing or only a final summary
- **With TTY (`Tty: true`)**: Rich outputs real-time progress updates with ANSI escape codes

By enabling TTY mode in docker exec, we get the same output you'd see in an interactive terminal, which we can then parse.

### Example: Demucs Voice Separation

Demucs (via demucs-inference) uses Rich progress bars that output percentage updates like:

```
⠸ test_audio.opus   ━━━━━━━━━━━━━━━━━━━━━━ 45%  0:00:01
```

In TTY mode, this updates in real-time. We parse the `\d+%` pattern to extract progress.

**Reference implementation:** `internal/pkg/voice/demucs_manager.go` → `execInContainerWithProgress()`

### Flow

```
docker exec -t container_name command
    ↓
ContainerExecAttach with Tty: true
    ↓
resp.Reader contains raw TTY stream (ANSI escape codes + text)
    ↓
parseProgressPercent() extracts \d+% patterns
    ↓
ProgressCallback(percent)
    ↓
handler.IncrementProgress() or similar
    ↓
WebSocket → frontend
```

### Key Points

- Uses `docker exec` with `Tty: true` to get real-time Rich/tqdm output
- **Direct stream parsing** - no Docker logging infrastructure involved
- Progress is **exact** (actual percentage from the tool)
- Simple regex pattern (`\d+%`) works for most progress bars
- Stdout/stderr are **combined** in TTY mode (no multiplexing)

### Implementation

```go
// Docker exec with TTY mode
execConfig := container.ExecOptions{
    Cmd:          cmd,
    AttachStdout: true,
    AttachStderr: true,
    Tty:          true,  // Enable TTY for Rich progress output
}

resp, _ := cli.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{Tty: true})

// Read and parse TTY stream
buf := make([]byte, 4096)
for {
    n, _ := resp.Reader.Read(buf)
    if pct := parseProgressPercent(buf[:n]); pct >= 0 {
        progressCallback(pct)
    }
}

// Simple percentage parser
func parseProgressPercent(data []byte) int {
    // Look for \d+% pattern in the raw TTY output
    // Works even with ANSI escape codes mixed in
}
```

### When to Use Approach 2 vs Approach 3

| Scenario | Approach |
|----------|----------|
| Background service logs (`docker-compose up`) | Approach 2 (Container Log Pattern Matching) |
| One-shot command (`docker exec`) | Approach 3 (TTY Exec Output Parsing) |
| Tool outputs milestones/stages | Approach 2 (define milestone patterns) |
| Tool outputs real-time percentages | Approach 3 (parse `\d+%`) |
| Need to watch logs after the fact | Approach 2 (logs are persisted) |
| Need real-time progress during exec | Approach 3 (direct stream) |

---

## Comparison

| Aspect | Docker SDK (Approach 1) | Log Parsing (Approach 2) | TTY Exec (Approach 3) |
|--------|-------------------------|--------------------------|------------------------|
| Use case | Docker operations | Background services | One-shot exec commands |
| Data source | Docker SDK JSON stream | Container logs | Exec TTY stream |
| Progress data | Structured (bytes) | Unstructured (milestones) | Unstructured (percentages) |
| Parsing method | `json.Decode()` | Regex milestone matching | Regex `\d+%` pattern |
| Milestones needed | No | Yes | No |
| Progress accuracy | Exact (bytes) | Estimated (stages) | Exact (percentage) |
| Real-time updates | Yes | Yes | Yes |
| Complexity | Lower | Higher | Medium |
| Example | Image pull | Ichiran DB init | Demucs separation |

---

## Langkit Handler Integration

All approaches ultimately call langkit's `MessageHandler` interface:

```go
// For regular task progress
IncrementProgress(taskID string, increment, total, priority int,
                  operation, descr, size string)

// For download progress (shows humanized bytes)
IncrementDownloadProgress(taskID string, increment, total, priority int,
                          operation, descr, heightClass, humanizedSize string)
```

**Important:** These methods expect **increments** (delta from last update), not absolute values. This matches the CLI progressbar API (`progressbar.Add(x)`).

The frontend `ProgressManager.svelte` displays progress based on the `type` field in the payload:
- `type: "download"` → Shows `humanizedSize` (e.g., "1.2 GB / 3.1 GB")
- `type: ""` (empty) → Shows `current/total` counts
