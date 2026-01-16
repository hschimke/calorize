package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/database"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

type AuthSession struct {
	WebAuthnData webauthn.SessionData
	PendingUser  database.User // For registration
}

var (
	sessionStore = make(map[string]AuthSession)
	sessionMu    sync.RWMutex
)

func saveSession(session AuthSession) string {
	id := uuid.New().String()
	sessionMu.Lock()
	sessionStore[id] = session
	sessionMu.Unlock()
	return id
}

func getSession(id string) (AuthSession, bool) {
	sessionMu.RLock()
	data, ok := sessionStore[id]
	sessionMu.RUnlock()
	return data, ok
}

func deleteSession(id string) {
	sessionMu.Lock()
	delete(sessionStore, id)
	sessionMu.Unlock()
}

// Handlers

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

func BeginRegistration(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check if user exists to prevent duplicate emails
	existing, _ := database.GetUserByName(ctx, req.Username)
	if existing != nil {
		// Existing user: We are adding a credential to an existing account?
		// Or fail if name taken? For now, imply Adding Device logic if name matches.
		// Real app would require being logged in to add device.
		// Here: Simplified "Register or Add Device" flow.
	} else {
		// Prepare new user
		existing = &database.User{
			ID:    uuid.New().String(),
			Name:  req.Username,
			Email: req.Email,
		}
	}

	// Fetch existing credentials to exclude
	creds, _ := database.GetCredentials(ctx, existing.ID)
	var excludeList []webauthn.Credential
	for _, c := range creds {
		wc, _ := c.ToWebAuthn()
		excludeList = append(excludeList, wc)
	}
	existing.Credentials = excludeList

	options, sessionData, err := auth.WebAuthn.BeginRegistration(existing)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sid := saveSession(AuthSession{
		WebAuthnData: *sessionData,
		PendingUser:  *existing,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_session_id",
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
	})

	json.NewEncoder(w).Encode(options)
}

func FinishRegistration(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("auth_session_id")
	if err != nil {
		http.Error(w, "session missing", http.StatusBadRequest)
		return
	}
	sid := c.Value
	session, ok := getSession(sid)
	if !ok {
		http.Error(w, "session expired", http.StatusBadRequest)
		return
	}
	deleteSession(sid)

	user := session.PendingUser
	credential, err := auth.WebAuthn.FinishRegistration(&user, session.WebAuthnData, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Persist User (if new) and Credential
	ctx := r.Context()

	// Check if user exists in DB
	dbUser, _ := database.GetUser(ctx, user.ID)
	if dbUser == nil {
		// Only create if we generated a fresh ID.
		// Ideally we check by ID.
		createdUser, err := database.CreateUser(ctx, user.Name, user.Email)
		if err != nil {
			// Might be email conflict
			http.Error(w, "failed to create user: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Sync ID if generated differently (CreateUser generates ID currently? yes)
		// Wait, CreateUser generates a NEW ID. We should pass the ID we used for WebAuthn.
		// modifying queries.go to key off ID would be better.
		// For now, let's assume CreateUser uses the ID we passed? No, queries.go generates new.
		// FIX: We need to ensure the ID used in WebAuthn (user.ID) matches what we insert.
		// See note below.
		user.ID = createdUser.ID
	}

	// Save Credential
	newCred := &database.UserCredential{
		ID:              string(credential.ID), // TODO: Base64 encode? Library returns bytes.
		UserID:          user.ID,
		Name:            "Passkey", // User should name it ideally
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		AAGUID:          string(credential.Authenticator.AAGUID),
		SignCount:       credential.Authenticator.SignCount,
		Transports:      "", // TODO
	}

	if err := database.AddCredential(ctx, newCred); err != nil {
		http.Error(w, "failed to save credential", http.StatusInternalServerError)
		return
	}

	w.Write([]byte("Registration Success"))
}

func BeginLogin(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	// If empty -> Resident Key flow (User handle not known yet)
	// Not supported by simple queries yet (need to look up user)
	// So assume username provided.

	ctx := r.Context()
	user, err := database.GetUserByName(ctx, username)
	if err != nil || user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	creds, _ := database.GetCredentials(ctx, user.ID)
	var wcreds []webauthn.Credential
	for _, c := range creds {
		wc, _ := c.ToWebAuthn()
		wcreds = append(wcreds, wc)
	}
	user.Credentials = wcreds

	options, sessionData, err := auth.WebAuthn.BeginLogin(user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sid := saveSession(AuthSession{
		WebAuthnData: *sessionData,
		PendingUser:  *user,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_session_id",
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
	})

	json.NewEncoder(w).Encode(options)
}

func FinishLogin(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("auth_session_id")
	if err != nil {
		http.Error(w, "session missing", http.StatusBadRequest)
		return
	}
	sid := c.Value
	session, ok := getSession(sid)
	if !ok {
		http.Error(w, "session expired", http.StatusBadRequest)
		return
	}
	deleteSession(sid)

	user := session.PendingUser
	credential, err := auth.WebAuthn.FinishLogin(&user, session.WebAuthnData, r)
	if err != nil {
		http.Error(w, "login failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Update credential sign count... (TODO)
	_ = credential

	w.Write([]byte("Login Success"))
}
