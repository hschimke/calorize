# Calorie Goal Feature — Design Spec

**Date:** 2026-03-30

## Overview

Allow a user to set a daily calorie goal. The app derives weekly and monthly targets automatically (daily × 7, daily × 30.4). A progress bar on the dashboard, food log, and stats pages shows progress toward the goal for the active period.

---

## Data Storage

**Migration:** Add `calorie_goal INTEGER` (nullable) to the `users` table.

- Nullable means "no goal set" — the UI hides all goal-related elements when null.
- No separate settings or goals table; this is the simplest model for a single scalar goal.

**Model change:** `User` struct gains `CalorieGoal *int` field (pointer to allow null).

---

## API

Two new endpoints, consistent with the existing `/account` handler pattern:

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/account/profile` | Returns current user's profile including `calorie_goal` |
| `PUT` | `/account/profile` | Updates `calorie_goal`; send `null` to clear |

Request body for `PUT /account/profile`:
```json
{ "calorie_goal": 2000 }
```
or to clear:
```json
{ "calorie_goal": null }
```

Reuses `GetUserByID` for the GET handler. `UpdateUser` must be extended to include `calorie_goal` in its UPDATE query (it currently only sets `name`, `email`, `disabled_at`). No new db functions needed beyond the column addition.

---

## Account Page (`account.html`)

A new "Goals" panel is added below the existing Passkeys panel.

**Layout:**
```
┌─ Goals ──────────────────────────────────────┐
│  Daily Calorie Goal                           │
│  [  2000  ] kcal        [Save Goal]           │
│                                               │
│  Weekly: 14,000 kcal  •  Monthly: ~60,900 kcal│
└───────────────────────────────────────────────┘
```

- Number input pre-filled with current goal (empty if not set).
- Derived weekly/monthly values update live as the user types (daily × 7, daily × 30.4).
- "Save Goal" calls `PUT /account/profile`.
- Clearing the input and saving sends `null`, removing the goal.
- Uses existing `.panel`, `.btn`, `.btn-primary` design system classes.

---

## Goal Display (Dashboard, Food Log, Stats)

A progress bar appears below the Calories stat card **only when a goal is set**. When no goal is set, no empty space is shown.

**Visual:**
```
┌─ Calories ──────┐
│   1,240         │
│ ████████░░ 62%  │
│ 760 remaining   │
└─────────────────┘
```

**Behavior:**
- Bar fills proportionally to `consumed / goal`.
- Capped at 100% when over goal; bar turns red (`--color-danger`).
- Label shows "X remaining" normally; switches to "X over goal" in red when exceeded.
- Period scaling: foodlog and dashboard always use the daily goal. The stats page scales based on the active period (×7 for week, ×30.4 for month).

**Implementation:** A shared `renderCalorieGoalBar(consumed, goal)` helper added to `js/ui.js`. Each page calls this helper after loading its calorie data. Returns an HTML element (or null if goal is null) that is inserted below the Calories stat card.

---

## Out of Scope

- Macro goals (protein, carbs, fat) — deferred; can be added later with a `user_goals` table migration if needed.
- Per-day or custom period goals.
- Goal history or tracking over time.
