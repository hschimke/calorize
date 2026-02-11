#!/bin/bash
# 01_basics.sh: Food & Recipe creation, Nutrients

# Source common if not running from runner
if [ -z "$BASE_URL" ]; then
    source "$(dirname "$0")/common.sh"
fi

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
export BANANA_ID=$(echo $CANONICAL_BANANA | jq -r .id)
log_info "✅ Created Banana ID: $BANANA_ID"

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
export MILK_ID=$(echo $CANONICAL_MILK | jq -r .id)
log_info "✅ Created Milk ID: $MILK_ID"

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
export RECIPE_ID=$(echo $RECIPE_WS | jq -r .id)
log_info "✅ Created Recipe ID: $RECIPE_ID"

echo "Verifying Ingredients..."
FETCHED_RECIPE=$(curl -s "$BASE_URL/foods/$RECIPE_ID")
INGREDIENT_COUNT=$(echo $FETCHED_RECIPE | jq '.ingredients | length')
if [ "$INGREDIENT_COUNT" -ne 2 ]; then
    log_err "Expected 2 ingredients, got $INGREDIENT_COUNT"
    echo $FETCHED_RECIPE | jq .
    exit 1
fi
log_info "✅ Recipe ingredients verified"

# Check specific ingredient exists (Banana ID)
BANANA_AMOUNT=$(echo $FETCHED_RECIPE | jq -r ".ingredients[] | select(.ingredient_id==\"$BANANA_ID\") | .amount")
if [ "$BANANA_AMOUNT" != "100" ]; then
    log_err "Expected Banana amount 100, got $BANANA_AMOUNT"
    exit 1
fi
log_info "✅ Banana ingredient amount verified"

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
log_info "✅ Created Orange ID: $ORANGE_ID"

echo "Verifying Nutrients..."
FETCHED_ORANGE=$(curl -s "$BASE_URL/foods/$ORANGE_ID")
NUTRIENT_COUNT=$(echo $FETCHED_ORANGE | jq '.nutrients | length')
if [ "$NUTRIENT_COUNT" -ne 2 ]; then
    log_err "Expected 2 nutrients, got $NUTRIENT_COUNT"
    echo $FETCHED_ORANGE | jq .
    exit 1
fi

VIT_C=$(echo $FETCHED_ORANGE | jq -r '.nutrients[] | select(.name=="Vitamin C") | .amount')
if [ "$VIT_C" != "53.2" ]; then
    log_err "Expected Vitamin C 53.2, got $VIT_C"
    exit 1
fi
log_info "✅ Nutrients verified"
