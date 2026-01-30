package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func ListRecipes(userId UserID) ([]Food, error) {
	query := `
		SELECT 
			id, creator_id, family_id, version, is_current, name, 
			calories, protein, carbs, fat, type, 
			measurement_unit, measurement_amount, public, created_at, deleted_at 
		FROM foods 
		WHERE creator_id = ? AND type = 'recipe' AND is_current = true AND deleted_at IS NULL
	`
	rows, err := db.Query(query, userId)
	if err != nil {
		return nil, fmt.Errorf("listing recipes: %w", err)
	}
	defer rows.Close()

	var recipes []Food
	for rows.Next() {
		var r Food
		err := rows.Scan(
			&r.ID, &r.CreatorID, &r.FamilyID, &r.Version, &r.IsCurrent, &r.Name,
			&r.Calories, &r.Protein, &r.Carbs, &r.Fat, &r.Type,
			&r.MeasurementUnit, &r.MeasurementAmount, &r.Public, &r.CreatedAt, &r.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning recipe: %w", err)
		}
		recipes = append(recipes, r)
	}
	return recipes, nil
}

func GetRecipe(id RecipeID) (*Food, error) {
	query := `
		SELECT 
			id, creator_id, family_id, version, is_current, name, 
			calories, protein, carbs, fat, type, 
			measurement_unit, measurement_amount, public, created_at, deleted_at 
		FROM foods 
		WHERE id = ? AND type = 'recipe'
	`
	row := db.QueryRow(query, id)

	var r Food
	err := row.Scan(
		&r.ID, &r.CreatorID, &r.FamilyID, &r.Version, &r.IsCurrent, &r.Name,
		&r.Calories, &r.Protein, &r.Carbs, &r.Fat, &r.Type,
		&r.MeasurementUnit, &r.MeasurementAmount, &r.Public, &r.CreatedAt, &r.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Or specific error
		}
		return nil, fmt.Errorf("getting recipe: %w", err)
	}

	// Fetch ingredients
	ingredientsQuery := `
		SELECT recipe_id, ingredient_id, amount
		FROM recipe_items
		WHERE recipe_id = ?
	`
	iRows, err := db.Query(ingredientsQuery, r.ID)
	if err != nil {
		return nil, fmt.Errorf("getting recipe ingredients: %w", err)
	}
	defer iRows.Close()

	for iRows.Next() {
		var item RecipeItems
		if err := iRows.Scan(&item.RecipeID, &item.IngredientID, &item.Amount); err != nil {
			return nil, fmt.Errorf("scanning ingredient: %w", err)
		}
		r.Ingredients = append(r.Ingredients, item)
	}

	return &r, nil
}

func GetRecipeVersions(id RecipeID) ([]Food, error) {
	// First get the family_id from the passed id
	var familyID FoodFamilyID
	err := db.QueryRow("SELECT family_id FROM foods WHERE id = ?", id).Scan(&familyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting recipe family: %w", err)
	}

	query := `
		SELECT 
			id, creator_id, family_id, version, is_current, name, 
			calories, protein, carbs, fat, type, 
			measurement_unit, measurement_amount, public, created_at, deleted_at 
		FROM foods 
		WHERE family_id = ? AND type = 'recipe' AND deleted_at IS NULL
		ORDER BY version DESC
	`
	rows, err := db.Query(query, familyID)
	if err != nil {
		return nil, fmt.Errorf("listing recipe versions: %w", err)
	}
	defer rows.Close()

	var versions []Food
	for rows.Next() {
		var r Food
		err := rows.Scan(
			&r.ID, &r.CreatorID, &r.FamilyID, &r.Version, &r.IsCurrent, &r.Name,
			&r.Calories, &r.Protein, &r.Carbs, &r.Fat, &r.Type,
			&r.MeasurementUnit, &r.MeasurementAmount, &r.Public, &r.CreatedAt, &r.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning recipe version: %w", err)
		}
		versions = append(versions, r)
	}
	return versions, nil
}

func CreateRecipe(recipe Food) (*Food, error) {
	recipe.Type = "recipe" // Enforce type
	if recipe.ID == FoodID(uuid.Nil) {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generating id: %w", err)
		}
		recipe.ID = FoodID(id)
	}
	if recipe.FamilyID == FoodFamilyID(uuid.Nil) {
		// New recipe, new family
		recipe.FamilyID = FoodFamilyID(recipe.ID)
	}
	recipe.Version = 1
	recipe.IsCurrent = true
	if recipe.CreatedAt.IsZero() {
		recipe.CreatedAt = time.Now()
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
		recipe.ID, recipe.CreatorID, recipe.FamilyID, recipe.Version, recipe.IsCurrent, recipe.Name,
		recipe.Calories, recipe.Protein, recipe.Carbs, recipe.Fat, recipe.Type,
		recipe.MeasurementUnit, recipe.MeasurementAmount, recipe.Public, recipe.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting recipe: %w", err)
	}

	// Insert ingredients
	stmt, err := tx.Prepare("INSERT INTO recipe_items (recipe_id, ingredient_id, amount) VALUES (?, ?, ?)")
	if err != nil {
		return nil, fmt.Errorf("preparing ingredients stmt: %w", err)
	}
	defer stmt.Close()

	for _, item := range recipe.Ingredients {
		if _, err := stmt.Exec(recipe.ID, item.IngredientID, item.Amount); err != nil {
			return nil, fmt.Errorf("inserting ingredient: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing recipe: %w", err)
	}

	return &recipe, nil
}

func UpdateRecipe(id RecipeID, recipe Food) (*Food, error) {
	// 1. Fetch current version to get family_id and verify existence
	current, err := GetRecipe(id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("recipe not found")
	}

	// 2. Prepare new version
	newID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generating new id: %w", err)
	}
	recipe.ID = FoodID(newID)
	recipe.FamilyID = current.FamilyID
	recipe.Version = current.Version + 1
	recipe.IsCurrent = true
	recipe.Type = "recipe"
	if recipe.CreatedAt.IsZero() {
		recipe.CreatedAt = time.Now()
	}
	// Keep creator unless specified (usually stays same)
	if recipe.CreatorID == UserID(uuid.Nil) {
		recipe.CreatorID = current.CreatorID
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// 3. Set old version is_current = false
	_, err = tx.Exec("UPDATE foods SET is_current = false WHERE id = ?", current.ID)
	if err != nil {
		return nil, fmt.Errorf("deprecating old version: %w", err)
	}

	// 4. Insert new version
	query := `
		INSERT INTO foods (
			id, creator_id, family_id, version, is_current, name, 
			calories, protein, carbs, fat, type, 
			measurement_unit, measurement_amount, public, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = tx.Exec(query,
		recipe.ID, recipe.CreatorID, recipe.FamilyID, recipe.Version, recipe.IsCurrent, recipe.Name,
		recipe.Calories, recipe.Protein, recipe.Carbs, recipe.Fat, recipe.Type,
		recipe.MeasurementUnit, recipe.MeasurementAmount, recipe.Public, recipe.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting new recipe version: %w", err)
	}

	// 5. Insert ingredients
	stmt, err := tx.Prepare("INSERT INTO recipe_items (recipe_id, ingredient_id, amount) VALUES (?, ?, ?)")
	if err != nil {
		return nil, fmt.Errorf("preparing ingredients stmt: %w", err)
	}
	defer stmt.Close()

	for _, item := range recipe.Ingredients {
		if _, err := stmt.Exec(recipe.ID, item.IngredientID, item.Amount); err != nil {
			return nil, fmt.Errorf("inserting ingredient: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing update: %w", err)
	}

	return &recipe, nil
}

func DeleteRecipe(id RecipeID) error {
	// Soft delete the entire family?
	// The prompt implies deleting "a recipe".
	// Let's assume finding the family and deleting all versions, or just marking the current one deleted?
	// Standard practice for "Delete" in this context is likely to hide it from lists.
	// If we just delete the current version, the previous version becomes "current"? No, that's rollback.
	// Let's soft-delete all foods with the same family_id.

	var familyID FoodFamilyID
	err := db.QueryRow("SELECT family_id FROM foods WHERE id = ?", id).Scan(&familyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // Already gone
		}
		return fmt.Errorf("finding recipe to delete: %w", err)
	}

	_, err = db.Exec("UPDATE foods SET deleted_at = ? WHERE family_id = ?", time.Now(), familyID)
	if err != nil {
		return fmt.Errorf("deleting recipe family: %w", err)
	}

	return nil
}
