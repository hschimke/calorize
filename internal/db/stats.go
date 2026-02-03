package db

import (
	"fmt"
	"time"
)

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

	// Normalize date to start of day
	y, m, d := date.Date()
	todayStart := time.Date(y, m, d, 0, 0, 0, 0, date.Location())

	switch period {
	case "day":
		start = todayStart
		end = start.AddDate(0, 0, 1)
	case "week":
		// Assume week starting Monday? Or Sunday. Let's say Monday.
		weekday := int(todayStart.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = todayStart.AddDate(0, 0, -weekday+1)
		end = start.AddDate(0, 0, 7)
	case "month":
		start = time.Date(y, m, 1, 0, 0, 0, 0, date.Location())
		end = start.AddDate(0, 1, 0) // Start of next month
	default:
		return RangeStats{}, fmt.Errorf("invalid period: %s", period)
	}

	query := `
		SELECT 
			SUM(COALESCE((le.amount / CASE WHEN f.measurement_amount = 0 THEN 1 ELSE f.measurement_amount END) * f.calories, le.calories, 0)) as calories,
			SUM(COALESCE((le.amount / CASE WHEN f.measurement_amount = 0 THEN 1 ELSE f.measurement_amount END) * f.protein, 0)) as protein,
			SUM(COALESCE((le.amount / CASE WHEN f.measurement_amount = 0 THEN 1 ELSE f.measurement_amount END) * f.carbs, 0)) as carbs,
			SUM(COALESCE((le.amount / CASE WHEN f.measurement_amount = 0 THEN 1 ELSE f.measurement_amount END) * f.fat, 0)) as fat
		FROM food_log_entries le
		LEFT JOIN foods f ON le.food_id = f.id
		WHERE le.user_id = ? AND le.logged_at >= ? AND le.logged_at < ?
		AND le.deleted_at IS NULL
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
	s.Date = start.Format("2006-01-02") // Just label with start date

	return s, nil
}
