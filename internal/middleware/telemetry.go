package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/google/uuid"
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
)

func generateRequestID() string {
	u, err := uuid.NewV7()
	if err == nil {
		return u.String()
	}
	// Fallback to random hex
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// RequestTelemetry middleware extracts or generates an X-Request-Id, sets response headers,
// and injects the request ID into the request context.
func RequestTelemetry(buildVersion string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get("X-Request-Id")
			if reqID == "" {
				reqID = generateRequestID()
			}

			w.Header().Set("X-Request-Id", reqID)
			if buildVersion != "" {
				w.Header().Set("X-Calorize-Build", buildVersion)
			}

			ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetRequestID retrieves the request ID from the context, or returns "" if absent.
func GetRequestID(ctx context.Context) string {
	if val := ctx.Value(RequestIDKey); val != nil {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}
