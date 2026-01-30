package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Bare user functions
func GetUser(userName string) (*User, error) {
	query := `SELECT id, name, email, disabled_at, created_at FROM users WHERE name = ?`
	row := db.QueryRow(query, userName)

	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.DisabledAt, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Return nil if user not found, typical pattern or could return error
		}
		return nil, fmt.Errorf("getting user: %w", err)
	}
	return &user, nil
}

func GetUserByID(id UserID) (*User, error) {
	query := `SELECT id, name, email, disabled_at, created_at FROM users WHERE id = ?`
	row := db.QueryRow(query, id)

	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.DisabledAt, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return &user, nil
}

func CreateUser(user User) (*User, error) {
	if user.ID == UserID(uuid.Nil) {
		newID, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("creating user: %w", err)
		}
		user.ID = UserID(newID)
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}

	query := `INSERT INTO users (id, name, email, disabled_at, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err := db.Exec(query, user.ID, user.Name, user.Email, user.DisabledAt, user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	return &user, nil
}

func UpdateUser(user User) (*User, error) {
	query := `UPDATE users SET name = ?, email = ?, disabled_at = ? WHERE id = ?`
	_, err := db.Exec(query, user.Name, user.Email, user.DisabledAt, user.ID)
	if err != nil {
		return nil, fmt.Errorf("updating user: %w", err)
	}
	return &user, nil
}

func DeleteUser(user User) error {
	query := `DELETE FROM users WHERE id = ?`
	_, err := db.Exec(query, user.ID)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	return nil
}

// User Auth functions
func AddUserCredential(user User, auth UserCredential) error {
	if len(auth.ID) == 0 {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("adding user credential: %w", err)
		}
		auth.ID = UserCredentialID(id[:])
	}
	if auth.CreatedAt.IsZero() {
		auth.CreatedAt = time.Now()
	}
	if auth.LastUsedAt.IsZero() {
		auth.LastUsedAt = time.Now() // Initialize to now
	}

	transportsJSON, err := json.Marshal(auth.Transports)
	if err != nil {
		return fmt.Errorf("marshaling transports: %w", err)
	}

	query := `INSERT INTO user_credentials (
		id, user_id, name, public_key, attestation_type, aaguid, sign_count, transports, backup_eligible, backup_state, created_at, last_used_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = db.Exec(query,
		auth.ID,
		user.ID,
		auth.Name,
		auth.PublicKey,
		auth.AttestationType,
		auth.AAGUID,
		auth.SignCount,
		string(transportsJSON),
		auth.BackupEligible,
		auth.BackupState,
		auth.CreatedAt,
		auth.LastUsedAt,
	)
	if err != nil {
		return fmt.Errorf("adding user credential: %w", err)
	}
	return nil
}

func RemoveUserCredential(user User, auth UserCredential) error {
	query := `DELETE FROM user_credentials WHERE id = ? AND user_id = ?`
	_, err := db.Exec(query, auth.ID, user.ID)
	if err != nil {
		return fmt.Errorf("removing user credential: %w", err)
	}
	return nil
}

func GetUserCredentials(user User) ([]UserCredential, error) {
	query := `SELECT id, user_id, name, public_key, attestation_type, aaguid, sign_count, transports, backup_eligible, backup_state, created_at, last_used_at FROM user_credentials WHERE user_id = ?`
	rows, err := db.Query(query, user.ID)
	if err != nil {
		return nil, fmt.Errorf("getting user credentials: %w", err)
	}
	defer rows.Close()

	var credentials []UserCredential
	for rows.Next() {
		var c UserCredential
		var transportsRaw []byte // Scan into generic bytes or string
		err := rows.Scan(
			&c.ID,
			&c.UserID,
			&c.Name,
			&c.PublicKey,
			&c.AttestationType,
			&c.AAGUID,
			&c.SignCount,
			&transportsRaw,
			&c.BackupEligible,
			&c.BackupState,
			&c.CreatedAt,
			&c.LastUsedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning credential: %w", err)
		}

		if len(transportsRaw) > 0 {
			if err := json.Unmarshal(transportsRaw, &c.Transports); err != nil {
				// Just log or ignore? Better to fail or default to empty?
				// For now, let's treat it as empty if error, but logging would be better.
				// c.Transports will remain nil/empty
			}
		}

		credentials = append(credentials, c)
	}
	return credentials, nil
}

func SetCredentialLastUsed(user User, auth UserCredential) error {
	query := `UPDATE user_credentials SET last_used_at = ? WHERE id = ? AND user_id = ?`
	_, err := db.Exec(query, time.Now(), auth.ID, user.ID)
	if err != nil {
		return fmt.Errorf("setting credential last used: %w", err)
	}
	return nil
}
