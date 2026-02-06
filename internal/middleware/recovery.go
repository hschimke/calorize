package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer middleware recovers from panic and returns a 500 response.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				// Log the panic
				slog.Error("request panic",
					"error", rvr,
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
				)

				// Respond with 500
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
