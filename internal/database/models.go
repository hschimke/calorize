package database

import (
	"database/sql"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// User represents the users table and implements webauthn.User
type User struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Email      string       `json:"email"`
	DisabledAt sql.NullTime `json:"disabled_at"`
	CreatedAt  time.Time    `json:"created_at"`

	Credentials []webauthn.Credential `json:"-"` // Loaded separately if needed
}

func (u *User) WebAuthnID() []byte {
	return []byte(u.ID)
}

func (u *User) WebAuthnName() string {
	return u.Name
}

func (u *User) WebAuthnDisplayName() string {
	return u.Name // User name as display name
}

func (u *User) WebAuthnIcon() string {
	return "" // Optional
}

func (u *User) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

// UserCredential represents the user_credentials table
type UserCredential struct {
	ID              string       `json:"id"` // Credential ID (base64 or hex)
	UserID          string       `json:"user_id"`
	Name            string       `json:"name"`
	PublicKey       []byte       `json:"public_key"`
	AttestationType string       `json:"attestation_type"`
	AAGUID          string       `json:"aaguid"`
	SignCount       uint32       `json:"sign_count"`
	Transports      string       `json:"transports"` // JSON
	CreatedAt       time.Time    `json:"created_at"`
	LastUsedAt      sql.NullTime `json:"last_used_at"`
}

type Food struct {
	ID                string       `json:"id"`
	FamilyID          string       `json:"family_id"`
	Version           int          `json:"version"`
	IsCurrent         bool         `json:"is_current"`
	Name              string       `json:"name"`
	Calories          float64      `json:"calories"`
	Protein           float64      `json:"protein"`
	Carbs             float64      `json:"carbs"`
	Fat               float64      `json:"fat"`
	Type              string       `json:"type"`
	MeasurementUnit   string       `json:"measurement_unit"`
	MeasurementAmount float64      `json:"measurement_amount"`
	CreatedAt         time.Time    `json:"created_at"`
	DeletedAt         sql.NullTime `json:"deleted_at"`

	Nutrients []Nutrient `json:"nutrients"`
}

type Nutrient struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}

type Log struct {
	ID        string       `json:"id"`
	UserID    string       `json:"user_id"`
	FoodID    string       `json:"food_id"`
	Amount    float64      `json:"amount"`
	MealTag   string       `json:"meal_tag"`
	LoggedAt  time.Time    `json:"logged_at"`
	CreatedAt time.Time    `json:"created_at"`
	DeletedAt sql.NullTime `json:"deleted_at"`
}
