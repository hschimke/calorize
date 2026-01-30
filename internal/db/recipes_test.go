package db

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func createTestUser(t *testing.T) *User {
	u := User{
		Name:      "Test User " + uuid.NewString(),
		Email:     "test+" + uuid.NewString() + "@example.com",
		CreatedAt: time.Now(),
	}
	created, err := CreateUser(u)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return created
}

func createTestIngredient(t *testing.T, user *User, name string) *Food {
	id, _ := uuid.NewV7()
	f := Food{
		ID:                FoodID(id),
		CreatorID:         user.ID,
		FamilyID:          FoodFamilyID(id),
		Version:           1,
		IsCurrent:         true,
		Name:              name,
		Type:              "food",
		MeasurementUnit:   "g",
		MeasurementAmount: 100,
		Calories:          100,
		Protein:           10,
		Carbs:             10,
		Fat:               2,
		CreatedAt:         time.Now(),
	}

	_, err := db.Exec(`
        INSERT INTO foods (
            id, creator_id, family_id, version, is_current, name, type, 
            calories, protein, carbs, fat, measurement_unit, measurement_amount, public, created_at
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, f.ID, f.CreatorID, f.FamilyID, f.Version, f.IsCurrent, f.Name, f.Type,
		f.Calories, f.Protein, f.Carbs, f.Fat, f.MeasurementUnit, f.MeasurementAmount, f.Public, f.CreatedAt)
	if err != nil {
		t.Fatalf("failed to insert test ingredient: %v", err)
	}

	return &f
}

func TestRecipeLifecycle(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)
	flour := createTestIngredient(t, user, "Flour")
	sugar := createTestIngredient(t, user, "Sugar")

	// 1. Create Recipe
	recipe := Food{
		CreatorID: user.ID,
		Name:      "Cake",
		Ingredients: []RecipeItems{
			{IngredientID: flour.ID, Amount: 500},
			{IngredientID: sugar.ID, Amount: 200},
		},
	}

	created, err := CreateRecipe(recipe)
	if err != nil {
		t.Fatalf("CreateRecipe failed: %v", err)
	}
	if len(created.ID) == 0 {
		t.Errorf("Created recipe ID is empty")
	}
	if created.Name != "Cake" {
		t.Errorf("Expected name 'Cake', got '%s'", created.Name)
	}
	if len(created.Ingredients) != 2 {
		t.Errorf("Expected 2 ingredients, got %d", len(created.Ingredients))
	}
	if created.Version != 1 {
		t.Errorf("Expected version 1, got %d", created.Version)
	}

	// 2. Get Recipe
	fetched, err := GetRecipe(RecipeID(created.ID))
	if err != nil {
		t.Fatalf("GetRecipe failed: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("Fetched ID mismatch")
	}
	if len(fetched.Ingredients) != 2 {
		t.Errorf("Expected 2 ingredients, got %d", len(fetched.Ingredients))
	}
	if fetched.Ingredients[0].IngredientID != flour.ID {
		// Ingredients order might not be guaranteed unless sorted.
		// Let's just check existence if order fails, or assume insertion order (usually preserved in simple cases).
		// For robustness, maybe check set equality.
		foundFlour := false
		for _, ing := range fetched.Ingredients {
			if ing.IngredientID == flour.ID {
				foundFlour = true
				break
			}
		}
		if !foundFlour {
			t.Errorf("Flour ingredient not found in fetched recipe")
		}
	}

	// 3. List Recipes
	list, err := ListRecipes(user.ID)
	if err != nil {
		t.Fatalf("ListRecipes failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("Expected 1 recipe, got %d", len(list))
	}
	if list[0].ID != created.ID {
		t.Errorf("List ID mismatch")
	}

	// 4. Update Recipe
	created.Name = "Chocolate Cake"
	// Ensure ingredients are properly set for update
	created.Ingredients = []RecipeItems{
		{IngredientID: flour.ID, Amount: 600},
	}
	updated, err := UpdateRecipe(RecipeID(created.ID), *created)
	if err != nil {
		t.Fatalf("UpdateRecipe failed: %v", err)
	}
	if updated.ID == created.ID {
		t.Errorf("Updated recipe should have new ID")
	}
	if updated.FamilyID != created.FamilyID {
		t.Errorf("Updated recipe should have same FamilyID")
	}
	if updated.Version != 2 {
		t.Errorf("Expected version 2, got %d", updated.Version)
	}
	if updated.Name != "Chocolate Cake" {
		t.Errorf("Expected name 'Chocolate Cake', got '%s'", updated.Name)
	}

	// Verify old version is not current
	old, err := GetRecipe(RecipeID(created.ID))
	if err != nil {
		t.Fatalf("GetRecipe (old) failed: %v", err)
	}
	if old.IsCurrent {
		t.Errorf("Old version should not be current")
	}

	// Verify ListRecipes only shows current
	listV2, err := ListRecipes(user.ID)
	if err != nil {
		t.Fatalf("ListRecipes (v2) failed: %v", err)
	}
	if len(listV2) != 1 {
		t.Errorf("Expected 1 recipe, got %d", len(listV2))
	}
	if listV2[0].ID != updated.ID {
		t.Errorf("List should verify updated version")
	}

	// 5. Get Versions
	versions, err := GetRecipeVersions(RecipeID(updated.ID))
	if err != nil {
		t.Fatalf("GetRecipeVersions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("Expected 2 versions, got %d", len(versions))
	}
	if versions[0].Version != 2 {
		t.Errorf("Expected newest version 2 first, got %d", versions[0].Version)
	}
	if versions[1].Version != 1 {
		t.Errorf("Expected older version 1 second, got %d", versions[1].Version)
	}

	// 6. Delete Recipe
	err = DeleteRecipe(RecipeID(updated.ID))
	if err != nil {
		t.Fatalf("DeleteRecipe failed: %v", err)
	}

	// Verify deletion
	listAfterDelete, err := ListRecipes(user.ID)
	if err != nil {
		t.Fatalf("ListRecipes (after delete) failed: %v", err)
	}
	if len(listAfterDelete) != 0 {
		t.Errorf("Expected 0 recipes, got %d", len(listAfterDelete))
	}

	// Verify versions are all deleted
	versionsAfterDelete, err := GetRecipeVersions(RecipeID(updated.ID))
	if err != nil {
		t.Fatalf("GetRecipeVersions (after delete) failed: %v", err)
	}
	if len(versionsAfterDelete) != 0 {
		t.Errorf("Expected 0 versions after delete, got %d", len(versionsAfterDelete))
	}
}

// Helper to check deep equality if needed, but manual checks are fine.
func deepEqual(t *testing.T, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expected %+v, got %+v", expected, actual)
	}
}
