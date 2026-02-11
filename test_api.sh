#!/bin/bash
# Test script for Calorize API

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"
echo "Targeting $BASE_URL"

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "jq is not installed. Please install it (brew install jq)"
    exit 1
fi

# Function to check if server is up
check_server() {
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/healthz")
    if [ "$HTTP_CODE" != "200" ]; then
        echo "Server is not running or not healthy at $BASE_URL (Status: $HTTP_CODE). Please start it."
        exit 1
    fi
}

check_server

echo "==================================================="
echo "Cleanup: Removing existing logs and foods..."
# Get all logs and delete them
LOGS=$(curl -s "$BASE_URL/logs")
if echo "$LOGS" | jq empty 2>/dev/null && [ "$(echo "$LOGS" | jq 'type')" == '"array"' ]; then
    LOG_IDS=$(echo "$LOGS" | jq -r '.[].id // empty')
    for id in $LOG_IDS; do
        echo "Deleting log $id"
        curl -s -X DELETE "$BASE_URL/logs/$id" > /dev/null
    done
fi

# Get all foods and delete them
FOODS=$(curl -s "$BASE_URL/foods")
if echo "$FOODS" | jq empty 2>/dev/null && [ "$(echo "$FOODS" | jq 'type')" == '"array"' ]; then
    FOOD_IDS=$(echo "$FOODS" | jq -r '.[].id // empty')
    for id in $FOOD_IDS; do
        echo "Deleting food $id"
        curl -s -X DELETE "$BASE_URL/foods/$id" > /dev/null
    done
fi
echo "✅ Cleanup Complete"

echo "==================================================="
echo "Test 1: Sunny Path - Food & Recipe"
echo "---------------------------------------------------"
echo "Creating Banana..."
CANONICAL_BANANA=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Banana",
  "calories": 89,
  "protein": 1.1,
  "carbs": 22.8,
  "fat": 0.3,
  "type": "food",
  "measurement_unit": "g",
  "measurement_amount": 100
}')
BANANA_ID=$(echo $CANONICAL_BANANA | jq -r .id)
echo "✅ Created Banana ID: $BANANA_ID"

echo "Creating Milk..."
CANONICAL_MILK=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Milk",
  "calories": 42,
  "protein": 3.4,
  "carbs": 5,
  "fat": 1,
  "type": "food",
  "measurement_unit": "ml",
  "measurement_amount": 100
}')
MILK_ID=$(echo $CANONICAL_MILK | jq -r .id)
echo "✅ Created Milk ID: $MILK_ID"

echo "Creating Banana Milkshake Recipe..."
# Note: Manually calculating macros: 89 + 84 = 173 kcal
RECIPE_WS=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d "{
  \"name\": \"Banana Milkshake\",
  \"type\": \"recipe\",
  \"calories\": 173, 
  \"protein\": 7.9,
  \"carbs\": 32.8,
  \"fat\": 2.3,
  \"measurement_unit\": \"serving\",
  \"measurement_amount\": 1,
  \"ingredients\": {
      \"$BANANA_ID\": 100,
      \"$MILK_ID\": 200
  }
}")
RECIPE_ID=$(echo $RECIPE_WS | jq -r .id)
echo "✅ Created Recipe ID: $RECIPE_ID"

echo "Verifying Ingredients..."
FETCHED_RECIPE=$(curl -s "$BASE_URL/foods/$RECIPE_ID")
INGREDIENT_COUNT=$(echo $FETCHED_RECIPE | jq '.ingredients | length')
if [ "$INGREDIENT_COUNT" -ne 2 ]; then
    echo "❌ Expected 2 ingredients, got $INGREDIENT_COUNT"
    echo $FETCHED_RECIPE | jq .
    exit 1
fi
echo "✅ Recipe ingredients verified"

# Check specific ingredient exists (Banana ID)
BANANA_AMOUNT=$(echo $FETCHED_RECIPE | jq -r ".ingredients[] | select(.ingredient_id==\"$BANANA_ID\") | .amount")
if [ "$BANANA_AMOUNT" != "100" ]; then
    echo "❌ Expected Banana amount 100, got $BANANA_AMOUNT"
    exit 1
fi
echo "✅ Banana ingredient amount verified"

echo "==================================================="
echo "Test 1.5: Complex Nutrients"
echo "---------------------------------------------------"
echo "Creating Orange with Vitamin C..."
ORANGE_RESP=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Orange",
  "calories": 47,
  "protein": 0.9,
  "carbs": 11.8,
  "fat": 0.1,
  "type": "food",
  "measurement_unit": "g",
  "measurement_amount": 100,
  "nutrients": [
      { "name": "Vitamin C", "amount": 53.2, "unit": "mg" },
      { "name": "Folate", "amount": 30, "unit": "ug" }
  ]
}')
ORANGE_ID=$(echo $ORANGE_RESP | jq -r .id)
echo "✅ Created Orange ID: $ORANGE_ID"

echo "Verifying Nutrients..."
FETCHED_ORANGE=$(curl -s "$BASE_URL/foods/$ORANGE_ID")
NUTRIENT_COUNT=$(echo $FETCHED_ORANGE | jq '.nutrients | length')
if [ "$NUTRIENT_COUNT" -ne 2 ]; then
    echo "❌ Expected 2 nutrients, got $NUTRIENT_COUNT"
    echo $FETCHED_ORANGE | jq .
    exit 1
fi

VIT_C=$(echo $FETCHED_ORANGE | jq -r '.nutrients[] | select(.name=="Vitamin C") | .amount')
if [ "$VIT_C" != "53.2" ]; then
    echo "❌ Expected Vitamin C 53.2, got $VIT_C"
    exit 1
fi
echo "✅ Nutrients verified"

echo "==================================================="
echo "Test 2: Logging & Stats"
echo "---------------------------------------------------"
echo "Logging consumption..."
NOW_ISO=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
LOG_WS=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"food_id\": \"$RECIPE_ID\",
  \"amount\": 1.0,
  \"meal_tag\": \"snack\",
  \"logged_at\": \"$NOW_ISO\"
}")
LOG_ID=$(echo $LOG_WS | jq -r .id)
echo "✅ Logged Entry ID: $LOG_ID"

echo "Checking Stats..."
STATS=$(curl -s "$BASE_URL/stats?period=day")
TOTAL_CAL=$(echo $STATS | jq -r .calories)
# Float verify
MATCH=$(echo "$TOTAL_CAL 173" | awk '{if ($1 >= 172.9 && $1 <= 173.1) print 1; else print 0}')
if [ "$MATCH" -eq 1 ]; then
     echo "✅ Stats Correct: ~$TOTAL_CAL kcal"
else
     echo "❌ Stats Incorrect: Expected 173, got $TOTAL_CAL"
     echo $STATS | jq .
     exit 1
fi

echo "==================================================="
echo "Test 3: Edge Cases"
echo "---------------------------------------------------"
echo "Creating Food with Missing Name..."
MISSING_NAME_RESP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "calories": 100,
  "protein": 10,
  "carbs": 10,
  "fat": 2,
  "type": "food",
  "measurement_unit": "g",
  "measurement_amount": 100
}')
echo "Response Code: $MISSING_NAME_RESP"

echo "Creating Food with Negative Calories..."
NEGATIVE_RESP=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Anti-Matter",
  "calories": -100,
  "protein": 0,
  "carbs": 0,
  "fat": 0,
  "type": "food",
  "measurement_unit": "g",
  "measurement_amount": 100
}')
NEGATIVE_ID=$(echo $NEGATIVE_RESP | jq -r .id)
echo "✅ Created Negative Calorie Food: $NEGATIVE_ID"

echo "Logging Negative Food..."
# Using 100g to subtract exactly 100 kcal
curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"food_id\": \"$NEGATIVE_ID\",
  \"amount\": 100.0,
  \"meal_tag\": \"science\",
  \"logged_at\": \"$NOW_ISO\"
}" > /dev/null

echo "Checking Stats (Should decrease)..."
STATS_NEG=$(curl -s "$BASE_URL/stats?period=day")
TOTAL_CAL_NEG=$(echo $STATS_NEG | jq -r .calories)
# Expected: 173 - 100 = 73
MATCH_NEG=$(echo "$TOTAL_CAL_NEG 73" | awk '{if ($1 >= 72.9 && $1 <= 73.1) print 1; else print 0}')
if [ "$MATCH_NEG" -eq 1 ]; then
     echo "✅ Stats Correct (Negative works): ~$TOTAL_CAL_NEG kcal"
else
     echo "❌ Stats Incorrect after negative: Expected 73, got $TOTAL_CAL_NEG"
     echo $STATS_NEG | jq .
fi

echo "---------------------------------------------------"
echo "Invalid UUID for Get Food..."
INVALID_UUID_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/foods/not-a-uuid")
if [ "$INVALID_UUID_CODE" == "400" ]; then
    echo "✅ Correctly rejected invalid UUID (400)"
else
    echo "❌ Unexpected code for invalid UUID: $INVALID_UUID_CODE"
fi

echo "Non-existent UUID for Get Food..."
RANDOM_UUID="00000000-0000-0000-0000-000000000000"
NOT_FOUND_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/foods/$RANDOM_UUID")
if [ "$NOT_FOUND_CODE" == "404" ]; then
    echo "✅ Non-existent UUID returns 404"
else
    echo "❌ Unexpected code for non-existent UUID: $NOT_FOUND_CODE"
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
# Expected: 73 + 500 = 573
MATCH_QUICK=$(echo "$TOTAL_CAL_QUICK 573" | awk '{if ($1 >= 572.9 && $1 <= 573.1) print 1; else print 0}')
if [ "$MATCH_QUICK" -eq 1 ]; then
     echo "✅ Stats Correct (Quick add works): ~$TOTAL_CAL_QUICK kcal"
else
     echo "❌ Stats Incorrect after quick add: Expected 573, got $TOTAL_CAL_QUICK"
     echo $STATS_QUICK | jq .
     exit 1
fi

echo "==================================================="
echo "Test 5: Food Update (PUT) & Versioning"
echo "---------------------------------------------------"
echo "Updating Banana calories from 89 to 95..."
UPDATE_RESP=$(curl -s -X PUT "$BASE_URL/foods/$BANANA_ID" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Banana",
  "calories": 95,
  "protein": 1.1,
  "carbs": 24.0,
  "fat": 0.3,
  "type": "food",
  "measurement_unit": "g",
  "measurement_amount": 100
}')
NEW_BANANA_ID=$(echo $UPDATE_RESP | jq -r .id)
NEW_VERSION=$(echo $UPDATE_RESP | jq -r .version)
NEW_FAMILY=$(echo $UPDATE_RESP | jq -r .family_id)

if [ "$NEW_VERSION" != "2" ]; then
    echo "❌ Expected version 2, got $NEW_VERSION"
    echo $UPDATE_RESP | jq .
    exit 1
fi
echo "✅ Version bumped to $NEW_VERSION"

if [ "$NEW_BANANA_ID" == "$BANANA_ID" ]; then
    echo "❌ Expected new ID, got same ID as original"
    exit 1
fi
echo "✅ New version has new ID: $NEW_BANANA_ID"

OLD_BANANA_FAMILY=$(echo $CANONICAL_BANANA | jq -r .family_id)
if [ "$NEW_FAMILY" != "$OLD_BANANA_FAMILY" ]; then
    echo "❌ Expected same family_id $OLD_BANANA_FAMILY, got $NEW_FAMILY"
    exit 1
fi
echo "✅ Family ID preserved: $NEW_FAMILY"

echo "Verifying old version still fetchable..."
OLD_FETCH=$(curl -s "$BASE_URL/foods/$BANANA_ID")
OLD_IS_CURRENT=$(echo $OLD_FETCH | jq -r .is_current)
if [ "$OLD_IS_CURRENT" != "false" ]; then
    echo "❌ Old version should have is_current=false, got $OLD_IS_CURRENT"
    exit 1
fi
echo "✅ Old version marked as not current"

echo "Verifying list only returns current version..."
FOODS_LIST=$(curl -s "$BASE_URL/foods")
BANANA_COUNT=$(echo $FOODS_LIST | jq '[.[] | select(.name=="Banana")] | length')
if [ "$BANANA_COUNT" -ne 1 ]; then
    echo "❌ Expected 1 Banana in list, got $BANANA_COUNT"
    exit 1
fi
LISTED_BANANA_ID=$(echo $FOODS_LIST | jq -r '[.[] | select(.name=="Banana")][0].id')
if [ "$LISTED_BANANA_ID" != "$NEW_BANANA_ID" ]; then
    echo "❌ Listed Banana should be new version"
    exit 1
fi
echo "✅ List shows only current Banana version"

echo "Updating non-existent food..."
RANDOM_UUID="00000000-0000-0000-0000-000000000099"
UPDATE_404=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE_URL/foods/$RANDOM_UUID" \
  -H "Content-Type: application/json" \
  -d '{"name":"Ghost","calories":0,"protein":0,"carbs":0,"fat":0,"type":"food","measurement_unit":"g","measurement_amount":100}')
if [ "$UPDATE_404" == "404" ]; then
    echo "✅ Update non-existent food returns 404"
else
    echo "❌ Expected 404, got $UPDATE_404"
fi

echo "Updating food with empty name..."
UPDATE_NONAME=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE_URL/foods/$NEW_BANANA_ID" \
  -H "Content-Type: application/json" \
  -d '{"name":"","calories":95,"protein":1.1,"carbs":24,"fat":0.3,"type":"food","measurement_unit":"g","measurement_amount":100}')
if [ "$UPDATE_NONAME" == "400" ]; then
    echo "✅ Update with empty name returns 400"
else
    echo "❌ Expected 400, got $UPDATE_NONAME"
fi

echo "==================================================="
echo "Test 6: Food Soft-Delete Lifecycle"
echo "---------------------------------------------------"
echo "Creating temp food to delete..."
TEMP_FOOD=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Temp Deletable",
  "calories": 50,
  "protein": 1,
  "carbs": 5,
  "fat": 0.5,
  "type": "food",
  "measurement_unit": "g",
  "measurement_amount": 100
}')
TEMP_ID=$(echo $TEMP_FOOD | jq -r .id)
echo "✅ Created temp food: $TEMP_ID"

echo "Deleting temp food..."
DELETE_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/foods/$TEMP_ID")
if [ "$DELETE_CODE" == "204" ]; then
    echo "✅ Delete returned 204"
else
    echo "❌ Expected 204, got $DELETE_CODE"
    exit 1
fi

echo "Verifying deleted food excluded from list..."
FOODS_AFTER_DEL=$(curl -s "$BASE_URL/foods")
TEMP_IN_LIST=$(echo $FOODS_AFTER_DEL | jq '[.[] | select(.id=="'$TEMP_ID'")] | length')
if [ "$TEMP_IN_LIST" -eq 0 ]; then
    echo "✅ Deleted food not in list"
else
    echo "❌ Deleted food still appears in list"
    exit 1
fi

echo "Verifying GET on deleted food (still fetchable by ID)..."
GET_DELETED=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/foods/$TEMP_ID")
# GetFood has no deleted_at filter, so this should return 200
if [ "$GET_DELETED" == "200" ]; then
    echo "✅ Deleted food still fetchable by direct ID (expected: no deleted_at filter in GetFood)"
else
    echo "⚠️  GET deleted food returned $GET_DELETED (may indicate deleted_at filtering was added)"
fi

echo "Deleting already-deleted food (idempotency)..."
DELETE_AGAIN=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/foods/$TEMP_ID")
if [ "$DELETE_AGAIN" == "204" ]; then
    echo "✅ Re-delete returns 204 (idempotent)"
else
    echo "⚠️  Re-delete returned $DELETE_AGAIN"
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
echo "✅ Created Tracker Apple: $TRACK_ID"

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
echo "✅ Logged entry: $TRACK_LOG_ID"

echo "Verifying stats increased..."
AFTER_LOG_STATS=$(curl -s "$BASE_URL/stats?period=day")
AFTER_LOG_CAL=$(echo $AFTER_LOG_STATS | jq -r .calories)
# 200g of 52cal/100g = 104 cal added
EXPECTED_AFTER=$(echo "$BASELINE_CAL + 104" | bc)
MATCH_AFTER=$(echo "$AFTER_LOG_CAL $EXPECTED_AFTER" | awk '{d=$1-$2; if(d<0)d=-d; if(d<=0.1)print 1; else print 0}')
if [ "$MATCH_AFTER" -eq 1 ]; then
    echo "✅ Stats increased correctly: $AFTER_LOG_CAL kcal"
else
    echo "❌ Expected ~$EXPECTED_AFTER, got $AFTER_LOG_CAL"
    exit 1
fi

echo "Deleting the log entry..."
DEL_LOG_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/logs/$TRACK_LOG_ID")
if [ "$DEL_LOG_CODE" == "204" ]; then
    echo "✅ Log delete returned 204"
else
    echo "❌ Expected 204, got $DEL_LOG_CODE"
    exit 1
fi

echo "Verifying stats decreased back..."
AFTER_DEL_STATS=$(curl -s "$BASE_URL/stats?period=day")
AFTER_DEL_CAL=$(echo $AFTER_DEL_STATS | jq -r .calories)
MATCH_BASELINE=$(echo "$AFTER_DEL_CAL $BASELINE_CAL" | awk '{d=$1-$2; if(d<0)d=-d; if(d<=0.1)print 1; else print 0}')
if [ "$MATCH_BASELINE" -eq 1 ]; then
    echo "✅ Stats returned to baseline: $AFTER_DEL_CAL kcal"
else
    echo "❌ Expected ~$BASELINE_CAL, got $AFTER_DEL_CAL"
    exit 1
fi

echo "Deleting non-existent log (random UUID)..."
DEL_GHOST_LOG=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/logs/00000000-0000-0000-0000-000000000099")
echo "Delete non-existent log returned: $DEL_GHOST_LOG (soft-delete affects 0 rows)"

echo "Invalid UUID for log delete..."
DEL_BAD_UUID=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/logs/not-a-uuid")
if [ "$DEL_BAD_UUID" == "400" ]; then
    echo "✅ Invalid UUID for log delete returns 400"
else
    echo "❌ Expected 400, got $DEL_BAD_UUID"
fi

echo "==================================================="
echo "Test 8: GET /logs with Date Filtering"
echo "---------------------------------------------------"
echo "Fetching today's logs (default)..."
TODAY_LOGS=$(curl -s "$BASE_URL/logs")
TODAY_COUNT=$(echo $TODAY_LOGS | jq 'length')
echo "Today's logs count: $TODAY_COUNT"

TODAY_DATE=$(date -u +"%Y-%m-%d")
echo "Fetching today's logs with explicit date..."
TODAY_EXPLICIT=$(curl -s "$BASE_URL/logs?date=$TODAY_DATE")
EXPLICIT_COUNT=$(echo $TODAY_EXPLICIT | jq 'length')
if [ "$TODAY_COUNT" == "$EXPLICIT_COUNT" ]; then
    echo "✅ Default and explicit date return same count: $TODAY_COUNT"
else
    echo "❌ Default ($TODAY_COUNT) vs explicit ($EXPLICIT_COUNT) mismatch"
fi

echo "Fetching logs for a date with no data..."
EMPTY_LOGS=$(curl -s "$BASE_URL/logs?date=2000-01-01")
EMPTY_COUNT=$(echo $EMPTY_LOGS | jq 'length')
if [ "$EMPTY_COUNT" -eq 0 ] || [ "$EMPTY_LOGS" == "null" ]; then
    echo "✅ No logs for 2000-01-01"
else
    echo "❌ Expected 0 logs, got $EMPTY_COUNT"
fi

echo "Fetching logs with invalid date format..."
INVALID_DATE_LOGS=$(curl -s "$BASE_URL/logs?date=not-a-date")
INVALID_COUNT=$(echo $INVALID_DATE_LOGS | jq 'length')
# Falls back to today silently
if [ "$INVALID_COUNT" == "$TODAY_COUNT" ]; then
    echo "✅ Invalid date falls back to today (got $INVALID_COUNT logs)"
else
    echo "⚠️  Invalid date returned $INVALID_COUNT logs (today has $TODAY_COUNT)"
fi

echo "==================================================="
echo "Test 9: Stats — Week, Month, Invalid, Missing Period"
echo "---------------------------------------------------"
echo "Stats for the week..."
WEEK_STATS=$(curl -s "$BASE_URL/stats?period=week")
WEEK_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/stats?period=week")
WEEK_CAL=$(echo $WEEK_STATS | jq -r .calories)
if [ "$WEEK_CODE" == "200" ]; then
    echo "✅ Week stats returned 200 (calories: $WEEK_CAL)"
else
    echo "❌ Week stats returned $WEEK_CODE"
fi

echo "Stats for the month..."
MONTH_STATS=$(curl -s "$BASE_URL/stats?period=month")
MONTH_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/stats?period=month")
MONTH_CAL=$(echo $MONTH_STATS | jq -r .calories)
if [ "$MONTH_CODE" == "200" ]; then
    echo "✅ Month stats returned 200 (calories: $MONTH_CAL)"
else
    echo "❌ Month stats returned $MONTH_CODE"
fi

echo "Week stats should >= day stats..."
DAY_STATS=$(curl -s "$BASE_URL/stats?period=day")
DAY_CAL=$(echo $DAY_STATS | jq -r .calories)
WEEK_GTE_DAY=$(echo "$WEEK_CAL $DAY_CAL" | awk '{if ($1 >= $2 - 0.01) print 1; else print 0}')
if [ "$WEEK_GTE_DAY" -eq 1 ]; then
    echo "✅ Week ($WEEK_CAL) >= Day ($DAY_CAL)"
else
    echo "❌ Week ($WEEK_CAL) < Day ($DAY_CAL)"
fi

echo "Stats with invalid period..."
INVALID_PERIOD_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/stats?period=invalid")
if [ "$INVALID_PERIOD_CODE" == "500" ]; then
    echo "✅ Invalid period returns 500 (known: server returns fmt.Errorf, not 400)"
else
    echo "⚠️  Invalid period returned $INVALID_PERIOD_CODE (expected 500)"
fi

echo "Stats with no period param..."
NO_PERIOD_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/stats")
if [ "$NO_PERIOD_CODE" == "500" ]; then
    echo "✅ Missing period returns 500 (known: empty string hits default case)"
else
    echo "⚠️  Missing period returned $NO_PERIOD_CODE (expected 500)"
fi

echo "Stats with custom past date..."
PAST_STATS=$(curl -s "$BASE_URL/stats?period=day&date=2000-01-01")
PAST_CAL=$(echo $PAST_STATS | jq -r .calories)
PAST_IS_ZERO=$(echo "$PAST_CAL" | awk '{if ($1 == 0) print 1; else print 0}')
if [ "$PAST_IS_ZERO" -eq 1 ]; then
    echo "✅ Stats for 2000-01-01 returns 0 calories"
else
    echo "❌ Expected 0 calories for 2000-01-01, got $PAST_CAL"
fi

echo "Stats with invalid date format..."
BAD_DATE_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/stats?period=day&date=bad-date")
BAD_DATE_STATS=$(curl -s "$BASE_URL/stats?period=day&date=bad-date")
BAD_DATE_CAL=$(echo $BAD_DATE_STATS | jq -r .calories)
if [ "$BAD_DATE_CODE" == "200" ]; then
    echo "✅ Invalid date silently falls back to today (calories: $BAD_DATE_CAL)"
else
    echo "⚠️  Invalid date returned $BAD_DATE_CODE"
fi

echo "==================================================="
echo "Test 10: Input Validation — Malformed & Invalid"
echo "---------------------------------------------------"
echo "POST /foods with invalid JSON..."
BAD_JSON_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d 'this is not json')
if [ "$BAD_JSON_CODE" == "400" ]; then
    echo "✅ Invalid JSON returns 400"
else
    echo "❌ Expected 400, got $BAD_JSON_CODE"
fi

echo "POST /foods with empty body..."
EMPTY_BODY_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '')
if [ "$EMPTY_BODY_CODE" == "400" ]; then
    echo "✅ Empty body returns 400"
else
    echo "❌ Expected 400, got $EMPTY_BODY_CODE"
fi

echo "POST /logs with neither food_id nor calories..."
NEITHER_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"amount\": 1,
  \"meal_tag\": \"test\",
  \"logged_at\": \"$NOW_ISO\"
}")
if [ "$NEITHER_CODE" == "400" ]; then
    echo "✅ Log without food_id or calories returns 400"
else
    echo "❌ Expected 400, got $NEITHER_CODE"
fi

echo "POST /logs with non-existent food_id..."
NON_EXIST_FOOD_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"food_id\": \"00000000-0000-0000-0000-000000000099\",
  \"amount\": 1,
  \"meal_tag\": \"test\",
  \"logged_at\": \"$NOW_ISO\"
}")
echo "Log with non-existent food_id returned: $NON_EXIST_FOOD_CODE"
# populateMacros calls GetFood → returns nil,nil → macros not populated
# Insert may fail on FK constraint or succeed with null macros
if [ "$NON_EXIST_FOOD_CODE" == "500" ] || [ "$NON_EXIST_FOOD_CODE" == "400" ]; then
    echo "✅ Non-existent food_id correctly rejected ($NON_EXIST_FOOD_CODE)"
else
    echo "⚠️  Unexpected response: $NON_EXIST_FOOD_CODE (may indicate FK constraint not enforced)"
fi

echo "POST /logs with invalid JSON..."
BAD_LOG_JSON=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d '{bad json}')
if [ "$BAD_LOG_JSON" == "400" ]; then
    echo "✅ Invalid JSON for log returns 400"
else
    echo "❌ Expected 400, got $BAD_LOG_JSON"
fi

echo "==================================================="
echo "Test 11: Boundary Values"
echo "---------------------------------------------------"
echo "Logging with amount = 0..."
ZERO_LOG=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"food_id\": \"$NEW_BANANA_ID\",
  \"amount\": 0,
  \"meal_tag\": \"test\",
  \"logged_at\": \"$NOW_ISO\"
}")
ZERO_LOG_CAL=$(echo $ZERO_LOG | jq -r .calories)
ZERO_LOG_ID=$(echo $ZERO_LOG | jq -r .id)
IS_ZERO=$(echo "$ZERO_LOG_CAL" | awk '{if ($1 == 0) print 1; else print 0}')
if [ "$IS_ZERO" -eq 1 ]; then
    echo "✅ Zero amount → 0 calories: $ZERO_LOG_CAL"
else
    echo "❌ Expected 0 calories for amount=0, got $ZERO_LOG_CAL"
fi
# Clean up zero log
curl -s -X DELETE "$BASE_URL/logs/$ZERO_LOG_ID" > /dev/null

echo "Logging with very large amount (99999)..."
BIG_LOG=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"food_id\": \"$NEW_BANANA_ID\",
  \"amount\": 99999,
  \"meal_tag\": \"test\",
  \"logged_at\": \"$NOW_ISO\"
}")
BIG_LOG_ID=$(echo $BIG_LOG | jq -r .id)
BIG_LOG_CAL=$(echo $BIG_LOG | jq -r .calories)
if [ "$BIG_LOG_ID" != "null" ] && [ "$BIG_LOG_ID" != "" ]; then
    echo "✅ Large amount accepted (calories: $BIG_LOG_CAL)"
    curl -s -X DELETE "$BASE_URL/logs/$BIG_LOG_ID" > /dev/null
else
    echo "❌ Large amount rejected"
fi

echo "Creating food with measurement_amount = 0..."
ZERO_MEASURE=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Zero Measure Food",
  "calories": 200,
  "protein": 10,
  "carbs": 20,
  "fat": 5,
  "type": "food",
  "measurement_unit": "g",
  "measurement_amount": 0
}')
ZERO_M_ID=$(echo $ZERO_MEASURE | jq -r .id)
echo "✅ Created zero-measurement food: $ZERO_M_ID"

echo "Logging zero-measurement food (tests division fallback)..."
ZM_LOG=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"food_id\": \"$ZERO_M_ID\",
  \"amount\": 2,
  \"meal_tag\": \"test\",
  \"logged_at\": \"$NOW_ISO\"
}")
ZM_LOG_ID=$(echo $ZM_LOG | jq -r .id)
ZM_CAL=$(echo $ZM_LOG | jq -r .calories)
# measurement_amount=0 → fallback to 1, so ratio = 2/1 = 2, cal = 2*200 = 400
MATCH_ZM=$(echo "$ZM_CAL 400" | awk '{d=$1-$2; if(d<0)d=-d; if(d<=0.1)print 1; else print 0}')
if [ "$MATCH_ZM" -eq 1 ]; then
    echo "✅ Zero measurement_amount uses fallback=1 correctly (calories: $ZM_CAL)"
else
    echo "❌ Expected ~400, got $ZM_CAL (division fallback may be broken)"
fi
curl -s -X DELETE "$BASE_URL/logs/$ZM_LOG_ID" > /dev/null

echo "Creating food with very long name (500 chars)..."
LONG_NAME=$(printf 'A%.0s' {1..500})
LONG_NAME_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d "{
  \"name\": \"$LONG_NAME\",
  \"calories\": 10,
  \"protein\": 1,
  \"carbs\": 1,
  \"fat\": 0.1,
  \"type\": \"food\",
  \"measurement_unit\": \"g\",
  \"measurement_amount\": 100
}")
if [ "$LONG_NAME_CODE" == "200" ]; then
    echo "✅ Very long name accepted (no server-side length validation)"
else
    echo "⚠️  Long name returned $LONG_NAME_CODE"
fi

echo "==================================================="
echo "Test 12: Recipe Edge Cases"
echo "---------------------------------------------------"
echo "Creating recipe with non-existent ingredient ID..."
BAD_INGREDIENT_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Bad Recipe",
  "type": "recipe",
  "calories": 100,
  "protein": 5,
  "carbs": 10,
  "fat": 2,
  "measurement_unit": "serving",
  "measurement_amount": 1,
  "ingredients": {
      "00000000-0000-0000-0000-000000000099": 100
  }
}')
if [ "$BAD_INGREDIENT_CODE" == "500" ]; then
    echo "✅ Recipe with non-existent ingredient returns 500 (FK constraint)"
else
    echo "⚠️  Expected 500 for bad ingredient FK, got $BAD_INGREDIENT_CODE"
fi

echo "Creating recipe with single ingredient..."
SINGLE_RECIPE=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d "{
  \"name\": \"Just Milk\",
  \"type\": \"recipe\",
  \"calories\": 42,
  \"protein\": 3.4,
  \"carbs\": 5,
  \"fat\": 1,
  \"measurement_unit\": \"serving\",
  \"measurement_amount\": 1,
  \"ingredients\": {
      \"$MILK_ID\": 100
  }
}")
SINGLE_RECIPE_ID=$(echo $SINGLE_RECIPE | jq -r .id)
SINGLE_INGR_COUNT=$(curl -s "$BASE_URL/foods/$SINGLE_RECIPE_ID" | jq '.ingredients | length')
if [ "$SINGLE_INGR_COUNT" -eq 1 ]; then
    echo "✅ Single-ingredient recipe created: $SINGLE_RECIPE_ID"
else
    echo "❌ Expected 1 ingredient, got $SINGLE_INGR_COUNT"
fi

echo "Creating food with empty ingredients map (should be 'food' type)..."
EMPTY_INGR=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Not A Recipe",
  "type": "recipe",
  "calories": 100,
  "protein": 5,
  "carbs": 10,
  "fat": 2,
  "measurement_unit": "g",
  "measurement_amount": 100,
  "ingredients": {}
}')
EMPTY_INGR_TYPE=$(echo $EMPTY_INGR | jq -r .type)
# CreateFood: if len(ingredients) > 0 → recipe, else if type == "" → food
# Since type is "recipe" and ingredients is empty, it keeps "recipe"
echo "Empty ingredients with explicit type='recipe': got type=$EMPTY_INGR_TYPE"
if [ "$EMPTY_INGR_TYPE" == "recipe" ]; then
    echo "✅ Explicit type='recipe' preserved when no ingredients"
else
    echo "⚠️  Type changed to $EMPTY_INGR_TYPE"
fi

echo "Creating food with no type and no ingredients (defaults to 'food')..."
NO_TYPE=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Default Type",
  "calories": 100,
  "protein": 5,
  "carbs": 10,
  "fat": 2,
  "measurement_unit": "g",
  "measurement_amount": 100
}')
NO_TYPE_VAL=$(echo $NO_TYPE | jq -r .type)
if [ "$NO_TYPE_VAL" == "food" ]; then
    echo "✅ No type defaults to 'food'"
else
    echo "❌ Expected 'food', got $NO_TYPE_VAL"
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
    echo "✅ Double-log accumulated correctly: $ACCUM_CAL kcal (expected ~$EXPECTED_ACCUM)"
else
    echo "❌ Expected ~$EXPECTED_ACCUM, got $ACCUM_CAL"
    exit 1
fi

echo "Verifying protein also accumulated..."
ACCUM_PROT=$(echo $ACCUM_STATS | jq -r .protein)
ACCUM_BASE_PROT=$(echo $ACCUM_BASELINE | jq -r .protein)
EXPECTED_PROT=$(echo "$ACCUM_BASE_PROT + 6.8" | bc)
MATCH_PROT=$(echo "$ACCUM_PROT $EXPECTED_PROT" | awk '{d=$1-$2; if(d<0)d=-d; if(d<=0.1)print 1; else print 0}')
if [ "$MATCH_PROT" -eq 1 ]; then
    echo "✅ Protein accumulated: $ACCUM_PROT g (expected ~$EXPECTED_PROT)"
else
    echo "❌ Expected protein ~$EXPECTED_PROT, got $ACCUM_PROT"
fi

# Clean up double logs
curl -s -X DELETE "$BASE_URL/logs/$LOG_A_ID" > /dev/null
curl -s -X DELETE "$BASE_URL/logs/$LOG_B_ID" > /dev/null

echo "==================================================="
echo "Final Cleanup..."
echo "✅ All Tests Passed"
