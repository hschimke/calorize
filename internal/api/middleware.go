package api

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"azule.info/calorize/internal/database"
	"github.com/google/uuid"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// wrappedWriter wraps http.ResponseWriter to capture status code and bytes written
type wrappedWriter struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

func (w *wrappedWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *wrappedWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// RequestLogger logs request details
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := uuid.New().String()

		ww := &wrappedWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default
		}

		// Add RequestID to context? Optional, but good practice.
		// For now just logging it.
		// r.Header.Set("X-Request-ID", requestID) // Don't modify request headers usually, maybe response headers
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(ww, r)

		slog.Info("Request handled",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
			"status", ww.statusCode,
			"bytes", ww.bytes,
			"duration", time.Since(start).String(),
		)
	})
}

// Recoverer recovers from panics and logs the error
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("Panic recovered",
					"error", err,
					"stack", string(debug.Stack()),
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

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
			slog.Error("Database error checking session", "error", err)
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
