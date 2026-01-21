package database

import (
	"context"
	"testing"
	"time"
)

func TestFlexibleNutrients(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	ctx := context.Background()

	t.Run("Create and Get Food with Nutrients", func(t *testing.T) {
		f := &Food{
			Name:              "Fortified Cereal",
			Calories:          150,
			Protein:           3,
			Carbs:             30,
			Fat:               1,
			Type:              "food",
			MeasurementUnit:   "g",
			MeasurementAmount: 40,
			Nutrients: []Nutrient{
				{Name: "Fiber", Amount: 5, Unit: "g"},
				{Name: "Iron", Amount: 8, Unit: "mg"},
			},
		}

		if err := CreateFood(ctx, f); err != nil {
			t.Fatalf("CreateFood failed: %v", err)
		}

		// Verify GetFood
		got, err := GetFood(ctx, f.ID)
		if err != nil {
			t.Fatalf("GetFood failed: %v", err)
		}

		if len(got.Nutrients) != 2 {
			t.Errorf("expected 2 nutrients, got %d", len(got.Nutrients))
		}

		// Check one
		foundFiber := false
		for _, n := range got.Nutrients {
			if n.Name == "Fiber" {
				foundFiber = true
				if n.Amount != 5 {
					t.Errorf("expected Fiber 5, got %f", n.Amount)
				}
				if n.Unit != "g" {
					t.Errorf("expected Fiber unit g, got %s", n.Unit)
				}
			}
		}
		if !foundFiber {
			t.Error("Fiber nutrient not found")
		}
	})

	t.Run("GetStats with Flexible Nutrients", func(t *testing.T) {
		// 1. Create Food
		f := &Food{
			Name:              "Super Drink",
			Calories:          100,
			MeasurementAmount: 100, // 100ml
			Type:              "food",
			Nutrients: []Nutrient{
				{Name: "Vitamin C", Amount: 60, Unit: "mg"},
			},
		}
		if err := CreateFood(ctx, f); err != nil {
			t.Fatalf("CreateFood failed: %v", err)
		}

		// Create User
		user := &User{
			ID:    "user123",
			Name:  "Test User",
			Email: "test@example.com",
		}
		// Need a way to create user? DB schema has users table.
		// There is no CreateUser function in `database` package exposed?
		// Let's check `auth` or just insert manually for test.
		_, err := DB.ExecContext(ctx, "INSERT INTO users (id, name, email) VALUES (?, ?, ?)", user.ID, user.Name, user.Email)
		if err != nil {
			t.Fatalf("Create user failed: %v", err)
		}

		// 2. Log it twice.
		// Log 1: 50ml -> 0.5 ratio -> 50 cal, 30mg Vit C
		log1 := &Log{
			ID:        "log1",
			UserID:    user.ID,
			FoodID:    f.ID,
			Amount:    50,
			MealTag:   "breakfast",
			LoggedAt:  time.Now(),
			CreatedAt: time.Now(),
		}
		// Insert log manually or use helper if exists?
		// `log_queries.go` likely has CreateLog?
		// Let's just insert manually since I don't want to check log_queries right now and I have DB access.
		_, err = DB.ExecContext(ctx, `
			INSERT INTO logs (id, user_id, food_id, amount, meal_tag, logged_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, log1.ID, log1.UserID, log1.FoodID, log1.Amount, log1.MealTag, log1.LoggedAt, log1.CreatedAt)
		if err != nil {
			t.Fatalf("Insert log1 failed: %v", err)
		}

		// Log 2: 200ml -> 2.0 ratio -> 200 cal, 120mg Vit C
		log2 := &Log{
			ID:        "log2",
			UserID:    user.ID,
			FoodID:    f.ID,
			Amount:    200,
			MealTag:   "lunch",
			LoggedAt:  time.Now(),
			CreatedAt: time.Now(),
		}
		_, err = DB.ExecContext(ctx, `
			INSERT INTO logs (id, user_id, food_id, amount, meal_tag, logged_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, log2.ID, log2.UserID, log2.FoodID, log2.Amount, log2.MealTag, log2.LoggedAt, log2.CreatedAt)
		if err != nil {
			t.Fatalf("Insert log2 failed: %v", err)
		}

		// 3. Get Stats
		start := time.Now().Add(-1 * time.Hour)
		end := time.Now().Add(1 * time.Hour)
		stats, err := GetStats(ctx, user.ID, start, end)
		if err != nil {
			t.Fatalf("GetStats failed: %v", err)
		}

		// Check Calories (50 + 200 = 250)
		if stats.TotalCalories != 250 {
			t.Errorf("expected 250 calories, got %f", stats.TotalCalories)
		}

		// Check Vitamin C (30 + 120 = 150)
		found := false
		for _, n := range stats.TotalNutrients {
			if n.Name == "Vitamin C" {
				found = true
				if n.Amount != 150 {
					t.Errorf("expected 150mg Vitamin C, got %f", n.Amount)
				}
				if n.Unit != "mg" {
					t.Errorf("expected unit mg, got %s", n.Unit)
				}
			}
		}
		if !found {
			t.Error("Vitamin C not found in stats")
		}
	})
}
