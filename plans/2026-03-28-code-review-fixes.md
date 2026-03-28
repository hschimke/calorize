# Code Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all issues raised in the 2026-03-28 code review of the FDC importer improvements and passkey management feature.

**Architecture:** Fixes span the Go backend (importer, DB layer, API handlers) and vanilla JS frontend (api.js, ui.js, account.js). Each task is independently committable. No schema changes needed.

**Tech Stack:** Go 1.22+, standard `net/http`, SQLite, vanilla JS (ES modules), no build step.

---

## Files to Modify

| File | Changes |
|------|---------|
| `.gitignore` | Add `*.db-shm` and `*.db-wal` |
| `internal/importer/fdc.go` | Fix decode error handling; fix calorie warning levels |
| `internal/db/users.go` | Add `ErrCredentialNotFound` sentinel; add `RowsAffected` checks to `RenameCredential` and `RemoveUserCredential` |
| `internal/api/account.go` | Handle `ErrCredentialNotFound` with 404; fix `LastUsedAt`; add `"errors"` import |
| `static-web/js/api.js` | Add `credentials: 'same-origin'` to fetch options |
| `static-web/js/ui.js` | Add `showInput(message, defaultValue)` export |
| `static-web/js/account.js` | Import `showInput`; replace `prompt()` in `renamePasskey`; remove dead `className` |
| `tests/07_account_passkeys.sh` | New integration test file |
| `test_runner.sh` | Add `tests/07_account_passkeys.sh` to test list |

---

## Task 1: Clean up .gitignore and remove committed WAL files

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Add WAL file patterns to .gitignore**

In `.gitignore`, after the `*.db` line (line 35), add:

```
*.db-shm
*.db-wal
```

The relevant section currently reads:
```
# other
*.db
.worktrees/
```

Change it to:
```
# other
*.db
*.db-shm
*.db-wal
.worktrees/
```

- [ ] **Step 2: Untrack the committed WAL files**

```bash
git rm --cached internal/db/test.db-shm internal/db/test.db-wal internal/importer/test.db-shm internal/importer/test.db-wal
```

Expected output: four lines of `rm 'internal/...'`

- [ ] **Step 3: Verify the files are untracked but still present on disk**

```bash
git status
ls internal/db/test.db-shm internal/importer/test.db-shm
```

Expected: `git status` shows the four files as "Changes to be committed: deleted" and they still exist locally.

- [ ] **Step 4: Commit**

```bash
git add .gitignore
git commit -m "chore: ignore SQLite WAL files and untrack committed test artifacts"
```

---

## Task 2: Fix calorie source warning levels in FDC importer

**Files:**
- Modify: `internal/importer/fdc.go` (around lines 242–244)

The Atwater resolution paths (`atwater_specific`, `atwater_general`) are standard and expected for Foundation Foods. Only `kj_converted` and `macro_estimate` represent genuinely unusual fallbacks worth warning about.

- [ ] **Step 1: Change the calorie source warning block**

Find this block (around line 242):
```go
// Warn on unusual resolution paths or suspicious values
if calSource != "kcal" {
    slog.Warn("non-standard calorie source", "fdcId", fdcFood.FdcID, "description", fdcFood.Description, "cal_source", calSource, "calories", calories)
}
```

Replace with:
```go
// Log unusual resolution paths
if calSource == "kj_converted" || calSource == "macro_estimate" {
    slog.Warn("unusual calorie source", "fdcId", fdcFood.FdcID, "description", fdcFood.Description, "cal_source", calSource, "calories", calories)
} else if calSource != "kcal" {
    slog.Debug("non-kcal calorie source", "fdcId", fdcFood.FdcID, "description", fdcFood.Description, "cal_source", calSource, "calories", calories)
}
```

- [ ] **Step 2: Build to verify no compilation errors**

```bash
go build ./internal/importer/...
```

Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/importer/fdc.go
git commit -m "fix: demote Atwater calorie resolution to debug log level"
```

---

## Task 3: Fix JSON stream decode error handling in FDC importer

**Files:**
- Modify: `internal/importer/fdc.go` (around lines 83–90)

The current code calls `continue` on any decode error. This is only safe for `*json.UnmarshalTypeError` (field has wrong type — decoder consumed the full token). A `json.SyntaxError` or `io.ErrUnexpectedEOF` corrupts the stream position, and subsequent calls to `decoder.More()` and `decoder.Decode()` will either loop on the same error or produce garbage.

- [ ] **Step 1: Add `encoding/json` and `errors` imports if not already present**

Check the import block at the top of `internal/importer/fdc.go`. The file already imports `"encoding/json"`. It does **not** currently import `"errors"`. Add `"errors"` to the import block:

```go
import (
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "log/slog"
    "os"
    "time"

    "azule.info/calorize/internal/db"
)
```

- [ ] **Step 2: Replace the decode error handling block**

Find this block (around line 83):
```go
var fdcFood FdcFood
err := decoder.Decode(&fdcFood)
if err != nil {
    errCount++
    slog.Error("error decoding FDC record", "index", count+errCount+skippedCount, "error", err)
    // Non-fatal: skip malformed record and continue
    continue
}
```

Replace with:
```go
var fdcFood FdcFood
err := decoder.Decode(&fdcFood)
if err != nil {
    var typeErr *json.UnmarshalTypeError
    if errors.As(err, &typeErr) {
        // Field type mismatch: decoder consumed the full token cleanly, safe to skip
        errCount++
        slog.Warn("skipping FDC record with type mismatch", "index", count+errCount+skippedCount, "field", typeErr.Field, "error", err)
        continue
    }
    // Syntax error or I/O error: stream position is undefined, cannot recover
    return fmt.Errorf("unrecoverable decode error at index %d: %w", count+errCount+skippedCount, err)
}
```

- [ ] **Step 3: Build and run unit tests**

```bash
go build ./internal/importer/... && go test ./internal/importer/...
```

Expected: builds clean, tests pass (or "no test files").

- [ ] **Step 4: Commit**

```bash
git add internal/importer/fdc.go
git commit -m "fix: only skip FDC decode errors that leave stream in clean state"
```

---

## Task 4: Add ErrCredentialNotFound and RowsAffected checks in db/users.go

**Files:**
- Modify: `internal/db/users.go`

`RenameCredential` and `RemoveUserCredential` currently discard `sql.Result`. Zero rows affected (credential ID not found or doesn't belong to this user) silently returns nil — the caller receives a success signal for a no-op operation.

- [ ] **Step 1: Add `ErrCredentialNotFound` sentinel and the `"errors"` import**

At the top of `internal/db/users.go`, the import block already includes `"errors"`. Add `ErrCredentialNotFound` as a package-level variable near the top of the file, after the imports:

```go
var ErrCredentialNotFound = errors.New("credential not found")
```

- [ ] **Step 2: Update `RemoveUserCredential` to check rows affected**

Find the current implementation (around line 152):
```go
func RemoveUserCredential(user User, auth UserCredential) error {
    query := `DELETE FROM user_credentials WHERE id = ? AND user_id = ?`
    _, err := db.Exec(query, auth.ID, user.ID)
    if err != nil {
        return fmt.Errorf("removing user credential: %w", err)
    }
    return nil
}
```

Replace with:
```go
func RemoveUserCredential(user User, auth UserCredential) error {
    query := `DELETE FROM user_credentials WHERE id = ? AND user_id = ?`
    result, err := db.Exec(query, auth.ID, user.ID)
    if err != nil {
        return fmt.Errorf("removing user credential: %w", err)
    }
    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("removing user credential: %w", err)
    }
    if rows == 0 {
        return ErrCredentialNotFound
    }
    return nil
}
```

- [ ] **Step 3: Update `RenameCredential` to check rows affected**

Find the current implementation (added in the passkey feature, near the end of the credential functions):
```go
func RenameCredential(userID UserID, credID UserCredentialID, name string) error {
    query := `UPDATE user_credentials SET name = ? WHERE id = ? AND user_id = ?`
    _, err := db.Exec(query, name, credID, userID)
    if err != nil {
        return fmt.Errorf("renaming credential: %w", err)
    }
    return nil
}
```

Replace with:
```go
func RenameCredential(userID UserID, credID UserCredentialID, name string) error {
    query := `UPDATE user_credentials SET name = ? WHERE id = ? AND user_id = ?`
    result, err := db.Exec(query, name, credID, userID)
    if err != nil {
        return fmt.Errorf("renaming credential: %w", err)
    }
    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("renaming credential: %w", err)
    }
    if rows == 0 {
        return ErrCredentialNotFound
    }
    return nil
}
```

- [ ] **Step 4: Build and run unit tests**

```bash
go build ./internal/db/... && go test ./internal/db/...
```

Expected: builds clean, tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/db/users.go
git commit -m "fix: return ErrCredentialNotFound when rename/delete affects zero rows"
```

---

## Task 5: Handle ErrCredentialNotFound as 404 in account handlers

**Files:**
- Modify: `internal/api/account.go`

The `deletePasskeyHandler` and `renamePasskeyHandler` currently treat all non-nil errors from the DB as 500. Now that `db.ErrCredentialNotFound` exists, these handlers need to translate it to a 404 response. Also fix `addPasskeyFinishHandler` to capture `time.Now()` once and use it for both `CreatedAt` and `LastUsedAt`, so a newly-registered passkey doesn't display a misleading "Last used [creation time]" date.

- [ ] **Step 1: Add `"errors"` to the import block in account.go**

The current import block:
```go
import (
    "encoding/base64"
    "encoding/json"
    "log/slog"
    "net/http"
    "time"

    "azule.info/calorize/internal/auth"
    "azule.info/calorize/internal/db"
    "github.com/google/uuid"
)
```

Add `"errors"`:
```go
import (
    "encoding/base64"
    "encoding/json"
    "errors"
    "log/slog"
    "net/http"
    "time"

    "azule.info/calorize/internal/auth"
    "azule.info/calorize/internal/db"
    "github.com/google/uuid"
)
```

- [ ] **Step 2: Update the delete handler's error handling**

Find in `deletePasskeyHandler` (around line 93):
```go
if err := db.RemoveUserCredential(*user, db.UserCredential{ID: db.UserCredentialID(credIDBytes)}); err != nil {
    slog.Error("failed to remove credential", "error", err)
    http.Error(w, "Failed to delete passkey", http.StatusInternalServerError)
    return
}
```

Replace with:
```go
if err := db.RemoveUserCredential(*user, db.UserCredential{ID: db.UserCredentialID(credIDBytes)}); err != nil {
    if errors.Is(err, db.ErrCredentialNotFound) {
        http.Error(w, "Passkey not found", http.StatusNotFound)
        return
    }
    slog.Error("failed to remove credential", "error", err)
    http.Error(w, "Failed to delete passkey", http.StatusInternalServerError)
    return
}
```

- [ ] **Step 3: Update the rename handler's error handling**

Find in `renamePasskeyHandler` (around line 122):
```go
if err := db.RenameCredential(userID, db.UserCredentialID(credIDBytes), body.Name); err != nil {
    slog.Error("failed to rename credential", "error", err)
    http.Error(w, "Failed to rename passkey", http.StatusInternalServerError)
    return
}
```

Replace with:
```go
if err := db.RenameCredential(userID, db.UserCredentialID(credIDBytes), body.Name); err != nil {
    if errors.Is(err, db.ErrCredentialNotFound) {
        http.Error(w, "Passkey not found", http.StatusNotFound)
        return
    }
    slog.Error("failed to rename credential", "error", err)
    http.Error(w, "Failed to rename passkey", http.StatusInternalServerError)
    return
}
```

- [ ] **Step 4: Fix LastUsedAt in addPasskeyFinishHandler**

Find in `addPasskeyFinishHandler` (around line 187):
```go
if err := db.AddUserCredential(*user, db.UserCredential{
    ID:              db.UserCredentialID(credential.ID),
    Name:            "New Passkey",
    PublicKey:       credential.PublicKey,
    AttestationType: credential.AttestationType,
    AAGUID:          uuid.UUID(credential.Authenticator.AAGUID).String(),
    SignCount:       credential.Authenticator.SignCount,
    BackupEligible:  credential.Flags.BackupEligible,
    BackupState:     credential.Flags.BackupState,
    CreatedAt:       time.Now().UTC(),
    LastUsedAt:      time.Now().UTC(),
}); err != nil {
```

Replace with (capture `now` once so both fields get the exact same timestamp):
```go
now := time.Now().UTC()
if err := db.AddUserCredential(*user, db.UserCredential{
    ID:              db.UserCredentialID(credential.ID),
    Name:            "New Passkey",
    PublicKey:       credential.PublicKey,
    AttestationType: credential.AttestationType,
    AAGUID:          uuid.UUID(credential.Authenticator.AAGUID).String(),
    SignCount:       credential.Authenticator.SignCount,
    BackupEligible:  credential.Flags.BackupEligible,
    BackupState:     credential.Flags.BackupState,
    CreatedAt:       now,
    LastUsedAt:      now,
}); err != nil {
```

- [ ] **Step 5: Build to verify no compilation errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add internal/api/account.go
git commit -m "fix: return 404 for missing passkey; align LastUsedAt with CreatedAt on registration"
```

---

## Task 6: Add credentials: 'same-origin' to fetch options in api.js

**Files:**
- Modify: `static-web/js/api.js` (around line 17)

The `request()` helper relies on the browser's default credential behaviour for cookie auth. Making it explicit prevents subtle breakage if the client is ever used cross-origin or embedded in a different context.

- [ ] **Step 1: Update the fetch options object**

Find in the `request()` method (around line 17):
```js
const options = {
    method,
    headers: {
        'Content-Type': 'application/json',
    },
};
```

Replace with:
```js
const options = {
    method,
    credentials: 'same-origin',
    headers: {
        'Content-Type': 'application/json',
    },
};
```

- [ ] **Step 2: Commit**

```bash
git add static-web/js/api.js
git commit -m "fix: explicitly set credentials: same-origin on all API fetch calls"
```

---

## Task 7: Add showInput to ui.js and use it in account.js rename flow

**Files:**
- Modify: `static-web/js/ui.js`
- Modify: `static-web/js/account.js`

Replace the native `window.prompt()` call in `renamePasskey` with a styled modal that uses the project's design system variables — consistent with how `showConfirm` works.

- [ ] **Step 1: Add `showInput` export to ui.js**

Append the following to the end of `static-web/js/ui.js` (after the closing brace of `showConfirm`):

```js
// ============================================================
// Input Dialog
// ============================================================

export function showInput(message, defaultValue = '') {
    return new Promise((resolve) => {
        const overlay = document.createElement('div');
        overlay.style.cssText = [
            'position: fixed',
            'inset: 0',
            'background: rgba(0,0,0,0.4)',
            'display: flex',
            'align-items: center',
            'justify-content: center',
            'z-index: 2000',
        ].join('; ');

        const dialog = document.createElement('div');
        dialog.style.cssText = [
            'background: var(--color-surface)',
            'border-radius: var(--radius-md)',
            'padding: var(--space-6)',
            'max-width: 360px',
            'width: 90%',
            'box-shadow: 0 8px 32px rgba(0,0,0,0.2)',
        ].join('; ');

        const msg = document.createElement('p');
        msg.textContent = message;
        msg.style.cssText = 'margin: 0 0 var(--space-4); font-size: 1rem; color: var(--color-text);';

        const input = document.createElement('input');
        input.type = 'text';
        input.value = defaultValue;
        input.style.cssText = 'display: block; margin-bottom: var(--space-5);';

        const actions = document.createElement('div');
        actions.style.cssText = 'display: flex; gap: var(--space-3); justify-content: flex-end;';

        const cancelBtn = document.createElement('button');
        cancelBtn.textContent = 'Cancel';
        cancelBtn.className = 'btn btn-secondary';
        cancelBtn.onclick = () => { overlay.remove(); resolve(null); };

        const saveBtn = document.createElement('button');
        saveBtn.textContent = 'Save';
        saveBtn.className = 'btn btn-primary';
        saveBtn.onclick = () => { overlay.remove(); resolve(input.value.trim() || null); };

        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') saveBtn.click();
            if (e.key === 'Escape') cancelBtn.click();
        });

        actions.appendChild(cancelBtn);
        actions.appendChild(saveBtn);
        dialog.appendChild(msg);
        dialog.appendChild(input);
        dialog.appendChild(actions);
        overlay.appendChild(dialog);
        document.body.appendChild(overlay);

        input.focus();
        input.select();
    });
}
```

- [ ] **Step 2: Update account.js to import and use showInput**

Change the import line at the top of `static-web/js/account.js`:
```js
import { showToast, showConfirm } from './ui.js';
```
→
```js
import { showToast, showConfirm, showInput } from './ui.js';
```

- [ ] **Step 3: Replace prompt() in renamePasskey**

Find in `account.js` (around line 74):
```js
async function renamePasskey(id, currentName) {
    const name = prompt('New name for this passkey:', currentName);
    if (!name || name.trim() === '') return;
    try {
        await api.renamePasskey(id, name.trim());
```

Replace with:
```js
async function renamePasskey(id, currentName) {
    const name = await showInput('New name for this passkey:', currentName);
    if (!name) return;
    try {
        await api.renamePasskey(id, name);
```

(Note: `showInput` already trims whitespace and returns `null` for blank, so `name.trim()` is no longer needed.)

- [ ] **Step 4: Commit**

```bash
git add static-web/js/ui.js static-web/js/account.js
git commit -m "fix: replace native prompt() with styled modal for passkey rename"
```

---

## Task 8: Fix dead className and minor cosmetic issues in account.js / account.go

**Files:**
- Modify: `static-web/js/account.js` (line 34)

- [ ] **Step 1: Remove the dead className assignment**

Find in `account.js` (around line 33):
```js
const detailEl = document.createElement('small');
detailEl.className = 'color-text-muted';
const createdDate = new Date(pk.created_at).toLocaleDateString();
```

Remove the `className` assignment entirely:
```js
const detailEl = document.createElement('small');
const createdDate = new Date(pk.created_at).toLocaleDateString();
```

`color-text-muted` is a CSS custom property, not a class. The `<small>` inside `.food-info` already receives the correct muted color via the `.food-info small` rule in `main.css`.

- [ ] **Step 2: Commit**

```bash
git add static-web/js/account.js
git commit -m "fix: remove dead color-text-muted className on passkey detail element"
```

---

## Task 9: Add integration tests for account passkey endpoints

**Files:**
- Create: `tests/07_account_passkeys.sh`
- Modify: `test_runner.sh`

These tests run against a server started with `DEV_AUTH=true`. In dev mode the dev user has no WebAuthn credentials, so the list endpoint returns an empty array. This lets us verify:
1. List endpoint returns 200 and a JSON array.
2. Rename endpoint returns 404 for a non-existent credential ID (verifies the ErrCredentialNotFound fix).
3. Delete endpoint returns 400 when user has no credentials (the guard prevents deleting the "last" passkey when count is 0).

- [ ] **Step 1: Create tests/07_account_passkeys.sh**

```bash
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
```

- [ ] **Step 2: Make the test file executable**

```bash
chmod +x tests/07_account_passkeys.sh
```

- [ ] **Step 3: Add the test to test_runner.sh**

Find in `test_runner.sh`:
```bash
TEST_FILES=(
    "tests/01_basics.sh"
    "tests/02_food_mgmt.sh"
    "tests/03_logging.sh"
    "tests/04_stats_reads.sh"
    "tests/05_input_validation.sh"
)
```

Replace with:
```bash
TEST_FILES=(
    "tests/01_basics.sh"
    "tests/02_food_mgmt.sh"
    "tests/03_logging.sh"
    "tests/04_stats_reads.sh"
    "tests/05_input_validation.sh"
    "tests/07_account_passkeys.sh"
)
```

- [ ] **Step 4: Verify the test runs (requires server running with DEV_AUTH=true)**

```bash
DEV_AUTH=true go run ./cmd/api-server/main.go &
sleep 2
source tests/common.sh && source tests/07_account_passkeys.sh
kill %1
```

Expected output:
```
[INFO] ✅ GET /account/passkeys → 200, array
[INFO] ✅ PATCH /account/passkeys/{nonexistent} → 404
[INFO] ✅ DELETE /account/passkeys/{id} with 0 passkeys → 400
```

- [ ] **Step 5: Commit**

```bash
git add tests/07_account_passkeys.sh test_runner.sh
git commit -m "test: add integration tests for account passkey endpoints"
```

---

## Verification

After all tasks are complete, run:

```bash
go build ./... && go test ./...
```

Expected: all packages build clean, all Go tests pass.

To run the full integration suite:
```bash
DEV_AUTH=true go run ./cmd/api-server/main.go &
sleep 2
./test_runner.sh
kill %1
```
