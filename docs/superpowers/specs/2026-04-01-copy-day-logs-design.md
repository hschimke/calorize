# Copy Day Logs Feature

**Date:** 2026-04-01  
**Branch:** feature/foodlog-copy  
**Status:** Approved

## Overview

Allow users to copy food log entries from any previous date into the currently selected date. Users choose a source date (defaulting to the day before the target) and select which meal categories to copy. A preview shows what will be copied before confirming.

## Backend

### New endpoint: `POST /logs/copy`

**Request body:**
```json
{
  "from_date": "YYYY-MM-DD",
  "to_date": "YYYY-MM-DD",
  "meal_tags": ["breakfast", "lunch", "dinner", "snack"]
}
```

**Behavior:**
- Authenticated endpoint (same auth as other `/logs` routes)
- Fetches all non-deleted `food_log_entries` for `from_date` belonging to the requesting user
- Filters to only entries whose `meal_tag` is in `meal_tags`
- Inserts a new copy of each matched entry: same `food_id`, `portion_name`, `amount`, `calories`, `protein`, `carbs`, `fat`, `note`, `meal_tag` — but new UUID and `logged_at = time.Now().UTC()`
- Returns `{ "count": N }` where N is the number of entries copied
- If no entries match the filter, returns `{ "count": 0 }` (not an error)

**Validation:**
- `from_date` must parse as `YYYY-MM-DD`
- `to_date` must parse as `YYYY-MM-DD`
- `from_date` must not equal `to_date`
- `meal_tags` must be non-empty

**New code:**
- `internal/db/food-log-entries.go`: add `CopyFoodLogEntries(userID, fromDate, toDate, mealTags)` — fetches matching entries and bulk-inserts copies
- `internal/api/api.go`: add `copyLogEntriesHandler` and register `POST /logs/copy`

## Frontend

### Trigger

A "Copy from..." secondary button (`btn btn-secondary`) placed inline with the "Logs" `<h2>` in `foodlog.html`, using a flex row to keep them aligned.

### Modal

A native `<dialog>` element appended to `#app`. Opens when the button is clicked.

**Controls:**
- Date input (type="date") labelled "Copy from", defaulting to `currentDate − 1 day`
- Four checkboxes: Breakfast, Lunch, Dinner, Snack — all checked by default

**Preview panel:**
- Auto-updates on any change to the date or meal checkboxes
- Calls `api.getLogs(fromDate)` then filters client-side by checked meal tags
- Renders entries grouped by meal tag: meal name as a sub-heading, then entry name + calorie count per item
- If no entries match: displays "No entries for selected meals on this date."

**Action buttons:**
- "Copy N entries" (primary) — label reflects current preview count; disabled when count is 0
- "Cancel" (secondary) — closes the dialog without action

### Confirm flow

1. `POST /logs/copy` with `{ from_date, to_date: currentDate, meal_tags: [...checkedTags] }`
2. On success: close modal, call `loadLogs()`, `showToast("Copied N entries")`
3. On failure: `showToast("Failed to copy entries: " + error, 'error')`

### New API client method

```js
// api.js
async copyLogs(fromDate, toDate, mealTags) {
    return await this.request('/logs/copy', 'POST', {
        from_date: fromDate,
        to_date: toDate,
        meal_tags: mealTags,
    });
}
```

## Data Flow

```
User clicks "Copy from..."
→ Modal opens; from_date = currentDate − 1 day; all meals checked
→ Frontend: GET /logs?date={from_date} → render preview
→ User adjusts date or meals → preview re-renders
→ User clicks "Copy N entries"
→ POST /logs/copy { from_date, to_date, meal_tags }
→ Modal closes
→ loadLogs() refreshes target day
→ showToast("Copied N entries")
```

## Constraints & Notes

- `logged_at` on copied entries: if `to_date` is today in the client's timezone, uses `time.Now().UTC()`; if `to_date` is a historical date, uses noon on that date in the client's timezone (converted to UTC). This keeps entries within the correct day boundary regardless of UTC offset.
- The copy is additive: existing entries on the target date are not modified or deduplicated
- The preview fetch (`GET /logs`) happens client-side on every change; no new read endpoint is needed
- All existing CSS/component classes apply; no new design tokens needed
