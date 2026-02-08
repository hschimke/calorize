package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

	// Public Routes
	auth.RegisterAuthPaths(mux) // Auth endpoints must be public
	mux.Handle("GET /healthz", http.HandlerFunc(healthHandler))

	// Protected Routes (API)
	apiMux := http.NewServeMux()
	api.RegisterApiPaths(apiMux)

	// Middleware wrapping for protected routes
	var protectedHandler http.Handler
	if os.Getenv("DEV_AUTH") == "true" {
		slog.Warn("DEV_AUTH enabled - using insecure dev user authentication")
		devAuthMiddleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), auth.UserIDContextKey, devUserID)
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}
		// Apply dev auth to apiMux
		protectedHandler = devAuthMiddleware(apiMux)
	} else {
		// Apply real auth to apiMux
		protectedHandler = middleware.RequireAuth(apiMux)
	}

	// Mount protected handler at root "/" to catch all non-matched routes
	// Note: Since we registered specific paths on 'mux' above, those will take precedence.
	// Any other path will fall through to "/" which we handle with our protected mux.
	mux.Handle("/", protectedHandler)

	// Global Middleware (Logger & Recovery) - wraps everything
	// Order: Logger -> Recoverer -> Mux
	// Logger is outer so it logs the completion (even if 500)
	// Recoverer is next so it catches panics from Mux

	// Note: If Recoverer handles a panic, it writes 500. Logger will see that status if using a wrapped writer.

	finalHandler := middleware.Logger(middleware.Recoverer(mux))

	// 4. Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	serverAddr := ":" + port

	server := &http.Server{
		Addr:    serverAddr,
		Handler: finalHandler,
	}

	go func() {
		slog.Info("server starting", "addr", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("server shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server exited")
}

// --- Handler ---

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
}
