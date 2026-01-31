package db

import (
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

	created, err := CreateFood(recipe)
	if err != nil {
		t.Fatalf("CreateFood failed: %v", err)
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
	if created.Type != "recipe" {
		t.Errorf("Expected type 'recipe', got '%s'", created.Type)
	}

	// 2. Get Recipe
	fetched, err := GetFood(created.ID)
	if err != nil {
		t.Fatalf("GetFood failed: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("Fetched ID mismatch")
	}
	if len(fetched.Ingredients) != 2 {
		t.Errorf("Expected 2 ingredients, got %d", len(fetched.Ingredients))
	}
	if fetched.Ingredients[0].IngredientID != flour.ID {
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

	// 3. List Recipes (GetFoods should return recipes now)
	list, err := GetFoods(user.ID)
	if err != nil {
		t.Fatalf("GetFoods failed: %v", err)
	}
	// We might have the ingredients created earlier in the list too, so check if our recipe is there.
	foundRecipe := false
	for _, f := range list {
		if f.ID == created.ID {
			foundRecipe = true
			break
		}
	}
	if !foundRecipe {
		t.Errorf("Recipe not found in GetFoods list")
	}

	// 4. Update Recipe
	created.Name = "Chocolate Cake"
	// Ensure ingredients are properly set for update
	created.Ingredients = []RecipeItems{
		{IngredientID: flour.ID, Amount: 600},
	}
	updated, err := UpdateFood(created.ID, *created)
	if err != nil {
		t.Fatalf("UpdateFood failed: %v", err)
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
	old, err := GetFood(created.ID)
	if err != nil {
		t.Fatalf("GetFood (old) failed: %v", err)
	}
	if old.IsCurrent {
		t.Errorf("Old version should not be current")
	}

	// Verify GetFoods only shows current
	listV2, err := GetFoods(user.ID)
	if err != nil {
		t.Fatalf("GetFoods (v2) failed: %v", err)
	}
	foundUpdated := false
	for _, f := range listV2 {
		if f.ID == updated.ID {
			foundUpdated = true
			break
		}
		if f.ID == created.ID {
			t.Errorf("Old version found in list")
		}
	}
	if !foundUpdated {
		t.Errorf("Updated recipe not found in GetFoods list")
	}

	// 5. Get Versions
	versions, err := GetFoodVersions(updated.ID)
	if err != nil {
		t.Fatalf("GetFoodVersions failed: %v", err)
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
	err = DeleteFood(updated.ID)
	if err != nil {
		t.Fatalf("DeleteFood failed: %v", err)
	}

	// Verify deletion
	listAfterDelete, err := GetFoods(user.ID)
	if err != nil {
		t.Fatalf("GetFoods (after delete) failed: %v", err)
	}
	for _, f := range listAfterDelete {
		if f.ID == updated.ID {
			t.Errorf("Deleted recipe found in list")
		}
	}

	// Verify versions are all deleted
	versionsAfterDelete, err := GetFoodVersions(updated.ID)
	if err != nil {
		t.Fatalf("GetFoodVersions (after delete) failed: %v", err)
	}
	if len(versionsAfterDelete) != 0 {
		t.Errorf("Expected 0 versions after delete, got %d", len(versionsAfterDelete))
	}
}
