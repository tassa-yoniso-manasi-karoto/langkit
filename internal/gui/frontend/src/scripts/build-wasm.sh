#!/bin/bash
# build-wasm.sh - Build script for WebAssembly module

set -e  # Exit immediately if a command exits with a non-zero status

# Text formatting 
BOLD="\033[1m"
GREEN="\033[0;32m"
YELLOW="\033[0;33m"
RED="\033[0;31m"
CYAN="\033[0;36m"
RESET="\033[0m"

echo -e "${BOLD}${CYAN}Langkit WebAssembly Build Script${RESET}"
echo "======================="

# Check that wasm-pack is installed
if ! command -v wasm-pack &> /dev/null; then
    echo -e "${RED}Error: wasm-pack is not installed.${RESET}"
    echo "Please install it with: cargo install wasm-pack"
    exit 1
fi

# Check for Rust toolchain
if ! command -v rustc &> /dev/null; then
    echo -e "${RED}Error: Rust is not installed.${RESET}"
    echo "Please install it from https://rustup.rs/"
    exit 1
fi

# Ensure we have wasm32 target
if ! rustup target list | grep -q "wasm32-unknown-unknown (installed)"; then
    echo -e "${YELLOW}Adding wasm32-unknown-unknown target...${RESET}"
    rustup target add wasm32-unknown-unknown
fi

# Get script directory and frontend root
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
FRONTEND_ROOT="$( cd "$SCRIPT_DIR/../../" && pwd )"

# Create directories if they don't exist
WASM_SRC_DIR="$FRONTEND_ROOT/src/wasm"
WASM_OUT_DIR="$FRONTEND_ROOT/public/wasm"

echo "Frontend root: $FRONTEND_ROOT"
echo "WASM source directory: $WASM_SRC_DIR"
echo "WASM output directory: $WASM_OUT_DIR"

if [ ! -d "$WASM_SRC_DIR" ]; then
    echo -e "${RED}Error: Source directory $WASM_SRC_DIR does not exist.${RESET}"
    exit 1
fi

mkdir -p "$WASM_OUT_DIR"

# Move to the source directory
echo -e "${CYAN}Building WebAssembly module from $WASM_SRC_DIR...${RESET}"
cd "$WASM_SRC_DIR" || { echo -e "${RED}Error: Could not change to directory $WASM_SRC_DIR${RESET}"; exit 1; }

# Debug: List current directory contents
echo "Current directory: $(pwd)"
ls -la
echo "Looking for Cargo.toml..."
if [ ! -f "Cargo.toml" ]; then
    echo -e "${RED}Error: Cargo.toml not found in $(pwd)${RESET}"
    exit 1
fi
echo "Cargo.toml found!"

# Before building, extract version from Cargo.toml
VERSION=$(grep -m 1 '^version' Cargo.toml | sed 's/.*"\(.*\)".*/\1/')
if [ -z "$VERSION" ]; then
    VERSION="n/a"  # Default version if not found
fi

echo "VERSION: $VERSION"

# Build the WebAssembly module optimized for size
echo "Running wasm-pack build..."
# Try without the advanced flags first
wasm-pack build \
    --target web \
    --release

# Check if build was successful
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: WebAssembly build failed.${RESET}"
    cd - > /dev/null  # Return to original directory
    exit 1
fi

# Create build info file for cache busting and versioning
BUILD_TIMESTAMP=$(date +%s)
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

echo "Creating build info file..."
cat > pkg/build-info.json << EOF
{
  "version": "$VERSION",
  "timestamp": $BUILD_TIMESTAMP,
  "buildDate": "$BUILD_DATE"
}
EOF

# Copy the built files to the right location
echo "Copying WebAssembly files to $WASM_OUT_DIR..."
cp pkg/log_engine_bg.wasm "$WASM_OUT_DIR"/
cp pkg/log_engine.js "$WASM_OUT_DIR"/
cp pkg/build-info.json "$WASM_OUT_DIR"/

# Return to original directory
cd - > /dev/null

# Log the size of the WebAssembly file
WASM_FILE="$WASM_OUT_DIR/log_engine_bg.wasm"
if [ -f "$WASM_FILE" ]; then
    WASM_SIZE=$(du -h "$WASM_FILE" | cut -f1)
    WASM_SIZE_BYTES=$(du -b "$WASM_FILE" | cut -f1)
    
    echo -e "${GREEN}WebAssembly module built successfully!${RESET}"
    echo -e "Version: ${CYAN}$VERSION${RESET}"
    echo -e "Size: ${CYAN}$WASM_SIZE${RESET} ($WASM_SIZE_BYTES bytes)"
    echo -e "Output directory: ${CYAN}$WASM_OUT_DIR${RESET}"
    echo -e "Build timestamp: ${CYAN}$BUILD_DATE${RESET}"
else
    echo -e "${RED}Error: WebAssembly file not found after build.${RESET}"
    exit 1
fi

# Add to package.json scripts section
echo -e "\n${CYAN}To add this build script to your package.json:${RESET}"
echo -e '  "scripts": {'
echo -e '    "build:wasm": "bash ./scripts/build-wasm.sh",'
echo -e '    "dev:wasm": "nodemon --watch src/wasm -e rs --exec npm run build:wasm",'
echo -e '    "build": "npm run build:wasm && wails build",'
echo -e '    "dev": "concurrently \\"npm run dev:wasm\\" \\"wails dev\\""'
echo -e '  }'

echo -e "\n${GREEN}Done!${RESET}"