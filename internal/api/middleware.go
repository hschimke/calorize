package api

import (
	"context"
	"net/http"
	"time"

	"azule.info/calorize/internal/database"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// AuthMiddleware validates the session cookie and populates the context with UserID.
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := cookie.Value
		session, err := database.GetSessionByToken(r.Context(), token)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if session == nil {
			// Invalid or expired
			// Clear cookie
			http.SetCookie(w, &http.Cookie{
				Name:     "session_token",
				Value:    "",
				Path:     "/",
				Expires:  time.Unix(0, 0),
				HttpOnly: true,
			})
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Add UserID to context
		ctx := context.WithValue(r.Context(), UserIDKey, session.UserID)
		next(w, r.WithContext(ctx))
	}
}

// GetUserID retrieves the user ID from the context.
func GetUserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(UserIDKey).(string)
	return id, ok
}
