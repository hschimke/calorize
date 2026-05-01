# Stats Consistency Panel — Design Spec

**Date:** 2026-04-30  
**Status:** Approved

## Context

The stats page currently shows total calories, protein, carbs, and fat for a selected period (day/week/month), plus several charts. Users set a `calorie_goal` but there is no feedback on how consistently they hit it, nor any smoothed view of their intake over time. This adds a "Consistency" panel with rolling averages and goal-tracking metrics to make the stats page more motivating and actionable.

---

## Backend

### New endpoint: `GET /stats/consistency`

Query parameters:
- `tz_offset` (int, minutes) — JavaScript `getTimezoneOffset()` value, same convention as existing stats endpoints

Response:
```json
{
  "rolling_7d_calories":  1842.0,
  "rolling_30d_calories": 1923.0,
  "streak":               5,
  "hit_days_30d":         23,
  "tracked_days_30d":     28,
  "calorie_goal":         2000
}
```

Fields:
- `rolling_7d_calories` — average daily calories over the 7 complete days ending yesterday
- `rolling_30d_calories` — average daily calories over the 30 complete days ending yesterday
- `streak` — consecutive days (ending yesterday) where at least one entry was logged AND daily total ≤ `calorie_goal`
- `hit_days_30d` — days within the trailing 30 days where daily total > 0 AND ≤ `calorie_goal`
- `tracked_days_30d` — days within the trailing 30 days where at least one entry was logged (denominator for hit rate)
- `calorie_goal` — user's goal (passed through so the frontend doesn't need a separate profile fetch)

**"On target" definition:** a day is on target when at least one entry was logged (`day_calories > 0`) AND the daily total is ≤ `calorie_goal`. Days with no entries break the streak and are excluded from `tracked_days_30d`.

**Rolling averages exclude today** (trailing complete days only) so partial-day data doesn't skew the average. Streak also starts from yesterday for the same reason.

### New Go function: `db.GetConsistencyStats`

Location: `internal/db/stats.go`

Signature:
```go
func (db *DB) GetConsistencyStats(userID string, tzOffsetMins int, now time.Time) (ConsistencyStats, error)
```

New struct:
```go
type ConsistencyStats struct {
    Rolling7dCalories  float64 `json:"rolling_7d_calories"`
    Rolling30dCalories float64 `json:"rolling_30d_calories"`
    Streak             int     `json:"streak"`
    HitDays30d         int     `json:"hit_days_30d"`
    TrackedDays30d     int     `json:"tracked_days_30d"`
    CalorieGoal        int     `json:"calorie_goal"`
}
```

Implementation uses a single query whose results drive all four computed fields:

**Query — Daily totals** (reuses existing timezone-shift pattern):
```sql
SELECT
    date(datetime(logged_at, ?)) AS day,
    SUM(COALESCE(calories, 0)) AS day_calories
FROM food_log_entries
WHERE user_id = ?
  AND logged_at >= ?   -- 365 days ago at midnight (local) — generous window for streak
  AND logged_at < ?    -- start of today (local)
  AND deleted_at IS NULL
GROUP BY day
ORDER BY day DESC
```

Go code processes the result set (a map of day → calories) to compute:
- **`rolling_7d_calories`** — sum of `day_calories` for days within trailing 7 days, divided by 7
- **`rolling_30d_calories`** — sum of `day_calories` for days within trailing 30 days, divided by 30
- **`streak`** — walk backwards from yesterday; count consecutive days where `day_calories > 0 AND day_calories <= calorie_goal`; stop at the first day that fails or has no entry
- **`hit_days_30d`** / **`tracked_days_30d`** — within the 30-day window, count days where `day_calories > 0 AND day_calories <= calorie_goal` and days where `day_calories > 0` respectively

The handler fetches `calorie_goal` from the users table before calling `GetConsistencyStats` (the same way other handlers access user profile data). If `calorie_goal` is NULL or 0, `streak` and hit-rate fields are returned as 0.

### Route registration

Add in `internal/api/api.go` alongside the existing stats routes:
```go
mux.Handle("GET /stats/consistency", requireAuth(http.HandlerFunc(h.getConsistencyStatsHandler)))
```

---

## Frontend

### HTML — `static-web/stat-ui.html`

New panel inserted between the `.stats-container` and `#panel-macro`:
```html
<div class="panel" id="panel-consistency">
    <h2>Consistency</h2>
    <div class="stats-container">
        <div class="stat-card">
            <h3>7-Day Avg</h3>
            <span id="stat-avg-7d">—</span>
            <small>cal/day</small>
        </div>
        <div class="stat-card">
            <h3>30-Day Avg</h3>
            <span id="stat-avg-30d">—</span>
            <small>cal/day</small>
        </div>
        <div class="stat-card">
            <h3>Streak</h3>
            <span id="stat-streak">—</span>
            <small>days on target</small>
        </div>
        <div class="stat-card">
            <h3>On Track</h3>
            <span id="stat-hit-rate">—</span>
            <small>last 30 days</small>
        </div>
    </div>
</div>
```

### JS — `static-web/js/api.js`

New method:
```js
async getConsistencyStats() {
    const tzOffset = new Date().getTimezoneOffset();
    return this.get(`/stats/consistency?tz_offset=${tzOffset}`);
}
```

### JS — `static-web/js/stat-ui.js`

New function `loadConsistencyStats()` called once during `init()`:
- Calls `api.getConsistencyStats()`
- Updates `#stat-avg-7d` and `#stat-avg-30d` with rounded integer values
- Updates `#stat-streak` — if streak = 0 shows "0", otherwise shows count with flame suffix (e.g. `"5 🔥"`)
- Updates `#stat-hit-rate` — shows `"hit_days_30d / tracked_days_30d"` (e.g. `"23/28"`)
- If `calorie_goal` is 0/null, Streak and On Track show `"—"` with no subtext change (the `<small>` already explains)
- On error: silently leaves values as `"—"` (non-critical panel)

The panel is **not** re-fetched when the period selector changes — it always reflects the trailing window.

---

## Verification

1. Start the server: `DEV_AUTH=true go run ./cmd/api-server/main.go`
2. Log food entries across several days (including some days over goal, some under)
3. `GET /stats/consistency?tz_offset=0` — confirm all six fields present and values are arithmetically correct against raw log data
4. Set `calorie_goal = 0` on the test user — confirm `streak` and hit rate fields return 0/0
5. Open `stat-ui.html` in browser — confirm the Consistency panel renders with correct values
6. Switch period selector (day/week/month) — confirm the Consistency panel does not change
7. Run `go test ./internal/db/...` — confirm existing tests pass
