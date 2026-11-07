#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${1:-${BASE_URL:-http://localhost:8080}}"
METRICS_URL="${2:-${METRICS_URL:-http://localhost:9090}}"

# Strip trailing slash if present
BASE_URL="${BASE_URL%/}"
METRICS_URL="${METRICS_URL%/}"

echo "Targeting API at: $BASE_URL"
echo "Targeting Metrics at: $METRICS_URL"

echo "1. Testing /healthz and /api/v1/healthz..."
RESP_HEALTH1=$(curl -sSf "${BASE_URL}/healthz")
echo "$RESP_HEALTH1" | grep -q '"status":"ok"'
RESP_HEALTH2=$(curl -sSf "${BASE_URL}/api/v1/healthz")
echo "$RESP_HEALTH2" | grep -q '"status":"ok"'
echo "   -> PASS"

echo "2. Testing /readyz and /api/v1/readyz..."
RESP_READYZ1=$(curl -sSf "${BASE_URL}/readyz")
echo "$RESP_READYZ1" | grep -q '"status":"ready"'
RESP_READYZ2=$(curl -sSf "${BASE_URL}/api/v1/readyz")
echo "$RESP_READYZ2" | grep -q '"status":"ready"'
echo "   -> PASS"

echo "3. Testing X-Request-Id echo..."
RES_ID=$(curl -sI -H "X-Request-Id: smoke-test-id" "${BASE_URL}/healthz" | tr -d '\r' | awk -F': ' 'tolower($1)=="x-request-id" {print $2}')
if [ "$RES_ID" != "smoke-test-id" ]; then
    echo "ERROR: Expected X-Request-Id 'smoke-test-id', got '$RES_ID'"
    exit 1
fi
echo "   -> PASS (echoed: $RES_ID)"

echo "4. Testing X-Calorize-Build header..."
BUILD_HDR=$(curl -sI "${BASE_URL}/healthz" | tr -d '\r' | awk -F': ' 'tolower($1)=="x-calorize-build" {print $2}')
if [ -z "$BUILD_HDR" ]; then
    echo "ERROR: Expected non-empty X-Calorize-Build header"
    exit 1
fi
echo "   -> PASS (build: $BUILD_HDR)"

echo "5. Testing /metrics on METRICS_URL..."
METRICS_OUT=$(curl -sSf "${METRICS_URL}/metrics")
echo "$METRICS_OUT" | grep -q "calorize_http_requests_total"
echo "   -> PASS"

echo "6. Testing POST /api/v1/client-events..."
EVENTS_RESP=$(curl -sSf -X POST "${BASE_URL}/api/v1/client-events" \
    -H "Content-Type: application/json" \
    -d '{"events":[{"kind":"js_error","message":"smoke test error?secret=token#frag","route":"/smoke-test","build":"smoke"}]}')
echo "$EVENTS_RESP" | grep -q '"status":"accepted"'
echo "   -> PASS"

echo "7. Testing 404 error format..."
# Test root-level nonexistent route
STATUS_404=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/nonexistent-route-smoke")
if [ "$STATUS_404" != "404" ]; then
    echo "ERROR: Expected 404 status code, got '$STATUS_404'"
    exit 1
fi
CT_404=$(curl -sI "${BASE_URL}/nonexistent-route-smoke" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type" {print $2}')
if ! echo "$CT_404" | grep -qi "application/json"; then
    echo "ERROR: Expected Content-Type application/json, got '$CT_404'"
    exit 1
fi
BODY_404=$(curl -s "${BASE_URL}/nonexistent-route-smoke")
echo "$BODY_404" | grep -q '"code":"NOT_FOUND"'

# Test api/v1 nonexistent route
STATUS_404_V1=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/api/v1/nonexistent-route-smoke")
if [ "$STATUS_404_V1" != "404" ]; then
    echo "ERROR: Expected 404 status code for /api/v1, got '$STATUS_404_V1'"
    exit 1
fi
CT_404_V1=$(curl -sI "${BASE_URL}/api/v1/nonexistent-route-smoke" | tr -d '\r' | awk -F': ' 'tolower($1)=="content-type" {print $2}')
if ! echo "$CT_404_V1" | grep -qi "application/json"; then
    echo "ERROR: Expected Content-Type application/json for /api/v1, got '$CT_404_V1'"
    exit 1
fi
BODY_404_V1=$(curl -s "${BASE_URL}/api/v1/nonexistent-route-smoke")
echo "$BODY_404_V1" | grep -q '"code":"NOT_FOUND"'
echo "   -> PASS"

echo "All telemetry smoke tests passed successfully!"
