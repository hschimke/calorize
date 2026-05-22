#!/bin/bash
# 10_weight_tracker.sh: Integration tests for weight tracker features

if [ -z "$BASE_URL" ]; then
    source "$(dirname "$0")/common.sh"
fi

echo "==================================================="
echo "Test: Weight Tracker Endpoints"
echo "---------------------------------------------------"

# Helper to verify HTTP status code
verify_status() {
    local got=$1
    local expected=$2
    local msg=$3
    if [ "$got" != "$expected" ]; then
        log_err "Expected $expected, got $got for $msg"
        exit 1
    fi
}

# Cleanup any pre-existing weight logs (just in case)
echo "Preparing test environment..."
CLEANUP_LOGS=$(curl -s "$BASE_URL/weight")
if is_valid_json "$CLEANUP_LOGS"; then
    CLEANUP_IDS=$(echo "$CLEANUP_LOGS" | jq -r '.[].id // empty')
    for id in $CLEANUP_IDS; do
        curl -s -X DELETE "$BASE_URL/weight/$id" > /dev/null
    done
fi

# Test 1: GET /account/profile has default weight_unit and weight_goal fields
echo "Checking profile weight defaults..."
RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/account/profile")
BODY=$(echo "$RESP" | sed '$d')
CODE=$(echo "$RESP" | tail -n 1)
verify_status "$CODE" "200" "GET /account/profile"

UNIT=$(echo "$BODY" | jq -r '.weight_unit')
if [ "$UNIT" != "kg" ]; then
    log_err "Expected default weight_unit to be 'kg', got '$UNIT'"
    exit 1
fi
GOAL=$(echo "$BODY" | jq -r '.weight_goal')
if [ "$GOAL" != "null" ]; then
    log_err "Expected default weight_goal to be null, got '$GOAL'"
    exit 1
fi
log_info "✅ Default profile values are correct (unit: kg, goal: null)"

# Test 2: PUT /account/profile updates goal and unit
echo "Updating profile weight settings..."
PUT_RESP=$(curl -s -w "\n%{http_code}" -X PUT "$BASE_URL/account/profile" \
  -H "Content-Type: application/json" \
  -d '{"weight_goal": 170.0, "weight_unit": "lbs"}')
PUT_BODY=$(echo "$PUT_RESP" | sed '$d')
PUT_CODE=$(echo "$PUT_RESP" | tail -n 1)
verify_status "$PUT_CODE" "200" "PUT /account/profile"

RETURNED_UNIT=$(echo "$PUT_BODY" | jq -r '.weight_unit')
RETURNED_GOAL=$(echo "$PUT_BODY" | jq -r '.weight_goal')
if [ "$RETURNED_UNIT" != "lbs" ] || [ "$RETURNED_GOAL" != "170" ]; then
    log_err "Failed to update profile. Unit: $RETURNED_UNIT, Goal: $RETURNED_GOAL"
    exit 1
fi
log_info "✅ Profile weight settings updated to 170.0 lbs"

# Test 3: POST /weight logs initial weight
echo "Logging initial weight (180.0 lbs on 2026-05-20)..."
LOG1_RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/weight" \
  -H "Content-Type: application/json" \
  -d '{"weight": 180.0, "unit": "lbs", "logged_at": "2026-05-20T12:00:00Z"}')
LOG1_BODY=$(echo "$LOG1_RESP" | sed '$d')
LOG1_CODE=$(echo "$LOG1_RESP" | tail -n 1)
verify_status "$LOG1_CODE" "200" "POST /weight log 1"

LOG1_ID=$(echo "$LOG1_BODY" | jq -r '.id')
if [ -z "$LOG1_ID" ] || [ "$LOG1_ID" == "null" ]; then
    log_err "Failed to get weight log ID: $LOG1_BODY"
    exit 1
fi
log_info "✅ First weight log created (ID: $LOG1_ID)"

# Test 4: POST /weight logs subsequent weight
echo "Logging second weight (176.0 lbs on 2026-05-22)..."
LOG2_RESP=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/weight" \
  -H "Content-Type: application/json" \
  -d '{"weight": 176.0, "unit": "lbs", "logged_at": "2026-05-22T12:00:00Z"}')
LOG2_BODY=$(echo "$LOG2_RESP" | sed '$d')
LOG2_CODE=$(echo "$LOG2_RESP" | tail -n 1)
verify_status "$LOG2_CODE" "200" "POST /weight log 2"

LOG2_ID=$(echo "$LOG2_BODY" | jq -r '.id')
if [ -z "$LOG2_ID" ] || [ "$LOG2_ID" == "null" ]; then
    log_err "Failed to get weight log ID: $LOG2_BODY"
    exit 1
fi
log_info "✅ Second weight log created (ID: $LOG2_ID)"

# Test 5: GET /weight lists all weight logs
echo "Listing weight logs..."
LIST_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/weight")
LIST_BODY=$(echo "$LIST_RESP" | sed '$d')
LIST_CODE=$(echo "$LIST_RESP" | tail -n 1)
verify_status "$LIST_CODE" "200" "GET /weight"

COUNT=$(echo "$LIST_BODY" | jq 'length')
if [ "$COUNT" != "2" ]; then
    log_err "Expected 2 weight logs, got $COUNT"
    exit 1
fi

# Logs should be in DESC order by logged_at, so index 0 is most recent (176.0)
FIRST_LOG_WEIGHT=$(echo "$LIST_BODY" | jq -r '.[0].weight')
SECOND_LOG_WEIGHT=$(echo "$LIST_BODY" | jq -r '.[1].weight')
if [ "$FIRST_LOG_WEIGHT" != "176" ] || [ "$SECOND_LOG_WEIGHT" != "180" ]; then
    log_err "Weight logs not sorted correctly. Index 0 weight: $FIRST_LOG_WEIGHT, Index 1 weight: $SECOND_LOG_WEIGHT"
    exit 1
fi
log_info "✅ GET /weight list returned sorted entries"

# Test 6: GET /weight/stats returns correct stats
echo "Reading weight stats..."
STATS_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/weight/stats")
STATS_BODY=$(echo "$STATS_RESP" | sed '$d')
STATS_CODE=$(echo "$STATS_RESP" | tail -n 1)
verify_status "$STATS_CODE" "200" "GET /weight/stats"

CURR_W=$(echo "$STATS_BODY" | jq -r '.current_weight')
START_W=$(echo "$STATS_BODY" | jq -r '.start_weight')
CHANGE=$(echo "$STATS_BODY" | jq -r '.weight_change')
PROGRESS=$(echo "$STATS_BODY" | jq -r '.goal_progress')
STATS_UNIT=$(echo "$STATS_BODY" | jq -r '.weight_unit')

if [ "$CURR_W" != "176" ] || [ "$START_W" != "180" ] || [ "$CHANGE" != "-4" ] || [ "$PROGRESS" != "40" ] || [ "$STATS_UNIT" != "lbs" ]; then
    log_err "Stats calculation error. Got: current=$CURR_W, start=$START_W, change=$CHANGE, progress=$PROGRESS, unit=$STATS_UNIT"
    exit 1
fi
log_info "✅ Weight stats calculations are correct (current: 176, start: 180, change: -4, progress: 40%)"

# Test 7: PUT /weight/{id} updates log
echo "Updating log 2 weight to 175.0 lbs..."
PUT_LOG_RESP=$(curl -s -w "\n%{http_code}" -X PUT "$BASE_URL/weight/$LOG2_ID" \
  -H "Content-Type: application/json" \
  -d '{"weight": 175.0, "unit": "lbs"}')
PUT_LOG_BODY=$(echo "$PUT_LOG_RESP" | sed '$d')
PUT_LOG_CODE=$(echo "$PUT_LOG_RESP" | tail -n 1)
verify_status "$PUT_LOG_CODE" "200" "PUT /weight/$LOG2_ID"

# Confirm updated stats
STATS_RESP2=$(curl -s "$BASE_URL/weight/stats")
CURR_W2=$(echo "$STATS_RESP2" | jq -r '.current_weight')
CHANGE2=$(echo "$STATS_RESP2" | jq -r '.weight_change')
PROGRESS2=$(echo "$STATS_RESP2" | jq -r '.goal_progress')

if [ "$CURR_W2" != "175" ] || [ "$CHANGE2" != "-5" ] || [ "$PROGRESS2" != "50" ]; then
    log_err "Stats calculation after update is incorrect. Got: current=$CURR_W2, change=$CHANGE2, progress=$PROGRESS2"
    exit 1
fi
log_info "✅ Weight log successfully updated (new stats current: 175, change: -5, progress: 50%)"

# Test 8: DELETE /weight/{id} soft-deletes a log entry
echo "Soft deleting weight log 2..."
DEL_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/weight/$LOG2_ID")
verify_status "$DEL_CODE" "204" "DELETE /weight/$LOG2_ID"

# Verify GET /weight lists only 1 active log
LIST_RESP2=$(curl -s "$BASE_URL/weight")
COUNT2=$(echo "$LIST_RESP2" | jq 'length')
if [ "$COUNT2" != "1" ]; then
    log_err "Expected 1 weight log after delete, got $COUNT2"
    exit 1
fi
REMAINING_ID=$(echo "$LIST_RESP2" | jq -r '.[0].id')
if [ "$REMAINING_ID" != "$LOG1_ID" ]; then
    log_err "Incorrect log remains. Expected $LOG1_ID, got $REMAINING_ID"
    exit 1
fi
log_info "✅ Weight log soft-deletion verified"

# Final Cleanup of remains
curl -s -X DELETE "$BASE_URL/weight/$LOG1_ID" > /dev/null
# Restore default profile values
curl -s -X PUT "$BASE_URL/account/profile" \
  -H "Content-Type: application/json" \
  -d '{"weight_goal": null, "weight_unit": "kg"}' > /dev/null

log_info "✅ Weight Tracker integration tests passed!"
