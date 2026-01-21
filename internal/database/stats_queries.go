package database

import (
	"context"
	"time"
)

type Stats struct {
	TotalCalories  float64    `json:"total_calories"`
	TotalProtein   float64    `json:"total_protein"`
	TotalCarbs     float64    `json:"total_carbs"`
	TotalFat       float64    `json:"total_fat"`
	TotalNutrients []Nutrient `json:"total_nutrients"`
}

func GetStats(ctx context.Context, userID string, start, end time.Time) (*Stats, error) {
	// 1. Fetch all logs in range with food info
	query := `
		SELECT 
			l.food_id,
			l.amount,
			f.measurement_amount,
			f.calories,
			f.protein,
			f.carbs,
			f.fat
		FROM logs l
		JOIN foods f ON l.food_id = f.id
		WHERE l.user_id = ? 
		  AND l.logged_at >= ? 
		  AND l.logged_at < ?
		  AND l.deleted_at IS NULL
	`

	rows, err := DB.QueryContext(ctx, query, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var s Stats
	foodFactors := make(map[string]float64) // food_id -> total number of "servings" (amount / measurement_amount)

	for rows.Next() {
		var (
			foodID                 string
			logAmount              float64
			measureAmount          float64
			cals, prot, carbs, fat float64
		)
		if err := rows.Scan(&foodID, &logAmount, &measureAmount, &cals, &prot, &carbs, &fat); err != nil {
			return nil, err
		}

		if measureAmount == 0 {
			continue // avoid division by zero
		}
		ratio := logAmount / measureAmount

		// Aggregate standard macros
		s.TotalCalories += cals * ratio
		s.TotalProtein += prot * ratio
		s.TotalCarbs += carbs * ratio
		s.TotalFat += fat * ratio

		// Aggregate factor for flexible nutrients
		foodFactors[foodID] += ratio
	}

	if len(foodFactors) == 0 {
		s.TotalNutrients = []Nutrient{} // ensure valid empty slice
		return &s, nil
	}

	// 2. Fetch flexible nutrients for the foods involved
	// We can query all nutrients for these food IDs.
	// Since we might have many IDs, we can construct an IN clause or JOIN.
	// Actually, easier to SELECT ... FROM food_nutrients WHERE food_id IN (...) is dangerous with limit.
	// Better:
	// SELECT fn.food_id, fn.name, fn.amount, fn.unit
	// FROM food_nutrients fn
	// JOIN logs l ON fn.food_id = l.food_id
	// WHERE l.user_id = ? ...
	// But that duplicates rows if multiple logs.

	// Let's iterate distinct IDs in chunks if needed, or if small enough, just all?
	// Or use a temporary table?
	// Simplest for now: Iterate and select? No, N+1.
	// Let's use the JOIN approach but DISTINCT?
	// SELECT DISTINCT fn.food_id, fn.name, fn.amount, fn.unit
	// FROM food_nutrients fn
	// JOIN logs l ON fn.food_id = l.food_id
	// WHERE l.user_id = ? AND l.logged_at >= ? ...

	flexQuery := `
		SELECT DISTINCT fn.food_id, fn.name, fn.amount, fn.unit
		FROM food_nutrients fn
		JOIN logs l ON fn.food_id = l.food_id
		WHERE l.user_id = ? 
		  AND l.logged_at >= ? 
		  AND l.logged_at < ?
		  AND l.deleted_at IS NULL
	`
	flexRows, err := DB.QueryContext(ctx, flexQuery, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer flexRows.Close()

	type nutriKey struct {
		Name string
		Unit string
	}
	nutriSums := make(map[nutriKey]float64)

	for flexRows.Next() {
		var (
			foodID string
			name   string
			amount float64
			unit   string
		)
		if err := flexRows.Scan(&foodID, &name, &amount, &unit); err != nil {
			return nil, err
		}

		factor := foodFactors[foodID]
		nutriSums[nutriKey{name, unit}] += amount * factor
	}

	for k, v := range nutriSums {
		s.TotalNutrients = append(s.TotalNutrients, Nutrient{
			Name:   k.Name,
			Unit:   k.Unit,
			Amount: v,
		})
	}

	return &s, nil
}
