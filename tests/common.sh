#!/bin/bash

# Configuration
export BASE_URL="${BASE_URL:-http://localhost:8080/api/v1}"
export NOW_ISO=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_err()  { echo -e "${RED}[FAIL]${NC} $1"; }

# Check dependencies
if ! command -v jq &> /dev/null; then
    log_err "jq is not installed. Please install it (brew install jq)"
    exit 1
fi

# Function to check if server is up
check_server() {
    log_info "Targeting $BASE_URL"
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/healthz")
    if [ "$HTTP_CODE" != "200" ]; then
        log_err "Server is not running or not healthy at $BASE_URL (Status: $HTTP_CODE). Please start it."
        exit 1
    fi
}

# Helper to validate JSON response (prevents jq crashes on HTML errors)
# Usage: if is_valid_json "$RESPONSE"; then ...
is_valid_json() {
    echo "$1" | jq empty 2>/dev/null
}

# Cleanup function
cleanup_all() {
    log_info "Cleanup: Removing existing logs and foods..."
    
    # Delete Logs
    LOGS=$(curl -s "$BASE_URL/logs")
    if is_valid_json "$LOGS" && [ "$(echo "$LOGS" | jq 'type')" == '"array"' ]; then
        LOG_IDS=$(echo "$LOGS" | jq -r '.[].id // empty')
        for id in $LOG_IDS; do
            # curl -s -X DELETE "$BASE_URL/logs/$id" > /dev/null
            # Parallelize for speed? No, keep simple for now.
            curl -s -X DELETE "$BASE_URL/logs/$id" > /dev/null
        done
    fi

    # Delete Foods
    FOODS=$(curl -s "$BASE_URL/foods")
    if is_valid_json "$FOODS" && [ "$(echo "$FOODS" | jq 'type')" == '"array"' ]; then
        FOOD_IDS=$(echo "$FOODS" | jq -r '.[].id // empty')
        for id in $FOOD_IDS; do
            curl -s -X DELETE "$BASE_URL/foods/$id" > /dev/null
        done
    fi
    log_info "✅ Cleanup Complete"
}
