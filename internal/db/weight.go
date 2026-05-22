package db

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

func GetWeightLogs(userID UserID, limit int) ([]WeightLog, error) {
	var query string
	var rows interface {
		Next() bool
		Scan(...any) error
		Close() error
		Err() error
	}
	var err error

	if limit > 0 {
		query = `
			SELECT id, user_id, weight, unit, logged_at, created_at, deleted_at
			FROM weight_logs
			WHERE user_id = ? AND deleted_at IS NULL
			ORDER BY logged_at DESC, created_at DESC
			LIMIT ?
		`
		rows, err = db.Query(query, userID, limit)
	} else {
		query = `
			SELECT id, user_id, weight, unit, logged_at, created_at, deleted_at
			FROM weight_logs
			WHERE user_id = ? AND deleted_at IS NULL
			ORDER BY logged_at DESC, created_at DESC
		`
		rows, err = db.Query(query, userID)
	}

	if err != nil {
		return nil, fmt.Errorf("listing weight logs: %w", err)
	}
	defer rows.Close()

	var logs []WeightLog
	for rows.Next() {
		var log WeightLog
		err := rows.Scan(&log.ID, &log.UserID, &log.Weight, &log.Unit, &log.LoggedAt, &log.CreatedAt, &log.DeletedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning weight log: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating weight logs: %w", err)
	}

	if logs == nil {
		logs = []WeightLog{}
	}
	return logs, nil
}

func GetWeightLog(id WeightLogID) (*WeightLog, error) {
	query := `
		SELECT id, user_id, weight, unit, logged_at, created_at, deleted_at
		FROM weight_logs
		WHERE id = ? AND deleted_at IS NULL
	`
	row := db.QueryRow(query, id)
	var log WeightLog
	err := row.Scan(&log.ID, &log.UserID, &log.Weight, &log.Unit, &log.LoggedAt, &log.CreatedAt, &log.DeletedAt)
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func CreateWeightLog(log WeightLog) (*WeightLog, error) {
	newID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}

	if log.LoggedAt.IsZero() {
		log.LoggedAt = time.Now().UTC()
	}

	_, err = db.Exec(`
		INSERT INTO weight_logs (id, user_id, weight, unit, logged_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		newID, log.UserID, log.Weight, log.Unit, log.LoggedAt, log.CreatedAt)
	if err != nil {
		return nil, err
	}

	log.ID = WeightLogID(newID)
	return &log, nil
}

func UpdateWeightLog(id WeightLogID, log WeightLog) (*WeightLog, error) {
	_, err := db.Exec(`
		UPDATE weight_logs
		SET weight = ?, unit = ?, logged_at = ?
		WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		log.Weight, log.Unit, log.LoggedAt, id, log.UserID)
	if err != nil {
		return nil, err
	}
	log.ID = id
	return &log, nil
}

func DeleteWeightLog(id WeightLogID, userID UserID) error {
	_, err := db.Exec(`
		UPDATE weight_logs
		SET deleted_at = ?
		WHERE id = ? AND user_id = ?`,
		time.Now().UTC(), id, userID)
	return err
}
