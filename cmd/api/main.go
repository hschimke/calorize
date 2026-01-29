package main

import (
	"log/slog"
	"net/http"
	"os"

	"azule.info/calorize/internal/api"
	"azule.info/calorize/internal/database"
)

func main() {
	// Initialize structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	dbPath := "calorize.db"
	if os.Getenv("DB_PATH") != "" {
		dbPath = os.Getenv("DB_PATH")
	}

	if err := database.InitDB(dbPath); err != nil {
		slog.Error("Failed to init database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	slog.Info("Database initialized successfully", "path", dbPath)

	// Start Server
	server := api.NewServer()

	// Middleware Chain: RequestLogger -> Recoverer -> CORS -> Mux
	// CORS Middleware
	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		server.ServeHTTP(w, r)
	})

	// Wrap with logging and recovery
	handler := api.RequestLogger(api.Recoverer(corsHandler))

	port := "8383"
	slog.Info("Server starting", "port", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
