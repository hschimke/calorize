# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Calorize is a calorie counting system with a Go backend API and vanilla JavaScript frontend, using SQLite for storage.

## Common Commands

### Running the API server
```bash
# Development mode (skips WebAuthn, uses hardcoded dev user)
DEV_AUTH=true go run ./cmd/api-server/main.go

# Production mode (requires WebAuthn configuration)
WEBAUTHN_RP_ID=calorize.example.com WEBAUTHN_RP_ORIGINS=https://calorize.example.com go run ./cmd/api-server/main.go
```

### Running tests
```bash
# Integration tests (requires API server running with DEV_AUTH=true)
./test_runner.sh

# Go unit tests
go test ./internal/db/...

# Single test file (integration)
source tests/common.sh && source tests/02_food_mgmt.sh
```

### Environment variables
- `PORT` — Server port (default: 8080)
- `DB_PATH` — SQLite database path (default: ./test.db)
- `DEV_AUTH` — Set to "true" to bypass WebAuthn for development
- `WEBAUTHN_RP_ID`, `WEBAUTHN_RP_ORIGINS`, `WEBAUTHN_RP_DISPLAY_NAME` — WebAuthn configuration

## Architecture

### Backend (`cmd/` and `internal/`)
- **No web framework** — uses Go's standard `net/http` with `http.ServeMux` (Go 1.22+ method routing: `GET /foods`, `POST /foods/{id}`)
- **Auth**: WebAuthn (FIDO2) for login/registration, PASETO tokens for API sessions (not JWT)
- **Database**: SQLite via `glebarez/go-sqlite` (pure Go driver), migrations via `pressly/goose`

### Package layout
- `cmd/api-server/` — Entry point, server setup
- `internal/api/` — HTTP handlers and route registration
- `internal/db/` — Models, queries, and migrations
- `internal/auth/` — WebAuthn setup; `internal/auth/token/` — PASETO token creation/validation
- `internal/middleware/` — Auth, CORS, logging, recovery middleware
- `TODO.md` — Future improvements and technical debt tracking

### Frontend (`static-web/`)
- Vanilla HTML/CSS/JS, no build step
- `js/api.js` contains the API client class used by all pages

### Middleware chain
Global: Logger → Recoverer → CORS → Router. Protected routes are wrapped with `RequireAuth` which checks Bearer token (PASETO) or session cookie.

### Key data patterns
- **Food versioning**: Foods are immutable. Updates create a new version linked by `family_id`; only the latest has `is_current=true`.
- **Recipes**: A food with `type='recipe'` that references other foods via `recipe_items` table.
- **Soft deletes**: Foods and log entries use `deleted_at` timestamps, never hard-deleted.
- **Denormalized logs**: `food_log_entries` stores calories/protein/carbs/fat directly so data survives food deletion.
- **UUIDs**: All primary keys are text UUIDs generated via `google/uuid`.

### Database migrations
SQL files in `internal/db/migrations/`, auto-applied on startup via goose. When adding a migration, follow the existing `NNNNN_description.sql` naming pattern.

## Frontend Design System

All frontend styles live in **`static-web/css/main.css`**. No inline `<style>` blocks in HTML files. No CSS injected from JavaScript.

### Font
`Inter, -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif`

### CSS Custom Properties
| Token | Value | Use |
|---|---|---|
| `--color-primary` | `#2563eb` | CTA buttons |
| `--color-primary-hover` | `#1d4ed8` | Button hover |
| `--color-danger` | `#dc2626` | Destructive actions |
| `--color-danger-bg` | `#fee2e2` | Danger button background |
| `--color-text` | `#111827` | Body text |
| `--color-text-muted` | `#6b7280` | Labels, secondary info |
| `--color-border` | `#e5e7eb` | Dividers, input borders |
| `--color-bg` | `#f9fafb` | Page background |
| `--color-surface` | `#ffffff` | Cards, panels |
| `--color-surface-muted` | `#f3f4f6` | Controls bg, segmented control |
| `--color-nav` | `#111827` | Navigation bar |
| `--space-1` – `--space-8` | `4px` – `32px` | Spacing scale |
| `--radius-sm` | `4px` | Inputs, small buttons |
| `--radius-md` | `8px` | Cards, panels |
| `--radius-pill` | `20px` | Segmented controls, period buttons |

### Button Classes
- `.btn` — base styles (always combine with a variant)
- `.btn-primary` — blue, white text (CTA / submit)
- `.btn-secondary` — muted gray (secondary actions, no destructive)
- `.btn-danger` — red tint (delete/destructive)
- `.btn-sm` — smaller padding modifier for inline list buttons

### Component Classes
| Class | Purpose |
|---|---|
| `.container` | Max-width 800px centered layout |
| `.panel` | White card with border, replaces ad-hoc section backgrounds |
| `.controls-bar` | Date picker / filter row |
| `.toggle-container` | Segmented radio control |
| `.stat-card` / `.stats-container` | Metric display cards |
| `.period-selector` / `.period-btn` | Time period filter buttons |
| `.item-list` | Shared log/food `<ul>` list with dividers |
| `.food-info` / `.food-actions` | Flex row layout inside list items |
| `.nutrient-row` | Grid row for nutrient/ingredient entries |
| `.ingredient-add-row` | Flex row for the ingredient search + add controls |

### Toast Notifications
Import and call `showToast` from `js/ui.js`:
```js
import { showToast, showConfirm } from './ui.js';

showToast("Saved successfully");           // green
showToast("Something went wrong", 'error'); // red
```

### Confirm Dialogs
Replace `confirm()` with the async `showConfirm`:
```js
const ok = await showConfirm("Delete this item?");
if (!ok) return;
```
