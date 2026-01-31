package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"time"

	"azule.info/calorize/internal/api"
	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/db"
	"azule.info/calorize/internal/middleware"
	"github.com/google/uuid"
)

func setupDevUser() (db.UserID, error) {
	name := "dev_user"
	u, err := db.GetUser(name)
	if err != nil {
		return db.UserID(uuid.Nil), err
	}
	if u != nil {
		return u.ID, nil
	}
	// Create
	newUser := db.User{
		Name:      name,
		Email:     "dev@example.com",
		CreatedAt: time.Now(),
	}
	created, err := db.CreateUser(newUser)
	if err != nil {
		return db.UserID(uuid.Nil), err
	}
	return created.ID, nil
}

func main() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	ctx := context.Background()
	logger.InfoContext(ctx, "Starting API server...")

	slog.SetDefault(logger)

	devUserID, err := setupDevUser()
	if err != nil {
		slog.Error("failed to setup dev user", "error", err)
		os.Exit(1)
	}
	slog.Info("dev user ready", "user_id", devUserID)

	mux := http.NewServeMux()

	auth.RegisterAuthPaths(mux)
	api.RegisterApiPaths(mux)

	mux.Handle("GET /hello/{name}", http.HandlerFunc(helloHandler))

	// Middleware
	var finalHandler http.Handler
	if os.Getenv("DEV_AUTH") == "true" {
		slog.Warn("DEV_AUTH enabled - using insecure dev user authentication")
		devAuthMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), auth.UserIDContextKey, devUserID)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
		finalHandler = middleware.Logger(devAuthMiddleware(mux))
	} else {
		finalHandler = middleware.Logger(middleware.RequireAuth(mux))
	}

	// 4. Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	serverAddr := ":" + port
	slog.Info("server starting", "addr", serverAddr)

	err = http.ListenAndServe(serverAddr, finalHandler)
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
