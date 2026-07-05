#!/bin/bash
# 03_logging.sh: Food Logging, Stats, and Deletion

if [ -z "$BASE_URL" ]; then
    source "$(dirname "$0")/common.sh"
fi

if [ -z "$RECIPE_ID" ] || [ -z "$MILK_ID" ]; then
    log_err "Required IDs (RECIPE_ID, MILK_ID) not set. Run 01_basics.sh first."
    exit 1
fi

echo "==================================================="
echo "Test 2: Logging & Stats"
echo "---------------------------------------------------"
echo "Logging consumption..."
LOG_WS=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"food_id\": \"$RECIPE_ID\",
  \"amount\": 1.0,
  \"meal_tag\": \"snack\",
  \"logged_at\": \"$NOW_ISO\"
}")
LOG_ID=$(echo $LOG_WS | jq -r .id)
log_info "✅ Logged Entry ID: $LOG_ID"

echo "Checking Stats..."
STATS=$(curl -s "$BASE_URL/stats?period=day")
TOTAL_CAL=$(echo $STATS | jq -r .calories)
# Float verify
MATCH=$(echo "$TOTAL_CAL 173" | awk '{if ($1 >= 172.9 && $1 <= 173.1) print 1; else print 0}')
if [ "$MATCH" -eq 1 ]; then
     log_info "✅ Stats Correct: ~$TOTAL_CAL kcal"
else
     log_err "Stats Incorrect: Expected 173, got $TOTAL_CAL"
     echo $STATS | jq .
     exit 1
fi

echo "==================================================="
echo "Test 4: Calories Only Log"
echo "---------------------------------------------------"
echo "Logging 500 calories (quick add)..."
curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"calories\": 500,
  \"amount\": 1,
  \"meal_tag\": \"snack\",
  \"logged_at\": \"$NOW_ISO\"
}" > /dev/null

echo "Checking Stats (Should increase)..."
STATS_QUICK=$(curl -s "$BASE_URL/stats?period=day")
TOTAL_CAL_QUICK=$(echo $STATS_QUICK | jq -r .calories)
# Expected: 173 + 500 = 673
MATCH_QUICK=$(echo "$TOTAL_CAL_QUICK 673" | awk '{if ($1 >= 672.9 && $1 <= 673.1) print 1; else print 0}')
if [ "$MATCH_QUICK" -eq 1 ]; then
     log_info "✅ Stats Correct (Quick add works): ~$TOTAL_CAL_QUICK kcal"
else
     log_err "Stats Incorrect after quick add: Expected 673, got $TOTAL_CAL_QUICK"
     echo $STATS_QUICK | jq .
     exit 1
fi

echo "==================================================="
echo "Test 7: Log Deletion + Stats Recalculation"
echo "---------------------------------------------------"
echo "Creating trackable food..."
TRACK_FOOD=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Tracker Apple",
  "calories": 52,
  "protein": 0.3,
  "carbs": 14,
  "fat": 0.2,
  "type": "food",
  "measurement_unit": "g",
  "measurement_amount": 100
}')
TRACK_ID=$(echo $TRACK_FOOD | jq -r .id)
log_info "✅ Created Tracker Apple: $TRACK_ID"

echo "Getting baseline stats..."
BASELINE_STATS=$(curl -s "$BASE_URL/stats?period=day")
BASELINE_CAL=$(echo $BASELINE_STATS | jq -r .calories)
echo "Baseline calories: $BASELINE_CAL"

echo "Logging 200g of Tracker Apple..."
TRACK_LOG=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"food_id\": \"$TRACK_ID\",
  \"amount\": 200,
  \"meal_tag\": \"lunch\",
  \"logged_at\": \"$NOW_ISO\"
}")
TRACK_LOG_ID=$(echo $TRACK_LOG | jq -r .id)
log_info "✅ Logged entry: $TRACK_LOG_ID"

echo "Verifying stats increased..."
AFTER_LOG_STATS=$(curl -s "$BASE_URL/stats?period=day")
AFTER_LOG_CAL=$(echo $AFTER_LOG_STATS | jq -r .calories)
# 200g of 52cal/100g = 104 cal added
EXPECTED_AFTER=$(echo "$BASELINE_CAL + 104" | bc)
MATCH_AFTER=$(echo "$AFTER_LOG_CAL $EXPECTED_AFTER" | awk '{d=$1-$2; if(d<0)d=-d; if(d<=0.1)print 1; else print 0}')
if [ "$MATCH_AFTER" -eq 1 ]; then
    log_info "✅ Stats increased correctly: $AFTER_LOG_CAL kcal"
else
    log_err "Expected ~$EXPECTED_AFTER, got $AFTER_LOG_CAL"
    exit 1
fi

echo "Deleting the log entry..."
DEL_LOG_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/logs/$TRACK_LOG_ID")
if [ "$DEL_LOG_CODE" == "204" ]; then
    log_info "✅ Log delete returned 204"
else
    log_err "Expected 204, got $DEL_LOG_CODE"
    exit 1
fi

echo "Verifying stats decreased back..."
AFTER_DEL_STATS=$(curl -s "$BASE_URL/stats?period=day")
AFTER_DEL_CAL=$(echo $AFTER_DEL_STATS | jq -r .calories)
MATCH_BASELINE=$(echo "$AFTER_DEL_CAL $BASELINE_CAL" | awk '{d=$1-$2; if(d<0)d=-d; if(d<=0.1)print 1; else print 0}')
if [ "$MATCH_BASELINE" -eq 1 ]; then
    log_info "✅ Stats returned to baseline: $AFTER_DEL_CAL kcal"
else
    log_err "Expected ~$BASELINE_CAL, got $AFTER_DEL_CAL"
    exit 1
fi

# Clean up temp food
# curl -s -X DELETE "$BASE_URL/foods/$TRACK_ID" > /dev/null

echo "Deleting non-existent log (random UUID)..."
DEL_GHOST_LOG=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/logs/00000000-0000-0000-0000-000000000099")
echo "Delete non-existent log returned: $DEL_GHOST_LOG (soft-delete affects 0 rows)"

echo "Invalid UUID for log delete..."
DEL_BAD_UUID=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/logs/not-a-uuid")
if [ "$DEL_BAD_UUID" == "400" ]; then
    log_info "✅ Invalid UUID for log delete returns 400"
else
    log_err "Expected 400, got $DEL_BAD_UUID"
fi

echo "==================================================="
echo "Test 13: Double Logging & Accumulation"
echo "---------------------------------------------------"
echo "Getting current stats baseline..."
ACCUM_BASELINE=$(curl -s "$BASE_URL/stats?period=day")
ACCUM_BASE_CAL=$(echo $ACCUM_BASELINE | jq -r .calories)

echo "Logging Milk twice (100ml each)..."
LOG_A=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"food_id\": \"$MILK_ID\",
  \"amount\": 100,
  \"meal_tag\": \"breakfast\",
  \"logged_at\": \"$NOW_ISO\"
}")
LOG_A_ID=$(echo $LOG_A | jq -r .id)

LOG_B=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"food_id\": \"$MILK_ID\",
  \"amount\": 100,
  \"meal_tag\": \"snack\",
  \"logged_at\": \"$NOW_ISO\"
}")
LOG_B_ID=$(echo $LOG_B | jq -r .id)

echo "Checking stats accumulated correctly..."
ACCUM_STATS=$(curl -s "$BASE_URL/stats?period=day")
ACCUM_CAL=$(echo $ACCUM_STATS | jq -r .calories)
# Milk: 42 cal per 100ml. Two logs of 100ml = 84 cal added
EXPECTED_ACCUM=$(echo "$ACCUM_BASE_CAL + 84" | bc)
MATCH_ACCUM=$(echo "$ACCUM_CAL $EXPECTED_ACCUM" | awk '{d=$1-$2; if(d<0)d=-d; if(d<=0.1)print 1; else print 0}')
if [ "$MATCH_ACCUM" -eq 1 ]; then
    log_info "✅ Double-log accumulated correctly: $ACCUM_CAL kcal (expected ~$EXPECTED_ACCUM)"
else
    log_err "Expected ~$EXPECTED_ACCUM, got $ACCUM_CAL"
    exit 1
fi

echo "Verifying protein also accumulated..."
ACCUM_PROT=$(echo $ACCUM_STATS | jq -r .protein)
ACCUM_BASE_PROT=$(echo $ACCUM_BASELINE | jq -r .protein)
EXPECTED_PROT=$(echo "$ACCUM_BASE_PROT + 6.8" | bc)
MATCH_PROT=$(echo "$ACCUM_PROT $EXPECTED_PROT" | awk '{d=$1-$2; if(d<0)d=-d; if(d<=0.1)print 1; else print 0}')
if [ "$MATCH_PROT" -eq 1 ]; then
    log_info "✅ Protein accumulated: $ACCUM_PROT g (expected ~$EXPECTED_PROT)"
else
    log_err "Expected protein ~$EXPECTED_PROT, got $ACCUM_PROT"
fi

# Clean up double logs (keep cleanup to avoid affecting subsequent stats tests?)
# Actually Test 8 & 9 are read-only stats tests, so existing data + these logs is fine.
# Test 3 in edge cases does negative calories but uses separate food.
# I'll leave them or delete them. Original script deleted them at end of section.
curl -s -X DELETE "$BASE_URL/logs/$LOG_A_ID" > /dev/null
curl -s -X DELETE "$BASE_URL/logs/$LOG_B_ID" > /dev/null

echo "==================================================="
echo "Test 14: GET /logs returns entries in chronological order"
echo "---------------------------------------------------"
TODAY_DATE=$(date -u +"%Y-%m-%d")
LOGGED_LATE="${TODAY_DATE}T20:00:00Z"
LOGGED_EARLY="${TODAY_DATE}T06:00:00Z"
LOGGED_MID="${TODAY_DATE}T12:00:00Z"

echo "Creating logs out of chronological order (20:00, then 06:00, then 12:00)..."
ORDER_LOG_1=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{\"food_id\": \"$MILK_ID\", \"amount\": 100, \"meal_tag\": \"dinner\", \"logged_at\": \"$LOGGED_LATE\"}")
ORDER_LOG_1_ID=$(echo $ORDER_LOG_1 | jq -r .id)

ORDER_LOG_2=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{\"food_id\": \"$MILK_ID\", \"amount\": 100, \"meal_tag\": \"breakfast\", \"logged_at\": \"$LOGGED_EARLY\"}")
ORDER_LOG_2_ID=$(echo $ORDER_LOG_2 | jq -r .id)

ORDER_LOG_3=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{\"food_id\": \"$MILK_ID\", \"amount\": 100, \"meal_tag\": \"lunch\", \"logged_at\": \"$LOGGED_MID\"}")
ORDER_LOG_3_ID=$(echo $ORDER_LOG_3 | jq -r .id)

echo "Fetching logs for today and checking order..."
ORDER_LOGS=$(curl -s "$BASE_URL/logs?date=$TODAY_DATE")
ORDER_IDS=$(echo "$ORDER_LOGS" | jq -r --arg a "$ORDER_LOG_1_ID" --arg b "$ORDER_LOG_2_ID" --arg c "$ORDER_LOG_3_ID" \
  '[.[] | select(.id == $a or .id == $b or .id == $c)] | .[].id')
EXPECTED_ORDER=$(printf "%s\n%s\n%s" "$ORDER_LOG_2_ID" "$ORDER_LOG_3_ID" "$ORDER_LOG_1_ID")

if [ "$ORDER_IDS" == "$EXPECTED_ORDER" ]; then
    log_info "✅ Logs returned in chronological order by logged_at"
else
    log_err "Logs not in chronological order. Expected: [$EXPECTED_ORDER], Got: [$ORDER_IDS]"
    exit 1
fi

curl -s -X DELETE "$BASE_URL/logs/$ORDER_LOG_1_ID" > /dev/null
curl -s -X DELETE "$BASE_URL/logs/$ORDER_LOG_2_ID" > /dev/null
curl -s -X DELETE "$BASE_URL/logs/$ORDER_LOG_3_ID" > /dev/null
