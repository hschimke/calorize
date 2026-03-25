package db

import (
	"fmt"
	"time"
)

func GetStatsByDay(userID UserID, period string, date time.Time, tzOffsetMins int) ([]RangeStats, error) {
	if period == "day" {
		return nil, fmt.Errorf("breakdown not supported for period: day")
	}

	var start, end time.Time
	loc := date.Location()
	y, m, d := date.Date()
	localStart := time.Date(y, m, d, 0, 0, 0, 0, loc)

	switch period {
	case "week":
		weekday := int(localStart.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = localStart.AddDate(0, 0, -weekday+1).UTC()
		end = start.AddDate(0, 0, 7)
	case "month":
		start = time.Date(y, m, 1, 0, 0, 0, 0, loc).UTC()
		end = start.AddDate(0, 1, 0)
	default:
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	// Shift logged_at (UTC) into the client's local timezone before extracting
	// the date, so entries near midnight are attributed to the correct local day.
	// tzOffsetMins is the raw JS getTimezoneOffset() value: negative for UTC+X zones.
	// We negate it to get the actual UTC offset to add.
	shiftMins := -tzOffsetMins
	shiftExpr := fmt.Sprintf("%+d minutes", shiftMins)

	query := `
		SELECT
			date(datetime(logged_at, ?)) AS day,
			SUM(COALESCE(calories, 0)),
			SUM(COALESCE(protein, 0)),
			SUM(COALESCE(carbs, 0)),
			SUM(COALESCE(fat, 0))
		FROM food_log_entries
		WHERE user_id = ? AND logged_at >= ? AND logged_at < ?
		  AND deleted_at IS NULL
		GROUP BY day
		ORDER BY day
	`

	rows, err := db.Query(query, shiftExpr, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("querying stats by day: %w", err)
	}
	defer rows.Close()

	// Build a map of date string → RangeStats from DB results
	byDate := make(map[string]RangeStats)
	for rows.Next() {
		var s RangeStats
		if err := rows.Scan(&s.Date, &s.Calories, &s.Protein, &s.Carbs, &s.Fat); err != nil {
			return nil, fmt.Errorf("scanning stats row: %w", err)
		}
		byDate[s.Date] = s
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating stats rows: %w", err)
	}

	// Build complete slice, zero-filling missing days
	var result []RangeStats
	for cur := start.In(loc); cur.Before(end.In(loc)); cur = cur.AddDate(0, 0, 1) {
		dateStr := cur.Format("2006-01-02")
		if s, ok := byDate[dateStr]; ok {
			result = append(result, s)
		} else {
			result = append(result, RangeStats{Date: dateStr})
		}
	}

	return result, nil
}

type RangeStats struct {
	Date     string  `json:"date"` // YYYY-MM-DD
	Calories float64 `json:"calories"`
	Protein  float64 `json:"protein"`
	Carbs    float64 `json:"carbs"`
	Fat      float64 `json:"fat"`
}

func GetStats(userID UserID, period string, date time.Time) (RangeStats, error) {
	// Calculate start and end times based on period
	// We assume 'date' is in the user's timezone or meaningful to them.
	// We'll treat it as UTC for DB comparison or rely on truncated strings if using SQLite functions.
	//
	// Recommended: Calculate range in Go, pass as parameters.
	// Period: 'day', 'week', 'month'

	var start, end time.Time

	loc := date.Location()
	y, m, d := date.Date()
	localStart := time.Date(y, m, d, 0, 0, 0, 0, loc)

	switch period {
	case "day":
		start = localStart.UTC()
		end = start.Add(24 * time.Hour)
	case "week":
		// Assume week starting Monday
		weekday := int(localStart.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = localStart.AddDate(0, 0, -weekday+1).UTC()
		end = start.AddDate(0, 0, 7)
	case "month":
		start = time.Date(y, m, 1, 0, 0, 0, 0, loc).UTC()
		end = start.AddDate(0, 1, 0) // Start of next month

	default:
		return RangeStats{}, fmt.Errorf("invalid period: %s", period)
	}

	query := `
		SELECT 
			SUM(COALESCE(calories, 0)) as calories,
			SUM(COALESCE(protein, 0)) as protein,
			SUM(COALESCE(carbs, 0)) as carbs,
			SUM(COALESCE(fat, 0)) as fat
		FROM food_log_entries
		WHERE user_id = ? AND logged_at >= ? AND logged_at < ?
		AND deleted_at IS NULL
	`

	row := db.QueryRow(query, userID, start, end)

	var s RangeStats
	// SQLite SUM returns NULL if no rows, scan might fail if not nullable pointers.
	// Use sql.NullFloat64 or pointers.
	var cal, prot, carb, fat *float64
	if err := row.Scan(&cal, &prot, &carb, &fat); err != nil {
		return RangeStats{}, fmt.Errorf("scanning stats: %w", err)
	}

	if cal != nil {
		s.Calories = *cal
	}
	if prot != nil {
		s.Protein = *prot
	}
	if carb != nil {
		s.Carbs = *carb
	}
	if fat != nil {
		s.Fat = *fat
	}
	s.Date = start.In(loc).Format("2006-01-02") // Just label with start date

	return s, nil
}
