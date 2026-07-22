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

	// Each copy records the source entry it was copied from.
	sources, err := GetFoodLogEntries(user.ID, yesterday)
	if err != nil {
		t.Fatalf("GetFoodLogEntries (yesterday) failed: %v", err)
	}
	sourceByTag := map[string]FoodLogEntryID{}
	for _, e := range sources {
		if e.UserID == user.ID {
			sourceByTag[e.MealTag] = e.ID
		}
	}
	for _, e := range mine {
		if e.CopiedFromID == nil {
			t.Errorf("Copied %s entry should record copied_from_id", e.MealTag)
			continue
		}
		if *e.CopiedFromID != sourceByTag[e.MealTag] {
			t.Errorf("Copied %s entry points at %v, expected source %v", e.MealTag, *e.CopiedFromID, sourceByTag[e.MealTag])
		}
	}
	// And the sources themselves have no lineage.
	for _, e := range sources {
		if e.UserID == user.ID && e.CopiedFromID != nil {
			t.Errorf("Source %s entry should have no copied_from_id", e.MealTag)
		}
	}
}

func TestGetFoodLogLineageSummary(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)
	other := createTestUser(t)
	food := createTestIngredient(t, user, "Chain Test Food")

	// Log an entry two days ago, then copy day-2-ago -> yesterday -> today,
	// building a chain: original -> copy1 -> copy2.
	twoDaysAgo := time.Now().UTC().AddDate(0, 0, -2)
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	today := time.Now().UTC()

	original, err := CreateFoodLogEntry(FoodLogEntry{
		UserID:   user.ID,
		FoodID:   &food.ID,
		Amount:   100,
		MealTag:  "breakfast",
		LoggedAt: twoDaysAgo,
	})
	if err != nil {
		t.Fatalf("CreateFoodLogEntry failed: %v", err)
	}

	if _, err := CopyFoodLogEntries(user.ID, twoDaysAgo, []string{"breakfast"}, yesterday); err != nil {
		t.Fatalf("CopyFoodLogEntries (day 1) failed: %v", err)
	}
	if _, err := CopyFoodLogEntries(user.ID, yesterday, []string{"breakfast"}, today); err != nil {
		t.Fatalf("CopyFoodLogEntries (day 2) failed: %v", err)
	}

	todayEntries, err := GetFoodLogEntries(user.ID, today)
	if err != nil {
		t.Fatalf("GetFoodLogEntries (today) failed: %v", err)
	}
	var leaf *FoodLogEntry
	for i := range todayEntries {
		if todayEntries[i].UserID == user.ID && todayEntries[i].CopiedFromID != nil {
			leaf = &todayEntries[i]
			break
		}
	}
	if leaf == nil {
		t.Fatal("Expected a copied entry on today")
	}

	// Chain depth 2 back to the original.
	summary, err := GetFoodLogLineageSummary(leaf.ID, user.ID)
	if err != nil {
		t.Fatalf("GetFoodLogLineageSummary failed: %v", err)
	}
	if summary == nil {
		t.Fatal("Expected a summary for own entry")
	}
	if summary.Origin.ID != original.ID {
		t.Errorf("Origin should be the original entry, got %v", summary.Origin.ID)
	}
	if summary.Copies != 2 {
		t.Errorf("Expected 2 copy-steps, got %d", summary.Copies)
	}
	if summary.Origin.Food == nil || summary.Origin.Food.Name != "Chain Test Food" {
		t.Errorf("Origin should have its food hydrated")
	}

	// Ownership: another user gets nothing.
	foreign, err := GetFoodLogLineageSummary(leaf.ID, other.ID)
	if err != nil {
		t.Fatalf("GetFoodLogLineageSummary (other user) failed: %v", err)
	}
	if foreign != nil {
		t.Errorf("Other users should not see the entry's lineage")
	}

	// Non-copy entry: origin is itself, 0 copies.
	selfSummary, err := GetFoodLogLineageSummary(original.ID, user.ID)
	if err != nil {
		t.Fatalf("GetFoodLogLineageSummary (original) failed: %v", err)
	}
	if selfSummary == nil || selfSummary.Origin.ID != original.ID || selfSummary.Copies != 0 {
		t.Errorf("Non-copy entry should be its own origin with 0 copies")
	}

	// Deleted origin stays reported, flagged via DeletedAt.
	if err := DeleteFoodLogEntry(original.ID, user.ID); err != nil {
		t.Fatalf("DeleteFoodLogEntry failed: %v", err)
	}
	afterDelete, err := GetFoodLogLineageSummary(leaf.ID, user.ID)
	if err != nil {
		t.Fatalf("GetFoodLogLineageSummary (after delete) failed: %v", err)
	}
	if afterDelete == nil || afterDelete.Origin.ID != original.ID {
		t.Fatal("Deleted origin should still be reported")
	}
	if afterDelete.Origin.DeletedAt == nil {
		t.Errorf("Deleted origin should carry DeletedAt")
	}
	if afterDelete.Copies != 2 {
		t.Errorf("Chain depth should be unchanged after origin deletion, got %d", afterDelete.Copies)
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
