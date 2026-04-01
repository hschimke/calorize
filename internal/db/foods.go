package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GetUserFoods returns only foods created by the given user (no public/FDC foods).
func GetUserFoods(userID UserID) ([]Food, error) {
	query := `
		SELECT
			id, creator_id, family_id, version, is_current, name,
			calories, protein, carbs, fat, type,
			measurement_unit, measurement_amount, servings, public, external_id,
			brand_owner, barcode, ingredients_text, category, created_at, deleted_at
		FROM foods
		WHERE creator_id = ? AND is_current = true AND deleted_at IS NULL
	`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing user foods: %w", err)
	}
	defer rows.Close()

	var foods []Food
	for rows.Next() {
		var f Food
		err := rows.Scan(
			&f.ID, &f.CreatorID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name,
			&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type,
			&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.ExternalID,
			&f.BrandOwner, &f.Barcode, &f.IngredientsText, &f.Category, &f.CreatedAt, &f.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning food: %w", err)
		}
		foods = append(foods, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating user foods: %w", err)
	}
	if foods == nil {
		foods = []Food{}
	}
	return foods, nil
}

// buildSourceFilter returns an additional AND clause and args to filter the public-foods
// branch of a query. disabledSources must already be validated against GetAvailableSources —
// the values are used as LIKE pattern prefixes and are never interpolated into SQL, but
// callers are responsible for ensuring they are known source keys (e.g. "afcd", "fdc", "off").
func buildSourceFilter(disabledSources []string, hidePublicUserFoods bool) (string, []any) {
	var sb strings.Builder
	var args []any
	if hidePublicUserFoods {
		sb.WriteString(" AND creator_id IS NULL")
	}
	for _, source := range disabledSources {
		sb.WriteString(" AND external_id NOT LIKE ? ESCAPE ?")
		args = append(args, source+"_%", `\`)
	}
	return sb.String(), args
}

func GetFoods(userID UserID, disabledSources []string, hidePublicUserFoods bool) ([]Food, error) {
	sourceClause, sourceArgs := buildSourceFilter(disabledSources, hidePublicUserFoods)

	query := `
		SELECT
			id, creator_id, family_id, version, is_current, name,
			calories, protein, carbs, fat, type,
			measurement_unit, measurement_amount, servings, public, external_id,
			brand_owner, barcode, ingredients_text, category, created_at, deleted_at
		FROM foods
		WHERE creator_id = ? AND is_current = true AND deleted_at IS NULL
		UNION ALL
		SELECT
			id, creator_id, family_id, version, is_current, name,
			calories, protein, carbs, fat, type,
			measurement_unit, measurement_amount, servings, public, external_id,
			brand_owner, barcode, ingredients_text, category, created_at, deleted_at
		FROM foods
		WHERE public = true AND is_current = true AND deleted_at IS NULL
		  AND (creator_id != ? OR creator_id IS NULL)` + sourceClause

	args := append([]any{userID, userID}, sourceArgs...)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing foods: %w", err)
	}
	defer rows.Close()

	foods := []Food{}
	for rows.Next() {
		var f Food
		err := rows.Scan(
			&f.ID, &f.CreatorID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name,
			&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type,
			&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.ExternalID,
			&f.BrandOwner, &f.Barcode, &f.IngredientsText, &f.Category, &f.CreatedAt, &f.DeletedAt,
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

// knownSources is the fixed set of import source prefixes the application recognises.
var knownSources = []string{"afcd", "fdc", "off"}

// GetAvailableSources returns the subset of knownSources that have at least one active
// food in the database. Each source is checked with an explicit range bound
// (external_id >= 'src_' AND external_id < 'src'+1) so SQLite can always use a
// SEARCH on idx_foods_external_id rather than a full table scan. (LIKE-based patterns
// do not trigger the index on this database.)
func GetAvailableSources() ([]string, error) {
	sources := []string{}
	for _, source := range knownSources {
		lo := source + "_"
		// Upper bound: increment the last byte of the source name to form a tight range,
		// e.g. "afcd" → "afce", "fdc" → "fdd", "off" → "ofg".
		hi := source[:len(source)-1] + string(source[len(source)-1]+1)

		var exists bool
		err := db.QueryRow(
			`SELECT EXISTS(
				SELECT 1 FROM foods
				WHERE external_id >= ? AND external_id < ?
				  AND is_current = 1 AND deleted_at IS NULL
				LIMIT 1
			)`, lo, hi,
		).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("checking source %s: %w", source, err)
		}
		if exists {
			sources = append(sources, source)
		}
	}
	return sources, nil
}

// GetFoodsByIDs batch-fetches a set of foods and their relations avoiding the N+1 problem.
func GetFoodsByIDs(ids []FoodID) (map[FoodID]*Food, error) {
	if len(ids) == 0 {
		return make(map[FoodID]*Food), nil
	}

	args := make([]any, len(ids))
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		args[i] = id
		placeholders[i] = "?"
	}
	inClause := strings.Join(placeholders, ",")

	query := fmt.Sprintf(`
		SELECT
			id, creator_id, family_id, version, is_current, name,
			calories, protein, carbs, fat, type,
			measurement_unit, measurement_amount, servings, public, external_id,
			brand_owner, barcode, ingredients_text, category, created_at, deleted_at
		FROM foods
		WHERE id IN (%s)
	`, inClause)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("getting foods by ids: %w", err)
	}
	defer rows.Close()

	foodMap := make(map[FoodID]*Food)
	for rows.Next() {
		var f Food
		err := rows.Scan(
			&f.ID, &f.CreatorID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name,
			&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type,
			&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.ExternalID,
			&f.BrandOwner, &f.Barcode, &f.IngredientsText, &f.Category, &f.CreatedAt, &f.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning food: %w", err)
		}
		foodMap[f.ID] = &f
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating foods: %w", err)
	}

	if len(foodMap) == 0 {
		return foodMap, nil
	}

	// Fetch nutrients
	nutrientsQuery := fmt.Sprintf(`
		SELECT food_id, name, amount, unit
		FROM food_nutrients
		WHERE food_id IN (%s)
	`, inClause)
	nRows, err := db.Query(nutrientsQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("getting food nutrients by ids: %w", err)
	}
	defer nRows.Close()

	for nRows.Next() {
		var n FoodNutrient
		if err := nRows.Scan(&n.FoodID, &n.Name, &n.Amount, &n.Unit); err != nil {
			return nil, fmt.Errorf("scanning nutrient: %w", err)
		}
		if f := foodMap[n.FoodID]; f != nil {
			f.Nutrients = append(f.Nutrients, n)
		}
	}
	if err := nRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating nutrients: %w", err)
	}

	// Fetch portions
	portionsQuery := fmt.Sprintf(`SELECT food_id, name, amount, unit, gram_weight FROM food_portions WHERE food_id IN (%s)`, inClause)
	pRows, err := db.Query(portionsQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("getting food portions by ids: %w", err)
	}
	defer pRows.Close()

	for pRows.Next() {
		var p FoodPortion
		if err := pRows.Scan(&p.FoodID, &p.Name, &p.Amount, &p.Unit, &p.GramWeight); err != nil {
			return nil, fmt.Errorf("scanning portion: %w", err)
		}
		if f := foodMap[p.FoodID]; f != nil {
			f.Portions = append(f.Portions, p)
		}
	}
	if err := pRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating portions: %w", err)
	}

	// Fetch ingredients
	ingredientsQuery := fmt.Sprintf(`SELECT recipe_id, ingredient_id, amount FROM recipe_items WHERE recipe_id IN (%s)`, inClause)
	iRows, err := db.Query(ingredientsQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("getting recipe items by ids: %w", err)
	}
	defer iRows.Close()

	for iRows.Next() {
		var i RecipeItems
		if err := iRows.Scan(&i.RecipeID, &i.IngredientID, &i.Amount); err != nil {
			return nil, fmt.Errorf("scanning recipe item: %w", err)
		}
		if f := foodMap[FoodID(i.RecipeID)]; f != nil {
			f.Ingredients = append(f.Ingredients, i)
		}
	}
	if err := iRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating recipe items: %w", err)
	}

	return foodMap, nil
}

func GetFood(id FoodID) (*Food, error) {
	query := `
		SELECT
			id, creator_id, family_id, version, is_current, name,
			calories, protein, carbs, fat, type,
			measurement_unit, measurement_amount, servings, public, external_id,
			brand_owner, barcode, ingredients_text, category, created_at, deleted_at
		FROM foods
		WHERE id = ? AND deleted_at IS NULL
	`
	row := db.QueryRow(query, id)

	var f Food
	err := row.Scan(
		&f.ID, &f.CreatorID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name,
		&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type,
		&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.ExternalID,
		&f.BrandOwner, &f.Barcode, &f.IngredientsText, &f.Category, &f.CreatedAt, &f.DeletedAt,
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

	// Fetch portions
	var portionErr error
	f.Portions, portionErr = fetchPortionsForFood(f.ID)
	if portionErr != nil {
		return nil, portionErr
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
			measurement_unit, measurement_amount, servings, public, external_id,
			brand_owner, barcode, ingredients_text, category, created_at, deleted_at
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
			&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.ExternalID,
			&f.BrandOwner, &f.Barcode, &f.IngredientsText, &f.Category, &f.CreatedAt, &f.DeletedAt,
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
			measurement_unit, measurement_amount, servings, public, external_id,
			brand_owner, barcode, ingredients_text, category, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(query,
		food.ID, food.CreatorID, food.FamilyID, food.Version, food.IsCurrent, food.Name,
		food.Calories, food.Protein, food.Carbs, food.Fat, food.Type,
		food.MeasurementUnit, food.MeasurementAmount, food.Servings, food.Public, food.ExternalID,
		food.BrandOwner, food.Barcode, food.IngredientsText, food.Category, food.CreatedAt,
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

	// Insert portions
	pstmt, err := tx.Prepare("INSERT INTO food_portions (food_id, name, amount, unit, gram_weight) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("preparing portions stmt: %w", err)
	}
	defer pstmt.Close()

	for _, p := range food.Portions {
		if _, err := pstmt.Exec(food.ID, p.Name, p.Amount, p.Unit, p.GramWeight); err != nil {
			return fmt.Errorf("inserting portion: %w", err)
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
		       f.measurement_unit, f.measurement_amount, f.servings, f.public, f.external_id,
		       f.brand_owner, f.barcode, f.ingredients_text, f.category, f.created_at, f.deleted_at
		FROM foods f
		INNER JOIN (
			SELECT f2.family_id, MAX(l.logged_at) AS last_used
			FROM food_log_entries l
			INNER JOIN foods f2 ON l.food_id = f2.id
			WHERE l.user_id = ? AND l.food_id IS NOT NULL AND l.deleted_at IS NULL
			GROUP BY f2.family_id
			ORDER BY last_used DESC
			LIMIT ?
		) recent ON f.family_id = recent.family_id
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
			&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.ExternalID,
			&f.BrandOwner, &f.Barcode, &f.IngredientsText, &f.Category, &f.CreatedAt, &f.DeletedAt,
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

func SearchFoods(userID UserID, q string, limit int, disabledSources []string, hidePublicUserFoods bool) ([]Food, error) {
	if limit <= 0 {
		limit = 20
	}
	escaped := strings.ReplaceAll(q, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `%`, `\%`)
	escaped = strings.ReplaceAll(escaped, `_`, `\_`)
	pattern := escaped + "%"
	const escChar = `\`

	sourceClause, sourceArgs := buildSourceFilter(disabledSources, hidePublicUserFoods)

	query := `
		SELECT id, creator_id, family_id, version, is_current, name,
		       calories, protein, carbs, fat, type,
		       measurement_unit, measurement_amount, servings, public, external_id,
		       brand_owner, barcode, ingredients_text, category, created_at, deleted_at
		FROM (
			SELECT id, creator_id, family_id, version, is_current, name,
			       calories, protein, carbs, fat, type,
			       measurement_unit, measurement_amount, servings, public, external_id,
			       brand_owner, barcode, ingredients_text, category, created_at, deleted_at
			FROM foods
			WHERE creator_id = ? AND is_current = true AND deleted_at IS NULL
			  AND name LIKE ? ESCAPE ?
			LIMIT ?
		)
		UNION ALL
		SELECT id, creator_id, family_id, version, is_current, name,
		       calories, protein, carbs, fat, type,
		       measurement_unit, measurement_amount, servings, public, external_id,
		       brand_owner, barcode, ingredients_text, category, created_at, deleted_at
		FROM (
			SELECT id, creator_id, family_id, version, is_current, name,
			       calories, protein, carbs, fat, type,
			       measurement_unit, measurement_amount, servings, public, external_id,
			       brand_owner, barcode, ingredients_text, category, created_at, deleted_at
			FROM foods
			WHERE public = true AND is_current = true AND deleted_at IS NULL
			  AND (creator_id != ? OR creator_id IS NULL)
			  AND name LIKE ? ESCAPE ?` + sourceClause + `
			LIMIT ?
		)
	`

	// Build args: user branch args, then public branch args (userID, pattern, escChar, sourceArgs..., limit)
	args := []any{userID, pattern, escChar, limit, userID, pattern, escChar}
	args = append(args, sourceArgs...)
	args = append(args, limit)

	rows, err := db.Query(query, args...)
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
			&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.ExternalID,
			&f.BrandOwner, &f.Barcode, &f.IngredientsText, &f.Category, &f.CreatedAt, &f.DeletedAt,
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

func fetchPortionsForFood(foodID FoodID) ([]FoodPortion, error) {
	rows, err := db.Query(
		`SELECT food_id, name, amount, unit, gram_weight FROM food_portions WHERE food_id = ?`,
		foodID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting food portions: %w", err)
	}
	defer rows.Close()
	var portions []FoodPortion
	for rows.Next() {
		var p FoodPortion
		if err := rows.Scan(&p.FoodID, &p.Name, &p.Amount, &p.Unit, &p.GramWeight); err != nil {
			return nil, fmt.Errorf("scanning portion: %w", err)
		}
		portions = append(portions, p)
	}
	return portions, rows.Err()
}

func GetFoodByExternalID(extID string) (*Food, error) {
	query := `
		SELECT
			id, creator_id, family_id, version, is_current, name,
			calories, protein, carbs, fat, type,
			measurement_unit, measurement_amount, servings, public, external_id,
			brand_owner, barcode, ingredients_text, category, created_at, deleted_at
		FROM foods
		WHERE external_id = ? AND is_current = true AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`
	row := db.QueryRow(query, extID)

	var f Food
	err := row.Scan(
		&f.ID, &f.CreatorID, &f.FamilyID, &f.Version, &f.IsCurrent, &f.Name,
		&f.Calories, &f.Protein, &f.Carbs, &f.Fat, &f.Type,
		&f.MeasurementUnit, &f.MeasurementAmount, &f.Servings, &f.Public, &f.ExternalID,
		&f.BrandOwner, &f.Barcode, &f.IngredientsText, &f.Category, &f.CreatedAt, &f.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Or specific error
		}
		return nil, fmt.Errorf("getting food by external id: %w", err)
	}
	f.Portions, err = fetchPortionsForFood(f.ID)
	if err != nil {
		return nil, err
	}
	return &f, nil
}
