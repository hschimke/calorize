# Gemini Context: Calorize

This project is a lightweight calorie counting system featuring a Go backend API and a vanilla JavaScript frontend, using SQLite for data persistence.

## Project Overview

- **Backend:** Go 1.25+ using the standard library's `net/http` (with Go 1.22+ method routing).
- **Frontend:** Vanilla HTML/CSS/JS with no build step.
- **Database:** SQLite via `glebarez/go-sqlite` (pure Go driver) with `goose` for migrations.
- **Authentication:** WebAuthn (FIDO2) for login/registration, PASETO tokens for API sessions.
- **Key Architectures:** 
    - **Immutable Foods:** Updates create new versions linked by `family_id`; only the latest is `is_current`.
    - **Denormalized Logs:** Calories and macros are stored directly in log entries to survive food deletions or changes.
    - **Soft Deletes:** Records use `deleted_at` timestamps rather than hard deletions.

## Building and Running

### API Server
The server supports a development mode that bypasses WebAuthn and uses a hardcoded dev user.

- **Development Mode:**
  ```bash
  DEV_AUTH=true go run ./cmd/api-server/main.go
  ```
- **Production Mode:**
  ```bash
  WEBAUTHN_RP_ID=yourdomain.com WEBAUTHN_RP_ORIGINS=https://yourdomain.com go run ./cmd/api-server/main.go
  ```
- **Environment Variables:**
    - `PORT`: Server port (default: 8080)
    - `DB_PATH`: SQLite database path (default: `./test.db`)
    - `DEV_AUTH`: Set to `true` to bypass WebAuthn.

### Frontend
The frontend is served from the `static-web/` directory. It can be served by the Go API or a reverse proxy like Caddy (see `Caddyfile`).

## Testing

- **Integration Tests:** Requires the API server to be running with `DEV_AUTH=true`.
  ```bash
  ./test_runner.sh
  ```
- **Go Unit Tests:**
  ```bash
  go test ./internal/db/...
  ```

## Development Conventions

### Backend
- **Routing:** Use Go 1.22+ `ServeMux` patterns (e.g., `GET /foods/{id}`).
- **Models:** Defined in `internal/db/model.go`. Use UUIDs for all primary keys.
- **Migrations:** SQL files in `internal/db/migrations/`. Follow the `NNNNN_description.sql` naming pattern.
- **Tokens:** Use PASETO (v2) for API sessions, implemented in `internal/auth/token/`.

### Frontend
- **Styling:** All styles live in `static-web/css/main.css`. Use CSS Custom Properties defined in the `:root`. Avoid inline styles or JS-injected CSS.
- **API Client:** Use the class in `static-web/js/api.js` for all backend communication.
- **UI Components:**
    - Use `showToast(message, type)` for notifications.
    - Use `await showConfirm(message)` for confirmation dialogs.

## Directory Structure
- `cmd/api-server/`: Application entry point.
- `internal/api/`: API handlers and routing.
- `internal/db/`: Database operations, models, and migrations.
- `internal/auth/`: WebAuthn and PASETO token logic.
- `static-web/`: Frontend assets (HTML, CSS, JS).
- `tests/`: Integration test scripts.
- `TODO.md`: Future improvements and architectural debt.
