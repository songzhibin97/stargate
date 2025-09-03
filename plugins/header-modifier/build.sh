#!/bin/bash

# Build script for WASM plugin using TinyGo

set -e

echo "Building WASM plugin: header-modifier"

# Check if TinyGo is installed
if ! command -v tinygo &> /dev/null; then
    echo "TinyGo is not installed. Please install TinyGo first:"
    echo "https://tinygo.org/getting-started/install/"
    exit 1
fi

# Create output directory if it doesn't exist
mkdir -p ../../wasm-plugins

# Build the WASM module
echo "Compiling Go code to WASM..."
tinygo build -o ../../wasm-plugins/header-modifier.wasm -target wasi main.go

# Check if build was successful
if [ -f "../../wasm-plugins/header-modifier.wasm" ]; then
    echo " Successfully built header-modifier.wasm"
    echo "📁 Output: wasm-plugins/header-modifier.wasm"
    
    # Show file size
    size=$(wc -c < "../../wasm-plugins/header-modifier.wasm")
    echo "File size: $size bytes"
else
    echo " Build failed"
    exit 1
fi

echo "🎉 Build completed successfully!"
