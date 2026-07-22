#!/bin/bash
# 11_copy_lineage.sh: Food copy endpoint + copy lineage (foods and log entries)

if [ -z "$BASE_URL" ]; then
    source "$(dirname "$0")/common.sh"
fi

echo "==================================================="
echo "Test: Food Copy & Lineage"
echo "---------------------------------------------------"

# Create the original food
ORIGINAL=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Lineage Original",
  "calories": 250,
  "protein": 12,
  "carbs": 30,
  "fat": 8,
  "type": "food",
  "measurement_unit": "g",
  "measurement_amount": 100
}')
ORIGINAL_ID=$(echo $ORIGINAL | jq -r .id)
ORIGINAL_FAMILY=$(echo $ORIGINAL | jq -r .family_id)
if [ -z "$ORIGINAL_ID" ] || [ "$ORIGINAL_ID" == "null" ]; then
    log_err "Failed to create Lineage Original"
    exit 1
fi
log_info "✅ Created Lineage Original: $ORIGINAL_ID"

# Copy it
COPY1=$(curl -s -X POST "$BASE_URL/foods/$ORIGINAL_ID/copy")
COPY1_ID=$(echo $COPY1 | jq -r .id)
COPY1_FAMILY=$(echo $COPY1 | jq -r .family_id)
COPY1_FROM=$(echo $COPY1 | jq -r .copied_from_id)
if [ -z "$COPY1_ID" ] || [ "$COPY1_ID" == "null" ]; then
    log_err "Failed to copy food"
    echo $COPY1 | jq .
    exit 1
fi
if [ "$COPY1_FROM" != "$ORIGINAL_ID" ]; then
    log_err "Copy's copied_from_id ($COPY1_FROM) should be the original ($ORIGINAL_ID)"
    exit 1
fi
if [ "$COPY1_FAMILY" == "$ORIGINAL_FAMILY" ]; then
    log_err "Copy should start a new family"
    exit 1
fi
VERSION=$(echo $COPY1 | jq -r .version)
if [ "$VERSION" != "1" ]; then
    log_err "Copy should be version 1, got $VERSION"
    exit 1
fi
log_info "✅ Copied food: $COPY1_ID (new family, copied_from recorded)"

# Copy the copy (chain: original -> copy1 -> copy2)
COPY2=$(curl -s -X POST "$BASE_URL/foods/$COPY1_ID/copy")
COPY2_ID=$(echo $COPY2 | jq -r .id)
COPY2_FROM=$(echo $COPY2 | jq -r .copied_from_id)
if [ "$COPY2_FROM" != "$COPY1_ID" ]; then
    log_err "Second copy's copied_from_id ($COPY2_FROM) should be the first copy ($COPY1_ID)"
    exit 1
fi
log_info "✅ Copied the copy: $COPY2_ID"

# Lineage of the leaf: 2 ancestors (copy1, original) and a 3-level tree
LINEAGE=$(curl -s "$BASE_URL/foods/$COPY2_ID/lineage")
ANC_COUNT=$(echo $LINEAGE | jq '.ancestors | length')
if [ "$ANC_COUNT" != "2" ]; then
    log_err "Expected 2 ancestors, got $ANC_COUNT"
    echo $LINEAGE | jq .
    exit 1
fi
ANC0=$(echo $LINEAGE | jq -r '.ancestors[0].food_id')
ANC1=$(echo $LINEAGE | jq -r '.ancestors[1].food_id')
if [ "$ANC0" != "$COPY1_ID" ] || [ "$ANC1" != "$ORIGINAL_ID" ]; then
    log_err "Ancestors should be [copy1, original] nearest-first, got [$ANC0, $ANC1]"
    exit 1
fi
TREE_ROOT_FAMILY=$(echo $LINEAGE | jq -r '.tree.family_id')
if [ "$TREE_ROOT_FAMILY" != "$ORIGINAL_FAMILY" ]; then
    log_err "Tree root should be the original's family"
    echo $LINEAGE | jq .
    exit 1
fi
GRANDCHILD=$(echo $LINEAGE | jq -r '.tree.children[0].children[0].food_id')
if [ "$GRANDCHILD" != "$COPY2_ID" ]; then
    log_err "Tree should chain original -> copy1 -> copy2, got grandchild $GRANDCHILD"
    echo $LINEAGE | jq .
    exit 1
fi
log_info "✅ Lineage: 2 ancestors and 3-level tree verified"

# Rename the first copy; lineage must survive the new version
RENAMED=$(curl -s -X PUT "$BASE_URL/foods/$COPY1_ID" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Lineage Copy Renamed",
  "calories": 250,
  "protein": 12,
  "carbs": 30,
  "fat": 8,
  "type": "food",
  "measurement_unit": "g",
  "measurement_amount": 100
}')
RENAMED_ID=$(echo $RENAMED | jq -r .id)
RENAMED_FROM=$(echo $RENAMED | jq -r .copied_from_id)
if [ "$RENAMED_FROM" != "$ORIGINAL_ID" ]; then
    log_err "copied_from_id should survive an update, got $RENAMED_FROM"
    exit 1
fi
LINEAGE2=$(curl -s "$BASE_URL/foods/$COPY2_ID/lineage")
ANC_COUNT2=$(echo $LINEAGE2 | jq '.ancestors | length')
TREE_CHILD_FOOD=$(echo $LINEAGE2 | jq -r '.tree.children[0].food.name')
if [ "$ANC_COUNT2" != "2" ] || [ "$TREE_CHILD_FOOD" != "Lineage Copy Renamed" ]; then
    log_err "Lineage should stay intact after rename (ancestors=$ANC_COUNT2, child=$TREE_CHILD_FOOD)"
    echo $LINEAGE2 | jq .
    exit 1
fi
log_info "✅ Lineage intact after renaming the copy (tree shows current version)"

# Copying a nonexistent food returns 404
MISSING_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/foods/00000000-0000-0000-0000-000000000000/copy")
if [ "$MISSING_CODE" == "404" ]; then
    log_info "✅ Copy of nonexistent food returns 404"
else
    log_err "Expected 404 for nonexistent food, got $MISSING_CODE"
    exit 1
fi

# Log-entry copy lineage: log the food yesterday, copy the day, check copied_from_id
YESTERDAY=$(date -u -v-1d +"%Y-%m-%d" 2>/dev/null || date -u -d "yesterday" +"%Y-%m-%d")
YESTERDAY_ISO="${YESTERDAY}T12:00:00Z"
TODAY=$(date -u +"%Y-%m-%d")

SRC_LOG=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{\"food_id\": \"$RENAMED_ID\", \"amount\": 100, \"meal_tag\": \"snack\", \"logged_at\": \"$YESTERDAY_ISO\"}")
SRC_LOG_ID=$(echo $SRC_LOG | jq -r .id)
if [ -z "$SRC_LOG_ID" ] || [ "$SRC_LOG_ID" == "null" ]; then
    log_err "Failed to create source log entry"
    exit 1
fi
SRC_FROM=$(echo $SRC_LOG | jq -r '.copied_from_id // "null"')
if [ "$SRC_FROM" != "null" ]; then
    log_err "Fresh log entry should have no copied_from_id, got $SRC_FROM"
    exit 1
fi

COPY_RESULT=$(curl -s -X POST "$BASE_URL/logs/copy" \
  -H "Content-Type: application/json" \
  -d "{\"from_date\": \"$YESTERDAY\", \"to_date\": \"$TODAY\", \"meal_tags\": [\"snack\"]}")
COPY_COUNT=$(echo $COPY_RESULT | jq -r .count)
if [ "$COPY_COUNT" != "1" ]; then
    log_err "Expected 1 log entry copied, got $COPY_COUNT"
    exit 1
fi

TODAY_LOGS=$(curl -s "$BASE_URL/logs?date=$TODAY")
COPIED_FROM=$(echo $TODAY_LOGS | jq -r --arg src "$SRC_LOG_ID" '.[] | select(.copied_from_id == $src) | .copied_from_id')
if [ "$COPIED_FROM" == "$SRC_LOG_ID" ]; then
    log_info "✅ Copied log entry records copied_from_id"
else
    log_err "Copied log entry should record copied_from_id = $SRC_LOG_ID"
    echo $TODAY_LOGS | jq .
    exit 1
fi

# Log lineage summary: copied entry traces back to its origin with 1 copy-step
COPIED_LOG_ID=$(echo $TODAY_LOGS | jq -r --arg src "$SRC_LOG_ID" '.[] | select(.copied_from_id == $src) | .id')
LOG_LINEAGE=$(curl -s "$BASE_URL/logs/$COPIED_LOG_ID/lineage")
ORIGIN_ID=$(echo $LOG_LINEAGE | jq -r '.origin.id')
COPIES=$(echo $LOG_LINEAGE | jq -r '.copies')
if [ "$ORIGIN_ID" == "$SRC_LOG_ID" ] && [ "$COPIES" == "1" ]; then
    log_info "✅ Log lineage summary: origin + 1 copy-step"
else
    log_err "Expected origin=$SRC_LOG_ID copies=1, got origin=$ORIGIN_ID copies=$COPIES"
    echo $LOG_LINEAGE | jq .
    exit 1
fi

# Lineage of a nonexistent log entry returns 404
LOG_MISSING_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/logs/00000000-0000-0000-0000-000000000000/lineage")
if [ "$LOG_MISSING_CODE" == "404" ]; then
    log_info "✅ Lineage of nonexistent log entry returns 404"
else
    log_err "Expected 404 for nonexistent log entry lineage, got $LOG_MISSING_CODE"
    exit 1
fi

# Cleanup: logs first, then foods
curl -s -X DELETE "$BASE_URL/logs/$SRC_LOG_ID" > /dev/null
echo $TODAY_LOGS | jq -r --arg src "$SRC_LOG_ID" '.[] | select(.copied_from_id == $src) | .id' | while read id; do
    curl -s -X DELETE "$BASE_URL/logs/$id" > /dev/null
done
curl -s -X DELETE "$BASE_URL/foods/$COPY2_ID" > /dev/null
curl -s -X DELETE "$BASE_URL/foods/$COPY1_ID" > /dev/null
curl -s -X DELETE "$BASE_URL/foods/$ORIGINAL_ID" > /dev/null
log_info "✅ Copy lineage cleanup done"

echo "==================================================="
echo "Food Copy & Lineage tests complete"
echo "==================================================="
