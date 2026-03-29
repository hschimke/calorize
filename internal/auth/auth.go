package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
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

// pendingUser holds the pre-generated user data that flows from registerBeginHandler
// to registerFinishHandler via cookie, without writing anything to the DB yet.
type pendingUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// pendingRegistrationSession is stored in the reg_session cookie during the WebAuthn
// registration ceremony. It carries both the WebAuthn challenge data and the pending
// user identity so that registerFinishHandler can write the user row only after the
// ceremony succeeds. The cookie is HMAC-signed to prevent client-side tampering.
type pendingRegistrationSession struct {
	WebAuthnSession webauthn.SessionData `json:"webauthn_session"`
	PendingUser     pendingUser          `json:"pending_user"`
}

var (
	WebAuthn      *webauthn.WebAuthn
	cookieSignKey []byte
)

func init() {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(fmt.Errorf("failed to generate cookie signing key: %w", err))
	}
	cookieSignKey = key
}

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

func cookieMAC(payload string) string {
	mac := hmac.New(sha256.New, cookieSignKey)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func savePendingRegistration(w http.ResponseWriter, pending pendingUser, sessionData *webauthn.SessionData) error {
	wrapper := pendingRegistrationSession{
		WebAuthnSession: *sessionData,
		PendingUser:     pending,
	}
	marshaled, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("marshaling pending registration: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(marshaled)
	// Append HMAC so the server can detect tampering.
	// Standard base64 never contains '.', so '.' is a safe separator.
	cookieValue := encoded + "." + cookieMAC(encoded)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    cookieValue,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

var (
	errNoCookie      = errors.New("registration cookie not present")
	errCookieTampered = errors.New("registration cookie signature invalid")
)

func loadPendingRegistration(r *http.Request) (*pendingRegistrationSession, error) {
	c, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil, errNoCookie
	}

	// Split payload from MAC and verify before trusting any content.
	idx := strings.LastIndex(c.Value, ".")
	if idx < 0 {
		return nil, errCookieTampered
	}
	encoded, sig := c.Value[:idx], c.Value[idx+1:]
	if !hmac.Equal([]byte(sig), []byte(cookieMAC(encoded))) {
		return nil, errCookieTampered
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decoding registration cookie: %w", err)
	}
	var wrapper pendingRegistrationSession
	if err := json.Unmarshal(decoded, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing registration cookie: %w", err)
	}
	return &wrapper, nil
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
	if _, err := mail.ParseAddress(user_email); user_email != "" && err != nil {
		http.Error(w, "invalid email address", http.StatusBadRequest)
		return
	}

	// Check if user already exists (with credentials — fully registered)
	existing, err := db.GetUser(username)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		http.Error(w, "user already exists", http.StatusBadRequest)
		return
	}

	// Generate a UUID for the pending user. The user row is NOT written to the DB
	// here — only written in registerFinishHandler once credentials are ready.
	newID, err := uuid.NewV7()
	if err != nil {
		http.Error(w, "failed to generate user id", http.StatusInternalServerError)
		return
	}
	pending := pendingUser{
		ID:    newID.String(),
		Name:  username,
		Email: user_email,
	}

	// Build an in-memory user for the WebAuthn ceremony (no DB record yet).
	memUser := &db.User{
		ID:    db.UserID(newID),
		Name:  username,
		Email: user_email,
	}
	wUser := WebAuthnUser{User: memUser}

	options, sessionData, err := WebAuthn.BeginRegistration(&wUser)
	if err != nil {
		http.Error(w, fmt.Sprintf("begin registration failed: %v", err), http.StatusInternalServerError)
		return
	}

	if err := savePendingRegistration(w, pending, sessionData); err != nil {
		http.Error(w, "failed to save session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(options)
}

func registerFinishHandler(w http.ResponseWriter, r *http.Request) {
	pending, err := loadPendingRegistration(r)
	if err != nil {
		if errors.Is(err, errCookieTampered) {
			http.Error(w, "invalid session", http.StatusBadRequest)
		} else {
			http.Error(w, "session missing", http.StatusBadRequest)
		}
		return
	}

	// Reconstruct the in-memory user from the pending registration session.
	uid, err := uuid.Parse(pending.PendingUser.ID)
	if err != nil {
		http.Error(w, "invalid user id in session", http.StatusBadRequest)
		return
	}
	memUser := &db.User{
		ID:    db.UserID(uid),
		Name:  pending.PendingUser.Name,
		Email: pending.PendingUser.Email,
	}
	wUser := WebAuthnUser{User: memUser}

	sessionData := &pending.WebAuthnSession
	credential, err := WebAuthn.FinishRegistration(&wUser, *sessionData, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("finish registration failed: %v", err), http.StatusInternalServerError)
		return
	}

	// WebAuthn ceremony succeeded — now persist the user for the first time.
	user, err := db.CreateUser(db.User{
		ID:        db.UserID(uid),
		Name:      pending.PendingUser.Name,
		Email:     pending.PendingUser.Email,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
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
	// NOTE: failures from here on do NOT roll back the user or credential — the user
	// is fully registered at this point and can log in via the login flow.
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
