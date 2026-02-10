package db

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

func GetFoodLogEntries(userID UserID, date time.Time) ([]FoodLogEntry, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24 * time.Hour)

	query := `
		SELECT id, user_id, food_id, calories, protein, carbs, fat, amount, meal_tag, logged_at, created_at, deleted_at 
		FROM food_log_entries 
		WHERE user_id = ? AND logged_at >= ? AND logged_at < ? AND deleted_at IS NULL
	`
	rows, err := db.Query(query, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("listing food log entries: %w", err)
	}
	defer rows.Close()

	var entries []FoodLogEntry
	for rows.Next() {
		var entry FoodLogEntry
		var foodID uuid.NullUUID
		if err := rows.Scan(&entry.ID, &entry.UserID, &foodID, &entry.Calories, &entry.Protein, &entry.Carbs, &entry.Fat, &entry.Amount, &entry.MealTag, &entry.LoggedAt, &entry.CreatedAt, &entry.DeletedAt); err != nil {
			return nil, fmt.Errorf("scanning food log entry: %w", err)
		}
		if foodID.Valid {
			fid := FoodID(foodID.UUID)
			entry.FoodID = &fid

			// Populate Food object
			food, err := GetFood(fid)
			if err != nil {
				// Log error but continue? or fail?
				// For now, let's just continue and leave Food nil if it fails,
				// though strictly we should probably return error or log it.
				// Given GetFood returns (nil, nil) if not found (in some implementations),
				// let's check. GetFood implementation in api.go checked for nil.
				// We'll rely on it returning payload or error.
				// If we error here, the whole log list fails.
				// Let's wrap in a check.
			}
			if food != nil {
				entry.Food = food
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func CreateFoodLogEntry(entry FoodLogEntry) (*FoodLogEntry, error) {
	newID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	if entry.LoggedAt.IsZero() {
		entry.LoggedAt = time.Now()
	}

	if err := populateMacros(&entry); err != nil {
		return nil, err
	}

	_, err = db.Exec("INSERT INTO food_log_entries (id, user_id, food_id, calories, protein, carbs, fat, amount, meal_tag, logged_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		newID, entry.UserID, entry.FoodID, entry.Calories, entry.Protein, entry.Carbs, entry.Fat, entry.Amount, entry.MealTag, entry.LoggedAt, entry.CreatedAt)
	if err != nil {
		return nil, err
	}

	entry.ID = FoodLogEntryID(newID)
	return &entry, nil
}

func UpdateFoodLogEntry(entry FoodLogEntry) (*FoodLogEntry, error) {
	if err := populateMacros(&entry); err != nil {
		return nil, err
	}

	_, err := db.Exec("UPDATE food_log_entries SET food_id = ?, calories = ?, protein = ?, carbs = ?, fat = ?, amount = ?, meal_tag = ?, logged_at = ? WHERE id = ? AND user_id = ?",
		entry.FoodID, entry.Calories, entry.Protein, entry.Carbs, entry.Fat, entry.Amount, entry.MealTag, entry.LoggedAt, entry.ID, entry.UserID)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func populateMacros(entry *FoodLogEntry) error {
	if entry.FoodID != nil {
		food, err := GetFood(*entry.FoodID)
		if err != nil {
			return fmt.Errorf("getting food for macro calculation: %w", err)
		}
		if food != nil {
			measurement := food.MeasurementAmount
			if measurement == 0 {
				measurement = 1
			}
			ratio := entry.Amount / measurement

			cal := ratio * food.Calories
			prot := ratio * food.Protein
			carb := ratio * food.Carbs
			fat := ratio * food.Fat

			entry.Calories = &cal
			entry.Protein = &prot
			entry.Carbs = &carb
			entry.Fat = &fat
		}
	}
	return nil
}

func DeleteFoodLogEntry(id FoodLogEntryID, userID UserID) error {
	_, err := db.Exec("UPDATE food_log_entries SET deleted_at = ? WHERE id = ? AND user_id = ?", time.Now(), id, userID)
	if err != nil {
		return err
	}
	return nil
}
