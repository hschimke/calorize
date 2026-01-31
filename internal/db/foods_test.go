package db

import (
	"testing"
)

func TestFoodLifecycle(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)

	// 1. Create Food with Nutrients
	food := Food{
		CreatorID:         user.ID,
		Name:              "Banana",
		Calories:          105,
		Protein:           1.3,
		Carbs:             27,
		Fat:               0.4,
		MeasurementUnit:   "medium",
		MeasurementAmount: 1,
		Nutrients: []FoodNutrient{
			{Name: "Potassium", Amount: 422, Unit: "mg"},
			{Name: "Vitamin C", Amount: 10, Unit: "mg"},
		},
	}

	created, err := CreateFood(food)
	if err != nil {
		t.Fatalf("CreateFood failed: %v", err)
	}
	if len(created.ID) == 0 {
		t.Errorf("Created food ID is empty")
	}
	if created.Name != "Banana" {
		t.Errorf("Expected name 'Banana', got '%s'", created.Name)
	}
	if len(created.Nutrients) != 2 {
		t.Errorf("Expected 2 nutrients, got %d", len(created.Nutrients))
	}
	if created.Version != 1 {
		t.Errorf("Expected version 1, got %d", created.Version)
	}

	// 2. Get Food
	fetched, err := GetFood(created.ID)
	if err != nil {
		t.Fatalf("GetFood failed: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("Fetched ID mismatch")
	}
	if len(fetched.Nutrients) != 2 {
		t.Errorf("Expected 2 nutrients, got %d", len(fetched.Nutrients))
	}
	// Verify nutrient content
	foundK := false
	for _, n := range fetched.Nutrients {
		if n.Name == "Potassium" && n.Amount == 422 {
			foundK = true
		}
	}
	if !foundK {
		t.Errorf("Potassium nutrient not found or incorrect")
	}

	// 3. List Foods
	// Should create another food to test listing multiple? Or just one is fine.
	list, err := GetFoods(user.ID)
	if err != nil {
		t.Fatalf("GetFoods failed: %v", err)
	}
	// Might be > 1 if createTestIngredient used same user, but createTestUser makes unique user.
	if len(list) != 1 {
		t.Errorf("Expected 1 food, got %d", len(list))
	}
	if list[0].ID != created.ID {
		t.Errorf("List ID mismatch")
	}
	// Verify ListFoods does NOT return nutrients (impl choice for performance)
	if len(list[0].Nutrients) != 0 {
		t.Errorf("GetFoods should not return nutrients by default")
	}

	// 4. Update Food
	created.Name = "Ripe Banana"
	created.Calories = 110
	// Ensure nutrients are passed for update
	created.Nutrients = []FoodNutrient{
		{Name: "Potassium", Amount: 450, Unit: "mg"},
	}
	updated, err := UpdateFood(created.ID, *created)
	if err != nil {
		t.Fatalf("UpdateFood failed: %v", err)
	}
	if updated.ID == created.ID {
		t.Errorf("Updated food should have new ID")
	}
	if updated.FamilyID != created.FamilyID {
		t.Errorf("Updated food should have same FamilyID")
	}
	if updated.Version != 2 {
		t.Errorf("Expected version 2, got %d", updated.Version)
	}
	if updated.Name != "Ripe Banana" {
		t.Errorf("Expected name 'Ripe Banana', got '%s'", updated.Name)
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
	if len(listV2) != 1 {
		t.Errorf("Expected 1 food, got %d", len(listV2))
	}
	if listV2[0].ID != updated.ID {
		t.Errorf("List should confirm updated version")
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
		t.Errorf("Expected newest version 2 first")
	}

	// 6. Delete Food
	err = DeleteFood(updated.ID)
	if err != nil {
		t.Fatalf("DeleteFood failed: %v", err)
	}

	listAfterDelete, err := GetFoods(user.ID)
	if err != nil {
		t.Fatalf("GetFoods (after delete) failed: %v", err)
	}
	if len(listAfterDelete) != 0 {
		t.Errorf("Expected 0 foods, got %d", len(listAfterDelete))
	}

	versionsAfterDelete, err := GetFoodVersions(updated.ID)
	if err != nil {
		t.Fatalf("GetFoodVersions (after delete) failed: %v", err)
	}
	if len(versionsAfterDelete) != 0 {
		t.Errorf("Expected 0 versions after delete")
	}
}
