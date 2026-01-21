#!/bin/bash
set -e

BASE_URL="http://localhost:8383"
USER_ID="test-stats-user-$(date +%s)"
DATE="2024-01-01"

echo "Testing Calorize API - Statistics"
echo "-----------------------------------"

# 1. Create a Food (100 cal, 10p, 10c, 2f)
echo "1. Creating Test Food..."
FOOD_RES=$(curl -s -X POST $BASE_URL/foods -d '{"name":"TestStatFood", "calories":100, "protein":10, "carbs":10, "fat":2, "measurement_unit":"count", "measurement_amount":1}')
FOOD_ID=$(echo $FOOD_RES | jq -r '.id')
echo "✅ Food created: $FOOD_ID"

# 2. Log 2 items today (Total: 200 cal)
echo "2. Logging 2 items..."
# Log 1
curl -s -o /dev/null -X POST $BASE_URL/logs \
    -H "X-User-ID: $USER_ID" \
    -d "$(jq -n --arg fid "$FOOD_ID" --arg date "${DATE}T08:00:00Z" '{food_id: $fid, amount: 1, meal_tag: "breakfast", logged_at: $date}')"

# Log 2
curl -s -o /dev/null -X POST $BASE_URL/logs \
    -H "X-User-ID: $USER_ID" \
    -d "$(jq -n --arg fid "$FOOD_ID" --arg date "${DATE}T12:00:00Z" '{food_id: $fid, amount: 1, meal_tag: "lunch", logged_at: $date}')"

echo "✅ Items logged."

# 3. Query Stats (Day)
echo "3. Querying Daily Stats..."
STATS_RES=$(curl -s "$BASE_URL/stats?user_id=$USER_ID&date=$DATE&period=day")
echo "$STATS_RES" | jq .

CAL=$(echo $STATS_RES | jq -r '.total_calories')
if [ "$CAL" != "200" ]; then
    echo "❌ Expected 200 calories, got $CAL"
    exit 1
fi
echo "✅ Calories match (200)."

# 4. Query Stats (Month)
echo "4. Querying Monthly Stats..."
MONTH_RES=$(curl -s "$BASE_URL/stats?user_id=$USER_ID&date=$DATE&period=month")
MCAL=$(echo $MONTH_RES | jq -r '.total_calories')
if [ "$MCAL" != "200" ]; then
    echo "❌ Expected 200 calories (month), got $MCAL"
    exit 1
fi
echo "✅ Monthly Calories match."

echo "-----------------------------------"
echo "🎉 Statistics Test Passed!"
