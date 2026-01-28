package auth

import (
	"fmt"

	"github.com/go-webauthn/webauthn/webauthn"
)

var WebAuthn *webauthn.WebAuthn

func InitWebAuthn() error {
	var err error

	// Domain logic: In production this should be env var
	// Domain logic: In production this should be env var
	// For local dev: calorize.test
	origin := "https://calorize.test"
	wconfig := &webauthn.Config{
		RPDisplayName: "Calorize Local",
		RPID:          "calorize.test",
		RPOrigins:     []string{origin},
	}

	WebAuthn, err = webauthn.New(wconfig)
	if err != nil {
		return fmt.Errorf("failed to create webauthn: %w", err)
	}

	return nil
}
