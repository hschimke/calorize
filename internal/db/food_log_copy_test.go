package db

import (
	"testing"
	"time"
)

func TestCopyFoodLogEntries(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)
	food := createTestIngredient(t, user, "Copy Test Food")
	food.Calories = 200
	food.MeasurementAmount = 100
	updated, err := UpdateFood(food.ID, *food)
	if err != nil {
		t.Fatalf("UpdateFood failed: %v", err)
	}

	yesterday := time.Now().UTC().AddDate(0, 0, -1)

	// Create breakfast, lunch, and dinner entries on yesterday
	for _, tc := range []struct {
		amount  float64
		mealTag string
	}{
		{100, "breakfast"},
		{150, "lunch"},
		{200, "dinner"},
	} {
		_, err := CreateFoodLogEntry(FoodLogEntry{
			UserID:   user.ID,
			FoodID:   &updated.ID,
			Amount:   tc.amount,
			MealTag:  tc.mealTag,
			LoggedAt: yesterday,
		})
		if err != nil {
			t.Fatalf("CreateFoodLogEntry (%s) failed: %v", tc.mealTag, err)
		}
	}

	// Copy only breakfast and lunch to "now"
	copyTime := time.Now().UTC()
	count, err := CopyFoodLogEntries(user.ID, yesterday, []string{"breakfast", "lunch"}, copyTime)
	if err != nil {
		t.Fatalf("CopyFoodLogEntries failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 entries copied, got %d", count)
	}

	// The copied entries are stamped with copyTime, so query using copyTime's date
	copied, err := GetFoodLogEntries(user.ID, copyTime)
	if err != nil {
		t.Fatalf("GetFoodLogEntries (today) failed: %v", err)
	}

	// Filter to this test's user (other tests may run concurrently on same db)
	var mine []FoodLogEntry
	for _, e := range copied {
		if e.UserID == user.ID {
			mine = append(mine, e)
		}
	}

	if len(mine) != 2 {
		t.Fatalf("Expected 2 copied entries, got %d", len(mine))
	}

	tags := map[string]bool{}
	for _, e := range mine {
		tags[e.MealTag] = true
	}
	if !tags["breakfast"] {
		t.Error("Expected a breakfast entry")
	}
	if !tags["lunch"] {
		t.Error("Expected a lunch entry")
	}
	if tags["dinner"] {
		t.Error("Dinner should not have been copied")
	}

	// logged_at must be within 2 seconds of copyTime
	for _, e := range mine {
		diff := e.LoggedAt.Sub(copyTime)
		if diff < 0 {
			diff = -diff
		}
		if diff > 2*time.Second {
			t.Errorf("logged_at %v not near copyTime %v", e.LoggedAt, copyTime)
		}
	}
}

func TestCopyFoodLogEntries_Empty(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)
	// No entries exist for this user; copying should return 0 without error
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	count, err := CopyFoodLogEntries(user.ID, yesterday, []string{"breakfast"}, time.Now().UTC())
	if err != nil {
		t.Fatalf("CopyFoodLogEntries on empty day failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 entries copied, got %d", count)
	}
}
