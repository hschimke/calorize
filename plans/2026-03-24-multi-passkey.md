# Plan: Passkey Management (Account Page)

## Context
Users currently can only register one passkey at initial sign-up, and there is no way to add more or remove old ones. This adds a protected Account page where users can view all their passkeys, add new ones (triggering a second WebAuthn registration ceremony for an authenticated user), delete existing ones, and rename them.

The `user_credentials` table already supports multiple credentials per user and all necessary DB helpers (`GetUserCredentials`, `AddUserCredential`, `RemoveUserCredential`) already exist. The WebAuthn library's `BeginRegistration` automatically populates `ExcludeCredentials` from `WebAuthnCredentials()`, preventing re-registration of existing passkeys.

---

## Files to Create or Modify

### New files
- `internal/api/account.go` — passkey management API handlers
- `static-web/account.html` — Account page
- `static-web/js/account.js` — Account page JS

### Modified files
- `internal/db/users.go` — add `RenameCredential` DB helper
- `internal/api/api.go` — call `RegisterAccountPaths` in `RegisterApiPaths`
- `static-web/js/api.js` — add client methods for passkey management
- `static-web/js/bootstrap.js` — add "Account" nav link

---

## Backend

### 1. `internal/db/users.go` — Add rename helper

```go
func RenameCredential(userID UserID, credID UserCredentialID, name string) error {
    query := `UPDATE user_credentials SET name = ? WHERE id = ? AND user_id = ?`
    _, err := db.Exec(query, name, credID, userID)
    return err
}
```

### 2. `internal/api/account.go` (new file)

Register these routes (all protected via existing RequireAuth middleware since they go on `apiMux`):

```
GET    /account/passkeys           → list passkeys
DELETE /account/passkeys/{id}      → delete a passkey
PATCH  /account/passkeys/{id}      → rename a passkey
POST   /account/passkeys/begin     → begin adding new passkey (WebAuthn)
POST   /account/passkeys/finish    → finish adding new passkey (WebAuthn)
```

**List handler** — returns JSON array of safe credential view structs (no public keys):
```go
type passkeyView struct {
    ID         string    `json:"id"`          // base64url-encoded credential ID
    Name       string    `json:"name"`
    CreatedAt  time.Time `json:"created_at"`
    LastUsedAt time.Time `json:"last_used_at"`
}
```

**Delete handler**:
- Decode base64url `{id}` → `UserCredentialID`
- Fetch user's credentials count; reject with 400 if only 1 remains (can't lock yourself out)
- Call `db.RemoveUserCredential(user, cred)`

**Rename handler** — accepts `{"name": "..."}` JSON body, calls `db.RenameCredential()`

**Add passkey begin handler**:
- Get `userID` from context → fetch `*db.User`
- Wrap in `auth.WebAuthnUser{User: user}`
- Call `auth.WebAuthn.BeginRegistration(&wUser)` — library automatically excludes existing credentials
- Save session data to `reg_session` cookie (reuse `auth.saveSession` — make it exported or inline the cookie write)
- Return options JSON

**Add passkey finish handler**:
- Get `userID` from context → fetch `*db.User`
- Load session from `reg_session` cookie (reuse `auth.loadSession`)
- Call `auth.WebAuthn.FinishRegistration(&wUser, sessionData, r)`
- Save credential via `db.AddUserCredential(user, db.UserCredential{...})` with `Name: "New Passkey"`
- Clear `reg_session` cookie
- Return `{"message": "Passkey added"}`

> **Note on session helpers**: `saveSession`, `loadSession`, `clearSession` are currently unexported in `internal/auth`. Since `internal/api` is a sibling package, they must be exported (capitalize to `SaveSession`, `LoadSession`, `ClearSession`) or the begin/finish handlers must live in `internal/auth`. The cleanest approach: export the three helpers in `auth.go`.

### 3. `internal/api/api.go`
Add `RegisterAccountPaths(mux)` to `RegisterApiPaths`:
```go
func RegisterApiPaths(mux *http.ServeMux) {
    RegisterLogsPaths(mux)
    RegisterFoodsPaths(mux)
    RegisterStatsPaths(mux)
    RegisterAccountPaths(mux)
}
```

---

## Frontend

### 4. `static-web/js/api.js` — Add methods
```js
async getPasskeys()                     // GET /account/passkeys
async addPasskey()                      // begin + WebAuthn ceremony + finish
async deletePasskey(id)                 // DELETE /account/passkeys/{id}
async renamePasskey(id, name)           // PATCH /account/passkeys/{id}
```

`addPasskey()` flow mirrors `register()` and `login()` already in `api.js`:
1. `POST /account/passkeys/begin` → get options
2. `navigator.credentials.create(options)` (WebAuthn)
3. Encode response, `POST /account/passkeys/finish`

### 5. `static-web/account.html` (new page)
Standard page structure matching existing pages (panel + item-list). Sections:
- Passkeys list: name, created date, last-used date, Rename button, Delete button
- "Add New Passkey" primary button at top

### 6. `static-web/js/account.js` (new JS module)
- `loadPasskeys()` — renders passkeys list
- `addPasskey()` — calls `api.addPasskey()`, shows toast, reloads list
- `deletePasskey(id)` — `showConfirm` → `api.deletePasskey(id)` → reload
- `renamePasskey(id)` — inline edit or prompt, calls `api.renamePasskey(id, name)` → reload

### 7. `static-web/js/bootstrap.js`
Add `{ href: "/account.html", text: "Account" }` to the logged-in nav links array.

---

## Store User ID in localStorage

### `internal/auth/auth.go`
Both `registerFinishHandler` and `loginFinishHandler` currently return `{"message": "...", "token": "..."}`. Add `user_id` to each response:
```go
json.NewEncoder(w).Encode(map[string]string{
    "message": "Login Success",
    "token":   t,
    "user_id": uuid.UUID(user.ID).String(),
})
```

### `static-web/js/auth.js`
- `set_login(response)` — also store `response.user_id` under `"user_id"` key
- `unset_login()` — also remove `"user_id"` from localStorage
- Export a `get_user_id()` helper that returns `localStorage.getItem("user_id")`

No other files need changing — `api.js` login/register methods already pass the full response object to `set_login`.

---

## Verification
1. `DEV_AUTH=true go run ./cmd/api-server/main.go`
2. Navigate to `/account.html` — passkeys list loads (one passkey for dev user if any)
3. Click "Add New Passkey" — browser WebAuthn prompt appears, credential saved, list refreshes
4. Delete a passkey — confirm dialog → removed; trying to delete the last one shows an error
5. Rename a passkey — name updates in list
6. In production (with real WebAuthn): register, then add a second passkey → both appear, login works with either
