#!/bin/bash
set -e

BASE_URL="http://localhost:8383"

echo "Testing Calorize API - Recipes"
echo "-----------------------------------"

# 1. Create Flour
echo "1. Creating Flour..."
FLOUR_RES=$(curl -s -X POST $BASE_URL/foods -d '{"name":"Flour", "calories":364, "protein":10, "carbs":76, "fat":1, "measurement_unit":"g", "measurement_amount":100}')
FLOUR_ID=$(echo $FLOUR_RES | jq -r '.id')
echo "✅ Flour created: $FLOUR_ID"

# 2. Create Sugar
echo "2. Creating Sugar..."
SUGAR_RES=$(curl -s -X POST $BASE_URL/foods -d '{"name":"Sugar", "calories":387, "protein":0, "carbs":100, "fat":0, "measurement_unit":"g", "measurement_amount":100}')
SUGAR_ID=$(echo $SUGAR_RES | jq -r '.id')
echo "✅ Sugar created: $SUGAR_ID"

# 3. Create Cookie Recipe
echo "3. Creating Cookie Recipe..."
# Recipe: 200g Flour, 100g Sugar
RECIPE_PAYLOAD=$(jq -n \
                  --arg flour "$FLOUR_ID" \
                  --arg sugar "$SUGAR_ID" \
                  '{
                    name: "Cookie",
                    type: "recipe",
                    calories: 800,
                    protein: 20,
                    carbs: 252,
                    fat: 2,
                    measurement_unit: "count",
                    measurement_amount: 10,
                    ingredients: {
                        ($flour): 200,
                        ($sugar): 100
                    }
                  }')

RECIPE_RES=$(curl -s -X POST $BASE_URL/foods -d "$RECIPE_PAYLOAD")
RECIPE_ID=$(echo $RECIPE_RES | jq -r '.id')
echo "✅ Recipe created: $RECIPE_ID"

# 4. Fetch Recipe and Verify Ingredients
echo "4. Fetching Recipe..."
FETCH_RES=$(curl -s "$BASE_URL/foods/$RECIPE_ID")

echo "$FETCH_RES" | jq .

# Verify raw output contains ingredients
echo "$FETCH_RES" | grep -q "Flour" && echo "✅ Found 'Flour' in ingredients" || (echo "❌ 'Flour' missing"; exit 1)
echo "$FETCH_RES" | grep -q "Sugar" && echo "✅ Found 'Sugar' in ingredients" || (echo "❌ 'Sugar' missing"; exit 1)

echo "-----------------------------------"
echo "🎉 Recipe Test Passed!"
