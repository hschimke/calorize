package db

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

func GetFoodLogEntries(userID UserID, date time.Time) ([]FoodLogEntry, error) {
	query := `
		SELECT id, user_id, food_id, amount, meal_tag, logged_at, created_at, deleted_at 
		FROM food_log_entries 
		WHERE user_id = ? AND date(logged_at) = date(?) AND deleted_at IS NULL
	`
	rows, err := db.Query(query, userID, date)
	if err != nil {
		return nil, fmt.Errorf("listing food log entries: %w", err)
	}
	defer rows.Close()

	var entries []FoodLogEntry
	for rows.Next() {
		var entry FoodLogEntry
		if err := rows.Scan(&entry.ID, &entry.UserID, &entry.FoodID, &entry.Amount, &entry.MealTag, &entry.LoggedAt, &entry.CreatedAt, &entry.DeletedAt); err != nil {
			return nil, fmt.Errorf("scanning food log entry: %w", err)
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

	_, err = db.Exec("INSERT INTO food_log_entries (id, user_id, food_id, amount, meal_tag, logged_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		newID, entry.UserID, entry.FoodID, entry.Amount, entry.MealTag, entry.LoggedAt, entry.CreatedAt)
	if err != nil {
		return nil, err
	}

	entry.ID = FoodLogEntryID(newID)
	return &entry, nil

}

func UpdateFoodLogEntry(entry FoodLogEntry) (*FoodLogEntry, error) {
	_, err := db.Exec("UPDATE food_log_entries SET food_id = ?, amount = ?, meal_tag = ?, logged_at = ? WHERE id = ? AND user_id = ?",
		entry.FoodID, entry.Amount, entry.MealTag, entry.LoggedAt, entry.ID, entry.UserID)
	if err != nil {
		return nil, err
	}
	return &entry, nil

}

func DeleteFoodLogEntry(id FoodLogEntryID, userID UserID) error {
	_, err := db.Exec("DELETE FROM food_log_entries WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return err
	}
	return nil
}
