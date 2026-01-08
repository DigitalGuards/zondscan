#!/bin/bash
# Build all Docker images locally
# Usage: ./scripts/docker-build.sh [--no-cache]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Parse arguments
NO_CACHE=""
if [[ "$1" == "--no-cache" ]]; then
    NO_CACHE="--no-cache"
    echo "Building without cache..."
fi

echo "========================================"
echo "Building Docker images for QRL Zond Explorer"
echo "========================================"
echo ""

# Build frontend
echo "[1/3] Building frontend image..."
docker build $NO_CACHE -t zond-explorer-frontend:latest "$PROJECT_ROOT/ExplorerFrontend"
echo "Frontend image built successfully!"
echo ""

# Build backend
echo "[2/3] Building backend image..."
docker build $NO_CACHE -t zond-explorer-backend:latest "$PROJECT_ROOT/backendAPI"
echo "Backend image built successfully!"
echo ""

# Build syncer
echo "[3/3] Building syncer image..."
docker build $NO_CACHE -t zond-explorer-syncer:latest "$PROJECT_ROOT/Zond2mongoDB"
echo "Syncer image built successfully!"
echo ""

echo "========================================"
echo "All images built successfully!"
echo "========================================"
echo ""
echo "Images created:"
docker images | grep zond-explorer
