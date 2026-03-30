# Calorie Goal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-user daily calorie goal, derivable to weekly/monthly, with a progress bar on the dashboard, food log, and stats pages.

**Architecture:** `calorie_goal INTEGER` column on the `users` table; two new `/account/profile` endpoints (GET/PUT) in the existing account handler; a shared `renderCalorieGoalBar()` helper in `js/ui.js` used by all three display pages.

**Tech Stack:** Go 1.22+, SQLite via goose migrations, vanilla JS ES modules, no build step.

---

## File Map

| Action | File | Purpose |
|--------|------|---------|
| Create | `internal/db/migrations/00010_add_calorie_goal_to_users.sql` | Schema change |
| Modify | `internal/db/model.go` | Add `CalorieGoal *int` to `User` struct |
| Modify | `internal/db/users.go` | Update SELECT/UPDATE queries for `calorie_goal` |
| Create | `internal/db/users_test.go` | Unit test for calorie goal persistence |
| Modify | `internal/api/account.go` | Add profile GET/PUT handlers and register routes |
| Create | `tests/08_account_profile.sh` | Integration test for profile endpoints |
| Modify | `test_runner.sh` | Add new test file to runner |
| Modify | `static-web/js/api.js` | Add `getProfile()` / `updateProfile()` |
| Modify | `static-web/css/main.css` | Goal bar + goal form CSS |
| Modify | `static-web/js/ui.js` | Add `renderCalorieGoalBar()` |
| Modify | `static-web/account.html` | Goals panel HTML |
| Modify | `static-web/js/account.js` | Load/save goal logic |
| Modify | `static-web/js/dashboard.js` | Fetch profile, render bar |
| Modify | `static-web/js/stat-ui.js` | Fetch profile, render bar (period-aware) |
| Modify | `static-web/foodlog.html` | Add calorie summary stat card |
| Modify | `static-web/js/foodlog.js` | Fetch profile, compute total, render bar |

---

### Task 1: DB Migration

**Files:**
- Create: `internal/db/migrations/00010_add_calorie_goal_to_users.sql`

- [ ] **Step 1: Create the migration file**

```sql
-- +goose Up
ALTER TABLE users ADD COLUMN calorie_goal INTEGER;

-- +goose Down
ALTER TABLE users DROP COLUMN calorie_goal;
```

- [ ] **Step 2: Verify the migration runs**

```bash
DB_PATH=/tmp/calorize_test.db go run ./cmd/api-server/main.go &
sleep 1 && kill %1
```

Expected: server starts and logs "database initialized" with no migration errors.

- [ ] **Step 3: Commit**

```bash
git add internal/db/migrations/00010_add_calorie_goal_to_users.sql
git commit -m "feat: add calorie_goal column to users table"
```

---

### Task 2: Go Model Update

**Files:**
- Modify: `internal/db/model.go` (lines 19–25, the `User` struct)

- [ ] **Step 1: Add `CalorieGoal` to the `User` struct**

In `internal/db/model.go`, update the `User` struct:

```go
type User struct {
	ID          UserID     `json:"id"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	DisabledAt  *time.Time `json:"disabled_at"`
	CalorieGoal *int       `json:"calorie_goal"`
	CreatedAt   time.Time  `json:"created_at"`
}
```

- [ ] **Step 2: Verify the project still compiles**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/db/model.go
git commit -m "feat: add CalorieGoal field to User model"
```

---

### Task 3: DB Query Updates + Unit Test

**Files:**
- Modify: `internal/db/users.go`
- Create: `internal/db/users_test.go`

- [ ] **Step 1: Write the failing unit test**

Create `internal/db/users_test.go`:

```go
package db

import (
	"testing"
)

func TestUpdateUserCalorieGoal(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)

	goal := 2000
	user.CalorieGoal = &goal

	updated, err := UpdateUser(*user)
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}
	if updated.CalorieGoal == nil || *updated.CalorieGoal != 2000 {
		t.Fatalf("expected CalorieGoal=2000, got %v", updated.CalorieGoal)
	}

	// Reload from DB and verify persistence
	fetched, err := GetUserByID(user.ID)
	if err != nil || fetched == nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if fetched.CalorieGoal == nil || *fetched.CalorieGoal != 2000 {
		t.Fatalf("expected persisted CalorieGoal=2000, got %v", fetched.CalorieGoal)
	}

	// Clear the goal
	user.CalorieGoal = nil
	if _, err := UpdateUser(*user); err != nil {
		t.Fatalf("UpdateUser (clear) failed: %v", err)
	}
	fetched2, _ := GetUserByID(user.ID)
	if fetched2.CalorieGoal != nil {
		t.Fatalf("expected CalorieGoal=nil after clear, got %v", fetched2.CalorieGoal)
	}
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/db/... -run TestUpdateUserCalorieGoal -v
```

Expected: FAIL — `UpdateUser` doesn't persist `calorie_goal` yet, and SELECT doesn't return it.

- [ ] **Step 3: Update the three `GetUser*` functions in `internal/db/users.go`**

Replace each `SELECT id, name, email, disabled_at, created_at FROM users` query and its matching `row.Scan(...)` call. There are three functions to update: `GetUser`, `GetUserByEmail`, `GetUserByID`.

For `GetUser` (and identical changes for the other two):

```go
func GetUser(userName string) (*User, error) {
	query := `SELECT id, name, email, disabled_at, calorie_goal, created_at FROM users WHERE name = ?`
	row := db.QueryRow(query, userName)

	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.DisabledAt, &user.CalorieGoal, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting user: %w", err)
	}
	return &user, nil
}
```

Apply the same SELECT and Scan change to `GetUserByEmail` and `GetUserByID` (same pattern, different WHERE clause).

- [ ] **Step 4: Update `UpdateUser` in `internal/db/users.go`**

```go
func UpdateUser(user User) (*User, error) {
	query := `UPDATE users SET name = ?, email = ?, disabled_at = ?, calorie_goal = ? WHERE id = ?`
	_, err := db.Exec(query, user.Name, user.Email, user.DisabledAt, user.CalorieGoal, user.ID)
	if err != nil {
		return nil, fmt.Errorf("updating user: %w", err)
	}
	return &user, nil
}
```

- [ ] **Step 5: Run the test again to confirm it passes**

```bash
go test ./internal/db/... -run TestUpdateUserCalorieGoal -v
```

Expected: PASS

- [ ] **Step 6: Run the full db test suite**

```bash
go test ./internal/db/... -v
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/db/users.go internal/db/users_test.go
git commit -m "feat: persist calorie_goal in user queries and UpdateUser"
```

---

### Task 4: Profile API Handlers

**Files:**
- Modify: `internal/api/account.go`
- Create: `tests/08_account_profile.sh`
- Modify: `test_runner.sh`

- [ ] **Step 1: Write the integration test**

Create `tests/08_account_profile.sh`:

```bash
#!/bin/bash
# 08_account_profile.sh: Account profile (calorie goal) endpoints

if [ -z "$BASE_URL" ]; then
    source "$(dirname "$0")/common.sh"
fi

echo "==================================================="
echo "Test: Account Profile / Calorie Goal"
echo "---------------------------------------------------"

# Test 1: GET /account/profile returns 200 with calorie_goal field
echo "Fetching profile..."
PROF_RESP=$(curl -s -w "\n%{http_code}" "$BASE_URL/account/profile")
PROF_BODY=$(echo "$PROF_RESP" | head -n -1)
PROF_CODE=$(echo "$PROF_RESP" | tail -n 1)

if [ "$PROF_CODE" != "200" ]; then
    log_err "Expected 200, got $PROF_CODE"
    exit 1
fi
if ! echo "$PROF_BODY" | jq -e 'has("calorie_goal")' > /dev/null 2>&1; then
    log_err "Response missing calorie_goal field: $PROF_BODY"
    exit 1
fi
log_info "✅ GET /account/profile → 200, has calorie_goal"

# Test 2: PUT /account/profile sets goal to 2000
echo "Setting calorie goal to 2000..."
PUT_RESP=$(curl -s -w "\n%{http_code}" -X PUT "$BASE_URL/account/profile" \
  -H "Content-Type: application/json" \
  -d '{"calorie_goal": 2000}')
PUT_BODY=$(echo "$PUT_RESP" | head -n -1)
PUT_CODE=$(echo "$PUT_RESP" | tail -n 1)

if [ "$PUT_CODE" != "200" ]; then
    log_err "Expected 200, got $PUT_CODE"
    exit 1
fi
RETURNED_GOAL=$(echo "$PUT_BODY" | jq '.calorie_goal')
if [ "$RETURNED_GOAL" != "2000" ]; then
    log_err "Expected calorie_goal=2000 in response, got $RETURNED_GOAL"
    exit 1
fi
log_info "✅ PUT /account/profile {calorie_goal: 2000} → 200"

# Test 3: GET confirms persisted value
echo "Verifying persisted goal..."
PROF2_BODY=$(curl -s "$BASE_URL/account/profile")
GOAL_VAL=$(echo "$PROF2_BODY" | jq '.calorie_goal')
if [ "$GOAL_VAL" != "2000" ]; then
    log_err "Expected persisted calorie_goal=2000, got $GOAL_VAL"
    exit 1
fi
log_info "✅ GET /account/profile after PUT → calorie_goal=2000"

# Test 4: PUT clears goal with null
echo "Clearing calorie goal..."
CLEAR_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PUT "$BASE_URL/account/profile" \
  -H "Content-Type: application/json" \
  -d '{"calorie_goal": null}')
if [ "$CLEAR_CODE" != "200" ]; then
    log_err "Expected 200 on clear, got $CLEAR_CODE"
    exit 1
fi
PROF3_BODY=$(curl -s "$BASE_URL/account/profile")
GOAL_NULL=$(echo "$PROF3_BODY" | jq '.calorie_goal')
if [ "$GOAL_NULL" != "null" ]; then
    log_err "Expected calorie_goal=null after clear, got $GOAL_NULL"
    exit 1
fi
log_info "✅ PUT /account/profile {calorie_goal: null} clears goal"
```

Make it executable:
```bash
chmod +x tests/08_account_profile.sh
```

- [ ] **Step 2: Add the test to `test_runner.sh`**

In `test_runner.sh`, add `"tests/08_account_profile.sh"` to the `TEST_FILES` array, after `07_account_passkeys.sh`:

```bash
TEST_FILES=(
    "tests/01_basics.sh"
    "tests/02_food_mgmt.sh"
    "tests/03_logging.sh"
    "tests/04_stats_reads.sh"
    "tests/05_input_validation.sh"
    "tests/07_account_passkeys.sh"
    "tests/08_account_profile.sh"
)
```

- [ ] **Step 3: Add the profile handlers to `internal/api/account.go`**

At the top of the file, add a `profileView` type after `passkeyView`:

```go
type profileView struct {
	CalorieGoal *int `json:"calorie_goal"`
}
```

Add the two handler functions before `RegisterAccountPaths`:

```go
func getProfileHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profileView{CalorieGoal: user.CalorieGoal})
}

func updateProfileHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		CalorieGoal *int `json:"calorie_goal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}
	user.CalorieGoal = body.CalorieGoal
	if _, err := db.UpdateUser(*user); err != nil {
		slog.Error("failed to update profile", "error", err)
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profileView{CalorieGoal: user.CalorieGoal})
}
```

- [ ] **Step 4: Register the new routes in `RegisterAccountPaths`**

```go
func RegisterAccountPaths(mux *http.ServeMux) {
	mux.HandleFunc("GET /account/profile", getProfileHandler)
	mux.HandleFunc("PUT /account/profile", updateProfileHandler)
	mux.HandleFunc("GET /account/passkeys", listPasskeysHandler)
	mux.HandleFunc("DELETE /account/passkeys/{id}", deletePasskeyHandler)
	mux.HandleFunc("PATCH /account/passkeys/{id}", renamePasskeyHandler)
	mux.HandleFunc("POST /account/passkeys/begin", addPasskeyBeginHandler)
	mux.HandleFunc("POST /account/passkeys/finish", addPasskeyFinishHandler)
}
```

- [ ] **Step 5: Compile**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Run the integration test**

```bash
DEV_AUTH=true go run ./cmd/api-server/main.go &
sleep 2
./test_runner.sh
kill %1
```

Expected: all tests pass including the new `08_account_profile.sh`.

- [ ] **Step 7: Commit**

```bash
git add internal/api/account.go tests/08_account_profile.sh test_runner.sh
git commit -m "feat: add GET/PUT /account/profile endpoints for calorie goal"
```

---

### Task 5: JS API Client Methods

**Files:**
- Modify: `static-web/js/api.js`

- [ ] **Step 1: Add `getProfile` and `updateProfile` to the `API` class**

In `static-web/js/api.js`, add these two methods in the `// --- Account / Passkeys ---` section, after `renamePasskey`:

```js
// --- Profile ---

async getProfile() {
    return await this.request('/account/profile');
}

async updateProfile(data) {
    return await this.request('/account/profile', 'PUT', data);
}
```

- [ ] **Step 2: Verify manually**

Open the browser console on any authenticated page and run:
```js
import { api } from '/js/api.js';
api.getProfile().then(console.log);
```

Expected: `{ calorie_goal: null }` (or current value if already set).

- [ ] **Step 3: Commit**

```bash
git add static-web/js/api.js
git commit -m "feat: add getProfile/updateProfile API client methods"
```

---

### Task 6: Goal Bar CSS

**Files:**
- Modify: `static-web/css/main.css`

- [ ] **Step 1: Add goal bar and goal form styles**

In `static-web/css/main.css`, add the following block after the `.stat-card span` rule (around line 308):

```css
/* ============================================================
   Goal Bar
   ============================================================ */
.goal-bar-wrap {
  margin-top: var(--space-2);
}

.goal-bar {
  height: 6px;
  background: var(--color-border);
  border-radius: var(--radius-sm);
  overflow: hidden;
  margin-bottom: var(--space-1);
}

.goal-bar-fill {
  height: 100%;
  background: var(--color-primary);
  transition: width 0.3s ease;
}

.goal-bar-fill.over {
  background: var(--color-danger);
}

.goal-bar-label {
  font-size: 0.75rem;
  color: var(--color-text-muted);
}

.goal-bar-label.over {
  color: var(--color-danger);
}

/* ============================================================
   Goal Form (account page)
   ============================================================ */
.goal-input-row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex-wrap: wrap;
}

.goal-input-row input[type="number"] {
  width: 120px;
}

.goal-derived-label {
  margin: var(--space-2) 0 0;
  font-size: 0.875rem;
  color: var(--color-text-muted);
}
```

- [ ] **Step 2: Commit**

```bash
git add static-web/css/main.css
git commit -m "feat: add goal bar and goal form CSS"
```

---

### Task 7: `renderCalorieGoalBar` Helper

**Files:**
- Modify: `static-web/js/ui.js`

- [ ] **Step 1: Write a failing manual test (no JS unit test framework — verify in browser after Task 8)**

The test criteria to verify once the account page is wired up:
- With `goal=null`: function returns `null`
- With `consumed=1200, goal=2000`: bar is 60% wide, label says "800 remaining"
- With `consumed=2200, goal=2000`: bar is 100% wide, fill and label have `.over` class, label says "200 over goal"

- [ ] **Step 2: Add `renderCalorieGoalBar` to `static-web/js/ui.js`**

Append at the end of the file:

```js
// ============================================================
// Calorie Goal Bar
// ============================================================

/**
 * Renders a calorie progress bar toward a goal.
 * @param {number} consumed - Calories consumed.
 * @param {number|null} goal - Daily calorie goal. Returns null if not set.
 * @returns {HTMLElement|null}
 */
export function renderCalorieGoalBar(consumed, goal) {
    if (goal == null) return null;

    const pct = Math.min(100, Math.round((consumed / goal) * 100));
    const over = consumed > goal;

    const wrap = document.createElement('div');
    wrap.className = 'goal-bar-wrap';

    const bar = document.createElement('div');
    bar.className = 'goal-bar';

    const fill = document.createElement('div');
    fill.className = 'goal-bar-fill' + (over ? ' over' : '');
    fill.style.width = `${pct}%`;
    bar.appendChild(fill);

    const label = document.createElement('div');
    label.className = 'goal-bar-label' + (over ? ' over' : '');
    label.textContent = over
        ? `${Math.round(consumed - goal)} over goal`
        : `${Math.round(goal - consumed)} remaining`;

    wrap.appendChild(bar);
    wrap.appendChild(label);
    return wrap;
}
```

- [ ] **Step 3: Commit**

```bash
git add static-web/js/ui.js
git commit -m "feat: add renderCalorieGoalBar helper to ui.js"
```

---

### Task 8: Account Page — Goals Panel

**Files:**
- Modify: `static-web/account.html`
- Modify: `static-web/js/account.js`

- [ ] **Step 1: Add the Goals panel to `account.html`**

After the closing `</div>` of the Passkeys panel (after line 25), add:

```html
        <div class="panel">
            <h2>Goals</h2>
            <div class="form-row">
                <div class="form-group">
                    <label for="calorie-goal-input">Daily Calorie Goal</label>
                    <div class="goal-input-row">
                        <input type="number" id="calorie-goal-input" min="0" step="1" placeholder="e.g. 2000">
                        <span>kcal</span>
                        <button id="btn-save-goal" class="btn btn-primary">Save Goal</button>
                    </div>
                    <p id="goal-derived" class="goal-derived-label"></p>
                </div>
            </div>
        </div>
```

- [ ] **Step 2: Add goal functions to `static-web/js/account.js`**

Add the import for `renderCalorieGoalBar` is not needed on this page — but `showToast` is already imported. Add these three functions after the existing `addPasskey` function:

```js
function updateDerived(daily) {
    const el = document.getElementById('goal-derived');
    if (!daily || daily <= 0) {
        el.textContent = '';
        return;
    }
    const weekly = (daily * 7).toLocaleString();
    const monthly = Math.round(daily * 30.4).toLocaleString();
    el.textContent = `Weekly: ${weekly} kcal · Monthly: ~${monthly} kcal`;
}

async function loadProfile() {
    try {
        const profile = await api.getProfile();
        if (profile && profile.calorie_goal != null) {
            document.getElementById('calorie-goal-input').value = profile.calorie_goal;
            updateDerived(profile.calorie_goal);
        }
    } catch (e) {
        showToast('Failed to load profile', 'error');
    }
}

async function saveGoal() {
    const input = document.getElementById('calorie-goal-input');
    const raw = input.value.trim();
    const goal = raw === '' ? null : parseInt(raw, 10);
    if (raw !== '' && (isNaN(goal) || goal <= 0)) {
        showToast('Please enter a valid calorie goal', 'error');
        return;
    }
    try {
        await api.updateProfile({ calorie_goal: goal });
        showToast('Goal saved');
        updateDerived(goal);
    } catch (e) {
        showToast(e.message || 'Failed to save goal', 'error');
    }
}
```

- [ ] **Step 3: Wire up the new functions in the `DOMContentLoaded` listener**

Update the existing listener at the bottom of `account.js`:

```js
document.addEventListener('DOMContentLoaded', () => {
    loadPasskeys();
    loadProfile();
    document.getElementById('btn-add-passkey').addEventListener('click', addPasskey);
    document.getElementById('calorie-goal-input').addEventListener('input', (e) => {
        const val = parseInt(e.target.value, 10);
        updateDerived(isNaN(val) ? null : val);
    });
    document.getElementById('btn-save-goal').addEventListener('click', saveGoal);
});
```

- [ ] **Step 4: Verify manually**

Start the dev server (`DEV_AUTH=true go run ./cmd/api-server/main.go`) and open `http://localhost:8080/account.html`.
- Goals panel is visible below Passkeys.
- Enter 2000, confirm derived shows "Weekly: 14,000 kcal · Monthly: ~60,800 kcal".
- Click Save Goal, toast appears.
- Refresh — input should be pre-filled with 2000.
- Clear the input, save — goal is cleared (refresh shows empty input).

- [ ] **Step 5: Commit**

```bash
git add static-web/account.html static-web/js/account.js
git commit -m "feat: add calorie goal panel to account page"
```

---

### Task 9: Dashboard Goal Bar

**Files:**
- Modify: `static-web/js/dashboard.js`

- [ ] **Step 1: Update `dashboard.js` to fetch the profile and render the bar**

Replace the entire `dashboard.js` with:

```js
import { api } from './api.js';
import { getLocalDateString } from './utils.js';
import { renderCalorieGoalBar } from './ui.js';

async function updateDashboard() {
    try {
        const today = getLocalDateString();
        const [stats, profile, logs] = await Promise.all([
            api.getStats('day', today),
            api.getProfile(),
            api.getLogs(today),
        ]);

        const calories = Math.round(stats?.calories || 0);
        document.getElementById('dashboard-calories').textContent = calories;
        document.getElementById('dashboard-protein').textContent = Math.round(stats?.protein || 0);
        document.getElementById('dashboard-carbs').textContent = Math.round(stats?.carbs || 0);
        document.getElementById('dashboard-fat').textContent = Math.round(stats?.fat || 0);

        const calCard = document.getElementById('dashboard-calories').parentElement;
        calCard.querySelector('.goal-bar-wrap')?.remove();
        const bar = renderCalorieGoalBar(calories, profile?.calorie_goal ?? null);
        if (bar) calCard.appendChild(bar);

        const logsList = document.getElementById('dashboard-logs-list');
        logsList.innerHTML = '';

        if (logs && logs.length > 0) {
            logs.forEach(log => {
                const li = document.createElement('li');
                let text = '';
                if (log.food) {
                    text = `${log.food.name} (${log.amount}x)`;
                } else {
                    text = 'Quick Add';
                }
                if (log.calories) {
                    text += ` - ${Math.round(log.calories)} kcal`;
                }
                if (log.meal_tag) {
                    text += ` [${log.meal_tag}]`;
                }
                li.textContent = text;
                logsList.appendChild(li);
            });
        } else {
            logsList.innerHTML = '<li>No logs for today</li>';
        }

    } catch (error) {
        console.error("Failed to load dashboard data:", error);
    }
}

window.addEventListener('load', updateDashboard);
```

- [ ] **Step 2: Verify manually**

Open `http://localhost:8080/dashboard.html`.
- With a goal set (e.g. 2000 kcal), the Calories card shows a progress bar below the number.
- With no goal set, no bar appears.

- [ ] **Step 3: Commit**

```bash
git add static-web/js/dashboard.js
git commit -m "feat: show calorie goal bar on dashboard"
```

---

### Task 10: Stats Page Goal Bar

**Files:**
- Modify: `static-web/js/stat-ui.js`

- [ ] **Step 1: Update `stat-ui.js` to fetch profile once and render a period-aware bar**

Replace the entire `stat-ui.js` with:

```js
import { api } from './api.js';
import { getLocalDateString } from './utils.js';
import { drawMacroBar, drawMealBars, drawDayBars, drawWeekBars } from './charts.js';
import { renderCalorieGoalBar } from './ui.js';

let lastPeriod = 'day';
let lastDate = null;
let lastStats = null;
let lastExtra = null;
let profile = null;

async function init() {
    const buttons = document.querySelectorAll('.period-btn');
    buttons.forEach(btn => {
        btn.addEventListener('click', () => {
            buttons.forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            loadStats(btn.dataset.period);
        });
    });

    let resizeTimer;
    window.addEventListener('resize', () => {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(redraw, 100);
    });

    profile = await api.getProfile().catch(() => null);
    loadStats('day');
}

async function loadStats(period, date) {
    if (!date) date = getLocalDateString();
    lastPeriod = period;
    lastDate = date;

    updateDisplay({ calories: 0, protein: 0, carbs: 0, fat: 0 });

    try {
        if (period === 'day') {
            const [stats, logs] = await Promise.all([
                api.getStats(period, date),
                api.getLogs(date),
            ]);
            lastStats = stats || { calories: 0, protein: 0, carbs: 0, fat: 0 };
            lastExtra = logs || [];
        } else {
            const [stats, breakdown] = await Promise.all([
                api.getStats(period, date),
                api.getStatsBreakdown(period, date),
            ]);
            lastStats = stats || { calories: 0, protein: 0, carbs: 0, fat: 0 };
            lastExtra = breakdown || [];
        }
    } catch (e) {
        console.error('Failed to load stats', e);
        lastStats = { calories: 0, protein: 0, carbs: 0, fat: 0 };
        lastExtra = [];
    }

    updateDisplay(lastStats);
    showPanels(period);
    redraw();
}

function redraw() {
    if (!lastStats) return;
    const { protein = 0, carbs = 0, fat = 0 } = lastStats;

    drawMacroBar(document.getElementById('chart-macro'), { protein, carbs, fat });

    if (lastPeriod === 'day') {
        const meals = { breakfast: 0, lunch: 0, dinner: 0, snack: 0 };
        (lastExtra || []).forEach(e => {
            const tag = e.meal_tag || 'snack';
            meals[tag] = (meals[tag] || 0) + (e.calories ?? 0);
        });
        drawMealBars(document.getElementById('chart-meals'), meals);
    } else if (lastPeriod === 'week') {
        drawDayBars(document.getElementById('chart-days'), lastExtra || []);
    } else if (lastPeriod === 'month') {
        drawWeekBars(document.getElementById('chart-weeks'), lastExtra || []);
    }
}

function showPanels(period) {
    document.getElementById('panel-macro').style.display = '';
    document.getElementById('panel-meals').style.display = period === 'day' ? '' : 'none';
    document.getElementById('panel-days').style.display = period === 'week' ? '' : 'none';
    document.getElementById('panel-weeks').style.display = period === 'month' ? '' : 'none';
}

function updateDisplay(stats) {
    document.getElementById('stat-calories').textContent = Math.round(stats.calories || 0);
    document.getElementById('stat-protein').textContent = Math.round(stats.protein || 0);
    document.getElementById('stat-carbs').textContent = Math.round(stats.carbs || 0);
    document.getElementById('stat-fat').textContent = Math.round(stats.fat || 0);

    const dailyGoal = profile?.calorie_goal ?? null;
    let scaledGoal = null;
    if (dailyGoal != null) {
        if (lastPeriod === 'week') scaledGoal = dailyGoal * 7;
        else if (lastPeriod === 'month') scaledGoal = Math.round(dailyGoal * 30.4);
        else scaledGoal = dailyGoal;
    }

    const calCard = document.getElementById('stat-calories').parentElement;
    calCard.querySelector('.goal-bar-wrap')?.remove();
    const bar = renderCalorieGoalBar(Math.round(stats.calories || 0), scaledGoal);
    if (bar) calCard.appendChild(bar);
}

window.addEventListener('load', init);
```

- [ ] **Step 2: Verify manually**

Open `http://localhost:8080/stat-ui.html`.
- Day view: bar shows daily goal progress.
- Week view: bar goal is daily × 7.
- Month view: bar goal is daily × 30.4 (rounded).
- Switching periods updates the bar.

- [ ] **Step 3: Commit**

```bash
git add static-web/js/stat-ui.js
git commit -m "feat: show period-aware calorie goal bar on stats page"
```

---

### Task 11: Foodlog Page Goal Bar

**Files:**
- Modify: `static-web/foodlog.html`
- Modify: `static-web/js/foodlog.js`

- [ ] **Step 1: Add a calorie summary stat card to `foodlog.html`**

Between the closing `</div>` of the `.panel` (Add Log form) and the `<div class="logs-list-section">`, add:

```html
        <div class="stats-container">
            <div class="stat-card" id="foodlog-cal-card">
                <h3>Calories</h3>
                <span id="foodlog-calories">0</span>
            </div>
        </div>
```

- [ ] **Step 2: Add profile fetch and calorie total rendering to `foodlog.js`**

At the top of `foodlog.js`, add the import:

```js
import { renderCalorieGoalBar } from './ui.js';
```

Add a module-level variable alongside the existing ones:

```js
let goalProfile = null;
```

In the `init` function, fetch the profile once before `loadLogs()`:

```js
async function init() {
    // ... existing date picker and food search setup ...

    goalProfile = await api.getProfile().catch(() => null);

    loadLogs();

    // ... existing form submit and mode toggle setup ...
}
```

In `loadLogs`, after populating the list, compute the total and update the calorie card. Add the following at the end of the `try` block in `loadLogs`, after the list rendering:

```js
        const totalCals = Math.round(
            (logs || []).reduce((sum, log) => sum + (log.calories || 0), 0)
        );
        document.getElementById('foodlog-calories').textContent = totalCals;

        const calCard = document.getElementById('foodlog-cal-card');
        calCard.querySelector('.goal-bar-wrap')?.remove();
        const bar = renderCalorieGoalBar(totalCals, goalProfile?.calorie_goal ?? null);
        if (bar) calCard.appendChild(bar);
```

- [ ] **Step 3: Verify manually**

Open `http://localhost:8080/foodlog.html`.
- Calories card appears showing the day's total.
- With a goal set, the bar appears below the number.
- Adding or removing a log entry updates both the total and the bar.

- [ ] **Step 4: Commit**

```bash
git add static-web/foodlog.html static-web/js/foodlog.js
git commit -m "feat: show calorie goal bar on food log page"
```

---

### Task 12: Final Verification

- [ ] **Step 1: Run full Go tests**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 2: Run integration tests**

```bash
DEV_AUTH=true go run ./cmd/api-server/main.go &
sleep 2
./test_runner.sh
kill %1
```

Expected: all tests pass including `08_account_profile.sh`.

- [ ] **Step 3: Manual end-to-end walkthrough**

1. Open `http://localhost:8080/account.html` — set goal to 2000.
2. Open `http://localhost:8080/foodlog.html` — log a food, confirm calorie total and bar update.
3. Open `http://localhost:8080/dashboard.html` — bar appears on Calories card.
4. Open `http://localhost:8080/stat-ui.html` — bar appears; switch to Week view, bar goal is 14,000.
5. Return to account, clear the goal — confirm bars disappear on all pages after reload.
