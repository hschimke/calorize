#!/bin/bash
set -e

BASE_URL="http://localhost:8383"

echo "Testing Calorize API - Versioning"
echo "-----------------------------------"

# 1. Create Initial Version
echo "1. Creating Burger (v1)..."
V1_RES=$(curl -s -X POST $BASE_URL/foods -d '{"name":"Burger", "calories":500, "protein":25, "carbs":40, "fat":25, "measurement_unit":"count", "measurement_amount":1}')
V1_ID=$(echo $V1_RES | jq -r '.id')
FAMILY_ID=$(echo $V1_RES | jq -r '.family_id')
V1_VERSION=$(echo $V1_RES | jq -r '.version')
V1_CURRENT=$(echo $V1_RES | jq -r '.is_current')

echo "✅ Burger v1 created. ID: $V1_ID, Family: $FAMILY_ID, Version: $V1_VERSION, Current: $V1_CURRENT"

if [ "$V1_VERSION" != "1" ] || [ "$V1_CURRENT" != "true" ]; then
    echo "❌ Expected Version 1 and Current=true"
    exit 1
fi

# 2. Update Burger (Create v2)
echo "2. Updating Burger (v2) - Adjusting calories..."
# Send Family ID to trigger update
V2_PAYLOAD=$(jq -n \
                  --arg fam "$FAMILY_ID" \
                  '{
                    name: "Burger",
                    family_id: $fam,
                    calories: 550, 
                    protein: 25,
                    carbs: 40,
                    fat: 30,
                    measurement_unit: "count",
                    measurement_amount: 1
                  }')

V2_RES=$(curl -s -X POST $BASE_URL/foods -d "$V2_PAYLOAD")

V2_ID=$(echo $V2_RES | jq -r '.id')
V2_VERSION=$(echo $V2_RES | jq -r '.version')
V2_CURRENT=$(echo $V2_RES | jq -r '.is_current')

echo "✅ Burger v2 created. ID: $V2_ID, Version: $V2_VERSION, Current: $V2_CURRENT"

if [ "$V2_VERSION" != "2" ] || [ "$V2_CURRENT" != "true" ]; then
    echo "❌ Expected Version 2 and Current=true"
    exit 1
fi

if [ "$V1_ID" == "$V2_ID" ]; then
    echo "❌ Expected different IDs for versions"
    exit 1
fi

# 3. Verify v1 is no longer current
echo "3. Verifying v1 status..."
V1_FETCH=$(curl -s "$BASE_URL/foods/$V1_ID")
V1_FETCH_CURRENT=$(echo $V1_FETCH | jq -r '.is_current')

echo "ℹ️ v1 Current Status: $V1_FETCH_CURRENT"

if [ "$V1_FETCH_CURRENT" != "false" ]; then
    echo "❌ Expected v1 to be Not Current"
    exit 1
fi

echo "-----------------------------------"
echo "🎉 Versioning Test Passed!"
