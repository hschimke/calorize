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
			COALESCE(SUM( (l.amount / f.measurement_amount) * f.calories ), 0),
			COALESCE(SUM( (l.amount / f.measurement_amount) * f.protein ), 0),
			COALESCE(SUM( (l.amount / f.measurement_amount) * f.carbs ), 0),
			COALESCE(SUM( (l.amount / f.measurement_amount) * f.fat ), 0)
		FROM logs l
		JOIN foods f ON l.food_id = f.id
		WHERE l.user_id = ? 
		  AND l.logged_at >= ? 
		  AND l.logged_at < ? 
		  AND l.deleted_at IS NULL
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
