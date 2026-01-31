package db

import (
	"testing"
	"time"
)

func createTestLogEntry(t *testing.T, user *User, food *Food, amount float64, logTime time.Time) *FoodLogEntry {
	// CreateFoodLogEntry helper might assume functional db setup.
	// Since we haven't implemented CreateFoodLogEntry manually in this session but seeing it referenced in context,
	// I'll assume it exists or use manual insert if checks fail.
	// previous `view_file` showed it exists in `food-log-entries.go`.

	entry := FoodLogEntry{
		UserID:   user.ID,
		FoodID:   food.ID,
		Amount:   amount,
		MealTag:  "breakfast",
		LoggedAt: logTime,
	}
	created, err := CreateFoodLogEntry(entry)
	if err != nil {
		t.Fatalf("CreateFoodLogEntry failed: %v", err)
	}
	return created
}

func TestGetStats(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)

	// Food: 100kcal per 100g
	food := createTestIngredient(t, user, "Test Food")
	// Update with specific macros
	food.Calories = 100
	food.Protein = 10
	food.Carbs = 20
	food.Fat = 5
	food.MeasurementAmount = 100
	updatedFood, err := UpdateFood(food.ID, *food)
	if err != nil {
		t.Fatalf("UpdateFood failed: %v", err)
	}

	today := time.Now()

	// Log 1: 200g (200kcal, 20p, 40c, 10f)
	createTestLogEntry(t, user, updatedFood, 200, today)

	// Log 2: 50g (50kcal, 5p, 10c, 2.5f)
	createTestLogEntry(t, user, updatedFood, 50, today)

	// Total: 250kcal, 25p, 50c, 12.5f

	// Verify Daily Stats
	res, err := GetStats(user.ID, "day", today)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	stats, ok := res.(DailyStats)
	if !ok {
		t.Fatalf("Expected DailyStats, got %T", res)
	}

	if stats.Calories != 250 {
		t.Errorf("Expected 250 calories, got %f", stats.Calories)
	}
	if stats.Protein != 25 {
		t.Errorf("Expected 25 protein, got %f", stats.Protein)
	}
	if stats.Carbs != 50 {
		t.Errorf("Expected 50 carbs, got %f", stats.Carbs)
	}
	if stats.Fat != 12.5 {
		t.Errorf("Expected 12.5 fat, got %f", stats.Fat)
	}

	// Verify Empty Stats (Yesterday)
	yesterday := today.AddDate(0, 0, -1)
	resEmpty, err := GetStats(user.ID, "day", yesterday)
	if err != nil {
		t.Fatalf("GetStats(yesterday) failed: %v", err)
	}
	statsEmpty := resEmpty.(DailyStats)
	if statsEmpty.Calories != 0 {
		t.Errorf("Expected 0 calories for yesterday, got %f", statsEmpty.Calories)
	}
}

// Re-using createTestUser/Ingredient from recipe test package context if allowed,
// otherwise need to duplicate or assume shared package.
// They are in the same package `db`, so it should share test helpers if in same directory?
// Yes, `go test ./internal/db` compiles all test files in package together.
