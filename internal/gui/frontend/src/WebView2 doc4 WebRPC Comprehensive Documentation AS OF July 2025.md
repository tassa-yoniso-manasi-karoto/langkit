# WebRPC Implementation Details: Comprehensive Documentation

WebRPC is a schema-driven RPC framework that generates type-safe client and server code from simple schema definitions. Built on standard HTTP/JSON, it provides a simpler alternative to gRPC while maintaining type safety and code generation benefits. Created by the go-chi team, WebRPC emphasizes simplicity and pragmatism over feature completeness.

## Quick Start Example

### Basic RIDL Schema Syntax

```ridl
webrpc = v1              # Protocol version (required)
name = myapp             # Schema name
version = v1.0.0         # Your API version

# Enum with underlying type
enum Role: uint32
  - USER
  - ADMIN
  - MODERATOR = 10       # Explicit value

enum Status: string
  - active
  - inactive
  - pending

# Struct definition
struct User
  - id: uint64
    + json = "user_id"                 # JSON field name
    + go.field.name = ID               # Go field name  
    + go.tag.db = "id"                 # Custom Go tags
  - username: string
  - email?: string                     # Optional field
  - role: Role
  - metadata?: map<string,any>         # Optional nested map
  - tags?: []string                    # Optional array
  - createdAt: timestamp               # ISO 8601 format

# Service definition  
service UserService
  @deprecated: "Use UserServiceV2"     # Service-level annotation
  
  - GetUser(userID: uint64) => (user: User)
    @internal                          # Method annotation
    
  - CreateUser(user: User) => (user: User)
    @deprecated: "Use CreateUserV2"
```

### Code Generation

```bash
# Install webrpc-gen
go install github.com/webrpc/webrpc/cmd/webrpc-gen@latest

# Generate Go server and client
webrpc-gen -schema=api.ridl -target=golang -pkg=api -server -client -out=./api.gen.go

# Generate TypeScript client only
webrpc-gen -schema=api.ridl -target=typescript -client -out=./client.gen.ts

# Multiple targets
webrpc-gen -schema=api.ridl -target=golang -server -out=./server.gen.go
webrpc-gen -schema=api.ridl -target=typescript -client -out=./web/client.gen.ts
```

## Core Type System

WebRPC supports a focused set of types designed to map cleanly between languages:

### Primitive Types
- **Integers**: `uint8`, `uint16`, `uint32`, `uint64`, `int8`, `int16`, `int32`, `int64`
- **Floats**: `float32`, `float64`
- **Boolean**: `bool`
- **String**: `string` (UTF-8)
- **Bytes**: `byte` (alias for uint8)
- **Null**: `null`
- **Any**: `any` (maps to interface{}/any in Go, any in TypeScript)

### Complex Types
- **Timestamp**: `timestamp` - Must be ISO 8601 format (`YYYY-MM-DDTHH:mm:ss.sssZ`)
- **List**: `[]T` where T is any type (e.g., `[]string`, `[][]int32`)
- **Map**: `map<K,V>` where K is string/integer, V is any type
- **Enum**: Named constants with string or integer underlying type
- **Struct**: Named object with typed fields

### Optional Fields
Fields are required by default. Make them optional with `?`:
```ridl
struct UpdateRequest
  - name?: string       # Can be null/undefined
  - email?: string      # Can be null/undefined  
  - userId: uint64      # Required, cannot be null
```

## Schema Organization

### Import System
Split large schemas across multiple files:

```ridl
# types.ridl
webrpc = v1

struct User
  - id: uint64
  - name: string

enum Status: string
  - active
  - inactive
```

```ridl
# api.ridl
webrpc = v1
name = myapp
version = v1.0.0

import ./types.ridl          # Import all types

import ./auth.ridl
  - AuthToken               # Import specific types
  - LoginRequest
  - LoginResponse

service API
  - GetUser(id: uint64) => (user: User)
  - Login(req: LoginRequest) => (resp: LoginResponse)
```

## 1. Routing Behavior

WebRPC uses the chi router internally to handle HTTP routing with an RPC-style approach. The framework generates HTTP handlers that implement the standard `http.Handler` interface, making it compatible with the entire Go HTTP ecosystem.

### Service Method Routing

Each WebRPC service generates a single HTTP handler that manages all methods for that service. The routing follows a predictable pattern:

```
POST /<ServiceName>/<MethodName>
```

All RPC calls use POST with JSON payloads. Here's how routing works internally:

```go
// Generated handler manages internal routing
webrpcHandler := NewExampleServiceServer(&ExampleServiceRPC{})
r.Handle("/*", webrpcHandler) // Chi wildcard catches all paths

// The handler internally demultiplexes based on the request path
// Example routes generated:
// POST /ExampleService/GetUser
// POST /ExampleService/CreateUser
// POST /ExampleService/UpdateUser
```

### Multiple Services on Same Base Path

**Multiple services CAN share the same base path** - the generated handler internally distinguishes services by their name in the URL:

```go
r := chi.NewRouter()
r.Use(middleware.RequestID)
r.Use(middleware.Logger)
r.Use(middleware.Recoverer)

// Multiple services on different patterns
r.Handle("/api/v1/users/*", userServiceHandler)
r.Handle("/api/v1/posts/*", postServiceHandler)
r.Handle("/api/v1/payments/*", paymentServiceHandler)

// Or all services on root with internal routing
r.Handle("/*", multiServiceHandler)
```

The generated code includes version headers for compatibility tracking:

```go
const WebrpcHeader = "Webrpc"
const WebrpcHeaderValue = "webrpc;gen-golang@v0.17.0;example@v0.0.1"
```

## Generated Code Structure

### Go Server Generation

The generated Go code provides:

1. **Type Definitions**: All structs and enums from your schema
2. **Service Interface**: Interface your implementation must satisfy
3. **HTTP Handler**: Automatically handles routing, JSON marshaling, error handling
4. **Client Code**: Optional typed client for calling the service

```go
// Generated interface (you implement this)
type UserService interface {
    GetUser(ctx context.Context, userID uint64) (*User, error)
    CreateUser(ctx context.Context, user *User) (*User, error)
}

// Your implementation
type userServiceImpl struct {
    db Database
}

func (s *userServiceImpl) GetUser(ctx context.Context, userID uint64) (*User, error) {
    return s.db.FindUser(userID)
}

// Wire it up
handler := NewUserServiceServer(&userServiceImpl{db: myDB})
http.ListenAndServe(":8080", handler)
```

### TypeScript Client Generation

The generated TypeScript provides:

1. **Type Definitions**: Interfaces for all types with proper TypeScript types
2. **Client Class**: Fully typed methods matching your service
3. **Error Classes**: Typed error handling

```typescript
// Generated types
export interface User {
  user_id: number      // Note: respects json field names
  username: string
  email?: string       // Optional fields become optional
  role: Role
  metadata?: {[key: string]: any}
  tags?: string[]
  createdAt: string    // Timestamps are strings
}

// Generated client usage
const client = new UserService('http://localhost:8080', fetch);

// Fully typed calls
const user = await client.getUser({ userID: 123 });
// TypeScript knows user.user_id is a number

// Error handling
try {
  await client.createUser({ user: newUser });
} catch (err) {
  if (err instanceof WebrpcError) {
    console.log(err.code, err.message);
  }
}
```

### Generated File Structure

```go
// api.gen.go contains:

// 1. Version functions
func WebRPCVersion() string { return "v1" }
func WebRPCSchemaVersion() string { return "v1.0.0" }
func WebRPCSchemaHash() string { return "abc123..." }

// 2. Type definitions
type Role uint32
const (
    Role_USER Role = 0
    Role_ADMIN Role = 1  
)

type User struct {
    ID       uint64                 `json:"user_id"`
    Username string                 `json:"username"`
    Email    *string                `json:"email,omitempty"`
    Role     Role                   `json:"role"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
    Tags     []string               `json:"tags,omitempty"`
    CreatedAt time.Time             `json:"createdAt"`
}

// 3. Service interface
type UserService interface {
    GetUser(ctx context.Context, userID uint64) (*User, error)
    CreateUser(ctx context.Context, user *User) (*User, error)
}

// 4. HTTP handler constructor
func NewUserServiceServer(svc UserService) http.Handler {
    // Returns handler with all routing/marshaling logic
}

// 5. Optional client
type userServiceClient struct {
    client HTTPClient
    urls   [2]string
}

func NewUserServiceClient(addr string, client HTTPClient) UserService {
    // Returns client implementation
}
```

## 2. Concurrent Request Handling

WebRPC inherits Go's standard HTTP server concurrency model, where each request is handled in its own goroutine automatically.

### Concurrency Model

- Each HTTP request runs in a separate goroutine
- **No built-in rate limiting or request queuing** - must be implemented as middleware
- Thread-safe by design through standard Go patterns
- Context objects are unique per request

### Panic Recovery

WebRPC uses chi's middleware.Recoverer for graceful panic handling:

```go
r := chi.NewRouter()
r.Use(middleware.Recoverer) // Catches panics

// When a handler panics:
// 1. Panic is caught by recovery middleware
// 2. 500 Internal Server Error returned
// 3. Panic is logged
// 4. Server continues running
```

### Implementing Rate Limiting

Since WebRPC doesn't include built-in rate limiting, here's how to add it:

```go
func rateLimitMiddleware(requestsPerSecond int) func(http.Handler) http.Handler {
    limiter := rate.NewLimiter(rate.Limit(requestsPerSecond), requestsPerSecond)
    
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

## 3. Custom Middleware Integration

WebRPC servers work seamlessly with standard HTTP middleware patterns, providing multiple integration points.

### Authentication Middleware

The generated servers have an `OnRequest` hook for custom logic:

```go
// Using OnRequest hook
webrpcHandler := NewExampleServiceServer(&ExampleServiceRPC{})

webrpcHandler.OnRequest = func(w http.ResponseWriter, r *http.Request) error {
    token := r.Header.Get("Authorization")
    if token == "" {
        return ErrWebrpcEndpoint.WithCause(errors.New("unauthorized"))
    }
    
    if !validateToken(token) {
        return ErrWebrpcEndpoint.WithCause(errors.New("invalid token"))
    }
    
    return nil // Continue processing
}
```

### Method-Specific Authentication

For more granular control, access method names in middleware:

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract method name from path
        path := r.URL.Path
        parts := strings.Split(path, "/")
        
        // Skip auth for certain methods
        if len(parts) >= 3 && parts[2] == "HealthCheck" {
            next.ServeHTTP(w, r)
            return
        }
        
        // Authenticate other methods
        if r.Header.Get("Authorization") == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

### Accessing Raw HTTP Objects

WebRPC provides context keys to access raw HTTP objects in handlers:

```go
// Context keys provided by WebRPC
const (
    HTTPResponseWriterCtxKey = "HTTPResponseWriter"
    HTTPRequestCtxKey       = "HTTPRequest"
    ServiceNameCtxKey       = "ServiceName"
    MethodNameCtxKey        = "MethodName"
)

// Access in handler implementation
func (s *ExampleServiceRPC) GetUser(ctx context.Context, header map[string]string, userID uint64) (uint32, *User, error) {
    // Access raw HTTP request
    if req, ok := ctx.Value(HTTPRequestCtxKey).(*http.Request); ok {
        clientIP := req.RemoteAddr
        userAgent := req.Header.Get("User-Agent")
        // Use request data for logging or logic
    }
    
    // Access raw HTTP response writer
    if w, ok := ctx.Value(HTTPResponseWriterCtxKey).(http.ResponseWriter); ok {
        w.Header().Set("X-Custom-Header", "custom-value")
    }
    
    return 200, &User{ID: userID, Username: "user"}, nil
}
```

### Request/Response Logging

Comprehensive logging middleware example:

```go
type responseWriter struct {
    http.ResponseWriter
    statusCode int
    size       int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
    size, err := rw.ResponseWriter.Write(b)
    rw.size += size
    return size, err
}

func loggingMiddleware(logger *log.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            wrapped := &responseWriter{
                ResponseWriter: w,
                statusCode:     200,
            }
            
            next.ServeHTTP(wrapped, r)
            
            logger.Printf(
                "Method: %s, Path: %s, Status: %d, Size: %d, Duration: %v, IP: %s",
                r.Method, r.URL.Path, wrapped.statusCode, wrapped.size,
                time.Since(start), r.RemoteAddr,
            )
        })
    }
}
```

### Metrics Collection

Integration with Prometheus:

```go
var (
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "webrpc_request_duration_seconds",
            Help: "Request duration in seconds",
        },
        []string{"method", "path", "status"},
    )
    
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "webrpc_requests_total",
            Help: "Total number of requests",
        },
        []string{"method", "path", "status"},
    )
)

func metricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        timer := prometheus.NewTimer(
            requestDuration.WithLabelValues(r.Method, r.URL.Path, ""),
        )
        
        wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
        next.ServeHTTP(wrapped, r)
        
        timer.ObserveDuration()
        requestsTotal.WithLabelValues(
            r.Method, r.URL.Path, strconv.Itoa(wrapped.statusCode),
        ).Inc()
    })
}
```

## 4. Error Response Format

WebRPC uses a consistent JSON structure for all error responses, with built-in error types and support for custom errors.

### Standard Error JSON Structure

```json
{
  "error": {
    "code": 100,
    "name": "UserNotFound",
    "message": "user not found",
    "cause": "database query failed: no rows found"
  }
}
```

### Built-in Error Types

WebRPC provides predefined errors with HTTP status mappings:

```go
var (
    ErrWebrpcEndpoint           = WebRPCError{Code: 0, Name: "WebrpcEndpoint", Message: "endpoint error", HTTPStatus: 400}
    ErrWebrpcBadRoute          = WebRPCError{Code: -2, Name: "WebrpcBadRoute", Message: "bad route", HTTPStatus: 404}
    ErrWebrpcBadMethod         = WebRPCError{Code: -3, Name: "WebrpcBadMethod", Message: "bad method", HTTPStatus: 405}
    ErrWebrpcBadRequest        = WebRPCError{Code: -4, Name: "WebrpcBadRequest", Message: "bad request", HTTPStatus: 400}
    ErrWebrpcServerPanic       = WebRPCError{Code: -7, Name: "WebrpcServerPanic", Message: "server panic", HTTPStatus: 500}
    ErrWebrpcInternalError     = WebRPCError{Code: -8, Name: "WebrpcInternalError", Message: "internal error", HTTPStatus: 500}
)
```

### Custom Error Definition

Define custom errors in your schema:

```ridl
webrpc = v1
name = user-service
version = v1.0.0

# Custom error definitions with HTTP status codes
error 100 UserNotFound "user not found" HTTP 404
error 101 InvalidCredentials "invalid credentials" HTTP 401
error 102 RateLimited "too many requests" HTTP 429
error 103 ValidationFailed "validation failed" HTTP 400
error 104 InsufficientPermissions "insufficient permissions" HTTP 403
```

### Error Handling in Practice

```go
func (s *UserService) GetUser(ctx context.Context, userID uint64) (*User, error) {
    // Simple error return
    if userID == 0 {
        return nil, ErrValidationFailed
    }
    
    // Error with additional context
    user, err := s.db.GetUser(userID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, ErrUserNotFound.WithCause(
                fmt.Errorf("user %d not found in database", userID),
            )
        }
        // Don't expose internal errors
        return nil, ErrInternalError.WithCause(
            fmt.Errorf("database error for user %d", userID),
        )
    }
    
    return user, nil
}
```

### Custom Error Response Fields

For validation errors with field-specific messages:

```go
type ValidationErrors struct {
    Fields []FieldError `json:"fields"`
}

type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

func (s *UserService) CreateUser(ctx context.Context, user *User) (*User, error) {
    var errors []FieldError
    
    if user.Email == "" {
        errors = append(errors, FieldError{
            Field:   "email",
            Message: "email is required",
        })
    }
    
    if len(user.Username) < 3 {
        errors = append(errors, FieldError{
            Field:   "username",
            Message: "username must be at least 3 characters",
        })
    }
    
    if len(errors) > 0 {
        errJSON, _ := json.Marshal(ValidationErrors{Fields: errors})
        return nil, ErrValidationFailed.WithCause(
            fmt.Errorf("%s", errJSON),
        )
    }
    
    // Create user...
}
```

## 5. Client Retry Behavior

**Important**: WebRPC's generated TypeScript clients do NOT include built-in retry logic. They use simple fetch() calls without automatic retries, timeouts, or circuit breakers.

### Generated Client Structure

The generated TypeScript client follows this pattern:

```typescript
export class UserService {
  private hostname: string
  private fetch: Fetch
  
  constructor(hostname: string, fetch?: Fetch) {
    this.hostname = hostname.replace(/\/$/, '')
    this.fetch = fetch || (window.fetch as any)
  }
  
  getUser = (args: GetUserArgs): Promise<GetUserReturn> => {
    return this.fetch(
      this.url('GetUser'),
      createHTTPRequest(args)
    ).then(res => buildResponse(res))
  }
}

// Usage patterns
const client = new UserService('http://localhost:8080');

// With custom fetch (for auth headers, etc)
const customFetch = (input: RequestInfo, init?: RequestInit) => {
  init = init || {};
  init.headers = {
    ...init.headers,
    'Authorization': `Bearer ${token}`
  };
  return fetch(input, init);
};

const authClient = new UserService('http://localhost:8080', customFetch);

### Implementing Exponential Backoff

Here's a complete retry implementation with exponential backoff:

```typescript
interface RetryConfig {
  maxAttempts: number;
  initialDelay: number;
  maxDelay: number;
  backoffMultiplier: number;
  timeout: number;
  retryableStatusCodes: number[];
}

const defaultRetryConfig: RetryConfig = {
  maxAttempts: 3,
  initialDelay: 100,
  maxDelay: 5000,
  backoffMultiplier: 2,
  timeout: 30000,
  retryableStatusCodes: [408, 429, 500, 502, 503, 504]
};

class WebRPCClientWithRetry {
  private baseUrl: string;
  private retryConfig: RetryConfig;
  
  constructor(baseUrl: string, config?: Partial<RetryConfig>) {
    this.baseUrl = baseUrl;
    this.retryConfig = { ...defaultRetryConfig, ...config };
  }
  
  async call<T>(method: string, params: any): Promise<T> {
    let attempt = 0;
    let delay = this.retryConfig.initialDelay;
    
    while (attempt < this.retryConfig.maxAttempts) {
      try {
        // Add timeout using AbortSignal
        const controller = new AbortController();
        const timeoutId = setTimeout(
          () => controller.abort(),
          this.retryConfig.timeout
        );
        
        const response = await fetch(`${this.baseUrl}/${method}`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Accept': 'application/json'
          },
          body: JSON.stringify(params),
          signal: controller.signal
        });
        
        clearTimeout(timeoutId);
        
        // Check if response is successful
        if (response.ok) {
          return await response.json();
        }
        
        // Check if error is retryable
        if (!this.retryConfig.retryableStatusCodes.includes(response.status)) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        // Retryable error - continue to retry logic
        
      } catch (error) {
        attempt++;
        
        // If last attempt or non-retryable error, throw
        if (attempt >= this.retryConfig.maxAttempts) {
          throw error;
        }
        
        // Wait with exponential backoff and jitter
        const jitter = Math.random() * 0.1 * delay;
        await new Promise(resolve => 
          setTimeout(resolve, delay + jitter)
        );
        
        // Increase delay
        delay = Math.min(
          delay * this.retryConfig.backoffMultiplier,
          this.retryConfig.maxDelay
        );
      }
    }
    
    throw new Error('Max retry attempts exceeded');
  }
}
```

### Circuit Breaker Implementation

```typescript
class CircuitBreaker {
  private failures: number = 0;
  private lastFailureTime: number = 0;
  private state: 'CLOSED' | 'OPEN' | 'HALF_OPEN' = 'CLOSED';
  
  constructor(
    private failureThreshold: number = 5,
    private recoveryTimeout: number = 60000
  ) {}
  
  async execute<T>(fn: () => Promise<T>): Promise<T> {
    if (this.state === 'OPEN') {
      if (Date.now() - this.lastFailureTime < this.recoveryTimeout) {
        throw new Error('Circuit breaker is OPEN');
      }
      this.state = 'HALF_OPEN';
    }
    
    try {
      const result = await fn();
      this.onSuccess();
      return result;
    } catch (error) {
      this.onFailure();
      throw error;
    }
  }
  
  private onSuccess(): void {
    this.failures = 0;
    this.state = 'CLOSED';
  }
  
  private onFailure(): void {
    this.failures++;
    this.lastFailureTime = Date.now();
    
    if (this.failures >= this.failureThreshold) {
      this.state = 'OPEN';
    }
  }
}

// Usage with WebRPC client
const circuitBreaker = new CircuitBreaker();
const client = new WebRPCClientWithRetry('https://api.example.com');

async function callWithCircuitBreaker(method: string, params: any) {
  return circuitBreaker.execute(() => client.call(method, params));
}
```

## 6. Schema Organization Best Practices

Organizing WebRPC schemas effectively is crucial for maintainability and scalability.

### Single Service Per File Pattern

```ridl
# users.ridl
webrpc = v1
name = user-service
version = v1.0.0

struct User
  - id: uint64
  - username: string
  - email: string
  - profile?: UserProfile
  - createdAt: timestamp
  - updatedAt?: timestamp

struct UserProfile
  - bio?: string
  - avatarUrl?: string
  - location?: string

service UserService
  - GetUser(userID: uint64) => (user: User)
  - CreateUser(user: User) => (user: User)
  - UpdateUser(userID: uint64, updates: UserProfile) => (user: User)
  - DeleteUser(userID: uint64) => (success: bool)
```

### Shared Types Organization

```
api/
├── schemas/
│   ├── common/
│   │   ├── types.ridl      # Shared data structures
│   │   ├── errors.ridl     # Common error definitions
│   │   └── enums.ridl      # Shared enumerations
│   ├── services/
│   │   ├── users/
│   │   │   ├── v1/
│   │   │   │   └── users.ridl
│   │   │   └── v2/
│   │   │       └── users.ridl
│   │   ├── orders/
│   │   │   └── v1/
│   │   │       └── orders.ridl
│   │   └── payments/
│   │       └── v1/
│   │           └── payments.ridl
│   └── build/
│       └── Makefile        # Schema compilation scripts
```

### Versioning Strategies

#### URL-Based Versioning

```ridl
# users-v1.ridl
service UserServiceV1
  - GetUser(userID: uint64) => (user: User)

# users-v2.ridl  
service UserServiceV2
  - GetUser(userID: uint64) => (user: User, metadata: UserMetadata)
```

Mount different versions on different paths:

```go
r.Handle("/api/v1/*", userServiceV1Handler)
r.Handle("/api/v2/*", userServiceV2Handler)
```

#### Backward Compatible Evolution

```ridl
# Original schema
struct User
  - id: uint64
  - username: string
  - email: string

# Evolved schema (backward compatible)
struct User
  - id: uint64
  - username: string
  - email: string
  - phoneNumber?: string      # Optional - safe to add
  - emailVerified?: bool      # Optional - safe to add
  - preferences?: UserPrefs   # Optional - safe to add
```

### Build Integration

```makefile
# Makefile for schema management
SCHEMAS := $(wildcard schemas/**/*.ridl)
GO_OUT := ./pkg/api
TS_OUT := ./web/src/api

.PHONY: generate
generate: generate-go generate-ts

generate-go:
	@for schema in $(SCHEMAS); do \
		webrpc-gen -schema=$$schema \
			-target=golang \
			-pkg=api \
			-server \
			-out=$(GO_OUT)/$$(basename $$schema .ridl).gen.go; \
	done

generate-ts:
	@for schema in $(SCHEMAS); do \
		webrpc-gen -schema=$$schema \
			-target=typescript \
			-client \
			-out=$(TS_OUT)/$$(basename $$schema .ridl).gen.ts; \
	done

validate:
	@for schema in $(SCHEMAS); do \
		webrpc-gen -schema=$$schema -validate-only || exit 1; \
	done
```

## 7. Context Propagation

WebRPC fully integrates with Go's context pattern, providing automatic propagation and cancellation support.

### Context Flow Through Stack

All generated handler methods receive `context.Context` as the first parameter:

```go
func (s *ExampleServiceRPC) GetUser(
    ctx context.Context,
    header map[string]string,
    userID uint64,
) (uint32, *User, error) {
    // Context is available throughout the method
    return 200, &User{ID: userID, Username: "example"}, nil
}
```

### Passing Custom Values

```go
// Middleware to add user info to context
func authContextMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract and validate token
        token := r.Header.Get("Authorization")
        user, err := validateTokenAndGetUser(token)
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        // Add user to context
        ctx := context.WithValue(r.Context(), "user", user)
        ctx = context.WithValue(ctx, "requestID", generateRequestID())
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Access in handler
func (s *UserService) GetProfile(ctx context.Context) (*Profile, error) {
    user, ok := ctx.Value("user").(*User)
    if !ok {
        return nil, ErrUnauthorized
    }
    
    requestID := ctx.Value("requestID").(string)
    log.Printf("Request %s: Getting profile for user %d", requestID, user.ID)
    
    return s.getProfileForUser(ctx, user.ID)
}
```

### Cancellation on Client Disconnect

Context automatically propagates cancellation:

```go
func (s *DataService) ProcessLargeDataset(ctx context.Context, datasetID string) error {
    // Long-running operation with cancellation checks
    dataset, err := s.loadDataset(datasetID)
    if err != nil {
        return err
    }
    
    for i, record := range dataset.Records {
        select {
        case <-ctx.Done():
            // Client disconnected or timeout
            log.Printf("Processing cancelled at record %d: %v", i, ctx.Err())
            return ctx.Err()
        default:
            // Continue processing
            if err := s.processRecord(ctx, record); err != nil {
                return err
            }
        }
    }
    
    return nil
}
```

### Timeout Handling

```go
// Middleware for request timeouts
func timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx, cancel := context.WithTimeout(r.Context(), timeout)
            defer cancel()
            
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Handler respecting timeouts
func (s *SearchService) Search(ctx context.Context, query string) ([]*Result, error) {
    // Create channels for results and errors
    resultsCh := make(chan []*Result, 1)
    errorCh := make(chan error, 1)
    
    go func() {
        results, err := s.performSearch(query)
        if err != nil {
            errorCh <- err
            return
        }
        resultsCh <- results
    }()
    
    select {
    case <-ctx.Done():
        return nil, fmt.Errorf("search timeout: %w", ctx.Err())
    case err := <-errorCh:
        return nil, err
    case results := <-resultsCh:
        return results, nil
    }
}
```

## 8. File Upload/Download

WebRPC doesn't have built-in binary streaming support, requiring base64 encoding or hybrid approaches.

### Base64 File Upload

```go
// Schema definition
type FileUploadRequest struct {
    Filename    string            `json:"filename"`
    ContentType string            `json:"contentType"`
    Data        string            `json:"data"` // base64
    Metadata    map[string]string `json:"metadata,omitempty"`
}

func (s *FileService) UploadFile(
    ctx context.Context,
    req *FileUploadRequest,
) (*FileResponse, error) {
    // Decode base64 data
    fileData, err := base64.StdEncoding.DecodeString(req.Data)
    if err != nil {
        return nil, ErrInvalidFileData.WithCause(err)
    }
    
    // Validate file size (base64 adds ~33% overhead)
    maxSize := int64(10 * 1024 * 1024) // 10MB decoded
    if int64(len(fileData)) > maxSize {
        return nil, ErrFileTooLarge
    }
    
    // Save file
    fileID := generateFileID()
    path := filepath.Join(s.uploadDir, fileID)
    
    if err := os.WriteFile(path, fileData, 0644); err != nil {
        return nil, ErrInternalError.WithCause(err)
    }
    
    // Store metadata
    s.storeFileMetadata(fileID, req.Filename, req.ContentType, req.Metadata)
    
    return &FileResponse{
        FileID: fileID,
        Size:   int64(len(fileData)),
        URL:    fmt.Sprintf("/files/%s", fileID),
    }, nil
}
```

### Chunked Upload for Large Files

```go
type ChunkUploadRequest struct {
    UploadID    string `json:"uploadId"`
    ChunkIndex  int    `json:"chunkIndex"`
    TotalChunks int    `json:"totalChunks"`
    Data        string `json:"data"` // base64 chunk
    IsLast      bool   `json:"isLast"`
}

func (s *FileService) UploadChunk(
    ctx context.Context,
    req *ChunkUploadRequest,
) (*ChunkResponse, error) {
    // Decode chunk
    chunkData, err := base64.StdEncoding.DecodeString(req.Data)
    if err != nil {
        return nil, ErrInvalidChunkData
    }
    
    // Save chunk to temporary location
    chunkPath := filepath.Join(
        s.tempDir,
        req.UploadID,
        fmt.Sprintf("chunk_%d", req.ChunkIndex),
    )
    
    if err := os.MkdirAll(filepath.Dir(chunkPath), 0755); err != nil {
        return nil, ErrInternalError
    }
    
    if err := os.WriteFile(chunkPath, chunkData, 0644); err != nil {
        return nil, ErrInternalError
    }
    
    // If last chunk, combine all chunks
    if req.IsLast {
        fileID, err := s.combineChunks(req.UploadID, req.TotalChunks)
        if err != nil {
            return nil, err
        }
        
        return &ChunkResponse{
            ChunkIndex: req.ChunkIndex,
            Completed:  true,
            FileID:     fileID,
        }, nil
    }
    
    return &ChunkResponse{
        ChunkIndex: req.ChunkIndex,
        Completed:  false,
    }, nil
}
```

### Hybrid Approach with Direct Binary Upload

```go
// 1. Create upload session via WebRPC
func (s *FileService) CreateUploadSession(
    ctx context.Context,
    req *CreateSessionRequest,
) (*UploadSession, error) {
    sessionID := generateSessionID()
    uploadToken := generateSecureToken()
    
    s.sessions.Store(sessionID, &sessionData{
        Token:       uploadToken,
        Filename:    req.Filename,
        ContentType: req.ContentType,
        ExpiresAt:   time.Now().Add(1 * time.Hour),
    })
    
    return &UploadSession{
        SessionID:   sessionID,
        UploadURL:   fmt.Sprintf("/upload/%s", sessionID),
        UploadToken: uploadToken,
    }, nil
}

// 2. Handle binary upload separately
func (s *FileService) HandleBinaryUpload(w http.ResponseWriter, r *http.Request) {
    sessionID := chi.URLParam(r, "sessionID")
    
    // Validate session
    data, ok := s.sessions.Load(sessionID)
    if !ok {
        http.Error(w, "Invalid session", http.StatusBadRequest)
        return
    }
    
    session := data.(*sessionData)
    if r.Header.Get("X-Upload-Token") != session.Token {
        http.Error(w, "Invalid token", http.StatusUnauthorized)
        return
    }
    
    // Handle multipart upload
    file, header, err := r.ParseMultipartForm(32 << 20) // 32MB max
    if err != nil {
        http.Error(w, "Failed to parse form", http.StatusBadRequest)
        return
    }
    
    // Process file...
}
```

## 9. Testing Strategies

Comprehensive testing approaches for WebRPC services.

### Unit Testing Services

```go
func TestUserService_GetUser(t *testing.T) {
    // Setup
    mockDB := &MockDatabase{
        users: map[uint64]*User{
            1: {ID: 1, Username: "alice", Email: "alice@example.com"},
            2: {ID: 2, Username: "bob", Email: "bob@example.com"},
        },
    }
    
    service := &UserService{db: mockDB}
    
    tests := []struct {
        name     string
        userID   uint64
        wantUser *User
        wantErr  error
    }{
        {
            name:     "existing user",
            userID:   1,
            wantUser: mockDB.users[1],
            wantErr:  nil,
        },
        {
            name:     "non-existent user",
            userID:   999,
            wantUser: nil,
            wantErr:  ErrUserNotFound,
        },
        {
            name:     "invalid user ID",
            userID:   0,
            wantUser: nil,
            wantErr:  ErrValidationFailed,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            user, err := service.GetUser(context.Background(), tt.userID)
            
            if err != tt.wantErr {
                t.Errorf("GetUser() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if !reflect.DeepEqual(user, tt.wantUser) {
                t.Errorf("GetUser() = %v, want %v", user, tt.wantUser)
            }
        })
    }
}
```

### Integration Testing

```go
func TestIntegration_UserService(t *testing.T) {
    // Create test server
    service := &UserService{db: NewInMemoryDB()}
    handler := NewUserServiceServer(service)
    server := httptest.NewServer(handler)
    defer server.Close()
    
    // Create client
    client := NewUserServiceClient(server.URL, &http.Client{})
    
    // Test user creation
    newUser := &User{
        Username: "testuser",
        Email:    "test@example.com",
    }
    
    created, err := client.CreateUser(context.Background(), newUser)
    if err != nil {
        t.Fatalf("CreateUser failed: %v", err)
    }
    
    // Test retrieval
    retrieved, err := client.GetUser(context.Background(), created.ID)
    if err != nil {
        t.Fatalf("GetUser failed: %v", err)
    }
    
    if retrieved.Username != newUser.Username {
        t.Errorf("Username = %v, want %v", retrieved.Username, newUser.Username)
    }
}
```

### Mock Client for Testing

```go
type MockUserServiceClient struct {
    GetUserFunc    func(ctx context.Context, userID uint64) (*User, error)
    CreateUserFunc func(ctx context.Context, user *User) (*User, error)
}

func (m *MockUserServiceClient) GetUser(ctx context.Context, userID uint64) (*User, error) {
    if m.GetUserFunc != nil {
        return m.GetUserFunc(ctx, userID)
    }
    return nil, nil
}

// Use in tests
func TestUserHandler(t *testing.T) {
    mockClient := &MockUserServiceClient{
        GetUserFunc: func(ctx context.Context, userID uint64) (*User, error) {
            return &User{ID: userID, Username: "mock"}, nil
        },
    }
    
    handler := NewUserHandler(mockClient)
    // Test handler logic...
}
```

### Load Testing

```go
func TestLoad_UserService(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping load test")
    }
    
    server := setupTestServer()
    defer server.Close()
    
    client := NewUserServiceClient(server.URL, &http.Client{
        Transport: &http.Transport{
            MaxIdleConns:        100,
            MaxIdleConnsPerHost: 20,
        },
    })
    
    const (
        numWorkers    = 50
        requestsEach  = 1000
    )
    
    start := time.Now()
    var wg sync.WaitGroup
    errors := make(chan error, numWorkers*requestsEach)
    
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for j := 0; j < requestsEach; j++ {
                _, err := client.GetUser(context.Background(), uint64(j))
                if err != nil {
                    errors <- err
                }
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    elapsed := time.Since(start)
    totalRequests := numWorkers * requestsEach
    rps := float64(totalRequests) / elapsed.Seconds()
    
    errorCount := len(errors)
    errorRate := float64(errorCount) / float64(totalRequests) * 100
    
    t.Logf("Load test results:")
    t.Logf("- Total requests: %d", totalRequests)
    t.Logf("- Duration: %v", elapsed)
    t.Logf("- Requests/second: %.2f", rps)
    t.Logf("- Error rate: %.2f%%", errorRate)
    
    if errorRate > 1.0 {
        t.Errorf("Error rate too high: %.2f%%", errorRate)
    }
}
```

## 10. Performance Tuning

Optimizing WebRPC services for production workloads.

### Connection Pooling Configuration

```go
// Optimized HTTP client for WebRPC
func NewOptimizedHTTPClient() *http.Client {
    return &http.Client{
        Transport: &http.Transport{
            // Connection pooling
            MaxIdleConns:        100,
            MaxIdleConnsPerHost: 20,
            MaxConnsPerHost:     50,
            IdleConnTimeout:     90 * time.Second,
            
            // Timeouts
            TLSHandshakeTimeout:   10 * time.Second,
            ExpectContinueTimeout: 1 * time.Second,
            
            // Keep-alive
            DialContext: (&net.Dialer{
                Timeout:   30 * time.Second,
                KeepAlive: 30 * time.Second,
            }).DialContext,
            
            // Enable HTTP/2
            ForceAttemptHTTP2: true,
            
            // Compression
            DisableCompression: false,
        },
        Timeout: 30 * time.Second,
    }
}

// Use with WebRPC client
client := NewUserServiceClient(baseURL, NewOptimizedHTTPClient())
```

### Request/Response Compression

```go
// Gzip compression middleware
type gzipResponseWriter struct {
    io.Writer
    http.ResponseWriter
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
    return w.Writer.Write(b)
}

func compressionMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Check if client accepts gzip
        if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
            next.ServeHTTP(w, r)
            return
        }
        
        // Set compression headers
        w.Header().Set("Content-Encoding", "gzip")
        w.Header().Del("Content-Length") // Remove as it will change
        
        // Create gzip writer
        gz := gzip.NewWriter(w)
        defer gz.Close()
        
        // Wrap response writer
        gzw := &gzipResponseWriter{Writer: gz, ResponseWriter: w}
        
        next.ServeHTTP(gzw, r)
    })
}
```

### Memory Optimization

```go
// Object pooling to reduce GC pressure
var bufferPool = sync.Pool{
    New: func() interface{} {
        return bytes.NewBuffer(make([]byte, 0, 4096))
    },
}

var userPool = sync.Pool{
    New: func() interface{} {
        return &User{}
    },
}

func (s *UserService) GetUser(ctx context.Context, userID uint64) (*User, error) {
    // Get user from pool
    user := userPool.Get().(*User)
    
    // Reset fields
    *user = User{}
    
    // Populate user
    if err := s.db.GetUser(userID, user); err != nil {
        userPool.Put(user) // Return to pool on error
        return nil, err
    }
    
    // Note: Caller should return to pool after use
    return user, nil
}
```

### Performance Monitoring

```go
// Comprehensive metrics middleware
func metricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Parse method from path
        path := r.URL.Path
        parts := strings.Split(path, "/")
        method := "unknown"
        if len(parts) >= 3 {
            method = fmt.Sprintf("%s.%s", parts[1], parts[2])
        }
        
        // Track in-flight requests
        inflightRequests.Inc()
        defer inflightRequests.Dec()
        
        // Wrap response writer
        wrapped := &responseWriter{
            ResponseWriter: w,
            statusCode:     200,
        }
        
        // Process request
        next.ServeHTTP(wrapped, r)
        
        // Record metrics
        duration := time.Since(start)
        requestDuration.WithLabelValues(method, strconv.Itoa(wrapped.statusCode)).
            Observe(duration.Seconds())
        requestsTotal.WithLabelValues(method, strconv.Itoa(wrapped.statusCode)).
            Inc()
        requestSize.WithLabelValues(method).
            Observe(float64(r.ContentLength))
        responseSize.WithLabelValues(method).
            Observe(float64(wrapped.size))
    })
}
```

### Production Configuration Example

```go
func setupProductionServer() *http.Server {
    // Service setup
    service := &UserService{
        db:    productionDB,
        cache: redis.NewClient(&redis.Options{
            Addr:         "localhost:6379",
            PoolSize:     100,
            MinIdleConns: 10,
        }),
    }
    
    // Router with middleware
    r := chi.NewRouter()
    
    // Standard middleware
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Recoverer)
    
    // Custom middleware
    r.Use(compressionMiddleware)
    r.Use(metricsMiddleware)
    r.Use(rateLimitMiddleware(1000)) // 1000 req/s
    r.Use(timeoutMiddleware(30 * time.Second))
    
    // WebRPC handler
    webrpcHandler := NewUserServiceServer(service)
    r.Handle("/*", webrpcHandler)
    
    // Server configuration
    return &http.Server{
        Addr:         ":8080",
        Handler:      r,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
        
        // Connection limits
        MaxHeaderBytes: 1 << 20, // 1MB
    }
}
```

## Streaming Support (Experimental)

While the main documentation states WebRPC has no streaming support, there is experimental Server-Sent Events (SSE) support using the `stream` keyword:

```ridl
# Experimental streaming syntax
service ChatService
  - SendMessage(username: string, text: string)
  - SubscribeMessages(username: string) => stream (message: Message)
```

This generates SSE endpoints but should be considered experimental and not relied upon for production use.

## Common Patterns and Best Practices

### 1. Header Passing Pattern

For passing request headers (auth tokens, request IDs, etc):

```ridl
service APIService
  - GetData(headers: map<string,string>, id: uint64) => (data: Data)
```

### 2. Batch Operations

```ridl
struct BatchRequest
  - ids: []uint64
  - options?: BatchOptions

struct BatchResponse  
  - results: map<uint64,Result>
  - errors: map<uint64,string>

service BatchService
  - BatchGet(req: BatchRequest) => (resp: BatchResponse)
```

### 3. Pagination Pattern

```ridl
struct PageRequest
  - page?: uint32      # Default 1
  - pageSize?: uint32  # Default 20
  - sort?: string
  - filter?: map<string,string>

struct PageResponse
  - items: []Item
  - total: uint64
  - page: uint32
  - pageSize: uint32
  - hasNext: bool
```

### 4. Async Operations Pattern

```ridl
struct AsyncOperation
  - id: string
  - status: OperationStatus  # enum: pending, processing, completed, failed
  - result?: any
  - error?: string
  - createdAt: timestamp
  - updatedAt: timestamp

service AsyncService
  - StartOperation(req: OperationRequest) => (operation: AsyncOperation)
  - GetOperation(id: string) => (operation: AsyncOperation)
  - CancelOperation(id: string) => (success: bool)
```

## Migration Tips from Other Systems

### From REST APIs
- Think in methods, not resources
- Use service methods like `GetUser`, not `GET /users/:id`  
- Batch operations are natural: `BatchGetUsers(ids: []uint64)`

### From gRPC
- No .proto files, use simpler RIDL
- No streaming (except experimental SSE)
- JSON instead of Protocol Buffers
- No field numbers or binary compatibility

### From GraphQL
- No query language, just method calls
- Simpler error handling
- No resolver complexity
- Type safety without runtime overhead

## CLI Options and Customization

The `webrpc-gen` tool supports various options:

```bash
# Custom package name (Go)
webrpc-gen -schema=api.ridl -target=golang -pkg=myapi -out=./myapi.gen.go

# Generate only server (no client)
webrpc-gen -schema=api.ridl -target=golang -server -out=./server.gen.go

# Custom import path (Go modules)
webrpc-gen -schema=api.ridl -target=golang \
  -import-path=github.com/mycompany/myapp/api \
  -out=./api.gen.go

# TypeScript with custom options
webrpc-gen -schema=api.ridl -target=typescript \
  -client \
  -namePrefix=My \
  -out=./my-client.gen.ts
```

## Conclusion

WebRPC provides a powerful yet simple framework for building type-safe RPC services. Its integration with standard HTTP/JSON makes it accessible while maintaining the benefits of schema-driven development. The patterns and examples provided in this documentation cover the essential aspects of building production-ready WebRPC services, from basic routing to advanced performance optimization.

Key takeaways include:
- Schema-first design with RIDL
- Automatic code generation for type safety
- Standard HTTP/JSON for easy debugging
- Flexible middleware integration
- Manual retry/timeout implementation required
- Experimental streaming via SSE
- Strong integration with go-chi router

By following these patterns and best practices, developers can build robust, scalable, and maintainable RPC services with WebRPC, particularly well-suited for internal services and applications where you control both client and server.