#!/bin/bash
# 07_account_passkeys.sh: Account passkey management endpoints

if [ -z "$BASE_URL" ]; then
    source "$(dirname "$0")/common.sh"
fi

echo "==================================================="
echo "Test: Account Passkey Management"
echo "---------------------------------------------------"

# Test 1: List passkeys returns 200 and a JSON array
echo "Listing passkeys..."
LIST_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/account/passkeys")
LIST_BODY=$(echo "$LIST_RESP" | head -n -1)
LIST_CODE=$(echo "$LIST_RESP" | tail -n 1)

if [ "$LIST_CODE" != "200" ]; then
    log_err "Expected 200, got $LIST_CODE"
    exit 1
fi
if ! echo "$LIST_BODY" | jq -e 'type == "array"' > /dev/null 2>&1; then
    log_err "Expected JSON array, got: $LIST_BODY"
    exit 1
fi
log_info "✅ GET /account/passkeys → 200, array"

# Test 2: Rename a non-existent passkey returns 404
FAKE_ID="AAAAAAAAAAAAAAAAAAA"
echo "Renaming non-existent passkey (expect 404)..."
RENAME_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$BASE_URL/account/passkeys/$FAKE_ID" \
  -H "Content-Type: application/json" \
  -d '{"name": "Ghost Key"}')

if [ "$RENAME_CODE" != "404" ]; then
    log_err "Expected 404 for rename of missing passkey, got $RENAME_CODE"
    exit 1
fi
log_info "✅ PATCH /account/passkeys/{nonexistent} → 404"

# Test 3: Delete when no passkeys exist returns 400 (cannot delete last/only)
echo "Deleting passkey when none exist (expect 400)..."
DELETE_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/account/passkeys/$FAKE_ID")

if [ "$DELETE_CODE" != "400" ]; then
    log_err "Expected 400 for delete with no passkeys, got $DELETE_CODE"
    exit 1
fi
log_info "✅ DELETE /account/passkeys/{id} with 0 passkeys → 400"

echo "==================================================="
