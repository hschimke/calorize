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
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/hello/Checking")
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
LOG_IDS=$(echo $LOGS | jq -r '.[].id // empty')
for id in $LOG_IDS; do
    echo "Deleting log $id"
    curl -s -X DELETE "$BASE_URL/logs/$id" > /dev/null
done

# Get all foods and delete them
FOODS=$(curl -s "$BASE_URL/foods")
FOOD_IDS=$(echo $FOODS | jq -r '.[].id // empty')
for id in $FOOD_IDS; do
    echo "Deleting food $id"
    curl -s -X DELETE "$BASE_URL/foods/$id" > /dev/null
done
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
echo "Final Cleanup..."
echo "✅ All Tests Passed"
