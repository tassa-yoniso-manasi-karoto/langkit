# Langkit Architecture Strategy: Navigating WebView2 Limitations and Future Portability

## Executive Summary

Langkit faces critical reliability issues with WebView2's single-threaded message pump architecture when using Wails' native bridge. This document outlines a strategic migration to standard web protocols using WebRPC for RPC operations and WebSocket for real-time updates. This approach ensures reliability, maintainability, type safety, and future portability—potentially including integration with Anki's Qt WebEngine.

## Current Challenges

### WebView2/Wails Bridge Issues
- **Language validation hangs**: Validation spinner remains indefinitely despite responsive UI
- **Export debug report failures**: Bridge saturation prevents critical debugging functionality  
- **Lost promises**: Multiple concurrent async operations corrupt bridge state
- **Debouncing ineffective**: Even 800ms delays don't prevent bridge issues

### Root Cause
WebView2 requires all operations to occur on a single UI thread with an active message pump. Rapid or concurrent Wails method calls can overwhelm this pump, causing operations to fail silently with promises never resolving.

## Architectural Solution: Hybrid Communication

### Core Principle
Minimize Wails bridge usage by implementing standard web protocols for all critical operations. WebRPC was chosen over raw HTTP for its type safety, minimal boilerplate, and familiar RPC-style calling patterns that closely match the existing Wails API.

### Communication Channels

#### 1. WebRPC API (New) - Synchronous Operations
- **Purpose**: Request/response operations requiring reliability with type safety
- **Port**: `:8081/rpc/` (alongside existing WebSocket)
- **Framework**: WebRPC - schema-driven RPC with generated TypeScript client
- **Benefits**:
  - Strongly-typed client/server contract
  - Generated TypeScript client matches current Wails calling patterns
  - Single schema file defines entire API
  - Maintained by go-chi developers
- **Use cases**:
  - Language validation
  - Settings persistence
  - File operations
  - Export operations
  - Any operation that currently fails on Windows

#### 2. WebSocket (Existing) - Asynchronous Updates  
- **Purpose**: Real-time state synchronization
- **Port**: `:8081/ws` (dynamically allocated)
- **Current use**: LLM provider state updates
- **Future use**: Progress notifications, live logs, state changes

#### 3. Wails Bridge (Deprecated Path)
- **Purpose**: Native OS integration only
- **Use cases**: 
  - File/folder dialogs
  - System tray operations
  - Window management
  - One-time initialization calls

### Why WebRPC Over Alternatives

**vs Raw HTTP (httprouter):**
- WebRPC provides automatic TypeScript client generation
- Type safety across frontend/backend boundary
- Less boilerplate code to maintain
- Schema serves as living API documentation

**vs gRPC-Web:**
- Simpler, uses JSON instead of Protocol Buffers
- No complex proxy setup required
- Lighter weight for local communication
- Better browser DevTools support

**vs tRPC:**
- Backend-agnostic (not tied to TypeScript)
- Works naturally with Go backend
- More suitable for polyglot environments

**vs OpenAPI/Swagger:**
- RPC-style matches current Wails patterns (method calls vs REST resources)
- Much simpler schema format (RIDL vs verbose YAML/JSON)
- Cleaner generated code without REST abstractions
- Direct method calls: `client.validateLanguage()` vs `apiClient.languageApi.validateLanguage()`
- Better suited for internal desktop app communication vs public APIs
- Example schema comparison:
  ```ridl
  # WebRPC (simple)
  service LangkitService
    - ValidateLanguage(tag: string, single: bool) => (valid: bool)
  ```
  vs OpenAPI's 30+ lines of YAML for the same endpoint

**Note:** OpenAPI would be preferable for public APIs needing extensive documentation, multiple client languages, or REST conventions. For internal desktop app communication, WebRPC's simplicity is a strength.

**Migration Benefits:**
- Generated TypeScript client has similar API to current Wails methods
- Schema-first approach ensures API consistency
- Actively maintained by go-chi team
- Minimal learning curve from Wails patterns

## Migration Strategy

### Phase 1: Critical Fixes (Immediate)
1. Define WebRPC schema for language validation service
2. Generate Go server and TypeScript client
3. Verify Windows reliability improvement
4. Add comprehensive logging for debugging

### Phase 2: Systematic Migration (1-2 months)
Priority order based on failure frequency:
1. Export debug report → WebRPC service method
2. Settings save/load → WebRPC service methods  
3. Media file operations → WebRPC service methods
4. Status checks (Docker, FFmpeg, etc.) → WebRPC with polling

### Phase 3: WebSocket Enhancement (Future)
- Expand WebSocket protocol for bidirectional communication
- Implement proper message typing and routing
- Add reconnection logic and queue management

## Implementation Guidelines

### When to Use Each Channel

**WebRPC API**:
- Operations that must complete reliably
- File I/O or other potentially blocking operations
- Anything called multiple times in succession
- Operations needing timeout control
- Methods benefiting from type safety

**WebSocket**:
- Server-initiated updates
- Progress/status notifications
- Real-time log streaming
- State synchronization

**Wails Bridge**:
- Native dialog boxes
- System integration features
- Truly one-off operations

### Example WebRPC Schema

```ridl
webrpc = v1
name = langkit
version = v1.0.0

struct LanguageValidation
  - tag: string
  - single: bool
  
struct ValidationResponse
  - valid: bool
  - standard: string
  - error: string

struct RomanizationInfo
  - styles: []string
  - default: string

struct DebugExportResponse
  - file: string
  - size: int64

service LangkitService
  - ValidateLanguage(validation: LanguageValidation) => (response: ValidationResponse)
  - SaveSettings(settings: map<string,any>) => (success: bool, error: string)
  - GetRomanization(lang: string) => (info: RomanizationInfo)
  - ExportDebugReport() => (response: DebugExportResponse)
```

**Generated TypeScript usage (similar to current Wails pattern):**
```typescript
// Before (Wails):
const response = await ValidateLanguageTag(tag, true);

// After (WebRPC):
const response = await client.validateLanguage({tag, single: true});
```

## Future Portability Considerations

### Qt WebEngine Integration
Given Langkit's role as an Anki companion app:

1. **Dual Runtime Support**: Architecture supports running standalone or within Anki
2. **Zero Wails Dependency**: Standard web protocols work in any WebView
3. **Shared Backend**: Same Go backend serves both Wails and Qt WebEngine frontends

### Benefits of Protocol-Based Architecture
- **Platform Independence**: Works identically on all platforms
- **Debuggability**: Standard HTTP/WS tools (curl, wscat, browser DevTools)
- **Type Safety**: Single schema generates both server interfaces and client code
- **Testability**: Easy to mock and test endpoints
- **Maintainability**: No complex WebView2 workarounds
- **Future-proof**: Ready for alternative frontends (Qt, Electron, or even web)

## Technical Considerations

### Security
- Bind to localhost only
- Optional API key for future remote access
- CORS configured for local origins only

### Performance
- Local WebRPC latency: ~0.1-0.5ms (negligible)
- WebSocket message latency: ~0.01-0.05ms
- Both faster than current Wails bridge issues
- WebRPC adds minimal overhead over raw HTTP

### Error Handling
- WebRPC: Schema-defined errors with automatic HTTP status mapping
- WebSocket: Structured error messages with recovery instructions
- Timeouts: 2-second default for all RPC operations
- Type-safe error handling in generated clients

### WebRPC Limitations (Acceptable for Desktop Apps)
- No built-in streaming (covered by WebSocket)
- JSON-only serialization (sufficient for settings/validation operations)
- Schema version tracked but no automatic compatibility checking (manageable with controlled deployments)
- Limited to Chi router (Chi is excellent for this use case)

## Success Metrics

1. **Reliability**: Zero hanging operations on Windows
2. **Maintainability**: Reduced codebase complexity
3. **Portability**: Able to run frontend in non-Wails environment
4. **Performance**: Sub-millisecond local operation latency

## Conclusion

By migrating from Wails' proprietary bridge to WebRPC and WebSocket protocols, Langkit gains reliability, maintainability, type safety, and future portability. WebRPC's schema-driven approach provides similar developer experience to Wails with generated TypeScript clients, while using standard HTTP underneath. This architecture acknowledges WebView2's limitations rather than fighting them, following patterns proven successful by projects like RWKV-Runner. The gradual migration path ensures stability while moving toward a fully portable architecture that could one day run seamlessly within Anki's Qt WebEngine or any other web runtime.