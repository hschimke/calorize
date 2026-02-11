#!/bin/bash
# 02_food_mgmt.sh: Food Update (PUT) & Deletion

if [ -z "$BASE_URL" ]; then
    source "$(dirname "$0")/common.sh"
fi

if [ -z "$BANANA_ID" ]; then
    log_err "BANANA_ID not set. Please run 01_basics.sh first."
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
export NEW_BANANA_ID=$(echo $UPDATE_RESP | jq -r .id)
NEW_VERSION=$(echo $UPDATE_RESP | jq -r .version)
NEW_FAMILY=$(echo $UPDATE_RESP | jq -r .family_id)

if [ "$NEW_VERSION" != "2" ]; then
    log_err "Expected version 2, got $NEW_VERSION"
    echo $UPDATE_RESP | jq .
    exit 1
fi
log_info "✅ Version bumped to $NEW_VERSION"

if [ "$NEW_BANANA_ID" == "$BANANA_ID" ]; then
    log_err "Expected new ID, got same ID as original"
    exit 1
fi
log_info "✅ New version has new ID: $NEW_BANANA_ID"

# Fetch original to check family ID match
# Using original BANANA_ID to fetch (it's updated/soft-deleted? No, just old version)
# Actually original script had CANONICAL_BANANA variable with the response.
# I need to fetch it again or store it. I'll fetch it.
OLD_FETCH=$(curl -s "$BASE_URL/foods/$BANANA_ID")
OLD_BANANA_FAMILY=$(echo $OLD_FETCH | jq -r .family_id)
# Wait, Test 5 in original script used $CANONICAL_BANANA to check family ID.
# I didn't export CANONICAL_BANANA.
# But fetching the old ID works.

if [ "$NEW_FAMILY" != "$OLD_BANANA_FAMILY" ]; then
    log_err "Expected same family_id $OLD_BANANA_FAMILY, got $NEW_FAMILY"
    exit 1
fi
log_info "✅ Family ID preserved: $NEW_FAMILY"

echo "Verifying old version still fetchable..."
OLD_IS_CURRENT=$(echo $OLD_FETCH | jq -r .is_current)
if [ "$OLD_IS_CURRENT" != "false" ]; then
    log_err "Old version should have is_current=false, got $OLD_IS_CURRENT"
    exit 1
fi
log_info "✅ Old version marked as not current"

echo "Verifying list only returns current version..."
FOODS_LIST=$(curl -s "$BASE_URL/foods")
BANANA_COUNT=$(echo $FOODS_LIST | jq '[.[] | select(.name=="Banana")] | length')
if [ "$BANANA_COUNT" -ne 1 ]; then
    log_err "Expected 1 Banana in list, got $BANANA_COUNT"
    exit 1
fi
LISTED_BANANA_ID=$(echo $FOODS_LIST | jq -r '[.[] | select(.name=="Banana")][0].id')
if [ "$LISTED_BANANA_ID" != "$NEW_BANANA_ID" ]; then
    log_err "Listed Banana should be new version"
    exit 1
fi
log_info "✅ List shows only current Banana version"

echo "Updating non-existent food..."
RANDOM_UUID="00000000-0000-0000-0000-000000000099"
UPDATE_404=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE_URL/foods/$RANDOM_UUID" \
  -H "Content-Type: application/json" \
  -d '{"name":"Ghost","calories":0,"protein":0,"carbs":0,"fat":0,"type":"food","measurement_unit":"g","measurement_amount":100}')
if [ "$UPDATE_404" == "404" ]; then
    log_info "✅ Update non-existent food returns 404"
else
    log_err "Expected 404, got $UPDATE_404"
fi

echo "Updating food with empty name..."
UPDATE_NONAME=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE_URL/foods/$NEW_BANANA_ID" \
  -H "Content-Type: application/json" \
  -d '{"name":"","calories":95,"protein":1.1,"carbs":24,"fat":0.3,"type":"food","measurement_unit":"g","measurement_amount":100}')
if [ "$UPDATE_NONAME" == "400" ]; then
    log_info "✅ Update with empty name returns 400"
else
    log_err "Expected 400, got $UPDATE_NONAME"
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
log_info "✅ Created temp food: $TEMP_ID"

echo "Deleting temp food..."
DELETE_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/foods/$TEMP_ID")
if [ "$DELETE_CODE" == "204" ]; then
    log_info "✅ Delete returned 204"
else
    log_err "Expected 204, got $DELETE_CODE"
    exit 1
fi

echo "Verifying deleted food excluded from list..."
FOODS_AFTER_DEL=$(curl -s "$BASE_URL/foods")
TEMP_IN_LIST=$(echo $FOODS_AFTER_DEL | jq '[.[] | select(.id=="'$TEMP_ID'")] | length')
if [ "$TEMP_IN_LIST" -eq 0 ]; then
    log_info "✅ Deleted food not in list"
else
    log_err "Deleted food still appears in list"
    exit 1
fi

echo "Verifying GET on deleted food (still fetchable by ID)..."
GET_DELETED=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/foods/$TEMP_ID")
# GetFood has no deleted_at filter, so this should return 200
if [ "$GET_DELETED" == "200" ]; then
    log_info "✅ Deleted food still fetchable by direct ID (expected: no deleted_at filter in GetFood)"
else
    log_warn "GET deleted food returned $GET_DELETED (may indicate deleted_at filtering was added)"
fi

echo "Deleting already-deleted food (idempotency)..."
DELETE_AGAIN=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/foods/$TEMP_ID")
if [ "$DELETE_AGAIN" == "204" ]; then
    log_info "✅ Re-delete returns 204 (idempotent)"
else
    log_warn "Re-delete returned $DELETE_AGAIN"
fi
