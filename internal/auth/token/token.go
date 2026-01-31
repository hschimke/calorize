package token

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	paseto "aidanwoods.dev/go-paseto/v2"
	"azule.info/calorize/internal/db"
	"github.com/google/uuid"
)

var secretKey paseto.V4SymmetricKey

func init() {
	keyHex := os.Getenv("PASETO_SECRET_KEY")
	if keyHex == "" {
		// Default dev key for local development convenience - DO NOT USE IN PRODUCTION
		// This ensures existing dev workflows don't break immediately
		slog.Warn("PASETO_SECRET_KEY not set - using insecure dev key")
		// Hardcoded "random" hex string for dev
		keyHex = "MANATEES ARE GREAT__AND You know it or YOU ARE A LIAR!!!!!!!!!!!"
	}

	var err error
	secretKey, err = paseto.V4SymmetricKeyFromHex(keyHex)
	if err != nil {
		panic(fmt.Errorf("invalid PASETO_SECRET_KEY hex: %w", err))
	}
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
