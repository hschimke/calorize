#!/bin/bash
# Test script for Calorize API

set -e

BASE_URL="http://localhost:8080"
echo "Targeting $BASE_URL"

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "jq is not installed. Please install it (brew install jq)"
    exit 1
fi

# Function to check if server is up
check_server() {
    # Hello endpoint doesn't need auth, but main.go wrapped everything with DevAuth anyway.
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/hello/Checking")
    if [ "$HTTP_CODE" != "200" ]; then
        echo "Server is not running or not healthy at $BASE_URL (Status: $HTTP_CODE). Please start it."
        exit 1
    fi
}

check_server

echo "---------------------------------------------------"
echo "1. Getting Hello..."
curl -s "$BASE_URL/hello/Tester"
echo ""

echo "---------------------------------------------------"
echo "2. Creating Food..."
FOOD_WS=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Test Banana",
  "calories": 105,
  "protein": 1.3,
  "carbs": 27,
  "fat": 0.4,
  "type": "basic",
  "measurement_unit": "g",
  "measurement_amount": 100
}') 
echo "Response: $FOOD_WS"
FOOD_ID=$(echo $FOOD_WS | jq -r .id)

if [ "$FOOD_ID" == "null" ]; then
    echo "Failed to create food"
    exit 1
fi
echo "Created Food ID: $FOOD_ID"

echo "---------------------------------------------------"
echo "3. Listing Foods..."
curl -s "$BASE_URL/foods" | jq .

echo "---------------------------------------------------"
echo "4. Getting Specific Food..."
curl -s "$BASE_URL/foods/$FOOD_ID" | jq .

echo "---------------------------------------------------"
echo "5. Updating Food (New Version)..."
UPDATED_WS=$(curl -s -X PUT "$BASE_URL/foods/$FOOD_ID" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Updated Banana",
  "calories": 110,
  "protein": 1.5,
  "carbs": 28,
  "fat": 0.5,
  "type": "basic",
  "measurement_unit": "g",
  "measurement_amount": 100
}')
echo "Response: $UPDATED_WS"
UPDATED_ID=$(echo $UPDATED_WS | jq -r .id)
echo "Updated Food ID (New Version): $UPDATED_ID"

echo "---------------------------------------------------"
echo "6. Logging Food..."
# Assuming we log the newly created/updated food
LOG_WS=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{
  \"food_id\": \"$UPDATED_ID\",
  \"amount\": 1.0,
  \"meal_tag\": \"breakfast\",
  \"logged_at\": \"$(date -u +"%Y-%m-%dT%H:%M:%SZ")\"
}")
echo "Log Entry: $LOG_WS"

echo "---------------------------------------------------"
echo "7. Getting Logs..."
curl -s "$BASE_URL/logs" | jq .

echo "---------------------------------------------------"
echo "8. Getting Stats (Day)..."
curl -s "$BASE_URL/stats?period=day" | jq .

echo "---------------------------------------------------"
echo "9. Deleting Food..."
curl -s -X DELETE "$BASE_URL/foods/$FOOD_ID"
echo "Deleted initial version (may still be visible if history preserved, but Listing Foods should check is_current)"

echo "---------------------------------------------------"
echo "10. Listing Foods Again..."
curl -s "$BASE_URL/foods" | jq .
