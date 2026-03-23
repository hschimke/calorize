package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func GetFoods(userID UserID) ([]Food, error) {
	query := `
		SELECT
			id, creator_id, family_id, version, is_current, name,
			calories, protein, carbs, fat, type,
			measurement_unit, measurement_amount, servings, public, external_id, created_at, deleted_at
		FROM foods
		WHERE creator_id = ? AND is_current = true AND deleted_at IS NULL
		UNION
		SELECT
			id, creator_id, family_id, version, is_current, name,
			calories, protein, carbs, fat, type,
			measurement_unit, measurement_amount, servings, public, external_id, created_at, deleted_at
		FROM foods
		WHERE public = true AND is_current = true AND deleted_at IS NULL
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
			&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.ExternalID, &f.CreatedAt, &f.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning food: %w", err)
		}
		foods = append(foods, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating foods: %w", err)
	}
	return foods, nil
}

func GetFood(id FoodID) (*Food, error) {
	query := `
		SELECT
			id, creator_id, family_id, version, is_current, name,
			calories, protein, carbs, fat, type,
			measurement_unit, measurement_amount, servings, public, external_id, created_at, deleted_at
		FROM foods
		WHERE id = ? AND deleted_at IS NULL
	`
	row := db.QueryRow(query, id)

	var f Food
	err := row.Scan(
		&f.ID, &f.CreatorID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name,
		&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type,
		&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.ExternalID, &f.CreatedAt, &f.DeletedAt,
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
	if err := nRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating nutrients: %w", err)
	}

	// Fetch ingredients
	ingredientsQuery := `
		SELECT recipe_id, ingredient_id, amount
		FROM recipe_items
		WHERE recipe_id = ?
	`
	iRows, err := db.Query(ingredientsQuery, f.ID)
	if err != nil {
		return nil, fmt.Errorf("getting food ingredients: %w", err)
	}
	defer iRows.Close()

	for iRows.Next() {
		var i RecipeItems
		if err := iRows.Scan(&i.RecipeID, &i.IngredientID, &i.Amount); err != nil {
			return nil, fmt.Errorf("scanning ingredient: %w", err)
		}
		f.Ingredients = append(f.Ingredients, i)
	}
	if err := iRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating ingredients: %w", err)
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
			measurement_unit, measurement_amount, servings, public, external_id, created_at, deleted_at
		FROM foods
		WHERE family_id = ? AND deleted_at IS NULL
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
			&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.ExternalID, &f.CreatedAt, &f.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning food version: %w", err)
		}
		versions = append(versions, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating food versions: %w", err)
	}
	return versions, nil
}

func insertFoodData(tx *sql.Tx, food *Food) error {
	query := `
		INSERT INTO foods (
			id, creator_id, family_id, version, is_current, name,
			calories, protein, carbs, fat, type,
			measurement_unit, measurement_amount, servings, public, external_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(query,
		food.ID, food.CreatorID, food.FamilyID, food.Version, food.IsCurrent, food.Name,
		food.Calories, food.Protein, food.Carbs, food.Fat, food.Type,
		food.MeasurementUnit, food.MeasurementAmount, food.Servings, food.Public, food.ExternalID, food.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting food: %w", err)
	}

	// Insert nutrients
	stmt, err := tx.Prepare("INSERT INTO food_nutrients (food_id, name, amount, unit) VALUES (?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("preparing nutrients stmt: %w", err)
	}
	defer stmt.Close()

	for _, n := range food.Nutrients {
		if _, err := stmt.Exec(food.ID, n.Name, n.Amount, n.Unit); err != nil {
			return fmt.Errorf("inserting nutrient: %w", err)
		}
	}

	// Insert ingredients
	if len(food.Ingredients) > 0 {
		istmt, err := tx.Prepare("INSERT INTO recipe_items (recipe_id, ingredient_id, amount) VALUES (?, ?, ?)")
		if err != nil {
			return fmt.Errorf("preparing ingredients stmt: %w", err)
		}
		defer istmt.Close()

		for _, i := range food.Ingredients {
			if _, err := istmt.Exec(food.ID, i.IngredientID, i.Amount); err != nil {
				return fmt.Errorf("inserting ingredient: %w", err)
			}
		}
	}

	return nil
}

func CreateFood(food Food) (*Food, error) {
	if len(food.Ingredients) > 0 {
		food.Type = "recipe"
	} else if food.Type == "" {
		food.Type = "food"
	}
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
	if food.Servings == 0 {
		food.Servings = 1
	}
	if food.CreatedAt.IsZero() {
		food.CreatedAt = time.Now().UTC()
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	if err := insertFoodData(tx, &food); err != nil {
		return nil, err
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
	if food.Servings == 0 {
		food.Servings = 1
	}
	if len(food.Ingredients) > 0 {
		food.Type = "recipe"
	} else if food.Type == "" {
		food.Type = "food"
	}
	if food.CreatedAt.IsZero() {
		food.CreatedAt = time.Now().UTC()
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

	_, err = tx.Exec("UPDATE foods SET is_current = false WHERE family_id = ?", current.FamilyID)
	if err != nil {
		return nil, fmt.Errorf("deprecating old version: %w", err)
	}

	if err := insertFoodData(tx, &food); err != nil {
		return nil, fmt.Errorf("inserting new food version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing update: %w", err)
	}

	return &food, nil
}

func GetRecentFoods(userID UserID, limit int) ([]Food, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT f.id, f.creator_id, f.family_id, f.version, f.is_current, f.name,
		       f.calories, f.protein, f.carbs, f.fat, f.type,
		       f.measurement_unit, f.measurement_amount, f.servings, f.public, f.created_at, f.deleted_at
		FROM foods f
		INNER JOIN (
			SELECT food_id, MAX(logged_at) AS last_used
			FROM food_log_entries
			WHERE user_id = ? AND food_id IS NOT NULL AND deleted_at IS NULL
			GROUP BY food_id
			ORDER BY last_used DESC
			LIMIT ?
		) recent ON f.id = recent.food_id
		WHERE f.is_current = true AND f.deleted_at IS NULL
		ORDER BY recent.last_used DESC
	`
	rows, err := db.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("getting recent foods: %w", err)
	}
	defer rows.Close()

	var foods []Food
	for rows.Next() {
		var f Food
		err := rows.Scan(
			&f.ID, &f.CreatorID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name,
			&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type,
			&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.CreatedAt, &f.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning recent food: %w", err)
		}
		foods = append(foods, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating recent foods: %w", err)
	}
	if foods == nil {
		foods = []Food{}
	}
	return foods, nil
}

func SearchFoods(userID UserID, q string, limit int) ([]Food, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `
		SELECT id, creator_id, family_id, version, is_current, name,
		       calories, protein, carbs, fat, type,
		       measurement_unit, measurement_amount, servings, public, created_at, deleted_at
		FROM (
			SELECT id, creator_id, family_id, version, is_current, name,
			       calories, protein, carbs, fat, type,
			       measurement_unit, measurement_amount, servings, public, created_at, deleted_at
			FROM foods
			WHERE creator_id = ? AND is_current = true AND deleted_at IS NULL
			  AND name LIKE ? ESCAPE ?
			LIMIT ?
		)
		UNION
		SELECT id, creator_id, family_id, version, is_current, name,
		       calories, protein, carbs, fat, type,
		       measurement_unit, measurement_amount, servings, public, created_at, deleted_at
		FROM (
			SELECT id, creator_id, family_id, version, is_current, name,
			       calories, protein, carbs, fat, type,
			       measurement_unit, measurement_amount, servings, public, created_at, deleted_at
			FROM foods
			WHERE public = true AND is_current = true AND deleted_at IS NULL
			  AND name LIKE ? ESCAPE ?
			LIMIT ?
		)
	`
	escaped := strings.ReplaceAll(q, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `%`, `\%`)
	escaped = strings.ReplaceAll(escaped, `_`, `\_`)
	pattern := escaped + "%"
	const escChar = `\`
	rows, err := db.Query(query, userID, pattern, escChar, limit, pattern, escChar, limit)
	if err != nil {
		return nil, fmt.Errorf("searching foods: %w", err)
	}
	defer rows.Close()

	var foods []Food
	for rows.Next() {
		var f Food
		err := rows.Scan(
			&f.ID, &f.CreatorID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name,
			&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type,
			&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.CreatedAt, &f.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning food search result: %w", err)
		}
		foods = append(foods, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating food search results: %w", err)
	}
	if foods == nil {
		foods = []Food{}
	}
	return foods, nil
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

	_, err = db.Exec("UPDATE foods SET deleted_at = ? WHERE family_id = ?", time.Now().UTC(), familyID)
	if err != nil {
		return fmt.Errorf("deleting food family: %w", err)
	}

	return nil
}

func GetFoodByExternalID(extID string) (*Food, error) {
	query := `
		SELECT
			id, creator_id, family_id, version, is_current, name,
			calories, protein, carbs, fat, type,
			measurement_unit, measurement_amount, servings, public, external_id, created_at, deleted_at
		FROM foods
		WHERE external_id = ? AND is_current = true AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`
	row := db.QueryRow(query, extID)

	var f Food
	err := row.Scan(
		&f.ID, &f.CreatorID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name,
		&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type,
		&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.ExternalID, &f.CreatedAt, &f.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Or specific error
		}
		return nil, fmt.Errorf("getting food by external id: %w", err)
	}
	return &f, nil
}
