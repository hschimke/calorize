package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func main() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	ctx := context.Background()
	logger.InfoContext(ctx, "Starting API server...")

	slog.SetDefault(logger)

	mux := http.NewServeMux()

	mux.Handle("GET /hello/{name}", loggingMiddleware(http.HandlerFunc(helloHandler)))

	// 4. Start the server
	serverAddr := ":8080"
	slog.Info("server starting", "addr", serverAddr)
	
	err := http.ListenAndServe(serverAddr, mux)
	if err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Move to the next handler
		next.ServeHTTP(w, r.WithContext(r.Context()))

		// Log the completion of the request
		slog.Info("request processed",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// --- Handler ---

func helloHandler(w http.ResponseWriter, r *http.Request) {
	// Extracting path parameter
	name := r.PathValue("name")
	
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello, " + name + "!"))
}