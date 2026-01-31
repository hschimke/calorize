package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func GetFoods(userID UserID) ([]Food, error) {
	query := `
		SELECT 
			id, creator_id, family_id, version, is_current, name, 
			calories, protein, carbs, fat, type, 
			measurement_unit, measurement_amount, public, created_at, deleted_at 
		FROM foods 
		WHERE (creator_id = ? OR public = true) AND type = 'food' AND is_current = true AND deleted_at IS NULL
	`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing foods: %w", err)
	}
	defer rows.Close()

	var foods []Food
	for rows.Next() {
		var f Food
		err := rows.Scan(
			&f.ID, &f.CreatorID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name,
			&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type,
			&f.MeasurementUnit, &f.MeasurementAmount, &f.Public, &f.CreatedAt, &f.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning food: %w", err)
		}
		foods = append(foods, f)
	}
	return foods, nil
}

func GetFood(id FoodID) (*Food, error) {
	query := `
		SELECT 
			id, creator_id, family_id, version, is_current, name, 
			calories, protein, carbs, fat, type, 
			measurement_unit, measurement_amount, public, created_at, deleted_at 
		FROM foods 
		WHERE id = ? AND type = 'food'
	`
	row := db.QueryRow(query, id)

	var f Food
	err := row.Scan(
		&f.ID, &f.CreatorID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name,
		&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type,
		&f.MeasurementUnit, &f.MeasurementAmount, &f.Public, &f.CreatedAt, &f.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Or specific error
		}
		return nil, fmt.Errorf("getting food: %w", err)
	}

	// Fetch nutrients
	nutrientsQuery := `
		SELECT food_id, name, amount, unit
		FROM food_nutrients
		WHERE food_id = ?
	`
	nRows, err := db.Query(nutrientsQuery, f.ID)
	if err != nil {
		return nil, fmt.Errorf("getting food nutrients: %w", err)
	}
	defer nRows.Close()

	for nRows.Next() {
		var n FoodNutrient
		if err := nRows.Scan(&n.FoodID, &n.Name, &n.Amount, &n.Unit); err != nil {
			return nil, fmt.Errorf("scanning nutrient: %w", err)
		}
		f.Nutrients = append(f.Nutrients, n)
	}

	return &f, nil
}

func GetFoodVersions(id FoodID) ([]Food, error) {
	var familyID FoodFamilyID
	err := db.QueryRow("SELECT family_id FROM foods WHERE id = ?", id).Scan(&familyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting food family: %w", err)
	}

	query := `
		SELECT 
			id, creator_id, family_id, version, is_current, name, 
			calories, protein, carbs, fat, type, 
			measurement_unit, measurement_amount, public, created_at, deleted_at 
		FROM foods 
		WHERE family_id = ? AND type = 'food' AND deleted_at IS NULL
		ORDER BY version DESC
	`
	rows, err := db.Query(query, familyID)
	if err != nil {
		return nil, fmt.Errorf("listing food versions: %w", err)
	}
	defer rows.Close()

	var versions []Food
	for rows.Next() {
		var f Food
		err := rows.Scan(
			&f.ID, &f.CreatorID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name,
			&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type,
			&f.MeasurementUnit, &f.MeasurementAmount, &f.Public, &f.CreatedAt, &f.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning food version: %w", err)
		}
		versions = append(versions, f)
	}
	return versions, nil
}

func CreateFood(food Food) (*Food, error) {
	food.Type = "food" // Enforce type
	if food.ID == FoodID(uuid.Nil) {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generating id: %w", err)
		}
		food.ID = FoodID(id)
	}
	if food.FamilyID == FoodFamilyID(uuid.Nil) {
		food.FamilyID = FoodFamilyID(food.ID)
	}
	food.Version = 1
	food.IsCurrent = true
	if food.CreatedAt.IsZero() {
		food.CreatedAt = time.Now()
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO foods (
			id, creator_id, family_id, version, is_current, name, 
			calories, protein, carbs, fat, type, 
			measurement_unit, measurement_amount, public, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = tx.Exec(query,
		food.ID, food.CreatorID, food.FamilyID, food.Version, food.IsCurrent, food.Name,
		food.Calories, food.Protein, food.Carbs, food.Fat, food.Type,
		food.MeasurementUnit, food.MeasurementAmount, food.Public, food.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting food: %w", err)
	}

	// Insert nutrients
	stmt, err := tx.Prepare("INSERT INTO food_nutrients (food_id, name, amount, unit) VALUES (?, ?, ?, ?)")
	if err != nil {
		return nil, fmt.Errorf("preparing nutrients stmt: %w", err)
	}
	defer stmt.Close()

	for _, n := range food.Nutrients {
		if _, err := stmt.Exec(food.ID, n.Name, n.Amount, n.Unit); err != nil {
			return nil, fmt.Errorf("inserting nutrient: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing food: %w", err)
	}

	return &food, nil
}

func UpdateFood(id FoodID, food Food) (*Food, error) {
	current, err := GetFood(id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("food not found")
	}

	newID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generating new id: %w", err)
	}
	food.ID = FoodID(newID)
	food.FamilyID = current.FamilyID
	food.Version = current.Version + 1
	food.IsCurrent = true
	food.Type = "food"
	if food.CreatedAt.IsZero() {
		food.CreatedAt = time.Now()
	}
	// Keep creator unless specified
	if food.CreatorID == UserID(uuid.Nil) {
		food.CreatorID = current.CreatorID
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("UPDATE foods SET is_current = false WHERE id = ?", current.ID)
	if err != nil {
		return nil, fmt.Errorf("deprecating old version: %w", err)
	}

	query := `
		INSERT INTO foods (
			id, creator_id, family_id, version, is_current, name, 
			calories, protein, carbs, fat, type, 
			measurement_unit, measurement_amount, public, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = tx.Exec(query,
		food.ID, food.CreatorID, food.FamilyID, food.Version, food.IsCurrent, food.Name,
		food.Calories, food.Protein, food.Carbs, food.Fat, food.Type,
		food.MeasurementUnit, food.MeasurementAmount, food.Public, food.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting new food version: %w", err)
	}

	stmt, err := tx.Prepare("INSERT INTO food_nutrients (food_id, name, amount, unit) VALUES (?, ?, ?, ?)")
	if err != nil {
		return nil, fmt.Errorf("preparing nutrients stmt: %w", err)
	}
	defer stmt.Close()

	for _, n := range food.Nutrients {
		if _, err := stmt.Exec(food.ID, n.Name, n.Amount, n.Unit); err != nil {
			return nil, fmt.Errorf("inserting nutrient: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing update: %w", err)
	}

	return &food, nil
}

func DeleteFood(id FoodID) error {
	var familyID FoodFamilyID
	err := db.QueryRow("SELECT family_id FROM foods WHERE id = ?", id).Scan(&familyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("finding food to delete: %w", err)
	}

	_, err = db.Exec("UPDATE foods SET deleted_at = ? WHERE family_id = ?", time.Now(), familyID)
	if err != nil {
		return fmt.Errorf("deleting food family: %w", err)
	}

	return nil
}
