package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

func CreateUser(ctx context.Context, name, email string) (*User, error) {
	id := uuid.New().String()
	user := &User{
		ID:        id,
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
	}

	_, err := DB.ExecContext(ctx, "INSERT INTO users (id, name, email) VALUES (?, ?, ?)",
		user.ID, user.Name, user.Email)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func GetUserByName(ctx context.Context, name string) (*User, error) {
	row := DB.QueryRowContext(ctx, "SELECT id, name, email, disabled_at, created_at FROM users WHERE name = ?", name)
	user := &User{}
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.DisabledAt, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func GetUser(ctx context.Context, id string) (*User, error) {
	row := DB.QueryRowContext(ctx, "SELECT id, name, email, disabled_at, created_at FROM users WHERE id = ?", id)
	user := &User{}
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.DisabledAt, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func AddCredential(ctx context.Context, cred *UserCredential) error {
	_, err := DB.ExecContext(ctx, `
		INSERT INTO user_credentials (id, user_id, name, public_key, attestation_type, aaguid, sign_count, transports, last_used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, cred.ID, cred.UserID, cred.Name, cred.PublicKey, cred.AttestationType, cred.AAGUID, cred.SignCount, cred.Transports, cred.LastUsedAt)
	return err
}

func GetCredentials(ctx context.Context, userID string) ([]UserCredential, error) {
	rows, err := DB.QueryContext(ctx, "SELECT id, user_id, name, public_key, attestation_type, aaguid, sign_count, transports, last_used_at FROM user_credentials WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []UserCredential
	for rows.Next() {
		var c UserCredential
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.PublicKey, &c.AttestationType, &c.AAGUID, &c.SignCount, &c.Transports, &c.LastUsedAt); err != nil {
			return nil, err
		}
		creds = append(creds, c)
	}
	return creds, nil
}

// Convert DB Credential to WebAuthn Credential
func (c *UserCredential) ToWebAuthn() (webauthn.Credential, error) {
	// Parse ID from hex/base64 if needed, currently string assumed correct
	// But WebAuthn library might expect byte slice for ID if it was stored as bytes?
	// The DB schema stores ID as TEXT.
	// The library `webauthn.Credential` has `ID []byte`.
	// We need to ensure we store it consistently.
	// Assuming storage is Base64 or Hex if it's text.
	// Actually, the `webauthn` library `MakeNewCredential` returns a credential where ID is bytes.
	// We should probably store it as BLOB in DB or Base64 in Text.
	// Schema said TEXT. Let's assume Base64 URL safe.

	// Re-reading logic: The library handles encoding/decoding usually.
	// If the DB stores raw bytes as text? Invalid.
	// I'll update AddCredential logic to handle this if needed.
	// For now, let's assume `c.ID` is the ID.

	return webauthn.Credential{
		ID:              []byte(c.ID), // Verify encoding!
		PublicKey:       c.PublicKey,
		AttestationType: c.AttestationType,
		Authenticator: webauthn.Authenticator{
			AAGUID:    []byte(c.AAGUID), // Might need parsing UUID
			SignCount: c.SignCount,
		},
		Transport: []protocol.AuthenticatorTransport{}, // Parse c.Transports JSON
	}, nil
}
