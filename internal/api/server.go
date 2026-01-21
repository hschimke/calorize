package api

import (
	"net/http"

	"azule.info/calorize/internal/auth"
)

func NewServer() *http.ServeMux {
	mux := http.NewServeMux()

	// Initialize WebAuthn
	if err := auth.InitWebAuthn(); err != nil {
		panic(err)
	}

	// Middleware (Logging, CORS, etc) - Skipping for MVP

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Auth Routes
	mux.HandleFunc("POST /auth/register/begin", BeginRegistration)
	mux.HandleFunc("POST /auth/register/finish", FinishRegistration)
	mux.HandleFunc("POST /auth/login/begin", BeginLogin)
	mux.HandleFunc("POST /auth/login/finish", FinishLogin)

	// Food Routes
	mux.HandleFunc("GET /foods", AuthMiddleware(GetFoods))
	mux.HandleFunc("POST /foods", AuthMiddleware(CreateFood))
	mux.HandleFunc("GET /foods/{id}", AuthMiddleware(GetFood))
	mux.HandleFunc("PUT /foods/{id}", AuthMiddleware(UpdateFood))
	mux.HandleFunc("DELETE /foods/{id}", AuthMiddleware(DeleteFood))

	// Log Routes
	mux.HandleFunc("GET /logs", AuthMiddleware(GetLogs))
	mux.HandleFunc("POST /logs", AuthMiddleware(CreateLog))
	mux.HandleFunc("DELETE /logs/{id}", AuthMiddleware(DeleteLog))

	// Stats Routes
	mux.HandleFunc("GET /stats", AuthMiddleware(GetStats))

	// Static Web
	fs := http.FileServer(http.Dir("./static-web"))
	// StripPrefix is usually needed if we mount on a subpath, but at root it's fine.
	// However, to ensure we don't conflict with API routes if we had overlapping names.
	// Since API routes are explicit "GET /foo", this catch-all "/" works for index.html etc.
	mux.Handle("/", fs)

	return mux
}
