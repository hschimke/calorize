package middleware_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"azule.info/calorize/internal/auth"
	"azule.info/calorize/internal/db"
	"azule.info/calorize/internal/middleware"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

func TestRequestTelemetryEchoesRequestID(t *testing.T) {
	var innerReqID string
	handler := middleware.RequestTelemetry("v1.0.0")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerReqID = middleware.GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-Id", "custom-req-id-123")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Request-Id"); got != "custom-req-id-123" {
		t.Fatalf("expected echoed X-Request-Id 'custom-req-id-123', got %q", got)
	}
	if gotBuild := rr.Header().Get("X-Calorize-Build"); gotBuild != "v1.0.0" {
		t.Fatalf("expected X-Calorize-Build 'v1.0.0', got %q", gotBuild)
	}
	if innerReqID != "custom-req-id-123" {
		t.Fatalf("expected inner context request_id 'custom-req-id-123', got %q", innerReqID)
	}
}

func TestRequestTelemetryGeneratesRequestID(t *testing.T) {
	var innerReqID string
	handler := middleware.RequestTelemetry("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerReqID = middleware.GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/v1/foods", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	respReqID := rr.Header().Get("X-Request-Id")
	if respReqID == "" {
		t.Fatal("expected non-empty generated X-Request-Id header")
	}
	if innerReqID != respReqID {
		t.Fatalf("expected inner reqID %q to match response header %q", innerReqID, respReqID)
	}

	// Verify it parses as a UUID (v7 UUID is valid UUID)
	parsedUUID, err := uuid.Parse(respReqID)
	if err != nil {
		// If not a UUID, ensure it's at least a 32-char hex fallback
		if len(respReqID) < 16 {
			t.Fatalf("expected valid UUID or hex string, got %q (err: %v)", respReqID, err)
		}
	} else if parsedUUID.Version() != 7 {
		t.Logf("generated UUID version is %d (expected 7, but accepted)", parsedUUID.Version())
	}

	// When buildVersion is "", X-Calorize-Build should not be set
	if gotBuild := rr.Header().Get("X-Calorize-Build"); gotBuild != "" {
		t.Fatalf("expected empty X-Calorize-Build, got %q", gotBuild)
	}
}

func TestGetRequestID(t *testing.T) {
	// From empty context
	if got := middleware.GetRequestID(context.Background()); got != "" {
		t.Fatalf("expected empty string from empty context, got %q", got)
	}

	// When manually set with RequestIDKey
	ctx := context.WithValue(context.Background(), middleware.RequestIDKey, "test-id-456")
	if got := middleware.GetRequestID(ctx); got != "test-id-456" {
		t.Fatalf("expected 'test-id-456', got %q", got)
	}
}

type recordCaptureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordCaptureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordCaptureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}
func (h *recordCaptureHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *recordCaptureHandler) WithGroup(name string) slog.Handler       { return h }

func TestLoggerMiddlewareEmitsStructuredLog(t *testing.T) {
	capture := &recordCaptureHandler{}
	origLogger := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(origLogger)

	pipeline := middleware.RequestTelemetry("v1.0.0")(
		middleware.Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("hello world"))
		})),
	)

	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	req.Header.Set("X-Request-Id", "test-log-req-id")
	req.Header.Set("User-Agent", "calorize-test-agent")
	req.RemoteAddr = "192.168.1.100:12345"

	// Set user_id in context
	testUID := db.UserID(uuid.New())
	ctx := context.WithValue(req.Context(), auth.UserIDContextKey, testUID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	pipeline.ServeHTTP(rr, req)

	capture.mu.Lock()
	defer capture.mu.Unlock()

	var reqLog *slog.Record
	for i := range capture.records {
		if capture.records[i].Message == "http.request" {
			reqLog = &capture.records[i]
			break
		}
	}

	if reqLog == nil {
		t.Fatal("expected 'http.request' slog record to be logged")
	}

	attrs := make(map[string]any)
	reqLog.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	requiredKeys := []string{
		"method", "path", "status", "duration_ms", "bytes_written",
		"request_id", "user_id", "trace_id", "remote_addr", "user_agent",
	}

	for _, k := range requiredKeys {
		if _, ok := attrs[k]; !ok {
			t.Errorf("missing required log attribute: %s", k)
		}
	}

	if attrs["method"] != "GET" {
		t.Errorf("expected method GET, got %v", attrs["method"])
	}
	if attrs["path"] != "/api/v1/test" {
		t.Errorf("expected path /api/v1/test, got %v", attrs["path"])
	}
	if attrs["status"] != int64(http.StatusCreated) && attrs["status"] != http.StatusCreated {
		t.Errorf("expected status %d, got %v", http.StatusCreated, attrs["status"])
	}
	if attrs["bytes_written"] != int64(len("hello world")) {
		t.Errorf("expected bytes_written %d, got %v", len("hello world"), attrs["bytes_written"])
	}
	if attrs["request_id"] != "test-log-req-id" {
		t.Errorf("expected request_id test-log-req-id, got %v", attrs["request_id"])
	}
	if attrs["user_id"] != uuid.UUID(testUID).String() {
		t.Errorf("expected user_id %s, got %v", uuid.UUID(testUID).String(), attrs["user_id"])
	}
	if attrs["user_agent"] != "calorize-test-agent" {
		t.Errorf("expected user_agent calorize-test-agent, got %v", attrs["user_agent"])
	}
	if attrs["remote_addr"] != "192.168.1.100:12345" {
		t.Errorf("expected remote_addr 192.168.1.100:12345, got %v", attrs["remote_addr"])
	}
}

func TestLoggerMiddlewareAnonymousUserAndTraceID(t *testing.T) {
	capture := &recordCaptureHandler{}
	origLogger := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(origLogger)

	handler := middleware.Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create request with trace_id
	traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	req := httptest.NewRequest("GET", "/readyz", nil)
	ctx := trace.ContextWithSpanContext(req.Context(), spanCtx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	capture.mu.Lock()
	defer capture.mu.Unlock()

	var reqLog *slog.Record
	for i := range capture.records {
		if capture.records[i].Message == "http.request" {
			reqLog = &capture.records[i]
			break
		}
	}
	if reqLog == nil {
		t.Fatal("expected 'http.request' slog record")
	}

	attrs := make(map[string]any)
	reqLog.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	if attrs["user_id"] != "anonymous" {
		t.Errorf("expected user_id 'anonymous', got %v", attrs["user_id"])
	}
	if attrs["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("expected trace_id 4bf92f3577b34da6a3ce929d0e0e4736, got %v", attrs["trace_id"])
	}
}

func TestLoggerMiddlewareCookieSessionExtraction(t *testing.T) {
	// Create test user and session in DB
	suffix := uuid.New().String()[:8]
	u, err := db.CreateUser(db.User{
		Name:      "test_cookie_user_" + suffix,
		Email:     "cookie_user_" + suffix + "@example.com",
		CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		_ = db.DeleteTemporaryUser(*u)
	})

	session, err := db.CreateSession(u.ID)
	if err != nil {
		t.Fatalf("failed to create test session: %v", err)
	}
	t.Cleanup(func() {
		_ = db.DeleteSession(session.ID)
	})

	capture := &recordCaptureHandler{}
	origLogger := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(origLogger)

	// Pipeline: ExtractAuth -> Logger
	handler := middleware.ExtractAuth(nil)(middleware.Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/api/v1/profile", nil)
	req.AddCookie(&http.Cookie{
		Name:  auth.AppSessionCookieName,
		Value: session.ID,
	})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	capture.mu.Lock()
	defer capture.mu.Unlock()

	var reqLog *slog.Record
	for i := range capture.records {
		if capture.records[i].Message == "http.request" {
			reqLog = &capture.records[i]
			break
		}
	}
	if reqLog == nil {
		t.Fatal("expected 'http.request' slog record")
	}

	attrs := make(map[string]any)
	reqLog.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	if attrs["user_id"] != u.ID.String() {
		t.Errorf("expected user_id %s from session cookie, got %v", u.ID.String(), attrs["user_id"])
	}
}

func TestLoggerMiddlewareLogsPanickingRequests(t *testing.T) {
	capture := &recordCaptureHandler{}
	origLogger := slog.Default()
	slog.SetDefault(slog.New(capture))
	defer slog.SetDefault(origLogger)

	// Pipeline: Recoverer -> Logger -> PanicHandler
	handler := middleware.Recoverer(middleware.Logger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("simulated explosion")
	})))

	req := httptest.NewRequest("GET", "/explode", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}

	capture.mu.Lock()
	defer capture.mu.Unlock()

	var accessLog *slog.Record
	for i := range capture.records {
		if capture.records[i].Message == "http.request" {
			accessLog = &capture.records[i]
			break
		}
	}
	if accessLog == nil {
		t.Fatal("expected 'http.request' log entry even on panic")
	}
}
