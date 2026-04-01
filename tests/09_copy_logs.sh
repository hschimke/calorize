#!/bin/bash
# 09_copy_logs.sh: Copy logs between days

if [ -z "$BASE_URL" ]; then
    source "$(dirname "$0")/common.sh"
fi

echo "==================================================="
echo "Test: Copy Logs Between Days"
echo "---------------------------------------------------"

# Create a test food for this test
COPY_FOOD=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Copy Test Food",
  "calories": 100,
  "protein": 10,
  "carbs": 20,
  "fat": 5,
  "type": "food",
  "measurement_unit": "g",
  "measurement_amount": 100
}')
COPY_FOOD_ID=$(echo $COPY_FOOD | jq -r .id)
if [ -z "$COPY_FOOD_ID" ] || [ "$COPY_FOOD_ID" == "null" ]; then
    log_err "Failed to create Copy Test Food"
    exit 1
fi
log_info "✅ Created Copy Test Food: $COPY_FOOD_ID"

# Compute yesterday in YYYY-MM-DD (macOS + Linux compatible)
YESTERDAY=$(date -u -v-1d +"%Y-%m-%d" 2>/dev/null || date -u -d "yesterday" +"%Y-%m-%d")
YESTERDAY_ISO="${YESTERDAY}T12:00:00Z"
TODAY=$(date -u +"%Y-%m-%d")

# Log breakfast, lunch, and dinner on yesterday
LOG_BFAST=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{\"food_id\": \"$COPY_FOOD_ID\", \"amount\": 100, \"meal_tag\": \"breakfast\", \"logged_at\": \"$YESTERDAY_ISO\"}")
LOG_BFAST_ID=$(echo $LOG_BFAST | jq -r .id)
if [ -z "$LOG_BFAST_ID" ] || [ "$LOG_BFAST_ID" == "null" ]; then
    log_err "Failed to log breakfast entry"
    exit 1
fi
log_info "✅ Logged breakfast on $YESTERDAY: $LOG_BFAST_ID"

LOG_LUNCH=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{\"food_id\": \"$COPY_FOOD_ID\", \"amount\": 150, \"meal_tag\": \"lunch\", \"logged_at\": \"$YESTERDAY_ISO\"}")
LOG_LUNCH_ID=$(echo $LOG_LUNCH | jq -r .id)
if [ -z "$LOG_LUNCH_ID" ] || [ "$LOG_LUNCH_ID" == "null" ]; then
    log_err "Failed to log lunch entry"
    exit 1
fi
log_info "✅ Logged lunch on $YESTERDAY: $LOG_LUNCH_ID"

LOG_DINNER=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{\"food_id\": \"$COPY_FOOD_ID\", \"amount\": 200, \"meal_tag\": \"dinner\", \"logged_at\": \"$YESTERDAY_ISO\"}")
LOG_DINNER_ID=$(echo $LOG_DINNER | jq -r .id)
if [ -z "$LOG_DINNER_ID" ] || [ "$LOG_DINNER_ID" == "null" ]; then
    log_err "Failed to log dinner entry"
    exit 1
fi
log_info "✅ Logged dinner on $YESTERDAY: $LOG_DINNER_ID"

# Copy only breakfast and lunch to today
echo "Copying breakfast and lunch from $YESTERDAY to $TODAY..."
COPY_RESULT=$(curl -s -X POST "$BASE_URL/logs/copy" \
  -H "Content-Type: application/json" \
  -d "{\"from_date\": \"$YESTERDAY\", \"to_date\": \"$TODAY\", \"meal_tags\": [\"breakfast\", \"lunch\"]}")
COPY_COUNT=$(echo $COPY_RESULT | jq -r .count)
if [ "$COPY_COUNT" == "2" ]; then
    log_info "✅ Copy returned count: $COPY_COUNT"
else
    log_err "Expected count 2, got $COPY_COUNT"
    echo $COPY_RESULT | jq .
    exit 1
fi

# Verify entries appear on today
echo "Verifying copied entries appear on $TODAY..."
TODAY_LOGS=$(curl -s "$BASE_URL/logs?date=$TODAY")
BFAST_COUNT=$(echo $TODAY_LOGS | jq '[.[] | select(.meal_tag == "breakfast")] | length')
LUNCH_COUNT=$(echo $TODAY_LOGS | jq '[.[] | select(.meal_tag == "lunch")] | length')
if [ "$BFAST_COUNT" -ge 1 ] && [ "$LUNCH_COUNT" -ge 1 ]; then
    log_info "✅ Found breakfast ($BFAST_COUNT) and lunch ($LUNCH_COUNT) entries on $TODAY"
else
    log_err "Missing expected entries today: breakfast=$BFAST_COUNT lunch=$LUNCH_COUNT"
    echo $TODAY_LOGS | jq .
    exit 1
fi

DINNER_COUNT=$(echo $TODAY_LOGS | jq '[.[] | select(.meal_tag == "dinner")] | length')
if [ "$DINNER_COUNT" -ne 0 ]; then
    log_err "Dinner should not have been copied, found $DINNER_COUNT entries"
    echo $TODAY_LOGS | jq .
    exit 1
fi
log_info "✅ Dinner correctly excluded ($DINNER_COUNT entries)"

# Validation: same from_date and to_date should return 400
echo "Testing from_date == to_date returns 400..."
SAME_DATE_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/logs/copy" \
  -H "Content-Type: application/json" \
  -d "{\"from_date\": \"$TODAY\", \"to_date\": \"$TODAY\", \"meal_tags\": [\"breakfast\"]}")
if [ "$SAME_DATE_CODE" == "400" ]; then
    log_info "✅ Same date returns 400"
else
    log_err "Expected 400 for same date, got $SAME_DATE_CODE"
    exit 1
fi

# Validation: empty meal_tags should return 400
echo "Testing empty meal_tags returns 400..."
EMPTY_TAGS_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/logs/copy" \
  -H "Content-Type: application/json" \
  -d "{\"from_date\": \"$YESTERDAY\", \"to_date\": \"$TODAY\", \"meal_tags\": []}")
if [ "$EMPTY_TAGS_CODE" == "400" ]; then
    log_info "✅ Empty meal_tags returns 400"
else
    log_err "Expected 400 for empty meal_tags, got $EMPTY_TAGS_CODE"
    exit 1
fi

# Validation: copying day with no entries returns count 0 (not an error)
echo "Testing copy of empty source date returns count 0..."
FAR_PAST="1990-01-01"
EMPTY_RESULT=$(curl -s -X POST "$BASE_URL/logs/copy" \
  -H "Content-Type: application/json" \
  -d "{\"from_date\": \"$FAR_PAST\", \"to_date\": \"$TODAY\", \"meal_tags\": [\"breakfast\"]}")
EMPTY_COUNT=$(echo $EMPTY_RESULT | jq -r .count)
if [ "$EMPTY_COUNT" == "0" ]; then
    log_info "✅ Empty source day returns count 0"
else
    log_err "Expected count 0 for empty source day, got $EMPTY_COUNT"
    exit 1
fi

# Cleanup
curl -s -X DELETE "$BASE_URL/logs/$LOG_BFAST_ID" > /dev/null
curl -s -X DELETE "$BASE_URL/logs/$LOG_LUNCH_ID" > /dev/null
curl -s -X DELETE "$BASE_URL/logs/$LOG_DINNER_ID" > /dev/null
# Also delete only the copies we made (filter by the test food ID to avoid deleting other tests' entries)
TODAY_LOGS_AFTER=$(curl -s "$BASE_URL/logs?date=$TODAY")
echo $TODAY_LOGS_AFTER | jq -r --arg fid "$COPY_FOOD_ID" '.[] | select(.food_id == $fid) | .id' | while read id; do
    curl -s -X DELETE "$BASE_URL/logs/$id" > /dev/null
done
curl -s -X DELETE "$BASE_URL/foods/$COPY_FOOD_ID" > /dev/null
log_info "✅ Copy logs cleanup done"

echo "==================================================="
echo "Copy Logs tests complete"
echo "==================================================="
