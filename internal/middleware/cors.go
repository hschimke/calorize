package middleware

import (
	"net/http"
	"os"
	"strings"
)

var allowedOrigins []string

func init() {
	if origins := os.Getenv("WEBAUTHN_RP_ORIGINS"); origins != "" {
		allowedOrigins = strings.Split(origins, ",")
	}
}

func isOriginAllowed(origin string) bool {
	if len(allowedOrigins) == 0 {
		return true // No restriction in dev mode
	}
	for _, o := range allowedOrigins {
		if strings.TrimSpace(o) == origin {
			return true
		}
	}
	return false
}

// CORS middleware adds necessary headers for Cross-Origin Resource Sharing.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin != "" && isOriginAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else if len(allowedOrigins) == 0 {
			// Dev fallback: allow all without credentials
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// If it's a preflight request, respond with 200 OK and return
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Otherwise, pass to the next handler
		next.ServeHTTP(w, r)
	})
}
