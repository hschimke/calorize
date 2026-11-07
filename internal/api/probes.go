package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"azule.info/calorize/internal/db"
)

func RegisterProbePaths(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", HealthHandler)
	mux.HandleFunc("GET /readyz", ReadyHandler)
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func ReadyHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		slog.WarnContext(r.Context(), "readiness probe failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unavailable","error":"database unreachable"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}
