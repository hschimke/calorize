package database

import (
	"context"
	"testing"
)

func TestCreateFood(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	t.Run("Create New Food", func(t *testing.T) {
		f := &Food{
			Name:              "Banana",
			Calories:          89,
			Protein:           1.1,
			Carbs:             22.8,
			Fat:               0.3,
			Type:              "food", // Changed from "fruit"
			MeasurementUnit:   "g",
			MeasurementAmount: 100,
		}

		if err := CreateFood(ctx, f); err != nil {
			t.Fatalf("CreateFood failed: %v", err)
		}

		if f.ID == "" {
			t.Error("expected ID to be generated")
		}
		if f.FamilyID == "" {
			t.Error("expected FamilyID to be generated")
		}
		if f.Version != 1 {
			t.Errorf("expected Version to be 1, got %d", f.Version)
		}
		if !f.IsCurrent {
			t.Error("expected IsCurrent to be true")
		}
	})

	t.Run("Create New Version", func(t *testing.T) {
		// First create a food
		f1 := &Food{
			Name:              "Apple",
			Calories:          52,
			MeasurementAmount: 100,
			Type:              "food",
		}
		if err := CreateFood(ctx, f1); err != nil {
			t.Fatalf("CreateFood initial failed: %v", err)
		}

		// Create a new version of the same food (family)
		f2 := &Food{
			FamilyID:          f1.FamilyID,
			Name:              "Apple",
			Calories:          55, // Changed calories
			MeasurementAmount: 100,
			Type:              "food",
		}

		if err := CreateFood(ctx, f2); err != nil {
			t.Fatalf("CreateFood version failed: %v", err)
		}

		if f2.Version != 2 {
			t.Errorf("expected Version to be 2, got %d", f2.Version)
		}
		if !f2.IsCurrent {
			t.Error("expected new version to be current")
		}

		// Verify old version is not current
		oldF, err := GetFood(ctx, f1.ID)
		if err != nil {
			t.Fatalf("GetFood old failed: %v", err)
		}
		if oldF.IsCurrent {
			t.Error("expected old version to NOT be current")
		}

		// Verify new version is current
		newF, err := GetFood(ctx, f2.ID)
		if err != nil {
			t.Fatalf("GetFood new failed: %v", err)
		}
		if !newF.IsCurrent {
			t.Error("expected new version to be current")
		}
	})
}

func TestGetFoods(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	// Insert some foods
	foods := []Food{
		{Name: "Carrot", Calories: 41, Type: "food"},
		{Name: "Broccoli", Calories: 34, Type: "food"},
	}
	for _, f := range foods {
		// Need to copy f to pass pointer
		food := f
		if err := CreateFood(ctx, &food); err != nil {
			t.Fatalf("failed to create food %s: %v", food.Name, err)
		}
	}

	t.Run("Get All Current Foods", func(t *testing.T) {
		got, err := GetFoods(ctx, true)
		if err != nil {
			t.Fatalf("GetFoods failed: %v", err)
		}

		if len(got) != 2 {
			t.Errorf("expected 2 foods, got %d", len(got))
		}
	})
}
