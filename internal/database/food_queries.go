package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// CreateFood creates a new food. If familyID is empty, it generates a new one (First version).
// If familyID is provided, it creates a new version for that family.
func CreateFood(ctx context.Context, f *Food) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	if f.FamilyID == "" {
		f.FamilyID = uuid.New().String() // New Family
		f.Version = 1
		f.IsCurrent = true
	} else {
		// New Version logic:
		// 1. Get current version number? Or caller provides?
		// 2. Mark old current as false.
		// Caller should handle transaction preferably, but MVP:
		// We'll assume caller handles "unsetting" old current if needed, or we do it here.
		// Let's do it here.
		_, err := DB.ExecContext(ctx, "UPDATE foods SET is_current = 0 WHERE family_id = ?", f.FamilyID)
		if err != nil {
			return err
		}
		// Version is provided by caller or we query?
		// We'll assume caller computed it or we query max.
		// For simplicity, let's query max version.
		var maxVer int
		err = DB.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM foods WHERE family_id = ?", f.FamilyID).Scan(&maxVer)
		if err != nil {
			// ignore error? no.
		}
		f.Version = maxVer + 1
		f.IsCurrent = true
	}
	f.CreatedAt = time.Now()

	_, err := DB.ExecContext(ctx, `
		INSERT INTO foods (id, family_id, version, is_current, name, calories, protein, carbs, fat, type, measurement_unit, measurement_amount, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, f.ID, f.FamilyID, f.Version, f.IsCurrent, f.Name, f.Calories, f.Protein, f.Carbs, f.Fat, f.Type, f.MeasurementUnit, f.MeasurementAmount, f.CreatedAt)
	if err != nil {
		return err
	}

	// Insert Nutrients
	for _, n := range f.Nutrients {
		_, err := DB.ExecContext(ctx, "INSERT INTO food_nutrients (food_id, name, amount, unit) VALUES (?, ?, ?, ?)", f.ID, n.Name, n.Amount, n.Unit)
		if err != nil {
			return err
		}
	}

	return nil
}

func GetFoods(ctx context.Context, onlyCurrent bool) ([]Food, error) {
	query := "SELECT id, family_id, version, is_current, name, calories, protein, carbs, fat, type, measurement_unit, measurement_amount, created_at FROM foods"
	if onlyCurrent {
		query += " WHERE is_current = 1 AND deleted_at IS NULL"
	} else {
		query += " WHERE deleted_at IS NULL"
	}
	query += " ORDER BY name ASC"

	rows, err := DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var foods []Food
	foodMap := make(map[string]*Food) // Pointers to slice elements would be tricky as slice reallocates.
	// So let's store indices or pointers to elements if we preallocate, or just append to slice and rely on ID map to index.

	// Better: Read all foods, then fetch all relevant nutrients.
	for rows.Next() {
		var f Food
		if err := rows.Scan(&f.ID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name, &f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type, &f.MeasurementUnit, &f.MeasurementAmount, &f.CreatedAt); err != nil {
			return nil, err
		}
		foods = append(foods, f)
	}

	if len(foods) == 0 {
		return foods, nil
	}

	// Store pointers to foods in a map for easy assignment
	for i := range foods {
		foodMap[foods[i].ID] = &foods[i]
	}

	// Fetch Nutrients for these foods
	// Simple approach: SELECT * FROM food_nutrients WHERE food_id IN ( ... )
	// But building IN clause is tedious.
	// Alternative: JOIN with the same WHERE clause as foods.
	nuQuery := `
		SELECT fn.food_id, fn.name, fn.amount, fn.unit
		FROM food_nutrients fn
		JOIN foods f ON f.id = fn.food_id
	`
	if onlyCurrent {
		nuQuery += " WHERE f.is_current = 1 AND f.deleted_at IS NULL"
	} else {
		nuQuery += " WHERE f.deleted_at IS NULL"
	}

	nuRows, err := DB.QueryContext(ctx, nuQuery)
	if err != nil {
		return nil, err
	}
	defer nuRows.Close()

	for nuRows.Next() {
		var foodID string
		var n Nutrient
		if err := nuRows.Scan(&foodID, &n.Name, &n.Amount, &n.Unit); err != nil {
			return nil, err
		}
		if f, ok := foodMap[foodID]; ok {
			f.Nutrients = append(f.Nutrients, n)
		}
	}

	return foods, nil
}

func GetFood(ctx context.Context, id string) (*Food, error) {
	row := DB.QueryRowContext(ctx, "SELECT id, family_id, version, is_current, name, calories, protein, carbs, fat, type, measurement_unit, measurement_amount, created_at FROM foods WHERE id = ?", id)
	var f Food
	err := row.Scan(&f.ID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name, &f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type, &f.MeasurementUnit, &f.MeasurementAmount, &f.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Fetch Nutrients
	nuRows, err := DB.QueryContext(ctx, "SELECT name, amount, unit FROM food_nutrients WHERE food_id = ?", f.ID)
	if err != nil {
		return nil, err
	}
	defer nuRows.Close()

	for nuRows.Next() {
		var n Nutrient
		if err := nuRows.Scan(&n.Name, &n.Amount, &n.Unit); err != nil {
			return nil, err
		}
		f.Nutrients = append(f.Nutrients, n)
	}

	return &f, nil
}

// AddRecipeItems adds ingredients to a recipe food
func AddRecipeItems(ctx context.Context, recipeID string, items map[string]float64) error {
	// items: food_id -> amount
	for foodID, amount := range items {
		_, err := DB.ExecContext(ctx, "INSERT INTO recipe_items (recipe_id, ingredient_id, amount) VALUES (?, ?, ?)", recipeID, foodID, amount)
		if err != nil {
			// clean up?
			return err
		}
	}
	return nil
}

func GetRecipeItems(ctx context.Context, recipeID string) ([]struct {
	Food   Food
	Amount float64
}, error) {
	rows, err := DB.QueryContext(ctx, `
		SELECT ingredient_id, ingredient_name, ingredient_amount 
		FROM recipe_details
		WHERE recipe_id = ?
	`, recipeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []struct {
		Food   Food
		Amount float64
	}
	for rows.Next() {
		var r struct {
			Food   Food
			Amount float64
		}
		// Only scanning basics for now
		if err := rows.Scan(&r.Food.ID, &r.Food.Name, &r.Amount); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}
func DeleteFood(ctx context.Context, id string) error {
	// Soft delete the specific version?
	// Or soft delete the entire family?
	// Usually delete "banana" means delete the whole family.
	// But `id` is a version ID.
	// Let's look up the family ID and delete all versions (or mark them deleted).
	// Or just mark this version?
	// If I delete "Banana v1" but "Banana v2" exists, what happens?
	// User intent: "Delete this food". Usually implies "Delete the Food Concept".
	// Let's delete the FAMILY.

	// 1. Get Family ID
	var familyID string
	err := DB.QueryRowContext(ctx, "SELECT family_id FROM foods WHERE id = ?", id).Scan(&familyID)
	if err != nil {
		return err
	}

	// 2. Mark all as deleted
	_, err = DB.ExecContext(ctx, "UPDATE foods SET deleted_at = ? WHERE family_id = ?", time.Now(), familyID)
	return err
}
