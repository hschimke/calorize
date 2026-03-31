package db

import (
	"testing"

	"github.com/google/uuid"
)

func TestUpdateUserPreferences(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)

	// Defaults should be false
	if user.ClownMode {
		t.Error("expected ClownMode=false for new user")
	}
	if user.HidePublicUserFoods {
		t.Error("expected HidePublicUserFoods=false for new user")
	}

	// Set both flags via UpdateUserPreferences with no disabled sources
	user.ClownMode = true
	user.HidePublicUserFoods = true
	if err := UpdateUserPreferences(*user, []string{}); err != nil {
		t.Fatalf("UpdateUserPreferences failed: %v", err)
	}

	fetched, err := GetUserByID(user.ID)
	if err != nil || fetched == nil {
		t.Fatalf("GetUserByID after update failed: %v", err)
	}
	if !fetched.ClownMode {
		t.Error("expected ClownMode=true after update")
	}
	if !fetched.HidePublicUserFoods {
		t.Error("expected HidePublicUserFoods=true after update")
	}

	// Clear both flags
	user.ClownMode = false
	user.HidePublicUserFoods = false
	if err := UpdateUserPreferences(*user, []string{}); err != nil {
		t.Fatalf("UpdateUserPreferences (clear) failed: %v", err)
	}
	fetched2, _ := GetUserByID(user.ID)
	if fetched2.ClownMode {
		t.Error("expected ClownMode=false after clear")
	}
	if fetched2.HidePublicUserFoods {
		t.Error("expected HidePublicUserFoods=false after clear")
	}
}

func TestDisabledSourcesRoundTrip(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)

	// Initially empty
	sources, err := GetDisabledSources(user.ID)
	if err != nil {
		t.Fatalf("GetDisabledSources (empty) failed: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 disabled sources, got %d", len(sources))
	}

	// Set two sources
	if err := SetDisabledSources(user.ID, []string{"afcd", "fdc"}); err != nil {
		t.Fatalf("SetDisabledSources failed: %v", err)
	}
	sources, err = GetDisabledSources(user.ID)
	if err != nil {
		t.Fatalf("GetDisabledSources after set failed: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 disabled sources, got %d", len(sources))
	}

	// Update to a different set (replace, not append)
	if err := SetDisabledSources(user.ID, []string{"off"}); err != nil {
		t.Fatalf("SetDisabledSources (replace) failed: %v", err)
	}
	sources, err = GetDisabledSources(user.ID)
	if err != nil {
		t.Fatalf("GetDisabledSources after replace failed: %v", err)
	}
	if len(sources) != 1 || sources[0] != "off" {
		t.Errorf("expected [off], got %v", sources)
	}

	// Clear to empty
	if err := SetDisabledSources(user.ID, []string{}); err != nil {
		t.Fatalf("SetDisabledSources (clear) failed: %v", err)
	}
	sources, err = GetDisabledSources(user.ID)
	if err != nil {
		t.Fatalf("GetDisabledSources after clear failed: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources after clear, got %d", len(sources))
	}
}

func TestUpdateUserPreferencesAtomicity(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)

	// Set initial state
	user.ClownMode = true
	if err := UpdateUserPreferences(*user, []string{"afcd"}); err != nil {
		t.Fatalf("initial UpdateUserPreferences failed: %v", err)
	}

	// Verify both were written together
	fetched, err := GetUserByID(user.ID)
	if err != nil || fetched == nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if !fetched.ClownMode {
		t.Error("expected ClownMode=true")
	}
	sources, err := GetDisabledSources(user.ID)
	if err != nil {
		t.Fatalf("GetDisabledSources failed: %v", err)
	}
	if len(sources) != 1 || sources[0] != "afcd" {
		t.Errorf("expected [afcd], got %v", sources)
	}
}

func TestSearchFoodsWithDisabledSources(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)

	// Insert a food tagged as an AFCD import
	extID := "afcd_TEST001"
	foodID := uuid.NewString()
	_, err := db.Exec(`
		INSERT INTO foods (
			id, family_id, version, is_current, name, type,
			calories, protein, carbs, fat, measurement_unit, measurement_amount, servings,
			public, external_id, created_at
		) VALUES (
			?, ?, 1, true, 'TestAFCDFood', 'food',
			100, 10, 10, 2, 'g', 100, 1,
			true, ?, datetime('now')
		)`, foodID, foodID, extID)
	if err != nil {
		t.Fatalf("failed to insert test AFCD food: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM foods WHERE external_id = ?", extID)
	})

	// Without filter: food should appear
	results, err := SearchFoods(user.ID, "TestAFCD", 50, nil, false)
	if err != nil {
		t.Fatalf("SearchFoods (no filter) failed: %v", err)
	}
	found := false
	for _, r := range results {
		if r.ExternalID != nil && *r.ExternalID == extID {
			found = true
		}
	}
	if !found {
		t.Error("expected AFCD food in results when no source filter applied")
	}

	// With afcd disabled: food should be excluded
	results, err = SearchFoods(user.ID, "TestAFCD", 50, []string{"afcd"}, false)
	if err != nil {
		t.Fatalf("SearchFoods (afcd disabled) failed: %v", err)
	}
	for _, r := range results {
		if r.ExternalID != nil && *r.ExternalID == extID {
			t.Error("AFCD food should be excluded when source 'afcd' is disabled")
		}
	}
}

func TestGetFoodsHidePublicUserFoods(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	owner := createTestUser(t)
	viewer := createTestUser(t)

	// Create a public food owned by 'owner'
	food := Food{
		CreatorID:         owner.ID,
		Name:              "OtherUserPublicFood",
		Calories:          50,
		Protein:           5,
		Carbs:             5,
		Fat:               1,
		Type:              "food",
		MeasurementUnit:   "g",
		MeasurementAmount: 100,
		Servings:          1,
		Public:            true,
	}
	created, err := CreateFood(food)
	if err != nil {
		t.Fatalf("CreateFood failed: %v", err)
	}
	t.Cleanup(func() { DeleteFood(created.ID) })

	// viewer with hidePublicUserFoods=false should see the food
	results, err := SearchFoods(viewer.ID, "OtherUserPublic", 50, nil, false)
	if err != nil {
		t.Fatalf("SearchFoods (hide=false) failed: %v", err)
	}
	found := false
	for _, r := range results {
		if r.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected other user's public food to appear when hide=false")
	}

	// viewer with hidePublicUserFoods=true should NOT see the food
	results, err = SearchFoods(viewer.ID, "OtherUserPublic", 50, nil, true)
	if err != nil {
		t.Fatalf("SearchFoods (hide=true) failed: %v", err)
	}
	for _, r := range results {
		if r.ID == created.ID {
			t.Error("other user's public food should be hidden when hidePublicUserFoods=true")
		}
	}

	// owner should always see their own food regardless of the flag
	results, err = SearchFoods(owner.ID, "OtherUserPublic", 50, nil, true)
	if err != nil {
		t.Fatalf("SearchFoods (owner, hide=true) failed: %v", err)
	}
	ownFound := false
	for _, r := range results {
		if r.ID == created.ID {
			ownFound = true
		}
	}
	if !ownFound {
		t.Error("owner should always see their own food even when hidePublicUserFoods=true")
	}
}
