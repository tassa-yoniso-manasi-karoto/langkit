# WebRPC code generation Makefile

WEBRPC := webrpc-gen
SCHEMAS_DIR := schemas
GO_OUT_DIR := ../internal/api/generated
TS_OUT_DIR := ../internal/gui/frontend/src/api/generated

# Find all service schemas
SERVICE_SCHEMAS := $(wildcard $(SCHEMAS_DIR)/services/*.ridl)

.PHONY: all clean generate-go generate-ts validate

all: generate-go generate-ts

# Create output directories
$(GO_OUT_DIR) $(TS_OUT_DIR):
	mkdir -p $@

# Validate all schemas (by attempting to generate)
validate:
	@echo "Validating schemas..."
	@for schema in $(SERVICE_SCHEMAS); do \
		echo "Validating $$schema"; \
		schema_path=$$(pwd)/$$schema; \
		$(WEBRPC) -schema=$$schema_path -target=golang -pkg=generated -server -client -out=/tmp/validate.tmp || exit 1; \
		rm -f /tmp/validate.tmp; \
	done
	@echo "All schemas valid!"

# Generate Go code
generate-go: $(GO_OUT_DIR)
	@echo "Generating Go code..."
	@for schema in $(SERVICE_SCHEMAS); do \
		service=$$(basename $$schema .ridl); \
		echo "Generating Go code for $$service"; \
		schema_path=$$(pwd)/$$schema; \
		$(WEBRPC) -schema=$$schema_path -target=golang -pkg=generated -server -client \
			-out=$(GO_OUT_DIR)/$${service}.gen.go || exit 1; \
	done
	@echo "Go code generation complete!"

# Generate TypeScript code
generate-ts: $(TS_OUT_DIR)
	@echo "Generating TypeScript code..."
	@for schema in $(SERVICE_SCHEMAS); do \
		service=$$(basename $$schema .ridl); \
		echo "Generating TypeScript code for $$service"; \
		schema_path=$$(pwd)/$$schema; \
		$(WEBRPC) -schema=$$schema_path -target=typescript -client \
			-out=$(TS_OUT_DIR)/$${service}.gen.ts || exit 1; \
	done
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