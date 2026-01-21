package database

import (
	"context"
	"time"
)

type Stats struct {
	TotalCalories float64 `json:"total_calories"`
	TotalProtein  float64 `json:"total_protein"`
	TotalCarbs    float64 `json:"total_carbs"`
	TotalFat      float64 `json:"total_fat"`
}

func GetStats(ctx context.Context, userID string, start, end time.Time) (*Stats, error) {
	// join logs with foods
	// calculate (log.amount / food.measurement_amount) * food.nutrient
	query := `
		SELECT 
			COALESCE(SUM(calories), 0),
			COALESCE(SUM(protein), 0),
			COALESCE(SUM(carbs), 0),
			COALESCE(SUM(fat), 0)
		FROM logs_with_nutrients
		WHERE user_id = ? 
		  AND logged_at >= ? 
		  AND logged_at < ?
	`

	var s Stats
	err := DB.QueryRowContext(ctx, query, userID, start, end).Scan(
		&s.TotalCalories,
		&s.TotalProtein,
		&s.TotalCarbs,
		&s.TotalFat,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
