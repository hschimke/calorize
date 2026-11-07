package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"time"

	"azule.info/calorize/internal/api"
	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/db"
	"azule.info/calorize/internal/importer"
	"azule.info/calorize/internal/middleware"
	"azule.info/calorize/internal/telemetry"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var GitSHA = "dev"

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
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
		// Adds "source":{"function":"...","file":"...","line":...} to trace log origin
		AddSource: true,
		// Standardize attribute keys or customize timestamps
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Example: rename "msg" to "message" for systems like OpenSearch/Datadog
			if a.Key == slog.MessageKey {
				a.Key = "message"
			}
			return a
		},
	}
	// Change NewTextHandler -> NewJSONHandler
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	ctx := context.Background()
	logger.InfoContext(ctx, "Starting API server...", "build", GitSHA)

	// Everything using slog.Default() (including slog.Info, slog.Error, etc.)
	// will now automatically format as JSON.
	slog.SetDefault(logger)

	telemetry.InitMetrics()

	tp, err := telemetry.InitTracer(context.Background(), "calorize-api", GitSHA)
	if err != nil {
		slog.Error("failed to initialize tracer", "error", err)
	}
	shutdownTracer := func() {
		if tp == nil {
			return
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shutdown tracer provider", "error", err)
		}
	}

	v1Mux := http.NewServeMux()

	// Public Routes
	auth.RegisterAuthPaths(v1Mux) // Auth endpoints must be public
	api.RegisterProbePaths(v1Mux)
	api.RegisterClientEventsPaths(v1Mux)

	// Protected Routes (API)
	apiMux := http.NewServeMux()
	api.RegisterApiPaths(apiMux)
	apiMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		api.WriteError(w, r, db.ErrNotFound)
	})

	// Dev user setup if DEV_AUTH is enabled
	var devUID *db.UserID
	if os.Getenv("DEV_AUTH") == "true" {
		slog.Warn("DEV_AUTH enabled - using insecure dev user authentication")
		devUserID, err := setupDevUser()
		if err != nil {
			slog.Error("failed to setup dev user", "error", err)
			os.Exit(1)
		}
		slog.Info("dev user ready", "user_id", devUserID)
		devUID = &devUserID
	}

	// Apply auth gate to protected routes
	// RecordRoute sits outside RequireAuth so 401s are still labelled by route.
	protectedHandler := middleware.RecordRoute("/api/v1", apiMux, middleware.RequireAuth(apiMux))

	// Mount protected handler at root "/" of v1Mux to catch all non-matched routes
	// Note: Since we registered specific paths on 'v1Mux' above, those will take precedence.
	// Any other path will fall through to "/" which we handle with our protected mux.
	v1Mux.Handle("/", protectedHandler)

	mainMux := http.NewServeMux()
	api.RegisterProbePaths(mainMux)
	mainMux.Handle("/api/v1/", http.StripPrefix("/api/v1", middleware.RecordRoute("/api/v1", v1Mux, v1Mux)))
	mainMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		api.WriteError(w, r, db.ErrNotFound)
	})

	// Global Middleware Pipeline:
	// 1. otelhttp: Traces incoming HTTP requests and propagates W3C trace context
	// 2. RequestTelemetry: Injects/echoes X-Request-Id and X-Calorize-Build
	// 3. Recoverer: Catches panics and serves 500
	// 4. CORS: Handles CORS preflight and access headers
	// 5. ExtractAuth: Non-blocking credential identification (populates user_id in context)
	// 6. Logger: Structured slog access log (runs with user_id & trace_id in context, logs even on panic via defer)
	finalHandler := otelhttp.NewHandler(
		middleware.RequestTelemetry(GitSHA)(
			middleware.Recoverer(
				middleware.CORS(
					middleware.ExtractAuth(devUID)(
						middleware.Logger(middleware.RecordRoute("", mainMux, mainMux)),
					),
				),
			),
		),
		"calorize-api",
	)

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

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	var serverFailed atomic.Bool

	go func() {
		slog.Info("server starting", "addr", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			serverFailed.Store(true)
			select {
			case quit <- syscall.SIGTERM:
			default: // a shutdown is already pending
			}
		}
	}()

	// 5. Start internal metrics server
	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9090"
	}
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{
		Addr:              ":" + metricsPort,
		Handler:           metricsMux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	go func() {
		slog.Info("metrics server starting", "addr", ":"+metricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics server failed", "error", err)
		}
	}()

	// Session cleanup task
	sessionCleanupTicker := time.NewTicker(1 * time.Hour)
	cleanupDone := make(chan struct{})
	var wg sync.WaitGroup

	wg.Go(func() {
		slog.Info("session cleanup daemon starting")
		for {
			select {
			case <-sessionCleanupTicker.C:
				if n, err := db.CleanupExpiredSessions(); err != nil {
					slog.Error("session cleanup failed", "error", err)
				} else {
					telemetry.SessionCleanupsTotal.Add(float64(n))
				}
			case <-cleanupDone:
				slog.Info("session cleanup daemon stopping")
				sessionCleanupTicker.Stop()
				return
			}
		}
	})

	importerTicker := time.NewTicker(24 * time.Hour)
	importerDone := make(chan struct{})
	importDir := os.Getenv("IMPORT_DIR")
	if importDir == "" {
		importDir = "./imports"
	}
	importedDir := os.Getenv("IMPORTED_DIR")
	if importedDir == "" {
		importedDir = "./imported"
	}
	wg.Go(func() {
		slog.Info("importer scanner daemon starting")

		// Run initial scan
		if err := importer.ScanAndImport(importDir, importedDir, importerDone); err != nil {
			slog.Error("initial importer scan failed", "error", err)
		}

		for {
			select {
			case <-importerTicker.C:
				if err := importer.ScanAndImport(importDir, importedDir, importerDone); err != nil {
					slog.Error("importer scan failed", "error", err)
				}
			case <-importerDone:
				slog.Info("importer scanner daemon stopping")
				importerTicker.Stop()
				return
			}
		}
	})

	// Wait for shutdown signal
	<-quit

	slog.Info("server shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	if err := metricsServer.Shutdown(ctx); err != nil {
		slog.Error("metrics server forced to shutdown", "error", err)
	}

	// Signal cleanup to stop
	close(cleanupDone)
	close(importerDone)
	wg.Wait()

	// Close the database connection pool last, after all goroutines have stopped.
	if err := db.Close(); err != nil {
		slog.Error("failed to close database", "error", err)
	} else {
		slog.Info("database closed")
	}

	shutdownTracer()

	if serverFailed.Load() {
		slog.Error("server exited after failure")
		os.Exit(1)
	}
	slog.Info("server exited")
}
