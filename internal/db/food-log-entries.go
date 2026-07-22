package db

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

func GetFoodLogEntries(userID UserID, date time.Time) ([]FoodLogEntry, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location()).UTC()
	end := start.Add(24 * time.Hour)

	query := `
		SELECT id, user_id, food_id, portion_name, calories, protein, carbs, fat, amount, meal_tag, note, copied_from_id, logged_at, created_at, deleted_at
		FROM food_log_entries
		WHERE user_id = ? AND logged_at >= ? AND logged_at < ? AND deleted_at IS NULL
		ORDER BY logged_at ASC, created_at ASC
	`
	rows, err := db.Query(query, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("listing food log entries: %w", err)
	}
	defer rows.Close()

	var entries []FoodLogEntry
	uniqueFoodIDs := make([]FoodID, 0)
	foodIDSet := make(map[FoodID]struct{})

	for rows.Next() {
		var entry FoodLogEntry
		var foodID uuid.NullUUID
		if err := rows.Scan(&entry.ID, &entry.UserID, &foodID, &entry.PortionName, &entry.Calories, &entry.Protein, &entry.Carbs, &entry.Fat, &entry.Amount, &entry.MealTag, &entry.Note, nullFoodLogEntryID{&entry.CopiedFromID}, &entry.LoggedAt, &entry.CreatedAt, &entry.DeletedAt); err != nil {
			return nil, fmt.Errorf("scanning food log entry: %w", err)
		}
		if foodID.Valid {
			fid := FoodID(foodID.UUID)
			entry.FoodID = &fid
			if _, exists := foodIDSet[fid]; !exists {
				foodIDSet[fid] = struct{}{}
				uniqueFoodIDs = append(uniqueFoodIDs, fid)
			}
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating food log entries: %w", err)
	}

	if len(uniqueFoodIDs) > 0 {
		foodMap, err := GetFoodsByIDs(uniqueFoodIDs)
		if err != nil {
			slog.Error("failed to batch get foods for log entries", "error", err)
		} else {
			for i := range entries {
				if entries[i].FoodID != nil {
					if f, ok := foodMap[*entries[i].FoodID]; ok {
						entries[i].Food = f
					}
				}
			}
		}
	}

	return entries, nil
}

func CreateFoodLogEntry(entry FoodLogEntry) (*FoodLogEntry, error) {
	newID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	if entry.LoggedAt.IsZero() {
		entry.LoggedAt = time.Now().UTC()
	}

	if err := populateMacros(&entry); err != nil {
		return nil, err
	}

	_, err = db.Exec("INSERT INTO food_log_entries (id, user_id, food_id, portion_name, calories, protein, carbs, fat, amount, meal_tag, note, logged_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		newID, entry.UserID, entry.FoodID, entry.PortionName, entry.Calories, entry.Protein, entry.Carbs, entry.Fat, entry.Amount, entry.MealTag, entry.Note, entry.LoggedAt, entry.CreatedAt)
	if err != nil {
		return nil, err
	}

	entry.ID = FoodLogEntryID(newID)
	return &entry, nil
}

func populateMacros(entry *FoodLogEntry) error {
	if entry.FoodID != nil {
		food, err := GetFood(*entry.FoodID)
		if err != nil {
			return fmt.Errorf("getting food for macro calculation: %w", err)
		}
		if food == nil {
			return fmt.Errorf("food not found: %v", *entry.FoodID)
		}

		var ratio float64
		if entry.PortionName != nil {
			// Find portion gram weight
			var gramWeight float64
			found := false
			for _, p := range food.Portions {
				if p.Name == *entry.PortionName {
					gramWeight = p.GramWeight
					found = true
					break
				}
			}
			if found {
				// Nutrients in DB are per food.MeasurementAmount (usually 100 for FDC)
				// So if 1 portion = 28g, and we log 1 portion, and base is 100g:
				// (1 * 28) / 100 = 0.28 multiplier for the per-100 values.
				base := food.MeasurementAmount
				if base == 0 {
					base = 1
				}
				ratio = (entry.Amount * gramWeight) / base
			} else {
				// Fallback
				slog.Warn("portion not found, falling back to default measurement", "portion", *entry.PortionName, "food_id", food.ID)
				measurement := food.MeasurementAmount
				if measurement == 0 {
					measurement = 1
				}
				ratio = entry.Amount / measurement
			}
		} else {
			// Default logic
			measurement := food.MeasurementAmount
			if measurement == 0 {
				measurement = 1
			}
			ratio = entry.Amount / measurement
		}

		cal := ratio * food.Calories
		prot := ratio * food.Protein
		carb := ratio * food.Carbs
		fat := ratio * food.Fat

		entry.Calories = &cal
		entry.Protein = &prot
		entry.Carbs = &carb
		entry.Fat = &fat
	}
	return nil
}

// CopyFoodLogEntries copies log entries from fromDate (filtered by mealTags) to new entries
// stamped with loggedAt. Returns the number of entries created.
func CopyFoodLogEntries(userID UserID, fromDate time.Time, mealTags []string, loggedAt time.Time) (int, error) {
	entries, err := GetFoodLogEntries(userID, fromDate)
	if err != nil {
		return 0, fmt.Errorf("fetching source entries: %w", err)
	}

	tagSet := make(map[string]bool, len(mealTags))
	for _, t := range mealTags {
		tagSet[t] = true
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if !tagSet[entry.MealTag] {
			continue
		}
		newID, err := uuid.NewV7()
		if err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("generating uuid: %w", err)
		}
		_, err = tx.Exec(
			`INSERT INTO food_log_entries
			 (id, user_id, food_id, portion_name, calories, protein, carbs, fat, amount, meal_tag, note, copied_from_id, logged_at, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newID, userID, entry.FoodID, entry.PortionName,
			entry.Calories, entry.Protein, entry.Carbs, entry.Fat,
			entry.Amount, entry.MealTag, entry.Note, entry.ID,
			loggedAt, time.Now().UTC(),
		)
		if err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("inserting copied entry: %w", err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing transaction: %w", err)
	}
	return count, nil
}

func DeleteFoodLogEntry(id FoodLogEntryID, userID UserID) error {
	_, err := db.Exec("UPDATE food_log_entries SET deleted_at = ? WHERE id = ? AND user_id = ?", time.Now().UTC(), id, userID)
	if err != nil {
		return err
	}
	return nil
}
