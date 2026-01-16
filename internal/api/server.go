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
	mux.HandleFunc("GET /foods", GetFoods)
	mux.HandleFunc("POST /foods", CreateFood)
	mux.HandleFunc("GET /foods/{id}", GetFood)

	// Log Routes
	mux.HandleFunc("GET /logs", GetLogs)
	mux.HandleFunc("POST /logs", CreateLog)
	mux.HandleFunc("DELETE /logs/{id}", DeleteLog)

	return mux
}
