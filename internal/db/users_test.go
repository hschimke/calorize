package db

import (
	"testing"
)

func TestUpdateUserCalorieGoal(t *testing.T) {
	if db == nil {
		t.Skip("Database not initialized")
	}

	user := createTestUser(t)

	goal := 2000
	user.CalorieGoal = &goal

	updated, err := UpdateUser(*user)
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}
	if updated.CalorieGoal == nil || *updated.CalorieGoal != 2000 {
		t.Fatalf("expected CalorieGoal=2000, got %v", updated.CalorieGoal)
	}

	// Reload from DB and verify persistence
	fetched, err := GetUserByID(user.ID)
	if err != nil || fetched == nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if fetched.CalorieGoal == nil || *fetched.CalorieGoal != 2000 {
		t.Fatalf("expected persisted CalorieGoal=2000, got %v", fetched.CalorieGoal)
	}

	// Clear the goal
	user.CalorieGoal = nil
	if _, err := UpdateUser(*user); err != nil {
		t.Fatalf("UpdateUser (clear) failed: %v", err)
	}
	fetched2, _ := GetUserByID(user.ID)
	if fetched2.CalorieGoal != nil {
		t.Fatalf("expected CalorieGoal=nil after clear, got %v", fetched2.CalorieGoal)
	}
}
