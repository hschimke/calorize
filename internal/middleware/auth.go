package middleware

import (
	"context"
	"net/http"
	"strings"

	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/auth/token"
	"azule.info/calorize/internal/db"
	"github.com/google/uuid"
)

// RequireAuth middleware ensures the user is authenticated via Bearer Token OR Cookie
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userID db.UserID
		var err error

		// 1. Check Bearer Token
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				var uid db.UserID
				uid, err = token.Validate(tokenStr)
				if err == nil {
					userID = uid
				}
			}
		}

		// 2. Check Cookie (if token auth failed or wasn't present)
		if authHeader != "" && userID == db.UserID(uuid.Nil) {
			// Token provided but invalid
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if userID == db.UserID(uuid.Nil) {
			// No token or no header, check Cookie
			cookie, cErr := r.Cookie(auth.AppSessionCookieName)
			if cErr == nil && cookie.Value != "" {
				session, sErr := db.GetSession(cookie.Value)
				if sErr == nil && session != nil {
					userID = session.UserID
				}
			}
		}

		if userID == db.UserID(uuid.Nil) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 3. Verify user exists and is active
		user, err := db.GetUserByID(userID)
		if err != nil {
			// Log error? For now just unauthorized
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if user == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if user.DisabledAt != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Set user_id in context
		ctx := context.WithValue(r.Context(), auth.UserIDContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
