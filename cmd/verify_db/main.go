package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"azule.info/calorize/internal/database"
)

func main() {
	dbPath := "verify.db"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)

	if err := database.InitDB(dbPath); err != nil {
		log.Fatalf("Failed to init db: %v", err)
	}

	ctx := context.Background()

	// 1. Create User
	userID := "user-123"
	_, err := database.DB.Exec("INSERT INTO users (id, name, email) VALUES (?, ?, ?)", userID, "Test User", "test@example.com")
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}

	// 2. Create Food
	food := &database.Food{
		Name:              "Banana",
		Calories:          89,
		Protein:           1.1,
		Carbs:             22.8,
		Fat:               0.3,
		Type:              "food",
		MeasurementUnit:   "g",
		MeasurementAmount: 100, // 100g = 89kcal
	}
	if err := database.CreateFood(ctx, food); err != nil {
		log.Fatalf("Failed to create food: %v", err)
	}
	fmt.Printf("Created Food: %s (ID: %s)\n", food.Name, food.ID)

	// 3. Create Log
	// Log 200g of Banana (Should be 178kcal)
	logEntry := &database.Log{
		UserID:   userID,
		FoodID:   food.ID,
		Amount:   200,
		MealTag:  "breakfast",
		LoggedAt: time.Now(),
	}
	if err := database.CreateLog(ctx, logEntry); err != nil {
		log.Fatalf("Failed to create log: %v", err)
	}
	fmt.Printf("Created Log for 200g of Banana\n")

	// 4. Verify Stats (uses logs_with_nutrients view)
	// Start/End cover now
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now().Add(24 * time.Hour)

	stats, err := database.GetStats(ctx, userID, start, end)
	if err != nil {
		log.Fatalf("Failed to get stats: %v", err)
	}

	expectedCals := 89.0 * 2.0
	if stats.TotalCalories != expectedCals {
		log.Fatalf("Stats mismatch! Expected %.2f, got %.2f", expectedCals, stats.TotalCalories)
	}
	fmt.Printf("Stats Verified! Total Calories: %.2f\n", stats.TotalCalories)

	// 5. Verify Recipe View (if possible)
	// Let's create a recipe
	recipe := &database.Food{
		Name:              "Banana Split",
		Calories:          0, // calculated from items usually, but here field exists?
		Protein:           0,
		Carbs:             0,
		Fat:               0,
		Type:              "recipe",
		MeasurementUnit:   "serving",
		MeasurementAmount: 1,
	}
	if err := database.CreateFood(ctx, recipe); err != nil {
		log.Fatalf("Failed to create recipe: %v", err)
	}

	// Add Banana to recipe
	items := map[string]float64{
		food.ID: 150, // 150g banana
	}
	if err := database.AddRecipeItems(ctx, recipe.ID, items); err != nil {
		log.Fatalf("Failed to add recipe items: %v", err)
	}

	// Verify Recipe Items (uses recipe_details view)
	rItems, err := database.GetRecipeItems(ctx, recipe.ID)
	if err != nil {
		log.Fatalf("Failed to get recipe items: %v", err)
	}

	if len(rItems) != 1 {
		log.Fatalf("Expected 1 recipe item, got %d", len(rItems))
	}
	if rItems[0].Amount != 150 { // Updated struct in food_queries.go has Amount
		log.Fatalf("Expected amount 150, got %.2f", rItems[0].Amount)
	}
	// Note: In my update to food_queries.go, I changed the struct returned?
	// Let's check food_queries.go signature.
	// Original signature: []struct{ Food Food; Amount float64 }
	// My update changed the QUERY but did I update the scanning logic and struct?
	// I replaced lines 106-113.
	// Ah, I might have broken the Go code if I changed the SELECT columns but not the SCAN destination!
	// Let's check food_queries.go again.

	fmt.Println("Recipe View Verified!")
}
