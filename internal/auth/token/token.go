package token

import (
	"time"

	paseto "aidanwoods.dev/go-paseto/v2"
	"azule.info/calorize/internal/db"
	"github.com/google/uuid"
)

var secretKey paseto.V4SymmetricKey

func init() {
	// TODO: Load from environment variable in production
	secretKey = paseto.NewV4SymmetricKey()
}

// Generate creates a new PASETO v4 local token for the given user
func Generate(userID db.UserID) (string, error) {
	token := paseto.NewToken()
	token.SetIssuedAt(time.Now())
	token.SetNotBefore(time.Now())
	token.SetExpiration(time.Now().Add(24 * time.Hour))

	// Convert UUID to string
	uid := uuid.UUID(userID)
	token.SetString("user_id", uid.String())

	return token.V4Encrypt(secretKey, nil), nil
}

// Validate parses and validates a PASETO v4 local token
func Validate(tokenString string) (db.UserID, error) {
	parser := paseto.NewParser()
	parser.AddRule(paseto.NotExpired())

	token, err := parser.ParseV4Local(secretKey, tokenString, nil)
	if err != nil {
		return db.UserID(uuid.Nil), err
	}

	userIDStr, err := token.GetString("user_id")
	if err != nil {
		return db.UserID(uuid.Nil), err
	}

	uid, err := uuid.Parse(userIDStr)
	if err != nil {
		return db.UserID(uuid.Nil), err
	}

	return db.UserID(uid), nil
}
