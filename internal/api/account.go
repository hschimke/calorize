package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/db"
	"github.com/google/uuid"
)

type passkeyView struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
}

func RegisterAccountPaths(mux *http.ServeMux) {
	mux.HandleFunc("GET /account/passkeys", listPasskeysHandler)
	mux.HandleFunc("DELETE /account/passkeys/{id}", deletePasskeyHandler)
	mux.HandleFunc("PATCH /account/passkeys/{id}", renamePasskeyHandler)
	mux.HandleFunc("POST /account/passkeys/begin", addPasskeyBeginHandler)
	mux.HandleFunc("POST /account/passkeys/finish", addPasskeyFinishHandler)
}

func listPasskeysHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	creds, err := db.GetUserCredentials(*user)
	if err != nil {
		slog.Error("failed to get user credentials", "error", err)
		http.Error(w, "Failed to get passkeys", http.StatusInternalServerError)
		return
	}

	views := make([]passkeyView, 0, len(creds))
	for _, c := range creds {
		views = append(views, passkeyView{
			ID:         base64.RawURLEncoding.EncodeToString(c.ID),
			Name:       c.Name,
			CreatedAt:  c.CreatedAt,
			LastUsedAt: c.LastUsedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(views)
}

func deletePasskeyHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	credIDBytes, err := base64.RawURLEncoding.DecodeString(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid passkey ID", http.StatusBadRequest)
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	creds, err := db.GetUserCredentials(*user)
	if err != nil {
		http.Error(w, "Failed to get passkeys", http.StatusInternalServerError)
		return
	}
	if len(creds) <= 1 {
		http.Error(w, "Cannot delete the only passkey", http.StatusBadRequest)
		return
	}

	if err := db.RemoveUserCredential(*user, db.UserCredential{ID: db.UserCredentialID(credIDBytes)}); err != nil {
		if errors.Is(err, db.ErrCredentialNotFound) {
			http.Error(w, "Passkey not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to remove credential", "error", err)
		http.Error(w, "Failed to delete passkey", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func renamePasskeyHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	credIDBytes, err := base64.RawURLEncoding.DecodeString(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid passkey ID", http.StatusBadRequest)
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if err := db.RenameCredential(userID, db.UserCredentialID(credIDBytes), body.Name); err != nil {
		if errors.Is(err, db.ErrCredentialNotFound) {
			http.Error(w, "Passkey not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to rename credential", "error", err)
		http.Error(w, "Failed to rename passkey", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func addPasskeyBeginHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	wUser := auth.WebAuthnUser{User: user}
	options, sessionData, err := auth.WebAuthn.BeginRegistration(&wUser)
	if err != nil {
		slog.Error("begin registration failed", "error", err)
		http.Error(w, "Failed to begin passkey registration", http.StatusInternalServerError)
		return
	}

	if err := auth.SaveSession(w, sessionData); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(options)
}

func addPasskeyFinishHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserID(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionData, err := auth.LoadSession(r)
	if err != nil {
		http.Error(w, "Session missing or expired", http.StatusBadRequest)
		return
	}

	user, err := db.GetUserByID(userID)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	wUser := auth.WebAuthnUser{User: user}
	credential, err := auth.WebAuthn.FinishRegistration(&wUser, *sessionData, r)
	if err != nil {
		slog.Error("finish registration failed", "error", err)
		http.Error(w, "Failed to finish passkey registration", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	if err := db.AddUserCredential(*user, db.UserCredential{
		ID:              db.UserCredentialID(credential.ID),
		Name:            "New Passkey",
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		AAGUID:          uuid.UUID(credential.Authenticator.AAGUID).String(),
		SignCount:       credential.Authenticator.SignCount,
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
		CreatedAt:       now,
		LastUsedAt:      now,
	}); err != nil {
		slog.Error("failed to save new passkey", "error", err)
		http.Error(w, "Failed to save passkey", http.StatusInternalServerError)
		return
	}

	auth.ClearSession(w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Passkey added"})
}
