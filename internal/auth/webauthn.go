package auth

import (
	"fmt"

	"github.com/go-webauthn/webauthn/webauthn"
)

var WebAuthn *webauthn.WebAuthn

func InitWebAuthn() error {
	var err error

	// Domain logic: In production this should be env var
	// For local dev: localhost
	origin := "http://localhost:8080"
	wconfig := &webauthn.Config{
		RPDisplayName: "Calorize",
		RPID:          "localhost",
		RPOrigins:     []string{origin},
	}

	WebAuthn, err = webauthn.New(wconfig)
	if err != nil {
		return fmt.Errorf("failed to create webauthn: %w", err)
	}

	return nil
}
