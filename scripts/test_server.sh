#!/bin/bash
set -e

BASE_URL="http://localhost:8383"
USER_ID="test-user-$(date +%s)"

echo "Testing Calorize API at $BASE_URL"
echo "-----------------------------------"

# 1. Health Check
echo "1. Checking Health..."
RESPONSE=$(curl -s -w "%{http_code}" $BASE_URL/health)
HTTP_CODE=${RESPONSE: -3}
if [ "$HTTP_CODE" -ne 200 ]; then
    echo "❌ Health check failed! Code: $HTTP_CODE"
    exit 1
fi
echo "✅ Health check passed."

# 2. Create Food
echo "2. Creating Food (Banana)..."
FOOD_PAYLOAD='{"name":"Banana", "calories":105, "protein":1.3, "carbs":27, "fat":0.3, "measurement_unit":"count", "measurement_amount":1}'
FOOD_RES=$(curl -s -X POST $BASE_URL/foods -d "$FOOD_PAYLOAD")

FOOD_ID=$(echo $FOOD_RES | jq -r '.id')
if [ "$FOOD_ID" == "null" ] || [ -z "$FOOD_ID" ]; then
    echo "❌ Failed to create food. Response: $FOOD_RES"
    exit 1
fi
echo "✅ Food created. ID: $FOOD_ID"

# 3. Create Log
echo "3. Logging Food..."
LOG_PAYLOAD=$(jq -n \
                  --arg fid "$FOOD_ID" \
                  --arg tag "snack" \
                  '{food_id: $fid, amount: 2, meal_tag: $tag}')

LOG_RES=$(curl -s -X POST $BASE_URL/logs \
            -H "X-User-ID: $USER_ID" \
            -d "$LOG_PAYLOAD")

LOG_ID=$(echo $LOG_RES | jq -r '.id')
if [ "$LOG_ID" == "null" ]; then
     echo "❌ Failed to log food. Response: $LOG_RES"
     exit 1
fi
echo "✅ Food logged. Log ID: $LOG_ID"

# 4. Get Logs
echo "4. Fetching Logs..."
LOGS_RES=$(curl -s "$BASE_URL/logs?user_id=$USER_ID")
COUNT=$(echo $LOGS_RES | jq 'length')

if [ "$COUNT" -lt 1 ]; then
    echo "❌ No logs found! Response: $LOGS_RES"
    exit 1
fi

echo "✅ Logs fetched. Count: $COUNT"
echo "$LOGS_RES" | jq .

echo "-----------------------------------"
echo "🎉 All Tests Passed!"
