package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"azule.info/calorize/internal/api"
	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/middleware"
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	ctx := context.Background()
	logger.InfoContext(ctx, "Starting API server...")

	slog.SetDefault(logger)

	mux := http.NewServeMux()

	auth.RegisterAuthPaths(mux)
	api.RegisterApiPaths(mux)

	mux.Handle("GET /hello/{name}", http.HandlerFunc(helloHandler))

	// Wrap the entire mux with the logging middleware
	finalHandler := middleware.Logger(mux)

	// 4. Start the server
	serverAddr := ":8080"
	slog.Info("server starting", "addr", serverAddr)

	err := http.ListenAndServe(serverAddr, finalHandler)
	if err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// --- Handler ---

func helloHandler(w http.ResponseWriter, r *http.Request) {
	// Extracting path parameter
	name := r.PathValue("name")

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello, " + name + "!"))
}
