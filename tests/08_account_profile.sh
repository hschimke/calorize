#!/bin/bash
# 08_account_profile.sh: Account profile (calorie goal) endpoints

if [ -z "$BASE_URL" ]; then
    source "$(dirname "$0")/common.sh"
fi

echo "==================================================="
echo "Test: Account Profile / Calorie Goal"
echo "---------------------------------------------------"

# Test 1: GET /account/profile returns 200 with calorie_goal field
echo "Fetching profile..."
PROF_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/account/profile")
PROF_BODY=$(echo "$PROF_RESP" | head -n -1)
PROF_CODE=$(echo "$PROF_RESP" | tail -n 1)

if [ "$PROF_CODE" != "200" ]; then
    log_err "Expected 200, got $PROF_CODE"
    exit 1
fi
if ! echo "$PROF_BODY" | jq -e 'has("calorie_goal")' > /dev/null 2>&1; then
    log_err "Response missing calorie_goal field: $PROF_BODY"
    exit 1
fi
log_info "✅ GET /account/profile → 200, has calorie_goal"

# Test 2: PUT /account/profile sets goal to 2000
echo "Setting calorie goal to 2000..."
PUT_RESP=$(curl -s -w "\n%{http_code}" -X PUT "$BASE_URL/account/profile" \
  -H "Content-Type: application/json" \
  -d '{"calorie_goal": 2000}')
PUT_BODY=$(echo "$PUT_RESP" | head -n -1)
PUT_CODE=$(echo "$PUT_RESP" | tail -n 1)

if [ "$PUT_CODE" != "200" ]; then
    log_err "Expected 200, got $PUT_CODE"
    exit 1
fi
RETURNED_GOAL=$(echo "$PUT_BODY" | jq '.calorie_goal')
if [ "$RETURNED_GOAL" != "2000" ]; then
    log_err "Expected calorie_goal=2000 in response, got $RETURNED_GOAL"
    exit 1
fi
log_info "✅ PUT /account/profile {calorie_goal: 2000} → 200"

# Test 3: GET confirms persisted value
echo "Verifying persisted goal..."
PROF2_BODY=$(curl -s "$BASE_URL/account/profile")
GOAL_VAL=$(echo "$PROF2_BODY" | jq '.calorie_goal')
if [ "$GOAL_VAL" != "2000" ]; then
    log_err "Expected persisted calorie_goal=2000, got $GOAL_VAL"
    exit 1
fi
log_info "✅ GET /account/profile after PUT → calorie_goal=2000"

# Test 4: PUT clears goal with null
echo "Clearing calorie goal..."
CLEAR_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE_URL/account/profile" \
  -H "Content-Type: application/json" \
  -d '{"calorie_goal": null}')
if [ "$CLEAR_CODE" != "200" ]; then
    log_err "Expected 200 on clear, got $CLEAR_CODE"
    exit 1
fi
PROF3_BODY=$(curl -s "$BASE_URL/account/profile")
GOAL_NULL=$(echo "$PROF3_BODY" | jq '.calorie_goal')
if [ "$GOAL_NULL" != "null" ]; then
    log_err "Expected calorie_goal=null after clear, got $GOAL_NULL"
    exit 1
fi
log_info "✅ PUT /account/profile {calorie_goal: null} clears goal"
