# Internal API Package

This package provides the WebRPC API server and service implementations for Langkit. It includes a service registry pattern for scalable service management and automatic code generation from RIDL schemas.

## Architecture

- **registry.go** - Service registry for managing multiple WebRPC services
- **server.go** - Generic WebRPC server with middleware support
- **services/** - Individual service implementations
- **generated/** - Auto-generated code from RIDL schemas (DO NOT EDIT)

## Code Generation

WebRPC code is generated from RIDL schemas located in `/api/schemas/`. 

To regenerate code after schema changes:

```bash
# From the repository root
cd api
make generate-go    # Generates Go server/client code
make generate-ts    # Generates TypeScript client code
make all           # Generates both Go and TypeScript
```

The generated files are:
- Go: `internal/api/generated/*.gen.go`
- TypeScript: `internal/gui/frontend/src/api/generated/*.gen.ts`

## Adding New Services

This guide covers the complete process of migrating Wails methods to WebRPC services, including handling complex dependencies and avoiding Go import cycles.

### Overview

WebRPC services provide a type-safe, reliable alternative to Wails' native bridge methods. This is particularly important for Windows where WebView2's message pump limitations can cause reliability issues. The migration follows a specific pattern to maintain clean architecture and avoid circular dependencies.

### Step 1: Create the RIDL Schema

Create a new schema file in `/api/schemas/services/`. For example, `logging.ridl`:

```ridl
webrpc = v1
name = langkit-logging
version = v1.0.0

# Service description

struct LogEntry
  - lvl?: float64         # Log level (-1=TRACE to 5=FATAL)
  - msg?: string          # Log message
  - comp?: string         # Component name
  - ctx?: map<string,any> # Context information

service LoggingService
  - BackendLogger(component: string, logJson: string) => ()
  - SetTraceLogs(enable: bool) => ()
  - GetTraceLogs() => (enabled: bool)
```

**Important Schema Guidelines:**
- Use optional fields (?) for nullable values
- Keep method signatures simple - complex objects should be JSON strings
- Return empty responses `=> ()` for void methods
- Use descriptive names that match the original Wails methods

### Step 2: Import the Schema

Add your service to the main schema file `/api/schemas/api.ridl`:

```ridl
import ./services/logging.ridl
```

### Step 3: Generate Code

From the `/api` directory:

```bash
make all  # Generates both Go and TypeScript code
```

This creates:
- Go interfaces and server: `internal/api/generated/api.gen.go`
- TypeScript client: `internal/gui/frontend/src/api/generated/api.gen.ts`

### Step 4: Handle Dependencies (Avoiding Import Cycles)

If your service needs access to core application logic, you MUST use the Provider interface pattern to avoid import cycles.

#### Create a Provider Interface

In `internal/api/interfaces/`, create an interface for what your service needs:

```go
package interfaces

// LoggingProvider interface for the logging service
type LoggingProvider interface {
    SetTraceLogs(enable bool)
    GetTraceLogs() bool
    ZeroLog() *zerolog.Logger
}
```

**Critical Pattern for Complex Types:**
If your methods need to accept complex types from the `core` package, use `interface{}`:

```go
type DryRunProvider interface {
    // The config parameter should be *core.DryRunConfig
    SetDryRunConfig(config interface{})  // Use interface{} to avoid importing core
}
```

### Step 5: Implement the Service

Create your service in `internal/api/services/`:

```go
package services

import (
    "context"
    "net/http"
    
    "github.com/rs/zerolog"
    
    "github.com/tassa-yoniso-manasi-karoto/langkit/internal/api"
    "github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/generated"
    "github.com/tassa-yoniso-manasi-karoto/langkit/internal/api/interfaces"
)

// Compile-time check that LoggingService implements api.Service
var _ api.Service = (*LoggingService)(nil)

type LoggingService struct {
    logger   zerolog.Logger
    provider interfaces.LoggingProvider
    handler  http.Handler
}

func NewLoggingService(logger zerolog.Logger, provider interfaces.LoggingProvider) *LoggingService {
    svc := &LoggingService{
        logger:   logger,
        provider: provider,
    }
    
    // Create the WebRPC handler
    svc.handler = generated.NewLoggingServiceServer(svc)
    
    return svc
}

// Required api.Service methods
func (s *LoggingService) Name() string { return "LoggingService" }
func (s *LoggingService) Handler() http.Handler { return s.handler }
func (s *LoggingService) Description() string { return "Logging and diagnostics service" }

// Implement your service methods
func (s *LoggingService) SetTraceLogs(ctx context.Context, enable bool) error {
    s.provider.SetTraceLogs(enable)
    return nil
}
```

### Step 6: Implement the Provider in Core

In your core handler (e.g., `internal/core/handler.go`), implement the provider interface:

```go
// Add compile-time assertion
var _ interfaces.LoggingProvider = (*GUIHandler)(nil)

// If using interface{} for complex types, perform type assertion:
func (h *GUIHandler) SetDryRunConfig(config interface{}) {
    dryRunConfig, ok := config.(*DryRunConfig)
    if !ok && config != nil {
        h.logger.Error().Msg("Invalid config type")
        return
    }
    // ... implementation
}
```

### Step 7: Register the Service

In `internal/gui/app.go`, register your service during initialization:

```go
// Register logging service (handler implements LoggingProvider)
loggingSvc := services.NewLoggingService(*a.getLogger(), handler)
if err := apiServer.RegisterService(loggingSvc); err != nil {
    a.getLogger().Fatal().Err(err).Msg("Failed to register logging service")
}
```

### Step 8: Create TypeScript Wrappers

Create drop-in replacements for Wails methods in `internal/gui/frontend/src/api/services/`:

```typescript
import { LoggingService } from '../generated/api.gen';
import { getAPIBaseUrl, defaultFetch } from '../client';

let loggingServiceInstance: LoggingService | null = null;

async function getLoggingService(): Promise<LoggingService> {
    if (!loggingServiceInstance) {
        const baseUrl = await getAPIBaseUrl();
        loggingServiceInstance = new LoggingService(baseUrl, defaultFetch);
    }
    return loggingServiceInstance;
}

// Drop-in replacement maintaining exact Wails signature
export async function SetTraceLogs(enable: boolean): Promise<void> {
    const service = await getLoggingService();
    await service.setTraceLogs({ enable });
}
```

CRITICAL: When importing type make sure to use a dedicated "import type" as in this example:
```typescript
import { ProcessingService } from '../generated/api.gen';
import type { ProcessRequest, ProcessingStatus } from '../generated/api.gen';
```
If you don't use "import type" the bundler tree-shaking optimization will throw an error:
	webkit: [Error] SyntaxError: Importing binding name 'ProcessRequest' is not found.
	V8: Uncaught SyntaxError: The requested module '/src/api/generated/api.gen.ts' does not provide an export named 'ProcessRequest'

### Step 9: Update Frontend Usage

Replace Wails method calls with your new WebRPC methods:

```typescript
// Before:
import { SetTraceLogs } from '../../../wailsjs/go/gui/App';

// After:
import { SetTraceLogs } from '../api/services/logging';
```

### Common Patterns and Best Practices

#### WebSocket Broadcasting
If your service needs to broadcast events, inject the `WebsocketService` interface:

```go
type MyService struct {
    wsServer interfaces.WebsocketService
}

// Usage:
s.wsServer.Emit("event.name", data)
```

#### Error Handling
- Don't return errors to frontend for logging operations
- Return meaningful errors for critical operations
- Log errors server-side before returning

#### Performance Considerations
- High-frequency methods (like logging) should minimize overhead
- Consider batching for operations that may be called rapidly
- Use appropriate context timeouts

### Troubleshooting

**Import Cycle Errors:**
- Always use interfaces in the `api/interfaces` package
- Never import `core` types directly into API services
- Use `interface{}` for complex types and perform type assertions

**Missing Methods:**
- Ensure your service implements all three `api.Service` methods
- Check that generated interfaces match your RIDL schema

**TypeScript Errors:**
- Run `make all` after any schema changes
- Ensure singleton pattern is used for service instances
- Match parameter names exactly with RIDL schema

### Complete Example

See the `LoggingService` migration in commits for a complete example of migrating 9 Wails methods to WebRPC, including handling WebSocket broadcasting and complex dependencies.

## Development

The Makefile supports additional commands:
- `make validate` - Validate all schemas without generating code
- `make clean` - Remove all generated files
- `make watch` - Auto-regenerate on schema changes (requires inotifywait)