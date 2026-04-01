# Copy Day Logs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Copy from..." button to the food log screen that copies log entries from a chosen source date (with meal-tag filtering and a live preview) into the currently selected date.

**Architecture:** New `POST /logs/copy` backend endpoint performs the atomic copy; the frontend fetches the source day's entries via the existing `GET /logs` endpoint for the preview, then fires a single copy request on confirm. `logged_at` is set to the current time for today's target, or noon on the target date for historical targets.

**Tech Stack:** Go 1.22 `net/http`, SQLite via `glebarez/go-sqlite`, vanilla JS ES modules, native `<dialog>` element.

**Spec:** `docs/superpowers/specs/2026-04-01-copy-day-logs-design.md`

---

## File Map

| File | Change |
|---|---|
| `internal/db/food-log-entries.go` | Add `CopyFoodLogEntries` function |
| `internal/db/food_log_copy_test.go` | New — unit tests for `CopyFoodLogEntries` |
| `internal/api/api.go` | Add `copyLogEntriesRequest`, `copyLogEntriesHandler`, register route |
| `tests/09_copy_logs.sh` | New — integration tests for `POST /logs/copy` |
| `test_runner.sh` | Add `tests/09_copy_logs.sh` to the test list |
| `static-web/js/api.js` | Add `copyLogs` method |
| `static-web/css/main.css` | Add `.section-header` utility class and `<dialog>` styles |
| `static-web/foodlog.html` | Add "Copy from..." button, `<dialog>` modal markup |
| `static-web/js/foodlog.js` | Add `initCopyDialog` and `loadCopyPreview` functions, wire into `init` |

---

## Task 1: DB layer — `CopyFoodLogEntries`

**Files:**
- Create: `internal/db/food_log_copy_test.go`
- Modify: `internal/db/food-log-entries.go`

- [ ] **Step 1: Write the failing unit test**

Create `internal/db/food_log_copy_test.go`:

```go
package db

import (
	"testing"
	"time"
)

func TestCopyFoodLogEntries(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)
	food := createTestIngredient(t, user, "Copy Test Food")
	food.Calories = 200
	food.MeasurementAmount = 100
	updated, err := UpdateFood(food.ID, *food)
	if err != nil {
		t.Fatalf("UpdateFood failed: %v", err)
	}

	yesterday := time.Now().UTC().AddDate(0, 0, -1)

	// Create breakfast, lunch, and dinner entries on yesterday
	for _, tc := range []struct {
		amount  float64
		mealTag string
	}{
		{100, "breakfast"},
		{150, "lunch"},
		{200, "dinner"},
	} {
		_, err := CreateFoodLogEntry(FoodLogEntry{
			UserID:   user.ID,
			FoodID:   &updated.ID,
			Amount:   tc.amount,
			MealTag:  tc.mealTag,
			LoggedAt: yesterday,
		})
		if err != nil {
			t.Fatalf("CreateFoodLogEntry (%s) failed: %v", tc.mealTag, err)
		}
	}

	// Copy only breakfast and lunch to "now"
	copyTime := time.Now().UTC()
	count, err := CopyFoodLogEntries(user.ID, yesterday, []string{"breakfast", "lunch"}, copyTime)
	if err != nil {
		t.Fatalf("CopyFoodLogEntries failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 entries copied, got %d", count)
	}

	// The copied entries are stamped with copyTime, so query using copyTime's date
	copied, err := GetFoodLogEntries(user.ID, copyTime)
	if err != nil {
		t.Fatalf("GetFoodLogEntries (today) failed: %v", err)
	}

	// Filter to this test's user (other tests may run concurrently on same db)
	var mine []FoodLogEntry
	for _, e := range copied {
		if e.UserID == user.ID {
			mine = append(mine, e)
		}
	}

	if len(mine) != 2 {
		t.Fatalf("Expected 2 copied entries, got %d", len(mine))
	}

	tags := map[string]bool{}
	for _, e := range mine {
		tags[e.MealTag] = true
	}
	if !tags["breakfast"] {
		t.Error("Expected a breakfast entry")
	}
	if !tags["lunch"] {
		t.Error("Expected a lunch entry")
	}
	if tags["dinner"] {
		t.Error("Dinner should not have been copied")
	}

	// logged_at must be within 2 seconds of copyTime
	for _, e := range mine {
		diff := e.LoggedAt.Sub(copyTime)
		if diff < 0 {
			diff = -diff
		}
		if diff > 2*time.Second {
			t.Errorf("logged_at %v not near copyTime %v", e.LoggedAt, copyTime)
		}
	}
}

func TestCopyFoodLogEntries_Empty(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)
	// No entries exist for this user; copying should return 0 without error
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	count, err := CopyFoodLogEntries(user.ID, yesterday, []string{"breakfast"}, time.Now().UTC())
	if err != nil {
		t.Fatalf("CopyFoodLogEntries on empty day failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 entries copied, got %d", count)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/db/... -run TestCopyFoodLogEntries -v
```

Expected: FAIL — `undefined: CopyFoodLogEntries`

- [ ] **Step 3: Implement `CopyFoodLogEntries` in `internal/db/food-log-entries.go`**

Add this function at the end of `internal/db/food-log-entries.go`:

```go
// CopyFoodLogEntries copies log entries from fromDate (filtered by mealTags) to new entries
// stamped with loggedAt. Returns the number of entries created.
func CopyFoodLogEntries(userID UserID, fromDate time.Time, mealTags []string, loggedAt time.Time) (int, error) {
	entries, err := GetFoodLogEntries(userID, fromDate)
	if err != nil {
		return 0, fmt.Errorf("fetching source entries: %w", err)
	}

	tagSet := make(map[string]bool, len(mealTags))
	for _, t := range mealTags {
		tagSet[t] = true
	}

	count := 0
	for _, entry := range entries {
		if !tagSet[entry.MealTag] {
			continue
		}
		newID, err := uuid.NewV7()
		if err != nil {
			return count, err
		}
		_, err = db.Exec(
			`INSERT INTO food_log_entries
			 (id, user_id, food_id, portion_name, calories, protein, carbs, fat, amount, meal_tag, note, logged_at, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newID, userID, entry.FoodID, entry.PortionName,
			entry.Calories, entry.Protein, entry.Carbs, entry.Fat,
			entry.Amount, entry.MealTag, entry.Note,
			loggedAt, time.Now().UTC(),
		)
		if err != nil {
			return count, fmt.Errorf("inserting copied entry: %w", err)
		}
		count++
	}
	return count, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
go test ./internal/db/... -run TestCopyFoodLogEntries -v
```

Expected output:
```
--- PASS: TestCopyFoodLogEntries (...)
--- PASS: TestCopyFoodLogEntries_Empty (...)
PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/db/food-log-entries.go internal/db/food_log_copy_test.go
git commit -m "feat(db): add CopyFoodLogEntries for copying log entries between days"
```

---

## Task 2: API handler `POST /logs/copy`

**Files:**
- Modify: `internal/api/api.go`

- [ ] **Step 1: Add the request type, handler, and route registration**

In `internal/api/api.go`, add the following immediately after the `deleteLogEntryHandler` function (around line 543):

```go
type copyLogEntriesRequest struct {
	FromDate string   `json:"from_date"`
	ToDate   string   `json:"to_date"`
	MealTags []string `json:"meal_tags"`
}

func copyLogEntriesHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req copyLogEntriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.MealTags) == 0 {
		http.Error(w, "meal_tags must not be empty", http.StatusBadRequest)
		return
	}

	loc := getClientLocation(r)

	fromDate, err := time.ParseInLocation("2006-01-02", req.FromDate, loc)
	if err != nil {
		http.Error(w, "Invalid from_date, expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	toDate, err := time.ParseInLocation("2006-01-02", req.ToDate, loc)
	if err != nil {
		http.Error(w, "Invalid to_date, expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	if req.FromDate == req.ToDate {
		http.Error(w, "from_date and to_date must be different", http.StatusBadRequest)
		return
	}

	// Use current time when copying to today; noon on the target date for historical days
	now := time.Now()
	var loggedAt time.Time
	if req.ToDate == now.In(loc).Format("2006-01-02") {
		loggedAt = now.UTC()
	} else {
		loggedAt = time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 12, 0, 0, 0, loc).UTC()
	}

	count, err := db.CopyFoodLogEntries(userID, fromDate, req.MealTags, loggedAt)
	if err != nil {
		slog.Error("failed to copy log entries", "error", err)
		http.Error(w, "Failed to copy log entries", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]int{"count": count}); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}
```

In `RegisterLogsPaths`, add the new route **before** the existing `POST /logs` line so the more-specific pattern is explicit:

```go
func RegisterLogsPaths(mux *http.ServeMux) {
	mux.HandleFunc("GET /logs", getLogsHandler)
	mux.HandleFunc("POST /logs", createLogEntryHandler)
	mux.HandleFunc("POST /logs/copy", copyLogEntriesHandler)
	mux.HandleFunc("DELETE /logs/{id}", deleteLogEntryHandler)
}
```

- [ ] **Step 2: Verify the server compiles**

```bash
go build ./...
```

Expected: no output (clean build).

- [ ] **Step 3: Commit**

```bash
git add internal/api/api.go
git commit -m "feat(api): add POST /logs/copy endpoint"
```

---

## Task 3: Integration test for `POST /logs/copy`

**Files:**
- Create: `tests/09_copy_logs.sh`
- Modify: `test_runner.sh`

- [ ] **Step 1: Create `tests/09_copy_logs.sh`**

```bash
#!/bin/bash
# 09_copy_logs.sh: Copy logs between days

if [ -z "$BASE_URL" ]; then
    source "$(dirname "$0")/common.sh"
fi

echo "==================================================="
echo "Test: Copy Logs Between Days"
echo "---------------------------------------------------"

# Create a test food for this test
COPY_FOOD=$(curl -s -X POST "$BASE_URL/foods" \
  -H "Content-Type: application/json" \
  -d '{
  "name": "Copy Test Food",
  "calories": 100,
  "protein": 10,
  "carbs": 20,
  "fat": 5,
  "type": "food",
  "measurement_unit": "g",
  "measurement_amount": 100
}')
COPY_FOOD_ID=$(echo $COPY_FOOD | jq -r .id)
log_info "✅ Created Copy Test Food: $COPY_FOOD_ID"

# Compute yesterday in YYYY-MM-DD (macOS + Linux compatible)
YESTERDAY=$(date -u -v-1d +"%Y-%m-%d" 2>/dev/null || date -u -d "yesterday" +"%Y-%m-%d")
YESTERDAY_ISO="${YESTERDAY}T12:00:00Z"
TODAY=$(date -u +"%Y-%m-%d")

# Log breakfast, lunch, and dinner on yesterday
LOG_BFAST=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{\"food_id\": \"$COPY_FOOD_ID\", \"amount\": 100, \"meal_tag\": \"breakfast\", \"logged_at\": \"$YESTERDAY_ISO\"}")
LOG_BFAST_ID=$(echo $LOG_BFAST | jq -r .id)
log_info "✅ Logged breakfast on $YESTERDAY: $LOG_BFAST_ID"

LOG_LUNCH=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{\"food_id\": \"$COPY_FOOD_ID\", \"amount\": 150, \"meal_tag\": \"lunch\", \"logged_at\": \"$YESTERDAY_ISO\"}")
LOG_LUNCH_ID=$(echo $LOG_LUNCH | jq -r .id)
log_info "✅ Logged lunch on $YESTERDAY: $LOG_LUNCH_ID"

LOG_DINNER=$(curl -s -X POST "$BASE_URL/logs" \
  -H "Content-Type: application/json" \
  -d "{\"food_id\": \"$COPY_FOOD_ID\", \"amount\": 200, \"meal_tag\": \"dinner\", \"logged_at\": \"$YESTERDAY_ISO\"}")
LOG_DINNER_ID=$(echo $LOG_DINNER | jq -r .id)
log_info "✅ Logged dinner on $YESTERDAY: $LOG_DINNER_ID"

# Copy only breakfast and lunch to today
echo "Copying breakfast and lunch from $YESTERDAY to $TODAY..."
COPY_RESULT=$(curl -s -X POST "$BASE_URL/logs/copy" \
  -H "Content-Type: application/json" \
  -d "{\"from_date\": \"$YESTERDAY\", \"to_date\": \"$TODAY\", \"meal_tags\": [\"breakfast\", \"lunch\"]}")
COPY_COUNT=$(echo $COPY_RESULT | jq -r .count)
if [ "$COPY_COUNT" == "2" ]; then
    log_info "✅ Copy returned count: $COPY_COUNT"
else
    log_err "Expected count 2, got $COPY_COUNT"
    echo $COPY_RESULT | jq .
    exit 1
fi

# Verify entries appear on today
echo "Verifying copied entries appear on $TODAY..."
TODAY_LOGS=$(curl -s "$BASE_URL/logs?date=$TODAY")
BFAST_COUNT=$(echo $TODAY_LOGS | jq '[.[] | select(.meal_tag == "breakfast")] | length')
LUNCH_COUNT=$(echo $TODAY_LOGS | jq '[.[] | select(.meal_tag == "lunch")] | length')
if [ "$BFAST_COUNT" -ge 1 ] && [ "$LUNCH_COUNT" -ge 1 ]; then
    log_info "✅ Found breakfast ($BFAST_COUNT) and lunch ($LUNCH_COUNT) entries on $TODAY"
else
    log_err "Missing expected entries today: breakfast=$BFAST_COUNT lunch=$LUNCH_COUNT"
    echo $TODAY_LOGS | jq .
    exit 1
fi

# Validation: same from_date and to_date should return 400
echo "Testing from_date == to_date returns 400..."
SAME_DATE_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/logs/copy" \
  -H "Content-Type: application/json" \
  -d "{\"from_date\": \"$TODAY\", \"to_date\": \"$TODAY\", \"meal_tags\": [\"breakfast\"]}")
if [ "$SAME_DATE_CODE" == "400" ]; then
    log_info "✅ Same date returns 400"
else
    log_err "Expected 400 for same date, got $SAME_DATE_CODE"
    exit 1
fi

# Validation: empty meal_tags should return 400
echo "Testing empty meal_tags returns 400..."
EMPTY_TAGS_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/logs/copy" \
  -H "Content-Type: application/json" \
  -d "{\"from_date\": \"$YESTERDAY\", \"to_date\": \"$TODAY\", \"meal_tags\": []}")
if [ "$EMPTY_TAGS_CODE" == "400" ]; then
    log_info "✅ Empty meal_tags returns 400"
else
    log_err "Expected 400 for empty meal_tags, got $EMPTY_TAGS_CODE"
    exit 1
fi

# Validation: copying day with no entries returns count 0 (not an error)
echo "Testing copy of empty source date returns count 0..."
FAR_PAST="1990-01-01"
EMPTY_RESULT=$(curl -s -X POST "$BASE_URL/logs/copy" \
  -H "Content-Type: application/json" \
  -d "{\"from_date\": \"$FAR_PAST\", \"to_date\": \"$TODAY\", \"meal_tags\": [\"breakfast\"]}")
EMPTY_COUNT=$(echo $EMPTY_RESULT | jq -r .count)
if [ "$EMPTY_COUNT" == "0" ]; then
    log_info "✅ Empty source day returns count 0"
else
    log_err "Expected count 0 for empty source day, got $EMPTY_COUNT"
    exit 1
fi

# Cleanup
curl -s -X DELETE "$BASE_URL/logs/$LOG_BFAST_ID" > /dev/null
curl -s -X DELETE "$BASE_URL/logs/$LOG_LUNCH_ID" > /dev/null
curl -s -X DELETE "$BASE_URL/logs/$LOG_DINNER_ID" > /dev/null
# Also delete the copies we made (they're in today's logs)
TODAY_LOGS_AFTER=$(curl -s "$BASE_URL/logs?date=$TODAY")
echo $TODAY_LOGS_AFTER | jq -r '.[].id' | while read id; do
    curl -s -X DELETE "$BASE_URL/logs/$id" > /dev/null
done
curl -s -X DELETE "$BASE_URL/foods/$COPY_FOOD_ID" > /dev/null
log_info "✅ Copy logs cleanup done"

echo "==================================================="
echo "Copy Logs tests complete"
echo "==================================================="
```

- [ ] **Step 2: Make it executable**

```bash
chmod +x tests/09_copy_logs.sh
```

- [ ] **Step 3: Add to `test_runner.sh`**

In `test_runner.sh`, update the `TEST_FILES` array to include the new test file (add after `tests/08_account_profile.sh`):

```bash
TEST_FILES=(
    "tests/01_basics.sh"
    "tests/02_food_mgmt.sh"
    "tests/03_logging.sh"
    "tests/04_stats_reads.sh"
    "tests/05_input_validation.sh"
    "tests/07_account_passkeys.sh"
    "tests/08_account_profile.sh"
    "tests/09_copy_logs.sh"
)
```

- [ ] **Step 4: Run the integration test against a running server**

Start the server in one terminal:
```bash
DEV_AUTH=true go run ./cmd/api-server/main.go
```

Run the test in another terminal:
```bash
source tests/common.sh && source tests/09_copy_logs.sh
```

Expected: all `✅` lines, no `[FAIL]` lines.

- [ ] **Step 5: Commit**

```bash
git add tests/09_copy_logs.sh test_runner.sh
git commit -m "test: add integration tests for POST /logs/copy"
```

---

## Task 4: API client method

**Files:**
- Modify: `static-web/js/api.js`

- [ ] **Step 1: Add `copyLogs` method**

In `static-web/js/api.js`, add `copyLogs` after `deleteLog`:

```js
async copyLogs(fromDate, toDate, mealTags) {
    const tzOffset = new Date().getTimezoneOffset();
    return await this.request(`/logs/copy?tz_offset=${tzOffset}`, 'POST', {
        from_date: fromDate,
        to_date: toDate,
        meal_tags: mealTags,
    });
}
```

- [ ] **Step 2: Commit**

```bash
git add static-web/js/api.js
git commit -m "feat(api-client): add copyLogs method"
```

---

## Task 5: CSS and HTML markup

**Files:**
- Modify: `static-web/css/main.css`
- Modify: `static-web/foodlog.html`

- [ ] **Step 1: Add `.section-header` utility class and dialog styles to `main.css`**

At the end of `static-web/css/main.css`, add:

```css
/* ============================================================
   Section Header (heading + inline action button)
   ============================================================ */
.section-header {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.section-header h2 {
  margin: 0;
}

/* ============================================================
   Copy Day Dialog
   ============================================================ */
dialog {
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  padding: var(--space-6);
  max-width: 480px;
  width: 100%;
  background: var(--color-surface);
}

dialog::backdrop {
  background: rgba(0, 0, 0, 0.4);
}

dialog h2 {
  margin-top: 0;
}

.copy-meal-checks {
  display: flex;
  gap: var(--space-4);
  flex-wrap: wrap;
  margin: var(--space-3) 0;
}

.copy-meal-checks label {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  cursor: pointer;
}

#copy-preview {
  margin: var(--space-4) 0;
  min-height: 40px;
  color: var(--color-text-muted);
  font-size: 0.9rem;
}

#copy-preview strong {
  display: block;
  color: var(--color-text);
  margin-top: var(--space-2);
}

.copy-preview-list {
  list-style: none;
  padding: 0;
  margin: var(--space-1) 0 0 var(--space-3);
}

.copy-dialog-actions {
  display: flex;
  gap: var(--space-3);
  margin-top: var(--space-4);
}
```

- [ ] **Step 2: Update `foodlog.html`**

Replace the `logs-list-section` div and add the dialog. Find this block:

```html
        <div class="logs-list-section">
            <h2>Logs</h2>
            <ul id="logs-list" class="item-list">
                <!-- Logs will be populated here -->
            </ul>
        </div>
```

Replace with:

```html
        <div class="logs-list-section">
            <div class="section-header">
                <h2>Logs</h2>
                <button type="button" id="copy-day-btn" class="btn btn-secondary btn-sm">Copy from...</button>
            </div>
            <ul id="logs-list" class="item-list">
                <!-- Logs will be populated here -->
            </ul>
        </div>

        <dialog id="copy-day-dialog">
            <h2>Copy from another day</h2>
            <div class="form-group">
                <label for="copy-from-date">Copy from date</label>
                <input type="date" id="copy-from-date">
            </div>
            <div class="form-group">
                <label>Meals to copy</label>
                <div class="copy-meal-checks">
                    <label><input type="checkbox" name="copy_meal" value="breakfast" checked> Breakfast</label>
                    <label><input type="checkbox" name="copy_meal" value="lunch" checked> Lunch</label>
                    <label><input type="checkbox" name="copy_meal" value="dinner" checked> Dinner</label>
                    <label><input type="checkbox" name="copy_meal" value="snack" checked> Snack</label>
                </div>
            </div>
            <div id="copy-preview"></div>
            <div class="copy-dialog-actions">
                <button type="button" id="copy-confirm-btn" class="btn btn-primary" disabled>Copy 0 entries</button>
                <button type="button" id="copy-cancel-btn" class="btn btn-secondary">Cancel</button>
            </div>
        </dialog>
```

- [ ] **Step 3: Commit**

```bash
git add static-web/css/main.css static-web/foodlog.html
git commit -m "feat(ui): add copy-from dialog markup and styles"
```

---

## Task 6: JS modal logic

**Files:**
- Modify: `static-web/js/foodlog.js`

- [ ] **Step 1: Add `initCopyDialog` and `loadCopyPreview` to `foodlog.js`**

Add both functions before the `window.addEventListener('load', init)` line at the bottom of `static-web/js/foodlog.js`:

```js
function initCopyDialog() {
    const dialog = document.getElementById('copy-day-dialog');
    const openBtn = document.getElementById('copy-day-btn');
    const cancelBtn = document.getElementById('copy-cancel-btn');
    const confirmBtn = document.getElementById('copy-confirm-btn');
    const fromDateInput = document.getElementById('copy-from-date');

    openBtn.addEventListener('click', () => {
        // Default from-date: one day before currentDate
        const d = new Date(currentDate + 'T12:00:00');
        d.setDate(d.getDate() - 1);
        fromDateInput.value = getLocalDateString(d);

        // Reset all meal checkboxes to checked
        document.querySelectorAll('input[name="copy_meal"]').forEach(cb => { cb.checked = true; });

        dialog.showModal();
        loadCopyPreview();
    });

    cancelBtn.addEventListener('click', () => dialog.close());

    // Close on backdrop click
    dialog.addEventListener('click', (e) => {
        if (e.target === dialog) dialog.close();
    });

    fromDateInput.addEventListener('change', loadCopyPreview);
    document.querySelectorAll('input[name="copy_meal"]').forEach(cb => {
        cb.addEventListener('change', loadCopyPreview);
    });

    confirmBtn.addEventListener('click', async () => {
        const fromDate = fromDateInput.value;
        const checkedTags = [...document.querySelectorAll('input[name="copy_meal"]:checked')]
            .map(cb => cb.value);
        confirmBtn.disabled = true;
        try {
            const result = await api.copyLogs(fromDate, currentDate, checkedTags);
            dialog.close();
            loadLogs();
            showToast(`Copied ${result.count} ${result.count === 1 ? 'entry' : 'entries'}`);
        } catch (e) {
            showToast('Failed to copy entries: ' + e.message, 'error');
            confirmBtn.disabled = false;
        }
    });
}

async function loadCopyPreview() {
    const fromDate = document.getElementById('copy-from-date').value;
    const checkedTags = new Set(
        [...document.querySelectorAll('input[name="copy_meal"]:checked')].map(cb => cb.value)
    );
    const preview = document.getElementById('copy-preview');
    const confirmBtn = document.getElementById('copy-confirm-btn');

    preview.textContent = 'Loading...';
    confirmBtn.disabled = true;

    let logs;
    try {
        logs = await api.getLogs(fromDate);
    } catch (e) {
        preview.textContent = 'Failed to load preview.';
        return;
    }

    const filtered = (logs || []).filter(log => checkedTags.has(log.meal_tag));

    if (filtered.length === 0) {
        preview.textContent = 'No entries for selected meals on this date.';
        confirmBtn.textContent = 'Copy 0 entries';
        return;
    }

    // Group by meal tag in display order
    const groups = {};
    for (const log of filtered) {
        (groups[log.meal_tag] = groups[log.meal_tag] || []).push(log);
    }

    preview.textContent = '';
    for (const meal of ['breakfast', 'lunch', 'dinner', 'snack']) {
        if (!groups[meal]) continue;
        const heading = document.createElement('strong');
        heading.textContent = meal.charAt(0).toUpperCase() + meal.slice(1);
        preview.appendChild(heading);
        const ul = document.createElement('ul');
        ul.className = 'copy-preview-list';
        for (const log of groups[meal]) {
            const li = document.createElement('li');
            const name = log.food ? log.food.name : (log.note ? `[qc] ${log.note}` : 'Quick Add');
            const cals = log.calories != null ? ` (${Math.round(log.calories)} kcal)` : '';
            li.textContent = `${name}${cals}`;
            ul.appendChild(li);
        }
        preview.appendChild(ul);
    }

    confirmBtn.disabled = false;
    confirmBtn.textContent = `Copy ${filtered.length} ${filtered.length === 1 ? 'entry' : 'entries'}`;
}
```

- [ ] **Step 2: Call `initCopyDialog()` at the end of the `init` function**

Find the end of the `init` function (currently ends with `document.getElementById('add-log-form').addEventListener('submit', addLog);` and the mode radio loop). Add `initCopyDialog();` as the last statement inside `init`:

```js
async function init() {
    // ... existing code ...

    // Mode toggling
    const modeRadios = document.getElementsByName('entry_mode');
    modeRadios.forEach(radio => {
        radio.addEventListener('change', toggleMode);
    });

    initCopyDialog();  // ← add this line
}
```

- [ ] **Step 3: Verify the page loads without errors**

Start the server:
```bash
DEV_AUTH=true go run ./cmd/api-server/main.go
```

Open `http://localhost:8080` in a browser and navigate to the Food Log page. Open the browser console — there should be no JS errors. The "Copy from..." button should be visible next to the Logs heading.

- [ ] **Step 4: Manual smoke test**

1. Log a couple of entries for yesterday (change the date picker to yesterday, add some foods, change back to today)
2. Click "Copy from..." — the dialog should open with yesterday's date selected and all meals checked
3. The preview should show the entries you just logged
4. Uncheck one meal — the preview should update and the button count should decrease
5. Click "Copy N entries" — the dialog closes, the log list reloads, and a toast appears
6. Verify the copied entries appear in today's log list

- [ ] **Step 5: Commit**

```bash
git add static-web/js/foodlog.js
git commit -m "feat(foodlog): add copy-from-day modal with preview"
```
