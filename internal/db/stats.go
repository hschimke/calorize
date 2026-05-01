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

type ConsistencyStats struct {
	Rolling7dCalories  float64 `json:"rolling_7d_calories"`
	Rolling30dCalories float64 `json:"rolling_30d_calories"`
	Streak             int     `json:"streak"`
	HitDays30d         int     `json:"hit_days_30d"`
	TrackedDays30d     int     `json:"tracked_days_30d"`
	CalorieGoal        int     `json:"calorie_goal"`
}

// GetConsistencyStats computes rolling averages, streak, and 30-day hit rate.
// calorieGoal is the user's daily calorie target (0 means not set).
// now is the reference time; today's partial data is excluded.
func GetConsistencyStats(userID UserID, tzOffsetMins int, calorieGoal int, now time.Time) (ConsistencyStats, error) {
	shiftMins := -tzOffsetMins
	shiftExpr := fmt.Sprintf("%+d minutes", shiftMins)

	loc := time.FixedZone("Client", -tzOffsetMins*60)
	nowLocal := now.In(loc)
	y, m, d := nowLocal.Date()
	todayStart := time.Date(y, m, d, 0, 0, 0, 0, loc).UTC()
	windowStart := todayStart.AddDate(0, 0, -365)

	query := `
		SELECT
			date(datetime(logged_at, ?)) AS day,
			SUM(COALESCE(calories, 0)) AS day_calories
		FROM food_log_entries
		WHERE user_id = ?
		  AND logged_at >= ? AND logged_at < ?
		  AND deleted_at IS NULL
		GROUP BY day
		ORDER BY day DESC
	`

	rows, err := db.Query(query, shiftExpr, userID, windowStart, todayStart)
	if err != nil {
		return ConsistencyStats{}, fmt.Errorf("querying consistency stats: %w", err)
	}
	defer rows.Close()

	dayCalories := make(map[string]float64)
	for rows.Next() {
		var day string
		var cals float64
		if err := rows.Scan(&day, &cals); err != nil {
			return ConsistencyStats{}, fmt.Errorf("scanning consistency row: %w", err)
		}
		dayCalories[day] = cals
	}
	if err := rows.Err(); err != nil {
		return ConsistencyStats{}, fmt.Errorf("iterating consistency rows: %w", err)
	}

	var sum7d, sum30d float64
	var hitDays30d, trackedDays30d, streak int
	// If no goal is set, streak stays 0 and we stop iterating after 30 days.
	streakBroken := calorieGoal == 0

	for i := 1; i <= 365; i++ {
		if i > 30 && streakBroken {
			break
		}
		day := todayStart.AddDate(0, 0, -i).In(loc).Format("2006-01-02")
		cals, hasEntry := dayCalories[day]

		if i <= 7 {
			sum7d += cals
		}
		if i <= 30 {
			sum30d += cals
			if hasEntry && cals > 0 {
				trackedDays30d++
				if calorieGoal > 0 && cals <= float64(calorieGoal) {
					hitDays30d++
				}
			}
		}
		if !streakBroken {
			if hasEntry && cals > 0 && cals <= float64(calorieGoal) {
				streak++
			} else {
				streakBroken = true
			}
		}
	}

	return ConsistencyStats{
		Rolling7dCalories:  sum7d / 7.0,
		Rolling30dCalories: sum30d / 30.0,
		Streak:             streak,
		HitDays30d:         hitDays30d,
		TrackedDays30d:     trackedDays30d,
		CalorieGoal:        calorieGoal,
	}, nil
}
