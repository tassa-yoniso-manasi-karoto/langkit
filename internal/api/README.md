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

1. Create a new RIDL schema in `/api/schemas/services/`
2. Run `make generate-go` from the `/api` directory
3. Implement the generated interface in `services/`
4. Register the service in `registry.go`

## Development

The Makefile supports additional commands:
- `make validate` - Validate all schemas without generating code
- `make clean` - Remove all generated files
- `make watch` - Auto-regenerate on schema changes (requires inotifywait)