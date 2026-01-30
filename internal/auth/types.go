package auth

import (
	"azule.info/calorize/internal/db"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

type WebAuthnUser struct {
	*db.User
}

func (u *WebAuthnUser) WebAuthnID() []byte {
	// uuid from db.User
	// cast to uuid.UUID to access MarshalBinary
	id := uuid.UUID(u.User.ID)
	b, _ := id.MarshalBinary()
	return b
}

func (u *WebAuthnUser) WebAuthnName() string {
	return u.User.Name
}

func (u *WebAuthnUser) WebAuthnDisplayName() string {
	return u.User.Name
}

func (u *WebAuthnUser) WebAuthnIcon() string {
	return "" // Optional
}

func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	creds, err := db.GetUserCredentials(*u.User)
	if err != nil {
		return nil
	}

	var res []webauthn.Credential
	for _, c := range creds {
		var transports []protocol.AuthenticatorTransport
		for _, t := range c.Transports {
			transports = append(transports, protocol.AuthenticatorTransport(t))
		}

		res = append(res, webauthn.Credential{
			ID:              c.ID[:], // uuid.UUID is [16]byte, webauthn expects []byte. Slicing it works.
			PublicKey:       c.PublicKey,
			AttestationType: c.AttestationType,
			Transport:       transports,
			Flags: webauthn.CredentialFlags{
				UserPresent:    true, // simplified assumption or we need to store flags?
				UserVerified:   true, // simplified
				BackupEligible: c.BackupEligible,
				BackupState:    c.BackupState,
			},
			Authenticator: webauthn.Authenticator{
				AAGUID:    mustParseAAGUID(c.AAGUID),
				SignCount: c.SignCount,
				// CloneWarning: false, // Not stored in model
			},
		})
	}
	return res
}

func mustParseAAGUID(s string) []byte {
	// We stored it as string in DB.
	// We need to return []byte.
	// If it's a UUID string, parse it.
	u, err := uuid.Parse(s)
	if err == nil {
		return u[:]
	}
	return []byte(s) // Fallback if it wasn't a UUID
}
