#!/bin/bash
set -e

echo "Building archive-images tools..."
echo ""

# Build archive-images
echo "→ Building archive-images..."
go build -o archive-images ./cmd/archive-images
echo "  ✓ archive-images"

# Build cleanup tool
echo "→ Building cleanup-tool..."
go build -o cleanup-tool ./cmd/cleanup
echo "  ✓ cleanup-tool"

echo ""
echo "Build complete!"
echo ""
echo "Available tools:"
echo "  ./archive-images  - Organize files into personal data categories"
echo "  ./cleanup-tool    - Remove unnecessary files (Downloads, installers, cache)"
echo ""
echo "For help, run:"
echo "  ./archive-images -help"
echo "  ./cleanup-tool -help"
