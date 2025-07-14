# WebRPC code generation Makefile

WEBRPC := webrpc-gen
SCHEMAS_DIR := schemas
GO_OUT_DIR := ../internal/api/generated
TS_OUT_DIR := ../internal/gui/frontend/src/api/generated

# Main API schema that imports all services
API_SCHEMA := $(SCHEMAS_DIR)/api.ridl

.PHONY: all clean generate-go generate-ts validate

all: generate-go generate-ts

# Create output directories
$(GO_OUT_DIR) $(TS_OUT_DIR):
	mkdir -p $@

# Validate the main API schema
validate:
	@echo "Validating API schema..."
	@schema_path=$$(pwd)/$(API_SCHEMA); \
	$(WEBRPC) -schema=$$schema_path -target=golang -pkg=generated -server -client -out=/tmp/validate.tmp || exit 1; \
	rm -f /tmp/validate.tmp
	@echo "API schema valid!"

# Generate Go code
generate-go: $(GO_OUT_DIR)
	@echo "Generating Go code..."
	@schema_path=$$(pwd)/$(API_SCHEMA); \
	$(WEBRPC) -schema=$$schema_path -target=golang -pkg=generated -server -client \
		-out=$(GO_OUT_DIR)/api.gen.go || exit 1
	@echo "Go code generation complete!"

# Generate TypeScript code
generate-ts: $(TS_OUT_DIR)
	@echo "Generating TypeScript code..."
	@schema_path=$$(pwd)/$(API_SCHEMA); \
	$(WEBRPC) -schema=$$schema_path -target=typescript -client \
		-out=$(TS_OUT_DIR)/api.gen.ts || exit 1
	@echo "TypeScript code generation complete!"

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	@rm -f $(GO_OUT_DIR)/*.gen.go
	@rm -f $(TS_OUT_DIR)/*.gen.ts
	@echo "Clean complete!"

# Watch for schema changes and regenerate
watch:
	@echo "Watching for schema changes..."
	@while true; do \
		$(MAKE) all; \
		inotifywait -qre close_write $(SCHEMAS_DIR); \
	done