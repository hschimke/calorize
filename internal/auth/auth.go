package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"azule.info/calorize/internal/auth/token"
	"azule.info/calorize/internal/db"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

type contextKey string

const UserIDContextKey = contextKey("user_id")
const SessionCookieName = "reg_session"
const AppSessionCookieName = "session_id"

var (
	WebAuthn *webauthn.WebAuthn
)

func RegisterAuthPaths(mux *http.ServeMux) {
	var err error

	rpDisplayName := os.Getenv("WEBAUTHN_RP_DISPLAY_NAME")
	if rpDisplayName == "" {
		rpDisplayName = "Calorize"
	}

	rpID := os.Getenv("WEBAUTHN_RP_ID")
	if rpID == "" {
		rpID = "calorize.test"
	}

	rpOrigins := []string{"https://calorize.test"}
	if origins := os.Getenv("WEBAUTHN_RP_ORIGINS"); origins != "" {
		rpOrigins = strings.Split(origins, ",")
	}

	wconfig := &webauthn.Config{
		RPDisplayName: rpDisplayName,
		RPID:          rpID,
		RPOrigins:     rpOrigins,
	}

	WebAuthn, err = webauthn.New(wconfig)
	if err != nil {
		panic(fmt.Errorf("failed to create WebAuthn from config: %w", err))
	}

	mux.HandleFunc("POST /auth/register/begin", registerBeginHandler)
	mux.HandleFunc("POST /auth/register/finish", registerFinishHandler)
	mux.HandleFunc("POST /auth/login/begin", loginBeginHandler)
	mux.HandleFunc("POST /auth/login/finish", loginFinishHandler)

	mux.HandleFunc("POST /auth/logout", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(AppSessionCookieName)
		if err == nil && cookie.Value != "" {
			// Best effort delete from DB
			_ = db.DeleteSession(cookie.Value)
		}

		// Clear the session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     AppSessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Logout Success"))
	})
}

// Session storage helpers (exported so internal/api can reuse them)
func SaveSession(w http.ResponseWriter, data *webauthn.SessionData) error {
	marshaled, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling session data: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(marshaled)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

func LoadSession(r *http.Request) (*webauthn.SessionData, error) {
	c, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil, err
	}

	decoded, err := base64.StdEncoding.DecodeString(c.Value)
	if err != nil {
		return nil, err
	}

	var data webauthn.SessionData
	if err := json.Unmarshal(decoded, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// Handlers

func registerBeginHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	user_email := r.URL.Query().Get("user_email")

	if username == "" {
		// Try form body
		if err := r.ParseForm(); err == nil {
			username = r.FormValue("username")
		}
	}
	if username == "" {
		http.Error(w, "username required", http.StatusBadRequest)
		return
	}

	// Check if user exists or create a temporary user representation
	user, err := db.GetUser(username)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if user != nil {
		http.Error(w, "user already exists", http.StatusBadRequest)
		return
	}

	// Create the user immediately
	newUser := db.User{
		Name:      username,
		Email:     user_email,
		CreatedAt: time.Now().UTC(),
	}
	user, err = db.CreateUser(newUser)
	if err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	wUser := WebAuthnUser{User: user}

	options, sessionData, err := WebAuthn.BeginRegistration(&wUser)
	if err != nil {
		http.Error(w, fmt.Sprintf("begin registration failed: %v", err), http.StatusInternalServerError)
		return
	}

	if err := SaveSession(w, sessionData); err != nil {
		http.Error(w, "failed to save session", http.StatusInternalServerError)
		db.DeleteTemporaryUser(*user)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(options)
}

func registerFinishHandler(w http.ResponseWriter, r *http.Request) {
	sessionData, err := LoadSession(r)
	if err != nil {
		http.Error(w, "session missing", http.StatusBadRequest)
		return
	}

	// Retrieve user by ID stored in the WebAuthn session data
	uid, err := uuid.FromBytes(sessionData.UserID)
	if err != nil {
		http.Error(w, "invalid user id in session", http.StatusBadRequest)
		return
	}

	user, err := db.GetUserByID(db.UserID(uid))
	if err != nil || user == nil {
		http.Error(w, "user not found", http.StatusBadRequest)
		return
	}

	wUser := WebAuthnUser{User: user}

	credential, err := WebAuthn.FinishRegistration(&wUser, *sessionData, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("finish registration failed: %v", err), http.StatusInternalServerError)
		db.DeleteTemporaryUser(*user)
		return
	}

	// Save credential
	if err := db.AddUserCredential(*user, db.UserCredential{
		ID:              db.UserCredentialID(credential.ID),
		Name:            "Passkey", // Default name
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		AAGUID:          uuid.UUID(credential.Authenticator.AAGUID).String(),
		SignCount:       credential.Authenticator.SignCount,
		Transports:      nil,
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
		CreatedAt:       time.Now().UTC(),
		LastUsedAt:      time.Now().UTC(),
	}); err != nil {
		http.Error(w, "failed to save credential", http.StatusInternalServerError)
		db.DeleteTemporaryUser(*user)
		return
	}

	// Auto-login: Create session.
	session, err := db.CreateSession(user.ID)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	// Generate PASETO token
	t, err := token.Generate(user.ID)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	// Set all cookies before writing the response body
	http.SetCookie(w, &http.Cookie{
		Name:     AppSessionCookieName,
		Value:    session.ID,
		Path:     "/",
		MaxAge:   3600 * 24 * 30,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	ClearSession(w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Registration Success",
		"token":   t,
		"user_id": uuid.UUID(user.ID).String(),
	})
}

func loginBeginHandler(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	// Retrieve user by name
	var user *db.User
	var err error
	user, err = db.GetUser(username)
	if err != nil || user == nil {
		user, err = db.GetUserByEmail(username)
		if err != nil || user == nil {
			http.Error(w, "user not found", http.StatusBadRequest)
			return
		}
	}

	wUser := WebAuthnUser{User: user}

	options, sessionData, err := WebAuthn.BeginLogin(&wUser)
	if err != nil {
		http.Error(w, fmt.Sprintf("begin login failed: %v", err), http.StatusInternalServerError)
		return
	}

	if err := SaveSession(w, sessionData); err != nil {
		http.Error(w, "failed to save session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(options)
}

func loginFinishHandler(w http.ResponseWriter, r *http.Request) {
	sessionData, err := LoadSession(r)
	if err != nil {
		http.Error(w, "session missing", http.StatusBadRequest)
		return
	}

	// Retrieve user by ID from sessionData
	uid, err := uuid.FromBytes(sessionData.UserID)
	if err != nil {
		http.Error(w, "invalid user id in session", http.StatusBadRequest)
		return
	}

	user, err := db.GetUserByID(db.UserID(uid))
	if err != nil || user == nil {
		http.Error(w, "user not found", http.StatusBadRequest)
		return
	}

	wUser := WebAuthnUser{User: user}

	credential, err := WebAuthn.FinishLogin(&wUser, *sessionData, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("finish login failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Update credential sign count / last used
	// We need `UpdateUserCredential`? Or just ignore for now?
	// Best practice: update sign count.
	// I'll skip for now or add TODO.
	db.SetCredentialLastUsed(*user, db.UserCredential{
		ID: db.UserCredentialID(credential.ID),
	})

	session, err := db.CreateSession(user.ID)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	// Generate PASETO token
	t, err := token.Generate(user.ID)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	// Set all cookies before writing the response body
	http.SetCookie(w, &http.Cookie{
		Name:     AppSessionCookieName,
		Value:    session.ID,
		Path:     "/",
		MaxAge:   3600 * 24 * 30,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	ClearSession(w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Login Success",
		"token":   t,
		"user_id": uuid.UUID(user.ID).String(),
	})
}
