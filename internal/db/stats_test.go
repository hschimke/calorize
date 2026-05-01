package db

import (
	"testing"
	"time"
)

func createTestLogEntry(t *testing.T, user *User, food *Food, amount float64, logTime time.Time) *FoodLogEntry {
	entry := FoodLogEntry{
		UserID:   user.ID,
		FoodID:   &food.ID,
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

	today := time.Now().UTC()

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

	stats := res

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
	statsEmpty := resEmpty
	if statsEmpty.Calories != 0 {
		t.Errorf("Expected 0 calories for yesterday, got %f", statsEmpty.Calories)
	}
}

func TestGetConsistencyStats(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)
	food := createTestIngredient(t, user, "Consistency Test Food")
	// food is 100 cal/100g, so amount (grams) == calories stored

	goal := 2000
	user.CalorieGoal = &goal
	user, err := UpdateUser(*user)
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	now := time.Now().UTC()
	createTestLogEntry(t, user, food, 1800, now.AddDate(0, 0, -1)) // hit: 1800 ≤ 2000, streak day 1
	createTestLogEntry(t, user, food, 2200, now.AddDate(0, 0, -2)) // miss: 2200 > 2000, breaks streak
	createTestLogEntry(t, user, food, 1900, now.AddDate(0, 0, -3)) // hit, but streak already broken

	stats, err := GetConsistencyStats(user.ID, 0, goal, now)
	if err != nil {
		t.Fatalf("GetConsistencyStats failed: %v", err)
	}
	if stats.Streak != 1 {
		t.Errorf("expected streak=1, got %d", stats.Streak)
	}
	if stats.HitDays30d != 2 {
		t.Errorf("expected hit_days_30d=2, got %d", stats.HitDays30d)
	}
	if stats.TrackedDays30d != 3 {
		t.Errorf("expected tracked_days_30d=3, got %d", stats.TrackedDays30d)
	}
	expected7d := (1800.0 + 2200.0 + 1900.0) / 7.0
	if diff := stats.Rolling7dCalories - expected7d; diff < -1 || diff > 1 {
		t.Errorf("expected rolling_7d_calories≈%.2f, got %.2f", expected7d, stats.Rolling7dCalories)
	}
	expected30d := (1800.0 + 2200.0 + 1900.0) / 30.0
	if diff := stats.Rolling30dCalories - expected30d; diff < -1 || diff > 1 {
		t.Errorf("expected rolling_30d_calories≈%.2f, got %.2f", expected30d, stats.Rolling30dCalories)
	}
	if stats.CalorieGoal != goal {
		t.Errorf("expected calorie_goal=%d, got %d", goal, stats.CalorieGoal)
	}
}

func TestGetConsistencyStatsNoGoal(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)
	food := createTestIngredient(t, user, "No Goal Test Food")

	now := time.Now().UTC()
	createTestLogEntry(t, user, food, 1800, now.AddDate(0, 0, -1))

	stats, err := GetConsistencyStats(user.ID, 0, 0, now)
	if err != nil {
		t.Fatalf("GetConsistencyStats (no goal) failed: %v", err)
	}
	if stats.Streak != 0 {
		t.Errorf("expected streak=0 when no goal, got %d", stats.Streak)
	}
	if stats.HitDays30d != 0 {
		t.Errorf("expected hit_days_30d=0 when no goal, got %d", stats.HitDays30d)
	}
	if stats.TrackedDays30d != 1 {
		t.Errorf("expected tracked_days_30d=1, got %d", stats.TrackedDays30d)
	}
	if stats.Rolling7dCalories <= 0 {
		t.Errorf("expected rolling_7d_calories>0, got %.2f", stats.Rolling7dCalories)
	}
}
