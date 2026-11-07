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

// ExtractAuth identifies the caller if valid credentials (token, session cookie, or dev user)
// are present and injects the user_id into context. It does NOT block unauthenticated requests.
func ExtractAuth(devUserID *db.UserID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if devUserID != nil && *devUserID != db.UserID(uuid.Nil) {
				ctx := context.WithValue(r.Context(), auth.UserIDContextKey, *devUserID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			var userID db.UserID

			// 1. Check Bearer Token
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				if after, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
					tokenStr := strings.TrimSpace(after)
					if uid, err := token.Validate(tokenStr); err == nil {
						userID = uid
					}
				}
			}

			// 2. Check Cookie (if token auth failed or wasn't present)
			if userID == db.UserID(uuid.Nil) {
				if cookie, cErr := r.Cookie(auth.AppSessionCookieName); cErr == nil && cookie.Value != "" {
					if session, sErr := db.GetSession(cookie.Value); sErr == nil && session != nil {
						userID = session.UserID
					}
				}
			}

			// 3. Verify user exists and is active
			if userID != db.UserID(uuid.Nil) {
				if user, err := db.GetUserByID(userID); err == nil && user != nil && user.DisabledAt == nil {
					ctx := context.WithValue(r.Context(), auth.UserIDContextKey, userID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAuth middleware ensures the user was authenticated by ExtractAuth.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := r.Context().Value(auth.UserIDContextKey)
		if v == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		uid, ok := v.(db.UserID)
		if !ok || uid == db.UserID(uuid.Nil) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
