#!/bin/bash
set -euo pipefail

REPO_DIR="sandbox/calorize"
COMPOSE_FILE="docker-compose.yml"
COMPOSE_DIR="docker/"

echo ">>> Pulling latest changes..."
cd "$REPO_DIR"
git pull

# Capture the git commit SHA to use as the PWA service worker cache version.
# This ensures clients re-download assets after every deploy.
BUILD_HASH=$(git rev-parse --short HEAD)
echo ">>> Build hash: $BUILD_HASH"

cd "$COMPOSE_DIR"

echo ">>> Building images..."
docker compose -f "$COMPOSE_FILE" build --build-arg BUILD_HASH="$BUILD_HASH"

echo ">>> Restarting services..."
docker compose -f "$COMPOSE_FILE" down
docker compose -f "$COMPOSE_FILE" up -d

echo ">>> Deploy complete (cache version: $BUILD_HASH)."
