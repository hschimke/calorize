package db

import (
	"testing"
	"time"
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

func TestGetRecentFoods(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)

	// No log entries: should return empty slice without error
	recent, err := GetRecentFoods(user.ID, 50)
	if err != nil {
		t.Fatalf("GetRecentFoods (empty) failed: %v", err)
	}
	if len(recent) != 0 {
		t.Errorf("Expected 0 recent foods, got %d", len(recent))
	}

	// Create foods and log entries
	apple := createTestIngredient(t, user, "Apple")
	banana := createTestIngredient(t, user, "Banana")
	carrot := createTestIngredient(t, user, "Carrot")

	now := time.Now().UTC()
	// Log apple first (oldest), then carrot, then banana (most recent)
	createTestLogEntry(t, user, apple, 100, now.Add(-2*time.Hour))
	createTestLogEntry(t, user, carrot, 100, now.Add(-1*time.Hour))
	createTestLogEntry(t, user, banana, 100, now)

	recent, err = GetRecentFoods(user.ID, 50)
	if err != nil {
		t.Fatalf("GetRecentFoods failed: %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("Expected 3 recent foods, got %d", len(recent))
	}
	// Verify order: banana (most recent) first, apple (oldest) last
	if recent[0].ID != banana.ID {
		t.Errorf("Expected banana first (most recent), got %s", recent[0].Name)
	}
	if recent[1].ID != carrot.ID {
		t.Errorf("Expected carrot second, got %s", recent[1].Name)
	}
	if recent[2].ID != apple.ID {
		t.Errorf("Expected apple third (oldest), got %s", recent[2].Name)
	}

	// Limit is respected: only return 2
	limited, err := GetRecentFoods(user.ID, 2)
	if err != nil {
		t.Fatalf("GetRecentFoods (limited) failed: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("Expected 2 recent foods with limit=2, got %d", len(limited))
	}
	// Should still be most recent first
	if limited[0].ID != banana.ID {
		t.Errorf("Expected banana first with limit, got %s", limited[0].Name)
	}
}

func TestGetRecentFoodsUserIsolation(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user1 := createTestUser(t)
	user2 := createTestUser(t)

	// Create distinct foods for each user
	apple := createTestIngredient(t, user1, "Apple")
	mango := createTestIngredient(t, user2, "Mango")

	now := time.Now().UTC()
	createTestLogEntry(t, user1, apple, 100, now)
	createTestLogEntry(t, user2, mango, 100, now)

	// user1 should see only apple, not mango
	recent, err := GetRecentFoods(user1.ID, 50)
	if err != nil {
		t.Fatalf("GetRecentFoods (user1) failed: %v", err)
	}
	for _, f := range recent {
		if f.ID == mango.ID {
			t.Errorf("GetRecentFoods returned user2's food (mango) for user1")
		}
	}
	if len(recent) != 1 || recent[0].ID != apple.ID {
		t.Errorf("Expected only apple for user1, got %d results", len(recent))
	}
}

func TestSearchFoods(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)

	// No results: should return empty slice without error
	results, err := SearchFoods(user.ID, "zzznomatch", 50)
	if err != nil {
		t.Fatalf("SearchFoods (no results) failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for no-match query, got %d", len(results))
	}

	// Create foods
	createTestIngredient(t, user, "Banana")
	createTestIngredient(t, user, "Blueberry")
	createTestIngredient(t, user, "Apple")

	// Prefix match: "Ban" should return "Banana"
	results, err = SearchFoods(user.ID, "Ban", 50)
	if err != nil {
		t.Fatalf("SearchFoods (prefix) failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result for 'Ban', got %d", len(results))
	}
	if results[0].Name != "Banana" {
		t.Errorf("Expected 'Banana', got '%s'", results[0].Name)
	}

	// Case-insensitive: "ban" should match "Banana"
	results, err = SearchFoods(user.ID, "ban", 50)
	if err != nil {
		t.Fatalf("SearchFoods (case-insensitive) failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result for 'ban', got %d", len(results))
	}
	if results[0].Name != "Banana" {
		t.Errorf("Expected 'Banana', got '%s'", results[0].Name)
	}

	// "B" prefix should match both "Banana" and "Blueberry"
	results, err = SearchFoods(user.ID, "B", 50)
	if err != nil {
		t.Fatalf("SearchFoods ('B' prefix) failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'B', got %d", len(results))
	}

	// Limit is respected
	limited, err := SearchFoods(user.ID, "B", 1)
	if err != nil {
		t.Fatalf("SearchFoods (limit) failed: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("Expected 1 result with limit=1, got %d", len(limited))
	}

	// LIKE special chars are treated as literals, not wildcards
	// "Ban%" should match nothing (no food named "Ban%...")
	results, err = SearchFoods(user.ID, "Ban%", 50)
	if err != nil {
		t.Fatalf("SearchFoods (LIKE special char) failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for query 'Ban%%' (percent treated literally), got %d", len(results))
	}

	// "_" as a literal: "Banan_" should not match "Banana" (underscore is literal, not wildcard)
	results, err = SearchFoods(user.ID, "Banan_", 50)
	if err != nil {
		t.Fatalf("SearchFoods (underscore literal) failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for query 'Banan_' (underscore treated literally), got %d", len(results))
	}

	// Public food from another user should appear in results
	user2 := createTestUser(t)
	publicFood := createTestIngredient(t, user2, "PublicBanana")
	// Mark the food as public
	_, err = db.Exec("UPDATE foods SET public = true WHERE id = ?", publicFood.ID)
	if err != nil {
		t.Fatalf("Failed to mark food public: %v", err)
	}
	t.Cleanup(func() { _ = DeleteFood(publicFood.ID) })
	results, err = SearchFoods(user.ID, "PublicBan", 50)
	if err != nil {
		t.Fatalf("SearchFoods (public food) failed: %v", err)
	}
	found := false
	for _, r := range results {
		if r.ID == publicFood.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected public food 'PublicBanana' to appear in search results for user1, got %d results", len(results))
	}
}
