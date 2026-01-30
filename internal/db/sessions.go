package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Session
//
//	id (TEXT/UUID)
//	user_id (TEXT/UUID)
//	created_at
//	expires_at
type Session struct {
	ID        string    `json:"id"`
	UserID    UserID    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func CreateSession(userID UserID) (*Session, error) {
	// Create a new session ID (UUID)
	sessionID := uuid.New().String()
	now := time.Now()
	// Set expiry to 30 days for example
	expiresAt := now.Add(30 * 24 * time.Hour)

	query := `INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(query, sessionID, userID, now, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	return &Session{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}, nil
}

func GetSession(sessionID string) (*Session, error) {
	query := `SELECT id, user_id, created_at, expires_at FROM sessions WHERE id = ?`
	row := db.QueryRow(query, sessionID)

	var session Session
	err := row.Scan(&session.ID, &session.UserID, &session.CreatedAt, &session.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Return nil if session not found
		}
		return nil, fmt.Errorf("getting session: %w", err)
	}

	// Check expiry
	if time.Now().After(session.ExpiresAt) {
		_ = DeleteSession(sessionID) // Clean up expired session
		return nil, nil
	}

	return &session, nil
}

func DeleteSession(sessionID string) error {
	query := `DELETE FROM sessions WHERE id = ?`
	_, err := db.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}
