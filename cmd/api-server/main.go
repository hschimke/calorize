package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
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

	v1Mux := http.NewServeMux()

	// Public Routes
	auth.RegisterAuthPaths(v1Mux) // Auth endpoints must be public
	v1Mux.Handle("GET /healthz", http.HandlerFunc(healthHandler))

	// Protected Routes (API)
	apiMux := http.NewServeMux()
	api.RegisterApiPaths(apiMux)

	// Middleware wrapping for protected routes
	var protectedHandler http.Handler
	if os.Getenv("DEV_AUTH") == "true" {
		slog.Warn("DEV_AUTH enabled - using insecure dev user authentication")
		devUserID, err := setupDevUser()
		if err != nil {
			slog.Error("failed to setup dev user", "error", err)
			os.Exit(1)
		}
		slog.Info("dev user ready", "user_id", devUserID)
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

	// Mount protected handler at root "/" of v1Mux to catch all non-matched routes
	// Note: Since we registered specific paths on 'v1Mux' above, those will take precedence.
	// Any other path will fall through to "/" which we handle with our protected mux.
	v1Mux.Handle("/", protectedHandler)

	mainMux := http.NewServeMux()
	mainMux.Handle("/api/v1/", http.StripPrefix("/api/v1", v1Mux))

	// Global Middleware (Logger & Recovery) - wraps everything
	// Order: Logger -> Recoverer -> Mux
	// Logger is outer so it logs the completion (even if 500)
	// Recoverer is next so it catches panics from Mux

	// Note: If Recoverer handles a panic, it writes 500. Logger will see that status if using a wrapped writer.

	finalHandler := middleware.Logger(middleware.Recoverer(middleware.CORS(mainMux)))

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

	// Session cleanup task
	sessionCleanupTicker := time.NewTicker(1 * time.Hour)
	cleanupDone := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("session cleanup daemon starting")
		for {
			select {
			case <-sessionCleanupTicker.C:
				if err := db.CleanupExpiredSessions(); err != nil {
					slog.Error("session cleanup failed", "error", err)
				}
			case <-cleanupDone:
				slog.Info("session cleanup daemon stopping")
				sessionCleanupTicker.Stop()
				return
			}
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

	// Signal cleanup to stop
	close(cleanupDone)
	wg.Wait()

	slog.Info("server exited")
}

// --- Handler ---

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
}
