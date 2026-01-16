package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

func CreateLog(ctx context.Context, l *Log) error {
	if l.ID == "" {
		l.ID = uuid.New().String()
	}
	l.CreatedAt = time.Now()

	_, err := DB.ExecContext(ctx, `
		INSERT INTO logs (id, user_id, food_id, amount, meal_tag, logged_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, l.ID, l.UserID, l.FoodID, l.Amount, l.MealTag, l.LoggedAt, l.CreatedAt)
	return err
}

func DeleteLog(ctx context.Context, id string) error {
	_, err := DB.ExecContext(ctx, "UPDATE logs SET deleted_at = ? WHERE id = ?", time.Now(), id)
	return err
}

func GetLogsRange(ctx context.Context, userID string, start, end time.Time) ([]Log, error) {
	// Join with foods to get details? Or just return IDs?
	// API usually needs hydrated data.
	// For now, return Logs. Frontend can fetch foods or we hydrate.
	// Let's just return Logs.

	rows, err := DB.QueryContext(ctx, `
		SELECT id, user_id, food_id, amount, meal_tag, logged_at, created_at 
		FROM logs 
		WHERE user_id = ? AND logged_at >= ? AND logged_at < ? AND deleted_at IS NULL
		ORDER BY logged_at ASC
	`, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []Log
	for rows.Next() {
		var l Log
		if err := rows.Scan(&l.ID, &l.UserID, &l.FoodID, &l.Amount, &l.MealTag, &l.LoggedAt, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}
